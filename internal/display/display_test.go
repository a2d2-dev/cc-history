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
