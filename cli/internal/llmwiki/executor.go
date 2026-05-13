package llmwiki

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/boshu2/agentops/cli/internal/daemon"
)

// Stage names the four operations the executor can run on a tick.
type Stage string

const (
	StageIngest  Stage = "ingest"
	StageQuery   Stage = "query"
	StageLint    Stage = "lint"
	StagePromote Stage = "promote"
)

// DefaultLintIntervalHours is the default interval between LINT runs when the
// LoopJobSpec does not specify one. Karpathy's pattern runs lint roughly daily.
const DefaultLintIntervalHours = 24

// LoopJobSpec is the JSON payload of a JobTypeLLMWikiLoop job.
type LoopJobSpec struct {
	// Vault is the root path containing raw/ + wiki/.
	Vault string `json:"vault"`
	// Stages is an optional whitelist; if empty, all stages are eligible and
	// the executor selects one via SelectStage.
	Stages []Stage `json:"stages,omitempty"`
	// LintIntervalHours controls when LINT becomes eligible. 0 → use default.
	LintIntervalHours int `json:"lint_interval_hours,omitempty"`
}

// StageHandler executes a single stage against a vault. Implementations MUST be
// idempotent per the per-stage contract documented in the package doc comment.
type StageHandler interface {
	Run(ctx context.Context, vault string, attempt int) (StageResult, error)
}

// StageResult captures what a stage handler did this tick.
type StageResult struct {
	Stage         Stage    `json:"stage"`
	Attempt       int      `json:"attempt"`
	ArtifactsPath []string `json:"artifacts_path,omitempty"`
	Skipped       bool     `json:"skipped,omitempty"`
	SkipReason    string   `json:"skip_reason,omitempty"`
}

// LLMWikiLoopExecutor implements daemon.JobExecutor for JobTypeLLMWikiLoop.
//
// Stage handlers are injected so tests can stub them. Production handlers come
// from cli/internal/llmwiki/stages.go (introduced in soc-8inr.8). A nil handler
// means "stage selected but no implementation wired"; Execute records that as
// Skipped rather than erroring, which lets soc-8inr.7 ship without blocking on
// stage implementations.
type LLMWikiLoopExecutor struct {
	Ingest  StageHandler
	Query   StageHandler
	Lint    StageHandler
	Promote StageHandler

	// Now is injected for testability; defaults to time.Now.
	Now func() time.Time
}

// JobTypes declares this executor handles JobTypeLLMWikiLoop.
func (e *LLMWikiLoopExecutor) JobTypes() []daemon.JobType {
	return []daemon.JobType{daemon.JobTypeLLMWikiLoop}
}

// RunJob executes ONE tick of the Karpathy loop. The Supervisor wraps this
// call with claim/heartbeat/terminal-record bookkeeping; we contribute only
// the per-tick stage execution.
//
// Behavior:
//  1. Parse LoopJobSpec from the claim payload.
//  2. Select a stage via SelectStage (or honor the Stages whitelist).
//  3. Look up the matching handler. If nil, return a Skipped result.
//  4. Invoke handler.Run with the QueueJobState attempt counter.
//  5. Emit artifacts so the supervisor can record them in the ledger.
func (e *LLMWikiLoopExecutor) RunJob(ctx context.Context, claim daemon.QueueLease) (daemon.JobExecutionResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if claim.Job.JobType != daemon.JobTypeLLMWikiLoop {
		return daemon.JobExecutionResult{}, fmt.Errorf("llmwiki executor does not support job type %s", claim.Job.JobType)
	}

	spec, err := parseLoopJobSpec(claim.Job.Payload)
	if err != nil {
		return daemon.JobExecutionResult{}, err
	}
	if spec.Vault == "" {
		return daemon.JobExecutionResult{}, errors.New("llmwiki executor: spec.vault is required")
	}

	now := e.now()
	stage, err := e.pickStage(spec, now)
	if err != nil {
		return daemon.JobExecutionResult{}, err
	}

	handler := e.handlerFor(stage)
	attempt := claim.Job.Attempt
	if attempt < 1 {
		attempt = 1
	}

	if handler == nil {
		// Allow soc-8inr.8 to wire handlers later without breaking the executor.
		result := StageResult{
			Stage:      stage,
			Attempt:    attempt,
			Skipped:    true,
			SkipReason: "handler-not-wired",
		}
		return execResult(result), nil
	}

	result, err := handler.Run(ctx, spec.Vault, attempt)
	if err != nil {
		return daemon.JobExecutionResult{}, fmt.Errorf("llmwiki stage %s failed: %w", stage, err)
	}
	if result.Stage == "" {
		result.Stage = stage
	}
	if result.Attempt == 0 {
		result.Attempt = attempt
	}
	return execResult(result), nil
}

