package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func preflightRuntimeAvailability(runtimeCommand string) error {
	if GetDryRun() {
		return nil
	}
	command := strings.TrimSpace(runtimeCommand)
	if command == "" {
		command = "claude"
	}
	if _, err := exec.LookPath(command); err != nil {
		return fmt.Errorf("runtime command %q not found on PATH (required for spawning phase sessions)", command)
	}
	return nil
}

func resolveGoalAndStartPhase(opts phasedEngineOptions, args []string, cwd string) (string, int, error) {
	goal := ""
	if len(args) > 0 {
		goal = args[0]
	}

	startPhase := phaseNameToNum(opts.From)
	if startPhase == 0 {
		return "", 0, fmt.Errorf("unknown phase: %q (valid: discovery, implementation, validation)", opts.From)
	}

	if startPhase >= 2 && goal == "" {
		existing, err := loadPhasedState(cwd)
		if err == nil && existing.EpicID != "" {
			goal = existing.Goal
		}
	}
	if goal == "" && startPhase <= 1 {
		return "", 0, fmt.Errorf("goal is required (provide as argument)")
	}

	return goal, startPhase, nil
}

func newPhasedState(opts phasedEngineOptions, startPhase int, goal string) *phasedState {
	return &phasedState{
		SchemaVersion: 1,
		Goal:          goal,
		Phase:         startPhase,
		StartPhase:    startPhase,
		Cycle:         1,
		FastPath:      opts.FastPath,
		TestFirst:     opts.TestFirst,
		SwarmFirst:    opts.SwarmFirst,
		Verdicts:      make(map[string]string),
		Attempts:      make(map[string]int),
		StartedAt:     time.Now().Format(time.RFC3339),
		Opts:          opts,
	}
}

func resumePhasedStateIfNeeded(cwd string, opts phasedEngineOptions, startPhase int, goal string, state *phasedState) (string, error) {
	spawnCwd := cwd
	if startPhase <= 1 {
		return spawnCwd, nil
	}

	existing, err := loadPhasedState(cwd)
	if err != nil {
		return spawnCwd, nil
	}

	state.EpicID = existing.EpicID
	state.FastPath = existing.FastPath || opts.FastPath
	state.SwarmFirst = existing.SwarmFirst || opts.SwarmFirst
	if existing.Verdicts != nil {
		state.Verdicts = existing.Verdicts
	}
	if existing.Attempts != nil {
		state.Attempts = existing.Attempts
	}
	if goal == "" {
		state.Goal = existing.Goal
	}

	if opts.NoWorktree || existing.WorktreePath == "" {
		return spawnCwd, nil
	}
	if _, statErr := os.Stat(existing.WorktreePath); statErr != nil {
		return "", fmt.Errorf("worktree %s from previous run no longer exists (was it removed?)", existing.WorktreePath)
	}

	spawnCwd = existing.WorktreePath
	state.WorktreePath = existing.WorktreePath
	state.RunID = existing.RunID
	fmt.Printf("Resuming in existing worktree: %s\n", spawnCwd)
	return spawnCwd, nil
}

func setupWorktreeLifecycle(cwd, originalCwd string, opts phasedEngineOptions, state *phasedState) (string, func(success bool, logPath string) error, error) {
	spawnCwd := cwd
	noopCleanup := func(bool, string) error { return nil }

	if opts.NoWorktree || GetDryRun() || state.WorktreePath != "" {
		return spawnCwd, noopCleanup, nil
	}

	worktreePath, runID, err := createWorktree(cwd)
	if err != nil {
		return "", noopCleanup, fmt.Errorf("create worktree: %w", err)
	}

	spawnCwd = worktreePath
	state.WorktreePath = worktreePath
	state.RunID = runID
	fmt.Printf("Worktree created: %s (detached)\n", worktreePath)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		if sig, ok := <-sigCh; ok {
			fmt.Fprintf(os.Stderr, "\nInterrupted (%v). Worktree preserved at: %s\n", sig, worktreePath)
			// Write terminal metadata so `ao rpi status` shows "interrupted" instead of "running".
			state.TerminalStatus = "interrupted"
			state.TerminalReason = fmt.Sprintf("signal: %v", sig)
			state.TerminatedAt = time.Now().Format(time.RFC3339)
			_ = savePhasedState(spawnCwd, state)
			os.Exit(1)
		}
	}()

	cleanup := func(success bool, logPath string) error {
		signal.Stop(sigCh)
		close(sigCh)

		if !success {
			fmt.Fprintf(os.Stderr, "Worktree preserved for debugging: %s\n", worktreePath)
			return nil
		}

		if mergeErr := mergeWorktree(originalCwd, worktreePath, runID); mergeErr != nil {
			fmt.Fprintf(os.Stderr, "Merge failed: %v\nWorktree preserved at: %s\n", mergeErr, worktreePath)
			return fmt.Errorf("worktree merge failed: %w", mergeErr)
		}
		if rmErr := removeWorktree(originalCwd, worktreePath, runID); rmErr != nil {
			fmt.Fprintf(os.Stderr, "Cleanup failed: %v\nWorktree may require manual removal: %s\n", rmErr, worktreePath)
			logFailureContext(logPath, state.RunID, "cleanup", rmErr)
			return fmt.Errorf("worktree cleanup failed: %w", rmErr)
		}
		return nil
	}

	return spawnCwd, cleanup, nil
}

func ensureStateRunID(state *phasedState) {
	if state.RunID != "" {
		return
	}
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	state.RunID = hex.EncodeToString(b)
}

func initializeRunArtifacts(spawnCwd string, startPhase int, state *phasedState, opts phasedEngineOptions) (string, string, string, []PhaseProgress, error) {
	stateDir := filepath.Join(spawnCwd, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return "", "", "", nil, fmt.Errorf("create state directory: %w", err)
	}

	logPath := filepath.Join(stateDir, "phased-orchestration.log")
	statusPath := filepath.Join(stateDir, "live-status.md")
	var allPhases []PhaseProgress

	if startPhase == 1 {
		cleanPhaseSummaries(stateDir)
	}

	fmt.Printf("\n=== RPI Phased: %s ===\n", state.Goal)
	fmt.Printf("Starting from phase %d (%s)\n", startPhase, phases[startPhase-1].Name)
	fmt.Println("Monitor in a second terminal: ao rpi status --watch")

	if opts.LiveStatus {
		allPhases = buildAllPhases(phases)
		fmt.Printf("Live phase status file: %s\n", statusPath)
		if err := WriteLiveStatus(statusPath, allPhases, startPhase-1); err != nil {
			VerbosePrintf("Warning: could not initialize live status: %v\n", err)
		}
	}

	return stateDir, logPath, statusPath, allPhases, nil
}
