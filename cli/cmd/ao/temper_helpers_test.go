// practices: [llm-eval-harness, ai-assisted-dev]
package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/types"
)

func TestParseMarkdownMetadata(t *testing.T) {
	t.Run("full metadata", func(t *testing.T) {
		content := `# Learning

**ID**: L42
**Maturity**: established
**Utility**: 0.85
**Confidence**: 0.92
**Schema Version**: 2
**Status**: tempered

Content here.
`
		var meta artifactMetadata
		parseMarkdownMetadata(content, &meta)

		if meta.ID != "L42" {
			t.Errorf("ID = %q, want %q", meta.ID, "L42")
		}
		if meta.Maturity != types.MaturityEstablished {
			t.Errorf("Maturity = %q, want %q", meta.Maturity, types.MaturityEstablished)
		}
		if meta.Utility < 0.84 || meta.Utility > 0.86 {
			t.Errorf("Utility = %f, want ~0.85", meta.Utility)
		}
		if meta.Confidence < 0.91 || meta.Confidence > 0.93 {
			t.Errorf("Confidence = %f, want ~0.92", meta.Confidence)
		}
		if meta.SchemaVersion != 2 {
			t.Errorf("SchemaVersion = %d, want 2", meta.SchemaVersion)
		}
		if !meta.Tempered {
			t.Error("expected Tempered = true")
		}
	})

	t.Run("no metadata", func(t *testing.T) {
		content := "# Just a title\n\nSome content without metadata."
		var meta artifactMetadata
		parseMarkdownMetadata(content, &meta)

		if meta.ID != "" {
			t.Errorf("ID = %q, want empty", meta.ID)
		}
	})

	t.Run("locked status", func(t *testing.T) {
		content := "**Status**: locked"
		var meta artifactMetadata
		parseMarkdownMetadata(content, &meta)

		if !meta.Tempered {
			t.Error("expected Tempered = true for locked status")
		}
	})
}

func TestParseFrontmatterMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "frontmatter.md")
	content := `---
id: learning-frontmatter
maturity: candidate
utility: 0.72
confidence: 0.83
reward_count: 4
schema_version: 1
status: locked
---
# Learning
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	meta := &artifactMetadata{
		Maturity:   types.MaturityProvisional,
		Utility:    0.5,
		Confidence: 0.5,
	}
	parseFrontmatterMetadata(path, meta)

	if meta.ID != "learning-frontmatter" {
		t.Errorf("ID = %q, want %q", meta.ID, "learning-frontmatter")
	}
	if meta.Maturity != types.MaturityCandidate {
		t.Errorf("Maturity = %q, want %q", meta.Maturity, types.MaturityCandidate)
	}
	if meta.Utility < 0.71 || meta.Utility > 0.73 {
		t.Errorf("Utility = %f, want ~0.72", meta.Utility)
	}
	if meta.Confidence < 0.82 || meta.Confidence > 0.84 {
		t.Errorf("Confidence = %f, want ~0.83", meta.Confidence)
	}
	if meta.FeedbackCount != 4 {
		t.Errorf("FeedbackCount = %d, want 4", meta.FeedbackCount)
	}
	if meta.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", meta.SchemaVersion)
	}
	if !meta.Tempered {
		t.Error("expected Tempered = true")
	}
}

func TestExpandDirectoryFlat(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "a.md"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "b.jsonl"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "c.txt"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "subdir", "d.md"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	got := expandDirectoryFlat(tmpDir)
	if len(got) != 2 {
		t.Errorf("expandDirectoryFlat() returned %d files, want 2 (a.md, b.jsonl); got %v", len(got), got)
	}
}

func TestExpandDirectoryRecursive(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "a.md"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "sub", "b.jsonl"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "sub", "c.txt"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := expandDirectoryRecursive(tmpDir, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expandDirectoryRecursive() returned %d files, want 2; got %v", len(got), got)
	}
}

func TestParseJSONLMetadata(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("valid JSONL", func(t *testing.T) {
		content := `{"id":"L42","maturity":"established","utility":0.85,"confidence":0.9,"reward_count":5}`
		path := filepath.Join(tmpDir, "valid.jsonl")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		meta := &artifactMetadata{}
		err := parseJSONLMetadata(path, meta)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if meta.ID != "L42" {
			t.Errorf("ID = %q, want %q", meta.ID, "L42")
		}
		if meta.Maturity != types.MaturityEstablished {
			t.Errorf("Maturity = %q, want %q", meta.Maturity, types.MaturityEstablished)
		}
		if meta.FeedbackCount != 5 {
			t.Errorf("FeedbackCount = %d, want 5", meta.FeedbackCount)
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		meta := &artifactMetadata{}
		err := parseJSONLMetadata(filepath.Join(tmpDir, "nope.jsonl"), meta)
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("empty file", func(t *testing.T) {
		path := filepath.Join(tmpDir, "empty.jsonl")
		if err := os.WriteFile(path, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		meta := &artifactMetadata{}
		err := parseJSONLMetadata(path, meta)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if meta.ID != "" {
			t.Errorf("ID = %q, want empty", meta.ID)
		}
	})
}
