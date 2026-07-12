// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"github.com/spf13/cobra"

	goalsadapter "github.com/boshu2/agentops/cli/internal/adapters/goals"
	goalscommand "github.com/boshu2/agentops/cli/internal/commands/goals"
	"github.com/boshu2/agentops/cli/internal/goalsapp"
)

// newGoalsCommand composes the replacement owner without registering it. The
// legacy goals tree remains the sole live owner until the atomic cutover.
func newGoalsCommand() *cobra.Command {
	resolver := goalsadapter.PathResolver{}
	return goalscommand.NewModule(goalscommand.UseCases{
		Simple: goalsapp.SimpleService{},
	}, goalscommand.HostOptions{
		OutputMode:       GetOutput,
		ResolveGoalsPath: resolver.Resolve,
		TemplateValues:   templateCompletionValues,
	}).Command()
}
