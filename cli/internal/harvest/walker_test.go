package harvest

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestDiscoverRigs_FindsNestedAgentsDirs(t *testing.T) {
	root := t.TempDir()

	// Create {root}/myproject/crew/alpha/.agents/learnings/
	agentsDir := filepath.Join(root, "myproject", "crew", "alpha", ".agents")
	mustMkdirAll(t, filepath.Join(agentsDir, "learnings"))
	mustMkdirAll(t, filepath.Join(agentsDir, "patterns"))
	mustWriteFile(t, filepath.Join(agentsDir, "AGENTS.md"), "# agents")

	opts := WalkOptions{
		Roots:       []string{root},
		MaxFileSize: 1048576,
		SkipDirs:    []string{"archive", ".tmp", "node_modules", "vendor"},
		IncludeDirs: []string{"learnings", "patterns", "research"},
	}

	rigs, err := DiscoverRigs(opts)
	if err != nil {
		t.Fatalf("DiscoverRigs returned error: %v", err)
	}

	// Filter out global hub if it exists on this machine.
	rigs = filterByRoot(rigs, root)

	if len(rigs) != 1 {
		t.Fatalf("expected 1 rig, got %d: %+v", len(rigs), rigs)
	}

	ri := rigs[0]
	if ri.Project != "myproject" {
		t.Errorf("Project = %q, want %q", ri.Project, "myproject")
	}
	if ri.Crew != "alpha" {
		t.Errorf("Crew = %q, want %q", ri.Crew, "alpha")
	}
	if ri.Rig != "myproject-alpha" {
		t.Errorf("Rig = %q, want %q", ri.Rig, "myproject-alpha")
	}
	if ri.Path != agentsDir {
		t.Errorf("Path = %q, want %q", ri.Path, agentsDir)
	}
	if ri.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1", ri.FileCount)
	}

	sort.Strings(ri.Subdirs)
	if len(ri.Subdirs) != 2 || ri.Subdirs[0] != "learnings" || ri.Subdirs[1] != "patterns" {
		t.Errorf("Subdirs = %v, want [learnings patterns]", ri.Subdirs)
	}
}

func TestDiscoverRigs_SkipsArchiveDirs(t *testing.T) {
	root := t.TempDir()

	// Create .agents/ inside an archive/ directory -- should be skipped.
	archiveAgents := filepath.Join(root, "myproject", "archive", "old", ".agents")
	mustMkdirAll(t, archiveAgents)
	mustWriteFile(t, filepath.Join(archiveAgents, "AGENTS.md"), "# old")

	// Create a valid .agents/ that should be found.
	validAgents := filepath.Join(root, "myproject", ".agents")
	mustMkdirAll(t, validAgents)
	mustWriteFile(t, filepath.Join(validAgents, "AGENTS.md"), "# valid")

	opts := WalkOptions{
		Roots:       []string{root},
		MaxFileSize: 1048576,
		SkipDirs:    []string{"archive", ".tmp", "node_modules", "vendor"},
		IncludeDirs: []string{"learnings"},
	}

	rigs, err := DiscoverRigs(opts)
	if err != nil {
		t.Fatalf("DiscoverRigs returned error: %v", err)
	}

	rigs = filterByRoot(rigs, root)

	if len(rigs) != 1 {
		t.Fatalf("expected 1 rig (archive skipped), got %d: %+v", len(rigs), rigs)
	}

	if rigs[0].Path != validAgents {
		t.Errorf("Path = %q, want %q", rigs[0].Path, validAgents)
	}
}

