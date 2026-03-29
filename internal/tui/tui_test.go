package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/a2d2-dev/cc-history/internal/parser"
)

func makeSession(msgs []*parser.Message) *parser.Session {
	return &parser.Session{ID: "test", Messages: msgs}
}

func makeMsg(role, text string) *parser.Message {
	return &parser.Message{
		Role:      role,
		Text:      text,
		Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
	}
}

func TestNewModel_Empty(t *testing.T) {
	m := newModel(makeSession(nil))
	if m.totalLines != 0 {
		t.Errorf("expected 0 items for empty session, got %d", m.totalLines)
	}
}

func TestNewModel_UserAssistantMessages(t *testing.T) {
	s := makeSession([]*parser.Message{
		makeMsg("user", "hello"),
		makeMsg("assistant", "world"),
	})
	m := newModel(s)
	if m.totalLines < 2 {
		t.Errorf("expected at least 2 items, got %d", m.totalLines)
	}
	// Check role labels appear in rendered text.
	full := strings.Join(collectTexts(m.items), "\n")
	if !strings.Contains(full, "user") {
		t.Error("expected 'user' in rendered output")
	}
	if !strings.Contains(full, "asst") {
		t.Error("expected 'asst' in rendered output")
	}
}

func TestToolCallFoldToggle(t *testing.T) {
	tc := &parser.ToolCall{ID: "1", Name: "ReadFile", Arguments: `{"path":"/tmp/x"}`}
	msg := &parser.Message{
		Role:      "assistant",
		Text:      "using tool",
		Timestamp: time.Now(),
		ToolCalls: []*parser.ToolCall{tc},
	}
	s := makeSession([]*parser.Message{msg})
	m := newModel(s)

	countBefore := m.totalLines

	// Expand tool calls for message 0.
	m.expanded[0] = true
	m.rebuildItems()
	countAfter := m.totalLines

	if countAfter <= countBefore {
		t.Errorf("expected more items when expanded (%d <= %d)", countAfter, countBefore)
	}
}

func TestWordWrap(t *testing.T) {
	long := strings.Repeat("word ", 20)
	wrapped := wordWrap(long, 20)
	for _, line := range strings.Split(wrapped, "\n") {
		if len([]rune(line)) > 22 { // allow slight overrun for long words
			t.Errorf("line too long (%d): %q", len([]rune(line)), line)
		}
	}
}

func TestScrollClamp(t *testing.T) {
	msgs := make([]*parser.Message, 50)
	for i := range msgs {
		msgs[i] = makeMsg("user", "line")
	}
	m := newModel(makeSession(msgs))
	m.height = 10
	m.cursor = 9999
	m.clampCursor()
	if m.cursor > m.maxCursor() {
		t.Errorf("cursor %d exceeds maxCursor %d", m.cursor, m.maxCursor())
	}
}

// collectTexts extracts all item texts.
func collectTexts(items []item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.text
	}
	return out
}
