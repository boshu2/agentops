package main

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/boshu2/agentops/cli/internal/rpi"
	"github.com/spf13/cobra"
)

var (
	rpiMaxCycles             int
	rpiRepoFilter            string
	rpiSupervisor            bool
	rpiRalph                 bool
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
	rpiCompile               bool
	rpiCompileInterval       time.Duration
	rpiCompileSince          string
	rpiCompileDefrag         bool
)

// Type aliases — canonical definitions live in internal/rpi.
type (
	nextWorkEntry             = rpi.NextWorkEntry
	nextWorkProofRef          = rpi.NextWorkProofRef
	nextWorkItem              = rpi.NextWorkItem
	queueSelection            = rpi.QueueSelection
	queuePreflightDecision    = rpi.QueuePreflightDecision
	nextWorkProofDecision     = rpi.NextWorkProofDecision
	evidenceOnlyClosureProof  = rpi.EvidenceOnlyClosureProof
	evidenceOnlyClosurePacket = rpi.EvidenceOnlyClosurePacket
	loopCycleResult           = rpi.LoopCycleResult
	compileProducerState      = rpi.CompileProducerState
)

var errQueueClaimConflict = rpi.ErrQueueClaimConflict

// Loop control constants — canonical definitions live in internal/rpi.
var (
	loopContinue = rpi.LoopContinue
	loopBreak    = rpi.LoopBreak
	loopReturn   = rpi.LoopReturn
)

var (
	queueProofTargetPattern     = rpi.QueueProofTargetPattern
	queueProofPacketPathPattern = rpi.QueueProofPacketPathPattern
	preflightQueueSelectionFn   = preflightQueueSelection
	queuePreflightConsumedBy    = "ao-rpi-loop:preflight"
)

func init() {
	loopCmd := &cobra.Command{
		Use:   "loop [goal]",
		Short: "Run continuous RPI cycles from next-work queue",
		Long: `Execute RPI cycles in a loop, consuming from next-work.jsonl.

Each cycle drives a queue item through the full phased RPI engine:
  1. Read unconsumed items from .agents/rpi/next-work.jsonl
  2. Pick highest-severity item as goal (or use explicit goal)
  3. Run: ao rpi phased "<goal>" (discovery → implementation → validation)
  4. Claim the queue item while it runs, then consume on success or release on failure
  5. Re-read next-work.jsonl (post-mortem may have harvested new items)
  6. Repeat until queue empty or max-cycles reached

Queue semantics:
  - An item is only marked consumed after the phased engine completes without error.
  - Queue items are claimed before execution and released back to available state
    on interruption or failure so harvested work can continue compounding.
  - Task failures record failed_at per item for retry ordering, but do not consume
    sibling items in the same harvested batch.
  - Already-consumed items and currently-claimed items are skipped (idempotent).

Examples:
  ao rpi loop                          # consume from queue until stable
  ao rpi loop "improve test coverage"  # run one cycle with explicit goal
  ao rpi loop --max-cycles 3           # cap at 3 iterations
  ao rpi loop --repo-filter agentops   # only process items targeting agentops
  ao rpi loop --dry-run                # show what would run`,
		Args: cobra.MaximumNArgs(1),
		RunE: runRPILoop,
	}

	addRPILoopFlags(loopCmd)

	rpiCmd.AddCommand(loopCmd)
}

func addRPILoopFlags(cmd *cobra.Command) {
	cmd.Flags().IntVar(&rpiMaxCycles, "max-cycles", 0, "Maximum cycles (0 = unlimited, stop when queue empty)")
	cmd.Flags().StringVar(&rpiRepoFilter, "repo-filter", "", "Only process queue items targeting this repo (empty = all)")
	cmd.Flags().BoolVar(&rpiSupervisor, "supervisor", false, "Enable autonomous supervisor mode (lease lock, self-heal, retries, gates, cleanup)")
	cmd.Flags().BoolVar(&rpiRalph, "ralph", false, "Enable Ralph-mode preset for unattended external loop supervision (implies supervisor defaults with safe nonstop settings)")
	cmd.Flags().StringVar(&rpiFailurePolicy, "failure-policy", "stop", "Cycle failure policy: stop|continue")
	cmd.Flags().IntVar(&rpiCycleRetries, "cycle-retries", 0, "Automatic retry count per cycle after a failed attempt")
	cmd.Flags().DurationVar(&rpiRetryBackoff, "retry-backoff", 30*time.Second, "Backoff between cycle retry attempts")
	cmd.Flags().DurationVar(&rpiCycleDelay, "cycle-delay", 0, "Delay between completed cycles")
	cmd.Flags().BoolVar(&rpiLease, "lease", false, "Acquire a single-flight supervisor lease lock before running")
	cmd.Flags().StringVar(&rpiLeasePath, "lease-path", filepath.Join(".agents", "rpi", "supervisor.lock"), "Lease lock file path (absolute or repo-relative)")
	cmd.Flags().DurationVar(&rpiLeaseTTL, "lease-ttl", 2*time.Minute, "Lease heartbeat TTL for supervisor lock metadata")
	cmd.Flags().BoolVar(&rpiDetachedHeal, "detached-heal", false, "Auto-create/switch to a named branch when HEAD is detached")
	cmd.Flags().StringVar(&rpiDetachedBranchPrefix, "detached-branch-prefix", "codex/auto-rpi", "Branch prefix used by detached HEAD self-heal")
	cmd.Flags().BoolVar(&rpiAutoClean, "auto-clean", false, "Run stale RPI cleanup before each phased cycle")
	cmd.Flags().DurationVar(&rpiAutoCleanStaleAfter, "auto-clean-stale-after", 24*time.Hour, "Only auto-clean runs older than this age")
	cmd.Flags().BoolVar(&rpiEnsureCleanup, "ensure-cleanup", false, "Run stale-run cleanup after each cycle (cleanup guarantee)")
	cmd.Flags().BoolVar(&rpiCleanupPruneWorktrees, "cleanup-prune-worktrees", true, "Run git worktree prune during supervisor cleanup")
	cmd.Flags().BoolVar(&rpiCleanupPruneBranches, "cleanup-prune-branches", false, "Run legacy branch cleanup during supervisor cleanup")
	cmd.Flags().StringVar(&rpiGatePolicy, "gate-policy", "off", "Quality/security gate policy: off|best-effort|required")
	cmd.Flags().StringVar(&rpiValidateFastScript, "gate-fast-script", filepath.Join("scripts", "validate-go-fast.sh"), "Fast validation gate script path")
	cmd.Flags().StringVar(&rpiSecurityGateScript, "gate-security-script", filepath.Join("scripts", "security-gate.sh"), "Security gate script path")
	cmd.Flags().StringVar(&rpiLandingPolicy, "landing-policy", "off", "Landing policy after successful cycle: off|commit|sync-push")
	cmd.Flags().StringVar(&rpiLandingBranch, "landing-branch", "", "Landing target branch (empty resolves origin/HEAD, then current branch, then main)")
	cmd.Flags().StringVar(&rpiLandingCommitMessage, "landing-commit-message", "chore(rpi): autonomous cycle {{cycle}}", "Commit message template for landing policies that commit")
	cmd.Flags().StringVar(&rpiLandingLockPath, "landing-lock-path", filepath.Join(".agents", "rpi", "landing.lock"), "Landing lock file path for synchronized integration (absolute or repo-relative)")
	cmd.Flags().StringVar(&rpiBDSyncPolicy, "bd-sync-policy", "auto", "Legacy bd landing checkpoint policy: auto|always|never (auto/always run 'bd export -o /dev/null' on current bd releases)")
	cmd.Flags().DurationVar(&rpiCommandTimeout, "command-timeout", 20*time.Minute, "Timeout for supervisor external commands (git/bd/gate scripts)")
	cmd.Flags().StringVar(&rpiKillSwitchPath, "kill-switch-path", filepath.Join(".agents", "rpi", "KILL"), "Supervisor kill-switch file path checked at cycle boundaries (absolute or repo-relative)")
	cmd.Flags().BoolVar(&rpiCompile, "compile", false, "Enable Compile producer cadence before queue selection")
	cmd.Flags().DurationVar(&rpiCompileInterval, "compile-interval", 30*time.Minute, "Minimum interval between Compile producer ticks (0 = every cycle)")
	cmd.Flags().StringVar(&rpiCompileSince, "compile-since", "26h", "Lookback window for Compile mine producer")
	cmd.Flags().BoolVar(&rpiCompileDefrag, "compile-defrag", false, "Run defrag sweep after Compile mine producer tick")
}

