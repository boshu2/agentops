// practices: [agile-manifesto, dora-metrics]
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// tmuxExecutor implements PhaseExecutor by spawning a detached mayor tmux
// session that orchestrates worker tmux sessions for a single phase.
type tmuxExecutor struct {
	tmuxCommand    string
	runtimeCommand string
	phaseTimeout   time.Duration
	pollInterval   time.Duration
	workerCount    int
}

func (t *tmuxExecutor) Name() string { return "tmux" }

func (t *tmuxExecutor) Execute(_ context.Context, prompt, cwd, runID string, phaseNum int) error {
	tmuxBin, err := defaultLookPath(nil)(t.tmuxCommand)
	if err != nil {
		return fmt.Errorf("tmux binary %q not found: %w", t.tmuxCommand, err)
	}

	sessionName := tmuxSessionName(runID, phaseNum)
	promptPath, err := t.writePromptFile(cwd, runID, phaseNum, prompt)
	if err != nil {
		return fmt.Errorf("write prompt file: %w", err)
	}
	exitCodePath := tmuxExitCodePath(cwd, runID, phaseNum)

	runtimeExe, runtimeArgs, err := tmuxRuntimeInvocationTemplate(t.runtimeCommand)
	if err != nil {
		return err
	}
	workerScript, err := t.writeWorkerScript(cwd, runID, phaseNum)
	if err != nil {
		return fmt.Errorf("write worker script: %w", err)
	}
	mayorScript, err := t.writeMayorScript(cwd, runID, phaseNum)
	if err != nil {
		return fmt.Errorf("write mayor script: %w", err)
	}

	workers := t.effectiveWorkerCount()
	args := []string{
		"new-session", "-d",
		"-s", sessionName,
		"-c", cwd,
		mayorScript,
		tmuxBin,
		sessionName,
		cwd,
		workerScript,
		promptPath,
		exitCodePath,
		strconv.Itoa(workers),
		runtimeExe,
	}
	args = append(args, runtimeArgs...)

	cmd := exec.Command(tmuxBin, args...)
	cmd.Env = cleanEnvNoClaude()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("spawn mayor tmux session %q: %w", sessionName, err)
	}
	if _, err := appendRPIC2Event(cwd, rpiC2EventInput{
		RunID:   runID,
		Phase:   phaseNum,
		Backend: "tmux",
		Source:  "runtime_tmux",
		Type:    "phase.tmux.started",
		Message: fmt.Sprintf("tmux mayor session %q started with %d worker(s)", sessionName, workers),
		Details: map[string]any{
			"session": sessionName,
			"workers": workers,
		},
	}); err != nil {
		VerbosePrintf("Warning: could not append tmux start event: %v\n", err)
	}

	fmt.Printf("Tmux mayor session %q spawned with %d worker(s) for phase %d\n", sessionName, workers, phaseNum)

	ctx, cancel := context.WithTimeout(context.Background(), t.phaseTimeout)
	defer cancel()

	waitErr := t.waitForCompletion(ctx, tmuxBin, sessionName, exitCodePath)
	for i := 1; i <= workers; i++ {
		logPath := fmt.Sprintf("%s.w%d.jsonl", exitCodePath, i)
		if err := appendRPIC2WorkerLogEvents(cwd, runID, phaseNum, "tmux", strconv.Itoa(i), logPath); err != nil {
			VerbosePrintf("Warning: could not append tmux worker events from %s: %v\n", logPath, err)
		}
	}
	eventType := "phase.tmux.completed"
	eventMessage := fmt.Sprintf("tmux phase %d completed", phaseNum)
	if waitErr != nil {
		eventType = "phase.tmux.failed"
		eventMessage = waitErr.Error()
	}
	if _, err := appendRPIC2Event(cwd, rpiC2EventInput{
		RunID:   runID,
		Phase:   phaseNum,
		Backend: "tmux",
		Source:  "runtime_tmux",
		Type:    eventType,
		Message: eventMessage,
		Details: map[string]any{
			"session": sessionName,
		},
	}); err != nil {
		VerbosePrintf("Warning: could not append tmux completion event: %v\n", err)
	}

	t.killWorkerSessions(tmuxBin, sessionName, workers)
	t.killSession(tmuxBin, sessionName)
	return waitErr
}

