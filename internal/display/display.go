// Package display formats parsed session messages for CLI output.
package display

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/a2d2-dev/cc-history/internal/parser"
)

const (
	summaryMaxRunes = 120
	argMaxRunes     = 80
)

// PrintSession writes all messages in session to w in chronological order.
// Format per line: [HH:MM:SS]  role  summary
func PrintSession(w io.Writer, session *parser.Session) {
	for _, msg := range session.Messages {
		printMessage(w, msg)
	}
}

func printMessage(w io.Writer, msg *parser.Message) {
	// Skip messages with no role (e.g. system/turn_duration records).
	if msg.Role == "" {
		return
	}

	ts := msg.Timestamp.Format("15:04:05")
	role := formatRole(msg.Role)

	// Print text summary if present.
	if text := strings.TrimSpace(msg.Text); text != "" {
		fmt.Fprintf(w, "[%s]  %s  %s\n", ts, role, truncate(text, summaryMaxRunes))
	}

	// Print each tool call on its own line.
	for _, tc := range msg.ToolCalls {
		fmt.Fprintf(w, "[%s]  %s  [%s %s]\n", ts, role, tc.Name, formatArgs(tc.Arguments))
	}

	// If no text and no tool calls, still emit a blank-content line.
	if strings.TrimSpace(msg.Text) == "" && len(msg.ToolCalls) == 0 {
		fmt.Fprintf(w, "[%s]  %s  (no content)\n", ts, role)
	}
}

// formatRole returns a fixed-width role label.
func formatRole(role string) string {
	switch role {
	case "user":
		return "user"
	case "assistant":
		return "asst"
	default:
		r := role
		if utf8.RuneCountInString(r) > 4 {
			runes := []rune(r)
			r = string(runes[:4])
		}
		return fmt.Sprintf("%-4s", r)
	}
}

// truncate returns s truncated to maxRunes runes, appending "…" if cut.
func truncate(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "…"
}

// formatArgs extracts a short human-readable summary from JSON tool arguments.
// It shows up to the first 2 key=value pairs and truncates long values.
func formatArgs(raw string) string {
	if raw == "" || raw == "{}" {
		return ""
	}

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		// Not valid JSON — show raw truncated.
		return truncate(raw, argMaxRunes)
	}

	var parts []string
	count := 0
	for k, v := range m {
		if count >= 2 {
			parts = append(parts, "…")
			break
		}
		valStr := fmt.Sprintf("%v", v)
		valStr = truncate(strings.ReplaceAll(valStr, "\n", "\\n"), argMaxRunes)
		parts = append(parts, k+"="+valStr)
		count++
	}
	return strings.Join(parts, " ")
}