var (
	runRPISupervisedCycleFn  func(context.Context, string, string, int, int, rpiLoopSupervisorConfig) error = runRPISupervisedCycle
	runCompileProducerTickFn                                                                                = runCompileProducerTick
)

func runRPILoop(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	cfg, err := resolveLoopSupervisorConfig(cmd, cwd)
	if err != nil {
		return err
	}

	explicitGoal := ""
	if len(args) > 0 {
		explicitGoal = args[0]
	}

	nextWorkPath := filepath.Join(cwd, ".agents", "rpi", "next-work.jsonl")
	if err := os.MkdirAll(filepath.Join(cwd, ".agents", "rpi"), 0750); err != nil {
		return fmt.Errorf("ensure .agents/rpi directory: %w", err)
	}

	if cfg.LeaseEnabled && !GetDryRun() {
		runID := generateRunID()
		lease, leaseErr := acquireSupervisorLease(cwd, cfg.LeasePath, cfg.LeaseTTL, runID)
		if leaseErr != nil {
			return leaseErr
		}
		defer func() {
			if releaseErr := lease.Release(); releaseErr != nil {
				VerbosePrintf("Warning: could not release supervisor lease: %v\n", releaseErr)
			}
		}()
		fmt.Printf("Supervisor lease acquired: %s (run=%s)\n", lease.Path(), runID)
	}

	return executeLoopCycles(cwd, explicitGoal, nextWorkPath, cfg)
}

// errKillSwitchActivated is a sentinel returned when the kill switch fires
// during cycle retries, signaling a clean early exit without queue mutation.
var errKillSwitchActivated = fmt.Errorf("kill switch activated")

// executeLoopCycles runs the main RPI loop consuming from the next-work queue.
func executeLoopCycles(cwd, explicitGoal, nextWorkPath string, cfg rpiLoopSupervisorConfig) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cycle := 0
	executedCycles := 0
	compileState := compileProducerState{}
	for {
		cycle++

		if rpiMaxCycles > 0 && cycle > rpiMaxCycles {
			fmt.Printf("\nReached max cycles (%d). Stopping.\n", rpiMaxCycles)
			break
		}
		stop, err := applyCycleDelay(ctx, cycle, cfg)
		if err != nil {
			return err
		}
		if stop {
			break
		}

		loopLabel := loopSurfaceDisplayName(cfg)
		fmt.Printf("\n=== %s Loop: Cycle %d ===\n", loopLabel, cycle)
		if err := maybeRunCompileProducerCadence(cwd, explicitGoal, cfg, &compileState); err != nil {
			return err
		}

		goal, sel, action, err := resolveLoopGoal(cwd, explicitGoal, nextWorkPath, cfg)
		if err != nil {
			return err
		}
		if action == loopBreak {
			break
		}

		fmt.Printf("Running phased engine for: %q\n", goal)
		executedCycles++

		result, err := runCycleWithRetries(ctx, cwd, goal, cycle, executedCycles, nextWorkPath, sel, explicitGoal, cfg)
		if err != nil {
			return err
		}
		if result == loopBreak {
			break
		}
	}

	fmt.Printf("\n%s loop finished after %d cycle(s).\n", loopSurfaceDisplayName(cfg), executedCycles)
	return nil
}

func loopSurfaceDisplayName(cfg rpiLoopSupervisorConfig) string {
	if cfg.Surface == loopSurfaceEvolve {
		return "Evolve"
	}
	return "RPI"
}

