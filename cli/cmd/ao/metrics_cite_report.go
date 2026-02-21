package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/types"
)

func init() {
	citeReportCmd := &cobra.Command{
		Use:   "cite-report",
		Short: "Aggregated citation report",
		Long: `Produce an aggregated report from citation data.

Shows:
  - Total citations, unique artifacts, unique sessions
  - Hit rate (artifacts cited in 2+ sessions)
  - Top-10 most-cited artifacts
  - Uncited learnings
  - Staleness candidates (30/60/90 days)
  - Feedback closure rate

Examples:
  ao metrics cite-report
  ao metrics cite-report --days 90
  ao metrics cite-report -o json`,
		RunE: runMetricsCiteReport,
	}
	citeReportCmd.Flags().Int("days", 30, "Period in days")
	citeReportCmd.Flags().Bool("json", false, "Output as JSON (alias for -o json)")
	metricsCmd.AddCommand(citeReportCmd)
}

// citeReportData holds the aggregated citation report.
type citeReportData struct {
	TotalCitations   int             `json:"total_citations"`
	UniqueArtifacts  int             `json:"unique_artifacts"`
	UniqueSessions   int             `json:"unique_sessions"`
	HitRate          float64         `json:"hit_rate"`
	HitCount         int             `json:"hit_count"`
	TopArtifacts     []artifactCount `json:"top_artifacts"`
	UncitedLearnings []string        `json:"uncited_learnings,omitempty"`
	Staleness        map[string]int  `json:"staleness"`
	FeedbackTotal    int             `json:"feedback_total"`
	FeedbackGiven    int             `json:"feedback_given"`
	FeedbackRate     float64         `json:"feedback_rate"`
	Days             int             `json:"days"`
	PeriodStart      time.Time       `json:"period_start"`
	PeriodEnd        time.Time       `json:"period_end"`
}

type artifactCount struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
}

func runMetricsCiteReport(cmd *cobra.Command, args []string) error {
	days, _ := cmd.Flags().GetInt("days")
	jsonFlag, _ := cmd.Flags().GetBool("json")
	jsonOutput := jsonFlag || GetOutput() == "json"

	baseDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	allCitations, err := ratchet.LoadCitations(baseDir)
	if err != nil {
		VerbosePrintf("Warning: load citations: %v\n", err)
	}
	for i := range allCitations {
		allCitations[i].ArtifactPath = canonicalArtifactPath(baseDir, allCitations[i].ArtifactPath)
		allCitations[i].SessionID = canonicalSessionID(allCitations[i].SessionID)
	}
	now := time.Now()
	periodStart := now.AddDate(0, 0, -days)
	if len(allCitations) == 0 {
		report := buildCiteReport(baseDir, nil, nil, days, periodStart, now)
		if jsonOutput {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(report)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "No citation data found.")
		return nil
	}

	stats := filterCitationsForPeriod(allCitations, periodStart, now)
	filtered := stats.citations

	report := buildCiteReport(baseDir, filtered, allCitations, days, periodStart, now)

	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	printCiteReport(cmd.OutOrStdout(), report)
	return nil
}

