// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import "context"

// LoopWriterPort is the BC3 Loop write-side. Callers — evolve's
// Step 6 cycle-logger, the `ao evolve` v2 supervisor, and any future
// loop-event ingester — depend on this port so they can record cycle
// outcomes without depending directly on the local-only
// cycle-history.jsonl append.
//
// Contract:
//
//   - Append MUST add the entry to the ledger. If entry.Number is 0,
//     adapters MUST auto-assign the next sequential number; otherwise
//     entry.Number is honored as-is.
//   - Append MUST NOT allow duplicate Numbers — re-appending an entry
//     with an existing Number returns a non-nil error.
//   - Append returns the entry as recorded (with Number filled in if
//     auto-assigned).
//   - Context cancellation MUST be honored on a best-effort basis.
//
// See docs/contracts/ubiquitous-language.md (BC3 row). Sibling:
// LoopReaderPort (cycle 102 read-side). soc-y5vh epic.
type LoopWriterPort interface {
	Append(ctx context.Context, entry CycleEntry) (CycleEntry, error)
}