func maybeRunCompileProducerCadence(cwd, explicitGoal string, cfg rpiLoopSupervisorConfig, state *compileProducerState) error {
	if explicitGoal != "" || !cfg.CompileEnabled {
		return nil
	}
	if GetDryRun() {
		fmt.Println("[dry-run] Skipping Compile producer cadence.")
		return nil
	}
	if state != nil && cfg.CompileInterval > 0 && !state.LastTick.IsZero() {
		if elapsed := time.Since(state.LastTick); elapsed < cfg.CompileInterval {
			return nil
		}
	}
	if err := runCompileProducerTickFn(cwd, cfg); err != nil {
		wrapped := wrapCycleFailure(cycleFailureInfrastructure, "compile producer", err)
		if cfg.ShouldContinueAfterFailure() {
			VerbosePrintf("Warning: %v\n", wrapped)
			return nil
		}
		return wrapped
	}
	if state != nil {
		state.LastTick = time.Now()
	}
	return nil
}

func runCompileProducerTick(cwd string, cfg rpiLoopSupervisorConfig) error {
	aoCommand := cmp.Or(strings.TrimSpace(cfg.AOCommand), "ao")
	since := cmp.Or(strings.TrimSpace(cfg.CompileSince), "26h")

	mineArgs := []string{"mine", "--emit-work-items", "--since", since, "--quiet"}
	fmt.Printf("Compile producer tick: %s %s\n", aoCommand, strings.Join(mineArgs, " "))
	if err := loopCommandRunner(cwd, cfg.CommandTimeout, aoCommand, mineArgs...); err != nil {
		return fmt.Errorf("compile mine producer failed: %w", err)
	}

	if !cfg.CompileDefrag {
		return nil
	}

	defragArgs := []string{"defrag", "--prune", "--dedup", "--oscillation-sweep", "--quiet"}
	fmt.Printf("Compile producer defrag: %s %s\n", aoCommand, strings.Join(defragArgs, " "))
	if err := loopCommandRunner(cwd, cfg.CommandTimeout, aoCommand, defragArgs...); err != nil {
		return fmt.Errorf("compile defrag sweep failed: %w", err)
	}
	return nil
}

// cancelableSleep blocks for d or until ctx is canceled, whichever comes first.
func cancelableSleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// applyCycleDelay handles inter-cycle delay and kill-switch checking.
// Returns (true, nil) when the loop should stop.
func applyCycleDelay(ctx context.Context, cycle int, cfg rpiLoopSupervisorConfig) (bool, error) {
	killSwitchSet, killErr := isLoopKillSwitchSet(cfg)
	if killErr != nil {
		return false, killErr
	}
	if killSwitchSet {
		fmt.Printf("Kill switch detected (%s). Stopping loop.\n", cfg.KillSwitchPath)
		return true, nil
	}
	if cycle > 1 && cfg.CycleDelay > 0 {
		fmt.Printf("\nSleeping %s before next cycle...\n", cfg.CycleDelay.Round(time.Second))
		if err := cancelableSleep(ctx, cfg.CycleDelay); err != nil {
			return true, nil
		}
	}
	return false, nil
}

// resolveLoopGoal determines the goal and queue selection for a cycle.
// Returns the goal string, optional queue selection, and a loop action.
func resolveLoopGoal(cwd, explicitGoal, nextWorkPath string, cfg rpiLoopSupervisorConfig) (string, *queueSelection, loopCycleResult, error) {
	goal := explicitGoal
	var sel *queueSelection

	if goal == "" {
		for {
			entries, err := readQueueEntries(nextWorkPath)
			if err != nil {
				VerbosePrintf("Warning: %v\n", err)
			}
			sel = selectHighestSeverityEntry(entries, rpiRepoFilter)
			if sel == nil {
				printEmptyQueueMessage(cfg)
				return "", nil, loopBreak, nil
			}
			if GetDryRun() {
				goal = sel.Item.Title
				fmt.Printf("From queue: %s\n", goal)
				break
			}
			decision, preflightErr := preflightQueueSelectionFn(cwd, sel, cfg)
			if preflightErr != nil {
				return "", nil, loopReturn, preflightErr
			}
			if !decision.Consume {
				goal = sel.Item.Title
				fmt.Printf("From queue: %s\n", goal)
				break
			}
			if err := markItemConsumed(nextWorkPath, sel.EntryIndex, sel.ItemIndex, queuePreflightConsumedBy); err != nil {
				if errors.Is(err, errQueueClaimConflict) {
					continue
				}
				return "", nil, loopReturn, fmt.Errorf("consume preflight queue item %q: %w", sel.Item.Title, err)
			}
			reason := strings.TrimSpace(decision.Reason)
			if reason == "" {
				reason = "repo preflight indicates the work is already satisfied"
			}
			fmt.Printf("Queue preflight consumed %q: %s\n", sel.Item.Title, reason)
		}
	}

	if goal == "" {
		fmt.Println("No goal and empty queue. Nothing to do.")
		return "", nil, loopBreak, nil
	}

	if GetDryRun() {
		fmt.Printf("[dry-run] Would run phased engine for: %q\n", goal)
		if explicitGoal == "" {
			fmt.Println("[dry-run] Queue not consumed in dry-run. Showing first cycle only.")
		}
		return goal, sel, loopBreak, nil
	}

	return goal, sel, loopContinue, nil
}

func printEmptyQueueMessage(cfg rpiLoopSupervisorConfig) {
	if cfg.Surface != loopSurfaceEvolve {
		fmt.Println("No unconsumed work in queue. Flywheel stable.")
		return
	}
	fmt.Println("No unconsumed work in next-work queue.")
	if GetDryRun() {
		fmt.Println("[dry-run] Evolve generator fallback would inspect bd ready, ao goals measure, testing improvements, validation/bug-hunt passes, drift cleanup, and feature suggestions before dormancy.")
		return
	}
	fmt.Println("Evolve queue stable. Generator fallback layers must be exhausted before treating this as dormancy.")
}

