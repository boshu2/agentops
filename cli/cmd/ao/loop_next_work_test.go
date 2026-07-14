//go:build legacy

// practices: [dora-metrics, lean-startup]
package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/evolve/ladder"
	"github.com/spf13/cobra"
)

// fakeBeadRunner is a test double implementing ladder.BeadRunner.
type fakeBeadRunner struct {
	ReadyList      []ladder.Bead
	ReadyByTypeMap map[string][]ladder.Bead
	ShowMap        map[string]ladder.Bead
	InProgressList []ladder.Bead
}

func (f *fakeBeadRunner) Ready(_ context.Context) ([]ladder.Bead, error) {
	return f.ReadyList, nil
}

func (f *fakeBeadRunner) ReadyByType(_ context.Context, t string) ([]ladder.Bead, error) {
	return f.ReadyByTypeMap[t], nil
}

func (f *fakeBeadRunner) Show(_ context.Context, id string) (ladder.Bead, error) {
	b, ok := f.ShowMap[id]
	if !ok {
		return ladder.Bead{}, errors.New("not found")
	}
	return b, nil
}

func (f *fakeBeadRunner) InProgress(_ context.Context) ([]ladder.Bead, error) {
	return f.InProgressList, nil
}

// fakeGrep mocks the grep enrichment so tests stay hermetic.
type fakeGrep struct{}

func (fakeGrep) Grep(_ context.Context, _ string, _ []string) ([]string, error) {
	return nil, nil
}

// withFakeNextWorkRunners installs the supplied fakes for the duration of the
// test and restores production runners on cleanup.
func withFakeNextWorkRunners(t *testing.T, br ladder.BeadRunner, gr ladder.GrepRunner) {
	t.Helper()
	prevBR, prevGR := evolveNextWorkRunnerOverride, evolveNextWorkGrepOverride
	evolveNextWorkRunnerOverride = br
	evolveNextWorkGrepOverride = gr
	t.Cleanup(func() {
		evolveNextWorkRunnerOverride = prevBR
		evolveNextWorkGrepOverride = prevGR
	})
}

func TestLoopNextWorkUsesResolvedTrackerContext(t *testing.T) {
	root := t.TempDir()
	chdirTo(t, root)
	binDir := t.TempDir()
	tracePath := filepath.Join(t.TempDir(), "tracker.trace")
	script := `#!/bin/sh
printf 'binary=%s|pwd=%s|beads=%s|argv=%s\n' "${0##*/}" "$(pwd -P)" "${BEADS_DIR-<unset>}" "$*" >> "$TRACKER_TRACE"
printf '[{"id":"ag-1","title":"small change","description":"change cli/x.go acceptance: then","issue_type":"task","status":"ready"}]\n'
`
	for _, tracker := range []string{"br", "bd"} {
		if err := os.WriteFile(filepath.Join(binDir, tracker), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	ambientLedger := filepath.Join(t.TempDir(), "ambient-ledger")
	t.Setenv("AGENTOPS_TRACKER", "br")
	t.Setenv("BEADS_DIR", ambientLedger)
	t.Setenv("TRACKER_TRACE", tracePath)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+"/usr/bin"+string(os.PathListSeparator)+"/bin")
	oldLookPath := trackerLookPath
	trackerLookPath = exec.LookPath
	t.Cleanup(func() { trackerLookPath = oldLookPath })

	oldMode := evolveNextWorkMode
	oldInclude := evolveNextWorkIncludeOperator
	oldJSON := evolveNextWorkJSON
	oldBinary := evolveNextWorkBDBinary
	oldRunner := evolveNextWorkRunnerOverride
	oldGrep := evolveNextWorkGrepOverride
	oldClock := evolveNextWorkClock
	evolveNextWorkMode = loopModeBurst
	evolveNextWorkIncludeOperator = false
	evolveNextWorkJSON = true
	evolveNextWorkBDBinary = ""
	evolveNextWorkRunnerOverride = nil
	evolveNextWorkGrepOverride = fakeGrep{}
	evolveNextWorkClock = func() time.Time { return time.Unix(1_700_000_000, 0).UTC() }
	t.Cleanup(func() {
		evolveNextWorkMode = oldMode
		evolveNextWorkIncludeOperator = oldInclude
		evolveNextWorkJSON = oldJSON
		evolveNextWorkBDBinary = oldBinary
		evolveNextWorkRunnerOverride = oldRunner
		evolveNextWorkGrepOverride = oldGrep
		evolveNextWorkClock = oldClock
	})

	run := func(ctx context.Context) error {
		cmd := &cobra.Command{}
		cmd.SetContext(ctx)
		var output strings.Builder
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		return runLoopNextWork(cmd, nil)
	}
	physicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}

	if err := run(context.Background()); err != nil {
		t.Fatalf("run with selected BR: %v", err)
	}
	data, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("read BR trace: %v", err)
	}
	wantBR := "binary=br|pwd=" + physicalRoot + "|beads=" + ambientLedger + "|argv=ready --json"
	if got := strings.TrimSpace(string(data)); got != wantBR {
		t.Fatalf("BR trace = %q, want %q", got, wantBR)
	}

	if err := os.Remove(tracePath); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-canceled run error = %T %v, want context cancellation", err, err)
	}
	if _, err := os.Stat(tracePath); !os.IsNotExist(err) {
		t.Fatalf("pre-canceled run launched tracker: %v", err)
	}

	evolveNextWorkBDBinary = filepath.Join(binDir, "bd")
	if err := run(context.Background()); err != nil {
		t.Fatalf("run with --bd-binary override: %v", err)
	}
	data, err = os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("read BD trace: %v", err)
	}
	wantBD := "binary=bd|pwd=" + physicalRoot + "|beads=<unset>|argv=ready --json"
	if got := strings.TrimSpace(string(data)); got != wantBD {
		t.Fatalf("BD override trace = %q, want %q", got, wantBD)
	}
}

