// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	counciladapter "github.com/boshu2/agentops/cli/internal/adapters/council_gate"
	"github.com/boshu2/agentops/cli/internal/clicontract"
	councilcommands "github.com/boshu2/agentops/cli/internal/commands/council_gate"
	"github.com/boshu2/agentops/cli/internal/councilgate"
)

func newCouncilGateModule(workDir string, requireCrossFamily bool) councilcommands.Module {
	reader := counciladapter.Reader{WorkDir: workDir}
	service := councilgate.NewService(reader, councilgate.Policy{RequireCrossFamily: requireCrossFamily})
	return councilcommands.NewModule(service)
}

func systemCouncilGateModule() councilcommands.Module {
	service := councilgate.NewService(counciladapter.Reader{}, councilgate.Policy{})
	return councilcommands.NewModule(service)
}

var councilGateModule = systemCouncilGateModule()
var councilGateCommand = councilGateModule.Command()

func runCouncilGate(rt tickRuntime, paths []string) error {
	command := newCouncilGateModule(rt.workDir, rt.requireCrossFamily).Command()
	command.SetIn(rt.stdin)
	command.SetOut(rt.stdout)
	command.SetErr(rt.stderr)
	command.SetArgs(paths)
	return command.ExecuteContext(rootCmd.Context())
}

func init() {
	if err := clicontract.Attach(councilGateCommand, councilGateModule.Contract()); err != nil {
		panic(err)
	}
	rootCmd.AddCommand(councilGateCommand)
}
