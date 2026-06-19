package main

import (
	"bytes"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func withMoatCmdReset(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		evalScenarioMoatScorecards = nil
		evalScenarioMoatOutput = ""
	})
}

func runScenarioMoatCmd(t *testing.T, scorecards []string, outPath string) (string, error) {
	t.Helper()
	cmd := evalScenarioMoatCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	t.Cleanup(func() { cmd.SetOut(nil); cmd.SetErr(nil) }) // age-ztf8: shared command; don't leak the writer
	evalScenarioMoatScorecards = scorecards
	evalScenarioMoatOutput = outPath
	err := cmd.RunE(cmd, nil)
	return buf.String(), err
}

func TestEvalScenarioMoatRejectsPlumbingScorecard(t *testing.T) {
	withMoatCmdReset(t)
	root := repoRootForEvalCmd(t)
	plumbing := filepath.Join(root, "evals/scenarios/fixtures/scenario-ab-fact-recall-plumbing.scorecard.json")
	_, err := runScenarioMoatCmd(t, []string{plumbing}, "")
	if err == nil {
		t.Fatal("expected error when aggregating moat_eligible=false scorecard")
	}
	if !strings.Contains(err.Error(), "moat_eligible=false") {
		t.Fatalf("error = %q, want moat_eligible=false rejection", err.Error())
	}
}

func TestEvalScenarioMoatPositiveFromFixture(t *testing.T) {
	withMoatCmdReset(t)
	root := repoRootForEvalCmd(t)
	valid := filepath.Join(root, "evals/scenarios/fixtures/scenario-ab-valid-redacted.scorecard.json")
	out := filepath.Join(t.TempDir(), "moat-claim.json")
	_, err := runScenarioMoatCmd(t, []string{valid}, out)
	if err != nil {
		t.Fatalf("runScenarioMoatCmd: %v", err)
	}
}

func repoRootForEvalCmd(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}
