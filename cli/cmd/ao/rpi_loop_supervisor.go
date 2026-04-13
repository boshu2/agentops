package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
)

const (
	loopFailurePolicyStop     = "stop"
	loopFailurePolicyContinue = "continue"

	loopSurfaceRPI    = "rpi"
	loopSurfaceEvolve = "evolve"

	loopGatePolicyOff        = "off"
	loopGatePolicyBestEffort = "best-effort"
	loopGatePolicyRequired   = "required"

	loopLandingPolicyOff      = "off"
	loopLandingPolicyCommit   = "commit"
	loopLandingPolicySyncPush = "sync-push"

	loopBDSyncPolicyAuto   = "auto"
	loopBDSyncPolicyAlways = "always"
	loopBDSyncPolicyNever  = "never"
)

const defaultLoopCommandTimeout = 20 * time.Minute

var (
	loopExecCommandContext  = exec.CommandContext
	loopLookPath            = exec.LookPath
	loopCommandRunner       = runLoopCommandWithTimeout
	loopCommandOutputRunner = runLoopCommandOutputWithTimeout
)

type rpiLoopSupervisorConfig struct {
	Surface               string
	RalphPreset           bool
	FailurePolicy         string
	CycleRetries          int
	RetryBackoff          time.Duration
	CycleDelay            time.Duration
	CompileEnabled        bool
	CompileInterval       time.Duration
	CompileSince          string
	CompileDefrag         bool
	LeaseEnabled          bool
	LeasePath             string
	LeaseTTL              time.Duration
	DetachedHeal          bool
	DetachedBranchPrefix  string
	AutoClean             bool
	AutoCleanStaleAfter   time.Duration
	EnsureCleanup         bool
	CleanupPruneWorktrees bool
	CleanupPruneBranches  bool
	GatePolicy            string
	ValidateFastScript    string
	SecurityGateScript    string
	LandingPolicy         string
	LandingBranch         string
	LandingCommitMessage  string
	LandingLockPath       string
	BDSyncPolicy          string
	CommandTimeout        time.Duration
	KillSwitchPath        string
	RuntimeMode           string
	RuntimeCommand        string
	AOCommand             string
	BDCommand             string
	TmuxCommand           string
}

func resolveLoopSupervisorConfig(cmd *cobra.Command, cwd string) (rpiLoopSupervisorConfig, error) {
	cfg := buildBaseLoopConfig()
	if cmd != nil && cmd.Name() == "evolve" {
		cfg.Surface = loopSurfaceEvolve
	}

	if rpiSupervisor || rpiRalph {
		applySupervisorDefaults(cmd, &cfg)
	}
	if rpiRalph {
		applyRalphDefaults(cmd, &cfg)
	}

	if err := validateLoopConfigValues(&cfg, cmd); err != nil {
		return cfg, err
	}
	if err := validateLoopConfigPolicies(cfg); err != nil {
		return cfg, err
	}
	resolveLoopConfigPaths(&cfg, cwd)

	toolchain, err := resolveRPIToolchainDefaults()
	if err != nil {
		return cfg, err
	}
	cfg.RuntimeMode = toolchain.RuntimeMode
	cfg.RuntimeCommand = toolchain.RuntimeCommand
	cfg.AOCommand = toolchain.AOCommand
	cfg.BDCommand = toolchain.BDCommand
	cfg.TmuxCommand = toolchain.TmuxCommand

	return cfg, nil
}

// buildBaseLoopConfig creates an rpiLoopSupervisorConfig from global flag values.
func buildBaseLoopConfig() rpiLoopSupervisorConfig {
	return rpiLoopSupervisorConfig{
		Surface:               loopSurfaceRPI,
		RalphPreset:           rpiRalph,
		FailurePolicy:         strings.ToLower(strings.TrimSpace(rpiFailurePolicy)),
		CycleRetries:          rpiCycleRetries,
		RetryBackoff:          rpiRetryBackoff,
		CycleDelay:            rpiCycleDelay,
		CompileEnabled:        rpiCompile,
		CompileInterval:       rpiCompileInterval,
		CompileSince:          strings.TrimSpace(rpiCompileSince),
		CompileDefrag:         rpiCompileDefrag,
		LeaseEnabled:          rpiLease,
		LeasePath:             rpiLeasePath,
		LeaseTTL:              rpiLeaseTTL,
		DetachedHeal:          rpiDetachedHeal,
		DetachedBranchPrefix:  rpiDetachedBranchPrefix,
		AutoClean:             rpiAutoClean,
		AutoCleanStaleAfter:   rpiAutoCleanStaleAfter,
		EnsureCleanup:         rpiEnsureCleanup,
		CleanupPruneWorktrees: rpiCleanupPruneWorktrees,
		CleanupPruneBranches:  rpiCleanupPruneBranches,
		GatePolicy:            strings.ToLower(strings.TrimSpace(rpiGatePolicy)),
		ValidateFastScript:    rpiValidateFastScript,
		SecurityGateScript:    rpiSecurityGateScript,
		LandingPolicy:         strings.ToLower(strings.TrimSpace(rpiLandingPolicy)),
		LandingBranch:         strings.TrimSpace(rpiLandingBranch),
		LandingCommitMessage:  rpiLandingCommitMessage,
		LandingLockPath:       rpiLandingLockPath,
		BDSyncPolicy:          strings.ToLower(strings.TrimSpace(rpiBDSyncPolicy)),
		CommandTimeout:        rpiCommandTimeout,
		KillSwitchPath:        strings.TrimSpace(rpiKillSwitchPath),
	}
}

