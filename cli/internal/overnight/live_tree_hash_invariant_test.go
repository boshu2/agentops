package overnight

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/overnight/fixture"
)

// ==========================================================================
// na-1iv - TestRunLoop_LiveTreeHashInvariant_AllStatuses
//
// Micro-epic 3 landed TestRunLoop_LiveTreeHashInvariant in loop_resume_test.go
// with only the StatusDone happy-path case implemented. na-1iv asks for
// coverage of the other terminal IterationStatus values. This file extends
// that coverage via a table-driven L2 test that forces RunLoop into each status
// deterministically (using SetTestFitnessInjector for fitness-driven paths)
// and asserts the core invariant:
//
//   For every iteration emitted by RunLoop:
//     iter.Status.IsCorpusCompounded() == (hashAfterThatIter != hashBeforeThatIter)
//
// where hash is a deterministic SHA-256 over the .agents/ subtree excluding
// RunLoop's own runtime artifacts (.agents/overnight/**). See
// loop_resume_test.go `agentsHash` for the helper we mirror here as a local
// copy so this file is self-contained and does not depend on an existing test
// file's helper declaration order.
//
// Semantic notes (from the M8 Option A consolidation council):
//   StatusDone                         -> corpus compounded (live tree mutated)
//   StatusDegraded                     -> MEASURE failed pre-commit; rollback
//                                        ran, live tree unchanged.
//   StatusHaltedOnRegressionPostCommit -> commit succeeded, then late metadata
//                                        integrity caught post-commit drift.
//   StatusRolledBackPreCommit          -> REDUCE stage failure; rollback ran.
//   StatusHaltedOnRegressionPreCommit  -> strict-mode fitness regression before
//                                        commit; rollback ran.
//   StatusFailed                       -> unrecoverable INGEST/CHECKPOINT/COMMIT
//                                        error path.
//
// This file locks all statuses RunLoop can emit today. Rows use narrow,
// test-only hooks where needed so production options stay free of artificial
// fault-injection knobs.

// liveTreeHashInvariantCase describes a single row of the table test.
type liveTreeHashInvariantCase struct {
	// name is the Go subtest name.
	name string
	// wantStatus is the terminal IterationStatus the case expects on the
	// last iteration. If zero, any status is accepted (not used here).
	wantStatus IterationStatus
	// setup installs any test-hook state (fitness injector, fault
	// injection) and returns a cleanup closure. The cleanup MUST restore
	// all global test hooks to their zero value.
	setup func(t *testing.T) func()
	// buildOpts returns the RunLoopOptions for the case. cwd and outputDir
	// are pre-filled by the driver.
	buildOpts func(cwd, outputDir, runID string) RunLoopOptions
	// seedPriorDone writes a prior done iteration so the next RunLoop call
	// can produce a regression status as its first new iteration.
	seedPriorDone bool
	// wantErr is true for RunLoop paths that intentionally return an error
	// after appending the terminal iteration to result.Iterations.
	wantErr bool
}

