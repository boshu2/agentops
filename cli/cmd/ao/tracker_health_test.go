// practices: [dora-metrics, distributed-tracing]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTrackerFixture(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bd-fixture.sh")
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write tracker fixture: %v", err)
	}
	return path
}

func TestDetectTrackerHealth_Healthy(t *testing.T) {
	command := writeTrackerFixture(t, `#!/bin/sh
set -eu
case "${1:-}" in
  ready)
    printf '[]\n'
    ;;
  list)
    printf '[{"id":"ag-123"}]\n'
    ;;
  *)
    printf '[]\n'
    ;;
esac
`)

	health := detectTrackerHealth(command, nil)
	if !health.Healthy {
		t.Fatalf("healthy = false, want true: %+v", health)
	}
	if health.Mode != "beads" {
		t.Fatalf("mode = %q, want beads", health.Mode)
	}
	if !strings.Contains(health.Reason, "succeeded") {
		t.Fatalf("reason = %q, want probe-success hint", health.Reason)
	}
}

func TestDetectTrackerHealth_Degraded(t *testing.T) {
	command := writeTrackerFixture(t, `#!/bin/sh
set -eu
printf 'column "crystallizes" could not be found in any table in scope\n' >&2
exit 1
`)

	health := detectTrackerHealth(command, nil)
	if health.Healthy {
		t.Fatalf("healthy = true, want false: %+v", health)
	}
	if health.Mode != "tasklist" {
		t.Fatalf("mode = %q, want tasklist", health.Mode)
	}
	if !strings.Contains(health.Error, "crystallizes") {
		t.Fatalf("error = %q, want beads schema failure", health.Error)
	}
}

func TestProcessDiscoveryPhase_TrackerDegradedUsesTasklistFallback(t *testing.T) {
	root := t.TempDir()
	rpiDir := filepath.Join(root, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	plansDir := filepath.Join(root, ".agents", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plansDir, "2026-03-24-tasklist-plan.md"), []byte("# Plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	command := writeTrackerFixture(t, `#!/bin/sh
set -eu
printf 'column "crystallizes" could not be found in any table in scope\n' >&2
exit 1
`)

	// Create a mock council pre-mortem report so the fail-closed gate passes.
	councilDir := filepath.Join(root, ".agents", "council")
	if err := os.MkdirAll(councilDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mockReport := "---\ntype: pre-mortem\n---\n# Pre-Mortem\n\n## Council Verdict: PASS\n"
	if err := os.WriteFile(filepath.Join(councilDir, "2026-03-24-pre-mortem-tasklist.md"), []byte(mockReport), 0o644); err != nil {
		t.Fatal(err)
	}

	state := newTestPhasedState().
		WithRunID("tasklist-run").
		WithGoal("execute Codex no-beads proof")
	state.Opts = phasedEngineOptions{BDCommand: command}

	logPath := filepath.Join(rpiDir, "phased-orchestration.log")
	if err := processDiscoveryPhase(root, state, logPath); err != nil {
		t.Fatalf("processDiscoveryPhase: %v", err)
	}
	if state.TrackerMode != "tasklist" {
		t.Fatalf("tracker_mode = %q, want tasklist", state.TrackerMode)
	}
	if state.EpicID != "" {
		t.Fatalf("epic_id = %q, want empty for tasklist fallback", state.EpicID)
	}

	data, err := os.ReadFile(filepath.Join(rpiDir, "execution-packet.json"))
	if err != nil {
		t.Fatalf("read execution packet: %v", err)
	}
	archivedData, err := os.ReadFile(filepath.Join(rpiDir, "runs", state.RunID, executionPacketFile))
	if err != nil {
		t.Fatalf("read archived execution packet: %v", err)
	}
	if string(archivedData) != string(data) {
		t.Fatalf("archived execution packet does not match latest alias:\nlatest:\n%s\narchived:\n%s", data, archivedData)
	}
	var packet executionPacket
	if err := json.Unmarshal(data, &packet); err != nil {
		t.Fatalf("parse execution packet: %v", err)
	}
	if packet.TrackerMode != "tasklist" {
		t.Fatalf("packet tracker_mode = %q, want tasklist", packet.TrackerMode)
	}
	if packet.PlanPath == "" {
		t.Fatal("packet plan_path empty, want discovery fallback plan")
	}
	if packet.TrackerHealth == nil || packet.TrackerHealth.Healthy {
		t.Fatalf("packet tracker_health = %+v, want degraded tracker", packet.TrackerHealth)
	}
}
