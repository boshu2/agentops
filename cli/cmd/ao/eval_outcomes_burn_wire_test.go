package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/evalsubstrate"
	"github.com/spf13/cobra"
)

// writeBurnLedgerFile seeds a HoldoutBurnLedger JSON file.
func writeBurnLedgerFile(t *testing.T, path string, led evalsubstrate.HoldoutBurnLedger) {
	t.Helper()
	blob, err := json.Marshal(led)
	if err != nil {
		t.Fatalf("marshal ledger: %v", err)
	}
	if err := os.WriteFile(path, blob, 0o644); err != nil {
		t.Fatalf("write ledger %s: %v", path, err)
	}
}

func readBurnLedgerFile(t *testing.T, path string) evalsubstrate.HoldoutBurnLedger {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ledger %s: %v", path, err)
	}
	var led evalsubstrate.HoldoutBurnLedger
	if err := json.Unmarshal(raw, &led); err != nil {
		t.Fatalf("parse ledger %s: %v", path, err)
	}
	return led
}

func writeHoldoutScore(t *testing.T, path, suite, gt, runID string) {
	t.Helper()
	s := outcomesScore{
		SourceTaskID:       "task-x",
		JudgeContentHash:   "sha256:abc",
		Aggregate:          0.9,
		Threshold:          0.8,
		Split:              "holdout",
		SuiteRef:           suite,
		GroundTruthVersion: gt,
		RunID:              runID,
	}
	blob, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal score: %v", err)
	}
	if err := os.WriteFile(path, blob, 0o644); err != nil {
		t.Fatalf("write score %s: %v", path, err)
	}
}

// ingestCmdForTest returns a cobra command whose stdout is discarded and
// restores the process-global ingest flag vars after the test.
func ingestCmdForTest(t *testing.T, burnLedger string) *cobra.Command {
	t.Helper()
	origLedger, origHash := evalOutcomesIngestBurnLedger, evalOutcomesIngestExpectHash
	t.Cleanup(func() {
		evalOutcomesIngestBurnLedger = origLedger
		evalOutcomesIngestExpectHash = origHash
	})
	evalOutcomesIngestBurnLedger = burnLedger
	evalOutcomesIngestExpectHash = ""
	cmd := &cobra.Command{}
	cmd.SetOut(&cobraDiscard{})
	return cmd
}

// TestApplyOutcomesBurn_RefusesAtCommandLevel_WhenQuotaExhausted is the gate #3
// RUNTIME proof (ag-vbwx): with a --burn-ledger whose (suite,gt) quota is already
// spent, `ao eval outcomes ingest` of a holdout score REFUSES at the command
// level (not merely in the unit-tested primitive), and leaves the ledger
// unmutated. This closes the council 2026-05-30 caveat that registerOutcomesBurn
// was wired into no command path.
func TestApplyOutcomesBurn_RefusesAtCommandLevel_WhenQuotaExhausted(t *testing.T) {
	dir := t.TempDir()
	ledgerPath := filepath.Join(dir, "ledger.json")
	writeBurnLedgerFile(t, ledgerPath, evalsubstrate.HoldoutBurnLedger{
		Budget:  1,
		Records: []evalsubstrate.BurnRecord{{SuiteRef: "suite-x", GTVersion: "gt-1", RunID: "r0"}},
	})
	scorePath := filepath.Join(dir, "score.json")
	writeHoldoutScore(t, scorePath, "suite-x", "gt-1", "run-cloud-2")

	cmd := ingestCmdForTest(t, ledgerPath)
	if err := runEvalOutcomesIngest(cmd, []string{scorePath}); err == nil {
		t.Fatal("ingest must refuse at the command level when the holdout quota is exhausted (gate #3 runtime)")
	}
	after := readBurnLedgerFile(t, ledgerPath)
	if got := after.Spent("suite-x", "gt-1"); got != 1 {
		t.Errorf("refused ingest must not mutate the ledger, Spent=%d want 1", got)
	}
}

