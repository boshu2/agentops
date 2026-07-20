// Package flywheelapp owns the filesystem and clock effects behind the
// `ao flywheel` command family: it reads the durable citation ledger (via
// internal/evidence) and knowledge stores, computes flywheel metrics, and
// renders the status and namespace-comparison reports. The flywheel command
// module is a thin Cobra presentation seam over this application logic and
// performs no direct effect itself.
package flywheelapp

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/boshu2/agentops/cli/internal/types"
)

// namespaceComparison holds side-by-side metrics for two namespaces.
type namespaceComparison struct {
	Primary       *types.FlywheelMetrics `json:"primary"`
	Shadow        *types.FlywheelMetrics `json:"shadow"`
	ShadowName    string                 `json:"shadow_name"`
	SigmaDelta    float64                `json:"sigma_delta"`
	RhoDelta      float64                `json:"rho_delta"`
	VelocityDelta float64                `json:"velocity_delta"`
}

// Compare computes and renders the primary-vs-shadow namespace comparison. It
// reports measurements only; it performs no promotion, routing, activation, or
// rollback.
func Compare(w io.Writer, outputMode string, days int, shadowNamespace string, verbosef func(format string, args ...any)) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	primaryMetrics, err := computeMetricsForNamespace(cwd, days, primaryMetricNamespace, verbosef)
	if err != nil {
		return fmt.Errorf("compute primary metrics: %w", err)
	}

	shadowMetrics, err := computeMetricsForNamespace(cwd, days, shadowNamespace, verbosef)
	if err != nil {
		return fmt.Errorf("compute shadow metrics: %w", err)
	}

	comp := buildNamespaceComparison(primaryMetrics, shadowMetrics, shadowNamespace)

	switch outputMode {
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
		Primary:       primary,
		Shadow:        shadow,
		ShadowName:    canonicalMetricNamespace(shadowName),
		SigmaDelta:    shadow.Sigma - primary.Sigma,
		RhoDelta:      shadow.Rho - primary.Rho,
		VelocityDelta: shadow.Velocity - primary.Velocity,
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
}

// Status computes and renders comprehensive flywheel health.
func Status(w io.Writer, outputMode string, days int, statusNamespace string, verbosef func(format string, args ...any)) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	metrics, err := computeMetricsForNamespace(cwd, days, statusNamespace, verbosef)
	if err != nil {
		return fmt.Errorf("compute metrics: %w", err)
	}
	metricNamespace := canonicalMetricNamespace(statusNamespace)
	// Always compute golden signals — they provide the honest health assessment.
	populateGoldenSignals(cwd, days, metrics)

	switch outputMode {
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
		printFlywheelStatus(w, metrics, days)
		fprintGoldenSignals(w, metrics.GoldenSignals)
	}

	return nil
}

// printFlywheelStatus prints a focused flywheel status display.
func printFlywheelStatus(w io.Writer, m *types.FlywheelMetrics, days int) {
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
		days)
	if m.StigmergicScorecard != nil {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  STIGMERGIC SCORECARD:")
		fmt.Fprintf(w, "    Signals: %d findings, %d planning rules, Premortem checks: %d\n",
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
