package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/boshu2/agentops/cli/internal/config"
	"github.com/boshu2/agentops/cli/internal/llm"
	ovn "github.com/boshu2/agentops/cli/internal/overnight"
)

// DreamSubCycleOptions configures a Dream sub-cycle when invoked as part of
// an evolve umbrella run. Lighter than a full overnight run: no council, no
// morning report, no lock (the caller owns the lock).
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
	Iterations    int                `json:"iterations"`
	PlateauReason string             `json:"plateau_reason,omitempty"`
	Degraded      []string           `json:"degraded,omitempty"`
	Tier1Forge    *tier1ForgeSummary `json:"tier1_forge,omitempty"`
}

type tier1ForgeSummary struct {
	FilesProcessed int    `json:"files_processed"`
	FilesSkipped   int    `json:"files_skipped,omitempty"`
	SessionsWrote  int    `json:"sessions_wrote"`
	Errors         int    `json:"errors,omitempty"`
	Mode           string `json:"mode,omitempty"`
	Queued         int    `json:"queued,omitempty"`
	QueueDir       string `json:"queue_dir,omitempty"`
}

// RunDreamSubCycle executes the Dream knowledge-compounding loop as a
// sub-cycle within an evolve umbrella run. It runs INGEST → REDUCE →
// MEASURE → COMMIT iterations until a halt condition fires, then runs
// the Tier 1 forge post-loop hook on recent sessions.
//
// Unlike runOvernightStart, this does NOT acquire locks, run council,
// write the morning report, or manage keep-awake. The caller owns all of
// those concerns.
func RunDreamSubCycle(ctx context.Context, opts DreamSubCycleOptions) (*DreamSubCycleResult, error) {
	if opts.Cwd == "" {
		return nil, fmt.Errorf("dream sub-cycle: cwd is required")
	}
	if opts.OutputDir == "" {
		opts.OutputDir = filepath.Join(opts.Cwd, ".agents", "evolve", "dream-latest")
	}
	if opts.RunID == "" {
		opts.RunID = time.Now().UTC().Format("20060102T150405Z")
	}
	if opts.RunTimeout <= 0 {
		opts.RunTimeout = 30 * time.Minute
	}
	if opts.MaxIterations <= 0 {
		opts.MaxIterations = 10
	}
	if opts.LogWriter == nil {
		opts.LogWriter = io.Discard
	}

	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("dream sub-cycle: mkdir: %w", err)
	}

	// Crash recovery before running.
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
	}

	// Post-loop: Tier 1 forge on recent sessions.
	outDir := filepath.Join(opts.Cwd, ".agents", "ao", "sessions")
	t1Summary, t1Err := runDreamTier1ForgePostLoop(opts.Cwd, outDir, "ao-evolve-dream-tier1")
	if t1Err != nil {
		result.Degraded = append(result.Degraded, fmt.Sprintf("tier1-forge: %v", t1Err))
	} else if t1Summary != nil {
		result.Tier1Forge = t1Summary
	}

	if !opts.Quiet {
		fmt.Fprintf(opts.LogWriter, "dream sub-cycle: %d iterations, plateau=%q, degraded=%d\n",
			result.Iterations, result.PlateauReason, len(result.Degraded))
	}

	return result, nil
}

func runDreamTier1ForgePostLoop(cwd, outDir, ingestedBy string) (*tier1ForgeSummary, error) {
	if os.Getenv(llm.KillSwitchEnv) == "1" {
		return nil, nil
	}
	sessions, err := collectRecentSessionJSONL(cwd)
	if err != nil {
		return nil, fmt.Errorf("collect sessions: %w", err)
	}
	if len(sessions) == 0 {
		return nil, nil
	}

	if queueResult, handled, err := enqueueForgeTier1ToCuratorQueue(sessions); handled {
		if err != nil {
			return nil, err
		}
		return &tier1ForgeSummary{
			Mode:     "dream-worker-queue",
			Queued:   queueResult.JobsQueued,
			QueueDir: queueResult.QueueDir,
		}, nil
	}

	t1Opts := resolveTier1Options(sessions, outDir, cwd)
	if t1Opts == nil {
		return nil, nil
	}
	t1Opts.IngestedBy = ingestedBy

	t1Result, err := llm.RunForgeTier1(*t1Opts)
	if err != nil {
		return nil, err
	}
	if t1Result == nil {
		return nil, nil
	}
	return &tier1ForgeSummary{
		Mode:           "local-llm",
		FilesProcessed: t1Result.FilesProcessed,
		FilesSkipped:   t1Result.FilesSkipped,
		SessionsWrote:  len(t1Result.SessionsWrote),
		Errors:         len(t1Result.Errors),
	}, nil
}

// resolveTier1Options builds Tier1Options from config, returning nil when
// no curator model is configured (opt-in).
func resolveTier1Options(sessions []string, outDir, cwd string) *llm.Tier1Options {
	resolved := resolveConfigForTier1()
	if resolved.model == "" {
		return nil
	}
	return &llm.Tier1Options{
		SourcePaths: sessions,
		OutputDir:   outDir,
		Model:       resolved.model,
		Endpoint:    resolved.endpoint,
		Quiet:       true,
		Workspace:   cwd,
		IngestedBy:  "ao-evolve-dream-tier1",
	}
}

type tier1ConfigResolved struct {
	model    string
	endpoint string
}

func resolveConfigForTier1() tier1ConfigResolved {
	// Same resolution path as runPostLoopTier1Forge in overnight.go.
	resolved := config.Resolve("", "", false)
	model, _ := resolved.DreamCuratorModel.Value.(string)
	endpoint, _ := resolved.DreamCuratorOllamaURL.Value.(string)
	return tier1ConfigResolved{model: model, endpoint: endpoint}
}
