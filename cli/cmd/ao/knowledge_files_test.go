package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWalkKnowledgeFiles_FiltersByExtensionAndSorts(t *testing.T) {
	dir := t.TempDir()

	// Build a nested layout to exercise recursion.
	mustWriteFile(t, filepath.Join(dir, "b.md"), "b")
	mustWriteFile(t, filepath.Join(dir, "a.md"), "a")
	mustWriteFile(t, filepath.Join(dir, "nested", "c.md"), "c")
	mustWriteFile(t, filepath.Join(dir, "nested", "skip.txt"), "no")
	mustWriteFile(t, filepath.Join(dir, "nested", "deeper", "d.MD"), "d") // case-insensitive
	mustWriteFile(t, filepath.Join(dir, "ignored.json"), "{}")

	got := walkKnowledgeFiles(dir, ".md")
	want := []string{
		filepath.Join(dir, "a.md"),
		filepath.Join(dir, "b.md"),
		filepath.Join(dir, "nested", "c.md"),
		filepath.Join(dir, "nested", "deeper", "d.MD"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("walkKnowledgeFiles md = %v\nwant %v", got, want)
	}
}

func TestWalkKnowledgeFiles_MultipleExtensions(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "a.md"), "x")
	mustWriteFile(t, filepath.Join(dir, "b.json"), "{}")
	mustWriteFile(t, filepath.Join(dir, "c.yaml"), "y")

	got := walkKnowledgeFiles(dir, ".md", ".json")
	want := []string{
		filepath.Join(dir, "a.md"),
		filepath.Join(dir, "b.json"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("walkKnowledgeFiles md+json = %v, want %v", got, want)
	}
}

func TestWalkKnowledgeFiles_EmptyDirReturnsNil(t *testing.T) {
	dir := t.TempDir()
	got := walkKnowledgeFiles(dir, ".md")
	if got != nil {
		t.Fatalf("walkKnowledgeFiles on empty dir = %v, want nil", got)
	}
}

func TestWalkKnowledgeFiles_MissingDirReturnsNil(t *testing.T) {
	got := walkKnowledgeFiles("/definitely/not/here/xyz123", ".md")
	if got != nil {
		t.Fatalf("walkKnowledgeFiles on missing dir = %v, want nil", got)
	}
}

func TestWalkKnowledgeFiles_EmptyDirArgReturnsNil(t *testing.T) {
	got := walkKnowledgeFiles("", ".md")
	if got != nil {
		t.Fatalf("walkKnowledgeFiles empty dir = %v, want nil", got)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
