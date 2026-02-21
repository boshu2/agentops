package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/config"
)

var (
	configShow bool
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long: `View and manage AgentOps configuration.

Configuration priority (highest to lowest):
  1. Command-line flags
  2. Environment variables (AGENTOPS_*)
  3. Project config (.agentops/config.yaml)
  4. Home config (~/.agentops/config.yaml)
  5. Defaults

Environment variables:
  AGENTOPS_CONFIG     - Explicit config file path (overrides default project config location)
  AGENTOPS_OUTPUT     - Default output format (table, json, yaml)
  AGENTOPS_BASE_DIR   - Data directory path
  AGENTOPS_VERBOSE    - Enable verbose output (true/1)
  AGENTOPS_NO_SC      - Disable Smart Connections (true/1)
  AGENTOPS_RPI_WORKTREE_MODE - RPI worktree policy (auto|always|never)
  AGENTOPS_RPI_RUNTIME / AGENTOPS_RPI_RUNTIME_MODE - RPI runtime mode (auto|direct|stream)
  AGENTOPS_RPI_RUNTIME_COMMAND - Runtime command used by ao rpi phased (default: claude)
  AGENTOPS_RPI_AO_COMMAND - ao command used for ratchet/checkpoint calls (default: ao)
  AGENTOPS_RPI_BD_COMMAND - bd command used for epic/child checks (default: bd)
  AGENTOPS_RPI_TMUX_COMMAND - tmux command used for status liveness probes (default: tmux)
  AGENTOPS_FLYWHEEL_AUTO_PROMOTE_THRESHOLD - Default auto-promote age threshold (e.g. 24h)

Examples:
  ao config --show           # Show resolved configuration
  ao config --show -o json   # Output as JSON`,
	RunE: runConfig,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.Flags().BoolVar(&configShow, "show", false, "Show resolved configuration with sources")
}

func runConfig(cmd *cobra.Command, args []string) error {
	if !configShow {
		// Show help if no flags
		return cmd.Help()
	}

	// Get resolved config with sources
	resolved := config.Resolve(GetOutput(), "", GetVerbose())

	if GetOutput() == "json" {
		data, _ := json.MarshalIndent(resolved, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	// Print table format
	fmt.Println("AgentOps Configuration")
	fmt.Println("=====================")
	fmt.Println()

	fmt.Println("Config files:")
	homeConfig := filepath.Join(os.Getenv("HOME"), ".agentops", "config.yaml")
	if _, err := os.Stat(homeConfig); err == nil {
		fmt.Printf("  ✓ Home:    %s\n", homeConfig)
	} else {
		fmt.Printf("  ✗ Home:    %s (not found)\n", homeConfig)
	}

	cwd, _ := os.Getwd()
	projectConfig := filepath.Join(cwd, ".agentops", "config.yaml")
	if _, err := os.Stat(projectConfig); err == nil {
		fmt.Printf("  ✓ Project: %s\n", projectConfig)
	} else {
		fmt.Printf("  ✗ Project: %s (not found)\n", projectConfig)
	}

	fmt.Println()
	fmt.Println("Resolved values:")
	fmt.Printf("  output:   %v  (from %s)\n", resolved.Output.Value, resolved.Output.Source)
	fmt.Printf("  base_dir: %v  (from %s)\n", resolved.BaseDir.Value, resolved.BaseDir.Source)
	fmt.Printf("  verbose:  %v  (from %s)\n", resolved.Verbose.Value, resolved.Verbose.Source)
	fmt.Printf("  rpi.worktree_mode:  %v  (from %s)\n", resolved.RPIWorktreeMode.Value, resolved.RPIWorktreeMode.Source)
	fmt.Printf("  rpi.runtime_mode:   %v  (from %s)\n", resolved.RPIRuntimeMode.Value, resolved.RPIRuntimeMode.Source)
	fmt.Printf("  rpi.runtime_command: %v  (from %s)\n", resolved.RPIRuntimeCommand.Value, resolved.RPIRuntimeCommand.Source)
	fmt.Printf("  rpi.ao_command:     %v  (from %s)\n", resolved.RPIAOCommand.Value, resolved.RPIAOCommand.Source)
	fmt.Printf("  rpi.bd_command:     %v  (from %s)\n", resolved.RPIBDCommand.Value, resolved.RPIBDCommand.Source)
	fmt.Printf("  rpi.tmux_command:   %v  (from %s)\n", resolved.RPITmuxCommand.Value, resolved.RPITmuxCommand.Source)

	fmt.Println()
	fmt.Println("Environment variables (if set):")
	envVars := []string{
		"AGENTOPS_CONFIG",
		"AGENTOPS_OUTPUT",
		"AGENTOPS_BASE_DIR",
		"AGENTOPS_VERBOSE",
		"AGENTOPS_NO_SC",
		"AGENTOPS_RPI_WORKTREE_MODE",
		"AGENTOPS_RPI_RUNTIME",
		"AGENTOPS_RPI_RUNTIME_MODE",
		"AGENTOPS_RPI_RUNTIME_COMMAND",
		"AGENTOPS_RPI_AO_COMMAND",
		"AGENTOPS_RPI_BD_COMMAND",
		"AGENTOPS_RPI_TMUX_COMMAND",
		"AGENTOPS_FLYWHEEL_AUTO_PROMOTE_THRESHOLD",
	}
	anySet := false
	for _, env := range envVars {
		if v := os.Getenv(env); v != "" {
			fmt.Printf("  %s=%s\n", env, v)
			anySet = true
		}
	}
	if !anySet {
		fmt.Println("  (none set)")
	}

	return nil
}
