package goals

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveGoalsFile_ExplicitPath(t *testing.T) {
	if got := ResolveGoalsFile("/tmp/custom-goals.yaml"); got != "/tmp/custom-goals.yaml" {
		t.Errorf("ResolveGoalsFile(explicit) = %q, want /tmp/custom-goals.yaml", got)
	}
}

func TestResolveGoalsFile_PrefersGOALSmd(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "GOALS.md"), []byte("# Goals\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "GOALS.yaml"), []byte("version: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	if got := ResolveGoalsFile(""); got != "GOALS.md" {
		t.Errorf("ResolveGoalsFile(\"\") = %q, want GOALS.md (preferred over GOALS.yaml)", got)
	}
}

func TestResolveGoalsFile_FallsBackToYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "GOALS.yaml"), []byte("version: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	if got := ResolveGoalsFile(""); got != "GOALS.yaml" {
		t.Errorf("ResolveGoalsFile(\"\") = %q, want GOALS.yaml (fallback)", got)
	}
}

func TestResolveGoalsFile_DefaultsToGOALSmd(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if got := ResolveGoalsFile(""); got != "GOALS.md" {
		t.Errorf("ResolveGoalsFile(\"\") = %q, want GOALS.md (default for new projects)", got)
	}
}
