package display

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/a2d2-dev/cc-history/internal/parser"
)

func makeSession(texts []string) *parser.Session {
	s := &parser.Session{ID: "test"}
	for i, t := range texts {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		s.Messages = append(s.Messages, &parser.Message{
			UUID:      fmt.Sprintf("uuid-%d", i),
			Role:      role,
			Text:      t,
			Timestamp: time.Date(2024, 1, 1, 12, 0, i, 0, time.UTC),
		})
	}
	return s
}

func TestFilterSession_plaintext(t *testing.T) {
	session := makeSession([]string{
		"hello world",
		"foo bar",
		"hello again",
	})

	var sb strings.Builder
	if err := FilterSession(&sb, session, "hello", false); err != nil {
		t.Fatal(err)
	}

	out := sb.String()
	if !strings.Contains(out, "hello world") {
		t.Error("expected 'hello world' in output")
	}
	if !strings.Contains(out, "hello again") {
		t.Error("expected 'hello again' in output")
	}
	if strings.Contains(out, "foo bar") {
		t.Error("unexpected 'foo bar' in output")
	}
	// Separator between non-contiguous groups.
	if !strings.Contains(out, "--") {
		t.Error("expected '--' separator between non-contiguous matches")
	}
}

func TestFilterSession_regex(t *testing.T) {
	session := makeSession([]string{
		"error: something failed",
		"all good",
		"warning: check this",
	})

	var sb strings.Builder
	if err := FilterSession(&sb, session, `(error|warning):`, true); err != nil {
		t.Fatal(err)
	}

	out := sb.String()
	if !strings.Contains(out, "error:") {
		t.Error("expected 'error:' in output")
	}
	if !strings.Contains(out, "warning:") {
		t.Error("expected 'warning:' in output")
	}
	if strings.Contains(out, "all good") {
		t.Error("unexpected 'all good' in output")
	}
}

func TestFilterSession_invalidRegex(t *testing.T) {
	session := makeSession([]string{"test"})
	var sb strings.Builder
	err := FilterSession(&sb, session, "[invalid", true)
	if err == nil {
		t.Error("expected error for invalid regex")
	}
}

func TestFilterSession_noSeparatorForContiguous(t *testing.T) {
	session := makeSession([]string{
		"match one",
		"match two",
		"no match",
	})

	var sb strings.Builder
	if err := FilterSession(&sb, session, "match", false); err != nil {
		t.Fatal(err)
	}

	out := sb.String()
	// Contiguous matches should NOT have a separator.
	if strings.Contains(out, "--") {
		t.Error("unexpected '--' separator for contiguous matches")
	}
}
