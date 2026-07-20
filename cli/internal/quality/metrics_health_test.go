package quality

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCountConstraints_DualRoot: constraints live in BOTH the canonical
// .agents/ao/constraints (where doctor's split fixer migrates files) and the
// legacy .agents/constraints — CountConstraints must count both roots, exactly
// like every other section routed through KnowledgeSectionDirs. A constraint
// that exists ONLY under the canonical root must count.
func TestCountConstraints_DualRoot(t *testing.T) {
	base := t.TempDir()
	canonical := filepath.Join(base, ".agents", "ao", "constraints")
	if err := os.MkdirAll(canonical, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(canonical, "only-canonical.md"), []byte("# c\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := CountConstraints(base); got != 1 {
		t.Fatalf("CountConstraints = %d, want 1 (constraint only under canonical .agents/ao/constraints must count)", got)
	}

	// Legacy root still counts, and the two roots SUM.
	legacy := filepath.Join(base, ".agents", "constraints")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "legacy.yaml"), []byte("k: v\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := CountConstraints(base); got != 2 {
		t.Fatalf("CountConstraints = %d, want 2 (canonical + legacy)", got)
	}

	// index.json stays excluded in BOTH roots; other .json files count.
	if err := os.WriteFile(filepath.Join(canonical, "index.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "index.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(canonical, "real.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := CountConstraints(base); got != 3 {
		t.Fatalf("CountConstraints = %d, want 3 (index.json excluded in both roots)", got)
	}
}

// TestCountConstraints_MissingDirsCountZero: neither root existing is 0, not an error.
func TestCountConstraints_MissingDirsCountZero(t *testing.T) {
	if got := CountConstraints(t.TempDir()); got != 0 {
		t.Fatalf("CountConstraints = %d, want 0 for missing constraint dirs", got)
	}
}
