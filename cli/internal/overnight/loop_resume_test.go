package overnight

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// TestRunLoop_CrashAtIter2_ResumeRehydrates exercises the Micro-epic 2
// persistence + rehydration path end-to-end:
//
//  1. Seed a corpus fixture.
//  2. Run RunLoop with fault injection to panic after iter 2 persists.
//  3. Confirm iter-1.json and iter-2.json exist on disk under
//     <outputDir>/<runID>/iterations/.
//  4. Run RunLoop again with the same RunID and confirm the returned
//     result.Iterations contains all 5 iterations in ascending index
//     order with RunID-prefixed IDs.
//
// The fixture is a small non-empty state-machine corpus. The 150-learning
// L2 fixture remains covered by TestRunLoop_L2_FullIteration_CorpusQualityMoves.
// HOME is isolated so harvest.Promote does not touch the real hub.
func TestRunLoop_CrashAtIter2_ResumeRehydrates(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	restore := stubInjectRefresh(t)
	defer restore()

	dir := t.TempDir()
	generateStateMachineFixture(t, dir)

	outputDir := filepath.Join(dir, ".agents", "overnight", "crash-test")
	runID := "test-run-crash-1"
	iterDir := filepath.Join(outputDir, runID, "iterations")

	opts := RunLoopOptions{
		Cwd:            dir,
		OutputDir:      outputDir,
		RunID:          runID,
		RunTimeout:     30 * time.Second,
		MaxIterations:  5,
		PlateauEpsilon: 0.01,
		PlateauWindowK: 2,
		WarnOnly:       true,
		LogWriter:      io.Discard,
	}

	// Phase 1: run to iter 2, then "crash" via fault injection.
	SetFaultInjectionAfterIter(2)
	t.Cleanup(func() { SetFaultInjectionAfterIter(0) })

	func() {
		defer func() { _ = recover() }() // swallow the injected panic
		_, _ = RunLoop(context.Background(), opts)
	}()

	// After the crash, iter-1.json and iter-2.json must exist on disk.
	assertFileExists(t, filepath.Join(iterDir, "iter-1.json"))
	assertFileExists(t, filepath.Join(iterDir, "iter-2.json"))

	// Sanity: each persisted file has the correct RunID prefix in its ID.
	for i := 1; i <= 2; i++ {
		raw, err := os.ReadFile(filepath.Join(iterDir, fmt.Sprintf("iter-%d.json", i)))
		if err != nil {
			t.Fatalf("read iter-%d.json: %v", i, err)
		}
		var it IterationSummary
		if err := json.Unmarshal(raw, &it); err != nil {
			t.Fatalf("unmarshal iter-%d: %v", i, err)
		}
		wantID := fmt.Sprintf("%s-iter-%d", runID, i)
		if string(it.ID) != wantID {
			t.Errorf("persisted iter-%d.ID = %q, want %q", i, it.ID, wantID)
		}
	}

	// Phase 2: clear fault, resume. Expect 5 iterations total.
	SetFaultInjectionAfterIter(0)
	result, err := RunLoop(context.Background(), opts)
	if err != nil {
		t.Fatalf("resume RunLoop: %v", err)
	}
	if result == nil {
		t.Fatal("resume RunLoop returned nil result")
	}
	if len(result.Iterations) != 5 {
		t.Fatalf("expected 5 iterations after resume, got %d", len(result.Iterations))
	}
	for i, it := range result.Iterations {
		if it.Index != i+1 {
			t.Errorf("iter[%d].Index = %d, want %d", i, it.Index, i+1)
		}
		wantPrefix := runID + "-iter-"
		if !strings.HasPrefix(string(it.ID), wantPrefix) {
			t.Errorf("iter[%d].ID = %q, want prefix %q", i, it.ID, wantPrefix)
		}
	}
}

// TestRunLoop_IterationID_UsesRunIDPrefix is the regression guard for
// the IterationID contract-drift fix (types.go line 9 documents
// "<run-id>-iter-<N>" format but loop.go was generating "iter-N"
// without the run-id prefix).
func TestRunLoop_IterationID_UsesRunIDPrefix(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	restore := stubInjectRefresh(t)
	defer restore()

	dir := t.TempDir()
	generateStateMachineFixture(t, dir)

	opts := RunLoopOptions{
		Cwd:            dir,
		OutputDir:      filepath.Join(dir, ".agents", "overnight", "id-test"),
		RunID:          "contract-id-run",
		RunTimeout:     30 * time.Second,
		MaxIterations:  2,
		PlateauEpsilon: 0.01,
		PlateauWindowK: 2,
		WarnOnly:       true,
		LogWriter:      io.Discard,
	}
	result, err := RunLoop(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}
	if result == nil || len(result.Iterations) == 0 {
		t.Fatal("expected at least 1 iteration")
	}
	for _, it := range result.Iterations {
		want := fmt.Sprintf("%s-iter-%d", opts.RunID, it.Index)
		if string(it.ID) != want {
			t.Errorf("iter.ID = %q, want %q", it.ID, want)
		}
	}
}

