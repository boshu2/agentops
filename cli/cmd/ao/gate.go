package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/boshu2/agentops/cli/internal/pool"
)

var (
	gateNote      string
	gateReason    string
	gateOlderThan string
	gateTier      string
)

var gateCmd = &cobra.Command{
	Use:   "gate",
	Short: "Human review gates",
	Long: `Manage human review gates for bronze-tier candidates.

Bronze-tier candidates (score 0.50-0.69) require human review
before promotion. The gate command provides the review interface.

Examples:
  ao gate pending
  ao gate approve <candidate-id>
  ao gate reject <candidate-id> --reason="Too vague"`,
}

var gatePendingCmd = &cobra.Command{
	Use:   "pending",
	Short: "List candidates pending review",
	Long: `List bronze-tier candidates awaiting human review.

Shows age/urgency with oldest items first.
Highlights items approaching 24h auto-promote threshold.

Examples:
  ao gate pending
  ao gate pending --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if GetDryRun() {
			fmt.Println("[dry-run] Would list pending gate reviews")
			return nil
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		p := pool.NewPool(cwd)

		entries, err := p.ListPendingReview()
		if err != nil {
			return fmt.Errorf("list pending: %w", err)
		}

		return outputGatePending(entries)
	},
}

func outputGatePending(entries []pool.PoolEntry) error {
	switch GetOutput() {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)

	case "yaml":
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(entries)

	default:
		return outputGatePendingTable(entries)
	}
}

// entryUrgency returns a human-readable urgency label for a pool entry.
func entryUrgency(e pool.PoolEntry) string {
	switch {
	case e.ApproachingAutoPromote:
		return "HIGH (approaching 24h)"
	case e.Age > 12*time.Hour:
		return "MEDIUM"
	default:
		return "LOW"
	}
}

// outputGatePendingTable renders pending gate entries as a formatted table.
func outputGatePendingTable(entries []pool.PoolEntry) error {
	if len(entries) == 0 {
		fmt.Println("No pending reviews")
		return nil
	}

	fmt.Printf("Pending Reviews (%d)\n", len(entries))
	fmt.Println("==================")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	//nolint:errcheck // CLI tabwriter output to stdout, errors unlikely and non-recoverable
	fmt.Fprintln(w, "ID\tTIER\tAGE\tUTILITY\tURGENCY")
	//nolint:errcheck // CLI tabwriter output to stdout
	fmt.Fprintln(w, "--\t----\t---\t-------\t-------")

	for _, e := range entries {
		//nolint:errcheck // CLI tabwriter output to stdout
		fmt.Fprintf(w, "%s\t%s\t%s\t%.2f\t%s\n",
			truncateID(e.Candidate.ID, 16),
			e.Candidate.Tier,
			e.AgeString,
			e.Candidate.Utility,
			entryUrgency(e),
		)
	}

	if err := w.Flush(); err != nil {
		return err
	}

	printAutoPromoteWarning(entries)
	return nil
}

// printAutoPromoteWarning prints a warning if any entries are approaching auto-promote.
func printAutoPromoteWarning(entries []pool.PoolEntry) {
	fmt.Println()
	approaching := 0
	for _, e := range entries {
		if e.ApproachingAutoPromote {
			approaching++
		}
	}
	if approaching > 0 {
		fmt.Printf("! %d candidate(s) approaching 24h auto-promote threshold\n", approaching)
	}
}

var gateApproveCmd = &cobra.Command{
	Use:   "approve <candidate-id>",
	Short: "Approve candidate for promotion",
	Long: `Approve a bronze-tier candidate for promotion.

Records reviewer identity and triggers promotion flow.

Examples:
  ao gate approve cand-abc123
  ao gate approve cand-abc123 --note="Good specificity, approved"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		candidateID := args[0]

		if GetDryRun() {
			fmt.Printf("[dry-run] Would approve candidate %s", candidateID)
			if gateNote != "" {
				fmt.Printf(" with note: %s", gateNote)
			}
			fmt.Println()
			return nil
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		p := pool.NewPool(cwd)

		// Get reviewer from system user (not spoofable via env)
		reviewer := GetCurrentUser()

		if err := p.Approve(candidateID, gateNote, reviewer); err != nil {
			return fmt.Errorf("approve candidate: %w", err)
		}

		fmt.Printf("Approved: %s\n", candidateID)
		fmt.Printf("Reviewer: %s\n", reviewer)
		if gateNote != "" {
			fmt.Printf("Note: %s\n", gateNote)
		}

		// Suggest next step
		fmt.Println()
		fmt.Printf("To promote: ao pool promote %s\n", candidateID)

		return nil
	},
}

