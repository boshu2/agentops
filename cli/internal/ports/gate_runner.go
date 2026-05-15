// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import "context"

// GateName identifies a CI/validation gate by its canonical name (e.g.
// "registry-check", "validate-three-gap-supergate"). Values match the
// job names declared in .github/workflows/validate.yml.
type GateName string

// GateVerdict is the outcome of a single gate run. Status names the
// terminal state; Reason is a short human-readable explanation;
// LogTail is a short trailing slice of the gate's captured output
// (capped at 4096 bytes by adapters that honor the contract). Adapters
// MAY return an empty LogTail when the gate's output is not available.
type GateVerdict struct {
	Status  GateStatus
	Reason  string
	LogTail string
}

// GateStatus enumerates the gate-run outcomes. PASS = exit 0 with no
// blocking issues. WARN = non-blocking advisory; the gate ran but
// surfaced concerns. FAIL = blocking. SKIP = gate decided not to run
// (e.g. greenfield SKIP, structural-availability gate, etc.). UNKNOWN
// = the adapter couldn't decide; callers MAY treat as FAIL conservative
// or SKIP optimistic per their own policy.
type GateStatus string

const (
	GateStatusPass    GateStatus = "PASS"
	GateStatusWarn    GateStatus = "WARN"
	GateStatusFail    GateStatus = "FAIL"
	GateStatusSkip    GateStatus = "SKIP"
	GateStatusUnknown GateStatus = "UNKNOWN"
)

// GateRunRequest configures a single gate invocation. Name is required;
// Env is an optional map of environment variables the adapter MAY pass
// through to the underlying gate. Adapters that don't model env may
// ignore it.
type GateRunRequest struct {
	Name GateName
	Env  map[string]string
}

// GateRunnerPort is the BC2 Validation read+execute side. Callers —
// evolve's Step 5 regression gate, the /rpi validation phase, the
// supergate composer in scripts/check-three-gap-supergate.sh's Go
// twin, and any future per-PR gate-runner — depend on this port so
// the gate-running surface can be exercised against an in-memory
// adapter without standing up the real exec/subprocess machinery.
//
// Contract:
//
//   - Run MUST return a non-nil GateVerdict on success even when
//     Status is UNKNOWN.
//   - Reason MUST be non-empty.
//   - When req.Name is empty, Run MUST return UNKNOWN with reason
//     "empty GateName".
//   - The adapter decides whether unknown gate names map to
//     UNKNOWN (optimistic, may be a typo) or FAIL (conservative,
//     no such gate exists). Adapters MUST document the choice.
//   - Context cancellation MUST be honored on a best-effort basis.
//
// See docs/contracts/ubiquitous-language.md (BC2 row) for the
// canonical Validation context surface. soc-wxh5 epic tracks BC2
// port extraction; this is the first port in that epic.
type GateRunnerPort interface {
	Run(ctx context.Context, req GateRunRequest) (GateVerdict, error)
}
