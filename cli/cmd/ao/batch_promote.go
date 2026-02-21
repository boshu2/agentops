package main

import (
	"crypto/sha256"
	"encoding/hex"
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

	// List all pending candidates
	entries, err := p.List(pool.ListOptions{
		Status: types.PoolStatusPending,
	})
	if err != nil {
		return fmt.Errorf("list pending: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No pending candidates found")
		return nil
	}

	// Load citations for checking
	citations, err := ratchet.LoadCitations(cwd)
	if err != nil {
		VerbosePrintf("Warning: could not load citations: %v\n", err)
	}

	// Build citation count map keyed by candidate ID
	citationCounts := buildCitationCounts(citations, cwd)

	// Load existing promoted content for duplicate detection
	promotedContent := loadPromotedContent(cwd)

	result := batchPromoteResult{
		Pending: len(entries),
	}

	for _, entry := range entries {
		if batchPromoteForce {
			if err := promoteEntry(p, entry, &result); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to promote %s: %v\n", entry.Candidate.ID, err)
				result.Skipped++
				result.Reasons = append(result.Reasons, skipReason{
					CandidateID: entry.Candidate.ID,
					Reason:      fmt.Sprintf("error: %v", err),
				})
			}
			continue
		}

		// Check criteria
		if reason := checkPromotionCriteria(cwd, entry, minAge, citationCounts, promotedContent); reason != "" {
			result.Skipped++
			result.Reasons = append(result.Reasons, skipReason{
				CandidateID: entry.Candidate.ID,
				Reason:      reason,
			})
			continue
		}

		if err := promoteEntry(p, entry, &result); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to promote %s: %v\n", entry.Candidate.ID, err)
			result.Skipped++
			result.Reasons = append(result.Reasons, skipReason{
				CandidateID: entry.Candidate.ID,
				Reason:      fmt.Sprintf("error: %v", err),
			})
			continue
		}
		promotedContent[normalizeContent(entry.Candidate.Content)] = true
	}

	return outputBatchResult(result)
}

// checkPromotionCriteria returns a skip reason if the candidate does not qualify, or "" if it qualifies.
func checkPromotionCriteria(baseDir string, entry pool.PoolEntry, minAge time.Duration, citationCounts map[string]int, promotedContent map[string]bool) string {
	// Check age
	if entry.Age < minAge {
		return fmt.Sprintf("too young (%s < %s)", entry.AgeString, minAge)
	}

	// Check citations
	if citationCounts[entry.Candidate.ID] < 1 {
		// Also check by file path in case citations reference the pool file
		entryPath := canonicalArtifactKey(baseDir, entry.FilePath)
		if entry.FilePath == "" || (citationCounts[entry.FilePath] < 1 && citationCounts[entryPath] < 1) {
			return "no citations"
		}
	}

	// Check for duplicate content
	contentKey := normalizeContent(entry.Candidate.Content)
	if promotedContent[contentKey] {
		return "duplicate of already-promoted content"
	}

	return ""
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

	dirs := []string{
		filepath.Join(baseDir, ".agents", "learnings"),
		filepath.Join(baseDir, ".agents", "patterns"),
	}

	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		files, err := filepath.Glob(filepath.Join(dir, "*.md"))
		if err != nil {
			continue
		}

		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			key := normalizeContent(string(data))
			content[key] = true
		}
	}

	return content
}

// normalizeContent creates a normalized key for content comparison using content hashing.
// Lowercases, collapses whitespace, then SHA256 hashes the full normalized content.
// This avoids false positives from naive prefix truncation.
func normalizeContent(s string) string {
	s = strings.ToLower(s)
	s = strings.Join(strings.Fields(s), " ")
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
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
