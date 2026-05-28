// practices: [dora-metrics, sre]
package main

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/quality"
	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/storage"
	"github.com/boshu2/agentops/cli/internal/types"
)

var (
	metricsDays int
)

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Knowledge flywheel metrics",
	Long: `Track and report on knowledge flywheel metrics.

The flywheel equation:
  dK/dt = I(t) - δ·K + σ·ρ·K - B(K, K_crit)

Where:
  K     = Total knowledge artifacts
  I(t)  = New knowledge inflow
  δ     = Average age of active knowledge in days
  σ     = Retrieval coverage (0-1)
  ρ     = Decision influence rate among surfaced artifacts (0-1)
  B()   = Breakdown function at capacity

Operational escape velocity: σ × ρ > δ/100 → Knowledge compounds

Commands:
  baseline   Capture current flywheel state
  report     Show metrics with escape velocity status`,
}

func init() {
	metricsCmd.GroupID = "core"
	rootCmd.AddCommand(metricsCmd)

	// baseline subcommand
	baselineCmd := &cobra.Command{
		Use:   "baseline",
		Short: "Capture current flywheel state",
		Long: `Capture a baseline snapshot of the knowledge flywheel.

Records:
  - Total artifact counts by tier
  - Citation counts and patterns
  - Current σ, ρ estimates
  - Escape velocity status

Output is saved to .agents/ao/metrics/baseline-YYYY-MM-DD.json

Examples:
  ao metrics baseline
  ao metrics baseline --days 7
  ao metrics baseline --json`,
		RunE: runMetricsBaseline,
	}
	baselineCmd.Flags().IntVar(&metricsDays, "days", 7, "Period in days for metrics calculation")
	metricsCmd.AddCommand(baselineCmd)

	// report subcommand
	reportCmd := &cobra.Command{
		Use:   "report",
		Short: "Show flywheel metrics report",
		Long: `Display a formatted report of knowledge flywheel metrics.

Shows:
  - Core parameters (δ, σ, ρ)
  - Derived values (σ×ρ, velocity)
  - Escape velocity status
  - Artifact counts by tier
  - Trend indicators

Examples:
  ao metrics report
  ao metrics report --days 30
  ao metrics report --json`,
		RunE: runMetricsReport,
	}
	reportCmd.Flags().IntVar(&metricsDays, "days", 7, "Period in days for metrics calculation")
	metricsCmd.AddCommand(reportCmd)

	// cite subcommand - record a citation event
	citeCmd := &cobra.Command{
		Use:   "cite <artifact-path>",
		Short: "Record a citation event",
		Long: `Record that an artifact was cited in this session.

Citation events drive the knowledge flywheel:
  - Increases ρ (citation rate)
  - Contributes to σ×ρ calculation
  - Can trigger auto-promotion after threshold

Examples:
  ao metrics cite .agents/learnings/mutex-pattern.md
  ao metrics cite .agents/patterns/error-handling.md --type applied
  ao metrics cite .agents/research/oauth.md --session abc123`,
		Args: cobra.ExactArgs(1),
		RunE: runMetricsCite,
	}
	var citeType, citeSession, citeQuery, citeVendor string
	citeCmd.Flags().StringVar(&citeType, "type", "reference", "Citation type: recall, reference, applied")
	citeCmd.Flags().StringVar(&citeSession, "session", "", "Session ID (auto-detected if not provided)")
	citeCmd.Flags().StringVar(&citeQuery, "query", "", "Search query that surfaced this artifact")
	citeCmd.Flags().StringVar(&citeVendor, "vendor", "", "Model vendor attribution: claude, codex")
	metricsCmd.AddCommand(citeCmd)
}

// periodCitationStats holds citation statistics for a period
type periodCitationStats struct {
	citations   []types.CitationEvent
	uniqueCited map[string]bool
}

// normalizeArtifactPath resolves citation/file paths to a stable absolute form.
func normalizeArtifactPath(baseDir, artifactPath string) string {
	return canonicalArtifactPath(baseDir, artifactPath)
}

