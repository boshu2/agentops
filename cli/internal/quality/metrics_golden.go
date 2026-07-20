package quality

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

// CanonicalCitationType normalises citation type strings while preserving
// legacy values that existing citation logs and commands still use.
func CanonicalCitationType(ct string) string {
	switch strings.ToLower(strings.TrimSpace(ct)) {
	case types.CitationTypeApplied, "apply":
		return types.CitationTypeApplied
	case types.CitationTypeUsedInFinalArtifact, "used", "used_in_final_artifact", "used-in-final", "final-artifact":
		return types.CitationTypeUsedInFinalArtifact
	case types.CitationTypeHelpful:
		return types.CitationTypeHelpful
	case types.CitationTypeHarmful:
		return types.CitationTypeHarmful
	case types.CitationTypeRefuted:
		return types.CitationTypeRefuted
	case types.CitationTypeReference, "ref", "manual-reference":
		return types.CitationTypeReference
	case types.CitationTypeSkillLoaded, "skill-loaded":
		return types.CitationTypeSkillLoaded
	case "retrieved", "retrieve", "pulled", "pull":
		return types.CitationTypeRetrieved
	default:
		return strings.ToLower(strings.TrimSpace(ct))
	}
}

var researchRefPattern = regexp.MustCompile(`(?:/[^ \n\t"'\\]]*\.agents/research/[A-Za-z0-9._/-]+\.md|\.agents/research/[A-Za-z0-9._/-]+\.md)`)

// PopulateGoldenSignals fills metrics with the derived health verdicts.
func PopulateGoldenSignals(baseDir string, days int, findingsSection string, metrics *types.FlywheelMetrics) {
	if metrics == nil || metrics.GoldenSignals != nil {
		return
	}
	if gs, err := ComputeGoldenSignals(baseDir, days, findingsSection); err == nil {
		metrics.GoldenSignals = gs
	}
}

// ComputeGoldenSignals calculates the four golden signals.
func ComputeGoldenSignals(baseDir string, days int, findingsSection string) (*types.GoldenSignals, error) {
	gs := &types.GoldenSignals{}

	trend7d, trend30d, trendVerdict, err := ComputeVelocityTrend(baseDir)
	if err == nil {
		gs.VelocityTrend7d = trend7d
		gs.VelocityTrend30d = trend30d
		gs.TrendVerdict = trendVerdict
	}

	highPct, medDelta, appliedRatio, pipeVerdict := ComputeCitationPipeline(baseDir, days)
	gs.HighUtilityCitationPct = highPct
	gs.MedianUtilityDelta = medDelta
	gs.AppliedToRetrievedRatio = appliedRatio
	gs.PipelineVerdict = pipeVerdict

	orphanCount, orphanPct, avgAge, closureVerdict := ComputeResearchClosure(baseDir, findingsSection)
	gs.OrphanedResearchCount = orphanCount
	gs.OrphanedResearchPct = orphanPct
	gs.AvgOrphanAgeDays = avgAge
	gs.ClosureVerdict = closureVerdict

	gini, activePct, topBottom, concVerdict := ComputeReuseConcentration(baseDir, days)
	gs.CitationGini = gini
	gs.ActivePoolPct = activePct
	gs.Top10BottomRatio = topBottom
	gs.ConcentrationVerdict = concVerdict

	gs.OverallVerdict = ComputeOverallVerdict(gs)

	return gs, nil
}

// ComputeVelocityTrend reads metric baselines and computes the slope of velocity.
func ComputeVelocityTrend(baseDir string) (trend7d, trend30d float64, verdict string, err error) {
	metricsDir := filepath.Join(baseDir, ".agents", "ao", "metrics")
	entries, err := os.ReadDir(metricsDir)
	if err != nil {
		return 0, 0, "stagnant", nil
	}

	type velocityPoint struct {
		dayOffset float64
		velocity  float64
	}

	var points []velocityPoint
	now := time.Now()

	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "baseline-") || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(metricsDir, e.Name()))
		if err != nil {
			continue
		}
		var m types.FlywheelMetrics
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		dayOffset := now.Sub(m.Timestamp).Hours() / 24.0
		points = append(points, velocityPoint{dayOffset: dayOffset, velocity: m.Velocity})
	}

	if len(points) < 3 {
		return 0, 0, "stagnant", nil
	}

	sort.Slice(points, func(i, j int) bool {
		return points[i].dayOffset < points[j].dayOffset
	})

	var xs7, ys7 []float64
	for _, p := range points {
		if p.dayOffset <= 7 {
			xs7 = append(xs7, p.dayOffset)
			ys7 = append(ys7, p.velocity)
		}
	}
	if len(xs7) >= 2 {
		trend7d = LinearRegressionSlope(xs7, ys7)
		trend7d = -trend7d
	}

	var xs30, ys30 []float64
	for _, p := range points {
		if p.dayOffset <= 30 {
			xs30 = append(xs30, p.dayOffset)
			ys30 = append(ys30, p.velocity)
		}
	}
	if len(xs30) >= 2 {
		trend30d = LinearRegressionSlope(xs30, ys30)
		trend30d = -trend30d
	}

	trendRef := trend30d
	if len(xs30) < 3 {
		trendRef = trend7d
	}
	switch {
	case trendRef > 0.01:
		verdict = "compounding"
	case trendRef < -0.01:
		verdict = "decaying"
	default:
		verdict = "stagnant"
	}

	return trend7d, trend30d, verdict, nil
}

