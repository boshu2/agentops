package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var phaseHeartbeatInterval = time.Minute

// isPhaseTimeoutError returns true when err was produced by a phase wall-clock timeout.
// It matches the error strings produced by both directExecutor and streamExecutor.
func isPhaseTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "timed out after")
}

// phaseSummaryExists reports whether the phase-N-summary.md artifact was written inside
// spawnCwd, indicating the Claude session finished its PHASE SUMMARY CONTRACT obligation
// before being killed by the timeout watchdog.
func phaseSummaryExists(spawnCwd string, phaseNum int) bool {
	path := filepath.Join(spawnCwd, ".agents", "rpi", fmt.Sprintf("phase-%d-summary.md", phaseNum))
	_, err := os.Stat(path)
	return err == nil
}

// rescuePhaseOnTimeout recovers from a phase timeout when the phase summary artifact
// already exists. This handles the race where the implementation session completes all
// work but is killed by the orchestrator's wall-clock before it can exit cleanly.
// Returns true when the rescue succeeds; caller should continue to post-phase processing.
func rescuePhaseOnTimeout(spawnCwd string, p phase, timeoutErr error) bool {
	if !isPhaseTimeoutError(timeoutErr) {
		return false
	}
	if !phaseSummaryExists(spawnCwd, p.Num) {
		return false
	}
	fmt.Printf("Phase %d (%s): timed out, but the phase summary was written — session completed its work before the watchdog fired.\n", p.Num, p.Name)
	fmt.Printf("Continuing to post-phase validation. Use --phase-timeout=<duration> to raise the limit.\n")
	return true
}