func isRetrievableArtifactPath(baseDir, artifactPath string) bool {
	p := filepath.ToSlash(normalizeArtifactPath(baseDir, artifactPath))
	learningsRoot := filepath.ToSlash(filepath.Join(baseDir, ".agents", "learnings")) + "/"
	patternsRoot := filepath.ToSlash(filepath.Join(baseDir, ".agents", "patterns")) + "/"
	findingsRoot := filepath.ToSlash(filepath.Join(baseDir, ".agents", SectionFindings)) + "/"
	return strings.HasPrefix(p, learningsRoot) || strings.HasPrefix(p, patternsRoot) || strings.HasPrefix(p, findingsRoot)
}

func isFindingArtifactPath(baseDir, artifactPath string) bool {
	p := filepath.ToSlash(normalizeArtifactPath(baseDir, artifactPath))
	findingsRoot := filepath.ToSlash(filepath.Join(baseDir, ".agents", SectionFindings)) + "/"
	return strings.HasPrefix(p, findingsRoot)
}

func retrievableCitationStats(baseDir string, citations []types.CitationEvent) (uniqueCount, evidenceCount int) {
	unique := make(map[string]bool)
	evidence := make(map[string]bool)
	for _, c := range citations {
		if !isRetrievableArtifactPath(baseDir, c.ArtifactPath) {
			continue
		}
		artifactPath := normalizeArtifactPath(baseDir, c.ArtifactPath)
		unique[artifactPath] = true
		if citationEventIsHighConfidence(c) {
			evidence[artifactPath] = true
		}
	}
	return len(unique), len(evidence)
}

// filterCitationsForPeriod filters citations to a time period
func filterCitationsForPeriod(citations []types.CitationEvent, start, end time.Time) periodCitationStats {
	stats := periodCitationStats{
		uniqueCited: make(map[string]bool),
	}
	for _, c := range citations {
		if c.CitedAt.After(start) && c.CitedAt.Before(end) {
			stats.citations = append(stats.citations, c)
			stats.uniqueCited[c.ArtifactPath] = true
		}
	}
	return stats
}

func computeOperationalSigmaRho(totalArtifacts, uniqueCited, evidenceBacked int) (sigma, rho float64) {
	return quality.ComputeOperationalSigmaRho(totalArtifacts, uniqueCited, evidenceBacked)
}

// computeSigmaRho keeps the historical signature used in tests and callers,
// but the operational semantics are retrieval coverage and evidence-backed use.
func computeSigmaRho(totalArtifacts, uniqueCited, evidenceBacked, _ int) (sigma, rho float64) {
	return computeOperationalSigmaRho(totalArtifacts, uniqueCited, evidenceBacked)
}

func escapeVelocityThreshold(delta float64) float64 {
	return quality.EscapeVelocityThreshold(delta)
}

// countLoopMetrics counts learnings created vs found for loop closure
func countLoopMetrics(baseDir string, periodStart time.Time, periodCitations []types.CitationEvent) (created, found int) {
	created, _ = countNewArtifactsInDir(filepath.Join(baseDir, ".agents", "learnings"), periodStart)
	for _, c := range periodCitations {
		if strings.Contains(filepath.ToSlash(canonicalArtifactPath(baseDir, c.ArtifactPath)), "/learnings/") {
			found++
		}
	}
	return created, found
}

// countBypassCitations counts prior art bypass citations
func countBypassCitations(citations []types.CitationEvent) int {
	count := 0
	for _, c := range citations {
		if c.CitationType == "bypass" || strings.HasPrefix(c.ArtifactPath, "bypass:") {
			count++
		}
	}
	return count
}

// computeMetrics calculates flywheel metrics for a period.
func computeMetrics(baseDir string, days int) (*types.FlywheelMetrics, error) {
	return computeMetricsForNamespace(baseDir, days, primaryMetricNamespace)
}