func buildCiteReport(baseDir string, filtered []types.CitationEvent, all []types.CitationEvent, days int, start, end time.Time) citeReportData {
	report := citeReportData{
		TotalCitations: len(filtered),
		Days:           days,
		PeriodStart:    start,
		PeriodEnd:      end,
		Staleness:      make(map[string]int),
	}

	// Unique artifacts and sessions
	artifactCounts := make(map[string]int)
	sessions := make(map[string]bool)
	// Track which sessions cite each artifact (for hit rate)
	artifactSessions := make(map[string]map[string]bool)

	for _, c := range filtered {
		canonicalPath := canonicalArtifactPath(baseDir, c.ArtifactPath)
		artifactCounts[canonicalPath]++
		sessions[c.SessionID] = true
		if artifactSessions[canonicalPath] == nil {
			artifactSessions[canonicalPath] = make(map[string]bool)
		}
		artifactSessions[canonicalPath][c.SessionID] = true

		report.FeedbackTotal++
		if c.FeedbackGiven {
			report.FeedbackGiven++
		}
	}
	report.UniqueArtifacts = len(artifactCounts)
	report.UniqueSessions = len(sessions)

	// Hit rate: artifacts cited in 2+ distinct sessions
	for _, sessMap := range artifactSessions {
		if len(sessMap) >= 2 {
			report.HitCount++
		}
	}
	if report.UniqueArtifacts > 0 {
		report.HitRate = float64(report.HitCount) / float64(report.UniqueArtifacts)
	}

	// Feedback closure rate
	if report.FeedbackTotal > 0 {
		report.FeedbackRate = float64(report.FeedbackGiven) / float64(report.FeedbackTotal)
	}

	// Top-10 most-cited
	type kv struct {
		path  string
		count int
	}
	var sorted []kv
	for p, c := range artifactCounts {
		sorted = append(sorted, kv{p, c})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})
	limit := 10
	if len(sorted) < limit {
		limit = len(sorted)
	}
	for _, s := range sorted[:limit] {
		report.TopArtifacts = append(report.TopArtifacts, artifactCount{Path: s.path, Count: s.count})
	}

	// Uncited learnings
	learningsDir := filepath.Join(baseDir, ".agents", "learnings")
	if _, err := os.Stat(learningsDir); err == nil {
		files, _ := filepath.Glob(filepath.Join(learningsDir, "*.md"))
		citedSet := make(map[string]bool)
		for _, c := range all {
			citedSet[canonicalArtifactPath(baseDir, c.ArtifactPath)] = true
		}
		for _, f := range files {
			if !citedSet[canonicalArtifactPath(baseDir, f)] {
				report.UncitedLearnings = append(report.UncitedLearnings, f)
			}
		}
	}

	// Staleness candidates from ALL citations
	now := time.Now()
	lastCited := make(map[string]time.Time)
	for _, c := range all {
		canonicalPath := canonicalArtifactPath(baseDir, c.ArtifactPath)
		if t, ok := lastCited[canonicalPath]; !ok || c.CitedAt.After(t) {
			lastCited[canonicalPath] = c.CitedAt
		}
	}
	for _, threshold := range []int{30, 60, 90} {
		cutoff := now.AddDate(0, 0, -threshold)
		count := 0
		for _, t := range lastCited {
			if t.Before(cutoff) {
				count++
			}
		}
		report.Staleness[fmt.Sprintf("%dd", threshold)] = count
	}

	return report
}

func printCiteReport(w io.Writer, r citeReportData) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Citation Report")
	fmt.Fprintln(w, "===============")
	fmt.Fprintf(w, "Period: %s to %s (%d days)\n\n",
		r.PeriodStart.Format("2006-01-02"),
		r.PeriodEnd.Format("2006-01-02"),
		r.Days)

	fmt.Fprintln(w, "SUMMARY:")
	fmt.Fprintf(w, "  Total citations:     %d\n", r.TotalCitations)
	fmt.Fprintf(w, "  Unique artifacts:    %d\n", r.UniqueArtifacts)
	fmt.Fprintf(w, "  Unique sessions:     %d\n", r.UniqueSessions)
	fmt.Fprintf(w, "  Hit rate (2+ sess):  %.0f%% (%d/%d)\n", r.HitRate*100, r.HitCount, r.UniqueArtifacts)
	fmt.Fprintln(w)

	if len(r.TopArtifacts) > 0 {
		fmt.Fprintln(w, "TOP CITED ARTIFACTS:")
		for i, a := range r.TopArtifacts {
			fmt.Fprintf(w, "  %2d. %s (%d)\n", i+1, a.Path, a.Count)
		}
		fmt.Fprintln(w)
	}

	if len(r.UncitedLearnings) > 0 {
		fmt.Fprintln(w, "UNCITED LEARNINGS:")
		for _, u := range r.UncitedLearnings {
			fmt.Fprintf(w, "  - %s\n", u)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "STALENESS:")
	for _, d := range []string{"30d", "60d", "90d"} {
		fmt.Fprintf(w, "  Not cited in %s: %d\n", d, r.Staleness[d])
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "FEEDBACK:")
	fmt.Fprintf(w, "  Closure rate: %.0f%% (%d/%d)\n", r.FeedbackRate*100, r.FeedbackGiven, r.FeedbackTotal)
}
