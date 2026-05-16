package doctor

// Knowledge subsystem detectors and fixers.
//
// This file implements the six knowledge failure modes from the Phase 2
// analysis. Four are auto-fixable, two are detect-only:
//
//	fm-knowledge-missing-substructure       (auto) — re-create missing store subdirs
//	fm-knowledge-corrupt-index-lines        (auto) — drop malformed search-index lines
//	fm-knowledge-torn-append-line           (auto) — drop a torn trailing index line
//	fm-knowledge-orphaned-flywheel-learnings(auto) — consolidate split learnings dirs
//	fm-knowledge-stale-index-drift          (detect-only) — index drifted from artifacts
//	fm-knowledge-false-freshness            (detect-only) — freshness from FS modtime
//
// Detectors are PURE: they only stat and read. Every fixer disk write flows
// through Mutate — there is no os.WriteFile/os.Remove/os.Rename/os.Create in
// this file. There is no Mkdir op in the canonical seven-op enum; the
// missing-substructure fixer therefore creates a directory by staging a
// `.gitkeep` placeholder inside the run's quarantine and renaming it into the
// target path via Op Rename, whose executeAtomic MkdirAll's the parent. That
// keeps directory creation routed through the single chokepoint.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Time-window constants for freshness analysis.
const (
	hour = time.Hour
	day  = 24 * time.Hour
)

// nowProvider returns the current time. It is a package var so tests can pin a
// deterministic clock for the false-freshness detector.
var nowProvider = time.Now

// absDuration returns the absolute value of a duration.
func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// parseIndexTime parses an index timestamp, accepting RFC3339 and RFC3339Nano.
func parseIndexTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

// freshnessSignals computes the two competing freshness signals for a sessions
// directory: fsNewest (max file modtime) and contentNewest (max parsed
// Session.Date across session JSONL files). parsedAny reports whether at least
// one session `date` field was parsed. It is pure: stat + read only.
func freshnessSignals(sessionsDir string, entries []os.DirEntry) (fsNewest, contentNewest time.Time, parsedAny bool) {
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if info, err := e.Info(); err == nil && info.ModTime().After(fsNewest) {
			fsNewest = info.ModTime()
		}
		if !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(sessionsDir, e.Name()))
		if err != nil {
			continue
		}
		for _, t := range splitNonEmptyLines(raw) {
			var obj struct {
				Date string `json:"date"`
			}
			if json.Unmarshal([]byte(t), &obj) != nil || obj.Date == "" {
				continue
			}
			if d, derr := parseIndexTime(obj.Date); derr == nil {
				parsedAny = true
				if d.After(contentNewest) {
					contentNewest = d
				}
			}
		}
	}
	return fsNewest, contentNewest, parsedAny
}

// knowledgeBaseDir returns the structured knowledge-store base, <cwd>/.agents/ao.
func knowledgeBaseDir(env *DetectEnv) string {
	return filepath.Join(env.CWD, ".agents", "ao")
}

// searchIndexPath returns the search-index JSONL path under the knowledge store.
func searchIndexPath(env *DetectEnv) string {
	return filepath.Join(knowledgeBaseDir(env), "index", "search-index.jsonl")
}

// requiredSubdirs is the three-subdir structural contract enforced by
// storage.FileStorage.Init for the knowledge store.
var requiredSubdirs = []string{"sessions", "index", "provenance"}

// indexableExts is the artifact file extension set the search index mirrors.
var indexableExts = map[string]bool{".md": true, ".jsonl": true}

// artifactSubdirs is the set of .agents subdirectories whose artifacts the
// search index is expected to mirror.
var artifactSubdirs = []string{"learnings", "patterns", "research", "retros", "candidates"}

// init registers all six knowledge detectors and four knowledge fixers.
func init() {
	RegisterDetector(missingSubstructureDetector{})
	RegisterDetector(corruptIndexLinesDetector{})
	RegisterDetector(tornAppendLineDetector{})
	RegisterDetector(orphanedFlywheelLearningsDetector{})
	RegisterDetector(staleIndexDriftDetector{})
	RegisterDetector(falseFreshnessDetector{})

	RegisterFixer(missingSubstructureFixer{})
	RegisterFixer(corruptIndexLinesFixer{})
	RegisterFixer(tornAppendLineFixer{})
	RegisterFixer(orphanedFlywheelLearningsFixer{})
	RegisterFixer(staleIndexDriftFixer{})
	RegisterFixer(falseFreshnessFixer{})
}

