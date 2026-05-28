// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import (
	"context"
	"errors"
)

// InMemoryWorkspace is a WorkspacePort that records setup and cleanup
// calls in memory.
type InMemoryWorkspace struct {
	Setups   []WorkspaceRequest
	Cleanups []WorkspaceRequest
}

func NewInMemoryWorkspace() *InMemoryWorkspace {
	return &InMemoryWorkspace{
		Setups:   []WorkspaceRequest{},
		Cleanups: []WorkspaceRequest{},
	}
}

func (w *InMemoryWorkspace) Setup(ctx context.Context, req WorkspaceRequest) (WorkspaceResult, error) {
	if err := ctx.Err(); err != nil {
		return WorkspaceResult{}, err
	}
	if req.WorkspaceID == "" {
		return WorkspaceResult{}, errors.New("ports: WorkspacePort.Setup workspace id required")
	}
	w.Setups = append(w.Setups, cloneWorkspaceRequest(req))
	return WorkspaceResult{WorkspaceID: req.WorkspaceID, Path: req.Path, Status: "setup", Reason: "workspace setup recorded"}, nil
}

func (w *InMemoryWorkspace) Cleanup(ctx context.Context, req WorkspaceRequest) (WorkspaceResult, error) {
	if err := ctx.Err(); err != nil {
		return WorkspaceResult{}, err
	}
	if req.WorkspaceID == "" {
		return WorkspaceResult{}, errors.New("ports: WorkspacePort.Cleanup workspace id required")
	}
	w.Cleanups = append(w.Cleanups, cloneWorkspaceRequest(req))
	return WorkspaceResult{WorkspaceID: req.WorkspaceID, Path: req.Path, Status: "cleanup", Reason: "workspace cleanup recorded"}, nil
}

func cloneWorkspaceRequest(req WorkspaceRequest) WorkspaceRequest {
	metadata := make(map[string]string, len(req.Metadata))
	for key, value := range req.Metadata {
		metadata[key] = value
	}
	req.Metadata = metadata
	return req
}

var _ WorkspacePort = (*InMemoryWorkspace)(nil)
