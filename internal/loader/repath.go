package loader

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// RepathSession rewrites filePath replacing every occurrence of oldCWD with
// newCWD in the "cwd" JSON field of each JSONL line.  The file is written
// atomically (write to temp file in same directory, then rename).
func RepathSession(filePath, oldCWD, newCWD string) error {
	if oldCWD == newCWD {
		return nil
	}

	src, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer src.Close()

	dir := filepath.Dir(filePath)
	tmp, err := os.CreateTemp(dir, "cc-repath-*.jsonl")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	success := false
	defer func() {
		tmp.Close()
		if !success {
			os.Remove(tmpPath) //nolint:errcheck // best-effort cleanup on failure
		}
	}()

	// Build the exact JSON byte sequences to find and replace.
	// Matching `"cwd":<value>` ensures we only replace the cwd field.
	oldJSON, err := json.Marshal(oldCWD)
	if err != nil {
		return fmt.Errorf("marshal old cwd: %w", err)
	}
	newJSON, err := json.Marshal(newCWD)
	if err != nil {
		return fmt.Errorf("marshal new cwd: %w", err)
	}
	oldPattern := append([]byte(`"cwd":`), oldJSON...)
	newPattern := append([]byte(`"cwd":`), newJSON...)

	w := bufio.NewWriter(tmp)
	scanner := bufio.NewScanner(src)
	const maxBuf = 16 * 1024 * 1024
	scanner.Buffer(make([]byte, maxBuf), maxBuf)
	for scanner.Scan() {
		line := scanner.Bytes()
		line = bytes.ReplaceAll(line, oldPattern, newPattern)
		if _, err = w.Write(line); err != nil {
			return fmt.Errorf("write: %w", err)
		}
		if err = w.WriteByte('\n'); err != nil {
			return fmt.Errorf("write newline: %w", err)
		}
	}
	if err = scanner.Err(); err != nil {
		return fmt.Errorf("scan: %w", err)
	}
	if err = w.Flush(); err != nil {
		return fmt.Errorf("flush: %w", err)
	}
	src.Close()
	tmp.Close()

	if err = os.Rename(tmpPath, filePath); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	success = true
	return nil
}
