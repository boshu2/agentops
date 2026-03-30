package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

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
	var citeType, citeSession, citeQuery string
	citeCmd.Flags().StringVar(&citeType, "type", "reference", "Citation type: recall, reference, applied")
	citeCmd.Flags().StringVar(&citeSession, "session", "", "Session ID (auto-detected if not provided)")
	citeCmd.Flags().StringVar(&citeQuery, "query", "", "Search query that surfaced this artifact")
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
		switch effectiveCitationFeedbackType(c.CitationType) {
		case "applied", "reference":
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
	if totalArtifacts > 0 {
		sigma = float64(uniqueCited) / float64(totalArtifacts)
		if sigma > 1.0 {
			sigma = 1.0
		}
	}
	if uniqueCited > 0 {
		rho = float64(evidenceBacked) / float64(uniqueCited)
		if rho > 1.0 {
			rho = 1.0
		}
	}
	return sigma, rho
}

// computeSigmaRho keeps the historical signature used in tests and callers,
// but the operational semantics are retrieval coverage and evidence-backed use.
func computeSigmaRho(totalArtifacts, uniqueCited, evidenceBacked, _ int) (sigma, rho float64) {
	return computeOperationalSigmaRho(totalArtifacts, uniqueCited, evidenceBacked)
}

func escapeVelocityThreshold(delta float64) float64 {
	if delta <= 0 {
		return 0
	}
	return delta / 100.0
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
	}
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
	tierCounts := map[string]int{
		"observation": 0,
		"learning":    0,
		"pattern":     0,
		"retro":       0,
		"skill":       0,
		"core":        0,
	}
	total := 0

	// Tier locations
	tierDirs := map[string]string{
		"observation": filepath.Join(baseDir, ".agents", "candidates"),
		"learning":    filepath.Join(baseDir, ".agents", "learnings"),
		"pattern":     filepath.Join(baseDir, ".agents", "patterns"),
	}

	for tier, dir := range tierDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		files, err := filepath.Glob(filepath.Join(dir, "*.md"))
		if err != nil {
			continue
		}
		// Also count JSONL
		jsonlFiles, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
		files = append(files, jsonlFiles...)

		tierCounts[tier] = len(files)
		total += len(files)
	}

	// Count research artifacts
	researchDir := filepath.Join(baseDir, ".agents", "research")
	if _, err := os.Stat(researchDir); err == nil {
		files, _ := filepath.Glob(filepath.Join(researchDir, "*.md"))
		tierCounts["observation"] += len(files)
		total += len(files)
	}

	// Count retros (separate tier from learnings for consistent counts)
	retrosDir := filepath.Join(baseDir, ".agents", "retros")
	if _, err := os.Stat(retrosDir); err == nil {
		files, _ := filepath.Glob(filepath.Join(retrosDir, "*.md"))
		tierCounts["retro"] = len(files)
		total += len(files)
	}

	// Count sessions
	sessionsDir := filepath.Join(baseDir, storage.DefaultBaseDir, storage.SessionsDir)
	if _, err := os.Stat(sessionsDir); err == nil {
		files, _ := filepath.Glob(filepath.Join(sessionsDir, "*.jsonl"))
		total += len(files)
	}

	return total, tierCounts, nil
}

// countNewArtifacts counts artifacts created after a time.
func countNewArtifacts(baseDir string, since time.Time) (int, error) {
	count := 0

	dirs := []string{
		filepath.Join(baseDir, ".agents", "learnings"),
		filepath.Join(baseDir, ".agents", "patterns"),
		filepath.Join(baseDir, ".agents", "candidates"),
		filepath.Join(baseDir, ".agents", "research"),
		filepath.Join(baseDir, ".agents", "retros"),
	}

	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if info.ModTime().After(since) {
				count++
			}
			return nil
		}); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to walk %s: %v\n", dir, err)
		}
	}

	return count, nil
}

