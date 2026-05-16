// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import "context"

// HypothesisVerdict is the current empirical status of one evolve
// hypothesis. The string type stays open so file-backed adapters can
// round-trip legacy ledger rows that include explanatory suffixes.
type HypothesisVerdict string

const (
	HypothesisVerdictPending   HypothesisVerdict = "PENDING"
	HypothesisVerdictVerified  HypothesisVerdict = "VERIFIED"
	HypothesisVerdictFalsified HypothesisVerdict = "FALSIFIED"
)

// HypothesisRecord is one row in evolve's hypothesis ledger
// (.agents/evolve/hypotheses.jsonl). The ledger is local runtime
// state, but the shape is a BC3 Loop contract: a patch makes a
// falsifiable claim, names the future check cycle, and later records
// evidence for the verdict.
type HypothesisRecord struct {
	ID           string            `json:"id"`
	CycleLanded  int               `json:"cycle_landed,omitempty"`
	CheckAtCycle int               `json:"check_at_cycle,omitempty"`
	Patch        string            `json:"patch,omitempty"`
	Hypothesis   string            `json:"hypothesis,omitempty"`
	Measure      string            `json:"measure,omitempty"`
	Verdict      HypothesisVerdict `json:"verdict,omitempty"`
	Evidence     []string          `json:"evidence,omitempty"`
}

// HypothesisLedgerPort is the BC3 Loop port for evolve's empirical
// improvement ledger. Callers - evolve's cycle closeout, post-mortem
// checks, convergence audits, and future hypothesis-verdict workers -
// depend on this port so hypothesis tracking does not depend directly
// on the local JSONL file shape.
//
// Contract:
//
//   - Append MUST reject an empty ID.
//   - Append MUST reject duplicate IDs with a non-nil error.
//   - Append returns the record as stored.
//   - List returns records in append order.
//   - Find returns (zero-value, false, nil) when the ID is unknown.
//   - Returned records MUST be safe for callers to mutate.
//   - Context cancellation MUST be honored on a best-effort basis.
//
// See docs/contracts/ubiquitous-language.md (BC3 row). This port is
// paired with ConvergenceCheckPort so the loop can state both "what
// are we testing?" and "are we done?" in typed domain terms.
type HypothesisLedgerPort interface {
	Append(ctx context.Context, record HypothesisRecord) (HypothesisRecord, error)
	List(ctx context.Context) ([]HypothesisRecord, error)
	Find(ctx context.Context, id string) (HypothesisRecord, bool, error)
}
