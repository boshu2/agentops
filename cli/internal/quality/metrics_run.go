package quality

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

// ParseUtilityFromMarkdown extracts utility from markdown front matter.
func ParseUtilityFromMarkdown(path string) float64 {
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

// ParseUtilityFromJSONL extracts utility from the first line of a JSONL file.
func ParseUtilityFromJSONL(path string) float64 {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer func() {
		_ = f.Close()
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

// ParseUtilityFromFile extracts utility value from JSONL or markdown front matter.
func ParseUtilityFromFile(path string) float64 {
	if strings.HasSuffix(path, ".md") {
		return ParseUtilityFromMarkdown(path)
	}
	return ParseUtilityFromJSONL(path)
}

// CollectUtilityValuesFromDir walks a directory and collects utility values from files.
func CollectUtilityValuesFromDir(dir string) []float64 {
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
		if u := ParseUtilityFromFile(path); u > 0 {
			values = append(values, u)
		}
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to walk %s: %v\n", dir, err)
	}
	return values
}

// ComputeUtilityMetrics calculates MemRL utility statistics from a list of dirs.
func ComputeUtilityMetrics(dirs []string) UtilityStats {
	var utilities []float64
	for _, dir := range dirs {
		utilities = append(utilities, CollectUtilityValuesFromDir(dir)...)
	}
	return ComputeUtilityStats(utilities)
}

// RetroHasLearnings checks whether a retro markdown file contains a learnings section.
func RetroHasLearnings(path string) bool {
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	text := string(content)
	return strings.Contains(text, "## Learnings") ||
		strings.Contains(text, "## Key Learnings") ||
		strings.Contains(text, "### Learnings")
}

// CountRetros counts retro artifacts and how many have associated learnings since a time.
func CountRetros(baseDir string, since time.Time) (total int, withLearnings int, err error) {
	for _, retrosDir := range KnowledgeSectionDirs(baseDir, "retro") {
		if _, statErr := os.Stat(retrosDir); os.IsNotExist(statErr) {
			continue
		}

		if walkErr := filepath.Walk(retrosDir, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".md") || !info.ModTime().After(since) {
				return nil
			}
			total++
			if RetroHasLearnings(path) {
				withLearnings++
			}
			return nil
		}); walkErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to walk %s: %v\n", retrosDir, walkErr)
		}
	}

	return total, withLearnings, nil
}

// CountNewArtifactsInDir counts artifacts created after a time in a specific directory.
func CountNewArtifactsInDir(dir string, since time.Time) (int, error) {
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

// CountNewArtifactsInDirs counts artifacts created since a time across multiple dirs.
func CountNewArtifactsInDirs(dirs []string, since time.Time) (int, error) {
	count := 0
	for _, dir := range dirs {
		c, _ := CountNewArtifactsInDir(dir, since)
		count += c
	}
	return count, nil
}

// IsKnowledgeFile returns true if path ends with .md or .jsonl.
func IsKnowledgeFile(path string) bool {
	return strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".jsonl")
}

// KnowledgeSectionDirs returns both locations a knowledge section may live in:
// the canonical .agents/ao/<section> (where doctor's split fixer migrates
// files) and the legacy .agents/<section> fallback. Metrics read both so no
// split or migrated state hides artifacts (see doctor's
// fm-knowledge-orphaned-flywheel-learnings).
func KnowledgeSectionDirs(baseDir, section string) []string {
	return []string{
		filepath.Join(baseDir, ".agents", "ao", section),
		filepath.Join(baseDir, ".agents", section),
	}
}

// CountArtifactsByTier counts knowledge artifacts by tier under baseDir/.agents.
// sessionsDir, when non-empty, is included in the total count for sessions.
func CountArtifactsByTier(baseDir, sessionsDir string) (int, map[string]int, error) {
	tierCounts := map[string]int{
		"observation": 0,
		"learning":    0,
		"pattern":     0,
		"retro":       0,
		"skill":       0,
		"core":        0,
	}
	total := 0

	tierSections := map[string]string{
		"observation": "candidates",
		"learning":    "learnings",
		"pattern":     "patterns",
	}

	for tier, section := range tierSections {
		for _, dir := range KnowledgeSectionDirs(baseDir, section) {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				continue
			}
			files, err := filepath.Glob(filepath.Join(dir, "*.md"))
			if err != nil {
				continue
			}
			jsonlFiles, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
			files = append(files, jsonlFiles...)

			tierCounts[tier] += len(files)
			total += len(files)
		}
	}

	for _, researchDir := range KnowledgeSectionDirs(baseDir, "research") {
		if _, err := os.Stat(researchDir); err == nil {
			files, _ := filepath.Glob(filepath.Join(researchDir, "*.md"))
			tierCounts["observation"] += len(files)
			total += len(files)
		}
	}

	for _, retrosDir := range KnowledgeSectionDirs(baseDir, "retro") {
		if _, err := os.Stat(retrosDir); err == nil {
			files, _ := filepath.Glob(filepath.Join(retrosDir, "*.md"))
			tierCounts["retro"] += len(files)
			total += len(files)
		}
	}

	if sessionsDir != "" {
		if _, err := os.Stat(sessionsDir); err == nil {
			files, _ := filepath.Glob(filepath.Join(sessionsDir, "*.jsonl"))
			total += len(files)
		}
	}

	return total, tierCounts, nil
}

