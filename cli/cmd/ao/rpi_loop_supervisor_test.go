package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestResolveLoopSupervisorConfig_AppliesSupervisorDefaults(t *testing.T) {
	t.Setenv("AGENTOPS_RPI_RUNTIME", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME_MODE", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_AO_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_BD_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_TMUX_COMMAND", "")
	prev := snapshotLoopSupervisorGlobals()
	defer restoreLoopSupervisorGlobals(prev)

	rpiSupervisor = true
	rpiFailurePolicy = "stop"
	rpiCycleRetries = 0
	rpiCycleDelay = 0
	rpiCompile = false
	rpiCompileInterval = 0
	rpiCompileSince = ""
	rpiCompileDefrag = false
	rpiLease = false
	rpiDetachedHeal = false
	rpiAutoClean = false
	rpiEnsureCleanup = false
	rpiCleanupPruneBranches = false
	rpiGatePolicy = "off"
	rpiLandingPolicy = "off"
	rpiLandingLockPath = ""
	rpiBDSyncPolicy = "auto"
	rpiLeaseTTL = 2 * time.Minute
	rpiAutoCleanStaleAfter = 24 * time.Hour
	rpiLeasePath = ".agents/rpi/supervisor.lock"

	cmd := newLoopSupervisorTestCommand()
	tmpDir := t.TempDir()

	cfg, err := resolveLoopSupervisorConfig(cmd, tmpDir)
	if err != nil {
		t.Fatalf("resolveLoopSupervisorConfig: %v", err)
	}
	if cfg.FailurePolicy != loopFailurePolicyContinue {
		t.Fatalf("failure policy: got %q, want %q", cfg.FailurePolicy, loopFailurePolicyContinue)
	}
	if cfg.CycleRetries != 1 {
		t.Fatalf("cycle retries: got %d, want 1", cfg.CycleRetries)
	}
	if cfg.CycleDelay != 5*time.Minute {
		t.Fatalf("cycle delay: got %s, want 5m", cfg.CycleDelay)
	}
	if !cfg.CompileEnabled {
		t.Fatal("expected Compile cadence to be enabled in supervisor defaults")
	}
	if cfg.CompileInterval != 30*time.Minute {
		t.Fatalf("compile interval: got %s, want 30m", cfg.CompileInterval)
	}
	if cfg.CompileSince != "26h" {
		t.Fatalf("compile since: got %q, want %q", cfg.CompileSince, "26h")
	}
	if !cfg.CompileDefrag {
		t.Fatal("expected Compile defrag to be enabled in supervisor defaults")
	}
	if !cfg.LeaseEnabled {
		t.Fatal("expected lease to be enabled in supervisor defaults")
	}
	if cfg.DetachedHeal {
		t.Fatal("expected detached heal to be disabled in supervisor defaults")
	}
	if !cfg.AutoClean {
		t.Fatal("expected auto-clean to be enabled in supervisor defaults")
	}
	if !cfg.EnsureCleanup {
		t.Fatal("expected ensure-cleanup to be enabled in supervisor defaults")
	}
	if !cfg.CleanupPruneBranches {
		t.Fatal("expected cleanup-prune-branches to be enabled in supervisor defaults")
	}
	if cfg.GatePolicy != loopGatePolicyRequired {
		t.Fatalf("gate policy: got %q, want %q", cfg.GatePolicy, loopGatePolicyRequired)
	}
	if cfg.LandingLockPath != filepath.Join(tmpDir, ".agents", "rpi", "landing.lock") {
		t.Fatalf("landing lock path: got %q, want %q", cfg.LandingLockPath, filepath.Join(tmpDir, ".agents", "rpi", "landing.lock"))
	}
	if cfg.KillSwitchPath != filepath.Join(tmpDir, ".agents", "rpi", "KILL") {
		t.Fatalf("kill switch path: got %q, want %q", cfg.KillSwitchPath, filepath.Join(tmpDir, ".agents", "rpi", "KILL"))
	}
}

func TestResolveLoopSupervisorConfig_SupervisorNilCommandUsesDefaults(t *testing.T) {
	t.Setenv("AGENTOPS_RPI_RUNTIME", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME_MODE", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_AO_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_BD_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_TMUX_COMMAND", "")
	prev := snapshotLoopSupervisorGlobals()
	defer restoreLoopSupervisorGlobals(prev)

	rpiSupervisor = true
	rpiFailurePolicy = "stop"
	rpiCycleRetries = 0
	rpiCycleDelay = 0
	rpiCompile = false
	rpiCompileInterval = 0
	rpiCompileSince = ""
	rpiCompileDefrag = false
	rpiLease = false
	rpiDetachedHeal = false
	rpiAutoClean = false
	rpiEnsureCleanup = false
	rpiCleanupPruneBranches = false
	rpiGatePolicy = "off"
	rpiLandingPolicy = "off"
	rpiLandingLockPath = ""
	rpiBDSyncPolicy = "auto"
	rpiLeaseTTL = 2 * time.Minute
	rpiAutoCleanStaleAfter = 24 * time.Hour
	rpiLeasePath = ".agents/rpi/supervisor.lock"

	cfg, err := resolveLoopSupervisorConfig(nil, t.TempDir())
	if err != nil {
		t.Fatalf("resolveLoopSupervisorConfig nil command: %v", err)
	}
	if cfg.FailurePolicy != loopFailurePolicyContinue {
		t.Fatalf("failure policy: got %q, want %q", cfg.FailurePolicy, loopFailurePolicyContinue)
	}
	if !cfg.CompileEnabled || !cfg.CompileDefrag {
		t.Fatalf("expected compile defaults with nil command; got compile=%v defrag=%v", cfg.CompileEnabled, cfg.CompileDefrag)
	}
	if !cfg.LeaseEnabled || !cfg.AutoClean || !cfg.EnsureCleanup {
		t.Fatalf("expected lease/auto-clean/ensure-cleanup defaults with nil command; got lease=%v auto=%v ensure=%v", cfg.LeaseEnabled, cfg.AutoClean, cfg.EnsureCleanup)
	}
}

