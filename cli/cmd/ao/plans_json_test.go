// practices: [agent-ergonomics]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/types"
)

// withJSONOutput sets the global output format to json for the duration of a
// test and restores it afterward.
func withJSONOutput(t *testing.T) {
	t.Helper()
	prev := output
	output = "json"
	t.Cleanup(func() { output = prev })
}

func TestPlansList_JSONEmptyManifest(t *testing.T) {
	chdirTemp(t)
	withJSONOutput(t)

	out, err := captureStdout(t, func() error {
		return runPlansList(&cobra.Command{}, nil)
	})
	if err != nil {
		t.Fatalf("runPlansList: %v", err)
	}
	var entries []types.PlanManifestEntry
	if jerr := json.Unmarshal([]byte(out), &entries); jerr != nil {
		t.Fatalf("plans list --json on empty manifest is not valid JSON: %v\noutput: %q", jerr, out)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty array, got %d entries", len(entries))
	}
}

func TestPlansList_JSONWithEntries(t *testing.T) {
	dir := chdirTemp(t)
	if err := os.MkdirAll(filepath.Join(dir, ".agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(dir, "plan-a.md")
	if err := os.WriteFile(planPath, []byte("# Plan A"), 0o644); err != nil {
		t.Fatal(err)
	}

	prevName, prevProj, prevBeads, prevDry := planName, planProjectPath, planBeadsID, dryRun
	planName, planProjectPath, planBeadsID, dryRun = "plan-a", "/proj", "ol-1", false
	t.Cleanup(func() {
		planName, planProjectPath, planBeadsID, dryRun = prevName, prevProj, prevBeads, prevDry
	})
	if err := runPlansRegister(&cobra.Command{}, []string{planPath}); err != nil {
		t.Fatalf("runPlansRegister: %v", err)
	}

	withJSONOutput(t)
	out, err := captureStdout(t, func() error {
		return runPlansList(&cobra.Command{}, nil)
	})
	if err != nil {
		t.Fatalf("runPlansList: %v", err)
	}
	var entries []types.PlanManifestEntry
	if jerr := json.Unmarshal([]byte(out), &entries); jerr != nil {
		t.Fatalf("plans list --json is not valid JSON: %v\noutput: %q", jerr, out)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].PlanName != "plan-a" {
		t.Errorf("plan_name = %q, want plan-a", entries[0].PlanName)
	}
}

func TestPlansSearch_JSONEmptyManifest(t *testing.T) {
	chdirTemp(t)
	withJSONOutput(t)

	out, err := captureStdout(t, func() error {
		return runPlansSearch(&cobra.Command{}, []string{"anything"})
	})
	if err != nil {
		t.Fatalf("runPlansSearch: %v", err)
	}
	var entries []types.PlanManifestEntry
	if jerr := json.Unmarshal([]byte(out), &entries); jerr != nil {
		t.Fatalf("plans search --json is not valid JSON: %v\noutput: %q", jerr, out)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty array, got %d entries", len(entries))
	}
}

func TestPlansDiff_JSONEmptyManifest(t *testing.T) {
	chdirTemp(t)
	withJSONOutput(t)

	out, err := captureStdout(t, func() error {
		return runPlansDiff(&cobra.Command{}, nil)
	})
	if err != nil {
		t.Fatalf("runPlansDiff: %v", err)
	}
	var drifts []driftEntry
	if jerr := json.Unmarshal([]byte(out), &drifts); jerr != nil {
		t.Fatalf("plans diff --json is not valid JSON: %v\noutput: %q", jerr, out)
	}
	if len(drifts) != 0 {
		t.Errorf("expected empty array, got %d drifts", len(drifts))
	}
}
