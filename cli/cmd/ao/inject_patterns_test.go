package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePatternFile(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("full pattern file", func(t *testing.T) {
		content := `# Mutex Guard Pattern

Always acquire mutex before accessing shared state.
Release in defer to prevent deadlocks.

## Example
...
`
		path := filepath.Join(tmpDir, "mutex-guard.md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		p, err := parsePatternFile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Name != "Mutex Guard Pattern" {
			t.Errorf("Name = %q, want %q", p.Name, "Mutex Guard Pattern")
		}
		if p.Description == "" {
			t.Error("expected non-empty description")
		}
		if p.FilePath != path {
			t.Errorf("FilePath = %q, want %q", p.FilePath, path)
		}
	})

	t.Run("front matter utility is parsed and skipped in description", func(t *testing.T) {
		content := `---
utility: 0.92
---
# High Utility Pattern

Use this pattern first.
`
		path := filepath.Join(tmpDir, "high-utility.md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		p, err := parsePatternFile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Utility != 0.92 {
			t.Errorf("Utility = %f, want 0.92", p.Utility)
		}
		if p.Description == "utility: 0.92" {
			t.Error("front matter leaked into description")
		}
	})

	t.Run("no title uses filename", func(t *testing.T) {
		content := `Some description without a heading.
`
		path := filepath.Join(tmpDir, "no-title.md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		p, err := parsePatternFile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Name != "no-title" {
			t.Errorf("Name = %q, want %q", p.Name, "no-title")
		}
	})

	t.Run("empty file", func(t *testing.T) {
		path := filepath.Join(tmpDir, "empty.md")
		if err := os.WriteFile(path, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		p, err := parsePatternFile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Name != "empty" {
			t.Errorf("Name = %q, want %q", p.Name, "empty")
		}
		if p.Description != "" {
			t.Errorf("Description = %q, want empty", p.Description)
		}
	})

	t.Run("title with description below", func(t *testing.T) {
		content := `# My Pattern

The actual description starts here.
`
		path := filepath.Join(tmpDir, "titled.md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		p, err := parsePatternFile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Name != "My Pattern" {
			t.Errorf("Name = %q, want %q", p.Name, "My Pattern")
		}
		if p.Description == "" {
			t.Error("expected non-empty description")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		_, err := parsePatternFile(filepath.Join(tmpDir, "nope.md"))
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})
}

func TestCollectPatterns(t *testing.T) {
	tmpDir := t.TempDir()

	// Create patterns directory
	patternsDir := filepath.Join(tmpDir, ".agents", "patterns")
	if err := os.MkdirAll(patternsDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(patternsDir, "mutex.md"), []byte("# Mutex Pattern\n\nUse mutex for shared state."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(patternsDir, "pool.md"), []byte("# Connection Pooling\n\nPool database connections."), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("collects all patterns", func(t *testing.T) {
		got, err := collectPatterns(tmpDir, "", 10, "", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("got %d patterns, want 2", len(got))
		}
	})

	t.Run("filters by query", func(t *testing.T) {
		got, err := collectPatterns(tmpDir, "mutex", 10, "", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Errorf("got %d patterns for 'mutex', want 1", len(got))
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		got, err := collectPatterns(tmpDir, "", 1, "", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) > 1 {
			t.Errorf("got %d patterns, want at most 1", len(got))
		}
	})

	t.Run("no patterns directory", func(t *testing.T) {
		emptyDir := t.TempDir()
		got, err := collectPatterns(emptyDir, "", 10, "", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("ranks by utility when freshness is similar", func(t *testing.T) {
		highUtilityPath := filepath.Join(patternsDir, "utility-high.md")
		lowUtilityPath := filepath.Join(patternsDir, "utility-low.md")
		if err := os.WriteFile(highUtilityPath, []byte("---\nutility: 0.95\n---\n# High Utility\n\nHigh utility pattern."), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(lowUtilityPath, []byte("---\nutility: 0.20\n---\n# Low Utility\n\nLow utility pattern."), 0644); err != nil {
			t.Fatal(err)
		}

		got, err := collectPatterns(tmpDir, "utility", 10, "", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) < 2 {
			t.Fatalf("expected at least 2 utility patterns, got %d", len(got))
		}
		if got[0].Name != "High Utility" {
			t.Errorf("expected highest-ranked utility pattern first, got %q", got[0].Name)
		}
	})
}

func TestCollectPatterns_DeduplicatesGlobalLocalPath(t *testing.T) {
	tmpDir := t.TempDir()
	patternsDir := filepath.Join(tmpDir, ".agents", "patterns")
	if err := os.MkdirAll(patternsDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(patternsDir, "dedupe.md"),
		[]byte("# Dedupe Pattern\n\nUse one copy when local and global roots overlap."),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	got, err := collectPatterns(tmpDir, "dedupe", 10, patternsDir, 0.8)
	if err != nil {
		t.Fatalf("collectPatterns: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d patterns, want 1: %#v", len(got), got)
	}
	if got[0].Global {
		t.Fatalf("overlapping local/global pattern was marked global: %#v", got[0])
	}
	if got[0].Name != "Dedupe Pattern" {
		t.Fatalf("Name = %q, want Dedupe Pattern", got[0].Name)
	}
}