// withFixedNextWorkClock pins the timestamp clock.
func withFixedNextWorkClock(t *testing.T, ts time.Time) {
	t.Helper()
	prev := evolveNextWorkClock
	evolveNextWorkClock = func() time.Time { return ts }
	t.Cleanup(func() { evolveNextWorkClock = prev })
}

// TestLoopNextWork_Step1Pick exercises the happy path: shape-compatible
// bead picked at step 1 with JSON output.
func TestLoopNextWork_Step1Pick(t *testing.T) {
	dir := chdirTemp(t)
	withFixedNextWorkClock(t, time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC))
	withFakeNextWorkRunners(t, &fakeBeadRunner{
		ReadyList: []ladder.Bead{
			{
				ID:          "soc-x",
				Title:       "implement next-work",
				Description: "Edit cli/foo.go. ## Scenarios when X then Y. Follows soc-prev.",
			},
			{ID: "soc-alt-a"},
			{ID: "soc-alt-b"},
		},
	}, fakeGrep{})

	out, err := executeCommand("loop", "next-work", "--json")
	if err != nil {
		t.Fatalf("err: %v\nout=%s", err, out)
	}
	start := strings.Index(out, "{")
	if start < 0 {
		t.Fatalf("no JSON in output: %q", out)
	}
	var rec ladder.Recommendation
	if err := json.Unmarshal([]byte(out[start:]), &rec); err != nil {
		t.Fatalf("decode: %v\nout=%s", err, out)
	}
	if rec.RecommendedBead != "soc-x" {
		t.Errorf("bead = %q, want soc-x", rec.RecommendedBead)
	}
	if rec.LadderStepMatched != 1 {
		t.Errorf("step = %d, want 1", rec.LadderStepMatched)
	}

	// Decision log should have one row.
	logPath := filepath.Join(dir, evolveNextWorkLogRel)
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("log rows = %d, want 1", len(lines))
	}
	var row nextWorkDecisionLogRow
	if err := json.Unmarshal([]byte(lines[0]), &row); err != nil {
		t.Fatalf("decode row: %v", err)
	}
	if row.RecommendedBead != "soc-x" || row.LadderStepMatched != 1 {
		t.Errorf("log row = %+v", row)
	}
}