func TestRPILoop_ResolveLoopSupervisorConfig_RalphPreset(t *testing.T) {
	t.Setenv("AGENTOPS_RPI_RUNTIME", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME_MODE", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_AO_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_BD_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_TMUX_COMMAND", "")
	prev := snapshotLoopSupervisorGlobals()
	defer restoreLoopSupervisorGlobals(prev)

	rpiSupervisor = false
	rpiRalph = true
	rpiFailurePolicy = "stop"
	rpiCycleRetries = 0
	rpiCycleDelay = 0
	rpiCompile = false
	rpiCompileInterval = 0
	rpiCompileSince = ""
	rpiCompileDefrag = false
	rpiLease = false
	rpiDetachedHeal = false
	rpiAutoClean = false
	rpiEnsureCleanup = false
	rpiCleanupPruneBranches = false
	rpiGatePolicy = "off"
	rpiLandingLockPath = ""
	rpiLeaseTTL = 2 * time.Minute
	rpiAutoCleanStaleAfter = 24 * time.Hour
	rpiLeasePath = ".agents/rpi/supervisor.lock"

	cmd := newLoopSupervisorTestCommand()
	cfg, err := resolveLoopSupervisorConfig(cmd, t.TempDir())
	if err != nil {
		t.Fatalf("resolveLoopSupervisorConfig: %v", err)
	}
	if !cfg.RalphPreset {
		t.Fatal("expected Ralph preset to be recorded in config")
	}
	if cfg.FailurePolicy != loopFailurePolicyContinue {
		t.Fatalf("failure policy = %q, want %q", cfg.FailurePolicy, loopFailurePolicyContinue)
	}
	if cfg.CycleDelay != 2*time.Minute {
		t.Fatalf("cycle delay = %s, want 2m", cfg.CycleDelay)
	}
	if !cfg.CompileEnabled || !cfg.CompileDefrag {
		t.Fatalf("expected compile+defrag enabled in Ralph mode; got compile=%v defrag=%v", cfg.CompileEnabled, cfg.CompileDefrag)
	}
	if cfg.CompileInterval != 30*time.Minute {
		t.Fatalf("compile interval = %s, want 30m", cfg.CompileInterval)
	}
	if cfg.CompileSince != "26h" {
		t.Fatalf("compile since = %q, want %q", cfg.CompileSince, "26h")
	}
	if !cfg.LeaseEnabled || !cfg.AutoClean || !cfg.EnsureCleanup {
		t.Fatalf("expected lease/auto-clean/ensure-cleanup all true; got lease=%v auto=%v ensure=%v", cfg.LeaseEnabled, cfg.AutoClean, cfg.EnsureCleanup)
	}
	if !cfg.CleanupPruneBranches {
		t.Fatal("expected cleanup-prune-branches=true in Ralph mode")
	}
	if !cfg.DetachedHeal {
		t.Fatal("expected detached-heal=true in Ralph mode")
	}
	if cfg.GatePolicy != loopGatePolicyRequired {
		t.Fatalf("gate policy = %q, want %q", cfg.GatePolicy, loopGatePolicyRequired)
	}
}

func TestRPILoop_ResolveLoopSupervisorConfig_RalphHonorsExplicitOverrides(t *testing.T) {
	t.Setenv("AGENTOPS_RPI_RUNTIME", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME_MODE", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_AO_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_BD_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_TMUX_COMMAND", "")
	prev := snapshotLoopSupervisorGlobals()
	defer restoreLoopSupervisorGlobals(prev)

	rpiSupervisor = false
	rpiRalph = true
	rpiFailurePolicy = "stop"
	rpiCycleRetries = 0
	rpiCycleDelay = 0
	rpiCompile = false
	rpiCompileInterval = 0
	rpiCompileSince = ""
	rpiCompileDefrag = false
	rpiLease = false
	rpiDetachedHeal = false
	rpiAutoClean = false
	rpiEnsureCleanup = false
	rpiCleanupPruneBranches = false
	rpiGatePolicy = "off"
	rpiLandingLockPath = ""
	rpiLeaseTTL = 2 * time.Minute
	rpiAutoCleanStaleAfter = 24 * time.Hour
	rpiLeasePath = ".agents/rpi/supervisor.lock"

	cmd := newLoopSupervisorTestCommand()
	if err := cmd.Flags().Set("failure-policy", "stop"); err != nil {
		t.Fatalf("set failure-policy: %v", err)
	}
	if err := cmd.Flags().Set("cycle-delay", "45s"); err != nil {
		t.Fatalf("set cycle-delay: %v", err)
	}
	if err := cmd.Flags().Set("compile", "false"); err != nil {
		t.Fatalf("set compile: %v", err)
	}
	if err := cmd.Flags().Set("compile-defrag", "false"); err != nil {
		t.Fatalf("set compile-defrag: %v", err)
	}
	if err := cmd.Flags().Set("compile-interval", "5m"); err != nil {
		t.Fatalf("set compile-interval: %v", err)
	}
	if err := cmd.Flags().Set("compile-since", "4h"); err != nil {
		t.Fatalf("set compile-since: %v", err)
	}
	rpiFailurePolicy = "stop"
	rpiCycleDelay = 45 * time.Second
	rpiCompile = false
	rpiCompileDefrag = false
	rpiCompileInterval = 5 * time.Minute
	rpiCompileSince = "4h"

	cfg, err := resolveLoopSupervisorConfig(cmd, t.TempDir())
	if err != nil {
		t.Fatalf("resolveLoopSupervisorConfig: %v", err)
	}
	if cfg.FailurePolicy != loopFailurePolicyStop {
		t.Fatalf("failure policy = %q, want explicit %q", cfg.FailurePolicy, loopFailurePolicyStop)
	}
	if cfg.CycleDelay != 45*time.Second {
		t.Fatalf("cycle delay = %s, want explicit 45s", cfg.CycleDelay)
	}
	if cfg.CompileEnabled {
		t.Fatal("compile should honor explicit false override")
	}
	if cfg.CompileDefrag {
		t.Fatal("compile-defrag should honor explicit false override")
	}
	if cfg.CompileInterval != 5*time.Minute {
		t.Fatalf("compile interval = %s, want explicit 5m", cfg.CompileInterval)
	}
	if cfg.CompileSince != "4h" {
		t.Fatalf("compile since = %q, want explicit %q", cfg.CompileSince, "4h")
	}
}

