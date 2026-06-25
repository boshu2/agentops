package wiki

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAtomicRewrite_DelegatesAtomicWrite is the direct regression guard for the
// storage.AtomicWriteFile delegation (age-uja6): it must overwrite the existing
// page's content exactly and land the 0644 page mode, regardless of the prior
// file's mode. A broken delegation (wrong perm, partial/!overwriting write)
// fails here even though FixWiki's higher-level tests stay green.
func TestAtomicRewrite_DelegatesAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "page.md")
	// Seed an existing page with longer, different content and a tighter mode so
	// the assertions below would catch a partial overwrite or a leaked mode.
	if err := os.WriteFile(path, []byte("stale, longer body that must be fully replaced\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	const want = "fresh page body\n"
	if err := atomicRewrite(path, want); err != nil {
		t.Fatalf("atomicRewrite: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != want {
		t.Fatalf("content = %q, want %q", got, want)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Fatalf("mode = %#o, want 0644 (wiki pages are world-readable)", perm)
	}
}
