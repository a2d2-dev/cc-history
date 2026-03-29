package prompt_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/a2d2-dev/cc-history/internal/parser"
	"github.com/a2d2-dev/cc-history/internal/prompt"
)

func buildSession(turns []struct{ role, text string }) *parser.Session {
	ts := time.Date(2026, 3, 29, 10, 0, 0, 0, time.UTC)
	s := &parser.Session{ID: "test-session"}
	for _, t := range turns {
		s.Messages = append(s.Messages, &parser.Message{
			Role:      t.role,
			Text:      t.text,
			Timestamp: ts,
			CWD:       "/work/project",
			GitBranch: "main",
		})
	}
	return s
}

func TestBuild_AllMessages(t *testing.T) {
	session := buildSession([]struct{ role, text string }{
		{"user", "Hello, how are you?"},
		{"assistant", "I am fine, thank you!"},
		{"user", "What can you do?"},
		{"assistant", "Many things!"},
	})

	result, err := prompt.Build(session, prompt.Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Turns) != 4 {
		t.Errorf("expected 4 turns, got %d", len(result.Turns))
	}
	if result.CWD != "/work/project" {
		t.Errorf("expected CWD /work/project, got %q", result.CWD)
	}
	if result.Date != "2026-03-29" {
		t.Errorf("expected date 2026-03-29, got %q", result.Date)
	}
	if result.GitBranch != "main" {
		t.Errorf("expected branch main, got %q", result.GitBranch)
	}
	if !strings.Contains(result.ReconstructedPrompt, "Hello, how are you?") {
		t.Error("reconstructed prompt should contain first user input")
	}
	if !strings.Contains(result.ReconstructedPrompt, "What can you do?") {
		t.Error("reconstructed prompt should contain second user input")
	}
}

func TestBuild_Range(t *testing.T) {
	session := buildSession([]struct{ role, text string }{
		{"user", "msg1"},
		{"assistant", "resp1"},
		{"user", "msg2"},
		{"assistant", "resp2"},
		{"user", "msg3"},
	})

	result, err := prompt.Build(session, prompt.Options{Start: 2, End: 4})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Turns) != 3 {
		t.Errorf("expected 3 turns (2-4), got %d", len(result.Turns))
	}
	if result.Turns[0].Index != 2 {
		t.Errorf("first turn should have index 2, got %d", result.Turns[0].Index)
	}
	if result.Turns[0].Text != "resp1" {
		t.Errorf("first turn text should be resp1, got %q", result.Turns[0].Text)
	}
	if !strings.Contains(result.ReconstructedPrompt, "msg2") {
		t.Error("reconstructed prompt should include msg2")
	}
	if strings.Contains(result.ReconstructedPrompt, "msg1") {
		t.Error("reconstructed prompt should not include msg1 (out of range)")
	}
}

func TestBuild_InvalidRange(t *testing.T) {
	session := buildSession([]struct{ role, text string }{
		{"user", "hello"},
	})

	_, err := prompt.Build(session, prompt.Options{Start: 5, End: 10})
	if err == nil {
		t.Error("expected error for out-of-bounds range")
	}
}

func TestBuild_StartGtEnd(t *testing.T) {
	session := buildSession([]struct{ role, text string }{
		{"user", "hello"},
		{"assistant", "world"},
	})

	_, err := prompt.Build(session, prompt.Options{Start: 3, End: 1})
	if err == nil {
		t.Error("expected error when start > end")
	}
}

func TestBuild_AssistantSummaryTruncation(t *testing.T) {
	longText := strings.Repeat("a", 500)
	session := buildSession([]struct{ role, text string }{
		{"user", "question"},
		{"assistant", longText},
	})

	result, err := prompt.Build(session, prompt.Options{MaxSummaryRunes: 100})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assistantTurn := result.Turns[1]
	if !assistantTurn.Summary {
		t.Error("expected Summary=true for truncated assistant response")
	}
	if len([]rune(assistantTurn.Text)) > 101 { // 100 chars + ellipsis
		t.Errorf("expected truncated text, got %d runes", len([]rune(assistantTurn.Text)))
	}
}

func TestWrite_Output(t *testing.T) {
	session := buildSession([]struct{ role, text string }{
		{"user", "What is Go?"},
		{"assistant", "Go is a language."},
	})

	result, err := prompt.Build(session, prompt.Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf bytes.Buffer
	if err := prompt.Write(&buf, result); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Reconstructed Prompt") {
		t.Error("output should contain 'Reconstructed Prompt' section")
	}
	if !strings.Contains(out, "What is Go?") {
		t.Error("output should contain user message")
	}
	if !strings.Contains(out, "/work/project") {
		t.Error("output should contain CWD")
	}
}