func preflightQueueSelection(cwd string, sel *queueSelection, cfg rpiLoopSupervisorConfig) (queuePreflightDecision, error) {
	_ = cfg
	if sel == nil {
		return queuePreflightDecision{}, nil
	}
	proof := classifyNextWorkCompletionProof(cwd, sel.SourceEpic, sel.Item)
	if !proof.Complete {
		return queuePreflightDecision{}, nil
	}
	switch proof.Source {
	case "completed_run":
		return queuePreflightDecision{
			Consume: true,
			Reason:  fmt.Sprintf("matched completed RPI run %s for source_epic %s", proof.Detail, sel.SourceEpic),
		}, nil
	case "execution_packet":
		return queuePreflightDecision{
			Consume: true,
			Reason:  fmt.Sprintf("matched execution packet proof for run %s", proof.Detail),
		}, nil
	case "evidence_only_closure":
		return queuePreflightDecision{
			Consume: true,
			Reason:  fmt.Sprintf("matched evidence-only closure proof for %s", proof.Detail),
		}, nil
	}
	return queuePreflightDecision{}, nil
}

func classifyNextWorkCompletionProof(cwd string, sourceEpic string, item nextWorkItem) nextWorkProofDecision {
	if item.ProofRef != nil {
		switch item.ProofRef.Kind {
		case "completed_run":
			if run := findCompletedRunByID(cwd, item.ProofRef.RunID); run != nil {
				return nextWorkProofDecision{Complete: true, Source: "completed_run", Detail: run.RunID}
			}
		case "execution_packet":
			// Prefer proof_ref.path: if the artifact file exists and is a
			// non-empty JSON object, that is sufficient proof regardless of
			// whether we can correlate a run ID in the registry.
			if packetPath := strings.TrimSpace(item.ProofRef.Path); packetPath != "" {
				absPath := packetPath
				if !filepath.IsAbs(absPath) {
					absPath = filepath.Join(cwd, absPath)
				}
				if executionPacketPathIsValid(absPath) {
					detail := packetPath
					if item.ProofRef.RunID != "" {
						detail = item.ProofRef.RunID
					}
					return nextWorkProofDecision{Complete: true, Source: "execution_packet", Detail: detail}
				}
			}
			// Fall back to run-registry lookup when no path is set or the
			// file does not yet exist.
			if run := findCompletedRunByID(cwd, item.ProofRef.RunID); run != nil {
				return nextWorkProofDecision{Complete: true, Source: "execution_packet", Detail: run.RunID}
			}
		case "evidence_only_closure":
			if proof := findEvidenceOnlyClosureProofByTarget(cwd, item.ProofRef.TargetID); proof != nil {
				return nextWorkProofDecision{
					Complete: true,
					Source:   "evidence_only_closure",
					Detail:   fmt.Sprintf("%s (%s)", proof.TargetID, proof.PacketPath),
				}
			}
		}
	}

	if run := findCompletedRunForQueueSelection(cwd, &queueSelection{
		Item:       item,
		SourceEpic: sourceEpic,
	}); run != nil {
		return nextWorkProofDecision{Complete: true, Source: "completed_run", Detail: run.RunID}
	}
	if proof := findEvidenceOnlyClosureProofForQueueSelection(cwd, &queueSelection{
		Item:       item,
		SourceEpic: sourceEpic,
	}); proof != nil {
		return nextWorkProofDecision{
			Complete: true,
			Source:   "evidence_only_closure",
			Detail:   fmt.Sprintf("%s (%s)", proof.TargetID, proof.PacketPath),
		}
	}

	return nextWorkProofDecision{}
}

func findCompletedRunByID(cwd, runID string) *rpiRunInfo {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil
	}

	_, historical := discoverRPIRunsRegistryFirst(cwd)
	for i := range historical {
		run := &historical[i]
		if run.Status != "completed" {
			continue
		}
		if strings.TrimSpace(run.RunID) == runID {
			return run
		}
	}
	return nil
}

func findCompletedRunForQueueSelection(cwd string, sel *queueSelection) *rpiRunInfo {
	if sel == nil {
		return nil
	}

	goal := strings.TrimSpace(sel.Item.Title)
	sourceEpic := strings.TrimSpace(sel.SourceEpic)
	if goal == "" || sourceEpic == "" {
		return nil
	}

	_, historical := discoverRPIRunsRegistryFirst(cwd)
	var best *rpiRunInfo
	for i := range historical {
		run := &historical[i]
		if run.Status != "completed" {
			continue
		}
		if strings.TrimSpace(run.Goal) != goal {
			continue
		}
		runEpic := strings.TrimSpace(run.EpicID)
		runID := strings.TrimSpace(run.RunID)
		if sourceEpic != runEpic && sourceEpic != runID {
			continue
		}
		if best == nil || run.StartedAt > best.StartedAt {
			best = run
		}
	}
	return best
}

func findEvidenceOnlyClosureProofForQueueSelection(cwd string, sel *queueSelection) *evidenceOnlyClosureProof {
	if sel == nil {
		return nil
	}

	for _, targetID := range queueProofTargetIDs(sel) {
		if proof := findEvidenceOnlyClosureProofByTarget(cwd, targetID); proof != nil {
			return proof
		}
	}
	return nil
}

func findEvidenceOnlyClosureProofByTarget(cwd, targetID string) *evidenceOnlyClosureProof {
	if packetPath, ok := findValidEvidenceOnlyClosurePacket(cwd, targetID); ok {
		return &evidenceOnlyClosureProof{
			TargetID:   targetID,
			PacketPath: packetPath,
		}
	}
	return nil
}

// Thin wrappers delegating to internal/rpi pure functions.
func queueProofTargetIDs(sel *queueSelection) []string { return rpi.QueueProofTargetIDs(sel) }

func findValidEvidenceOnlyClosurePacket(cwd, targetID string) (string, bool) {
	if strings.TrimSpace(targetID) == "" {
		return "", false
	}

	safeTargetID := strings.ReplaceAll(strings.TrimSpace(targetID), "/", "_")
	roots := collectSearchRoots(cwd)
	for _, root := range roots {
		for _, relPath := range []string{
			filepath.Join(".agents", "releases", "evidence-only-closures", safeTargetID+".json"),
			filepath.Join(".agents", "council", "evidence-only-closures", safeTargetID+".json"),
		} {
			packetPath := filepath.Join(root, relPath)
			if packetIsValidForTarget(packetPath, targetID) {
				return packetPath, true
			}
		}
	}
	return "", false
}

