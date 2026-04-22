package main

import (
	"io"
	"os"

	"github.com/boshu2/agentops/cli/embedded"
	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/spf13/cobra"
)

var goalsInitNonInteractive bool
var goalsInitTemplate string

// Type aliases for test compatibility.
type goalTemplate = goals.GoalTemplate
type goalTemplateGate = goals.GoalTemplateGate

var validTemplateNames = goals.ValidTemplateNames

var goalsInitCmd = &cobra.Command{
	Use:     "init",
	Short:   "Bootstrap a new GOALS.md file",
	GroupID: "management",
	RunE: func(cmd *cobra.Command, args []string) error {
		return goals.RunInit(goals.InitOptions{
			NonInteractive: goalsInitNonInteractive,
			Template:       goalsInitTemplate,
			GoalsFile:      resolveGoalsFile(),
			JSON:           goalsJSON,
			DryRun:         dryRun,
			Stdin:          os.Stdin,
			TemplatesFS:    embedded.TemplatesFS,
		})
	},
}

// Thin wrappers for test compatibility.

func buildDefaultGoalFile() *goals.GoalFile          { return goals.BuildDefaultGoalFile() }
func buildInteractiveGoalFile(r io.Reader) (*goals.GoalFile, error) { return goals.BuildInteractiveGoalFile(r) }
func detectGates(root string) []goals.Goal            { return goals.DetectGates(root) }
func autoDetectTemplate(root string) string           { return goals.AutoDetectTemplate(root) }

func loadTemplate(name string) (*goalTemplate, error) {
	return goals.LoadTemplate(embedded.TemplatesFS, name)
}

func templateGatesToGoals(tmpl *goalTemplate) []goals.Goal {
	return goals.TemplateGatesToGoals(tmpl)
}

func init() {
	goalsInitCmd.Flags().BoolVar(&goalsInitNonInteractive, "non-interactive", false, "Use defaults without prompting")
	goalsInitCmd.Flags().StringVar(&goalsInitTemplate, "template", "", "Goal template (go-cli, python-lib, web-app, rust-cli, generic)")
	goalsCmd.AddCommand(goalsInitCmd)

	_ = goalsInitCmd.RegisterFlagCompletionFunc("template", staticCompletionFunc(templateCompletionValues()...))
}
