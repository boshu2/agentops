// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import "context"

// WorkspaceRequest identifies one runtime workspace or worktree.
// WorkspaceID is required and should be stable for the lifecycle being
// managed. Path is optional for adapters that derive paths from ids.
type WorkspaceRequest struct {
	WorkspaceID string
	Path        string
	Metadata    map[string]string
}

// WorkspaceResult is the result of a setup or cleanup operation.
type WorkspaceResult struct {
	WorkspaceID string
	Path        string
	Status      string
	Reason      string
}

// WorkspacePort is the Runtime Shell surface for workspace lifecycle
// operations. It absorbs worktree setup and cleanup hooks so those
// operations can run as explicit runtime adapter calls.
//
// Contract:
//
//   - Setup and Cleanup reject an empty WorkspaceID.
//   - Successful results MUST include WorkspaceID and non-empty Status.
//   - Context cancellation MUST be honored on a best-effort basis.
type WorkspacePort interface {
	Setup(ctx context.Context, req WorkspaceRequest) (WorkspaceResult, error)
	Cleanup(ctx context.Context, req WorkspaceRequest) (WorkspaceResult, error)
}