// executionPacketPathIsValid returns true when the given path resolves to a
// readable, non-empty JSON object that carries an "objective" or "run_id"
// field; the minimum proof that a real execution packet was written there.
// It intentionally avoids full schema validation so it stays tolerant of
// minor version drift.
func executionPacketPathIsValid(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var packet struct {
		Objective string `json:"objective"`
		RunID     string `json:"run_id"`
	}
	if err := json.Unmarshal(data, &packet); err != nil {
		return false
	}
	return strings.TrimSpace(packet.Objective) != "" || strings.TrimSpace(packet.RunID) != ""
}

func packetIsValidForTarget(packetPath, targetID string) bool {
	data, err := os.ReadFile(packetPath)
	if err != nil {
		return false
	}

	var packet evidenceOnlyClosurePacket
	if err := json.Unmarshal(data, &packet); err != nil {
		return false
	}
	if strings.TrimSpace(packet.TargetID) != strings.TrimSpace(targetID) {
		return false
	}
	switch packet.EvidenceMode {
	case "commit", "staged", "worktree":
	default:
		return false
	}
	return len(packet.Evidence.Artifacts) > 0
}

// runCycleWithRetries executes a single cycle with retry logic and handles
// success/failure queue marking.
func runCycleWithRetries(ctx context.Context, cwd, goal string, cycle, executedCycles int, nextWorkPath string, sel *queueSelection, explicitGoal string, cfg rpiLoopSupervisorConfig) (loopCycleResult, error) {
	if err := claimQueueSelection(nextWorkPath, sel, cycle); err != nil {
		if errors.Is(err, errQueueClaimConflict) && explicitGoal == "" {
			fmt.Printf("Queue contention for %q; another consumer won the claim. Continuing.\n", goal)
			return loopContinue, nil
		}
		return loopReturn, err
	}

	start := time.Now()
	cycleErr := executeCycleAttempts(ctx, cwd, goal, cycle, executedCycles, cfg)
	elapsed := time.Since(start).Round(time.Second)

	// Kill switch fired mid-retry: clean exit without queue mutation.
	if cycleErr == errKillSwitchActivated {
		releaseQueueSelection(nextWorkPath, sel, false)
		return loopBreak, nil
	}

	if cycleErr != nil {
		return handleCycleFailure(cycleErr, cycle, elapsed, nextWorkPath, sel, explicitGoal, cfg)
	}

	fmt.Printf("Cycle %d completed in %s\n", cycle, elapsed)
	markQueueEntryConsumed(nextWorkPath, sel)

	if explicitGoal != "" {
		fmt.Println("Explicit goal completed.")
		return loopBreak, nil
	}
	return loopContinue, nil
}

// executeCycleAttempts runs the phased engine with retry attempts, checking
// the kill switch before each attempt. Returns errKillSwitchActivated when
// the kill switch fires mid-retry (clean exit, no queue mutation).
func executeCycleAttempts(ctx context.Context, cwd, goal string, cycle, executedCycles int, cfg rpiLoopSupervisorConfig) error {
	maxAttempts := cfg.MaxCycleAttempts()
	var cycleErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		killSwitchSet, killErr := isLoopKillSwitchSet(cfg)
		if killErr != nil {
			return killErr
		}
		if killSwitchSet {
			fmt.Printf("Kill switch detected (%s). Stopping loop before cycle execution.\n", cfg.KillSwitchPath)
			return errKillSwitchActivated
		}
		cycleErr = runRPISupervisedCycleFn(ctx, cwd, goal, cycle, attempt, cfg)
		if cycleErr == nil {
			return nil
		}
		if attempt >= maxAttempts {
			return cycleErr
		}
		fmt.Printf("Cycle %d attempt %d/%d failed: %v\n", cycle, attempt, maxAttempts, cycleErr)
		if cfg.RetryBackoff > 0 {
			fmt.Printf("Retrying in %s...\n", cfg.RetryBackoff.Round(time.Second))
			if err := cancelableSleep(ctx, cfg.RetryBackoff); err != nil {
				return err // signal cancellation, not kill switch
			}
		}
	}
	return cycleErr
}

// handleCycleFailure processes a cycle failure by marking the queue entry and
// deciding whether to continue or stop the loop.
func handleCycleFailure(cycleErr error, cycle int, elapsed time.Duration, nextWorkPath string, sel *queueSelection, explicitGoal string, cfg rpiLoopSupervisorConfig) (loopCycleResult, error) {
	fmt.Printf("Cycle %d failed after %s: %v\n", cycle, elapsed, cycleErr)
	markQueueEntryFailed(nextWorkPath, sel, cycleErr)

	if cfg.ShouldContinueAfterFailure() && explicitGoal == "" {
		fmt.Printf("Failure policy %q: continuing to next queue item.\n", cfg.FailurePolicy)
		return loopContinue, nil
	}

	fmt.Println("Stopping loop due to failure policy.")
	return loopReturn, cycleErr
}

func claimQueueSelection(nextWorkPath string, sel *queueSelection, cycle int) error {
	if sel == nil {
		return nil
	}
	claimedBy := fmt.Sprintf("ao-rpi-loop:cycle-%d", cycle)
	if err := markItemClaimed(nextWorkPath, sel.EntryIndex, sel.ItemIndex, claimedBy); err != nil {
		return fmt.Errorf("claim queue item %q: %w", sel.Item.Title, err)
	}
	sel.ClaimedBy = claimedBy
	fmt.Printf("Queue item claimed: %q\n", sel.Item.Title)
	return nil
}

func releaseQueueSelection(nextWorkPath string, sel *queueSelection, failed bool) {
	if sel == nil {
		return
	}
	var markErr error
	if failed {
		markErr = markItemFailedOwned(nextWorkPath, sel.EntryIndex, sel.ItemIndex, sel.ClaimedBy)
	} else {
		markErr = releaseItemClaimOwned(nextWorkPath, sel.EntryIndex, sel.ItemIndex, sel.ClaimedBy)
	}
	if markErr != nil {
		VerbosePrintf("Warning: could not release queue item claim: %v\n", markErr)
		return
	}
	if failed {
		fmt.Printf("Queue item released for retry after task failure: %q\n", sel.Item.Title)
		return
	}
	fmt.Printf("Queue item released after interruption/transient failure: %q\n", sel.Item.Title)
}

