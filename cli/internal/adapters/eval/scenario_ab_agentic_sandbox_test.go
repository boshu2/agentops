package eval

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/scenario"
)

// TestWorkspaceCommandRunner_Integration_ControlArmDeniesCorpusRead (age-gocli-audit
// -remediation-6fybr.2, FIRST CHECK): on darwin, a control-arm (without-gold) emitted
// command that cats a resolved corpusDenyPaths entry must be DENIED — the read fails,
// the corpus bytes never surface. Before the fix the runner used `bash -lc` with no
// sandbox and the read SUCCEEDED, voiding the A/B.
func TestWorkspaceCommandRunner_Integration_ControlArmDeniesCorpusRead(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only: macOS Seatbelt sandbox-exec integration")
	}
	if _, err := os.Stat(macOSSandboxExec); err != nil {
		t.Skipf("sandbox-exec unavailable: %v", err)
	}

	root := t.TempDir()
	agents := filepath.Join(root, ".agents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(agents, "sentinel.txt")
	if err := os.WriteFile(secret, []byte("CORPUS-SECRET-MUST-NOT-READ"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Stable, empty HOME so ~/.agents adds nothing surprising to the deny set.
	t.Setenv("HOME", t.TempDir())
	// corpusDenyPaths resolves the corpus root from os.Getwd(), exactly as the codex
	// arm does; chdir into the fake repo so `root/.agents` is denied. t.Chdir restores.
	t.Chdir(root)
	workDir := t.TempDir()

	// Control arm (withGold=false): reading the denied corpus file must fail.
	stdout, exitCode, err := defaultWorkspaceCommandRunner(context.Background(), workDir, "cat "+secret, false)
	if err != nil {
		t.Fatalf("runner returned a Go error (want a denied read = nonzero exit, no Go error): %v", err)
	}
	if exitCode == 0 {
		t.Fatalf("control arm read the corpus (isolation void): exit=0 stdout=%q", stdout)
	}
	if strings.Contains(stdout, "CORPUS-SECRET-MUST-NOT-READ") {
		t.Fatalf("control arm leaked corpus bytes: stdout=%q", stdout)
	}
}

// TestWorkspaceCommandRunner_Integration_ControlArmAllowsWorkspaceWrite (acceptance e):
// the deny set covers corpus roots ONLY, so a control-arm write+read inside the temp
// workspace must still succeed under confinement.
func TestWorkspaceCommandRunner_Integration_ControlArmAllowsWorkspaceWrite(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only: macOS Seatbelt sandbox-exec integration")
	}
	if _, err := os.Stat(macOSSandboxExec); err != nil {
		t.Skipf("sandbox-exec unavailable: %v", err)
	}

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)
	workDir := t.TempDir()

	stdout, exitCode, err := defaultWorkspaceCommandRunner(
		context.Background(), workDir,
		"echo workspace-ok > out.txt && cat out.txt", false)
	if err != nil {
		t.Fatalf("runner error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("control-arm workspace write/read must succeed: exit=%d stdout=%q", exitCode, stdout)
	}
	if !strings.Contains(stdout, "workspace-ok") {
		t.Fatalf("workspace read = %q, want workspace-ok", stdout)
	}
	if _, statErr := os.Stat(filepath.Join(workDir, "out.txt")); statErr != nil {
		t.Fatalf("workspace write did not land: %v", statErr)
	}
}

// TestSandboxedShellCmd_FailClosed (acceptance b, c): the emitted-command executor
// FAILS CLOSED cross-platform — no GOOS skip. An empty deny set fails closed on every
// platform; a non-darwin build with no Seatbelt fails closed (never a bare shell);
// darwin wraps sandbox-exec + `bash` with the deny profile.
func TestSandboxedShellCmd_FailClosed(t *testing.T) {
	// Empty deny set → fail closed on EVERY platform.
	if cmd, err := sandboxedShellCmd(context.Background(), nil, "cat /x"); err == nil || cmd != nil {
		t.Errorf("empty deny set must fail closed; got cmd=%v err=%v", cmd, err)
	}

	cmd, err := sandboxedShellCmd(context.Background(), []string{"/r/.agents", "/r/.ao"}, "cat /r/.agents/x")
	switch runtime.GOOS {
	case "darwin":
		if err != nil || cmd == nil {
			t.Fatalf("darwin: expected a sandboxed command; got cmd=%v err=%v", cmd, err)
		}
		if !strings.Contains(cmd.Path, "sandbox-exec") {
			t.Errorf("darwin: must wrap sandbox-exec; got path %q", cmd.Path)
		}
		joined := strings.Join(cmd.Args, " ")
		if !strings.Contains(joined, "deny file-read*") || !strings.Contains(joined, "bash") {
			t.Errorf("darwin: args must carry the deny profile + bash; got %q", joined)
		}
		// A non-login shell: `bash -c`, never `-lc`.
		if !strings.Contains(joined, "-c") || strings.Contains(joined, "-lc") {
			t.Errorf("darwin: must use a non-login shell (bash -c, not -lc); got %q", joined)
		}
	default:
		if err == nil || cmd != nil {
			t.Errorf("non-darwin must fail closed (no unisolated arm); got cmd=%v err=%v", cmd, err)
		}
	}
}

// TestWorkspaceCommand_WithGold_UnconfinedRegression (acceptance d): with_gold corpus
// access is UNCHANGED — the treatment arm runs unconfined (no sandbox-exec wrapper) in
// a non-login shell rooted at the workspace, so it can still read the corpus.
func TestWorkspaceCommand_WithGold_UnconfinedRegression(t *testing.T) {
	cmd, err := workspaceCommand(context.Background(), "/wd", "cat corpus", true)
	if err != nil {
		t.Fatalf("with-gold arm must build a command, got err=%v", err)
	}
	if strings.Contains(cmd.Path, "sandbox-exec") {
		t.Errorf("with-gold arm must NOT be Seatbelt-confined; got path %q", cmd.Path)
	}
	joined := strings.Join(cmd.Args, " ")
	if !strings.Contains(joined, "bash") || !strings.Contains(joined, "cat corpus") {
		t.Errorf("with-gold arm must run the command via bash; got %q", joined)
	}
	if strings.Contains(joined, "-lc") {
		t.Errorf("must not use a login shell; got %q", joined)
	}
	if cmd.Dir != "/wd" {
		t.Errorf("with-gold arm cmd.Dir = %q, want the workspace /wd", cmd.Dir)
	}
}

// TestWorkspaceCommand_ControlArm_ConfinedRegression (acceptance a, b): the control arm
// is confined through the SAME corpus-deny + Seatbelt machinery as the codex arm and
// FAILS CLOSED on non-darwin — never an unisolated command.
func TestWorkspaceCommand_ControlArm_ConfinedRegression(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", t.TempDir())
	t.Chdir(root)

	cmd, err := workspaceCommand(context.Background(), t.TempDir(), "cat "+filepath.Join(root, ".agents", "x"), false)
	switch runtime.GOOS {
	case "darwin":
		if err != nil || cmd == nil {
			t.Fatalf("darwin control arm must build a confined command; got cmd=%v err=%v", cmd, err)
		}
		if !strings.Contains(cmd.Path, "sandbox-exec") {
			t.Errorf("darwin control arm must be Seatbelt-confined; got path %q", cmd.Path)
		}
		joined := strings.Join(cmd.Args, " ")
		if !strings.Contains(joined, filepath.Join(root, ".agents")) {
			t.Errorf("control arm deny profile must include the resolved corpus root; got %q", joined)
		}
	default:
		if err == nil || cmd != nil {
			t.Errorf("non-darwin control arm must fail closed; got cmd=%v err=%v", cmd, err)
		}
	}
}

// TestAgenticRunner_PropagatesWithGoldToRunCmd (the runCmd-loses-withGold bug): the
// runner now threads the withGold bit into every emitted-command call, so the control
// arm is confined and the treatment arm is not. Before the fix the hook signature had
// no withGold and the bit was dropped.
func TestAgenticRunner_PropagatesWithGoldToRunCmd(t *testing.T) {
	orig := agenticRunnerHooks
	t.Cleanup(func() { agenticRunnerHooks = orig })

	var seen []bool
	agenticRunnerHooks.runCodex = func(_ context.Context, _ string, _ string, _ bool) (string, int, error) {
		return `{"commands":["echo hi"],"done":true,"summary":"done"}`, 1, nil
	}
	agenticRunnerHooks.runCmd = func(_ context.Context, _ string, _ string, withGold bool) (string, int, error) {
		seen = append(seen, withGold)
		return "", 0, nil
	}

	sc := scenario.Scenario{ID: "s-withgold", Goal: "echo hi", Narrative: "trivial"}
	if _, err := (agenticScenarioRunner{}).RunArm(context.Background(), sc, true); err != nil {
		t.Fatalf("treatment RunArm: %v", err)
	}
	if _, err := (agenticScenarioRunner{}).RunArm(context.Background(), sc, false); err != nil {
		t.Fatalf("control RunArm: %v", err)
	}
	if len(seen) != 2 || seen[0] != true || seen[1] != false {
		t.Fatalf("runCmd withGold sequence = %v, want [true false]", seen)
	}
}

// TestJudgeAndControlArmShareCorpusDenyMachinery (acceptance d, judges stay
// corpus-denied): the codex judge path (runCodexExec → allowCorpus=false →
// sandboxedCodexCmd) and the agentic control arm (sandboxedShellCmd) both derive their
// deny set from corpusDenyPaths and both FAIL CLOSED on an empty deny set. Neither can
// ever run unisolated, so judge grading stays corpus-denied.
func TestJudgeAndControlArmShareCorpusDenyMachinery(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", t.TempDir())
	deny, err := corpusDenyPaths(root)
	if err != nil {
		t.Fatalf("corpusDenyPaths: %v", err)
	}
	if len(deny) == 0 {
		t.Fatal("shared corpusDenyPaths must be non-empty for a repo with .agents")
	}
	// Both executors refuse an empty deny set on every platform (fail-closed).
	if _, err := sandboxedCodexCmd(context.Background(), nil, []string{"exec"}); err == nil {
		t.Error("judge/codex arm must fail closed on empty deny set")
	}
	if _, err := sandboxedShellCmd(context.Background(), nil, "cat x"); err == nil {
		t.Error("control shell arm must fail closed on empty deny set")
	}
}