func computeMetricsForNamespace(baseDir string, days int, namespace string) (*types.FlywheelMetrics, error) {
	now := time.Now()
	periodStart := now.AddDate(0, 0, -days)

	metrics := &types.FlywheelMetrics{
		Timestamp:   now,
		PeriodStart: periodStart,
		PeriodEnd:   now,
		TierCounts:  make(map[string]int),
	}

	// Delta is tracked operationally as average age of active knowledge in days.
	metrics.Delta = computeHealthDelta(baseDir)

	// Count artifacts
	totalArtifacts, tierCounts, err := countArtifacts(baseDir)
	if err != nil {
		VerbosePrintf("Warning: count artifacts: %v\n", err)
	}
	metrics.TotalArtifacts = totalArtifacts
	metrics.TierCounts = tierCounts

	// Load and filter citations
	citations, err := ratchet.LoadCitations(baseDir)
	if err != nil {
		VerbosePrintf("Warning: load citations: %v\n", err)
	}
	for i := range citations {
		citations[i].ArtifactPath = canonicalArtifactPath(baseDir, citations[i].ArtifactPath)
		citations[i].SessionID = canonicalSessionID(citations[i].SessionID)
		citations[i].MetricNamespace = canonicalMetricNamespace(citations[i].MetricNamespace)
	}
	citations = filterCitationsByMetricNamespace(citations, namespace)
	stats := filterCitationsForPeriod(citations, periodStart, now)
	metrics.CitationsThisPeriod = len(stats.citations)
	metrics.UniqueCitedArtifacts = len(stats.uniqueCited)

	// Calculate σ and ρ
	// σ denominator: only count retrievable artifacts (learnings + patterns),
	// not candidates, research, retros, or sessions which inject never retrieves.
	retrievable := metrics.TierCounts["learning"] + metrics.TierCounts["pattern"]
	retrievableUnique, retrievableEvidence := retrievableCitationStats(baseDir, stats.citations)
	metrics.Sigma, metrics.Rho = computeSigmaRho(
		retrievable, retrievableUnique, retrievableEvidence, days,
	)
	metrics.SigmaRho = metrics.Sigma * metrics.Rho
	threshold := escapeVelocityThreshold(metrics.Delta)
	metrics.Velocity = metrics.SigmaRho - threshold
	metrics.AboveEscapeVelocity = metrics.SigmaRho > threshold

	// Count new and stale artifacts
	if newCount, err := countNewArtifacts(baseDir, periodStart); err == nil {
		metrics.NewArtifacts = newCount
	}
	if staleCount, err := countStaleArtifacts(baseDir, citations, 90); err == nil {
		metrics.StaleArtifacts = staleCount
	}

	// Loop closure metrics
	metrics.LearningsCreated, metrics.LearningsFound = countLoopMetrics(baseDir, periodStart, stats.citations)
	if metrics.LearningsCreated > 0 {
		metrics.LoopClosureRatio = float64(metrics.LearningsFound) / float64(metrics.LearningsCreated)
	}
	metrics.PriorArtBypasses = countBypassCitations(stats.citations)

	// Retros
	retros, retrosWithLearnings, _ := countRetros(baseDir, periodStart)
	metrics.TotalRetros = retros
	metrics.RetrosWithLearnings = retrosWithLearnings

	// MemRL utility metrics
	utilityStats := computeUtilityMetrics(baseDir)
	metrics.MeanUtility = utilityStats.mean
	metrics.UtilityStdDev = utilityStats.stdDev
	metrics.HighUtilityCount = utilityStats.highCount
	metrics.LowUtilityCount = utilityStats.lowCount

	return metrics, nil
}

// countArtifacts counts knowledge artifacts by tier.
func countArtifacts(baseDir string) (int, map[string]int, error) {
	sessionsDir := filepath.Join(baseDir, storage.DefaultBaseDir, storage.SessionsDir)
	return quality.CountArtifactsByTier(baseDir, sessionsDir)
}

// countNewArtifacts counts artifacts created after a time.
func countNewArtifacts(baseDir string, since time.Time) (int, error) {
	return quality.CountNewArtifacts(baseDir, since)
}

// buildLastCitedMap builds a map of normalized artifact path → last citation time.
func buildLastCitedMap(baseDir string, citations []types.CitationEvent) map[string]time.Time {
	return quality.BuildLastCitedMap(citations, func(p string) string {
		return normalizeArtifactPath(baseDir, p)
	})
}

// isKnowledgeFile returns true if path ends with .md or .jsonl.
func isKnowledgeFile(path string) bool {
	return quality.IsKnowledgeFile(path)
}

