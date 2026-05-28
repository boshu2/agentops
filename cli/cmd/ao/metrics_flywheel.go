// practices: [dora-metrics, sre]
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/boshu2/agentops/cli/internal/types"
	"github.com/spf13/cobra"
)

// flywheelGolden is kept for backward compatibility (--golden flag) but golden
// signals now always compute. The flag is a no-op.
var flywheelGolden bool
var flywheelStatusNamespace string

// flywheelCmd provides a convenient alias for flywheel status operations.
var flywheelCmd = &cobra.Command{
	Use:   "flywheel",
	Short: "Knowledge flywheel operations",
	Long: `Knowledge flywheel operations and status.

The flywheel equation:
  dK/dt = I(t) - δ·K + σ·ρ·K - B(K, K_crit)

Operational escape velocity: σρ > δ/100 → Knowledge compounds

Commands:
  status   Show comprehensive flywheel health
  gate     Check release readiness against closure, retrieval, and rho gates

Examples:
  ao flywheel status
  ao flywheel gate
  ao flywheel status --json`,
}

func init() {
	flywheelCmd.GroupID = "core"
	rootCmd.AddCommand(flywheelCmd)

	// flywheel status subcommand
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show flywheel health status",
		Long: `Display comprehensive flywheel health status.

Shows:
  - Delta (δ): Average age of active knowledge in days
  - Sigma (σ): Retrieval coverage
  - Rho (ρ): Decision influence among surfaced artifacts
  - Velocity: σρ - δ/100 (net operational growth)
  - Status: COMPOUNDING / NEAR ESCAPE / DECAYING

Examples:
  ao flywheel status
  ao flywheel status --days 30
  ao flywheel status --json`,
		RunE: runFlywheelStatus,
	}
	statusCmd.Flags().IntVar(&metricsDays, "days", 7, "Period in days for metrics calculation")
	statusCmd.Flags().StringVar(&flywheelStatusNamespace, "namespace", primaryMetricNamespace, "Citation namespace to evaluate (primary by default)")
	statusCmd.Flags().BoolVar(&flywheelGolden, "golden", false, "Show golden signals (always shown; flag kept for compatibility)")
	_ = statusCmd.Flags().MarkHidden("golden")
	flywheelCmd.AddCommand(statusCmd)

	// flywheel compare subcommand
	compareCmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare primary vs shadow namespace metrics",
		Long: `Compare retrieval quality between primary and shadow namespaces.

Shows sigma, rho, and escape velocity side-by-side.
Use this to decide whether the shadow scorer is ready for promotion.

Promotion rule:
  Shadow must beat primary on sigma AND show non-regressing rho.

Rollback rule:
  Stop writing to the promoted namespace. No data rewrite needed.

Examples:
  ao flywheel compare
  ao flywheel compare --shadow experimental
  ao flywheel compare --json`,
		RunE: runFlywheelCompare,
	}
	compareCmd.Flags().StringVar(&flywheelCompareNamespace, "shadow", "shadow", "Shadow namespace to compare against primary")
	flywheelCmd.AddCommand(compareCmd)
}

var flywheelCompareNamespace string

// namespaceComparison holds side-by-side metrics for two namespaces.
type namespaceComparison struct {
	Primary          *types.FlywheelMetrics `json:"primary"`
	Shadow           *types.FlywheelMetrics `json:"shadow"`
	ShadowName       string                 `json:"shadow_name"`
	SigmaDelta       float64                `json:"sigma_delta"`
	RhoDelta         float64                `json:"rho_delta"`
	VelocityDelta    float64                `json:"velocity_delta"`
	PromotionReady   bool                   `json:"promotion_ready"`
	PromotionReason  string                 `json:"promotion_reason"`
	RollbackContract string                 `json:"rollback_contract"`
}

func runFlywheelCompare(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	primaryMetrics, err := computeMetricsForNamespace(cwd, metricsDays, primaryMetricNamespace)
	if err != nil {
		return fmt.Errorf("compute primary metrics: %w", err)
	}

	shadowMetrics, err := computeMetricsForNamespace(cwd, metricsDays, flywheelCompareNamespace)
	if err != nil {
		return fmt.Errorf("compute shadow metrics: %w", err)
	}

	comp := buildNamespaceComparison(primaryMetrics, shadowMetrics, flywheelCompareNamespace)

	w := cmd.OutOrStdout()
	switch GetOutput() {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(comp)
	default:
		printNamespaceComparison(w, comp)
	}
	return nil
}