var gateRejectCmd = &cobra.Command{
	Use:   "reject <candidate-id>",
	Short: "Reject candidate",
	Long: `Reject a candidate with a required reason.

Records in audit trail for future analysis.

Examples:
  ao gate reject cand-abc123 --reason="Lacks specificity"
  ao gate reject cand-abc123 --reason="Duplicate of existing pattern"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		candidateID := args[0]

		if gateReason == "" {
			return fmt.Errorf("--reason is required for rejection")
		}

		if GetDryRun() {
			fmt.Printf("[dry-run] Would reject candidate %s with reason: %s\n", candidateID, gateReason)
			return nil
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		p := pool.NewPool(cwd)

		// Get reviewer from system user (not spoofable via env)
		reviewer := GetCurrentUser()

		if err := p.Reject(candidateID, gateReason, reviewer); err != nil {
			return fmt.Errorf("reject candidate: %w", err)
		}

		fmt.Printf("Rejected: %s\n", candidateID)
		fmt.Printf("Reviewer: %s\n", reviewer)
		fmt.Printf("Reason: %s\n", gateReason)

		return nil
	},
}

var gateBulkApproveCmd = &cobra.Command{
	Use:   "bulk-approve",
	Short: "Bulk approve silver candidates",
	Long: `Approve all silver-tier candidates older than a threshold.

Silver candidates auto-promote after 24h if not rejected.
This command accelerates the process for reviewed batches.

Examples:
  ao gate bulk-approve
  ao gate bulk-approve --older-than=12h
  ao gate bulk-approve --older-than=24h --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Parse older-than duration
		threshold := 24 * time.Hour
		if gateOlderThan != "" {
			var err error
			threshold, err = time.ParseDuration(gateOlderThan)
			if err != nil {
				return fmt.Errorf("invalid duration %q: %w", gateOlderThan, err)
			}
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		p := pool.NewPool(cwd)

		// Get reviewer from system user (not spoofable via env)
		reviewer := GetCurrentUser()

		approved, err := p.BulkApprove(threshold, reviewer, GetDryRun())
		if err != nil {
			return fmt.Errorf("bulk approve: %w", err)
		}

		if GetDryRun() {
			if len(approved) == 0 {
				fmt.Println("[dry-run] No candidates match criteria")
			} else {
				fmt.Printf("[dry-run] Would approve %d candidate(s):\n", len(approved))
				for _, id := range approved {
					fmt.Printf("  - %s\n", id)
				}
			}
			return nil
		}

		if len(approved) == 0 {
			fmt.Println("No candidates matched criteria")
		} else {
			fmt.Printf("Approved %d candidate(s):\n", len(approved))
			for _, id := range approved {
				fmt.Printf("  - %s\n", id)
			}
		}

		return nil
	},
}

func init() {
	gateCmd.GroupID = "core"
	rootCmd.AddCommand(gateCmd)

	// Add subcommands
	gateCmd.AddCommand(gatePendingCmd)
	gateCmd.AddCommand(gateApproveCmd)
	gateCmd.AddCommand(gateRejectCmd)
	gateCmd.AddCommand(gateBulkApproveCmd)

	// Add flags
	gateApproveCmd.Flags().StringVar(&gateNote, "note", "", "Optional approval note")
	gateRejectCmd.Flags().StringVar(&gateReason, "reason", "", "Required rejection reason")
	_ = gateRejectCmd.MarkFlagRequired("reason") //nolint:errcheck

	gateBulkApproveCmd.Flags().StringVar(&gateOlderThan, "older-than", "24h", "Age threshold for bulk approval")
	gateBulkApproveCmd.Flags().StringVar(&gateTier, "tier", "silver", "Tier to bulk approve (default: silver)")
	_ = gateBulkApproveCmd.RegisterFlagCompletionFunc("tier", staticCompletionFunc("bronze", "silver", "gold"))
}
