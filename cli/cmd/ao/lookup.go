// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/config"
	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/search"
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
	lookupGold      bool
	lookupPointers  bool
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
	lookupCmd.Flags().BoolVar(&lookupGold, "gold", false, "Retrieve from the sanitized OKF gold wiki (.ao/wiki) instead of the raw .agents/ corpus")
	lookupCmd.Flags().BoolVar(&lookupPointers, "pointers", false, "Compact pointers only (type · title · path · score) — no bodies; the token-cheap mode for decision-point pulls")
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

	// Mode 1: Lookup by ID — fetches ONE artifact's full content, so --pointers
	// (body-free) is contradictory and would otherwise leak a body past the
	// compact contract. Refuse rather than silently emit the full artifact.
	if len(args) > 0 {
		if lookupPointers {
			return fmt.Errorf("--pointers applies to --query/--bead retrieval, not by-ID fetch")
		}
		return lookupByID(cwd, args[0], cfg)
	}

	// Mode 2 or 3: Query-based or bead-based
	if lookupQuery == "" && lookupBead == "" {
		return fmt.Errorf("provide an ID argument, --query, or --bead flag")
	}

	return lookupByQuery(cwd, cfg)
}

// lookupByID federates a by-ID point read across the three knowledge sources
// (learnings, patterns, findings). It preserves the FIRST hard (non-NotFound)
// probe error and returns ErrArtifactNotFound ONLY when every source was a clean
// miss — an unreachable/corrupt source must never masquerade as absence, which
// could otherwise drive a wrong close/skip (age-gascity-port-slate-irye.5;
// mirrors gascity storeref.Resolve).
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

	found, err := resolveFederated(
		func() (bool, error) {
			learnings, err := collectLearnings(cwd, "", MaxLearningsToInject*5, globalLearningsDir, 1.0)
			if err != nil {
				return false, fmt.Errorf("search learnings for %s: %w", id, err)
			}
			for _, l := range learnings {
				if matchesID(l.ID, l.Source, id) {
					return true, outputLearning(cwd, l)
				}
			}
			return false, nil
		},
		func() (bool, error) {
			patterns, err := collectPatterns(cwd, "", MaxPatternsToInject*5, globalPatternsDir, 1.0)
			if err != nil {
				return false, fmt.Errorf("search patterns for %s: %w", id, err)
			}
			for _, p := range patterns {
				if matchesID(p.Name, p.FilePath, id) {
					return true, outputPattern(cwd, p)
				}
			}
			return false, nil
		},
		func() (bool, error) {
			findings, err := collectFindings(cwd, "", MaxPatternsToInject*5, globalFindingsDir, 1.0)
			if err != nil {
				return false, fmt.Errorf("search findings for %s: %w", id, err)
			}
			for _, f := range findings {
				if matchesID(f.ID, f.Source, id) {
					return true, outputFinding(cwd, f)
				}
			}
			return false, nil
		},
	)
	if err != nil {
		return err
	}
	if found {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrArtifactNotFound, id)
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
// collectGoldKnowledge retrieves learnings/patterns/findings from the OKF gold
// wiki (.ao/wiki) — the sanitized, durable layer — reusing the same scorers as
// the raw path. No global/canon merge: gold is already curated. Missing gold
// sections yield empty results (run `ao wiki gold` first).
// goldDocFiles drops OKF reserved catalog files (index.md, log.md) so they are
// never surfaced as knowledge during gold retrieval.
func goldDocFiles(paths []string) []string {
	out := paths[:0]
	for _, p := range paths {
		switch filepath.Base(p) {
		case "index.md", "log.md":
			continue
		}
		out = append(out, p)
	}
	return out
}

