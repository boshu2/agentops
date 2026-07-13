package doctor

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// UsageError marks invalid maintenance command input.
type UsageError struct{ Err error }

func (failure *UsageError) Error() string { return failure.Err.Error() }
func (failure *UsageError) Unwrap() error { return failure.Err }

// UndoRequest carries caller-owned options for a doctor undo invocation.
type UndoRequest struct {
	RunID  string
	Strict bool
	DryRun bool
}

// GCRequest carries caller-owned options for a doctor GC invocation.
type GCRequest struct {
	Before string
	Yes    bool
	DryRun bool
}

// MaintenanceRuntime snapshots process state needed by maintenance use cases.
type MaintenanceRuntime interface {
	RepoRoot(context.Context) (string, error)
}

// MaintenanceGateway executes doctor undo and garbage collection.
type MaintenanceGateway interface {
	Undo(context.Context, string, UndoRequest) (*UndoResult, error)
	GC(context.Context, string, time.Time, bool, bool) (GCResult, error)
}

// MaintenanceService owns doctor undo and GC application orchestration.
type MaintenanceService struct {
	runtime MaintenanceRuntime
	gateway MaintenanceGateway
}

// NewMaintenanceService constructs the doctor maintenance application service.
func NewMaintenanceService(runtime MaintenanceRuntime, gateway MaintenanceGateway) MaintenanceService {
	return MaintenanceService{runtime: runtime, gateway: gateway}
}

// Undo resolves the repository root and executes an undo request.
func (service MaintenanceService) Undo(ctx context.Context, request UndoRequest) (*UndoResult, error) {
	root, err := service.runtime.RepoRoot(ctx)
	if err != nil {
		return nil, &RuntimeError{Err: err}
	}
	return service.gateway.Undo(ctx, root, request)
}

// GC validates command input, resolves the repository root, and executes GC.
func (service MaintenanceService) GC(ctx context.Context, request GCRequest) (GCResult, error) {
	if !request.Yes || strings.TrimSpace(request.Before) == "" {
		return GCResult{DryRun: request.DryRun}, &UsageError{Err: fmt.Errorf("doctor: gc requires --yes and --before <date>")}
	}
	cutoff, err := time.Parse("2006-01-02", request.Before)
	if err != nil {
		return GCResult{DryRun: request.DryRun}, &UsageError{Err: fmt.Errorf("invalid --before date (want YYYY-MM-DD): %w", err)}
	}
	root, err := service.runtime.RepoRoot(ctx)
	if err != nil {
		return GCResult{DryRun: request.DryRun}, &RuntimeError{Err: err}
	}
	return service.gateway.GC(ctx, root, cutoff, request.Yes, request.DryRun)
}
