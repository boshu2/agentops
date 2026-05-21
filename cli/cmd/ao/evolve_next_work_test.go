// practices: [dora-metrics, lean-startup]
package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/evolve/ladder"
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

// withFixedNextWorkClock pins the timestamp clock.
func withFixedNextWorkClock(t *testing.T, ts time.Time) {
	t.Helper()
	prev := evolveNextWorkClock
	evolveNextWorkClock = func() time.Time { return ts }
	t.Cleanup(func() { evolveNextWorkClock = prev })
}

// TestEvolveNextWork_Step1Pick exercises the happy path: shape-compatible
// bead picked at step 1 with JSON output.
func TestEvolveNextWork_Step1Pick(t *testing.T) {
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

	out, err := executeCommand("evolve", "next-work", "--json")
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

// TestEvolveNextWork_PrimitiveTestFailsRecommendsScout exercises the step-3
// scout-mode rationale.
func TestEvolveNextWork_PrimitiveTestFailsRecommendsScout(t *testing.T) {
	chdirTemp(t)
	withFakeNextWorkRunners(t, &fakeBeadRunner{
		ReadyList: []ladder.Bead{
			{ID: "soc-vague", Title: "vague", Description: "make better"},
		},
	}, fakeGrep{})

	out, err := executeCommand("evolve", "next-work", "--json")
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

// TestEvolveNextWork_LadderExhaustionEmitsBlockedHint covers the terminal
// "ladder exhausted" recommendation.
func TestEvolveNextWork_LadderExhaustionEmitsBlockedHint(t *testing.T) {
	chdirTemp(t)
	withFakeNextWorkRunners(t, &fakeBeadRunner{}, fakeGrep{})

	out, err := executeCommand("evolve", "next-work", "--json")
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
	if !strings.Contains(rec.Rationale, "ao evolve blocked") {
		t.Errorf("rationale missing blocked hint: %q", rec.Rationale)
	}
}

// TestEvolveNextWork_HumanReadableFallback covers the non-JSON output path.
func TestEvolveNextWork_HumanReadableFallback(t *testing.T) {
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

	out, err := executeCommand("evolve", "next-work")
	if err != nil {
		t.Fatalf("err: %v\nout=%s", err, out)
	}
	if !strings.Contains(out, "next-work: soc-h (step 1)") {
		t.Errorf("human output: %q", out)
	}
}

// TestEvolveNextWork_RegisteredOnEvolve confirms registration under evolveCmd.
func TestEvolveNextWork_RegisteredOnEvolve(t *testing.T) {
	var found bool
	for _, sub := range evolveCmd.Commands() {
		if sub.Name() == "next-work" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("evolve next-work subcommand should be registered on evolveCmd")
	}
}

// TestEvolveNextWork_IncludeOperatorShape exercises the flag-driven step-1
// override.
func TestEvolveNextWork_IncludeOperatorShape(t *testing.T) {
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

	out, err := executeCommand("evolve", "next-work", "--include-operator-shape", "--json")
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