func buildNamespaceComparison(primary, shadow *types.FlywheelMetrics, shadowName string) *namespaceComparison {
	comp := &namespaceComparison{
		Primary:          primary,
		Shadow:           shadow,
		ShadowName:       canonicalMetricNamespace(shadowName),
		SigmaDelta:       shadow.Sigma - primary.Sigma,
		RhoDelta:         shadow.Rho - primary.Rho,
		VelocityDelta:    shadow.Velocity - primary.Velocity,
		RollbackContract: "Stop reading/writing the shadow namespace. Primary data is never mutated by shadow runs.",
	}

	// Promotion rule: shadow sigma > primary sigma AND shadow rho >= primary rho (non-regressing)
	shadowBeatsSigma := shadow.Sigma > primary.Sigma
	rhoNonRegressing := shadow.Rho >= primary.Rho-0.01 // 1% tolerance for noise
	if shadowBeatsSigma && rhoNonRegressing {
		comp.PromotionReady = true
		comp.PromotionReason = fmt.Sprintf("Shadow sigma (%.3f) > primary sigma (%.3f) and rho non-regressing (%.3f vs %.3f)",
			shadow.Sigma, primary.Sigma, shadow.Rho, primary.Rho)
	} else if !shadowBeatsSigma {
		comp.PromotionReason = fmt.Sprintf("Shadow sigma (%.3f) does not beat primary sigma (%.3f)",
			shadow.Sigma, primary.Sigma)
	} else {
		comp.PromotionReason = fmt.Sprintf("Shadow rho regressed (%.3f vs primary %.3f)",
			shadow.Rho, primary.Rho)
	}

	return comp
}

func printNamespaceComparison(w io.Writer, comp *namespaceComparison) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Namespace Comparison: primary vs "+comp.ShadowName)
	fmt.Fprintln(w, "  ═══════════════════════════════════════")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %-20s  %-12s  %-12s  %-10s\n", "Metric", "Primary", comp.ShadowName, "Delta")
	fmt.Fprintln(w, "  ────────────────────  ────────────  ────────────  ──────────")
	fmt.Fprintf(w, "  %-20s  %-12.3f  %-12.3f  %+.3f\n", "sigma (retrieval)", comp.Primary.Sigma, comp.Shadow.Sigma, comp.SigmaDelta)
	fmt.Fprintf(w, "  %-20s  %-12.3f  %-12.3f  %+.3f\n", "rho (influence)", comp.Primary.Rho, comp.Shadow.Rho, comp.RhoDelta)
	fmt.Fprintf(w, "  %-20s  %-12.3f  %-12.3f  %+.3f\n", "sigma*rho", comp.Primary.SigmaRho, comp.Shadow.SigmaRho, comp.Shadow.SigmaRho-comp.Primary.SigmaRho)
	fmt.Fprintf(w, "  %-20s  %-12.3f  %-12.3f  %+.3f\n", "velocity", comp.Primary.Velocity, comp.Shadow.Velocity, comp.VelocityDelta)
	fmt.Fprintf(w, "  %-20s  %-12.1f  %-12.1f  %+.1f\n", "delta (avg age)", comp.Primary.Delta, comp.Shadow.Delta, comp.Shadow.Delta-comp.Primary.Delta)
	fmt.Fprintln(w)

	if comp.PromotionReady {
		fmt.Fprintln(w, "  PROMOTION: READY")
	} else {
		fmt.Fprintln(w, "  PROMOTION: NOT READY")
	}
	fmt.Fprintf(w, "  Reason: %s\n", comp.PromotionReason)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Rollback: %s\n", comp.RollbackContract)
	fmt.Fprintln(w)
}

// runFlywheelStatus displays comprehensive flywheel health.
func runFlywheelStatus(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	metrics, err := computeMetricsForNamespace(cwd, metricsDays, flywheelStatusNamespace)
	if err != nil {
		return fmt.Errorf("compute metrics: %w", err)
	}
	metricNamespace := canonicalMetricNamespace(flywheelStatusNamespace)
	if scorecard, err := loadStigmergicScorecard(cwd); err == nil {
		metrics.StigmergicScorecard = &types.StigmergicScorecard{
			PromotedFindings:       scorecard.PromotedFindings,
			PlanningRules:          scorecard.PlanningRules,
			PreMortemChecks:        scorecard.PreMortemChecks,
			QueueEntries:           scorecard.QueueEntries,
			UnconsumedBatches:      scorecard.UnconsumedBatches,
			UnconsumedItems:        scorecard.UnconsumedItems,
			HighSeverityUnconsumed: scorecard.HighSeverityUnconsumed,
		}
	}

	// Always compute golden signals — they provide the honest health assessment.
	populateGoldenSignals(cwd, metricsDays, metrics)

	w := cmd.OutOrStdout()
	switch GetOutput() {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"metric_namespace":            metricNamespace,
			"status":                      metrics.HealthStatus(),
			"delta":                       metrics.Delta,
			"sigma":                       metrics.Sigma,
			"rho":                         metrics.Rho,
			"sigma_rho":                   metrics.SigmaRho,
			"velocity":                    metrics.Velocity,
			"compounding":                 metrics.HealthCompounding(),
			"escape_velocity_status":      metrics.EscapeVelocityStatus(),
			"escape_velocity_compounding": metrics.AboveEscapeVelocity,
			"scorecard":                   metrics.StigmergicScorecard,
			"golden_signals":              metrics.GoldenSignals,
			"metrics":                     metrics,
		})

	case "yaml":
		enc := yaml.NewEncoder(w)
		if err := enc.Encode(map[string]any{
			"metric_namespace":            metricNamespace,
			"status":                      metrics.HealthStatus(),
			"delta":                       metrics.Delta,
			"sigma":                       metrics.Sigma,
			"rho":                         metrics.Rho,
			"sigma_rho":                   metrics.SigmaRho,
			"velocity":                    metrics.Velocity,
			"compounding":                 metrics.HealthCompounding(),
			"escape_velocity_status":      metrics.EscapeVelocityStatus(),
			"escape_velocity_compounding": metrics.AboveEscapeVelocity,
			"scorecard":                   metrics.StigmergicScorecard,
			"golden_signals":              metrics.GoldenSignals,
		}); err != nil {
			_ = enc.Close()
			return err
		}
		return enc.Close()

	default:
		printFlywheelStatus(w, metrics)
		fprintGoldenSignals(w, metrics.GoldenSignals)
	}

	return nil
}

