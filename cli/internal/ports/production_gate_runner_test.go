// practices: [hexagonal-architecture, tdd]
package ports

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Sibling pattern: cycle 114 finding_compiler_adapter_test.go.
// First subprocess-based adapter test — uses temp repo root with a
// fake scripts/check-<name>.sh that exits with a specific code.

func newTempRepoWithGate(t *testing.T, name string, script string) string {
	t.Helper()
	root := t.TempDir()
	scriptDir := filepath.Join(root, "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(scriptDir, "check-"+name+".sh")
	if err := os.WriteFile(scriptPath, []byte("#!/usr/bin/env bash\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestProductionGateRunner_Exit0IsPass(t *testing.T) {
	root := newTempRepoWithGate(t, "ok", `echo all-good; exit 0`)
	g := NewProductionGateRunner(root)
	v, err := g.Run(context.Background(), GateRunRequest{Name: "ok"})
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != GateStatusPass {
		t.Fatalf("Status = %s, want PASS", v.Status)
	}
	if v.Reason == "" {
		t.Fatal("Reason must be non-empty")
	}
}

func TestProductionGateRunner_Exit1IsFail(t *testing.T) {
	root := newTempRepoWithGate(t, "broken", `echo bad; exit 1`)
	g := NewProductionGateRunner(root)
	v, _ := g.Run(context.Background(), GateRunRequest{Name: "broken"})
	if v.Status != GateStatusFail {
		t.Fatalf("Status = %s, want FAIL", v.Status)
	}
}

func TestProductionGateRunner_Exit2IsWarn(t *testing.T) {
	root := newTempRepoWithGate(t, "advisory", `echo warn; exit 2`)
	g := NewProductionGateRunner(root)
	v, _ := g.Run(context.Background(), GateRunRequest{Name: "advisory"})
	if v.Status != GateStatusWarn {
		t.Fatalf("Status = %s, want WARN", v.Status)
	}
}

func TestProductionGateRunner_Exit75IsSkip(t *testing.T) {
	root := newTempRepoWithGate(t, "structural", `echo skip; exit 75`)
	g := NewProductionGateRunner(root)
	v, _ := g.Run(context.Background(), GateRunRequest{Name: "structural"})
	if v.Status != GateStatusSkip {
		t.Fatalf("Status = %s, want SKIP", v.Status)
	}
}

func TestProductionGateRunner_MissingScriptIsUnknown(t *testing.T) {
	root := t.TempDir()
	g := NewProductionGateRunner(root)
	v, _ := g.Run(context.Background(), GateRunRequest{Name: "does-not-exist"})
	if v.Status != GateStatusUnknown {
		t.Fatalf("Status = %s, want UNKNOWN", v.Status)
	}
}

func TestProductionGateRunner_EmptyNameIsUnknown(t *testing.T) {
	g := NewProductionGateRunner(t.TempDir())
	v, _ := g.Run(context.Background(), GateRunRequest{Name: ""})
	if v.Status != GateStatusUnknown {
		t.Fatalf("Status = %s, want UNKNOWN", v.Status)
	}
	if v.Reason != "empty GateName" {
		t.Fatalf("Reason = %q, want %q (port contract)", v.Reason, "empty GateName")
	}
}

func TestProductionGateRunner_LogTailCaptured(t *testing.T) {
	root := newTempRepoWithGate(t, "noisy", `echo first; echo second; exit 0`)
	g := NewProductionGateRunner(root)
	v, _ := g.Run(context.Background(), GateRunRequest{Name: "noisy"})
	if v.LogTail == "" {
		t.Fatal("LogTail empty")
	}
	for _, want := range []string{"first", "second"} {
		if !strings.Contains(v.LogTail, want) {
			t.Fatalf("LogTail missing %q: %q", want, v.LogTail)
		}
	}
}

func TestProductionGateRunner_EnvPassedThrough(t *testing.T) {
	root := newTempRepoWithGate(t, "envcheck", `echo "GATE_TEST_VAR=$GATE_TEST_VAR"; exit 0`)
	g := NewProductionGateRunner(root)
	v, _ := g.Run(context.Background(), GateRunRequest{
		Name: "envcheck",
		Env:  map[string]string{"GATE_TEST_VAR": "honored"},
	})
	if !strings.Contains(v.LogTail, "GATE_TEST_VAR=honored") {
		t.Fatalf("env not passed through: %q", v.LogTail)
	}
}

func TestProductionGateRunner_EmptyRepoRootErrors(t *testing.T) {
	g := NewProductionGateRunner("")
	_, err := g.Run(context.Background(), GateRunRequest{Name: "x"})
	if err == nil {
		t.Fatal("expected error on empty repoRoot, got nil")
	}
}

func TestProductionGateRunner_HonorsContextCancellation(t *testing.T) {
	g := NewProductionGateRunner(t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := g.Run(ctx, GateRunRequest{Name: "x"})
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
