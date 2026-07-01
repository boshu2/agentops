// practices: [bdd-gherkin, llm-eval-harness]
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/boshu2/agentops/cli/internal/goalsfitness"
	"github.com/boshu2/agentops/cli/internal/scenario"
	"github.com/boshu2/agentops/cli/internal/scenarioresults"
	"github.com/spf13/cobra"
)

// gateCheckPrefix marks an acceptance-vector check that references a GOALS.md
// Gates-table entry by ID (e.g. "gate:go-cli-builds") instead of carrying a
// literal shell command. The referenced gate's Check column is executed.
const gateCheckPrefix = "gate:"

// scenarioShape classifies how a scenario can be evaluated.
type scenarioShape string

const (
	// shapeGate means the scenario carries >=1 acceptance vector with a
	// mechanical check command — it is evaluated by running those commands.
	shapeGate scenarioShape = "gate"
	// shapeJudgment means the scenario has no mechanical check — it cannot be
	// evaluated by this command and is recorded as skip/attestation-needed.
	shapeJudgment scenarioShape = "judgment"
)

// scenarioEvaluateNow is the injectable clock for deterministic judged_at /
// generated_at timestamps in tests.
var scenarioEvaluateNow = time.Now

// scenarioEvaluateFlags holds the evaluate subcommand's flag values.
var scenarioEvaluateFlags = struct {
	DirectiveID string
	All         bool
	JSON        bool
	Timeout     time.Duration
	RunID       string
}{
	Timeout: 2 * time.Minute,
	RunID:   "ao-scenario-evaluate",
}

// scenarioEvaluation is the per-scenario outcome reported by the command. It
// mirrors the persisted scenarioresults.ScenarioResult plus the shape and any
// non-persisted skip reason (missing/retired links write no result).
type scenarioEvaluation struct {
	ScenarioID  string   `json:"scenario_id"`
	DirectiveID string   `json:"directive_id"`
	Shape       string   `json:"shape,omitempty"`
	Verdict     string   `json:"verdict,omitempty"`
	Score       float64  `json:"score"`
	Threshold   float64  `json:"threshold"`
	Evidence    []string `json:"evidence,omitempty"`
	// Recorded is false when no result was written (missing spec file, retired
	// scenario, or an unwritable directive ID) — the artifact stays untouched
	// so the downstream aggregator keeps reporting "missing", never a verdict.
	Recorded bool   `json:"recorded"`
	Note     string `json:"note,omitempty"`
}

// scenarioEvaluateReport is the full command output.
type scenarioEvaluateReport struct {
	RunID       string               `json:"run_id"`
	Iteration   int                  `json:"iteration"`
	Artifact    string               `json:"artifact"`
	Written     int                  `json:"written"`
	Evaluations []scenarioEvaluation `json:"evaluations"`
}

var scenarioEvaluateCmd = &cobra.Command{
	Use:   "evaluate",
	Short: "Evaluate directive-linked scenarios and record satisfaction results",
	Long: `Evaluate the executable-spec scenarios linked to GOALS.md directives and
append their verdicts to ` + scenarioresults.ArtifactRelPath + ` — the producer
side of the 'ao goals measure --scenarios-only' satisfaction panel.

A scenario is GATE-SHAPED when it declares acceptance_vectors carrying a
mechanical "check" command. Each check runs under a per-check timeout; a check
of the form "gate:<id>" resolves through the GOALS.md Gates table and runs
that gate's Check command. The scenario's score is the fraction of checks
that passed, and its verdict is pass/fail against its own
satisfaction_threshold. A check that could not run (timeout, skip exit 77,
unresolvable gate reference) yields verdict "skip" — never a fabricated pass
or fail.

A scenario with no mechanical check is JUDGMENT-SHAPED: it is recorded as
verdict "skip" with attestation-needed evidence, because this command cannot
honestly evaluate it. Missing or retired scenarios write no result at all, so
the downstream aggregator keeps reporting them as missing.

  ao eval scenario evaluate --all                 evaluate every directive's scenarios
  ao eval scenario evaluate --directive d-foo     evaluate one directive's scenarios
  ao eval scenario evaluate --all --json          machine-readable report`,
	Args: cobra.NoArgs,
	RunE: runScenarioEvaluate,
}