// isStaleArtifact returns true if the artifact was modified before staleThreshold and
// has no citation at or after staleThreshold.
func isStaleArtifact(baseDir, path string, modTime time.Time, staleThreshold time.Time, lastCited map[string]time.Time) bool {
	return quality.IsStaleArtifact(path, modTime, staleThreshold, lastCited, func(p string) string {
		return normalizeArtifactPath(baseDir, p)
	})
}

// countStaleInDir counts stale artifacts in one directory.
func countStaleInDir(baseDir, dir string, staleThreshold time.Time, lastCited map[string]time.Time) int {
	return quality.CountStaleInDir(dir, staleThreshold, lastCited, func(p string) string {
		return normalizeArtifactPath(baseDir, p)
	})
}

// countStaleArtifacts counts artifacts not cited in N days.
func countStaleArtifacts(baseDir string, citations []types.CitationEvent, staleDays int) (int, error) {
	return quality.CountStaleArtifacts(baseDir, citations, staleDays, func(p string) string {
		return normalizeArtifactPath(baseDir, p)
	})
}

func printMetricsParameters(m *types.FlywheelMetrics) { quality.PrintMetricsParameters(m) }
func printMetricsDerived(m *types.FlywheelMetrics)    { quality.PrintMetricsDerived(m) }
func printMetricsCounts(m *types.FlywheelMetrics)     { quality.PrintMetricsCounts(m) }
func printMetricsLoopClosure(m *types.FlywheelMetrics) {
	quality.PrintMetricsLoopClosure(m)
}
func printMetricsUtility(m *types.FlywheelMetrics) { quality.PrintMetricsUtility(m) }

// printMetricsTable prints a formatted metrics table.
func printMetricsTable(m *types.FlywheelMetrics) { quality.PrintMetricsTable(m) }

// countNewArtifactsInDir counts artifacts created after a time in a specific directory.
func countNewArtifactsInDir(dir string, since time.Time) (int, error) {
	return quality.CountNewArtifactsInDir(dir, since)
}

// retroHasLearnings checks whether a retro markdown file contains a learnings section.
func retroHasLearnings(path string) bool {
	return quality.RetroHasLearnings(path)
}

// countRetros counts retro artifacts and how many have associated learnings.
func countRetros(baseDir string, since time.Time) (total int, withLearnings int, err error) {
	return quality.CountRetros(baseDir, since)
}

// utilityStats holds computed utility statistics.
type utilityStats struct {
	mean      float64
	stdDev    float64
	highCount int // utility > 0.7
	lowCount  int // utility < 0.3
}

// computeUtilityStats calculates statistics from a slice of utility values.
func computeUtilityStats(utilities []float64) utilityStats {
	s := quality.ComputeUtilityStats(utilities)
	return utilityStats{
		mean:      s.Mean,
		stdDev:    s.StdDev,
		highCount: s.HighCount,
		lowCount:  s.LowCount,
	}
}

// computeUtilityMetrics calculates MemRL utility statistics from learnings.
func computeUtilityMetrics(baseDir string) utilityStats {
	s := quality.ComputeUtilityMetrics([]string{
		filepath.Join(baseDir, ".agents", "learnings"),
		filepath.Join(baseDir, ".agents", "patterns"),
	})
	return utilityStats{
		mean:      s.Mean,
		stdDev:    s.StdDev,
		highCount: s.HighCount,
		lowCount:  s.LowCount,
	}
}

// parseUtilityFromFile extracts utility value from JSONL or markdown front matter.
func parseUtilityFromFile(path string) float64 {
	return quality.ParseUtilityFromFile(path)
}

// collectUtilityValuesFromDir walks a directory and collects utility values from files.
func collectUtilityValuesFromDir(dir string) []float64 {
	return quality.CollectUtilityValuesFromDir(dir)
}

// parseUtilityFromMarkdown extracts utility from markdown front matter.
func parseUtilityFromMarkdown(path string) float64 {
	return quality.ParseUtilityFromMarkdown(path)
}

// parseUtilityFromJSONL extracts utility from the first line of a JSONL file.
func parseUtilityFromJSONL(path string) float64 {
	return quality.ParseUtilityFromJSONL(path)
}
