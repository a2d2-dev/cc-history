package display_test

import (
	"strings"
	"testing"
	"time"

	"github.com/a2d2-dev/cc-history/internal/display"
	"github.com/a2d2-dev/cc-history/internal/parser"
)

func ts(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func TestPrintSession_TextMessage(t *testing.T) {
	session := &parser.Session{
		ID: "test",
		Messages: []*parser.Message{
			{
				Role:      "user",
				Timestamp: ts("2024-01-01T10:00:00Z"),
				Text:      "Hello, what is the weather?",
			},
			{
				Role:      "assistant",
				Timestamp: ts("2024-01-01T10:00:01Z"),
				Text:      "I don't have access to real-time weather data.",
			},
		},
	}

	var b strings.Builder
	display.PrintSession(&b, session)
	out := b.String()

	if !strings.Contains(out, "[10:00:00]") {
		t.Errorf("missing timestamp in output: %s", out)
	}
	if !strings.Contains(out, "user") {
		t.Errorf("missing user role: %s", out)
	}
	if !strings.Contains(out, "asst") {
		t.Errorf("missing asst role: %s", out)
	}
	if !strings.Contains(out, "Hello, what is the weather?") {
		t.Errorf("missing user message text: %s", out)
	}
	if !strings.Contains(out, "I don't have access") {
		t.Errorf("missing assistant text: %s", out)
	}
}

func TestPrintSession_ToolCall(t *testing.T) {
	session := &parser.Session{
		ID: "test",
		Messages: []*parser.Message{
			{
				Role:      "assistant",
				Timestamp: ts("2024-01-01T10:00:00Z"),
				ToolCalls: []*parser.ToolCall{
					{
						ID:        "tc1",
						Name:      "Read",
						Arguments: `{"file_path":"/etc/hosts"}`,
					},
				},
			},
		},
	}

	var b strings.Builder
	display.PrintSession(&b, session)
	out := b.String()

	if !strings.Contains(out, "Read") {
		t.Errorf("missing tool name in output: %s", out)
	}
	if !strings.Contains(out, "file_path") {
		t.Errorf("missing tool argument key in output: %s", out)
	}
}

func TestPrintSession_SkipsNoRoleMessages(t *testing.T) {
	session := &parser.Session{
		ID: "test",
		Messages: []*parser.Message{
			{
				Role:      "",
				Timestamp: ts("2024-01-01T10:00:00Z"),
				Text:      "system noise",
			},
			{
				Role:      "user",
				Timestamp: ts("2024-01-01T10:00:01Z"),
				Text:      "real message",
			},
		},
	}

	var b strings.Builder
	display.PrintSession(&b, session)
	out := b.String()

	if strings.Contains(out, "system noise") {
		t.Errorf("should skip messages with no role, but got: %s", out)
	}
	if !strings.Contains(out, "real message") {
		t.Errorf("should include user message: %s", out)
	}
}

func TestPrintSession_LongTextTruncated(t *testing.T) {
	longText := strings.Repeat("a", 200)
	session := &parser.Session{
		ID: "test",
		Messages: []*parser.Message{
			{
				Role:      "user",
				Timestamp: ts("2024-01-01T10:00:00Z"),
				Text:      longText,
			},
		},
	}

	var b strings.Builder
	display.PrintSession(&b, session)
	out := b.String()

	// Should contain truncation indicator.
	if !strings.Contains(out, "…") {
		t.Errorf("expected truncation indicator for long text: %s", out)
	}
	// Full text should not appear.
	if strings.Contains(out, longText) {
		t.Error("long text should be truncated, not printed in full")
	}
}

// TC-1: User message with only a normal (non-error) tool_result should be silent.
func TestPrintSession_ToolResultOnly_NoOutput(t *testing.T) {
	session := &parser.Session{
		ID: "test",
		Messages: []*parser.Message{
			{
				Role:      "user",
				Timestamp: ts("2024-01-01T13:18:43Z"),
				ToolResults: []*parser.ToolResult{
					{ToolUseID: "tc1", ToolName: "Edit", Content: "ok", IsError: false},
				},
			},
		},
	}

	var b strings.Builder
	display.PrintSession(&b, session)
	out := b.String()

	if strings.Contains(out, "(no content)") {
		t.Errorf("should not print '(no content)' for tool_result-only message: %s", out)
	}
	// No message line should appear for this user turn.
	if strings.Contains(out, "[13:18:43]") {
		t.Errorf("should produce no message line for successful tool_result-only message: %s", out)
	}
}

// TC-2: User message with only an error tool_result should surface the error.
func TestPrintSession_ToolResultError_ShowsError(t *testing.T) {
	session := &parser.Session{
		ID: "test",
		Messages: []*parser.Message{
			{
				Role:      "user",
				Timestamp: ts("2024-01-01T13:18:43Z"),
				ToolResults: []*parser.ToolResult{
					{ToolUseID: "tc1", ToolName: "Bash", Content: "permission denied", IsError: true},
				},
			},
		},
	}

	var b strings.Builder
	display.PrintSession(&b, session)
	out := b.String()

	if !strings.Contains(out, "tool error") {
		t.Errorf("should print 'tool error' for error tool_result: %s", out)
	}
	if !strings.Contains(out, "Bash") {
		t.Errorf("should include tool name in error line: %s", out)
	}
	if strings.Contains(out, "(no content)") {
		t.Errorf("should not print '(no content)' for error tool_result: %s", out)
	}
}

// TC-3: Assistant message with only thinking blocks should produce no output line.
func TestPrintSession_ThinkingOnly_Silent(t *testing.T) {
	session := &parser.Session{
		ID: "test",
		Messages: []*parser.Message{
			{
				Role:        "assistant",
				Timestamp:   ts("2024-01-01T10:00:00Z"),
				HasThinking: true,
				// Text and ToolCalls are empty.
			},
		},
	}

	var b strings.Builder
	display.PrintSession(&b, session)
	out := b.String()

	if strings.Contains(out, "(no content)") {
		t.Errorf("thinking-only assistant message should not produce '(no content)': %s", out)
	}
	if strings.Contains(out, "[10:00:00]") {
		t.Errorf("thinking-only assistant message should produce no message line: %s", out)
	}
}

// TC-4: Session header should appear at the top of PrintSession output.
func TestPrintSession_Header(t *testing.T) {
	session := &parser.Session{
		ID:       "3b8fc2a1-1234-5678-abcd-000000000000",
		FilePath: "/home/user/.claude/sessions/test.jsonl",
		Messages: []*parser.Message{
			{
				Role:      "user",
				Timestamp: ts("2024-01-01T10:00:00Z"),
				Text:      "hello",
			},
			{
				Role:      "assistant",
				Timestamp: ts("2024-01-01T10:05:30Z"),
				Text:      "hi there",
			},
		},
	}

	var b strings.Builder
	display.PrintSession(&b, session)
	out := b.String()

	if !strings.HasPrefix(out, "─── session") {
		t.Errorf("output should start with session header: %s", out)
	}
	if !strings.Contains(out, "3b8fc2a1") {
		t.Errorf("header should include abbreviated session ID: %s", out)
	}
	if !strings.Contains(out, "test.jsonl") {
		t.Errorf("header should include file path: %s", out)
	}
	if !strings.Contains(out, "2 messages") {
		t.Errorf("header should show message count: %s", out)
	}
	if !strings.Contains(out, "2024-01-01") {
		t.Errorf("header should include date: %s", out)
	}
}

// TC-5: Normal user text message should still display correctly (regression guard).
func TestPrintSession_NormalTextUnchanged(t *testing.T) {
	session := &parser.Session{
		ID: "test",
		Messages: []*parser.Message{
			{
				Role:      "user",
				Timestamp: ts("2024-01-01T10:00:00Z"),
				Text:      "please help me",
			},
		},
	}

	var b strings.Builder
	display.PrintSession(&b, session)
	out := b.String()

	if !strings.Contains(out, "please help me") {
		t.Errorf("normal text message should still be displayed: %s", out)
	}
	if !strings.Contains(out, "[10:00:00]") {
		t.Errorf("normal text message timestamp missing: %s", out)
	}
}
