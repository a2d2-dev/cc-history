package loader_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/a2d2-dev/cc-history/internal/loader"
)

func TestScanJSONL_ReturnsFiles(t *testing.T) {
	root := t.TempDir()

	sub := filepath.Join(root, "proj-abc")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(root, "session1.jsonl"),
		filepath.Join(sub, "session2.jsonl"),
	}
	for _, p := range want {
		if err := os.WriteFile(p, []byte(`{}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Non-jsonl file — should not appear.
	if err := os.WriteFile(filepath.Join(root, "readme.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := loader.ScanJSONL(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sort.Strings(got)
	sort.Strings(want)

	if len(got) != len(want) {
		t.Fatalf("got %d files, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("file[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestScanJSONL_EmptyDir(t *testing.T) {
	root := t.TempDir()
	got, err := loader.ScanJSONL(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestScanJSONL_NotExist(t *testing.T) {
	_, err := loader.ScanJSONL("/nonexistent-path-cc-history-test")
	if err == nil {
		t.Fatal("expected error for nonexistent directory, got nil")
	}
}

func TestScanJSONL_IsFile(t *testing.T) {
	f, err := os.CreateTemp("", "*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	_, err = loader.ScanJSONL(f.Name())
	if err == nil {
		t.Fatal("expected error when path is a file, got nil")
	}
}

func TestScanJSONL_Performance(t *testing.T) {
	root := t.TempDir()

	// Create 1000 .jsonl files across 10 subdirs.
	for i := 0; i < 10; i++ {
		sub := filepath.Join(root, fmt.Sprintf("dir%d", i))
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		for j := 0; j < 100; j++ {
			name := filepath.Join(sub, fmt.Sprintf("s%d.jsonl", j))
			if err := os.WriteFile(name, []byte(`{}`), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}

	start := time.Now()
	got, err := loader.ScanJSONL(root)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1000 {
		t.Fatalf("expected 1000 files, got %d", len(got))
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("scan took %v, want < 100ms", elapsed)
	}
}
