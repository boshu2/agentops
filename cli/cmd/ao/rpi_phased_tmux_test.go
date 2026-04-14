package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestTmuxExecutor_Name(t *testing.T) {
	exec := &tmuxExecutor{}
	if got := exec.Name(); got != "tmux" {
		t.Errorf("Name() = %q, want %q", got, "tmux")
	}
}

func TestTmuxSessionName_Format(t *testing.T) {
	got := tmuxSessionName("abcdef1234567890", 1)
	if got != "ao-rpi-abcdef12-p1" {
		t.Errorf("tmuxSessionName = %q", got)
	}
	got2 := tmuxSessionName("abc", 2)
	if got2 != "ao-rpi-abc-p2" {
		t.Errorf("tmuxSessionName(short) = %q", got2)
	}
}

func TestTmuxSessionName_ShellSafe(t *testing.T) {
	safePattern := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	for _, in := range []string{"run.id", "run id", "run@#$", "path/to/run"} {
		got := tmuxSessionName(in, 1)
		if !safePattern.MatchString(got) {
			t.Fatalf("unsafe session name: %q", got)
		}
	}
}

func TestTmuxRuntimeInvocationTemplate(t *testing.T) {
	exe, args, err := tmuxRuntimeInvocationTemplate("claude")
	if err != nil {
		t.Fatalf("tmuxRuntimeInvocationTemplate(claude): %v", err)
	}
	if exe != "claude" {
		t.Fatalf("exe = %q", exe)
	}
	if len(args) != 5 || args[0] != "-p" || args[1] != "--output-format" || args[2] != "stream-json" || args[3] != "--include-partial-messages" || args[4] != "--verbose" {
		t.Fatalf("args = %#v", args)
	}

	exe2, args2, err := tmuxRuntimeInvocationTemplate("codex")
	if err != nil {
		t.Fatalf("tmuxRuntimeInvocationTemplate(codex): %v", err)
	}
	if exe2 != "codex" {
		t.Fatalf("exe2 = %q", exe2)
	}
	if len(args2) != 2 || args2[0] != "exec" || args2[1] != "--json" {
		t.Fatalf("args2 = %#v", args2)
	}
}

func TestSelectExecutorFromCaps_Tmux(t *testing.T) {
	opts := defaultPhasedEngineOptions()
	opts.RuntimeMode = "tmux"
	opts.TmuxWorkers = 2
	exec, reason := selectExecutorFromCaps(backendCapabilities{RuntimeMode: "tmux"}, "", nil, opts)
	if exec.Name() != "tmux" {
		t.Fatalf("executor = %q", exec.Name())
	}
	if reason != "runtime=tmux" {
		t.Fatalf("reason = %q", reason)
	}
	tmux, ok := exec.(*tmuxExecutor)
	if !ok {
		t.Fatalf("expected *tmuxExecutor, got %T", exec)
	}
	if tmux.workerCount != 2 {
		t.Fatalf("workerCount = %d", tmux.workerCount)
	}
}

func TestValidateRuntimeMode_Tmux(t *testing.T) {
	for _, mode := range []string{"tmux", "TMUX", "auto", "direct", "stream"} {
		if err := validateRuntimeMode(mode); err != nil {
			t.Fatalf("validateRuntimeMode(%q): %v", mode, err)
		}
	}
	if err := validateRuntimeMode("bogus"); err == nil {
		t.Fatal("expected error for bogus mode")
	}
}

func TestTmuxHelpers_FilterWorkers(t *testing.T) {
	base := "ao-rpi-abcd1234-p2"
	sessions := []string{
		base,
		base + "-w1",
		base + "-w2",
		"other",
	}
	got := filterTmuxWorkerSessions(sessions, base)
	if len(got) != 2 {
		t.Fatalf("expected 2 workers, got %d", len(got))
	}
}

func TestTmuxExecutorE2ENudgeTwoWorkers(t *testing.T) {
	fixture := setupTmuxNudgeE2EFixture(t)
	done := startTmuxNudgeE2E(t, fixture)

	waitForTmuxNudgeWorkers(t, fixture)
	sendTmuxNudgeSequence(t, fixture)
	assertTmuxNudgeE2EComplete(t, done)
	assertTmuxNudgeLog(t, fixture)
}

