package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ===========================================================================
// Coverage tests for rpi_loop_supervisor.go — targeting zero-coverage functions
// ===========================================================================

// --- buildBaseLoopConfig ---

func TestBuildBaseLoopConfig(t *testing.T) {
	prev := snapshotLoopSupervisorGlobals()
	defer restoreLoopSupervisorGlobals(prev)

	configureBaseLoopConfigGlobals()
	cfg := buildBaseLoopConfig()

	assertBaseLoopRetryConfig(t, cfg)
	assertBaseLoopCompileConfig(t, cfg)
	assertBaseLoopLeaseConfig(t, cfg)
	assertBaseLoopCleanupConfig(t, cfg)
	assertBaseLoopGateConfig(t, cfg)
	assertBaseLoopLandingConfig(t, cfg)
	assertBaseLoopCommandConfig(t, cfg)
}

func configureBaseLoopConfigGlobals() {
	rpiFailurePolicy = "continue"
	rpiCycleRetries = 5
	rpiRetryBackoff = 30 * time.Second
	rpiCycleDelay = 10 * time.Minute
	rpiCompile = true
	rpiCompileInterval = 45 * time.Minute
	rpiCompileSince = "12h"
	rpiCompileDefrag = true
	rpiLease = true
	rpiLeasePath = "/tmp/test.lock"
	rpiLeaseTTL = 5 * time.Minute
	rpiDetachedHeal = true
	rpiDetachedBranchPrefix = "rpi-"
	rpiAutoClean = true
	rpiAutoCleanStaleAfter = 48 * time.Hour
	rpiEnsureCleanup = true
	rpiCleanupPruneWorktrees = true
	rpiCleanupPruneBranches = true
	rpiGatePolicy = "best-effort"
	rpiValidateFastScript = "validate.sh"
	rpiSecurityGateScript = "security.sh"
	rpiLandingPolicy = "commit"
	rpiLandingBranch = "main"
	rpiLandingCommitMessage = "auto commit"
	rpiLandingLockPath = "/tmp/landing.lock"
	rpiBDSyncPolicy = "always"
	rpiCommandTimeout = 30 * time.Minute
	rpiKillSwitchPath = "/tmp/KILL"
}

func assertBaseLoopRetryConfig(t *testing.T, cfg rpiLoopSupervisorConfig) {
	t.Helper()
	if cfg.FailurePolicy != "continue" {
		t.Errorf("FailurePolicy = %q, want continue", cfg.FailurePolicy)
	}
	if cfg.CycleRetries != 5 {
		t.Errorf("CycleRetries = %d, want 5", cfg.CycleRetries)
	}
	if cfg.RetryBackoff != 30*time.Second {
		t.Errorf("RetryBackoff = %v, want 30s", cfg.RetryBackoff)
	}
	if cfg.CycleDelay != 10*time.Minute {
		t.Errorf("CycleDelay = %v, want 10m", cfg.CycleDelay)
	}
}

func assertBaseLoopCompileConfig(t *testing.T, cfg rpiLoopSupervisorConfig) {
	t.Helper()
	if !cfg.CompileEnabled {
		t.Error("expected CompileEnabled=true")
	}
	if cfg.CompileInterval != 45*time.Minute {
		t.Errorf("CompileInterval = %v, want 45m", cfg.CompileInterval)
	}
	if cfg.CompileSince != "12h" {
		t.Errorf("CompileSince = %q, want 12h", cfg.CompileSince)
	}
	if !cfg.CompileDefrag {
		t.Error("expected CompileDefrag=true")
	}
}

func assertBaseLoopLeaseConfig(t *testing.T, cfg rpiLoopSupervisorConfig) {
	t.Helper()
	if !cfg.LeaseEnabled {
		t.Error("expected LeaseEnabled=true")
	}
	if cfg.LeasePath != "/tmp/test.lock" {
		t.Errorf("LeasePath = %q", cfg.LeasePath)
	}
	if cfg.LeaseTTL != 5*time.Minute {
		t.Errorf("LeaseTTL = %v", cfg.LeaseTTL)
	}
	if !cfg.DetachedHeal {
		t.Error("expected DetachedHeal=true")
	}
	if cfg.DetachedBranchPrefix != "rpi-" {
		t.Errorf("DetachedBranchPrefix = %q", cfg.DetachedBranchPrefix)
	}
}

