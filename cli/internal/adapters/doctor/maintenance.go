package doctor

import (
	"context"
	"time"

	doctorapp "github.com/boshu2/agentops/cli/internal/doctor"
)

// MaintenanceRuntime snapshots process state for doctor maintenance requests.
type MaintenanceRuntime struct{}

// RepoRoot returns the current doctor target directory.
func (MaintenanceRuntime) RepoRoot(ctx context.Context) (string, error) {
	return (ReadRuntime{}).RepoRoot(ctx)
}

// MaintenanceGateway adapts maintenance use cases to the existing engine.
type MaintenanceGateway struct{}

// Undo executes the doctor undo engine.
func (MaintenanceGateway) Undo(_ context.Context, root string, request doctorapp.UndoRequest) (*doctorapp.UndoResult, error) {
	return doctorapp.Undo(root, request.RunID, request.Strict, request.DryRun)
}

// GC executes the doctor garbage-collection engine.
func (MaintenanceGateway) GC(_ context.Context, root string, cutoff time.Time, yes, dryRun bool) (doctorapp.GCResult, error) {
	return doctorapp.GC(root, cutoff, yes, dryRun)
}
