// practices: [output-contract-parity, safe-degradation]
package orchestration

import (
	"context"
	"fmt"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// BeadsFloorAdapter is the always-available beads "floor" of the
// safe-degradation ladder (NTM -> Claude-native -> beads). It is the tier
// every degradation path terminates in, so it MUST never fail to place
// work for lack of a richer engine.
//
// Its sole contractual job in this foundation is OUTPUT-CONTRACT PARITY:
// it emits the same OrchestrationResult shape every other tier emits, so a
// caller that has degraded all the way to the floor reads the outcome
// exactly as it would read an NTM or Claude-native result. Parity is what
// makes the degradation correctness-preserving rather than lossy.
//
// This is a thin stub representing the floor, not a bd execution engine.
// The real bd-driven sequential loop (claim a ready bead, run it, record a
// verdict, advance) is layered on later and will populate ResultPaths and
// Verdict from actual bd state. What is established here is the contract:
// whatever the real loop does, it returns a parity-conformant
// OrchestrationResult with Backend == ports.BackendBeads.
//
// The zero value is ready to use.
type BeadsFloorAdapter struct{}

// beadsFloorPlaceholderPath is the repo-relative artifact path the floor
// stub reports until the real bd-driven loop replaces it with the actual
// paths a run wrote. It exists so the stub satisfies the non-empty
// ResultPaths requirement of the parity contract.
const beadsFloorPlaceholderPath = ".agents/orchestration/beads-floor.placeholder"

// Run executes a single unit of work on the beads floor for the given
// taskID and returns a parity-conformant OrchestrationResult tagged with
// Backend == ports.BackendBeads.
//
// In this foundation it is a stub: it does not yet drive bd. It returns a
// result that always passes its own Validate check (SchemaVersion ==
// SchemaVersionV1, a non-empty ResultPaths, and a valid WARN/MEDIUM
// verdict signalling "floor stub, not a real bd run"). The WARN/MEDIUM
// pairing is deliberate — it advertises that the floor produced a
// well-formed result without claiming the high-confidence PASS a real bd
// run would earn. The real bd-driven loop will replace the body while
// keeping this signature and the Backend tag.
//
// It honors ctx cancellation on a best-effort basis: if the context is
// already done it returns that error without fabricating a result, so a
// cancelled caller never receives a misleading floor verdict.
func (BeadsFloorAdapter) Run(ctx context.Context, taskID string) (OrchestrationResult, error) {
	if err := ctx.Err(); err != nil {
		return OrchestrationResult{}, fmt.Errorf("beads floor: context done before run: %w", err)
	}

	result := OrchestrationResult{
		SchemaVersion: SchemaVersionV1,
		Backend:       ports.BackendBeads,
		ResultPaths:   []string{beadsFloorPlaceholderPath},
		Verdict: Verdict{
			Status:     VerdictStatusWarn,
			Confidence: VerdictConfidenceMedium,
		},
		TaskID: taskID,
	}

	// Self-check parity before returning: the floor is the contract's
	// reference adapter, so it must never emit a non-conformant result.
	if err := result.Validate(); err != nil {
		return OrchestrationResult{}, fmt.Errorf("beads floor: emitted non-conformant result: %w", err)
	}
	return result, nil
}
