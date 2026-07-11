// Package capabilities owns the Cobra presentation for the capabilities
// command. The handler delegates document construction and only renders it.
package capabilities

import (
	"encoding/json"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	capabilitiesapp "github.com/boshu2/agentops/cli/internal/capabilities"
	"github.com/boshu2/agentops/cli/internal/clicontract"
)

type Module struct {
	builder capabilitiesapp.Builder
	output  func() string
}

func NewModule(builder capabilitiesapp.Builder, output func() string) Module {
	return Module{builder: builder, output: output}
}

func (Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{
		ID: "ao.capabilities",
		Profiles: clicontract.ProfileDefault |
			clicontract.ProfileFlywheel |
			clicontract.ProfileLegacy |
			clicontract.ProfileCombined,
		Args:        clicontract.ArgsPolicy{Name: "none", Validate: cobra.NoArgs},
		Output:      clicontract.OutputStructured,
		Effects:     clicontract.EffectPure,
		ExitClasses: map[int]clicontract.ExitClass{0: clicontract.ExitSuccess, 1: clicontract.ExitFailure},
	}
}

func (module Module) Command() *cobra.Command {
	command := &cobra.Command{
		Use:   "capabilities",
		Short: "Print the machine-readable CLI contract (JSON)",
		Long: `Print the machine-readable contract for the whole ao CLI as JSON.

This is the first command an agent should run to discover the command
surface, flag conventions, exit-code dictionary, and every other
machine-readable surface — no external documentation lookup required.

Output is always JSON; it is stable across patch versions (pinned by
contract_version).`,
	}
	command.RunE = func(command *cobra.Command, _ []string) error {
		return module.render(command)
	}
	return command
}

func (module Module) render(command *cobra.Command) error {
	document := module.builder.Build()
	if module.output != nil && module.output() == "yaml" {
		data, err := yaml.Marshal(document)
		if err != nil {
			return err
		}
		_, err = command.OutOrStdout().Write(data)
		return err
	}
	encoder := json.NewEncoder(command.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(document)
}