// runScenarioEvaluate drives the evaluate flow: resolve directives, evaluate
// each linked scenario, and append the results via the production writer.
func runScenarioEvaluate(cmd *cobra.Command, _ []string) error {
	if !scenarioEvaluateFlags.All && scenarioEvaluateFlags.DirectiveID == "" {
		return fmt.Errorf("choose a scope: --all for every directive, or --directive <stable-id>")
	}

	projectRoot := measureProjectRoot()
	goalsPath := resolveGoalsFile()
	patcher, _, err := goals.LoadGoalsPatcher(goalsPath)
	if err != nil {
		return fmt.Errorf("loading directives: %w", err)
	}
	directives := goals.FilterDirectives(patcher.Directives(), 0, scenarioEvaluateFlags.DirectiveID)
	if len(directives) == 0 {
		return fmt.Errorf("no directive matches --directive %q", scenarioEvaluateFlags.DirectiveID)
	}

	gates, err := loadGatesByID(goalsPath)
	if err != nil {
		return fmt.Errorf("loading gates table: %w", err)
	}

	evaluations := evaluateDirectiveLinks(directives, gates)
	report, err := appendScenarioResults(projectRoot, evaluations)
	if err != nil {
		return err
	}
	return emitScenarioEvaluateReport(cmd.OutOrStdout(), report)
}

// evaluateDirectiveLinks evaluates every scenario linked from directives.
func evaluateDirectiveLinks(directives []goals.ParsedDirective, gates map[string]string) []scenarioEvaluation {
	var evaluations []scenarioEvaluation
	dirs := goals.DefaultScenarioDirs()
	for _, d := range directives {
		for _, sid := range d.Scenarios {
			evaluations = append(evaluations, evaluateLinkedScenario(d, sid, dirs, gates))
		}
	}
	return evaluations
}

// evaluateLinkedScenario evaluates one directive→scenario link. Missing,
// unreadable, or retired scenarios produce an unrecorded evaluation (no
// artifact write) so "could not evaluate" is never converted into a verdict.
func evaluateLinkedScenario(d goals.ParsedDirective, sid string, dirs []string, gates map[string]string) scenarioEvaluation {
	ev := scenarioEvaluation{ScenarioID: sid, DirectiveID: d.StableID}
	spec, err := loadScenarioSpec(dirs, sid)
	switch {
	case err != nil:
		ev.Note = fmt.Sprintf("unreadable scenario spec: %v", err)
		return ev
	case spec == nil:
		ev.Note = "no scenario file found on the search path"
		return ev
	case spec.Status == "retired":
		ev.Note = "scenario is retired; not evaluated"
		return ev
	}
	if ev.DirectiveID == "" {
		ev.DirectiveID = spec.DirectiveID
	}
	if !scenarioresults.ValidDirectiveID(ev.DirectiveID) || !scenarioresults.ValidScenarioID(sid) {
		ev.Note = "no valid stable directive/scenario ID; result would not validate"
		return ev
	}

	if spec.SatisfactionThreshold < 0 || spec.SatisfactionThreshold > 1 {
		// An out-of-range threshold is a malformed spec. Silently normalizing it
		// and then certifying a pass would fabricate satisfaction for a contract
		// nobody wrote; record skip with the defect named instead.
		ev.Shape = string(shapeJudgment)
		ev.Threshold = normalizeScenarioThreshold(spec.SatisfactionThreshold)
		ev.Verdict = scenarioresults.VerdictSkip
		ev.Evidence = []string{fmt.Sprintf(
			"invalid satisfaction_threshold %g (must be in (0,1]); spec must be fixed before this scenario can certify",
			spec.SatisfactionThreshold)}
		ev.Recorded = true
		return ev
	}

	ev.Threshold = normalizeScenarioThreshold(spec.SatisfactionThreshold)
	checks := mechanicalChecks(spec)
	if len(checks) == 0 {
		ev.Shape = string(shapeJudgment)
		ev.Verdict = scenarioresults.VerdictSkip
		ev.Evidence = []string{"attestation-needed: no mechanical acceptance-vector check; requires judge/human attestation"}
		ev.Recorded = true
		return ev
	}

	if len(checks) < len(spec.AcceptanceVectors) {
		// Mixed shape: some vectors are mechanical, some need attestation. A pass
		// scored over the mechanical subset alone would certify the scenario with
		// incomplete evidence (fail-open). Run the mechanical checks for their
		// evidence value, but the recorded verdict stays skip/attestation-needed.
		ev.Shape = string(shapeJudgment)
		_, score, evidence := runScenarioChecks(checks, gates, ev.Threshold)
		ev.Score = score
		unchecked := len(spec.AcceptanceVectors) - len(checks)
		ev.Evidence = append([]string{fmt.Sprintf(
			"attestation-needed: %d of %d acceptance vectors carry no mechanical check; mechanical subset ran for evidence only",
			unchecked, len(spec.AcceptanceVectors))}, evidence...)
		ev.Verdict = scenarioresults.VerdictSkip
		ev.Recorded = true
		return ev
	}

	ev.Shape = string(shapeGate)
	ev.Verdict, ev.Score, ev.Evidence = runScenarioChecks(checks, gates, ev.Threshold)
	ev.Recorded = true
	return ev
}

