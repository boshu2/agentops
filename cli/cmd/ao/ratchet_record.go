// practices: [dora-metrics, continuous-delivery]
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

func init() {
	recordSubCmd := &cobra.Command{
		Use:     "record <step>",
		GroupID: "progression",
		Short:   "Record step completion",
		Long: `Record that a workflow step has been completed.

This locks progress - the ratchet engages.

Examples:
  ao ratchet record research --output .agents/research/topic.md
  ao ratchet record plan --input .agents/specs/spec-v2.md --output epic:ol-0001
  ao ratchet record implement --output issue:ol-0002 --tier 1`,
		Args: cobra.ExactArgs(1),
		RunE: runRatchetRecord,
	}
	recordSubCmd.Flags().StringVar(&ratchetInput, "input", "", "Input artifact path")
	recordSubCmd.Flags().StringVar(&ratchetOutput, "output", "", "Output artifact path (required)")
	recordSubCmd.Flags().IntVar(&ratchetTier, "tier", -1, "Quality tier (0-4)")
	recordSubCmd.Flags().BoolVar(&ratchetLock, "lock", true, "Lock the step (engage ratchet)")
	recordSubCmd.Flags().IntVar(&ratchetCycle, "cycle", 0, "RPI cycle number (1 for first, 2+ for iterations)")
	recordSubCmd.Flags().StringVar(&ratchetParentEpic, "parent-epic", "", "Parent epic ID from prior RPI cycle")
	_ = recordSubCmd.MarkFlagRequired("output") //nolint:errcheck
	ratchetCmd.AddCommand(recordSubCmd)
}

// runRatchetRecord records step completion.
func runRatchetRecord(cmd *cobra.Command, args []string) error {
	stepName := args[0]
	step := ratchet.ParseStep(stepName)
	if step == "" {
		return fmt.Errorf("unknown step: %s", stepName)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	if GetDryRun() {
		fmt.Printf("Would record step: %s\n", step)
		fmt.Printf("  Input: %s\n", ratchetInput)
		fmt.Printf("  Output: %s\n", ratchetOutput)
		fmt.Printf("  Locked: %v\n", ratchetLock)
		return nil
	}

	chain, err := ratchet.LoadChain(cwd)
	if err != nil {
		return fmt.Errorf("load chain: %w", err)
	}

	entry := ratchet.ChainEntry{
		Step:       step,
		Timestamp:  time.Now(),
		Input:      ratchetInput,
		Output:     ratchetOutput,
		Locked:     ratchetLock,
		Cycle:      ratchetCycle,
		ParentEpic: ratchetParentEpic,
	}

	if ratchetTier >= 0 && ratchetTier <= 4 {
		tier := ratchet.Tier(ratchetTier)
		entry.Tier = &tier
	}

	if err := chain.Append(entry); err != nil {
		return fmt.Errorf("record entry: %w", err)
	}

	fmt.Printf("Recorded: %s → %s\n", step, ratchetOutput)
	if ratchetLock {
		fmt.Println("Ratchet engaged ✓")
	}

	return nil
}
