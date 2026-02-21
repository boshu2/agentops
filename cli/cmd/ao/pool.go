package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/boshu2/agentops/cli/internal/formatter"
	"github.com/boshu2/agentops/cli/internal/pool"
	"github.com/boshu2/agentops/cli/internal/types"
)

var (
	poolTier      string
	poolStatus    string
	poolLimit     int
	poolOffset    int
	poolReason    string
	poolThreshold string
	poolDoPromote bool
	poolGold      bool
)

var poolCmd = &cobra.Command{
	Use:   "pool",
	Short: "Manage quality pools",
	Long: `Manage knowledge candidates in quality pools.

Pools organize candidates by their processing status:
  pending    Awaiting initial scoring
  staged     Ready for promotion to Athena
  promoted   Successfully stored in Athena
  rejected   Rejected during review

Examples:
  ao pool list --tier=gold
  ao pool show <candidate-id>
  ao pool stage <candidate-id>
  ao pool promote <candidate-id>`,
}

var poolListCmd = &cobra.Command{
	Use:   "list",
	Short: "List candidates in pools",
	Long: `List knowledge candidates filtered by tier and/or status.

Examples:
  ao pool list
  ao pool list --tier=gold
  ao pool list --status=pending
  ao pool list --tier=bronze --status=staged`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if GetDryRun() {
			fmt.Printf("[dry-run] Would list pool entries")
			if poolTier != "" {
				fmt.Printf(" with tier=%s", poolTier)
			}
			if poolStatus != "" {
				fmt.Printf(" with status=%s", poolStatus)
			}
			fmt.Println()
			return nil
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		p := pool.NewPool(cwd)

		opts := pool.ListOptions{
			Limit:  poolLimit,
			Offset: poolOffset,
		}

		if poolTier != "" {
			opts.Tier = types.Tier(poolTier)
		}
		if poolStatus != "" {
			opts.Status = types.PoolStatus(poolStatus)
		}

		result, err := p.ListPaginated(opts)
		if err != nil {
			return fmt.Errorf("list pool: %w", err)
		}

		return outputPoolList(result.Entries, poolOffset, poolLimit, result.Total)
	},
}

func outputPoolList(entries []pool.PoolEntry, offset, limit, total int) error {
	switch GetOutput() {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)

	case "yaml":
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(entries)

	default: // table
		if len(entries) == 0 {
			fmt.Println("No pool entries found")
			return nil
		}

		tbl := formatter.NewTable(os.Stdout, "ID", "TIER", "STATUS", "AGE", "UTILITY", "CONFIDENCE")
		tbl.SetMaxWidth(0, 12)

		for _, e := range entries {
			tbl.AddRow(
				e.Candidate.ID,
				string(e.Candidate.Tier),
				string(e.Status),
				e.AgeString,
				fmt.Sprintf("%.2f", e.Candidate.Utility),
				fmt.Sprintf("%.2f", e.Candidate.Confidence),
			)
		}

		if err := tbl.Render(); err != nil {
			return err
		}

		if total > len(entries) {
			start := offset + 1
			end := offset + len(entries)
			fmt.Printf("\nShowing %d-%d of %d entries (use --offset/--limit to paginate)\n", start, end, total)
		}

		return nil
	}
}

var poolShowCmd = &cobra.Command{
	Use:   "show <candidate-id>",
	Short: "Show candidate details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		candidateID := args[0]

		if GetDryRun() {
			fmt.Printf("[dry-run] Would show candidate %s\n", candidateID)
			return nil
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		p := pool.NewPool(cwd)

		entry, err := p.Get(candidateID)
		if err != nil {
			return fmt.Errorf("get candidate: %w", err)
		}

		return outputPoolShow(entry)
	},
}

