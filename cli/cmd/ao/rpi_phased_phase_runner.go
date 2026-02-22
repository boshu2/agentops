package main

import (
	"fmt"
	"path/filepath"
	"time"
)

// runPhaseLoop executes phases sequentially and applies standard fatal logging on failures.
func runPhaseLoop(cwd, spawnCwd string, state *phasedState, startPhase int, opts phasedEngineOptions, statusPath string, allPhases []PhaseProgress, logPath string, executor PhaseExecutor) error {
	for i := startPhase; i <= len(phases); i++ {
		p := phases[i-1]
		if err := runSinglePhase(cwd, spawnCwd, state, startPhase, p, opts, statusPath, allPhases, logPath, executor); err != nil {
			return logAndFailPhase(state, p.Name, logPath, spawnCwd, err)
		}
	}
	return nil
}

func runSinglePhase(cwd, spawnCwd string, state *phasedState, startPhase int, p phase, opts phasedEngineOptions, statusPath string, allPhases []PhaseProgress, logPath string, executor PhaseExecutor) error {
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
	if opts.LiveStatus {
		retryKey := fmt.Sprintf("phase_%d", p.Num)
		updateLivePhaseStatus(statusPath, allPhases, p.Num, "starting", state.Attempts[retryKey], "")
	}

	if GetDryRun() {
		fmt.Printf("[dry-run] Would spawn: %s -p '%s'\n", effectiveRuntimeCommand(state.Opts.RuntimeCommand), prompt)
		if !opts.NoWorktree && p.Num == startPhase {
			runID := generateRunID()
			fmt.Printf("[dry-run] Would create worktree: ../%s-rpi-%s/ (branch: rpi/%s)\n",
				filepath.Base(cwd), runID, runID)
		}
		logPhaseTransition(logPath, state.RunID, p.Name, "dry-run")
		return nil
	}

	// Spawn phase session.
	fmt.Printf("Spawning: %s -p '%s'\n", effectiveRuntimeCommand(state.Opts.RuntimeCommand), prompt)
	start := time.Now()
	updateRunHeartbeat(spawnCwd, state.RunID)

	if err := executor.Execute(prompt, spawnCwd, state.RunID, p.Num); err != nil {
		if opts.LiveStatus {
			retryKey := fmt.Sprintf("phase_%d", p.Num)
			updateLivePhaseStatus(statusPath, allPhases, p.Num, "failed", state.Attempts[retryKey], err.Error())
		}
		logPhaseTransition(logPath, state.RunID, p.Name, fmt.Sprintf("FAILED: %v", err))
		return fmt.Errorf("phase %d (%s) failed: %w", p.Num, p.Name, err)
	}

	elapsed := time.Since(start).Round(time.Second)
	fmt.Printf("Phase %d completed in %s\n", p.Num, elapsed)
	logPhaseTransition(logPath, state.RunID, p.Name, fmt.Sprintf("completed in %s", elapsed))
	if opts.LiveStatus {
		retryKey := fmt.Sprintf("phase_%d", p.Num)
		updateLivePhaseStatus(statusPath, allPhases, p.Num, "completed", state.Attempts[retryKey], "")
	}

	pr := &phaseResult{
		SchemaVersion:   1,
		RunID:           state.RunID,
		Phase:           p.Num,
		PhaseName:       p.Name,
		Status:          "completed",
		Retries:         state.Attempts[fmt.Sprintf("phase_%d", p.Num)],
		Verdicts:        state.Verdicts,
		StartedAt:       start.Format(time.RFC3339),
		CompletedAt:     time.Now().Format(time.RFC3339),
		DurationSeconds: elapsed.Seconds(),
	}
	if err := writePhaseResult(spawnCwd, pr); err != nil {
		VerbosePrintf("Warning: could not write phase result: %v\n", err)
	}
	updateRunHeartbeat(spawnCwd, state.RunID)

	if err := postPhaseProcessing(spawnCwd, state, p.Num, logPath); err != nil {
		if retryErr, ok := err.(*gateFailError); ok {
			retried, retryErr2 := handleGateRetry(spawnCwd, state, p.Num, retryErr, logPath, spawnCwd, statusPath, allPhases, executor)
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

	if handoffDetected(spawnCwd, p.Num) {
		fmt.Printf("Phase %d: handoff detected — phase reported context degradation\n", p.Num)
		logPhaseTransition(logPath, state.RunID, p.Name, "HANDOFF detected — context degradation")
	}

	writePhaseSummary(spawnCwd, state, p.Num)
	recordRatchetCheckpoint(p.Step, state.Opts.AOCommand)

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
		fmt.Printf("Epic: %s\n", state.EpicID)
	}
	fmt.Printf("Verdicts: %v\n", state.Verdicts)
	logPhaseTransition(logPath, state.RunID, "complete", fmt.Sprintf("epic=%s verdicts=%v", state.EpicID, state.Verdicts))
}
