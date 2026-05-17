// practices: [tdd, bdd-gherkin]
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/verdictledger"
)

// TestGoalsMeasure_ProducerWritesVerdictLedger proves the F5.2 producer
// hookup: a completed `ao goals measure --scenarios-only` run appends one
// verdict-ledger iteration record per directive, capturing each directive's
// scenario verdict. Without this the F5 re-steer engine sees an empty ledger.
func TestGoalsMeasure_ProducerWritesVerdictLedger(t *testing.T) {
	dir := setupMeasureScenarioProject(t, goalsMDWithScenarioGate, true)
	goalsMeasureScenariosOnly = true

	if _, err := captureStdout(t, func() error {
		return goalsMeasureCmd.RunE(goalsMeasureCmd, nil)
	}); err != nil {
		t.Fatalf("goals measure --scenarios-only: %v", err)
	}

	ledgerPath := filepath.Join(dir, filepath.FromSlash(verdictledger.ArtifactRelPath))
	if _, err := os.Stat(ledgerPath); err != nil {
		t.Fatalf("verdict ledger not written at %s: %v", ledgerPath, err)
	}
	ledger, err := verdictledger.LoadPath(ledgerPath)
	if err != nil {
		t.Fatalf("LoadPath(ledger): %v", err)
	}

	// Two directives in the fixture: d-ship-parser (both scenarios pass) and
	// d-harden-loader (its scenario fails its 0.5 threshold).
	if got := len(ledger.Records); got != 2 {
		t.Fatalf("ledger records = %d, want 2 (%+v)", got, ledger.Records)
	}

	verdicts := map[string]string{}
	for _, r := range ledger.Records {
		if r.RecordType != verdictledger.RecordIteration {
			t.Errorf("record type = %q, want iteration", r.RecordType)
		}
		verdicts[r.DirectiveID] = r.ScenarioVerdict
	}
	if got := verdicts["d-ship-parser"]; got != verdictledger.VerdictPass {
		t.Errorf("d-ship-parser verdict = %q, want pass", got)
	}
	if got := verdicts["d-harden-loader"]; got != verdictledger.VerdictFail {
		t.Errorf("d-harden-loader verdict = %q, want fail", got)
	}
}

// TestGoalsMeasure_ProducerAppendsAcrossRuns proves the producer hookup is
// append-only: a second measure run adds a second iteration per directive
// rather than clobbering the first — exactly the cross-run accumulation the
// F5.2 failure-streak detection depends on.
func TestGoalsMeasure_ProducerAppendsAcrossRuns(t *testing.T) {
	dir := setupMeasureScenarioProject(t, goalsMDWithScenarioGate, true)
	goalsMeasureScenariosOnly = true

	for i := 0; i < 3; i++ {
		if _, err := captureStdout(t, func() error {
			return goalsMeasureCmd.RunE(goalsMeasureCmd, nil)
		}); err != nil {
			t.Fatalf("measure run %d: %v", i, err)
		}
	}

	ledger, err := verdictledger.LoadPath(filepath.Join(dir, filepath.FromSlash(verdictledger.ArtifactRelPath)))
	if err != nil {
		t.Fatalf("LoadPath(ledger): %v", err)
	}
	// 3 runs * 2 directives = 6 iteration records.
	if got := len(ledger.Records); got != 6 {
		t.Fatalf("ledger records = %d, want 6", got)
	}
	if got := ledger.IterationCount("d-harden-loader"); got != 3 {
		t.Errorf("IterationCount(d-harden-loader) = %d, want 3", got)
	}
	// d-harden-loader fails every run -> a 3-run failure streak accumulates.
	if got := ledger.FailureStreak("d-harden-loader"); got != 3 {
		t.Errorf("FailureStreak(d-harden-loader) = %d, want 3", got)
	}
}

// TestGoalsMeasure_ProducerDoesNotChangeStdout proves the producer hookup is
// additive: enabling it does not alter the measure command's stdout (the
// ledger is a side artifact only).
func TestGoalsMeasure_ProducerDoesNotChangeStdout(t *testing.T) {
	dir := setupMeasureScenarioProject(t, goalsMDWithScenarioGate, true)
	goalsMeasureScenariosOnly = true

	out, err := captureStdout(t, func() error {
		return goalsMeasureCmd.RunE(goalsMeasureCmd, nil)
	})
	if err != nil {
		t.Fatalf("measure: %v", err)
	}
	if !bytes.Contains([]byte(out), []byte("Scenario satisfaction")) {
		t.Errorf("stdout missing scenario table; got: %q", out)
	}
	// The ledger path must NOT appear in stdout — it is a silent side artifact.
	if bytes.Contains([]byte(out), []byte(verdictledger.ArtifactRelPath)) {
		t.Errorf("stdout leaked verdict-ledger path: %q", out)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(verdictledger.ArtifactRelPath))); err != nil {
		t.Fatalf("ledger artifact missing: %v", err)
	}
}
