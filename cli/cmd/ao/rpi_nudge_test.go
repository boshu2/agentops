package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveNudgeTargets_AllWorkers(t *testing.T) {
	base := "ao-rpi-run12345-p1"
	sessions := []string{base, base + "-w2", base + "-w1", "other"}

	got, err := resolveNudgeTargets(sessions, base, true, 0)
	if err != nil {
		t.Fatalf("resolveNudgeTargets(all workers): %v", err)
	}
	if len(got) != 2 || got[0] != base+"-w1" || got[1] != base+"-w2" {
		t.Fatalf("unexpected targets: %#v", got)
	}
}

func TestResolveNudgeTargets_DefaultMayor(t *testing.T) {
	base := "ao-rpi-run12345-p2"
	sessions := []string{base}

	got, err := resolveNudgeTargets(sessions, base, false, 0)
	if err != nil {
		t.Fatalf("resolveNudgeTargets(default mayor): %v", err)
	}
	if len(got) != 1 || got[0] != base {
		t.Fatalf("unexpected targets: %#v", got)
	}
}

func TestResolveNudgeTargets_DefaultAmbiguousWorkers(t *testing.T) {
	base := "ao-rpi-run12345-p3"
	sessions := []string{base + "-w1", base + "-w2"}

	_, err := resolveNudgeTargets(sessions, base, false, 0)
	if err == nil || !strings.Contains(err.Error(), "multiple worker sessions") {
		t.Fatalf("expected ambiguous worker error, got: %v", err)
	}
}

func TestResolveNudgeTargets_OneWorker(t *testing.T) {
	base := "ao-rpi-run12345-p1"
	sessions := []string{base, base + "-w1", base + "-w2"}

	got, err := resolveNudgeTargets(sessions, base, false, 2)
	if err != nil {
		t.Fatalf("resolveNudgeTargets(worker): %v", err)
	}
	if len(got) != 1 || got[0] != base+"-w2" {
		t.Fatalf("unexpected targets: %#v", got)
	}
}

func TestResolveNudgePhase(t *testing.T) {
	phase, err := resolveNudgePhase(&phasedState{Phase: 2}, 0)
	if err != nil {
		t.Fatalf("resolveNudgePhase(state): %v", err)
	}
	if phase != 2 {
		t.Fatalf("phase = %d, want 2", phase)
	}

	phase, err = resolveNudgePhase(&phasedState{Phase: 2}, 3)
	if err != nil {
		t.Fatalf("resolveNudgePhase(flag): %v", err)
	}
	if phase != 3 {
		t.Fatalf("phase = %d, want 3", phase)
	}

	if _, err := resolveNudgePhase(&phasedState{Phase: 2}, 9); err == nil {
		t.Fatal("expected error for invalid phase")
	}
}

func TestValidateRPINudgeTargetSelection(t *testing.T) {
	origAllWorkers := rpiNudgeAllWorkers
	origWorker := rpiNudgeWorker
	t.Cleanup(func() {
		rpiNudgeAllWorkers = origAllWorkers
		rpiNudgeWorker = origWorker
	})

	rpiNudgeAllWorkers = true
	rpiNudgeWorker = 1
	if err := validateRPINudgeTargetSelection(); err == nil {
		t.Fatal("expected selection conflict error")
	}

	rpiNudgeAllWorkers = false
	rpiNudgeWorker = 1
	if err := validateRPINudgeTargetSelection(); err != nil {
		t.Fatalf("expected worker-only selection to pass, got: %v", err)
	}
}