// TestApplyOutcomesBurn_PersistsAcrossInvocations proves the burn is durable: two
// distinct holdout ingests under a budget of 2 both succeed and each appends one
// record to the on-disk ledger; the third (quota now exhausted) refuses. This is
// the cross-invocation persistence the council required.
func TestApplyOutcomesBurn_PersistsAcrossInvocations(t *testing.T) {
	dir := t.TempDir()
	ledgerPath := filepath.Join(dir, "ledger.json")
	writeBurnLedgerFile(t, ledgerPath, evalsubstrate.HoldoutBurnLedger{Budget: 2})
	scorePath := filepath.Join(dir, "score.json")

	cmd := ingestCmdForTest(t, ledgerPath)

	writeHoldoutScore(t, scorePath, "suite-x", "gt-1", "run-1")
	if err := runEvalOutcomesIngest(cmd, []string{scorePath}); err != nil {
		t.Fatalf("first holdout ingest under budget must succeed: %v", err)
	}
	if got := readBurnLedgerFile(t, ledgerPath).Spent("suite-x", "gt-1"); got != 1 {
		t.Fatalf("first ingest must persist +1 burn, Spent=%d want 1", got)
	}

	writeHoldoutScore(t, scorePath, "suite-x", "gt-1", "run-2")
	if err := runEvalOutcomesIngest(cmd, []string{scorePath}); err != nil {
		t.Fatalf("second holdout ingest under budget must succeed: %v", err)
	}
	if got := readBurnLedgerFile(t, ledgerPath).Spent("suite-x", "gt-1"); got != 2 {
		t.Fatalf("second ingest must persist a 2nd burn, Spent=%d want 2", got)
	}

	writeHoldoutScore(t, scorePath, "suite-x", "gt-1", "run-3")
	if err := runEvalOutcomesIngest(cmd, []string{scorePath}); err == nil {
		t.Fatal("third holdout ingest must refuse — budget 2 is exhausted")
	}
	if got := readBurnLedgerFile(t, ledgerPath).Spent("suite-x", "gt-1"); got != 2 {
		t.Errorf("refused third ingest must not append, Spent=%d want 2", got)
	}
}

// TestApplyOutcomesBurn_EmptyPathNoOp: with no --burn-ledger configured, holdout
// ingest is unaffected (enforcement off; dev/legacy flows preserved).
func TestApplyOutcomesBurn_EmptyPathNoOp(t *testing.T) {
	dir := t.TempDir()
	scorePath := filepath.Join(dir, "score.json")
	writeHoldoutScore(t, scorePath, "suite-x", "gt-1", "run-1")

	cmd := ingestCmdForTest(t, "") // empty burn-ledger path
	if err := runEvalOutcomesIngest(cmd, []string{scorePath}); err != nil {
		t.Fatalf("no --burn-ledger configured must not enforce gate #3: %v", err)
	}
}

// TestApplyOutcomesBurn_DevSplitNoBurn: a dev-split score never burns, even when
// the configured ledger is already at quota — only holdout grades consume quota.
func TestApplyOutcomesBurn_DevSplitNoBurn(t *testing.T) {
	dir := t.TempDir()
	ledgerPath := filepath.Join(dir, "ledger.json")
	writeBurnLedgerFile(t, ledgerPath, evalsubstrate.HoldoutBurnLedger{
		Budget:  1,
		Records: []evalsubstrate.BurnRecord{{SuiteRef: "suite-x", GTVersion: "gt-1", RunID: "r0"}},
	})
	scorePath := filepath.Join(dir, "score.json")
	dev := outcomesScore{SourceTaskID: "t", Split: "dev", SuiteRef: "suite-x", GroundTruthVersion: "gt-1", RunID: "d1", Aggregate: 0.9, Threshold: 0.8}
	blob, _ := json.Marshal(dev)
	if err := os.WriteFile(scorePath, blob, 0o644); err != nil {
		t.Fatalf("write dev score: %v", err)
	}

	cmd := ingestCmdForTest(t, ledgerPath)
	if err := runEvalOutcomesIngest(cmd, []string{scorePath}); err != nil {
		t.Fatalf("dev-split ingest must never burn or refuse: %v", err)
	}
	if got := readBurnLedgerFile(t, ledgerPath).Spent("suite-x", "gt-1"); got != 1 {
		t.Errorf("dev-split ingest must not append a burn, Spent=%d want 1", got)
	}
}
