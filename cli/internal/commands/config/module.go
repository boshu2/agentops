// Package config owns Cobra presentation for the config command family.
package config

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	configapp "github.com/boshu2/agentops/cli/internal/config"
)

var showEnvironmentKeys = []string{
	"AGENTOPS_CONFIG", "AGENTOPS_OUTPUT", "AGENTOPS_BASE_DIR", "AGENTOPS_VERBOSE", "AGENTOPS_NO_SC",
	"AGENTOPS_RPI_WORKTREE_MODE", "AGENTOPS_RPI_RUNTIME", "AGENTOPS_RPI_RUNTIME_MODE",
	"AGENTOPS_RPI_RUNTIME_COMMAND", "AGENTOPS_RPI_AO_COMMAND", "AGENTOPS_RPI_BD_COMMAND", "AGENTOPS_RPI_TMUX_COMMAND",
	"AGENTOPS_FLYWHEEL_AUTO_PROMOTE_THRESHOLD", "AGENTOPS_MODEL_TIER", "AGENTOPS_COUNCIL_MODEL_TIER",
	"AGENTOPS_DREAM_REPORT_DIR", "AGENTOPS_DREAM_RUN_TIMEOUT", "AGENTOPS_DREAM_KEEP_AWAKE",
}

var modelEnvironmentKeys = []string{"AGENTOPS_MODEL_TIER", "AGENTOPS_COUNCIL_MODEL_TIER", "COUNCIL_CLAUDE_MODEL"}

type UseCases interface {
	Show(context.Context, string, bool) (configapp.ShowResult, error)
	Models(context.Context) (configapp.ModelsResult, error)
	WriteModels(context.Context, configapp.ModelsWriteRequest) (configapp.ModelsWriteResult, error)
}

type Module struct {
	useCases UseCases
	output   func() string
	verbose  func() bool
	dryRun   func() bool
}

type options struct {
	show     bool
	setTier  string
	setSkill string
}

func NewModule(useCases UseCases, output func() string, verbose func() bool, dryRun func() bool) Module {
	return Module{useCases: useCases, output: output, verbose: verbose, dryRun: dryRun}
}

func (Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{
		ID:       "ao.config",
		Profiles: clicontract.ProfileDefault | clicontract.ProfileFlywheel | clicontract.ProfileLegacy | clicontract.ProfileCombined,
		Args:     clicontract.ArgsPolicy{Name: "none", Validate: cobra.NoArgs}, Output: clicontract.OutputText,
		Effects:     clicontract.EffectFilesystem | clicontract.EffectEnvironment,
		ExitClasses: map[int]clicontract.ExitClass{0: clicontract.ExitSuccess, 1: clicontract.ExitFailure},
	}
}

func (module Module) Command() *cobra.Command {
	var commandOptions options
	root := &cobra.Command{
		Use: "config", Short: "Manage configuration", Args: cobra.NoArgs,
		Long: configLong,
	}
	root.Flags().BoolVar(&commandOptions.show, "show", false, "Show resolved configuration with sources")
	root.RunE = func(command *cobra.Command, _ []string) error {
		if !commandOptions.show {
			return command.Help()
		}
		result, err := module.useCases.Show(command.Context(), module.output(), module.verbose())
		if err != nil {
			return err
		}
		return renderShow(command, module.output(), result)
	}
	models := module.modelsCommand(&commandOptions)
	root.AddCommand(models)
	return root
}