// applyRalphDefaults overlays operator-safe nonstop defaults for external loop
// supervision while preserving explicit CLI flag precedence.
func applyRalphDefaults(cmd *cobra.Command, cfg *rpiLoopSupervisorConfig) {
	if !loopFlagChanged(cmd, "lease") {
		cfg.LeaseEnabled = true
	}
	if !loopFlagChanged(cmd, "detached-heal") {
		cfg.DetachedHeal = true
	}
	if !loopFlagChanged(cmd, "auto-clean") {
		cfg.AutoClean = true
	}
	if !loopFlagChanged(cmd, "ensure-cleanup") {
		cfg.EnsureCleanup = true
	}
	if !loopFlagChanged(cmd, "cleanup-prune-branches") {
		cfg.CleanupPruneBranches = true
	}
	if !loopFlagChanged(cmd, "failure-policy") {
		cfg.FailurePolicy = loopFailurePolicyContinue
	}
	if !loopFlagChanged(cmd, "cycle-retries") {
		cfg.CycleRetries = 1
	}
	if !loopFlagChanged(cmd, "cycle-delay") {
		cfg.CycleDelay = 2 * time.Minute
	}
	if !loopFlagChanged(cmd, "gate-policy") {
		cfg.GatePolicy = loopGatePolicyRequired
	}
}

// applySupervisorDefaults overrides config values with supervisor-mode defaults
// for any flags the user has not explicitly set.
func applySupervisorDefaults(cmd *cobra.Command, cfg *rpiLoopSupervisorConfig) {
	applySupervisorBoolDefaults(cmd, cfg)
	applySupervisorPolicyDefaults(cmd, cfg)
}

func loopFlagChanged(cmd *cobra.Command, name string) bool {
	return cmd != nil && cmd.Flags().Changed(name)
}

func applySupervisorBoolDefaults(cmd *cobra.Command, cfg *rpiLoopSupervisorConfig) {
	if !loopFlagChanged(cmd, "compile") {
		cfg.CompileEnabled = true
	}
	if !loopFlagChanged(cmd, "compile-defrag") {
		cfg.CompileDefrag = true
	}
	if !loopFlagChanged(cmd, "lease") {
		cfg.LeaseEnabled = true
	}
	if !loopFlagChanged(cmd, "detached-heal") {
		cfg.DetachedHeal = false
	}
	if !loopFlagChanged(cmd, "auto-clean") {
		cfg.AutoClean = true
	}
	if !loopFlagChanged(cmd, "ensure-cleanup") {
		cfg.EnsureCleanup = true
	}
	if !loopFlagChanged(cmd, "cleanup-prune-branches") {
		cfg.CleanupPruneBranches = true
	}
}

func applySupervisorPolicyDefaults(cmd *cobra.Command, cfg *rpiLoopSupervisorConfig) {
	if !loopFlagChanged(cmd, "compile-interval") {
		cfg.CompileInterval = 30 * time.Minute
	}
	if !loopFlagChanged(cmd, "compile-since") {
		cfg.CompileSince = "26h"
	}
	if !loopFlagChanged(cmd, "failure-policy") {
		cfg.FailurePolicy = loopFailurePolicyContinue
	}
	if !loopFlagChanged(cmd, "cycle-retries") {
		cfg.CycleRetries = 1
	}
	if !loopFlagChanged(cmd, "cycle-delay") {
		cfg.CycleDelay = 5 * time.Minute
	}
	if !loopFlagChanged(cmd, "gate-policy") {
		cfg.GatePolicy = loopGatePolicyRequired
	}
}

// validateLoopConfigValues checks numeric constraints and applies defaults for
// zero-valued fields. cmd may be nil when rpiSupervisor is false.
func validateLoopConfigValues(cfg *rpiLoopSupervisorConfig, cmd *cobra.Command) error {
	if err := validateLoopNumericConstraints(cfg); err != nil {
		return err
	}
	applyLoopTimingDefaults(cfg, cmd)
	applyLoopPathDefaults(cfg)
	return nil
}