// mechanicalChecks returns the scenario's acceptance vectors that carry a
// non-empty mechanical check command.
func mechanicalChecks(spec *scenario.Scenario) []scenario.AcceptanceVector {
	var out []scenario.AcceptanceVector
	for _, v := range spec.AcceptanceVectors {
		if strings.TrimSpace(v.Check) != "" {
			out = append(out, v)
		}
	}
	return out
}

// runScenarioChecks executes each mechanical check and folds the per-check
// outcomes into a scenario verdict, score, and evidence trail.
//
// Score is the fraction of checks that passed. The verdict is pass/fail by
// score vs threshold ONLY when every check produced real pass/fail evidence;
// if any check could not run (timeout, exit 77 skip, unresolvable gate
// reference) the scenario verdict is "skip" — incomplete evidence is never
// laundered into a pass or a fail.
func runScenarioChecks(checks []scenario.AcceptanceVector, gates map[string]string, threshold float64) (verdict string, score float64, evidence []string) {
	passed, skipped := 0, 0
	for _, v := range checks {
		outcome, detail := runOneCheck(v, gates)
		switch outcome {
		case resultPassOutcome:
			passed++
		case resultSkipOutcome:
			skipped++
		}
		evidence = append(evidence, fmt.Sprintf("%s: %s — %s", v.Dimension, outcome, detail))
	}
	score = float64(passed) / float64(len(checks))
	if skipped > 0 {
		return scenarioresults.VerdictSkip, score, evidence
	}
	return verdictForScore(score, threshold), score, evidence
}

// Per-check outcome labels (aligned with goals.Measurement.Result values).
const (
	resultPassOutcome = "pass"
	resultFailOutcome = "fail"
	resultSkipOutcome = "skip"
)

// runOneCheck resolves and executes a single acceptance-vector check, returning
// its pass/fail/skip outcome and a human-readable evidence detail.
func runOneCheck(v scenario.AcceptanceVector, gates map[string]string) (outcome, detail string) {
	command := strings.TrimSpace(v.Check)
	if gateID, ok := strings.CutPrefix(command, gateCheckPrefix); ok {
		resolved, found := gates[strings.TrimSpace(gateID)]
		if !found {
			return resultSkipOutcome, fmt.Sprintf("unresolvable gate reference %q (not in the GOALS.md Gates table)", command)
		}
		command = resolved
	}

	m := goals.MeasureOne(goals.Goal{ID: "scenario-check", Check: command}, scenarioEvaluateFlags.Timeout)
	detail = fmt.Sprintf("`%s` -> %s", command, m.Result)
	if m.Result != resultPassOutcome && m.Output != "" {
		detail += ": " + m.Output
	}
	return m.Result, detail
}

// verdictForScore maps a completed-evidence score onto pass/fail against the
// scenario's own satisfaction threshold (equality passes). It is only called
// when every check produced pass/fail evidence, and it deliberately matches
// the goalsfitness countSatisfied comparison (score >= threshold) so a
// persisted verdict never disagrees with the aggregator's satisfied count.
func verdictForScore(score, threshold float64) string {
	if score >= threshold {
		return scenarioresults.VerdictPass
	}
	return scenarioresults.VerdictFail
}

// normalizeScenarioThreshold clamps an absent or out-of-range spec threshold to
// the directive-layer default so a persisted result always validates and a
// zero threshold can never make an unevaluated scenario count as satisfied.
func normalizeScenarioThreshold(t float64) float64 {
	if t <= 0 || t > 1 {
		return goalsfitness.DefaultScenarioThreshold
	}
	return t
}

// appendScenarioResults persists the recorded evaluations through the
// production scenarioresults.Writer and assembles the command report. When
// nothing was recorded the artifact is left untouched (no empty-run bump).
func appendScenarioResults(projectRoot string, evaluations []scenarioEvaluation) (*scenarioEvaluateReport, error) {
	report := &scenarioEvaluateReport{
		RunID:       scenarioEvaluateFlags.RunID,
		Artifact:    scenarioresults.ArtifactRelPath,
		Evaluations: evaluations,
	}
	if report.Evaluations == nil {
		report.Evaluations = []scenarioEvaluation{}
	}

	results := persistableResults(evaluations)
	report.Written = len(results)
	if len(results) == 0 {
		return report, nil
	}

	report.Iteration = nextIteration(projectRoot)
	writer := scenarioresults.Writer{Now: scenarioEvaluateNow}
	if _, err := writer.Append(projectRoot, report.RunID, report.Iteration, results); err != nil {
		return nil, fmt.Errorf("writing scenario results: %w", err)
	}
	return report, nil
}

