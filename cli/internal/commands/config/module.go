// Package config owns Cobra presentation for the config command family.
package config

import (
	"context"
	"fmt"

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

type UseCases interface {
	Show(context.Context, string, bool) (configapp.ShowResult, error)
}

type Module struct {
	useCases UseCases
	host     clicontract.HostOptions
}

type options struct {
	show bool
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
	return root
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
