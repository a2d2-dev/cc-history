package loader

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// newUUID generates a random UUID v4 string (xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx).
func newUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	), nil
}

// DuplicateSession copies a session JSONL file into the same directory with a
// new UUID. All sessionId values within the JSONL are replaced with the new
// UUID. Returns the path of the new file and the new session UUID.
func DuplicateSession(filePath string) (newPath, newID string, err error) {
	newID, err = newUUID()
	if err != nil {
		return "", "", fmt.Errorf("generate uuid: %w", err)
	}

	dir := filepath.Dir(filePath)
	newPath = filepath.Join(dir, newID+".jsonl")

	src, err := os.Open(filePath)
	if err != nil {
		return "", "", fmt.Errorf("open source: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(newPath)
	if err != nil {
		return "", "", fmt.Errorf("create dest: %w", err)
	}
	defer func() {
		dst.Close()
		if err != nil {
			os.Remove(newPath) //nolint:errcheck // best-effort cleanup
		}
	}()

	// Extract the original session ID from the first line so we can replace it.
	oldID := firstSessionID(filePath)

	w := bufio.NewWriter(dst)
	scanner := bufio.NewScanner(src)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if oldID != "" {
			line = bytes.ReplaceAll(line, []byte(oldID), []byte(newID))
		}
		// Also replace filename references (old UUID in string context).
		oldBase := strings.TrimSuffix(filepath.Base(filePath), ".jsonl")
		if oldBase != oldID && oldBase != "" {
			line = bytes.ReplaceAll(line, []byte(oldBase), []byte(newID))
		}
		if _, err = w.Write(line); err != nil {
			return "", "", fmt.Errorf("write line: %w", err)
		}
		if err = w.WriteByte('\n'); err != nil {
			return "", "", fmt.Errorf("write newline: %w", err)
		}
	}
	if err = scanner.Err(); err != nil {
		return "", "", fmt.Errorf("scan source: %w", err)
	}
	if err = w.Flush(); err != nil {
		return "", "", fmt.Errorf("flush: %w", err)
	}
	return newPath, newID, nil
}