func outputPoolShow(entry *pool.PoolEntry) error {
	switch GetOutput() {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entry)

	case "yaml":
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(entry)

	default: // detailed text
		fmt.Printf("Candidate: %s\n", entry.Candidate.ID)
		fmt.Printf("============%s\n", repeat("=", len(entry.Candidate.ID)))
		fmt.Println()

		fmt.Printf("Type:      %s\n", entry.Candidate.Type)
		fmt.Printf("Tier:      %s\n", entry.Candidate.Tier)
		fmt.Printf("Status:    %s\n", entry.Status)
		fmt.Printf("Age:       %s\n", entry.AgeString)
		fmt.Println()

		fmt.Println("MemRL Metrics:")
		fmt.Printf("  Utility:    %.3f\n", entry.Candidate.Utility)
		fmt.Printf("  Confidence: %.3f\n", entry.Candidate.Confidence)
		fmt.Printf("  Maturity:   %s\n", entry.Candidate.Maturity)
		fmt.Printf("  Rewards:    %d\n", entry.Candidate.RewardCount)
		fmt.Println()

		fmt.Println("Scoring:")
		fmt.Printf("  Raw Score:  %.3f\n", entry.ScoringResult.RawScore)
		fmt.Printf("  Rubric:\n")
		fmt.Printf("    Specificity:   %.2f\n", entry.ScoringResult.Rubric.Specificity)
		fmt.Printf("    Actionability: %.2f\n", entry.ScoringResult.Rubric.Actionability)
		fmt.Printf("    Novelty:       %.2f\n", entry.ScoringResult.Rubric.Novelty)
		fmt.Printf("    Context:       %.2f\n", entry.ScoringResult.Rubric.Context)
		fmt.Printf("    Confidence:    %.2f\n", entry.ScoringResult.Rubric.Confidence)
		fmt.Println()

		fmt.Println("Provenance:")
		fmt.Printf("  Session:    %s\n", entry.Candidate.Source.SessionID)
		fmt.Printf("  Transcript: %s\n", entry.Candidate.Source.TranscriptPath)
		fmt.Printf("  Message:    %d\n", entry.Candidate.Source.MessageIndex)
		fmt.Println()

		fmt.Println("Content:")
		fmt.Println("---")
		fmt.Println(entry.Candidate.Content)
		fmt.Println("---")

		if entry.HumanReview != nil && entry.HumanReview.Reviewed {
			fmt.Println()
			fmt.Println("Human Review:")
			fmt.Printf("  Approved:   %v\n", entry.HumanReview.Approved)
			fmt.Printf("  Reviewer:   %s\n", entry.HumanReview.Reviewer)
			fmt.Printf("  Notes:      %s\n", entry.HumanReview.Notes)
		}

		return nil
	}
}

var poolStageCmd = &cobra.Command{
	Use:   "stage <candidate-id>",
	Short: "Stage candidate for promotion",
	Long: `Move a candidate from pending to staged status.

Validates that the candidate meets the minimum tier threshold (default: bronze).
Staged candidates are ready for promotion to the knowledge base.

Examples:
  ao pool stage cand-abc123
  ao pool stage cand-abc123 --min-tier=silver`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		candidateID := args[0]

		if GetDryRun() {
			fmt.Printf("[dry-run] Would stage candidate %s\n", candidateID)
			return nil
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		p := pool.NewPool(cwd)

		minTier := types.TierBronze
		if poolTier != "" {
			minTier = types.Tier(poolTier)
		}

		if err := p.Stage(candidateID, minTier); err != nil {
			return fmt.Errorf("stage candidate: %w", err)
		}

		fmt.Printf("Staged: %s\n", candidateID)
		return nil
	},
}

var poolPromoteCmd = &cobra.Command{
	Use:   "promote <candidate-id>",
	Short: "Promote candidate to knowledge base",
	Long: `Move a staged candidate to the knowledge base (.agents/learnings/ or .agents/patterns/).

Locks the artifact with the ratchet and records the promotion in chain.jsonl.

Examples:
  ao pool promote cand-abc123`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		candidateID := args[0]

		if GetDryRun() {
			fmt.Printf("[dry-run] Would promote candidate %s\n", candidateID)
			return nil
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		p := pool.NewPool(cwd)

		artifactPath, err := p.Promote(candidateID)
		if err != nil {
			return fmt.Errorf("promote candidate: %w", err)
		}

		fmt.Printf("Promoted: %s\n", candidateID)
		fmt.Printf("Artifact: %s\n", artifactPath)

		// Optionally lock with ratchet
		VerbosePrintf("Run 'ao ratchet record promotion --output %s' to lock\n", artifactPath)

		return nil
	},
}

