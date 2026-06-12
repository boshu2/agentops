package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexDispatchRefusesAPIKeyAuthBeforeWorkerExecution(t *testing.T) {
	repo := newCodexDispatchRepo(t)
	marker := filepath.Join(repo, "worker-ran")
	writeFakeCodexBinary(t)
	t.Setenv("FAKE_CODEX_MARKER", marker)
	t.Setenv("OPENAI_API_KEY", "sk-test")

	packetPath, receiptPath := writeCodexDispatchPacket(t, repo, codexDispatchPacketOptions{})
	_, err := executeCommand("codex", "dispatch", "--packet", packetPath, "--json")
	if err == nil {
		t.Fatalf("codex dispatch succeeded, want unsafe auth failure")
	}
	if !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("dispatch error = %q, want OPENAI_API_KEY refusal", err.Error())
	}
	assertPathAbsent(t, marker)
	assertPathAbsent(t, receiptPath)
}

func TestCodexDispatchRequiresChatGPTStatus(t *testing.T) {
	repo := newCodexDispatchRepo(t)
	marker := filepath.Join(repo, "worker-ran")
	writeFakeCodexBinary(t)
	t.Setenv("FAKE_CODEX_MARKER", marker)
	t.Setenv("FAKE_CODEX_LOGIN_STATUS", "Logged in using API key")
	t.Setenv("OPENAI_API_KEY", "")

	packetPath, receiptPath := writeCodexDispatchPacket(t, repo, codexDispatchPacketOptions{})
	_, err := executeCommand("codex", "dispatch", "--packet", packetPath, "--json")
	if err == nil {
		t.Fatalf("codex dispatch succeeded, want ChatGPT auth failure")
	}
	if !strings.Contains(err.Error(), "ChatGPT subscription auth") {
		t.Fatalf("dispatch error = %q, want ChatGPT subscription auth failure", err.Error())
	}
	assertPathAbsent(t, marker)
	assertPathAbsent(t, receiptPath)
}

func TestCodexDispatchWritesReceiptAndClosesStdin(t *testing.T) {
	repo := newCodexDispatchRepo(t)
	promptCapture := filepath.Join(repo, "prompt-capture.txt")
	writeFakeCodexBinary(t)
	t.Setenv("FAKE_CODEX_PROMPT_CAPTURE", promptCapture)
	t.Setenv("FAKE_CODEX_FINAL_MESSAGE", "VERDICT: PASS\nCOMMANDS RUN:\n- fake codex exec")
	t.Setenv("FAKE_CODEX_STDOUT", "{\"event\":\"ok\"}")
	t.Setenv("OPENAI_API_KEY", "")

	packetPath, receiptPath := writeCodexDispatchPacket(t, repo, codexDispatchPacketOptions{})
	out, err := executeCommand("codex", "dispatch", "--packet", packetPath, "--json")
	if err != nil {
		t.Fatalf("codex dispatch returned error: %v\noutput:\n%s", err, out)
	}

	var printed codexRunReceipt
	if err := json.Unmarshal([]byte(out), &printed); err != nil {
		t.Fatalf("unmarshal printed receipt: %v\noutput:\n%s", err, out)
	}
	fileReceipt := readCodexDispatchReceipt(t, receiptPath)
	if fileReceipt.PacketID != printed.PacketID {
		t.Fatalf("receipt file packet_id = %q, printed packet_id = %q", fileReceipt.PacketID, printed.PacketID)
	}
	if fileReceipt.AuthMode != "chatgpt-subscription" {
		t.Fatalf("auth_mode = %q, want chatgpt-subscription", fileReceipt.AuthMode)
	}
	if fileReceipt.AuthStatus != "Logged in using ChatGPT" {
		t.Fatalf("auth_status = %q, want Logged in using ChatGPT", fileReceipt.AuthStatus)
	}
	if fileReceipt.Stdin.Mode != "pipe-prompt" {
		t.Fatalf("stdin.mode = %q, want pipe-prompt", fileReceipt.Stdin.Mode)
	}
	if fileReceipt.Stdin.BytesWritten == 0 || fileReceipt.Stdin.ClosedAt == "" {
		t.Fatalf("stdin receipt did not record closed prompt bytes: %+v", fileReceipt.Stdin)
	}
	if fileReceipt.ExitCode != 0 || fileReceipt.TimedOut {
		t.Fatalf("unexpected run status: exit=%d timed_out=%v", fileReceipt.ExitCode, fileReceipt.TimedOut)
	}
	if fileReceipt.Verdict.Status != "PASS" {
		t.Fatalf("verdict.status = %q, want PASS", fileReceipt.Verdict.Status)
	}
	assertFileContains(t, promptCapture, "dispatch acceptance prompt")
	assertFileContains(t, filepath.Join(repo, fileReceipt.Outputs.FinalMessagePath), "VERDICT: PASS")
	assertFileContains(t, filepath.Join(repo, fileReceipt.Outputs.JSONLPath), "{\"event\":\"ok\"}")
	assertPathAbsent(t, filepath.Join(repo, ".agents", "ao", "codex", "state.json"))
}

