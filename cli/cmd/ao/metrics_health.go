package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/types"
	"github.com/spf13/cobra"
)

// healthMetrics holds the computed flywheel health metrics.
type healthMetrics struct {
	// Sigma is retrieval effectiveness: cited learnings / injected learnings (last 10 sessions).
	Sigma float64 `json:"sigma"`

	// Rho is citation rate: fraction of injected knowledge that influenced a decision.
	Rho float64 `json:"rho"`

	// Delta is average age of active learnings in days.
	Delta float64 `json:"delta"`

	// EscapeVelocity is true when sigma*rho > delta/100 (compounding).
	EscapeVelocity bool `json:"escape_velocity"`

	// KnowledgeStock holds total counts of learnings, patterns, and constraints.
	KnowledgeStock knowledgeStock `json:"knowledge_stock"`

	// LoopDominance describes R1 vs B1 balance.
	LoopDominance loopDominance `json:"loop_dominance"`
}

// knowledgeStock tracks total knowledge artifact counts.
type knowledgeStock struct {
	Learnings   int `json:"learnings"`
	Patterns    int `json:"patterns"`
	Findings    int `json:"findings"`
	Constraints int `json:"constraints"`
	Total       int `json:"total"`
}

// loopDominance tracks R1 (reinforcing) vs B1 (balancing) loop metrics.
type loopDominance struct {
	// R1 is new learnings created per session (reinforcing loop).
	R1 float64 `json:"r1"`
	// B1 is learnings decayed per session (balancing loop).
	B1 float64 `json:"b1"`
	// Dominant is which loop is dominant: "R1" or "B1".
	Dominant string `json:"dominant"`
}

var metricsHealthCmd = &cobra.Command{
	Use:   "health",
	Short: "Show flywheel health metrics",
	Long: `Display flywheel health metrics including escape velocity status.

Metrics:
  sigma (retrieval coverage):      surfaced retrievable artifacts / total retrievable artifacts
  rho   (decision influence):      evidence-backed surfaced artifacts / surfaced artifacts
  delta (avg age):                 average age of active learnings in days
  escape_velocity:                 sigma * rho > delta/100 (compounding vs decaying)
  knowledge_stock:                 total learnings, patterns, constraints
  loop_dominance:                  R1 (new/session) vs B1 (decayed/session)

Examples:
  ao metrics health
  ao metrics health --json`,
	RunE: runMetricsHealth,
}

func init() {
	metricsCmd.AddCommand(metricsHealthCmd)
}

// runMetricsHealth computes and displays flywheel health metrics.
func runMetricsHealth(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	hm, err := computeHealthMetrics(cwd)
	if err != nil {
		return fmt.Errorf("compute health metrics: %w", err)
	}

	w := cmd.OutOrStdout()
	switch GetOutput() {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(hm)
	default:
		printHealthTable(w, hm)
	}

	return nil
}

// computeHealthMetrics gathers all health metrics from the repo.
func computeHealthMetrics(baseDir string) (*healthMetrics, error) {
	hm := &healthMetrics{}

	// Load citations
	citations, err := ratchet.LoadCitations(baseDir)
	if err != nil {
		VerbosePrintf("Warning: load citations: %v\n", err)
	}
	for i := range citations {
		citations[i].ArtifactPath = canonicalArtifactPath(baseDir, citations[i].ArtifactPath)
		citations[i].SessionID = canonicalSessionID(citations[i].SessionID)
	}

	// Compute sigma and rho from last 10 sessions
	hm.Sigma, hm.Rho = computeHealthSigmaRho(baseDir, citations)

	// Compute delta (average age of active learnings in days)
	hm.Delta = computeHealthDelta(baseDir)

	// Escape velocity: sigma * rho > delta / 100
	hm.EscapeVelocity = hm.Sigma*hm.Rho > escapeVelocityThreshold(hm.Delta)

	// Knowledge stock
	hm.KnowledgeStock = computeKnowledgeStock(baseDir)

	// Loop dominance
	hm.LoopDominance = computeLoopDominance(baseDir, citations)

	return hm, nil
}

