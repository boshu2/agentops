// practices: [pragmatic-programmer, twelve-factor-app]
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/adapters/worktreeconfig"
	"github.com/boshu2/agentops/cli/internal/doctor"
)

var (
	// Global flags
	dryRun   bool
	verbose  bool
	output   string
	jsonFlag bool
	cfgFile  string
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:     "ao",
	Version: version,
	Short:   "AgentOps CLI: validation gate + provenance for agent work",
	Long: `ao is the CLI for AgentOps: a verification membrane for agent work. The loop
produces validated output with proof — no verdict = not done.

The operating loop:
  /plan -> /implement -> /validate      Shape, build, and judge one behavior
  ao gate check --fast --scope head     The release gate before any push
  ao land <bead>                        Land bead-backed work through the pawl

For AI agents:
  ao capabilities     Machine-readable CLI contract (JSON) — run this first.
  ao robot-docs       Paste-ready agent handbook.
  Append --json to any read-side command for structured output.

If a command you relied on is gone, see docs/MIGRATION.md — every removed
surface has a row naming its replacement (and the restore path when one exists).

Use "ao <command> --help" for more information about a command.`,
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := negotiateOutput(cmd); err != nil {
			return err
		}
		syncConfigFlagToEnv()
		if err := worktreeconfig.SanitizeGitProcessEnv(); err != nil {
			return err
		}
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		if err := worktreeconfig.RepairSharedCoreWorktreeConfig(cwd); err != nil {
			return err
		}

		// Build App struct from resolved flag values and inject into context.
		app := NewApp()
		app.DryRun = dryRun
		app.Verbose = verbose
		app.Output = output
		app.JSON = jsonFlag
		app.CfgFile = cfgFile
		app.WorkDir = cwd
		cmd.SetContext(context.WithValue(cmd.Context(), appKey, app))

		return nil
	},
}

func negotiateOutput(cmd *cobra.Command) error {
	outputFlag := cmd.Root().PersistentFlags().Lookup("output")
	jsonOutputFlag := cmd.Root().PersistentFlags().Lookup("json")
	requestedOutput := outputFlag != nil && outputFlag.Changed
	requestedJSON := jsonOutputFlag != nil && jsonOutputFlag.Changed && jsonFlag

	switch output {
	case "table", "json", "yaml":
	default:
		return fmt.Errorf("unsupported output format %q (want table, json, or yaml)", output)
	}
	if requestedJSON && requestedOutput && output != "json" {
		return fmt.Errorf("conflicting output formats: --json requests json while --output requests %s", output)
	}
	if requestedJSON {
		output = "json"
	}
	return nil
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	executedCmd, err := rootCmd.ExecuteC()
	if err != nil {
		var lintErr *AgentsLintError
		if errors.As(err, &lintErr) {
			os.Exit(lintErr.ExitCode)
		}
		var docErr *doctorExitError
		if errors.As(err, &docErr) {
			// Exit 1 means findings are present — a normal diagnostic result,
			// not a failure, so it carries no stderr noise. Higher codes are
			// genuine failures; surface the reason on stderr (doctor commands
			// set SilenceErrors, so cobra prints nothing itself).
			if docErr.ExitCode() != doctor.ExitFindings && docErr.Error() != "" {
				fmt.Fprintln(os.Stderr, "ao doctor: "+docErr.Error())
			}
			os.Exit(docErr.ExitCode())
		}
		var gateErr *gateExitError
		if errors.As(err, &gateErr) {
			// The exit code IS the verdict in `ao validate --gate`. A FAIL (1)
			// carries no stderr noise (detail already went to stderr); an
			// internal error (2) surfaces its reason.
			if gateErr.ExitCode() == gateExitInternal && gateErr.Error() != "" {
				fmt.Fprintln(os.Stderr, "ao validate: "+gateErr.Error())
			}
			os.Exit(gateErr.ExitCode())
		}
		var planPawlErr *planPawlExitError
		if errors.As(err, &planPawlErr) {
			// The exit code IS the decision in `ao plan-pawl decide`: 3 REDO,
			// 4 BLOCKED. The decision already went to stdout and the command
			// silences cobra's error print; surface a reason only on a usage error.
			if planPawlErr.ExitCode() == planPawlExitUsage && planPawlErr.Error() != "" {
				fmt.Fprintln(os.Stderr, "ao plan-pawl: "+planPawlErr.Error())
			}
			os.Exit(planPawlErr.ExitCode())
		}
		var pawlReviewErr *pawlReviewExitError
		if errors.As(err, &pawlReviewErr) {
			// The exit code IS the verdict in `ao pawl review` (0 CONFIRMED · 3 REFUTED ·
			// 4 --converge advisory-only · 2 usage). The script already printed the
			// verdict + defects; propagate the code with no extra cobra noise.
			os.Exit(pawlReviewErr.ExitCode())
		}
		var reconcileErr *reconcileExitError
		if errors.As(err, &reconcileErr) {
			// ao provenance reconcile: 0 clean · 1 unbound/emit-failed · 2 usage. The
			// command already printed its report/reason; propagate the code cleanly.
			os.Exit(reconcileErr.ExitCode())
		}
		var verifyPrePushErr *verifyPrePushExitError
		if errors.As(err, &verifyPrePushErr) {
			// The exit code IS the decision in `ao verify pre-push` (0 allow · 1
			// refuse). The gate already printed the human refusal to stderr and
			// silences cobra's error print; just map to the process exit code.
			os.Exit(verifyPrePushErr.ExitCode())
		}
		var landErr *landExitError
		if errors.As(err, &landErr) {
			// The exit code IS the outcome in `ao land` (0 landed; a re-exec'd
			// child's code, or a pawl-land / gate / push failure otherwise). The
			// underlying step already printed its reason via streamed stdio;
			// propagate the code with no extra cobra noise.
			os.Exit(landErr.ExitCode())
		}
		var beadsErr *beadsExitError
		if errors.As(err, &beadsErr) {
			// The exit code IS the verdict in `ao beads verify|lint|audit`:
			// 1 means stale/flagged beads found. The verdict already went to
			// stdout and the command silenced cobra's error print, so there is
			// nothing more to surface here — just map to the process exit code.
			os.Exit(beadsErr.ExitCode())
		}
		var tickErr *tickExitError
		if errors.As(err, &tickErr) {
			if tickErr.Error() != "" {
				fmt.Fprintln(os.Stderr, tickErr.Error())
			}
			os.Exit(tickErr.ExitCode())
		}
		var scanErr *corpusScanExitError
		if errors.As(err, &scanErr) {
			// The exit code IS the verdict for `ao corpus scan`: 1 means a leak
			// marker (or unreadable file) was detected — fail closed. The
			// report already went to stdout/stderr, so nothing more to surface.
			os.Exit(scanErr.ExitCode())
		}
		var govErr *governorExitError
		if errors.As(err, &govErr) {
			// The exit code IS the decision for `ao governor budget`: 3 means HARDEN
			// (error budget burned — stop the line). The verdict already went to
			// stdout and the command silences cobra's error print, so nothing more
			// to surface — just map to the process exit code.
			os.Exit(govErr.ExitCode())
		}
		var wikiHealthErr *wikiHealthExitError
		if errors.As(err, &wikiHealthErr) {
			// The exit code IS the verdict for `ao wiki lint`: 1 means blocking
			// structural defects were found. The report already went to stdout
			// and the command silences cobra's error print, so nothing more to
			// surface — just map to the process exit code.
			os.Exit(wikiHealthErr.ExitCode())
		}
		printRemovedCommandHint(os.Stderr, rootCmd, err)
		printRequiredFlagHint(executedCmd, err)
		os.Exit(1)
	}
}