func (t *tmuxExecutor) waitForCompletion(ctx context.Context, tmuxBin, sessionName, exitCodePath string) error {
	ticker := time.NewTicker(t.effectivePollInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("tmux phase timed out after %v (session %s)", t.phaseTimeout, sessionName)
		case <-ticker.C:
			if tmuxHasSession(tmuxBin, sessionName) {
				continue
			}
			return t.readExitCode(exitCodePath, sessionName)
		}
	}
}

func (t *tmuxExecutor) readExitCode(exitCodePath, sessionName string) error {
	for range 4 {
		data, err := os.ReadFile(exitCodePath)
		if err == nil {
			code := strings.TrimSpace(string(data))
			if code == "0" {
				return nil
			}
			return fmt.Errorf("tmux phase session %q exited with code %s", sessionName, code)
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("tmux session %q exited but no exit-code file found at %s", sessionName, exitCodePath)
}

func (t *tmuxExecutor) writePromptFile(cwd, runID string, phaseNum int, prompt string) (string, error) {
	dir := rpiRunRegistryDir(cwd, runID)
	if dir == "" {
		dir = filepath.Join(cwd, ".agents", "rpi", "runs", "scratch")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("phase-%d-prompt.txt", phaseNum))
	return path, os.WriteFile(path, []byte(prompt), 0o600)
}

func (t *tmuxExecutor) writeWorkerScript(cwd, runID string, phaseNum int) (string, error) {
	dir := rpiRunRegistryDir(cwd, runID)
	if dir == "" {
		dir = filepath.Join(cwd, ".agents", "rpi", "runs", "scratch")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("phase-%d-worker.sh", phaseNum))
	content := `#!/usr/bin/env bash
set -euo pipefail
prompt_file="$1"
exit_file="$2"
output_file="$3"
runtime_exe="$4"
shift 4
prompt="$(cat "$prompt_file")"
mkdir -p "$(dirname "$output_file")"
printf '{"type":"rpi_worker_start","timestamp":"%s","worker":"%s"}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "${RPI_TMUX_WORKER_ID:-0}" >> "$output_file"
set +e
"$runtime_exe" "$@" "$prompt" 2>&1 | tee -a "$output_file"
code=${PIPESTATUS[0]}
set -e
printf '{"type":"rpi_worker_end","timestamp":"%s","worker":"%s","exit_code":%d}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "${RPI_TMUX_WORKER_ID:-0}" "$code" >> "$output_file"
printf "%s\n" "$code" > "$exit_file"
exit "$code"
`
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil { // #nosec G306
		return "", err
	}
	return path, nil
}

func (t *tmuxExecutor) writeMayorScript(cwd, runID string, phaseNum int) (string, error) {
	dir := rpiRunRegistryDir(cwd, runID)
	if dir == "" {
		dir = filepath.Join(cwd, ".agents", "rpi", "runs", "scratch")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("phase-%d-mayor.sh", phaseNum))
	content := `#!/usr/bin/env bash
set -euo pipefail
tmux_bin="$1"
session_base="$2"
cwd="$3"
worker_script="$4"
prompt_file="$5"
exit_base="$6"
workers="$7"
runtime_exe="$8"
shift 8
runtime_args=("$@")

for i in $(seq 1 "$workers"); do
  worker_session="${session_base}-w${i}"
  worker_exit="${exit_base}.w${i}"
  worker_log="${exit_base}.w${i}.jsonl"
  "$tmux_bin" new-session -d -s "$worker_session" -c "$cwd" env RPI_TMUX_WORKER_ID="$i" RPI_TMUX_WORKER_LOG="$worker_log" "$worker_script" "$prompt_file" "$worker_exit" "$worker_log" "$runtime_exe" "${runtime_args[@]}"
done

while true; do
  alive=0
  for i in $(seq 1 "$workers"); do
    if "$tmux_bin" has-session -t "${session_base}-w${i}" >/dev/null 2>&1; then
      alive=1
      break
    fi
  done
  if [[ "$alive" -eq 0 ]]; then
    break
  fi
  sleep 1
done

overall=0
for i in $(seq 1 "$workers"); do
  worker_exit="${exit_base}.w${i}"
  code="$(cat "$worker_exit" 2>/dev/null || echo 1)"
  if [[ "$code" != "0" ]]; then
    overall="$code"
  fi
done

printf "%s\n" "$overall" > "$exit_base"
exit "$overall"
`
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil { // #nosec G306
		return "", err
	}
	return path, nil
}

func (t *tmuxExecutor) killSession(tmuxBin, sessionName string) {
	cmd := exec.Command(tmuxBin, "kill-session", "-t", sessionName)
	if err := cmd.Run(); err != nil {
		VerbosePrintf("tmux kill-session %s: %v (may already be dead)\n", sessionName, err)
	}
}

func (t *tmuxExecutor) killWorkerSessions(tmuxBin, sessionBase string, workers int) {
	for i := 1; i <= workers; i++ {
		session := fmt.Sprintf("%s-w%d", sessionBase, i)
		cmd := exec.Command(tmuxBin, "kill-session", "-t", session)
		_ = cmd.Run()
	}
}

func (t *tmuxExecutor) effectivePollInterval() time.Duration {
	if t.pollInterval > 0 {
		return t.pollInterval
	}
	return 5 * time.Second
}

func (t *tmuxExecutor) effectiveWorkerCount() int {
	if t.workerCount <= 0 {
		return 1
	}
	return t.workerCount
}

func tmuxRuntimeInvocationTemplate(command string) (string, []string, error) {
	cmd := effectiveRuntimeCommand(command)
	executable, prefixArgs := splitRuntimeCommand(cmd)
	if executable == "" {
		return "", nil, fmt.Errorf("runtime command is empty")
	}
	switch runtimeBinaryName(cmd) {
	case "codex":
		// Emit JSONL events so leads can tail/filter tool-call streams.
		return executable, append(prefixArgs, "exec", "--json"), nil
	case "claude":
		// stream-json includes structured events (messages, tool calls, deltas).
		return executable, append(prefixArgs, "-p", "--output-format", "stream-json", "--include-partial-messages", "--verbose"), nil
	default:
		return executable, append(prefixArgs, "-p"), nil
	}
}

func tmuxSessionName(runID string, phaseNum int) string {
	id := runID
	if len(id) > 8 {
		id = id[:8]
	}
	name := fmt.Sprintf("ao-rpi-%s-p%d", id, phaseNum)
	return tmuxSanitizeName(name)
}

func tmuxExitCodePath(cwd, runID string, phaseNum int) string {
	dir := rpiRunRegistryDir(cwd, runID)
	if dir == "" {
		dir = filepath.Join(cwd, ".agents", "rpi", "runs", "scratch")
	}
	return filepath.Join(dir, fmt.Sprintf("phase-%d-exit", phaseNum))
}

var shellSafeRe = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func tmuxSanitizeName(name string) string {
	return shellSafeRe.ReplaceAllString(name, "")
}

func tmuxHasSession(tmuxBin, sessionName string) bool {
	cmd := exec.Command(tmuxBin, "has-session", "-t", sessionName)
	return cmd.Run() == nil
}

func listTmuxSessions(tmuxBin string) ([]string, error) {
	cmd := exec.Command(tmuxBin, "list-sessions", "-F", "#{session_name}")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list tmux sessions: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var sessions []string
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if s != "" {
			sessions = append(sessions, s)
		}
	}
	return sessions, nil
}

func filterTmuxWorkerSessions(sessions []string, phaseSessionBase string) []string {
	var out []string
	prefix := phaseSessionBase + "-w"
	for _, s := range sessions {
		if strings.HasPrefix(s, prefix) {
			out = append(out, s)
		}
	}
	return out
}

func sendTmuxNudge(tmuxBin, sessionName, message string) error {
	cmd := exec.Command(tmuxBin, "send-keys", "-t", sessionName, message, "C-m")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("send nudge to %s: %w", sessionName, err)
	}
	return nil
}
