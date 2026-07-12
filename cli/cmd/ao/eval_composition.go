package main

import (
	"github.com/spf13/cobra"

	evaladapter "github.com/boshu2/agentops/cli/internal/adapters/eval"
	evalcommand "github.com/boshu2/agentops/cli/internal/commands/eval"
	aoeval "github.com/boshu2/agentops/cli/internal/eval"
)

func newEvalCommand() *cobra.Command {
	runtime := evaladapter.Runtime{}
	return evalcommand.NewModule(evalcommand.UseCases{
		Core:       aoeval.CoreService{Runtime: runtime},
		Cleanup:    aoeval.CleanupService{Runtime: runtime},
		Task:       aoeval.TaskService{Runtime: runtime},
		Suite:      aoeval.SuiteService{Runtime: runtime},
		Outcomes:   aoeval.OutcomesService{Runtime: runtime},
		Scenario:   aoeval.ScenarioService{Runtime: runtime},
		ScenarioAB: aoeval.ScenarioABService{Runtime: runtime},
		Aliases:    evalAliasAdapter{},
		Bench:      evalBenchAdapter{},
	}, newEvalHostOptions()).Command()
}

func newEvalHostOptions() evalcommand.HostOptions {
	return evalcommand.HostOptions{
		OutputMode:  func(*cobra.Command) string { return GetOutput() },
		Verbose:     func(*cobra.Command) bool { return GetVerbose() },
		ProjectRoot: measureProjectRoot,
		GoalsPath:   resolveGoalsFile,
		DryRun:      func(*cobra.Command) bool { return GetDryRun() },
	}
}

func init() { rootCmd.AddCommand(newEvalCommand()) }