// ComputeCitationPipeline measures whether citations are delivering value.
func ComputeCitationPipeline(baseDir string, days int) (highPct, medianDelta, appliedRatio float64, verdict string) {
	citationsPath := filepath.Join(baseDir, ".agents", "ao", "citations.jsonl")
	feedbackPath := filepath.Join(baseDir, ".agents", "ao", "feedback.jsonl")
	now := time.Now()
	cutoff := now.AddDate(0, 0, -days)

	var applied, retrieved int
	citations := ReadJSONLFile(citationsPath)
	for _, raw := range citations {
		var c struct {
			CitedAt      time.Time `json:"cited_at"`
			CitationType string    `json:"citation_type"`
		}
		if json.Unmarshal(raw, &c) != nil || c.CitedAt.Before(cutoff) {
			continue
		}
		switch CanonicalCitationType(c.CitationType) {
		case types.CitationTypeApplied, types.CitationTypeUsedInFinalArtifact, types.CitationTypeHelpful:
			applied++
		case types.CitationTypeRetrieved:
			retrieved++
		}
	}
	total := applied + retrieved
	if total > 0 {
		appliedRatio = float64(applied) / float64(total)
	}

	var deltas []float64
	var highUtility int
	feedback := ReadJSONLFile(feedbackPath)
	for _, raw := range feedback {
		var f struct {
			RecordedAt    time.Time `json:"recorded_at"`
			Reward        float64   `json:"reward"`
			UtilityBefore float64   `json:"utility_before"`
			UtilityAfter  float64   `json:"utility_after"`
		}
		if json.Unmarshal(raw, &f) != nil || f.RecordedAt.Before(cutoff) {
			continue
		}
		deltas = append(deltas, f.UtilityAfter-f.UtilityBefore)
		if f.Reward > 0.6 {
			highUtility++
		}
	}

	if len(deltas) == 0 {
		return 0, 0, appliedRatio, "insufficient-data"
	}

	highPct = float64(highUtility) / float64(len(deltas)) * 100.0
	sort.Float64s(deltas)
	n := len(deltas)
	if n%2 == 0 {
		medianDelta = (deltas[n/2-1] + deltas[n/2]) / 2.0
	} else {
		medianDelta = deltas[n/2]
	}

	switch {
	case highPct > 60:
		verdict = "reinforcing"
	case highPct > 30:
		verdict = "inert"
	default:
		verdict = "degrading"
	}

	return highPct, medianDelta, appliedRatio, verdict
}

// ComputeResearchClosure measures whether research is being mined into learnings.
func ComputeResearchClosure(baseDir, findingsSection string) (orphanCount int, orphanPct float64, avgAgeDays float64, verdict string) {
	var researchFiles []os.DirEntry
	for _, researchDir := range KnowledgeSectionDirs(baseDir, "research") {
		if entries, err := os.ReadDir(researchDir); err == nil {
			researchFiles = append(researchFiles, entries...)
		}
	}
	if len(researchFiles) == 0 {
		return 0, 0, 0, "starved"
	}

	researchRefs := make(map[string]bool)
	for _, section := range []string{"learnings", findingsSection} {
		for _, dir := range KnowledgeSectionDirs(baseDir, section) {
			collectResearchRefsFromDir(dir, researchRefs)
		}
	}

	now := time.Now()
	var totalAgeDays float64
	var mdCount int

	for _, re := range researchFiles {
		if re.IsDir() || !strings.HasSuffix(re.Name(), ".md") {
			continue
		}
		mdCount++

		researchName := re.Name()
		researchStem := strings.TrimSuffix(researchName, ".md")
		closed := researchRefs[researchName] || researchRefs[researchStem]

		if !closed {
			orphanCount++
			info, err := re.Info()
			if err == nil {
				totalAgeDays += now.Sub(info.ModTime()).Hours() / 24.0
			}
		}
	}

	if mdCount == 0 {
		return 0, 0, 0, "starved"
	}

	orphanPct = float64(orphanCount) / float64(mdCount) * 100.0
	if orphanCount > 0 {
		avgAgeDays = totalAgeDays / float64(orphanCount)
	}

	switch {
	case orphanPct <= 10:
		verdict = "mining"
	case orphanPct < 50:
		verdict = "hoarding"
	default:
		verdict = "unmined"
	}

	return orphanCount, orphanPct, avgAgeDays, verdict
}

