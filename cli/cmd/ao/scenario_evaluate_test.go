// practices: [tdd, bdd-gherkin]
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/scenarioresults"
)

// evaluateGoalsMD is a GOALS.md fixture with a Gates table (one passing stub
// gate) and three directives whose linked scenarios cover the gate-shaped
// pass, judgment-shaped, and gate-shaped fail lanes.
const evaluateGoalsMD = "# Goals\n\n" +
	"Mission.\n\n" +
	"## Gates\n\n" +
	"| ID | Check | Weight | Description |\n" +
	"|----|-------|--------|-------------|\n" +
	"| stub-pass | `exit 0` | 5 | Always green stub gate |\n\n" +
	"## Directives\n\n" +
	"### 1. Gate-shaped passing directive\n\n" +
	"**Directive ID:** d-gate-pass\n" +
	"**Steer:** maintain\n" +
	"**Scenarios:** s-2026-05-01-001\n\n" +
	"### 2. Judgment-shaped directive\n\n" +
	"**Directive ID:** d-judgment\n" +
	"**Steer:** maintain\n" +
	"**Scenarios:** s-2026-05-01-002\n\n" +
	"### 3. Gate-shaped failing directive\n\n" +
	"**Directive ID:** d-gate-fail\n" +
	"**Steer:** maintain\n" +
	"**Scenarios:** s-2026-05-01-003\n"

// writeScenarioSpec writes one scenario JSON into spec/scenarios/ under root.
func writeScenarioSpec(t *testing.T, root, id, directiveID string, threshold float64, vectorsJSON string) {
	t.Helper()
	specDir := filepath.Join(root, "spec", "scenarios")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec/scenarios: %v", err)
	}
	doc := fmt.Sprintf(`{
  "id": %q,
  "directive_id": %q,
  "version": 1,
  "date": "2026-05-01",
  "goal": "fixture goal",
  "narrative": "fixture narrative",
  "expected_outcome": "fixture outcome",
  "satisfaction_threshold": %g,
  %s
  "source": "human",
  "status": "active"
}`, id, directiveID, threshold, vectorsJSON)
	if err := os.WriteFile(filepath.Join(specDir, id+".json"), []byte(doc), 0o644); err != nil {
		t.Fatalf("write scenario %s: %v", id, err)
	}
}

// setupScenarioEvaluateProject builds a temp project with GOALS.md, chdirs into
// it, and scopes the goalsFile/output globals the command reads.
func setupScenarioEvaluateProject(t *testing.T, goalsMD string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "GOALS.md"), []byte(goalsMD), 0o644); err != nil {
		t.Fatalf("write GOALS.md: %v", err)
	}
	t.Chdir(dir)
	oldGoalsFile := goalsFile
	oldOutput := output
	t.Cleanup(func() {
		goalsFile = oldGoalsFile
		output = oldOutput
	})
	goalsFile = "GOALS.md"
	return dir
}

// decodeEvaluateReport parses the --json report out of command output.
func decodeEvaluateReport(t *testing.T, out string) scenarioEvaluateReport {
	t.Helper()
	var report scenarioEvaluateReport
	dec := json.NewDecoder(strings.NewReader(out))
	if err := dec.Decode(&report); err != nil {
		t.Fatalf("decode evaluate report: %v\noutput: %s", err, out)
	}
	return report
}

// consumerReports runs the REAL consumer (`ao goals measure --scenarios-only`
// path) against root and returns its per-directive reports keyed by ID.
func consumerReports(t *testing.T, root string) map[string]directiveScenarioReport {
	t.Helper()
	var buf bytes.Buffer
	if err := runScenariosOnly("GOALS.md", root, true, &buf); err != nil {
		t.Fatalf("runScenariosOnly: %v", err)
	}
	var payload measureScenarioJSON
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("decode consumer payload: %v\noutput: %s", err, buf.String())
	}
	byID := make(map[string]directiveScenarioReport, len(payload.Directives))
	for _, d := range payload.Directives {
		byID[d.DirectiveID] = d
	}
	return byID
}

