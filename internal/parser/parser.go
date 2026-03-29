// Package parser parses Claude Code JSONL session files into Go structs.
package parser

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

// --------------------------------------------------------------------------
// Raw JSON envelope types (unexported)
// --------------------------------------------------------------------------

type rawRecord struct {
	Type        string          `json:"type"`
	Subtype     string          `json:"subtype"`
	UUID        string          `json:"uuid"`
	ParentUUID  *string         `json:"parentUuid"`
	SessionID   string          `json:"sessionId"`
	Timestamp   string          `json:"timestamp"`
	IsSidechain bool            `json:"isSidechain"`
	GitBranch   string          `json:"gitBranch"`
	CWD         string          `json:"cwd"`
	Version     string          `json:"version"`
	Message     *rawMessage     `json:"message"`
	DurationMs  *float64        `json:"durationMs"`
	// ToolUseResult holds duration and other metadata for tool_result user messages.
	ToolUseResult json.RawMessage `json:"toolUseResult"`
}

type rawMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
	Model   string          `json:"model"`
	ID      string          `json:"id"`
}

type rawContentItem struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	ID        string          `json:"id"`          // tool_use
	Name      string          `json:"name"`        // tool_use
	Input     json.RawMessage `json:"input"`       // tool_use
	ToolUseID string          `json:"tool_use_id"` // tool_result
	Content   json.RawMessage `json:"content"`     // tool_result (string or array)
	IsError   bool            `json:"is_error"`    // tool_result
}

type rawDuration struct {
	DurationMs float64 `json:"durationMs"`
}

// --------------------------------------------------------------------------
// Public domain model
// --------------------------------------------------------------------------

// Session represents one parsed JSONL session file.
type Session struct {
	ID       string
	FilePath string
	Messages []*Message
}

// Message is a single entry in a Claude Code session conversation.
type Message struct {
	UUID        string
	ParentUUID  string
	SessionID   string
	Type        string // "user" | "assistant" | "system"
	Subtype     string // e.g. "turn_duration", "api_error"
	Timestamp   time.Time
	Role        string // "user" | "assistant"
	Text        string // combined text content
	IsSidechain bool
	GitBranch   string
	CWD         string
	Version     string
	// ToolCalls is populated for assistant messages that invoke tools.
	ToolCalls []*ToolCall
	// ToolResults is populated for user messages that return tool output.
	ToolResults []*ToolResult
	// HasThinking is true when the message contained at least one thinking block.
	HasThinking bool
	// TurnDurationMs is set for system/turn_duration records.
	TurnDurationMs float64
}

// ToolCall represents a single tool invocation by the assistant.
type ToolCall struct {
	ID        string
	Name      string
	Arguments string // JSON-encoded input
	// Fields below are populated during the linking pass.
	Result     string
	IsError    bool
	DurationMs float64
}

// ToolResult is the output of a tool call, found in a user message.
type ToolResult struct {
	ToolUseID  string
	ToolName   string // populated during link pass from matching ToolCall
	Content    string
	IsError    bool
	DurationMs float64
}

// --------------------------------------------------------------------------
// Parsing entry points
// --------------------------------------------------------------------------

// ParseFile reads a JSONL session file at path and returns a Session.
// Malformed or empty files never cause a panic; corrupted lines are skipped
// with a log warning.
func ParseFile(path string) (*Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	return ParseReader(path, f)
}

