// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import "context"

// OperatorIntent captures one human-in-the-loop request to the factory.
// Kind names the intent shape (e.g. "rescope", "halt", "promote",
// "approve"); Subject is a free-form reference to what the intent is
// about (a bd ID, a commit SHA, a file path); Note is a free-form
// human-readable annotation.
type OperatorIntent struct {
	Kind    string
	Subject string
	Note    string
}

// OperatorPort is the BC4 Factory operator-facing surface. Callers —
// the /halt skill, /rescope skill, /handoff skill, and any future
// human-in-the-loop nudge — depend on this port so they can record
// operator intent into the factory event stream without coupling to
// a specific persistence backend.
//
// Contract:
//
//   - Record MUST accept any non-empty Kind. Adapters MAY validate
//     specific Kind values; the in-memory adapter accepts all.
//   - Empty Kind is a structural-rejection error.
//   - List returns all recorded intents, most-recent first.
//   - Context cancellation MUST be honored on a best-effort basis.
//
// See docs/contracts/ubiquitous-language.md (BC4 row). soc-2klg epic
// tracks BC4 port extraction; this is the first port in that epic.
// Sibling: EventBusPort (next cycle) carries the same intents to
// downstream subscribers asynchronously.
type OperatorPort interface {
	Record(ctx context.Context, intent OperatorIntent) error
	List(ctx context.Context) ([]OperatorIntent, error)
}
