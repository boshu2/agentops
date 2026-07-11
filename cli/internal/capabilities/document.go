// Package capabilities owns the application policy for the machine-readable
// CLI contract. It is independent of Cobra and the host runtime.
package capabilities

import "github.com/boshu2/agentops/cli/internal/clicontract"

const ContractVersion = "1.1"

type Document struct {
	SchemaVersion    string                       `json:"schema_version"`
	Tool             string                       `json:"tool"`
	ToolVersion      string                       `json:"tool_version"`
	ContractVersion  string                       `json:"contract_version"`
	Platform         Platform                     `json:"platform"`
	GlobalFlags      []Flag                       `json:"global_flags"`
	OutputFormats    []string                     `json:"output_formats"`
	ExitCodes        map[string]string            `json:"exit_codes"`
	CommandExitCodes map[string]map[string]string `json:"command_exit_codes"`
	EnvVars          map[string]string            `json:"env_vars"`
	RobotSurfaces    map[string]string            `json:"robot_surfaces"`
	CommandGroups    []CommandGroup               `json:"command_groups"`
	Commands         []clicontract.Command        `json:"commands"`
}

type Platform struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

type Flag struct {
	Name        string `json:"name"`
	Shorthand   string `json:"shorthand,omitempty"`
	Description string `json:"description"`
}

type Command struct {
	Name  string `json:"name"`
	Short string `json:"short"`
}

type CommandGroup struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Commands []Command `json:"commands"`
}

type Snapshot struct {
	GlobalFlags   []Flag
	CommandGroups []CommandGroup
	Commands      []clicontract.Command
}

type SurfaceReader interface {
	Snapshot(commandExitCodes map[string]map[string]string) Snapshot
}

type PlatformReader interface {
	Platform() Platform
}

type Builder interface {
	Build() Document
}

type Service struct {
	toolVersion string
	surface     SurfaceReader
	platform    PlatformReader
}

func NewService(toolVersion string, surface SurfaceReader, platform PlatformReader) Service {
	return Service{toolVersion: toolVersion, surface: surface, platform: platform}
}

func (service Service) Build() Document {
	commandExits := commandExitCodes()
	snapshot := service.surface.Snapshot(commandExits)
	return Document{
		SchemaVersion:    ContractVersion,
		Tool:             "ao",
		ToolVersion:      service.toolVersion,
		ContractVersion:  ContractVersion,
		Platform:         service.platform.Platform(),
		GlobalFlags:      snapshot.GlobalFlags,
		OutputFormats:    []string{"table", "json", "yaml"},
		ExitCodes:        exitCodes(),
		CommandExitCodes: commandExits,
		EnvVars:          envVars(),
		RobotSurfaces:    robotSurfaces(),
		CommandGroups:    snapshot.CommandGroups,
		Commands:         snapshot.Commands,
	}
}

func exitCodes() map[string]string {
	return map[string]string{
		"0": "success",
		"1": "error: usage error, runtime failure, or — for diagnostic commands — findings present",
		"2": "diagnostic: partial result or bead claimed (command-specific; see that command's --help)",
	}
}

func commandExitCodes() map[string]map[string]string {
	return map[string]map[string]string{
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
}

func envVars() map[string]string {
	return map[string]string{
		"AGENTOPS_CONFIG":     "path to the config file (overridden by --config)",
		"NO_COLOR":            "disable ANSI styling on all output",
		"AO_DOCTOR_LOG_LEVEL": "trace|debug|info|warn|error — verbosity of the doctor surface",
	}
}

func robotSurfaces() map[string]string {
	return map[string]string{
		"capabilities":        "ao capabilities — this contract",
		"robot_docs":          "ao robot-docs — paste-ready agent handbook (Markdown)",
		"doctor_triage":       "ao doctor --robot-triage — mega-command health triage JSON",
		"doctor_capabilities": "ao doctor capabilities — extended doctor contract + exit codes",
		"json_everywhere":     "append --json (or -o json) to any read-side command for structured output",
	}
}