func TestAcquireSupervisorLease_SingleFlight(t *testing.T) {
	tmpDir := t.TempDir()
	leasePath := filepath.Join(tmpDir, "supervisor.lock")

	lease1, err := acquireSupervisorLease(tmpDir, leasePath, 2*time.Minute, "run-1")
	if err != nil {
		t.Fatalf("acquire first lease: %v", err)
	}

	if _, err := acquireSupervisorLease(tmpDir, leasePath, 2*time.Minute, "run-2"); err == nil {
		t.Fatal("expected second lease acquisition to fail while first is held")
	}

	if err := lease1.Release(); err != nil {
		t.Fatalf("release first lease: %v", err)
	}

	lease3, err := acquireSupervisorLease(tmpDir, leasePath, 2*time.Minute, "run-3")
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	defer func() { _ = lease3.Release() }()
}

func TestShouldRunBDSync(t *testing.T) {
	prevLookPath := loopLookPath
	defer func() { loopLookPath = prevLookPath }()

	tmpDir := t.TempDir()

	loopLookPath = func(_ string) (string, error) {
		return "", fmt.Errorf("not found")
	}
	run, err := shouldRunBDSync(tmpDir, loopBDSyncPolicyAuto, "bd")
	if err != nil {
		t.Fatalf("auto policy with missing bd should not error: %v", err)
	}
	if run {
		t.Fatal("auto policy should skip when bd is unavailable")
	}

	loopLookPath = func(_ string) (string, error) {
		return "/usr/bin/bd", nil
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, ".beads"), 0755); err != nil {
		t.Fatal(err)
	}
	run, err = shouldRunBDSync(tmpDir, loopBDSyncPolicyAuto, "bd")
	if err != nil {
		t.Fatalf("auto policy with bd/.beads should not error: %v", err)
	}
	if !run {
		t.Fatal("auto policy should run when bd exists and .beads exists")
	}

	loopLookPath = func(_ string) (string, error) {
		return "", fmt.Errorf("not found")
	}
	if _, err := shouldRunBDSync(tmpDir, loopBDSyncPolicyAlways, "bd"); err == nil {
		t.Fatal("always policy should error when bd is unavailable")
	}
}

func TestShouldRunBDSync_UsesConfiguredCommand(t *testing.T) {
	prevLookPath := loopLookPath
	defer func() { loopLookPath = prevLookPath }()

	var lookedUp string
	loopLookPath = func(name string) (string, error) {
		lookedUp = name
		return "/usr/bin/" + name, nil
	}

	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, ".beads"), 0755); err != nil {
		t.Fatal(err)
	}

	run, err := shouldRunBDSync(tmpDir, loopBDSyncPolicyAuto, "bd-custom")
	if err != nil {
		t.Fatalf("shouldRunBDSync returned error: %v", err)
	}
	if !run {
		t.Fatal("expected auto policy to run when custom command resolves and .beads exists")
	}
	if lookedUp != "bd-custom" {
		t.Fatalf("lookPath called with %q, want %q", lookedUp, "bd-custom")
	}
}

func TestRenderLandingCommitMessage(t *testing.T) {
	msg := renderLandingCommitMessage("cycle={{cycle}} attempt={{attempt}} goal={{goal}}", 4, 2, "ship it")
	if !strings.Contains(msg, "cycle=4") || !strings.Contains(msg, "attempt=2") || !strings.Contains(msg, "goal=ship it") {
		t.Fatalf("unexpected rendered message: %q", msg)
	}
}

func TestRunGateScript(t *testing.T) {
	tmpDir := t.TempDir()
	missing := filepath.Join("scripts", "missing.sh")
	if err := runGateScript(tmpDir, missing, false, time.Second); err != nil {
		t.Fatalf("optional missing gate should not fail: %v", err)
	}
	if err := runGateScript(tmpDir, missing, true, time.Second); err == nil {
		t.Fatal("required missing gate should fail")
	}
}

type supervisorLandingSyncPushFailureCase struct {
	failingRunnerCall string
	runnerErr         error
	expectedError     string
	rebaseAbortOutput string
	rebaseAbortErr    error
}

type supervisorLandingCommandCalls struct {
	runner []string
	output []string
}

func TestRunSupervisorLanding_SyncPush_RebaseFailureAborts(t *testing.T) {
	calls := runSupervisorLandingSyncPushFailure(t, supervisorLandingSyncPushFailureCase{
		failingRunnerCall: "git rebase origin/main",
		runnerErr:         fmt.Errorf("simulated rebase conflict"),
		expectedError:     "landing rebase failed",
	})

	assertSupervisorLandingContainsCall(t, calls.output, "git rebase --abort")
	assertSupervisorLandingContainsCall(t, calls.runner, "git status -sb")
}