// TestRunLoop_RequiresRunID asserts the normalize/validation path
// returns an error when RunID is empty.
func TestRunLoop_RequiresRunID(t *testing.T) {
	dir := t.TempDir()
	opts := RunLoopOptions{
		Cwd:       dir,
		OutputDir: filepath.Join(dir, "out"),
		// RunID deliberately omitted.
		RunTimeout:    5 * time.Second,
		MaxIterations: 1,
		LogWriter:     io.Discard,
	}
	_, err := RunLoop(context.Background(), opts)
	if err == nil {
		t.Fatal("expected error when RunID is empty, got nil")
	}
	if !strings.Contains(err.Error(), "RunID") {
		t.Errorf("error should mention RunID, got %v", err)
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file %s to exist: %v", path, err)
	}
}

// TestRunLoop_PostCommitHalt_RehydratesAsBaseline closes the
// Micro-epic 2 known-limitation (pm-20260410-m2-005). Before
// Micro-epic 3, an iter persisted with the legacy "rolled-back"
// string — whether it was a pre-commit rollback OR a post-commit
// regression halt — was skipped during rehydration, which meant a
// resumed run computed prevSnapshot from a stale earlier iteration.
//
// The fix: rehydration uses IsCorpusCompounded(), which returns true
// for StatusHaltedOnRegressionPostCommit. This test plants a prior
// history on disk (iter-1 StatusDone, iter-2 StatusHaltedOnRegression
// PostCommit), then runs RunLoop with MaxIterations=3 and asserts
// the new iter-3 used iter-2's FitnessAfter as its FitnessBefore —
// i.e., the post-commit halt was treated as a valid baseline.
func TestRunLoop_PostCommitHalt_RehydratesAsBaseline(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	restore := stubInjectRefresh(t)
	defer restore()

	dir := t.TempDir()
	generateStateMachineFixture(t, dir)

	outputDir := filepath.Join(dir, ".agents", "overnight", "postcommit-halt")
	runID := "test-run-postcommit"
	iterDir := filepath.Join(outputDir, runID, "iterations")
	if err := os.MkdirAll(iterDir, 0o755); err != nil {
		t.Fatalf("mkdir iter dir: %v", err)
	}

	// Plant iter-1: StatusDone with a known FitnessAfter. This would
	// be the rehydration baseline under the old broken predicate.
	iter1 := IterationSummary{
		ID:         IterationID(fmt.Sprintf("%s-iter-1", runID)),
		Index:      1,
		StartedAt:  time.Unix(1000, 0).UTC(),
		FinishedAt: time.Unix(1001, 0).UTC(),
		Duration:   "1s",
		Status:     StatusDone,
		FitnessAfter: map[string]any{
			"citation_coverage": 0.50,
		},
		FitnessDelta: 0.10,
	}
	if err := writeIterationAtomic(iterDir, iter1); err != nil {
		t.Fatalf("write iter-1: %v", err)
	}

	// Plant iter-2: StatusHaltedOnRegressionPostCommit with a LATER
	// FitnessAfter. Under Micro-epic 3, this iteration IS the valid
	// rehydration baseline because its corpus compounded on disk
	// before the halt fired.
	iter2 := IterationSummary{
		ID:         IterationID(fmt.Sprintf("%s-iter-2", runID)),
		Index:      2,
		StartedAt:  time.Unix(2000, 0).UTC(),
		FinishedAt: time.Unix(2001, 0).UTC(),
		Duration:   "1s",
		Status:     StatusHaltedOnRegressionPostCommit,
		FitnessAfter: map[string]any{
			"citation_coverage": 0.70,
		},
		FitnessDelta: 0.05,
	}
	if err := writeIterationAtomic(iterDir, iter2); err != nil {
		t.Fatalf("write iter-2: %v", err)
	}
	// Companion marker for iter-2 (matches the production write path).
	if err := writeCommittedButFlaggedMarker(iterDir, 2); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	opts := RunLoopOptions{
		Cwd:             dir,
		OutputDir:       outputDir,
		RunID:           runID,
		RunTimeout:      30 * time.Second,
		MaxIterations:   3, // 2 from disk + 1 new one
		PlateauEpsilon:  0.01,
		PlateauWindowK:  2,
		WarnOnly:        true, // do not halt on regressions in the new iter
		RegressionFloor: 0.99, // effectively disable floor for the test
		LogWriter:       io.Discard,
	}

	result, err := RunLoop(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}
	if result == nil {
		t.Fatal("RunLoop returned nil result")
	}
	if len(result.Iterations) != 3 {
		t.Fatalf("expected 3 iterations (2 rehydrated + 1 new), got %d",
			len(result.Iterations))
	}

	// Load persisted iter-2 from disk and confirm its status survived
	// the round-trip through LoadIterations.
	priorIters, _, err := LoadIterations(iterDir, runID)
	if err != nil {
		t.Fatalf("LoadIterations: %v", err)
	}
	var persistedIter2 *IterationSummary
	for i := range priorIters {
		if priorIters[i].Index == 2 {
			persistedIter2 = &priorIters[i]
			break
		}
	}
	if persistedIter2 == nil {
		t.Fatal("iter-2 not found in persisted iterations")
	}
	if persistedIter2.Status != StatusHaltedOnRegressionPostCommit {
		t.Errorf("iter-2.Status = %q, want %q",
			persistedIter2.Status, StatusHaltedOnRegressionPostCommit)
	}

	// The marker should be present for iter-2 (planted above) and
	// ListCommittedButFlaggedMarkers must surface it.
	markers, err := ListCommittedButFlaggedMarkers(iterDir)
	if err != nil {
		t.Fatalf("ListCommittedButFlaggedMarkers: %v", err)
	}
	if len(markers) != 1 || markers[0] != 2 {
		t.Errorf("markers = %v, want [2]", markers)
	}

	// CRITICAL ASSERTION: the new iter-3 must have FitnessBefore
	// matching iter-2's FitnessAfter (0.70), NOT iter-1's (0.50).
	// Under the old broken predicate, rehydration walked past iter-2
	// and used iter-1 as the baseline. Under Micro-epic 3's
	// IsCorpusCompounded predicate, iter-2 is the baseline.
	newIter := result.Iterations[2]
	if newIter.Index != 3 {
		t.Fatalf("new iter index = %d, want 3", newIter.Index)
	}
	if newIter.FitnessBefore == nil {
		t.Fatalf("new iter-3.FitnessBefore is nil — rehydration produced no baseline")
	}
	got, ok := newIter.FitnessBefore["citation_coverage"].(float64)
	if !ok {
		t.Fatalf("new iter-3.FitnessBefore[citation_coverage] missing or wrong type: %v",
			newIter.FitnessBefore["citation_coverage"])
	}
	const want = 0.70
	if got != want {
		t.Errorf("new iter-3.FitnessBefore[citation_coverage] = %v, want %v "+
			"(rehydration did not use iter-2 as baseline)", got, want)
	}
}