// ---------------------------------------------------------------------------
// Shared JSONL helpers (pure).
// ---------------------------------------------------------------------------

// indexLineValid reports whether a trimmed JSONL line parses as a search-index
// entry: valid JSON with a non-empty string `path` field. Blank input is not
// valid (callers skip blank lines before calling this).
func indexLineValid(trimmed string) bool {
	if trimmed == "" {
		return false
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
		return false
	}
	raw, ok := obj["path"]
	if !ok {
		return false
	}
	var p string
	if err := json.Unmarshal(raw, &p); err != nil {
		return false
	}
	return p != ""
}

// splitNonEmptyLines splits raw file bytes on newline, trimming each line and
// dropping empty results. Order is preserved.
func splitNonEmptyLines(raw []byte) []string {
	var out []string
	for _, ln := range strings.Split(string(raw), "\n") {
		t := strings.TrimSpace(ln)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// FM: fm-knowledge-missing-substructure (auto-fixable)
// ---------------------------------------------------------------------------

// missingSubstructureDetector flags a knowledge store whose base exists but is
// missing one or more of the sessions/, index/, provenance/ subdirectories.
type missingSubstructureDetector struct{}

func (missingSubstructureDetector) ID() string           { return "fm-knowledge-missing-substructure" }
func (missingSubstructureDetector) Subsystem() string    { return "knowledge" }
func (missingSubstructureDetector) Severity() string     { return "P2" }
func (missingSubstructureDetector) EstimatedCostMS() int { return 2 }
func (missingSubstructureDetector) OnlineRequired() bool { return false }
func (missingSubstructureDetector) QuickPath() bool      { return true }
func (missingSubstructureDetector) Describe() string {
	return "knowledge store base exists but is missing required subdirectories"
}

// missingSubdirs returns the required subdir names that are absent (or occupied
// by a non-directory) under base. If base itself is absent or not a directory
// it returns nil — that is a different (uninitialized-store) failure mode.
func missingSubdirs(base string) []string {
	info, err := os.Stat(base)
	if err != nil || !info.IsDir() {
		return nil
	}
	var missing []string
	for _, sub := range requiredSubdirs {
		st, err := os.Stat(filepath.Join(base, sub))
		if err != nil || !st.IsDir() {
			missing = append(missing, sub)
		}
	}
	return missing
}

func (d missingSubstructureDetector) Detect(env *DetectEnv) ([]Finding, error) {
	base := knowledgeBaseDir(env)
	if _, err := os.Stat(base); err != nil {
		return nil, nil
	}
	missing := missingSubdirs(base)
	if len(missing) == 0 {
		return nil, nil
	}
	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      fmt.Sprintf("knowledge store missing subdirs: %s", strings.Join(missing, ", ")),
		Confidence: 1.0,
		Evidence: Evidence{
			File:  ".agents/ao",
			Query: "for d in sessions index provenance; do test -d .agents/ao/$d || echo missing $d; done",
		},
		Remediation: Remediation{
			Command:          "ao doctor --fix --only " + d.ID(),
			ExplainCommand:   "ao doctor explain " + d.ID(),
			AutoFixable:      true,
			EstimatedActions: len(missing),
		},
	}}, nil
}

// missingSubstructureFixer re-creates absent knowledge-store subdirectories by
// routing a .gitkeep placeholder through Mutate Rename (which MkdirAll's the
// destination parent), since the canonical op enum has no Mkdir variant.
type missingSubstructureFixer struct{}

func (missingSubstructureFixer) ID() string { return "fm-knowledge-missing-substructure" }
func (missingSubstructureFixer) Preconditions() []string {
	return []string{
		".agents/ao exists and is a directory",
		"no required subdir name is occupied by a regular file",
	}
}
func (missingSubstructureFixer) WritesTo() []string { return []string{".agents/ao"} }
func (missingSubstructureFixer) Ops() []string      { return []string{"Rename"} }
func (missingSubstructureFixer) Reversible() bool   { return true }
func (missingSubstructureFixer) Idempotent() bool   { return true }
func (missingSubstructureFixer) AutoFixable() bool  { return true }