func validateLoopNumericConstraints(cfg *rpiLoopSupervisorConfig) error {
	if cfg.CycleRetries < 0 {
		return fmt.Errorf("cycle-retries must be >= 0")
	}
	if cfg.RetryBackoff < 0 {
		return fmt.Errorf("retry-backoff must be >= 0")
	}
	if cfg.CycleDelay < 0 {
		return fmt.Errorf("cycle-delay must be >= 0")
	}
	if cfg.CompileInterval < 0 {
		return fmt.Errorf("compile-interval must be >= 0")
	}
	if cfg.CommandTimeout < 0 {
		return fmt.Errorf("command-timeout must be >= 0")
	}
	return nil
}

func applyLoopTimingDefaults(cfg *rpiLoopSupervisorConfig, cmd *cobra.Command) {
	if cfg.LeaseTTL <= 0 {
		cfg.LeaseTTL = 2 * time.Minute
	}
	if cfg.CommandTimeout == 0 {
		cfg.CommandTimeout = defaultLoopCommandTimeout
	}
	if cfg.AutoCleanStaleAfter <= 0 {
		cfg.AutoCleanStaleAfter = 24 * time.Hour
	}
	if rpiSupervisor && !loopFlagChanged(cmd, "auto-clean-stale-after") {
		cfg.AutoCleanStaleAfter = 0
	}
}

func applyLoopPathDefaults(cfg *rpiLoopSupervisorConfig) {
	if cfg.LeasePath == "" {
		cfg.LeasePath = filepath.Join(".agents", "rpi", "supervisor.lock")
	}
	if cfg.LandingLockPath == "" {
		cfg.LandingLockPath = filepath.Join(".agents", "rpi", "landing.lock")
	}
	if cfg.KillSwitchPath == "" {
		cfg.KillSwitchPath = filepath.Join(".agents", "rpi", "KILL")
	}
}

// validateLoopConfigPolicies validates enum-style policy fields.
func validateLoopConfigPolicies(cfg rpiLoopSupervisorConfig) error {
	if cfg.FailurePolicy != loopFailurePolicyStop && cfg.FailurePolicy != loopFailurePolicyContinue {
		return fmt.Errorf("invalid failure-policy %q (valid: stop|continue)", cfg.FailurePolicy)
	}
	switch cfg.GatePolicy {
	case loopGatePolicyOff, loopGatePolicyBestEffort, loopGatePolicyRequired:
	default:
		return fmt.Errorf("invalid gate-policy %q (valid: off|best-effort|required)", cfg.GatePolicy)
	}
	switch cfg.LandingPolicy {
	case loopLandingPolicyOff, loopLandingPolicyCommit, loopLandingPolicySyncPush:
	default:
		return fmt.Errorf("invalid landing-policy %q (valid: off|commit|sync-push)", cfg.LandingPolicy)
	}
	switch cfg.BDSyncPolicy {
	case loopBDSyncPolicyAuto, loopBDSyncPolicyAlways, loopBDSyncPolicyNever:
	default:
		return fmt.Errorf("invalid bd-sync-policy %q (valid: auto|always|never)", cfg.BDSyncPolicy)
	}
	return nil
}

// resolveLoopConfigPaths converts relative paths to absolute paths.
func resolveLoopConfigPaths(cfg *rpiLoopSupervisorConfig, cwd string) {
	if !filepath.IsAbs(cfg.LeasePath) {
		cfg.LeasePath = filepath.Join(cwd, cfg.LeasePath)
	}
	if !filepath.IsAbs(cfg.LandingLockPath) {
		cfg.LandingLockPath = filepath.Join(cwd, cfg.LandingLockPath)
	}
	if !filepath.IsAbs(cfg.KillSwitchPath) {
		cfg.KillSwitchPath = filepath.Join(cwd, cfg.KillSwitchPath)
	}
}

func (c rpiLoopSupervisorConfig) MaxCycleAttempts() int {
	return c.CycleRetries + 1
}

func (c rpiLoopSupervisorConfig) ShouldContinueAfterFailure() bool {
	return c.FailurePolicy == loopFailurePolicyContinue
}

type cycleFailureKind string

const (
	cycleFailureTask           cycleFailureKind = "task"
	cycleFailureInfrastructure cycleFailureKind = "infrastructure"
)

type cycleFailureError struct {
	kind cycleFailureKind
	err  error
}

func (e *cycleFailureError) Error() string {
	return e.err.Error()
}

func (e *cycleFailureError) Unwrap() error {
	return e.err
}

func wrapCycleFailure(kind cycleFailureKind, stage string, err error) error {
	if err == nil {
		return nil
	}
	var existing *cycleFailureError
	if errors.As(err, &existing) {
		return err
	}
	if stage != "" {
		err = fmt.Errorf("%s: %w", stage, err)
	}
	return &cycleFailureError{kind: kind, err: err}
}

func shouldMarkQueueEntryFailed(err error) bool {
	return !isInfrastructureCycleFailure(err)
}

func isInfrastructureCycleFailure(err error) bool {
	var failure *cycleFailureError
	if !errors.As(err, &failure) {
		return false
	}
	return failure.kind == cycleFailureInfrastructure
}

type landingScope struct {
	baselineDirtyPaths map[string]struct{}
}

