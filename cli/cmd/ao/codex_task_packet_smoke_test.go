//go:build legacy

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexTaskPacketSmokeFixtureWritesSchemaValidReceipt(t *testing.T) {
	repo := newCodexDispatchRepo(t)
	writeFakeCodexBinary(t)
	t.Setenv("FAKE_CODEX_FINAL_MESSAGE", validCodexFinalVerdictForTest("PASS"))
	t.Setenv("FAKE_CODEX_STDOUT", "{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\"}}")
	t.Setenv("OPENAI_API_KEY", "")

	packetPath, receiptPath := writeCodexDispatchPacket(t, repo, codexDispatchPacketOptions{
		Sandbox:     "read-only",
		ArgvSandbox: "read-only",
	})
	out, err := executeCommand("codex", "dispatch", "--packet", packetPath, "--json")
	if err != nil {
		t.Fatalf("codex dispatch smoke returned error: %v\noutput:\n%s", err, out)
	}

	var printed codexRunReceipt
	if err := json.Unmarshal([]byte(out), &printed); err != nil {
		t.Fatalf("unmarshal printed smoke receipt: %v\noutput:\n%s", err, out)
	}
	receipt := readCodexDispatchReceipt(t, receiptPath)
	if err := validateCodexRunReceipt(receipt); err != nil {
		t.Fatalf("smoke receipt validation failed: %v\nreceipt: %+v", err, receipt)
	}
	if receipt.PacketID != printed.PacketID {
		t.Fatalf("receipt packet_id = %q, printed packet_id = %q", receipt.PacketID, printed.PacketID)
	}
	if receipt.Sandbox != "read-only" {
		t.Fatalf("sandbox = %q, want read-only", receipt.Sandbox)
	}
	if receipt.AuthMode != "chatgpt-subscription" || !strings.Contains(receipt.AuthStatus, "ChatGPT") {
		t.Fatalf("auth receipt = mode %q status %q, want ChatGPT subscription", receipt.AuthMode, receipt.AuthStatus)
	}
	if receipt.Stdin.Mode != "pipe-prompt" || receipt.Stdin.ClosedAt == "" || receipt.Stdin.BytesWritten == 0 {
		t.Fatalf("stdin receipt did not prove closed prompt pipe: %+v", receipt.Stdin)
	}
	if receipt.Outputs.ReceiptPath == "" || len(receipt.CommandsRun) == 0 || strings.TrimSpace(receipt.CommandsRun[0].Command) == "" {
		t.Fatalf("receipt missing output path or command evidence: %+v", receipt)
	}
	if receipt.Verdict.Status != "PASS" || receipt.Verdict.JudgeModelFamily == "" {
		t.Fatalf("verdict = %+v, want independent PASS verdict", receipt.Verdict)
	}
	assertFileContains(t, filepath.Join(repo, receipt.Outputs.FinalMessagePath), "VERDICT: PASS")
	assertFileContains(t, filepath.Join(repo, receipt.Outputs.JSONLPath), "item.completed")
}

func TestCodexTaskPacketSmokeRejectsInteractiveStdinBeforeWorker(t *testing.T) {
	repo := newCodexDispatchRepo(t)
	marker := filepath.Join(repo, "worker-ran")
	writeFakeCodexBinary(t)
	t.Setenv("FAKE_CODEX_MARKER", marker)
	t.Setenv("OPENAI_API_KEY", "")

	packetPath, receiptPath := writeCodexDispatchPacket(t, repo, codexDispatchPacketOptions{})
	data, err := os.ReadFile(packetPath)
	if err != nil {
		t.Fatalf("read packet: %v", err)
	}
	var packet codexTaskPacket
	if err := json.Unmarshal(data, &packet); err != nil {
		t.Fatalf("unmarshal packet: %v", err)
	}
	packet.Execution.Stdin = codexTaskStdin{Mode: "inherit-interactive"}
	data, err = json.MarshalIndent(packet, "", "  ")
	if err != nil {
		t.Fatalf("marshal packet: %v", err)
	}
	if err := os.WriteFile(packetPath, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write packet: %v", err)
	}

	_, err = executeCommand("codex", "dispatch", "--packet", packetPath, "--json")
	if err == nil {
		t.Fatalf("codex dispatch succeeded, want interactive stdin rejection")
	}
	if !strings.Contains(err.Error(), "inherit-interactive stdin") {
		t.Fatalf("dispatch error = %q, want inherit-interactive stdin rejection", err.Error())
	}
	assertPathAbsent(t, marker)
	assertPathAbsent(t, receiptPath)
}