// buildLastCitedMap builds a map of normalized artifact path → last citation time.
func buildLastCitedMap(baseDir string, citations []types.CitationEvent) map[string]time.Time {
	lastCited := make(map[string]time.Time)
	for _, c := range citations {
		norm := normalizeArtifactPath(baseDir, c.ArtifactPath)
		if norm == "" {
			continue
		}
		if t, ok := lastCited[norm]; !ok || c.CitedAt.After(t) {
			lastCited[norm] = c.CitedAt
		}
	}
	return lastCited
}

// isKnowledgeFile returns true if path ends with .md or .jsonl.
func isKnowledgeFile(path string) bool {
	return strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".jsonl")
}

// isStaleArtifact returns true if the artifact was modified before staleThreshold and
// has no citation at or after staleThreshold.
func isStaleArtifact(baseDir, path string, modTime time.Time, staleThreshold time.Time, lastCited map[string]time.Time) bool {
	if modTime.After(staleThreshold) {
		return false
	}
	norm := normalizeArtifactPath(baseDir, path)
	last, ok := lastCited[norm]
	return !ok || last.Before(staleThreshold)
}

// countStaleInDir counts stale artifacts in one directory.
func countStaleInDir(baseDir, dir string, staleThreshold time.Time, lastCited map[string]time.Time) int {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return 0
	}
	staleCount := 0
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !isKnowledgeFile(path) {
			return nil
		}
		if isStaleArtifact(baseDir, path, info.ModTime(), staleThreshold, lastCited) {
			staleCount++
		}
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to walk %s: %v\n", dir, err)
	}
	return staleCount
}

// countStaleArtifacts counts artifacts not cited in N days.
func countStaleArtifacts(baseDir string, citations []types.CitationEvent, staleDays int) (int, error) {
	staleThreshold := time.Now().AddDate(0, 0, -staleDays)
	lastCited := buildLastCitedMap(baseDir, citations)

	dirs := []string{
		filepath.Join(baseDir, ".agents", "learnings"),
		filepath.Join(baseDir, ".agents", "patterns"),
	}
	total := 0
	for _, dir := range dirs {
		total += countStaleInDir(baseDir, dir, staleThreshold, lastCited)
	}
	return total, nil
}

func printMetricsParameters(m *types.FlywheelMetrics) {
	fmt.Println("PARAMETERS:")
	fmt.Printf("  δ (avg age):        %.1f days active knowledge age\n", m.Delta)
	fmt.Printf("  σ (retrieval):      %.2f (%d%% retrievable artifacts surfaced)\n",
		m.Sigma, int(m.Sigma*100))
	fmt.Printf("  ρ (influence):      %.2f (%d%% of surfaced artifacts evidenced)\n", m.Rho, int(m.Rho*100))
	fmt.Println()
}

func printMetricsDerived(m *types.FlywheelMetrics) {
	velocitySign := "+"
	if m.Velocity < 0 {
		velocitySign = ""
	}
	statusIndicator := "✗"
	if m.AboveEscapeVelocity {
		statusIndicator = "✓"
	}
	threshold := escapeVelocityThreshold(m.Delta)
	fmt.Println("DERIVED:")
	fmt.Printf("  σ × ρ = %.3f\n", m.SigmaRho)
	fmt.Printf("  δ/100 = %.3f\n", threshold)
	fmt.Println("  ────────────────")
	fmt.Printf("  VELOCITY: %s%.3f (escape=%s %s)\n", velocitySign, m.Velocity, m.EscapeVelocityStatus(), statusIndicator)
	fmt.Printf("  HEALTH:   %s\n", m.HealthStatus())
	fmt.Println()
}