func assertBaseLoopCleanupConfig(t *testing.T, cfg rpiLoopSupervisorConfig) {
	t.Helper()
	if !cfg.AutoClean {
		t.Error("expected AutoClean=true")
	}
	if cfg.AutoCleanStaleAfter != 48*time.Hour {
		t.Errorf("AutoCleanStaleAfter = %v", cfg.AutoCleanStaleAfter)
	}
}

func assertBaseLoopGateConfig(t *testing.T, cfg rpiLoopSupervisorConfig) {
	t.Helper()
	if cfg.GatePolicy != "best-effort" {
		t.Errorf("GatePolicy = %q", cfg.GatePolicy)
	}
	if cfg.ValidateFastScript != "validate.sh" {
		t.Errorf("ValidateFastScript = %q", cfg.ValidateFastScript)
	}
	if cfg.SecurityGateScript != "security.sh" {
		t.Errorf("SecurityGateScript = %q", cfg.SecurityGateScript)
	}
}

func assertBaseLoopLandingConfig(t *testing.T, cfg rpiLoopSupervisorConfig) {
	t.Helper()
	if cfg.LandingPolicy != "commit" {
		t.Errorf("LandingPolicy = %q", cfg.LandingPolicy)
	}
	if cfg.LandingBranch != "main" {
		t.Errorf("LandingBranch = %q", cfg.LandingBranch)
	}
	if cfg.LandingCommitMessage != "auto commit" {
		t.Errorf("LandingCommitMessage = %q", cfg.LandingCommitMessage)
	}
	if cfg.BDSyncPolicy != "always" {
		t.Errorf("BDSyncPolicy = %q", cfg.BDSyncPolicy)
	}
}

func assertBaseLoopCommandConfig(t *testing.T, cfg rpiLoopSupervisorConfig) {
	t.Helper()
	if cfg.CommandTimeout != 30*time.Minute {
		t.Errorf("CommandTimeout = %v", cfg.CommandTimeout)
	}
	if cfg.KillSwitchPath != "/tmp/KILL" {
		t.Errorf("KillSwitchPath = %q", cfg.KillSwitchPath)
	}
}

// --- validateLoopNumericConstraints ---