// CountNewArtifacts counts new artifacts since a time across the standard agent dirs.
func CountNewArtifacts(baseDir string, since time.Time) (int, error) {
	var dirs []string
	for _, section := range []string{"learnings", "patterns", "candidates", "research", "retro"} {
		dirs = append(dirs, KnowledgeSectionDirs(baseDir, section)...)
	}
	return CountNewArtifactsInDirs(dirs, since)
}

// BuildLastCitedMap builds a map of normalized artifact path → last citation time.
// normalize is a function that resolves a citation path to a stable absolute form.
func BuildLastCitedMap(citations []types.CitationEvent, normalize func(string) string) map[string]time.Time {
	lastCited := make(map[string]time.Time)
	for _, c := range citations {
		norm := normalize(c.ArtifactPath)
		if norm == "" {
			continue
		}
		if t, ok := lastCited[norm]; !ok || c.CitedAt.After(t) {
			lastCited[norm] = c.CitedAt
		}
	}
	return lastCited
}

// IsStaleArtifact reports whether an artifact is stale relative to a threshold.
func IsStaleArtifact(path string, modTime time.Time, staleThreshold time.Time, lastCited map[string]time.Time, normalize func(string) string) bool {
	if modTime.After(staleThreshold) {
		return false
	}
	norm := normalize(path)
	last, ok := lastCited[norm]
	return !ok || last.Before(staleThreshold)
}

// CountStaleInDir counts stale artifacts in one directory.
func CountStaleInDir(dir string, staleThreshold time.Time, lastCited map[string]time.Time, normalize func(string) string) int {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return 0
	}
	staleCount := 0
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !IsKnowledgeFile(path) {
			return nil
		}
		if IsStaleArtifact(path, info.ModTime(), staleThreshold, lastCited, normalize) {
			staleCount++
		}
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to walk %s: %v\n", dir, err)
	}
	return staleCount
}

// CountStaleArtifacts counts artifacts not cited in staleDays days under .agents/learnings and .agents/patterns.
func CountStaleArtifacts(baseDir string, citations []types.CitationEvent, staleDays int, normalize func(string) string) (int, error) {
	staleThreshold := time.Now().AddDate(0, 0, -staleDays)
	lastCited := BuildLastCitedMap(citations, normalize)

	var dirs []string
	for _, section := range []string{"learnings", "patterns"} {
		dirs = append(dirs, KnowledgeSectionDirs(baseDir, section)...)
	}
	total := 0
	for _, dir := range dirs {
		total += CountStaleInDir(dir, staleThreshold, lastCited, normalize)
	}
	return total, nil
}

// PrintMetricsParameters prints the σ/ρ/δ parameter section.
func PrintMetricsParameters(m *types.FlywheelMetrics) {
	fmt.Println("PARAMETERS:")
	fmt.Printf("  δ (avg age):        %.1f days active knowledge age\n", m.Delta)
	fmt.Printf("  σ (retrieval):      %.2f (%d%% retrievable artifacts surfaced)\n",
		m.Sigma, int(m.Sigma*100))
	fmt.Printf("  ρ (influence):      %.2f (%d%% of surfaced artifacts evidenced)\n", m.Rho, int(m.Rho*100))
	fmt.Println()
}

// PrintMetricsDerived prints the derived velocity/health section.
func PrintMetricsDerived(m *types.FlywheelMetrics) {
	velocitySign := "+"
	if m.Velocity < 0 {
		velocitySign = ""
	}
	statusIndicator := "✗"
	if m.AboveEscapeVelocity {
		statusIndicator = "✓"
	}
	threshold := EscapeVelocityThreshold(m.Delta)
	fmt.Println("DERIVED:")
	fmt.Printf("  σ × ρ = %.3f\n", m.SigmaRho)
	fmt.Printf("  δ/100 = %.3f\n", threshold)
	fmt.Println("  ────────────────")
	fmt.Printf("  VELOCITY: %s%.3f (escape=%s %s)\n", velocitySign, m.Velocity, m.EscapeVelocityStatus(), statusIndicator)
	fmt.Printf("  HEALTH:   %s\n", m.HealthStatus())
	fmt.Println()
}

// PrintMetricsCounts prints the COUNTS / TIER DISTRIBUTION sections.
func PrintMetricsCounts(m *types.FlywheelMetrics) {
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

// PrintMetricsLoopClosure prints the R1 loop closure section if non-empty.
func PrintMetricsLoopClosure(m *types.FlywheelMetrics) {
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

// PrintMetricsUtility prints the MemRL utility statistics section if non-empty.
func PrintMetricsUtility(m *types.FlywheelMetrics) {
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

// PrintMetricsTable prints a fully formatted flywheel metrics report.
func PrintMetricsTable(m *types.FlywheelMetrics) {
	fmt.Println()
	fmt.Println("Knowledge Flywheel Metrics")
	fmt.Println("==========================")
	fmt.Printf("Period: %s to %s\n\n",
		m.PeriodStart.Format("2006-01-02"),
		m.PeriodEnd.Format("2006-01-02"))
	PrintMetricsParameters(m)
	PrintMetricsDerived(m)
	PrintMetricsCounts(m)
	fmt.Printf("STATUS: %s\n", m.HealthStatus())
	if m.HealthStatus() != m.EscapeVelocityStatus() {
		fmt.Printf("ESCAPE: %s\n", m.EscapeVelocityStatus())
	}
	PrintMetricsLoopClosure(m)
	PrintMetricsUtility(m)
}
