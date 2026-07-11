// practices: [sre, resilience-patterns]
package main

import (
	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/doctor"
	"github.com/boshu2/agentops/cli/internal/quality"
)

var (
	doctorJSON bool
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check AgentOps health",
	Long: `Run health checks on your AgentOps installation.

Validates that all required components are present and configured.
Optional components are reported as warnings but do not cause failure.

Examples:
  ao doctor
  ao doctor --json`,
	RunE: runDoctor,
}

func init() {
	doctorCmd.GroupID = "core"
	doctorCmd.Flags().BoolVar(&doctorJSON, "json", false, "Output results as JSON")
	// Attach the diagnose-and-repair engine surface: additive flags
	// (--fix, --dry-run, --only, ...) plus subcommands (fix, undo, explain,
	// capabilities, health, robot-docs, gc, ls, diff). The legacy 16-check
	// behavior is preserved when no engine flag/subcommand is used.
	registerDoctorSurface()
	rootCmd.AddCommand(doctorCmd)
}

// gatherDoctorChecks runs all doctor checks and returns the results.
func gatherDoctorChecks() []quality.Check {
	return newLegacyDoctorService().Checks(rootCmd.Context())
}

func runDoctor(cmd *cobra.Command, args []string) error {
	// Engine-flag invocations (--fix, --explain, --robot-triage) route entirely
	// through the diagnose-and-repair engine.
	if doctorFix || doctorExplainFlag != "" || doctorRobotTriage {
		return runDoctorEngineDefault(cmd)
	}

	// `ao doctor --json` is the engine's machine surface: a single diagnose
	// Report (schema_version, exit_code, findings).
	if doctorWantsJSON() {
		return runDoctorEngineDefault(cmd)
	}

	// Human-readable form: the legacy check table, with engine findings
	// appended additively so existing output never regresses.
	checks := gatherDoctorChecks()
	if err := quality.RunDoctor(quality.DoctorOptions{
		JSON:   doctorJSON,
		Checks: checks,
		Stdout: cmd.OutOrStdout(),
	}); err != nil {
		return err
	}
	// Additive: also run the registered failure-mode detectors and surface
	// their findings without disturbing the legacy `checks` output above. With
	// the FOUNDATION wave's empty registry this is a no-op; later waves light up.
	return appendEngineFindings(cmd)
}

// appendEngineFindings runs the doctor engine's detectors and appends their
// findings to the human-readable `ao doctor` output. It never alters the legacy
// `checks` output or the bare command's exit semantics — a doctorExitError is
// only returned when engine findings exist (mapped to exit 1).
func appendEngineFindings(cmd *cobra.Command) error {
	// With no detectors registered there is nothing to add and no reason to
	// create a run directory; skip entirely so the legacy command stays
	// side-effect-free.
	if len(doctor.Detectors()) == 0 {
		return nil
	}
	// JSON callers receive the engine Report from the dedicated --json path in
	// runDoctor; never emit a second JSON document here.
	if doctorWantsJSON() {
		return nil
	}
	opts, err := doctorEngineOptions()
	if err != nil {
		return nil // never let the engine break the legacy command
	}
	rep, derr := doctor.Diagnose(opts)
	if derr != nil || rep == nil || len(rep.Findings) == 0 {
		return nil
	}
	renderEngineFindings(cmd, rep)
	return exitErr(rep.ExitCode, "doctor findings present")
}