func (module Module) modelsCommand(commandOptions *options) *cobra.Command {
	command := &cobra.Command{Use: "models", Short: "Show model cost tier configuration", Long: modelsLong, Args: cobra.NoArgs}
	command.Flags().StringVar(&commandOptions.setTier, "set-tier", "", "Set the default model cost tier (quality, balanced, budget)")
	command.Flags().StringVar(&commandOptions.setSkill, "set-skill", "", "Set a skill-specific tier override (e.g. council=quality)")
	_ = command.RegisterFlagCompletionFunc("set-tier", staticValues("quality", "balanced", "budget"))
	command.RunE = func(command *cobra.Command, _ []string) error {
		if commandOptions.setTier != "" || commandOptions.setSkill != "" {
			result, err := module.useCases.WriteModels(command.Context(), configapp.ModelsWriteRequest{
				DefaultTier: commandOptions.setTier, Skill: commandOptions.setSkill, DryRun: module.dryRun != nil && module.dryRun(),
			})
			if err != nil {
				return err
			}
			return renderModelsWrite(command, module.output(), commandOptions.setTier, commandOptions.setSkill, result)
		}
		result, err := module.useCases.Models(command.Context())
		if err != nil {
			return err
		}
		return renderModels(command, module.output(), result)
	}
	return command
}

func renderShow(command *cobra.Command, output string, result configapp.ShowResult) error {
	if output == "json" {
		return writeJSON(command, result.Resolved, "marshal config")
	}
	w := command.OutOrStdout()
	fmt.Fprintln(w, "AgentOps Configuration")
	fmt.Fprintln(w, "=====================")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Config files:")
	printConfigFile(w, "Home:   ", result.ConfigFiles.HomePath, result.ConfigFiles.HomeExists)
	printConfigFile(w, "Project:", result.ConfigFiles.ProjectPath, result.ConfigFiles.ProjectExists)
	r := result.Resolved
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Resolved values:")
	fmt.Fprintf(w, "  output:   %v  (from %s)\n", r.Output.Value, r.Output.Source)
	fmt.Fprintf(w, "  base_dir: %v  (from %s)\n", r.BaseDir.Value, r.BaseDir.Source)
	fmt.Fprintf(w, "  verbose:  %v  (from %s)\n", r.Verbose.Value, r.Verbose.Source)
	fmt.Fprintf(w, "  rpi.worktree_mode:  %v  (from %s)\n", r.RPIWorktreeMode.Value, r.RPIWorktreeMode.Source)
	fmt.Fprintf(w, "  rpi.runtime_mode:   %v  (from %s)\n", r.RPIRuntimeMode.Value, r.RPIRuntimeMode.Source)
	fmt.Fprintf(w, "  rpi.runtime_command: %v  (from %s)\n", r.RPIRuntimeCommand.Value, r.RPIRuntimeCommand.Source)
	fmt.Fprintf(w, "  rpi.ao_command:     %v  (from %s)\n", r.RPIAOCommand.Value, r.RPIAOCommand.Source)
	fmt.Fprintf(w, "  rpi.bd_command:     %v  (from %s)\n", r.RPIBDCommand.Value, r.RPIBDCommand.Source)
	fmt.Fprintf(w, "  rpi.tmux_command:   %v  (from %s)\n", r.RPITmuxCommand.Value, r.RPITmuxCommand.Source)
	fmt.Fprintf(w, "  dream.report_dir:   %v  (from %s)\n", r.DreamReportDir.Value, r.DreamReportDir.Source)
	fmt.Fprintf(w, "  dream.run_timeout:  %v  (from %s)\n", r.DreamRunTimeout.Value, r.DreamRunTimeout.Source)
	fmt.Fprintf(w, "  dream.keep_awake:   %v  (from %s)\n", r.DreamKeepAwake.Value, r.DreamKeepAwake.Source)
	fmt.Fprintf(w, "  dream.runners:      %v  (from %s)\n", r.DreamRunners.Value, r.DreamRunners.Source)
	fmt.Fprintf(w, "  dream.scheduler:    %v  (from %s)\n", r.DreamScheduler.Value, r.DreamScheduler.Source)
	fmt.Fprintf(w, "  dream.schedule_at:  %v  (from %s)\n", r.DreamScheduleAt.Value, r.DreamScheduleAt.Source)
	fmt.Fprintf(w, "  dream.consensus:    %v  (from %s)\n", r.DreamConsensus.Value, r.DreamConsensus.Source)
	fmt.Fprintf(w, "  dream.creative:     %v  (from %s)\n", r.DreamCreativeLane.Value, r.DreamCreativeLane.Source)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Environment variables (if set):")
	printEnvironment(w, showEnvironmentKeys, result.Environment, "  ")
	return nil
}

