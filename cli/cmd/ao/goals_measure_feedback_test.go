package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/verdictledger"
)

// TestRecordVerdictLedgerIterations_AutoDraftsFailToPass is the F5.4 re-wiring
// contract (ag-uzy): `ao goals measure` recording a fail→pass verdict-ledger
// transition must trigger the feedback compiler to auto-draft a learning. This
// trigger was orphaned when #515 removed the daemon that used to invoke it.
func TestRecordVerdictLedgerIterations_AutoDraftsFailToPass(t *testing.T) {
	root := t.TempDir()
	learningsDir := filepath.Join(root, "docs", "learnings")
	var buf bytes.Buffer

	failReport := []directiveScenarioReport{{
		DirectiveID:          "d-rewire-test",
		ScenarioVerdict:      verdictledger.VerdictFail,
		ScenarioSatisfaction: 0.40,
		ScenarioCount:        5,
		EvaluatedCount:       5,
	}}
	passReport := []directiveScenarioReport{{
		DirectiveID:          "d-rewire-test",
		ScenarioVerdict:      verdictledger.VerdictPass,
		ScenarioSatisfaction: 1.0,
		ScenarioCount:        5,
		EvaluatedCount:       5,
	}}

	drafts := func() []string {
		files, _ := filepath.Glob(filepath.Join(learningsDir, "*fail-to-pass.md"))
		return files
	}

	// Measure run 1: failing — no transition yet, no draft.
	recordVerdictLedgerIterations(root, failReport, &buf)
	if got := drafts(); len(got) != 0 {
		t.Fatalf("no draft expected after a failing run, got %v", got)
	}

	// Measure run 2: now passing — fail→pass transition triggers the auto-draft.
	recordVerdictLedgerIterations(root, passReport, &buf)
	got := drafts()
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 auto-drafted learning after fail→pass, got %d (%v)", len(got), got)
	}

	content, err := os.ReadFile(got[0])
	if err != nil {
		t.Fatalf("read draft: %v", err)
	}
	if !strings.Contains(string(content), "status: draft") {
		t.Errorf("draft must carry 'status: draft' (human-promotion gate); got:\n%s", content)
	}
	if !strings.Contains(string(content), "directive_id: d-rewire-test") {
		t.Errorf("draft must record directive_id 'd-rewire-test'; got:\n%s", content)
	}

	// Measure run 3: another pass with no intervening fail — idempotent, the
	// existing transition's draft is skipped, no duplicate is written.
	recordVerdictLedgerIterations(root, passReport, &buf)
	if got := drafts(); len(got) != 1 {
		t.Fatalf("expected idempotent re-run to keep exactly 1 draft, got %d (%v)", len(got), got)
	}
}

// TestRecordVerdictLedgerIterations_DraftErrorDoesNotFailMeasure proves the
// compiler call is guarded: when the learnings directory cannot be created
// (parent path is a regular file), recording iterations still completes and
// surfaces only a warning — `ao goals measure` is never failed by a draft error.
func TestRecordVerdictLedgerIterations_DraftErrorDoesNotFailMeasure(t *testing.T) {
	root := t.TempDir()

	// Make docs/ a regular file so docs/learnings/ MkdirAll fails inside the compiler.
	if err := os.WriteFile(filepath.Join(root, "docs"), []byte("not a dir"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	report := []directiveScenarioReport{
		{DirectiveID: "d-guard", ScenarioVerdict: verdictledger.VerdictFail, ScenarioSatisfaction: 0.2, ScenarioCount: 3, EvaluatedCount: 3},
		{DirectiveID: "d-guard", ScenarioVerdict: verdictledger.VerdictPass, ScenarioSatisfaction: 1.0, ScenarioCount: 3, EvaluatedCount: 3},
	}

	var buf bytes.Buffer
	// First the fail, then the pass (which triggers the failing compile). Neither
	// call panics or aborts; the function returns normally.
	recordVerdictLedgerIterations(root, report[:1], &buf)
	recordVerdictLedgerIterations(root, report[1:], &buf)

	if !strings.Contains(buf.String(), "feedback compiler draft") {
		t.Errorf("expected a guarded warning about the failed draft, got stderr:\n%s", buf.String())
	}
}
