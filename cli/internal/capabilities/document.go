// Package capabilities builds the machine-readable contract from the live
// Cobra surface. It does not advertise removed lifecycle controllers.
package capabilities

import "github.com/boshu2/agentops/cli/internal/clicontract"

const ContractVersion = "2.0"

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
	commandExits := map[string]map[string]string{}
	snapshot := service.surface.Snapshot(commandExits)
	return Document{
		SchemaVersion:    ContractVersion,
		Tool:             "ao",
		ToolVersion:      service.toolVersion,
		ContractVersion:  ContractVersion,
		Platform:         service.platform.Platform(),
		GlobalFlags:      snapshot.GlobalFlags,
		OutputFormats:    []string{"table", "json", "yaml"},
		ExitCodes:        map[string]string{"0": "success", "1": "failure or findings", "2": "partial diagnostic or usage failure"},
		CommandExitCodes: commandExits,
		EnvVars: map[string]string{
			"AGENTOPS_CONFIG":     "path to the ao config file (overridden by --config)",
			"AGENTOPS_GATE_RANGE": "optional explicit changed-path range for deterministic checks",
			"AGENTOPS_OUTPUT":     "default structured output format",
			"AO_BIN":              "explicit ao binary used by deterministic repository scripts",
			"AO_HOME":             "optional local evidence home",
			"NO_COLOR":            "disable ANSI styling",
		},
		RobotSurfaces: map[string]string{
			"capabilities":    "ao capabilities — this contract",
			"robot_docs":      "ao robot-docs — generated CLI handbook",
			"json_everywhere": "append --json (or -o json) to read-side commands",
		},
		CommandGroups: snapshot.CommandGroups,
		Commands:      snapshot.Commands,
	}
}
