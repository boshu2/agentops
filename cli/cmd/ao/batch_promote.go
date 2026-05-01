package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/pool"
	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/types"
)

var (
	batchPromoteForce  bool
	batchPromoteMinAge string
)

const duplicatePromotedContentReason = "duplicate of already-promoted content"

// skipReason records why a candidate was skipped.
type skipReason struct {
	CandidateID string `json:"candidate_id"`
	Reason      string `json:"reason"`
}

// batchPromoteResult holds the summary of a batch-promote run.
type batchPromoteResult struct {
	Pending  int          `json:"pending"`
	Promoted int          `json:"promoted"`
	Skipped  int          `json:"skipped"`
	Reasons  []skipReason `json:"skipped_reasons,omitempty"`
}

var poolBatchPromoteCmd = &cobra.Command{
	Use:   "batch-promote",
	Short: "Bulk promote pending candidates to knowledge base",
	Long: `Promote pending pool candidates that meet promotion criteria.

Criteria (unless --force):
  - Age > 24h (candidate has had time to settle)
  - Has been cited at least once
  - Not a duplicate of already-promoted content

Flags:
  --dry-run   Show what would be promoted without executing
  --force     Promote all pending candidates regardless of criteria
  --min-age   Minimum age threshold (default: 24h)

Examples:
  ao pool batch-promote
  ao pool batch-promote --dry-run
  ao pool batch-promote --force
  ao pool batch-promote --min-age=12h`,
	RunE: runBatchPromote,
}

func init() {
	poolCmd.AddCommand(poolBatchPromoteCmd)

	poolBatchPromoteCmd.Flags().BoolVar(&batchPromoteForce, "force", false, "Promote all pending regardless of criteria")
	poolBatchPromoteCmd.Flags().StringVar(&batchPromoteMinAge, "min-age", "24h", "Minimum age for promotion eligibility")
}

// recordPromoteSkip records a skipped entry with the given reason.
func recordPromoteSkip(result *batchPromoteResult, candidateID, reason string) {
	result.Skipped++
	result.Reasons = append(result.Reasons, skipReason{
		CandidateID: candidateID,
		Reason:      reason,
	})
}

// tryPromoteEntry attempts to promote a single entry, recording a skip on error.
func tryPromoteEntry(p *pool.Pool, entry pool.PoolEntry, result *batchPromoteResult) error {
	if err := promoteEntry(p, entry, result); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to promote %s: %v\n", entry.Candidate.ID, err)
		recordPromoteSkip(result, entry.Candidate.ID, fmt.Sprintf("error: %v", err))
		return err
	}
	return nil
}

// processPromotionCandidate evaluates and promotes a single candidate entry.
// Returns true if the candidate was promoted (for content tracking purposes).
func processPromotionCandidate(p *pool.Pool, entry pool.PoolEntry, cwd string, minAge time.Duration, citationCounts map[string]int, promotedContent map[string]bool, result *batchPromoteResult) bool {
	if batchPromoteForce {
		_ = tryPromoteEntry(p, entry, result)
		return false
	}

	if reason := checkPromotionCriteria(cwd, entry, minAge, citationCounts, promotedContent, true); reason != "" {
		recordPromoteSkip(result, entry.Candidate.ID, reason)
		if isDuplicatePromotedReason(reason) {
			rejectDuplicatePromotedCandidate(p, entry, reason, "batch-promote")
		}
		return false
	}

	if tryPromoteEntry(p, entry, result) != nil {
		return false
	}
	return true
}

