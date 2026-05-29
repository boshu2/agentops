// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import "context"

// InMemoryOrchestration is a deterministic OrchestrationPort
// implementation. It does not retain per-call state; "in-memory" means
// the availability of each swarm engine is injected as fields and the
// degradation ladder is evaluated purely from them.
type InMemoryOrchestration struct {
	// NTMAvailable reports whether the NTM swarm runtime can take work.
	NTMAvailable bool
	// ClaudeAvailable reports whether the Claude-native runtime can
	// take work.
	ClaudeAvailable bool
}

// Select resolves the orchestration backend for the given work,
// applying the ladder: explicit Pin wins; OptOut routes to the beads
// floor; otherwise NTM if available, else Claude if available, else the
// beads floor.
func (o *InMemoryOrchestration) Select(ctx context.Context, work WorkSpec) (SelectionTrace, error) {
	if err := ctx.Err(); err != nil {
		return SelectionTrace{}, err
	}

	if work.Pin != "" {
		return SelectionTrace{
			Chosen:     work.Pin,
			Reason:     "explicit-pin",
			Considered: []Backend{work.Pin},
		}, nil
	}

	if work.OptOut {
		return SelectionTrace{
			Chosen:     BackendBeads,
			Reason:     "opt-out-to-beads-floor",
			Considered: []Backend{BackendBeads},
		}, nil
	}

	considered := []Backend{BackendNTM}
	if o.NTMAvailable {
		return SelectionTrace{
			Chosen:     BackendNTM,
			Reason:     "ntm-available",
			Considered: considered,
		}, nil
	}

	considered = append(considered, BackendClaude)
	if o.ClaudeAvailable {
		return SelectionTrace{
			Chosen:     BackendClaude,
			Reason:     "ntm-unavailable-claude-available",
			Considered: considered,
		}, nil
	}

	considered = append(considered, BackendBeads)
	return SelectionTrace{
		Chosen:     BackendBeads,
		Reason:     "degraded-to-beads-floor",
		Considered: considered,
	}, nil
}

// Compile-time assertion: InMemoryOrchestration satisfies the port.
var _ OrchestrationPort = (*InMemoryOrchestration)(nil)