func TestRunSupervisorLanding_SyncPush_FetchFailure_RecoversState(t *testing.T) {
	calls := runSupervisorLandingSyncPushFailure(t, supervisorLandingSyncPushFailureCase{
		failingRunnerCall: "git fetch origin main",
		runnerErr:         fmt.Errorf("simulated fetch outage"),
		expectedError:     "landing fetch failed",
		rebaseAbortOutput: "fatal: No rebase in progress?",
		rebaseAbortErr:    fmt.Errorf("exit status 128"),
	})

	assertSupervisorLandingContainsCall(t, calls.output, "git rebase --abort")
	assertSupervisorLandingContainsCall(t, calls.runner, "git status -sb")
}

func runSupervisorLandingSyncPushFailure(t *testing.T, tc supervisorLandingSyncPushFailureCase) supervisorLandingCommandCalls {
	t.Helper()

	prevRunner := loopCommandRunner
	prevOutputRunner := loopCommandOutputRunner
	defer func() {
		loopCommandRunner = prevRunner
		loopCommandOutputRunner = prevOutputRunner
	}()

	calls := supervisorLandingCommandCalls{}
	loopCommandRunner = func(_ string, _ time.Duration, name string, args ...string) error {
		call := supervisorLandingCommandCall(name, args)
		calls.runner = append(calls.runner, call)
		if call == tc.failingRunnerCall {
			return tc.runnerErr
		}
		return nil
	}
	loopCommandOutputRunner = func(_ string, _ time.Duration, name string, args ...string) (string, error) {
		call := supervisorLandingCommandCall(name, args)
		calls.output = append(calls.output, call)
		switch call {
		case "git status --porcelain":
			return " M somefile.go\n", nil
		case "git diff --name-only HEAD --":
			return "somefile.go\n", nil
		case "git symbolic-ref --quiet --short refs/remotes/origin/HEAD":
			return "origin/main", nil
		case "git rebase --abort":
			return tc.rebaseAbortOutput, tc.rebaseAbortErr
		default:
			return "", nil
		}
	}

	cfg := rpiLoopSupervisorConfig{
		LandingPolicy:  loopLandingPolicySyncPush,
		BDSyncPolicy:   loopBDSyncPolicyNever,
		CommandTimeout: time.Minute,
	}
	err := runSupervisorLanding(t.TempDir(), cfg, 1, 1, "ship", &landingScope{
		baselineDirtyPaths: map[string]struct{}{},
	})
	if err == nil || !strings.Contains(err.Error(), tc.expectedError) {
		t.Fatalf("expected %s, got: %v", tc.expectedError, err)
	}
	if !strings.Contains(err.Error(), "state recovered") {
		t.Fatalf("expected state recovery details in error, got: %v", err)
	}

	return calls
}

func supervisorLandingCommandCall(name string, args []string) string {
	if len(args) == 0 {
		return name
	}
	return name + " " + strings.Join(args, " ")
}

func assertSupervisorLandingContainsCall(t *testing.T, calls []string, want string) {
	t.Helper()

	for _, call := range calls {
		if call == want {
			return
		}
	}
	t.Fatalf("expected %s call, got calls: %v", want, calls)
}

func TestRunSupervisorLanding_CommitPolicy_RespectsLandingLock(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "landing.lock")

	landingLease, err := acquireSupervisorLease(tmpDir, lockPath, 2*time.Minute, "landing-run-locked")
	if err != nil {
		t.Fatalf("acquire landing lease: %v", err)
	}
	defer func() {
		if err := landingLease.Release(); err != nil {
			t.Fatalf("release landing lease: %v", err)
		}
	}()

	cfg := rpiLoopSupervisorConfig{
		LandingPolicy:        loopLandingPolicyCommit,
		LandingLockPath:      lockPath,
		LandingCommitMessage: "chore(rpi): autonomous cycle {{cycle}}",
		CommandTimeout:       time.Minute,
	}
	err = runSupervisorLanding(tmpDir, cfg, 1, 1, "ship", &landingScope{
		baselineDirtyPaths: map[string]struct{}{},
	})
	if err == nil {
		t.Fatal("expected landing lock contention error")
	}
	if !strings.Contains(err.Error(), "landing lock acquisition failed") {
		t.Fatalf("expected landing lock acquisition error, got: %v", err)
	}
}

func TestRunSupervisorLanding_CommitPolicy_LockContentionThenSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "landing.lock")

	landingLease, err := acquireSupervisorLease(tmpDir, lockPath, 2*time.Minute, "landing-run-locked")
	if err != nil {
		t.Fatalf("acquire landing lease: %v", err)
	}

	cfg := rpiLoopSupervisorConfig{
		LandingPolicy:        loopLandingPolicyCommit,
		LandingLockPath:      lockPath,
		LandingCommitMessage: "chore(rpi): autonomous cycle {{cycle}}",
		CommandTimeout:       time.Minute,
	}

	err = runSupervisorLanding(tmpDir, cfg, 1, 1, "ship", &landingScope{
		baselineDirtyPaths: map[string]struct{}{},
	})
	if err == nil || !strings.Contains(err.Error(), "landing lock acquisition failed") {
		t.Fatalf("expected lock contention failure, got: %v", err)
	}

	if err := landingLease.Release(); err != nil {
		t.Fatalf("release landing lease: %v", err)
	}

	prevOutputRunner := loopCommandOutputRunner
	defer func() { loopCommandOutputRunner = prevOutputRunner }()
	loopCommandOutputRunner = func(_ string, _ time.Duration, name string, args ...string) (string, error) {
		if name == "git" && len(args) >= 2 && args[0] == "status" && args[1] == "--porcelain" {
			return "", nil
		}
		return "", nil
	}

	if err := runSupervisorLanding(tmpDir, cfg, 1, 2, "ship", &landingScope{
		baselineDirtyPaths: map[string]struct{}{},
	}); err != nil {
		t.Fatalf("expected landing to succeed after lock release, got: %v", err)
	}
}