func (f missingSubstructureFixer) Fix(ctx *MutateContext, env *DetectEnv, _ []Finding) (FixResult, error) {
	res := FixResult{FixerID: f.ID(), FindingIDs: []string{f.ID()}}
	base := knowledgeBaseDir(env)
	if info, err := os.Stat(base); err != nil || !info.IsDir() {
		res.Err = fmt.Errorf("doctor: %s: .agents/ao absent or not a directory (refused_unsafe)", f.ID())
		return res, res.Err
	}
	// Re-read state; refuse if a required slot is occupied by a regular file.
	var missing []string
	for _, sub := range requiredSubdirs {
		p := filepath.Join(base, sub)
		st, err := os.Stat(p)
		if err == nil && !st.IsDir() {
			res.Err = fmt.Errorf("doctor: %s: %s exists as a non-directory; quarantine manually (refused_unsafe)", f.ID(), p)
			return res, res.Err
		}
		if err != nil {
			missing = append(missing, sub)
		}
	}
	if len(missing) == 0 {
		res.Fixed = true
		return res, nil
	}
	for _, sub := range missing {
		dest := filepath.Join(base, sub, ".gitkeep")
		if err := f.createDirViaRename(ctx, dest); err != nil {
			res.Err = err
			return res, err
		}
		res.ActionsTaken++
	}
	if len(missingSubdirs(base)) != 0 {
		res.Err = fmt.Errorf("doctor: %s: fix did not eliminate the finding", f.ID())
		return res, res.Err
	}
	res.Fixed = true
	return res, nil
}