func TestCodexDispatchTimeoutWritesReceipt(t *testing.T) {
	repo := newCodexDispatchRepo(t)
	writeFakeCodexBinary(t)
	t.Setenv("FAKE_CODEX_SLEEP_SECONDS", "2")
	t.Setenv("OPENAI_API_KEY", "")

	packetPath, receiptPath := writeCodexDispatchPacket(t, repo, codexDispatchPacketOptions{TimeoutSeconds: 1})
	out, err := executeCommand("codex", "dispatch", "--packet", packetPath, "--json")
	if err == nil {
		t.Fatalf("codex dispatch succeeded, want timeout failure")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("dispatch error = %q, want timeout", err.Error())
	}
	if strings.TrimSpace(out) == "" {
		t.Fatalf("timeout did not print receipt JSON")
	}
	receipt := readCodexDispatchReceipt(t, receiptPath)
	if !receipt.TimedOut {
		t.Fatalf("timed_out = false, want true")
	}
	if receipt.ExitCode != -1 {
		t.Fatalf("exit_code = %d, want -1", receipt.ExitCode)
	}
	if receipt.Verdict.Status != "ERROR" {
		t.Fatalf("verdict.status = %q, want ERROR", receipt.Verdict.Status)
	}
	if receipt.FailureReason == "" {
		t.Fatalf("failure_reason is empty")
	}
}

type codexDispatchPacketOptions struct {
	TimeoutSeconds int
}

func writeCodexDispatchPacket(t *testing.T, repo string, opts codexDispatchPacketOptions) (packetPath string, receiptPath string) {
	t.Helper()
	if opts.TimeoutSeconds == 0 {
		opts.TimeoutSeconds = 30
	}
	runDir := filepath.Join(".agents", "codex", "runs", "dispatch-test")
	taskDir := filepath.Join(".agents", "codex", "tasks", "dispatch-test")
	promptPath := filepath.Join(taskDir, "prompt.md")
	finalPath := filepath.Join(runDir, "final.md")
	jsonlPath := filepath.Join(runDir, "events.jsonl")
	receiptRelPath := filepath.Join(runDir, "receipt.json")
	packetRelPath := filepath.Join(taskDir, "packet.json")
	argv := []string{
		"codex",
		"exec",
		"--json",
		"--sandbox",
		"workspace-write",
		"--output-last-message",
		finalPath,
	}

	if err := os.MkdirAll(filepath.Join(repo, taskDir), 0o750); err != nil {
		t.Fatalf("create task dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, promptPath), []byte("dispatch acceptance prompt\n"), 0o600); err != nil {
		t.Fatalf("write prompt: %v", err)
	}

	packet := codexTaskPacket{
		SchemaVersion: 1,
		PacketID:      "codex-dispatch-test",
		CreatedAt:     "2026-06-12T16:00:00Z",
		Objective:     "Run Codex dispatch acceptance test.",
		Role:          "worker",
		CWD:           repo,
		Sandbox:       "workspace-write",
		Auth: codexTaskAuthGuard{
			RequiredMode:           "chatgpt-subscription",
			RejectEnv:              []string{"OPENAI_API_KEY"},
			LoginStatusMustContain: "Logged in using ChatGPT",
			ForbidAPIKey:           true,
		},
		Dispatch: codexTaskDispatchPolicy{
			Mode:        "non-mutating",
			MutatesRepo: false,
			Command:     append([]string(nil), argv...),
		},
		Execution: codexTaskExecution{
			Argv:           append([]string(nil), argv...),
			Stdin:          codexTaskStdin{Mode: "pipe-prompt", CloseAfterPrompt: true},
			TimeoutSeconds: opts.TimeoutSeconds,
			PromptPath:     promptPath,
		},
		Output: codexTaskOutputContract{
			CaptureMode:      "output-last-message",
			FinalMessagePath: finalPath,
			JSONLPath:        jsonlPath,
			ReceiptPath:      receiptRelPath,
		},
		Evidence: codexTaskEvidenceContract{
			ReceiptPath:      receiptRelPath,
			RequiredCommands: []string{"codex exec"},
			Artifacts:        []string{finalPath, jsonlPath},
		},
		StopCondition: "Receipt captures output, stdin, timeout, and verdict.",
	}
	data, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		t.Fatalf("marshal packet: %v", err)
	}
	packetAbsPath := filepath.Join(repo, packetRelPath)
	if err := os.WriteFile(packetAbsPath, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write packet: %v", err)
	}
	return packetAbsPath, filepath.Join(repo, receiptRelPath)
}

