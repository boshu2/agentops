package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureHookStdout(t *testing.T, fn func() error) string {
	t.Helper()
	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = writer
	runErr := fn()
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatalf("close writer: %v", closeErr)
	}
	os.Stdout = original
	data, readErr := io.ReadAll(reader)
	if readErr != nil {
		t.Fatalf("read stdout: %v", readErr)
	}
	if runErr != nil {
		t.Fatalf("hook returned error: %v", runErr)
	}
	return string(data)
}

func TestHooksRunRedactsSensitiveDiff(t *testing.T) {
	diff := "API_TOKEN=not-a-secret-fixture\nAuthorization: Bearer plain-text"
	redacted := redactSensitiveDiff(diff)
	if strings.Contains(redacted, "not-a-secret-fixture") || strings.Contains(redacted, "plain-text") {
		t.Fatalf("diff was not redacted: %q", redacted)
	}
	if !strings.Contains(redacted, "API_TOKEN=[REDACTED]") || !strings.Contains(redacted, "Authorization: Bearer [REDACTED]") {
		t.Fatalf("redacted markers missing: %q", redacted)
	}
}

func TestHooksRunRatchetAdvanceFallback(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)
	t.Setenv("PATH", "/usr/bin:/bin")
	t.Setenv("AGENTOPS_HOOKS_DISABLED", "0")
	t.Setenv("AGENTOPS_AUTOCHAIN", "")

	payload := []byte(`{"tool_input":{"command":"ao ratchet record research"},"tool_response":{"exit_code":0}}`)
	output := captureHookStdout(t, func() error {
		return runRatchetAdvanceHook(payload)
	})

	if !strings.Contains(output, "Suggested next skill: plan") {
		t.Fatalf("ratchet output = %q, want plan suggestion", output)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("output is not JSON: %v; %q", err, output)
	}
	if _, err := os.Stat(filepath.Join(tmp, ".agents", "ao", ".ratchet-advance-fired")); err != nil {
		t.Fatalf("dedup flag not written: %v", err)
	}
}

func TestHooksRunQualitySignalsWritesFingerprintOnly(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)
	t.Setenv("CLAUDE_SESSION_ID", "test-quality")
	t.Setenv("AGENTOPS_HOOKS_DISABLED", "0")
	t.Setenv("AGENTOPS_QUALITY_SIGNALS_DISABLED", "")

	payload := []byte(`{"prompt":"private prompt fixture"}`)
	if err := runQualitySignalsHook(payload); err != nil {
		t.Fatalf("first quality run: %v", err)
	}
	if err := runQualitySignalsHook(payload); err != nil {
		t.Fatalf("second quality run: %v", err)
	}

	fingerprint, err := os.ReadFile(filepath.Join(tmp, ".agents", "ao", ".last-prompt"))
	if err != nil {
		t.Fatalf("read fingerprint: %v", err)
	}
	if strings.Contains(string(fingerprint), "private prompt fixture") {
		t.Fatalf("raw prompt leaked into fingerprint file: %q", string(fingerprint))
	}
	logData, err := os.ReadFile(filepath.Join(tmp, ".agents", "signals", "session-quality.jsonl"))
	if err != nil {
		t.Fatalf("read signal log: %v", err)
	}
	if !strings.Contains(string(logData), `"signal_type":"repeated_prompt"`) {
		t.Fatalf("repeated prompt signal missing: %q", string(logData))
	}
}
