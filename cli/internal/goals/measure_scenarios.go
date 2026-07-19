package goals

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/boshu2/agentops/cli/internal/goalsfitness"
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
// pre-existing Snapshot shape (unchanged); the new top-level keys are purely
// additive so existing snapshot consumers keep working.
type measureScenarioJSON struct {
	// Mode records "scenarios-only" when --scenarios-only was used, and "full"
	// for the default gate+scenario run. Snapshot/metadata consumers read this
	// to know whether shell gate commands were executed.
	Mode string `json:"mode"`
	// Snapshot is the gate-measurement snapshot. It is omitted under
	// --scenarios-only because no gate commands run in that mode.
	Snapshot *Snapshot `json:"snapshot,omitempty"`
	// Directives is the per-directive scenario-satisfaction roll-up.
	Directives []directiveScenarioReport `json:"directives"`
}

const (
	measureModeFull          = "full"
	measureModeScenariosOnly = "scenarios-only"
)

// MeasureScenariosOptions configures RunMeasureScenarios. It carries the gate
// measurement inputs plus the scenario-satisfaction project root and the
// presentation choices resolved by the command module.
type MeasureScenariosOptions struct {
	GoalsFile     string
	ProjectRoot   string
	GoalID        string
	ExcludeTag    string
	Directives    bool
	Timeout       time.Duration
	TotalTimeout  time.Duration
	ScenariosOnly bool
	JSON          bool
	Verbose       bool
	Stdout        io.Writer
	Stderr        io.Writer
}

// RunMeasureScenarios runs the gate measurement AND appends the additive
// per-directive scenario-satisfaction report. --scenarios-only skips the gate
// run entirely; --directives defers to the plain directives dump.
func RunMeasureScenarios(opts MeasureScenariosOptions) error {
	if opts.ScenariosOnly {
		return runScenariosOnly(opts.GoalsFile, opts.ProjectRoot, opts.JSON, opts.Stdout)
	}
	measureOpts := MeasureOptions{
		GoalID:       opts.GoalID,
		ExcludeTag:   opts.ExcludeTag,
		Directives:   opts.Directives,
		GoalsFile:    opts.GoalsFile,
		Timeout:      opts.Timeout,
		TotalTimeout: opts.TotalTimeout,
		JSON:         opts.JSON,
		Verbose:      opts.Verbose,
		Stdout:       opts.Stdout,
		Stderr:       opts.Stderr,
	}
	// --directives is a directives-only dump; scenario satisfaction does not
	// apply, so defer entirely to the existing behavior.
	if opts.Directives {
		return RunMeasure(measureOpts)
	}
	if opts.JSON {
		return runMeasureJSONWithScenarios(measureOpts, opts.GoalsFile, opts.ProjectRoot)
	}
	return runMeasureHumanWithScenarios(measureOpts, opts.GoalsFile, opts.ProjectRoot)
}

// runMeasureJSONWithScenarios captures RunMeasure's snapshot JSON, then
// re-emits a combined payload carrying both the snapshot and the per-directive
// scenario-satisfaction report. The snapshot shape itself is unchanged.
func runMeasureJSONWithScenarios(opts MeasureOptions, goalsFile, projectRoot string) error {
	// RunMeasure encodes the snapshot JSON directly to its Stdout. Redirect
	// that into a buffer so the only thing on the real stdout is the combined
	// snapshot+scenarios payload (a single valid JSON document).
	realStdout := opts.Stdout
	var buf bytes.Buffer
	opts.Stdout = &buf
	if err := RunMeasure(opts); err != nil {
		return err
	}
	var snap Snapshot
	if err := json.Unmarshal(buf.Bytes(), &snap); err != nil {
		return fmt.Errorf("decoding measurement snapshot: %w", err)
	}
	reports, err := evaluateDirectiveScenarios(goalsFile, projectRoot)
	if err != nil {
		return err
	}
	return emitMeasureScenarioJSON(realStdout, measureModeFull, &snap, reports)
}

// runMeasureHumanWithScenarios runs the gate measurement (its human table
// prints as before), then appends the scenario-satisfaction table below it.
func runMeasureHumanWithScenarios(opts MeasureOptions, goalsFile, projectRoot string) error {
	if err := RunMeasure(opts); err != nil {
		return err
	}
	reports, err := evaluateDirectiveScenarios(goalsFile, projectRoot)
	if err != nil {
		return err
	}
	renderScenarioReports(opts.Stdout, measureModeFull, reports)
	return nil
}

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
	patcher, _, err := LoadGoalsPatcher(goalsFile)
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
func buildDirectiveScenarioReport(agg *goalsfitness.Aggregator, d ParsedDirective, threshold float64) directiveScenarioReport {
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
// SKIPS shell gate-command execution entirely. It never calls RunMeasure (and
// therefore never spawns a gate subprocess), so it is safe to run in
// environments where gate commands are slow, unavailable, or undesired.
//
// Exit-code semantics (kept consistent with `ao goals measure`): a failing
// scenario verdict is a measurement outcome, not an invocation error, so it
// returns nil; a structurally-invalid input (unloadable GOALS.md, a malformed
// scenario threshold) returns an error.
func runScenariosOnly(goalsFile, projectRoot string, asJSON bool, stdout io.Writer) error {
	reports, err := evaluateDirectiveScenarios(goalsFile, projectRoot)
	if err != nil {
		return err
	}
	if asJSON {
		return emitMeasureScenarioJSON(stdout, measureModeScenariosOnly, nil, reports)
	}
	renderScenarioReports(stdout, measureModeScenariosOnly, reports)
	return nil
}

// emitMeasureScenarioJSON writes the combined measure+scenario JSON payload.
func emitMeasureScenarioJSON(w io.Writer, mode string, snap *Snapshot, reports []directiveScenarioReport) error {
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