func printMetricsCounts(m *types.FlywheelMetrics) {
	fmt.Println("COUNTS:")
	fmt.Printf("  Knowledge items:    %d\n", m.TotalArtifacts)
	fmt.Printf("  Citation events:    %d this period\n", m.CitationsThisPeriod)
	fmt.Printf("  Unique cited:       %d\n", m.UniqueCitedArtifacts)
	fmt.Printf("  New artifacts:      %d\n", m.NewArtifacts)
	fmt.Printf("  Stale (90d+):       %d\n", m.StaleArtifacts)
	fmt.Println()

	if len(m.TierCounts) > 0 {
		fmt.Println("TIER DISTRIBUTION:")
		for _, tier := range []string{"observation", "learning", "retro", "pattern", "skill", "core"} {
			if count, ok := m.TierCounts[tier]; ok && count > 0 {
				fmt.Printf("  %-12s: %d\n", tier, count)
			}
		}
		fmt.Println()
	}
}

func printMetricsLoopClosure(m *types.FlywheelMetrics) {
	if m.LearningsCreated == 0 && m.LearningsFound == 0 && m.TotalRetros == 0 {
		return
	}
	loopStatus := "OPEN"
	if m.LoopClosureRatio >= 1.0 {
		loopStatus = "CLOSED ✓"
	} else if m.LoopClosureRatio > 0 {
		loopStatus = "PARTIAL"
	}
	fmt.Println()
	fmt.Println("LOOP CLOSURE (R1):")
	fmt.Printf("  Learnings created:  %d\n", m.LearningsCreated)
	fmt.Printf("  Learnings found:    %d\n", m.LearningsFound)
	fmt.Printf("  Closure ratio:      %.2f (%s)\n", m.LoopClosureRatio, loopStatus)
	if m.TotalRetros > 0 {
		fmt.Printf("  Retros:             %d (%d with learnings)\n", m.TotalRetros, m.RetrosWithLearnings)
	}
	if m.PriorArtBypasses > 0 {
		fmt.Printf("  Prior art bypasses: %d (review recommended)\n", m.PriorArtBypasses)
	}
}

func printMetricsUtility(m *types.FlywheelMetrics) {
	if m.MeanUtility == 0 && m.HighUtilityCount == 0 && m.LowUtilityCount == 0 {
		return
	}
	fmt.Println()
	fmt.Println("UTILITY (MemRL):")
	fmt.Printf("  Mean utility:        %.3f\n", m.MeanUtility)
	fmt.Printf("  Std deviation:       %.3f\n", m.UtilityStdDev)
	fmt.Printf("  High utility (>0.7): %d\n", m.HighUtilityCount)
	fmt.Printf("  Low utility (<0.3):  %d\n", m.LowUtilityCount)
	switch {
	case m.MeanUtility >= 0.6:
		fmt.Printf("  Status:              HEALTHY ✓ (learnings are effective)\n")
	case m.MeanUtility >= 0.4:
		fmt.Printf("  Status:              NEUTRAL (need more feedback data)\n")
	default:
		fmt.Printf("  Status:              REVIEW ✗ (learnings may need updating)\n")
	}
}

// printMetricsTable prints a formatted metrics table.
func printMetricsTable(m *types.FlywheelMetrics) {
	fmt.Println()
	fmt.Println("Knowledge Flywheel Metrics")
	fmt.Println("==========================")
	fmt.Printf("Period: %s to %s\n\n",
		m.PeriodStart.Format("2006-01-02"),
		m.PeriodEnd.Format("2006-01-02"))
	printMetricsParameters(m)
	printMetricsDerived(m)
	printMetricsCounts(m)
	fmt.Printf("STATUS: %s\n", m.HealthStatus())
	if m.HealthStatus() != m.EscapeVelocityStatus() {
		fmt.Printf("ESCAPE: %s\n", m.EscapeVelocityStatus())
	}
	printMetricsLoopClosure(m)
	printMetricsUtility(m)
}

// countNewArtifactsInDir counts artifacts created after a time in a specific directory.
func countNewArtifactsInDir(dir string, since time.Time) (int, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return 0, nil
	}

	count := 0
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if info.ModTime().After(since) {
			count++
		}
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to walk %s: %v\n", dir, err)
	}

	return count, nil
}

