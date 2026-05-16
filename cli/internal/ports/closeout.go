// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import "context"

// CloseoutAction identifies one explicit closeout operation.
type CloseoutAction string

const (
	CloseoutActionHandoff  CloseoutAction = "handoff"
	CloseoutActionFlywheel CloseoutAction = "flywheel"
	CloseoutActionDefrag   CloseoutAction = "defrag"
	CloseoutActionMaintain CloseoutAction = "maintain"
)

// CloseoutRequest describes an explicit end-of-cycle or end-of-session
// closeout. RunID is optional for local/manual closeout but should be
// supplied by RPI/evolve cycle callers.
type CloseoutRequest struct {
	RunID    string
	Actions  []CloseoutAction
	Metadata map[string]string
}

// CloseoutResult reports what was completed and which artifacts were
// written. Artifacts are paths or durable references.
type CloseoutResult struct {
	Completed []CloseoutAction
	Artifacts []string
	Warnings  []string
}

// CloseoutPort is the hookless-first closeout surface. It absorbs
// Stop and SessionEnd maintenance hooks by making closeout an
// explicit lifecycle operation.
//
// Contract:
//
//   - Close rejects an empty Actions list.
//   - Returned slices MUST be safe for the caller to mutate.
//   - Context cancellation MUST be honored on a best-effort basis.
type CloseoutPort interface {
	Close(ctx context.Context, req CloseoutRequest) (CloseoutResult, error)
}
