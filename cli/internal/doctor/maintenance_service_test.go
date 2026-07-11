package doctor

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeMaintenanceRuntime struct {
	root string
	err  error
}

func (runtime fakeMaintenanceRuntime) RepoRoot(context.Context) (string, error) {
	return runtime.root, runtime.err
}

type fakeMaintenanceGateway struct {
	undoRoot    string
	undoRequest UndoRequest
	gcRoot      string
	gcCutoff    time.Time
	gcYes       bool
	gcDryRun    bool
}

func (gateway *fakeMaintenanceGateway) Undo(_ context.Context, root string, request UndoRequest) (*UndoResult, error) {
	gateway.undoRoot, gateway.undoRequest = root, request
	return &UndoResult{RunID: request.RunID}, nil
}

func (gateway *fakeMaintenanceGateway) GC(_ context.Context, root string, cutoff time.Time, yes, dryRun bool) (GCResult, error) {
	gateway.gcRoot, gateway.gcCutoff = root, cutoff
	gateway.gcYes, gateway.gcDryRun = yes, dryRun
	return GCResult{Matched: 2, DryRun: dryRun}, nil
}

func TestMaintenanceServiceDelegatesUndoAndGCThroughPorts(t *testing.T) {
	gateway := &fakeMaintenanceGateway{}
	service := NewMaintenanceService(fakeMaintenanceRuntime{root: "/repo"}, gateway)
	undo, err := service.Undo(context.Background(), UndoRequest{RunID: "latest", Strict: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if undo.RunID != "latest" || gateway.undoRoot != "/repo" || !gateway.undoRequest.Strict || !gateway.undoRequest.DryRun {
		t.Fatalf("undo=%+v gateway=%+v", undo, gateway)
	}
	gc, err := service.GC(context.Background(), GCRequest{Before: "2026-01-02", Yes: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	wantCutoff := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	if gc.Matched != 2 || gateway.gcRoot != "/repo" || !gateway.gcCutoff.Equal(wantCutoff) || !gateway.gcYes || !gateway.gcDryRun {
		t.Fatalf("gc=%+v gateway=%+v", gc, gateway)
	}
}

func TestMaintenanceServiceClassifiesRuntimeAndUsageFailures(t *testing.T) {
	service := NewMaintenanceService(fakeMaintenanceRuntime{err: errors.New("cwd")}, &fakeMaintenanceGateway{})
	_, err := service.Undo(context.Background(), UndoRequest{RunID: "latest"})
	var runtimeFailure *RuntimeError
	if !errors.As(err, &runtimeFailure) {
		t.Fatalf("undo error=%#v, want RuntimeError", err)
	}

	service = NewMaintenanceService(fakeMaintenanceRuntime{root: "/repo"}, &fakeMaintenanceGateway{})
	_, err = service.GC(context.Background(), GCRequest{Before: "not-a-date", Yes: true})
	var usageFailure *UsageError
	if !errors.As(err, &usageFailure) {
		t.Fatalf("gc error=%#v, want UsageError", err)
	}
}