// TestRunLoop_LiveTreeHashInvariant_AllStatuses drives the live-tree hash
// invariant across every IterationStatus value that can be deterministically
// produced from tests. The StatusDone case is also retested here so all
// exercisable statuses live in one scaffold; the original test in
// loop_resume_test.go is retained for its historical scope comment and as
// regression coverage of the single-case shape.
func TestRunLoop_LiveTreeHashInvariant_AllStatuses(t *testing.T) {
	cases := []liveTreeHashInvariantCase{
		{
			name:       "StatusDone_HappyPath",
			wantStatus: StatusDone,
			setup: func(t *testing.T) func() {
				// Constant high fitness, single iter -> no regression,
				// no plateau window possible. iter 1 commits and the
				// live tree mutates.
				SetTestFitnessInjector(injectConstantFitness(0.8))
				return func() { SetTestFitnessInjector(nil) }
			},
			buildOpts: func(cwd, outputDir, runID string) RunLoopOptions {
				return RunLoopOptions{
					Cwd:            cwd,
					OutputDir:      outputDir,
					RunID:          runID,
					RunTimeout:     30 * time.Second,
					MaxIterations:  1,
					PlateauEpsilon: 0.01,
					PlateauWindowK: 2,
					WarnOnly:       false,
					LogWriter:      io.Discard,
				}
			},
		},
		{
			name:          "StatusHaltedOnRegressionPreCommit_StrictRegression",
			wantStatus:    StatusHaltedOnRegressionPreCommit,
			seedPriorDone: true,
			setup: func(t *testing.T) func() {
				// A prior persisted StatusDone iteration supplies the 0.9
				// baseline; the first new live iteration reports 0.1 and
				// trips the strict pre-commit regression halt.
				SetTestFitnessInjector(injectConstantFitness(0.1))
				return func() { SetTestFitnessInjector(nil) }
			},
			buildOpts: func(cwd, outputDir, runID string) RunLoopOptions {
				return RunLoopOptions{
					Cwd:             cwd,
					OutputDir:       outputDir,
					RunID:           runID,
					RunTimeout:      30 * time.Second,
					MaxIterations:   2,
					PlateauEpsilon:  0.01,
					PlateauWindowK:  2,
					RegressionFloor: 0.05,
					WarnOnly:        false,
					LogWriter:       io.Discard,
				}
			},
		},
		{
			name:       "StatusDegraded_MeasureFailurePreCommit",
			wantStatus: StatusDegraded,
			setup: func(t *testing.T) func() {
				SetTestFitnessInjector(func(int) (FitnessSnapshot, error) {
					return FitnessSnapshot{}, errors.New("synthetic measure failure")
				})
				return func() { SetTestFitnessInjector(nil) }
			},
			buildOpts: func(cwd, outputDir, runID string) RunLoopOptions {
				return RunLoopOptions{
					Cwd:            cwd,
					OutputDir:      outputDir,
					RunID:          runID,
					RunTimeout:     30 * time.Second,
					MaxIterations:  1,
					PlateauEpsilon: 0.01,
					PlateauWindowK: 2,
					WarnOnly:       true,
					LogWriter:      io.Discard,
				}
			},
		},
		{
			name:       "StatusHaltedOnRegressionPostCommit_LegacyPath",
			wantStatus: StatusHaltedOnRegressionPostCommit,
			setup: func(t *testing.T) func() {
				SetTestFitnessInjector(injectConstantFitness(0.8))
				SetTestPostCommitFaultInjector(func(_ int, cwd string) error {
					path := filepath.Join(cwd, ".agents", "learnings", "learning-000.md")
					return os.WriteFile(path, []byte("# Fixture\n\nNo frontmatter here.\n"), 0o644)
				})
				return func() {
					SetTestFitnessInjector(nil)
					SetTestPostCommitFaultInjector(nil)
				}
			},
			buildOpts: func(cwd, outputDir, runID string) RunLoopOptions {
				return RunLoopOptions{
					Cwd:            cwd,
					OutputDir:      outputDir,
					RunID:          runID,
					RunTimeout:     30 * time.Second,
					MaxIterations:  1,
					PlateauEpsilon: 0.01,
					PlateauWindowK: 2,
					WarnOnly:       false,
					LogWriter:      io.Discard,
				}
			},
		},
		{
			name:       "StatusRolledBackPreCommit_ReduceFailure",
			wantStatus: StatusRolledBackPreCommit,
			setup: func(t *testing.T) func() {
				prev := refreshInjectCacheFn
				refreshInjectCacheFn = func(_ context.Context, stagingCwd string, _ io.Writer) (*InjectRefreshResult, error) {
					path := filepath.Join(stagingCwd, ".agents", "learnings", "learning-000.md")
					if err := os.WriteFile(path, []byte("# Fixture\n\nNo frontmatter here.\n"), 0o644); err != nil {
						return nil, err
					}
					return &InjectRefreshResult{
						Attempted: true,
						Succeeded: true,
						Method:    "in-process",
						Duration:  time.Millisecond,
					}, nil
				}
				return func() { refreshInjectCacheFn = prev }
			},
			buildOpts: func(cwd, outputDir, runID string) RunLoopOptions {
				return RunLoopOptions{
					Cwd:            cwd,
					OutputDir:      outputDir,
					RunID:          runID,
					RunTimeout:     30 * time.Second,
					MaxIterations:  1,
					PlateauEpsilon: 0.01,
					PlateauWindowK: 2,
					WarnOnly:       false,
					LogWriter:      io.Discard,
				}
			},
			wantErr: true,
		},
		{
			name:       "StatusFailed_IngestOrCheckpointError",
			wantStatus: StatusFailed,
			setup: func(t *testing.T) func() {
				SetTestIngestFaultInjector(func(int) error {
					return errors.New("synthetic ingest failure")
				})
				return func() { SetTestIngestFaultInjector(nil) }
			},
			buildOpts: func(cwd, outputDir, runID string) RunLoopOptions {
				return RunLoopOptions{
					Cwd:            cwd,
					OutputDir:      outputDir,
					RunID:          runID,
					RunTimeout:     30 * time.Second,
					MaxIterations:  1,
					PlateauEpsilon: 0.01,
					PlateauWindowK: 2,
					WarnOnly:       false,
					LogWriter:      io.Discard,
				}
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			restore := stubInjectRefresh(t)
			defer restore()

			if tc.setup != nil {
				cleanup := tc.setup(t)
				if cleanup != nil {
					defer cleanup()
				}
			}

			dir := t.TempDir()
			if err := fixture.GenerateFixture(dir, fixture.DefaultOpts()); err != nil {
				t.Fatalf("GenerateFixture: %v", err)
			}

			agentsDir := filepath.Join(dir, ".agents")
			runID := "hash-invariant-" + sanitizeRunID(tc.name)
			outputDir := filepath.Join(dir, ".agents", "overnight", runID)
			priorCount := 0
			if tc.seedPriorDone {
				seedPriorDoneIteration(t, outputDir, runID)
				priorCount = 1
			}

			opts := tc.buildOpts(dir, outputDir, runID)

			// Capture the baseline hash BEFORE RunLoop executes. Then compare
			// it to the hash after the newly emitted terminal iteration. Prior
			// seeded history, when present, is already included in the baseline.
			hashBefore, err := liveTreeHash(agentsDir)
			if err != nil {
				t.Fatalf("hashBefore: %v", err)
			}

			result, err := RunLoop(context.Background(), opts)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("RunLoop err=nil, want error for %s", tc.wantStatus)
				}
			} else if err != nil {
				t.Fatalf("RunLoop: %v", err)
			}
			if result == nil || len(result.Iterations) <= priorCount {
				t.Fatalf("result has no iterations: result=%+v", result)
			}

			hashAfter, err := liveTreeHash(agentsDir)
			if err != nil {
				t.Fatalf("hashAfter: %v", err)
			}

			newIters := result.Iterations[priorCount:]
			if len(newIters) != 1 {
				t.Fatalf("new iteration count = %d, want 1; statuses=%s",
					len(newIters), iterStatusSummary(result.Iterations))
			}
			lastIter := newIters[len(newIters)-1]
			if tc.wantStatus != "" && lastIter.Status != tc.wantStatus {
				t.Fatalf("last iter status = %q, want %q", lastIter.Status, tc.wantStatus)
			}

			// Core invariant for the newly emitted terminal iteration.
			treeChanged := hashBefore != hashAfter
			if lastIter.Status.IsCorpusCompounded() != treeChanged {
				t.Fatalf("invariant broken: status=%s compounded=%v treeChanged=%v "+
					"(hashBefore=%s hashAfter=%s). Iterations: %s",
					lastIter.Status, lastIter.Status.IsCorpusCompounded(), treeChanged,
					hashBefore, hashAfter,
					iterStatusSummary(result.Iterations))
			}

			// Additional case-specific assertions.
			switch tc.wantStatus {
			case StatusDone:
				if !lastIter.Status.IsCorpusCompounded() {
					t.Fatalf("StatusDone.IsCorpusCompounded()=false; want true")
				}
				if hashBefore == hashAfter {
					t.Fatalf("StatusDone did not mutate the live tree (hash unchanged)")
				}
			case StatusHaltedOnRegressionPreCommit:
				if lastIter.Status.IsCorpusCompounded() {
					t.Fatalf("StatusHaltedOnRegressionPreCommit.IsCorpusCompounded()=true; want false")
				}
				if hashBefore != hashAfter {
					t.Fatalf("StatusHaltedOnRegressionPreCommit mutated the live tree")
				}
			}
		})
	}
}

