// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"context"
	"fmt"

	"github.com/boshu2/agentops/cli/internal/daemon"
	"github.com/boshu2/agentops/cli/internal/ports"
)

// productionFactoryAdmission satisfies ports.FactoryAdmissionPort by
// delegating to a daemon.FactoryAdmissionEvidenceProvider (typically
// daemon.LocalFactoryAdmissionEvidenceProvider). Pairs with cycle
// 139's port scaffold — 14th of 14 ports now has both an in-memory
// test double and a production adapter.
//
// Translation boundary:
//   - daemon.FactoryRepoState → ports.FactoryRepoEvidence (1:1 field mapping)
//   - daemon.FactoryPRBlockerMatrix → ports.FactoryPREvidence (Blockers
//     reduced from []FactoryOpenPRBlocker structs to []string of
//     "PR#<num> <head>")
//   - daemon.FactoryCIBaselineEvidence → ports.FactoryCIEvidence
//     (Known passes through; Green = Status == "green")
type productionFactoryAdmission struct {
	provider daemon.FactoryAdmissionEvidenceProvider
}

// newProductionFactoryAdmission wraps a daemon evidence provider.
func newProductionFactoryAdmission(provider daemon.FactoryAdmissionEvidenceProvider) *productionFactoryAdmission {
	return &productionFactoryAdmission{provider: provider}
}

// ProbeRepoState translates daemon.FactoryRepoState to the port's
// FactoryRepoEvidence (the 3 fields map 1:1).
func (a *productionFactoryAdmission) ProbeRepoState(ctx context.Context) (ports.FactoryRepoEvidence, error) {
	if err := ctx.Err(); err != nil {
		return ports.FactoryRepoEvidence{}, err
	}
	if a.provider == nil {
		return ports.FactoryRepoEvidence{}, fmt.Errorf("productionFactoryAdmission: provider required")
	}
	state, err := a.provider.RepoState(ctx)
	if err != nil {
		return ports.FactoryRepoEvidence{}, fmt.Errorf("factory_admission: repo_state: %w", err)
	}
	return ports.FactoryRepoEvidence{
		HeadSHA:       state.HeadSHA,
		Dirty:         state.Dirty,
		TrackedAgents: state.TrackedAgents,
	}, nil
}

// ProbeOpenPRBlockers translates daemon.FactoryPRBlockerMatrix to the
// port's FactoryPREvidence. The daemon's []FactoryOpenPRBlocker struct
// slice is reduced to []string ("PR#<num> <head>") at the boundary so
// the port stays narrow.
func (a *productionFactoryAdmission) ProbeOpenPRBlockers(ctx context.Context, touched []string) (ports.FactoryPREvidence, error) {
	if err := ctx.Err(); err != nil {
		return ports.FactoryPREvidence{}, err
	}
	if a.provider == nil {
		return ports.FactoryPREvidence{}, fmt.Errorf("productionFactoryAdmission: provider required")
	}
	matrix, err := a.provider.OpenPRBlockers(ctx, touched)
	if err != nil {
		return ports.FactoryPREvidence{}, fmt.Errorf("factory_admission: open_pr_blockers: %w", err)
	}
	blockers := make([]string, 0, len(matrix.Blockers))
	for _, b := range matrix.Blockers {
		blockers = append(blockers, fmt.Sprintf("PR#%d %s", b.PRNumber, b.HeadRef))
	}
	return ports.FactoryPREvidence{
		Known:    matrix.Known,
		Blockers: blockers,
	}, nil
}

// ProbeMainCIBaseline translates daemon.FactoryCIBaselineEvidence to
// the port's FactoryCIEvidence. Known passes through; Green is true
// iff Status == FactoryCIStatusGreen.
func (a *productionFactoryAdmission) ProbeMainCIBaseline(ctx context.Context) (ports.FactoryCIEvidence, error) {
	if err := ctx.Err(); err != nil {
		return ports.FactoryCIEvidence{}, err
	}
	if a.provider == nil {
		return ports.FactoryCIEvidence{}, fmt.Errorf("productionFactoryAdmission: provider required")
	}
	baseline, err := a.provider.MainCIBaseline(ctx)
	if err != nil {
		return ports.FactoryCIEvidence{}, fmt.Errorf("factory_admission: main_ci_baseline: %w", err)
	}
	return ports.FactoryCIEvidence{
		Known: baseline.Known,
		Green: baseline.Baseline.Status == daemon.FactoryCIStatusGreen,
	}, nil
}

// Compile-time assertion: productionFactoryAdmission satisfies the port.
var _ ports.FactoryAdmissionPort = (*productionFactoryAdmission)(nil)