type tmuxNudgeE2EFixture struct {
	tmuxBin  string
	tmpDir   string
	logPath  string
	runID    string
	workerA  string
	workerB  string
	executor *tmuxExecutor
}

func setupTmuxNudgeE2EFixture(t *testing.T) tmuxNudgeE2EFixture {
	t.Helper()

	tmuxBin, err := defaultLookPath(nil)("tmux")
	if err != nil {
		t.Skipf("tmux not available: %v", err)
	}

	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "tmux-workers.log")

	runtimePath := filepath.Join(tmp, "mock-runtime.sh")
	runtimeScript := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
log=%q
worker="${RPI_TMUX_WORKER_ID:-0}"
echo "start:${worker}" >> "$log"
while IFS= read -r line; do
  echo "in:${worker}:${line}" >> "$log"
  if [[ "$line" == *"change-direction"* ]]; then
    echo "nudged:${worker}" >> "$log"
  fi
  if [[ "$line" == *"complete-now"* ]]; then
    exit 0
  fi
done
`, logPath)
	if err := os.WriteFile(runtimePath, []byte(runtimeScript), 0o700); err != nil {
		t.Fatalf("write runtime script: %v", err)
	}

	runID := "nudge1234"
	baseSession := tmuxSessionName(runID, 1)
	return tmuxNudgeE2EFixture{
		tmuxBin: tmuxBin,
		tmpDir:  tmp,
		logPath: logPath,
		runID:   runID,
		workerA: baseSession + "-w1",
		workerB: baseSession + "-w2",
		executor: &tmuxExecutor{
			tmuxCommand:    tmuxBin,
			runtimeCommand: runtimePath,
			phaseTimeout:   20 * time.Second,
			pollInterval:   200 * time.Millisecond,
			workerCount:    2,
		},
	}
}

func startTmuxNudgeE2E(t *testing.T, fixture tmuxNudgeE2EFixture) <-chan error {
	t.Helper()

	done := make(chan error, 1)
	go func() {
		done <- fixture.executor.Execute(context.Background(), "test prompt", fixture.tmpDir, fixture.runID, 1)
	}()
	return done
}

func waitForTmuxNudgeWorkers(t *testing.T, fixture tmuxNudgeE2EFixture) {
	t.Helper()

	deadline := time.Now().Add(8 * time.Second)
	for {
		sessions, err := listTmuxSessions(fixture.tmuxBin)
		if err == nil && tmuxNudgeWorkersReady(sessions, fixture) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("workers not ready before deadline")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func tmuxNudgeWorkersReady(sessions []string, fixture tmuxNudgeE2EFixture) bool {
	seen := map[string]bool{}
	for _, session := range sessions {
		if session == fixture.workerA || session == fixture.workerB {
			seen[session] = true
		}
	}
	return seen[fixture.workerA] && seen[fixture.workerB]
}

func sendTmuxNudgeSequence(t *testing.T, fixture tmuxNudgeE2EFixture) {
	t.Helper()

	nudges := []struct {
		label   string
		worker  string
		message string
	}{
		{"nudge workerA", fixture.workerA, "change-direction worker-a"},
		{"nudge workerB", fixture.workerB, "change-direction worker-b"},
		{"complete workerA", fixture.workerA, "complete-now"},
		{"complete workerB", fixture.workerB, "complete-now"},
	}
	for _, nudge := range nudges {
		if err := sendTmuxNudge(fixture.tmuxBin, nudge.worker, nudge.message); err != nil {
			t.Fatalf("%s: %v", nudge.label, err)
		}
	}
}

func assertTmuxNudgeE2EComplete(t *testing.T, done <-chan error) {
	t.Helper()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("tmux executor failed: %v", err)
		}
	case <-time.After(12 * time.Second):
		t.Fatal("timeout waiting for tmux executor completion")
	}
}

func assertTmuxNudgeLog(t *testing.T, fixture tmuxNudgeE2EFixture) {
	t.Helper()

	data, err := os.ReadFile(fixture.logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	log := string(data)
	if !strings.Contains(log, "nudged:1") {
		t.Fatalf("missing nudge confirmation for worker 1:\n%s", log)
	}
	if !strings.Contains(log, "nudged:2") {
		t.Fatalf("missing nudge confirmation for worker 2:\n%s", log)
	}
}

func TestTmuxExecutorImplementsPhaseExecutor(t *testing.T) {
	var _ PhaseExecutor = &tmuxExecutor{}
}

func TestTmuxSanitizeName(t *testing.T) {
	safePattern := regexp.MustCompile(`^[a-zA-Z0-9_-]*$`)
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"clean input", "ao-rpi-abc123-p1", "ao-rpi-abc123-p1"},
		{"dots removed", "ao.rpi.abc.p1", "aorpiabcp1"},
		{"spaces removed", "ao rpi abc p1", "aorpiabcp1"},
		{"special chars", "run@#$%^&*!", "run"},
		{"slashes removed", "path/to/run", "pathtorun"},
		{"underscores preserved", "ao_rpi_test", "ao_rpi_test"},
		{"empty string", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tmuxSanitizeName(tt.input)
			if got != tt.want {
				t.Errorf("tmuxSanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
			if !safePattern.MatchString(got) {
				t.Errorf("tmuxSanitizeName(%q) = %q, contains unsafe chars", tt.input, got)
			}
		})
	}
}

func TestSanitizeSessionName_SafeForTmux(t *testing.T) {
	// tmuxSessionName should always produce tmux-safe names
	safePattern := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	inputs := []struct {
		runID    string
		phaseNum int
	}{
		{"normal-run-id", 1},
		{"rpi-abcdef01", 2},
		{"has spaces bad", 3},
		{"special!@#$%", 1},
		{"path/traversal/../id", 2},
		{"dots.in.name", 1},
		{"very-long-run-id-that-exceeds-eight-chars", 3},
	}
	for _, tt := range inputs {
		name := fmt.Sprintf("%s-p%d", tt.runID, tt.phaseNum)
		t.Run(name, func(t *testing.T) {
			got := tmuxSessionName(tt.runID, tt.phaseNum)
			if got == "" {
				t.Error("tmuxSessionName returned empty string")
			}
			if !safePattern.MatchString(got) {
				t.Errorf("tmuxSessionName(%q, %d) = %q, not tmux-safe", tt.runID, tt.phaseNum, got)
			}
		})
	}
}

func TestTmuxExecutor_MissingExitFileEmitsFailureEvent(t *testing.T) {
	tmp := t.TempDir()
	tmuxPath := filepath.Join(tmp, "fake-tmux.sh")
	fakeTmux := `#!/usr/bin/env bash