func seedPriorDoneIteration(t *testing.T, outputDir, runID string) {
	t.Helper()
	iterDir := filepath.Join(outputDir, runID, "iterations")
	if err := writeIterationAtomic(iterDir, IterationSummary{
		ID:           IterationID(runID + "-iter-1"),
		Index:        1,
		Status:       StatusDone,
		FitnessAfter: map[string]any{"composite": 0.9},
	}); err != nil {
		t.Fatalf("seed prior iteration: %v", err)
	}
}

// sanitizeRunID lowercases and replaces underscores in a subtest name so the
// resulting runID is safe to use as a path component.
func sanitizeRunID(name string) string {
	out := strings.ToLower(name)
	out = strings.ReplaceAll(out, "_", "-")
	out = strings.ReplaceAll(out, " ", "-")
	return out
}

// iterStatusSummary returns a compact "[idx:status,...]" string for test
// failure diagnostics so a broken invariant shows which iter carried which
// status without requiring a separate dump.
func iterStatusSummary(iters []IterationSummary) string {
	parts := make([]string, 0, len(iters))
	for _, it := range iters {
		parts = append(parts, string(it.Status))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// liveTreeHash returns a deterministic SHA-256 over every regular file under
// dir, sorted by path, EXCLUDING .agents/overnight/** (RunLoop's own runtime
// artifacts which would mask the corpus-mutation predicate under test).
//
// This is a local copy of the agentsHash helper in loop_resume_test.go; it is
// duplicated (not referenced) so this file stays independent of compilation
// ordering and so the helper's exact exclusion list is visible inline for
// review. The two copies must stay in sync; if you change one, change the
// other.
func liveTreeHash(dir string) (string, error) {
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
		// Skip runtime artifacts written by RunLoop itself.
		if strings.HasPrefix(rel, "overnight"+string(filepath.Separator)) || rel == "overnight" {
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
