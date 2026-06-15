package corpus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestClassifyDir is the L2 integration test: it builds a temp corpus with the
// real on-disk shapes (fenced, no-fence, already-classified, a meta doc, a
// malformed record) and drives the whole walk → annotate → write path.
func TestClassifyDir(t *testing.T) {
	root := t.TempDir()
	mustWrite := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mustWrite("a-fenced.md", "---\ndate: 2026-06-14\n---\nbody\n")
	mustWrite("b-nofence.md", "# heading\nbody\n")
	mustWrite("c-classified.md", "---\nsensitivity: public\npublishable: true\n---\nbody\n")
	mustWrite("CORPUS-POLICY.md", "# policy\n")            // meta — skipped
	mustWrite("nested/d-malformed.md", "---\nbroken: [\n") // no close fence
	mustWrite("notmarkdown.txt", "ignored\n")

	// Dry run: report only, no writes.
	dry, err := ClassifyDir(root, false)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if dry.Applied {
		t.Error("dry run reported Applied=true")
	}
	if dry.Scanned != 4 { // a, b, c, d — not the .txt, not the meta
		t.Errorf("Scanned = %d, want 4", dry.Scanned)
	}
	if dry.Skipped != 1 { // CORPUS-POLICY.md
		t.Errorf("Skipped = %d, want 1", dry.Skipped)
	}
	if dry.Changed != 3 { // a, b, d need annotation; c already classified
		t.Errorf("Changed = %d, want 3 (files: %v)", dry.Changed, dry.ChangedFiles)
	}
	// Dry run must not have touched disk.
	if got, _ := os.ReadFile(filepath.Join(root, "a-fenced.md")); string(got) != "---\ndate: 2026-06-14\n---\nbody\n" {
		t.Error("dry run wrote to a-fenced.md")
	}

	// Apply: writes the changes.
	app, err := ClassifyDir(root, true)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if app.Changed != 3 {
		t.Errorf("apply Changed = %d, want 3", app.Changed)
	}
	got, _ := os.ReadFile(filepath.Join(root, "a-fenced.md"))
	if !strings.Contains(string(got), "sensitivity: unknown") || !strings.Contains(string(got), "publishable: false") {
		t.Errorf("a-fenced.md not annotated after apply:\n%s", got)
	}
	// The meta doc is untouched.
	meta, _ := os.ReadFile(filepath.Join(root, "CORPUS-POLICY.md"))
	if string(meta) != "# policy\n" {
		t.Errorf("CORPUS-POLICY.md was modified: %q", meta)
	}

	// Re-run after apply is a clean no-op (idempotent migration).
	again, err := ClassifyDir(root, true)
	if err != nil {
		t.Fatalf("rerun: %v", err)
	}
	if again.Changed != 0 {
		t.Errorf("rerun Changed = %d, want 0 (not idempotent): %v", again.Changed, again.ChangedFiles)
	}
}
