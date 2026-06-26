// practices: [hexagonal-architecture, safe-degradation]
package orchestration

import (
	"context"
	"fmt"
	"os"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// selectEnvVar is the environment variable that pins or opts out of a
// backend, mirroring the AGENTOPS_HOOKS_DISABLED style of explicit
// operator override. It is the typed-port analogue of the shell seam's
// AGENTOPS_ORCH variable in lib/orchestrate-select.sh.
const selectEnvVar = "AGENTOPS_ORCHESTRATION"

// Selector is the production OrchestrationPort implementation. It
// resolves a backend via the safe-degradation ladder
// NTM -> Claude-native -> beads floor, honoring an explicit Pin, the
// AGENTOPS_ORCHESTRATION env override, and an OptOut to the floor.
//
// It holds a CommandRunner so NTM availability is detected by
// capability (via ProbeNTM) rather than by `command -v`, and reads the
// env override at Select time so operators can flip routing without
// reconstructing the Selector.
type Selector struct {
	// runner drives ProbeNTM. It MUST be non-nil for the availability
	// step; the explicit-pin, env, and opt-out steps resolve without it.
	runner CommandRunner
}

// NewSelector builds a Selector backed by runner. runner is used only
// to probe NTM availability; the explicit-routing steps (Pin, env,
// OptOut) never touch it.
func NewSelector(runner CommandRunner) *Selector {
	return &Selector{runner: runner}
}

// compile-time assertion that Selector satisfies the port.
var _ ports.OrchestrationPort = (*Selector)(nil)

// Select resolves the backend for work. Resolution order, first match
// wins, mirroring lib/orchestrate-select.sh:
//
//  1. work.Pin set -> that backend.
//  2. AGENTOPS_ORCHESTRATION env: "off"/"beads" -> beads;
//     "ntm"/"claude"/"codex"/"omnigent" -> that backend (explicit pin/opt-out).
//  3. work.OptOut -> beads.
//  4. ProbeNTM reports Available -> ntm.
//  5. else -> claude.
//  6. floor -> beads (never reached as a no-op; claude is reachable, but
//     the floor is always selectable so work is never unplaceable).
//
// Context cancellation is honored on a best-effort basis: it is checked
// before the NTM probe and propagated into ProbeNTM.
func (s *Selector) Select(ctx context.Context, work ports.WorkSpec) (ports.SelectionTrace, error) {
	considered := []ports.Backend{}

	// Step 1: explicit Pin wins over everything.
	considered = append(considered, "pin")
	if work.Pin != "" {
		return ports.SelectionTrace{
			Chosen:     work.Pin,
			Reason:     fmt.Sprintf("explicit WorkSpec.Pin=%s", work.Pin),
			Considered: considered,
		}, nil
	}

	// Step 2: env override acts as an explicit pin / opt-out.
	considered = append(considered, "env")
	if env := os.Getenv(selectEnvVar); env != "" {
		switch env {
		case "off", "beads":
			return ports.SelectionTrace{
				Chosen:     ports.BackendBeads,
				Reason:     fmt.Sprintf("%s=%s -> beads floor (env opt-out)", selectEnvVar, env),
				Considered: considered,
			}, nil
		case "ntm":
			return ports.SelectionTrace{
				Chosen:     ports.BackendNTM,
				Reason:     fmt.Sprintf("%s=ntm (env pin)", selectEnvVar),
				Considered: considered,
			}, nil
		case "claude":
			return ports.SelectionTrace{
				Chosen:     ports.BackendClaude,
				Reason:     fmt.Sprintf("%s=claude (env pin)", selectEnvVar),
				Considered: considered,
			}, nil
		case "codex":
			return ports.SelectionTrace{
				Chosen:     ports.BackendCodex,
				Reason:     fmt.Sprintf("%s=codex (env pin)", selectEnvVar),
				Considered: considered,
			}, nil
		case "omnigent":
			return ports.SelectionTrace{
				Chosen:     ports.BackendOmnigent,
				Reason:     fmt.Sprintf("%s=omnigent (env pin)", selectEnvVar),
				Considered: considered,
			}, nil
		default:
			// Unknown value falls through to the availability ladder,
			// matching the shell seam's "unknown -> auto" behavior.
			considered = append(considered, "env-unknown")
		}
	}

	// Step 3: explicit OptOut routes to the beads floor.
	considered = append(considered, "optout")
	if work.OptOut {
		return ports.SelectionTrace{
			Chosen:     ports.BackendBeads,
			Reason:     "WorkSpec.OptOut -> beads floor",
			Considered: considered,
		}, nil
	}

	// Step 4: NTM availability (capability probe).
	considered = append(considered, "ntm")
	if err := ctx.Err(); err != nil {
		return ports.SelectionTrace{}, fmt.Errorf("selecting backend: %w", err)
	}
	caps, err := ProbeNTM(ctx, s.runner)
	if err != nil {
		return ports.SelectionTrace{}, fmt.Errorf("probing NTM availability: %w", err)
	}
	if caps.Available {
		return ports.SelectionTrace{
			Chosen:     ports.BackendNTM,
			Reason:     "NTM probe reports available -> ntm (preferred swarm runtime)",
			Considered: considered,
		}, nil
	}

	// Step 5: Claude-native fallback (the "worse NTM"; always present
	// in an agent session/CI context).
	considered = append(considered, "claude")
	considered = append(considered, "beads")
	return ports.SelectionTrace{
		Chosen:     ports.BackendClaude,
		Reason:     "NTM absent -> claude-native fallback (beads floor remains available)",
		Considered: considered,
	}, nil
}