// createDirViaRename creates the parent directory of dest by staging an empty
// .gitkeep placeholder inside the run quarantine and renaming it to dest. The
// Rename op's executeAtomic does MkdirAll(filepath.Dir(dest)), so the subdir is
// created through the chokepoint and recorded in actions.jsonl.
func (missingSubstructureFixer) createDirViaRename(ctx *MutateContext, dest string) error {
	rel, err := filepath.Rel(ctx.RepoRoot, dest)
	if err != nil {
		rel = filepath.Base(dest)
	}
	stage := filepath.Join(ctx.RunDir, "quarantine", "staged-mkdir", rel)
	if ctx.DryRun {
		fmt.Fprintf(os.Stderr, "[dry-run] would create %s via staged .gitkeep rename\n", filepath.Dir(dest))
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(stage), 0o755); err != nil {
		return fmt.Errorf("doctor: stage placeholder dir: %w", err)
	}
	if err := os.WriteFile(stage, nil, 0o644); err != nil {
		return fmt.Errorf("doctor: stage placeholder file: %w", err)
	}
	if _, err := Mutate(ctx, stage, Rename{To: dest}); err != nil {
		return fmt.Errorf("doctor: create dir %s: %w", filepath.Dir(dest), err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// FM: fm-knowledge-corrupt-index-lines (auto-fixable)
// ---------------------------------------------------------------------------

// corruptIndexLinesDetector flags non-JSON or schema-invalid lines anywhere in
// the search-index JSONL file.
type corruptIndexLinesDetector struct{}

func (corruptIndexLinesDetector) ID() string           { return "fm-knowledge-corrupt-index-lines" }
func (corruptIndexLinesDetector) Subsystem() string    { return "knowledge" }
func (corruptIndexLinesDetector) Severity() string     { return "P2" }
func (corruptIndexLinesDetector) EstimatedCostMS() int { return 5 }
func (corruptIndexLinesDetector) OnlineRequired() bool { return false }
func (corruptIndexLinesDetector) QuickPath() bool      { return false }
func (corruptIndexLinesDetector) Describe() string {
	return "search-index.jsonl contains malformed lines real readers silently skip"
}

// corruptIndexLineNos returns the 1-based line numbers of corrupt entries in
// the index, splitting raw bytes on newline so blank lines are tolerated.
func corruptIndexLineNos(raw []byte) []int {
	var bad []int
	for i, ln := range strings.Split(string(raw), "\n") {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		if !indexLineValid(t) {
			bad = append(bad, i+1)
		}
	}
	return bad
}

func (d corruptIndexLinesDetector) Detect(env *DetectEnv) ([]Finding, error) {
	idx := searchIndexPath(env)
	info, err := os.Stat(idx)
	if err != nil || info.Size() == 0 {
		return nil, nil
	}
	raw, err := os.ReadFile(idx)
	if err != nil {
		return nil, fmt.Errorf("doctor: read search index: %w", err)
	}
	bad := corruptIndexLineNos(raw)
	if len(bad) == 0 {
		return nil, nil
	}
	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      fmt.Sprintf("%d corrupt search-index line(s)", len(bad)),
		Confidence: 1.0,
		Evidence: Evidence{
			File:  ".agents/ao/index/search-index.jsonl",
			Lines: bad,
			Query: "compare wc -l vs ao index stats total_entries",
		},
		Remediation: Remediation{
			Command:          "ao doctor --fix --only " + d.ID(),
			ExplainCommand:   "ao doctor explain " + d.ID(),
			AutoFixable:      true,
			EstimatedActions: 1,
		},
	}}, nil
}

// corruptIndexLinesFixer rewrites search-index.jsonl keeping only valid entries.
// The whole original file is backed up by Mutate before the WriteFile overwrite.
type corruptIndexLinesFixer struct{}

func (corruptIndexLinesFixer) ID() string { return "fm-knowledge-corrupt-index-lines" }
func (corruptIndexLinesFixer) Preconditions() []string {
	return []string{
		".agents/ao/index/search-index.jsonl exists and is non-empty",
		"at least one line parses as a valid SearchIndexEntry",
	}
}
func (corruptIndexLinesFixer) WritesTo() []string {
	return []string{".agents/ao/index/search-index.jsonl"}
}
func (corruptIndexLinesFixer) Ops() []string     { return []string{"WriteFile"} }
func (corruptIndexLinesFixer) Reversible() bool  { return true }
func (corruptIndexLinesFixer) Idempotent() bool  { return true }
func (corruptIndexLinesFixer) AutoFixable() bool { return true }

func (f corruptIndexLinesFixer) Fix(ctx *MutateContext, env *DetectEnv, _ []Finding) (FixResult, error) {
	res := FixResult{FixerID: f.ID(), FindingIDs: []string{f.ID()}}
	idx := searchIndexPath(env)
	raw, err := os.ReadFile(idx)
	if err != nil {
		res.Err = fmt.Errorf("doctor: %s: read index: %w", f.ID(), err)
		return res, res.Err
	}
	var kept []string
	dropped := 0
	for _, t := range splitNonEmptyLines(raw) {
		if indexLineValid(t) {
			kept = append(kept, t)
		} else {
			dropped++
		}
	}
	if dropped == 0 {
		res.Fixed = true
		return res, nil
	}
	if len(kept) == 0 {
		res.Err = fmt.Errorf("doctor: %s: every line corrupt; run `ao store rebuild` (refused_unsafe)", f.ID())
		return res, res.Err
	}
	desired := []byte(strings.Join(kept, "\n") + "\n")
	r, err := Mutate(ctx, idx, WriteFile{Content: desired, Mode: 0o600})
	if err != nil {
		res.Err = fmt.Errorf("doctor: %s: rewrite index: %w", f.ID(), err)
		return res, res.Err
	}
	if r.OK {
		res.ActionsTaken = 1
	}
	res.Fixed = true
	return res, nil
}

// ---------------------------------------------------------------------------
// FM: fm-knowledge-torn-append-line (auto-fixable)
// ---------------------------------------------------------------------------

// tornAppendLineDetector flags an interrupted O_APPEND write to the search
// index: a final line that fails to parse AND is not newline-terminated.
type tornAppendLineDetector struct{}

func (tornAppendLineDetector) ID() string           { return "fm-knowledge-torn-append-line" }
func (tornAppendLineDetector) Subsystem() string    { return "knowledge" }
func (tornAppendLineDetector) Severity() string     { return "P2" }
func (tornAppendLineDetector) EstimatedCostMS() int { return 3 }
func (tornAppendLineDetector) OnlineRequired() bool { return false }
func (tornAppendLineDetector) QuickPath() bool      { return false }
func (tornAppendLineDetector) Describe() string {
	return "search-index.jsonl has a torn trailing line from an interrupted append"
}

// indexTorn reports whether raw bytes end in a torn trailing line: the final
// non-empty line fails to parse and there is no terminating newline.
func indexTorn(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}
	if bytes.HasSuffix(raw, []byte("\n")) {
		return false
	}
	lines := splitNonEmptyLines(raw)
	if len(lines) == 0 {
		return false
	}
	return !indexLineValid(lines[len(lines)-1])
}