func startPhaseHeartbeat(ctx context.Context, spawnCwd, runID string) func() {
	if runID == "" || phaseHeartbeatInterval <= 0 {
		return func() {}
	}
	hbCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(phaseHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-hbCtx.Done():
				return
			case <-ticker.C:
				updateRunHeartbeat(spawnCwd, runID)
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

// handlePostPhaseGate runs post-phase gate checking and retry logic.
// It is extracted from runSinglePhase to reduce its cyclomatic complexity.
func handlePostPhaseGate(ctx context.Context, spawnCwd string, state *phasedState, p phase, logPath, statusPath string, allPhases []PhaseProgress, executor PhaseExecutor) error {
	if err := postPhaseProcessing(spawnCwd, state, p.Num, logPath); err != nil {
		var retryErr *gateFailError
		if errors.As(err, &retryErr) {
			retried, retryErr2 := handleGateRetry(ctx, spawnCwd, state, p.Num, retryErr, logPath, spawnCwd, statusPath, allPhases, executor)
			if retryErr2 != nil {
				return retryErr2
			}
			if !retried {
				return fmt.Errorf("phase %d (%s): gate failed after max retries", p.Num, p.Name)
			}
			return nil
		}
		return err
	}
	return nil
}

// runPhaseLoop executes phases sequentially and applies standard fatal logging on failures.
// Fast-complexity runs still execute validation; their prompts use quick gates.
func runPhaseLoop(ctx context.Context, cwd, spawnCwd string, state *phasedState, startPhase int, opts phasedEngineOptions, statusPath string, allPhases []PhaseProgress, logPath string, executor PhaseExecutor) error {
	for i := startPhase; i <= len(phases); i++ {
		p := phases[i-1]
		if err := runSinglePhase(ctx, cwd, spawnCwd, state, startPhase, p, opts, statusPath, allPhases, logPath, executor); err != nil {
			return logAndFailPhase(state, p.Name, logPath, spawnCwd, err)
		}
	}
	return nil
}

// maybeLiveStatus updates the live-status file if live status tracking is enabled.
func maybeLiveStatus(opts phasedEngineOptions, statusPath string, allPhases []PhaseProgress, phaseNum int, status string, attempts int, detail string) {
	if opts.LiveStatus {
		updateLivePhaseStatus(statusPath, allPhases, phaseNum, status, attempts, detail)
	}
}

// handleDryRunPhase prints what would happen and returns true if dry-run mode is active.
func handleDryRunPhase(cwd string, state *phasedState, startPhase int, p phase, opts phasedEngineOptions, prompt, logPath string) bool {
	if !GetDryRun() {
		return false
	}
	fmt.Printf("[dry-run] Would spawn: %s\n", formatRuntimePromptInvocation(effectiveRuntimeCommand(state.Opts.RuntimeCommand), prompt))
	if !opts.NoWorktree && p.Num == startPhase {
		runID := generateRunID()
		fmt.Printf("[dry-run] Would create worktree: ../%s-rpi-%s/ (detached)\n",
			filepath.Base(cwd), runID)
	}
	logPhaseTransition(logPath, state.RunID, p.Name, "dry-run")
	return true
}

// executePhaseSession spawns the phase executor and records the result.
// On success it writes the phaseResult artifact and returns nil.
func executePhaseSession(ctx context.Context, spawnCwd string, state *phasedState, p phase, opts phasedEngineOptions, statusPath string, allPhases []PhaseProgress, logPath, prompt string, executor PhaseExecutor) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	fmt.Printf("Phase %d: spawning %s session...\n", p.Num, effectiveRuntimeCommand(state.Opts.RuntimeCommand))
	start := time.Now()
	updateRunHeartbeat(spawnCwd, state.RunID)
	stopHeartbeat := startPhaseHeartbeat(ctx, spawnCwd, state.RunID)
	defer stopHeartbeat()
	retryKey := fmt.Sprintf("phase_%d", p.Num)

	if err := executor.Execute(ctx, prompt, spawnCwd, state.RunID, p.Num); err != nil {
		maybeLiveStatus(opts, statusPath, allPhases, p.Num, "failed", state.Attempts[retryKey], err.Error())
		logPhaseTransition(logPath, state.RunID, p.Name, fmt.Sprintf("FAILED: %v", err))
		return fmt.Errorf("phase %d (%s) failed: %w", p.Num, p.Name, err)
	}

	elapsed := time.Since(start).Round(time.Second)
	fmt.Printf("Phase %d completed in %s\n", p.Num, elapsed)
	logPhaseTransition(logPath, state.RunID, p.Name, fmt.Sprintf("completed in %s", elapsed))
	maybeLiveStatus(opts, statusPath, allPhases, p.Num, "completed", state.Attempts[retryKey], "")

	pr := &phaseResult{
		SchemaVersion:   1,
		RunID:           state.RunID,
		Phase:           p.Num,
		PhaseName:       p.Name,
		Status:          "completed",
		Retries:         state.Attempts[retryKey],
		Backend:         executor.Name(),
		Verdicts:        state.Verdicts,
		StartedAt:       start.Format(time.RFC3339),
		CompletedAt:     time.Now().Format(time.RFC3339),
		DurationSeconds: elapsed.Seconds(),
	}
	if err := writePhaseResult(spawnCwd, pr); err != nil {
		VerbosePrintf("Warning: could not write phase result: %v\n", err)
	}
	updateRunHeartbeat(spawnCwd, state.RunID)
	return nil
}

func runSinglePhase(ctx context.Context, cwd, spawnCwd string, state *phasedState, startPhase int, p phase, opts phasedEngineOptions, statusPath string, allPhases []PhaseProgress, logPath string, executor PhaseExecutor) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	fmt.Printf("\n--- Phase %d: %s ---\n", p.Num, p.Name)
	state.Phase = p.Num
	if err := savePhasedState(spawnCwd, state); err != nil {
		VerbosePrintf("Warning: could not persist phase start state: %v\n", err)
	}

	prompt, err := buildPromptForPhase(spawnCwd, p.Num, state, nil)
	if err != nil {
		return fmt.Errorf("build prompt for phase %d: %w", p.Num, err)
	}

	logPhaseTransition(logPath, state.RunID, p.Name, "started")
	retryKey := fmt.Sprintf("phase_%d", p.Num)
	maybeLiveStatus(opts, statusPath, allPhases, p.Num, "starting", state.Attempts[retryKey], "")

	if handleDryRunPhase(cwd, state, startPhase, p, opts, prompt, logPath) {
		return nil
	}

	if err := executePhaseSession(ctx, spawnCwd, state, p, opts, statusPath, allPhases, logPath, prompt, executor); err != nil {
		if !rescuePhaseOnTimeout(spawnCwd, p, err) {
			return err
		}
		// Phase timed out but wrote its summary — treat as complete and fall through
		// to post-phase gate so the orchestrator can validate and continue.
		logPhaseTransition(logPath, state.RunID, p.Name, "timeout-rescued: summary artifact found, continuing")
	}

	if err := handlePostPhaseGate(ctx, spawnCwd, state, p, logPath, statusPath, allPhases, executor); err != nil {
		return err
	}

	if handoffDetected(spawnCwd, p.Num) {
		fmt.Printf("Phase %d: handoff detected — phase reported context degradation\n", p.Num)
		logPhaseTransition(logPath, state.RunID, p.Name, "HANDOFF detected — context degradation")
	}

	writePhaseSummary(spawnCwd, state, p.Num)

	// Write structured handoff for next phase
	handoff := buildPhaseHandoffFromState(state, p.Num, spawnCwd)
	if err := writePhaseHandoff(spawnCwd, handoff); err != nil {
		VerbosePrintf("Warning: could not write phase handoff: %v\n", err)
	} else {
		// Emit C2 event for dashboard observability
		if _, err := appendRPIC2Event(spawnCwd, rpiC2EventInput{
			RunID:   state.RunID,
			Phase:   p.Num,
			Backend: state.Backend,
			Source:  "orchestrator",
			Type:    "phase.handoff.written",
			Message: fmt.Sprintf("Phase %d handoff: %d artifacts, %d decisions, %d risks",
				p.Num, len(handoff.ArtifactsProduced), len(handoff.DecisionsMade), len(handoff.OpenRisks)),
			Details: map[string]any{"handoff": handoff},
		}); err != nil {
			VerbosePrintf("Warning: could not write C2 event: %v\n", err)
		}
	}

	recordRatchetCheckpoint(p.Step, state.Opts.AOCommand)
	if err := updatePhaseResultArtifacts(spawnCwd, state, p.Num, nil); err != nil {
		VerbosePrintf("Warning: could not refresh phase result artifacts: %v\n", err)
	}
	if err := updateExecutionPacketProof(spawnCwd, state); err != nil {
		VerbosePrintf("Warning: could not refresh execution packet proof: %v\n", err)
	}

	if err := savePhasedState(spawnCwd, state); err != nil {
		VerbosePrintf("Warning: could not save state: %v\n", err)
	}

	return nil
}