// markQueueEntryFailed marks the queue entry as failed when appropriate.
func markQueueEntryFailed(nextWorkPath string, sel *queueSelection, cycleErr error) {
	if sel == nil {
		return
	}
	if shouldMarkQueueEntryFailed(cycleErr) {
		releaseQueueSelection(nextWorkPath, sel, true)
	} else {
		releaseQueueSelection(nextWorkPath, sel, false)
	}
}

// markQueueEntryConsumed marks the specific queue item as consumed after success.
// When all items in the entry are consumed, the entry itself is marked consumed.
func markQueueEntryConsumed(nextWorkPath string, sel *queueSelection) {
	if sel == nil {
		return
	}
	if markErr := markItemConsumedOwned(nextWorkPath, sel.EntryIndex, sel.ItemIndex, "ao-rpi-loop", sel.ClaimedBy); markErr != nil {
		VerbosePrintf("Warning: could not mark queue item as consumed: %v\n", markErr)
	} else {
		fmt.Printf("Queue item consumed: %q\n", sel.Item.Title)
	}
}

// readUnconsumedItems reads next-work.jsonl and returns all unconsumed items
// across all entries, flattened. When repoFilter is non-empty, items with a
// non-empty TargetRepo that is neither "*" nor equal to repoFilter are skipped.
// Items without a TargetRepo (legacy) or with TargetRepo=="*" always pass.
func readUnconsumedItems(path string, repoFilter string) ([]nextWorkItem, error) {
	entries, err := readQueueEntries(path)
	if err != nil {
		return nil, err
	}

	var items []nextWorkItem
	for _, entry := range entries {
		for _, item := range entry.Items {
			if !isQueueItemSelectable(item) {
				continue
			}
			if repoFilter != "" && item.TargetRepo != "" && item.TargetRepo != "*" && item.TargetRepo != repoFilter {
				continue
			}
			items = append(items, item)
		}
	}
	return items, nil
}

// readQueueEntries reads next-work.jsonl and returns entries with at least one
// selectable queue item (with their 0-based index preserved for later marking).
// Malformed
// lines are skipped with a verbose warning. Missing files return nil, nil.
func readQueueEntries(path string) ([]nextWorkEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open next-work.jsonl: %w", err)
	}
	defer func() { _ = f.Close() }()

	var entries []nextWorkEntry
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	parseableIndex := -1

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		entry, err := parseNextWorkEntryLine(line)
		if err != nil {
			VerbosePrintf("Skipping malformed line: %v\n", err)
			continue
		}
		parseableIndex++
		entry.QueueIndex = parseableIndex

		if len(entry.Items) == 0 {
			continue
		}
		if entryHasExplicitItemLifecycle(entry) {
			recomputeEntryLifecycle(&entry)
		}
		// Skip entries that are already consumed. Legacy failed_at remains retry
		// metadata; proof-backed preflight decides whether stale work is satisfied.
		if entry.Consumed || normalizeClaimStatus(entry.Consumed, entry.ClaimStatus) == "consumed" {
			continue
		}
		// Skip entries where all items are either consumed or currently claimed.
		hasSelectableItem := false
		for _, item := range entry.Items {
			if isQueueItemSelectable(item) {
				hasSelectableItem = true
				break
			}
		}
		if !hasSelectableItem {
			continue
		}

		entries = append(entries, entry)
	}

	return entries, scanner.Err()
}

// Thin wrappers delegating pure logic to internal/rpi.

func parseNextWorkEntryLine(line string) (nextWorkEntry, error) {
	return rpi.ParseNextWorkEntryLine(line)
}

func hasLegacyFlatNextWorkItem(entry nextWorkEntry) bool {
	return rpi.HasLegacyFlatNextWorkItem(entry)
}

func normalizeClaimStatus(consumed bool, claimStatus string) string {
	return rpi.NormalizeClaimStatus(consumed, claimStatus)
}

func isQueueItemSelectable(item nextWorkItem) bool {
	return rpi.IsQueueItemSelectable(item)
}

func hasQueueItemLifecycleMetadata(item nextWorkItem) bool {
	return rpi.HasQueueItemLifecycleMetadata(item)
}

func entryHasExplicitItemLifecycle(entry nextWorkEntry) bool {
	for _, item := range entry.Items {
		if item.Consumed || hasQueueItemLifecycleMetadata(item) {
			return true
		}
	}
	return false
}

func shouldSkipLegacyFailedEntry(entry nextWorkEntry) bool {
	return rpi.ShouldSkipLegacyFailedEntry(entry)
}

func nextWorkSearchRoot(path string) string {
	return rpi.NextWorkSearchRoot(path)
}

// selectHighestSeverityEntry delegates to internal/rpi.SelectHighestSeverityEntry.
func selectHighestSeverityEntry(entries []nextWorkEntry, repoFilter string) *queueSelection {
	return rpi.SelectHighestSeverityEntry(entries, repoFilter)
}

func freshnessRank(item nextWorkItem) int { return rpi.FreshnessRank(item) }
func repoAffinityRank(item nextWorkItem, repoFilter string) int {
	return rpi.RepoAffinityRank(item, repoFilter)
}
func workTypeRank(item nextWorkItem) int { return rpi.WorkTypeRank(item) }
func selectHighestSeverityItem(items []nextWorkItem) string {
	return rpi.SelectHighestSeverityItem(items)
}
func severityRank(s string) int                        { return rpi.SeverityRank(s) }
func isFullyConsumed(entry *nextWorkEntry) bool        { return rpi.IsFullyConsumed(entry) }
func entryConsumedTime(entry *nextWorkEntry) time.Time { return rpi.EntryConsumedTime(entry) }
func recomputeEntryLifecycle(entry *nextWorkEntry)     { rpi.RecomputeEntryLifecycle(entry) }

func ensureQueueItemClaimable(status string, currentClaimedBy *string, claimedBy string) error {
	return rpi.EnsureQueueItemClaimable(status, currentClaimedBy, claimedBy)
}