// TestScenarioEvaluate_MixedVectorsNeverCertifyPass pins the fail-open the
// cross-family pawl caught: a scenario mixing one passing mechanical vector
// with one attestation-only vector must record skip/attestation-needed, never
// a pass scored over the mechanical subset alone (incomplete evidence).
// TestScenarioEvaluate_OutOfRangeThresholdDoesNotCertify pins the pawl's
// round-3 catch: a malformed spec (threshold outside (0,1]) must never have
// its threshold silently normalized and then certify a pass.
func TestScenarioEvaluate_OutOfRangeThresholdDoesNotCertify(t *testing.T) {
	root := setupScenarioEvaluateProject(t, evaluateGoalsMD)
	writeScenarioSpec(t, root, "s-2026-05-01-001", "d-gate-pass", 1.2,
		`"acceptance_vectors": [{"dimension": "mechanical", "threshold": 1.0, "check": "exit 0"}],`)

	out, err := executeCommand("eval", "scenario", "evaluate", "--all", "--json")
	if err != nil {
		t.Fatalf("scenario evaluate failed: %v\noutput: %s", err, out)
	}
	loaded, err := scenarioresults.Load(root, true)
	if err != nil {
		t.Fatalf("production loader rejected artifact: %v", err)
	}
	for _, r := range loaded.Artifact.Results {
		if r.ScenarioID != "s-2026-05-01-001" {
			continue
		}
		if r.Verdict != scenarioresults.VerdictSkip {
			t.Fatalf("out-of-range threshold verdict = %q, want skip (never certify a malformed spec)", r.Verdict)
		}
		joined := strings.Join(r.Evidence, " | ")
		if !strings.Contains(joined, "invalid satisfaction_threshold") {
			t.Fatalf("evidence must name the invalid threshold, got: %s", joined)
		}
		return
	}
	t.Fatalf("result for s-2026-05-01-001 not written")
}

func TestScenarioEvaluate_MixedVectorsNeverCertifyPass(t *testing.T) {
	root := setupScenarioEvaluateProject(t, evaluateGoalsMD)
	writeScenarioSpec(t, root, "s-2026-05-01-001", "d-gate-pass", 1.0,
		`"acceptance_vectors": [
			{"dimension": "mechanical", "threshold": 1.0, "check": "exit 0"},
			{"dimension": "judgment-only", "threshold": 1.0}
		],`)

	out, err := executeCommand("eval", "scenario", "evaluate", "--all", "--json")
	if err != nil {
		t.Fatalf("scenario evaluate failed: %v\noutput: %s", err, out)
	}
	report := decodeEvaluateReport(t, out)
	if report.Written != 1 {
		t.Fatalf("expected 1 written result, got %d (report: %+v)", report.Written, report)
	}
	loaded, err := scenarioresults.Load(root, true)
	if err != nil {
		t.Fatalf("production loader rejected artifact: %v", err)
	}
	var got scenarioresults.ScenarioResult
	found := false
	for _, r := range loaded.Artifact.Results {
		if r.ScenarioID == "s-2026-05-01-001" {
			got, found = r, true
		}
	}
	if !found {
		t.Fatalf("mixed-vector scenario result not written")
	}
	if got.Verdict != scenarioresults.VerdictSkip {
		t.Fatalf("mixed-vector scenario verdict = %q, want skip (never pass on partial evidence)", got.Verdict)
	}
	joined := strings.Join(got.Evidence, " | ")
	if !strings.Contains(joined, "attestation-needed") || !strings.Contains(joined, "1 of 2") {
		t.Fatalf("evidence must name the unevaluated vectors, got: %s", joined)
	}
	// The consumer must count it skipped, never satisfied.
	reports := consumerReports(t, root)
	d := reports["d-gate-pass"]
	if d.EvaluatedCount != 0 || d.ScenarioVerdict != "unknown" {
		t.Fatalf("consumer counted mixed scenario as evaluated (%d/%q); want 0/unknown", d.EvaluatedCount, d.ScenarioVerdict)
	}
}

