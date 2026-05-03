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
