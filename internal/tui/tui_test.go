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

func TestSearchRecomputeMatches(t *testing.T) {
	s := makeSession([]*parser.Message{
		makeMsg("user", "hello world"),
		makeMsg("assistant", "goodbye moon"),
	})
	m := newModel(s)

	m.searchQuery = "hello"
	m.recomputeMatches()
	if len(m.matches) == 0 {
		t.Error("expected matches for 'hello'")
	}

	m.searchQuery = "moon"
	m.recomputeMatches()
	if len(m.matches) == 0 {
		t.Error("expected matches for 'moon'")
	}

	m.searchQuery = "zzznomatch"
	m.recomputeMatches()
	if len(m.matches) != 0 {
		t.Errorf("expected 0 matches for 'zzznomatch', got %d", len(m.matches))
	}

	// Case-insensitive.
	m.searchQuery = "HELLO"
	m.recomputeMatches()
	if len(m.matches) == 0 {
		t.Error("expected case-insensitive match for 'HELLO'")
	}

	// Clear search.
	m.searchQuery = ""
	m.recomputeMatches()
	if len(m.matches) != 0 {
		t.Error("expected 0 matches when query is empty")
	}
}

func TestSearchScrollToMatch(t *testing.T) {
	msgs := make([]*parser.Message, 100)
	for i := range msgs {
		text := "line"
		if i == 80 {
			text = "findme"
		}
		msgs[i] = makeMsg("user", text)
	}
	m := newModel(makeSession(msgs))
	m.height = 20
	m.cursor = 0

	m.searchQuery = "findme"
	m.recomputeMatches()
	if len(m.matches) == 0 {
		t.Fatal("expected at least one match")
	}
	m.matchCursor = 0
	m.scrollToMatch()

	matchIdx := m.matches[0]
	vh := m.viewHeight()
	if matchIdx < m.cursor || matchIdx >= m.cursor+vh {
		t.Errorf("match at item %d not visible in viewport [%d, %d)", matchIdx, m.cursor, m.cursor+vh)
	}
}

func TestSessionSwitcher(t *testing.T) {
	s1 := makeSession([]*parser.Message{makeMsg("user", "session one")})
	s2 := makeSession([]*parser.Message{makeMsg("user", "session two")})
	s1.ID = "s1"
	s2.ID = "s2"

	m := newModelMulti([]*parser.Session{s1, s2}, 0)
	if m.sessionIdx != 0 {
		t.Errorf("expected sessionIdx 0, got %d", m.sessionIdx)
	}
	if m.session() != s1 {
		t.Error("expected session() to return s1")
	}

	// Switch to s2.
	m.sessionIdx = 1
	m.expanded = make(map[int]bool)
	m.cursor = 0
	m.rebuildItems()

	if m.session() != s2 {
		t.Error("expected session() to return s2 after switch")
	}
	full := strings.Join(collectTexts(m.items), "\n")
	if !strings.Contains(full, "session two") {
		t.Error("expected 'session two' in rendered output after switch")
	}
}

func TestPickerMode(t *testing.T) {
	s1 := makeSession([]*parser.Message{makeMsg("user", "alpha")})
	s2 := makeSession([]*parser.Message{makeMsg("user", "beta")})

	m := newModelMulti([]*parser.Session{s1, s2}, 0)
	m.mode = modePicker
	m.pickCursor = 1

	view := m.View()
	if !strings.Contains(view, "Sessions") {
		t.Error("picker view should show 'Sessions' header")
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
