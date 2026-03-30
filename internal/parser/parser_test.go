package parser_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/a2d2-dev/cc-history/internal/parser"
)

// --------------------------------------------------------------------------
// helpers
// --------------------------------------------------------------------------

func parseString(t *testing.T, jsonl string) *parser.Session {
	t.Helper()
	s, err := parser.ParseReader("test.jsonl", strings.NewReader(jsonl))
	if err != nil {
		t.Fatalf("ParseReader error: %v", err)
	}
	return s
}

// --------------------------------------------------------------------------
// ParseReader tests
// --------------------------------------------------------------------------

func TestParseReader_Empty(t *testing.T) {
	s := parseString(t, "")
	if s == nil {
		t.Fatal("expected non-nil session")
	}
	if len(s.Messages) != 0 {
		t.Errorf("want 0 messages, got %d", len(s.Messages))
	}
}

func TestParseReader_BlankLines(t *testing.T) {
	s := parseString(t, "\n\n\n")
	if len(s.Messages) != 0 {
		t.Errorf("want 0 messages, got %d", len(s.Messages))
	}
}

func TestParseReader_SessionID(t *testing.T) {
	line := `{"type":"user","sessionId":"sess-123","uuid":"u1","timestamp":"2026-01-01T00:00:00Z","message":{"role":"user","content":"hello"}}`
	s := parseString(t, line)
	if s.ID != "sess-123" {
		t.Errorf("want session id sess-123, got %q", s.ID)
	}
}

func TestParseReader_FilePath(t *testing.T) {
	s, _ := parser.ParseReader("/some/path.jsonl", strings.NewReader(""))
	if s.FilePath != "/some/path.jsonl" {
		t.Errorf("want /some/path.jsonl, got %q", s.FilePath)
	}
}

func TestParseReader_UserStringContent(t *testing.T) {
	line := `{"type":"user","sessionId":"s1","uuid":"u1","timestamp":"2026-01-01T00:00:00Z","message":{"role":"user","content":"hello world"}}`
	s := parseString(t, line)
	if len(s.Messages) != 1 {
		t.Fatalf("want 1 message, got %d", len(s.Messages))
	}
	msg := s.Messages[0]
	if msg.Type != "user" {
		t.Errorf("want type user, got %q", msg.Type)
	}
	if msg.Role != "user" {
		t.Errorf("want role user, got %q", msg.Role)
	}
	if msg.Text != "hello world" {
		t.Errorf("want text 'hello world', got %q", msg.Text)
	}
}

func TestParseReader_AssistantTextContent(t *testing.T) {
	line := `{"type":"assistant","sessionId":"s1","uuid":"a1","parentUuid":"u1","timestamp":"2026-01-01T00:00:01Z","message":{"role":"assistant","content":[{"type":"text","text":"Hi there"}]}}`
	s := parseString(t, line)
	if len(s.Messages) != 1 {
		t.Fatalf("want 1 message, got %d", len(s.Messages))
	}
	msg := s.Messages[0]
	if msg.Type != "assistant" {
		t.Errorf("want type assistant, got %q", msg.Type)
	}
	if msg.Text != "Hi there" {
		t.Errorf("want text 'Hi there', got %q", msg.Text)
	}
	if len(msg.ToolCalls) != 0 {
		t.Errorf("want 0 tool calls, got %d", len(msg.ToolCalls))
	}
}

func TestParseReader_AssistantMultipleTextBlocks(t *testing.T) {
	line := `{"type":"assistant","sessionId":"s1","uuid":"a1","timestamp":"2026-01-01T00:00:01Z","message":{"role":"assistant","content":[{"type":"text","text":"Part 1"},{"type":"text","text":"Part 2"}]}}`
	s := parseString(t, line)
	msg := s.Messages[0]
	if msg.Text != "Part 1\nPart 2" {
		t.Errorf("want 'Part 1\\nPart 2', got %q", msg.Text)
	}
}

func TestParseReader_AssistantToolCall(t *testing.T) {
	line := `{"type":"assistant","sessionId":"s1","uuid":"a1","timestamp":"2026-01-01T00:00:01Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"tc1","name":"Bash","input":{"command":"ls"}}]}}`
	s := parseString(t, line)
	msg := s.Messages[0]
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("want 1 tool call, got %d", len(msg.ToolCalls))
	}
	tc := msg.ToolCalls[0]
	if tc.ID != "tc1" {
		t.Errorf("want tc id 'tc1', got %q", tc.ID)
	}
	if tc.Name != "Bash" {
		t.Errorf("want name 'Bash', got %q", tc.Name)
	}
	if tc.Arguments != `{"command":"ls"}` {
		t.Errorf("want arguments '{\"command\":\"ls\"}', got %q", tc.Arguments)
	}
}