func runRPISupervisedCycle(ctx context.Context, cwd, goal string, cycle, attempt int, cfg rpiLoopSupervisorConfig) (retErr error) {
	if err := healDetachedHeadIfNeeded(cwd, cfg); err != nil {
		return err
	}
	var scope *landingScope
	if cfg.LandingPolicy != loopLandingPolicyOff {
		scope, retErr = captureLandingScope(cwd, cfg.CommandTimeout)
		if retErr != nil {
			return wrapCycleFailure(cycleFailureInfrastructure, "capture landing scope", retErr)
		}
	}

	if cfg.EnsureCleanup {
		defer func() {
			retErr = deferSupervisorCleanup(cwd, cfg, retErr)
		}()
	}

	opts := buildCycleEngineOptions(cwd, cfg)
	if err := runPhasedEngine(ctx, cwd, goal, opts); err != nil {
		return wrapCycleFailure(cycleFailureTask, "phased engine", err)
	}
	if err := runSupervisorGates(cwd, cfg); err != nil {
		return wrapCycleFailure(cycleFailureTask, "quality gates", err)
	}
	if err := runSupervisorLanding(cwd, cfg, cycle, attempt, goal, scope); err != nil {
		return wrapCycleFailure(cycleFailureInfrastructure, "landing", err)
	}
	return nil
}

func healDetachedHeadIfNeeded(cwd string, cfg rpiLoopSupervisorConfig) error {
	if !cfg.DetachedHeal {
		return nil
	}
	branch, healed, err := ensureLoopAttachedBranch(cwd, cfg.DetachedBranchPrefix)
	if err != nil {
		return wrapCycleFailure(cycleFailureInfrastructure, "detached-head self-heal", err)
	}
	if healed {
		fmt.Printf("Detached HEAD detected. Switched to branch: %s\n", branch)
	}
	return nil
}

func deferSupervisorCleanup(cwd string, cfg rpiLoopSupervisorConfig, retErr error) error {
	cleanupErr := runSupervisorCleanupWithBranchPrune(
		cwd,
		cfg.AutoCleanStaleAfter,
		cfg.CleanupPruneWorktrees,
		cfg.CleanupPruneBranches,
	)
	if cleanupErr == nil {
		return retErr
	}
	if retErr == nil {
		return wrapCycleFailure(cycleFailureInfrastructure, "supervisor cleanup", cleanupErr)
	}
	VerbosePrintf("Warning: supervisor cleanup after failure: %v\n", cleanupErr)
	return retErr
}

func buildCycleEngineOptions(cwd string, cfg rpiLoopSupervisorConfig) phasedEngineOptions {
	opts := defaultPhasedEngineOptions()
	opts.WorkingDir = cwd
	opts.AutoCleanStale = cfg.AutoClean
	opts.AutoCleanStaleAfter = cfg.AutoCleanStaleAfter
	opts.RuntimeMode = cfg.RuntimeMode
	opts.RuntimeCommand = cfg.RuntimeCommand
	opts.AOCommand = cfg.AOCommand
	opts.BDCommand = cfg.BDCommand
	opts.TmuxCommand = cfg.TmuxCommand
	opts.NoDashboard = true
	return opts
}

func ensureLoopAttachedBranch(cwd, branchPrefix string) (string, bool, error) {
	repoRoot, err := cliRPI.GetRepoRoot(cwd, worktreeTimeout)
	if err != nil {
		return "", false, err
	}
	return cliRPI.EnsureAttachedBranch(repoRoot, worktreeTimeout, branchPrefix)
}

func runSupervisorCleanupWithBranchPrune(cwd string, staleAfter time.Duration, pruneWorktrees, pruneBranches bool) error {
	return executeRPICleanup(cwd, "", true, pruneWorktrees, pruneBranches, GetDryRun(), staleAfter)
}

func isLoopKillSwitchSet(cfg rpiLoopSupervisorConfig) (bool, error) {
	if strings.TrimSpace(cfg.KillSwitchPath) == "" {
		return false, nil
	}
	info, err := os.Stat(cfg.KillSwitchPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("check kill switch %s: %w", cfg.KillSwitchPath, err)
	}
	if info.IsDir() {
		return false, fmt.Errorf("kill switch path is a directory: %s", cfg.KillSwitchPath)
	}
	return true, nil
}

