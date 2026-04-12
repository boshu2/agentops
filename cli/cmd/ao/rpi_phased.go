package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/autodev"
	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
)

var (
	phasedFrom         string
	phasedTestFirst    bool
	phasedNoTestFirst  bool
	phasedFastPath     bool
	phasedInteractive  bool
	phasedMaxRetries   int
	phasedPhaseTimeout time.Duration
	phasedStallTimeout time.Duration
	// phasedStreamStartupTimeout bounds how long stream backend can run without
	// receiving its first parsed event before falling back to direct execution.
	phasedStreamStartupTimeout time.Duration
	phasedNoWorktree           bool
	phasedLiveStatus           bool
	phasedSwarmFirst           bool
	phasedAutoCleanStale       bool
	phasedAutoCleanStaleAfter  time.Duration
	phasedRuntimeMode          string
	phasedRuntimeCommand       string
	phasedTmuxWorkers          int
	phasedNoBudget             bool
	phasedBudgetSpec           string
	phasedNoDashboard          bool
	phasedMixed                bool
	phasedDiscoveryArtifact    string
)

// phaseFailureReason is a thin alias for the internal PhaseFailureReason type.
type phaseFailureReason = cliRPI.PhaseFailureReason

// Phase failure reason constants delegate to internal/rpi.
var (
	failReasonTimeout = cliRPI.FailReasonTimeout
	failReasonStall   = cliRPI.FailReasonStall
	failReasonExit    = cliRPI.FailReasonExit
	failReasonUnknown = cliRPI.FailReasonUnknown
)

func init() {
	phasedCmd := &cobra.Command{
		Use:   "phased <goal>",
		Short: "Run RPI with fresh runtime session per phase",
		Long: `Orchestrate the full RPI lifecycle using 3 consolidated phases.

Each phase gets its own context window (Ralph Wiggum pattern):
  1. Discovery       — research + plan + pre-mortem (shared context, prompt cache hot)
  2. Implementation  — crank (fresh context for heavy work)
  3. Validation      — vibe + post-mortem (fresh eyes, independent of implementer)

This consolidation cuts cold starts from 6 to 3, keeps prompt cache warm
within each phase, and preserves the key isolation boundary: the implementer
session is never the validator session.

Between phases, the CLI reads filesystem artifacts, constructs prompts
via templates, and spawns the next session. Retry loops for gate failures
are handled within the session (discovery) or across sessions (validation).

Examples:
  ao rpi phased "add user authentication"       # full lifecycle (3 sessions)
  ao rpi phased --from=implementation "add auth" # skip to crank (needs epic)
  ao rpi phased --from=validation                # just vibe + post-mortem
  ao rpi phased --dry-run "add auth"             # show prompts without spawning
  ao rpi phased --fast-path "fix typo"           # force --quick for gates`,
		Args: cobra.MaximumNArgs(1),
		RunE: runRPIPhased,
	}

	phasedCmd.Flags().StringVar(&phasedFrom, "from", "discovery", "Start from phase (discovery, implementation, validation; aliases: research, plan, pre-mortem, crank, vibe, post-mortem)")
	phasedCmd.Flags().BoolVar(&phasedTestFirst, "test-first", true, "Default to strict-quality spec-first execution by passing --test-first to /crank")
	phasedCmd.Flags().BoolVar(&phasedNoTestFirst, "no-test-first", false, "Opt out of strict-quality spec-first execution (do not pass --test-first to /crank)")
	phasedCmd.Flags().BoolVar(&phasedFastPath, "fast-path", false, "Force fast path (--quick for gates)")
	phasedCmd.Flags().BoolVar(&phasedInteractive, "interactive", false, "Enable human gates at research and plan phases")
	phasedCmd.Flags().IntVar(&phasedMaxRetries, "max-retries", 3, "Maximum retry attempts per gate (default: 3)")
	phasedCmd.Flags().BoolVar(&phasedNoBudget, "no-budget", false, "Disable all phase budgets and run without time-box transitions")
	phasedCmd.Flags().StringVar(&phasedBudgetSpec, "budget", "", "Override phase budgets in seconds (<phase>:<seconds>, comma-separated), e.g. discovery:300,validation:120")
	phasedCmd.Flags().DurationVar(&phasedPhaseTimeout, "phase-timeout", 90*time.Minute, "Maximum wall-clock runtime per phase (0 disables timeout)")
	phasedCmd.Flags().DurationVar(&phasedStallTimeout, "stall-timeout", 10*time.Minute, "Maximum time without progress before declaring stall (0 disables)")
	phasedCmd.Flags().DurationVar(&phasedStreamStartupTimeout, "stream-startup-timeout", 45*time.Second, "Maximum time to wait for first stream event before falling back to direct execution (0 disables)")
	phasedCmd.Flags().BoolVar(&phasedNoWorktree, "no-worktree", false, "Disable worktree isolation (run in current directory)")
	phasedCmd.Flags().BoolVar(&phasedLiveStatus, "live-status", false, "Stream phase progress to a live-status.md file")
	phasedCmd.Flags().BoolVar(&phasedSwarmFirst, "swarm-first", true, "Default each phase to swarm/agent-team execution; fall back to direct execution if swarm runtime is unavailable")
	phasedCmd.Flags().BoolVar(&phasedAutoCleanStale, "auto-clean-stale", false, "Run stale-run cleanup before starting phased execution")
	phasedCmd.Flags().DurationVar(&phasedAutoCleanStaleAfter, "auto-clean-stale-after", 24*time.Hour, "Only clean stale runs older than this age when auto-clean is enabled")
	phasedCmd.Flags().StringVar(&phasedRuntimeMode, "runtime", "auto", "Phase runtime mode: auto|direct|stream|tmux")
	phasedCmd.Flags().StringVar(&phasedRuntimeCommand, "runtime-cmd", "claude", "Runtime command used for phase prompts (Claude uses '-p'; Codex uses 'exec')")
	phasedCmd.Flags().IntVar(&phasedTmuxWorkers, "tmux-workers", 1, "When --runtime tmux, number of worker sessions spawned per phase")
	phasedCmd.Flags().BoolVar(&phasedNoDashboard, "no-dashboard", false, "Disable auto-opening the web dashboard")
	phasedCmd.Flags().BoolVar(&phasedMixed, "mixed", false, "Enable cross-vendor mixed-model execution (planner and reviewer from different vendors)")
	phasedCmd.Flags().StringVar(&phasedDiscoveryArtifact, "discovery-artifact", "", "Path to a pre-validated discovery artifact (markdown) used to skip Phase 1 when combined with --from=implementation")

	rpiCmd.AddCommand(phasedCmd)
}