func TestResolveRPINudgeMessage(t *testing.T) {
	origMessage := rpiNudgeMessage
	t.Cleanup(func() {
		rpiNudgeMessage = origMessage
	})

	rpiNudgeMessage = "  from-flag  "
	got, err := resolveRPINudgeMessage([]string{"ignored", "args"})
	if err != nil {
		t.Fatalf("resolveRPINudgeMessage(flag): %v", err)
	}
	if got != "from-flag" {
		t.Fatalf("resolveRPINudgeMessage(flag) = %q, want %q", got, "from-flag")
	}

	rpiNudgeMessage = ""
	got, err = resolveRPINudgeMessage([]string{"  from", "args  "})
	if err != nil {
		t.Fatalf("resolveRPINudgeMessage(args): %v", err)
	}
	if got != "from args" {
		t.Fatalf("resolveRPINudgeMessage(args) = %q, want %q", got, "from args")
	}

	if _, err := resolveRPINudgeMessage(nil); err == nil {
		t.Fatal("expected empty message error")
	}
}

func TestAppendRPINudgeAudit(t *testing.T) {
	root := t.TempDir()
	runID := "audit-run-1"
	record := rpiNudgeRecord{
		Timestamp: "2026-02-26T00:00:00Z",
		RunID:     runID,
		Phase:     2,
		Targets:   []string{"ao-rpi-auditrun-p2-w1", "ao-rpi-auditrun-p2-w2"},
		Message:   "change direction",
	}

	if err := appendRPINudgeAudit(root, runID, record); err != nil {
		t.Fatalf("appendRPINudgeAudit: %v", err)
	}

	path := filepath.Join(rpiRunRegistryDir(root, runID), "nudges.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read nudges.jsonl: %v", err)
	}

	var got rpiNudgeRecord
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal audit record: %v", err)
	}
	if got.RunID != runID || got.Phase != 2 || got.Message != "change direction" {
		t.Fatalf("unexpected record: %#v", got)
	}
	if len(got.Targets) != 2 {
		t.Fatalf("unexpected targets: %#v", got.Targets)
	}
}

func TestRPINudgeCommand_E2EAllWorkers(t *testing.T) {
	fixture := setupRPINudgeE2EWorkers(t)

	chdirRPINudgeE2EWorkdir(t, fixture.tmp)

	if _, err := executeCommand(
		"rpi",
		"nudge",
		"--run-id", fixture.runID,
		"--phase", "1",
		"--all-workers",
		"--message", "change-direction now",
	); err != nil {
		t.Fatalf("execute ao rpi nudge: %v", err)
	}

	waitForRPINudgeE2ELog(
		t,
		fixture.logPath,
		10*time.Second,
		"did not observe nudges in both workers log",
		"in:1:change-direction now",
		"in:2:change-direction now",
	)
	assertRPINudgeE2EAudit(t, fixture.tmp, fixture.runID)
	stopRPINudgeE2EWorkers(fixture)
}

type rpiNudgeE2EFixture struct {
	tmuxBin string
	tmp     string
	runID   string
	phase   int
	worker1 string
	worker2 string
	logPath string
}

func setupRPINudgeE2EWorkers(t *testing.T) rpiNudgeE2EFixture {
	t.Helper()

	tmuxBin, err := defaultLookPath(nil)("tmux")
	if err != nil {
		t.Skipf("tmux not available: %v", err)
	}

	fixture := newRPINudgeE2EFixture(t, tmuxBin)
	t.Cleanup(func() {
		_ = exec.Command(fixture.tmuxBin, "kill-session", "-t", fixture.worker1).Run()
		_ = exec.Command(fixture.tmuxBin, "kill-session", "-t", fixture.worker2).Run()
	})

	writeRPINudgeE2EState(t, fixture.tmp, fixture.runID, fixture.phase)
	workerScriptPath := writeRPINudgeE2EWorkerScript(t, fixture.tmp)
	startRPINudgeE2EWorker(t, fixture, fixture.worker1, "1", workerScriptPath)
	startRPINudgeE2EWorker(t, fixture, fixture.worker2, "2", workerScriptPath)
	waitForRPINudgeE2ELog(t, fixture.logPath, 5*time.Second, "workers did not become ready", "ready:1", "ready:2")
	return fixture
}