set -euo pipefail
cmd="${1:-}"
case "$cmd" in
  new-session) exit 0 ;;
  has-session) exit 1 ;;
  kill-session) exit 0 ;;
  *) exit 0 ;;
esac
`
	if err := os.WriteFile(tmuxPath, []byte(fakeTmux), 0o700); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}

	execTmux := &tmuxExecutor{
		tmuxCommand:    tmuxPath,
		runtimeCommand: "echo",
		phaseTimeout:   2 * time.Second,
		pollInterval:   50 * time.Millisecond,
		workerCount:    1,
	}
	runID := "exit-missing-run"
	err := execTmux.Execute(context.Background(), "prompt", tmp, runID, 1)
	if err == nil {
		t.Fatal("expected missing exit-code file error")
	}
	if !strings.Contains(err.Error(), "exit-code file") {
		t.Fatalf("expected exit-code file error, got: %v", err)
	}

	events, loadErr := loadRPIC2Events(tmp, runID)
	if loadErr != nil {
		t.Fatalf("loadRPIC2Events: %v", loadErr)
	}
	foundFailure := false
	for _, ev := range events {
		if ev.Type == "phase.tmux.failed" && strings.Contains(ev.Message, "exit-code file") {
			foundFailure = true
			break
		}
	}
	if !foundFailure {
		t.Fatalf("expected phase.tmux.failed event with exit-code file message, events=%#v", events)
	}
}
