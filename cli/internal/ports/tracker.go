// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import "context"

// Issue is the driven-port view of one tracker issue (a bead). It carries the
// subset of fields the CLI's read paths consume; adapters populate what their
// backend exposes and leave the rest zero-valued.
type Issue struct {
	ID        string
	Title     string
	Status    string
	Type      string
	Priority  int
	Assignee  string
	UpdatedAt string
}

// IssueFilter narrows a List query. Zero-valued fields are not applied, so an
// empty filter lists everything the backend returns. Limit <= 0 means no limit.
type IssueFilter struct {
	Type          string // e.g. "epic", "bug", "task"
	Status        string // e.g. "in_progress", "open"
	Limit         int    // max items to return; <= 0 disables the cap
	All           bool   // include closed/all states when the backend supports it
	MetadataField string // "key=value" filter (bd: --metadata-field)
}

// IssueTracker is the driven port for epic/issue lifecycle operations.
//
// Implementations: tracker_bd (real, shells out to the `bd` binary),
// InMemoryTracker (in-memory test double). The previous create-only surface was
// widened (soc-ebgjk) to cover the read paths the CLI actually depends on —
// Ready/List/Show — which were reaching `bd` via scattered exec.Command calls.
//
// Contract:
//
//   - Mode reports the backend identity ("beads" | "tasklist" | "memory").
//   - Read methods (Ready/List/Show) are side-effect free.
//   - Show on a missing id returns a non-nil error.
//   - Context cancellation MUST be honored on a best-effort basis.
type IssueTracker interface {
	Mode() string // "beads" | "tasklist" | "memory"

	// Create paths.
	CreateEpic(ctx context.Context, title, body string) (epicID string, err error)
	CreateIssue(ctx context.Context, epicID, title, body string) (issueID string, err error)

	// Read paths.
	Ready(ctx context.Context) ([]Issue, error)
	List(ctx context.Context, filter IssueFilter) ([]Issue, error)
	Show(ctx context.Context, id string) (Issue, error)
}