// TestRunLoop_LiveTreeHashInvariant asserts the Micro-epic 3 core
// invariant: an iteration whose Status.IsCorpusCompounded() is true
// MUST have mutated the live tree's .agents/ subtree, and an
// iteration whose IsCorpusCompounded() is false MUST NOT have
// mutated it.
//
// Historical scope: this test covers the StatusDone happy-path shape that
// originally exposed the invariant. TestRunLoop_LiveTreeHashInvariant_AllStatuses
// covers every deterministic terminal status. The predicate logic is
// exhaustively unit-tested by TestIterationStatus_IsCorpusCompounded in
// types_test.go.
func TestRunLoop_LiveTreeHashInvariant(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	restore := stubInjectRefresh(t)
	defer restore()

	dir := t.TempDir()
	generateStateMachineFixture(t, dir)

	agentsDir := filepath.Join(dir, ".agents")
	hashBefore, err := agentsHash(agentsDir)
	if err != nil {
		t.Fatalf("hashBefore: %v", err)
	}

	opts := RunLoopOptions{
		Cwd:            dir,
		OutputDir:      filepath.Join(dir, ".agents", "overnight", "hash-invariant"),
		RunID:          "test-run-hash-invariant",
		RunTimeout:     30 * time.Second,
		MaxIterations:  1,
		PlateauEpsilon: 0.01,
		PlateauWindowK: 2,
		WarnOnly:       true,
		LogWriter:      io.Discard,
	}
	result, err := RunLoop(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}
	if len(result.Iterations) != 1 {
		t.Fatalf("expected 1 iteration, got %d", len(result.Iterations))
	}

	iter := result.Iterations[0]
	if !iter.Status.IsCorpusCompounded() {
		t.Fatalf("iter-1.Status = %q (IsCorpusCompounded=false); "+
			"expected a compounded happy-path iteration", iter.Status)
	}

	hashAfter, err := agentsHash(agentsDir)
	if err != nil {
		t.Fatalf("hashAfter: %v", err)
	}

	// Invariant: IsCorpusCompounded() == (hashAfter != hashBefore).
	// For StatusDone, we expect the .agents/ tree to have changed.
	if hashBefore == hashAfter {
		t.Errorf("iter-1 is StatusDone but .agents/ hash did not change "+
			"(before=%s after=%s): compounded-flag ↔ tree-mutation invariant broken",
			hashBefore, hashAfter)
	}

	// Non-StatusDone cases are covered in live_tree_hash_invariant_test.go.
}

