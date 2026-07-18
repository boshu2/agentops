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

	doctorcommands "github.com/boshu2/agentops/cli/internal/commands/doctor"
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
	Short:   "AgentOps CLI: deterministic checks and evidence records",
	Long: `ao provides deterministic repository checks and generic evidence records.
Semantic judgment belongs to the Validate skill. Git, delivery, retries, work
ownership, and continuation belong to the caller and repository.

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
		cwd, err := os.Getwd()
		if err != nil {
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
		var docErr *doctorcommands.ExitError
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
		var commandExit commandExitError
		if errors.As(err, &commandExit) {
			// Family modules can return a typed verdict without making the root
			// executable import their concrete error type. Command output already
			// carries the explanation; the root owns only process-code mapping.
			os.Exit(commandExit.ExitCode())
		}
		printRemovedCommandHint(os.Stderr, rootCmd, err)
		printRequiredFlagHint(executedCmd, err)
		os.Exit(1)
	}
}

type commandExitError interface {
	error
	ExitCode() int
}

func init() {
	// Command groups for organized help output
	rootCmd.AddGroup(
		&cobra.Group{ID: "start", Title: "Getting Started:"},
		&cobra.Group{ID: "core", Title: "Core Commands:"},
		&cobra.Group{ID: "workflow", Title: "Workflow:"},
		&cobra.Group{ID: "config", Title: "Configuration:"},
		&cobra.Group{ID: "comms", Title: "Evidence:"},
		&cobra.Group{ID: "knowledge", Title: "Knowledge:"},
		&cobra.Group{ID: "experimental", Title: "Optional knowledge tools:"},
	)

	// Global flags available to all commands
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Show what would happen without executing")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().StringVarP(&output, "output", "o", "table", "Output format (json, table, yaml)")
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "Output as JSON (shorthand for -o json)")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "Config file (default: ~/.agents/ao/config.yaml)")

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