func renderModels(command *cobra.Command, output string, result configapp.ModelsResult) error {
	if output == "json" {
		return writeJSON(command, result.Config.Models, "marshal models config")
	}
	w := command.OutOrStdout()
	cfg := result.Config
	fmt.Fprintln(w, "Model Cost Tiers")
	fmt.Fprintln(w, "================")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Default tier: %s\n\n", cfg.Models.DefaultTier)
	fmt.Fprintln(w, "  Available tiers:")
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
		fmt.Fprintf(w, "  %s %-10s  claude=%-8s  codex=%s\n", marker, name, tier.Claude, codex)
	}
	fmt.Fprintln(w)
	if len(cfg.Models.SkillOverrides) == 0 {
		fmt.Fprintln(w, "  Skill overrides: (none)")
	} else {
		fmt.Fprintln(w, "  Skill overrides:")
		skills := make([]string, 0, len(cfg.Models.SkillOverrides))
		for skill := range cfg.Models.SkillOverrides {
			skills = append(skills, skill)
		}
		sort.Strings(skills)
		for _, skill := range skills {
			tier, resolved := cfg.Models.SkillOverrides[skill], cfg.ResolveTier(skill)
			if tier == resolved {
				fmt.Fprintf(w, "    %-12s → %s\n", skill, tier)
			} else {
				fmt.Fprintf(w, "    %-12s → %s (resolves to %s)\n", skill, tier, resolved)
			}
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Environment overrides:")
	printEnvironment(w, modelEnvironmentKeys, result.Environment, "    ")
	return nil
}

func renderModelsWrite(command *cobra.Command, output, tier, skill string, result configapp.ModelsWriteResult) error {
	if output == "json" {
		return writeJSON(command, result, "marshal models write result")
	}
	w := command.OutOrStdout()
	verb := "Set"
	if result.DryRun {
		verb = "Would set"
	}
	if tier != "" {
		fmt.Fprintf(w, "%s default model tier to %q\n", verb, tier)
	}
	if skill != "" {
		parts := splitSkill(skill)
		fmt.Fprintf(w, "%s skill %q tier to %q\n", verb, parts[0], parts[1])
	}
	return nil
}

func splitSkill(value string) []string {
	for index := 0; index < len(value); index++ {
		if value[index] == '=' {
			return []string{value[:index], value[index+1:]}
		}
	}
	return []string{value, ""}
}

func writeJSON(command *cobra.Command, value any, label string) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	_, err = fmt.Fprintln(command.OutOrStdout(), string(data))
	return err
}

func printConfigFile(w interface{ Write([]byte) (int, error) }, label, path string, exists bool) {
	if exists {
		fmt.Fprintf(w, "  ✓ %s %s\n", label, path)
	} else {
		fmt.Fprintf(w, "  ✗ %s %s (not found)\n", label, path)
	}
}

func printEnvironment(w interface{ Write([]byte) (int, error) }, keys []string, values map[string]string, indent string) {
	printed := false
	for _, key := range keys {
		if value := values[key]; value != "" {
			fmt.Fprintf(w, "%s%s=%s\n", indent, key, value)
			printed = true
		}
	}
	if !printed {
		fmt.Fprintf(w, "%s(none set)\n", indent)
	}
}

func staticValues(values ...string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	return func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return sorted, cobra.ShellCompDirectiveNoFileComp
	}
}

const configLong = `View and manage AgentOps configuration.

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
  AGENTOPS_RPI_RUNTIME_COMMAND - Runtime command used by legacy internal RPI paths (default: claude)
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
  ao config --show --json   # Output as JSON`

const modelsLong = `Display the current model cost tier settings with sources.

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
  AGENTOPS_COUNCIL_MODEL_TIER=quality`
