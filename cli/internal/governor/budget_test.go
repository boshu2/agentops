// practices: [design-by-contract, ai-assisted-dev]
package governor

import (
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/yieldledger"
)

// gv appends a gate-verdict through the PRODUCTION writer (round-trip fidelity per
// the test-pyramid Fixture Fidelity rule), then the test Loads it back — never a
// hand-built in-memory Ledger.
func gv(t *testing.T, root, run, bead, disposition string, attempt int) {
	t.Helper()
	w := yieldledger.Writer{}
	headSHA := bead + "-h"
	if _, err := w.AppendGateVerdict(root, yieldledger.GateVerdictInput{
		BeadID:          bead,
		RunID:           run,
		TS:              time.Date(2026, 6, 22, 12, 0, attempt, 0, time.UTC),
		Difficulty:      1,
		PawlVerdictRef:  yieldledger.PawlVerdictRef{BeadID: bead, HeadSHA: headSHA},
		Disposition:     disposition,
		AuthorContextID: "ctx-" + bead,
		AuthorFamily:    "claude",
		HeadSHA:         headSHA,
		Attempt:         attempt,
	}); err != nil {
		t.Fatalf("append %s %s/a%d: %v", disposition, bead, attempt, err)
	}
}

func load(t *testing.T, root string) *yieldledger.Ledger {
	t.Helper()
	l, err := yieldledger.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return l
}

// escapePair writes a CONFIRMED then a higher-attempt REFUTED for one bead — the
// membrane let a false-done through and a later pass caught it (one escape).
func escapePair(t *testing.T, root, run, bead string) {
	gv(t, root, run, bead, yieldledger.DispositionConfirmed, 1)
	gv(t, root, run, bead, yieldledger.DispositionRefuted, 2)
}

func TestEvaluateBudget_CleanWindowShips(t *testing.T) {
	root := t.TempDir()
	for i := range 8 {
		gv(t, root, "r", "clean-"+string(rune('a'+i)), yieldledger.DispositionConfirmed, 1)
	}
	v := EvaluateBudget(load(t, root), DefaultBudgetConfig())
	if v.Decision != Ship {
		t.Fatalf("clean window decision = %q, want ship (%+v)", v.Decision, v)
	}
	if v.EscapesInWindow != 0 || v.ConfirmedInWindow != 8 {
		t.Fatalf("clean window: escapes=%d confirmed=%d, want 0/8", v.EscapesInWindow, v.ConfirmedInWindow)
	}
	if v.BurnRate != 0 {
		t.Fatalf("clean window burn_rate = %v, want 0", v.BurnRate)
	}
}

func TestEvaluateBudget_BurnedBudgetHardens(t *testing.T) {
	root := t.TempDir()
	// 8 clean confirms + 2 escape pairs -> confirmed=10, escapes=2, rate=0.20 > T(0.10)
	// -> burn=2.0, and confirmed(10) >= min(5) -> HARDEN.
	for i := range 8 {
		gv(t, root, "r", "clean-"+string(rune('a'+i)), yieldledger.DispositionConfirmed, 1)
	}
	escapePair(t, root, "r", "esc-1")
	escapePair(t, root, "r", "esc-2")

	v := EvaluateBudget(load(t, root), DefaultBudgetConfig())
	if v.ConfirmedInWindow != 10 || v.EscapesInWindow != 2 {
		t.Fatalf("confirmed=%d escapes=%d, want 10/2 (%+v)", v.ConfirmedInWindow, v.EscapesInWindow, v)
	}
	if v.RollingEscapeRate < 0.199 || v.RollingEscapeRate > 0.201 {
		t.Fatalf("rolling escape rate = %v, want ~0.20", v.RollingEscapeRate)
	}
	if v.BurnRate <= 1.0 {
		t.Fatalf("burn_rate = %v, want > 1.0", v.BurnRate)
	}
	if v.Decision != Harden {
		t.Fatalf("decision = %q, want harden (%+v)", v.Decision, v)
	}
}

func TestEvaluateBudget_InsideToleranceShips(t *testing.T) {
	root := t.TempDir()
	// 10 clean confirms + 1 escape pair -> confirmed=11, escapes=1, rate~0.09 < T(0.10)
	// -> burn<1.0 -> ship.
	for i := range 10 {
		gv(t, root, "r", "clean-"+string(rune('a'+i)), yieldledger.DispositionConfirmed, 1)
	}
	escapePair(t, root, "r", "esc-1")

	v := EvaluateBudget(load(t, root), DefaultBudgetConfig())
	if v.ConfirmedInWindow != 11 || v.EscapesInWindow != 1 {
		t.Fatalf("confirmed=%d escapes=%d, want 11/1", v.ConfirmedInWindow, v.EscapesInWindow)
	}
	if v.BurnRate > 1.0 {
		t.Fatalf("burn_rate = %v, want <= 1.0 (inside tolerance)", v.BurnRate)
	}
	if v.Decision != Ship {
		t.Fatalf("decision = %q, want ship (%+v)", v.Decision, v)
	}
}

