// practices: [design-by-contract, ai-assisted-dev]
package governor

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/yieldledger"
)

// ledgerPath replicates yieldledger's on-disk location (its own ledgerPath is
// unexported) so the read-only conformance test can snapshot the file.
func ledgerPath(t *testing.T, root string) string {
	t.Helper()
	return filepath.Join(root, filepath.FromSlash(yieldledger.ArtifactRelPath))
}

// SPC.TEST (age-wy3.4): the governor's conformance to the control-loop-model.md §6
// contract. §6 has five clauses; not all are governor-unit-scope — this maps each:
//
//   §6.1 Gates are DETERMINISTIC (no free-form LLM self-grade).
//        -> GOVERNOR SCOPE. Asserted: TestConformance_Deterministic — same ledger
//           yields the identical verdict on every call (pure function, no model).
//   §6.2 Fast loop terminates on a grounded verdict; cap is backstop only.
//        -> NOT governor scope (the fast loop is planpawl/decide.go).
//   §6.3 No self-modification inside a run (no gate added/removed/re-tuned in flight).
//        -> GOVERNOR SCOPE. Asserted: TestConformance_ReadOnly — the governor never
//           mutates the ledger it reads; it is a pure read.
//   §6.4 Escapes route to the slow loop.
//        -> Membrane scope; the governor CONSUMES escapes (DetectEscapes) as its
//           slow-loop input, it does not own routing.
//   §6.5 The orchestrator gates and routes, never reasons.
//        -> Upheld by construction: the governor is deterministic pure functions
//           over the ledger (no LLM, no I/O beyond the read) — covered by §6.1.
//
// The governor's three behavioral properties (budget ship-vs-harden, two-sided
// fitness rejects cry-wolf, special-cause-only ignores one-off noise) are proven in
// budget_test.go and noiseband_test.go; this file proves the CONTRACT-level clauses.

func confGV(t *testing.T, root, run, bead, disposition, domain string, attempt int) {
	t.Helper()
	w := yieldledger.Writer{}
	headSHA := bead + "-headsha0"
	if _, err := w.AppendGateVerdict(root, yieldledger.GateVerdictInput{
		BeadID:          bead,
		RunID:           run,
		TS:              time.Date(2026, 6, 22, 16, 0, attempt, 0, time.UTC),
		Difficulty:      1,
		PawlVerdictRef:  yieldledger.PawlVerdictRef{BeadID: bead, HeadSHA: headSHA},
		Disposition:     disposition,
		AuthorContextID: "ctx-" + bead,
		AuthorFamily:    "claude",
		HeadSHA:         headSHA,
		Attempt:         attempt,
		Domain:          domain,
	}); err != nil {
		t.Fatalf("append: %v", err)
	}
}

// seedMixedLedger builds a ledger with clean confirms + escapes (some sharing a
// domain) so both the budget and the noise-band have real signal to judge.
func seedMixedLedger(t *testing.T, root string) {
	for i := range 6 {
		confGV(t, root, "r", "clean-"+string(rune('a'+i)), yieldledger.DispositionConfirmed, "core", 1)
	}
	for _, b := range []string{"e1", "e2", "e3"} {
		confGV(t, root, "r", b, yieldledger.DispositionConfirmed, "concurrency", 1)
		confGV(t, root, "r", b, yieldledger.DispositionRefuted, "concurrency", 2)
	}
}

// TestConformance_Deterministic (§6.1): the governor's decisions are a fixed
// function of the ledger — repeated calls yield byte-identical verdicts. A
// non-deterministic governor (map-order, wall-clock, randomness) is the #1 cause of
// a self-improving loop that oscillates; this guards against introducing one.
func TestConformance_Deterministic(t *testing.T) {
	root := t.TempDir()
	seedMixedLedger(t, root)

	for range 5 {
		l := loadConf(t, root)
		b1 := EvaluateBudget(l, DefaultBudgetConfig())
		b2 := EvaluateBudget(l, DefaultBudgetConfig())
		if b1 != b2 {
			t.Fatalf("budget not deterministic on identical input: %+v vs %+v", b1, b2)
		}
		a1 := ShouldAdjust(l, DefaultNoiseBandConfig())
		a2 := ShouldAdjust(l, DefaultNoiseBandConfig())
		if a1.Decision != a2.Decision || len(a1.SpecialCauseDomains) != len(a2.SpecialCauseDomains) {
			t.Fatalf("noise-band not deterministic: %+v vs %+v", a1, a2)
		}
	}
}

// TestConformance_ReadOnly (§6.3): the governor never mutates the ledger it judges
// — no self-modification in a run. The on-disk ledger is byte-identical before and
// after evaluating both the budget and the noise-band.
func TestConformance_ReadOnly(t *testing.T) {
	root := t.TempDir()
	seedMixedLedger(t, root)
	path := ledgerPath(t, root)

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	l := loadConf(t, root)
	_ = EvaluateBudget(l, DefaultBudgetConfig())
	_ = ShouldAdjust(l, DefaultNoiseBandConfig())
	_ = FitnessAdmits(FitnessSnapshot{0.5, 0.1}, FitnessSnapshot{0.6, 0.1})

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("re-read ledger: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("governor mutated the ledger (§6.3 violation: no self-modification in a run)")
	}
}

func loadConf(t *testing.T, root string) *yieldledger.Ledger {
	t.Helper()
	l, err := yieldledger.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return l
}
