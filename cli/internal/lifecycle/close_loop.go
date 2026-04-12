// Package lifecycle (close_loop.go) provides an in-process entry point for the
// knowledge flywheel close-loop orchestrator. The business logic was extracted
// from cli/cmd/ao/flywheel_close_loop.go so that Dream's REDUCE stage (and
// tests) can drive the close-loop chain without shelling out to the CLI.
//
// The cmd/ao/flywheel_close_loop.go file remains as a thin cobra adapter that
// supplies the package-main-only helpers via function fields on CloseLoopOpts.
//
// IMPORTANT: This file is unrelated to feedback_loop.go (AnnealedAlpha MemRL
// helper). Do not merge the two.
package lifecycle

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/boshu2/agentops/cli/internal/pool"
	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/storage"
	"github.com/boshu2/agentops/cli/internal/types"
)

// CloseLoopIngestResult mirrors the pool-ingest output shape.
type CloseLoopIngestResult struct {
	FilesScanned     int      `json:"files_scanned"`
	CandidatesFound  int      `json:"candidates_found"`
	Added            int      `json:"added"`
	SkippedExisting  int      `json:"skipped_existing"`
	SkippedMalformed int      `json:"skipped_malformed"`
	Errors           int      `json:"errors"`
	AddedIDs         []string `json:"added_ids,omitempty"`
}

// CloseLoopAutoPromoteResult mirrors the auto-promote/promote output shape.
type CloseLoopAutoPromoteResult struct {
	Threshold  string   `json:"threshold"`
	Considered int      `json:"considered"`
	Promoted   int      `json:"promoted"`
	Skipped    int      `json:"skipped"`
	Artifacts  []string `json:"artifacts,omitempty"`
	SkippedIDs []string `json:"skipped_ids,omitempty"`
}

// CloseLoopAntiPatternResult captures maturity-transition counts and paths.
type CloseLoopAntiPatternResult struct {
	Eligible int      `json:"eligible"`
	Promoted int      `json:"promoted"`
	Paths    []string `json:"paths,omitempty"`
}

// CloseLoopStoreResult captures store-index upsert counts.
type CloseLoopStoreResult struct {
	Categorize bool   `json:"categorize"`
	Indexed    int    `json:"indexed"`
	IndexPath  string `json:"index_path,omitempty"`
}

// CloseLoopCitationFeedbackResult captures citation-to-utility counts.
type CloseLoopCitationFeedbackResult struct {
	Processed int `json:"processed"`
	Rewarded  int `json:"rewarded"`
	Skipped   int `json:"skipped"`
}

// CloseLoopResult is the aggregate result of a close-loop run. The shape is
// intentionally byte-compatible with the cobra command's historical JSON.
type CloseLoopResult struct {
	Ingest           CloseLoopIngestResult           `json:"ingest"`
	AutoPromote      CloseLoopAutoPromoteResult      `json:"auto_promote"`
	AntiPattern      CloseLoopAntiPatternResult      `json:"anti_pattern"`
	Store            CloseLoopStoreResult            `json:"store"`
	CitationFeedback CloseLoopCitationFeedbackResult `json:"citation_feedback"`
	MemoryPromoted   int                             `json:"memory_promoted"`
	// Degraded collects soft-fail warnings produced during the run. The cobra
	// adapter renders them to stderr; in-process callers can inspect them.
	Degraded []string `json:"degraded,omitempty"`
}

