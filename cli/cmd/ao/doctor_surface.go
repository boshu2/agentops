// practices: [sre, resilience-patterns]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/doctor"
)

// doctorExitError carries a doctor exit code out through cobra's RunE so that
// Execute() can map it to os.Exit. It reuses the established typed-error +
// errors.As pattern (see AgentsLintError).
type doctorExitError struct {
	code int
	msg  string
}

func (e *doctorExitError) Error() string { return e.msg }

// ExitCode returns the process exit code this error maps to.
func (e *doctorExitError) ExitCode() int { return e.code }

// doctorEngineFlags holds the additive flags for the new doctor engine surface.
var (
	doctorFix         bool
	doctorDryRun      bool
	doctorOnly        []string
	doctorSkip        []string
	doctorSince       string
	doctorOnline      bool
	doctorQuick       bool
	doctorSeverity    string
	doctorRobot       bool
	doctorRobotTriage bool
	doctorExplainFlag string
	doctorUndoStrict  bool
	doctorGCBefore    string
	doctorGCYes       bool
)

// registerDoctorSurface attaches the engine flags and subcommands to doctorCmd.
// It is invoked from doctor.go's init after doctorCmd is constructed.
func registerDoctorSurface() {
	f := doctorCmd.Flags()
	f.BoolVar(&doctorFix, "fix", false, "Apply fixers for findings (routes through mutate())")
	f.BoolVar(&doctorDryRun, "dry-run", false, "With --fix: print the plan, change nothing")
	f.StringSliceVar(&doctorOnly, "only", nil, "Scope to a subset of detectors or subsystems")
	f.StringSliceVar(&doctorSkip, "skip", nil, "Inverse of --only")
	f.StringVar(&doctorSince, "since", "", "Diff findings against an earlier run")
	f.BoolVar(&doctorOnline, "online", false, "Enable network probes (default: offline-only)")
	f.BoolVar(&doctorQuick, "quick", false, "Run only fast-path detectors (< 200ms)")
	f.StringVar(&doctorSeverity, "severity", "P3", "Minimum severity to emit (P0|P1|P2|P3)")
	f.BoolVar(&doctorRobot, "robot", false, "Alias for --json with structured wrapper")
	f.BoolVar(&doctorRobotTriage, "robot-triage", false, "Emit the mega-command triage JSON")
	f.StringVar(&doctorExplainFlag, "explain", "", "Expand a single finding by id")

	doctorCmd.AddCommand(
		newDoctorFixCmd(),
		newDoctorUndoCmd(),
		newDoctorExplainCmd(),
		newDoctorCapabilitiesCmd(),
		newDoctorHealthCmd(),
		newDoctorRobotDocsCmd(),
		newDoctorGcCmd(),
		newDoctorLsCmd(),
		newDoctorDiffCmd(),
	)

	// Doctor commands carry their result in the exit code. Findings (exit 1)
	// are a normal diagnostic outcome, not a failure, so cobra must not print
	// "Error: ..." to stderr. Genuine failures are surfaced by Execute()
	// instead (see root.go). SilenceErrors does not inherit — set it per cmd.
	doctorCmd.SilenceErrors = true
	for _, sub := range doctorCmd.Commands() {
		sub.SilenceErrors = true
	}
	// SilenceErrors also mutes flag-parse errors (e.g. an unknown flag), which
	// ARE genuine usage failures the caller must see. Restore them via an
	// explicit flag-error func (inherited by subcommands).
	doctorCmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		fmt.Fprintln(c.ErrOrStderr(), "Error:", err.Error())
		return err
	})
}

// doctorEngineOptions builds doctor.Options from the current process context.
func doctorEngineOptions() (doctor.Options, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return doctor.Options{}, &doctorExitError{code: doctor.ExitIOError, msg: err.Error()}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return doctor.Options{
		RepoRoot:    cwd,
		CWD:         cwd,
		HomeDir:     home,
		ToolVersion: version,
		Only:        doctorOnly,
		Skip:        doctorSkip,
		Quick:       doctorQuick,
		Online:      doctorOnline,
		Severity:    doctorSeverity,
		DryRun:      doctorDryRun,
		JSON:        doctorWantsJSON(),
		Now:         time.Now(),
	}, nil
}

// doctorWantsJSON reports whether the caller asked for JSON output.
func doctorWantsJSON() bool {
	return doctorJSON || doctorRobot || jsonFlag
}

