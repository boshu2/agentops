// practices: [agile-manifesto, dora-metrics]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/rpi"
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
		sig, err := rpi.ParseCancelSignal(tt.input)
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
	got := rpi.DescendantPIDs(100, procs)
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
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal meta: %v", err)
	}
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

func TestDiscoverSupervisorLeaseTargets_SkipsStaleLease(t *testing.T) {
	root := t.TempDir()
	lockPath := filepath.Join(root, ".agents", "rpi", "supervisor.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		t.Fatal(err)
	}
	meta := supervisorLeaseMetadata{
		RunID:     "stale-lease",
		PID:       100,
		ExpiresAt: time.Now().Add(-5 * time.Minute).UTC().Format(time.RFC3339),
	}
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal meta: %v", err)
	}
	if err := os.WriteFile(lockPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	procs := []processInfo{
		{PID: 100, PPID: 1, Command: "ao rpi loop --supervisor"},
	}
	targets := discoverSupervisorLeaseTargets(root, "", procs, map[string]struct{}{})
	if len(targets) != 0 {
		t.Fatalf("expected stale lease to be ignored, got %d targets", len(targets))
	}
}

func TestDiscoverRunRegistryTargets_SkipsMissingAndMalformedState(t *testing.T) {
	root := t.TempDir()
	runsDir := filepath.Join(root, ".agents", "rpi", "runs")
	if err := os.MkdirAll(runsDir, 0755); err != nil {
		t.Fatal(err)
	}

	missingStateDir := filepath.Join(runsDir, "missing-state")
	if err := os.MkdirAll(missingStateDir, 0755); err != nil {
		t.Fatal(err)
	}

	malformedStateDir := filepath.Join(runsDir, "malformed-state")
	if err := os.MkdirAll(malformedStateDir, 0755); err != nil {
		t.Fatal(err)
	}
	malformedPath := filepath.Join(malformedStateDir, phasedStateFile)
	if err := os.WriteFile(malformedPath, []byte("not-json"), 0644); err != nil {
		t.Fatal(err)
	}

	targets := discoverRunRegistryTargets(root, "", nil, map[string]struct{}{})
	if len(targets) != 0 {
		t.Fatalf("expected malformed/missing state to be skipped, got %d targets", len(targets))
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
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
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
		var raw map[string]any
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

// --- runRPICancel tests ---

func TestRPICancel_runRPICancel_noFlagsError(t *testing.T) {
	// Neither --all nor --run-id set
	oldAll := rpiCancelAll
	oldRunID := rpiCancelRunID
	rpiCancelAll = false
	rpiCancelRunID = ""
	defer func() {
		rpiCancelAll = oldAll
		rpiCancelRunID = oldRunID
	}()

	cmd := maturityCmd // reuse any cobra command; RunE is not called via cobra
	err := runRPICancel(cmd, nil)
	if err == nil {
		t.Fatal("expected error when neither --all nor --run-id set")
	}
	if !strings.Contains(err.Error(), "specify --all or --run-id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRPICancel_runRPICancel_badSignal(t *testing.T) {
	oldAll := rpiCancelAll
	oldSignal := rpiCancelSignal
	rpiCancelAll = true
	rpiCancelSignal = "BOGUS"
	defer func() {
		rpiCancelAll = oldAll
		rpiCancelSignal = oldSignal
	}()

	cmd := maturityCmd
	err := runRPICancel(cmd, nil)
	if err == nil {
		t.Fatal("expected error for bad signal")
	}
	if !strings.Contains(err.Error(), "unsupported signal") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRPICancel_executeCancelTargets_dryRun(t *testing.T) {
	oldDryRun := rpiCancelDryRun
	rpiCancelDryRun = true
	defer func() { rpiCancelDryRun = oldDryRun }()

	targets := []cancelTarget{
		{
			Kind:  "phased",
			RunID: "test-dry",
			PIDs:  []int{99999},
		},
	}

	captureJSONStdout(t, func() {
		failures := executeCancelTargets(targets, syscall.SIGTERM)
		if len(failures) != 0 {
			t.Fatalf("expected no failures in dry-run, got: %v", failures)
		}
	})
}

// --- cancelOneTarget tests ---

func TestRPICancel_cancelOneTarget_dryRun(t *testing.T) {
	oldDryRun := rpiCancelDryRun
	rpiCancelDryRun = true
	defer func() { rpiCancelDryRun = oldDryRun }()

	target := cancelTarget{
		Kind:  "supervisor",
		RunID: "dry-target",
		PIDs:  []int{99999},
	}

	captureJSONStdout(t, func() {
		failures := cancelOneTarget(target, syscall.SIGTERM, os.Getpid())
		if len(failures) != 0 {
			t.Fatalf("expected no failures in dry-run, got: %v", failures)
		}
	})
}

func TestRPICancel_cancelOneTarget_nonexistentPID(t *testing.T) {
	oldDryRun := rpiCancelDryRun
	rpiCancelDryRun = false
	defer func() { rpiCancelDryRun = oldDryRun }()

	// PID 99999999 should not exist; ESRCH error is silently ignored
	target := cancelTarget{
		Kind:  "phased",
		RunID: "no-pid",
		PIDs:  []int{99999999},
	}

	captureJSONStdout(t, func() {
		failures := cancelOneTarget(target, syscall.SIGTERM, os.Getpid())
		// ESRCH is ignored, so no failures
		if len(failures) != 0 {
			t.Fatalf("expected ESRCH to be ignored, got failures: %v", failures)
		}
	})
}

func TestRPICancel_cancelOneTarget_withStateUpdate(t *testing.T) {
	oldDryRun := rpiCancelDryRun
	rpiCancelDryRun = false
	defer func() { rpiCancelDryRun = oldDryRun }()

	root := t.TempDir()
	runID := "cancel-state-test"
	statePath := filepath.Join(root, ".agents", "rpi", "runs", runID, phasedStateFile)
	if err := os.MkdirAll(filepath.Dir(statePath), 0755); err != nil {
		t.Fatal(err)
	}

	state := map[string]any{
		"schema_version": 1,
		"run_id":         runID,
		"goal":           "test",
		"phase":          1,
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	data = append(data, '\n')
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		t.Fatal(err)
	}

	target := cancelTarget{
		Kind:      "phased",
		RunID:     runID,
		Root:      root,
		StatePath: statePath,
		PIDs:      []int{99999999}, // nonexistent, ESRCH ignored
	}

	captureJSONStdout(t, func() {
		failures := cancelOneTarget(target, syscall.SIGTERM, os.Getpid())
		if len(failures) != 0 {
			t.Fatalf("unexpected failures: %v", failures)
		}
	})

	// Verify state was updated
	updated, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(updated, &raw); err != nil {
		t.Fatal(err)
	}
	if raw["terminal_status"] != "interrupted" {
		t.Fatalf("expected terminal_status=interrupted, got %v", raw["terminal_status"])
	}
}

func TestRPICancel_discoverCancelTargets_sortOrder(t *testing.T) {
	// Create two roots, one with a supervisor lease and one with a run registry
	root1 := t.TempDir()
	root2 := t.TempDir()

	// Root1: supervisor lease with active process (self PID)
	leaseDir1 := filepath.Join(root1, ".agents", "rpi")
	if err := os.MkdirAll(leaseDir1, 0755); err != nil {
		t.Fatal(err)
	}
	meta1 := supervisorLeaseMetadata{
		RunID:     "z-lease",
		PID:       os.Getpid(),
		ExpiresAt: time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339),
	}
	data1, _ := json.Marshal(meta1)
	if err := os.WriteFile(filepath.Join(leaseDir1, "supervisor.lock"), data1, 0644); err != nil {
		t.Fatal(err)
	}

	procs := []processInfo{
		{PID: os.Getpid(), PPID: 1, Command: "test-process"},
	}

	targets := discoverCancelTargets([]string{root1, root2}, "", procs)
	// Should find at least the lease target
	for i := 1; i < len(targets); i++ {
		if targets[i-1].Kind > targets[i].Kind {
			t.Fatalf("targets not sorted by kind: %s > %s", targets[i-1].Kind, targets[i].Kind)
		}
	}
}

// --- loadActiveSupervisorLease tests ---

func TestRPICancel_loadActiveSupervisorLease_missingFile(t *testing.T) {
	_, ok := loadActiveSupervisorLease("/nonexistent/path", "", nil)
	if ok {
		t.Fatal("expected false for missing file")
	}
}

func TestRPICancel_loadActiveSupervisorLease_emptyRunID(t *testing.T) {
	tmp := t.TempDir()
	leasePath := filepath.Join(tmp, "supervisor.lock")
	meta := supervisorLeaseMetadata{
		RunID: "", // empty
		PID:   100,
	}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(leasePath, data, 0644); err != nil {
		t.Fatal(err)
	}
	_, ok := loadActiveSupervisorLease(leasePath, "", nil)
	if ok {
		t.Fatal("expected false for empty run ID")
	}
}

func TestRPICancel_loadActiveSupervisorLease_runIDMismatch(t *testing.T) {
	tmp := t.TempDir()
	leasePath := filepath.Join(tmp, "supervisor.lock")
	meta := supervisorLeaseMetadata{
		RunID:     "run-A",
		PID:       os.Getpid(),
		ExpiresAt: time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339),
	}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(leasePath, data, 0644); err != nil {
		t.Fatal(err)
	}

	procs := []processInfo{{PID: os.Getpid(), PPID: 1, Command: "test"}}
	_, ok := loadActiveSupervisorLease(leasePath, "run-B", procs)
	if ok {
		t.Fatal("expected false for run ID mismatch")
	}
}

func TestRPICancel_loadActiveSupervisorLease_processNotFound(t *testing.T) {
	tmp := t.TempDir()
	leasePath := filepath.Join(tmp, "supervisor.lock")
	meta := supervisorLeaseMetadata{
		RunID:     "run-C",
		PID:       99999999,
		ExpiresAt: time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339),
	}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(leasePath, data, 0644); err != nil {
		t.Fatal(err)
	}

	// Empty proc list: PID won't be found
	_, ok := loadActiveSupervisorLease(leasePath, "", nil)
	if ok {
		t.Fatal("expected false when process not in list")
	}
}
