// practices: [design-by-contract, ai-assisted-dev]
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/governor"
	"github.com/boshu2/agentops/cli/internal/yieldledger"
)

func govVerdict(t *testing.T, root, run, bead, disposition string, attempt int) {
	t.Helper()
	w := yieldledger.Writer{}
	headSHA := bead + "-h"
	if _, err := w.AppendGateVerdict(root, yieldledger.GateVerdictInput{
		BeadID:          bead,
		RunID:           run,
		TS:              time.Date(2026, 6, 22, 13, 0, attempt, 0, time.UTC),
		Difficulty:      1,
		PawlVerdictRef:  yieldledger.PawlVerdictRef{BeadID: bead, HeadSHA: headSHA},
		Disposition:     disposition,
		AuthorContextID: "ctx-" + bead,
		AuthorFamily:    "claude",
		HeadSHA:         headSHA,
		Attempt:         attempt,
	}); err != nil {
		t.Fatalf("append %s %s: %v", disposition, bead, err)
	}
}

// runGovBudget runs the command against root, returning stdout + the RunE error,
// with all package-global flag state restored after (shared rootCmd; .claude/rules/go.md).
func runGovBudget(t *testing.T, root string) (string, error) {
	t.Helper()
	prev := testProjectDir
	testProjectDir = root
	origJSON, origWin, origTol, origMin := govBudgetJSON, govBudgetWindow, govBudgetTolerance, govBudgetMinConf
	t.Cleanup(func() {
		testProjectDir = prev
		govBudgetJSON, govBudgetWindow, govBudgetTolerance, govBudgetMinConf = origJSON, origWin, origTol, origMin
		governorBudgetCmd.SetOut(nil)
	})
	govBudgetJSON = true
	govBudgetWindow, govBudgetTolerance, govBudgetMinConf = 0, 0, 0
	var buf bytes.Buffer
	governorBudgetCmd.SetOut(&buf)
	err := runGovernorBudget(governorBudgetCmd, nil)
	return buf.String(), err
}

// runGovNoiseBand runs the noise-band command against root, restoring flag state.
func runGovNoiseBand(t *testing.T, root string) (string, error) {
	t.Helper()
	prev := testProjectDir
	testProjectDir = root
	origJSON, origWin, origLim := govNoiseJSON, govNoiseWindow, govNoiseLimit
	t.Cleanup(func() {
		testProjectDir = prev
		govNoiseJSON, govNoiseWindow, govNoiseLimit = origJSON, origWin, origLim
		governorNoiseBandCmd.SetOut(nil)
	})
	govNoiseJSON = true
	govNoiseWindow, govNoiseLimit = 0, 0
	var buf bytes.Buffer
	governorNoiseBandCmd.SetOut(&buf)
	err := runGovernorNoiseBand(governorNoiseBandCmd, nil)
	return buf.String(), err
}

func TestGovernorNoiseBand_SpecialCauseAdjusts(t *testing.T) {
	root := t.TempDir()
	// 3 escapes in the same domain -> special-cause -> adjust.
	for _, b := range []string{"e1", "e2", "e3"} {
		w := yieldledger.Writer{}
		for _, d := range []struct {
			disp    string
			attempt int
		}{{yieldledger.DispositionConfirmed, 1}, {yieldledger.DispositionRefuted, 2}} {
			headSHA := b + "-headsha0"
			if _, err := w.AppendGateVerdict(root, yieldledger.GateVerdictInput{
				BeadID: b, RunID: "r", Difficulty: 1,
				TS:              time.Date(2026, 6, 22, 15, 0, d.attempt, 0, time.UTC),
				PawlVerdictRef:  yieldledger.PawlVerdictRef{BeadID: b, HeadSHA: headSHA},
				Disposition:     d.disp,
				AuthorContextID: "ctx-" + b, AuthorFamily: "claude",
				HeadSHA: headSHA, Attempt: d.attempt, Domain: "concurrency",
			}); err != nil {
				t.Fatalf("append: %v", err)
			}
		}
	}
	out, err := runGovNoiseBand(t, root)
	if err != nil {
		t.Fatalf("noise-band err: %v", err)
	}
	var v governor.AdjustVerdict
	if uerr := json.Unmarshal([]byte(out), &v); uerr != nil {
		t.Fatalf("unmarshal %q: %v", out, uerr)
	}
	if v.Decision != governor.Adjust {
		t.Fatalf("decision = %q, want adjust (%+v)", v.Decision, v)
	}
}

func TestGovernorBudget_CleanLedgerShips(t *testing.T) {
	root := t.TempDir()
	for i := range 6 {
		govVerdict(t, root, "r", "clean-"+string(rune('a'+i)), yieldledger.DispositionConfirmed, 1)
	}
	out, err := runGovBudget(t, root)
	if err != nil {
		t.Fatalf("clean ledger returned error (want ship/nil): %v", err)
	}
	var v governor.BudgetVerdict
	if uerr := json.Unmarshal([]byte(out), &v); uerr != nil {
		t.Fatalf("unmarshal verdict %q: %v", out, uerr)
	}
	if v.Decision != governor.Ship {
		t.Fatalf("decision = %q, want ship", v.Decision)
	}
}

// TestGovernorBudget_BurnedLedgerHardensWithExitCode is the load-bearing one: a
// burned budget must return a *governorExitError with code 3 so a calling gate/loop
// can mechanically stop the line.
func TestGovernorBudget_BurnedLedgerHardensWithExitCode(t *testing.T) {
	root := t.TempDir()
	for i := range 8 {
		govVerdict(t, root, "r", "clean-"+string(rune('a'+i)), yieldledger.DispositionConfirmed, 1)
	}
	// 2 escape pairs -> escape rate 0.20 > tolerance 0.10 over a 10-confirm window.
	for _, b := range []string{"esc-1", "esc-2"} {
		govVerdict(t, root, "r", b, yieldledger.DispositionConfirmed, 1)
		govVerdict(t, root, "r", b, yieldledger.DispositionRefuted, 2)
	}
	out, err := runGovBudget(t, root)
	if err == nil {
		t.Fatalf("burned budget returned nil, want a harden exit error. out=%s", out)
	}
	var govErr *governorExitError
	if !errors.As(err, &govErr) {
		t.Fatalf("error type = %T, want *governorExitError: %v", err, err)
	}
	if govErr.ExitCode() != hardenExitCode {
		t.Fatalf("exit code = %d, want %d (harden)", govErr.ExitCode(), hardenExitCode)
	}
	var v governor.BudgetVerdict
	if uerr := json.Unmarshal([]byte(out), &v); uerr != nil {
		t.Fatalf("unmarshal verdict %q: %v", out, uerr)
	}
	if v.Decision != governor.Harden {
		t.Fatalf("decision = %q, want harden", v.Decision)
	}
}
