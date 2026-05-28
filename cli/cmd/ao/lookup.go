// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/config"
	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/types"
)

var (
	lookupQuery     string
	lookupLimit     int
	lookupBead      string
	lookupJSON      bool
	lookupNoCite    bool
	lookupCiteType  string
	lookupSessionID string
)

var lookupCmd = &cobra.Command{
	Use:   "lookup [id]",
	Short: "Retrieve specific knowledge artifacts by ID or query",
	Long: `Lookup retrieves full content of specific knowledge artifacts.

Retrieve full content of specific knowledge artifacts on demand.
Use this to pull learnings, patterns, or any indexed knowledge
relevant to your current task.

Modes:
  ao lookup <id>                    # Fetch one learning by ID
  ao lookup --query "topic"         # Top matches by relevance
  ao lookup --bead ag-xyz           # Learnings from bead lineage

Examples:
  ao lookup learn-2026-02-22-cross-lang
  ao lookup --query "authentication" --limit 5
  ao lookup --bead ag-mrr
  ao lookup --query "anti-patterns" --json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runLookup,
}

func init() {
	lookupCmd.GroupID = "knowledge"
	rootCmd.AddCommand(lookupCmd)
	lookupCmd.Flags().StringVar(&lookupQuery, "query", "", "Search query for relevance matching")
	lookupCmd.Flags().IntVar(&lookupLimit, "limit", 3, "Maximum results to return")
	lookupCmd.Flags().StringVar(&lookupBead, "bead", "", "Filter by source bead ID")
	lookupCmd.Flags().BoolVar(&lookupJSON, "json", false, "JSON output")
	lookupCmd.Flags().BoolVar(&lookupNoCite, "no-cite", false, "Skip citation recording")
	lookupCmd.Flags().StringVar(&lookupCiteType, "cite", "retrieved", "Citation type to record for returned artifacts: retrieved, reference, applied")
	lookupCmd.Flags().StringVar(&lookupSessionID, "session", "", "Session ID for citation tracking")
}

func runLookup(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg, cfgErr := config.Load(nil)
	if cfgErr != nil {
		VerbosePrintf("Warning: config load: %v (using defaults)\n", cfgErr)
	}

	// Mode 1: Lookup by ID
	if len(args) > 0 {
		return lookupByID(cwd, args[0], cfg)
	}

	// Mode 2 or 3: Query-based or bead-based
	if lookupQuery == "" && lookupBead == "" {
		return fmt.Errorf("provide an ID argument, --query, or --bead flag")
	}

	return lookupByQuery(cwd, cfg)
}

// lookupByID searches all learning and pattern files for a matching ID.
func lookupByID(cwd, id string, cfg *config.Config) error {
	globalLearningsDir := ""
	globalFindingsDir := ""
	globalPatternsDir := ""
	if cfg != nil {
		globalLearningsDir = cfg.Paths.GlobalLearningsDir
		if globalLearningsDir != "" {
			globalFindingsDir = filepath.Join(filepath.Dir(globalLearningsDir), SectionFindings)
		}
		globalPatternsDir = cfg.Paths.GlobalPatternsDir
	}

	// Search learnings
	learnings, _ := collectLearnings(cwd, "", MaxLearningsToInject*5, globalLearningsDir, 1.0)
	for _, l := range learnings {
		if matchesID(l.ID, l.Source, id) {
			return outputLearning(cwd, l)
		}
	}

	// Search patterns
	patterns, _ := collectPatterns(cwd, "", MaxPatternsToInject*5, globalPatternsDir, 1.0)
	for _, p := range patterns {
		if matchesID(p.Name, p.FilePath, id) {
			return outputPattern(cwd, p)
		}
	}

	findings, _ := collectFindings(cwd, "", MaxPatternsToInject*5, globalFindingsDir, 1.0)
	for _, f := range findings {
		if matchesID(f.ID, f.Source, id) {
			return outputFinding(cwd, f)
		}
	}

	return fmt.Errorf("no artifact found matching ID: %s", id)
}

// matchesID checks if the given id matches the item's ID, name, or filename.
func matchesID(itemID, filePath, searchID string) bool {
	searchLower := strings.ToLower(searchID)
	if strings.ToLower(itemID) == searchLower {
		return true
	}
	if filePath != "" {
		base := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
		if strings.ToLower(base) == searchLower {
			return true
		}
		// Also check if the filename contains the search ID
		if strings.Contains(strings.ToLower(base), searchLower) {
			return true
		}
	}
	return false
}

// lookupByQuery uses the existing collectors with query filtering.
func lookupByQuery(cwd string, cfg *config.Config) error {
	globalLearningsDir := ""
	globalFindingsDir := ""
	globalPatternsDir := ""
	globalWeight := 0.8
	if cfg != nil {
		globalLearningsDir = cfg.Paths.GlobalLearningsDir
		if globalLearningsDir != "" {
			globalFindingsDir = filepath.Join(filepath.Dir(globalLearningsDir), SectionFindings)
		}
		globalPatternsDir = cfg.Paths.GlobalPatternsDir
		globalWeight = cfg.Paths.GlobalWeight
	}

	query := lookupQuery
	limit := lookupLimit

	// Collect and score learnings
	learnings, _ := collectLearnings(cwd, query, limit*3, globalLearningsDir, globalWeight)

	// Apply bead filter if specified
	if lookupBead != "" {
		learnings = filterByBead(learnings, lookupBead)
	}

	// Collect and score patterns
	patterns, _ := collectPatterns(cwd, query, limit, globalPatternsDir, globalWeight)

	// Collect and score findings
	findings, _ := collectFindings(cwd, query, limit, globalFindingsDir, globalWeight)

	// Trim to limit
	if len(learnings) > limit {
		learnings = learnings[:limit]
	}

	// Record citations
	if !lookupNoCite && (len(learnings) > 0 || len(patterns) > 0 || len(findings) > 0) {
		sessionID := resolveSessionID(lookupSessionID)
		citationQuery := query
		if citationQuery == "" {
			citationQuery = lookupBead
		}
		recordLookupCitations(cwd, learnings, patterns, findings, sessionID, citationQuery, lookupCiteType)
	}

	return outputResults(cwd, learnings, patterns, findings)
}

// filterByBead keeps only learnings whose SourceBead matches the given bead ID.
func filterByBead(learnings []learning, beadID string) []learning {
	beadLower := strings.ToLower(beadID)
	var filtered []learning
	for _, l := range learnings {
		if strings.ToLower(l.SourceBead) == beadLower {
			filtered = append(filtered, l)
		}
	}
	return filtered
}

// outputLearning renders a single learning result.
func outputLearning(cwd string, l learning) error {
	if lookupJSON {
		return outputJSON(l)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "## %s\n\n", l.ID)
	fmt.Fprintf(&sb, "**%s**\n", l.Title)
	fmt.Fprintf(&sb, "Utility: %.2f | Age: %s | Score: %.2f\n\n",
		l.Utility, formatLookupAge(l.AgeWeeks), l.CompositeScore)
	if l.Summary != "" {
		sb.WriteString(l.Summary + "\n\n")
	}
	sb.WriteString(formatLearningEvidenceBlock(l))

	// Read full file content if available
	if l.Source != "" {
		content, err := os.ReadFile(l.Source)
		if err == nil {
			sb.WriteString("---\n")
			sb.WriteString(string(content))
		}
	}

	fmt.Fprintf(&sb, "\nSource: %s\n", relPath(cwd, l.Source))
	if l.SourceBead != "" {
		fmt.Fprintf(&sb, "Source bead: %s", l.SourceBead)
		if l.SourcePhase != "" {
			fmt.Fprintf(&sb, " | Phase: %s", l.SourcePhase)
		}
		sb.WriteString("\n")
	}

	fmt.Println(sb.String())

	// Record citation for single lookup
	if !lookupNoCite && l.Source != "" {
		sessionID := resolveSessionID(lookupSessionID)
		event := types.CitationEvent{
			ArtifactPath:    canonicalArtifactPath(cwd, l.Source),
			SessionID:       sessionID,
			CitedAt:         time.Now(),
			CitationType:    canonicalCitationType(lookupCiteType),
			Query:           l.ID,
			MetricNamespace: defaultCitationMetricNamespace(),
			MatchConfidence: l.MatchConfidence,
			MatchProvenance: l.MatchProvenance,
			SectionHeading:  l.SectionHeading,
			SectionLocator:  l.SectionLocator,
		}
		if err := ratchet.RecordCitation(cwd, event); err != nil {
			VerbosePrintf("Warning: failed to record citation: %v\n", err)
		}
	}

	return nil
}

// outputPattern renders a single pattern result.
func outputPattern(cwd string, p pattern) error {
	if lookupJSON {
		return outputJSON(p)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "## %s\n\n", p.Name)
	if p.Description != "" {
		sb.WriteString(p.Description + "\n\n")
	}
	fmt.Fprintf(&sb, "Utility: %.2f | Age: %s | Score: %.2f\n\n",
		p.Utility, formatLookupAge(p.AgeWeeks), p.CompositeScore)

	if p.FilePath != "" {
		content, err := os.ReadFile(p.FilePath)
		if err == nil {
			sb.WriteString("---\n")
			sb.WriteString(string(content))
		}
		fmt.Fprintf(&sb, "\nSource: %s\n", relPath(cwd, p.FilePath))
	}

	fmt.Println(sb.String())

	if !lookupNoCite && p.FilePath != "" {
		sessionID := resolveSessionID(lookupSessionID)
		event := types.CitationEvent{
			ArtifactPath: canonicalArtifactPath(cwd, p.FilePath),
			SessionID:    sessionID,
			CitedAt:      time.Now(),
			CitationType: canonicalCitationType(lookupCiteType),
			Query:        p.Name,
		}
		if err := ratchet.RecordCitation(cwd, event); err != nil {
			VerbosePrintf("Warning: failed to record citation: %v\n", err)
		}
	}

	return nil
}

func outputFinding(cwd string, f knowledgeFinding) error {
	if lookupJSON {
		return outputJSON(f)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "## %s\n\n", f.ID)
	if f.Title != "" {
		fmt.Fprintf(&sb, "**%s**\n", f.Title)
	}
	if f.Severity != "" || f.Detectability != "" || f.Status != "" {
		fmt.Fprintf(&sb, "Severity: %s | Detectability: %s | Status: %s\n",
			emptyIfMissing(f.Severity), emptyIfMissing(f.Detectability), emptyIfMissing(f.Status))
	}
	fmt.Fprintf(&sb, "Utility: %.2f | Age: %s | Score: %.2f\n\n",
		f.Utility, formatLookupAge(f.AgeWeeks), f.CompositeScore)
	if f.Summary != "" {
		sb.WriteString(f.Summary + "\n\n")
	}
	if f.Source != "" {
		content, err := os.ReadFile(f.Source)
		if err == nil {
			sb.WriteString("---\n")
			sb.WriteString(string(content))
		}
		fmt.Fprintf(&sb, "\nSource: %s\n", relPath(cwd, f.Source))
	}

	fmt.Println(sb.String())

	if !lookupNoCite && f.Source != "" {
		sessionID := resolveSessionID(lookupSessionID)
		event := types.CitationEvent{
			ArtifactPath: canonicalArtifactPath(cwd, f.Source),
			SessionID:    sessionID,
			CitedAt:      time.Now(),
			CitationType: canonicalCitationType(lookupCiteType),
			Query:        f.ID,
		}
		if err := ratchet.RecordCitation(cwd, event); err != nil {
			VerbosePrintf("Warning: failed to record citation: %v\n", err)
		}
	}

	return nil
}

// outputResults renders multiple learnings, patterns, and findings.
func outputResults(cwd string, learnings []learning, patterns []pattern, findings []knowledgeFinding) error {
	if lookupJSON {
		result := struct {
			Learnings []learning         `json:"learnings"`
			Patterns  []pattern          `json:"patterns"`
			Findings  []knowledgeFinding `json:"findings"`
		}{
			Learnings: learnings,
			Patterns:  patterns,
			Findings:  findings,
		}
		return outputJSON(result)
	}

	if len(learnings) == 0 && len(patterns) == 0 && len(findings) == 0 {
		fmt.Println("No matching artifacts found.")
		return nil
	}

	for _, l := range learnings {
		fmt.Printf("## %s\n\n", l.ID)
		fmt.Printf("**%s**\n", l.Title)
		fmt.Printf("Utility: %.2f | Age: %s | Score: %.2f\n",
			l.Utility, formatLookupAge(l.AgeWeeks), l.CompositeScore)
		if l.Summary != "" {
			fmt.Printf("%s\n", l.Summary)
		}
		fmt.Print(formatLearningEvidenceBlock(l))
		fmt.Printf("Source: %s\n\n", relPath(cwd, l.Source))
	}

	for _, p := range patterns {
		fmt.Printf("## %s\n\n", p.Name)
		if p.Description != "" {
			fmt.Printf("%s\n", p.Description)
		}
		fmt.Printf("Score: %.2f\n", p.CompositeScore)
		if p.FilePath != "" {
			fmt.Printf("Source: %s\n", relPath(cwd, p.FilePath))
		}
		fmt.Println()
	}

	for _, f := range findings {
		fmt.Printf("## %s\n\n", f.ID)
		if f.Title != "" {
			fmt.Printf("**%s**\n", f.Title)
		}
		if f.Summary != "" {
			fmt.Printf("%s\n", f.Summary)
		}
		fmt.Printf("Score: %.2f\n", f.CompositeScore)
		if f.Source != "" {
			fmt.Printf("Source: %s\n", relPath(cwd, f.Source))
		}
		fmt.Println()
	}

	return nil
}

// outputJSON marshals any value to indented JSON and prints it.
func outputJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// relPath returns a relative path from cwd, or the original if it fails.
func relPath(cwd, path string) string {
	if rel, err := filepath.Rel(cwd, path); err == nil {
		return rel
	}
	return path
}

// formatLookupAge formats age in weeks as human-readable.
func formatLookupAge(ageWeeks float64) string {
	if ageWeeks < 0.14 {
		return "<1d"
	}
	days := ageWeeks * 7
	if days < 7 {
		return fmt.Sprintf("%.0fd", days)
	}
	if days < 30 {
		return fmt.Sprintf("%.0fw", ageWeeks)
	}
	return fmt.Sprintf("%.0fmo", days/30)
}

// recordLookupCitations records citations for lookup results.
func recordLookupCitations(cwd string, learnings []learning, patterns []pattern, findings []knowledgeFinding, sessionID, query, citationType string) {
	recordLookupCitationsInNamespace(cwd, learnings, patterns, findings, sessionID, query, citationType, defaultCitationMetricNamespace())
}

func recordLookupCitationsInNamespace(cwd string, learnings []learning, patterns []pattern, findings []knowledgeFinding, sessionID, query, citationType, namespace string) {
	citationType = canonicalCitationType(citationType)
	if citationType == "" {
		citationType = "retrieved"
	}
	canonicalNamespace := canonicalMetricNamespace(namespace)
	for _, l := range learnings {
		if l.Source == "" {
			continue
		}
		event := types.CitationEvent{
			ArtifactPath:    canonicalArtifactPath(cwd, l.Source),
			SessionID:       sessionID,
			CitedAt:         time.Now(),
			CitationType:    citationType,
			Query:           query,
			MetricNamespace: canonicalNamespace,
			MatchConfidence: l.MatchConfidence,
			MatchProvenance: l.MatchProvenance,
			SectionHeading:  l.SectionHeading,
			SectionLocator:  l.SectionLocator,
		}
		if err := ratchet.RecordCitation(cwd, event); err != nil {
			VerbosePrintf("Warning: record citation for %s: %v\n", l.ID, err)
		}
	}
	for _, p := range patterns {
		if p.FilePath == "" {
			continue
		}
		event := types.CitationEvent{
			ArtifactPath:    canonicalArtifactPath(cwd, p.FilePath),
			SessionID:       sessionID,
			CitedAt:         time.Now(),
			CitationType:    citationType,
			Query:           query,
			MetricNamespace: canonicalNamespace,
		}
		if err := ratchet.RecordCitation(cwd, event); err != nil {
			VerbosePrintf("Warning: record citation for %s: %v\n", p.Name, err)
		}
	}
	for _, f := range findings {
		if f.Source == "" {
			continue
		}
		event := types.CitationEvent{
			ArtifactPath:    canonicalArtifactPath(cwd, f.Source),
			SessionID:       sessionID,
			CitedAt:         time.Now(),
			CitationType:    citationType,
			Query:           query,
			MetricNamespace: canonicalNamespace,
		}
		if err := ratchet.RecordCitation(cwd, event); err != nil {
			VerbosePrintf("Warning: record citation for %s: %v\n", f.ID, err)
		}
	}
}

func formatLearningEvidenceBlock(l learning) string {
	if l.SectionHeading == "" && l.MatchedSnippet == "" {
		return ""
	}

	var sb strings.Builder
	if l.SectionHeading != "" {
		sb.WriteString("Evidence: ")
		sb.WriteString(l.SectionHeading)
		if l.SectionLocator != "" {
			sb.WriteString(" (")
			sb.WriteString(l.SectionLocator)
			sb.WriteString(")")
		}
		if l.MatchConfidence > 0 {
			fmt.Fprintf(&sb, " | Match: %.2f", l.MatchConfidence)
		}
		sb.WriteString("\n")
	}
	if l.MatchedSnippet != "" {
		sb.WriteString("Snippet: ")
		sb.WriteString(l.MatchedSnippet)
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	return sb.String()
}

func emptyIfMissing(v string) string {
	if v == "" {
		return "-"
	}
	return v
}
