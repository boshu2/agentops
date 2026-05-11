// practices: [dora-metrics, continuous-delivery]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestShouldMigrateFile(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		isDir bool
		want  bool
	}{
		{"markdown file", "test.md", false, true},
		{"nested markdown", "dir/sub/file.md", false, true},
		{"go file", "test.go", false, false},
		{"txt file", "test.txt", false, false},
		{"directory", "somedir", true, false},
		{"md directory", "test.md", true, false},
		{"json file", "data.json", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := fakeFileInfo{name: filepath.Base(tt.path), isDir: tt.isDir}
			got := shouldMigrateFile(tt.path, info)
			if got != tt.want {
				t.Errorf("shouldMigrateFile(%q, isDir=%v) = %v, want %v", tt.path, tt.isDir, got, tt.want)
			}
		})
	}
}

func TestFindSchemaInsertPoint(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  int
	}{
		{
			name:  "after Date field",
			lines: []string{"# Title", "**Date:** 2025-01-15", "some content"},
			want:  2,
		},
		{
			name:  "after Epic field",
			lines: []string{"# Title", "**Epic:** ol-0001", "some content"},
			want:  2,
		},
		{
			name:  "Date takes priority over heading",
			lines: []string{"# Title", "**Date:** 2025-01-15", "more"},
			want:  2,
		},
		{
			name:  "fallback to after heading",
			lines: []string{"# Title", "some content", "more content"},
			want:  1,
		},
		{
			name:  "no heading or date",
			lines: []string{"plain text", "more text"},
			want:  -1,
		},
		{
			name:  "empty lines",
			lines: []string{},
			want:  -1,
		},
		{
			name:  "heading at end only line",
			lines: []string{"# Title"},
			want:  -1,
		},
		{
			name:  "heading with content after",
			lines: []string{"# Title", "body"},
			want:  1,
		},
		{
			name:  "multiple headings uses first",
			lines: []string{"# First", "# Second", "body"},
			want:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findSchemaInsertPoint(tt.lines)
			if got != tt.want {
				t.Errorf("findSchemaInsertPoint() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestMigrateFile_AddsSchemaVersion(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.md")

	content := "# Test Article\n**Date:** 2025-01-15\nBody text here.\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	origDryRun := dryRun
	dryRun = false
	defer func() { dryRun = origDryRun }()

	result := migrateFile(path, info)
	if result != migrateResultSuccess {
		t.Fatalf("migrateFile() = %d, want migrateResultSuccess (%d)", result, migrateResultSuccess)
	}

	migrated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migrated file: %v", err)
	}

	if !strings.Contains(string(migrated), "**Schema Version:** 1") {
		t.Errorf("migrated file missing schema version field\nContent:\n%s", string(migrated))
	}
}

func TestMigrateFile_SkipsExistingSchemaVersion(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.md")

	content := "# Test\n**Schema Version:** 1\n**Date:** 2025-01-15\nBody.\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	result := migrateFile(path, info)
	if result != migrateResultSkipped {
		t.Errorf("migrateFile() = %d, want migrateResultSkipped (%d)", result, migrateResultSkipped)
	}
}

func TestMigrateFile_SkipsLowercaseSchemaVersion(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.md")

	content := "# Test\nschema_version: 1\n**Date:** 2025-01-15\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	result := migrateFile(path, info)
	if result != migrateResultSkipped {
		t.Errorf("migrateFile() = %d, want migrateResultSkipped", result)
	}
}

func TestMigrateFile_SkipsNoInsertPoint(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.md")

	content := "plain text with no heading or date field\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	result := migrateFile(path, info)
	if result != migrateResultSkipped {
		t.Errorf("migrateFile() = %d, want migrateResultSkipped", result)
	}
}

func TestMigrateFile_DryRunDoesNotWrite(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.md")

	content := "# Test\n**Date:** 2025-01-15\nBody.\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	origDryRun := dryRun
	dryRun = true
	defer func() { dryRun = origDryRun }()

	result := migrateFile(path, info)
	if result != migrateResultSuccess {
		t.Fatalf("migrateFile() dry-run = %d, want migrateResultSuccess", result)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	if string(after) != content {
		t.Errorf("file was modified during dry-run\nBefore: %s\nAfter: %s", content, string(after))
	}
}

func TestMigrateFile_NonexistentFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.md")

	info := fakeFileInfo{name: "does-not-exist.md", isDir: false}

	result := migrateFile(path, info)
	if result != migrateResultError {
		t.Errorf("migrateFile(nonexistent) = %d, want migrateResultError (%d)", result, migrateResultError)
	}
}

func TestMigrateFile_InsertAfterEpicField(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.md")

	content := "# Test\n**Epic:** ol-0001\nBody text.\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	origDryRun := dryRun
	dryRun = false
	defer func() { dryRun = origDryRun }()

	result := migrateFile(path, info)
	if result != migrateResultSuccess {
		t.Fatalf("migrateFile() = %d, want migrateResultSuccess", result)
	}

	migrated, _ := os.ReadFile(path)
	if !strings.Contains(string(migrated), "**Schema Version:** 1") {
		t.Errorf("missing schema version after Epic field\nContent:\n%s", string(migrated))
	}
}

func TestMigrateResult_Constants(t *testing.T) {
	if migrateResultSuccess == migrateResultSkipped {
		t.Error("migrateResultSuccess should differ from migrateResultSkipped")
	}
	if migrateResultSuccess == migrateResultError {
		t.Error("migrateResultSuccess should differ from migrateResultError")
	}
	if migrateResultSkipped == migrateResultError {
		t.Error("migrateResultSkipped should differ from migrateResultError")
	}
}

// --- runRatchetMigrate tests ---

func TestRatchetMigrate_runRatchetMigrate_dryRun(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)

	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	cmd := &cobra.Command{}
	got := captureJSONStdout(t, func() {
		err := runRatchetMigrate(cmd, nil)
		if err != nil {
			t.Fatalf("runRatchetMigrate dry-run: %v", err)
		}
	})

	if !strings.Contains(got, "Would migrate chain from") {
		t.Fatalf("expected dry-run output, got: %s", got)
	}
}

