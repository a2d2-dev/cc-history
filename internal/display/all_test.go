package display_test

import (
	"strings"
	"testing"
	"time"

	"github.com/a2d2-dev/cc-history/internal/display"
	"github.com/a2d2-dev/cc-history/internal/parser"
)

// makeSession builds a minimal session with the given messages.
func makeSession(id, cwd string, messages []*parser.Message) *parser.Session {
	for _, m := range messages {
		m.SessionID = id
		m.CWD = cwd
	}
	return &parser.Session{ID: id, FilePath: id + ".jsonl", Messages: messages}
}

func userMsg(text string, t time.Time) *parser.Message {
	return &parser.Message{Role: "user", Text: text, Timestamp: t}
}

func asstMsg(text string, t time.Time) *parser.Message {
	return &parser.Message{Role: "assistant", Text: text, Timestamp: t}
}

func TestPrintAllSessions_ChronologicalOrder(t *testing.T) {
	t1 := ts("2024-01-01T10:00:00Z")
	t2 := ts("2024-01-01T10:01:00Z")
	t3 := ts("2024-01-01T10:02:00Z")

	s1 := makeSession("sessA", "/proj/a", []*parser.Message{
		userMsg("hello from A", t1),
		asstMsg("reply from A", t3),
	})
	s2 := makeSession("sessB", "/proj/b", []*parser.Message{
		userMsg("hello from B", t2),
	})

	var b strings.Builder
	display.PrintAllSessions(&b, []*parser.Session{s1, s2}, false)
	out := b.String()

	// s1 separator should appear before s2 separator.
	posA := strings.Index(out, "sessA")
	posB := strings.Index(out, "sessB")
	if posA < 0 || posB < 0 {
		t.Fatalf("missing session IDs in output:\n%s", out)
	}
	// t1(A) < t2(B) < t3(A) — so sessA separator first, then sessB, then sessA message again
	if posA >= posB {
		t.Errorf("expected sessA separator before sessB separator; posA=%d posB=%d\n%s", posA, posB, out)
	}

	// "hello from A" (t1) should appear before "hello from B" (t2)
	posHelloA := strings.Index(out, "hello from A")
	posHelloB := strings.Index(out, "hello from B")
	if posHelloA >= posHelloB {
		t.Errorf("expected 'hello from A' (t1) before 'hello from B' (t2)")
	}
}

func TestPrintAllSessions_SeparatorFormat(t *testing.T) {
	t1 := ts("2024-03-15T08:30:00Z")
	s := makeSession("sess-001", "/work/dir", []*parser.Message{
		userMsg("test message", t1),
	})

	var b strings.Builder
	display.PrintAllSessions(&b, []*parser.Session{s}, false)
	out := b.String()

	if !strings.Contains(out, "--- session sess-001") {
		t.Errorf("expected separator with session ID, got:\n%s", out)
	}
	if !strings.Contains(out, "/work/dir") {
		t.Errorf("expected CWD in separator, got:\n%s", out)
	}
	if !strings.Contains(out, "2024-03-15") {
		t.Errorf("expected date in separator, got:\n%s", out)
	}
}

func TestPrintAllSessions_NoSepDisablesSeparator(t *testing.T) {
	t1 := ts("2024-01-01T10:00:00Z")
	t2 := ts("2024-01-01T10:01:00Z")

	s1 := makeSession("sessA", "/a", []*parser.Message{userMsg("msg A", t1)})
	s2 := makeSession("sessB", "/b", []*parser.Message{userMsg("msg B", t2)})

	var b strings.Builder
	display.PrintAllSessions(&b, []*parser.Session{s1, s2}, true)
	out := b.String()

	if strings.Contains(out, "--- session") {
		t.Errorf("expected no separator lines with --no-sep, got:\n%s", out)
	}
	if !strings.Contains(out, "msg A") || !strings.Contains(out, "msg B") {
		t.Errorf("expected messages to still appear, got:\n%s", out)
	}
}

