package search

import (
	"os"
	"path/filepath"
)

// TruncateText truncates a string to max length with ellipsis.
// Uses rune-safe slicing to avoid breaking multi-byte UTF-8 characters.
func TruncateText(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	// Fast path: byte length is an upper bound on rune count, so any input
	// whose byte length already fits needs no rune conversion.
	if len(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return "..."[:maxLen]
	}
	return string(runes[:maxLen-3]) + "..."
}

// AtomicWriteFile writes data to a temp file then renames into place,
// preventing corruption from crashes or concurrent writes.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".ao-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

// QuarantineLearning moves a learning file to .quarantine/ subdirectory.
func QuarantineLearning(path string) error {
	dir := filepath.Dir(path)
	quarantineDir := filepath.Join(dir, ".quarantine")
	if err := os.MkdirAll(quarantineDir, 0o755); err != nil {
		return err
	}
	base := filepath.Base(path)
	dest := filepath.Join(quarantineDir, base)
	return os.Rename(path, dest)
}