func requireQueueClaimOwner(currentClaimedBy *string, expectedClaimedBy string) error {
	return rpi.RequireQueueClaimOwner(currentClaimedBy, expectedClaimedBy)
}

// rewriteNextWorkFile rewrites the JSONL file with updated entries applied via
// the transform function. The full read-modify-write runs under an exclusive
// flock so concurrent queue consumers cannot interleave updates. Entries that
// could not be parsed are preserved verbatim. If the file does not exist,
// rewriteNextWorkFile is a no-op.
func rewriteNextWorkFile(path string, transform func(idx int, entry *nextWorkEntry) error) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open next-work.jsonl: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()
	if err := flockLock(f); err != nil {
		return fmt.Errorf("lock next-work.jsonl: %w", err)
	}
	defer func() {
		_ = flockUnlock(f)
	}()
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek next-work.jsonl: %w", err)
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("read next-work.jsonl: %w", err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var lines []string
	parseableIndex := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			lines = append(lines, line)
			continue
		}

		var entry nextWorkEntry
		if jsonErr := json.Unmarshal([]byte(line), &entry); jsonErr != nil {
			// Preserve malformed lines verbatim.
			lines = append(lines, line)
			continue
		}

		if err := transform(parseableIndex, &entry); err != nil {
			return err
		}
		rewritten, marshalErr := json.Marshal(entry)
		if marshalErr != nil {
			lines = append(lines, line)
		} else {
			lines = append(lines, string(rewritten))
		}
		parseableIndex++
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan next-work.jsonl: %w", err)
	}

	var out bytes.Buffer
	for _, l := range lines {
		out.WriteString(l)
		out.WriteByte('\n')
	}

	if err := f.Truncate(0); err != nil {
		return fmt.Errorf("truncate next-work.jsonl: %w", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek next-work.jsonl for write: %w", err)
	}
	if _, err := f.Write(out.Bytes()); err != nil {
		return fmt.Errorf("write next-work.jsonl: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync next-work.jsonl: %w", err)
	}
	return nil
}

// markEntryConsumed sets Consumed=true and ConsumedAt on the entry at entryIndex.
// entryIndex is the 0-based index of the entry among parseable JSON entries in
// the file (blank/malformed lines do not receive an index).
//
// Returns an error when the file does not exist so callers can distinguish a
// missing-queue situation from a successful no-op.
func markEntryConsumed(path string, entryIndex int, consumedBy string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("next-work.jsonl not found: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	return rewriteNextWorkFile(path, func(idx int, entry *nextWorkEntry) error {
		if idx != entryIndex {
			return nil
		}
		entry.Consumed = true
		entry.ClaimStatus = "consumed"
		entry.ClaimedAt = nil
		entry.ClaimedBy = nil
		entry.ConsumedAt = &now
		entry.ConsumedBy = &consumedBy
		entry.FailedAt = nil
		entry.CompletionEvidence = "bead_closed"
		entry.CompletionEvidenceAt = &now
		return nil
	})
}

// markItemConsumed sets Consumed=true on a specific item within an entry.
// When all items in the entry are consumed, the entry itself is marked consumed.
func markItemConsumed(path string, entryIndex int, itemIndex int, consumedBy string) error {
	return markItemConsumedOwned(path, entryIndex, itemIndex, consumedBy, "")
}

func markItemConsumedOwned(path string, entryIndex int, itemIndex int, consumedBy string, expectedClaimedBy string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("next-work.jsonl not found: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	targetFound := false
	err := rewriteNextWorkFile(path, func(idx int, entry *nextWorkEntry) error {
		if idx != entryIndex {
			return nil
		}
		targetFound = true
		if len(entry.Items) == 0 && hasLegacyFlatNextWorkItem(*entry) {
			if err := requireQueueClaimOwner(entry.ClaimedBy, expectedClaimedBy); err != nil {
				return err
			}
			entry.Consumed = true
			entry.ClaimStatus = "consumed"
			entry.ClaimedAt = nil
			entry.ClaimedBy = nil
			entry.ConsumedAt = &now
			entry.ConsumedBy = &consumedBy
			entry.FailedAt = nil
			return nil
		}
		if itemIndex < 0 || itemIndex >= len(entry.Items) {
			return errQueueClaimConflict
		}
		if err := requireQueueClaimOwner(entry.Items[itemIndex].ClaimedBy, expectedClaimedBy); err != nil {
			return err
		}
		entry.Items[itemIndex].Consumed = true
		entry.Items[itemIndex].ClaimStatus = "consumed"
		entry.Items[itemIndex].ClaimedBy = nil
		entry.Items[itemIndex].ClaimedAt = nil
		entry.Items[itemIndex].ConsumedBy = &consumedBy
		entry.Items[itemIndex].ConsumedAt = &now
		entry.Items[itemIndex].FailedAt = nil
		recomputeEntryLifecycle(entry)
		return nil
	})
	if err != nil {
		return err
	}
	if !targetFound {
		return errQueueClaimConflict
	}
	return nil
}

// markEntryFailed records a FailedAt timestamp on the entry at entryIndex without
// setting Consumed. This leaves the entry recoverable: set consumed=false to retry.
func markEntryFailed(path string, entryIndex int) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil // best-effort: missing file is a no-op
		}
		return fmt.Errorf("stat next-work.jsonl: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	targetFound := false
	err := rewriteNextWorkFile(path, func(idx int, entry *nextWorkEntry) error {
		if idx != entryIndex {
			return nil
		}
		targetFound = true
		entry.FailedAt = &now
		entry.ClaimStatus = "available"
		entry.ClaimedBy = nil
		entry.ClaimedAt = nil
		entry.Consumed = false
		return nil
	})
	if err != nil {
		return err
	}
	if !targetFound {
		return errQueueClaimConflict
	}
	return nil
}