func (d tornAppendLineDetector) Detect(env *DetectEnv) ([]Finding, error) {
	idx := searchIndexPath(env)
	info, err := os.Stat(idx)
	if err != nil || info.Size() == 0 {
		return nil, nil
	}
	raw, err := os.ReadFile(idx)
	if err != nil {
		return nil, fmt.Errorf("doctor: read search index: %w", err)
	}
	if !indexTorn(raw) {
		return nil, nil
	}
	lines := splitNonEmptyLines(raw)
	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      fmt.Sprintf("torn trailing index line (%d bytes, no terminating newline)", len(lines[len(lines)-1])),
		Confidence: 1.0,
		Evidence: Evidence{
			File:  ".agents/ao/index/search-index.jsonl",
			Lines: []int{len(lines)},
			Query: "tail -c 200 search-index.jsonl | tail -1 | json.loads -> non-zero exit",
		},
		Remediation: Remediation{
			Command:          "ao doctor --fix --only " + d.ID(),
			ExplainCommand:   "ao doctor explain " + d.ID(),
			AutoFixable:      true,
			EstimatedActions: 1,
		},
	}}, nil
}

// tornAppendLineFixer drops the torn trailing line and rewrites the index with
// a clean terminating newline. It refuses if a non-trailing line is also
// corrupt — that is corruptIndexLinesFixer's job, and the engine topo-sorts
// corrupt-index-lines before torn-append-line.
type tornAppendLineFixer struct{}

func (tornAppendLineFixer) ID() string { return "fm-knowledge-torn-append-line" }
func (tornAppendLineFixer) Preconditions() []string {
	return []string{
		".agents/ao/index/search-index.jsonl exists and is non-empty",
		"only the trailing line is torn; no non-trailing line is corrupt",
	}
}
func (tornAppendLineFixer) WritesTo() []string {
	return []string{".agents/ao/index/search-index.jsonl"}
}
func (tornAppendLineFixer) Ops() []string     { return []string{"WriteFile"} }
func (tornAppendLineFixer) Reversible() bool  { return true }
func (tornAppendLineFixer) Idempotent() bool  { return true }
func (tornAppendLineFixer) AutoFixable() bool { return true }

func (f tornAppendLineFixer) Fix(ctx *MutateContext, env *DetectEnv, _ []Finding) (FixResult, error) {
	res := FixResult{FixerID: f.ID(), FindingIDs: []string{f.ID()}}
	idx := searchIndexPath(env)
	raw, err := os.ReadFile(idx)
	if err != nil {
		res.Err = fmt.Errorf("doctor: %s: read index: %w", f.ID(), err)
		return res, res.Err
	}
	endsWithNewline := bytes.HasSuffix(raw, []byte("\n"))
	lines := splitNonEmptyLines(raw)
	var kept []string
	for i, t := range lines {
		isLast := i == len(lines)-1
		switch {
		case indexLineValid(t):
			kept = append(kept, t)
		case isLast && !endsWithNewline:
			// the torn trailing line — drop it
		default:
			res.Err = fmt.Errorf("doctor: %s: non-trailing corruption present; run fm-knowledge-corrupt-index-lines first (refused_unsafe)", f.ID())
			return res, res.Err
		}
	}
	desired := []byte(strings.Join(kept, "\n") + "\n")
	if bytes.Equal(desired, raw) {
		res.Fixed = true
		return res, nil
	}
	r, err := Mutate(ctx, idx, WriteFile{Content: desired, Mode: 0o600})
	if err != nil {
		res.Err = fmt.Errorf("doctor: %s: rewrite index: %w", f.ID(), err)
		return res, res.Err
	}
	if r.OK {
		res.ActionsTaken = 1
	}
	res.Fixed = true
	return res, nil
}

// ---------------------------------------------------------------------------
// FM: fm-knowledge-orphaned-flywheel-learnings (auto-fixable)
// ---------------------------------------------------------------------------

// orphanedFlywheelLearningsDetector flags learnings split across the canonical
// .agents/ao/learnings and the fallback .agents/learnings — a dual-location
// ambiguity that makes CheckFlywheelHealth's headline internally inconsistent.
type orphanedFlywheelLearningsDetector struct{}

func (orphanedFlywheelLearningsDetector) ID() string {
	return "fm-knowledge-orphaned-flywheel-learnings"
}
func (orphanedFlywheelLearningsDetector) Subsystem() string    { return "knowledge" }
func (orphanedFlywheelLearningsDetector) Severity() string     { return "P3" }
func (orphanedFlywheelLearningsDetector) EstimatedCostMS() int { return 3 }
func (orphanedFlywheelLearningsDetector) OnlineRequired() bool { return false }
func (orphanedFlywheelLearningsDetector) QuickPath() bool      { return false }
func (orphanedFlywheelLearningsDetector) Describe() string {
	return "flywheel learnings split across .agents/learnings and .agents/ao/learnings"
}