// computeHealthSigmaRho computes sigma (retrieval coverage) and rho (decision influence)
// from citation data restricted to the last 10 unique sessions.
func computeHealthSigmaRho(baseDir string, citations []types.CitationEvent) (sigma, rho float64) {
	if len(citations) == 0 {
		return 0, 0
	}

	// Find last 10 unique session IDs (most recent first)
	sessionOrder := lastNSessions(citations, 10)
	if len(sessionOrder) == 0 {
		return 0, 0
	}
	sessionSet := make(map[string]bool, len(sessionOrder))
	for _, s := range sessionOrder {
		sessionSet[s] = true
	}

	// Filter citations to only those sessions
	var filtered []types.CitationEvent
	for _, c := range citations {
		if sessionSet[c.SessionID] {
			filtered = append(filtered, c)
		}
	}

	// Count unique surfaced artifacts and which of those later had evidence-backed use.
	citedUnique := make(map[string]bool)
	evidenceUnique := make(map[string]bool)
	for _, c := range filtered {
		if isRetrievableArtifactPath(baseDir, c.ArtifactPath) {
			artifactPath := normalizeArtifactPath(baseDir, c.ArtifactPath)
			citedUnique[artifactPath] = true
			switch effectiveCitationFeedbackType(c.CitationType) {
			case "applied", "reference":
				evidenceUnique[artifactPath] = true
			}
		}
	}

	// Count total retrievable artifacts (learnings + patterns)
	totalRetrievable := countFilesInDir(filepath.Join(baseDir, ".agents", "learnings")) +
		countFilesInDir(filepath.Join(baseDir, ".agents", "patterns")) +
		countFilesInDir(filepath.Join(baseDir, ".agents", SectionFindings))

	return computeOperationalSigmaRho(totalRetrievable, len(citedUnique), len(evidenceUnique))
}

// lastNSessions returns the last N unique session IDs from citations, ordered by most recent citation.
func lastNSessions(citations []types.CitationEvent, n int) []string {
	// Track latest citation time per session
	sessionLatest := make(map[string]time.Time)
	for _, c := range citations {
		if c.SessionID == "" {
			continue
		}
		if t, ok := sessionLatest[c.SessionID]; !ok || c.CitedAt.After(t) {
			sessionLatest[c.SessionID] = c.CitedAt
		}
	}

	// Sort by recency (simple selection of top N)
	type sessionTime struct {
		id string
		t  time.Time
	}
	var sessions []sessionTime
	for id, t := range sessionLatest {
		sessions = append(sessions, sessionTime{id, t})
	}
	// Sort descending by time
	for i := 0; i < len(sessions); i++ {
		for j := i + 1; j < len(sessions); j++ {
			if sessions[j].t.After(sessions[i].t) {
				sessions[i], sessions[j] = sessions[j], sessions[i]
			}
		}
	}

	limit := n
	if limit > len(sessions) {
		limit = len(sessions)
	}
	result := make([]string, limit)
	for i := 0; i < limit; i++ {
		result[i] = sessions[i].id
	}
	return result
}

// countFilesInDir counts .md and .jsonl files in a directory (non-recursive).
func countFilesInDir(dir string) int {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return 0
	}
	count := 0
	mdFiles, _ := filepath.Glob(filepath.Join(dir, "*.md"))
	count += len(mdFiles)
	jsonlFiles, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	count += len(jsonlFiles)
	jsonFiles, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	count += len(jsonFiles)
	return count
}

// computeHealthDelta computes the average age in days of active learnings.
func computeHealthDelta(baseDir string) float64 {
	learningsDir := filepath.Join(baseDir, ".agents", "learnings")
	if _, err := os.Stat(learningsDir); os.IsNotExist(err) {
		return 0
	}

	now := time.Now()
	var totalAge float64
	var count int

	_ = filepath.Walk(learningsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") && !strings.HasSuffix(path, ".jsonl") && !strings.HasSuffix(path, ".json") {
			return nil
		}
		age := now.Sub(info.ModTime()).Hours() / 24.0
		totalAge += age
		count++
		return nil
	})

	if count == 0 {
		return 0
	}
	return totalAge / float64(count)
}

// computeKnowledgeStock counts learnings, patterns, and constraints.
func computeKnowledgeStock(baseDir string) knowledgeStock {
	ks := knowledgeStock{
		Learnings:   countFilesInDir(filepath.Join(baseDir, ".agents", "learnings")),
		Patterns:    countFilesInDir(filepath.Join(baseDir, ".agents", "patterns")),
		Findings:    countFilesInDir(filepath.Join(baseDir, ".agents", SectionFindings)),
		Constraints: countConstraints(baseDir),
	}
	ks.Total = ks.Learnings + ks.Patterns + ks.Findings + ks.Constraints
	return ks
}

// countConstraints counts constraint files. Constraints may live in
// .agents/constraints/ or as YAML/JSON files in other known locations.
func countConstraints(baseDir string) int {
	constraintsDir := filepath.Join(baseDir, ".agents", "constraints")
	if _, err := os.Stat(constraintsDir); os.IsNotExist(err) {
		return 0
	}
	count := 0
	mdFiles, _ := filepath.Glob(filepath.Join(constraintsDir, "*.md"))
	count += len(mdFiles)
	yamlFiles, _ := filepath.Glob(filepath.Join(constraintsDir, "*.yaml"))
	count += len(yamlFiles)
	jsonFiles, _ := filepath.Glob(filepath.Join(constraintsDir, "*.json"))
	for _, f := range jsonFiles {
		if filepath.Base(f) != "index.json" {
			count++
		}
	}
	return count
}

