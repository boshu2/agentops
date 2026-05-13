// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import "context"

// CycleEntry is one entry in the evolve loop's cycle ledger
// (.agents/evolve/cycle-history.jsonl). Number is the monotonic cycle
// counter; Mode names the run mode (e.g. "feature-scaffold-BC1",
// "healing"); Result is the outcome (improved/regressed/unchanged/
// harvested/quarantined); Commit is the short SHA of the cycle's
// commit when one was made (empty for local-only cycles); Milestone
// is the operator-facing summary string. StartedAt is the cycle
// boundary timestamp (RFC3339, empty when absent on disk). Title
// is the short human-facing label that operators stamp onto each
// cycle entry (e.g. "Sweep dead code"); it is the most commonly
// consulted free-text field for cycle-history audits.
//
// Fields beyond Number/Mode/Result/Commit/Milestone are widened
// on-demand for known consumers (per the cycle-157 post-mortem at
// docs/learnings/2026-05-13-bc-ports-narrowness-postmortem.md).
type CycleEntry struct {
	Number    int
	Mode      string
	Result    string
	Commit    string
	Milestone string
	StartedAt string
	Title     string
}

// LoopReaderPort is the BC3 Loop read-side. Callers — evolve's
// cycle-recovery bootstrap in Step 0, the /post-mortem aggregator,
// the dream-loop compounding analyzer, and any future cycle-history
// auditor — depend on this port so they can read the evolve loop's
// state without depending directly on the local-only
// cycle-history.jsonl format.
//
// Contract:
//
//   - Latest returns the highest-Number CycleEntry. When the ledger
//     is empty, returns a zero-value CycleEntry + nil error.
//   - Range returns entries [start, end] (inclusive). Empty range or
//     out-of-bounds returns an empty slice + nil error.
//   - IdleStreak returns the trailing count of entries whose Result
//     is "idle" or "unchanged" (the dormancy-quasi-stop signal).
//   - Context cancellation MUST be honored on a best-effort basis.
//
// See docs/contracts/ubiquitous-language.md (BC3 row) for the
// canonical Loop context surface. soc-y5vh epic tracks BC3 port
// extraction; this is the first port in that epic.
type LoopReaderPort interface {
	Latest(ctx context.Context) (CycleEntry, error)
	Range(ctx context.Context, start, end int) ([]CycleEntry, error)
	IdleStreak(ctx context.Context) (int, error)
}