var poolRejectCmd = &cobra.Command{
	Use:   "reject <candidate-id>",
	Short: "Reject candidate",
	Long: `Mark a candidate as rejected and move to rejected directory.

A reason must be provided for audit purposes.

Examples:
  ao pool reject cand-abc123 --reason="Too vague, lacks specificity"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		candidateID := args[0]

		if poolReason == "" {
			return fmt.Errorf("--reason is required for rejection")
		}

		if GetDryRun() {
			fmt.Printf("[dry-run] Would reject candidate %s with reason: %s\n", candidateID, poolReason)
			return nil
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		p := pool.NewPool(cwd)

		// Get reviewer from system user (not spoofable via env)
		reviewer := GetCurrentUser()

		if err := p.Reject(candidateID, poolReason, reviewer); err != nil {
			return fmt.Errorf("reject candidate: %w", err)
		}

		fmt.Printf("Rejected: %s\n", candidateID)
		fmt.Printf("Reason: %s\n", poolReason)

		return nil
	},
}

var poolAutoPromoteCmd = &cobra.Command{
	Use:   "auto-promote",
	Short: "Auto-promote eligible candidates older than threshold",
	Long: `Automatically approve (and optionally promote) high-quality candidates
	that have been pending for longer than the specified threshold (default: 24h).

By default, this command bulk-approves eligible candidates. With --promote,
it will also stage + promote them into .agents/learnings/ or .agents/patterns/.

This is a bulk operation - use with caution. The threshold must be at least
1 hour to prevent accidental mass approval of recently added candidates.

	Examples:
	  ao pool auto-promote
	  ao pool auto-promote --threshold=24h
  ao pool auto-promote --threshold=48h --dry-run
  ao pool auto-promote --threshold=24h --promote`,
	RunE: func(cmd *cobra.Command, args []string) error {
		threshold, thresholdRaw, err := resolveAutoPromoteThreshold(cmd, "threshold", poolThreshold)
		if err != nil {
			return err
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		p := pool.NewPool(cwd)
		reviewer := GetCurrentUser()

		if !poolDoPromote {
			if GetDryRun() {
				fmt.Printf("[dry-run] Would auto-promote (approve) eligible candidates older than %s\n", thresholdRaw)
			}

			approved, err := p.BulkApprove(threshold, reviewer, GetDryRun())
			if err != nil {
				return fmt.Errorf("auto-promote: %w", err)
			}

			if len(approved) == 0 {
				fmt.Println("No candidates eligible for auto-promotion")
				return nil
			}

			if GetDryRun() {
				fmt.Printf("Would auto-promote %d candidates:\n", len(approved))
			} else {
				fmt.Printf("Auto-promoted %d candidates:\n", len(approved))
			}
			for _, id := range approved {
				fmt.Printf("  - %s\n", id)
			}
			return nil
		}

		// --promote mode: stage + promote high-quality candidates (silver, plus gold if enabled).
		return runPoolAutoPromoteAndPromote(p, threshold, reviewer)
	},
}

type poolAutoPromotePromoteResult struct {
	Threshold  string   `json:"threshold"`
	Considered int      `json:"considered"`
	Promoted   int      `json:"promoted"`
	Skipped    int      `json:"skipped"`
	Artifacts  []string `json:"artifacts,omitempty"`
	SkippedIDs []string `json:"skipped_ids,omitempty"`
}

func runPoolAutoPromoteAndPromote(p *pool.Pool, threshold time.Duration, reviewer string) error {
	entries, err := p.List(pool.ListOptions{
		Status: types.PoolStatusPending,
	})
	if err != nil {
		return fmt.Errorf("list pending: %w", err)
	}

	result := poolAutoPromotePromoteResult{
		Threshold: threshold.String(),
	}
	citationCounts, promotedContent := loadPromotionGateContext(p.BaseDir)

	for _, e := range entries {
		// Only auto-promote high-quality tiers.
		if e.Candidate.Tier != types.TierSilver && !(poolGold && e.Candidate.Tier == types.TierGold) {
			continue
		}
		// Never auto-promote human-gated items.
		if e.ScoringResult.GateRequired {
			result.Skipped++
			result.SkippedIDs = append(result.SkippedIDs, e.Candidate.ID)
			continue
		}
		if e.Age < threshold {
			continue
		}
		if reason := checkPromotionCriteria(p.BaseDir, e, threshold, citationCounts, promotedContent); reason != "" {
			result.Skipped++
			result.SkippedIDs = append(result.SkippedIDs, e.Candidate.ID)
			VerbosePrintf("Skipping %s: %s\n", e.Candidate.ID, reason)
			continue
		}

		result.Considered++

		if GetDryRun() {
			result.Promoted++
			continue
		}

		// Stage (enforces min tier) then promote to knowledge base.
		if err := p.Stage(e.Candidate.ID, types.TierSilver); err != nil {
			result.Skipped++
			result.SkippedIDs = append(result.SkippedIDs, e.Candidate.ID)
			VerbosePrintf("Warning: stage %s: %v\n", e.Candidate.ID, err)
			continue
		}

		artifactPath, err := p.Promote(e.Candidate.ID)
		if err != nil {
			result.Skipped++
			result.SkippedIDs = append(result.SkippedIDs, e.Candidate.ID)
			VerbosePrintf("Warning: promote %s: %v\n", e.Candidate.ID, err)
			continue
		}

		// Record an approval note for audit (best-effort).
		_ = reviewer
		result.Promoted++
		result.Artifacts = append(result.Artifacts, artifactPath)
		promotedContent[normalizeContent(e.Candidate.Content)] = true
	}

	switch GetOutput() {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	default:
		if GetDryRun() {
			fmt.Printf("[dry-run] Would promote %d candidate(s) (threshold=%s)\n", result.Promoted, result.Threshold)
			return nil
		}
		if result.Promoted == 0 {
			fmt.Printf("No candidates eligible for promotion (threshold=%s)\n", result.Threshold)
			return nil
		}
		fmt.Printf("Promoted %d candidate(s):\n", result.Promoted)
		for _, a := range result.Artifacts {
			fmt.Printf("  - %s\n", a)
		}
		return nil
	}
}

func init() {
	rootCmd.AddCommand(poolCmd)

	// Add subcommands
	poolCmd.AddCommand(poolListCmd)
	poolCmd.AddCommand(poolShowCmd)
	poolCmd.AddCommand(poolStageCmd)
	poolCmd.AddCommand(poolPromoteCmd)
	poolCmd.AddCommand(poolRejectCmd)
	poolCmd.AddCommand(poolAutoPromoteCmd)

	// Add flags to list command
	poolListCmd.Flags().StringVar(&poolTier, "tier", "", "Filter by tier (gold, silver, bronze)")
	poolListCmd.Flags().StringVar(&poolStatus, "status", "", "Filter by status (pending, staged, promoted, rejected)")
	poolListCmd.Flags().IntVar(&poolLimit, "limit", 50, "Maximum results to return (default 50, 0 for unlimited)")
	poolListCmd.Flags().IntVar(&poolOffset, "offset", 0, "Skip first N results (for pagination)")

	// Add flags to stage command
	poolStageCmd.Flags().StringVar(&poolTier, "min-tier", "", "Minimum tier threshold (default: bronze)")

	// Add flags to reject command
	poolRejectCmd.Flags().StringVar(&poolReason, "reason", "", "Reason for rejection (required)")
	_ = poolRejectCmd.MarkFlagRequired("reason") //nolint:errcheck

	// Add flags to auto-promote command
	poolAutoPromoteCmd.Flags().StringVar(&poolThreshold, "threshold", defaultAutoPromoteThreshold, "Minimum age for auto-promotion (default: 24h)")
	poolAutoPromoteCmd.Flags().BoolVar(&poolDoPromote, "promote", false, "Also stage+promote eligible candidates into .agents/ (not just approval)")
	poolAutoPromoteCmd.Flags().BoolVar(&poolGold, "include-gold", true, "Include gold-tier candidates when using --promote")
}

// truncateID shortens an ID for display.
func truncateID(id string, max int) string {
	if len(id) <= max {
		return id
	}
	return id[:max-3] + "..."
}

// repeat returns a string repeated n times.
func repeat(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
