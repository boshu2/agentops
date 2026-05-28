// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import (
	"context"
	"fmt"
)

// InMemoryGateRunner is a GateRunnerPort backed by a fixed map of
// gate name → verdict that the adapter returns on Run. Intended for
// tests and CLI dry-runs of gate-composition logic without invoking
// real subprocesses.
//
// Unknown-name policy: by default, unknown gate names return UNKNOWN
// (not FAIL) — adapters MAY change this by setting UnknownIsFail at
// construction time. The default matches the optimistic
// "treat-typo-as-unknown" semantics most callers want during dev.
type InMemoryGateRunner struct {
	verdicts      map[GateName]GateVerdict
	UnknownIsFail bool
}

// NewInMemoryGateRunner returns an adapter that returns the supplied
// verdict for each gate name. Callers can mutate the returned struct's
// UnknownIsFail field if they want the conservative semantics.
func NewInMemoryGateRunner(verdicts map[GateName]GateVerdict) *InMemoryGateRunner {
	if verdicts == nil {
		verdicts = map[GateName]GateVerdict{}
	}
	return &InMemoryGateRunner{verdicts: verdicts}
}

// Run returns the configured verdict for req.Name. See package-level
// contract for empty-name and unknown-name semantics.
func (r *InMemoryGateRunner) Run(ctx context.Context, req GateRunRequest) (GateVerdict, error) {
	if err := ctx.Err(); err != nil {
		return GateVerdict{}, err
	}
	if req.Name == "" {
		return GateVerdict{Status: GateStatusUnknown, Reason: "empty GateName"}, nil
	}
	if v, ok := r.verdicts[req.Name]; ok {
		return v, nil
	}
	if r.UnknownIsFail {
		return GateVerdict{
			Status: GateStatusFail,
			Reason: fmt.Sprintf("unknown gate %q (UnknownIsFail=true)", req.Name),
		}, nil
	}
	return GateVerdict{
		Status: GateStatusUnknown,
		Reason: fmt.Sprintf("unknown gate %q (no configured verdict)", req.Name),
	}, nil
}

// Compile-time assertion: InMemoryGateRunner satisfies the port.
var _ GateRunnerPort = (*InMemoryGateRunner)(nil)
