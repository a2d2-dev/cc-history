package loader_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/a2d2-dev/cc-history/internal/loader"
)

// writeJSONLMessages writes a JSONL file with timestamped user messages.
func writeJSONLMessages(t *testing.T, dir, name, sessionID string, timestamps []time.Time) string {
	t.Helper()
	var content []byte
	for _, ts := range timestamps {
		type rec struct {
			Type      string `json:"type"`
			SessionID string `json:"sessionId"`
			Timestamp string `json:"timestamp"`
			Message   struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
		}
		r := rec{Type: "user", SessionID: sessionID, Timestamp: ts.Format(time.RFC3339Nano)}
		r.Message.Role = "user"
		r.Message.Content = "msg"
		b, _ := json.Marshal(r)
		content = append(content, b...)
		content = append(content, '\n')
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadAllSessions_SortedByTime(t *testing.T) {
	root := t.TempDir()

	t2 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	writeJSONLMessages(t, root, "later.jsonl", "sess-later", []time.Time{t2})
	writeJSONLMessages(t, root, "earlier.jsonl", "sess-earlier", []time.Time{t1})

	sessions, err := loader.LoadAllSessions(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	if sessions[0].ID != "sess-earlier" {
		t.Errorf("expected sess-earlier first, got %q", sessions[0].ID)
	}
	if sessions[1].ID != "sess-later" {
		t.Errorf("expected sess-later second, got %q", sessions[1].ID)
	}
}

func TestLoadAllSessions_EmptyDir(t *testing.T) {
	root := t.TempDir()
	sessions, err := loader.LoadAllSessions(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestLoadAllSessions_Performance(t *testing.T) {
	// 10 sessions x 100 messages = 1000 messages, must load < 2s (AC5).
	root := t.TempDir()
	ts := make([]time.Time, 100)
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range ts {
		ts[i] = base.Add(time.Duration(i) * time.Minute)
	}
	for i := 0; i < 10; i++ {
		writeJSONLMessages(t, root, fmt.Sprintf("s%d.jsonl", i), fmt.Sprintf("sess-%d", i), ts)
	}

	start := time.Now()
	sessions, err := loader.LoadAllSessions(root)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 10 {
		t.Fatalf("expected 10 sessions, got %d", len(sessions))
	}
	totalMsgs := 0
	for _, s := range sessions {
		totalMsgs += len(s.Messages)
	}
	if totalMsgs != 1000 {
		t.Fatalf("expected 1000 messages total, got %d", totalMsgs)
	}
	if elapsed > 2*time.Second {
		t.Errorf("LoadAllSessions took %v, want < 2s", elapsed)
	}
}