func TestParseReader_AssistantToolCallEmptyInput(t *testing.T) {
	line := `{"type":"assistant","sessionId":"s1","uuid":"a1","timestamp":"2026-01-01T00:00:01Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"tc2","name":"NoArgs","input":{}}]}}`
	s := parseString(t, line)
	tc := s.Messages[0].ToolCalls[0]
	if tc.Arguments == "" {
		t.Error("arguments should not be empty; want at least '{}'")
	}
}

func TestParseReader_UserToolResult(t *testing.T) {
	line := `{"type":"user","sessionId":"s1","uuid":"u2","timestamp":"2026-01-01T00:00:02Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tc1","content":"file1.txt\nfile2.txt","is_error":false}]},"toolUseResult":{"durationMs":42}}`
	s := parseString(t, line)
	msg := s.Messages[0]
	if len(msg.ToolResults) != 1 {
		t.Fatalf("want 1 tool result, got %d", len(msg.ToolResults))
	}
	tr := msg.ToolResults[0]
	if tr.ToolUseID != "tc1" {
		t.Errorf("want tool_use_id 'tc1', got %q", tr.ToolUseID)
	}
	if tr.Content != "file1.txt\nfile2.txt" {
		t.Errorf("want content 'file1.txt\\nfile2.txt', got %q", tr.Content)
	}
	if tr.IsError {
		t.Error("want is_error false")
	}
	if tr.DurationMs != 42 {
		t.Errorf("want duration 42, got %f", tr.DurationMs)
	}
}

func TestParseReader_UserToolResultError(t *testing.T) {
	line := `{"type":"user","sessionId":"s1","uuid":"u3","timestamp":"2026-01-01T00:00:03Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tc2","content":"command not found","is_error":true}]}}`
	s := parseString(t, line)
	tr := s.Messages[0].ToolResults[0]
	if !tr.IsError {
		t.Error("want is_error true")
	}
	if tr.Content != "command not found" {
		t.Errorf("unexpected content %q", tr.Content)
	}
}

func TestParseReader_ToolCallLinking(t *testing.T) {
	jsonl := strings.Join([]string{
		`{"type":"assistant","sessionId":"s1","uuid":"a1","timestamp":"2026-01-01T00:00:01Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"tc1","name":"Bash","input":{"command":"echo hi"}}]}}`,
		`{"type":"user","sessionId":"s1","uuid":"u2","timestamp":"2026-01-01T00:00:02Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tc1","content":"hi","is_error":false}]},"toolUseResult":{"durationMs":15.5}}`,
	}, "\n")

	s := parseString(t, jsonl)
	if len(s.Messages) != 2 {
		t.Fatalf("want 2 messages, got %d", len(s.Messages))
	}
	tc := s.Messages[0].ToolCalls[0]
	if tc.Result != "hi" {
		t.Errorf("want linked result 'hi', got %q", tc.Result)
	}
	if tc.DurationMs != 15.5 {
		t.Errorf("want linked duration 15.5, got %f", tc.DurationMs)
	}
	if tc.IsError {
		t.Error("want linked is_error false")
	}
}

func TestParseReader_SystemTurnDuration(t *testing.T) {
	line := `{"type":"system","subtype":"turn_duration","sessionId":"s1","uuid":"sys1","timestamp":"2026-01-01T00:00:10Z","durationMs":206735}`
	s := parseString(t, line)
	if len(s.Messages) != 1 {
		t.Fatalf("want 1 message, got %d", len(s.Messages))
	}
	msg := s.Messages[0]
	if msg.Type != "system" {
		t.Errorf("want type system, got %q", msg.Type)
	}
	if msg.Subtype != "turn_duration" {
		t.Errorf("want subtype turn_duration, got %q", msg.Subtype)
	}
	if msg.TurnDurationMs != 206735 {
		t.Errorf("want duration 206735, got %f", msg.TurnDurationMs)
	}
}

func TestParseReader_SystemAPIError(t *testing.T) {
	line := `{"type":"system","subtype":"api_error","sessionId":"s1","uuid":"sys2","timestamp":"2026-01-01T00:00:05Z"}`
	s := parseString(t, line)
	msg := s.Messages[0]
	if msg.Subtype != "api_error" {
		t.Errorf("want subtype api_error, got %q", msg.Subtype)
	}
}

