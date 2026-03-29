package loader

import (
	"os"
	"strings"
	"testing"
)

func TestRepathSession(t *testing.T) {
	// Write a minimal JSONL session file with two lines containing cwd.
	content := `{"type":"user","sessionId":"s1","cwd":"/old/path","message":{"role":"user","content":"hi"}}` + "\n" +
		`{"type":"assistant","sessionId":"s1","cwd":"/old/path","message":{"role":"assistant","content":"hello"}}` + "\n"

	f, err := os.CreateTemp(t.TempDir(), "repath-*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	path := f.Name()
	if _, err = f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if err = RepathSession(path, "/old/path", "/new/path"); err != nil {
		t.Fatalf("RepathSession: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result := string(got)
	if strings.Contains(result, "/old/path") {
		t.Errorf("old path still present in output:\n%s", result)
	}
	if !strings.Contains(result, `"cwd":"/new/path"`) {
		t.Errorf("new cwd not found in output:\n%s", result)
	}
}

func TestRepathSession_NoCWD(t *testing.T) {
	// Lines without cwd field should be passed through unchanged.
	content := `{"type":"user","sessionId":"s1","message":{"role":"user","content":"hi"}}` + "\n"

	f, err := os.CreateTemp(t.TempDir(), "repath-*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	path := f.Name()
	if _, err = f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if err = RepathSession(path, "/old/path", "/new/path"); err != nil {
		t.Fatalf("RepathSession: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimRight(string(got), "\n") != strings.TrimRight(content, "\n") {
		t.Errorf("content changed unexpectedly:\ngot: %s\nwant: %s", got, content)
	}
}

func TestRepathSession_SameOldNew(t *testing.T) {
	content := `{"cwd":"/same"}` + "\n"
	f, err := os.CreateTemp(t.TempDir(), "repath-*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	path := f.Name()
	if _, err = f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()

	// Same old and new path: should be a no-op.
	if err = RepathSession(path, "/same", "/same"); err != nil {
		t.Fatalf("RepathSession: %v", err)
	}
}