func TestScenarioEvaluate_ProducerConsumerRoundTrip(t *testing.T) {
	root := setupScenarioEvaluateProject(t, evaluateGoalsMD)
	// Gate-shaped, passing: one gate-reference check + one direct command.
	writeScenarioSpec(t, root, "s-2026-05-01-001", "d-gate-pass", 0.9,
		`"acceptance_vectors": [
			{"dimension": "gate-ref", "threshold": 0.9, "check": "gate:stub-pass"},
			{"dimension": "direct", "threshold": 0.9, "check": "exit 0"}
		],`)
	// Judgment-shaped: no mechanical check anywhere.
	writeScenarioSpec(t, root, "s-2026-05-01-002", "d-judgment", 0.8,
		`"acceptance_vectors": [{"dimension": "usability", "threshold": 0.8}],`)
	// Gate-shaped, failing.
	writeScenarioSpec(t, root, "s-2026-05-01-003", "d-gate-fail", 0.9,
		`"acceptance_vectors": [{"dimension": "correctness", "threshold": 0.9, "check": "exit 1"}],`)

	out, err := executeCommand("eval", "scenario", "evaluate", "--all", "--json")
	if err != nil {
		t.Fatalf("scenario evaluate failed: %v\noutput: %s", err, out)
	}
	report := decodeEvaluateReport(t, out)
	if report.Written != 3 {
		t.Fatalf("expected 3 written results, got %d (report: %+v)", report.Written, report)
	}
	if report.Iteration != 1 {
		t.Errorf("fresh artifact iteration = %d, want 1", report.Iteration)
	}
	if report.RunID != "ao-scenario-evaluate" {
		t.Errorf("run_id = %q, want ao-scenario-evaluate", report.RunID)
	}

	// The artifact must load through the PRODUCTION loader, strict mode.
	loaded, err := scenarioresults.Load(root, true)
	if err != nil {
		t.Fatalf("production loader rejected artifact: %v", err)
	}
	if loaded.Status != scenarioresults.StatusOK {
		t.Fatalf("artifact load status = %s, want ok", loaded.Status)
	}
	byScenario := map[string]scenarioresults.ScenarioResult{}
	for _, r := range loaded.Artifact.Results {
		byScenario[r.ScenarioID] = r
	}
	if got := byScenario["s-2026-05-01-001"]; got.Verdict != scenarioresults.VerdictPass || got.Score != 1.0 {
		t.Errorf("s-001 = verdict %q score %v, want pass 1.0", got.Verdict, got.Score)
	}
	judged := byScenario["s-2026-05-01-002"]
	if judged.Verdict != scenarioresults.VerdictSkip {
		t.Errorf("judgment-shaped verdict = %q, want skip (never a fabricated pass)", judged.Verdict)
	}
	if len(judged.Evidence) == 0 || !strings.Contains(judged.Evidence[0], "attestation-needed") {
		t.Errorf("judgment-shaped evidence missing attestation-needed marker: %v", judged.Evidence)
	}
	if got := byScenario["s-2026-05-01-003"]; got.Verdict != scenarioresults.VerdictFail || got.Score != 0.0 {
		t.Errorf("s-003 = verdict %q score %v, want fail 0.0", got.Verdict, got.Score)
	}

	// Full round trip: the real consumer/aggregator reads nonzero evaluated
	// counts back from what the producer wrote.
	reports := consumerReports(t, root)
	pass := reports["d-gate-pass"]
	if pass.EvaluatedCount != 1 || pass.ScenarioVerdict != "pass" || pass.ScenarioSatisfaction != 1.0 {
		t.Errorf("d-gate-pass = evaluated %d verdict %q satisfaction %v, want 1/pass/1.0",
			pass.EvaluatedCount, pass.ScenarioVerdict, pass.ScenarioSatisfaction)
	}
	fail := reports["d-gate-fail"]
	if fail.EvaluatedCount != 1 || fail.ScenarioVerdict != "fail" {
		t.Errorf("d-gate-fail = evaluated %d verdict %q, want 1/fail", fail.EvaluatedCount, fail.ScenarioVerdict)
	}
	judgment := reports["d-judgment"]
	if judgment.EvaluatedCount != 0 || judgment.ScenarioVerdict != "unknown" {
		t.Errorf("d-judgment = evaluated %d verdict %q, want 0/unknown (attestation lane stays honest)",
			judgment.EvaluatedCount, judgment.ScenarioVerdict)
	}
}

