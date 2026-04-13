package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/config"
)

var (
	configShow     bool
	modelsSetTier  string
	modelsSetSkill string
)

type configModelsWriteResult struct {
	Updated        bool              `json:"updated"`
	DefaultTier    string            `json:"default_tier,omitempty"`
	SkillOverrides map[string]string `json:"skill_overrides,omitempty"`
}

var configModelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Show model cost tier configuration",
	Long: `Display the current model cost tier settings with sources.

Cost tiers map to model quality levels:
  quality  → opus   (high-stakes decisions, architecture)
  balanced → sonnet (default, routine reviews)
  budget   → haiku  (quick checks, simple tasks)
  inherit  → uses default tier (falls back to balanced)

Configure in .agentops/config.yaml:
  models:
    default_tier: balanced
    skill_overrides:
      council: quality
      crank: budget

Or via environment variables:
  AGENTOPS_MODEL_TIER=budget
  AGENTOPS_COUNCIL_MODEL_TIER=quality`,
	RunE: runConfigModels,
}

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
  AGENTOPS_MODEL_TIER - Default model cost tier (quality/balanced/budget)
  AGENTOPS_COUNCIL_MODEL_TIER - Council-specific model tier override
  AGENTOPS_DREAM_REPORT_DIR - Default output directory for overnight Dream reports
  AGENTOPS_DREAM_RUN_TIMEOUT - Default timeout for overnight Dream runs
  AGENTOPS_DREAM_KEEP_AWAKE - Default keep-awake behavior for overnight Dream runs

Examples:
  ao config --show           # Show resolved configuration
  ao config --show --json   # Output as JSON`,
	RunE: runConfig,
}

func init() {
	configCmd.GroupID = "config"
	rootCmd.AddCommand(configCmd)
	configCmd.Flags().BoolVar(&configShow, "show", false, "Show resolved configuration with sources")
	configCmd.AddCommand(configModelsCmd)
	configModelsCmd.Flags().StringVar(&modelsSetTier, "set-tier", "", "Set the default model cost tier (quality, balanced, budget)")
	configModelsCmd.Flags().StringVar(&modelsSetSkill, "set-skill", "", "Set a skill-specific tier override (e.g. council=quality)")
}