func runSupervisorGates(cwd string, cfg rpiLoopSupervisorConfig) error {
	if cfg.GatePolicy == loopGatePolicyOff {
		return nil
	}

	runRequired := cfg.GatePolicy == loopGatePolicyRequired
	var failures []string
	for _, gatePath := range []string{cfg.ValidateFastScript, cfg.SecurityGateScript} {
		if strings.TrimSpace(gatePath) == "" {
			continue
		}
		if err := runGateScript(cwd, gatePath, runRequired, cfg.CommandTimeout); err != nil {
			if cfg.GatePolicy == loopGatePolicyBestEffort {
				VerbosePrintf("Warning: gate %s failed: %v\n", gatePath, err)
				continue
			}
			failures = append(failures, err.Error())
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("gate failures: %s", strings.Join(failures, "; "))
	}
	return nil
}

func runGateScript(cwd, scriptPath string, required bool, timeout time.Duration) error {
	path := scriptPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}

	info, err := os.Stat(path)
	if err != nil {
		if required {
			return fmt.Errorf("required gate script missing: %s", path)
		}
		fmt.Printf("Skipping optional gate (missing): %s\n", path)
		return nil
	}
	if info.IsDir() {
		if required {
			return fmt.Errorf("required gate path is a directory: %s", path)
		}
		fmt.Printf("Skipping optional gate (path is directory): %s\n", path)
		return nil
	}

	fmt.Printf("Running gate: %s\n", path)
	if err := loopCommandRunner(cwd, timeout, "bash", path); err != nil {
		return fmt.Errorf("gate script %s failed: %w", path, err)
	}
	return nil
}

func runSupervisorLanding(cwd string, cfg rpiLoopSupervisorConfig, cycle, attempt int, goal string, scope *landingScope) error {
	switch cfg.LandingPolicy {
	case loopLandingPolicyOff:
		return nil
	case loopLandingPolicyCommit:
		return runCommitLanding(cwd, cfg, cycle, attempt, goal, scope)
	case loopLandingPolicySyncPush:
		return runSyncPushLanding(cwd, cfg, cycle, attempt, goal, scope)
	default:
		return fmt.Errorf("unsupported landing policy: %s", cfg.LandingPolicy)
	}
}

func runCommitLanding(cwd string, cfg rpiLoopSupervisorConfig, cycle, attempt int, goal string, scope *landingScope) error {
	landingLock, err := acquireLandingLock(cwd, cfg)
	if err != nil {
		return fmt.Errorf("landing lock acquisition failed: %w", err)
	}
	if landingLock != nil {
		defer func() {
			if releaseErr := landingLock.Release(); releaseErr != nil {
				VerbosePrintf("Warning: could not release landing lock: %v\n", releaseErr)
			}
		}()
	}
	_, err = commitIfDirty(cwd, renderLandingCommitMessage(cfg.LandingCommitMessage, cycle, attempt, goal), cfg.CommandTimeout, scope)
	return err
}

func runSyncPushLanding(cwd string, cfg rpiLoopSupervisorConfig, cycle, attempt int, goal string, scope *landingScope) error {
	landingLock, err := acquireLandingLock(cwd, cfg)
	if err != nil {
		return fmt.Errorf("landing lock acquisition failed: %w", err)
	}
	if landingLock != nil {
		defer func() {
			if releaseErr := landingLock.Release(); releaseErr != nil {
				VerbosePrintf("Warning: could not release landing lock: %v\n", releaseErr)
			}
		}()
	}

	committed, err := commitIfDirty(cwd, renderLandingCommitMessage(cfg.LandingCommitMessage, cycle, attempt, goal), cfg.CommandTimeout, scope)
	if err != nil {
		return err
	}
	if !committed {
		fmt.Println("Landing: no commit performed.")
		return nil
	}
	return syncRebaseAndPush(cwd, cfg)
}

func syncRebaseAndPush(cwd string, cfg rpiLoopSupervisorConfig) error {
	targetBranch, err := resolveLandingBranch(cwd, cfg.LandingBranch, cfg.CommandTimeout)
	if err != nil {
		return err
	}
	if err := fetchAndRebase(cwd, cfg.CommandTimeout, targetBranch); err != nil {
		return err
	}
	if err := runBDSyncIfNeeded(cwd, cfg); err != nil {
		return err
	}
	if err := loopCommandRunner(cwd, cfg.CommandTimeout, "git", "push", "origin", "HEAD:"+targetBranch); err != nil {
		return fmt.Errorf("landing push failed: %w", err)
	}
	_ = loopCommandRunner(cwd, cfg.CommandTimeout, "git", "status", "-sb")
	return nil
}

func fetchAndRebase(cwd string, timeout time.Duration, targetBranch string) error {
	if err := loopCommandRunner(cwd, timeout, "git", "fetch", "origin", targetBranch); err != nil {
		return wrapSyncPushLandingFailure(cwd, timeout, "fetch", err)
	}
	if err := loopCommandRunner(cwd, timeout, "git", "rebase", "origin/"+targetBranch); err != nil {
		return wrapSyncPushLandingFailure(cwd, timeout, "rebase", err)
	}
	return nil
}

func runBDSyncIfNeeded(cwd string, cfg rpiLoopSupervisorConfig) error {
	runSync, err := shouldRunBDSync(cwd, cfg.BDSyncPolicy, cfg.BDCommand)
	if err != nil {
		return err
	}
	if runSync {
		if err := loopCommandRunner(cwd, cfg.CommandTimeout, cfg.BDCommand, "export", "-o", os.DevNull); err != nil {
			return fmt.Errorf("bd export checkpoint failed: %w", err)
		}
	}
	return nil
}

