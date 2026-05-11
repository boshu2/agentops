// practices: [dora-metrics, continuous-delivery]
package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

func init() {
	promoteSubCmd := &cobra.Command{
		Use:     "promote <artifact>",
		Aliases: []string{"p"},
		GroupID: "progression",
		Short:   "Record tier promotion",
		Long: `Record promotion of an artifact to a higher tier.

Validates promotion requirements before recording.

Tiers:
  0: Observation (.agents/candidates/)
  1: Learning (.agents/learnings/) - requires 2+ citations
  2: Pattern (.agents/patterns/) - requires 3+ sessions
  3: Skill (plugins/*/skills/) - requires SKILL.md format
  4: Core (CLAUDE.md) - requires 10+ uses

Examples:
  ao ratchet promote .agents/candidates/insight.md --to 1
  ao ratchet promote .agents/learnings/pattern.md --to 2`,
		Args: cobra.ExactArgs(1),
		RunE: runRatchetPromote,
	}
	promoteSubCmd.Flags().IntVar(&ratchetTier, "to", -1, "Target tier (0-4, required)")
	_ = promoteSubCmd.MarkFlagRequired("to") //nolint:errcheck
	ratchetCmd.AddCommand(promoteSubCmd)
}

// runRatchetPromote records tier promotion.
func runRatchetPromote(cmd *cobra.Command, args []string) error {
	artifact := args[0]
	targetTier := ratchet.Tier(ratchetTier)

	if targetTier < 0 || targetTier > 4 {
		return fmt.Errorf("tier must be 0-4, got %d", ratchetTier)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	w := cmd.OutOrStdout()
	if err := validatePromotion(cwd, artifact, targetTier, w); err != nil {
		return err
	}

	if GetDryRun() {
		fmt.Fprintf(w, "Would promote %s to tier %d (%s)\n", artifact, targetTier, targetTier.String())
		return nil
	}

	return recordPromotion(cwd, artifact, targetTier, w)
}

// validatePromotion checks that an artifact meets promotion requirements.
func validatePromotion(cwd, artifact string, targetTier ratchet.Tier, w io.Writer) error {
	validator, err := ratchet.NewValidator(cwd)
	if err != nil {
		return fmt.Errorf("create validator: %w", err)
	}

	result, err := validator.ValidateForPromotion(artifact, targetTier)
	if err != nil {
		return fmt.Errorf("validate promotion: %w", err)
	}

	if result.Valid {
		return nil
	}

	fmt.Fprintln(w, "Promotion blocked:")
	for _, issue := range result.Issues {
		fmt.Fprintf(w, "  - %s\n", issue)
	}
	return fmt.Errorf("promotion blocked: requirements not met")
}

// recordPromotion appends a promotion entry to the ratchet chain.
func recordPromotion(cwd, artifact string, targetTier ratchet.Tier, w io.Writer) error {
	chain, err := ratchet.LoadChain(cwd)
	if err != nil {
		return fmt.Errorf("load chain: %w", err)
	}

	entry := ratchet.ChainEntry{
		Step:      ratchet.Step("promotion"),
		Timestamp: time.Now(),
		Input:     artifact,
		Output:    targetTier.Location(),
		Tier:      &targetTier,
		Locked:    true,
	}

	if err := chain.Append(entry); err != nil {
		return fmt.Errorf("record promotion: %w", err)
	}

	fmt.Fprintf(w, "Promoted: %s → tier %d (%s)\n", artifact, targetTier, targetTier.String())
	return nil
}