// TestEvaluateBudget_SpecialCauseFloor is the anti-tampering guard: even a 100%
// escape rate must SHIP when the window holds fewer than min_confirmed verdicts —
// hardening on a thin window is tampering on common-cause noise (control-loop-model §4).
func TestEvaluateBudget_SpecialCauseFloor(t *testing.T) {
	root := t.TempDir()
	// 2 clean confirms + 1 escape pair -> confirmed=3 (< min 5). rate=1/3=0.33,
	// burn>1.0, but the floor blocks harden.
	gv(t, root, "r", "clean-a", yieldledger.DispositionConfirmed, 1)
	gv(t, root, "r", "clean-b", yieldledger.DispositionConfirmed, 1)
	escapePair(t, root, "r", "esc-1")

	v := EvaluateBudget(load(t, root), DefaultBudgetConfig())
	if v.ConfirmedInWindow >= v.MinConfirmed {
		t.Fatalf("precondition: confirmed(%d) must be < min(%d)", v.ConfirmedInWindow, v.MinConfirmed)
	}
	if v.BurnRate <= 1.0 {
		t.Fatalf("precondition: burn_rate(%v) should exceed 1.0 so the floor is what's tested", v.BurnRate)
	}
	if v.Decision != Ship {
		t.Fatalf("decision = %q, want ship — the special-cause floor must block hardening on a thin window (%+v)", v.Decision, v)
	}
}

// TestEvaluateBudget_CrossRunEscape proves the cross-run windowing: a CONFIRMED in
// run A that a REFUTED in run B catches is an escape (the per-run gauge cannot see
// this). The whole point of SPC.1's windowed detector.
func TestEvaluateBudget_CrossRunEscape(t *testing.T) {
	root := t.TempDir()
	for i := range 9 {
		gv(t, root, "rA", "clean-"+string(rune('a'+i)), yieldledger.DispositionConfirmed, 1)
	}
	// CONFIRMED in run A, later REFUTED in run B for the SAME bead -> cross-run escape.
	gv(t, root, "rA", "esc-x", yieldledger.DispositionConfirmed, 1)
	gv(t, root, "rB", "esc-x", yieldledger.DispositionRefuted, 2)

	v := EvaluateBudget(load(t, root), DefaultBudgetConfig())
	if v.EscapesInWindow != 1 {
		t.Fatalf("cross-run escapes = %d, want 1 (the per-run gauge would miss this) (%+v)", v.EscapesInWindow, v)
	}
}

// TestEvaluateBudget_RollingWindowDropsOldEscapes proves the window is rolling:
// old escapes age out, so a process that WAS hot but has since shipped clean work
// recovers to ship (no all-time inertia).
func TestEvaluateBudget_RollingWindowDropsOldEscapes(t *testing.T) {
	root := t.TempDir()
	// Old: 3 escape pairs (6 events) early in the stream.
	for i := range 3 {
		escapePair(t, root, "r", "old-esc-"+string(rune('a'+i)))
	}
	// Recent: 20 clean confirms (fills the default window 20) -> old escapes age out.
	for i := range 20 {
		gv(t, root, "r", "recent-"+string(rune('a'+i)), yieldledger.DispositionConfirmed, 1)
	}
	v := EvaluateBudget(load(t, root), DefaultBudgetConfig())
	if v.EscapesInWindow != 0 {
		t.Fatalf("rolling window still counts old escapes: escapes=%d, want 0 (window must drop them) (%+v)", v.EscapesInWindow, v)
	}
	if v.Decision != Ship {
		t.Fatalf("decision = %q, want ship after old escapes age out", v.Decision)
	}
}

// TestEvaluateBudget_MultiRefuteCountsOneEscape (REFUTED 2026-06-22 by cross-family
// pawl): a single escaped CONFIRMED that SEVERAL later verdicts refute is ONE
// membrane miss (DetectEscapes v1 one-per-bead), not one per refutation — counting
// every refute inflates the escape rate and can wrongly harden on one false-done.
func TestEvaluateBudget_MultiRefuteCountsOneEscape(t *testing.T) {
	root := t.TempDir()
	for i := range 8 {
		gv(t, root, "r", "clean-"+string(rune('a'+i)), yieldledger.DispositionConfirmed, 1)
	}
	// Bead A: CONFIRMED@1, then REFUTED@2 AND REFUTED@3 — still exactly ONE escape.
	gv(t, root, "r", "multi", yieldledger.DispositionConfirmed, 1)
	gv(t, root, "r", "multi", yieldledger.DispositionRefuted, 2)
	gv(t, root, "r", "multi", yieldledger.DispositionRefuted, 3)

	v := EvaluateBudget(load(t, root), DefaultBudgetConfig())
	if v.EscapesInWindow != 1 {
		t.Fatalf("multi-refute escapes = %d, want 1 (one escaped CONFIRMED is one miss, not one per refute) (%+v)", v.EscapesInWindow, v)
	}
}

func TestEvaluateBudget_NilLedgerShips(t *testing.T) {
	v := EvaluateBudget(nil, DefaultBudgetConfig())
	if v.Decision != Ship {
		t.Fatalf("nil ledger decision = %q, want ship (no signal)", v.Decision)
	}
}

func TestBudgetConfig_ResolveFillsDefaults(t *testing.T) {
	got := BudgetConfig{}.Resolve()
	if got.WindowSize != DefaultWindowSize || got.ToleranceEscapeRate != DefaultToleranceEscapeRate || got.MinConfirmed != DefaultMinConfirmed {
		t.Fatalf("Resolve of empty config = %+v, want defaults", got)
	}
	// A partial override keeps its set field, fills the rest.
	p := BudgetConfig{WindowSize: 7}.Resolve()
	if p.WindowSize != 7 || p.MinConfirmed != DefaultMinConfirmed {
		t.Fatalf("partial Resolve = %+v, want WindowSize=7 + default MinConfirmed", p)
	}
}