func acquireLandingLock(cwd string, cfg rpiLoopSupervisorConfig) (*supervisorLease, error) {
	if cfg.LandingPolicy == loopLandingPolicyOff {
		return nil, nil
	}
	if strings.TrimSpace(cfg.LandingLockPath) == "" {
		return nil, nil
	}

	runID := cfg.LandingPolicy + "-run-" + cliRPI.GenerateRunID()
	return acquireSupervisorLease(cwd, cfg.LandingLockPath, cfg.LeaseTTL, runID)
}

func wrapSyncPushLandingFailure(cwd string, timeout time.Duration, stage string, err error) error {
	if err == nil {
		return nil
	}
	if recoveryErr := recoverSyncPushLandingState(cwd, timeout); recoveryErr != nil {
		return fmt.Errorf("landing %s failed: %w (state recovery failed: %v)", stage, err, recoveryErr)
	}
	return fmt.Errorf("landing %s failed: %w (state recovered)", stage, err)
}

func recoverSyncPushLandingState(cwd string, timeout time.Duration) error {
	abortOut, abortErr := loopCommandOutputRunner(cwd, timeout, "git", "rebase", "--abort")
	if abortErr != nil && !isNoRebaseInProgressMessage(abortOut) {
		return fmt.Errorf("git rebase --abort failed: %w", abortErr)
	}
	if err := loopCommandRunner(cwd, timeout, "git", "status", "-sb"); err != nil {
		return fmt.Errorf("git status during state recovery failed: %w", err)
	}
	return nil
}

func isNoRebaseInProgressMessage(out string) bool {
	msg := strings.ToLower(strings.TrimSpace(out))
	if msg == "" {
		return false
	}
	return strings.Contains(msg, "no rebase in progress") || strings.Contains(msg, "no rebase to abort")
}

func renderLandingCommitMessage(template string, cycle, attempt int, goal string) string {
	msg := cmp.Or(strings.TrimSpace(template), "chore(rpi): autonomous cycle {{cycle}}")
	msg = strings.ReplaceAll(msg, "{{cycle}}", strconv.Itoa(cycle))
	msg = strings.ReplaceAll(msg, "{{attempt}}", strconv.Itoa(attempt))
	msg = strings.ReplaceAll(msg, "{{goal}}", goal)
	return msg
}

func commitIfDirty(cwd, message string, timeout time.Duration, scope *landingScope) (bool, error) {
	dirty, err := repoHasChanges(cwd, timeout)
	if err != nil {
		return false, err
	}
	if !dirty {
		fmt.Println("Landing: no changes to commit.")
		return false, nil
	}
	if scope == nil {
		return false, fmt.Errorf("landing scope missing for autonomous commit")
	}
	ownedPaths, err := computeOwnedDirtyPaths(cwd, timeout, scope)
	if err != nil {
		return false, err
	}
	if len(ownedPaths) == 0 {
		fmt.Println("Landing: only pre-existing dirty paths detected; skipping autonomous commit.")
		return false, nil
	}
	addArgs := append([]string{"add", "--"}, ownedPaths...)
	if err := loopCommandRunner(cwd, timeout, "git", addArgs...); err != nil {
		return false, fmt.Errorf("git add owned paths failed: %w", err)
	}
	if err := loopCommandRunner(cwd, timeout, "git", "commit", "-m", message); err != nil {
		return false, fmt.Errorf("git commit failed: %w", err)
	}
	return true, nil
}

func repoHasChanges(cwd string, timeout time.Duration) (bool, error) {
	out, err := loopCommandOutputRunner(cwd, timeout, "git", "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("git status failed: %w", err)
	}
	return strings.TrimSpace(out) != "", nil
}

func resolveLandingBranch(cwd, explicit string, timeout time.Duration) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit), nil
	}
	out, err := loopCommandOutputRunner(cwd, timeout, "git", "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	if err == nil {
		branch := strings.TrimSpace(strings.TrimPrefix(out, "origin/"))
		if branch != "" {
			return branch, nil
		}
	}
	out, err = loopCommandOutputRunner(cwd, timeout, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err == nil {
		branch := strings.TrimSpace(out)
		if branch != "" && branch != "HEAD" {
			return branch, nil
		}
	}
	return "main", nil
}

func shouldRunBDSync(cwd, policy, bdCommand string) (bool, error) {
	command := cmp.Or(strings.TrimSpace(bdCommand), "bd")
	switch policy {
	case loopBDSyncPolicyNever:
		return false, nil
	case loopBDSyncPolicyAlways:
		if _, err := loopLookPath(command); err != nil {
			return false, fmt.Errorf("bd-sync-policy=always but %s CLI not found on PATH", command)
		}
		return true, nil
	case loopBDSyncPolicyAuto:
		if _, err := loopLookPath(command); err != nil {
			return false, nil
		}
		if _, err := os.Stat(filepath.Join(cwd, ".beads")); err != nil {
			return false, nil
		}
		return true, nil
	default:
		return false, fmt.Errorf("unsupported bd-sync-policy: %s", policy)
	}
}

