package orchestration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProfilesContract_MergesTriVendor(t *testing.T) {
	root := findRepoRootForTest(t)
	profiles, err := LoadProfilesContract(root)
	if err != nil {
		t.Fatalf("LoadProfilesContract: %v", err)
	}
	tv, err := profiles.ProfileByID("tri-vendor")
	if err != nil {
		t.Fatalf("ProfileByID: %v", err)
	}
	if len(tv.Panes) != 3 {
		t.Fatalf("panes = %d, want 3", len(tv.Panes))
	}
	flat := tv.SpawnArgvFlat()
	want := []string{"--no-user", "--cc=1:opus", "--cod=1:gpt-5.5", "--agy=1"}
	if len(flat) < len(want) {
		t.Fatalf("spawn argv %v, want at least %v", flat, want)
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