func runBatchPromote(cmd *cobra.Command, args []string) error {
	minAge, err := time.ParseDuration(batchPromoteMinAge)
	if err != nil {
		return fmt.Errorf("invalid --min-age: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	p := pool.NewPool(cwd)

	entries, err := p.List(pool.ListOptions{Status: types.PoolStatusPending})
	if err != nil {
		return fmt.Errorf("list pending: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No pending candidates found")
		return nil
	}

	citations, err := ratchet.LoadCitations(cwd)
	if err != nil {
		VerbosePrintf("Warning: could not load citations: %v\n", err)
	}

	citationCounts := buildCitationCounts(citations, cwd)
	promotedContent := loadPromotedContent(cwd)
	result := batchPromoteResult{Pending: len(entries)}

	for _, entry := range entries {
		if processPromotionCandidate(p, entry, cwd, minAge, citationCounts, promotedContent, &result) {
			promotedContent[normalizeContent(entry.Candidate.Content)] = true
		}
	}

	return outputBatchResult(result)
}

// checkPromotionCriteria returns a skip reason if the candidate does not qualify, or "" if it qualifies.
// When requireCitations is true, the candidate must have at least 2 citations (the manual
// batch-promote path, where citation signal is the primary gate). When false (the automated
// flywheel close-loop path), the caller has already established signal via scoring tier +
// gate-not-required, so the citation gate is skipped to let fresh silver/gold candidates flow
// through — otherwise nothing ever gets promoted, cited, or indexed.
func checkPromotionCriteria(baseDir string, entry pool.PoolEntry, minAge time.Duration, citationCounts map[string]int, promotedContent map[string]bool, requireCitations bool) string {
	// Check age
	if entry.Age < minAge {
		return fmt.Sprintf("too young (%s < %s)", entry.AgeString, minAge)
	}

	if requireCitations {
		totalCitations := citationCounts[entry.Candidate.ID]
		entryPath := canonicalArtifactKey(baseDir, entry.FilePath)
		if entry.FilePath != "" {
			if c := citationCounts[entry.FilePath]; c > totalCitations {
				totalCitations = c
			}
			if c := citationCounts[entryPath]; c > totalCitations {
				totalCitations = c
			}
		}
		if totalCitations < 2 {
			return fmt.Sprintf("insufficient citations (%d < 2)", totalCitations)
		}
	}

	// Check utility threshold (must show positive signal)
	if entry.Candidate.Utility < 0.5 {
		return fmt.Sprintf("utility too low (%.2f < 0.50)", entry.Candidate.Utility)
	}

	// Check for duplicate content
	contentKey := normalizeContent(entry.Candidate.Content)
	if promotedContent[contentKey] {
		return duplicatePromotedContentReason
	}

	return ""
}

func isDuplicatePromotedReason(reason string) bool {
	return strings.Contains(reason, duplicatePromotedContentReason)
}

func rejectDuplicatePromotedCandidate(p *pool.Pool, entry pool.PoolEntry, reason, reviewer string) {
	if GetDryRun() {
		return
	}
	if err := p.Init(); err != nil {
		VerbosePrintf("Warning: init pool before duplicate reject %s: %v\n", entry.Candidate.ID, err)
		return
	}
	auditReason := "dedup: " + reason
	if err := p.Reject(entry.Candidate.ID, auditReason, reviewer); err != nil {
		VerbosePrintf("Warning: reject duplicate %s: %v\n", entry.Candidate.ID, err)
		if skipErr := p.RecordSkip(entry.Candidate.ID, auditReason, reviewer); skipErr != nil {
			VerbosePrintf("Warning: record duplicate skip %s: %v\n", entry.Candidate.ID, skipErr)
		}
	}
}

// promoteEntry promotes a single entry, respecting dry-run.
func promoteEntry(p *pool.Pool, entry pool.PoolEntry, result *batchPromoteResult) error {
	if GetDryRun() {
		fmt.Printf("[dry-run] Would promote: %s (tier=%s, age=%s)\n",
			entry.Candidate.ID, entry.Candidate.Tier, entry.AgeString)
		result.Promoted++
		return nil
	}

	// Normalize state transitions: pending -> staged -> promoted.
	if err := p.Stage(entry.Candidate.ID, types.TierBronze); err != nil {
		return fmt.Errorf("stage: %w", err)
	}

	artifactPath, err := p.Promote(entry.Candidate.ID)
	if err != nil {
		return err
	}

	fmt.Printf("Promoted: %s -> %s\n", entry.Candidate.ID, artifactPath)
	result.Promoted++
	return nil
}

// buildCitationCounts builds a map of candidate ID -> citation count.
func buildCitationCounts(citations []types.CitationEvent, baseDir string) map[string]int {
	counts := make(map[string]int)
	for _, c := range citations {
		// Count by artifact path
		counts[c.ArtifactPath]++
		canonicalPath := canonicalArtifactKey(baseDir, c.ArtifactPath)
		if canonicalPath != "" {
			counts[canonicalPath]++
		}

		// Also extract candidate ID from path if it's a pool entry
		// e.g., .agents/pool/pending/cand-abc123.json -> cand-abc123
		base := filepath.Base(c.ArtifactPath)
		if strings.HasSuffix(base, ".json") {
			id := strings.TrimSuffix(base, ".json")
			counts[id]++
		}
	}
	return counts
}

// loadPromotedContent loads content from already-promoted artifacts for duplicate detection.
func loadPromotedContent(baseDir string) map[string]bool {
	content := make(map[string]bool)
	for hash := range pool.LoadKnownPromotedContentHashes(baseDir) {
		content[hash] = true
	}
	return content
}

// normalizeContent returns the canonical promoted-body content hash used by
// Pool.Promote, pool reindex, batch promotion, and close-loop promotion.
func normalizeContent(s string) string {
	return pool.ContentHash(s)
}

// outputBatchResult prints the batch-promote summary.
func outputBatchResult(result batchPromoteResult) error {
	switch GetOutput() {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)

	default:
		fmt.Println()
		if GetDryRun() {
			fmt.Println("Batch Promote (dry-run)")
		} else {
			fmt.Println("Batch Promote Summary")
		}
		fmt.Println("=====================")
		fmt.Printf("  Pending:  %d\n", result.Pending)
		fmt.Printf("  Promoted: %d\n", result.Promoted)
		fmt.Printf("  Skipped:  %d\n", result.Skipped)

		if len(result.Reasons) > 0 {
			fmt.Println()
			fmt.Println("Skipped candidates:")
			for _, r := range result.Reasons {
				fmt.Printf("  - %s: %s\n", r.CandidateID, r.Reason)
			}
		}

		return nil
	}
}