// listLearningFiles returns the basenames of *.md and *.jsonl files directly
// inside dir, sorted. A missing directory yields an empty slice.
func listLearningFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if indexableExts[ext] {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// flywheelLearningsDirs returns the canonical (primary) and fallback learnings
// directory paths for the workspace.
func flywheelLearningsDirs(env *DetectEnv) (primary, fallback string) {
	primary = filepath.Join(knowledgeBaseDir(env), "learnings")
	fallback = filepath.Join(env.CWD, ".agents", "learnings")
	return primary, fallback
}

func (d orphanedFlywheelLearningsDetector) Detect(env *DetectEnv) ([]Finding, error) {
	primary, fallback := flywheelLearningsDirs(env)
	primaryFiles := listLearningFiles(primary)
	fallbackFiles := listLearningFiles(fallback)
	if len(primaryFiles) == 0 || len(fallbackFiles) == 0 {
		return nil, nil
	}
	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      fmt.Sprintf("learnings split: %d in .agents/learnings, %d in .agents/ao/learnings", len(fallbackFiles), len(primaryFiles)),
		Confidence: 1.0,
		Evidence: Evidence{
			File:  ".agents/learnings",
			Query: "ls .agents/learnings .agents/ao/learnings - both non-empty",
		},
		Remediation: Remediation{
			Command:          "ao doctor --fix --only " + d.ID(),
			ExplainCommand:   "ao doctor explain " + d.ID(),
			AutoFixable:      true,
			EstimatedActions: len(fallbackFiles),
		},
	}}, nil
}

// orphanedFlywheelLearningsFixer consolidates fallback learnings into the
// canonical .agents/ao/learnings directory via Mutate Rename per file. It
// refuses on a basename collision rather than clobber a distinct learning.
type orphanedFlywheelLearningsFixer struct{}

func (orphanedFlywheelLearningsFixer) ID() string {
	return "fm-knowledge-orphaned-flywheel-learnings"
}
func (orphanedFlywheelLearningsFixer) Preconditions() []string {
	return []string{
		"both .agents/learnings and .agents/ao/learnings hold learning files",
		"no basename collision between the two learnings directories",
	}
}
func (orphanedFlywheelLearningsFixer) WritesTo() []string {
	return []string{".agents/learnings", ".agents/ao/learnings"}
}
func (orphanedFlywheelLearningsFixer) Ops() []string     { return []string{"Rename"} }
func (orphanedFlywheelLearningsFixer) Reversible() bool  { return true }
func (orphanedFlywheelLearningsFixer) Idempotent() bool  { return true }
func (orphanedFlywheelLearningsFixer) AutoFixable() bool { return true }

func (f orphanedFlywheelLearningsFixer) Fix(ctx *MutateContext, env *DetectEnv, _ []Finding) (FixResult, error) {
	res := FixResult{FixerID: f.ID(), FindingIDs: []string{f.ID()}}
	primary, fallback := flywheelLearningsDirs(env)
	primaryFiles := listLearningFiles(primary)
	fallbackFiles := listLearningFiles(fallback)
	if len(fallbackFiles) == 0 {
		res.Fixed = true
		return res, nil
	}
	primarySet := make(map[string]bool, len(primaryFiles))
	for _, n := range primaryFiles {
		primarySet[n] = true
	}
	for _, n := range fallbackFiles {
		if primarySet[n] {
			res.Err = fmt.Errorf("doctor: %s: name collision %q in both learnings dirs; resolve manually (refused_unsafe)", f.ID(), n)
			return res, res.Err
		}
	}
	// Ensure the canonical dir exists; if absent, create it through the
	// chokepoint by staging the first move's .gitkeep is unnecessary — Rename's
	// executeAtomic MkdirAll's filepath.Dir(dest) for each moved file.
	for _, n := range fallbackFiles {
		src := filepath.Join(fallback, n)
		dest := filepath.Join(primary, n)
		r, err := Mutate(ctx, src, Rename{To: dest})
		if err != nil {
			res.Err = fmt.Errorf("doctor: %s: move %s: %w", f.ID(), n, err)
			return res, res.Err
		}
		if r.OK {
			res.ActionsTaken++
		}
	}
	if len(listLearningFiles(fallback)) != 0 && !ctx.DryRun {
		res.Err = fmt.Errorf("doctor: %s: fix did not eliminate the finding", f.ID())
		return res, res.Err
	}
	res.Fixed = true
	return res, nil
}

