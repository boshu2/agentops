package evalsubstrate

import "testing"

// TestHoldoutBurnLedger_Spent counts only burns matching the (suite, gt_version)
// pair under test — the ledger is global, so a burn against a different suite or
// a different ground-truth version must not consume this run's quota.
func TestHoldoutBurnLedger_Spent(t *testing.T) {
	led := HoldoutBurnLedger{
		Budget: 3,
		Records: []BurnRecord{
			{SuiteRef: "s1", GTVersion: "gt-v1", RunID: "r1"},
			{SuiteRef: "s1", GTVersion: "gt-v1", RunID: "r2"},
			{SuiteRef: "s1", GTVersion: "gt-v2", RunID: "r3"}, // different version
			{SuiteRef: "s2", GTVersion: "gt-v1", RunID: "r4"}, // different suite
		},
	}
	if got := led.Spent("s1", "gt-v1"); got != 2 {
		t.Errorf("Spent(s1,gt-v1) = %d, want 2 (other suite/version excluded)", got)
	}
	if got := led.Spent("s1", "gt-v2"); got != 1 {
		t.Errorf("Spent(s1,gt-v2) = %d, want 1", got)
	}
	if got := led.Spent("nope", "gt-v1"); got != 0 {
		t.Errorf("Spent(nope,gt-v1) = %d, want 0", got)
	}
}

// holdoutGateInputs builds GateInputs whose requested GT row is on the given split,
// with a burn ledger of the given budget and prior-spent count for (suite, gt).
func holdoutGateInputs(split string, budget, alreadySpent int) GateInputs {
	recs := make([]BurnRecord, alreadySpent)
	for i := range recs {
		recs[i] = BurnRecord{SuiteRef: "suite-x", GTVersion: "gt-1"}
	}
	return GateInputs{
		Suite:       &Suite{ID: "suite-x"},
		GTRequested: "gt-1",
		GroundTruth: []GroundTruthRow{{ID: "gt-1", Split: split}},
		BurnLedger:  &HoldoutBurnLedger{Budget: budget, Records: recs},
	}
}

// TestGate3_HoldoutBurnExhausted_Refuses: §6 #3 — a holdout-split run whose
// (suite, gt_version) quota is already spent must refuse. Reusing spent holdout
// invalidates the split's statistical guarantee (the load-bearing eval invariant).
func TestGate3_HoldoutBurnExhausted_Refuses(t *testing.T) {
	in := holdoutGateInputs("holdout", 2, 2) // budget 2, already spent 2
	r := gate3HoldoutBurn(in)
	if r == nil {
		t.Fatal("gate3 must refuse when holdout burn quota is exhausted")
	}
	if r.GateNumber != 3 {
		t.Errorf("GateNumber = %d, want 3", r.GateNumber)
	}
	if r.GateName != "holdout_burn_exhausted" {
		t.Errorf("GateName = %q, want holdout_burn_exhausted", r.GateName)
	}
}

// TestGate3_HoldoutWithinBudget_Passes: a holdout run with quota remaining passes.
func TestGate3_HoldoutWithinBudget_Passes(t *testing.T) {
	in := holdoutGateInputs("holdout", 3, 1) // budget 3, spent 1 → 2 remain
	if r := gate3HoldoutBurn(in); r != nil {
		t.Errorf("gate3 must pass with quota remaining, got refusal: %s", r.GateName)
	}
}

// TestGate3_DevSplit_NeverBurns: dev-split runs never consume holdout budget, so
// gate3 never fires even when the budget is fully spent. This is the dev-split-only
// escape valve the substrate-gap bead called out.
func TestGate3_DevSplit_NeverBurns(t *testing.T) {
	in := holdoutGateInputs("dev", 1, 5) // budget 1, "spent" 5 — but split is dev
	if r := gate3HoldoutBurn(in); r != nil {
		t.Errorf("gate3 must not fire on dev split, got refusal: %s", r.GateName)
	}
}

// TestGate3_NoLedger_Skips: when no burn ledger is wired (Day-4 input absent),
// gate3 is a no-op rather than a hard failure — preserves Day-2 manifest-only runs.
func TestGate3_NoLedger_Skips(t *testing.T) {
	in := GateInputs{
		Suite:       &Suite{ID: "suite-x"},
		GTRequested: "gt-1",
		GroundTruth: []GroundTruthRow{{ID: "gt-1", Split: "holdout"}},
		BurnLedger:  nil,
	}
	if r := gate3HoldoutBurn(in); r != nil {
		t.Errorf("gate3 must skip when no ledger is wired, got refusal: %s", r.GateName)
	}
}

// TestRunGates_IncludesGate3: gate3 is wired into RunGates in §6 order, so an
// exhausted-holdout run is refused through the aggregate entrypoint, not just the
// private function.
func TestRunGates_IncludesGate3(t *testing.T) {
	in := holdoutGateInputs("holdout", 1, 1)
	rs := RunGates(in)
	found := false
	for _, r := range rs {
		if r.GateNumber == 3 {
			found = true
		}
	}
	if !found {
		t.Error("RunGates must surface gate3 holdout-burn refusal")
	}
}