// TestLoopNextWork_PrimitiveTestFailsRecommendsScout exercises the step-3
// scout-mode rationale.
func TestLoopNextWork_PrimitiveTestFailsRecommendsScout(t *testing.T) {
	chdirTemp(t)
	withFakeNextWorkRunners(t, &fakeBeadRunner{
		ReadyList: []ladder.Bead{
			{ID: "soc-vague", Title: "vague", Description: "make better"},
		},
	}, fakeGrep{})

	out, err := executeCommand("loop", "next-work", "--json")
	if err != nil {
		t.Fatalf("err: %v\nout=%s", err, out)
	}
	start := strings.Index(out, "{")
	if start < 0 {
		t.Fatalf("no JSON in output: %q", out)
	}
	var rec ladder.Recommendation
	if err := json.Unmarshal([]byte(out[start:]), &rec); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rec.LadderStepMatched != 3 {
		t.Errorf("step = %d, want 3", rec.LadderStepMatched)
	}
	if !strings.Contains(rec.Rationale, "scout-mode") {
		t.Errorf("rationale: %q", rec.Rationale)
	}
}

// TestLoopNextWork_LadderExhaustionEmitsBlockedHint covers the terminal
// "ladder exhausted" recommendation.
func TestLoopNextWork_LadderExhaustionEmitsBlockedHint(t *testing.T) {
	chdirTemp(t)
	withFakeNextWorkRunners(t, &fakeBeadRunner{}, fakeGrep{})

	out, err := executeCommand("loop", "next-work", "--json")
	if err != nil {
		t.Fatalf("err: %v\nout=%s", err, out)
	}
	start := strings.Index(out, "{")
	if start < 0 {
		t.Fatalf("no JSON in output: %q", out)
	}
	var rec ladder.Recommendation
	if err := json.Unmarshal([]byte(out[start:]), &rec); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rec.RecommendedBead != "" {
		t.Errorf("bead = %q, want empty", rec.RecommendedBead)
	}
	if !strings.Contains(rec.Rationale, "ao loop blocked") {
		t.Errorf("rationale missing blocked hint: %q", rec.Rationale)
	}
}

// TestLoopNextWork_HumanReadableFallback covers the non-JSON output path.
func TestLoopNextWork_HumanReadableFallback(t *testing.T) {
	chdirTemp(t)
	withFakeNextWorkRunners(t, &fakeBeadRunner{
		ReadyList: []ladder.Bead{
			{
				ID:          "soc-h",
				Title:       "human",
				Description: "Edit cli/x.go. when X then Y. Follows soc-prev.",
			},
		},
	}, fakeGrep{})

	out, err := executeCommand("loop", "next-work")
	if err != nil {
		t.Fatalf("err: %v\nout=%s", err, out)
	}
	if !strings.Contains(out, "next-work: soc-h (step 1)") {
		t.Errorf("human output: %q", out)
	}
}

// TestLoopNextWork_RegisteredOnLoop confirms registration under loopCmd.
func TestLoopNextWork_RegisteredOnLoop(t *testing.T) {
	var found bool
	for _, sub := range loopCmd.Commands() {
		if sub.Name() == "next-work" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("evolve next-work subcommand should be registered on loopCmd")
	}
}

// TestLoopNextWork_IncludeOperatorShape exercises the flag-driven step-1
// override.
func TestLoopNextWork_IncludeOperatorShape(t *testing.T) {
	chdirTemp(t)
	withFakeNextWorkRunners(t, &fakeBeadRunner{
		ReadyList: []ladder.Bead{
			{
				ID:          "soc-ops",
				Title:       "operator scaffold",
				Description: "Edit cli/x.go. when X then Y. Follows soc-prev.",
				Labels:      []string{"operator-shape"},
			},
		},
	}, fakeGrep{})

	out, err := executeCommand("loop", "next-work", "--include-operator-shape", "--json")
	if err != nil {
		t.Fatalf("err: %v\nout=%s", err, out)
	}
	start := strings.Index(out, "{")
	if start < 0 {
		t.Fatalf("no JSON: %q", out)
	}
	var rec ladder.Recommendation
	if err := json.Unmarshal([]byte(out[start:]), &rec); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rec.RecommendedBead != "soc-ops" {
		t.Errorf("bead = %q, want soc-ops", rec.RecommendedBead)
	}
}
