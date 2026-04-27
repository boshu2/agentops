package main

import (
	"fmt"
	"strconv"

	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/spf13/cobra"
)

// validSteers delegates to goals.ValidSteers (used by tests).
var validSteers = goals.ValidSteers

var goalsSteerCmd = &cobra.Command{
	Use:     "steer",
	Short:   "Manage directives",
	GroupID: "management",
}

var (
	steerAddDescription string
	steerAddSteer       string
)

var goalsSteerAddCmd = &cobra.Command{
	Use:   "add <title>",
	Short: "Add a new directive",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return goals.RunSteerAdd(goals.SteerAddOptions{
			Title:       args[0],
			Description: steerAddDescription,
			Steer:       steerAddSteer,
			GoalsFile:   resolveGoalsFile(),
			JSON:        goalsJSON,
			DryRun:      dryRun,
		})
	},
}

var goalsSteerRemoveCmd = &cobra.Command{
	Use:   "remove <number>",
	Short: "Remove a directive by number",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		num, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("directive number must be an integer: %w", err)
		}
		return goals.RunSteerRemove(goals.SteerRemoveOptions{
			Number:    num,
			GoalsFile: resolveGoalsFile(),
			JSON:      goalsJSON,
			DryRun:    dryRun,
		})
	},
}

var goalsSteerPrioritizeCmd = &cobra.Command{
	Use:   "prioritize <number> <new-position>",
	Short: "Move a directive to a new position",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		num, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("directive number must be an integer: %w", err)
		}
		newPos, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("new position must be an integer: %w", err)
		}
		return goals.RunSteerPrioritize(goals.SteerPrioritizeOptions{
			Number:      num,
			NewPosition: newPos,
			GoalsFile:   resolveGoalsFile(),
			JSON:        goalsJSON,
			DryRun:      dryRun,
		})
	},
}

// loadMDGoals delegates to goals.LoadMDGoals (used by tests).
func loadMDGoals() (*goals.GoalFile, string, error) {
	return goals.LoadMDGoals(resolveGoalsFile())
}

// writeMDGoals delegates to goals.WriteMDGoals (used by tests).
func writeMDGoals(gf *goals.GoalFile, path string) error {
	return goals.WriteMDGoals(gf, path)
}

func init() {
	goalsSteerAddCmd.Flags().StringVar(&steerAddDescription, "description", "", "Directive description (required)")
	_ = goalsSteerAddCmd.MarkFlagRequired("description")
	goalsSteerAddCmd.Flags().StringVar(&steerAddSteer, "steer", "increase", "Steer direction (increase, decrease, hold, explore)")
	_ = goalsSteerAddCmd.RegisterFlagCompletionFunc("steer", staticCompletionFunc("increase", "decrease", "hold", "explore"))

	goalsSteerCmd.AddCommand(goalsSteerAddCmd)
	goalsSteerCmd.AddCommand(goalsSteerRemoveCmd)
	goalsSteerCmd.AddCommand(goalsSteerPrioritizeCmd)
	goalsCmd.AddCommand(goalsSteerCmd)
}
