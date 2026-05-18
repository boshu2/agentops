// practices: [dora-metrics, lean-startup]
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/spf13/cobra"
)

var (
	goalsMeasureGoalID        string
	goalsMeasureDirectives    bool
	goalsMeasureExcludeTag    string
	goalsMeasureTotalTimeout  int
	goalsMeasureScenariosOnly bool
)

var goalsMeasureCmd = &cobra.Command{
	Use:     "measure",
	Aliases: []string{"m"},
	Short:   "Run goal checks and produce a snapshot",
	GroupID: "measurement",
	RunE: func(cmd *cobra.Command, args []string) error {
		goalsFile := resolveGoalsFile()
		asJSON := goalsJSONOutput()

		// --scenarios-only evaluates ONLY executable-spec scenario results and
		// skips shell gate-command execution entirely (goals.RunMeasure is
		// never called, so no gate subprocess ever spawns).
		if goalsMeasureScenariosOnly {
			return runScenariosOnly(goalsFile, measureProjectRoot(), asJSON, cmd.OutOrStdout())
		}
		return runMeasureWithScenarios(cmd, goalsFile, asJSON)
	},
}

// runMeasureWithScenarios runs the existing gate measurement AND appends the
// additive per-directive scenario-satisfaction report. The gate run is
// unchanged: Steer/Gates output and exit behavior are preserved exactly.
func runMeasureWithScenarios(cmd *cobra.Command, goalsFile string, asJSON bool) error {
	opts := goals.MeasureOptions{
		GoalID:       goalsMeasureGoalID,
		ExcludeTag:   goalsMeasureExcludeTag,
		Directives:   goalsMeasureDirectives,
		GoalsFile:    goalsFile,
		Timeout:      time.Duration(goalsTimeout) * time.Second,
		TotalTimeout: time.Duration(goalsMeasureTotalTimeout) * time.Second,
		JSON:         asJSON,
		Verbose:      verbose,
		Stdout:       cmd.OutOrStdout(),
		Stderr:       cmd.ErrOrStderr(),
	}

	// --directives is a directives-only dump; scenario satisfaction does not
	// apply, so defer entirely to the existing behavior.
	if goalsMeasureDirectives {
		return goals.RunMeasure(opts)
	}

	if asJSON {
		return runMeasureJSONWithScenarios(opts, goalsFile)
	}
	return runMeasureHumanWithScenarios(cmd, opts, goalsFile)
}

// runMeasureJSONWithScenarios captures goals.RunMeasure's snapshot JSON, then
// re-emits a combined payload carrying both the snapshot and the per-directive
// scenario-satisfaction report. The snapshot shape itself is unchanged.
func runMeasureJSONWithScenarios(opts goals.MeasureOptions, goalsFile string) error {
	// goals.RunMeasure encodes the snapshot JSON directly to its Stdout.
	// Redirect that into a buffer so the only thing on the real stdout is the
	// combined snapshot+scenarios payload (a single valid JSON document).
	realStdout := opts.Stdout
	var buf bytes.Buffer
	opts.Stdout = &buf
	if err := goals.RunMeasure(opts); err != nil {
		return err
	}
	var snap goals.Snapshot
	if err := json.Unmarshal(buf.Bytes(), &snap); err != nil {
		return fmt.Errorf("decoding measurement snapshot: %w", err)
	}
	reports, err := evaluateDirectiveScenarios(goalsFile, measureProjectRoot())
	if err != nil {
		return err
	}
	recordVerdictLedgerIterations(measureProjectRoot(), reports, opts.Stderr)
	return emitMeasureScenarioJSON(realStdout, measureModeFull, &snap, reports)
}

// runMeasureHumanWithScenarios runs the gate measurement (its human table
// prints as before), then appends the scenario-satisfaction table below it.
func runMeasureHumanWithScenarios(cmd *cobra.Command, opts goals.MeasureOptions, goalsFile string) error {
	if err := goals.RunMeasure(opts); err != nil {
		return err
	}
	reports, err := evaluateDirectiveScenarios(goalsFile, measureProjectRoot())
	if err != nil {
		return err
	}
	recordVerdictLedgerIterations(measureProjectRoot(), reports, cmd.ErrOrStderr())
	renderScenarioReports(cmd.OutOrStdout(), measureModeFull, reports)
	return nil
}

func init() {
	goalsMeasureCmd.Flags().StringVar(&goalsMeasureGoalID, "goal", "", "Measure a single goal by ID")
	goalsMeasureCmd.Flags().BoolVar(&goalsMeasureDirectives, "directives", false, "Output directives as JSON (skip gate checks)")
	goalsMeasureCmd.Flags().StringVar(&goalsMeasureExcludeTag, "exclude-tag", "", "Skip goals whose Tags include this value (e.g. long-cycle)")
	goalsMeasureCmd.Flags().IntVar(&goalsMeasureTotalTimeout, "total-timeout", 0, "Overall measurement timeout in seconds (0 disables)")
	goalsMeasureCmd.Flags().BoolVar(&goalsMeasureScenariosOnly, "scenarios-only", false, "Evaluate only executable-spec scenario satisfaction; skip shell gate-command execution")
	goalsCmd.AddCommand(goalsMeasureCmd)
}
