// This file holds the llmwiki builder core: the Stage vocabulary, the
// StageHandler/StageResult contract, and the SelectStage scheduling heuristic.
// The Ingest/Query/Lint/Promote stage handlers live in stages.go.
//
// The daemon-job-executor wrapper (LLMWikiLoopExecutor + LoopJobSpec) that
// previously drove these stages as JobTypeLLMWikiLoop jobs was removed with the
// daemon carve (soc-2rtm0 wave 5). The builder core remains: it backs the
// wiki bounded context (internal/wiki) and the lock-file atomic writers used by
// internal/scope.
package llmwiki

import (
	"context"
	"os"
	"path/filepath"
	"time"
)

// Stage names the four operations the wiki loop can run on a tick.
type Stage string

const (
	StageIngest  Stage = "ingest"
	StageCompile Stage = "compile"
	StageQuery   Stage = "query"
	StageLint    Stage = "lint"
	StagePromote Stage = "promote"
)

// DefaultLintIntervalHours is the default interval between LINT runs when no
// interval is specified. Karpathy's pattern runs lint roughly daily.
const DefaultLintIntervalHours = 24

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