func logAndFailPhase(state *phasedState, phaseName, logPath, spawnCwd string, err error) error {
	logPhaseTransition(logPath, state.RunID, phaseName, fmt.Sprintf("FATAL: %v", err))
	logFailureContext(logPath, state.RunID, phaseName, err)
	// Write terminal metadata so `ao rpi status` shows "failed" with reason.
	state.TerminalStatus = "failed"
	state.TerminalReason = fmt.Sprintf("phase %s: %v", phaseName, err)
	state.TerminatedAt = time.Now().Format(time.RFC3339)
	if saveErr := savePhasedState(spawnCwd, state); saveErr != nil {
		VerbosePrintf("Warning: could not persist terminal state: %v\n", saveErr)
	}
	return err
}

func writeFinalPhasedReport(state *phasedState, logPath string) {
	fmt.Printf("\n=== RPI Phased Complete ===\n")
	fmt.Printf("Goal: %s\n", state.Goal)
	if state.EpicID != "" {
		if isPlanFileEpic(state.EpicID) {
			fmt.Printf("Plan file: %s\n", planFileFromEpic(state.EpicID))
		} else {
			fmt.Printf("Epic: %s\n", state.EpicID)
		}
	}
	fmt.Printf("Verdicts: %v\n", state.Verdicts)
	logPhaseTransition(logPath, state.RunID, "complete", fmt.Sprintf("epic=%s verdicts=%v", state.EpicID, state.Verdicts))
}