func collectGoldKnowledge(goldRoot, query string, limit int) ([]learning, []pattern, []knowledgeFinding) {
	now := nowFunc()
	tokens := queryTokens(strings.ToLower(query))
	qLower := strings.ToLower(query)
	// ε-exploration floor (cold-start): reserve a fraction of returned slots for
	// tail trails so under-explored gold gets surfaced/cited/reinforced. ε=0
	// (default) is identical to the prior top-K trim — zero behavior change.
	eps := search.ResolveRetrievalEpsilon()

	learnings := collectLocalLearnings(goldDocFiles(globLearningFiles(filepath.Join(goldRoot, SectionLearnings))), tokens, now)
	rankLearnings(learnings)
	learnings = search.ExploreSelect(learnings, limit, eps, nil)

	patterns, _ := collectPatternsFromDir(filepath.Join(goldRoot, SectionPatterns), qLower, now, false)
	rankPatterns(patterns) // utility-weighted rank before ExploreSelect (which assumes ranked input)
	patterns = search.ExploreSelect(patterns, limit, eps, nil)

	findings, _ := collectFindingsFromDir(filepath.Join(goldRoot, SectionFindings), qLower, now, false, false)
	rankFindings(findings) // utility-weighted rank before ExploreSelect (which assumes ranked input)
	findings = search.ExploreSelect(findings, limit, eps, nil)

	return learnings, patterns, findings
}

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

	var learnings []learning
	var patterns []pattern
	var findings []knowledgeFinding

	if lookupGold {
		// Retrieve from the sanitized OKF gold wiki — the durable, public-safe
		// layer — instead of raw .agents/. No global/canon merge: gold is
		// already the curated tier.
		learnings, patterns, findings = collectGoldKnowledge(filepath.Join(cwd, ".ao", "wiki"), query, limit)
	} else {
		// Surface a hard read failure instead of flattening it into an empty
		// result set — an unreachable source is not "no relevant knowledge"
		// (age-gascity-port-slate-irye.5). Missing .agents/ dirs return (nil, nil),
		// so this fires only on a genuine I/O/corrupt failure; the documented
		// "missing → empty" behavior is unchanged.
		var err error
		if learnings, err = collectLearnings(cwd, query, limit*3, globalLearningsDir, globalWeight); err != nil {
			return fmt.Errorf("collect learnings: %w", err)
		}
		if patterns, err = collectPatterns(cwd, query, limit, globalPatternsDir, globalWeight); err != nil {
			return fmt.Errorf("collect patterns: %w", err)
		}
		if findings, err = collectFindings(cwd, query, limit, globalFindingsDir, globalWeight); err != nil {
			return fmt.Errorf("collect findings: %w", err)
		}
	}

	// Apply bead filter if specified
	if lookupBead != "" {
		learnings = filterByBead(learnings, lookupBead)
	}

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
// outputPointers renders compact, body-free pointers — the token-cheap retrieval
// mode for decision-point pulls. One line per item (type · title · path · score),
// no Summary/evidence/bodies, so a mandatory pre-decision retrieval can't restack
// the prompt (the spray failure that killed the bookends: delta=0 @ 10.35M tokens).
func outputPointers(cwd string, learnings []learning, patterns []pattern, findings []knowledgeFinding) error {
	if len(learnings) == 0 && len(patterns) == 0 && len(findings) == 0 {
		fmt.Println("No matching artifacts found.")
		return nil
	}
	for _, l := range learnings {
		fmt.Printf("- [learning] %s — %s (score %.2f)\n", l.Title, relPath(cwd, l.Source), l.CompositeScore)
	}
	for _, p := range patterns {
		fmt.Printf("- [pattern] %s — %s (score %.2f)\n", p.Name, relPath(cwd, p.FilePath), p.CompositeScore)
	}
	for _, f := range findings {
		fmt.Printf("- [finding] %s — %s (score %.2f)\n", f.Title, relPath(cwd, f.Source), f.CompositeScore)
	}
	return nil
}

func outputResults(cwd string, learnings []learning, patterns []pattern, findings []knowledgeFinding) error {
	// --pointers wins over --json: the token-cheap, body-free contract must hold
	// on every output path (a --json --pointers combo must NOT emit bodies).
	if lookupPointers {
		return outputPointers(cwd, learnings, patterns, findings)
	}
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

// outputJSON prints an indented JSON value to stdout (same encoding as emitJSON).
func outputJSON(v any) error {
	return emitJSON(os.Stdout, v)
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
