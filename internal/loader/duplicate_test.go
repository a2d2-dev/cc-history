package loader

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDuplicateSession(t *testing.T) {
	dir := t.TempDir()
	origID := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	origFile := filepath.Join(dir, origID+".jsonl")

	// Write a minimal JSONL session file with the original session ID.
	lines := []map[string]any{
		{"type": "user", "sessionId": origID, "uuid": "uuid-1"},
		{"type": "assistant", "sessionId": origID, "uuid": "uuid-2"},
	}
	f, err := os.Create(origFile)
	if err != nil {
		t.Fatal(err)
	}
	enc := json.NewEncoder(f)
	for _, l := range lines {
		if err := enc.Encode(l); err != nil {
			f.Close()
			t.Fatal(err)
		}
	}
	f.Close()

	newPath, newID, err := DuplicateSession(origFile)
	if err != nil {
		t.Fatalf("DuplicateSession: %v", err)
	}

	// New path should be in the same directory.
	if filepath.Dir(newPath) != dir {
		t.Errorf("expected new file in %s, got %s", dir, newPath)
	}

	// New ID should differ from original.
	if newID == origID {
		t.Error("new session ID should differ from original")
	}

	// New file should exist.
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("new file not found: %v", err)
	}

	// Every line in new file should contain the new ID, not the old one.
	nf, err := os.Open(newPath)
	if err != nil {
		t.Fatal(err)
	}
	defer nf.Close()

	scanner := bufio.NewScanner(nf)
	lineCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		lineCount++
		var rec map[string]string
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("invalid JSON in duplicate: %v", err)
		}
		if rec["sessionId"] == origID {
			t.Errorf("line %d still has original sessionId", lineCount)
		}
		if rec["sessionId"] != newID {
			t.Errorf("line %d has unexpected sessionId %q, want %q", lineCount, rec["sessionId"], newID)
		}
	}
	if lineCount != 2 {
		t.Errorf("expected 2 lines, got %d", lineCount)
	}

	// Original file should still exist untouched.
	if _, err := os.Stat(origFile); err != nil {
		t.Errorf("original file was removed: %v", err)
	}
}

func TestNewUUID(t *testing.T) {
	a, err := newUUID()
	if err != nil {
		t.Fatal(err)
	}
	b, err := newUUID()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Error("two generated UUIDs should not be equal")
	}
	// UUIDs should be 36 chars (8-4-4-4-12).
	if len(a) != 36 {
		t.Errorf("expected uuid length 36, got %d", len(a))
	}
}
