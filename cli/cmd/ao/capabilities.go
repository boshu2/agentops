// practices: [twelve-factor-app, ai-assisted-dev, pragmatic-programmer]
package main

import (
	"encoding/json"
	"runtime"
	"sort"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

// capabilitiesContractVersion is the version of the top-level capabilities
// contract. Bump it on any breaking change to the JSON shape so agents can
// pin against a known schema.
const capabilitiesContractVersion = "1.1"

// capabilitiesDoc is the machine-readable contract for the whole `ao` CLI,
// emitted by `ao capabilities`. It lets an agent read the command surface,
// flag conventions, and exit-code dictionary straight from the tool instead
// of relying on an out-of-band doc lookup.
type capabilitiesDoc struct {
	SchemaVersion   string            `json:"schema_version"`
	Tool            string            `json:"tool"`
	ToolVersion     string            `json:"tool_version"`
	ContractVersion string            `json:"contract_version"`
	Platform        capPlatform       `json:"platform"`
	GlobalFlags     []capFlag         `json:"global_flags"`
	OutputFormats   []string          `json:"output_formats"`
	ExitCodes       map[string]string `json:"exit_codes"`
	// CommandExitCodes documents the per-command typed exit codes beyond the
	// global {0,1,2} — the codes an agent must interpret to branch on a verdict
	// (e.g. pawl REFUTED=3, plan-pawl BLOCKED=4, governor HARDEN=3). Only
	// default-build commands are listed; legacy/flywheel-tagged commands (tick,
	// corpus-scan) are omitted because they are absent from the shipping binary.
	CommandExitCodes map[string]map[string]string `json:"command_exit_codes"`
	EnvVars          map[string]string            `json:"env_vars"`
	RobotSurfaces    map[string]string            `json:"robot_surfaces"`
	CommandGroups    []capCommandGroup            `json:"command_groups"`
	Commands         []clicontract.Command        `json:"commands"`
}

type capPlatform struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

type capFlag struct {
	Name        string `json:"name"`
	Shorthand   string `json:"shorthand,omitempty"`
	Description string `json:"description"`
}

type capCommand struct {
	Name  string `json:"name"`
	Short string `json:"short"`
}

type capCommandGroup struct {
	ID       string       `json:"id"`
	Title    string       `json:"title"`
	Commands []capCommand `json:"commands"`
}

// capabilitiesExitCodes documents the exit codes an agent can rely on across
// the CLI. Diagnostic commands (doctor, goals) layer an extended dictionary on
// top — see robot_surfaces for where to read it.
var capabilitiesExitCodes = map[string]string{
	"0": "success",
	"1": "error: usage error, runtime failure, or — for diagnostic commands — findings present",
	"2": "diagnostic: partial result or bead claimed (command-specific; see that command's --help)",
}

// capabilitiesCommandExitCodes documents per-command typed exit codes beyond the
// global {0,1,2} dictionary — the codes `Execute()` propagates from typed errors
// so a shell/agent can branch on the verdict without parsing stdout (recon
// 2026-07-02 audit A7). Values are kept in lockstep with the code's own exit-code
// constants by TestCapabilitiesCommandExitCodesMatchConstants. Only default-build
// commands appear: `tick` (//go:build legacy) and `corpus-scan` (//go:build
// flywheel) are deliberately omitted — documenting codes for commands absent from
// the shipping binary would be the exact contract inaccuracy this fix closes.
var capabilitiesCommandExitCodes = map[string]map[string]string{
	"pawl review": {
		"0": "CONFIRMED — cross-family verdict written",
		"1": "hard error",
		"2": "usage error",
		"3": "REFUTED — auto-redo",
		"4": "--converge advisory-only (no lineage written)",
	},
	"plan-pawl decide": {
		"0": "PASS — the door opens",
		"2": "usage error",
		"3": "REDO — auto-redo loop (no human)",
		"4": "BLOCKED — a circuit breaker tripped (andon)",
		"5": "DEGRADED — transient panel lane loss below quorum; retry the panel",
	},
	"governor budget": {
		"0": "ship — within error budget",
		"3": "HARDEN — burn-rate over tolerance; stop the line",
	},
}

// capabilitiesEnvVars documents the environment variables the CLI honors.
var capabilitiesEnvVars = map[string]string{
	"AGENTOPS_CONFIG":     "path to the config file (overridden by --config)",
	"NO_COLOR":            "disable ANSI styling on all output",
	"AO_DOCTOR_LOG_LEVEL": "trace|debug|info|warn|error — verbosity of the doctor surface",
}

// capabilitiesRobotSurfaces points an agent at every machine-readable surface.
var capabilitiesRobotSurfaces = map[string]string{
	"capabilities":        "ao capabilities — this contract",
	"robot_docs":          "ao robot-docs — paste-ready agent handbook (Markdown)",
	"doctor_triage":       "ao doctor --robot-triage — mega-command health triage JSON",
	"doctor_capabilities": "ao doctor capabilities — extended doctor contract + exit codes",
	"json_everywhere":     "append --json (or -o json) to any read-side command for structured output",
}

var capabilitiesCmd = &cobra.Command{
	Use:   "capabilities",
	Short: "Print the machine-readable CLI contract (JSON)",
	Long: `Print the machine-readable contract for the whole ao CLI as JSON.

This is the first command an agent should run to discover the command
surface, flag conventions, exit-code dictionary, and every other
machine-readable surface — no external documentation lookup required.

Output is always JSON; it is stable across patch versions (pinned by
contract_version).`,
	RunE: runCapabilities,
}

func init() {
	capabilitiesCmd.GroupID = "core"
	rootCmd.AddCommand(capabilitiesCmd)
}

// buildCapabilitiesDoc assembles the capabilities contract by walking the
// live command tree, so it never drifts from the registered commands.
func buildCapabilitiesDoc() capabilitiesDoc {
	groups := map[string]*capCommandGroup{}
	var order []string
	for _, g := range rootCmd.Groups() {
		groups[g.ID] = &capCommandGroup{ID: g.ID, Title: g.Title}
		order = append(order, g.ID)
	}
	const ungrouped = ""
	groups[ungrouped] = &capCommandGroup{ID: ungrouped, Title: "Additional Commands:"}
	order = append(order, ungrouped)

	for _, c := range rootCmd.Commands() {
		if c.Hidden || c.Name() == "help" || c.Name() == "completion" {
			continue
		}
		g, ok := groups[c.GroupID]
		if !ok {
			g = groups[ungrouped]
		}
		g.Commands = append(g.Commands, capCommand{Name: c.Name(), Short: c.Short})
	}

	var commandGroups []capCommandGroup
	for _, id := range order {
		g := groups[id]
		if len(g.Commands) == 0 {
			continue
		}
		sort.Slice(g.Commands, func(i, j int) bool { return g.Commands[i].Name < g.Commands[j].Name })
		commandGroups = append(commandGroups, *g)
	}

	var globalFlags []capFlag
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		globalFlags = append(globalFlags, capFlag{
			Name:        f.Name,
			Shorthand:   f.Shorthand,
			Description: f.Usage,
		})
	})
	sort.Slice(globalFlags, func(i, j int) bool { return globalFlags[i].Name < globalFlags[j].Name })

	return capabilitiesDoc{
		SchemaVersion:    capabilitiesContractVersion,
		Tool:             "ao",
		ToolVersion:      version,
		ContractVersion:  capabilitiesContractVersion,
		Platform:         capPlatform{OS: runtime.GOOS, Arch: runtime.GOARCH},
		GlobalFlags:      globalFlags,
		OutputFormats:    []string{"table", "json", "yaml"},
		ExitCodes:        capabilitiesExitCodes,
		CommandExitCodes: capabilitiesCommandExitCodes,
		EnvVars:          capabilitiesEnvVars,
		RobotSurfaces:    capabilitiesRobotSurfaces,
		CommandGroups:    commandGroups,
		Commands:         clicontract.Inspect(rootCmd, capabilitiesCommandExitCodes),
	}
}

func runCapabilities(cmd *cobra.Command, _ []string) error {
	if GetOutput() == "yaml" {
		data, err := yaml.Marshal(buildCapabilitiesDoc())
		if err != nil {
			return err
		}
		_, err = cmd.OutOrStdout().Write(data)
		return err
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(buildCapabilitiesDoc())
}
