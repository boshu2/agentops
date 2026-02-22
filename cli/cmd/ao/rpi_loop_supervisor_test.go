package main

import (
	"fmt"
	"os"
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
	rpiLease = false
	rpiDetachedHeal = false
	rpiAutoClean = false
	rpiEnsureCleanup = false
	rpiGatePolicy = "off"
	rpiLandingPolicy = "off"
	rpiBDSyncPolicy = "auto"
	rpiLeaseTTL = 2 * time.Minute
	rpiAutoCleanStaleAfter = 24 * time.Hour
	rpiLeasePath = ".agents/rpi/supervisor.lock"

	cmd := newLoopSupervisorTestCommand()

	cfg, err := resolveLoopSupervisorConfig(cmd, t.TempDir())
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
	if !cfg.LeaseEnabled {
		t.Fatal("expected lease to be enabled in supervisor defaults")
	}
	if !cfg.DetachedHeal {
		t.Fatal("expected detached heal to be enabled in supervisor defaults")
	}
	if !cfg.AutoClean {
		t.Fatal("expected auto-clean to be enabled in supervisor defaults")
	}
	if !cfg.EnsureCleanup {
		t.Fatal("expected ensure-cleanup to be enabled in supervisor defaults")
	}
	if cfg.GatePolicy != loopGatePolicyRequired {
		t.Fatalf("gate policy: got %q, want %q", cfg.GatePolicy, loopGatePolicyRequired)
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

func TestRunSupervisorLanding_SyncPush_RebaseFailureAborts(t *testing.T) {
	prevRunner := loopCommandRunner
	prevOutputRunner := loopCommandOutputRunner
	defer func() {
		loopCommandRunner = prevRunner
		loopCommandOutputRunner = prevOutputRunner
	}()

	var runnerCalls []string
	var outputCalls []string
	loopCommandRunner = func(_ string, _ time.Duration, name string, args ...string) error {
		runnerCalls = append(runnerCalls, name+" "+strings.Join(args, " "))
		if name == "git" && len(args) >= 2 && args[0] == "rebase" && args[1] == "origin/main" {
			return fmt.Errorf("simulated rebase conflict")
		}
		return nil
	}
	loopCommandOutputRunner = func(_ string, _ time.Duration, name string, args ...string) (string, error) {
		outputCalls = append(outputCalls, name+" "+strings.Join(args, " "))
		if name == "git" && len(args) > 0 && args[0] == "status" {
			return "", nil
		}
		if name == "git" && len(args) > 0 && args[0] == "symbolic-ref" {
			return "origin/main", nil
		}
		if name == "git" && len(args) >= 2 && args[0] == "rebase" && args[1] == "--abort" {
			return "", nil
		}
		return "", nil
	}

	cfg := rpiLoopSupervisorConfig{
		LandingPolicy:  loopLandingPolicySyncPush,
		BDSyncPolicy:   loopBDSyncPolicyNever,
		CommandTimeout: time.Minute,
	}
	err := runSupervisorLanding(t.TempDir(), cfg, 1, 1, "ship", &landingScope{
		baselineDirtyPaths: map[string]struct{}{},
	})
	if err == nil || !strings.Contains(err.Error(), "landing rebase failed") {
		t.Fatalf("expected rebase failure, got: %v", err)
	}
	if !strings.Contains(err.Error(), "state recovered") {
		t.Fatalf("expected state recovery details in error, got: %v", err)
	}

	foundAbort := false
	for _, call := range outputCalls {
		if call == "git rebase --abort" {
			foundAbort = true
			break
		}
	}
	if !foundAbort {
		t.Fatalf("expected git rebase --abort call, got output calls: %v", outputCalls)
	}

	foundStatus := false
	for _, call := range runnerCalls {
		if call == "git status -sb" {
			foundStatus = true
			break
		}
	}
	if !foundStatus {
		t.Fatalf("expected git status -sb recovery call, got runner calls: %v", runnerCalls)
	}
}

func TestRunSupervisorLanding_SyncPush_FetchFailure_RecoversState(t *testing.T) {
	prevRunner := loopCommandRunner
	prevOutputRunner := loopCommandOutputRunner
	defer func() {
		loopCommandRunner = prevRunner
		loopCommandOutputRunner = prevOutputRunner
	}()

	var runnerCalls []string
	var outputCalls []string
	loopCommandRunner = func(_ string, _ time.Duration, name string, args ...string) error {
		runnerCalls = append(runnerCalls, name+" "+strings.Join(args, " "))
		if name == "git" && len(args) >= 3 && args[0] == "fetch" && args[1] == "origin" && args[2] == "main" {
			return fmt.Errorf("simulated fetch outage")
		}
		return nil
	}
	loopCommandOutputRunner = func(_ string, _ time.Duration, name string, args ...string) (string, error) {
		outputCalls = append(outputCalls, name+" "+strings.Join(args, " "))
		if name == "git" && len(args) >= 2 && args[0] == "status" && args[1] == "--porcelain" {
			return "", nil
		}
		if name == "git" && len(args) > 0 && args[0] == "symbolic-ref" {
			return "origin/main", nil
		}
		if name == "git" && len(args) >= 2 && args[0] == "rebase" && args[1] == "--abort" {
			return "fatal: No rebase in progress?", fmt.Errorf("exit status 128")
		}
		return "", nil
	}

	cfg := rpiLoopSupervisorConfig{
		LandingPolicy:  loopLandingPolicySyncPush,
		BDSyncPolicy:   loopBDSyncPolicyNever,
		CommandTimeout: time.Minute,
	}
	err := runSupervisorLanding(t.TempDir(), cfg, 1, 1, "ship", &landingScope{
		baselineDirtyPaths: map[string]struct{}{},
	})
	if err == nil || !strings.Contains(err.Error(), "landing fetch failed") {
		t.Fatalf("expected fetch failure, got: %v", err)
	}
	if !strings.Contains(err.Error(), "state recovered") {
		t.Fatalf("expected state recovery details in error, got: %v", err)
	}

	foundAbort := false
	for _, call := range outputCalls {
		if call == "git rebase --abort" {
			foundAbort = true
			break
		}
	}
	if !foundAbort {
		t.Fatalf("expected git rebase --abort call, got output calls: %v", outputCalls)
	}

	foundStatus := false
	for _, call := range runnerCalls {
		if call == "git status -sb" {
			foundStatus = true
			break
		}
	}
	if !foundStatus {
		t.Fatalf("expected git status -sb recovery call, got runner calls: %v", runnerCalls)
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

type loopSupervisorGlobals struct {
	rpiSupervisor            bool
	rpiFailurePolicy         string
	rpiCycleRetries          int
	rpiRetryBackoff          time.Duration
	rpiCycleDelay            time.Duration
	rpiLease                 bool
	rpiLeasePath             string
	rpiLeaseTTL              time.Duration
	rpiDetachedHeal          bool
	rpiDetachedBranchPrefix  string
	rpiAutoClean             bool
	rpiAutoCleanStaleAfter   time.Duration
	rpiEnsureCleanup         bool
	rpiCleanupPruneWorktrees bool
	rpiGatePolicy            string
	rpiValidateFastScript    string
	rpiSecurityGateScript    string
	rpiLandingPolicy         string
	rpiLandingBranch         string
	rpiLandingCommitMessage  string
	rpiBDSyncPolicy          string
	rpiCommandTimeout        time.Duration
}

func snapshotLoopSupervisorGlobals() loopSupervisorGlobals {
	return loopSupervisorGlobals{
		rpiSupervisor:            rpiSupervisor,
		rpiFailurePolicy:         rpiFailurePolicy,
		rpiCycleRetries:          rpiCycleRetries,
		rpiRetryBackoff:          rpiRetryBackoff,
		rpiCycleDelay:            rpiCycleDelay,
		rpiLease:                 rpiLease,
		rpiLeasePath:             rpiLeasePath,
		rpiLeaseTTL:              rpiLeaseTTL,
		rpiDetachedHeal:          rpiDetachedHeal,
		rpiDetachedBranchPrefix:  rpiDetachedBranchPrefix,
		rpiAutoClean:             rpiAutoClean,
		rpiAutoCleanStaleAfter:   rpiAutoCleanStaleAfter,
		rpiEnsureCleanup:         rpiEnsureCleanup,
		rpiCleanupPruneWorktrees: rpiCleanupPruneWorktrees,
		rpiGatePolicy:            rpiGatePolicy,
		rpiValidateFastScript:    rpiValidateFastScript,
		rpiSecurityGateScript:    rpiSecurityGateScript,
		rpiLandingPolicy:         rpiLandingPolicy,
		rpiLandingBranch:         rpiLandingBranch,
		rpiLandingCommitMessage:  rpiLandingCommitMessage,
		rpiBDSyncPolicy:          rpiBDSyncPolicy,
		rpiCommandTimeout:        rpiCommandTimeout,
	}
}

func restoreLoopSupervisorGlobals(prev loopSupervisorGlobals) {
	rpiSupervisor = prev.rpiSupervisor
	rpiFailurePolicy = prev.rpiFailurePolicy
	rpiCycleRetries = prev.rpiCycleRetries
	rpiRetryBackoff = prev.rpiRetryBackoff
	rpiCycleDelay = prev.rpiCycleDelay
	rpiLease = prev.rpiLease
	rpiLeasePath = prev.rpiLeasePath
	rpiLeaseTTL = prev.rpiLeaseTTL
	rpiDetachedHeal = prev.rpiDetachedHeal
	rpiDetachedBranchPrefix = prev.rpiDetachedBranchPrefix
	rpiAutoClean = prev.rpiAutoClean
	rpiAutoCleanStaleAfter = prev.rpiAutoCleanStaleAfter
	rpiEnsureCleanup = prev.rpiEnsureCleanup
	rpiCleanupPruneWorktrees = prev.rpiCleanupPruneWorktrees
	rpiGatePolicy = prev.rpiGatePolicy
	rpiValidateFastScript = prev.rpiValidateFastScript
	rpiSecurityGateScript = prev.rpiSecurityGateScript
	rpiLandingPolicy = prev.rpiLandingPolicy
	rpiLandingBranch = prev.rpiLandingBranch
	rpiLandingCommitMessage = prev.rpiLandingCommitMessage
	rpiBDSyncPolicy = prev.rpiBDSyncPolicy
	rpiCommandTimeout = prev.rpiCommandTimeout
}

func newLoopSupervisorTestCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "test-loop"}
	cmd.Flags().String("failure-policy", "stop", "")
	cmd.Flags().Int("cycle-retries", 0, "")
	cmd.Flags().Duration("cycle-delay", 0, "")
	cmd.Flags().Bool("lease", false, "")
	cmd.Flags().Bool("detached-heal", false, "")
	cmd.Flags().Bool("auto-clean", false, "")
	cmd.Flags().Bool("ensure-cleanup", false, "")
	cmd.Flags().String("gate-policy", "off", "")
	cmd.Flags().Duration("command-timeout", 20*time.Minute, "")
	return cmd
}