func newRPINudgeE2EFixture(t *testing.T, tmuxBin string) rpiNudgeE2EFixture {
	t.Helper()

	tmp := t.TempDir()
	runID := "nudge-e2e-01"
	phase := 1
	baseSession := tmuxSessionName(runID, phase)
	return rpiNudgeE2EFixture{
		tmuxBin: tmuxBin,
		tmp:     tmp,
		runID:   runID,
		phase:   phase,
		worker1: baseSession + "-w1",
		worker2: baseSession + "-w2",
		logPath: filepath.Join(tmp, "nudge-e2e.log"),
	}
}

func writeRPINudgeE2EState(t *testing.T, tmp string, runID string, phase int) {
	t.Helper()

	statePath := filepath.Join(tmp, ".agents", "rpi", "runs", runID, phasedStateFile)
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	state := &phasedState{
		SchemaVersion: 1,
		Goal:          "test nudge command",
		Phase:         phase,
		RunID:         runID,
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	if err := os.WriteFile(statePath, data, 0o644); err != nil {
		t.Fatalf("write state: %v", err)
	}
}

func writeRPINudgeE2EWorkerScript(t *testing.T, tmp string) string {
	t.Helper()

	workerScriptPath := filepath.Join(tmp, "worker.sh")
	workerScript := `#!/usr/bin/env bash
set -euo pipefail
worker="$1"
log="$2"
echo "ready:${worker}" >> "$log"
while true; do
  if IFS= read -r line; then
    echo "in:${worker}:${line}" >> "$log"
    if [[ "$line" == *"stop-now"* ]]; then
      exit 0
    fi
  else
    sleep 0.05
  fi
done
`
	if err := os.WriteFile(workerScriptPath, []byte(workerScript), 0o700); err != nil {
		t.Fatalf("write worker script: %v", err)
	}
	return workerScriptPath
}

func startRPINudgeE2EWorker(t *testing.T, fixture rpiNudgeE2EFixture, workerSession string, workerID string, workerScriptPath string) {
	t.Helper()

	err := exec.Command(fixture.tmuxBin, "new-session", "-d", "-s", workerSession, "-c", fixture.tmp, workerScriptPath, workerID, fixture.logPath).Run()
	if err != nil {
		t.Fatalf("spawn worker%s: %v", workerID, err)
	}
}

func waitForRPINudgeE2ELog(t *testing.T, logPath string, timeout time.Duration, failure string, want ...string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for {
		raw, _ := os.ReadFile(logPath)
		log := string(raw)
		if rpiNudgeE2ELogContainsAll(log, want) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s:\n%s", failure, log)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func rpiNudgeE2ELogContainsAll(log string, want []string) bool {
	for _, entry := range want {
		if !strings.Contains(log, entry) {
			return false
		}
	}
	return true
}

func chdirRPINudgeE2EWorkdir(t *testing.T, tmp string) {
	t.Helper()

	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir tmp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldCwd)
	})
}

func assertRPINudgeE2EAudit(t *testing.T, tmp string, runID string) {
	t.Helper()

	commands, err := loadRPIC2Commands(tmp, runID)
	if err != nil {
		t.Fatalf("loadRPIC2Commands: %v", err)
	}
	if len(commands) != 1 {
		t.Fatalf("len(commands) = %d, want 1", len(commands))
	}
	commandID := commands[0].CommandID
	if commandID == "" {
		t.Fatal("expected non-empty command_id")
	}

	events, err := loadRPIC2Events(tmp, runID)
	if err != nil {
		t.Fatalf("loadRPIC2Events: %v", err)
	}
	ackCount := 0
	for _, ev := range events {
		if ev.CommandID != commandID {
			continue
		}
		if ev.Type == "command.nudge.ack" {
			ackCount++
		}
	}
	if ackCount < 2 {
		t.Fatalf("expected >=2 ack events for command %s, got %d", commandID, ackCount)
	}
}

func stopRPINudgeE2EWorkers(fixture rpiNudgeE2EFixture) {
	_ = sendTmuxNudge(fixture.tmuxBin, fixture.worker1, "stop-now")
	_ = sendTmuxNudge(fixture.tmuxBin, fixture.worker2, "stop-now")
}