func TestParseReader_SkipsUnknownTypes(t *testing.T) {
	jsonl := strings.Join([]string{
		`{"type":"progress","sessionId":"s1","uuid":"p1","timestamp":"2026-01-01T00:00:00Z"}`,
		`{"type":"queue-operation","sessionId":"s1","uuid":"q1","timestamp":"2026-01-01T00:00:00Z"}`,
		`{"type":"file-history-snapshot","sessionId":"s1","uuid":"f1","timestamp":"2026-01-01T00:00:00Z"}`,
		`{"type":"user","sessionId":"s1","uuid":"u1","timestamp":"2026-01-01T00:00:01Z","message":{"role":"user","content":"hello"}}`,
	}, "\n")
	s := parseString(t, jsonl)
	if len(s.Messages) != 1 {
		t.Errorf("want 1 message (only user), got %d", len(s.Messages))
	}
}

func TestParseReader_MalformedLineSkipped(t *testing.T) {
	jsonl := strings.Join([]string{
		`not valid json {{{`,
		`{"type":"user","sessionId":"s1","uuid":"u1","timestamp":"2026-01-01T00:00:01Z","message":{"role":"user","content":"valid"}}`,
	}, "\n")
	s := parseString(t, jsonl)
	// Malformed line is skipped; valid line is parsed.
	if len(s.Messages) != 1 {
		t.Errorf("want 1 message, got %d", len(s.Messages))
	}
	if s.Messages[0].Text != "valid" {
		t.Errorf("want text 'valid', got %q", s.Messages[0].Text)
	}
}

func TestParseReader_AllMalformed(t *testing.T) {
	s := parseString(t, "{bad}\n{also bad}")
	if len(s.Messages) != 0 {
		t.Errorf("want 0 messages, got %d", len(s.Messages))
	}
}

func TestParseReader_Timestamp(t *testing.T) {
	line := `{"type":"user","sessionId":"s1","uuid":"u1","timestamp":"2026-03-17T10:07:46.343Z","message":{"role":"user","content":"hi"}}`
	s := parseString(t, line)
	msg := s.Messages[0]
	want := time.Date(2026, 3, 17, 10, 7, 46, 343_000_000, time.UTC)
	if !msg.Timestamp.Equal(want) {
		t.Errorf("want timestamp %v, got %v", want, msg.Timestamp)
	}
}

func TestParseReader_BadTimestampZeroTime(t *testing.T) {
	line := `{"type":"user","sessionId":"s1","uuid":"u1","timestamp":"not-a-time","message":{"role":"user","content":"hi"}}`
	s := parseString(t, line)
	if !s.Messages[0].Timestamp.IsZero() {
		t.Error("want zero time for bad timestamp")
	}
}

func TestParseReader_ParentUUID(t *testing.T) {
	line := `{"type":"assistant","sessionId":"s1","uuid":"a1","parentUuid":"u1","timestamp":"2026-01-01T00:00:01Z","message":{"role":"assistant","content":[]}}`
	s := parseString(t, line)
	if s.Messages[0].ParentUUID != "u1" {
		t.Errorf("want parentUUID 'u1', got %q", s.Messages[0].ParentUUID)
	}
}

func TestParseReader_NullParentUUID(t *testing.T) {
	line := `{"type":"user","sessionId":"s1","uuid":"u1","parentUuid":null,"timestamp":"2026-01-01T00:00:00Z","message":{"role":"user","content":"hi"}}`
	s := parseString(t, line)
	if s.Messages[0].ParentUUID != "" {
		t.Errorf("want empty parentUUID for null, got %q", s.Messages[0].ParentUUID)
	}
}

func TestParseReader_IsSidechain(t *testing.T) {
	line := `{"type":"user","sessionId":"s1","uuid":"u1","isSidechain":true,"timestamp":"2026-01-01T00:00:00Z","message":{"role":"user","content":"hi"}}`
	s := parseString(t, line)
	if !s.Messages[0].IsSidechain {
		t.Error("want isSidechain true")
	}
}

func TestParseReader_GitBranchCWDVersion(t *testing.T) {
	line := `{"type":"user","sessionId":"s1","uuid":"u1","gitBranch":"main","cwd":"/tmp","version":"2.1.50","timestamp":"2026-01-01T00:00:00Z","message":{"role":"user","content":"hi"}}`
	s := parseString(t, line)
	msg := s.Messages[0]
	if msg.GitBranch != "main" {
		t.Errorf("want gitBranch 'main', got %q", msg.GitBranch)
	}
	if msg.CWD != "/tmp" {
		t.Errorf("want cwd '/tmp', got %q", msg.CWD)
	}
	if msg.Version != "2.1.50" {
		t.Errorf("want version '2.1.50', got %q", msg.Version)
	}
}