// CloseLoopOpts configures a close-loop run. Callers (the cobra adapter or
// Dream's REDUCE stage) must supply the injected function fields, which carry
// the cmd/ao package-main helpers that remain outside internal/lifecycle.
type CloseLoopOpts struct {
	// PendingDir is the directory to ingest pending markdown from, relative to cwd.
	PendingDir string
	// Threshold is the minimum candidate age for auto-promotion.
	Threshold time.Duration
	// Quiet suppresses non-essential soft-fail warnings (hook-friendly mode).
	Quiet bool
	// DryRun routes all mutation-capable calls to read-only paths.
	DryRun bool
	// IncludeGold controls whether gold-tier candidates are eligible for
	// promotion (the cobra command currently passes true).
	IncludeGold bool

	// --- Injected callbacks (owned by cmd/ao package main) ---

	// ResolveIngestFiles resolves pending-markdown file paths. Mirrors
	// cmd/ao.resolveIngestFiles.
	ResolveIngestFiles func(cwd, pendingDir string, args []string) ([]string, error)
	// IngestFilesToPool ingests pending markdown files into the pool candidate
	// store. Mirrors cmd/ao.ingestPendingFilesToPool.
	IngestFilesToPool func(cwd string, files []string) (CloseLoopIngestResult, error)
	// AutoPromoteFn runs auto-promote+promote over pool candidates. Mirrors
	// cmd/ao.autoPromoteAndPromoteToArtifacts.
	AutoPromoteFn func(p *pool.Pool, threshold time.Duration, includeGold bool) (CloseLoopAutoPromoteResult, error)
	// ProcessCitationFeedback runs citation->utility feedback. Mirrors
	// cmd/ao.processCitationFeedback.
	ProcessCitationFeedback func(cwd string) (processed, rewarded, skipped int)
	// PromoteCitedLearnings auto-promotes learnings whose utility was bumped by
	// citation feedback. Mirrors cmd/ao.promoteCitedLearnings.
	PromoteCitedLearnings func(cwd string, quiet bool) int
	// PromoteToMemory promotes high-value learnings to MEMORY.md. Mirrors
	// cmd/ao.promoteToMemory.
	PromoteToMemory func(cwd string) (int, error)
	// FindLearningFile locates the on-disk path for a learning ID. Mirrors
	// cmd/ao.findLearningFile. This field is optional; if ApplyMaturityFn is
	// supplied the lifecycle layer does not call FindLearningFile directly.
	FindLearningFile func(baseDir, learningID string) (string, error)
	// ApplyMaturityFn applies all pending maturity transitions and returns the
	// summary. If non-nil, this overrides the lifecycle-internal implementation
	// (used to preserve byte-compatible behavior with the pre-refactor
	// cmd/ao.applyAllMaturityTransitions helper).
	ApplyMaturityFn func(cwd string) (MaturityTransitionSummary, error)
	// StoreIndexUpsertFn updates the store index for the provided paths and
	// returns the (indexed count, index path). If non-nil, this overrides the
	// lifecycle-internal implementation to preserve exact byte-compatibility
	// with the pre-refactor cmd/ao.storeIndexUpsert helper.
	StoreIndexUpsertFn func(baseDir string, paths []string, categorize bool) (int, string, error)

	// --- Optional overrides for testing ---

	// Now returns the current time. Defaults to time.Now.
	Now func() time.Time
}

// MaturityTransitionSummary holds the results of applying all maturity
// transitions. Exported so the cobra adapter can return compatible values.
type MaturityTransitionSummary struct {
	Total        int      `json:"total"`
	Applied      int      `json:"applied"`
	ChangedPaths []string `json:"changed_paths,omitempty"`
}