func TestCommitIfDirty_RepeatedCyclesInDirtyRepoCommitOnlyOwnedPaths(t *testing.T) {
	repoPath := t.TempDir()

	runGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s failed: %v (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
		}
		return string(out)
	}

	runGit("init", "-q")
	runGit("config", "user.email", "noreply@example.com")
	runGit("config", "user.name", "Test User")
	runGit("config", "commit.gpgsign", "false")
	runGit("checkout", "-q", "-b", "main")
	runGit("commit", "-q", "--allow-empty", "-m", "init")

	preExistingPath := filepath.Join(repoPath, "preexisting.txt")
	if err := os.WriteFile(preExistingPath, []byte("dirty baseline\n"), 0644); err != nil {
		t.Fatalf("write preexisting file: %v", err)
	}

	scope1, err := captureLandingScope(repoPath, time.Minute)
	if err != nil {
		t.Fatalf("capture scope 1: %v", err)
	}

	owned1Path := filepath.Join(repoPath, "owned-1.txt")
	if err := os.WriteFile(owned1Path, []byte("owned cycle 1\n"), 0644); err != nil {
		t.Fatalf("write owned-1 file: %v", err)
	}

	committed, err := commitIfDirty(repoPath, "cycle-1", time.Minute, scope1)
	if err != nil {
		t.Fatalf("commitIfDirty cycle 1: %v", err)
	}
	if !committed {
		t.Fatal("expected cycle 1 to produce a commit")
	}

	showHead := strings.TrimSpace(runGit("show", "--name-only", "--pretty=format:", "HEAD"))
	if showHead != "owned-1.txt" {
		t.Fatalf("expected HEAD to include only owned-1.txt, got %q", showHead)
	}

	statusAfterFirst := runGit("status", "--porcelain")
	if !strings.Contains(statusAfterFirst, " preexisting.txt") {
		t.Fatalf("expected preexisting dirty file to remain after cycle 1, got:\n%s", statusAfterFirst)
	}

	scope2, err := captureLandingScope(repoPath, time.Minute)
	if err != nil {
		t.Fatalf("capture scope 2: %v", err)
	}

	owned2Path := filepath.Join(repoPath, "owned-2.txt")
	if err := os.WriteFile(owned2Path, []byte("owned cycle 2\n"), 0644); err != nil {
		t.Fatalf("write owned-2 file: %v", err)
	}

	committed, err = commitIfDirty(repoPath, "cycle-2", time.Minute, scope2)
	if err != nil {
		t.Fatalf("commitIfDirty cycle 2: %v", err)
	}
	if !committed {
		t.Fatal("expected cycle 2 to produce a commit")
	}

	showLatest := strings.TrimSpace(runGit("show", "--name-only", "--pretty=format:", "HEAD"))
	if showLatest != "owned-2.txt" {
		t.Fatalf("expected latest commit to include only owned-2.txt, got %q", showLatest)
	}
	showPrevious := strings.TrimSpace(runGit("show", "--name-only", "--pretty=format:", "HEAD~1"))
	if showPrevious != "owned-1.txt" {
		t.Fatalf("expected previous commit to include only owned-1.txt, got %q", showPrevious)
	}

	statusAfterSecond := runGit("status", "--porcelain")
	if !strings.Contains(statusAfterSecond, " preexisting.txt") {
		t.Fatalf("expected preexisting dirty file to remain after cycle 2, got:\n%s", statusAfterSecond)
	}
}

func TestIsNoRebaseInProgressMessage(t *testing.T) {
	cases := []struct {
		name string
		msg  string
		want bool
	}{
		{name: "empty", msg: "", want: false},
		{name: "no rebase in progress", msg: "fatal: No rebase in progress?", want: true},
		{name: "no rebase to abort", msg: "fatal: no rebase to abort", want: true},
		{name: "other error", msg: "fatal: something else failed", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNoRebaseInProgressMessage(tc.msg); got != tc.want {
				t.Fatalf("isNoRebaseInProgressMessage(%q) = %v, want %v", tc.msg, got, tc.want)
			}
		})
	}
}

func TestShouldMarkQueueEntryFailed_InfraVsTask(t *testing.T) {
	taskErr := wrapCycleFailure(cycleFailureTask, "task", fmt.Errorf("task failed"))
	if !shouldMarkQueueEntryFailed(taskErr) {
		t.Fatal("task failure should mark queue entry failed")
	}

	infraErr := wrapCycleFailure(cycleFailureInfrastructure, "infra", fmt.Errorf("net timeout"))
	if shouldMarkQueueEntryFailed(infraErr) {
		t.Fatal("infrastructure failure should not mark queue entry failed")
	}

	if !shouldMarkQueueEntryFailed(fmt.Errorf("plain error")) {
		t.Fatal("uncategorized errors should remain fail-closed and mark queue entry failed")
	}
}

func TestIsLoopKillSwitchSet(t *testing.T) {
	tmpDir := t.TempDir()
	killPath := filepath.Join(tmpDir, "KILL")
	cfg := rpiLoopSupervisorConfig{KillSwitchPath: killPath}

	set, err := isLoopKillSwitchSet(cfg)
	if err != nil {
		t.Fatalf("isLoopKillSwitchSet (missing): %v", err)
	}
	if set {
		t.Fatal("expected kill switch to be unset when file is missing")
	}

	if err := os.WriteFile(killPath, []byte("stop\n"), 0644); err != nil {
		t.Fatalf("write kill switch: %v", err)
	}
	set, err = isLoopKillSwitchSet(cfg)
	if err != nil {
		t.Fatalf("isLoopKillSwitchSet (present): %v", err)
	}
	if !set {
		t.Fatal("expected kill switch to be set when file exists")
	}
}

