package goals

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathResolverPreservesLegacySelection(t *testing.T) {
	dir := t.TempDir()
	resolver := PathResolver{Root: dir}
	if got := resolver.Resolve("custom.yaml"); got != "custom.yaml" {
		t.Fatalf("explicit path = %q", got)
	}
	if got := resolver.Resolve(""); got != "GOALS.md" {
		t.Fatalf("empty directory fallback = %q", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "GOALS.yaml"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := resolver.Resolve(""); got != "GOALS.yaml" {
		t.Fatalf("yaml fallback = %q", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "GOALS.md"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := resolver.Resolve(""); got != "GOALS.md" {
		t.Fatalf("markdown precedence = %q", got)
	}
}

func TestPathResolverProjectRootPreservesCWD(t *testing.T) {
	dir := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	want, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if got := (PathResolver{}).ProjectRoot(); got != want {
		t.Fatalf("ProjectRoot() = %q, want cwd %q", got, want)
	}
}