// runPhasedEngine runs the full phased RPI lifecycle for goal in cwd.
// It is the programmatic entry point used by both the phased cobra command
// and the loop command, ensuring both share the same runtime contracts.
func runPhasedEngine(ctx context.Context, cwd, goal string, opts phasedEngineOptions) error {
	if strings.TrimSpace(opts.WorkingDir) == "" {
		opts.WorkingDir = cwd
	}
	args := []string{goal}
	if goal == "" {
		args = nil
	}
	return runRPIPhasedWithOpts(ctx, opts, args)
}

// runRPIPhased is the cobra RunE handler for `ao rpi phased`.
// It reads options from package-level cobra flag variables and delegates to runRPIPhasedWithOpts.
func runRPIPhased(cmd *cobra.Command, args []string) error {
	opts := phasedEngineOptions{
		From:                 phasedFrom,
		FastPath:             phasedFastPath,
		TestFirst:            phasedTestFirst,
		Interactive:          phasedInteractive,
		MaxRetries:           phasedMaxRetries,
		PhaseTimeout:         phasedPhaseTimeout,
		StallTimeout:         phasedStallTimeout,
		StreamStartupTimeout: phasedStreamStartupTimeout,
		NoWorktree:           phasedNoWorktree,
		LiveStatus:           phasedLiveStatus,
		SwarmFirst:           phasedSwarmFirst,
		AutoCleanStale:       phasedAutoCleanStale,
		AutoCleanStaleAfter:  phasedAutoCleanStaleAfter,
		StallCheckInterval:   stallCheckInterval,
		RuntimeMode:          phasedRuntimeMode,
		RuntimeCommand:       phasedRuntimeCommand,
		TmuxWorkers:          phasedTmuxWorkers,
		NoBudget:             phasedNoBudget,
		BudgetSpec:           phasedBudgetSpec,
		NoDashboard:          phasedNoDashboard,
		Mixed:                phasedMixed,
		DiscoveryArtifact:    phasedDiscoveryArtifact,
	}
	if phasedNoTestFirst {
		opts.TestFirst = false
	}

	// Apply config-based worktree mode if the --no-worktree flag was not explicitly set.
	if !cmd.Flags().Changed("no-worktree") {
		opts.NoWorktree = resolveWorktreeModeFromConfig(opts.NoWorktree)
	}
	toolchain, err := resolveRPIToolchain(
		cliRPI.Toolchain{
			RuntimeMode:    phasedRuntimeMode,
			RuntimeCommand: phasedRuntimeCommand,
		},
		rpiToolchainFlagSet{
			RuntimeMode:    cmd.Flags().Changed("runtime"),
			RuntimeCommand: cmd.Flags().Changed("runtime-cmd"),
		},
	)
	if err != nil {
		return err
	}
	opts.RuntimeMode = toolchain.RuntimeMode
	opts.RuntimeCommand = toolchain.RuntimeCommand
	opts.AOCommand = toolchain.AOCommand
	opts.BDCommand = toolchain.BDCommand
	opts.TmuxCommand = toolchain.TmuxCommand
	if cmd.Flags().Changed("auto-clean-stale-after") {
		opts.AutoCleanStale = true
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return runRPIPhasedWithOpts(ctx, opts, args)
}

// normalizeOptsCommands resolves all runtime/tool commands to their effective values.
func normalizeOptsCommands(opts *phasedEngineOptions) {
	opts.RuntimeMode = normalizeRuntimeMode(opts.RuntimeMode)
	opts.RuntimeCommand = effectiveRuntimeCommand(opts.RuntimeCommand)
	opts.AOCommand = effectiveAOCommand(opts.AOCommand)
	opts.BDCommand = effectiveBDCommand(opts.BDCommand)
	opts.TmuxCommand = effectiveTmuxCommand(opts.TmuxCommand)
	if opts.TmuxWorkers <= 0 {
		opts.TmuxWorkers = 1
	}
}

// applyComplexityFastPath classifies goal complexity and activates the fast path
// (skips council validation) for trivial goals.
func applyComplexityFastPath(state *phasedState, opts phasedEngineOptions) {
	complexity := classifyComplexity(state.Goal)
	state.Complexity = complexity
	fmt.Printf("RPI mode: rpi-phased (complexity: %s)\n", complexity)
	if complexity == ComplexityFast && !opts.FastPath {
		state.FastPath = true
		fmt.Println("Complexity: fast — skipping validation phase (phase 3)")
	}
}

// saveTerminalState writes a terminal status/reason to state and persists it.
func saveTerminalState(spawnCwd string, state *phasedState, status, reason string) {
	state.TerminalStatus = status
	state.TerminalReason = reason
	state.TerminatedAt = time.Now().Format(time.RFC3339)
	if err := savePhasedState(spawnCwd, state); err != nil {
		VerbosePrintf("Warning: could not persist %s terminal state: %v\n", status, err)
	}
}

// preflightOpts normalizes, validates, and checks runtime availability for opts.
func preflightOpts(opts *phasedEngineOptions) error {
	normalizeOptsCommands(opts)
	if opts.NoBudget && strings.TrimSpace(opts.BudgetSpec) != "" {
		return fmt.Errorf("cannot combine --no-budget with --budget")
	}
	if _, err := parsePhaseBudgetSpec(opts.BudgetSpec); err != nil {
		return err
	}
	if err := validateRuntimeMode(opts.RuntimeMode); err != nil {
		return err
	}
	if opts.RuntimeMode == "tmux" {
		if _, err := defaultLookPath(opts.LookPath)(opts.TmuxCommand); err != nil {
			return fmt.Errorf("tmux executable %q not found on PATH (required for runtime=tmux)", opts.TmuxCommand)
		}
	}
	return preflightRuntimeAvailability(opts.RuntimeCommand, opts.LookPath)
}

func minPositiveDuration(a, b time.Duration) time.Duration {
	return cliRPI.MinPositiveDuration(a, b)
}

// initExecutorAndPersist selects the executor backend for the run and persists
// the initial state with the backend name.
func initExecutorAndPersist(spawnCwd, logPath, statusPath string, allPhases []PhaseProgress, state *phasedState, opts phasedEngineOptions) PhaseExecutor {
	executor := selectExecutorWithLog(statusPath, allPhases, logPath, state.RunID, opts.LiveStatus, opts)
	state.Backend = executor.Name()
	if err := savePhasedState(spawnCwd, state); err != nil {
		VerbosePrintf("Warning: could not persist startup state: %v\n", err)
	}
	updateRunHeartbeat(spawnCwd, state.RunID)
	return executor
}

// runRPIPhasedWithOpts is the core implementation of the phased RPI lifecycle.
// All configuration is read from opts; no package-level globals are read after
// this point (except test-injection points: lookPath and spawnDirectFn).
// initPhasedState resolves the goal and start phase from args, creates the
// phasedState, applies complexity fast-path, and resumes from prior state if needed.
func initPhasedState(cwd string, opts phasedEngineOptions, args []string) (*phasedState, int, string, error) {
	goal, startPhase, err := resolveGoalAndStartPhase(opts, args, cwd)
	if err != nil {
		return nil, 0, "", err
	}
	state := newPhasedState(opts, startPhase, goal)
	applyComplexityFastPath(state, opts)

	spawnCwd, err := resumePhasedStateIfNeeded(cwd, opts, startPhase, goal, state)
	if err != nil {
		return nil, 0, "", err
	}
	if err := attachAutodevProgram(spawnCwd, state); err != nil {
		return nil, 0, "", err
	}
	return state, startPhase, spawnCwd, nil
}

type phasedRunLifecycle struct {
	cwd               string
	spawnCwd          string
	state             *phasedState
	startPhase        int
	preloadedArtifact *discoveryArtifact
	cleanupWorktree   func(success bool, logPath string) error
	logPath           string
	statusPath        string
	allPhases         []PhaseProgress
}

func runRPIPhasedWithOpts(ctx context.Context, opts phasedEngineOptions, args []string) (retErr error) {
	run, err := preparePhasedRun(&opts, args)
	if err != nil {
		return err
	}

	cleanupSuccess := false
	defer func() {
		if cleanupErr := run.cleanupWorktree(cleanupSuccess, run.logPath); cleanupErr != nil && retErr == nil {
			retErr = cleanupErr
		}
	}()

	if err := initializePhasedRun(run, opts); err != nil {
		return err
	}

	if dashSrv := startPhasedDashboardIfNeeded(run.spawnCwd, run.state, &opts); dashSrv != nil {
		defer shutdownDashboard(dashSrv)
	}

	runStart := time.Now()
	if err := runPhaseLoopWithBudgets(ctx, run.cwd, run.spawnCwd, run.state, run.startPhase, opts, run.statusPath, run.allPhases, run.logPath); err != nil {
		finalizeFailedPhasedRun(run.spawnCwd, run.state, runStart, err)
		return err
	}

	finalizeSuccessfulPhasedRun(run.spawnCwd, run.state, runStart, run.logPath)
	cleanupSuccess = true

	return nil
}

func preparePhasedRun(opts *phasedEngineOptions, args []string) (*phasedRunLifecycle, error) {
	cwd := strings.TrimSpace(opts.WorkingDir)
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}
	if err := preflightOpts(opts); err != nil {
		return nil, err
	}
	maybeAutoCleanStale(*opts, cwd)

	preloadedArtifact, args, err := preloadDiscoveryArtifact(opts.DiscoveryArtifact, args)
	if err != nil {
		return nil, err
	}

	originalCwd := cwd
	state, startPhase, spawnCwd, err := initPhasedState(cwd, *opts, args)
	if err != nil {
		return nil, err
	}

	spawnCwd, cleanupWorktree, err := setupWorktreeLifecycle(spawnCwd, originalCwd, *opts, state)
	if err != nil {
		return nil, err
	}
	return &phasedRunLifecycle{
		cwd:               cwd,
		spawnCwd:          spawnCwd,
		state:             state,
		startPhase:        startPhase,
		preloadedArtifact: preloadedArtifact,
		cleanupWorktree:   cleanupWorktree,
	}, nil
}