// ParseReader parses JSONL from r, tagging results with filePath.
// It is safe to call on empty or partially-written readers.
func ParseReader(filePath string, r io.Reader) (*Session, error) {
	session := &Session{FilePath: filePath}

	scanner := bufio.NewScanner(r)
	// 16 MB per line — tool output can be large.
	const maxBuf = 16 * 1024 * 1024
	scanner.Buffer(make([]byte, maxBuf), maxBuf)

	// toolCallMap maps tool_use id → ToolCall pointer for the link pass.
	toolCallMap := make(map[string]*ToolCall)
	// toolResultMap maps tool_use_id → ToolResult pointer for the link pass.
	toolResultMap := make(map[string]*ToolResult)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec rawRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			log.Printf("warning: %s line %d: malformed JSON, skipping: %v", filePath, lineNum, err)
			continue
		}

		if session.ID == "" && rec.SessionID != "" {
			session.ID = rec.SessionID
		}

		switch rec.Type {
		case "user", "assistant", "system":
			msg := buildMessage(&rec)
			for _, tc := range msg.ToolCalls {
				toolCallMap[tc.ID] = tc
			}
			for _, tr := range msg.ToolResults {
				toolResultMap[tr.ToolUseID] = tr
			}
			session.Messages = append(session.Messages, msg)
		// Other types (progress, queue-operation, file-history-snapshot, …) are ignored.
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", filePath, err)
	}

	// Link pass: populate ToolCall.Result/IsError/DurationMs from matching results,
	// and back-fill ToolResult.ToolName from the matching ToolCall.
	for id, tr := range toolResultMap {
		if tc, ok := toolCallMap[id]; ok {
			tc.Result = tr.Content
			tc.IsError = tr.IsError
			tc.DurationMs = tr.DurationMs
			tr.ToolName = tc.Name
		}
	}

	return session, nil
}

// --------------------------------------------------------------------------
// Internal helpers
// --------------------------------------------------------------------------

func buildMessage(rec *rawRecord) *Message {
	ts, _ := time.Parse(time.RFC3339Nano, rec.Timestamp)

	parentUUID := ""
	if rec.ParentUUID != nil {
		parentUUID = *rec.ParentUUID
	}

	msg := &Message{
		UUID:        rec.UUID,
		ParentUUID:  parentUUID,
		SessionID:   rec.SessionID,
		Type:        rec.Type,
		Subtype:     rec.Subtype,
		Timestamp:   ts,
		IsSidechain: rec.IsSidechain,
		GitBranch:   rec.GitBranch,
		CWD:         rec.CWD,
		Version:     rec.Version,
	}

	if rec.DurationMs != nil {
		msg.TurnDurationMs = *rec.DurationMs
	}

	if rec.Message != nil {
		msg.Role = rec.Message.Role
		parseContent(msg, rec)
	}

	return msg
}

func parseContent(msg *Message, rec *rawRecord) {
	raw := rec.Message.Content
	if len(raw) == 0 {
		return
	}

	switch raw[0] {
	case '"':
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			msg.Text = s
		} else {
			log.Printf("warning: session %s uuid %s: bad string content: %v", rec.SessionID, rec.UUID, err)
		}

	case '[':
		var items []rawContentItem
		if err := json.Unmarshal(raw, &items); err != nil {
			log.Printf("warning: session %s uuid %s: bad content array: %v", rec.SessionID, rec.UUID, err)
			return
		}
		var textParts []string
		for _, item := range items {
			switch item.Type {
			case "text":
				textParts = append(textParts, item.Text)
			case "tool_use":
				args := "{}"
				if len(item.Input) > 0 {
					args = string(item.Input)
				}
				msg.ToolCalls = append(msg.ToolCalls, &ToolCall{
					ID:        item.ID,
					Name:      item.Name,
					Arguments: args,
				})
			case "tool_result":
				dur := extractDurationMs(rec.ToolUseResult)
				msg.ToolResults = append(msg.ToolResults, &ToolResult{
					ToolUseID:  item.ToolUseID,
					Content:    extractToolResultContent(item.Content),
					IsError:    item.IsError,
					DurationMs: dur,
				})
			case "thinking":
				// Internal reasoning; do not extract text, but mark presence.
				msg.HasThinking = true
			}
		}
		msg.Text = strings.Join(textParts, "\n")
	}
}

// extractToolResultContent handles tool_result content which may be a plain
// string or an array of content blocks.
func extractToolResultContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	switch raw[0] {
	case '"':
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
	case '[':
		var items []rawContentItem
		if err := json.Unmarshal(raw, &items); err == nil {
			var parts []string
			for _, it := range items {
				if it.Type == "text" {
					parts = append(parts, it.Text)
				}
			}
			return strings.Join(parts, "\n")
		}
	}
	return string(raw)
}

// extractDurationMs extracts the durationMs field from a toolUseResult blob.
func extractDurationMs(raw json.RawMessage) float64 {
	if len(raw) == 0 {
		return 0
	}
	var d rawDuration
	if err := json.Unmarshal(raw, &d); err != nil {
		return 0
	}
	return d.DurationMs
}
