package loader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestArchiveRoot(t *testing.T) {
	cases := []struct {
		root string
		want string
	}{
		{"/home/user/.claude/projects", "/home/user/.claude/projects-archive"},
		{"/home/user/.claude/sessions", "/home/user/.claude/sessions-archive"},
		{"/tmp/custom", "/tmp/custom-archive"},
	}
	for _, c := range cases {
		got := ArchiveRoot(c.root)
		if got != c.want {
			t.Errorf("ArchiveRoot(%q) = %q, want %q", c.root, got, c.want)
		}
	}
}

func TestArchiveRestoreSession(t *testing.T) {
	// Set up a temporary sessions directory with a nested JSONL file.
	tmpDir := t.TempDir()
	sessionsRoot := filepath.Join(tmpDir, "projects")
	subDir := filepath.Join(sessionsRoot, "-home-user-myrepo")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(subDir, "abc123.jsonl")
	if err := os.WriteFile(filePath, []byte(`{"sessionId":"abc123"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Archive the session.
	dstPath, err := ArchiveSession(sessionsRoot, filePath)
	if err != nil {
		t.Fatalf("ArchiveSession: %v", err)
	}

	// Original file should be gone.
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("original file should not exist after archive")
	}

	// Archived file should exist under archive root.
	archiveRoot := ArchiveRoot(sessionsRoot)
	expectedDst := filepath.Join(archiveRoot, "-home-user-myrepo", "abc123.jsonl")
	if dstPath != expectedDst {
		t.Errorf("ArchiveSession dstPath = %q, want %q", dstPath, expectedDst)
	}
	if _, err := os.Stat(dstPath); err != nil {
		t.Errorf("archived file should exist: %v", err)
	}

	// Restore the session.
	restoredPath, err := RestoreSession(sessionsRoot, dstPath)
	if err != nil {
		t.Fatalf("RestoreSession: %v", err)
	}
	if restoredPath != filePath {
		t.Errorf("RestoreSession path = %q, want %q", restoredPath, filePath)
	}
	if _, err := os.Stat(restoredPath); err != nil {
		t.Errorf("restored file should exist: %v", err)
	}
	// Archive copy should be gone.
	if _, err := os.Stat(dstPath); !os.IsNotExist(err) {
		t.Error("archived file should not exist after restore")
	}
}

func TestArchiveSession_OutsideRoot(t *testing.T) {
	tmpDir := t.TempDir()
	sessionsRoot := filepath.Join(tmpDir, "projects")
	outsideFile := filepath.Join(tmpDir, "other.jsonl")
	if err := os.WriteFile(outsideFile, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ArchiveSession(sessionsRoot, outsideFile)
	if err == nil {
		t.Error("expected error when file is outside sessionsRoot")
	}
}