func TestFilterAllSessions_PatternMatch(t *testing.T) {
	t1 := ts("2024-01-01T10:00:00Z")
	t2 := ts("2024-01-01T10:01:00Z")
	t3 := ts("2024-01-01T10:02:00Z")

	s1 := makeSession("sessA", "/a", []*parser.Message{
		userMsg("go build error", t1),
		asstMsg("unrelated response", t2),
	})
	s2 := makeSession("sessB", "/b", []*parser.Message{
		userMsg("also a build issue", t3),
	})

	var b strings.Builder
	opts := display.FilterOptions{}
	if err := display.FilterAllSessions(&b, []*parser.Session{s1, s2}, "build", opts, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := b.String()

	if !strings.Contains(out, "go build error") {
		t.Errorf("expected 'go build error' in output:\n%s", out)
	}
	if !strings.Contains(out, "also a build issue") {
		t.Errorf("expected 'also a build issue' in output:\n%s", out)
	}
	if strings.Contains(out, "unrelated response") {
		t.Errorf("expected 'unrelated response' to be filtered out:\n%s", out)
	}
}

func TestFilterAllSessions_ContextFlags(t *testing.T) {
	t1 := ts("2024-01-01T10:00:00Z")
	t2 := ts("2024-01-01T10:01:00Z")
	t3 := ts("2024-01-01T10:02:00Z")

	s := makeSession("sess1", "/d", []*parser.Message{
		userMsg("context before", t1),
		asstMsg("TARGET match", t2),
		userMsg("context after", t3),
	})

	var b strings.Builder
	opts := display.FilterOptions{Before: 1, After: 1}
	if err := display.FilterAllSessions(&b, []*parser.Session{s}, "TARGET", opts, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := b.String()

	if !strings.Contains(out, "context before") {
		t.Errorf("expected before-context in output:\n%s", out)
	}
	if !strings.Contains(out, "TARGET match") {
		t.Errorf("expected match in output:\n%s", out)
	}
	if !strings.Contains(out, "context after") {
		t.Errorf("expected after-context in output:\n%s", out)
	}
}

func TestFilterAllSessions_NoSep(t *testing.T) {
	t1 := ts("2024-01-01T10:00:00Z")
	t2 := ts("2024-01-01T10:01:00Z")

	s1 := makeSession("sessX", "/x", []*parser.Message{userMsg("needle here", t1)})
	s2 := makeSession("sessY", "/y", []*parser.Message{userMsg("needle there", t2)})

	var b strings.Builder
	opts := display.FilterOptions{}
	if err := display.FilterAllSessions(&b, []*parser.Session{s1, s2}, "needle", opts, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := b.String()

	if strings.Contains(out, "--- session") {
		t.Errorf("expected no separator with noSep=true:\n%s", out)
	}
	if !strings.Contains(out, "needle here") || !strings.Contains(out, "needle there") {
		t.Errorf("expected both matches in output:\n%s", out)
	}
}

// sessionToMeta converts a *parser.Session to *parser.SessionMeta for testing.
// lastMsgFilePath is the FilePath of the session that should have its last
// message populated (simulates the current session).
func sessionToMeta(s *parser.Session, lastMsgFilePath string) *parser.SessionMeta {
	meta := &parser.SessionMeta{
		ID:       s.ID,
		FilePath: s.FilePath,
	}
	var lastMsg *parser.Message
	for _, m := range s.Messages {
		if m.Role == "" {
			continue
		}
		if meta.StartTime.IsZero() || m.Timestamp.Before(meta.StartTime) {
			meta.StartTime = m.Timestamp
		}
		if m.Timestamp.After(meta.EndTime) {
			meta.EndTime = m.Timestamp
			lastMsg = m
		}
	}
	if s.FilePath == lastMsgFilePath && lastMsg != nil {
		meta.LastMessage = lastMsg
	}
	return meta
}

func TestListSessions_AllSessionsListed(t *testing.T) {
	t1 := ts("2024-01-01T10:00:00Z")
	t2 := ts("2024-01-01T11:00:00Z")

	s1 := makeSession("sessA", "/proj/a", []*parser.Message{userMsg("msg A", t1)})
	s2 := makeSession("sessB", "/proj/b", []*parser.Message{asstMsg("msg B", t2)})

	metas := []*parser.SessionMeta{sessionToMeta(s1, ""), sessionToMeta(s2, "")}
	var b strings.Builder
	display.ListSessions(&b, metas, "")
	out := b.String()

	if !strings.Contains(out, "sessA") {
		t.Errorf("expected sessA in list output:\n%s", out)
	}
	if !strings.Contains(out, "sessB") {
		t.Errorf("expected sessB in list output:\n%s", out)
	}
}

func TestListSessions_CurrentMarked(t *testing.T) {
	t1 := ts("2024-01-01T10:00:00Z")

	s := makeSession("curr", "/proj", []*parser.Message{
		userMsg("first", t1),
		asstMsg("last reply", ts("2024-01-01T10:05:00Z")),
	})

	metas := []*parser.SessionMeta{sessionToMeta(s, "curr.jsonl")}
	var b strings.Builder
	display.ListSessions(&b, metas, "curr.jsonl")
	out := b.String()

	if !strings.Contains(out, "►") {
		t.Errorf("expected current session marker ► in output:\n%s", out)
	}
}

func TestListSessions_CurrentShowsLastMessage(t *testing.T) {
	t1 := ts("2024-01-01T10:00:00Z")
	t2 := ts("2024-01-01T10:05:00Z")

	s := makeSession("curr", "/proj", []*parser.Message{
		userMsg("early message", t1),
		asstMsg("the final answer", t2),
	})

	metas := []*parser.SessionMeta{sessionToMeta(s, "curr.jsonl")}
	var b strings.Builder
	display.ListSessions(&b, metas, "curr.jsonl")
	out := b.String()

	if !strings.Contains(out, "the final answer") {
		t.Errorf("expected last message content in output:\n%s", out)
	}
}

func TestListSessions_NonCurrentNoLastMessage(t *testing.T) {
	t1 := ts("2024-01-01T10:00:00Z")

	s1 := makeSession("sessA", "/a", []*parser.Message{asstMsg("other session msg", t1)})
	s2 := makeSession("current", "/b", []*parser.Message{asstMsg("current msg", ts("2024-01-01T11:00:00Z"))})

	metas := []*parser.SessionMeta{
		sessionToMeta(s1, "current.jsonl"),
		sessionToMeta(s2, "current.jsonl"),
	}
	var b strings.Builder
	display.ListSessions(&b, metas, "current.jsonl")
	out := b.String()

	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "    ") && strings.Contains(line, "other session msg") {
			t.Errorf("non-current session should not show last message indented:\n%s", out)
		}
	}
}

func TestListSessions_ChronologicalOrder(t *testing.T) {
	t1 := ts("2024-01-01T08:00:00Z")
	t2 := ts("2024-01-01T12:00:00Z")

	sLate := makeSession("late", "/b", []*parser.Message{userMsg("late msg", t2)})
	sEarly := makeSession("early", "/a", []*parser.Message{userMsg("early msg", t1)})

	// Pre-sort to match expected behaviour (ListSessions expects pre-sorted).
	metas := []*parser.SessionMeta{sessionToMeta(sEarly, ""), sessionToMeta(sLate, "")}
	var b strings.Builder
	display.ListSessions(&b, metas, "")
	out := b.String()

	posEarly := strings.Index(out, "early")
	posLate := strings.Index(out, "late")
	if posEarly >= posLate {
		t.Errorf("expected older session listed before newer session:\n%s", out)
	}
}
