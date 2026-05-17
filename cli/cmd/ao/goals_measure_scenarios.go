// practices: [dora-metrics, lean-startup, bdd-gherkin]
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/boshu2/agentops/cli/internal/goalsfitness"
	"github.com/boshu2/agentops/cli/internal/verdictledger"
)

// directiveScenarioReport is the per-directive scenario-satisfaction record
// added to `ao goals measure` output. It is ADDITIVE: it sits alongside the
// existing Steer/Gates measurement, never replacing it. A directive can be red
// on either signal (a failing gate, a failing scenario verdict, or both).
type directiveScenarioReport struct {
	DirectiveID          string   `json:"directive_id"`
	DirectiveNumber      int      `json:"directive_number"`
	ScenarioCount        int      `json:"scenario_count"`
	EvaluatedCount       int      `json:"evaluated_count"`
	MissingCount         int      `json:"missing_count"`
	ScenarioSatisfaction float64  `json:"scenario_satisfaction"`
	ScenarioThreshold    float64  `json:"scenario_threshold"`
	ScenarioVerdict      string   `json:"scenario_verdict"`
	Contributing         []string `json:"contributing"`
	Warning              string   `json:"warning,omitempty"`
}

// measureScenarioJSON is the combined JSON payload emitted by `ao goals
// measure -o json` once scenario satisfaction is wired in. The snapshot is the
// pre-existing `goals.Snapshot` shape (unchanged); the new top-level keys are
// purely additive so existing snapshot consumers keep working.
type measureScenarioJSON struct {
	// Mode records "scenarios-only" when --scenarios-only was used, and
	// "full" for the default gate+scenario run. Snapshot/metadata consumers
	// read this to know whether shell gate commands were executed.
	Mode string `json:"mode"`
	// Snapshot is the gate-measurement snapshot. It is omitted under
	// --scenarios-only because no gate commands run in that mode.
	Snapshot *goals.Snapshot `json:"snapshot,omitempty"`
	// Directives is the per-directive scenario-satisfaction roll-up.
	Directives []directiveScenarioReport `json:"directives"`
}

const (
	measureModeFull          = "full"
	measureModeScenariosOnly = "scenarios-only"
)

// evaluateDirectiveScenarios builds a per-directive scenario-satisfaction
// report for every directive in GOALS.md.
//
// It parses GOALS.md via the non-lossy patcher (the canonical directive
// reader), constructs a goalsfitness.DirectiveLink per directive, parses each
// directive's scenario threshold (default goalsfitness.DefaultScenarioThreshold
// == 0.8), and calls goalsfitness.EvaluateSatisfaction.
//
// A malformed scenario threshold in GOALS.md is a structurally-invalid input:
// it returns an error so a bad spec does not silently degrade a directive's
// gate. A missing scenario-results artifact is NOT an error — the aggregator
// yields an "unknown" verdict for every directive (clean skip, never a pass).
func evaluateDirectiveScenarios(goalsFile, projectRoot string) ([]directiveScenarioReport, error) {
	patcher, _, err := goals.LoadGoalsPatcher(goalsFile)
	if err != nil {
		return nil, fmt.Errorf("loading directives: %w", err)
	}
	agg, err := goalsfitness.NewAggregator(projectRoot, false)
	if err != nil {
		return nil, fmt.Errorf("loading scenario results: %w", err)
	}

	directives := patcher.Directives()
	reports := make([]directiveScenarioReport, 0, len(directives))
	for _, d := range directives {
		threshold, err := goalsfitness.ParseScenarioThreshold(d.ScenarioThreshold)
		if err != nil {
			return nil, fmt.Errorf("directive #%d %q: %w", d.Number, d.Title, err)
		}
		reports = append(reports, buildDirectiveScenarioReport(agg, d, threshold))
	}
	return reports, nil
}

// buildDirectiveScenarioReport evaluates one directive's scenario satisfaction.
// It calls Aggregate for the contributing scenario IDs (which feed Score) and
// EvaluateSatisfaction for the durable verdict and satisfaction fraction.
func buildDirectiveScenarioReport(agg *goalsfitness.Aggregator, d goals.ParsedDirective, threshold float64) directiveScenarioReport {
	link := goalsfitness.DirectiveLink{
		DirectiveID: d.StableID,
		ScenarioIDs: d.Scenarios,
	}
	aggregation := agg.Aggregate(link)
	sat := agg.EvaluateSatisfaction(link, threshold)

	contributing := aggregation.Contributing
	if contributing == nil {
		contributing = []string{}
	}
	return directiveScenarioReport{
		DirectiveID:          d.StableID,
		DirectiveNumber:      d.Number,
		ScenarioCount:        sat.Linked,
		EvaluatedCount:       sat.Evaluated,
		MissingCount:         sat.Missing,
		ScenarioSatisfaction: sat.Satisfaction,
		ScenarioThreshold:    sat.Threshold,
		ScenarioVerdict:      string(sat.Verdict),
		Contributing:         contributing,
		Warning:              sat.Warning,
	}
}

