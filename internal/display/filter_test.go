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

func noCtx(useRegex bool) FilterOptions { return FilterOptions{UseRegex: useRegex} }

func TestFilterSession_plaintext(t *testing.T) {
	session := makeSession([]string{
		"hello world",
		"foo bar",
		"hello again",
	})

	var sb strings.Builder
	if err := FilterSession(&sb, session, "hello", noCtx(false)); err != nil {
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
	if err := FilterSession(&sb, session, `(error|warning):`, noCtx(true)); err != nil {
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
	err := FilterSession(&sb, session, "[invalid", noCtx(true))
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
	if err := FilterSession(&sb, session, "match", noCtx(false)); err != nil {
		t.Fatal(err)
	}

	out := sb.String()
	// Contiguous matches should NOT have a separator.
	if strings.Contains(out, "--") {
		t.Error("unexpected '--' separator for contiguous matches")
	}
}

func TestFilterSession_afterContext(t *testing.T) {
	// msgs: 0=match, 1=ctx, 2=no, 3=match, 4=ctx
	session := makeSession([]string{
		"match first",
		"context after first",
		"not relevant",
		"match second",
		"context after second",
	})

	var sb strings.Builder
	opts := FilterOptions{After: 1}
	if err := FilterSession(&sb, session, "match", opts); err != nil {
		t.Fatal(err)
	}

	out := sb.String()
	if !strings.Contains(out, "match first") {
		t.Error("expected 'match first'")
	}
	if !strings.Contains(out, "context after first") {
		t.Error("expected 'context after first' (-A 1)")
	}
	if strings.Contains(out, "not relevant") {
		t.Error("unexpected 'not relevant'")
	}
	if !strings.Contains(out, "match second") {
		t.Error("expected 'match second'")
	}
	if !strings.Contains(out, "context after second") {
		t.Error("expected 'context after second' (-A 1)")
	}
	// Gap between first group and second match.
	if !strings.Contains(out, "--") {
		t.Error("expected '--' separator")
	}
}

func TestFilterSession_beforeContext(t *testing.T) {
	session := makeSession([]string{
		"before context",
		"not relevant",
		"match this",
	})

	var sb strings.Builder
	opts := FilterOptions{Before: 1}
	if err := FilterSession(&sb, session, "match", opts); err != nil {
		t.Fatal(err)
	}

	out := sb.String()
	if !strings.Contains(out, "not relevant") {
		t.Error("expected 'not relevant' as before-context (-B 1)")
	}
	if strings.Contains(out, "before context") {
		t.Error("unexpected 'before context' beyond -B 1 window")
	}
}

func TestFilterSession_contextFlag(t *testing.T) {
	session := makeSession([]string{
		"pre",
		"match",
		"post",
		"gap",
	})

	var sb strings.Builder
	opts := FilterOptions{Before: 1, After: 1}
	if err := FilterSession(&sb, session, "match", opts); err != nil {
		t.Fatal(err)
	}

	out := sb.String()
	if !strings.Contains(out, "pre") {
		t.Error("expected 'pre' as before-context")
	}
	if !strings.Contains(out, "post") {
		t.Error("expected 'post' as after-context")
	}
	if strings.Contains(out, "gap") {
		t.Error("unexpected 'gap'")
	}
}
