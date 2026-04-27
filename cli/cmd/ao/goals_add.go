package main

import (
	"context"
	"time"

	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/spf13/cobra"
)

var (
	goalsAddWeight      int
	goalsAddType        string
	goalsAddDescription string
)

var goalsAddCmd = &cobra.Command{
	Use:     "add <id> <check-command>",
	Aliases: []string{"a"},
	Short:   "Add a new goal",
	GroupID: "management",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return goals.RunAdd(context.Background(), goals.AddOptions{
			ID:          args[0],
			Check:       args[1],
			Weight:      goalsAddWeight,
			Type:        goalsAddType,
			Description: goalsAddDescription,
			GoalsFile:   resolveGoalsFile(),
			Timeout:     time.Duration(goalsTimeout) * time.Second,
			DryRun:      dryRun,
		})
	},
}

func init() {
	goalsAddCmd.Flags().IntVar(&goalsAddWeight, "weight", 5, "Goal weight (1-10)")
	goalsAddCmd.Flags().StringVar(&goalsAddType, "type", "", "Goal type (health, architecture, quality, meta)")
	goalsAddCmd.Flags().StringVar(&goalsAddDescription, "description", "", "Goal description")
	_ = goalsAddCmd.RegisterFlagCompletionFunc("type", staticCompletionFunc("health", "architecture", "quality", "meta"))
	goalsCmd.AddCommand(goalsAddCmd)
}