func runConfig(cmd *cobra.Command, args []string) error {
	if !configShow {
		// Show help if no flags
		return cmd.Help()
	}

	// Get resolved config with sources
	resolved := config.Resolve(GetOutput(), "", GetVerbose())

	if GetOutput() == "json" {
		data, err := json.MarshalIndent(resolved, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal config: %w", err)
		}
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
	fmt.Printf("  dream.report_dir:   %v  (from %s)\n", resolved.DreamReportDir.Value, resolved.DreamReportDir.Source)
	fmt.Printf("  dream.run_timeout:  %v  (from %s)\n", resolved.DreamRunTimeout.Value, resolved.DreamRunTimeout.Source)
	fmt.Printf("  dream.keep_awake:   %v  (from %s)\n", resolved.DreamKeepAwake.Value, resolved.DreamKeepAwake.Source)
	fmt.Printf("  dream.runners:      %v  (from %s)\n", resolved.DreamRunners.Value, resolved.DreamRunners.Source)
	fmt.Printf("  dream.scheduler:    %v  (from %s)\n", resolved.DreamScheduler.Value, resolved.DreamScheduler.Source)
	fmt.Printf("  dream.schedule_at:  %v  (from %s)\n", resolved.DreamScheduleAt.Value, resolved.DreamScheduleAt.Source)
	fmt.Printf("  dream.consensus:    %v  (from %s)\n", resolved.DreamConsensus.Value, resolved.DreamConsensus.Source)
	fmt.Printf("  dream.creative:     %v  (from %s)\n", resolved.DreamCreativeLane.Value, resolved.DreamCreativeLane.Source)

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
		"AGENTOPS_MODEL_TIER",
		"AGENTOPS_COUNCIL_MODEL_TIER",
		"AGENTOPS_DREAM_REPORT_DIR",
		"AGENTOPS_DREAM_RUN_TIMEOUT",
		"AGENTOPS_DREAM_KEEP_AWAKE",
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

func runConfigModels(_ *cobra.Command, _ []string) error {
	// Handle write operations if either flag is set.
	if modelsSetTier != "" || modelsSetSkill != "" {
		return handleModelsWrite()
	}

	cfg, err := config.Load(nil)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if GetOutput() == "json" {
		data, err := json.MarshalIndent(cfg.Models, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal models config: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println("Model Cost Tiers")
	fmt.Println("================")
	fmt.Println()

	fmt.Printf("  Default tier: %s\n", cfg.Models.DefaultTier)
	fmt.Println()

	fmt.Println("  Available tiers:")
	for _, name := range []string{"quality", "balanced", "budget"} {
		tier, ok := cfg.Models.Tiers[name]
		if !ok {
			continue
		}
		marker := " "
		if name == cfg.ResolveTier("") {
			marker = "*"
		}
		codex := tier.Codex
		if codex == "" {
			codex = "(default)"
		}
		fmt.Printf("  %s %-10s  claude=%-8s  codex=%s\n", marker, name, tier.Claude, codex)
	}

	fmt.Println()
	if len(cfg.Models.SkillOverrides) > 0 {
		fmt.Println("  Skill overrides:")
		for skill, tier := range cfg.Models.SkillOverrides {
			resolved := cfg.ResolveTier(skill)
			if tier == resolved {
				fmt.Printf("    %-12s → %s\n", skill, tier)
			} else {
				fmt.Printf("    %-12s → %s (resolves to %s)\n", skill, tier, resolved)
			}
		}
	} else {
		fmt.Println("  Skill overrides: (none)")
	}

	fmt.Println()
	fmt.Println("  Environment overrides:")
	modelEnvVars := []string{
		"AGENTOPS_MODEL_TIER",
		"AGENTOPS_COUNCIL_MODEL_TIER",
		"COUNCIL_CLAUDE_MODEL",
	}
	anyModelEnv := false
	for _, env := range modelEnvVars {
		if v := os.Getenv(env); v != "" {
			fmt.Printf("    %s=%s\n", env, v)
			anyModelEnv = true
		}
	}
	if !anyModelEnv {
		fmt.Println("    (none set)")
	}

	return nil
}

func handleModelsWrite() error {
	saveCfg := &config.Config{}
	result := configModelsWriteResult{Updated: true}

	if modelsSetTier != "" {
		if modelsSetTier == "inherit" {
			return fmt.Errorf("invalid tier %q for default: \"inherit\" is only valid for skill overrides", modelsSetTier)
		}
		if !config.ValidTiers[modelsSetTier] {
			return fmt.Errorf("invalid tier %q: must be one of quality, balanced, budget", modelsSetTier)
		}
		saveCfg.Models.DefaultTier = modelsSetTier
		result.DefaultTier = modelsSetTier
	}

	if modelsSetSkill != "" {
		parts := strings.SplitN(modelsSetSkill, "=", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("invalid --set-skill format %q: expected skill=tier (e.g. council=quality)", modelsSetSkill)
		}
		skill, tier := parts[0], parts[1]
		if !config.ValidTiers[tier] {
			return fmt.Errorf("invalid tier %q for skill %q: must be one of quality, balanced, budget, inherit", tier, skill)
		}
		saveCfg.Models.SkillOverrides = map[string]string{skill: tier}
		result.SkillOverrides = map[string]string{skill: tier}
	}

	if err := config.Save(saveCfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	if GetOutput() == "json" {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal models write result: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	if modelsSetTier != "" {
		fmt.Printf("Set default model tier to %q\n", modelsSetTier)
	}
	if modelsSetSkill != "" {
		parts := strings.SplitN(modelsSetSkill, "=", 2)
		fmt.Printf("Set skill %q tier to %q\n", parts[0], parts[1])
	}

	return nil
}