func initializePhasedRun(run *phasedRunLifecycle, opts phasedEngineOptions) error {
	if opts.OnSpawnCwdReady != nil {
		opts.OnSpawnCwdReady(run.spawnCwd)
	}

	ensureStateRunID(run.state)
	run.state.OrchestratorPID = os.Getpid()

	_, runLogPath, statusPath, allPhases, err := initializeRunArtifacts(run.spawnCwd, run.startPhase, run.state, opts)
	if err != nil {
		return err
	}
	run.logPath = runLogPath
	run.statusPath = statusPath
	run.allPhases = allPhases
	if err := writeExecutionPacketSeed(run.spawnCwd, run.state); err != nil {
		return err
	}
	if err := applyDiscoveryArtifactToPacket(run.spawnCwd, run.preloadedArtifact, run.startPhase, run.state.Goal); err != nil {
		return err
	}
	if err := updateExecutionPacketProof(run.spawnCwd, run.state); err != nil {
		VerbosePrintf("Warning: could not initialize execution packet proof: %v\n", err)
	}

	logPhaseTransition(run.logPath, run.state.RunID, "start", fmt.Sprintf("goal=%q from=%s complexity=%s fast_path=%v", run.state.Goal, opts.From, run.state.Complexity, run.state.FastPath))

	_ = initExecutorAndPersist(run.spawnCwd, run.logPath, run.statusPath, run.allPhases, run.state, opts)
	emitRunStarted(run.spawnCwd, run.state)
	return nil
}

