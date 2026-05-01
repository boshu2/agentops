package main

import (
	"cmp"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

var rpiIssueIDPattern = regexp.MustCompile(`^[a-z][a-z0-9]*-[a-z0-9][a-z0-9.]*$`)

func isIssueIDLike(goal string) bool {
	return rpiIssueIDPattern.MatchString(strings.TrimSpace(goal))
}

func preflightRuntimeAvailability(runtimeCommand string, lookPathFn gcLookFn) error {
	if GetDryRun() {
		return nil
	}
	command := cmp.Or(strings.TrimSpace(runtimeCommand), "claude")
	executable, _ := splitRuntimeCommand(command)
	if executable == "" {
		return fmt.Errorf("runtime command %q is empty", command)
	}
	if _, err := defaultLookPath(lookPathFn)(executable); err != nil {
		return fmt.Errorf("runtime executable %q (from %q) not found on PATH (required for spawning phase sessions)", executable, command)
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

	if startPhase == 1 {
		routedGoal, ok, err := resolveIssueStartRoute(goal, opts.BDCommand)
		if ok {
			if err != nil {
				tracker := detectTrackerHealth(opts.BDCommand, opts.LookPath)
				if tracker.Healthy {
					VerbosePrintf("Warning: issue-like goal %s was not resolvable via bd show; treating it as a discovery goal: %v\n", goal, err)
					return goal, startPhase, nil
				}
				VerbosePrintf("Warning: tracker degraded while resolving %s; starting at implementation with original issue id: %v\n", goal, err)
				return strings.TrimSpace(goal), 2, nil
			}
			return routedGoal, 2, nil
		}
	}

	return goal, startPhase, nil
}

func resolveIssueStartRoute(goal, bdCommand string) (string, bool, error) {
	goal = strings.TrimSpace(goal)
	if !isIssueIDLike(goal) {
		return "", false, nil
	}

	cmd := exec.Command(effectiveBDCommand(bdCommand), "show", goal, "--json")
	out, err := cmd.Output()
	if err != nil {
		return goal, true, fmt.Errorf("bd show %s --json: %w", goal, err)
	}

	isEpic, err := parseIssueTypeFromShowJSON(out)
	if err != nil {
		return goal, true, err
	}
	if isEpic {
		return goal, true, nil
	}
	if parent := parseParentIssueIDFromShowJSON(out); parent != "" {
		return parent, true, nil
	}
	return goal, true, nil
}

func parseParentIssueIDFromShowJSON(data []byte) string {
	var single map[string]any
	if err := json.Unmarshal(data, &single); err == nil {
		return parentIssueIDFromMap(single)
	}

	var multiple []map[string]any
	if err := json.Unmarshal(data, &multiple); err == nil {
		for _, entry := range multiple {
			if parent := parentIssueIDFromMap(entry); parent != "" {
				return parent
			}
		}
	}
	return ""
}

func parentIssueIDFromMap(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	for _, key := range []string{"parent", "parent_id", "parentId", "parent_epic", "parentEpic", "epic_id", "epicId"} {
		if parent := issueIDFromJSONValue(payload[key]); parent != "" {
			return parent
		}
	}
	for _, key := range []string{"issue", "metadata"} {
		if nested, ok := payload[key].(map[string]any); ok {
			if parent := parentIssueIDFromMap(nested); parent != "" {
				return parent
			}
		}
	}
	return ""
}

func issueIDFromJSONValue(value any) string {
	switch typed := value.(type) {
	case string:
		id := strings.TrimSpace(typed)
		if isIssueIDLike(id) {
			return id
		}
	case map[string]any:
		for _, key := range []string{"id", "issue_id", "issueId"} {
			if id := issueIDFromJSONValue(typed[key]); id != "" {
				return id
			}
		}
	}
	return ""
}

func newPhasedState(opts phasedEngineOptions, startPhase int, goal string) *phasedState {
	tracker := detectTrackerHealth(opts.BDCommand, opts.LookPath)
	s := &phasedState{
		SchemaVersion: 1,
		Goal:          goal,
		TrackerMode:   tracker.Mode,
		TrackerReason: tracker.Reason,
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
	if opts.RunID != "" {
		s.RunID = opts.RunID
	}
	if startPhase >= 2 && isIssueIDLike(goal) {
		s.EpicID = strings.TrimSpace(goal)
	}
	return s
}

// mergeExistingStateFields copies relevant fields from a previous run's state
// into the current state for phase resumption.
func mergeExistingStateFields(state *phasedState, existing *phasedState, opts phasedEngineOptions, goal string) {
	// Only carry forward epic_id when the user did NOT provide an explicit goal.
	// An explicit goal (especially a bead ID like "ag-xyz") signals new intent
	// and must not be overridden by stale state from a previous run.
	if goal == "" {
		state.EpicID = existing.EpicID
		state.Goal = existing.Goal
	} else if isIssueIDLike(goal) {
		// Explicit bead ID as goal overrides carried epic_id.
		state.EpicID = goal
	} else {
		// Explicit free-text goal (or malformed issue-like ID): clear stale
		// epic_id so downstream discovery or crank can resolve the correct one.
		state.EpicID = ""
	}
	if existing.TrackerMode != "" {
		state.TrackerMode = existing.TrackerMode
	}
	if existing.TrackerReason != "" {
		state.TrackerReason = existing.TrackerReason
	}
	state.FastPath = existing.FastPath || opts.FastPath
	state.SwarmFirst = existing.SwarmFirst || opts.SwarmFirst
	if existing.Verdicts != nil {
		state.Verdicts = existing.Verdicts
	}
	if existing.Attempts != nil {
		state.Attempts = existing.Attempts
	}
}

// resolveExistingWorktree checks whether the previous run's worktree still exists
// and returns the worktree path to use as spawnCwd.  Returns ("", nil) when no
// worktree should be used (NoWorktree flag or no prior worktree path).
func resolveExistingWorktree(state *phasedState, existing *phasedState, opts phasedEngineOptions) (string, error) {
	if opts.NoWorktree || existing.WorktreePath == "" {
		return "", nil
	}
	if _, statErr := os.Stat(existing.WorktreePath); statErr != nil {
		return "", fmt.Errorf("worktree %s from previous run no longer exists (was it removed?)", existing.WorktreePath)
	}
	state.WorktreePath = existing.WorktreePath
	state.RunID = existing.RunID
	fmt.Printf("Resuming in existing worktree: %s\n", existing.WorktreePath)
	_, _ = appendRPIC2Event(existing.WorktreePath, rpiC2EventInput{
		RunID:   state.RunID,
		Type:    "worktree.resumed",
		Message: fmt.Sprintf("Resumed existing worktree: %s", existing.WorktreePath),
		Details: map[string]string{"path": existing.WorktreePath},
	})
	return existing.WorktreePath, nil
}

func resumePhasedStateIfNeeded(cwd string, opts phasedEngineOptions, startPhase int, goal string, state *phasedState) (string, error) {
	if startPhase <= 1 {
		return cwd, nil
	}

	existing, err := loadPhasedState(cwd)
	if err != nil {
		return cwd, nil
	}

	mergeExistingStateFields(state, existing, opts, goal)

	wtPath, err := resolveExistingWorktree(state, existing, opts)
	if err != nil {
		return "", err
	}
	if wtPath != "" {
		return wtPath, nil
	}
	return cwd, nil
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
	if state.RunID == "" {
		state.RunID = runID
	}
	fmt.Printf("Worktree created: %s (detached)\n", worktreePath)
	_, _ = appendRPIC2Event(worktreePath, rpiC2EventInput{
		RunID:   state.RunID,
		Type:    "worktree.created",
		Message: fmt.Sprintf("Worktree created: %s", worktreePath),
		Details: map[string]string{"path": worktreePath, "run_id": runID},
	})

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
		_, _ = appendRPIC2Event(originalCwd, rpiC2EventInput{
			RunID:   state.RunID,
			Type:    "worktree.merged",
			Message: fmt.Sprintf("Merged worktree into %s", originalCwd),
		})
		if rmErr := removeWorktree(originalCwd, worktreePath, runID); rmErr != nil {
			fmt.Fprintf(os.Stderr, "Cleanup failed: %v\nWorktree may require manual removal: %s\n", rmErr, worktreePath)
			logFailureContext(logPath, state.RunID, "cleanup", rmErr)
			return fmt.Errorf("worktree cleanup failed: %w", rmErr)
		}
		_, _ = appendRPIC2Event(originalCwd, rpiC2EventInput{
			RunID:   state.RunID,
			Type:    "worktree.removed",
			Message: fmt.Sprintf("Removed worktree: %s", worktreePath),
		})
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
	if err := os.MkdirAll(stateDir, 0750); err != nil {
		return "", "", "", nil, fmt.Errorf("create state directory: %w", err)
	}

	logPath := filepath.Join(stateDir, "phased-orchestration.log")
	statusPath := filepath.Join(stateDir, "live-status.md")
	var allPhases []PhaseProgress

	if startPhase == 1 {
		cleanPhaseSummaries(stateDir)
	}

	fmt.Printf("\n=== RPI Phased: %s ===\n", state.Goal)
	_, _ = appendRPIC2Event(spawnCwd, rpiC2EventInput{
		RunID:   state.RunID,
		Phase:   startPhase,
		Type:    "rpi.started",
		Message: fmt.Sprintf("RPI started: %s (phase %d)", state.Goal, startPhase),
		Details: map[string]any{"goal": state.Goal, "start_phase": startPhase},
	})
	fmt.Printf("Starting from phase %d (%s)\n", startPhase, phases[startPhase-1].Name)
	if opts.NoDashboard || GetDryRun() {
		fmt.Println("Monitor: ao rpi status --watch  or  ao rpi serve " + state.RunID)
	}

	if opts.LiveStatus {
		allPhases = buildAllPhases(phases)
		fmt.Printf("Live phase status file: %s\n", statusPath)
		if err := WriteLiveStatus(statusPath, allPhases, startPhase-1); err != nil {
			VerbosePrintf("Warning: could not initialize live status: %v\n", err)
		}
	}

	return stateDir, logPath, statusPath, allPhases, nil
}