// retroHasLearnings checks whether a retro markdown file contains a learnings section.
func retroHasLearnings(path string) bool {
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	text := string(content)
	return strings.Contains(text, "## Learnings") ||
		strings.Contains(text, "## Key Learnings") ||
		strings.Contains(text, "### Learnings")
}

// countRetros counts retro artifacts and how many have associated learnings.
func countRetros(baseDir string, since time.Time) (total int, withLearnings int, err error) {
	retrosDir := filepath.Join(baseDir, ".agents", "retros")
	if _, err := os.Stat(retrosDir); os.IsNotExist(err) {
		return 0, 0, nil
	}

	if err := filepath.Walk(retrosDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") || !info.ModTime().After(since) {
			return nil
		}
		total++
		if retroHasLearnings(path) {
			withLearnings++
		}
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to walk %s: %v\n", retrosDir, err)
	}

	return total, withLearnings, nil
}

// utilityStats holds computed utility statistics.
type utilityStats struct {
	mean      float64
	stdDev    float64
	highCount int // utility > 0.7
	lowCount  int // utility < 0.3
}

// collectUtilityValuesFromDir walks a directory and collects utility values from files.
func collectUtilityValuesFromDir(dir string) []float64 {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	var values []float64
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".jsonl") && !strings.HasSuffix(path, ".md") {
			return nil
		}
		if u := parseUtilityFromFile(path); u > 0 {
			values = append(values, u)
		}
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to walk %s: %v\n", dir, err)
	}
	return values
}

// computeUtilityStats calculates statistics from a slice of utility values.
func computeUtilityStats(utilities []float64) utilityStats {
	var stats utilityStats
	if len(utilities) == 0 {
		return stats
	}
	var sum float64
	for _, u := range utilities {
		sum += u
	}
	stats.mean = sum / float64(len(utilities))
	var variance float64
	for _, u := range utilities {
		variance += (u - stats.mean) * (u - stats.mean)
	}
	stats.stdDev = math.Sqrt(variance / float64(len(utilities)))
	for _, u := range utilities {
		if u > 0.7 {
			stats.highCount++
		}
		if u < 0.3 {
			stats.lowCount++
		}
	}
	return stats
}

// computeUtilityMetrics calculates MemRL utility statistics from learnings.
func computeUtilityMetrics(baseDir string) utilityStats {
	var utilities []float64
	for _, dir := range []string{
		filepath.Join(baseDir, ".agents", "learnings"),
		filepath.Join(baseDir, ".agents", "patterns"),
	} {
		utilities = append(utilities, collectUtilityValuesFromDir(dir)...)
	}
	return computeUtilityStats(utilities)
}

// parseUtilityFromMarkdown extracts utility from markdown front matter.
func parseUtilityFromMarkdown(path string) float64 {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return 0
	}
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "---" {
			break
		}
		if strings.HasPrefix(line, "utility:") {
			var utility float64
			if _, parseErr := fmt.Sscanf(line, "utility: %f", &utility); parseErr == nil {
				return utility
			}
		}
	}
	return 0
}

// parseUtilityFromJSONL extracts utility from the first line of a JSONL file.
func parseUtilityFromJSONL(path string) float64 {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer func() {
		_ = f.Close() //nolint:errcheck // read-only utility parse, close error non-fatal
	}()
	scanner := bufio.NewScanner(f)
	if scanner.Scan() {
		var data map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &data); err == nil {
			if utility, ok := data["utility"].(float64); ok {
				return utility
			}
		}
	}
	return 0
}

// parseUtilityFromFile extracts utility value from JSONL or markdown front matter.
func parseUtilityFromFile(path string) float64 {
	if strings.HasSuffix(path, ".md") {
		return parseUtilityFromMarkdown(path)
	}
	return parseUtilityFromJSONL(path)
}