func runLoopCommandWithTimeout(cwd string, timeout time.Duration, name string, args ...string) error {
	timeout = normalizeLoopCommandTimeout(timeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := loopExecCommandContext(ctx, name, args...)
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("%s %s timed out after %s", name, strings.Join(args, " "), timeout)
	}
	return err
}

func runLoopCommandOutputWithTimeout(cwd string, timeout time.Duration, name string, args ...string) (string, error) {
	timeout = normalizeLoopCommandTimeout(timeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := loopExecCommandContext(ctx, name, args...)
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return strings.TrimSpace(string(out)), fmt.Errorf("%s %s timed out after %s", name, strings.Join(args, " "), timeout)
	}
	return strings.TrimSpace(string(out)), err
}

func normalizeLoopCommandTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return defaultLoopCommandTimeout
	}
	return timeout
}

func captureLandingScope(cwd string, timeout time.Duration) (*landingScope, error) {
	paths, err := collectDirtyPaths(cwd, timeout)
	if err != nil {
		return nil, err
	}
	return &landingScope{baselineDirtyPaths: paths}, nil
}

func computeOwnedDirtyPaths(cwd string, timeout time.Duration, scope *landingScope) ([]string, error) {
	current, err := collectDirtyPaths(cwd, timeout)
	if err != nil {
		return nil, err
	}
	var owned []string
	for path := range current {
		if _, existed := scope.baselineDirtyPaths[path]; existed {
			continue
		}
		owned = append(owned, path)
	}
	slices.Sort(owned)
	return owned, nil
}

func collectDirtyPaths(cwd string, timeout time.Duration) (map[string]struct{}, error) {
	paths := make(map[string]struct{})

	trackedOut, err := loopCommandOutputRunner(cwd, timeout, "git", "diff", "--name-only", "HEAD", "--")
	if err != nil {
		return nil, fmt.Errorf("git diff failed while collecting landing scope: %w", err)
	}
	appendDirtyPaths(paths, trackedOut)

	untrackedOut, err := loopCommandOutputRunner(cwd, timeout, "git", "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, fmt.Errorf("git ls-files failed while collecting landing scope: %w", err)
	}
	appendDirtyPaths(paths, untrackedOut)

	return paths, nil
}

func appendDirtyPaths(dest map[string]struct{}, output string) {
	for _, line := range strings.Split(output, "\n") {
		path := strings.TrimSpace(line)
		if path == "" {
			continue
		}
		dest[path] = struct{}{}
	}
}

type supervisorLease struct {
	path   string
	file   *os.File
	ttl    time.Duration
	stopCh chan struct{}
	doneCh chan struct{}
	meta   supervisorLeaseMetadata
}

// supervisorLeaseMetadata is a thin alias for rpi.SupervisorLeaseMetadata.
type supervisorLeaseMetadata = cliRPI.SupervisorLeaseMetadata

func acquireSupervisorLease(cwd, leasePath string, ttl time.Duration, runID string) (*supervisorLease, error) {
	if runID == "" {
		runID = generateRunID()
	}
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}

	path := leasePath
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}
	file, err := openLeaseFile(path)
	if err != nil {
		return nil, err
	}

	if err := flockLeaseFile(file, path); err != nil {
		_ = file.Close()
		return nil, err
	}

	return buildAndStartLease(file, path, cwd, ttl, runID)
}

func openLeaseFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return nil, fmt.Errorf("create lease directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open lease file: %w", err)
	}
	return file, nil
}

func flockLeaseFile(file *os.File, path string) error {
	if err := flockLockNB(file); err != nil {
		if errors.Is(err, errLockWouldBlock) {
			return fmt.Errorf("single-flight lease already held: %s", readLeaseHolderHint(path))
		}
		return fmt.Errorf("acquire lease lock: %w", err)
	}
	return nil
}

func buildAndStartLease(file *os.File, path, cwd string, ttl time.Duration, runID string) (*supervisorLease, error) {
	host, _ := os.Hostname()
	now := time.Now().UTC()
	meta := supervisorLeaseMetadata{
		RunID:      runID,
		PID:        os.Getpid(),
		Host:       host,
		CWD:        cwd,
		AcquiredAt: now.Format(time.RFC3339),
		RenewedAt:  now.Format(time.RFC3339),
		ExpiresAt:  now.Add(ttl).Format(time.RFC3339),
	}
	lease := &supervisorLease{
		path:   path,
		file:   file,
		ttl:    ttl,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
		meta:   meta,
	}
	if err := lease.writeMetadata(now); err != nil {
		_ = flockUnlock(file)
		_ = file.Close()
		return nil, err
	}
	lease.startHeartbeat()
	return lease, nil
}

func (l *supervisorLease) Path() string {
	return l.path
}

