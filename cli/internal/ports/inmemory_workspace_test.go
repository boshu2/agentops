// practices: [hexagonal-architecture, tdd]
package ports

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestInMemoryWorkspace_RecordsSetupAndCleanup(t *testing.T) {
	workspace := NewInMemoryWorkspace()
	setup, err := workspace.Setup(context.Background(), WorkspaceRequest{WorkspaceID: "run-1", Path: "/tmp/run-1"})
	if err != nil {
		t.Fatal(err)
	}
	cleanup, err := workspace.Cleanup(context.Background(), WorkspaceRequest{WorkspaceID: "run-1", Path: "/tmp/run-1"})
	if err != nil {
		t.Fatal(err)
	}
	if setup.Status != "setup" || cleanup.Status != "cleanup" {
		t.Fatalf("statuses = %q/%q, want setup/cleanup", setup.Status, cleanup.Status)
	}
	if len(workspace.Setups) != 1 || len(workspace.Cleanups) != 1 {
		t.Fatalf("recorded setup/cleanup = %d/%d, want 1/1", len(workspace.Setups), len(workspace.Cleanups))
	}
}

func TestInMemoryWorkspace_RejectsEmptyWorkspaceID(t *testing.T) {
	workspace := NewInMemoryWorkspace()
	_, err := workspace.Setup(context.Background(), WorkspaceRequest{})
	if err == nil {
		t.Fatal("expected setup error for empty workspace id")
	}
	if !strings.Contains(err.Error(), "workspace id required") {
		t.Fatalf("setup error = %v, want workspace id required", err)
	}
	_, err = workspace.Cleanup(context.Background(), WorkspaceRequest{})
	if err == nil {
		t.Fatal("expected cleanup error for empty workspace id")
	}
}

func TestInMemoryWorkspace_CopiesMetadata(t *testing.T) {
	workspace := NewInMemoryWorkspace()
	req := WorkspaceRequest{WorkspaceID: "run-1", Metadata: map[string]string{"role": "lead"}}
	if _, err := workspace.Setup(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	req.Metadata["role"] = "mutated"
	if workspace.Setups[0].Metadata["role"] != "lead" {
		t.Fatalf("metadata mutated to %q", workspace.Setups[0].Metadata["role"])
	}
}

func TestInMemoryWorkspace_HonorsContextCancellation(t *testing.T) {
	workspace := NewInMemoryWorkspace()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := workspace.Setup(ctx, WorkspaceRequest{WorkspaceID: "run-1"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
