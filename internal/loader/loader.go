package loader

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

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
