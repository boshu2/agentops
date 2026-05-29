// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import "context"

// Backend names an orchestration engine that can execute a unit of
// agent work. The values form a safe-degradation ladder: NTM is the
// preferred swarm runtime, Claude-native is the fallback when NTM is
// unavailable, and beads is the always-available floor. Codex is a
// pinnable engine that is never auto-selected by the default ladder.
type Backend string

const (
	// BackendNTM is the NTM tmux swarm runtime — the preferred engine
	// when available.
	BackendNTM Backend = "ntm"
	// BackendClaude is the Claude-native runtime — the fallback when
	// NTM is unavailable.
	BackendClaude Backend = "claude"
	// BackendCodex is the Codex runtime — selectable only via an
	// explicit Pin; the default ladder never auto-selects it.
	BackendCodex Backend = "codex"
	// BackendBeads is the beads floor — the always-available engine
	// that every degradation path terminates in.
	BackendBeads Backend = "beads"
)

// WorkSpec describes a single unit of orchestrable work that the port
// must place on a backend. It is intentionally minimal: the port only
// needs the caller's routing intent, not the work's payload.
type WorkSpec struct {
	// OptOut requests that the work bypass swarm engines and run on the
	// beads floor regardless of NTM/Claude availability.
	OptOut bool
	// Pin forces a specific backend, overriding both OptOut and the
	// availability ladder. The empty Backend ("") means auto-select.
	Pin Backend
}

// SelectionTrace is the resolved routing decision plus the reasoning
// that produced it. Considered lists the backends the ladder evaluated,
// in order, for auditability; it is safe for callers to mutate.
type SelectionTrace struct {
	Chosen     Backend
	Reason     string
	Considered []Backend
}

// OrchestrationPort selects an orchestration backend for a unit of
// work. It models the safe-degradation ladder NTM -> Claude-native ->
// beads floor, with an explicit Pin escape hatch and an OptOut path to
// the floor.
//
// Contract:
//
//   - A non-empty WorkSpec.Pin MUST win over everything else,
//     including OptOut and availability.
//   - WorkSpec.OptOut (with no Pin) MUST resolve to BackendBeads.
//   - Otherwise Select MUST choose BackendNTM when NTM is available,
//     else BackendClaude when Claude is available, else BackendBeads.
//   - BackendBeads is the floor: it MUST always be selectable so the
//     port never fails to place work for lack of an engine.
//   - SelectionTrace.Considered MUST record the ladder steps evaluated.
//   - Context cancellation MUST be honored on a best-effort basis.
//
// This port is the typed replacement for the ad hoc backend-selection
// shell logic in the spike's orchestrate-select.sh.
type OrchestrationPort interface {
	Select(ctx context.Context, work WorkSpec) (SelectionTrace, error)
}