func emitRunStarted(spawnCwd string, state *phasedState) {
	if _, evErr := appendRPIC2Event(spawnCwd, rpiC2EventInput{
		RunID: state.RunID, Phase: 0, Backend: state.Backend, Source: "orchestrator",
		Type: "run.started", Message: state.Goal,
		Details: map[string]any{"goal": state.Goal, "complexity": string(state.Complexity), "backend": state.Backend},
	}); evErr != nil {
		VerbosePrintf("Warning: could not emit run.started event: %v\n", evErr)
	}
}

func startPhasedDashboardIfNeeded(spawnCwd string, state *phasedState, opts *phasedEngineOptions) *http.Server {
	if opts.NoDashboard || GetDryRun() || !isTerminal() {
		return nil
	}
	srv, dashURL := startEmbeddedDashboard(spawnCwd, state.RunID, opts.NoDashboard)
	if srv == nil {
		return nil
	}
	fmt.Printf("Mission control: %s\n", dashURL)

	// When dashboard is active, suppress raw Claude session output from executors.
	// Orchestrator status lines (fmt.Printf) are unaffected — they go to os.Stdout directly.
	opts.StdoutWriter = io.Discard
	return srv
}

func finalizeFailedPhasedRun(spawnCwd string, state *phasedState, runStart time.Time, err error) {
	saveTerminalState(spawnCwd, state, "failed", err.Error())
	emitRunCompleted(spawnCwd, state, runStart)
	if proofErr := updateExecutionPacketProof(spawnCwd, state); proofErr != nil {
		VerbosePrintf("Warning: could not refresh failed-run proof artifact set: %v\n", proofErr)
	}
}

