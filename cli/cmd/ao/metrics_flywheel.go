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

Examples:
  ao flywheel status
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
	statusCmd.Flags().BoolVar(&flywheelGolden, "golden", false, "Show golden signals (always shown; flag kept for compatibility)")
	_ = statusCmd.Flags().MarkHidden("golden")
	flywheelCmd.AddCommand(statusCmd)
}

// runFlywheelStatus displays comprehensive flywheel health.
func runFlywheelStatus(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	metrics, err := computeMetrics(cwd, metricsDays)
	if err != nil {
		return fmt.Errorf("compute metrics: %w", err)
	}
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
		defer enc.Close()
		return enc.Encode(map[string]any{
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
		})

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
