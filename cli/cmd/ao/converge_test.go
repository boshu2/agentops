package main

import (
	"os"
	"path/filepath"
	"testing"
)

// L1 regression coverage for the `ao converge` command surface and its
// composable convergence primitives. The behavior-frozen acceptance suite lives
// in the staged converge_orchestration_acceptance_test.go; these tests are the
// committed in-package regression net.

func TestConvergeCommandIsRegisteredWithShort(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"converge"})
	if err != nil {
		t.Fatalf("ao converge not registered: %v", err)
	}
	if cmd.Name() != "converge" {
		t.Fatalf("resolved command = %q, want converge", cmd.Name())
	}
	if cmd.Short == "" {
		t.Fatal("ao converge Short is empty")
	}
}

func TestConvergeJudgeAgreementContextFloor(t *testing.T) {
	// Two distinct PASS contexts meet a floor of 2.
	conv, reasons := convergeJudgeAgreement(convergeRoundResult{ContextVerdicts: []convergeContextVerdict{
		{ContextID: "a", ModelFamily: "claude", Pass: true},
		{ContextID: "b", ModelFamily: "openai", Pass: true},
	}}, 2)
	if !conv || len(reasons) != 0 {
		t.Fatalf("two distinct PASS contexts: converged=%v reasons=%v, want converged/empty", conv, reasons)
	}
	// One distinct context is below the floor.
	conv, _ = convergeJudgeAgreement(convergeRoundResult{ContextVerdicts: []convergeContextVerdict{
		{ContextID: "a", ModelFamily: "claude", Pass: true},
	}}, 2)
	if conv {
		t.Fatal("one context met a floor of 2, want not converged")
	}
}

func TestConvergeFailTrackerConsecutive(t *testing.T) {
	tr := newConvergeFailTracker()
	tr.Observe(false)
	tr.Observe(false)
	if tr.Blocked() {
		t.Fatal("blocked after 2 fails, want not yet (limit is 3)")
	}
	tr.Observe(false)
	if !tr.Blocked() || tr.Status() != convergeStatusBlock {
		t.Fatalf("after 3 consecutive fails: blocked=%v status=%q, want true/BLOCK", tr.Blocked(), tr.Status())
	}
}

func TestConvergeResolveTransportNeverClaudePrint(t *testing.T) {
	// Every runtime branch must keep UsesClaudePrint false (LAW 0).
	for _, env := range []convergeEnv{
		{ClaudeSessionID: "c", CodexThreadID: ""},
		{ClaudeSessionID: "", CodexThreadID: "x"},
		{ClaudeSessionID: "", CodexThreadID: ""},
		{ClaudeSessionID: "c", CodexThreadID: "x"},
	} {
		if convergeResolveTransport(env).UsesClaudePrint {
			t.Fatalf("env %+v: UsesClaudePrint = true, MUST be false (LAW 0)", env)
		}
	}
}

func TestConvergeRunBoundedKillSwitch(t *testing.T) {
	killDir := t.TempDir()
	mustWriteKill(t, killDir)
	out := convergeRunBounded(convergeLoopConfig{MaxRounds: 5, MinContexts: 2, KillDir: killDir}, nil)
	if out.Status != convergeStatusKilled {
		t.Fatalf("with KILL present, status = %q, want %q", out.Status, convergeStatusKilled)
	}
}

func mustWriteKill(t *testing.T, killDir string) {
	t.Helper()
	dir := filepath.Join(killDir, ".agents", "rpi")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "KILL"), []byte("stop"), 0o644); err != nil {
		t.Fatal(err)
	}
}
