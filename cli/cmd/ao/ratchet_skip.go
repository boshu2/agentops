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
	skipSubCmd := &cobra.Command{
		Use:     "skip <step>",
		GroupID: "progression",
		Short:   "Record intentional skip",
		Long: `Record that a step was intentionally skipped.

Use this for valid workflow variations (e.g., skipping pre-mortem for bug fixes).

Examples:
  ao ratchet skip pre-mortem --reason "Bug fix, no spec needed"
  ao ratchet skip research --reason "Existing knowledge sufficient"`,
		Args: cobra.ExactArgs(1),
		RunE: runRatchetSkip,
	}
	skipSubCmd.Flags().StringVar(&ratchetReason, "reason", "", "Reason for skipping (required)")
	_ = skipSubCmd.MarkFlagRequired("reason") //nolint:errcheck
	ratchetCmd.AddCommand(skipSubCmd)
}

// runRatchetSkip records an intentional skip.
func runRatchetSkip(cmd *cobra.Command, args []string) error {
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
		fmt.Printf("Would skip step: %s\n", step)
		fmt.Printf("  Reason: %s\n", ratchetReason)
		return nil
	}

	chain, err := ratchet.LoadChain(cwd)
	if err != nil {
		return fmt.Errorf("load chain: %w", err)
	}

	entry := ratchet.ChainEntry{
		Step:      step,
		Timestamp: time.Now(),
		Skipped:   true,
		Reason:    ratchetReason,
		Locked:    true, // Skips are also locked
	}

	if err := chain.Append(entry); err != nil {
		return fmt.Errorf("record skip: %w", err)
	}

	fmt.Printf("Skipped: %s (reason: %s)\n", step, ratchetReason)

	return nil
}