// runScenariosOnly evaluates ONLY the executable-spec scenario results and
// SKIPS shell gate-command execution entirely. It never calls goals.RunMeasure
// (and therefore never spawns a gate subprocess), so it is safe to run in
// environments where gate commands are slow, unavailable, or undesired.
//
// Exit-code semantics (kept consistent with `ao goals measure`):
//
//	`ao goals measure` returns nil — and the process exits 0 — even when gate
//	checks fail; only a structurally-invalid input (unloadable GOALS.md, a
//	failed validation, a malformed scenario threshold) makes it return an
//	error and exit non-zero. --scenarios-only follows the SAME contract: a
//	failing scenario verdict is a measurement outcome, not an invocation
//	error, so it exits 0; a structurally-invalid input exits non-zero. The two
//	classes are deliberately distinct — callers gate on the JSON
//	scenario_verdict field for red/green, and on the exit code for "could the
//	measurement even run".
func runScenariosOnly(goalsFile, projectRoot string, asJSON bool, stdout io.Writer) error {
	reports, err := evaluateDirectiveScenarios(goalsFile, projectRoot)
	if err != nil {
		return err
	}
	recordVerdictLedgerIterations(projectRoot, reports, os.Stderr)
	if asJSON {
		return emitMeasureScenarioJSON(stdout, measureModeScenariosOnly, nil, reports)
	}
	renderScenarioReports(stdout, measureModeScenariosOnly, reports)
	return nil
}

// emitMeasureScenarioJSON writes the combined measure+scenario JSON payload.
func emitMeasureScenarioJSON(w io.Writer, mode string, snap *goals.Snapshot, reports []directiveScenarioReport) error {
	payload := measureScenarioJSON{
		Mode:       mode,
		Snapshot:   snap,
		Directives: reports,
	}
	if payload.Directives == nil {
		payload.Directives = []directiveScenarioReport{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

// renderScenarioReports prints the human-readable per-directive scenario table.
// It is appended below the existing gate table in full mode, or printed alone
// under --scenarios-only.
func renderScenarioReports(w io.Writer, mode string, reports []directiveScenarioReport) {
	fmt.Fprintf(w, "\nScenario satisfaction (mode: %s)\n", mode)
	fmt.Fprintf(w, "%-22s  %-8s  %10s  %9s  %s\n", "DIRECTIVE", "VERDICT", "SATISFIED", "THRESHOLD", "SCENARIOS")
	fmt.Fprintf(w, "%-22s  %-8s  %10s  %9s  %s\n", "----------------------", "--------", "----------", "---------", "---------")
	for _, r := range reports {
		id := r.DirectiveID
		if id == "" {
			id = fmt.Sprintf("#%d", r.DirectiveNumber)
		}
		fmt.Fprintf(w, "%-22s  %-8s  %9.0f%%  %8.0f%%  %d/%d (eval %d, missing %d)\n",
			id, r.ScenarioVerdict, r.ScenarioSatisfaction*100, r.ScenarioThreshold*100,
			r.EvaluatedCount, r.ScenarioCount, r.EvaluatedCount, r.MissingCount)
	}
}

// recordVerdictLedgerIterations appends one verdict-ledger iteration record
// per directive for a completed `ao goals measure` run (F5.1 producer hookup
// for the F5.2 re-steer policy engine).
//
// Per ADR-0006 §ITERATION an iteration is one completed measure run that
// records a scenario_verdict for the directive. This function is called only
// after evaluateDirectiveScenarios succeeds — a structurally-failed run
// returns before reaching here, so no record is written for it. Every
// directive that produced a report (including "unknown"-verdict directives
// with no linked scenarios) is recorded: "unknown" is a valid iteration
// outcome that breaks a failure streak.
//
// The append is purely additive: it never changes measure stdout. A write
// failure is logged to stderr and swallowed so a ledger I/O problem cannot
// turn a successful measurement into a non-zero exit. Directives without a
// stable d- ID (which the verdict ledger keys on) are skipped.
func recordVerdictLedgerIterations(projectRoot string, reports []directiveScenarioReport, stderr io.Writer) {
	writer := verdictledger.Writer{}
	runTime := time.Now().UTC()
	for _, r := range reports {
		if !verdictledger.ValidDirectiveID(r.DirectiveID) {
			continue
		}
		_, err := writer.AppendIteration(projectRoot, verdictledger.IterationInput{
			DirectiveID:          r.DirectiveID,
			RunTime:              runTime,
			ScenarioVerdict:      r.ScenarioVerdict,
			ScenarioSatisfaction: r.ScenarioSatisfaction,
			ScenarioCount:        r.ScenarioCount,
			EvaluatedCount:       r.EvaluatedCount,
		})
		if err != nil && stderr != nil {
			fmt.Fprintf(stderr, "warning: verdict ledger append for %s: %v\n", r.DirectiveID, err)
		}
	}
}

// measureProjectRoot returns the project root the scenario-results artifact is
// resolved against. CLI invocations run from the project root, matching the
// convention used by `ao goals trace`.
func measureProjectRoot() string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}