type loopSupervisorGlobals struct {
	rpiSupervisor            bool
	rpiRalph                 bool
	rpiFailurePolicy         string
	rpiCycleRetries          int
	rpiRetryBackoff          time.Duration
	rpiCycleDelay            time.Duration
	rpiCompile               bool
	rpiCompileInterval       time.Duration
	rpiCompileSince          string
	rpiCompileDefrag         bool
	rpiLease                 bool
	rpiLeasePath             string
	rpiLeaseTTL              time.Duration
	rpiDetachedHeal          bool
	rpiDetachedBranchPrefix  string
	rpiAutoClean             bool
	rpiAutoCleanStaleAfter   time.Duration
	rpiEnsureCleanup         bool
	rpiCleanupPruneWorktrees bool
	rpiCleanupPruneBranches  bool
	rpiGatePolicy            string
	rpiValidateFastScript    string
	rpiSecurityGateScript    string
	rpiLandingPolicy         string
	rpiLandingBranch         string
	rpiLandingCommitMessage  string
	rpiLandingLockPath       string
	rpiBDSyncPolicy          string
	rpiCommandTimeout        time.Duration
	rpiKillSwitchPath        string
}

func snapshotLoopSupervisorGlobals() loopSupervisorGlobals {
	return loopSupervisorGlobals{
		rpiSupervisor:            rpiSupervisor,
		rpiRalph:                 rpiRalph,
		rpiFailurePolicy:         rpiFailurePolicy,
		rpiCycleRetries:          rpiCycleRetries,
		rpiRetryBackoff:          rpiRetryBackoff,
		rpiCycleDelay:            rpiCycleDelay,
		rpiCompile:               rpiCompile,
		rpiCompileInterval:       rpiCompileInterval,
		rpiCompileSince:          rpiCompileSince,
		rpiCompileDefrag:         rpiCompileDefrag,
		rpiLease:                 rpiLease,
		rpiLeasePath:             rpiLeasePath,
		rpiLeaseTTL:              rpiLeaseTTL,
		rpiDetachedHeal:          rpiDetachedHeal,
		rpiDetachedBranchPrefix:  rpiDetachedBranchPrefix,
		rpiAutoClean:             rpiAutoClean,
		rpiAutoCleanStaleAfter:   rpiAutoCleanStaleAfter,
		rpiEnsureCleanup:         rpiEnsureCleanup,
		rpiCleanupPruneWorktrees: rpiCleanupPruneWorktrees,
		rpiCleanupPruneBranches:  rpiCleanupPruneBranches,
		rpiGatePolicy:            rpiGatePolicy,
		rpiValidateFastScript:    rpiValidateFastScript,
		rpiSecurityGateScript:    rpiSecurityGateScript,
		rpiLandingPolicy:         rpiLandingPolicy,
		rpiLandingBranch:         rpiLandingBranch,
		rpiLandingCommitMessage:  rpiLandingCommitMessage,
		rpiLandingLockPath:       rpiLandingLockPath,
		rpiBDSyncPolicy:          rpiBDSyncPolicy,
		rpiCommandTimeout:        rpiCommandTimeout,
		rpiKillSwitchPath:        rpiKillSwitchPath,
	}
}

func restoreLoopSupervisorGlobals(prev loopSupervisorGlobals) {
	rpiSupervisor = prev.rpiSupervisor
	rpiRalph = prev.rpiRalph
	rpiFailurePolicy = prev.rpiFailurePolicy
	rpiCycleRetries = prev.rpiCycleRetries
	rpiRetryBackoff = prev.rpiRetryBackoff
	rpiCycleDelay = prev.rpiCycleDelay
	rpiCompile = prev.rpiCompile
	rpiCompileInterval = prev.rpiCompileInterval
	rpiCompileSince = prev.rpiCompileSince
	rpiCompileDefrag = prev.rpiCompileDefrag
	rpiLease = prev.rpiLease
	rpiLeasePath = prev.rpiLeasePath
	rpiLeaseTTL = prev.rpiLeaseTTL
	rpiDetachedHeal = prev.rpiDetachedHeal
	rpiDetachedBranchPrefix = prev.rpiDetachedBranchPrefix
	rpiAutoClean = prev.rpiAutoClean
	rpiAutoCleanStaleAfter = prev.rpiAutoCleanStaleAfter
	rpiEnsureCleanup = prev.rpiEnsureCleanup
	rpiCleanupPruneWorktrees = prev.rpiCleanupPruneWorktrees
	rpiCleanupPruneBranches = prev.rpiCleanupPruneBranches
	rpiGatePolicy = prev.rpiGatePolicy
	rpiValidateFastScript = prev.rpiValidateFastScript
	rpiSecurityGateScript = prev.rpiSecurityGateScript
	rpiLandingPolicy = prev.rpiLandingPolicy
	rpiLandingBranch = prev.rpiLandingBranch
	rpiLandingCommitMessage = prev.rpiLandingCommitMessage
	rpiLandingLockPath = prev.rpiLandingLockPath
	rpiBDSyncPolicy = prev.rpiBDSyncPolicy
	rpiCommandTimeout = prev.rpiCommandTimeout
	rpiKillSwitchPath = prev.rpiKillSwitchPath
}

func newLoopSupervisorTestCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "test-loop"}
	cmd.Flags().String("failure-policy", "stop", "")
	cmd.Flags().Int("cycle-retries", 0, "")
	cmd.Flags().Duration("cycle-delay", 0, "")
	cmd.Flags().Bool("compile", false, "")
	cmd.Flags().Duration("compile-interval", 30*time.Minute, "")
	cmd.Flags().String("compile-since", "26h", "")
	cmd.Flags().Bool("compile-defrag", false, "")
	cmd.Flags().Bool("lease", false, "")
	cmd.Flags().Bool("ralph", false, "")
	cmd.Flags().Bool("detached-heal", false, "")
	cmd.Flags().Bool("auto-clean", false, "")
	cmd.Flags().Bool("ensure-cleanup", false, "")
	cmd.Flags().Bool("cleanup-prune-branches", false, "")
	cmd.Flags().String("gate-policy", "off", "")
	cmd.Flags().String("landing-lock-path", "", "")
	cmd.Flags().Duration("command-timeout", 20*time.Minute, "")
	return cmd
}

func TestDeferSupervisorCleanup_NoError(t *testing.T) {
	tmpDir := t.TempDir()

	// When cleanup succeeds and retErr is nil, deferSupervisorCleanup returns nil.
	// executeRPICleanup succeeds on dirs without stale runs (returns nil),
	// so this exercises the cleanupErr==nil && retErr==nil path.
	cfg := rpiLoopSupervisorConfig{
		AutoCleanStaleAfter:   24 * time.Hour,
		CleanupPruneWorktrees: false,
		CleanupPruneBranches:  false,
	}

	result := deferSupervisorCleanup(tmpDir, cfg, nil)
	if result != nil {
		t.Fatalf("expected nil when both cleanup and retErr succeed, got: %v", result)
	}
}

func TestDeferSupervisorCleanup_WithError(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := rpiLoopSupervisorConfig{
		AutoCleanStaleAfter:   24 * time.Hour,
		CleanupPruneWorktrees: false,
		CleanupPruneBranches:  false,
	}

	originalErr := fmt.Errorf("original cycle failure")
	// With no git repo, cleanup will also fail. But when retErr is non-nil,
	// the original error should be returned (not the cleanup error).
	result := deferSupervisorCleanup(tmpDir, cfg, originalErr)
	if result == nil {
		t.Fatal("expected error to propagate, got nil")
	}
	if result.Error() != originalErr.Error() {
		t.Fatalf("expected original error %q to propagate, got %q", originalErr.Error(), result.Error())
	}
}

func TestHealDetachedHead_NotDetached(t *testing.T) {
	tmpDir := t.TempDir()
	// When DetachedHeal is false, healDetachedHeadIfNeeded should be a no-op
	cfg := rpiLoopSupervisorConfig{
		DetachedHeal: false,
	}
	err := healDetachedHeadIfNeeded(tmpDir, cfg)
	if err != nil {
		t.Fatalf("expected no-op when DetachedHeal=false, got: %v", err)
	}
}

func TestEnsureLoopAttachedBranch_CreatesBranch(t *testing.T) {
	repoPath := t.TempDir()

	runGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s failed: %v (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
		}
		return strings.TrimSpace(string(out))
	}

	// Set up a git repo with a commit, then detach HEAD
	runGit("init", "-q")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test")
	runGit("config", "commit.gpgsign", "false")
	runGit("checkout", "-q", "-b", "main")
	runGit("commit", "-q", "--allow-empty", "-m", "init")
	commitHash := runGit("rev-parse", "HEAD")

	// Detach HEAD
	runGit("checkout", "-q", "--detach", commitHash)

	// Verify we're detached
	cmd := exec.Command("git", "symbolic-ref", "HEAD")
	cmd.Dir = repoPath
	if err := cmd.Run(); err == nil {
		t.Fatal("expected detached HEAD state")
	}

	branch, healed, err := ensureLoopAttachedBranch(repoPath, "ao-loop-")
	if err != nil {
		t.Fatalf("ensureLoopAttachedBranch: %v", err)
	}
	if !healed {
		t.Fatal("expected healed=true for detached HEAD")
	}
	if !strings.HasPrefix(branch, "ao-loop-") {
		t.Fatalf("expected branch to start with 'ao-loop-', got %q", branch)
	}

	// Verify HEAD is now attached to a branch
	currentBranch := runGit("symbolic-ref", "--short", "HEAD")
	if currentBranch != branch {
		t.Fatalf("expected HEAD on branch %q, got %q", branch, currentBranch)
	}
}

func TestRunBDSyncIfNeeded_Always(t *testing.T) {
	prevRunner := loopCommandRunner
	prevLookPath := loopLookPath
	defer func() {
		loopCommandRunner = prevRunner
		loopLookPath = prevLookPath
	}()

	loopLookPath = func(_ string) (string, error) {
		return "/usr/bin/bd", nil
	}

	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, ".beads"), 0755); err != nil {
		t.Fatal(err)
	}

	var calledWith []string
	loopCommandRunner = func(_ string, _ time.Duration, name string, args ...string) error {
		calledWith = append(calledWith, name+" "+strings.Join(args, " "))
		return nil
	}

	cfg := rpiLoopSupervisorConfig{
		BDSyncPolicy:   loopBDSyncPolicyAlways,
		BDCommand:      "bd",
		CommandTimeout: time.Minute,
	}
	if err := runBDSyncIfNeeded(tmpDir, cfg); err != nil {
		t.Fatalf("runBDSyncIfNeeded: %v", err)
	}

	found := false
	for _, call := range calledWith {
		if call == "bd export -o /dev/null" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'bd export -o /dev/null' call with always policy, got calls: %v", calledWith)
	}
}

