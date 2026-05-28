// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestQuarantineLearning_MovesFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "test-learning.md")
	if err := os.WriteFile(src, []byte("# Test Learning\nSome content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := quarantineLearning(src, "test reason"); err != nil {
		t.Fatalf("quarantineLearning: %v", err)
	}

	// Original should be gone
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("original file still exists after quarantine")
	}

	// Quarantined file should exist
	dest := filepath.Join(dir, ".quarantine", "test-learning.md")
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("quarantined file not found: %v", err)
	}
}

func TestQuarantineLearning_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "another-learning.md")
	if err := os.WriteFile(src, []byte("# Another"), 0o644); err != nil {
		t.Fatal(err)
	}

	quarantineDir := filepath.Join(dir, ".quarantine")
	// Verify .quarantine doesn't exist yet
	if _, err := os.Stat(quarantineDir); !os.IsNotExist(err) {
		t.Fatal(".quarantine dir should not exist before test")
	}

	if err := quarantineLearning(src, "missing dir test"); err != nil {
		t.Fatalf("quarantineLearning: %v", err)
	}

	// .quarantine dir should now exist
	info, err := os.Stat(quarantineDir)
	if err != nil {
		t.Fatalf(".quarantine dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error(".quarantine is not a directory")
	}
}