func TestRatchetMigrate_runRatchetMigrate_noLegacy(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)

	// Create .agents/ dir but no legacy chain
	if err := os.MkdirAll(filepath.Join(tmp, ".agents"), 0755); err != nil {
		t.Fatal(err)
	}

	oldDryRun := dryRun
	dryRun = false
	defer func() { dryRun = oldDryRun }()

	cmd := &cobra.Command{}
	err := runRatchetMigrate(cmd, nil)
	if err == nil {
		t.Fatal("expected error when no legacy chain")
	}
	if !strings.Contains(err.Error(), "migrate chain") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRatchetMigrate_runRatchetMigrate_withLegacy(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)

	// Create .agents/provenance/chain.yaml with valid YAML
	legacyDir := filepath.Join(tmp, ".agents", "provenance")
	aoDir := filepath.Join(tmp, ".agents", "ao")
	if err := os.MkdirAll(legacyDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(aoDir, 0755); err != nil {
		t.Fatal(err)
	}

	legacyContent := `entries:
  - step: research
    gate: research
    status: passed
    timestamp: "2026-01-01T00:00:00Z"
    artifact_path: ".agents/research/test.md"
`
	if err := os.WriteFile(filepath.Join(legacyDir, "chain.yaml"), []byte(legacyContent), 0644); err != nil {
		t.Fatal(err)
	}

	oldDryRun := dryRun
	dryRun = false
	defer func() { dryRun = oldDryRun }()

	cmd := &cobra.Command{}
	got := captureJSONStdout(t, func() {
		err := runRatchetMigrate(cmd, nil)
		if err != nil {
			t.Fatalf("runRatchetMigrate with legacy: %v", err)
		}
	})

	if !strings.Contains(got, "Migrated") || !strings.Contains(got, "Migration complete") {
		t.Fatalf("expected migration output, got: %s", got)
	}
}

// --- runMigrateArtifacts tests ---