func collectResearchRefsFromDir(dir string, refs map[string]bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, ref := range ExtractResearchRefsFromText(string(data)) {
			base := filepath.Base(ref)
			if base == "." || base == string(filepath.Separator) || base == "" {
				continue
			}
			refs[base] = true
			refs[strings.TrimSuffix(base, ".md")] = true
		}
	}
}

// ExtractResearchRefsFromText extracts research file references from text.
func ExtractResearchRefsFromText(content string) []string {
	matches := researchRefPattern.FindAllString(content, -1)
	if len(matches) == 0 {
		return nil
	}
	refs := make([]string, 0, len(matches))
	seen := make(map[string]bool, len(matches))
	for _, match := range matches {
		ref := strings.Trim(match, `"'[]()`)
		if ref == "" || seen[ref] {
			continue
		}
		seen[ref] = true
		refs = append(refs, ref)
	}
	return refs
}

// CountCitationsInPeriod reads a JSONL citations file and returns per-artifact counts.
func CountCitationsInPeriod(citationsPath string, cutoff time.Time) map[string]int {
	counts := make(map[string]int)
	for _, raw := range ReadJSONLFile(citationsPath) {
		var c struct {
			ArtifactPath string    `json:"artifact_path"`
			CitedAt      time.Time `json:"cited_at"`
		}
		if json.Unmarshal(raw, &c) != nil || c.CitedAt.Before(cutoff) {
			continue
		}
		counts[filepath.Base(c.ArtifactPath)]++
	}
	return counts
}

// LearningCitationCounts builds a float64 slice of citation counts.
func LearningCitationCounts(entries []os.DirEntry, citationCounts map[string]int) (counts []float64, cited, total int) {
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		total++
		c := citationCounts[e.Name()]
		if c > 0 {
			cited++
		}
		counts = append(counts, float64(c))
	}
	return counts, cited, total
}

// Top10BottomRatio computes the ratio of top 10% to bottom 90%.
func Top10BottomRatio(sorted []float64) float64 {
	n := len(sorted)
	top10Start := n - n/10
	if top10Start == n {
		top10Start = n - 1
	}
	var top10Sum, bottomSum float64
	for i, v := range sorted {
		if i >= top10Start {
			top10Sum += v
		} else {
			bottomSum += v
		}
	}
	if bottomSum > 0 {
		return top10Sum / bottomSum
	}
	if top10Sum > 0 {
		return 999.0
	}
	return 0
}

// ComputeReuseConcentration measures whether the knowledge pool is active.
func ComputeReuseConcentration(baseDir string, days int) (gini, activePct, topBottomRatio float64, verdict string) {
	citationsPath := filepath.Join(baseDir, ".agents", "ao", "citations.jsonl")
	cutoff := time.Now().AddDate(0, 0, -days)

	citationCounts := CountCitationsInPeriod(citationsPath, cutoff)

	var learningEntries []os.DirEntry
	for _, learningsDir := range KnowledgeSectionDirs(baseDir, "learnings") {
		if entries, err := os.ReadDir(learningsDir); err == nil {
			learningEntries = append(learningEntries, entries...)
		}
	}
	if len(learningEntries) == 0 {
		return 0, 0, 0, "dormant"
	}

	counts, cited, total := LearningCitationCounts(learningEntries, citationCounts)
	if total == 0 {
		return 0, 0, 0, "dormant"
	}

	activePct = float64(cited) / float64(total) * 100.0

	if len(counts) > 1 {
		gini = GiniCoefficient(counts)
	}

	if len(counts) >= 10 {
		sort.Float64s(counts)
		topBottomRatio = Top10BottomRatio(counts)
	}

	switch {
	case gini < 0.4 && activePct > 30:
		verdict = "distributed"
	case gini > 0.7 || activePct < 10:
		verdict = "dormant"
	default:
		verdict = "concentrated"
	}

	return gini, activePct, topBottomRatio, verdict
}