// agentsHash returns a deterministic SHA-256 over every regular file
// under dir, sorted by path, for the live-tree-hash invariant test.
// Skips the .agents/overnight/ subtree because that directory holds
// runtime artifacts (iter-<N>.json, markers, logs) that RunLoop
// writes regardless of whether corpus compounding succeeded, which
// would mask the predicate under test.
func agentsHash(dir string) (string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		// Skip runtime artifacts written by RunLoop itself — they are
		// not part of the corpus we're measuring.
		if strings.HasPrefix(rel, "overnight"+string(filepath.Separator)) ||
			rel == "overnight" {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	h := sha256.New()
	for _, f := range files {
		rel, _ := filepath.Rel(dir, f)
		h.Write([]byte(rel))
		h.Write([]byte{0})
		raw, err := os.ReadFile(f)
		if err != nil {
			return "", err
		}
		h.Write(raw)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// TestRunLoop_RehydrationSkipsPreCommitRollback is the mirror of the
// post-commit halt test: a planted iter-2 with status
// StatusRolledBackPreCommit must be SKIPPED as a rehydration
// baseline, and iter-1 (the earlier StatusDone) must win instead.
// This locks the "does not overcorrect" side of the Micro-epic 3
// predicate change.
func TestRunLoop_RehydrationSkipsPreCommitRollback(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	restore := stubInjectRefresh(t)
	defer restore()

	dir := t.TempDir()
	generateStateMachineFixture(t, dir)

	outputDir := filepath.Join(dir, ".agents", "overnight", "precommit-skip")
	runID := "test-run-precommit-skip"
	iterDir := filepath.Join(outputDir, runID, "iterations")
	if err := os.MkdirAll(iterDir, 0o755); err != nil {
		t.Fatalf("mkdir iter dir: %v", err)
	}

	// iter-1: StatusDone baseline at citation_coverage=0.50
	iter1 := IterationSummary{
		ID:           IterationID(fmt.Sprintf("%s-iter-1", runID)),
		Index:        1,
		StartedAt:    time.Unix(1000, 0).UTC(),
		FinishedAt:   time.Unix(1001, 0).UTC(),
		Duration:     "1s",
		Status:       StatusDone,
		FitnessAfter: map[string]any{"citation_coverage": 0.50},
		FitnessDelta: 0.10,
	}
	if err := writeIterationAtomic(iterDir, iter1); err != nil {
		t.Fatalf("write iter-1: %v", err)
	}

	// iter-2: StatusRolledBackPreCommit — corpus never compounded.
	// Even though its FitnessAfter has a higher value, rehydration
	// must skip it.
	iter2 := IterationSummary{
		ID:           IterationID(fmt.Sprintf("%s-iter-2", runID)),
		Index:        2,
		StartedAt:    time.Unix(2000, 0).UTC(),
		FinishedAt:   time.Unix(2001, 0).UTC(),
		Duration:     "1s",
		Status:       StatusRolledBackPreCommit,
		FitnessAfter: map[string]any{"citation_coverage": 0.80}, // trap value
		FitnessDelta: 0.0,
	}
	if err := writeIterationAtomic(iterDir, iter2); err != nil {
		t.Fatalf("write iter-2: %v", err)
	}

	opts := RunLoopOptions{
		Cwd:             dir,
		OutputDir:       outputDir,
		RunID:           runID,
		RunTimeout:      30 * time.Second,
		MaxIterations:   3,
		PlateauEpsilon:  0.01,
		PlateauWindowK:  2,
		WarnOnly:        true,
		RegressionFloor: 0.99,
		LogWriter:       io.Discard,
	}

	result, err := RunLoop(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}
	if len(result.Iterations) != 3 {
		t.Fatalf("expected 3 iterations, got %d", len(result.Iterations))
	}

	// The new iter-3's FitnessBefore must match iter-1's 0.50, not
	// iter-2's trap value of 0.80 — iter-2's pre-commit rollback is
	// not a valid baseline.
	newIter := result.Iterations[2]
	if newIter.FitnessBefore == nil {
		t.Fatal("new iter-3.FitnessBefore nil")
	}
	got, ok := newIter.FitnessBefore["citation_coverage"].(float64)
	if !ok {
		t.Fatalf("citation_coverage missing/wrong type")
	}
	const want = 0.50
	if got != want {
		t.Errorf("new iter-3.FitnessBefore[citation_coverage] = %v, want %v "+
			"(rehydration incorrectly used pre-commit rollback as baseline)", got, want)
	}
}