func TestScenarioEvaluate_TimeoutYieldsSkipNeverFail(t *testing.T) {
	root := setupScenarioEvaluateProject(t, evaluateGoalsMD)
	writeScenarioSpec(t, root, "s-2026-05-01-001", "d-gate-pass", 0.9,
		`"acceptance_vectors": [{"dimension": "slow", "threshold": 0.9, "check": "sleep 5"}],`)

	out, err := executeCommand("eval", "scenario", "evaluate",
		"--directive", "d-gate-pass", "--timeout", "100ms", "--json")
	if err != nil {
		t.Fatalf("scenario evaluate failed: %v\noutput: %s", err, out)
	}
	report := decodeEvaluateReport(t, out)
	if report.Written != 1 {
		t.Fatalf("expected 1 written result, got %d", report.Written)
	}
	loaded, err := scenarioresults.Load(root, true)
	if err != nil || loaded.Artifact == nil {
		t.Fatalf("load artifact: %v", err)
	}
	got := loaded.Artifact.Results[0]
	if got.Verdict != scenarioresults.VerdictSkip {
		t.Fatalf("timed-out check verdict = %q, want skip — a check that never ran must not become pass or fail", got.Verdict)
	}

	// Downstream, a skip stays non-evidence: evaluated 0, verdict unknown.
	reports := consumerReports(t, root)
	if d := reports["d-gate-pass"]; d.EvaluatedCount != 0 || d.ScenarioVerdict != "unknown" {
		t.Errorf("consumer after timeout = evaluated %d verdict %q, want 0/unknown", d.EvaluatedCount, d.ScenarioVerdict)
	}
}

func TestScenarioEvaluate_UnresolvableGateRefYieldsSkip(t *testing.T) {
	root := setupScenarioEvaluateProject(t, evaluateGoalsMD)
	writeScenarioSpec(t, root, "s-2026-05-01-001", "d-gate-pass", 0.9,
		`"acceptance_vectors": [{"dimension": "ghost", "threshold": 0.9, "check": "gate:does-not-exist"}],`)

	out, err := executeCommand("eval", "scenario", "evaluate", "--all", "--json")
	if err != nil {
		t.Fatalf("scenario evaluate failed: %v\noutput: %s", err, out)
	}
	loaded, err := scenarioresults.Load(root, true)
	if err != nil || loaded.Artifact == nil {
		t.Fatalf("load artifact: %v", err)
	}
	got := loaded.Artifact.Results[0]
	if got.Verdict != scenarioresults.VerdictSkip {
		t.Fatalf("unresolvable gate ref verdict = %q, want skip", got.Verdict)
	}
	if len(got.Evidence) == 0 || !strings.Contains(got.Evidence[0], "unresolvable gate reference") {
		t.Errorf("expected unresolvable-gate evidence, got %v", got.Evidence)
	}
}

func TestScenarioEvaluate_RerunSupersedesLatestPerScenario(t *testing.T) {
	root := setupScenarioEvaluateProject(t, evaluateGoalsMD)

	// A fake check command on PATH that flips from failing to passing between
	// runs — the supersede case is a real state change, not a replay.
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	fake := filepath.Join(binDir, "fakecheck")
	writeFake := func(exitCode int) {
		script := fmt.Sprintf("#!/bin/sh\nexit %d\n", exitCode)
		if err := os.WriteFile(fake, []byte(script), 0o755); err != nil { // #nosec G306 -- test fixture executable
			t.Fatalf("write fakecheck: %v", err)
		}
	}
	writeFake(1)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	writeScenarioSpec(t, root, "s-2026-05-01-001", "d-gate-pass", 0.9,
		`"acceptance_vectors": [{"dimension": "flip", "threshold": 0.9, "check": "fakecheck"}],`)

	if _, err := executeCommand("eval", "scenario", "evaluate", "--directive", "d-gate-pass"); err != nil {
		t.Fatalf("first evaluate: %v", err)
	}
	loaded, err := scenarioresults.Load(root, true)
	if err != nil || loaded.Artifact == nil {
		t.Fatalf("load after first run: %v", err)
	}
	if got := loaded.Artifact.Results[0].Verdict; got != scenarioresults.VerdictFail {
		t.Fatalf("first run verdict = %q, want fail", got)
	}

	writeFake(0)
	out, err := executeCommand("eval", "scenario", "evaluate", "--directive", "d-gate-pass", "--json")
	if err != nil {
		t.Fatalf("second evaluate: %v", err)
	}
	report := decodeEvaluateReport(t, out)
	if report.Iteration != 2 {
		t.Errorf("second run iteration = %d, want 2", report.Iteration)
	}

	loaded, err = scenarioresults.Load(root, true)
	if err != nil || loaded.Artifact == nil {
		t.Fatalf("load after second run: %v", err)
	}
	if n := len(loaded.Artifact.Results); n != 1 {
		t.Fatalf("expected exactly 1 merged result for the scenario, got %d", n)
	}
	if got := loaded.Artifact.Results[0].Verdict; got != scenarioresults.VerdictPass {
		t.Errorf("superseded verdict = %q, want pass (latest wins)", got)
	}
}

