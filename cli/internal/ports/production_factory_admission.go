// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import (
	"context"
	"fmt"

	"github.com/boshu2/agentops/cli/internal/daemon"
)

// ProductionFactoryAdmission satisfies FactoryAdmissionPort by
// delegating to a daemon.FactoryAdmissionEvidenceProvider (typically
// daemon.LocalFactoryAdmissionEvidenceProvider). Pairs with cycle
// 139's port scaffold — 14th of 14 ports now has both an in-memory
// test double and a production adapter.
//
// Translation boundary:
//   - daemon.FactoryRepoState → FactoryRepoEvidence (1:1 field mapping)
//   - daemon.FactoryPRBlockerMatrix → FactoryPREvidence (Blockers
//     reduced from []FactoryOpenPRBlocker structs to []string of
//     "PR#<num> <head>")
//   - daemon.FactoryCIBaselineEvidence → FactoryCIEvidence
//     (Known passes through; Green = Status == "green")
type ProductionFactoryAdmission struct {
	provider daemon.FactoryAdmissionEvidenceProvider
}

// NewProductionFactoryAdmission wraps a daemon evidence provider.
func NewProductionFactoryAdmission(provider daemon.FactoryAdmissionEvidenceProvider) *ProductionFactoryAdmission {
	return &ProductionFactoryAdmission{provider: provider}
}

// ProbeRepoState translates daemon.FactoryRepoState to the port's
// FactoryRepoEvidence (the 3 fields map 1:1).
func (a *ProductionFactoryAdmission) ProbeRepoState(ctx context.Context) (FactoryRepoEvidence, error) {
	if err := ctx.Err(); err != nil {
		return FactoryRepoEvidence{}, err
	}
	if a.provider == nil {
		return FactoryRepoEvidence{}, fmt.Errorf("ProductionFactoryAdmission: provider required")
	}
	state, err := a.provider.RepoState(ctx)
	if err != nil {
		return FactoryRepoEvidence{}, fmt.Errorf("factory_admission: repo_state: %w", err)
	}
	return FactoryRepoEvidence{
		HeadSHA:       state.HeadSHA,
		Dirty:         state.Dirty,
		TrackedAgents: state.TrackedAgents,
	}, nil
}

// ProbeOpenPRBlockers translates daemon.FactoryPRBlockerMatrix to the
// port's FactoryPREvidence. The daemon's []FactoryOpenPRBlocker struct
// slice is reduced to []string ("PR#<num> <head>") at the boundary so
// the port stays narrow.
func (a *ProductionFactoryAdmission) ProbeOpenPRBlockers(ctx context.Context, touched []string) (FactoryPREvidence, error) {
	if err := ctx.Err(); err != nil {
		return FactoryPREvidence{}, err
	}
	if a.provider == nil {
		return FactoryPREvidence{}, fmt.Errorf("ProductionFactoryAdmission: provider required")
	}
	matrix, err := a.provider.OpenPRBlockers(ctx, touched)
	if err != nil {
		return FactoryPREvidence{}, fmt.Errorf("factory_admission: open_pr_blockers: %w", err)
	}
	blockers := make([]string, 0, len(matrix.Blockers))
	for _, b := range matrix.Blockers {
		blockers = append(blockers, fmt.Sprintf("PR#%d %s", b.PRNumber, b.HeadRef))
	}
	return FactoryPREvidence{
		Known:    matrix.Known,
		Blockers: blockers,
	}, nil
}

// ProbeMainCIBaseline translates daemon.FactoryCIBaselineEvidence to
// the port's FactoryCIEvidence. Known passes through; Green is true
// iff Status == FactoryCIStatusGreen.
func (a *ProductionFactoryAdmission) ProbeMainCIBaseline(ctx context.Context) (FactoryCIEvidence, error) {
	if err := ctx.Err(); err != nil {
		return FactoryCIEvidence{}, err
	}
	if a.provider == nil {
		return FactoryCIEvidence{}, fmt.Errorf("ProductionFactoryAdmission: provider required")
	}
	baseline, err := a.provider.MainCIBaseline(ctx)
	if err != nil {
		return FactoryCIEvidence{}, fmt.Errorf("factory_admission: main_ci_baseline: %w", err)
	}
	return FactoryCIEvidence{
		Known: baseline.Known,
		Green: baseline.Baseline.Status == daemon.FactoryCIStatusGreen,
	}, nil
}

// Compile-time assertion: ProductionFactoryAdmission satisfies the port.
var _ FactoryAdmissionPort = (*ProductionFactoryAdmission)(nil)
