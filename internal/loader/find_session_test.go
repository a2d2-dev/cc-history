package loader_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/a2d2-dev/cc-history/internal/loader"
)

// writeJSONL writes a minimal JSONL file containing the given sessionId.
func writeJSONL(t *testing.T, dir, name, sessionID string) string {
	t.Helper()
	content := `{"type":"user","sessionId":"` + sessionID + `","timestamp":"2024-01-01T00:00:00Z"}`
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFindCurrentSession_EnvVarFilenameMatch(t *testing.T) {
	root := t.TempDir()
	sessionID := "abc123"
	want := writeJSONL(t, root, sessionID+".jsonl", sessionID)
	writeJSONL(t, root, "other.jsonl", "other-session")

	t.Setenv("CLAUDE_SESSION_ID", sessionID)

	got, isFallback, err := loader.FindCurrentSession(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if isFallback {
		t.Error("expected isFallback=false")
	}
}

func TestFindCurrentSession_EnvVarContentMatch(t *testing.T) {
	root := t.TempDir()
	sessionID := "sess-xyz"
	// File not named after session — must scan content.
	want := writeJSONL(t, root, "session-file.jsonl", sessionID)
	writeJSONL(t, root, "other.jsonl", "other-session")

	t.Setenv("CLAUDE_SESSION_ID", sessionID)

	got, isFallback, err := loader.FindCurrentSession(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if isFallback {
		t.Error("expected isFallback=false")
	}
}

func TestFindCurrentSession_FallbackMostRecent(t *testing.T) {
	root := t.TempDir()

	old := writeJSONL(t, root, "old.jsonl", "sess-old")
	recent := writeJSONL(t, root, "recent.jsonl", "sess-new")

	// Ensure recent has a later mod time.
	now := time.Now()
	if err := os.Chtimes(old, now.Add(-2*time.Second), now.Add(-2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(recent, now, now); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CLAUDE_SESSION_ID", "")

	got, isFallback, err := loader.FindCurrentSession(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != recent {
		t.Errorf("got %q, want %q", got, recent)
	}
	if !isFallback {
		t.Error("expected isFallback=true when CLAUDE_SESSION_ID is unset")
	}
}

func TestFindCurrentSession_EnvVarNotFoundFallback(t *testing.T) {
	root := t.TempDir()
	writeJSONL(t, root, "session.jsonl", "real-session")

	t.Setenv("CLAUDE_SESSION_ID", "nonexistent-session-id")

	_, isFallback, err := loader.FindCurrentSession(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isFallback {
		t.Error("expected isFallback=true when session ID not found")
	}
}

func TestFindCurrentSession_EmptyDir(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_SESSION_ID", "")

	_, _, err := loader.FindCurrentSession(root)
	if err == nil {
		t.Fatal("expected error for empty directory")
	}
}
