package main

import (
	"testing"

	"github.com/boshu2/agentops/cli/internal/evalsubstrate"
)

// TestEvalOutcomesIngest_HoldoutRegistersBurn proves the burn write-side
// (ag-hdqu0.5): ingesting a grade produced against the holdout split registers
// exactly one burn in the global ledger, so a cloud/Codex Outcomes run cannot
// silently bypass holdout quota the way a local holdout run cannot. A dev-split
// grade registers NO burn. The registered burn is what gate #3 (the read-side,
// #607) later refuses against once the budget is spent.
func TestEvalOutcomesIngest_HoldoutRegistersBurn(t *testing.T) {
	led := evalsubstrate.HoldoutBurnLedger{Budget: 5}

	holdout := outcomesScore{
		SourceTaskID:       "task-x",
		Split:              "holdout",
		SuiteRef:           "suite-x",
		GroundTruthVersion: "gt-1",
		RunID:              "run-cloud-1",
		Aggregate:          0.9,
		Threshold:          0.8,
	}
	after, err := registerOutcomesBurn(led, holdout)
	if err != nil {
		t.Fatalf("first holdout burn under budget must not refuse: %v", err)
	}
	if got := after.Spent("suite-x", "gt-1"); got != 1 {
		t.Fatalf("holdout grade must register exactly +1 burn, Spent=%d want 1", got)
	}
	rec := after.Records[len(after.Records)-1]
	if rec.SuiteRef != "suite-x" || rec.GTVersion != "gt-1" || rec.RunID != "run-cloud-1" {
		t.Errorf("burn record identity wrong: %+v", rec)
	}

	// A second distinct holdout run burns again (+1 each, no silent bypass) —
	// still under the budget of 5, so it is allowed.
	holdout2 := holdout
	holdout2.RunID = "run-cloud-2"
	after2, err := registerOutcomesBurn(after, holdout2)
	if err != nil {
		t.Fatalf("second holdout burn under budget must not refuse: %v", err)
	}
	if got := after2.Spent("suite-x", "gt-1"); got != 2 {
		t.Errorf("each holdout ingest burns once: Spent=%d want 2", got)
	}
}

// TestEvalOutcomesIngest_DevSplitNoBurn: a dev-split grade registers no burn —
// the dev split is reusable, so ingesting against it must never consume holdout
// quota.
func TestEvalOutcomesIngest_DevSplitNoBurn(t *testing.T) {
	led := evalsubstrate.HoldoutBurnLedger{Budget: 5}
	dev := outcomesScore{
		SourceTaskID:       "task-x",
		Split:              "dev",
		SuiteRef:           "suite-x",
		GroundTruthVersion: "gt-1",
		RunID:              "run-dev-1",
	}
	after, err := registerOutcomesBurn(led, dev)
	if err != nil {
		t.Fatalf("dev-split ingest must never refuse: %v", err)
	}
	if got := after.Spent("suite-x", "gt-1"); got != 0 {
		t.Errorf("dev-split grade must register NO burn, Spent=%d want 0", got)
	}
	if len(after.Records) != 0 {
		t.Errorf("dev-split ingest must not append records, got %d", len(after.Records))
	}
}

// TestRegisterOutcomesBurn_EmptySplitNoBurn: an unset split is treated as
// non-holdout (no burn) — fail safe toward not over-consuming quota, since gate
// #3 only fires on an explicit holdout split anyway.
func TestRegisterOutcomesBurn_EmptySplitNoBurn(t *testing.T) {
	led := evalsubstrate.HoldoutBurnLedger{Budget: 5}
	after, err := registerOutcomesBurn(led, outcomesScore{SourceTaskID: "t", SuiteRef: "s", GroundTruthVersion: "g"})
	if err != nil {
		t.Fatalf("empty-split ingest must never refuse: %v", err)
	}
	if len(after.Records) != 0 {
		t.Errorf("empty split must register no burn, got %d records", len(after.Records))
	}
}

// TestRegisterOutcomesBurn_RefusesWhenQuotaExhausted is the gate #3 write-side
// guard (ag-62g68): once the (suite_ref, gt_version) holdout quota is spent, a
// further holdout ingest is REFUSED — symmetric with the read-side gate #3
// (gates.go), which refuses a local run on the same condition. Without this, a
// cloud/Codex Outcomes run could silently re-burn an exhausted holdout split,
// the exact statistical leak the burn ledger exists to prevent (Managed Agents
// are not ZDR). The ledger is left unmutated on refusal (no partial append).
func TestRegisterOutcomesBurn_RefusesWhenQuotaExhausted(t *testing.T) {
	// Budget 1, one burn already spent for (suite-x, gt-1).
	led := evalsubstrate.HoldoutBurnLedger{
		Budget:  1,
		Records: []evalsubstrate.BurnRecord{{SuiteRef: "suite-x", GTVersion: "gt-1", RunID: "run-prior"}},
	}
	second := outcomesScore{
		SourceTaskID:       "task-x",
		Split:              "holdout",
		SuiteRef:           "suite-x",
		GroundTruthVersion: "gt-1",
		RunID:              "run-cloud-2",
		Aggregate:          0.9,
		Threshold:          0.8,
	}
	after, err := registerOutcomesBurn(led, second)
	if err == nil {
		t.Fatal("gate #3 write-side must refuse a holdout burn once the quota is exhausted")
	}
	if got := after.Spent("suite-x", "gt-1"); got != 1 {
		t.Errorf("refused burn must not mutate the ledger, Spent=%d want 1", got)
	}
}

// TestRegisterOutcomesBurn_NoCeilingWhenBudgetUnset: a non-positive Budget means
// no enforceable ceiling is configured (Day-4 input absent → gate #3 is a
// no-op), so holdout burns are recorded without refusal — exactly the read-side
// gate #3 posture.
func TestRegisterOutcomesBurn_NoCeilingWhenBudgetUnset(t *testing.T) {
	led := evalsubstrate.HoldoutBurnLedger{
		Records: []evalsubstrate.BurnRecord{{SuiteRef: "suite-x", GTVersion: "gt-1", RunID: "r0"}},
	}
	s := outcomesScore{Split: "holdout", SuiteRef: "suite-x", GroundTruthVersion: "gt-1", RunID: "r1"}
	after, err := registerOutcomesBurn(led, s)
	if err != nil {
		t.Fatalf("unset budget must not enforce a ceiling: %v", err)
	}
	if got := after.Spent("suite-x", "gt-1"); got != 2 {
		t.Errorf("unset-budget holdout burn must append, Spent=%d want 2", got)
	}
}
