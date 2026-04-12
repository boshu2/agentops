package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/boshu2/agentops/cli/internal/llm"
	ovn "github.com/boshu2/agentops/cli/internal/overnight"
)

// DreamSubCycleOptions configures a Dream sub-cycle when invoked as part of
// an evolve umbrella run. Lighter than a full overnight run: no council, no
// morning report, no lock acquisition (the caller owns the lock).
type DreamSubCycleOptions struct {
	Cwd           string
	OutputDir     string
	RunID         string
	RunTimeout    time.Duration
	MaxIterations int
	LogWriter     io.Writer
	Quiet         bool
}

// DreamSubCycleResult is the return value from RunDreamSubCycle.
type DreamSubCycleResult struct {
	Iterations    int
	PlateauReason string
	Degraded      []string
	Tier1Result   *llm.Tier1Result
}

// RunDreamSubCycle executes the Dream knowledge-compounding loop as a
// sub-cycle within an evolve umbrella run. It runs INGEST → REDUCE →
// MEASURE → COMMIT iterations until a halt condition fires, then optionally
// runs the Tier 1 forge post-loop hook on recent sessions.
//
// Unlike runOvernightStart, this does NOT:
//   - Acquire/release the run lock (caller owns it)
//   - Run Dream Council (evolve handles post-mortem at teardown)
//   - Write the morning report (evolve writes a unified report)
//   - Manage keep-awake (caller manages it)
func RunDreamSubCycle(ctx context.Context, opts DreamSubCycleOptions) (*DreamSubCycleResult, error) {
	if opts.Cwd == "" {
		return nil, fmt.Errorf("dream sub-cycle: cwd is required")
	}
	if opts.OutputDir == "" {
		opts.OutputDir = filepath.Join(opts.Cwd, ".agents", "overnight", "latest")
	}
	if opts.RunID == "" {
		opts.RunID = time.Now().UTC().Format("20060102T150405Z")
	}
	if opts.RunTimeout <= 0 {
		opts.RunTimeout = 30 * time.Minute // shorter than standalone Dream
	}
	if opts.MaxIterations <= 0 {
		opts.MaxIterations = 10 // conservative for sub-cycle
	}
	if opts.LogWriter == nil {
		opts.LogWriter = io.Discard
	}

	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("dream sub-cycle: mkdir: %w", err)
	}

	// Crash recovery before running (same as overnight startup).
	recoveryActions, recErr := ovn.RecoverFromCrash(opts.Cwd)
	result := &DreamSubCycleResult{}
	if recErr != nil {
		result.Degraded = append(result.Degraded, fmt.Sprintf("dream recovery: %v", recErr))
	}
	if len(recoveryActions) > 20 {
		result.Degraded = append(result.Degraded,
			fmt.Sprintf("dream recovery: cleaned up %d stale items", len(recoveryActions)))
	} else {
		for _, a := range recoveryActions {
			result.Degraded = append(result.Degraded, "dream recovery: "+a)
		}
	}

	// Build loop options.
	runOpts := ovn.RunLoopOptions{
		Cwd:           opts.Cwd,
		OutputDir:     opts.OutputDir,
		RunID:         opts.RunID,
		RunTimeout:    opts.RunTimeout,
		MaxIterations: opts.MaxIterations,
		LogWriter:     opts.LogWriter,
	}

	loopCtx, cancel := context.WithTimeout(ctx, opts.RunTimeout)
	defer cancel()

	loopResult, loopErr := ovn.RunLoop(loopCtx, runOpts)
	if loopResult != nil {
		result.Iterations = len(loopResult.Iterations)
		result.PlateauReason = loopResult.PlateauReason
		result.Degraded = append(result.Degraded, loopResult.Degraded...)
	}
	if loopErr != nil {
		result.Degraded = append(result.Degraded, fmt.Sprintf("dream loop: %v", loopErr))
		// Don't fail the whole evolve run on Dream failure — degrade.
	}

	// Post-loop: Tier 1 forge on recent sessions.
	summary := &overnightSummary{CloseLoop: make(map[string]any)}
	runPostLoopTier1Forge(ctx, opts.Cwd, summary, overnightSettings{})
	result.Degraded = append(result.Degraded, summary.Degraded...)
	if cl, ok := summary.CloseLoop["tier1_forge"]; ok {
		if t1map, ok := cl.(map[string]any); ok {
			if fp, _ := t1map["files_processed"].(int); fp > 0 {
				// Extract result for the caller.
				result.Tier1Result = &llm.Tier1Result{FilesProcessed: fp}
			}
		}
	}

	if !opts.Quiet {
		fmt.Fprintf(opts.LogWriter, "dream sub-cycle: %d iterations, plateau=%q, degraded=%d\n",
			result.Iterations, result.PlateauReason, len(result.Degraded))
	}

	return result, nil
}