// ---------------------------------------------------------------------------
// FM: fm-knowledge-stale-index-drift (detect-only)
// ---------------------------------------------------------------------------

// staleIndexDriftDetector flags a search index that has drifted from the live
// artifact tree: dead paths, content-mtime drift, and un-indexed files.
type staleIndexDriftDetector struct{}

func (staleIndexDriftDetector) ID() string           { return "fm-knowledge-stale-index-drift" }
func (staleIndexDriftDetector) Subsystem() string    { return "knowledge" }
func (staleIndexDriftDetector) Severity() string     { return "P2" }
func (staleIndexDriftDetector) EstimatedCostMS() int { return 12 }
func (staleIndexDriftDetector) OnlineRequired() bool { return false }
func (staleIndexDriftDetector) QuickPath() bool      { return false }
func (staleIndexDriftDetector) Describe() string {
	return "search index has drifted from on-disk artifacts (dead/drifted/un-indexed)"
}

// indexEntry is the minimal projection of a SearchIndexEntry the drift detector
// needs: the artifact path and the index-time modified timestamp.
type indexEntry struct {
	Path       string `json:"path"`
	ModifiedAt string `json:"modified_at"`
}

// driftCounts holds the three drift classes the stale-index detector reports.
type driftCounts struct {
	dead, contentDrift, unindexed int
}

// computeIndexDrift compares the index against the artifact tree. It is pure:
// stat + read only.
func computeIndexDrift(env *DetectEnv, raw []byte) driftCounts {
	var dc driftCounts
	indexed := make(map[string]bool)
	for _, t := range splitNonEmptyLines(raw) {
		var e indexEntry
		if err := json.Unmarshal([]byte(t), &e); err != nil || e.Path == "" {
			continue
		}
		indexed[e.Path] = true
		abs := filepath.Join(env.CWD, e.Path)
		fi, err := os.Stat(abs)
		if err != nil {
			dc.dead++
			continue
		}
		if mt, perr := parseIndexTime(e.ModifiedAt); perr == nil && fi.ModTime().After(mt) {
			dc.contentDrift++
		}
	}
	for _, sub := range artifactSubdirs {
		dir := filepath.Join(env.CWD, ".agents", sub)
		_ = filepath.WalkDir(dir, func(p string, de os.DirEntry, werr error) error {
			if werr != nil || de.IsDir() {
				return nil //nolint:nilerr // missing artifact dir is benign
			}
			if !indexableExts[strings.ToLower(filepath.Ext(de.Name()))] {
				return nil
			}
			rel, rerr := filepath.Rel(env.CWD, p)
			if rerr == nil && !indexed[rel] {
				dc.unindexed++
			}
			return nil
		})
	}
	return dc
}

func (d staleIndexDriftDetector) Detect(env *DetectEnv) ([]Finding, error) {
	idx := searchIndexPath(env)
	info, err := os.Stat(idx)
	if err != nil || info.Size() == 0 {
		return nil, nil
	}
	raw, err := os.ReadFile(idx)
	if err != nil {
		return nil, fmt.Errorf("doctor: read search index: %w", err)
	}
	dc := computeIndexDrift(env, raw)
	if dc.dead+dc.contentDrift+dc.unindexed == 0 {
		return nil, nil
	}
	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      fmt.Sprintf("%d dead path(s), %d content-drifted, %d un-indexed", dc.dead, dc.contentDrift, dc.unindexed),
		Confidence: 1.0,
		Evidence: Evidence{
			File:  ".agents/ao/index/search-index.jsonl",
			Query: "count .md/.jsonl under .agents artifact dirs vs index total_entries",
		},
		Remediation: Remediation{
			Command:        "ao store rebuild",
			ExplainCommand: "ao doctor explain " + d.ID(),
			AutoFixable:    false,
		},
	}}, nil
}

// staleIndexDriftFixer is a detect-only refuser: there is no safe localized
// auto-fix. The only sound repair is a full `ao store rebuild`, which is a
// heavyweight existing command, not a doctor patch.
type staleIndexDriftFixer struct{}

