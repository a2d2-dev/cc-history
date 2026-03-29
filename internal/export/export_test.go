package export_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	ccexport "github.com/a2d2-dev/cc-history/internal/export"
	"github.com/a2d2-dev/cc-history/internal/parser"
)

func makeSession() *parser.Session {
	ts, _ := time.Parse(time.RFC3339, "2024-01-15T10:30:00Z")
	return &parser.Session{
		ID:       "test-session-id",
		FilePath: "/tmp/test.jsonl",
		Messages: []*parser.Message{
			{
				UUID:      "uuid-1",
				Role:      "user",
				Timestamp: ts,
				Text:      "Hello, please list files.",
			},
			{
				UUID:      "uuid-2",
				Role:      "assistant",
				Timestamp: ts.Add(time.Second),
				Text:      "I'll list the files for you.",
				ToolCalls: []*parser.ToolCall{
					{
						ID:        "tool-1",
						Name:      "Bash",
						Arguments: `{"command":"ls -la"}`,
						Result:    "total 8\ndrwxr-xr-x 2 user user 4096 Jan 15 10:30 .\n",
						IsError:   false,
						DurationMs: 42,
					},
				},
			},
		},
	}
}

func TestToMarkdown_ContainsSessionID(t *testing.T) {
	var buf bytes.Buffer
	if err := ccexport.ToMarkdown(&buf, makeSession()); err != nil {
		t.Fatalf("ToMarkdown error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "test-session-id") {
		t.Errorf("expected session ID in output, got:\n%s", out)
	}
}

func TestToMarkdown_ContainsTimestamp(t *testing.T) {
	var buf bytes.Buffer
	if err := ccexport.ToMarkdown(&buf, makeSession()); err != nil {
		t.Fatalf("ToMarkdown error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "2024-01-15") {
		t.Errorf("expected date in output, got:\n%s", out)
	}
}

func TestToMarkdown_ContainsRoles(t *testing.T) {
	var buf bytes.Buffer
	if err := ccexport.ToMarkdown(&buf, makeSession()); err != nil {
		t.Fatalf("ToMarkdown error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"user", "assistant"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output", want)
		}
	}
}

func TestToMarkdown_ContainsToolCall(t *testing.T) {
	var buf bytes.Buffer
	if err := ccexport.ToMarkdown(&buf, makeSession()); err != nil {
		t.Fatalf("ToMarkdown error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Bash", "ls -la", "total 8"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestToMarkdown_SkipsEmptyRole(t *testing.T) {
	session := &parser.Session{
		ID: "s1",
		Messages: []*parser.Message{
			{Role: "", Text: "system noise"},
			{Role: "user", Text: "real message", Timestamp: time.Now()},
		},
	}
	var buf bytes.Buffer
	if err := ccexport.ToMarkdown(&buf, session); err != nil {
		t.Fatalf("ToMarkdown error: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "system noise") {
		t.Error("should not include messages with no role")
	}
	if !strings.Contains(out, "real message") {
		t.Error("should include user message")
	}
}

func TestToJSON_ValidJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := ccexport.ToJSON(&buf, makeSession()); err != nil {
		t.Fatalf("ToJSON error: %v", err)
	}
	var v interface{}
	if err := json.Unmarshal(buf.Bytes(), &v); err != nil {
		t.Fatalf("ToJSON produced invalid JSON: %v\nOutput:\n%s", err, buf.String())
	}
}

func TestToJSON_ContainsSessionID(t *testing.T) {
	var buf bytes.Buffer
	if err := ccexport.ToJSON(&buf, makeSession()); err != nil {
		t.Fatalf("ToJSON error: %v", err)
	}
	if !strings.Contains(buf.String(), "test-session-id") {
		t.Errorf("JSON output missing session ID:\n%s", buf.String())
	}
}

func TestToJSON_StructureMatchesModel(t *testing.T) {
	var buf bytes.Buffer
	if err := ccexport.ToJSON(&buf, makeSession()); err != nil {
		t.Fatalf("ToJSON error: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	for _, field := range []string{"ID", "FilePath", "Messages"} {
		if _, ok := result[field]; !ok {
			t.Errorf("expected field %q in JSON output", field)
		}
	}
}
