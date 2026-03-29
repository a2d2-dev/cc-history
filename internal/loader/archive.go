package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ArchiveRoot returns the archive directory for the given sessions root.
// If sessionsRoot ends in "projects", it replaces the last segment with
// "projects-archive". Otherwise it appends "-archive".
func ArchiveRoot(sessionsRoot string) string {
	clean := filepath.Clean(sessionsRoot)
	base := filepath.Base(clean)
	if base == "projects" {
		return filepath.Join(filepath.Dir(clean), "projects-archive")
	}
	return clean + "-archive"
}

// ArchiveSession moves a session file from sessionsRoot into the archive directory,
// preserving its relative path structure. Returns the destination path.
func ArchiveSession(sessionsRoot, filePath string) (string, error) {
	return moveSessionBetween(sessionsRoot, ArchiveRoot(sessionsRoot), filePath)
}

// RestoreSession moves a session file from the archive directory back into
// sessionsRoot, preserving its relative path structure. Returns the destination path.
func RestoreSession(sessionsRoot, filePath string) (string, error) {
	return moveSessionBetween(ArchiveRoot(sessionsRoot), sessionsRoot, filePath)
}

// moveSessionBetween moves filePath from srcRoot to dstRoot preserving the
// relative sub-path. Returns the destination path on success.
func moveSessionBetween(srcRoot, dstRoot, filePath string) (string, error) {
	cleanSrc := filepath.Clean(srcRoot)
	cleanFile := filepath.Clean(filePath)

	rel, err := filepath.Rel(cleanSrc, cleanFile)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("file %s is not under %s", filePath, srcRoot)
	}

	dst := filepath.Join(dstRoot, rel)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", fmt.Errorf("create archive dir: %w", err)
	}
	if err := os.Rename(cleanFile, dst); err != nil {
		return "", fmt.Errorf("move session file: %w", err)
	}
	return dst, nil
}
