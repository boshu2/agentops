package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/boshu2/agentops/cli/internal/quality"
	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/types"
	"github.com/spf13/cobra"
)

// healthMetrics holds the computed flywheel health metrics.
type healthMetrics struct {
	// MetricNamespace is the citation namespace used for sigma/rho calculations.
	MetricNamespace string `json:"metric_namespace"`

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

	// VendorMetrics holds per-vendor sigma/rho breakdown.
	VendorMetrics map[string]vendorMetrics `json:"vendor_metrics,omitempty"`
}

// vendorMetrics holds sigma/rho computed from citations attributed to a single vendor.
type vendorMetrics struct {
	Sigma           float64 `json:"sigma"`
	Rho             float64 `json:"rho"`
	SigmaRho        float64 `json:"sigma_rho"`
	CitationCount   int     `json:"citation_count"`
	UniqueArtifacts int     `json:"unique_artifacts"`
	EvidenceBacked  int     `json:"evidence_backed"`
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

var metricsHealthNamespace string

func init() {
	metricsCmd.AddCommand(metricsHealthCmd)
	metricsHealthCmd.Flags().StringVar(&metricsHealthNamespace, "namespace", primaryMetricNamespace, "Citation namespace to evaluate (primary by default)")
}

// runMetricsHealth computes and displays flywheel health metrics.
func runMetricsHealth(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	hm, err := computeHealthMetricsForNamespace(cwd, metricsHealthNamespace)
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
	return computeHealthMetricsForNamespace(baseDir, primaryMetricNamespace)
}

func computeHealthMetricsForNamespace(baseDir, namespace string) (*healthMetrics, error) {
	canonicalNamespace := canonicalMetricNamespace(namespace)
	hm := &healthMetrics{MetricNamespace: canonicalNamespace}

	// Load citations
	citations, err := ratchet.LoadCitations(baseDir)
	if err != nil {
		VerbosePrintf("Warning: load citations: %v\n", err)
	}
	for i := range citations {
		citations[i].ArtifactPath = canonicalArtifactPath(baseDir, citations[i].ArtifactPath)
		citations[i].SessionID = canonicalSessionID(citations[i].SessionID)
		citations[i].MetricNamespace = canonicalMetricNamespace(citations[i].MetricNamespace)
	}
	citations = filterCitationsByMetricNamespace(citations, canonicalNamespace)

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

	// Per-vendor metrics
	hm.VendorMetrics = computeVendorMetrics(baseDir, citations)

	return hm, nil
}

// countRetrievableArtifacts returns the total number of retrievable artifacts (learnings + patterns + findings).
func countRetrievableArtifacts(baseDir string) int {
	return countFilesInDir(filepath.Join(baseDir, ".agents", "learnings")) +
		countFilesInDir(filepath.Join(baseDir, ".agents", "patterns")) +
		countFilesInDir(filepath.Join(baseDir, ".agents", SectionFindings))
}

// computeVendorMetrics computes sigma/rho per model vendor from attributed citations.
func computeVendorMetrics(baseDir string, citations []types.CitationEvent) map[string]vendorMetrics {
	vendors := make(map[string][]types.CitationEvent)
	for _, c := range citations {
		v := c.ModelVendor
		if v == "" {
			v = "unattributed"
		}
		vendors[v] = append(vendors[v], c)
	}

	result := make(map[string]vendorMetrics)
	for vendor, vendorCitations := range vendors {
		unique, evidence := retrievableCitationStats(baseDir, vendorCitations)
		totalRetrievable := countRetrievableArtifacts(baseDir)
		var sigma, rho float64
		if totalRetrievable > 0 {
			sigma = float64(unique) / float64(totalRetrievable)
			if sigma > 1.0 {
				sigma = 1.0
			}
		}
		if unique > 0 {
			rho = float64(evidence) / float64(unique)
		}
		result[vendor] = vendorMetrics{
			Sigma:           sigma,
			Rho:             rho,
			SigmaRho:        sigma * rho,
			CitationCount:   len(vendorCitations),
			UniqueArtifacts: unique,
			EvidenceBacked:  evidence,
		}
	}
	return result
}

// EscapeVelocityTargets defines the target thresholds for escape velocity verification.
type EscapeVelocityTargets struct {
	MinSigma    float64 `json:"min_sigma"`
	MinRho      float64 `json:"min_rho"`
	MinSigmaRho float64 `json:"min_sigma_rho,omitempty"` // if 0, computed as delta/100
}

// DefaultEscapeVelocityTargets returns the canonical targets from the flywheel equation.
func DefaultEscapeVelocityTargets() EscapeVelocityTargets {
	return EscapeVelocityTargets{
		MinSigma: 0.30,
		MinRho:   0.65,
	}
}

// EscapeVelocityVerification holds the results of verifying a namespace against targets.
type EscapeVelocityVerification struct {
	Namespace    string                 `json:"namespace"`
	Targets      EscapeVelocityTargets  `json:"targets"`
	Actual       healthMetrics          `json:"actual"`
	SigmaPass    bool                   `json:"sigma_pass"`
	RhoPass      bool                   `json:"rho_pass"`
	SigmaRhoPass bool                   `json:"sigma_rho_pass"`
	AllPass      bool                   `json:"all_pass"`
	Failures     []string               `json:"failures,omitempty"`
}

// VerifyEscapeVelocity checks whether a namespace's metrics meet the target thresholds.
func VerifyEscapeVelocity(hm *healthMetrics, targets EscapeVelocityTargets) *EscapeVelocityVerification {
	v := &EscapeVelocityVerification{
		Namespace: hm.MetricNamespace,
		Targets:   targets,
		Actual:    *hm,
	}

	v.SigmaPass = hm.Sigma >= targets.MinSigma
	v.RhoPass = hm.Rho >= targets.MinRho

	sigmaRhoThreshold := targets.MinSigmaRho
	if sigmaRhoThreshold == 0 {
		sigmaRhoThreshold = escapeVelocityThreshold(hm.Delta)
	}
	v.SigmaRhoPass = hm.Sigma*hm.Rho > sigmaRhoThreshold

	if !v.SigmaPass {
		v.Failures = append(v.Failures, fmt.Sprintf("sigma %.3f < target %.3f", hm.Sigma, targets.MinSigma))
	}
	if !v.RhoPass {
		v.Failures = append(v.Failures, fmt.Sprintf("rho %.3f < target %.3f", hm.Rho, targets.MinRho))
	}
	if !v.SigmaRhoPass {
		v.Failures = append(v.Failures, fmt.Sprintf("sigma*rho %.3f <= threshold %.3f", hm.Sigma*hm.Rho, sigmaRhoThreshold))
	}

	v.AllPass = v.SigmaPass && v.RhoPass && v.SigmaRhoPass
	return v
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
			if citationEventIsHighConfidence(c) {
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

func lastNSessions(citations []types.CitationEvent, n int) []string {
	return quality.LastNSessions(citations, n)
}

func countFilesInDir(dir string) int { return quality.CountFilesInDir(dir) }

func computeHealthDelta(baseDir string) float64 { return quality.ComputeHealthDelta(baseDir) }

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

func countConstraints(baseDir string) int { return quality.CountConstraints(baseDir) }

// computeLoopDominance computes R1 (new learnings per session) and B1 (decayed per session).
func computeLoopDominance(baseDir string, citations []types.CitationEvent) loopDominance {
	ld := loopDominance{Dominant: "none"}

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

	switch {
	case ld.R1 > ld.B1:
		ld.Dominant = "R1"
	case ld.B1 > ld.R1:
		ld.Dominant = "B1"
	}

	return ld
}

func countUniqueSessions(citations []types.CitationEvent) int {
	return quality.CountUniqueSessions(citations)
}

func loadCycleHistory(baseDir string) []map[string]any { return quality.LoadCycleHistory(baseDir) }

// printHealthTable prints a formatted health metrics table.
func printHealthTable(w io.Writer, hm *healthMetrics) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flywheel Health")
	fmt.Fprintln(w, "===============")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Namespace: %s\n\n", hm.MetricNamespace)

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

	// Per-vendor metrics (if any attributed citations exist)
	if len(hm.VendorMetrics) > 0 {
		hasAttributed := false
		for v := range hm.VendorMetrics {
			if v != "unattributed" {
				hasAttributed = true
				break
			}
		}
		if hasAttributed {
			fmt.Fprintln(w, "VENDOR METRICS:")
			for vendor, vm := range hm.VendorMetrics {
				if vendor == "unattributed" {
					continue
				}
				fmt.Fprintf(w, "  %s: sigma=%.3f rho=%.3f sigma*rho=%.4f (citations=%d, unique=%d, evidence=%d)\n",
					vendor, vm.Sigma, vm.Rho, vm.SigmaRho, vm.CitationCount, vm.UniqueArtifacts, vm.EvidenceBacked)
			}
			if um, ok := hm.VendorMetrics["unattributed"]; ok {
				fmt.Fprintf(w, "  unattributed: %d citations\n", um.CitationCount)
			}
			fmt.Fprintln(w)
		}
	}
}