func (l *supervisorLease) Release() error {
	close(l.stopCh)
	<-l.doneCh

	unlockErr := flockUnlock(l.file)
	closeErr := l.file.Close()

	if unlockErr != nil {
		return fmt.Errorf("unlock lease: %w", unlockErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close lease file: %w", closeErr)
	}
	return nil
}

func (l *supervisorLease) startHeartbeat() {
	interval := l.ttl / 2
	if interval < 15*time.Second {
		interval = 15 * time.Second
	}
	ticker := time.NewTicker(interval)
	go func() {
		defer close(l.doneCh)
		defer ticker.Stop()
		for {
			select {
			case <-l.stopCh:
				return
			case now := <-ticker.C:
				if err := l.writeMetadata(now.UTC()); err != nil {
					VerbosePrintf("Warning: lease heartbeat update failed: %v\n", err)
				}
			}
		}
	}()
}

func (l *supervisorLease) writeMetadata(now time.Time) error {
	l.meta.RenewedAt = now.Format(time.RFC3339)
	l.meta.ExpiresAt = now.Add(l.ttl).Format(time.RFC3339)

	data, err := json.MarshalIndent(l.meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal lease metadata: %w", err)
	}
	data = append(data, '\n')

	if err := l.file.Truncate(0); err != nil {
		return fmt.Errorf("truncate lease file: %w", err)
	}
	if _, err := l.file.Seek(0, 0); err != nil {
		return fmt.Errorf("seek lease file: %w", err)
	}
	if _, err := l.file.Write(data); err != nil {
		return fmt.Errorf("write lease metadata: %w", err)
	}
	if err := l.file.Sync(); err != nil {
		return fmt.Errorf("sync lease metadata: %w", err)
	}
	return nil
}

func readLeaseHolderHint(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("lock=%s", path)
	}
	var meta supervisorLeaseMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return fmt.Sprintf("lock=%s", path)
	}
	if meta.RunID == "" {
		return fmt.Sprintf("lock=%s", path)
	}
	return fmt.Sprintf("run=%s pid=%d host=%s renewed_at=%s", meta.RunID, meta.PID, meta.Host, meta.RenewedAt)
}

// cycleTelemetryEntry is the JSON structure written to telemetry.jsonl.
type cycleTelemetryEntry struct {
	Timestamp    string `json:"timestamp"`
	Cycle        int    `json:"cycle"`
	Goal         string `json:"goal"`
	DurationMS   int64  `json:"duration_ms"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message"`
}

// emitCycleTelemetry appends a JSON line to .agents/rpi/telemetry.jsonl.
// Write errors are silently ignored so telemetry never crashes the loop.
func emitCycleTelemetry(cwd string, cycle int, goal string, duration time.Duration, err error) {
	dir := filepath.Join(cwd, ".agents", "rpi")
	_ = os.MkdirAll(dir, 0o750)

	entry := cycleTelemetryEntry{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Cycle:      cycle,
		Goal:       goal,
		DurationMS: duration.Milliseconds(),
		Status:     "success",
	}
	if err != nil {
		entry.Status = "failure"
		entry.ErrorMessage = err.Error()
	}

	data, marshalErr := json.Marshal(entry)
	if marshalErr != nil {
		return
	}
	data = append(data, '\n')

	f, openErr := os.OpenFile(filepath.Join(dir, "telemetry.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if openErr != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = f.Write(data)
}

// leaseHeartbeatEntry is the JSON structure written by the lease heartbeat.
type leaseHeartbeatEntry struct {
	Holder    string `json:"holder"`
	Heartbeat string `json:"heartbeat"`
	TTLMS     int64  `json:"ttl_ms"`
}

// startLeaseHeartbeat starts a goroutine that updates the lease file every
// ttl/2. Returns a stop function that signals the goroutine to exit.
func startLeaseHeartbeat(ctx context.Context, leasePath string, ttl time.Duration, runID string) func() {
	interval := ttl / 2
	if interval <= 0 {
		interval = time.Second
	}
	stopCh := make(chan struct{})
	doneCh := make(chan struct{})

	go func() {
		defer close(doneCh)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stopCh:
				return
			case <-ticker.C:
				writeHeartbeat(leasePath, runID, ttl)
			}
		}
	}()

	return func() {
		close(stopCh)
		<-doneCh
	}
}

// writeHeartbeat writes the heartbeat JSON to the lease path.
func writeHeartbeat(leasePath, runID string, ttl time.Duration) {
	entry := leaseHeartbeatEntry{
		Holder:    runID,
		Heartbeat: time.Now().UTC().Format(time.RFC3339),
		TTLMS:     ttl.Milliseconds(),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	// Atomic-ish write: write to temp file then rename.
	dir := filepath.Dir(leasePath)
	_ = os.MkdirAll(dir, 0o750)
	tmp := leasePath + ".heartbeat.tmp"
	if writeErr := os.WriteFile(tmp, data, 0o644); writeErr != nil {
		return
	}
	_ = os.Rename(tmp, leasePath)
}