// ComputeOverallVerdict combines the four signal verdicts into one.
func ComputeOverallVerdict(gs *types.GoldenSignals) string {
	positive := 0
	negative := 0

	switch gs.TrendVerdict {
	case "compounding":
		positive++
	case "decaying":
		negative++
	}
	switch gs.PipelineVerdict {
	case "reinforcing":
		positive++
	case "degrading":
		negative++
	}
	switch gs.ClosureVerdict {
	case "mining":
		positive++
	case "hoarding", "unmined":
		negative++
	}
	switch gs.ConcentrationVerdict {
	case "distributed":
		positive++
	case "dormant":
		negative++
	}

	switch {
	case positive >= 3:
		return "compounding"
	case negative >= 3:
		return "decaying"
	default:
		return "accumulating"
	}
}

// LinearRegressionSlope computes the least-squares slope.
func LinearRegressionSlope(xs, ys []float64) float64 {
	n := float64(len(xs))
	if n < 2 {
		return 0
	}
	var sumX, sumY, sumXY, sumX2 float64
	for i := range xs {
		sumX += xs[i]
		sumY += ys[i]
		sumXY += xs[i] * ys[i]
		sumX2 += xs[i] * xs[i]
	}
	denom := n*sumX2 - sumX*sumX
	if math.Abs(denom) < 1e-12 {
		return 0
	}
	return (n*sumXY - sumX*sumY) / denom
}

// GiniCoefficient computes the Gini coefficient for a set of values.
func GiniCoefficient(values []float64) float64 {
	n := len(values)
	if n == 0 {
		return 0
	}
	sorted := make([]float64, n)
	copy(sorted, values)
	sort.Float64s(sorted)

	var sum float64
	for _, v := range sorted {
		sum += v
	}
	if sum == 0 {
		return 0
	}

	var weightedSum float64
	for i, v := range sorted {
		weightedSum += float64(i+1) * v
	}
	g := (2*weightedSum)/(float64(n)*sum) - float64(n+1)/float64(n)
	if g < 0 {
		g = 0
	}
	return g
}

// FprintGoldenSignals displays golden signals in a formatted table.
func FprintGoldenSignals(w io.Writer, gs *types.GoldenSignals) {
	if gs == nil {
		fmt.Fprintln(w, "\nGolden Signals: insufficient data")
		return
	}

	fmt.Fprintln(w, "\n  GOLDEN SIGNALS:")
	fmt.Fprintln(w, "  ───────────────────────────────")
	fmt.Fprintf(w, "  1. Velocity Trend:    %s\n", gs.TrendVerdict)
	fmt.Fprintf(w, "     7d slope: %+.4f   30d slope: %+.4f\n", gs.VelocityTrend7d, gs.VelocityTrend30d)
	fmt.Fprintf(w, "  2. Citation Pipeline: %s\n", gs.PipelineVerdict)
	fmt.Fprintf(w, "     High-util: %.1f%%   Med delta: %+.4f   Applied: %.1f%%\n",
		gs.HighUtilityCitationPct, gs.MedianUtilityDelta, gs.AppliedToRetrievedRatio*100)
	fmt.Fprintf(w, "  3. Research Closure:  %s\n", gs.ClosureVerdict)
	fmt.Fprintf(w, "     Orphans: %d (%.1f%%)   Avg age: %.0fd\n",
		gs.OrphanedResearchCount, gs.OrphanedResearchPct, gs.AvgOrphanAgeDays)
	fmt.Fprintf(w, "  4. Reuse Concentr.:   %s\n", gs.ConcentrationVerdict)
	fmt.Fprintf(w, "     Gini: %.3f   Active: %.1f%%   Top10/Bot: %.1fx\n",
		gs.CitationGini, gs.ActivePoolPct, gs.Top10BottomRatio)
	fmt.Fprintln(w, "  ───────────────────────────────")
	fmt.Fprintf(w, "  OVERALL: %s\n", gs.OverallVerdict)
	fmt.Fprintln(w)
}

// ReadJSONLFile reads a JSONL file and returns raw JSON messages.
func ReadJSONLFile(path string) []json.RawMessage {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()

	var results []json.RawMessage
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		cp := make([]byte, len(line))
		copy(cp, line)
		results = append(results, json.RawMessage(cp))
	}
	return results
}