// pickStage applies the spec's whitelist to SelectStage. If a whitelist is
// present, the executor will only run a stage from that set; if SelectStage
// would prefer a stage outside the whitelist, the first whitelisted stage is
// used instead. This makes the Stages field a hard filter, not a hint.
func (e *LLMWikiLoopExecutor) pickStage(spec LoopJobSpec, now time.Time) (Stage, error) {
	preferred := SelectStage(spec.Vault, spec.LintIntervalHours, now)
	if len(spec.Stages) == 0 {
		return preferred, nil
	}
	for _, s := range spec.Stages {
		if s == preferred {
			return preferred, nil
		}
	}
	// Fall through: whitelist excludes the preferred stage. Honor the first
	// whitelisted stage that is a known stage.
	for _, s := range spec.Stages {
		if isKnownStage(s) {
			return s, nil
		}
	}
	return "", fmt.Errorf("llmwiki: spec.stages %v contains no known stages", spec.Stages)
}

func (e *LLMWikiLoopExecutor) handlerFor(stage Stage) StageHandler {
	switch stage {
	case StageIngest:
		return e.Ingest
	case StageQuery:
		return e.Query
	case StageLint:
		return e.Lint
	case StagePromote:
		return e.Promote
	default:
		return nil
	}
}

func (e *LLMWikiLoopExecutor) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

func isKnownStage(s Stage) bool {
	switch s {
	case StageIngest, StageQuery, StageLint, StagePromote:
		return true
	default:
		return false
	}
}

func parseLoopJobSpec(payload map[string]any) (LoopJobSpec, error) {
	var spec LoopJobSpec
	if len(payload) == 0 {
		return spec, errors.New("llmwiki executor: payload is required")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return spec, fmt.Errorf("llmwiki executor: marshal payload: %w", err)
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		return spec, fmt.Errorf("llmwiki executor: parse payload: %w", err)
	}
	return spec, nil
}

func execResult(r StageResult) daemon.JobExecutionResult {
	artifacts := map[string]string{
		"stage":   string(r.Stage),
		"attempt": fmt.Sprintf("%d", r.Attempt),
	}
	if r.Skipped {
		artifacts["skipped"] = "true"
		if r.SkipReason != "" {
			artifacts["skip_reason"] = r.SkipReason
		}
	}
	for i, p := range r.ArtifactsPath {
		artifacts[fmt.Sprintf("artifact_%d", i)] = p
	}
	return daemon.JobExecutionResult{Artifacts: artifacts}
}

// SelectStage chooses which stage to run on this tick based on vault state.
//
// Order of preference:
//  1. INGEST — if vault/raw/ has files newer than vault/wiki/.last-ingest.
//  2. LINT   — if now - last-lint > lintIntervalHours.
//  3. INGEST — conservative default (re-scan).
//
// QUERY and PROMOTE are never auto-selected; they are invoked on demand via
// the LoopJobSpec.Stages whitelist.
func SelectStage(vault string, lintIntervalHours int, now time.Time) Stage {
	if lintIntervalHours <= 0 {
		lintIntervalHours = DefaultLintIntervalHours
	}
	if rawHasNewerFiles(vault) {
		return StageIngest
	}
	if lintIsStale(vault, lintIntervalHours, now) {
		return StageLint
	}
	return StageIngest
}

// rawHasNewerFiles reports whether vault/raw/ contains any regular file with
// a mtime newer than vault/wiki/.last-ingest. Missing raw/ → false. Missing
// .last-ingest sentinel → true if raw/ has any regular file (the conservative
// "we have not ingested anything yet" interpretation).
func rawHasNewerFiles(vault string) bool {
	rawDir := filepath.Join(vault, "raw")
	rawEntries, err := os.ReadDir(rawDir)
	if err != nil || len(rawEntries) == 0 {
		return false
	}
	sentinel := filepath.Join(vault, "wiki", ".last-ingest")
	sentinelInfo, sentinelErr := os.Stat(sentinel)
	for _, entry := range rawEntries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if sentinelErr != nil {
			// No sentinel: any regular file qualifies.
			return true
		}
		if info.ModTime().After(sentinelInfo.ModTime()) {
			return true
		}
	}
	return false
}

// lintIsStale reports whether vault/wiki/.last-lint is older than the
// lintIntervalHours threshold. Missing sentinel → true (we should run lint).
func lintIsStale(vault string, lintIntervalHours int, now time.Time) bool {
	sentinel := filepath.Join(vault, "wiki", ".last-lint")
	info, err := os.Stat(sentinel)
	if err != nil {
		return true
	}
	threshold := time.Duration(lintIntervalHours) * time.Hour
	return now.Sub(info.ModTime()) > threshold
}
