package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- resolveGoalAndStartPhase ---

func TestResolveGoalAndStartPhase_Phase1WithGoal(t *testing.T) {
	goal, startPhase, err := resolveGoalAndStartPhase(phasedEngineOptions{From: "discovery"}, []string{"Fix the bug"}, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if goal != "Fix the bug" {
		t.Errorf("goal = %q, want %q", goal, "Fix the bug")
	}
	if startPhase != 1 {
		t.Errorf("startPhase = %d, want 1", startPhase)
	}
}

func TestResolveGoalAndStartPhase_Phase1WithoutGoal(t *testing.T) {
	_, _, err := resolveGoalAndStartPhase(phasedEngineOptions{From: "discovery"}, nil, t.TempDir())
	if err == nil {
		t.Fatal("expected error when phase 1 has no goal")
	}
}

func TestResolveGoalAndStartPhase_UnknownPhase(t *testing.T) {
	_, _, err := resolveGoalAndStartPhase(phasedEngineOptions{From: "nonexistent"}, nil, t.TempDir())
	if err == nil {
		t.Fatal("expected error for unknown phase name")
	}
}

func TestResolveGoalAndStartPhase_Phase2WithExistingState(t *testing.T) {
	tmp := t.TempDir()
	stateDir := filepath.Join(tmp, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	state := &phasedState{
		SchemaVersion: 1,
		Goal:          "Existing goal",
		Phase:         2,
		StartPhase:    1,
		Cycle:         1,
		EpicID:        "ag-123",
		Verdicts:      map[string]string{},
		Attempts:      map[string]int{},
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(filepath.Join(stateDir, phasedStateFile), data, 0644); err != nil {
		t.Fatal(err)
	}

	goal, startPhase, err := resolveGoalAndStartPhase(phasedEngineOptions{From: "implementation"}, nil, tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if goal != "Existing goal" {
		t.Errorf("goal = %q, want %q", goal, "Existing goal")
	}
	if startPhase != 2 {
		t.Errorf("startPhase = %d, want 2", startPhase)
	}
}

func TestResolveGoalAndStartPhase_Phase2WithArgGoal(t *testing.T) {
	goal, startPhase, err := resolveGoalAndStartPhase(phasedEngineOptions{From: "implementation"}, []string{"New goal"}, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if goal != "New goal" {
		t.Errorf("goal = %q, want %q", goal, "New goal")
	}
	if startPhase != 2 {
		t.Errorf("startPhase = %d, want 2", startPhase)
	}
}

// --- newPhasedState ---

func TestNewPhasedState_Defaults(t *testing.T) {
	opts := phasedEngineOptions{FastPath: true, TestFirst: true, SwarmFirst: true}
	state := newPhasedState(opts, 2, "test goal")

	if state.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", state.SchemaVersion)
	}
	if state.Goal != "test goal" {
		t.Errorf("Goal = %q, want %q", state.Goal, "test goal")
	}
	if state.Phase != 2 {
		t.Errorf("Phase = %d, want 2", state.Phase)
	}
	if state.StartPhase != 2 {
		t.Errorf("StartPhase = %d, want 2", state.StartPhase)
	}
	if state.Cycle != 1 {
		t.Errorf("Cycle = %d, want 1", state.Cycle)
	}
	if !state.FastPath {
		t.Error("FastPath should be true")
	}
	if !state.TestFirst {
		t.Error("TestFirst should be true")
	}
	if !state.SwarmFirst {
		t.Error("SwarmFirst should be true")
	}
	if state.Verdicts == nil {
		t.Error("Verdicts map should not be nil")
	}
	if state.Attempts == nil {
		t.Error("Attempts map should not be nil")
	}
	if state.StartedAt == "" {
		t.Error("StartedAt should be set")
	}
}

func TestNewPhasedState_RunIDPassthrough(t *testing.T) {
	opts := phasedEngineOptions{RunID: "test-run-abc123"}
	state := newPhasedState(opts, 1, "test goal")
	if state.RunID != "test-run-abc123" {
		t.Errorf("RunID = %q, want %q", state.RunID, "test-run-abc123")
	}
}

func TestNewPhasedState_RunIDEmpty(t *testing.T) {
	opts := phasedEngineOptions{}
	state := newPhasedState(opts, 1, "test goal")
	if state.RunID != "" {
		t.Errorf("RunID = %q, want empty (ensureStateRunID should generate later)", state.RunID)
	}
}

// --- mergeExistingStateFields ---

func TestMergeExistingStateFields_CopiesFields(t *testing.T) {
	state := newPhasedState(phasedEngineOptions{}, 2, "ag-42")
	existing := &phasedState{
		EpicID:   "ag-42",
		FastPath: true,
		Verdicts: map[string]string{"pre_mortem": "PASS"},
		Attempts: map[string]int{"phase_1": 2},
	}

	mergeExistingStateFields(state, existing, phasedEngineOptions{}, "ag-42")

	if state.EpicID != "ag-42" {
		t.Errorf("EpicID = %q, want %q", state.EpicID, "ag-42")
	}
	if !state.FastPath {
		t.Error("FastPath should be true from existing state")
	}
	if state.Verdicts["pre_mortem"] != "PASS" {
		t.Errorf("Verdicts[pre_mortem] = %q, want PASS", state.Verdicts["pre_mortem"])
	}
	if state.Attempts["phase_1"] != 2 {
		t.Errorf("Attempts[phase_1] = %d, want 2", state.Attempts["phase_1"])
	}
	if state.Goal != "ag-42" {
		t.Errorf("Goal should remain %q when provided", "ag-42")
	}
}

func TestMergeExistingStateFields_ExplicitBeadOverridesCarried(t *testing.T) {
	// When user passes an explicit bead ID that differs from carried state,
	// the explicit ID must win — prevents cross-epic contamination (ag-8a5d).
	state := newPhasedState(phasedEngineOptions{}, 2, "ag-pv9")
	existing := &phasedState{
		EpicID:   "ag-md3",
		Goal:     "old goal from previous run",
		Verdicts: map[string]string{},
		Attempts: map[string]int{},
	}

	mergeExistingStateFields(state, existing, phasedEngineOptions{}, "ag-pv9")

	if state.EpicID != "ag-pv9" {
		t.Errorf("EpicID = %q, want %q (explicit bead should override carried)", state.EpicID, "ag-pv9")
	}
	if state.Goal != "ag-pv9" {
		t.Errorf("Goal = %q, want %q", state.Goal, "ag-pv9")
	}
}

func TestMergeExistingStateFields_FreeTextGoalClearsEpic(t *testing.T) {
	// Free-text goal should clear stale epic_id so downstream can resolve fresh.
	state := newPhasedState(phasedEngineOptions{}, 2, "fix auth module")
	existing := &phasedState{
		EpicID:   "ag-old",
		Verdicts: map[string]string{},
		Attempts: map[string]int{},
	}

	mergeExistingStateFields(state, existing, phasedEngineOptions{}, "fix auth module")

	if state.EpicID != "" {
		t.Errorf("EpicID = %q, want empty (free-text goal should clear stale epic)", state.EpicID)
	}
}

func TestMergeExistingStateFields_BareAgPrefixClearsEpic(t *testing.T) {
	// Bare "ag-" without an ID suffix should NOT be treated as a bead ID.
	state := newPhasedState(phasedEngineOptions{}, 2, "ag-")
	existing := &phasedState{
		EpicID:   "ag-stale",
		Verdicts: map[string]string{},
		Attempts: map[string]int{},
	}

	mergeExistingStateFields(state, existing, phasedEngineOptions{}, "ag-")

	if state.EpicID != "" {
		t.Errorf("EpicID = %q, want empty (bare ag- prefix is not a valid bead ID)", state.EpicID)
	}
}

func TestMergeExistingStateFields_InheritsGoalWhenEmpty(t *testing.T) {
	state := newPhasedState(phasedEngineOptions{}, 2, "")
	existing := &phasedState{
		Goal:     "inherited goal",
		EpicID:   "ag-99",
		Verdicts: map[string]string{},
		Attempts: map[string]int{},
	}

	mergeExistingStateFields(state, existing, phasedEngineOptions{}, "")

	if state.Goal != "inherited goal" {
		t.Errorf("Goal = %q, want %q (should inherit from existing)", state.Goal, "inherited goal")
	}
}

func TestMergeExistingStateFields_FastPathOr(t *testing.T) {
	// FastPath should be true if either existing or opts has it.
	state := newPhasedState(phasedEngineOptions{}, 2, "test")
	existing := &phasedState{
		FastPath: false,
		Verdicts: map[string]string{},
		Attempts: map[string]int{},
	}

	mergeExistingStateFields(state, existing, phasedEngineOptions{FastPath: true}, "test")
	if !state.FastPath {
		t.Error("FastPath should be true from opts")
	}
}

func TestMergeExistingStateFields_SwarmFirstOr(t *testing.T) {
	state := newPhasedState(phasedEngineOptions{}, 2, "test")
	existing := &phasedState{
		SwarmFirst: true,
		Verdicts:   map[string]string{},
		Attempts:   map[string]int{},
	}

	mergeExistingStateFields(state, existing, phasedEngineOptions{SwarmFirst: false}, "test")
	if !state.SwarmFirst {
		t.Error("SwarmFirst should be true from existing")
	}
}

// --- resolveExistingWorktree ---

func TestResolveExistingWorktree_NoWorktreeFlag(t *testing.T) {
	state := newTestPhasedState()
	existing := &phasedState{WorktreePath: "/some/path"}
	opts := phasedEngineOptions{NoWorktree: true}

	path, err := resolveExistingWorktree(state, existing, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "" {
		t.Errorf("path = %q, want empty when NoWorktree is true", path)
	}
}

func TestResolveExistingWorktree_EmptyWorktreePath(t *testing.T) {
	state := newTestPhasedState()
	existing := &phasedState{WorktreePath: ""}
	opts := phasedEngineOptions{NoWorktree: false}

	path, err := resolveExistingWorktree(state, existing, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "" {
		t.Errorf("path = %q, want empty when existing has no worktree path", path)
	}
}

func TestResolveExistingWorktree_WorktreeDoesNotExist(t *testing.T) {
	state := newTestPhasedState()
	existing := &phasedState{WorktreePath: "/nonexistent/worktree/path"}
	opts := phasedEngineOptions{NoWorktree: false}

	_, err := resolveExistingWorktree(state, existing, opts)
	if err == nil {
		t.Fatal("expected error when worktree path does not exist")
	}
}

func TestResolveExistingWorktree_WorktreeExists(t *testing.T) {
	tmp := t.TempDir()
	state := newTestPhasedState()
	existing := &phasedState{WorktreePath: tmp, RunID: "run-abc"}
	opts := phasedEngineOptions{NoWorktree: false}

	path, err := resolveExistingWorktree(state, existing, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != tmp {
		t.Errorf("path = %q, want %q", path, tmp)
	}
	if state.WorktreePath != tmp {
		t.Errorf("state.WorktreePath = %q, want %q", state.WorktreePath, tmp)
	}
	if state.RunID != "run-abc" {
		t.Errorf("state.RunID = %q, want %q", state.RunID, "run-abc")
	}
}

// --- resumePhasedStateIfNeeded ---

func TestResumePhasedStateIfNeeded_Phase1ReturnsOriginalCwd(t *testing.T) {
	tmp := t.TempDir()
	state := newTestPhasedState()
	got, err := resumePhasedStateIfNeeded(tmp, phasedEngineOptions{}, 1, "test goal", state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != tmp {
		t.Errorf("got %q, want %q", got, tmp)
	}
}

func TestResumePhasedStateIfNeeded_Phase2NoExistingState(t *testing.T) {
	tmp := t.TempDir()
	state := newTestPhasedState()
	got, err := resumePhasedStateIfNeeded(tmp, phasedEngineOptions{}, 2, "test goal", state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// When no existing state found, should return original cwd.
	if got != tmp {
		t.Errorf("got %q, want %q", got, tmp)
	}
}

func TestSetupWorktreeLifecycle_PreservesPreseededRunID(t *testing.T) {
	repo := initTestRepo(t)

	t.Chdir(repo)

	state := newTestPhasedState().WithRunID("seeded-run-id")
	spawnCwd, cleanup, err := setupWorktreeLifecycle(repo, repo, phasedEngineOptions{}, state)
	if err != nil {
		t.Fatalf("setupWorktreeLifecycle: %v", err)
	}

	if state.RunID != "seeded-run-id" {
		t.Fatalf("RunID overwritten: got %q, want %q", state.RunID, "seeded-run-id")
	}
	if state.WorktreePath == "" {
		t.Fatal("expected WorktreePath to be set")
	}
	if spawnCwd != state.WorktreePath {
		t.Fatalf("spawnCwd = %q, want %q", spawnCwd, state.WorktreePath)
	}

	logPath := filepath.Join(repo, ".agents", "rpi", "setup-worktree-lifecycle.log")
	if err := cleanup(true, logPath); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
}

// --- ensureStateRunID ---

func TestEnsureStateRunID_AlreadySet(t *testing.T) {
	state := newTestPhasedState().WithRunID("existing-id")
	ensureStateRunID(state)
	if state.RunID != "existing-id" {
		t.Errorf("RunID = %q, should not change when already set", state.RunID)
	}
}

func TestEnsureStateRunID_GeneratesNew(t *testing.T) {
	state := newTestPhasedState()
	state.RunID = ""
	ensureStateRunID(state)
	if state.RunID == "" {
		t.Error("RunID should be generated when empty")
	}
	if len(state.RunID) != 8 {
		t.Errorf("RunID length = %d, want 8 (4 bytes hex-encoded)", len(state.RunID))
	}
}

func TestEnsureStateRunID_UniqueAcrossCalls(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		state := newTestPhasedState()
		state.RunID = ""
		ensureStateRunID(state)
		if ids[state.RunID] {
			t.Fatalf("duplicate RunID generated: %q", state.RunID)
		}
		ids[state.RunID] = true
	}
}

// --- initializeRunArtifacts ---

func TestInitializeRunArtifacts_CreatesStateDir(t *testing.T) {
	tmp := t.TempDir()
	state := newTestPhasedState().WithGoal("test goal")
	opts := phasedEngineOptions{LiveStatus: false}

	stateDir, logPath, statusPath, allPhases, err := initializeRunArtifacts(tmp, 1, state, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stateDir == "" {
		t.Error("stateDir should not be empty")
	}
	if _, err := os.Stat(stateDir); os.IsNotExist(err) {
		t.Errorf("state directory should exist: %s", stateDir)
	}
	if logPath == "" {
		t.Error("logPath should not be empty")
	}
	if statusPath == "" {
		t.Error("statusPath should not be empty")
	}
	if allPhases != nil {
		t.Error("allPhases should be nil when LiveStatus is disabled")
	}
}

func TestInitializeRunArtifacts_LiveStatusEnabled(t *testing.T) {
	tmp := t.TempDir()
	state := newTestPhasedState().WithGoal("test goal")
	opts := phasedEngineOptions{LiveStatus: true}

	_, _, statusPath, allPhases, err := initializeRunArtifacts(tmp, 1, state, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if allPhases == nil {
		t.Fatal("allPhases should not be nil when LiveStatus is enabled")
	}
	if len(allPhases) != len(phases) {
		t.Errorf("allPhases length = %d, want %d", len(allPhases), len(phases))
	}
	// Verify status file was written.
	if _, err := os.Stat(statusPath); os.IsNotExist(err) {
		t.Errorf("status file should be created at %s", statusPath)
	}
}

func TestInitializeRunArtifacts_CleansSummariesOnPhase1(t *testing.T) {
	tmp := t.TempDir()
	stateDir := filepath.Join(tmp, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Pre-create a stale summary.
	summaryPath := filepath.Join(stateDir, "phase-1-summary.md")
	if err := os.WriteFile(summaryPath, []byte("stale"), 0644); err != nil {
		t.Fatal(err)
	}

	state := newTestPhasedState().WithGoal("test")
	opts := phasedEngineOptions{LiveStatus: false}

	_, _, _, _, err := initializeRunArtifacts(tmp, 1, state, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Stale summary should be cleaned.
	if _, err := os.Stat(summaryPath); !os.IsNotExist(err) {
		t.Error("phase-1-summary.md should have been cleaned on phase 1 start")
	}
}

// --- C2 events ---

func TestWorktreeResumedEvent(t *testing.T) {
	tmp := t.TempDir()
	runID := "run-resumed-001"
	state := newTestPhasedState().WithRunID(runID)
	existing := &phasedState{WorktreePath: tmp, RunID: runID}
	opts := phasedEngineOptions{NoWorktree: false}

	path, err := resolveExistingWorktree(state, existing, opts)
	if err != nil {
		t.Fatalf("resolveExistingWorktree: %v", err)
	}
	if path != tmp {
		t.Fatalf("path = %q, want %q", path, tmp)
	}

	events, err := loadRPIC2Events(tmp, runID)
	if err != nil {
		t.Fatalf("loadRPIC2Events: %v", err)
	}
	found := false
	for _, ev := range events {
		if ev.Type == "worktree.resumed" {
			found = true
			if ev.RunID != runID {
				t.Errorf("event RunID = %q, want %q", ev.RunID, runID)
			}
			break
		}
	}
	if !found {
		t.Error("expected worktree.resumed event in events.jsonl")
	}
}

func TestWorktreeCreatedEvent(t *testing.T) {
	repo := initTestRepo(t)

	t.Chdir(repo)

	state := newTestPhasedState().WithRunID("run-created-001")
	spawnCwd, cleanup, err := setupWorktreeLifecycle(repo, repo, phasedEngineOptions{}, state)
	if err != nil {
		t.Fatalf("setupWorktreeLifecycle: %v", err)
	}
	defer func() {
		_ = cleanup(true, filepath.Join(repo, ".agents", "rpi", "test.log"))
	}()

	// The event is written to the worktree path (spawnCwd).
	events, err := loadRPIC2Events(spawnCwd, state.RunID)
	if err != nil {
		t.Fatalf("loadRPIC2Events: %v", err)
	}
	found := false
	for _, ev := range events {
		if ev.Type == "worktree.created" {
			found = true
			if ev.RunID != state.RunID {
				t.Errorf("event RunID = %q, want %q", ev.RunID, state.RunID)
			}
			break
		}
	}
	if !found {
		t.Error("expected worktree.created event in events.jsonl")
	}
}

func TestRPIStartedEvent(t *testing.T) {
	tmp := t.TempDir()
	runID := "run-started-001"
	state := newTestPhasedState().WithRunID(runID).WithGoal("test rpi started")
	opts := phasedEngineOptions{LiveStatus: false}

	_, _, _, _, err := initializeRunArtifacts(tmp, 1, state, opts)
	if err != nil {
		t.Fatalf("initializeRunArtifacts: %v", err)
	}

	events, err := loadRPIC2Events(tmp, runID)
	if err != nil {
		t.Fatalf("loadRPIC2Events: %v", err)
	}
	found := false
	for _, ev := range events {
		if ev.Type == "rpi.started" {
			found = true
			if ev.RunID != runID {
				t.Errorf("event RunID = %q, want %q", ev.RunID, runID)
			}
			if ev.Phase != 1 {
				t.Errorf("event Phase = %d, want 1", ev.Phase)
			}
			break
		}
	}
	if !found {
		t.Error("expected rpi.started event in events.jsonl")
	}
}

func TestWorktreeMergedAndRemovedEvents(t *testing.T) {
	repo := initTestRepo(t)

	t.Chdir(repo)

	state := newTestPhasedState().WithRunID("run-merge-001")
	_, cleanup, err := setupWorktreeLifecycle(repo, repo, phasedEngineOptions{}, state)
	if err != nil {
		t.Fatalf("setupWorktreeLifecycle: %v", err)
	}

	logPath := filepath.Join(repo, ".agents", "rpi", "test.log")
	if err := cleanup(true, logPath); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	// Merged and removed events are written to originalCwd (repo).
	events, err := loadRPIC2Events(repo, state.RunID)
	if err != nil {
		t.Fatalf("loadRPIC2Events: %v", err)
	}

	foundMerged := false
	foundRemoved := false
	for _, ev := range events {
		switch ev.Type {
		case "worktree.merged":
			foundMerged = true
			if ev.RunID != state.RunID {
				t.Errorf("merged event RunID = %q, want %q", ev.RunID, state.RunID)
			}
		case "worktree.removed":
			foundRemoved = true
			if ev.RunID != state.RunID {
				t.Errorf("removed event RunID = %q, want %q", ev.RunID, state.RunID)
			}
		}
	}
	if !foundMerged {
		t.Error("expected worktree.merged event in events.jsonl")
	}
	if !foundRemoved {
		t.Error("expected worktree.removed event in events.jsonl")
	}
}

// --- preflightRuntimeAvailability ---

func TestPreflightRuntimeAvailability_DryRunSkips(t *testing.T) {
	origDryRun := dryRun
	dryRun = true
	defer func() { dryRun = origDryRun }()

	// Even with a nonexistent command, dry-run should pass.
	err := preflightRuntimeAvailability("nonexistent-binary-xyz", nil)
	if err != nil {
		t.Errorf("dry-run should skip preflight check, got: %v", err)
	}
}

func TestPreflightRuntimeAvailability_WhitespaceDefaultsToClaude(t *testing.T) {
	origDryRun := dryRun
	dryRun = false
	defer func() { dryRun = origDryRun }()

	// When input is whitespace, cmp.Or defaults to "claude".
	// If claude is on PATH, no error; if not, error mentions "claude".
	err := preflightRuntimeAvailability("  ", nil)
	if err != nil {
		// Verify the error references the default "claude" command.
		if !strings.Contains(err.Error(), "claude") {
			t.Errorf("error = %q, should mention 'claude' as the default", err.Error())
		}
	}
	// If no error, claude was found on PATH, which is fine.
}

func TestPreflightRuntimeAvailability_NonexistentBinary(t *testing.T) {
	origDryRun := dryRun
	dryRun = false
	defer func() { dryRun = origDryRun }()

	err := preflightRuntimeAvailability("nonexistent-binary-xyz-99999", nil)
	if err == nil {
		t.Error("expected error for nonexistent binary")
	}
}

// --- resumePhasedStateIfNeeded coverage ---

func TestResumePhasedStateIfNeeded_NoState(t *testing.T) {
	// When no state file exists and startPhase > 1, the function should
	// return the original cwd without error (loadPhasedState fails silently).
	tmp := t.TempDir()
	state := newTestPhasedState()

	got, err := resumePhasedStateIfNeeded(tmp, phasedEngineOptions{NoWorktree: true}, 2, "resume goal", state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != tmp {
		t.Errorf("returned path = %q, want original cwd %q", got, tmp)
	}
	// State fields should remain at their defaults since no existing state was loaded.
	if state.EpicID != "" {
		t.Errorf("EpicID = %q, want empty (no existing state to merge)", state.EpicID)
	}
	if state.Goal != "test goal" {
		t.Errorf("Goal = %q, want %q (should keep original when no existing state)", state.Goal, "test goal")
	}
}

func TestResumePhasedStateIfNeeded_WithState(t *testing.T) {
	// Write a valid phased-state.json, then resume from phase 2.
	// Verify that existing state fields are merged into the current state.
	tmp := t.TempDir()
	stateDir := filepath.Join(tmp, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	existing := &phasedState{
		SchemaVersion: 1,
		Goal:          "original goal",
		Phase:         1,
		StartPhase:    1,
		Cycle:         1,
		EpicID:        "ag-resume-42",
		FastPath:      true,
		Verdicts:      map[string]string{"pre_mortem": "PASS"},
		Attempts:      map[string]int{"phase_1": 1},
	}
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, phasedStateFile), data, 0644); err != nil {
		t.Fatal(err)
	}

	state := newTestPhasedState()
	got, err := resumePhasedStateIfNeeded(tmp, phasedEngineOptions{NoWorktree: true}, 2, "", state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != tmp {
		t.Errorf("returned path = %q, want %q", got, tmp)
	}
	// Verify merged fields from existing state.
	if state.EpicID != "ag-resume-42" {
		t.Errorf("EpicID = %q, want %q", state.EpicID, "ag-resume-42")
	}
	if !state.FastPath {
		t.Error("FastPath should be true (merged from existing state)")
	}
	if state.Verdicts["pre_mortem"] != "PASS" {
		t.Errorf("Verdicts[pre_mortem] = %q, want %q", state.Verdicts["pre_mortem"], "PASS")
	}
	if state.Attempts["phase_1"] != 1 {
		t.Errorf("Attempts[phase_1] = %d, want 1", state.Attempts["phase_1"])
	}
	// Goal should be inherited from existing since we passed empty goal.
	if state.Goal != "original goal" {
		t.Errorf("Goal = %q, want %q (should inherit from existing when empty)", state.Goal, "original goal")
	}
}

func TestResumePhasedStateIfNeeded_Corrupt(t *testing.T) {
	// Write corrupt JSON to the state file. loadPhasedState should fail,
	// and resumePhasedStateIfNeeded should return cwd without error
	// (it treats load failure as "no existing state").
	tmp := t.TempDir()
	stateDir := filepath.Join(tmp, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, phasedStateFile), []byte("{corrupt json!!!"), 0644); err != nil {
		t.Fatal(err)
	}

	state := newTestPhasedState()
	got, err := resumePhasedStateIfNeeded(tmp, phasedEngineOptions{NoWorktree: true}, 2, "test goal", state)
	if err != nil {
		t.Fatalf("corrupt state should not cause error, got: %v", err)
	}
	if got != tmp {
		t.Errorf("returned path = %q, want original cwd %q", got, tmp)
	}
	// State should remain unmodified since load failed.
	if state.EpicID != "" {
		t.Errorf("EpicID = %q, want empty (corrupt state should not merge)", state.EpicID)
	}
}
