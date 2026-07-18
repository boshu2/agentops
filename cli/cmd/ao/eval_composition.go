// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"github.com/spf13/cobra"

	evaladapter "github.com/boshu2/agentops/cli/internal/adapters/eval"
	"github.com/boshu2/agentops/cli/internal/clicontract"
	evalcommands "github.com/boshu2/agentops/cli/internal/commands/eval"
	evalapp "github.com/boshu2/agentops/cli/internal/eval"
)

func init() {
	rootCmd.AddCommand(newEvalCommand())
}

// newEvalCommand wires the eval measurement surface: deterministic suite runs,
// run-record comparison, locked Tasks, holdout scenarios, and A/B verdicts.
// Eval is the Learn seat of the operating loop — a read/measure consumer of
// evidence that reports numbers and never retries, schedules, or promotes
// anything on its own. The Aliases and Bench seats are deliberately nil: their
// production implementations (session-outcome, chaos, bench) were retired with
// the legacy knowledge surface, and the module omits those subcommands when
// the seats are empty.
func newEvalCommand() *cobra.Command {
	runtime := evaladapter.Runtime{}
	module := evalcommands.NewModule(evalcommands.UseCases{
		Core:       evalapp.CoreService{Runtime: runtime},
		Cleanup:    evalapp.CleanupService{Runtime: runtime},
		Task:       evalapp.TaskService{Runtime: runtime},
		Suite:      evalapp.SuiteService{Runtime: runtime},
		Outcomes:   evalapp.OutcomesService{Runtime: runtime},
		Scenario:   evalapp.ScenarioService{Runtime: runtime},
		ScenarioAB: evalapp.ScenarioABService{Runtime: runtime},
	}, evalcommands.HostOptions{
		OutputMode: func(*cobra.Command) string { return GetOutput() },
		Verbose:    func(*cobra.Command) bool { return GetVerbose() },
		DryRun:     func(*cobra.Command) bool { return GetDryRun() },
		ProjectRoot: func() string {
			if dir, err := resolveProjectDir(); err == nil {
				return dir
			}
			return ""
		},
	})
	command := module.Command()
	if err := clicontract.Attach(command, module.Contract()); err != nil {
		panic(err)
	}
	return command
}
