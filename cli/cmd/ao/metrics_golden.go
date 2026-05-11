// practices: [dora-metrics, sre]
package main

import (
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/boshu2/agentops/cli/internal/quality"
	"github.com/boshu2/agentops/cli/internal/types"
)

func canonicalCitationType(ct string) string { return quality.CanonicalCitationType(ct) }

func populateGoldenSignals(baseDir string, days int, metrics *types.FlywheelMetrics) {
	quality.PopulateGoldenSignals(baseDir, days, SectionFindings, metrics)
}

func computeGoldenSignals(baseDir string, days int) (*types.GoldenSignals, error) {
	return quality.ComputeGoldenSignals(baseDir, days, SectionFindings)
}

func computeVelocityTrend(baseDir string) (trend7d, trend30d float64, verdict string, err error) {
	return quality.ComputeVelocityTrend(baseDir)
}

func computeCitationPipeline(baseDir string, days int) (highPct, medianDelta, appliedRatio float64, verdict string) {
	return quality.ComputeCitationPipeline(baseDir, days)
}

func computeResearchClosure(baseDir string) (orphanCount int, orphanPct float64, avgAgeDays float64, verdict string) {
	return quality.ComputeResearchClosure(baseDir, SectionFindings)
}

func extractResearchRefsFromText(content string) []string {
	return quality.ExtractResearchRefsFromText(content)
}

func countCitationsInPeriod(citationsPath string, cutoff time.Time) map[string]int {
	return quality.CountCitationsInPeriod(citationsPath, cutoff)
}

func learningCitationCounts(entries []os.DirEntry, citationCounts map[string]int) (counts []float64, cited, total int) {
	return quality.LearningCitationCounts(entries, citationCounts)
}

func top10BottomRatio(sorted []float64) float64 { return quality.Top10BottomRatio(sorted) }

func computeReuseConcentration(baseDir string, days int) (gini, activePct, topBottomRatio float64, verdict string) {
	return quality.ComputeReuseConcentration(baseDir, days)
}

func computeOverallVerdict(gs *types.GoldenSignals) string {
	return quality.ComputeOverallVerdict(gs)
}

func linearRegressionSlope(xs, ys []float64) float64 {
	return quality.LinearRegressionSlope(xs, ys)
}

func giniCoefficient(values []float64) float64 { return quality.GiniCoefficient(values) }

func fprintGoldenSignals(w io.Writer, gs *types.GoldenSignals) {
	quality.FprintGoldenSignals(w, gs)
}

func readJSONLFile(path string) []json.RawMessage { return quality.ReadJSONLFile(path) }