// printFlywheelStatus prints a focused flywheel status display.
func printFlywheelStatus(w io.Writer, m *types.FlywheelMetrics) {
	status := m.HealthStatus()
	escapeStatus := m.EscapeVelocityStatus()

	// Status indicator (ASCII for accessibility)
	var statusIcon string
	switch status {
	case "COMPOUNDING":
		statusIcon = "[COMPOUNDING]"
	case "ACCUMULATING":
		statusIcon = "[ACCUMULATING]"
	case "NEAR ESCAPE":
		statusIcon = "[NEAR_ESCAPE]"
	default:
		statusIcon = "[DECAYING]"
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Flywheel Health: %s\n", statusIcon)
	if escapeStatus != status {
		fmt.Fprintf(w, "  Escape Velocity: [%s]\n", escapeStatus)
	}
	fmt.Fprintln(w, "  ═══════════════════════════════")
	fmt.Fprintln(w)

	// Core equation
	fmt.Fprintln(w, "  EQUATION: dK/dt = I(t) - δ·K + σ·ρ·K")
	fmt.Fprintln(w, "  Operational check: σ × ρ > δ/100")
	fmt.Fprintln(w)

	// Parameters
	fmt.Fprintf(w, "  δ (avg age):    %.1f days\n", m.Delta)
	fmt.Fprintf(w, "  σ (retrieval):  %.2f (%d%% of retrievable artifacts surfaced)\n", m.Sigma, int(m.Sigma*100))
	fmt.Fprintf(w, "  ρ (influence):  %.2f (%d%% of surfaced artifacts evidenced)\n", m.Rho, int(m.Rho*100))
	fmt.Fprintln(w)

	// Critical comparison
	threshold := escapeVelocityThreshold(m.Delta)
	fmt.Fprintln(w, "  ESCAPE VELOCITY CHECK:")
	fmt.Fprintf(w, "    σ × ρ = %.3f\n", m.SigmaRho)
	fmt.Fprintf(w, "    δ/100 = %.3f\n", threshold)
	fmt.Fprintln(w, "    ───────────────")

	switch {
	case m.AboveEscapeVelocity:
		fmt.Fprintf(w, "    σρ > δ/100 ✓ (velocity: +%.3f)\n", m.Velocity)
		fmt.Fprintln(w, "    → Escape velocity is above threshold")
	case m.Velocity > -0.05:
		fmt.Fprintf(w, "    σρ ≈ δ/100 (velocity: %.3f)\n", m.Velocity)
		fmt.Fprintln(w, "    → Escape velocity is near threshold")
	default:
		fmt.Fprintf(w, "    σρ < δ/100 ✗ (velocity: %.3f)\n", m.Velocity)
		fmt.Fprintln(w, "    → Escape velocity is below threshold")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  RECOMMENDATIONS:")
		if m.Sigma < 0.3 {
			fmt.Fprintln(w, "    • Improve retrieval: use 'ao lookup' for on-demand knowledge")
		}
		if m.Rho < 0.3 {
			fmt.Fprintln(w, "    • Cite more learnings: reference artifacts in your work")
		}
		if m.StaleArtifacts > 5 {
			fmt.Fprintf(w, "    • Review %d stale artifacts (90+ days uncited)\n", m.StaleArtifacts)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Period: %s to %s (%d days)\n",
		m.PeriodStart.Format("2006-01-02"),
		m.PeriodEnd.Format("2006-01-02"),
		metricsDays)
	if m.StigmergicScorecard != nil {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  STIGMERGIC SCORECARD:")
		fmt.Fprintf(w, "    Signals: %d findings, %d planning rules, %d pre-mortem checks\n",
			m.StigmergicScorecard.PromotedFindings,
			m.StigmergicScorecard.PlanningRules,
			m.StigmergicScorecard.PreMortemChecks)
		fmt.Fprintf(w, "    Backlog: %d items, %d high severity, %d batches\n",
			m.StigmergicScorecard.UnconsumedItems,
			m.StigmergicScorecard.HighSeverityUnconsumed,
			m.StigmergicScorecard.UnconsumedBatches)
	}
	fmt.Fprintln(w)
	if m.GoldenSignals != nil && escapeStatus != status {
		fmt.Fprintf(w, "  Note: escape velocity is a necessary condition; overall health is %s.\n", status)
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, "  Tip: 'ao status' shows flywheel health alongside session info.")
}