func TestRatchetMigrate_runMigrateArtifacts_noArgs(t *testing.T) {
	tmp := t.TempDir()
	chdirTo(t, tmp)

	// Create .agents/ directory with a markdown file
	agentsDir := filepath.Join(tmp, ".agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}
	mdContent := "# Test Artifact\n**Date:** 2026-01-01\nSome content\n"
	if err := os.WriteFile(filepath.Join(agentsDir, "test.md"), []byte(mdContent), 0644); err != nil {
		t.Fatal(err)
	}

	oldDryRun := dryRun
	dryRun = false
	defer func() { dryRun = oldDryRun }()

	cmd := &cobra.Command{}
	got := captureJSONStdout(t, func() {
		err := runMigrateArtifacts(cmd, nil)
		if err != nil {
			t.Fatalf("runMigrateArtifacts no args: %v", err)
		}
	})

	if !strings.Contains(got, "Summary:") {
		t.Fatalf("expected summary output, got: %s", got)
	}
}

func TestRatchetMigrate_runMigrateArtifacts_withPath(t *testing.T) {
	tmp := t.TempDir()

	// Create a subdir with a markdown file
	subDir := filepath.Join(tmp, "learnings")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	mdContent := "# Learning\n**Date:** 2026-01-01\nContent\n"
	if err := os.WriteFile(filepath.Join(subDir, "learn.md"), []byte(mdContent), 0644); err != nil {
		t.Fatal(err)
	}

	oldDryRun := dryRun
	dryRun = false
	defer func() { dryRun = oldDryRun }()

	cmd := &cobra.Command{}
	got := captureJSONStdout(t, func() {
		err := runMigrateArtifacts(cmd, []string{subDir})
		if err != nil {
			t.Fatalf("runMigrateArtifacts with path: %v", err)
		}
	})

	if !strings.Contains(got, "Summary:") {
		t.Fatalf("expected summary output, got: %s", got)
	}
}

func TestRatchetMigrate_runMigrateArtifacts_dryRun(t *testing.T) {
	tmp := t.TempDir()

	mdContent := "# Artifact\n**Epic:** test-epic\nSome content\n"
	if err := os.WriteFile(filepath.Join(tmp, "artifact.md"), []byte(mdContent), 0644); err != nil {
		t.Fatal(err)
	}

	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	cmd := &cobra.Command{}
	got := captureJSONStdout(t, func() {
		err := runMigrateArtifacts(cmd, []string{tmp})
		if err != nil {
			t.Fatalf("runMigrateArtifacts dry-run: %v", err)
		}
	})

	if !strings.Contains(got, "Would add schema_version") {
		t.Fatalf("expected dry-run output, got: %s", got)
	}
}

// --- shouldMigrateFile tests ---

func TestRatchetMigrate_shouldMigrateFile_markdown(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "test.md")
	if err := os.WriteFile(p, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if !shouldMigrateFile(p, info) {
		t.Fatal("expected markdown file to be eligible")
	}
}

func TestRatchetMigrate_shouldMigrateFile_nonMarkdown(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "test.json")
	if err := os.WriteFile(p, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if shouldMigrateFile(p, info) {
		t.Fatal("expected non-markdown file to be ineligible")
	}
}

// --- findSchemaInsertPoint tests ---

func TestRatchetMigrate_findSchemaInsertPoint_afterDate(t *testing.T) {
	lines := []string{"# Title", "**Date:** 2026-01-01", "Content"}
	idx := findSchemaInsertPoint(lines)
	if idx != 2 {
		t.Fatalf("expected insert at 2, got %d", idx)
	}
}

func TestRatchetMigrate_findSchemaInsertPoint_afterEpic(t *testing.T) {
	lines := []string{"# Title", "**Epic:** test", "Content"}
	idx := findSchemaInsertPoint(lines)
	if idx != 2 {
		t.Fatalf("expected insert at 2, got %d", idx)
	}
}

func TestRatchetMigrate_findSchemaInsertPoint_afterHeading(t *testing.T) {
	lines := []string{"# Title", "Some content", "More"}
	idx := findSchemaInsertPoint(lines)
	if idx != 1 {
		t.Fatalf("expected insert at 1 (after heading), got %d", idx)
	}
}

func TestRatchetMigrate_findSchemaInsertPoint_noMatch(t *testing.T) {
	lines := []string{"no heading", "no date", "no epic"}
	idx := findSchemaInsertPoint(lines)
	if idx != -1 {
		t.Fatalf("expected -1 for no match, got %d", idx)
	}
}

// --- migrateFile tests ---

func TestRatchetMigrate_migrateFile_alreadyHasSchema(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "already.md")
	content := "# Title\n**Schema Version:** 1\nContent\n"
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	result := migrateFile(p, info)
	if result != migrateResultSkipped {
		t.Fatalf("expected skipped for file with schema, got %d", result)
	}
}

func TestRatchetMigrate_migrateFile_success(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "migrate-me.md")
	content := "# Title\n**Date:** 2026-01-01\nContent\n"
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	oldDryRun := dryRun
	dryRun = false
	defer func() { dryRun = oldDryRun }()

	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}

	captureJSONStdout(t, func() {
		result := migrateFile(p, info)
		if result != migrateResultSuccess {
			t.Fatalf("expected success, got %d", result)
		}
	})

	// Verify schema_version was added
	updated, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "**Schema Version:** 1") {
		t.Fatalf("expected schema version in updated file, got: %s", string(updated))
	}
}

func TestRatchetMigrate_migrateFile_noInsertPoint(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "no-heading.md")
	content := "plain text\nno heading\nno date\n"
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	result := migrateFile(p, info)
	if result != migrateResultSkipped {
		t.Fatalf("expected skipped for no insert point, got %d", result)
	}
}
