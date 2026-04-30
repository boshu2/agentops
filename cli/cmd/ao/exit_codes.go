// Package main: Olympus exit-code protocol constants.
//
// These constants define the quest-supervisor exit-code contract ported from
// the Olympus zeus daemon (source: olympus CLAUDE.md and cmd/ol Exit()
// literals). They are documented in
// .agents/findings/2026-04-29-existing-agentopsd-contract.md, which classifies
// each value as MERGE-compatible with the pre-Olympus agentopsd contract
// (zero conflicts: existing agentopsd emits only 0 on clean shutdown and 1 on
// error via cobra defaults; codes 2 and 42 are unused at the process level).
//
// Bead: agentops-91i (parent epic agentops-tqc, Olympus → agentopsd extraction).
//
// IMPORTANT: these constants are markers for callers and tests. Adding a new
// caller that actually emits 42 from agentopsd's own success path would be a
// BREAKING change vs the existing contract — see the finding doc's
// "Compatibility notes" section before wiring them into the daemon binary.
package main

// Olympus quest-supervisor exit codes.
const (
	// CodeMoreWork (0) — Olympus zeus protocol. The current step finished
	// successfully and the supervisor should continue with the next quest
	// item. Per .agents/findings/2026-04-29-existing-agentopsd-contract.md,
	// this is COMPATIBLE with existing agentopsd shutdown semantics (which
	// also uses 0 for clean shutdown; the Olympus reading is a strict
	// superset that callers opt into).
	CodeMoreWork = 0

	// CodeError (1) — Olympus zeus protocol. The runner is stuck or hit an
	// error the supervisor should treat as a halt-and-investigate. Per the
	// finding doc, this is COMPATIBLE with cobra's default error-exit
	// behavior (any RunE-returned error becomes exit 1).
	CodeError = 1

	// CodeBeadClaimed (2) — Olympus zeus protocol. A bead was claimed but
	// needs a separate worker process to make progress. Per the finding
	// doc, this is non-overlapping with existing agentopsd behavior (no
	// current code path emits 2 at the process level; the only `return 2`
	// in cli/internal/daemon/ is dreamStageOrder, which is an int-ordering
	// helper, not a process exit).
	CodeBeadClaimed = 2

	// CodeQuestComplete (42) — Olympus zeus protocol. The full quest is
	// finished; the supervisor should not loop again. Per the finding doc,
	// this is non-overlapping with existing agentopsd shutdown semantics
	// (which uses 0 for clean shutdown; 42 is currently unused).
	CodeQuestComplete = 42
)

// IsTerminal reports whether the supervisor should stop looping after
// observing this exit code. CodeQuestComplete and CodeError are terminal;
// CodeMoreWork and CodeBeadClaimed mean keep going (with or without a worker
// hop).
func IsTerminal(code int) bool {
	switch code {
	case CodeQuestComplete, CodeError:
		return true
	default:
		return false
	}
}
