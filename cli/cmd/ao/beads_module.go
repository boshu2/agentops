package main

import (
	beadsadapter "github.com/boshu2/agentops/cli/internal/adapters/beads"
	beadsapp "github.com/boshu2/agentops/cli/internal/beads"
	"github.com/boshu2/agentops/cli/internal/clicontract"
	beadscommands "github.com/boshu2/agentops/cli/internal/commands/beads"
)

func init() {
	tracker := currentBeadsTracker()
	runtime := beadsadapter.NewRuntime()
	repository := beadsadapter.NewKnowledgeRepository()
	knowledge := beadsapp.KnowledgeService{Tracker: tracker, Repository: repository, Clock: runtime}
	hygiene := beadsapp.HygieneService{Repository: beadsadapter.NewHygieneRepository(tracker)}
	scenario := beadsapp.ScenarioService{Repository: beadsadapter.NewScenarioRepository(tracker)}
	acceptance := beadsapp.AcceptanceService{Repository: beadsadapter.NewAcceptanceRepository(tracker)}
	directory := beadsapp.DirectoryService{Resolver: tracker, Inspector: tracker}
	recovery := beadsapp.RecoveryService{StaleSource: tracker, Claims: tracker, Runtime: runtime, Resolver: tracker, Reader: runtime}
	module := beadscommands.NewModule(
		tracker, beadsadapter.NewExecutor(tracker), directory, recovery,
		knowledge, hygiene, scenario, acceptance,
	)
	command := module.Command()
	command.GroupID = "knowledge"
	if err := clicontract.Attach(command, module.Contract()); err != nil {
		panic(err)
	}
	rootCmd.AddCommand(command)
}
