// practices: [hexagonal-architecture]
package ports

import "context"

// PreflightPort runs admission checks before out-of-session spawn.
type PreflightPort interface {
	Preflight(ctx context.Context, profile string) error
}

// VerifyPort checks live session ground truth against a profile.
type VerifyPort interface {
	Verify(ctx context.Context, profile, session string) error
}

// SubstrateProbePort probes external orchestration binaries.
type SubstrateProbePort interface {
	ProbeTools(ctx context.Context) error
}
