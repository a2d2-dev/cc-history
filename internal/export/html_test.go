package export_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	ccexport "github.com/a2d2-dev/cc-history/internal/export"
	"github.com/a2d2-dev/cc-history/internal/parser"
)

func TestToHTML_ContainsSessionID(t *testing.T) {
	var buf bytes.Buffer
	if err := ccexport.ToHTML(&buf, makeSession()); err != nil {
		t.Fatalf("ToHTML error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "test-session-id") {
		t.Errorf("expected session ID in HTML output")
	}
}

func TestToHTML_ContainsRoles(t *testing.T) {
	var buf bytes.Buffer
	if err := ccexport.ToHTML(&buf, makeSession()); err != nil {
		t.Fatalf("ToHTML error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"user", "assistant"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected role %q in HTML output", want)
		}
	}
}

func TestToHTML_ToolCallInDetails(t *testing.T) {
	var buf bytes.Buffer
	if err := ccexport.ToHTML(&buf, makeSession()); err != nil {
		t.Fatalf("ToHTML error: %v", err)
	}
	out := buf.String()
	// Tool calls should be wrapped in collapsible <details> elements.
	if !strings.Contains(out, "<details") {
		t.Errorf("expected <details> element for tool call collapsible")
	}
	if !strings.Contains(out, "<summary") {
		t.Errorf("expected <summary> element for tool call")
	}
	if !strings.Contains(out, "Bash") {
		t.Errorf("expected tool name 'Bash' in HTML output")
	}
}

func TestToHTML_ToolResultPresent(t *testing.T) {
	var buf bytes.Buffer
	if err := ccexport.ToHTML(&buf, makeSession()); err != nil {
		t.Fatalf("ToHTML error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "total 8") {
		t.Errorf("expected tool result content in HTML output")
	}
}

func TestToHTML_ValidHTMLStructure(t *testing.T) {
	var buf bytes.Buffer
	if err := ccexport.ToHTML(&buf, makeSession()); err != nil {
		t.Fatalf("ToHTML error: %v", err)
	}
	out := buf.String()
	for _, tag := range []string{"<!DOCTYPE html>", "<html", "<head>", "<body>", "</html>"} {
		if !strings.Contains(out, tag) {
			t.Errorf("expected HTML tag %q in output", tag)
		}
	}
}

func TestToHTML_InlineCSS(t *testing.T) {
	var buf bytes.Buffer
	if err := ccexport.ToHTML(&buf, makeSession()); err != nil {
		t.Fatalf("ToHTML error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "<style>") {
		t.Errorf("expected inline <style> block for self-contained HTML")
	}
	if !strings.Contains(out, "<script>") {
		t.Errorf("expected inline <script> block for self-contained HTML")
	}
}

func TestToHTML_SkipsEmptyRole(t *testing.T) {
	session := &parser.Session{
		ID: "s2",
		Messages: []*parser.Message{
			{Role: "", Text: "internal noise"},
			{Role: "user", Text: "visible message", Timestamp: time.Now()},
		},
	}
	var buf bytes.Buffer
	if err := ccexport.ToHTML(&buf, session); err != nil {
		t.Fatalf("ToHTML error: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "internal noise") {
		t.Error("should not include messages with empty role")
	}
	if !strings.Contains(out, "visible message") {
		t.Error("should include user message text")
	}
}

func TestToHTML_XSSEscaping(t *testing.T) {
	session := &parser.Session{
		ID: "xss-test",
		Messages: []*parser.Message{
			{
				Role:      "user",
				Timestamp: time.Now(),
				Text:      `<script>alert("xss")</script>`,
			},
		},
	}
	var buf bytes.Buffer
	if err := ccexport.ToHTML(&buf, session); err != nil {
		t.Fatalf("ToHTML error: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, `<script>alert("xss")</script>`) {
		t.Error("HTML output must escape user-supplied content to prevent XSS")
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Error("expected escaped script tag in HTML output")
	}
}

func TestToHTML_ErrorToolCall(t *testing.T) {
	ts := time.Now()
	session := &parser.Session{
		ID: "err-session",
		Messages: []*parser.Message{
			{
				Role:      "assistant",
				Timestamp: ts,
				ToolCalls: []*parser.ToolCall{
					{
						ID:        "tc-err",
						Name:      "Bash",
						Arguments: `{"command":"bad"}`,
						Result:    "command not found",
						IsError:   true,
					},
				},
			},
		},
	}
	var buf bytes.Buffer
	if err := ccexport.ToHTML(&buf, session); err != nil {
		t.Fatalf("ToHTML error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Error") {
		t.Error("expected 'Error' label for error tool result")
	}
	if !strings.Contains(out, "error-block") {
		t.Error("expected error-block CSS class for error results")
	}
}
