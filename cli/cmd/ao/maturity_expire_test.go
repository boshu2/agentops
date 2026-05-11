// practices: [cmm-process-maturity, dora-metrics]
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func createTestLearning(t *testing.T, dir, name, frontmatter string) {
	t.Helper()
	content := "---\n" + frontmatter + "\n---\n\n# Test Learning\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write test file %s: %v", name, err)
	}
}

func TestMaturityExpireScan(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	createTestLearning(t, learningsDir, "active.md", "valid_until: 2099-12-31")
	createTestLearning(t, learningsDir, "expired.md", "valid_until: 2020-01-01")
	createTestLearning(t, learningsDir, "no-expiry.md", "title: some learning")
	createTestLearning(t, learningsDir, "archived.md", "valid_until: 2020-01-01\nexpiry_status: archived")

	// Change to temp dir so runMaturityExpire picks it up
	t.Chdir(tmp)

	// Reset flags
	maturityArchive = false
	maturityExpire = true

	err := runMaturityExpire(nil)
	if err != nil {
		t.Fatalf("runMaturityExpire failed: %v", err)
	}
}

func TestMaturityExpireArchive(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	createTestLearning(t, learningsDir, "expired1.md", "valid_until: 2020-01-01")
	createTestLearning(t, learningsDir, "expired2.md", "valid_until: 2021-06-15")
	createTestLearning(t, learningsDir, "active.md", "valid_until: 2099-12-31")

	t.Chdir(tmp)

	maturityArchive = true
	maturityExpire = true
	dryRun = false

	err := runMaturityExpire(nil)
	if err != nil {
		t.Fatalf("runMaturityExpire with archive failed: %v", err)
	}

	// Verify expired files were moved
	archiveDir := filepath.Join(tmp, ".agents", "archive", "learnings")
	for _, name := range []string{"expired1.md", "expired2.md"} {
		if _, err := os.Stat(filepath.Join(archiveDir, name)); os.IsNotExist(err) {
			t.Errorf("expected %s to be archived, but not found in archive dir", name)
		}
		if _, err := os.Stat(filepath.Join(learningsDir, name)); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed from learnings dir", name)
		}
	}

	// Verify active file was NOT moved
	if _, err := os.Stat(filepath.Join(learningsDir, "active.md")); os.IsNotExist(err) {
		t.Error("active.md should not have been archived")
	}
}

func TestMaturityExpireNoValidUntil(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	createTestLearning(t, learningsDir, "no-date1.md", "title: learning one")
	createTestLearning(t, learningsDir, "no-date2.md", "maturity: provisional")
	createTestLearning(t, learningsDir, "has-date.md", "valid_until: 2099-01-01")

	t.Chdir(tmp)

	maturityArchive = false
	maturityExpire = true

	// Should not error — files without valid_until are counted as never-expiring
	err := runMaturityExpire(nil)
	if err != nil {
		t.Fatalf("runMaturityExpire failed: %v", err)
	}
}

func TestMaturityExpireMalformedYAML(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	createTestLearning(t, learningsDir, "bad-date.md", "valid_until: not-a-date")
	createTestLearning(t, learningsDir, "empty-date.md", "valid_until: ")
	createTestLearning(t, learningsDir, "good.md", "valid_until: 2099-12-31")

	t.Chdir(tmp)

	maturityArchive = false
	maturityExpire = true

	// Should not crash — malformed dates treated as never-expiring
	err := runMaturityExpire(nil)
	if err != nil {
		t.Fatalf("runMaturityExpire should not fail on malformed dates: %v", err)
	}
}
