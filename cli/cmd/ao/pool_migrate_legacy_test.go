// practices: [microservices, sre]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// ---------------------------------------------------------------------------
// outputPoolMigrateLegacyResult
// ---------------------------------------------------------------------------

func TestOutputPoolMigrateLegacyResult_Human(t *testing.T) {
	origOutput := output
	origDryRun := dryRun
	output = "table"
	dryRun = false
	defer func() { output = origOutput; dryRun = origDryRun }()

	result := poolMigrateLegacyResult{
		Scanned:  10,
		Eligible: 5,
		Moved:    3,
		Skipped:  2,
		Errors:   0,
	}

	out, err := captureStdout(t, func() error {
		return outputPoolMigrateLegacyResult(result)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "moved=3") {
		t.Errorf("missing moved count, got: %q", out)
	}
	if !strings.Contains(out, "scanned=10") {
		t.Errorf("missing scanned count, got: %q", out)
	}
}

func TestOutputPoolMigrateLegacyResult_DryRun(t *testing.T) {
	origOutput := output
	origDryRun := dryRun
	output = "table"
	dryRun = true
	defer func() { output = origOutput; dryRun = origDryRun }()

	result := poolMigrateLegacyResult{
		Moved: 2,
		Moves: []legacyMove{
			{From: "/tmp/old/a.md", To: "/tmp/new/a.md"},
			{From: "/tmp/old/b.md", To: "/tmp/new/b.md"},
		},
	}

	out, err := captureStdout(t, func() error {
		return outputPoolMigrateLegacyResult(result)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "[dry-run]") {
		t.Errorf("missing dry-run marker, got: %q", out)
	}
	if !strings.Contains(out, "a.md") {
		t.Errorf("missing move entry, got: %q", out)
	}
}

func TestOutputPoolMigrateLegacyResult_JSON(t *testing.T) {
	origOutput := output
	output = "json"
	defer func() { output = origOutput }()

	result := poolMigrateLegacyResult{
		Scanned: 5,
		Moved:   2,
	}

	out, err := captureStdout(t, func() error {
		return outputPoolMigrateLegacyResult(result)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed poolMigrateLegacyResult
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.Scanned != 5 {
		t.Errorf("Scanned = %d, want 5", parsed.Scanned)
	}
}