// ExecuteCloseLoop runs the flywheel close-loop chain in-process:
//
//  1. pool ingest (pending markdown -> pool candidates)
//  2. auto-promote + promote eligible candidates to artifacts
//  3. citation-to-utility feedback (BEFORE maturity so utility bumps are
//     reflected in this cycle)
//  4. auto-promote learnings whose utility was bumped by citation feedback
//  5. apply all maturity transitions across learnings and patterns
//  6. store index (categorize) for newly created/changed artifacts
//  7. promote high-value learnings to MEMORY.md (soft-fail)
//  8. generate skill drafts (soft-fail)
//
// The function does not write to stdout. All rendering stays in the caller.
// Soft-fail conditions are recorded in CloseLoopResult.Degraded.
//
// Required opts fields: ResolveIngestFiles, IngestFilesToPool, AutoPromoteFn,
// ProcessCitationFeedback, PromoteCitedLearnings, PromoteToMemory,
// FindLearningFile.
func ExecuteCloseLoop(cwd string, opts CloseLoopOpts) (*CloseLoopResult, error) {
	if err := validateCloseLoopOpts(opts); err != nil {
		return nil, err
	}

	result := &CloseLoopResult{}

	// 1) pool ingest
	ingestFiles, err := opts.ResolveIngestFiles(cwd, opts.PendingDir, nil)
	if err != nil {
		return result, err
	}
	result.Ingest, err = opts.IngestFilesToPool(cwd, ingestFiles)
	if err != nil {
		return result, err
	}

	// 2) auto-promote + promote
	p := pool.NewPool(cwd)
	result.AutoPromote, err = opts.AutoPromoteFn(p, opts.Threshold, opts.IncludeGold)
	if err != nil {
		return result, err
	}

	// 3) citation-to-utility feedback (before maturity transitions)
	processed, rewarded, skipped := opts.ProcessCitationFeedback(cwd)
	result.CitationFeedback.Processed = processed
	result.CitationFeedback.Rewarded = rewarded
	result.CitationFeedback.Skipped = skipped

	// 4) auto-promote learnings whose utility was bumped by citation feedback
	opts.PromoteCitedLearnings(cwd, opts.Quiet)

	// 5) apply ALL maturity transitions. Prefer the injected callback so the
	// cobra adapter can reuse its existing (well-tested) helper; fall back to
	// the lifecycle-internal implementation if only FindLearningFile is wired.
	var maturityResult MaturityTransitionSummary
	if opts.ApplyMaturityFn != nil {
		maturityResult, err = opts.ApplyMaturityFn(cwd)
	} else {
		maturityResult, err = applyAllMaturityTransitionsInternal(cwd, opts.DryRun, opts.FindLearningFile)
	}
	if err != nil {
		return result, err
	}
	result.AntiPattern.Eligible = maturityResult.Total
	result.AntiPattern.Promoted = maturityResult.Applied
	result.AntiPattern.Paths = maturityResult.ChangedPaths

	// 6) store index for newly created/changed artifacts
	pathsToIndex := append([]string{}, result.AutoPromote.Artifacts...)
	pathsToIndex = append(pathsToIndex, maturityResult.ChangedPaths...)
	result.Store.Categorize = true
	var indexed int
	var indexPath string
	if opts.StoreIndexUpsertFn != nil {
		indexed, indexPath, err = opts.StoreIndexUpsertFn(cwd, pathsToIndex, true)
	} else {
		indexed, indexPath, err = storeIndexUpsertLifecycle(cwd, pathsToIndex, true, opts.DryRun)
	}
	if err != nil {
		return result, err
	}
	result.Store.Indexed = indexed
	result.Store.IndexPath = indexPath

	// 7) promote high-value learnings to MEMORY.md (soft-fail)
	memoryPromoted, memErr := opts.PromoteToMemory(cwd)
	if memErr != nil {
		result.Degraded = append(result.Degraded, fmt.Sprintf("memory promotion: %v", memErr))
	}
	result.MemoryPromoted = memoryPromoted

	// 8) generate skill drafts (soft-fail)
	if draftResult, draftErr := ratchet.GenerateSkillDrafts(cwd); draftErr != nil {
		result.Degraded = append(result.Degraded, fmt.Sprintf("skill draft generation: %v", draftErr))
	} else if draftResult.Generated > 0 {
		result.Degraded = append(result.Degraded, fmt.Sprintf("info: generated %d skill draft(s)", draftResult.Generated))
	}

	return result, nil
}

func validateCloseLoopOpts(opts CloseLoopOpts) error {
	requiredCallbacks := []struct {
		name    string
		missing bool
	}{
		{name: "ResolveIngestFiles", missing: opts.ResolveIngestFiles == nil},
		{name: "IngestFilesToPool", missing: opts.IngestFilesToPool == nil},
		{name: "AutoPromoteFn", missing: opts.AutoPromoteFn == nil},
		{name: "ProcessCitationFeedback", missing: opts.ProcessCitationFeedback == nil},
		{name: "PromoteCitedLearnings", missing: opts.PromoteCitedLearnings == nil},
		{name: "PromoteToMemory", missing: opts.PromoteToMemory == nil},
	}
	for _, callback := range requiredCallbacks {
		if callback.missing {
			return fmt.Errorf("close-loop: %s callback is required", callback.name)
		}
	}
	if opts.ApplyMaturityFn == nil && opts.FindLearningFile == nil {
		return fmt.Errorf("close-loop: either ApplyMaturityFn or FindLearningFile is required")
	}
	return nil
}