func TestParseReader_AssistantThinkingSkipped(t *testing.T) {
	line := `{"type":"assistant","sessionId":"s1","uuid":"a1","timestamp":"2026-01-01T00:00:01Z","message":{"role":"assistant","content":[{"type":"thinking","thinking":"internal reasoning"},{"type":"text","text":"visible"}]}}`
	s := parseString(t, line)
	msg := s.Messages[0]
	// thinking content should not appear in Text
	if strings.Contains(msg.Text, "internal reasoning") {
		t.Error("thinking block should not appear in Text")
	}
	if msg.Text != "visible" {
		t.Errorf("want text 'visible', got %q", msg.Text)
	}
}

func TestParseReader_ToolResultArrayContent(t *testing.T) {
	line := `{"type":"user","sessionId":"s1","uuid":"u2","timestamp":"2026-01-01T00:00:02Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tc1","content":[{"type":"text","text":"line1"},{"type":"text","text":"line2"}],"is_error":false}]}}`
	s := parseString(t, line)
	tr := s.Messages[0].ToolResults[0]
	if tr.Content != "line1\nline2" {
		t.Errorf("want 'line1\\nline2', got %q", tr.Content)
	}
}

func TestParseReader_MultipleMessages(t *testing.T) {
	jsonl := strings.Join([]string{
		`{"type":"user","sessionId":"s1","uuid":"u1","timestamp":"2026-01-01T00:00:00Z","message":{"role":"user","content":"q1"}}`,
		`{"type":"assistant","sessionId":"s1","uuid":"a1","parentUuid":"u1","timestamp":"2026-01-01T00:00:01Z","message":{"role":"assistant","content":[{"type":"text","text":"a1"}]}}`,
		`{"type":"user","sessionId":"s1","uuid":"u2","parentUuid":"a1","timestamp":"2026-01-01T00:00:02Z","message":{"role":"user","content":"q2"}}`,
		`{"type":"assistant","sessionId":"s1","uuid":"a2","parentUuid":"u2","timestamp":"2026-01-01T00:00:03Z","message":{"role":"assistant","content":[{"type":"text","text":"a2"}]}}`,
	}, "\n")
	s := parseString(t, jsonl)
	if len(s.Messages) != 4 {
		t.Fatalf("want 4 messages, got %d", len(s.Messages))
	}
	if s.Messages[0].Text != "q1" {
		t.Errorf("msg[0].Text want 'q1', got %q", s.Messages[0].Text)
	}
	if s.Messages[3].Text != "a2" {
		t.Errorf("msg[3].Text want 'a2', got %q", s.Messages[3].Text)
	}
}

func TestParseReader_NoMessageField(t *testing.T) {
	// system record with no message field — should still parse.
	line := `{"type":"system","subtype":"turn_duration","sessionId":"s1","uuid":"sys1","timestamp":"2026-01-01T00:00:00Z","durationMs":1234}`
	s := parseString(t, line)
	if len(s.Messages) != 1 {
		t.Fatalf("want 1 message, got %d", len(s.Messages))
	}
	if s.Messages[0].Role != "" {
		t.Errorf("want empty role for system, got %q", s.Messages[0].Role)
	}
}

func TestParseReader_ToolCallNoDuration(t *testing.T) {
	jsonl := strings.Join([]string{
		`{"type":"assistant","sessionId":"s1","uuid":"a1","timestamp":"2026-01-01T00:00:01Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"tc1","name":"Read","input":{"path":"/tmp"}}]}}`,
		`{"type":"user","sessionId":"s1","uuid":"u2","timestamp":"2026-01-01T00:00:02Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tc1","content":"content"}]}}`,
	}, "\n")
	s := parseString(t, jsonl)
	tc := s.Messages[0].ToolCalls[0]
	if tc.DurationMs != 0 {
		t.Errorf("want 0 duration when toolUseResult absent, got %f", tc.DurationMs)
	}
	if tc.Result != "content" {
		t.Errorf("want result 'content', got %q", tc.Result)
	}
}

// --------------------------------------------------------------------------
// ParseFile tests
// --------------------------------------------------------------------------