func TestScenarioEvaluate_MissingScenarioWritesNothing(t *testing.T) {
	root := setupScenarioEvaluateProject(t, evaluateGoalsMD)
	// No spec files at all: every link is missing.
	out, err := executeCommand("eval", "scenario", "evaluate", "--all", "--json")
	if err != nil {
		t.Fatalf("scenario evaluate failed: %v\noutput: %s", err, out)
	}
	report := decodeEvaluateReport(t, out)
	if report.Written != 0 {
		t.Fatalf("expected 0 written results for missing specs, got %d", report.Written)
	}
	for _, ev := range report.Evaluations {
		if ev.Recorded {
			t.Errorf("missing scenario %s reported as recorded", ev.ScenarioID)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".agents", "rpi", "scenario-results.json")); !os.IsNotExist(err) {
		t.Fatalf("artifact must not be created when nothing was evaluated (stat err: %v)", err)
	}
	// Downstream stays fully unknown: zero evidence was never turned into a verdict.
	reports := consumerReports(t, root)
	if d := reports["d-gate-pass"]; d.ScenarioVerdict != "unknown" {
		t.Errorf("consumer verdict for unwritten scenario = %q, want unknown", d.ScenarioVerdict)
	}
}

func TestScenarioEvaluate_PartialScoreJudgedAgainstScenarioThreshold(t *testing.T) {
	root := setupScenarioEvaluateProject(t, evaluateGoalsMD)
	// 2 checks, 1 passes -> score 0.5. Threshold 0.5 -> pass (equality passes),
	// matching the aggregator's countSatisfied comparison exactly.
	writeScenarioSpec(t, root, "s-2026-05-01-001", "d-gate-pass", 0.5,
		`"acceptance_vectors": [
			{"dimension": "green", "threshold": 0.5, "check": "exit 0"},
			{"dimension": "red", "threshold": 0.5, "check": "exit 1"}
		],`)

	out, err := executeCommand("eval", "scenario", "evaluate", "--directive", "d-gate-pass", "--json")
	if err != nil {
		t.Fatalf("scenario evaluate failed: %v\noutput: %s", err, out)
	}
	loaded, err := scenarioresults.Load(root, true)
	if err != nil || loaded.Artifact == nil {
		t.Fatalf("load artifact: %v", err)
	}
	got := loaded.Artifact.Results[0]
	if got.Verdict != scenarioresults.VerdictPass || got.Score != 0.5 {
		t.Fatalf("partial result = verdict %q score %v, want pass 0.5", got.Verdict, got.Score)
	}
	// The consumer agrees: evaluated 1, satisfied at its own threshold.
	reports := consumerReports(t, root)
	if d := reports["d-gate-pass"]; d.EvaluatedCount != 1 || d.ScenarioVerdict != "pass" {
		t.Errorf("consumer = evaluated %d verdict %q, want 1/pass", d.EvaluatedCount, d.ScenarioVerdict)
	}
}

func TestScenarioEvaluate_RequiresScopeFlag(t *testing.T) {
	setupScenarioEvaluateProject(t, evaluateGoalsMD)
	_, err := executeCommand("eval", "scenario", "evaluate")
	if err == nil || !strings.Contains(err.Error(), "--all") {
		t.Fatalf("expected scope-flag error mentioning --all, got %v", err)
	}
}
