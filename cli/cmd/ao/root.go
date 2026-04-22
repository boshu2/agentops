package main

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/spf13/cobra"
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
	Use:   "ao",
	Short: "AgentOps Knowledge Compounding CLI",
	Long: `ao is the CLI for AgentOps, a software-factory control plane for repo-native agent work.

"Problem in. Value out. Intelligence compounds."

Software Factory Lane:
  ao factory start --goal "fix auth startup"
  /rpi "fix auth startup"   or   ao rpi phased "fix auth startup"
  ao codex stop

The Knowledge Flywheel underneath it:
  Sessions compound via .agents/ + Smart Connections.
  Others start fresh. You get smarter every session.

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
	if err := rootCmd.Execute(); err != nil {
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
