// Package config owns Cobra presentation for the config command family.
package config

import (
	"context"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	configapp "github.com/boshu2/agentops/cli/internal/config"
)

// showEnvironmentKeys deliberately omits AGENTOPS_NO_SC: it only toggles the
// dead Search.UseSmartConnections config field (no consumer outside config
// plumbing), so surfacing it would document a knob that does nothing.
var showEnvironmentKeys = []string{
	"AGENTOPS_CONFIG", "AGENTOPS_OUTPUT", "AGENTOPS_BASE_DIR", "AGENTOPS_VERBOSE",
}

var modelEnvironmentKeys = []string{"AGENTOPS_MODEL_TIER", "AGENTOPS_COUNCIL_MODEL_TIER", "COUNCIL_CLAUDE_MODEL"}

type UseCases interface {
	Show(context.Context, string, bool) (configapp.ShowResult, error)
	Models(context.Context) (configapp.ModelsResult, error)
	WriteModels(context.Context, configapp.ModelsWriteRequest) (configapp.ModelsWriteResult, error)
}

type Module struct {
	useCases UseCases
	host     clicontract.HostOptions
}

type options struct {
	show     bool
	setTier  string
	setSkill string
}

func NewModule(useCases UseCases, host clicontract.HostOptions) Module {
	return Module{useCases: useCases, host: host}
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
		// Only attribute the output format to "flag" when -o/--json was
		// actually passed; the host seam returns the cobra default ("table")
		// even when no flag was set, which used to misattribute config- or
		// default-sourced values as "(from flag)".
		result, err := module.useCases.Show(command.Context(), outputFlagValue(command, module.host.OutputMode()), module.host.Verbose())
		if err != nil {
			return err
		}
		return renderShow(command, module.host.OutputMode(), result)
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
				DefaultTier: commandOptions.setTier, Skill: commandOptions.setSkill, DryRun: module.host.DryRun != nil && module.host.DryRun(),
			})
			if err != nil {
				return err
			}
			return renderModelsWrite(command, module.host.OutputMode(), commandOptions.setTier, commandOptions.setSkill, result)
		}
		result, err := module.useCases.Models(command.Context())
		if err != nil {
			return err
		}
		return renderModels(command, module.host.OutputMode(), result)
	}
	return command
}

// outputFlagValue returns the negotiated output mode only when the user
// actually passed -o/--output or --json on the command line; otherwise it
// returns "" so resolution falls through to env, config files, and defaults.
func outputFlagValue(command *cobra.Command, negotiated string) string {
	for _, name := range []string{"output", "json"} {
		if flag := command.Flags().Lookup(name); flag != nil && flag.Changed {
			return negotiated
		}
	}
	return ""
}

func renderShow(command *cobra.Command, output string, result configapp.ShowResult) error {
	if output == "yaml" {
		return writeYAML(command, result.Resolved, "marshal config")
	}
	if output == "json" {
		return writeJSON(command, result.Resolved, "marshal config")
	}
	w := command.OutOrStdout()
	fmt.Fprintln(w, "AgentOps Configuration")
	fmt.Fprintln(w, "=====================")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Config files:")
	files := result.ConfigFiles
	printConfigLayer(w, "Home:   ", files.HomePath, files.HomeExists, files.HomeReadPath, files.HomeLegacy)
	printConfigLayer(w, "Project:", files.ProjectPath, files.ProjectExists, files.ProjectReadPath, files.ProjectLegacy)
	r := result.Resolved
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Resolved values:")
	fmt.Fprintf(w, "  output:   %v  (from %s)\n", r.Output.Value, r.Output.Source)
	fmt.Fprintf(w, "  base_dir: %v  (from %s)\n", r.BaseDir.Value, r.BaseDir.Source)
	fmt.Fprintf(w, "  verbose:  %v  (from %s)\n", r.Verbose.Value, r.Verbose.Source)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Environment variables (if set):")
	printEnvironment(w, showEnvironmentKeys, result.Environment, "  ")
	return nil
}

// printConfigLayer renders one config-file line. When the layer was read
// through a deprecated legacy fallback it shows the path actually loaded,
// labeled as deprecated, instead of reporting the canonical path as missing
// right after the loader warned about the legacy read.
func printConfigLayer(w interface{ Write([]byte) (int, error) }, label, path string, exists bool, readPath string, legacy bool) {
	if legacy && readPath != "" {
		fmt.Fprintf(w, "  ✓ %s %s (deprecated location; move to %s)\n", label, readPath, path)
		return
	}
	printConfigFile(w, label, path, exists)
}

func renderModels(command *cobra.Command, output string, result configapp.ModelsResult) error {
	if output == "yaml" {
		return writeYAML(command, result.Config.Models, "marshal models config")
	}
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
	if output == "yaml" {
		return writeYAML(command, result, "marshal models write result")
	}
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
	if err := clicontract.WriteJSON(command.OutOrStdout(), value); err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	return nil
}

func writeYAML(command *cobra.Command, value any, label string) error {
	if err := clicontract.WriteYAML(command.OutOrStdout(), value); err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	return nil
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
  3. Project config (.agents/ao/config.yaml)
  4. Home config (~/.agents/ao/config.yaml)
  5. Defaults

Environment variables:
  AGENTOPS_CONFIG     - Explicit config file path (overrides default project config location)
  AGENTOPS_OUTPUT     - Default output format (table, json, yaml)
  AGENTOPS_BASE_DIR   - Data directory path
  AGENTOPS_VERBOSE    - Enable verbose output (true/1)

Examples:
  ao config --show           # Show resolved configuration
  ao config --show --json   # Output as JSON`

const modelsLong = `Display the current model cost tier settings with sources.

Cost tiers map to model quality levels:
  quality  → opus   (high-stakes decisions, architecture)
  balanced → sonnet (default, routine reviews)
  budget   → haiku  (quick checks, simple tasks)
  inherit  → uses default tier (falls back to balanced)

Configure in .agents/ao/config.yaml:
  models:
    default_tier: balanced
    skill_overrides:
      council: quality
      crank: budget

Or via environment variables:
  AGENTOPS_MODEL_TIER=budget
  AGENTOPS_COUNCIL_MODEL_TIER=quality`
