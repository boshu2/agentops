package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseCancelSignal(t *testing.T) {
	tests := []struct {
		input    string
		wantErr  bool
		wantSign string
	}{
		{input: "TERM", wantSign: "terminated"},
		{input: "SIGTERM", wantSign: "terminated"},
		{input: "KILL", wantSign: "killed"},
		{input: "INT", wantSign: "interrupt"},
		{input: "bogus", wantErr: true},
	}
	for _, tt := range tests {
		sig, err := parseCancelSignal(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("expected error for signal %q", tt.input)
			}
			continue
		}
		if err != nil {
			t.Fatalf("signal %q: %v", tt.input, err)
		}
		if sig.String() != tt.wantSign {
			t.Fatalf("signal %q => %q, want %q", tt.input, sig.String(), tt.wantSign)
		}
	}
}

func TestDescendantPIDs(t *testing.T) {
	procs := []processInfo{
		{PID: 100, PPID: 1},
		{PID: 101, PPID: 100},
		{PID: 102, PPID: 101},
		{PID: 200, PPID: 1},
	}
	got := descendantPIDs(100, procs)
	if len(got) != 2 || got[0] != 101 || got[1] != 102 {
		t.Fatalf("descendants mismatch: got %v, want [101 102]", got)
	}
}

func TestCollectRunProcessPIDs_UsesOrchestratorPID(t *testing.T) {
	state := &phasedState{
		RunID:           "run-1",
		OrchestratorPID: 100,
	}
	procs := []processInfo{
		{PID: 100, PPID: 1, Command: "ao rpi phased"},
		{PID: 101, PPID: 100, Command: "claude -p prompt"},
		{PID: 201, PPID: 1, Command: "sleep 1000"},
	}
	got := collectRunProcessPIDs(state, procs)
	if len(got) != 2 || got[0] != 100 || got[1] != 101 {
		t.Fatalf("process targets mismatch: got %v, want [100 101]", got)
	}
}

func TestDiscoverSupervisorLeaseTargets(t *testing.T) {
	root := t.TempDir()
	lockPath := filepath.Join(root, ".agents", "rpi", "supervisor.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		t.Fatal(err)
	}
	meta := supervisorLeaseMetadata{
		RunID: "lease-run",
		PID:   100,
	}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(lockPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	procs := []processInfo{
		{PID: 100, PPID: 1, Command: "ao rpi loop --supervisor"},
		{PID: 101, PPID: 100, Command: "ao rpi phased"},
	}
	targets := discoverSupervisorLeaseTargets(root, "", procs, map[string]struct{}{})
	if len(targets) != 1 {
		t.Fatalf("expected one lease target, got %d", len(targets))
	}
	if targets[0].RunID != "lease-run" {
		t.Fatalf("unexpected run id: %q", targets[0].RunID)
	}
	if len(targets[0].PIDs) != 2 {
		t.Fatalf("expected lease pid and child, got %v", targets[0].PIDs)
	}
}

func TestMarkRunInterruptedByCancel(t *testing.T) {
	root := t.TempDir()
	runID := "run-cancel"
	runStatePath := filepath.Join(root, ".agents", "rpi", "runs", runID, phasedStateFile)
	flatStatePath := filepath.Join(root, ".agents", "rpi", phasedStateFile)
	if err := os.MkdirAll(filepath.Dir(runStatePath), 0755); err != nil {
		t.Fatal(err)
	}

	state := phasedState{
		SchemaVersion: 1,
		RunID:         runID,
		Goal:          "test",
		Phase:         1,
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	data = append(data, '\n')
	if err := os.WriteFile(runStatePath, data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(flatStatePath, data, 0644); err != nil {
		t.Fatal(err)
	}

	target := cancelTarget{
		RunID:     runID,
		Root:      root,
		StatePath: runStatePath,
	}
	if err := markRunInterruptedByCancel(target); err != nil {
		t.Fatalf("markRunInterruptedByCancel: %v", err)
	}

	check := func(path string) {
		t.Helper()
		updatedData, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var raw map[string]interface{}
		if err := json.Unmarshal(updatedData, &raw); err != nil {
			t.Fatal(err)
		}
		if raw["terminal_status"] != "interrupted" {
			t.Fatalf("terminal_status mismatch in %s: %v", path, raw["terminal_status"])
		}
		if raw["terminal_reason"] != "cancelled by ao rpi cancel" {
			t.Fatalf("terminal_reason mismatch in %s: %v", path, raw["terminal_reason"])
		}
		if raw["terminated_at"] == "" {
			t.Fatalf("terminated_at should be set in %s", path)
		}
	}

	check(runStatePath)
	check(flatStatePath)
}
