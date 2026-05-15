// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import "context"

// InMemoryFactoryAdmission is the test-double FactoryAdmissionPort.
// Construct with NewInMemoryFactoryAdmission and seed the Probe*
// return values directly via the public fields, or use the setter
// methods for chained construction.
//
// Concurrency: not safe for concurrent mutation; tests should set
// fields before the SUT calls into the adapter.
type InMemoryFactoryAdmission struct {
	RepoStateValue     FactoryRepoEvidence
	RepoStateErr       error
	OpenPRBlockersFunc func(touched []string) (FactoryPREvidence, error)
	MainCIBaselineVal  FactoryCIEvidence
	MainCIBaselineErr  error
}

// NewInMemoryFactoryAdmission returns an adapter that returns
// zero-value Probe results (Known=false everywhere, no error). Tests
// set the fields they care about before invoking the SUT.
func NewInMemoryFactoryAdmission() *InMemoryFactoryAdmission {
	return &InMemoryFactoryAdmission{}
}

// ProbeRepoState returns the seeded RepoStateValue + RepoStateErr.
func (a *InMemoryFactoryAdmission) ProbeRepoState(ctx context.Context) (FactoryRepoEvidence, error) {
	if err := ctx.Err(); err != nil {
		return FactoryRepoEvidence{}, err
	}
	return a.RepoStateValue, a.RepoStateErr
}

// ProbeOpenPRBlockers invokes OpenPRBlockersFunc if set; otherwise
// returns the zero value (Known=false, no blockers).
func (a *InMemoryFactoryAdmission) ProbeOpenPRBlockers(ctx context.Context, touched []string) (FactoryPREvidence, error) {
	if err := ctx.Err(); err != nil {
		return FactoryPREvidence{}, err
	}
	if a.OpenPRBlockersFunc != nil {
		return a.OpenPRBlockersFunc(touched)
	}
	return FactoryPREvidence{}, nil
}

// ProbeMainCIBaseline returns the seeded MainCIBaselineVal + Err.
func (a *InMemoryFactoryAdmission) ProbeMainCIBaseline(ctx context.Context) (FactoryCIEvidence, error) {
	if err := ctx.Err(); err != nil {
		return FactoryCIEvidence{}, err
	}
	return a.MainCIBaselineVal, a.MainCIBaselineErr
}

// Compile-time assertion: InMemoryFactoryAdmission satisfies the port.
var _ FactoryAdmissionPort = (*InMemoryFactoryAdmission)(nil)
