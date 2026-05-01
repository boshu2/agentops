package llmwiki

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckWriteScope_AllowsListedPaths(t *testing.T) {
	vault := t.TempDir()
	cases := []struct {
		name string
		rel  string
	}{
		{"sources file", "wiki/sources/foo.md"},
		{"sources nested", "wiki/sources/2026/05/foo.md"},
		{"entities file", "wiki/entities/bar.md"},
		{"concepts file", "wiki/concepts/baz.md"},
		{"synthesis file", "wiki/synthesis/2026-05-01-lint.md"},
		{"INDEX.md", "wiki/INDEX.md"},
		{"LOG.md", "wiki/LOG.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			abs := filepath.Join(vault, tc.rel)
			if err := CheckWriteScope(vault, abs); err != nil {
				t.Fatalf("CheckWriteScope(%s) returned error: %v", tc.rel, err)
			}
		})
	}
}

func TestStages_WriteScopeGuardRejectsAgentsPath(t *testing.T) {
	vault := t.TempDir()
	abs := filepath.Join(vault, ".agents", "foo.md")
	err := CheckWriteScope(vault, abs)
	if err == nil {
		t.Fatal("CheckWriteScope(.agents/foo.md) returned nil; want WriteScopeError")
	}
	var scopeErr *WriteScopeError
	if !errors.As(err, &scopeErr) {
		t.Fatalf("expected *WriteScopeError, got %T: %v", err, err)
	}
	if scopeErr.Path != abs {
		t.Errorf("scopeErr.Path = %q, want %q", scopeErr.Path, abs)
	}
}

func TestCheckWriteScope_RejectsAbsoluteOutsideVault(t *testing.T) {
	vault := t.TempDir()
	cases := []string{
		"/etc/passwd",
		filepath.Join(filepath.Dir(vault), "sibling.md"),
	}
	for _, abs := range cases {
		t.Run(abs, func(t *testing.T) {
			err := CheckWriteScope(vault, abs)
			if err == nil {
				t.Fatalf("CheckWriteScope(%s) returned nil; want error", abs)
			}
			var scopeErr *WriteScopeError
			if !errors.As(err, &scopeErr) {
				t.Fatalf("expected *WriteScopeError, got %T", err)
			}
		})
	}
}

func TestCheckWriteScope_RejectsWikiRoot(t *testing.T) {
	vault := t.TempDir()
	// wiki/ is a parent of allowed prefixes but is NOT itself allowed (only
	// the listed subdirs and the two named files are).
	err := CheckWriteScope(vault, filepath.Join(vault, "wiki", "rogue.md"))
	if err == nil {
		t.Fatal("CheckWriteScope(wiki/rogue.md) returned nil; want WriteScopeError")
	}
}

func TestCheckWriteScope_EmptyArgs(t *testing.T) {
	if err := CheckWriteScope("", "/tmp/foo"); err == nil {
		t.Error("expected error for empty vault")
	}
	if err := CheckWriteScope("/tmp", ""); err == nil {
		t.Error("expected error for empty path")
	}
}

func TestSafeAtomicWrite_ScopeAndAtomicity(t *testing.T) {
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "wiki", "sources"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Write to allowed path → succeeds.
	allowed := filepath.Join(vault, "wiki", "sources", "ok.md")
	if err := SafeAtomicWrite(vault, allowed, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("SafeAtomicWrite(allowed) failed: %v", err)
	}
	got, err := os.ReadFile(allowed)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(got) != "hello\n" {
		t.Errorf("file contents = %q, want %q", got, "hello\n")
	}

	// Write to disallowed path → WriteScopeError + no file created.
	bad := filepath.Join(vault, ".agents", "secret.md")
	if err := os.MkdirAll(filepath.Dir(bad), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	err = SafeAtomicWrite(vault, bad, []byte("nope\n"), 0o644)
	if err == nil {
		t.Fatal("SafeAtomicWrite(disallowed) returned nil; want WriteScopeError")
	}
	var scopeErr *WriteScopeError
	if !errors.As(err, &scopeErr) {
		t.Fatalf("expected *WriteScopeError, got %T: %v", err, err)
	}
	if _, statErr := os.Stat(bad); !os.IsNotExist(statErr) {
		t.Errorf("disallowed path was created despite scope error: stat err=%v", statErr)
	}
}