func markItemClaimed(path string, entryIndex int, itemIndex int, claimedBy string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("next-work.jsonl not found: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	targetFound := false
	err := rewriteNextWorkFile(path, func(idx int, entry *nextWorkEntry) error {
		if idx != entryIndex {
			return nil
		}
		targetFound = true
		if len(entry.Items) == 0 && hasLegacyFlatNextWorkItem(*entry) {
			if err := ensureQueueItemClaimable(normalizeClaimStatus(entry.Consumed, entry.ClaimStatus), entry.ClaimedBy, claimedBy); err != nil {
				return err
			}
			entry.ClaimStatus = "in_progress"
			entry.ClaimedBy = &claimedBy
			entry.ClaimedAt = &now
			entry.Consumed = false
			return nil
		}
		if itemIndex < 0 || itemIndex >= len(entry.Items) {
			return errQueueClaimConflict
		}
		if err := ensureQueueItemClaimable(normalizeClaimStatus(entry.Items[itemIndex].Consumed, entry.Items[itemIndex].ClaimStatus), entry.Items[itemIndex].ClaimedBy, claimedBy); err != nil {
			return err
		}
		entry.Items[itemIndex].ClaimStatus = "in_progress"
		entry.Items[itemIndex].ClaimedBy = &claimedBy
		entry.Items[itemIndex].ClaimedAt = &now
		entry.Items[itemIndex].Consumed = false
		recomputeEntryLifecycle(entry)
		return nil
	})
	if err != nil {
		return err
	}
	if !targetFound {
		return errQueueClaimConflict
	}
	return nil
}

func releaseItemClaim(path string, entryIndex int, itemIndex int) error {
	return releaseItemClaimOwned(path, entryIndex, itemIndex, "")
}

func markItemFailed(path string, entryIndex int, itemIndex int) error {
	return markItemFailedOwned(path, entryIndex, itemIndex, "")
}

func releaseItemClaimOwned(path string, entryIndex int, itemIndex int, expectedClaimedBy string) error {
	return releaseQueueItem(path, entryIndex, itemIndex, nil, expectedClaimedBy)
}

func markItemFailedOwned(path string, entryIndex int, itemIndex int, expectedClaimedBy string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return releaseQueueItem(path, entryIndex, itemIndex, &now, expectedClaimedBy)
}

func releaseQueueItem(path string, entryIndex int, itemIndex int, failedAt *string, expectedClaimedBy string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("next-work.jsonl not found: %w", err)
	}
	targetFound := false
	err := rewriteNextWorkFile(path, func(idx int, entry *nextWorkEntry) error {
		if idx != entryIndex {
			return nil
		}
		targetFound = true
		if len(entry.Items) == 0 && hasLegacyFlatNextWorkItem(*entry) {
			if err := requireQueueClaimOwner(entry.ClaimedBy, expectedClaimedBy); err != nil {
				return err
			}
			entry.ClaimStatus = "available"
			entry.ClaimedBy = nil
			entry.ClaimedAt = nil
			entry.Consumed = false
			if failedAt != nil {
				entry.FailedAt = failedAt
			}
			return nil
		}
		if itemIndex < 0 || itemIndex >= len(entry.Items) {
			return errQueueClaimConflict
		}
		if err := requireQueueClaimOwner(entry.Items[itemIndex].ClaimedBy, expectedClaimedBy); err != nil {
			return err
		}
		entry.Items[itemIndex].ClaimStatus = "available"
		entry.Items[itemIndex].ClaimedBy = nil
		entry.Items[itemIndex].ClaimedAt = nil
		entry.Items[itemIndex].Consumed = false
		if failedAt != nil {
			entry.Items[itemIndex].FailedAt = failedAt
		}
		recomputeEntryLifecycle(entry)
		return nil
	})
	if err != nil {
		return err
	}
	if !targetFound {
		return errQueueClaimConflict
	}
	return nil
}

// compactNextWorkFile removes entries from the JSONL queue where ALL items are
// consumed and the consumed_at timestamp is older than maxConsumedAge. Returns
// the number of compacted entries. If no entries qualify, the file is not
// rewritten.
func compactNextWorkFile(path string, maxConsumedAge time.Duration) (int, error) {
	// Single-pass compaction under flock to avoid TOCTOU races.
	rf, err := os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("open next-work.jsonl for compaction: %w", err)
	}
	defer func() { _ = rf.Close() }()
	if err := flockLock(rf); err != nil {
		return 0, fmt.Errorf("lock for compaction: %w", err)
	}
	defer func() { _ = flockUnlock(rf) }()

	if _, err := rf.Seek(0, io.SeekStart); err != nil {
		return 0, fmt.Errorf("seek for compaction read: %w", err)
	}
	data, err := io.ReadAll(rf)
	if err != nil {
		return 0, fmt.Errorf("read for compaction: %w", err)
	}

	cutoff := time.Now().Add(-maxConsumedAge)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var out bytes.Buffer
	removed := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			out.WriteString(line)
			out.WriteByte('\n')
			continue
		}
		var entry nextWorkEntry
		if json.Unmarshal([]byte(line), &entry) != nil {
			// Preserve malformed lines.
			out.WriteString(line)
			out.WriteByte('\n')
			continue
		}
		if isFullyConsumed(&entry) {
			ts := entryConsumedTime(&entry)
			if !ts.IsZero() && ts.Before(cutoff) {
				removed++
				continue
			}
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("scan for compaction rewrite: %w", err)
	}

	if err := rf.Truncate(0); err != nil {
		return 0, fmt.Errorf("truncate for compaction: %w", err)
	}
	if _, err := rf.Seek(0, io.SeekStart); err != nil {
		return 0, fmt.Errorf("seek for compaction: %w", err)
	}
	if _, err := rf.Write(out.Bytes()); err != nil {
		return 0, fmt.Errorf("write for compaction: %w", err)
	}
	if err := rf.Sync(); err != nil {
		return 0, fmt.Errorf("sync for compaction: %w", err)
	}
	if removed == 0 {
		return 0, nil
	}
	return removed, nil
}

// maybeCompactQueue runs queue compaction every `interval` cycles.
func maybeCompactQueue(path string, cycle int, interval int, maxAge time.Duration) {
	if interval <= 0 || cycle%interval != 0 {
		return
	}
	n, err := compactNextWorkFile(path, maxAge)
	if err != nil {
		VerbosePrintf("Warning: queue compaction failed: %v\n", err)
		return
	}
	if n > 0 {
		fmt.Printf("Queue compacted: removed %d consumed entries\n", n)
	}
}
