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
	Short:   "AgentOps Knowledge Compounding CLI",
	Long: `ao is the CLI for AgentOps, a software-factory control plane for repo-native agent work.

"Problem in. Value out. Intelligence compounds."

Software Factory Lane:
  ao factory start --goal "fix auth startup"
  /rpi "fix auth startup"   or   ao rpi phased "fix auth startup"
  ao codex stop

The Knowledge Flywheel underneath it:
  Sessions compound via .agents/ + Smart Connections.
  Others start fresh. You get smarter every session.

For AI agents:
  ao capabilities     Machine-readable CLI contract (JSON) — run this first.
  ao robot-docs       Paste-ready agent handbook.
  Append --json to any read-side command for structured output.

Use "ao <command> --help" for more information about a command.`,
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if jsonFlag {
			output = "json"
		}
		syncConfigFlagToEnv()
		if err := sanitizeGitProcessEnv(); err != nil {
			return err
		}
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		if err := repairSharedCoreWorktreeConfig(cwd); err != nil {
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
