// practices: [agile-manifesto, dora-metrics]
package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestRPIVerifyCommandRegistered(t *testing.T) {
	for _, command := range rpiCmd.Commands() {
		if command.Name() == "verify" {
			return
		}
	}
	t.Fatal("expected rpi verify command to be registered under rpi")
}

func TestRPIVerifyPassText(t *testing.T) {
	cwd := chdirTemp(t)
	if _, err := appendRPILedgerEvent(cwd, rpiLedgerEvent{RunID: "run-pass", Phase: "discovery", Action: "started", Details: map[string]any{"ok": true}}); err != nil {
		t.Fatalf("append event: %v", err)
	}

	oldOutput := output
	output = "table"
	t.Cleanup(func() { output = oldOutput })

	stdout, err := captureStdout(t, func() error { return runRPIVerify(nil, nil) })
	if err != nil {
		t.Fatalf("runRPIVerify returned error on valid ledger: %v", err)
	}
	if !strings.Contains(stdout, "PASS records=1") {
		t.Fatalf("expected PASS output, got %q", stdout)
	}
}

func TestRPIVerifyFailText(t *testing.T) {
	cwd := chdirTemp(t)
	if _, err := appendRPILedgerEvent(cwd, rpiLedgerEvent{RunID: "run-fail", Phase: "implementation", Action: "started", Details: map[string]any{"ok": true}}); err != nil {
		t.Fatalf("append event: %v", err)
	}
	corruptRPILedger(t, cwd)

	oldOutput := output
	output = "table"
	t.Cleanup(func() { output = oldOutput })

	stdout, err := captureStdout(t, func() error { return runRPIVerify(nil, nil) })
	if err == nil {
		t.Fatal("expected verification failure after corruption")
	}
	if !strings.Contains(stdout, "FAIL records=1") {
		t.Fatalf("expected FAIL output, got %q", stdout)
	}
	if !strings.Contains(stdout, "first_broken_index=1") {
		t.Fatalf("expected first_broken_index in output, got %q", stdout)
	}
}

func TestRPIVerifyPassJSON(t *testing.T) {
	cwd := chdirTemp(t)
	if _, err := appendRPILedgerEvent(cwd, rpiLedgerEvent{RunID: "run-json", Phase: "validation", Action: "completed", Details: map[string]any{"ok": true}}); err != nil {
		t.Fatalf("append event: %v", err)
	}

	oldOutput := output
	output = "json"
	t.Cleanup(func() { output = oldOutput })

	stdout, err := captureStdout(t, func() error { return runRPIVerify(nil, nil) })
	if err != nil {
		t.Fatalf("runRPIVerify returned error on valid ledger: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("expected JSON output, decode failed: %v; output=%q", err, stdout)
	}
	if payload["status"] != "PASS" {
		t.Fatalf("expected status PASS, got %v", payload["status"])
	}
	if payload["pass"] != true {
		t.Fatalf("expected pass=true, got %v", payload["pass"])
	}
	if int(payload["record_count"].(float64)) != 1 {
		t.Fatalf("expected record_count=1, got %v", payload["record_count"])
	}
}

func TestRPIVerifyLatestFlagAccepted(t *testing.T) {
	cwd := chdirTemp(t)
	if _, err := appendRPILedgerEvent(cwd, rpiLedgerEvent{RunID: "run-latest", Phase: "validation", Action: "completed", Details: map[string]any{"ok": true}}); err != nil {
		t.Fatalf("append event: %v", err)
	}

	stdout, err := executeCommand("rpi", "verify", "--latest", "--json")
	if err != nil {
		t.Fatalf("rpi verify --latest --json returned error: %v; output=%q", err, stdout)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("expected JSON output, decode failed: %v; output=%q", err, stdout)
	}
	if payload["status"] != "PASS" {
		t.Fatalf("expected status PASS, got %v", payload["status"])
	}
}

// captureStdout moved to testutil_test.go.

func corruptRPILedger(t *testing.T, cwd string) {
	t.Helper()
	ledgerPath := RPILedgerPath(cwd)
	data, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		t.Fatal("expected at least one ledger line")
	}
	lines[0] = strings.Replace(lines[0], "\"action\":\"started\"", "\"action\":\"tampered\"", 1)
	if err := os.WriteFile(ledgerPath, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatalf("write tampered ledger: %v", err)
	}
}