func TestDiscoverRigs_IncludesGlobalHub(t *testing.T) {
	// This test creates a fake home with .agents/ and verifies it's included.
	// We override HOME so the walker finds our temp dir.
	fakeHome := t.TempDir()
	globalAgents := filepath.Join(fakeHome, ".agents")
	mustMkdirAll(t, filepath.Join(globalAgents, "learnings"))
	mustMkdirAll(t, filepath.Join(globalAgents, "patterns"))

	origHome := os.Getenv("HOME")
	t.Setenv("HOME", fakeHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	// Use a nonexistent root so only the global hub is found.
	opts := WalkOptions{
		Roots:       []string{filepath.Join(fakeHome, "nonexistent-gt")},
		MaxFileSize: 1048576,
		SkipDirs:    []string{"archive"},
		IncludeDirs: []string{"learnings"},
	}

	rigs, err := DiscoverRigs(opts)
	if err != nil {
		t.Fatalf("DiscoverRigs returned error: %v", err)
	}

	if len(rigs) != 1 {
		t.Fatalf("expected 1 rig (global hub), got %d: %+v", len(rigs), rigs)
	}

	ri := rigs[0]
	if ri.Project != "global" {
		t.Errorf("Project = %q, want %q", ri.Project, "global")
	}
	if ri.Crew != "hub" {
		t.Errorf("Crew = %q, want %q", ri.Crew, "hub")
	}
	if ri.Rig != "global-hub" {
		t.Errorf("Rig = %q, want %q", ri.Rig, "global-hub")
	}
	if ri.Path != globalAgents {
		t.Errorf("Path = %q, want %q", ri.Path, globalAgents)
	}

	sort.Strings(ri.Subdirs)
	if len(ri.Subdirs) != 2 || ri.Subdirs[0] != "learnings" || ri.Subdirs[1] != "patterns" {
		t.Errorf("Subdirs = %v, want [learnings patterns]", ri.Subdirs)
	}
}

func TestDiscoverRigs_EmptyRoot(t *testing.T) {
	opts := WalkOptions{
		Roots:       []string{filepath.Join(t.TempDir(), "does-not-exist")},
		MaxFileSize: 1048576,
		SkipDirs:    []string{"archive"},
		IncludeDirs: []string{"learnings"},
	}

	// Override HOME to a dir without .agents/ so global hub doesn't appear.
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	rigs, err := DiscoverRigs(opts)
	if err != nil {
		t.Fatalf("expected no error for nonexistent root, got: %v", err)
	}

	if len(rigs) != 0 {
		t.Errorf("expected 0 rigs, got %d: %+v", len(rigs), rigs)
	}
}

func TestDiscoverRigs_MultipleProjects(t *testing.T) {
	root := t.TempDir()

	// Two projects with crew dirs.
	agents1 := filepath.Join(root, "alpha", "crew", "nami", ".agents")
	agents2 := filepath.Join(root, "beta", "crew", "zoro", ".agents")
	mustMkdirAll(t, agents1)
	mustMkdirAll(t, agents2)

	// One project without crew pattern.
	agents3 := filepath.Join(root, "gamma", ".agents")
	mustMkdirAll(t, agents3)

	opts := WalkOptions{
		Roots:       []string{root},
		MaxFileSize: 1048576,
		SkipDirs:    []string{"archive"},
		IncludeDirs: []string{"learnings"},
	}

	// Override HOME to avoid global hub interference.
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	rigs, err := DiscoverRigs(opts)
	if err != nil {
		t.Fatalf("DiscoverRigs returned error: %v", err)
	}

	if len(rigs) != 3 {
		t.Fatalf("expected 3 rigs, got %d: %+v", len(rigs), rigs)
	}

	// Build a map for easier assertion.
	byRig := make(map[string]RigInfo)
	for _, ri := range rigs {
		byRig[ri.Rig] = ri
	}

	cases := []struct {
		rig     string
		project string
		crew    string
	}{
		{"alpha-nami", "alpha", "nami"},
		{"beta-zoro", "beta", "zoro"},
		{"gamma-gamma", "gamma", "gamma"},
	}

	for _, tc := range cases {
		ri, ok := byRig[tc.rig]
		if !ok {
			t.Errorf("rig %q not found in results", tc.rig)
			continue
		}
		if ri.Project != tc.project {
			t.Errorf("rig %s: Project = %q, want %q", tc.rig, ri.Project, tc.project)
		}
		if ri.Crew != tc.crew {
			t.Errorf("rig %s: Crew = %q, want %q", tc.rig, ri.Crew, tc.crew)
		}
	}
}

// --- helpers ---

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

// filterByRoot returns only rigs whose Path is under the given root.
func filterByRoot(rigs []RigInfo, root string) []RigInfo {
	var out []RigInfo
	for _, ri := range rigs {
		if len(ri.Path) >= len(root) && ri.Path[:len(root)] == root {
			out = append(out, ri)
		}
	}
	return out
}