func init() {
	// Command groups for organized help output
	rootCmd.AddGroup(
		&cobra.Group{ID: "start", Title: "Getting Started:"},
		&cobra.Group{ID: "core", Title: "Core Commands:"},
		&cobra.Group{ID: "workflow", Title: "Workflow:"},
		&cobra.Group{ID: "config", Title: "Configuration:"},
		&cobra.Group{ID: "comms", Title: "Communication:"},
		&cobra.Group{ID: "knowledge", Title: "Knowledge:"},
		// The corpus/flywheel surface is experimental-tier (unproven — ADR-0004,
		// ADR-0011): kept and buildable, but demoted under its own header so the
		// spine (proven) commands lead the `ao --help` surface (age-h4y3).
		&cobra.Group{ID: "experimental", Title: "Experimental (corpus/flywheel):"},
	)

	// Global flags available to all commands
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Show what would happen without executing")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().StringVarP(&output, "output", "o", "table", "Output format (json, table, yaml)")
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "Output as JSON (shorthand for -o json)")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "Config file (default: ~/.agentops/config.yaml)")

	_ = rootCmd.RegisterFlagCompletionFunc("output", staticCompletionFunc("json", "table", "yaml"))

	// Turn opaque "unknown flag" errors into actionable typo hints. Inherited
	// by every subcommand that does not set its own FlagErrorFunc.
	rootCmd.SetFlagErrorFunc(flagErrorWithSuggestion)

	// When a parent command is invoked with --json, emit a machine-readable
	// subcommand listing instead of human help text. Inherited by all
	// subcommands; falls back to cobra's default help rendering otherwise.
	defaultHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		if maybeEmitGroupJSON(c) {
			return
		}
		defaultHelp(c, args)
	})
}

// GetDryRun returns the dry-run flag value for use by subcommands.
func GetDryRun() bool {
	return dryRun
}

// GetVerbose returns the verbose flag value for use by subcommands.
func GetVerbose() bool {
	return verbose
}

// GetOutput returns the output format for use by subcommands.
func GetOutput() string {
	return output
}

// GetConfigFile returns the config file path for use by subcommands.
func GetConfigFile() string {
	return cfgFile
}

// VerbosePrintf prints only when verbose mode is enabled.
func VerbosePrintf(format string, args ...any) {
	if verbose {
		fmt.Printf(format, args...)
	}
}

func syncConfigFlagToEnv() {
	path := strings.TrimSpace(GetConfigFile())
	if path == "" {
		return
	}
	_ = os.Setenv("AGENTOPS_CONFIG", path)
}

// GetCurrentUser returns the current system username.
// Uses os/user package for reliable identity, not spoofable via env vars.
func GetCurrentUser() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return "unknown"
}