// computeLoopDominance computes R1 (new learnings per session) and B1 (decayed per session).
func computeLoopDominance(baseDir string, citations []types.CitationEvent) loopDominance {
	ld := loopDominance{Dominant: "B1"}

	// Count sessions from cycle history or citations
	sessionCount := countUniqueSessions(citations)
	if sessionCount == 0 {
		return ld
	}

	// R1: new learnings per session
	// Count learnings created in the last 30 days as a proxy
	learningsDir := filepath.Join(baseDir, ".agents", "learnings")
	now := time.Now()
	newSince := now.AddDate(0, 0, -30)
	newLearnings, _ := countNewArtifactsInDir(learningsDir, newSince)
	ld.R1 = float64(newLearnings) / float64(sessionCount)

	// B1: decayed learnings per session
	// Use stale artifacts (90d+ uncited) as a proxy for decay
	staleSince := now.AddDate(0, 0, -90)
	staleLearnings := countStaleInDir(baseDir, learningsDir, staleSince, buildLastCitedMap(baseDir, citations))
	ld.B1 = float64(staleLearnings) / float64(sessionCount)

	if ld.R1 > ld.B1 {
		ld.Dominant = "R1"
	}

	return ld
}

// countUniqueSessions counts unique session IDs from citations.
func countUniqueSessions(citations []types.CitationEvent) int {
	seen := make(map[string]bool)
	for _, c := range citations {
		if c.SessionID != "" {
			seen[c.SessionID] = true
		}
	}
	return len(seen)
}

// loadCycleHistory loads cycle-history.jsonl entries. Returns nil on missing file.
func loadCycleHistory(baseDir string) []map[string]any {
	path := filepath.Join(baseDir, ".agents", "evolve", "cycle-history.jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() {
		_ = f.Close()
	}()

	var entries []map[string]any
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		var entry map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &entry); err == nil {
			entries = append(entries, entry)
		}
	}
	if err := scanner.Err(); err != nil {
		VerbosePrintf("Warning: reading cycle history: %v\n", err)
	}
	return entries
}

// printHealthTable prints a formatted health metrics table.
func printHealthTable(w io.Writer, hm *healthMetrics) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flywheel Health")
	fmt.Fprintln(w, "===============")
	fmt.Fprintln(w)

	// Core metrics
	fmt.Fprintln(w, "RETRIEVAL:")
	fmt.Fprintf(w, "  sigma (retrieval effectiveness): %.3f\n", hm.Sigma)
	fmt.Fprintf(w, "  rho   (decision influence):      %.3f\n", hm.Rho)
	fmt.Fprintf(w, "  delta (avg learning age, days):   %.1f\n", hm.Delta)
	fmt.Fprintln(w)

	// Escape velocity
	escapeStatus := "DECAYING"
	escapeIcon := "[!]"
	if hm.EscapeVelocity {
		escapeStatus = "COMPOUNDING"
		escapeIcon = "[+]"
	}
	fmt.Fprintln(w, "ESCAPE VELOCITY:")
	fmt.Fprintf(w, "  sigma * rho = %.4f\n", hm.Sigma*hm.Rho)
	fmt.Fprintf(w, "  delta / 100 = %.4f\n", escapeVelocityThreshold(hm.Delta))
	fmt.Fprintf(w, "  status:       %s %s\n", escapeStatus, escapeIcon)
	fmt.Fprintln(w)

	// Knowledge stock
	fmt.Fprintln(w, "KNOWLEDGE STOCK:")
	fmt.Fprintf(w, "  learnings:   %d\n", hm.KnowledgeStock.Learnings)
	fmt.Fprintf(w, "  patterns:    %d\n", hm.KnowledgeStock.Patterns)
	fmt.Fprintf(w, "  findings:    %d\n", hm.KnowledgeStock.Findings)
	fmt.Fprintf(w, "  constraints: %d\n", hm.KnowledgeStock.Constraints)
	fmt.Fprintf(w, "  total:       %d\n", hm.KnowledgeStock.Total)
	fmt.Fprintln(w)

	// Loop dominance
	fmt.Fprintln(w, "LOOP DOMINANCE:")
	fmt.Fprintf(w, "  R1 (new/session):     %.2f\n", hm.LoopDominance.R1)
	fmt.Fprintf(w, "  B1 (decayed/session): %.2f\n", hm.LoopDominance.B1)
	fmt.Fprintf(w, "  dominant:             %s\n", hm.LoopDominance.Dominant)
	fmt.Fprintln(w)
}