func writeFakeCodexBinary(t *testing.T) string {
	t.Helper()
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		t.Fatalf("create fake bin dir: %v", err)
	}
	path := filepath.Join(binDir, "codex")
	script := `#!/usr/bin/env bash
set -euo pipefail

if [[ "${1:-}" == "login" && "${2:-}" == "status" ]]; then
  printf '%s\n' "${FAKE_CODEX_LOGIN_STATUS:-Logged in using ChatGPT}"
  exit "${FAKE_CODEX_LOGIN_EXIT:-0}"
fi

if [[ "${1:-}" == "exec" ]]; then
  if [[ -n "${FAKE_CODEX_MARKER:-}" ]]; then
    printf 'ran\n' > "$FAKE_CODEX_MARKER"
  fi

  final=""
  for ((i=1; i<=$#; i++)); do
    if [[ "${!i}" == "--output-last-message" ]]; then
      j=$((i + 1))
      final="${!j:-}"
    fi
  done

  prompt="$(cat)"
  if [[ -n "${FAKE_CODEX_PROMPT_CAPTURE:-}" ]]; then
    printf '%s' "$prompt" > "$FAKE_CODEX_PROMPT_CAPTURE"
  fi

  if [[ -n "${FAKE_CODEX_SLEEP_SECONDS:-}" ]]; then
    sleep "$FAKE_CODEX_SLEEP_SECONDS"
  fi

  if [[ -n "$final" ]]; then
    mkdir -p "$(dirname "$final")"
    printf '%s\n' "${FAKE_CODEX_FINAL_MESSAGE:-VERDICT: PASS}" > "$final"
  fi

  printf '%s\n' "${FAKE_CODEX_STDOUT:-{\"event\":\"ok\"}}"
  exit "${FAKE_CODEX_EXIT_CODE:-0}"
fi

printf 'unexpected fake codex args: %s\n' "$*" >&2
exit 64
`
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return path
}

func newCodexDispatchRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runCodexDispatchGit(t, repo, "init")
	runCodexDispatchGit(t, repo, "config", "user.email", "test@example.com")
	runCodexDispatchGit(t, repo, "config", "user.name", "Test User")
	runCodexDispatchGit(t, repo, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("# fixture\n"), 0o600); err != nil {
		t.Fatalf("write readme: %v", err)
	}
	runCodexDispatchGit(t, repo, "add", "README.md")
	runCodexDispatchGit(t, repo, "commit", "-m", "init")
	return repo
}

func runCodexDispatchGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
}

func readCodexDispatchReceipt(t *testing.T, path string) codexRunReceipt {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read receipt: %v", err)
	}
	var receipt codexRunReceipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		t.Fatalf("unmarshal receipt: %v\n%s", err, string(data))
	}
	return receipt
}

func assertFileContains(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(data), want) {
		t.Fatalf("%s = %q, want to contain %q", path, string(data), want)
	}
}

func assertPathAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("%s exists, want absent", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
}