func finalizeSuccessfulPhasedRun(spawnCwd string, state *phasedState, runStart time.Time, logPath string) {
	saveTerminalState(spawnCwd, state, "completed", "all phases completed")
	emitRunCompleted(spawnCwd, state, runStart)
	if err := updateExecutionPacketProof(spawnCwd, state); err != nil {
		VerbosePrintf("Warning: could not refresh completed-run proof artifact set: %v\n", err)
	}

	writeFinalPhasedReport(state, logPath)
}

func attachAutodevProgram(cwd string, state *phasedState) error {
	rel := autodev.ResolveProgramPath(cwd)
	if rel == "" {
		state.ProgramPath = ""
		return nil
	}
	prog, _, err := autodev.LoadProgram(filepath.Join(cwd, rel))
	if err != nil {
		return fmt.Errorf("loading %s: %w", rel, err)
	}
	if errs := autodev.ValidateProgram(prog); len(errs) > 0 {
		var details []string
		for _, err := range errs {
			details = append(details, err.Error())
		}
		return fmt.Errorf("invalid %s: %s", rel, strings.Join(details, "; "))
	}
	state.ProgramPath = rel
	return nil
}

// emitRunCompleted appends a run.completed C2 event with timing and verdict data.
func emitRunCompleted(spawnCwd string, state *phasedState, runStart time.Time) {
	elapsed := time.Since(runStart).Round(time.Second)
	if _, err := appendRPIC2Event(spawnCwd, rpiC2EventInput{
		RunID: state.RunID, Phase: 0, Backend: state.Backend, Source: "orchestrator",
		Type: "run.completed", Message: fmt.Sprintf("completed in %s", elapsed),
		Details: map[string]any{"verdicts": state.Verdicts, "status": state.TerminalStatus, "elapsed_seconds": elapsed.Seconds()},
	}); err != nil {
		VerbosePrintf("Warning: could not emit run.completed event: %v\n", err)
	}
}