// printDoctorJSON marshals v as indented JSON to stdout.
func printDoctorJSON(cmd *cobra.Command, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return &doctorExitError{code: doctor.ExitIOError, msg: err.Error()}
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// exitErr wraps an exit code into a doctorExitError unless the code is success.
func exitErr(code int, msg string) error {
	if code == doctor.ExitHealthy {
		return nil
	}
	return &doctorExitError{code: code, msg: msg}
}

// runDoctorEngineDefault runs diagnose (or fix / explain / triage) for the
// default `ao doctor` invocation when an engine flag is present.
func runDoctorEngineDefault(cmd *cobra.Command) error {
	if doctorExplainFlag != "" {
		return runDoctorExplain(cmd, doctorExplainFlag)
	}
	opts, err := doctorEngineOptions()
	if err != nil {
		return err
	}
	if doctorRobotTriage {
		triage, rep, terr := doctor.RobotTriage(opts)
		if terr != nil {
			return &doctorExitError{code: doctor.ExitIOError, msg: terr.Error()}
		}
		if err := printDoctorJSON(cmd, triage); err != nil {
			return err
		}
		return exitErr(rep.ExitCode, "doctor findings present")
	}
	if doctorFix {
		return runDoctorFix(cmd, opts)
	}
	rep, derr := doctor.Diagnose(opts)
	if derr != nil {
		return &doctorExitError{code: doctor.ExitIOError, msg: derr.Error()}
	}
	if doctorWantsJSON() {
		if err := printDoctorJSON(cmd, rep); err != nil {
			return err
		}
	} else {
		renderEngineFindings(cmd, rep)
	}
	return exitErr(rep.ExitCode, "doctor findings present")
}

// renderEngineFindings appends a findings section to human-readable output.
func renderEngineFindings(cmd *cobra.Command, rep *doctor.Report) {
	w := cmd.OutOrStdout()
	if len(rep.Findings) == 0 {
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Failure-mode findings (%d):\n", len(rep.Findings))
	for _, f := range rep.Findings {
		fmt.Fprintf(w, "  [%s] %s — %s\n", f.Severity, f.ID, f.Title)
	}
}

// runDoctorFix runs the fix engine and renders/exits accordingly.
func runDoctorFix(cmd *cobra.Command, opts doctor.Options) error {
	rep, err := doctor.Fix(opts)
	if err != nil {
		return &doctorExitError{code: doctor.ExitIOError, msg: err.Error()}
	}
	if doctorWantsJSON() {
		if jerr := printDoctorJSON(cmd, rep); jerr != nil {
			return jerr
		}
	} else {
		w := cmd.OutOrStdout()
		fmt.Fprintf(w, "ao doctor fix — run %s\n", rep.RunID)
		fmt.Fprintf(w, "  findings: %d  actions: %d\n", rep.Summary.TotalFindings, rep.ActionsTaken)
		if rep.UndoCommand != "" {
			fmt.Fprintf(w, "  undo: %s\n", rep.UndoCommand)
		}
	}
	return exitErr(rep.ExitCode, "doctor fix incomplete")
}

// runDoctorExplain expands a single finding.
func runDoctorExplain(cmd *cobra.Command, findingID string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return &doctorExitError{code: doctor.ExitIOError, msg: err.Error()}
	}
	finding, ferr := doctor.Explain(cwd, findingID)
	if ferr != nil {
		return &doctorExitError{code: doctor.ExitNoInput, msg: ferr.Error()}
	}
	if doctorWantsJSON() {
		return printDoctorJSON(cmd, finding)
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s [%s] (%s)\n%s\n", finding.ID, finding.Severity, finding.Subsystem, finding.Title)
	fmt.Fprintf(w, "Remediation: %s\n", finding.Remediation.Command)
	return nil
}

func newDoctorFixCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "fix",
		Short: "Run detectors, then apply fixers (backs up before every mutation)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := doctorEngineOptions()
			if err != nil {
				return err
			}
			return runDoctorFix(cmd, opts)
		},
	}
}

func newDoctorUndoCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "undo <run-id>",
		Short: "Restore from .doctor/runs/<run-id>/backups/ (run-id may be 'latest')",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return &doctorExitError{code: doctor.ExitIOError, msg: err.Error()}
			}
			res, uerr := doctor.Undo(cwd, args[0], doctorUndoStrict, doctorDryRun)
			if uerr != nil {
				if doctorWantsJSON() {
					_ = printDoctorJSON(cmd, res)
				} else {
					fmt.Fprintf(cmd.ErrOrStderr(), "undo failed: %v\n", uerr)
				}
				return &doctorExitError{code: res.ExitCode, msg: uerr.Error()}
			}
			if doctorWantsJSON() {
				return printDoctorJSON(cmd, res)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "undo %s: restored=%d skipped=%d\n", res.RunID, res.Restored, res.Skipped)
			return nil
		},
	}
	c.Flags().BoolVar(&doctorUndoStrict, "strict", true, "Refuse if any backup is missing or hash-mismatched")
	c.Flags().BoolVar(&doctorDryRun, "dry-run", false, "Print the restore plan; do not execute")
	return c
}

func newDoctorExplainCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "explain <finding-id>",
		Short: "Expand a single finding with full evidence",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctorExplain(cmd, args[0])
		},
	}
}

func newDoctorCapabilitiesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "capabilities",
		Short: "Print the machine-readable doctor contract (JSON)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			caps := doctor.NewCapabilities(version)
			return printDoctorJSON(cmd, caps)
		},
	}
}

func newDoctorHealthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Cheap one-line liveness summary",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return &doctorExitError{code: doctor.ExitIOError, msg: err.Error()}
			}
			line, hr, herr := doctor.Health(cwd, version)
			if herr != nil {
				return &doctorExitError{code: doctor.ExitIOError, msg: herr.Error()}
			}
			if doctorWantsJSON() {
				if jerr := printDoctorJSON(cmd, hr); jerr != nil {
					return jerr
				}
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), line)
			}
			return exitErr(hr.ExitCode, "doctor health: not ok")
		},
	}
}

func newDoctorRobotDocsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "robot-docs",
		Short: "Print the paste-ready agent handbook (Markdown)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprint(cmd.OutOrStdout(), doctor.RobotDocs())
			return nil
		},
	}
}

func newDoctorGcCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "gc",
		Short: "Prune old runs (requires --yes and --before <date>)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return &doctorExitError{code: doctor.ExitIOError, msg: err.Error()}
			}
			var cutoff time.Time
			if doctorGCBefore != "" {
				parsed, perr := time.Parse("2006-01-02", doctorGCBefore)
				if perr != nil {
					return &doctorExitError{code: doctor.ExitUsage, msg: "invalid --before date (want YYYY-MM-DD)"}
				}
				cutoff = parsed
			}
			pruned, gerr := doctor.GC(cwd, cutoff, doctorGCYes)
			if gerr != nil {
				return &doctorExitError{code: doctor.ExitUsage, msg: gerr.Error()}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "pruned %d run(s)\n", pruned)
			return nil
		},
	}
	c.Flags().StringVar(&doctorGCBefore, "before", "", "Prune runs started before this date (YYYY-MM-DD)")
	c.Flags().BoolVar(&doctorGCYes, "yes", false, "Confirm pruning (required)")
	return c
}

func newDoctorLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List runs in .doctor/runs/",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return &doctorExitError{code: doctor.ExitIOError, msg: err.Error()}
			}
			runs, lerr := doctor.Ls(cwd)
			if lerr != nil {
				return &doctorExitError{code: doctor.ExitIOError, msg: lerr.Error()}
			}
			if doctorWantsJSON() {
				return printDoctorJSON(cmd, map[string]any{"runs": runs})
			}
			w := cmd.OutOrStdout()
			if len(runs) == 0 {
				fmt.Fprintln(w, "no doctor runs")
				return nil
			}
			for _, r := range runs {
				fmt.Fprintf(w, "%s  exit=%d  actions=%d\n", r.RunID, r.ExitCode, r.ActionCount)
			}
			return nil
		},
	}
}

func newDoctorDiffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diff",
		Short: "Show what --fix would change (read-only)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := doctorEngineOptions()
			if err != nil {
				return err
			}
			rep, derr := doctor.Diff(opts)
			if derr != nil {
				return &doctorExitError{code: doctor.ExitIOError, msg: derr.Error()}
			}
			if doctorWantsJSON() {
				return printDoctorJSON(cmd, rep)
			}
			renderEngineFindings(cmd, rep)
			if len(rep.Findings) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "clean diff: --fix would change nothing")
			}
			return nil
		},
	}
}
