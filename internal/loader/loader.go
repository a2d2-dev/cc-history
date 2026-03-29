package loader

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/a2d2-dev/cc-history/internal/parser"
)

// JSONLFile holds a path and its modification time.
type JSONLFile struct {
	Path    string
	ModTime time.Time
}

// DefaultSessionsPath returns the default Claude Code projects directory.
func DefaultSessionsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".claude", "projects"), nil
}

// ScanJSONL walks root recursively and returns all *.jsonl file paths.
// Returns a friendly error if root does not exist or is not a directory.
func ScanJSONL(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session directory not found: %s\n(use --path <dir> to specify an alternate path)", root)
		}
		return nil, fmt.Errorf("cannot access %s: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}

	var paths []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Skip unreadable entries gracefully.
			return nil
		}
		if !d.IsDir() && filepath.Ext(path) == ".jsonl" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}
	return paths, nil
}

// scanJSONLMeta walks root and returns files with modification times.
func scanJSONLMeta(root string) ([]JSONLFile, error) {
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session directory not found: %s", root)
		}
		return nil, fmt.Errorf("cannot access %s: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}

	var files []JSONLFile
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if !d.IsDir() && filepath.Ext(path) == ".jsonl" {
			fi, err := d.Info()
			if err != nil {
				return nil
			}
			files = append(files, JSONLFile{Path: path, ModTime: fi.ModTime()})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}
	return files, nil
}

// firstSessionID reads the first non-empty line of a JSONL file and returns
// its sessionId field, or "" if not found or on error.
func firstSessionID(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 4096)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec struct {
			SessionID string `json:"sessionId"`
		}
		if err := json.Unmarshal(line, &rec); err == nil && rec.SessionID != "" {
			return rec.SessionID
		}
		break // only check first non-empty line
	}
	return ""
}

// FindCurrentSession locates the current session file within root.
//
// Detection order:
//  1. If CLAUDE_SESSION_ID is set, find the JSONL file whose first record
//     matches that session ID (filename match first, then content scan).
//  2. Otherwise, return the most recently modified JSONL file.
//
// Returns (path, isFallback, error).
// isFallback is true when falling back to most-recent because the env var
// was unset or no matching file was found.
func FindCurrentSession(root string) (path string, isFallback bool, err error) {
	files, err := scanJSONLMeta(root)
	if err != nil {
		return "", false, err
	}
	if len(files) == 0 {
		return "", false, fmt.Errorf("no session files found in %s", root)
	}

	sessionID := os.Getenv("CLAUDE_SESSION_ID")
	if sessionID != "" {
		// Fast path: check if any filename contains the session ID.
		for _, f := range files {
			base := filepath.Base(f.Path)
			if base == sessionID+".jsonl" || base == sessionID {
				return f.Path, false, nil
			}
		}
		// Slow path: scan first line of each file for matching sessionId.
		for _, f := range files {
			if firstSessionID(f.Path) == sessionID {
				return f.Path, false, nil
			}
		}
		// Session ID set but not found — fall through to most-recent.
	}

	// Find most recently modified file.
	newest := files[0]
	for _, f := range files[1:] {
		if f.ModTime.After(newest.ModTime) {
			newest = f
		}
	}
	return newest.Path, true, nil
}

// FindSessionByID locates the JSONL file whose session ID matches id.
// It first checks filenames (id.jsonl), then scans the first line of each file.
// Returns an error if no match is found.
func FindSessionByID(root, id string) (string, error) {
	files, err := scanJSONLMeta(root)
	if err != nil {
		return "", err
	}
	// Fast path: filename match.
	for _, f := range files {
		base := filepath.Base(f.Path)
		if base == id+".jsonl" || base == id {
			return f.Path, nil
		}
	}
	// Slow path: scan first line of each file.
	for _, f := range files {
		if firstSessionID(f.Path) == id {
			return f.Path, nil
		}
	}
	return "", fmt.Errorf("session not found: %s", id)
}

// LoadAllSessionsMeta scans every JSONL file under root using lightweight
// metadata-only parsing (no content allocation). The session whose path equals
// currentFilePath gets its LastMessage populated. Results are sorted by
// StartTime (oldest first). Files that fail to scan are silently skipped.
func LoadAllSessionsMeta(root, currentFilePath string) ([]*parser.SessionMeta, error) {
	paths, err := ScanJSONL(root)
	if err != nil {
		return nil, err
	}
	metas := make([]*parser.SessionMeta, 0, len(paths))
	for _, p := range paths {
		m, err := parser.ParseFileMeta(p, p == currentFilePath)
		if err != nil {
			continue
		}
		metas = append(metas, m)
	}
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].StartTime.Before(metas[j].StartTime)
	})
	return metas, nil
}

// LoadAllSessions parses every JSONL file under root and returns all sessions
// sorted by their earliest message timestamp (oldest first).
// Files that cannot be parsed are silently skipped.
func LoadAllSessions(root string) ([]*parser.Session, error) {
	paths, err := ScanJSONL(root)
	if err != nil {
		return nil, err
	}
	sessions := make([]*parser.Session, 0, len(paths))
	for _, p := range paths {
		s, err := parser.ParseFile(p)
		if err != nil {
			continue
		}
		sessions = append(sessions, s)
	}
	sort.Slice(sessions, func(i, j int) bool {
		ti := firstMessageTime(sessions[i])
		tj := firstMessageTime(sessions[j])
		return ti.Before(tj)
	})
	return sessions, nil
}

// firstMessageTime returns the timestamp of the first message with a non-zero
// time in session, or the zero time if none exist.
func firstMessageTime(s *parser.Session) time.Time {
	for _, m := range s.Messages {
		if !m.Timestamp.IsZero() {
			return m.Timestamp
		}
	}
	return time.Time{}
}