func TestValidateLoopNumericConstraints(t *testing.T) {
	tests := []struct {
		name    string
		cfg     rpiLoopSupervisorConfig
		wantErr string
	}{
		{
			"valid config",
			rpiLoopSupervisorConfig{CycleRetries: 3, RetryBackoff: time.Second, CycleDelay: time.Minute, CommandTimeout: time.Minute},
			"",
		},
		{
			"negative retries",
			rpiLoopSupervisorConfig{CycleRetries: -1},
			"cycle-retries",
		},
		{
			"negative backoff",
			rpiLoopSupervisorConfig{CycleRetries: 0, RetryBackoff: -1},
			"retry-backoff",
		},
		{
			"negative delay",
			rpiLoopSupervisorConfig{CycleRetries: 0, CycleDelay: -1},
			"cycle-delay",
		},
		{
			"negative compile interval",
			rpiLoopSupervisorConfig{CycleRetries: 0, CompileInterval: -1},
			"compile-interval",
		},
		{
			"negative timeout",
			rpiLoopSupervisorConfig{CycleRetries: 0, CommandTimeout: -1},
			"command-timeout",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLoopNumericConstraints(&tt.cfg)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			} else {
				if err == nil {
					t.Fatal("expected error")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

// --- applyLoopTimingDefaults ---

func TestApplyLoopTimingDefaults(t *testing.T) {
	cfg := rpiLoopSupervisorConfig{
		LeaseTTL:            0,
		CommandTimeout:      0,
		AutoCleanStaleAfter: 0,
	}
	applyLoopTimingDefaults(&cfg, nil)

	if cfg.LeaseTTL != 2*time.Minute {
		t.Errorf("LeaseTTL = %v, want 2m", cfg.LeaseTTL)
	}
	if cfg.CommandTimeout != defaultLoopCommandTimeout {
		t.Errorf("CommandTimeout = %v, want %v", cfg.CommandTimeout, defaultLoopCommandTimeout)
	}
	if cfg.AutoCleanStaleAfter != 24*time.Hour {
		t.Errorf("AutoCleanStaleAfter = %v, want 24h", cfg.AutoCleanStaleAfter)
	}
}

// --- applyLoopPathDefaults ---

func TestApplyLoopPathDefaults(t *testing.T) {
	cfg := rpiLoopSupervisorConfig{}
	applyLoopPathDefaults(&cfg)

	if cfg.LeasePath == "" {
		t.Error("expected LeasePath to be set")
	}
	if cfg.LandingLockPath == "" {
		t.Error("expected LandingLockPath to be set")
	}
	if cfg.KillSwitchPath == "" {
		t.Error("expected KillSwitchPath to be set")
	}
}

func TestApplyLoopPathDefaults_PreserveExisting(t *testing.T) {
	cfg := rpiLoopSupervisorConfig{
		LeasePath:       "/custom/lease.lock",
		LandingLockPath: "/custom/landing.lock",
		KillSwitchPath:  "/custom/KILL",
	}
	applyLoopPathDefaults(&cfg)

	if cfg.LeasePath != "/custom/lease.lock" {
		t.Errorf("LeasePath = %q, expected custom path preserved", cfg.LeasePath)
	}
	if cfg.LandingLockPath != "/custom/landing.lock" {
		t.Errorf("LandingLockPath = %q, expected custom path preserved", cfg.LandingLockPath)
	}
	if cfg.KillSwitchPath != "/custom/KILL" {
		t.Errorf("KillSwitchPath = %q, expected custom path preserved", cfg.KillSwitchPath)
	}
}

// --- validateLoopConfigPolicies ---

func TestValidateLoopConfigPolicies_AllValid(t *testing.T) {
	validCombos := []rpiLoopSupervisorConfig{
		{FailurePolicy: "stop", GatePolicy: "off", LandingPolicy: "off", BDSyncPolicy: "auto"},
		{FailurePolicy: "continue", GatePolicy: "best-effort", LandingPolicy: "commit", BDSyncPolicy: "always"},
		{FailurePolicy: "stop", GatePolicy: "required", LandingPolicy: "sync-push", BDSyncPolicy: "never"},
	}
	for _, cfg := range validCombos {
		if err := validateLoopConfigPolicies(cfg); err != nil {
			t.Errorf("unexpected error for valid config: %v", err)
		}
	}
}

// --- resolveLoopConfigPaths ---

func TestResolveLoopConfigPaths(t *testing.T) {
	cwd := "/home/user/repo"
	cfg := rpiLoopSupervisorConfig{
		LeasePath:       ".agents/rpi/supervisor.lock",
		LandingLockPath: ".agents/rpi/landing.lock",
		KillSwitchPath:  ".agents/rpi/KILL",
	}
	resolveLoopConfigPaths(&cfg, cwd)

	if !filepath.IsAbs(cfg.LeasePath) {
		t.Errorf("LeasePath should be absolute, got %q", cfg.LeasePath)
	}
	if !filepath.IsAbs(cfg.LandingLockPath) {
		t.Errorf("LandingLockPath should be absolute, got %q", cfg.LandingLockPath)
	}
	if !filepath.IsAbs(cfg.KillSwitchPath) {
		t.Errorf("KillSwitchPath should be absolute, got %q", cfg.KillSwitchPath)
	}
}

func TestResolveLoopConfigPaths_AlreadyAbsolute(t *testing.T) {
	cfg := rpiLoopSupervisorConfig{
		LeasePath:       "/abs/lease.lock",
		LandingLockPath: "/abs/landing.lock",
		KillSwitchPath:  "/abs/KILL",
	}
	resolveLoopConfigPaths(&cfg, "/some/dir")

	if cfg.LeasePath != "/abs/lease.lock" {
		t.Errorf("already-absolute LeasePath should not change, got %q", cfg.LeasePath)
	}
}

// --- cycleFailureError ---

func TestCycleFailureError_Error(t *testing.T) {
	inner := fmt.Errorf("root cause")
	cfe := &cycleFailureError{kind: cycleFailureTask, err: inner}
	if cfe.Error() != "root cause" {
		t.Errorf("Error() = %q, want 'root cause'", cfe.Error())
	}
}

func TestCycleFailureError_Unwrap(t *testing.T) {
	inner := fmt.Errorf("root cause")
	cfe := &cycleFailureError{kind: cycleFailureInfrastructure, err: inner}
	if cfe.Unwrap() != inner {
		t.Error("Unwrap should return inner error")
	}
}

// --- wrapCycleFailure ---

func TestWrapCycleFailure_EmptyStage(t *testing.T) {
	err := fmt.Errorf("no stage")
	wrapped := wrapCycleFailure(cycleFailureTask, "", err)
	var cfe *cycleFailureError
	if !errors.As(wrapped, &cfe) {
		t.Fatal("expected cycleFailureError")
	}
	if cfe.Error() != "no stage" {
		t.Errorf("Error() = %q, want 'no stage'", cfe.Error())
	}
}

func TestWrapCycleFailure_WithStage(t *testing.T) {
	err := fmt.Errorf("base err")
	wrapped := wrapCycleFailure(cycleFailureTask, "commit", err)
	if !strings.Contains(wrapped.Error(), "commit") {
		t.Errorf("expected 'commit' in error, got %q", wrapped.Error())
	}
}

// --- shouldMarkQueueEntryFailed / isInfrastructureCycleFailure ---

func TestShouldMarkQueueEntryFailed(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"task failure", wrapCycleFailure(cycleFailureTask, "t", fmt.Errorf("fail")), true},
		{"infra failure", wrapCycleFailure(cycleFailureInfrastructure, "i", fmt.Errorf("fail")), false},
		{"plain error", fmt.Errorf("unknown"), true},
		{"nil error", nil, true}, // nil is not infra, so shouldMark returns true
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldMarkQueueEntryFailed(tt.err)
			if got != tt.want {
				t.Errorf("shouldMarkQueueEntryFailed = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsInfrastructureCycleFailure(t *testing.T) {
	infraErr := wrapCycleFailure(cycleFailureInfrastructure, "net", fmt.Errorf("timeout"))
	if !isInfrastructureCycleFailure(infraErr) {
		t.Error("expected infrastructure failure to be detected")
	}

	taskErr := wrapCycleFailure(cycleFailureTask, "build", fmt.Errorf("fail"))
	if isInfrastructureCycleFailure(taskErr) {
		t.Error("task failure should not be infrastructure")
	}

	plainErr := fmt.Errorf("plain error")
	if isInfrastructureCycleFailure(plainErr) {
		t.Error("plain error should not be infrastructure")
	}
}

// TestRenderLandingCommitMessage — canonical version in rpi_loop_supervisor_test.go

// --- appendDirtyPaths ---

func TestAppendDirtyPaths(t *testing.T) {
	paths := make(map[string]struct{})
	appendDirtyPaths(paths, "file1.go\nfile2.go\n\n  \nfile3.go")

	if len(paths) != 3 {
		t.Errorf("expected 3 paths, got %d", len(paths))
	}
	for _, p := range []string{"file1.go", "file2.go", "file3.go"} {
		if _, ok := paths[p]; !ok {
			t.Errorf("expected %q in paths", p)
		}
	}
}

// --- computeOwnedDirtyPaths ---

func TestComputeOwnedDirtyPaths(t *testing.T) {
	prevRunner := loopCommandOutputRunner
	defer func() { loopCommandOutputRunner = prevRunner }()

	loopCommandOutputRunner = func(_ string, _ time.Duration, name string, args ...string) (string, error) {
		if name == "git" && len(args) > 0 && args[0] == "diff" {
			return "file1.go\nfile2.go\nnew.go\n", nil
		}
		if name == "git" && len(args) > 0 && args[0] == "ls-files" {
			return "", nil
		}
		return "", nil
	}

	scope := &landingScope{
		baselineDirtyPaths: map[string]struct{}{
			"file1.go": {},
			"file2.go": {},
		},
	}

	owned, err := computeOwnedDirtyPaths(t.TempDir(), time.Minute, scope)
	if err != nil {
		t.Fatalf("computeOwnedDirtyPaths: %v", err)
	}
	if len(owned) != 1 {
		t.Fatalf("expected 1 owned path, got %d: %v", len(owned), owned)
	}
	if owned[0] != "new.go" {
		t.Errorf("expected new.go, got %q", owned[0])
	}
}

// TestIsNoRebaseInProgressMessage — canonical version in rpi_loop_supervisor_test.go

// --- runSupervisorLanding ---

func TestRunSupervisorLanding_UnsupportedPolicy(t *testing.T) {
	cfg := rpiLoopSupervisorConfig{
		LandingPolicy: "invalid-policy",
	}
	err := runSupervisorLanding(t.TempDir(), cfg, 1, 1, "test", nil)
	if err == nil {
		t.Fatal("expected error for unsupported landing policy")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("error = %q, want 'unsupported'", err.Error())
	}
}

func TestAcquireLandingLock_EmptyPath(t *testing.T) {
	cfg := rpiLoopSupervisorConfig{
		LandingPolicy:   loopLandingPolicyCommit,
		LandingLockPath: "",
	}
	lock, err := acquireLandingLock(t.TempDir(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lock != nil {
		t.Error("expected nil lock for empty path")
	}
}

// --- buildCycleEngineOptions ---

func TestBuildCycleEngineOptions(t *testing.T) {
	cwd := t.TempDir()
	cfg := rpiLoopSupervisorConfig{
		AutoClean:           true,
		AutoCleanStaleAfter: 48 * time.Hour,
		RuntimeMode:         "stream",
		RuntimeCommand:      "claude",
		AOCommand:           "ao",
		BDCommand:           "bd",
		TmuxCommand:         "tmux",
	}
	opts := buildCycleEngineOptions(cwd, cfg)

	if !opts.AutoCleanStale {
		t.Error("expected AutoCleanStale=true")
	}
	if opts.WorkingDir != cwd {
		t.Errorf("WorkingDir = %q, want %q", opts.WorkingDir, cwd)
	}
	if opts.AutoCleanStaleAfter != 48*time.Hour {
		t.Errorf("AutoCleanStaleAfter = %v", opts.AutoCleanStaleAfter)
	}
	if opts.RuntimeMode != "stream" {
		t.Errorf("RuntimeMode = %q", opts.RuntimeMode)
	}
	if opts.RuntimeCommand != "claude" {
		t.Errorf("RuntimeCommand = %q", opts.RuntimeCommand)
	}
	if opts.AOCommand != "ao" {
		t.Errorf("AOCommand = %q", opts.AOCommand)
	}
	if opts.BDCommand != "bd" {
		t.Errorf("BDCommand = %q", opts.BDCommand)
	}
	if opts.TmuxCommand != "tmux" {
		t.Errorf("TmuxCommand = %q", opts.TmuxCommand)
	}
}

// --- supervisorLease methods ---

func TestSupervisorLease_Path(t *testing.T) {
	tmpDir := t.TempDir()
	leasePath := filepath.Join(tmpDir, "test.lock")

	lease, err := acquireSupervisorLease(tmpDir, leasePath, 2*time.Minute, "test-run")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer func() { _ = lease.Release() }()

	if lease.Path() != leasePath {
		t.Errorf("Path() = %q, want %q", lease.Path(), leasePath)
	}
}

func TestSupervisorLease_AcquireRelease(t *testing.T) {
	tmpDir := t.TempDir()
	leasePath := filepath.Join(tmpDir, "acquire-release.lock")

	lease, err := acquireSupervisorLease(tmpDir, leasePath, 2*time.Minute, "ar-run")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}

	if err := lease.Release(); err != nil {
		t.Fatalf("release: %v", err)
	}

	// Should be able to acquire again after release
	lease2, err := acquireSupervisorLease(tmpDir, leasePath, 2*time.Minute, "ar-run-2")
	if err != nil {
		t.Fatalf("re-acquire: %v", err)
	}
	defer func() { _ = lease2.Release() }()
}

func TestAcquireSupervisorLease_DefaultRunID(t *testing.T) {
	tmpDir := t.TempDir()
	leasePath := filepath.Join(tmpDir, "default-runid.lock")

	lease, err := acquireSupervisorLease(tmpDir, leasePath, 0, "")
	if err != nil {
		t.Fatalf("acquire with empty runID: %v", err)
	}
	defer func() { _ = lease.Release() }()

	if lease.meta.RunID == "" {
		t.Error("expected non-empty RunID when empty was provided (should be generated)")
	}
	if lease.ttl != 2*time.Minute {
		t.Errorf("expected default TTL 2m, got %v", lease.ttl)
	}
}

func TestAcquireSupervisorLease_RelativePath(t *testing.T) {
	tmpDir := t.TempDir()
	// Relative path should be resolved against cwd
	lease, err := acquireSupervisorLease(tmpDir, "test.lock", 2*time.Minute, "rel-run")
	if err != nil {
		t.Fatalf("acquire with relative path: %v", err)
	}
	defer func() { _ = lease.Release() }()

	if !filepath.IsAbs(lease.Path()) {
		t.Errorf("expected absolute path, got %q", lease.Path())
	}
}

// --- readLeaseHolderHint ---

func TestReadLeaseHolderHint_NoFile(t *testing.T) {
	got := readLeaseHolderHint("/nonexistent/file.lock")
	if !strings.Contains(got, "lock=") {
		t.Errorf("expected 'lock=' fallback, got %q", got)
	}
}

func TestReadLeaseHolderHint_InvalidJSON(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "bad.lock")
	if err := os.WriteFile(tmpFile, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}
	got := readLeaseHolderHint(tmpFile)
	if !strings.Contains(got, "lock=") {
		t.Errorf("expected 'lock=' fallback for invalid JSON, got %q", got)
	}
}

func TestReadLeaseHolderHint_ValidMetadata(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "good.lock")
	meta := supervisorLeaseMetadata{
		RunID:     "test-run",
		PID:       12345,
		Host:      "testhost",
		RenewedAt: "2026-02-15T10:00:00Z",
	}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		t.Fatal(err)
	}
	got := readLeaseHolderHint(tmpFile)
	if !strings.Contains(got, "run=test-run") {
		t.Errorf("expected 'run=test-run' in hint, got %q", got)
	}
	if !strings.Contains(got, "pid=12345") {
		t.Errorf("expected 'pid=12345' in hint, got %q", got)
	}
}

func TestReadLeaseHolderHint_EmptyRunID(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "empty-runid.lock")
	meta := supervisorLeaseMetadata{
		PID: 12345,
	}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		t.Fatal(err)
	}
	got := readLeaseHolderHint(tmpFile)
	if !strings.Contains(got, "lock=") {
		t.Errorf("expected 'lock=' fallback for empty runID, got %q", got)
	}
}

// --- validateLoopConfigValues ---

func TestValidateLoopConfigValues(t *testing.T) {
	cfg := rpiLoopSupervisorConfig{
		CycleRetries:   0,
		RetryBackoff:   0,
		CycleDelay:     0,
		CommandTimeout: 0,
	}
	err := validateLoopConfigValues(&cfg, nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.CommandTimeout != defaultLoopCommandTimeout {
		t.Errorf("CommandTimeout should default, got %v", cfg.CommandTimeout)
	}
}

// --- MaxCycleAttempts ---

func TestMaxCycleAttempts(t *testing.T) {
	tests := []struct {
		retries int
		want    int
	}{
		{0, 1},
		{1, 2},
		{5, 6},
	}
	for _, tt := range tests {
		cfg := rpiLoopSupervisorConfig{CycleRetries: tt.retries}
		if got := cfg.MaxCycleAttempts(); got != tt.want {
			t.Errorf("MaxCycleAttempts(retries=%d) = %d, want %d", tt.retries, got, tt.want)
		}
	}
}

// --- ShouldContinueAfterFailure ---

func TestShouldContinueAfterFailure(t *testing.T) {
	tests := []struct {
		policy string
		want   bool
	}{
		{loopFailurePolicyStop, false},
		{loopFailurePolicyContinue, true},
	}
	for _, tt := range tests {
		cfg := rpiLoopSupervisorConfig{FailurePolicy: tt.policy}
		if got := cfg.ShouldContinueAfterFailure(); got != tt.want {
			t.Errorf("ShouldContinueAfterFailure(policy=%q) = %v, want %v", tt.policy, got, tt.want)
		}
	}
}

// --- shouldRunBDSync ---

func TestShouldRunBDSync_AutoNoBeads(t *testing.T) {
	prevLookPath := loopLookPath
	defer func() { loopLookPath = prevLookPath }()
	loopLookPath = func(_ string) (string, error) { return "/usr/bin/bd", nil }

	tmpDir := t.TempDir()
	// No .beads directory
	run, err := shouldRunBDSync(tmpDir, loopBDSyncPolicyAuto, "bd")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if run {
		t.Error("expected false when .beads directory is missing")
	}
}

func TestShouldRunBDSync_AlwaysPresent(t *testing.T) {
	prevLookPath := loopLookPath
	defer func() { loopLookPath = prevLookPath }()
	loopLookPath = func(_ string) (string, error) { return "/usr/bin/bd", nil }

	run, err := shouldRunBDSync(t.TempDir(), loopBDSyncPolicyAlways, "bd")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !run {
		t.Error("expected true for always policy with bd available")
	}
}

// TestDeferSupervisorCleanup_NoError — canonical version in rpi_loop_supervisor_test.go

// --- openLeaseFile ---

func TestOpenLeaseFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nested", "dir", "lease.lock")

	file, err := openLeaseFile(path)
	if err != nil {
		t.Fatalf("openLeaseFile: %v", err)
	}
	defer func() { _ = file.Close() }()

	// Verify directory was created
	if _, statErr := os.Stat(filepath.Dir(path)); statErr != nil {
		t.Errorf("expected directory to be created: %v", statErr)
	}
}

// --- collectDirtyPaths error paths ---

func TestCollectDirtyPaths_DiffError(t *testing.T) {
	prevRunner := loopCommandOutputRunner
	defer func() { loopCommandOutputRunner = prevRunner }()

	loopCommandOutputRunner = func(_ string, _ time.Duration, name string, args ...string) (string, error) {
		if name == "git" && len(args) > 0 && args[0] == "diff" {
			return "", fmt.Errorf("git diff failed")
		}
		return "", nil
	}

	_, err := collectDirtyPaths(t.TempDir(), time.Minute)
	if err == nil {
		t.Fatal("expected error from git diff failure")
	}
}

func TestCollectDirtyPaths_LsFilesError(t *testing.T) {
	prevRunner := loopCommandOutputRunner
	defer func() { loopCommandOutputRunner = prevRunner }()

	loopCommandOutputRunner = func(_ string, _ time.Duration, name string, args ...string) (string, error) {
		if name == "git" && len(args) > 0 && args[0] == "diff" {
			return "file.go\n", nil
		}
		if name == "git" && len(args) > 0 && args[0] == "ls-files" {
			return "", fmt.Errorf("git ls-files failed")
		}
		return "", nil
	}

	_, err := collectDirtyPaths(t.TempDir(), time.Minute)
	if err == nil {
		t.Fatal("expected error from git ls-files failure")
	}
}