func TestRunBDSyncIfNeeded_Never(t *testing.T) {
	prevRunner := loopCommandRunner
	defer func() { loopCommandRunner = prevRunner }()

	var called bool
	loopCommandRunner = func(_ string, _ time.Duration, name string, args ...string) error {
		called = true
		return nil
	}

	cfg := rpiLoopSupervisorConfig{
		BDSyncPolicy:   loopBDSyncPolicyNever,
		BDCommand:      "bd",
		CommandTimeout: time.Minute,
	}
	if err := runBDSyncIfNeeded(t.TempDir(), cfg); err != nil {
		t.Fatalf("runBDSyncIfNeeded: %v", err)
	}
	if called {
		t.Fatal("expected no command calls with never policy")
	}
}

func TestSyncRebaseAndPush_Success(t *testing.T) {
	prevRunner := loopCommandRunner
	prevOutputRunner := loopCommandOutputRunner
	defer func() {
		loopCommandRunner = prevRunner
		loopCommandOutputRunner = prevOutputRunner
	}()

	var runnerCalls []string
	loopCommandRunner = func(_ string, _ time.Duration, name string, args ...string) error {
		runnerCalls = append(runnerCalls, name+" "+strings.Join(args, " "))
		return nil
	}
	loopCommandOutputRunner = func(_ string, _ time.Duration, name string, args ...string) (string, error) {
		// resolveLandingBranch calls symbolic-ref first
		if name == "git" && len(args) > 0 && args[0] == "symbolic-ref" {
			return "origin/main", nil
		}
		return "", nil
	}

	cfg := rpiLoopSupervisorConfig{
		BDSyncPolicy:   loopBDSyncPolicyNever,
		CommandTimeout: time.Minute,
	}
	err := syncRebaseAndPush(t.TempDir(), cfg)
	if err != nil {
		t.Fatalf("syncRebaseAndPush: %v", err)
	}

	// Verify fetch, rebase, and push were called in order
	expectedSequence := []string{
		"git fetch origin main",
		"git rebase origin/main",
		"git push origin HEAD:main",
	}
	matchIdx := 0
	for _, call := range runnerCalls {
		if matchIdx < len(expectedSequence) && call == expectedSequence[matchIdx] {
			matchIdx++
		}
	}
	if matchIdx != len(expectedSequence) {
		t.Fatalf("expected git fetch/rebase/push sequence, got calls: %v", runnerCalls)
	}
}

func TestBuildCycleEngineOptions_NoDashboard(t *testing.T) {
	cfg := rpiLoopSupervisorConfig{
		AutoClean:           true,
		AutoCleanStaleAfter: 48 * time.Hour,
		RuntimeMode:         "direct",
	}
	opts := buildCycleEngineOptions(t.TempDir(), cfg)
	if !opts.NoDashboard {
		t.Error("expected NoDashboard=true for supervisor cycle engine options")
	}
}

func TestEmitCycleTelemetry_Success(t *testing.T) {
	dir := t.TempDir()
	emitCycleTelemetry(dir, 3, "improve docs", 5*time.Second, nil)

	telePath := filepath.Join(dir, ".agents", "rpi", "telemetry.jsonl")
	data, err := os.ReadFile(telePath)
	if err != nil {
		t.Fatalf("read telemetry file: %v", err)
	}
	var entry cycleTelemetryEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &entry); err != nil {
		t.Fatalf("unmarshal telemetry entry: %v", err)
	}
	if entry.Cycle != 3 {
		t.Errorf("expected cycle=3, got %d", entry.Cycle)
	}
	if entry.Goal != "improve docs" {
		t.Errorf("expected goal='improve docs', got %q", entry.Goal)
	}
	if entry.DurationMS != 5000 {
		t.Errorf("expected duration_ms=5000, got %d", entry.DurationMS)
	}
	if entry.Status != "success" {
		t.Errorf("expected status=success, got %q", entry.Status)
	}
	if entry.ErrorMessage != "" {
		t.Errorf("expected empty error_message, got %q", entry.ErrorMessage)
	}
	if entry.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}
}

func TestEmitCycleTelemetry_Failure(t *testing.T) {
	dir := t.TempDir()
	emitCycleTelemetry(dir, 7, "fix bug", 2*time.Second, fmt.Errorf("cycle failed: timeout"))

	telePath := filepath.Join(dir, ".agents", "rpi", "telemetry.jsonl")
	data, err := os.ReadFile(telePath)
	if err != nil {
		t.Fatalf("read telemetry file: %v", err)
	}
	var entry cycleTelemetryEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &entry); err != nil {
		t.Fatalf("unmarshal telemetry entry: %v", err)
	}
	if entry.Status != "failure" {
		t.Errorf("expected status=failure, got %q", entry.Status)
	}
	if entry.ErrorMessage != "cycle failed: timeout" {
		t.Errorf("expected error_message='cycle failed: timeout', got %q", entry.ErrorMessage)
	}
}

func TestStartLeaseHeartbeat_UpdatesFile(t *testing.T) {
	dir := t.TempDir()
	leasePath := filepath.Join(dir, "lease.json")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := startLeaseHeartbeat(ctx, leasePath, 100*time.Millisecond, "test-run-123")
	defer stop()

	// Wait enough for at least one heartbeat tick (ttl/2 = 50ms).
	time.Sleep(120 * time.Millisecond)

	data, err := os.ReadFile(leasePath)
	if err != nil {
		t.Fatalf("read lease heartbeat file: %v", err)
	}
	var entry leaseHeartbeatEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("unmarshal heartbeat: %v", err)
	}
	if entry.Holder != "test-run-123" {
		t.Errorf("expected holder='test-run-123', got %q", entry.Holder)
	}
	if entry.Heartbeat == "" {
		t.Error("expected non-empty heartbeat timestamp")
	}
	if entry.TTLMS != 100 {
		t.Errorf("expected ttl_ms=100, got %d", entry.TTLMS)
	}
}
