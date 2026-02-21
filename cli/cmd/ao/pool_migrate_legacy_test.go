package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateLegacyKnowledgeFiles_MovesEligibleAndRenamesOnCollision(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, ".agents", "knowledge")
	pendingDir := filepath.Join(sourceDir, "pending")
	if err := os.MkdirAll(sourceDir, 0o700); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.MkdirAll(pendingDir, 0o700); err != nil {
		t.Fatalf("mkdir pending: %v", err)
	}

	legacy := `---
type: learning
date: 2026-02-20
---

# Fix shell PATH mismatch for ao detection
`
	legacyPath := filepath.Join(sourceDir, "legacy.md")
	if err := os.WriteFile(legacyPath, []byte(legacy), 0o600); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	if err := os.WriteFile(filepath.Join(sourceDir, "notes.md"), []byte("# random note"), 0o600); err != nil {
		t.Fatalf("write random: %v", err)
	}

	// Pre-create destination so migration has to suffix with -migrated-1.
	if err := os.WriteFile(filepath.Join(pendingDir, "legacy.md"), []byte("# existing"), 0o600); err != nil {
		t.Fatalf("write existing pending: %v", err)
	}

	origDryRun := dryRun
	dryRun = false
	t.Cleanup(func() { dryRun = origDryRun })

	res, err := migrateLegacyKnowledgeFiles(sourceDir, pendingDir)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if res.Scanned != 2 || res.Eligible != 1 || res.Moved != 1 || res.Skipped != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}

	migratedPath := filepath.Join(pendingDir, "legacy-migrated-1.md")
	if _, err := os.Stat(migratedPath); err != nil {
		t.Fatalf("expected migrated file at %s: %v", migratedPath, err)
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("expected source legacy file moved, stat err=%v", err)
	}
}

func TestMigrateLegacyKnowledgeFiles_DryRun(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, ".agents", "knowledge")
	pendingDir := filepath.Join(sourceDir, "pending")
	if err := os.MkdirAll(sourceDir, 0o700); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}

	legacy := `---
type: learning
date: 2026-02-20
---

# Dry run learning
`
	legacyPath := filepath.Join(sourceDir, "legacy.md")
	if err := os.WriteFile(legacyPath, []byte(legacy), 0o600); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	origDryRun := dryRun
	dryRun = true
	t.Cleanup(func() { dryRun = origDryRun })

	res, err := migrateLegacyKnowledgeFiles(sourceDir, pendingDir)
	if err != nil {
		t.Fatalf("migrate dry-run: %v", err)
	}
	if res.Moved != 1 || len(res.Moves) != 1 {
		t.Fatalf("unexpected dry-run result: %+v", res)
	}
	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("source file should not move in dry-run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(pendingDir, "legacy.md")); !os.IsNotExist(err) {
		t.Fatalf("pending file should not exist in dry-run, stat err=%v", err)
	}
}