// applyAllMaturityTransitionsInternal scans .agents/learnings and
// .agents/patterns for pending maturity transitions and applies them. This is
// the lifecycle-owned fallback used only when ApplyMaturityFn is not supplied
// by the caller. The cobra adapter supplies its own well-tested helper via
// ApplyMaturityFn, which preserves byte-compatible behavior.
func applyAllMaturityTransitionsInternal(cwd string, dryRun bool, findLearningFile func(baseDir, learningID string) (string, error)) (MaturityTransitionSummary, error) {
	dirs := []string{
		filepath.Join(cwd, ".agents", "learnings"),
		filepath.Join(cwd, ".agents", "patterns"),
	}

	summary := MaturityTransitionSummary{}
	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		results, err := ratchet.ScanForMaturityTransitions(dir)
		if err != nil {
			return MaturityTransitionSummary{}, fmt.Errorf("scan transitions in %s: %w", dir, err)
		}

		summary.Total += len(results)
		if len(results) == 0 || dryRun {
			continue
		}

		for _, r := range results {
			learningPath, ferr := findLearningFile(filepath.Dir(dir), r.LearningID)
			if ferr != nil {
				continue
			}
			applied, aerr := ratchet.ApplyMaturityTransition(learningPath)
			if aerr != nil {
				continue
			}
			if applied.Transitioned {
				summary.Applied++
				summary.ChangedPaths = append(summary.ChangedPaths, learningPath)
			}
		}
	}

	return summary, nil
}

// indexEntryAlias is the lifecycle-local alias for the shared storage index
// entry type, mirroring cmd/ao.IndexEntry.
type indexEntryAlias = storage.SearchIndexEntry

// loadExistingIndexEntries reads existing entries from a JSONL index file
// (best-effort). Mirrors cmd/ao.loadExistingIndexEntries.
func loadExistingIndexEntries(indexPath string) map[string]indexEntryAlias {
	existing := make(map[string]indexEntryAlias)
	f, err := os.Open(indexPath)
	if err != nil {
		return existing
	}
	defer func() {
		_ = f.Close() //nolint:errcheck // best-effort
	}()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var e indexEntryAlias
		if err := json.Unmarshal(scanner.Bytes(), &e); err == nil && e.Path != "" {
			existing[e.Path] = e
		}
	}
	return existing
}

// upsertIndexPaths creates/updates index entries for the given paths, mirroring
// cmd/ao.upsertIndexPaths. Returns the number of paths successfully indexed.
func upsertIndexPaths(existing map[string]indexEntryAlias, paths []string, categorize bool) int {
	indexed := 0
	for _, p := range paths {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err != nil {
			continue
		}
		entry, err := storage.CreateSearchIndexEntry(p, categorize)
		if err != nil {
			continue
		}
		existing[p] = *entry
		indexed++
	}
	return indexed
}

// writeIndexFile writes the entries map as sorted JSONL to the given path.
// Mirrors cmd/ao.writeIndexFile.
func writeIndexFile(indexPath string, existing map[string]indexEntryAlias) error {
	if err := os.MkdirAll(filepath.Dir(indexPath), 0750); err != nil {
		return err
	}
	out, err := os.OpenFile(indexPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close() //nolint:errcheck // write completed
	}()

	pathsSorted := make([]string, 0, len(existing))
	for p := range existing {
		pathsSorted = append(pathsSorted, p)
	}
	sort.Strings(pathsSorted)

	enc := json.NewEncoder(out)
	for _, p := range pathsSorted {
		if err := enc.Encode(existing[p]); err != nil {
			return err
		}
	}
	return nil
}

// storeIndexUpsertLifecycle updates the store index for the provided paths.
// Mirrors cmd/ao.storeIndexUpsert. Returns (indexed count, index path, error).
// dryRun skips the write step but still computes the in-memory upsert count so
// callers get the same numbers they would see from the cobra command.
func storeIndexUpsertLifecycle(baseDir string, paths []string, categorize bool, dryRun bool) (int, string, error) {
	indexPath := filepath.Join(baseDir, storage.SearchIndexDir, storage.SearchIndexFileName)
	if len(paths) == 0 {
		return 0, indexPath, nil
	}

	existing := loadExistingIndexEntries(indexPath)
	indexed := upsertIndexPaths(existing, paths, categorize)

	if dryRun {
		return indexed, indexPath, nil
	}

	if err := writeIndexFile(indexPath, existing); err != nil {
		return indexed, indexPath, err
	}

	return indexed, indexPath, nil
}

// Ensure unused imports don't fail builds if types shifts occur.
var _ = types.TierSilver
