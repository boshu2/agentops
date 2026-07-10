package orchestration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProfilesContract_DualHasTwoRoleShapedPanes(t *testing.T) {
	root := findRepoRootForTest(t)
	profiles, err := LoadProfilesContract(root)
	if err != nil {
		t.Fatalf("LoadProfilesContract: %v", err)
	}
	dual, err := profiles.ProfileByID("dual-pane")
	if err != nil {
		t.Fatalf("ProfileByID: %v", err)
	}
	if len(dual.Panes) != 2 {
		t.Fatalf("panes = %d, want 2 role-shaped lanes", len(dual.Panes))
	}
	if dual.OwnerSkill != "agent-native" {
		t.Fatalf("owner_skill = %q, want agent-native", dual.OwnerSkill)
	}
	if dual.Panes[0].Role != "Orchestrator" || dual.Panes[1].Role != "Worker" {
		t.Fatalf("dual roles = %+v, want Orchestrator + Worker", dual.Panes)
	}
	assertProfileTokens(t, dual, []string{
		"--robot-spawn=${session}",
		"--spawn-cod=2",
		"--spawn-no-user",
		"--spawn-wait",
		"--spawn-dir=${worktree}",
	})
}

func TestLoadProfilesContract_TriHasThreeVendorPanes(t *testing.T) {
	root := findRepoRootForTest(t)
	profiles, err := LoadProfilesContract(root)
	if err != nil {
		t.Fatalf("LoadProfilesContract: %v", err)
	}
	tri, err := profiles.ProfileByID("tri-vendor")
	if err != nil {
		t.Fatalf("ProfileByID: %v", err)
	}
	if len(tri.Panes) != 3 {
		t.Fatalf("panes = %d, want 3 vendor lanes", len(tri.Panes))
	}
	if tri.Panes[0].Runtime != "claude" || tri.Panes[1].Runtime != "codex" || tri.Panes[2].Runtime != "agy" {
		t.Fatalf("tri runtimes = %+v, want claude + codex + agy", tri.Panes)
	}
	if tri.Panes[2].Role != "Verifier" || !tri.Panes[2].ReadOnly {
		t.Fatalf("third pane = %+v, want read-only Verifier", tri.Panes[2])
	}
	assertProfileTokens(t, tri, []string{
		"--robot-spawn=${session}",
		"--spawn-cod=1",
		"--spawn-no-user",
		"--spawn-wait",
		"--spawn-dir=${worktree}",
		"--spawn-cc=1",
		"--spawn-agy=1",
	})
}

func assertProfileTokens(t *testing.T, profile ProfileSpec, want []string) {
	t.Helper()
	flat := profile.SpawnArgvFlat()
	seen := make(map[string]bool, len(flat))
	for _, token := range flat {
		seen[token] = true
	}
	for _, token := range want {
		if !seen[token] {
			t.Fatalf("spawn argv %v missing %q", flat, token)
		}
	}
}

func TestVersionMeetsFloor(t *testing.T) {
	if !VersionMeetsFloor("1.2.3", "0.1.0") {
		t.Fatal("expected 1.2.3 >= 0.1.0")
	}
	if VersionMeetsFloor("0.0.1", "1.0.0") {
		t.Fatal("expected 0.0.1 < 1.0.0")
	}
}

func TestRunPreflight_AMDegradedWarns(t *testing.T) {
	root := findRepoRootForTest(t)
	seq := &preflightTestRunner{}
	result, err := RunPreflight(context.Background(), PreflightOptions{
		RepoRoot: root,
		Profile:  "tri-vendor",
		RunID:    "test-run",
		Runner:   seq,
	})
	if err != nil {
		t.Fatalf("RunPreflight: %v", err)
	}
	if !result.CoordinationDegraded {
		t.Fatal("expected coordination_degraded when am down")
	}
	if result.Verdict.Status == VerdictStatusFail {
		t.Fatalf("unexpected FAIL when only am degraded: %+v", result.Checks)
	}
}

var errAMDown = errors.New("am down")

type preflightTestRunner struct{}

func (preflightTestRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	if name == "am" {
		return nil, errAMDown
	}
	if name == "ntm" {
		return []byte(`{"capabilities":["tmux","git","persistent-host","agent-CLIs"]}`), nil
	}
	return []byte("1.2.0"), nil
}

func findRepoRootForTest(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, ToolsContractRelPath)); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("repo root not found")
		}
		wd = parent
	}
}