// persistableResults converts recorded evaluations into contract results.
func persistableResults(evaluations []scenarioEvaluation) []scenarioresults.ScenarioResult {
	judgedAt := scenarioEvaluateNow().UTC().Format(time.RFC3339)
	var results []scenarioresults.ScenarioResult
	for _, ev := range evaluations {
		if !ev.Recorded {
			continue
		}
		results = append(results, scenarioresults.ScenarioResult{
			ScenarioID:  ev.ScenarioID,
			DirectiveID: ev.DirectiveID,
			Score:       ev.Score,
			Threshold:   ev.Threshold,
			Verdict:     ev.Verdict,
			JudgedAt:    judgedAt,
			Evidence:    ev.Evidence,
		})
	}
	return results
}

// nextIteration returns the prior artifact's iteration + 1 (1 for a fresh
// artifact). A malformed artifact yields 1 here; the Writer itself still
// refuses to clobber a malformed artifact, so no data is lost.
func nextIteration(projectRoot string) int {
	loaded, err := scenarioresults.Load(projectRoot, false)
	if err == nil && loaded.Artifact != nil {
		return loaded.Artifact.Iteration + 1
	}
	return 1
}

// loadScenarioSpec reads the full scenario JSON for id from the ordered search
// dirs. A nil spec with nil error means "not found".
func loadScenarioSpec(dirs []string, id string) (*scenario.Scenario, error) {
	for _, dir := range dirs {
		path := filepath.Join(dir, id+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		var spec scenario.Scenario
		if err := json.Unmarshal(data, &spec); err != nil {
			return nil, fmt.Errorf("%s: invalid JSON: %w", path, err)
		}
		return &spec, nil
	}
	return nil, nil
}

// loadGatesByID parses the GOALS.md Gates table into an ID→Check map for
// resolving "gate:<id>" acceptance-vector checks.
func loadGatesByID(goalsPath string) (map[string]string, error) {
	data, err := os.ReadFile(goalsPath)
	if err != nil {
		return nil, err
	}
	gf, err := goals.ParseMarkdownGoals(data)
	if err != nil {
		return nil, err
	}
	gates := make(map[string]string, len(gf.Goals))
	for _, g := range gf.Goals {
		gates[g.ID] = g.Check
	}
	return gates, nil
}

// emitScenarioEvaluateReport renders the report as JSON or a human summary.
func emitScenarioEvaluateReport(w io.Writer, report *scenarioEvaluateReport) error {
	if scenarioEvaluateFlags.JSON || GetOutput() == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	for _, ev := range report.Evaluations {
		if !ev.Recorded {
			fmt.Fprintf(w, "%-20s %-10s not recorded — %s\n", ev.ScenarioID, "-", ev.Note)
			continue
		}
		fmt.Fprintf(w, "%-20s %-10s shape=%s score=%.2f threshold=%.2f\n",
			ev.ScenarioID, ev.Verdict, ev.Shape, ev.Score, ev.Threshold)
	}
	if report.Written == 0 {
		fmt.Fprintln(w, "No results recorded; artifact untouched.")
		return nil
	}
	fmt.Fprintf(w, "Wrote %d result(s) to %s (run %s, iteration %d)\n",
		report.Written, report.Artifact, report.RunID, report.Iteration)
	return nil
}

func init() {
	scenarioEvaluateCmd.Flags().StringVar(&scenarioEvaluateFlags.DirectiveID, "directive", "", "Evaluate only the directive with this stable Directive ID")
	scenarioEvaluateCmd.Flags().BoolVar(&scenarioEvaluateFlags.All, "all", false, "Evaluate every directive's linked scenarios")
	scenarioEvaluateCmd.Flags().BoolVar(&scenarioEvaluateFlags.JSON, "json", false, "Emit the machine-readable evaluation report")
	scenarioEvaluateCmd.Flags().DurationVar(&scenarioEvaluateFlags.Timeout, "timeout", 2*time.Minute, "Per-check execution timeout")
	scenarioEvaluateCmd.Flags().StringVar(&scenarioEvaluateFlags.RunID, "run-id", "ao-scenario-evaluate", "run_id recorded in the results artifact")
	scenarioCmd.AddCommand(scenarioEvaluateCmd)
}
