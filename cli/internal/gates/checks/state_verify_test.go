package checks

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/gates"
	"github.com/boshu2/agentops/cli/internal/ports"
)

func TestStateVerifyCheckRegistered(t *testing.T) {
	check, ok := gates.Default.Get("state.verify")
	if !ok {
		t.Fatal("state.verify gate is not registered")
	}
	if !check.Tiers.Has(gates.Fast) || !check.Tiers.Has(gates.Full) {
		t.Fatalf("state.verify tiers = %v, want Fast|Full", check.Tiers)
	}
	if !check.Blocking {
		t.Fatal("state.verify must be blocking")
	}
	if !check.AlwaysRun() {
		t.Fatal("state.verify should always run in gate check")
	}
	if check.Run == nil {
		t.Fatal("state.verify should be native Go-backed")
	}
}

func TestRunStateVerifyGatePassesCheckedInContracts(t *testing.T) {
	verdict, err := runStateVerifyGate(context.Background(), gates.RunContext{RepoRoot: repoRootForChecksTest(t)})
	if err != nil {
		t.Fatalf("runStateVerifyGate: %v", err)
	}
	if verdict.Status != ports.GateStatusPass {
		t.Fatalf("Status = %s, want PASS; reason=%s log=%s", verdict.Status, verdict.Reason, verdict.LogTail)
	}
}

func repoRootForChecksTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "schemas")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repo root not found")
		}
		dir = parent
	}
}
