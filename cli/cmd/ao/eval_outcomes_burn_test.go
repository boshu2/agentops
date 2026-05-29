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
	after := registerOutcomesBurn(led, holdout)
	if got := after.Spent("suite-x", "gt-1"); got != 1 {
		t.Fatalf("holdout grade must register exactly +1 burn, Spent=%d want 1", got)
	}
	rec := after.Records[len(after.Records)-1]
	if rec.SuiteRef != "suite-x" || rec.GTVersion != "gt-1" || rec.RunID != "run-cloud-1" {
		t.Errorf("burn record identity wrong: %+v", rec)
	}

	// A second distinct holdout run burns again (+1 each, no silent bypass).
	holdout2 := holdout
	holdout2.RunID = "run-cloud-2"
	after2 := registerOutcomesBurn(after, holdout2)
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
	after := registerOutcomesBurn(led, dev)
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
	after := registerOutcomesBurn(led, outcomesScore{SourceTaskID: "t", SuiteRef: "s", GroundTruthVersion: "g"})
	if len(after.Records) != 0 {
		t.Errorf("empty split must register no burn, got %d records", len(after.Records))
	}
}
