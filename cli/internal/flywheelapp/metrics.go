// practices: [dora-metrics, sre]
package flywheelapp

import (
	"path/filepath"
	"strings"
	"time"

	ratchet "github.com/boshu2/agentops/cli/internal/evidence"
	"github.com/boshu2/agentops/cli/internal/quality"
	"github.com/boshu2/agentops/cli/internal/storage"
	"github.com/boshu2/agentops/cli/internal/types"
)

// MetricsDaysDefault is the default period, in days, for flywheel metric
// commands. It preserves the historical `--days` default carried by the
// pre-carve package-global flag var.
const MetricsDaysDefault = 7

// The .agents knowledge sections the flywheel metrics scan.
const (
	SectionFindings = "findings"
)

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
	for _, section := range []string{"learnings", "patterns", SectionFindings} {
		for _, root := range quality.KnowledgeSectionDirs(baseDir, section) {
			if strings.HasPrefix(p, filepath.ToSlash(root)+"/") {
				return true
			}
		}
	}
	return false
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
	created, _ = quality.CountNewArtifactsInDirs(quality.KnowledgeSectionDirs(baseDir, "learnings"), periodStart)
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

func computeMetricsForNamespace(baseDir string, days int, namespace string, verbosef func(format string, args ...any)) (*types.FlywheelMetrics, error) {
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
		verbosef("Warning: count artifacts: %v\n", err)
	}
	metrics.TotalArtifacts = totalArtifacts
	metrics.TierCounts = tierCounts

	// Load and filter citations
	citations, err := ratchet.LoadCitations(baseDir)
	if err != nil {
		verbosef("Warning: load citations: %v\n", err)
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

// countStaleArtifacts counts artifacts not cited in N days.
func countStaleArtifacts(baseDir string, citations []types.CitationEvent, staleDays int) (int, error) {
	return quality.CountStaleArtifacts(baseDir, citations, staleDays, func(p string) string {
		return normalizeArtifactPath(baseDir, p)
	})
}

// countRetros counts retro artifacts and how many have associated learnings.
func countRetros(baseDir string, since time.Time) (total int, withLearnings int, err error) {
	return quality.CountRetros(baseDir, since)
}

func computeHealthDelta(baseDir string) float64 { return quality.ComputeHealthDelta(baseDir) }

// utilityStats holds computed utility statistics.
type utilityStats struct {
	mean      float64
	stdDev    float64
	highCount int // utility > 0.7
	lowCount  int // utility < 0.3
}

// computeUtilityMetrics calculates MemRL utility statistics from learnings.
func computeUtilityMetrics(baseDir string) utilityStats {
	s := quality.ComputeUtilityMetrics(append(
		quality.KnowledgeSectionDirs(baseDir, "learnings"),
		quality.KnowledgeSectionDirs(baseDir, "patterns")...,
	))
	return utilityStats{
		mean:      s.Mean,
		stdDev:    s.StdDev,
		highCount: s.HighCount,
		lowCount:  s.LowCount,
	}
}

const highConfidenceCitationThreshold = 0.7

func citationConfidenceScore(citationType string) float64 {
	switch canonicalCitationType(citationType) {
	case types.CitationTypeHelpful:
		return 0.9
	case types.CitationTypeUsedInFinalArtifact, types.CitationTypeApplied:
		return 0.9
	case types.CitationTypeReference:
		return 0.7
	case types.CitationTypeRetrieved:
		return 0.5
	default:
		return 0
	}
}

func normalizeCitationMatchConfidence(confidence float64) float64 {
	switch confidence {
	case 0, 0.5, 0.7, 0.9:
		return confidence
	}
	switch {
	case confidence >= highConfidenceCitationThreshold:
		return 0.9
	case confidence >= 0.5:
		return 0.7
	case confidence > 0:
		return 0.5
	default:
		return 0
	}
}

func citationEventIsHighConfidence(citation types.CitationEvent) bool {
	if canonicalCitationType(citation.CitationType) == types.CitationTypeHarmful ||
		canonicalCitationType(citation.CitationType) == types.CitationTypeRefuted {
		return false
	}
	if citation.MatchConfidence > 0 {
		return normalizeCitationMatchConfidence(citation.MatchConfidence) >= highConfidenceCitationThreshold
	}
	return citationConfidenceScore(citation.CitationType) >= highConfidenceCitationThreshold
}