func appendTimeBoxedMarker(spawnCwd string, p phase, budget time.Duration) error {
	return cliRPI.AppendTimeBoxedMarker(spawnCwd, p.Num, p.Name, budget)
}

func handleBudgetTimeout(spawnCwd string, state *phasedState, p phase, budget time.Duration, logPath string, phaseStart time.Time) bool {
	if budget <= 0 {
		return false
	}
	if err := appendTimeBoxedMarker(spawnCwd, p, budget); err != nil {
		VerbosePrintf("Warning: could not append [TIME-BOXED] marker: %v\n", err)
	}

	now := time.Now()
	// Write structured phase result with time_boxed status
	pr := &phaseResult{
		SchemaVersion:   1,
		RunID:           state.RunID,
		Phase:           p.Num,
		PhaseName:       p.Name,
		Status:          "time_boxed",
		StartedAt:       phaseStart.Format(time.RFC3339),
		CompletedAt:     now.Format(time.RFC3339),
		DurationSeconds: now.Sub(phaseStart).Seconds(),
	}
	if err := writePhaseResult(spawnCwd, pr); err != nil {
		VerbosePrintf("Warning: could not write time_boxed phase result: %v\n", err)
	}

	msg := fmt.Sprintf("[TIME-BOXED] Phase %s time-boxed at %ds (budget: %ds)", p.Name, int(budget.Seconds()), int(budget.Seconds()))
	fmt.Println(msg)
	logPhaseTransition(logPath, state.RunID, p.Name, msg)
	return true
}

func runPhaseLoopWithBudgets(ctx context.Context, cwd, spawnCwd string, state *phasedState, startPhase int, opts phasedEngineOptions, statusPath string, allPhases []PhaseProgress, logPath string) error {
	for i := startPhase; i <= len(phases); i++ {
		p := phases[i-1]
		if p.Num == 3 && state.FastPath && state.Complexity == ComplexityFast {
			fmt.Printf("\n--- Phase 3: validation (skipped — complexity: fast) ---\n")
			logPhaseTransition(logPath, state.RunID, "validation", "skipped — complexity: fast")
			continue
		}

		phaseOpts := opts
		budget, hasBudget, err := resolvePhaseBudget(state, p.Num)
		if err != nil {
			return fmt.Errorf("resolve budget for phase %d (%s): %w", p.Num, p.Name, err)
		}
		if hasBudget {
			phaseOpts.PhaseTimeout = minPositiveDuration(phaseOpts.PhaseTimeout, budget)
			logPhaseTransition(logPath, state.RunID, p.Name, fmt.Sprintf("budget guard active (phase-timeout=%s, budget=%s)", phaseOpts.PhaseTimeout, budget))
		}

		phaseExecutor := selectExecutorWithLog(statusPath, allPhases, logPath, state.RunID, phaseOpts.LiveStatus, phaseOpts)
		phaseStart := time.Now()
		if err := runSinglePhase(ctx, cwd, spawnCwd, state, startPhase, p, phaseOpts, statusPath, allPhases, logPath, phaseExecutor); err != nil {
			if hasBudget && isPhaseTimeoutError(err) && handleBudgetTimeout(spawnCwd, state, p, budget, logPath, phaseStart) {
				continue
			}
			return logAndFailPhase(state, p.Name, logPath, spawnCwd, err)
		}
	}
	return nil
}

// maybeAutoCleanStale runs stale-run cleanup before starting if the option is enabled.
// Errors are non-fatal and logged via VerbosePrintf.
func maybeAutoCleanStale(opts phasedEngineOptions, cwd string) {
	if !opts.AutoCleanStale {
		return
	}
	minAge := opts.AutoCleanStaleAfter
	if minAge <= 0 {
		minAge = 24 * time.Hour
	}
	fmt.Printf("Auto-cleaning stale runs older than %s before starting\n", minAge)
	if err := executeRPICleanup(cwd, "", true, false, false, GetDryRun(), minAge); err != nil {
		VerbosePrintf("Warning: auto-clean stale runs failed: %v\n", err)
	}
}