func TestParseFile_NotExist(t *testing.T) {
	_, err := parser.ParseFile("/nonexistent/path/file.jsonl")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestParseFile_EmptyFile(t *testing.T) {
	f, err := os.CreateTemp("", "*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	s, err := parser.ParseFile(f.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.Messages) != 0 {
		t.Errorf("want 0 messages, got %d", len(s.Messages))
	}
}

func TestParseFile_ValidFile(t *testing.T) {
	content := `{"type":"user","sessionId":"file-session","uuid":"u1","timestamp":"2026-01-01T00:00:00Z","message":{"role":"user","content":"from file"}}` + "\n"
	f, err := os.CreateTemp("", "*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()

	s, err := parser.ParseFile(f.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ID != "file-session" {
		t.Errorf("want session id 'file-session', got %q", s.ID)
	}
	if len(s.Messages) != 1 {
		t.Fatalf("want 1 message, got %d", len(s.Messages))
	}
	if s.Messages[0].Text != "from file" {
		t.Errorf("want text 'from file', got %q", s.Messages[0].Text)
	}
}

func TestParseFile_CorruptedLines(t *testing.T) {
	content := "garbage line\n" +
		`{"type":"user","sessionId":"s1","uuid":"u1","timestamp":"2026-01-01T00:00:00Z","message":{"role":"user","content":"ok"}}` + "\n" +
		"more garbage\n"

	f, err := os.CreateTemp("", "*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString(content)
	f.Close()

	s, err := parser.ParseFile(f.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.Messages) != 1 {
		t.Errorf("want 1 message (corrupted lines skipped), got %d", len(s.Messages))
	}
}

func TestParseFirstMsgPreview_StringContent(t *testing.T) {
	line := `{"type":"user","sessionId":"s1","uuid":"u1","timestamp":"2026-01-01T00:00:00Z","message":{"role":"user","content":"hello world"}}` + "\n"
	f, err := os.CreateTemp("", "*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString(line)
	f.Close()

	preview := parser.ParseFirstMsgPreview(f.Name())
	if preview != "hello world" {
		t.Errorf("want 'hello world', got %q", preview)
	}
}

func TestParseFirstMsgPreview_Truncation(t *testing.T) {
	longText := strings.Repeat("a", 60)
	line := `{"type":"user","sessionId":"s1","uuid":"u1","timestamp":"2026-01-01T00:00:00Z","message":{"role":"user","content":"` + longText + `"}}` + "\n"
	f, err := os.CreateTemp("", "*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString(line)
	f.Close()

	preview := parser.ParseFirstMsgPreview(f.Name())
	runes := []rune(preview)
	if len(runes) > 51 { // 50 + "…"
		t.Errorf("expected truncation at 50+ellipsis, got len=%d %q", len(runes), preview)
	}
	if !strings.HasSuffix(preview, "…") {
		t.Errorf("expected ellipsis suffix, got %q", preview)
	}
}

func TestParseFirstMsgPreview_SkipsAssistant(t *testing.T) {
	// assistant message first, then user message
	lines := `{"type":"assistant","sessionId":"s1","uuid":"u1","timestamp":"2026-01-01T00:00:00Z","message":{"role":"assistant","content":"I can help"}}` + "\n" +
		`{"type":"user","sessionId":"s1","uuid":"u2","timestamp":"2026-01-01T00:00:01Z","message":{"role":"user","content":"user question"}}` + "\n"
	f, err := os.CreateTemp("", "*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString(lines)
	f.Close()

	preview := parser.ParseFirstMsgPreview(f.Name())
	if preview != "user question" {
		t.Errorf("want 'user question', got %q", preview)
	}
}

func TestParseFirstMsgPreview_EmptyFile(t *testing.T) {
	f, err := os.CreateTemp("", "*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Close()

	preview := parser.ParseFirstMsgPreview(f.Name())
	if preview != "" {
		t.Errorf("want empty string for empty file, got %q", preview)
	}
}

func TestParseFileMeta_CWD(t *testing.T) {
	line := `{"type":"user","sessionId":"s1","uuid":"u1","timestamp":"2026-01-01T00:00:00Z","cwd":"/home/user/project","message":{"role":"user","content":"hi"}}` + "\n"
	f, err := os.CreateTemp("", "*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString(line)
	f.Close()

	meta, err := parser.ParseFileMeta(f.Name(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.CWD != "/home/user/project" {
		t.Errorf("want CWD '/home/user/project', got %q", meta.CWD)
	}
}
