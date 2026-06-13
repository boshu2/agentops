package main

import (
	"bytes"
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

func TestConvergeProductionCanaryIsTwoSided(t *testing.T) {
	// The real gate must bite the planted self-judge and accept the good fixture.
	res := convergeRunCanary(convergeProductionCanaryGate)
	if !res.Passed || !res.Proceed {
		t.Fatalf("production canary Passed=%v Proceed=%v, want both true: %s", res.Passed, res.Proceed, res.Message)
	}
	// A gate that accepts everything must abort the run.
	broken := convergeRunCanary(func(string) (bool, int) { return false, 0 })
	if broken.Proceed {
		t.Fatal("broken (accept-all) gate must not let converge proceed")
	}
	// A degenerate all-reject gate gives false confidence and must also fail.
	allReject := convergeRunCanary(func(string) (bool, int) { return true, convergeCanaryRejectCode })
	if allReject.Passed {
		t.Fatal("all-reject gate must fail the two-sided canary")
	}
}

// Hardening (refuter P1): the command RunE must actually exercise the kernel
// (canary -> transport (LAW-0) -> non-mutating judge packet) and emit a real
// plan, not a vacuous print. We assert the output reflects the wired kernel.
func TestConvergeCommandRunExercisesKernel(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"converge"})
	if err != nil {
		t.Fatalf("converge not found: %v", err)
	}
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.ParseFlags([]string{"--max-rounds", "4", "--min-contexts", "2"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("converge RunE error: %v", err)
	}
	s := out.String()
	for _, want := range []string{"transport=", "judge-role=", "max-rounds=4", "non-mutating"} {
		if !convContainsSubH(s, want) {
			t.Fatalf("converge output missing %q (kernel not wired into RunE?):\n%s", want, s)
		}
	}
}

// Hardening (refuter #8): convergeEvaluateRound honors the passed floor so it
// cannot silently disagree with convergeRunBounded's cfg.MinContexts.
func TestConvergeEvaluateRoundHonorsFloor(t *testing.T) {
	round := convergeRoundResult{ContextVerdicts: []convergeContextVerdict{
		{ContextID: "a", ModelFamily: "claude", Pass: true},
		{ContextID: "b", ModelFamily: "openai", Pass: true},
	}}
	// Two PASS contexts: passes the default floor (2)...
	if d := convergeEvaluateRound(round); !d.Pass {
		t.Fatal("2 contexts should pass the default floor of 2")
	}
	// ...but a floor of 3 must NOT be a vacuous pass.
	if d := convergeEvaluateRound(round, 3); d.Pass {
		t.Fatal("2 contexts must not pass a floor of 3 (evaluateRound must honor the floor)")
	}
}

func convContainsSubH(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return needle == ""
}
