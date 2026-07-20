// Package flywheel owns Cobra presentation for the `ao flywheel` command
// family (status and compare). The module builds its command tree with
// constructor-scoped flag state and host seams, delegating every filesystem and
// clock effect — reading the durable citation ledger (via internal/evidence)
// and knowledge stores, computing metrics, and rendering the reports — to
// internal/flywheelapp so the module itself performs no direct effect. It
// reports measurements only; it performs no promotion, routing, activation, or
// rollback.
package flywheel

import (
	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/flywheelapp"
)

// HostOptions carries the ambient CLI seams the flywheel commands read: the
// global -o/--output mode and the verbose diagnostic printer. Output selection
// and diagnostics flow through these seams so the module reads no package
// global.
type HostOptions struct {
	OutputMode func() string
	Verbosef   func(format string, args ...any)
}

// Module owns Cobra presentation for the flywheel command family.
type Module struct {
	host HostOptions
}

// NewModule constructs the flywheel command module from its host seams.
func NewModule(host HostOptions) Module {
	return Module{host: host}
}

// Contract declares the flywheel family's behavior: it accepts arbitrary
// positional args exactly as Cobra does today, emits a human table (JSON/YAML
// under the global -o flag), reads the durable citation and knowledge stores on
// the filesystem and stamps recency from the clock, and exits 0 on success or 1
// on a working-directory or metric-computation failure. The composition does
// not attach this contract to the command tree, preserving flywheel's
// synthesized (unattached) capabilities surface exactly.
func (Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{
		ID:       "ao.flywheel",
		Profiles: clicontract.ProfileDefault | clicontract.ProfileFlywheel | clicontract.ProfileLegacy | clicontract.ProfileCombined,
		Args:     clicontract.ArgsPolicy{Name: "arbitrary", Validate: cobra.ArbitraryArgs},
		Output:   clicontract.OutputText,
		Effects:  clicontract.EffectFilesystem | clicontract.EffectClock,
		ExitClasses: map[int]clicontract.ExitClass{
			0: clicontract.ExitSuccess,
			1: clicontract.ExitFailure,
		},
	}
}

// Command builds the `ao flywheel` command tree with constructor-scoped flag
// state. No package-level command or flag variable is used.
func (module Module) Command() *cobra.Command {
	flywheelCmd := &cobra.Command{
		Use:   "flywheel",
		Short: "Knowledge flywheel operations",
		Long: `Knowledge flywheel operations and status.

The flywheel equation:
  dK/dt = I(t) - δ·K + σ·ρ·K - B(K, K_crit)

Operational escape velocity: σρ > δ/100 → Knowledge compounds

Commands:
  status   Show comprehensive flywheel health
  compare  Compare read-only metric namespaces

Examples:
  ao flywheel status
  ao flywheel status --json`,
		GroupID: "experimental",
	}

	// flywheel status subcommand
	var (
		statusDays      int
		statusNamespace string
		statusGolden    bool
	)
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			return flywheelapp.Status(cmd.OutOrStdout(), module.host.OutputMode(), statusDays, statusNamespace, module.host.Verbosef)
		},
	}
	statusCmd.Flags().IntVar(&statusDays, "days", flywheelapp.MetricsDaysDefault, "Period in days for metrics calculation")
	statusCmd.Flags().StringVar(&statusNamespace, "namespace", flywheelapp.PrimaryMetricNamespace, "Citation namespace to evaluate (primary by default)")
	statusCmd.Flags().BoolVar(&statusGolden, "golden", false, "Show golden signals (always shown; flag kept for compatibility)")
	_ = statusCmd.Flags().MarkHidden("golden")
	flywheelCmd.AddCommand(statusCmd)

	// flywheel compare subcommand
	var compareNamespace string
	compareCmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare primary vs shadow namespace metrics",
		Long: `Compare retrieval quality between primary and shadow namespaces.

Shows sigma, rho, and escape velocity side-by-side.
This command reports measurements only. It does not recommend or perform
promotion, routing, activation, rollback, or any other state transition.

Examples:
  ao flywheel compare
  ao flywheel compare --shadow experimental
  ao flywheel compare --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return flywheelapp.Compare(cmd.OutOrStdout(), module.host.OutputMode(), flywheelapp.MetricsDaysDefault, compareNamespace, module.host.Verbosef)
		},
	}
	compareCmd.Flags().StringVar(&compareNamespace, "shadow", "shadow", "Shadow namespace to compare against primary")
	flywheelCmd.AddCommand(compareCmd)

	return flywheelCmd
}
