package doctor

import (
	"context"
	"testing"
)

func TestMaintenanceRuntimeReturnsCurrentDirectory(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	got, err := (MaintenanceRuntime{}).RepoRoot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Fatalf("root=%q, want %q", got, root)
	}
}
