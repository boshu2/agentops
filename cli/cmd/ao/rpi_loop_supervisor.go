package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
)

const (
	loopFailurePolicyStop     = "stop"
	loopFailurePolicyContinue = "continue"

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
	FailurePolicy         string
	CycleRetries          int
	RetryBackoff          time.Duration
	CycleDelay            time.Duration
	LeaseEnabled          bool
	LeasePath             string
	LeaseTTL              time.Duration
	DetachedHeal          bool
	DetachedBranchPrefix  string
	AutoClean             bool
	AutoCleanStaleAfter   time.Duration
	EnsureCleanup         bool
	CleanupPruneWorktrees bool
	GatePolicy            string
	ValidateFastScript    string
	SecurityGateScript    string
	LandingPolicy         string
	LandingBranch         string
	LandingCommitMessage  string
	LandingLockPath       string
	BDSyncPolicy          string
	CommandTimeout        time.Duration
	RuntimeMode           string
	RuntimeCommand        string
	AOCommand             string
	BDCommand             string
	TmuxCommand           string
}

func resolveLoopSupervisorConfig(cmd *cobra.Command, cwd string) (rpiLoopSupervisorConfig, error) {
	cfg := rpiLoopSupervisorConfig{
		FailurePolicy:         strings.ToLower(strings.TrimSpace(rpiFailurePolicy)),
		CycleRetries:          rpiCycleRetries,
		RetryBackoff:          rpiRetryBackoff,
		CycleDelay:            rpiCycleDelay,
		LeaseEnabled:          rpiLease,
		LeasePath:             rpiLeasePath,
		LeaseTTL:              rpiLeaseTTL,
		DetachedHeal:          rpiDetachedHeal,
		DetachedBranchPrefix:  rpiDetachedBranchPrefix,
		AutoClean:             rpiAutoClean,
		AutoCleanStaleAfter:   rpiAutoCleanStaleAfter,
		EnsureCleanup:         rpiEnsureCleanup,
		CleanupPruneWorktrees: rpiCleanupPruneWorktrees,
		GatePolicy:            strings.ToLower(strings.TrimSpace(rpiGatePolicy)),
		ValidateFastScript:    rpiValidateFastScript,
		SecurityGateScript:    rpiSecurityGateScript,
		LandingPolicy:         strings.ToLower(strings.TrimSpace(rpiLandingPolicy)),
		LandingBranch:         strings.TrimSpace(rpiLandingBranch),
		LandingCommitMessage:  rpiLandingCommitMessage,
		LandingLockPath:       rpiLandingLockPath,
		BDSyncPolicy:          strings.ToLower(strings.TrimSpace(rpiBDSyncPolicy)),
		CommandTimeout:        rpiCommandTimeout,
	}

	if rpiSupervisor {
		if !cmd.Flags().Changed("failure-policy") {
			cfg.FailurePolicy = loopFailurePolicyContinue
		}
		if !cmd.Flags().Changed("cycle-retries") {
			cfg.CycleRetries = 1
		}
		if !cmd.Flags().Changed("cycle-delay") {
			cfg.CycleDelay = 5 * time.Minute
		}
		if !cmd.Flags().Changed("lease") {
			cfg.LeaseEnabled = true
		}
		if !cmd.Flags().Changed("detached-heal") {
			cfg.DetachedHeal = true
		}
		if !cmd.Flags().Changed("auto-clean") {
			cfg.AutoClean = true
		}
		if !cmd.Flags().Changed("ensure-cleanup") {
			cfg.EnsureCleanup = true
		}
		if !cmd.Flags().Changed("gate-policy") {
			cfg.GatePolicy = loopGatePolicyRequired
		}
	}

	if cfg.CycleRetries < 0 {
		return cfg, fmt.Errorf("cycle-retries must be >= 0")
	}
	if cfg.RetryBackoff < 0 {
		return cfg, fmt.Errorf("retry-backoff must be >= 0")
	}
	if cfg.CycleDelay < 0 {
		return cfg, fmt.Errorf("cycle-delay must be >= 0")
	}
	if cfg.CommandTimeout < 0 {
		return cfg, fmt.Errorf("command-timeout must be >= 0")
	}
	if cfg.LeaseTTL <= 0 {
		cfg.LeaseTTL = 2 * time.Minute
	}
	if cfg.CommandTimeout == 0 {
		cfg.CommandTimeout = defaultLoopCommandTimeout
	}
	if cfg.AutoCleanStaleAfter <= 0 {
		cfg.AutoCleanStaleAfter = 24 * time.Hour
	}
	if cfg.LeasePath == "" {
		cfg.LeasePath = filepath.Join(".agents", "rpi", "supervisor.lock")
	}
	if cfg.LandingLockPath == "" {
		cfg.LandingLockPath = filepath.Join(".agents", "rpi", "landing.lock")
	}

	if cfg.FailurePolicy != loopFailurePolicyStop && cfg.FailurePolicy != loopFailurePolicyContinue {
		return cfg, fmt.Errorf("invalid failure-policy %q (valid: stop|continue)", cfg.FailurePolicy)
	}
	switch cfg.GatePolicy {
	case loopGatePolicyOff, loopGatePolicyBestEffort, loopGatePolicyRequired:
	default:
		return cfg, fmt.Errorf("invalid gate-policy %q (valid: off|best-effort|required)", cfg.GatePolicy)
	}
	switch cfg.LandingPolicy {
	case loopLandingPolicyOff, loopLandingPolicyCommit, loopLandingPolicySyncPush:
	default:
		return cfg, fmt.Errorf("invalid landing-policy %q (valid: off|commit|sync-push)", cfg.LandingPolicy)
	}
	switch cfg.BDSyncPolicy {
	case loopBDSyncPolicyAuto, loopBDSyncPolicyAlways, loopBDSyncPolicyNever:
	default:
		return cfg, fmt.Errorf("invalid bd-sync-policy %q (valid: auto|always|never)", cfg.BDSyncPolicy)
	}

	if !filepath.IsAbs(cfg.LeasePath) {
		cfg.LeasePath = filepath.Join(cwd, cfg.LeasePath)
	}
	if !filepath.IsAbs(cfg.LandingLockPath) {
		cfg.LandingLockPath = filepath.Join(cwd, cfg.LandingLockPath)
	}

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

func runRPISupervisedCycle(cwd, goal string, cycle, attempt int, cfg rpiLoopSupervisorConfig) (retErr error) {
	if cfg.DetachedHeal {
		branch, healed, err := ensureLoopAttachedBranch(cwd, cfg.DetachedBranchPrefix)
		if err != nil {
			return wrapCycleFailure(cycleFailureInfrastructure, "detached-head self-heal", err)
		}
		if healed {
			fmt.Printf("Detached HEAD detected. Switched to branch: %s\n", branch)
		}
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
			cleanupErr := runSupervisorCleanup(cwd, cfg.AutoCleanStaleAfter, cfg.CleanupPruneWorktrees)
			if cleanupErr == nil {
				return
			}
			if retErr == nil {
				retErr = wrapCycleFailure(cycleFailureInfrastructure, "supervisor cleanup", cleanupErr)
				return
			}
			VerbosePrintf("Warning: supervisor cleanup after failure: %v\n", cleanupErr)
		}()
	}

	opts := defaultPhasedEngineOptions()
	opts.AutoCleanStale = cfg.AutoClean
	opts.AutoCleanStaleAfter = cfg.AutoCleanStaleAfter
	opts.RuntimeMode = cfg.RuntimeMode
	opts.RuntimeCommand = cfg.RuntimeCommand
	opts.AOCommand = cfg.AOCommand
	opts.BDCommand = cfg.BDCommand
	opts.TmuxCommand = cfg.TmuxCommand

	if err := runPhasedEngine(cwd, goal, opts); err != nil {
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

func ensureLoopAttachedBranch(cwd, branchPrefix string) (string, bool, error) {
	repoRoot, err := cliRPI.GetRepoRoot(cwd, worktreeTimeout)
	if err != nil {
		return "", false, err
	}
	return cliRPI.EnsureAttachedBranch(repoRoot, worktreeTimeout, branchPrefix)
}

func runSupervisorCleanup(cwd string, staleAfter time.Duration, prune bool) error {
	return executeRPICleanup(cwd, "", true, prune, GetDryRun(), staleAfter)
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

		_, err := commitIfDirty(cwd, renderLandingCommitMessage(cfg.LandingCommitMessage, cycle, attempt, goal), cfg.CommandTimeout, scope)
		if err != nil {
			return err
		}
		return nil
	case loopLandingPolicySyncPush:
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
		targetBranch, err := resolveLandingBranch(cwd, cfg.LandingBranch, cfg.CommandTimeout)
		if err != nil {
			return err
		}
		if err := loopCommandRunner(cwd, cfg.CommandTimeout, "git", "fetch", "origin", targetBranch); err != nil {
			return wrapSyncPushLandingFailure(cwd, cfg.CommandTimeout, "fetch", err)
		}
		if err := loopCommandRunner(cwd, cfg.CommandTimeout, "git", "rebase", "origin/"+targetBranch); err != nil {
			return wrapSyncPushLandingFailure(cwd, cfg.CommandTimeout, "rebase", err)
		}

		runSync, err := shouldRunBDSync(cwd, cfg.BDSyncPolicy, cfg.BDCommand)
		if err != nil {
			return err
		}
		if runSync {
			if err := loopCommandRunner(cwd, cfg.CommandTimeout, cfg.BDCommand, "sync"); err != nil {
				return fmt.Errorf("bd sync failed: %w", err)
			}
		}

		if err := loopCommandRunner(cwd, cfg.CommandTimeout, "git", "push", "origin", "HEAD:"+targetBranch); err != nil {
			return fmt.Errorf("landing push failed: %w", err)
		}
		_ = loopCommandRunner(cwd, cfg.CommandTimeout, "git", "status", "-sb")
		return nil
	default:
		return fmt.Errorf("unsupported landing policy: %s", cfg.LandingPolicy)
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
	msg := strings.TrimSpace(template)
	if msg == "" {
		msg = "chore(rpi): autonomous cycle {{cycle}}"
	}
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
	command := strings.TrimSpace(bdCommand)
	if command == "" {
		command = "bd"
	}
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
	if err != nil && ctx.Err() == context.DeadlineExceeded {
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
	if err != nil && ctx.Err() == context.DeadlineExceeded {
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
	sort.Strings(owned)
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

type supervisorLeaseMetadata struct {
	RunID      string `json:"run_id"`
	PID        int    `json:"pid"`
	Host       string `json:"host"`
	CWD        string `json:"cwd"`
	AcquiredAt string `json:"acquired_at"`
	RenewedAt  string `json:"renewed_at"`
	ExpiresAt  string `json:"expires_at"`
}

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
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create lease directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("open lease file: %w", err)
	}

	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, fmt.Errorf("single-flight lease already held: %s", readLeaseHolderHint(path))
		}
		return nil, fmt.Errorf("acquire lease lock: %w", err)
	}

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
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
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

	unlockErr := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
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
