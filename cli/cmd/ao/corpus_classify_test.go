//go:build flywheel

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunCorpusClassify_DryRunReportsButDoesNotWrite drives the `ao corpus
// classify` command entry point in its default (dry-run) mode against a temp
// corpus and asserts it reports the right counts without touching disk.
func TestRunCorpusClassify_DryRunReportsButDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	orig := "---\ndate: 2026-06-14\n---\nbody\n"
	p := filepath.Join(dir, "a.md")
	if err := os.WriteFile(p, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}

	corpusClassifyApply = false
	out, err := captureStdout(t, func() error {
		return runCorpusClassify(corpusClassifyCmd, []string{dir})
	})
	if err != nil {
		t.Fatalf("runCorpusClassify: %v", err)
	}
	if !strings.Contains(out, "dry run") {
		t.Errorf("expected dry-run banner, got:\n%s", out)
	}
	if !strings.Contains(out, "needing defaults:  1") {
		t.Errorf("expected 1 record needing defaults, got:\n%s", out)
	}
	got, _ := os.ReadFile(p)
	if string(got) != orig {
		t.Errorf("dry run modified the file:\n%s", got)
	}
}

// TestRunCorpusClassify_ApplyWritesDefaults drives `ao corpus classify --apply`
// and asserts the safe defaults are written to the learning frontmatter.
func TestRunCorpusClassify_ApplyWritesDefaults(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.md")
	if err := os.WriteFile(p, []byte("---\ndate: 2026-06-14\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	corpusClassifyApply = true
	t.Cleanup(func() { corpusClassifyApply = false })
	if err := runCorpusClassify(corpusClassifyCmd, []string{dir}); err != nil {
		t.Fatalf("runCorpusClassify --apply: %v", err)
	}
	got, _ := os.ReadFile(p)
	if !strings.Contains(string(got), "sensitivity: unknown") || !strings.Contains(string(got), "publishable: false") {
		t.Errorf("apply did not write defaults:\n%s", got)
	}
}

// TestRunCorpusClassify_NonDirRejected asserts a non-directory argument is a
// clean error, not a panic.
func TestRunCorpusClassify_NonDirRejected(t *testing.T) {
	corpusClassifyApply = false
	err := runCorpusClassify(corpusClassifyCmd, []string{filepath.Join(t.TempDir(), "nope")})
	if err == nil {
		t.Fatal("expected error for a non-directory argument")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("unexpected error: %v", err)
	}
}
