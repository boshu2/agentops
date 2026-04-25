package plans

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFindAgentsDir_SkipsOsTempDir guards against the 2026-04-25
// regression where a stale /tmp/.agents/ from a prior test run was
// returned by FindAgentsDir for any startDir under /tmp. Operators
// running ao under the OS tempdir would inherit cruft state instead
// of getting the "no rig found" empty-string result.
func TestFindAgentsDir_SkipsOsTempDir(t *testing.T) {
	// Simulate the polluted-tempdir scenario by overriding TMPDIR to
	// a fresh dir, planting a .agents/ inside it, and pointing the
	// startDir at a sibling. FindAgentsDir must skip the planted
	// .agents/ at the (now overridden) os.TempDir() boundary.
	fakeTmp := t.TempDir()
	t.Setenv("TMPDIR", fakeTmp)

	if err := os.MkdirAll(filepath.Join(fakeTmp, ".agents"), 0o755); err != nil {
		t.Fatalf("seeding planted .agents: %v", err)
	}

	startDir := filepath.Join(fakeTmp, "child", "leaf")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatalf("creating startDir: %v", err)
	}

	got := FindAgentsDir(startDir)
	if got != "" {
		t.Errorf("FindAgentsDir(%q) = %q; want empty (must not return planted %s/.agents)",
			startDir, got, fakeTmp)
	}
}

// TestFindAgentsDir_ReturnsRealAncestor confirms the skip-tempdir
// fix doesn't break the happy path: a real .agents/ ancestor below
// the tempdir boundary is still returned.
func TestFindAgentsDir_ReturnsRealAncestor(t *testing.T) {
	fakeTmp := t.TempDir()
	t.Setenv("TMPDIR", fakeTmp)

	rigRoot := filepath.Join(fakeTmp, "myrig")
	rigAgents := filepath.Join(rigRoot, ".agents")
	if err := os.MkdirAll(rigAgents, 0o755); err != nil {
		t.Fatal(err)
	}
	startDir := filepath.Join(rigRoot, "deep", "nested", "leaf")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatal(err)
	}

	got := FindAgentsDir(startDir)
	if got != rigAgents {
		t.Errorf("FindAgentsDir(%q) = %q; want %q", startDir, got, rigAgents)
	}
}

// TestFindAgentsDir_ReturnsRigMarkerAncestor verifies the rig-marker
// fast path (.beads / crew / polecats) still works under the
// skip-tempdir rule.
func TestFindAgentsDir_ReturnsRigMarkerAncestor(t *testing.T) {
	fakeTmp := t.TempDir()
	t.Setenv("TMPDIR", fakeTmp)

	rigRoot := filepath.Join(fakeTmp, "myrig")
	if err := os.MkdirAll(filepath.Join(rigRoot, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	startDir := filepath.Join(rigRoot, "child")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatal(err)
	}

	got := FindAgentsDir(startDir)
	want := filepath.Join(rigRoot, ".agents")
	if got != want {
		t.Errorf("FindAgentsDir(%q) = %q; want %q", startDir, got, want)
	}
}