func (staleIndexDriftFixer) ID() string { return "fm-knowledge-stale-index-drift" }
func (staleIndexDriftFixer) Preconditions() []string {
	return []string{"detect-only: no auto-fix; run `ao store rebuild`"}
}
func (staleIndexDriftFixer) WritesTo() []string { return nil }
func (staleIndexDriftFixer) Ops() []string      { return nil }
func (staleIndexDriftFixer) Reversible() bool   { return true }
func (staleIndexDriftFixer) Idempotent() bool   { return true }
func (staleIndexDriftFixer) AutoFixable() bool  { return false }

func (f staleIndexDriftFixer) Fix(_ *MutateContext, _ *DetectEnv, _ []Finding) (FixResult, error) {
	err := fmt.Errorf("doctor: %s: detect-only — no safe localized auto-fix; run `ao store rebuild` to regenerate the index", f.ID())
	return FixResult{FixerID: f.ID(), FindingIDs: []string{f.ID()}, Fixed: false, Err: err}, err
}

// ---------------------------------------------------------------------------
// FM: fm-knowledge-false-freshness (detect-only)
// ---------------------------------------------------------------------------

// falseFreshnessDetector flags a knowledge store whose freshness signal (max FS
// modtime of sessions/) diverges from the authoritative newest Session.Date.
type falseFreshnessDetector struct{}

func (falseFreshnessDetector) ID() string           { return "fm-knowledge-false-freshness" }
func (falseFreshnessDetector) Subsystem() string    { return "knowledge" }
func (falseFreshnessDetector) Severity() string     { return "P3" }
func (falseFreshnessDetector) EstimatedCostMS() int { return 8 }
func (falseFreshnessDetector) OnlineRequired() bool { return false }
func (falseFreshnessDetector) QuickPath() bool      { return false }
func (falseFreshnessDetector) Describe() string {
	return "knowledge freshness derived from FS modtime diverges from Session.Date"
}

func (d falseFreshnessDetector) Detect(env *DetectEnv) ([]Finding, error) {
	sessionsDir := filepath.Join(knowledgeBaseDir(env), "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil || len(entries) == 0 {
		return nil, nil
	}
	fsNewest, contentNewest, parsedAny := freshnessSignals(sessionsDir, entries)
	fsAge := nowProvider().Sub(fsNewest)
	const skewTolerance = 24 * hour
	falseFresh := fsAge < 14*day && (!parsedAny || absDuration(fsNewest.Sub(contentNewest)) > skewTolerance)
	if !falseFresh {
		return nil, nil
	}
	detail := "sessions/ modtime is recent but no parseable session JSONL exists"
	if parsedAny {
		detail = fmt.Sprintf("FS modtime vs newest Session.Date skew = %s", absDuration(fsNewest.Sub(contentNewest)))
	}
	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      detail,
		Confidence: 0.9,
		Evidence: Evidence{
			File:  ".agents/ao/sessions",
			Query: "compare stat -c %Y of newest sessions/ file vs max session JSONL `date`",
		},
		Remediation: Remediation{
			Command:        "ao forge transcript",
			ExplainCommand: "ao doctor explain " + d.ID(),
			AutoFixable:    false,
		},
	}}, nil
}

// falseFreshnessFixer is a detect-only refuser: the defect is a measurement bug
// in CheckKnowledgeFreshness. There is no safe disk repair — fabricating or
// touching a session file would manufacture a false signal of a different kind.
type falseFreshnessFixer struct{}

func (falseFreshnessFixer) ID() string { return "fm-knowledge-false-freshness" }
func (falseFreshnessFixer) Preconditions() []string {
	return []string{"detect-only: no auto-fix; run `ao forge transcript`"}
}
func (falseFreshnessFixer) WritesTo() []string { return nil }
func (falseFreshnessFixer) Ops() []string      { return nil }
func (falseFreshnessFixer) Reversible() bool   { return true }
func (falseFreshnessFixer) Idempotent() bool   { return true }
func (falseFreshnessFixer) AutoFixable() bool  { return false }

func (f falseFreshnessFixer) Fix(_ *MutateContext, _ *DetectEnv, _ []Finding) (FixResult, error) {
	err := fmt.Errorf("doctor: %s: detect-only — freshness is a measurement bug; run `ao forge transcript` for a real session", f.ID())
	return FixResult{FixerID: f.ID(), FindingIDs: []string{f.ID()}, Fixed: false, Err: err}, err
}
