package overnight

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"
)

// sanitizeIterationID rejects path-traversal and filesystem-unsafe
// characters in iteration identifiers before they are interpolated
// into staging/prev/marker paths. Protects NewCheckpoint and
// RecoverFromCrash against crafted inputs that could escape
// .agents/overnight/. Closes pre-mortem findings S1/S2 from
// the Dream parent plan's Phase 3 vibe.
func sanitizeIterationID(id string) error {
	if id == "" {
		return errors.New("overnight: iterationID is empty")
	}
	if strings.ContainsAny(id, `/\`) {
		return fmt.Errorf("overnight: iterationID %q contains path separators", id)
	}
	if strings.Contains(id, "..") {
		return fmt.Errorf("overnight: iterationID %q contains parent-directory reference", id)
	}
	for _, r := range id {
		if !unicode.IsPrint(r) || unicode.IsSpace(r) {
			return fmt.Errorf("overnight: iterationID %q contains non-printable or whitespace character", id)
		}
	}
	return nil
}

// CheckpointedSubpaths enumerates the only locations under .agents/ that
// Dream's REDUCE stage is permitted to mutate.
//
// Every path is interpreted relative to the repository's .agents/ directory.
// Directories are recursively copied; file entries are checkpointed
// individually. The list is intentionally tiny — Dream's contract pins this
// boundary, and anything not in the list is considered outside Dream's
// jurisdiction. See docs/contracts/dream-run-contract.md.
var CheckpointedSubpaths = []string{
	"learnings",
	"findings",
	"patterns",
	"knowledge",
	"rpi/next-work.jsonl",
}

// markerStateReady indicates a checkpoint has staged and written its marker
// but has not yet completed the live swap. A process crashing in this state
// is recoverable via Rollback (which reverses any partial rename).
const markerStateReady = "READY"

// markerStateDone indicates a checkpoint's live swap has completed cleanly.
// A marker in DONE state is a historical record; Rollback on a DONE marker
// is a no-op on the live tree (the staging dir is still removed).
const markerStateDone = "DONE"

// prevRetention caps the number of .agents.prev.<iter> snapshots retained
// alongside the live tree. Two copies (previous + current) is enough to
// recover from a single bad iteration without growing disk unboundedly.
const prevRetention = 2

// Checkpoint is a two-phase-commit overlay over the bounded subset of
// .agents/ that Dream's REDUCE stage mutates.
//
// Lifecycle:
//
//  1. NewCheckpoint clones the live subpaths into StagingDir under
//     .agents/overnight/staging/<iter>/ and enforces maxBytes.
//  2. Callers mutate files inside StagingDir freely.
//  3. Commit writes a READY marker, swaps live ↔ staging atomically per
//     subpath (saving the displaced live copy under PrevDir), then flips
//     the marker to DONE and rotates old PrevDirs beyond retention.
//  4. Rollback (on error or panic) removes StagingDir outright and, if the
//     marker is READY, reverses any partial live swaps before deleting the
//     marker.
type Checkpoint struct {
	// IterationID is the stable iteration identifier the caller supplied.
	// Used to namespace staging, prev, and marker paths.
	IterationID string

	// StagingDir is the absolute path to the per-iteration staging root.
	// Contents live under StagingDir/.agents/<subpath>/.
	StagingDir string

	// PrevDir is the absolute path that will receive displaced live copies
	// during Commit. Populated on successful swap; empty before Commit.
	PrevDir string

	// LiveDir is the absolute path to the repository's .agents/ directory.
	LiveDir string

	// MarkerPath is the absolute path to the commit marker file. The file
	// is created by Commit and deleted by Rollback.
	MarkerPath string

	// CreatedAt is the wall-clock time NewCheckpoint finished staging.
	CreatedAt time.Time

	// SizeBytes is the total byte count of the staging clone, measured at
	// NewCheckpoint time. Used to enforce the maxBytes budget.
	SizeBytes int64
}

// MetadataIntegrityReport is the result of VerifyMetadataRoundTrip.
//
// A report with Pass=false indicates the REDUCE stage dropped one or more
// learning-file frontmatter keys between staging and live — the precise
// failure mode that pm-005 in the Dream pre-mortem pinpoints. The
// StrippedFields slice lists every (file, key) pair dropped so the caller
// can emit an actionable error.
type MetadataIntegrityReport struct {
	// Pass is true iff no frontmatter keys were dropped between the
	// staging snapshot and the live copy.
	Pass bool

	// StrippedFields enumerates every dropped (file, key) pair. Empty when
	// Pass is true.
	StrippedFields []StrippedField
}

// StrippedField is a single dropped frontmatter key, scoped to the
// learning file it was removed from.
type StrippedField struct {
	// File is the relative path under the learnings subtree
	// (e.g. "learnings/2026-04-09-foo.md").
	File string

	// Key is the frontmatter key that existed in staging but is missing
	// from the live copy.
	Key string
}

// NewCheckpoint stages a fresh checkpoint overlay for iterationID.
//
// It refuses when .agents/ is missing at cwd, and it enforces a hard cap
// of maxBytes on the total staging clone. On exceed the partial staging
// directory is deleted before the error is returned, so failure never
// leaves disk pressure behind. Missing optional subpaths are tolerated:
// if .agents/findings/ doesn't exist, the checkpoint still succeeds and
// the staging tree simply won't contain a findings subtree.
func NewCheckpoint(cwd, iterationID string, maxBytes int64) (*Checkpoint, error) {
	if cwd == "" {
		return nil, errors.New("overnight: NewCheckpoint requires a non-empty cwd")
	}
	if err := sanitizeIterationID(iterationID); err != nil {
		return nil, err
	}
	if maxBytes <= 0 {
		return nil, fmt.Errorf("overnight: NewCheckpoint maxBytes must be positive, got %d", maxBytes)
	}

	liveDir := filepath.Join(cwd, ".agents")
	info, err := os.Stat(liveDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("overnight: .agents/ missing at %s: %w", cwd, err)
		}
		return nil, fmt.Errorf("overnight: stat .agents/: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("overnight: %s is not a directory", liveDir)
	}

	overnightDir := filepath.Join(liveDir, "overnight")
	stagingDir := filepath.Join(overnightDir, "staging", iterationID)
	prevDir := filepath.Join(overnightDir, fmt.Sprintf("prev.%s", iterationID))
	markerPath := filepath.Join(overnightDir, fmt.Sprintf("COMMIT-MARKER.%s", iterationID))

	// Wipe any stale staging tree from a crashed prior run with the same
	// iteration id so the fresh clone starts clean.
	if err := os.RemoveAll(stagingDir); err != nil {
		return nil, fmt.Errorf("overnight: clear stale staging at %s: %w", stagingDir, err)
	}
	if err := os.MkdirAll(filepath.Join(stagingDir, ".agents"), 0o755); err != nil {
		return nil, fmt.Errorf("overnight: mkdir staging root: %w", err)
	}

	var totalBytes int64
	for _, sub := range CheckpointedSubpaths {
		src := filepath.Join(liveDir, sub)
		dst := filepath.Join(stagingDir, ".agents", sub)

		srcInfo, statErr := os.Stat(src)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				// Optional subpath: skip silently.
				continue
			}
			_ = os.RemoveAll(stagingDir)
			return nil, fmt.Errorf("overnight: stat %s: %w", src, statErr)
		}

		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			_ = os.RemoveAll(stagingDir)
			return nil, fmt.Errorf("overnight: mkdir staging parent for %s: %w", sub, err)
		}

		copied, copyErr := copyPath(src, dst, srcInfo, maxBytes-totalBytes)
		totalBytes += copied
		if copyErr != nil {
			_ = os.RemoveAll(stagingDir)
			return nil, fmt.Errorf("overnight: stage %s: %w", sub, copyErr)
		}
		if totalBytes > maxBytes {
			_ = os.RemoveAll(stagingDir)
			return nil, fmt.Errorf("overnight: staging exceeded maxBytes budget (%d > %d)", totalBytes, maxBytes)
		}
	}

	return &Checkpoint{
		IterationID: iterationID,
		StagingDir:  stagingDir,
		PrevDir:     prevDir,
		LiveDir:     liveDir,
		MarkerPath:  markerPath,
		CreatedAt:   time.Now().UTC(),
		SizeBytes:   totalBytes,
	}, nil
}

// Commit atomically swaps staged subpaths into the live .agents/ tree.
//
// Protocol:
//
//  1. Write a READY marker (JSON) so a crash after partial rename is
//     detectable by Rollback.
//  2. For each subpath that exists in staging, move the live copy aside
//     into PrevDir, then rename the staged copy into its live slot.
//  3. Flip the marker to DONE.
//  4. Rotate PrevDirs beyond retention.
//  5. Remove the now-empty staging tree.
//
// Commit is not safe to call concurrently against the same Checkpoint.
// Any error mid-flight leaves the marker in READY state so Rollback can
// reverse the partial swap.
func (cp *Checkpoint) Commit() error {
	if cp == nil {
		return errors.New("overnight: Commit on nil Checkpoint")
	}

	overnightDir := filepath.Dir(cp.PrevDir)
	if err := os.MkdirAll(overnightDir, 0o755); err != nil {
		return fmt.Errorf("overnight: ensure overnight dir: %w", err)
	}

	// Fresh prev dir for this iteration; wipe any leftover.
	if err := os.RemoveAll(cp.PrevDir); err != nil {
		return fmt.Errorf("overnight: clear stale prev dir: %w", err)
	}
	if err := os.MkdirAll(cp.PrevDir, 0o755); err != nil {
		return fmt.Errorf("overnight: mkdir prev dir: %w", err)
	}

	if err := writeMarker(cp.MarkerPath, markerStateReady, cp.IterationID, cp.CreatedAt); err != nil {
		return fmt.Errorf("overnight: write READY marker: %w", err)
	}

	type swap struct {
		sub        string
		prevTarget string // where live was moved to (may be empty if live didn't exist)
		liveHadIt  bool
	}
	var completed []swap

	for _, sub := range CheckpointedSubpaths {
		stagedPath := filepath.Join(cp.StagingDir, ".agents", sub)
		if _, err := os.Stat(stagedPath); err != nil {
			if os.IsNotExist(err) {
				// Subpath was optional and absent in staging; nothing to swap.
				continue
			}
			return fmt.Errorf("overnight: stat staged %s: %w", sub, err)
		}

		livePath := filepath.Join(cp.LiveDir, sub)
		prevPath := filepath.Join(cp.PrevDir, sub)

		if err := os.MkdirAll(filepath.Dir(prevPath), 0o755); err != nil {
			return fmt.Errorf("overnight: mkdir prev parent for %s: %w", sub, err)
		}
		if err := os.MkdirAll(filepath.Dir(livePath), 0o755); err != nil {
			return fmt.Errorf("overnight: mkdir live parent for %s: %w", sub, err)
		}

		liveHadIt := false
		if _, err := os.Stat(livePath); err == nil {
			if renameErr := os.Rename(livePath, prevPath); renameErr != nil {
				return fmt.Errorf("overnight: move live %s to prev: %w", sub, renameErr)
			}
			liveHadIt = true
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("overnight: stat live %s: %w", sub, err)
		}

		if err := os.Rename(stagedPath, livePath); err != nil {
			// Best-effort: restore the live copy we just moved aside so
			// the live tree isn't left with a missing subpath.
			if liveHadIt {
				_ = os.Rename(prevPath, livePath)
			}
			return fmt.Errorf("overnight: rename staged %s into live: %w", sub, err)
		}

		completed = append(completed, swap{sub: sub, prevTarget: prevPath, liveHadIt: liveHadIt})
	}

	if err := writeMarker(cp.MarkerPath, markerStateDone, cp.IterationID, cp.CreatedAt); err != nil {
		return fmt.Errorf("overnight: write DONE marker: %w", err)
	}

	// Rotate old prev dirs. Best-effort: rotation failures are logged via
	// the returned error on the next iteration, not fatal here.
	_ = rotatePrevDirs(overnightDir, prevRetention)

	// Drop the now-empty staging tree.
	if err := os.RemoveAll(cp.StagingDir); err != nil {
		return fmt.Errorf("overnight: remove staging after commit: %w", err)
	}

	_ = completed // retained for symmetry / future crash-recovery logic
	return nil
}

// Rollback removes the staging tree and, if a READY marker is present,
// reverses any partial live swaps using PrevDir.
//
// Rollback is safe to call in any state (pre-commit, mid-commit, post-commit).
// After a successful Commit the marker is DONE and Rollback only removes the
// marker and staging dir without touching the live tree.
func (cp *Checkpoint) Rollback() error {
	if cp == nil {
		return errors.New("overnight: Rollback on nil Checkpoint")
	}

	// Always drop the staging dir first — it cannot be useful after
	// rollback regardless of marker state.
	if err := os.RemoveAll(cp.StagingDir); err != nil {
		return fmt.Errorf("overnight: remove staging: %w", err)
	}

	state, _ := readMarkerState(cp.MarkerPath)
	if state == markerStateReady {
		// Partial commit: reverse each swap by moving prev back into live.
		for _, sub := range CheckpointedSubpaths {
			prevPath := filepath.Join(cp.PrevDir, sub)
			livePath := filepath.Join(cp.LiveDir, sub)
			if _, err := os.Stat(prevPath); err != nil {
				continue
			}
			// Remove any half-committed live copy before restoring prev.
			_ = os.RemoveAll(livePath)
			if err := os.MkdirAll(filepath.Dir(livePath), 0o755); err != nil {
				return fmt.Errorf("overnight: mkdir live parent for %s during rollback: %w", sub, err)
			}
			if err := os.Rename(prevPath, livePath); err != nil {
				return fmt.Errorf("overnight: restore %s from prev: %w", sub, err)
			}
		}
		// Clean up the emptied prev dir.
		_ = os.RemoveAll(cp.PrevDir)
	}

	if err := os.Remove(cp.MarkerPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("overnight: remove marker: %w", err)
	}
	return nil
}

// VerifyMetadataRoundTrip checks that every learning-file frontmatter key
// present in the LIVE (pre-REDUCE) snapshot still exists in the STAGING
// (post-REDUCE) tree. This is the pm-005 regression guard: a reducer that
// strips frontmatter fields it doesn't know about will fail this check.
//
// Walk direction: live → staging. REDUCE mutates the staging copy (per
// the V1 fix from Phase 3 vibe), so the comparison asks "did any key
// from the pristine baseline survive into the reducer's output?" A key
// in LIVE but missing from STAGING is a silent strip. Extra keys in
// STAGING that aren't in LIVE (e.g., harvest-promote imported a new
// file) are additions, NOT strips, and are intentionally ignored.
//
// Files that exist in LIVE but are entirely missing from STAGING are
// treated as LEGITIMATE DELETIONS (defrag-prune, dedup merge). They are
// not flagged; only in-place key strips on files that exist in both
// trees count as failures.
//
// The check computes a pure set-difference over ALL top-level frontmatter
// keys — it intentionally does NOT use a hardcoded allowlist, because any
// such allowlist is exactly the bug pm-005 warns about.
//
// A missing learnings subtree on either side is not treated as a failure
// (the check simply has nothing to compare); parse errors on individual
// files are tolerated as well, on the principle that a best-effort report
// beats a hard abort when one learning file has malformed YAML.
func VerifyMetadataRoundTrip(cp *Checkpoint) MetadataIntegrityReport {
	report := MetadataIntegrityReport{Pass: true}
	if cp == nil {
		return report
	}

	stagedRoot := filepath.Join(cp.StagingDir, ".agents", "learnings")
	liveRoot := filepath.Join(cp.LiveDir, "learnings")

	liveMeta := collectFrontmatter(liveRoot)
	if len(liveMeta) == 0 {
		return report
	}
	stagedMeta := collectFrontmatter(stagedRoot)

	// Stable iteration for deterministic report ordering.
	files := make([]string, 0, len(liveMeta))
	for f := range liveMeta {
		files = append(files, f)
	}
	sort.Strings(files)

	for _, rel := range files {
		liveKeys := liveMeta[rel]
		stagedKeys, ok := stagedMeta[rel]
		if !ok {
			// File was legitimately deleted by a REDUCE stage (e.g.,
			// defrag-prune removing stale entries, dedup merging
			// duplicates). Not a metadata strip.
			continue
		}
		for _, key := range sortedKeys(liveKeys) {
			if _, present := stagedKeys[key]; !present {
				report.StrippedFields = append(report.StrippedFields, StrippedField{
					File: filepath.ToSlash(filepath.Join("learnings", rel)),
					Key:  key,
				})
			}
		}
	}

	report.Pass = len(report.StrippedFields) == 0
	return report
}

// --- internal helpers ---------------------------------------------------

// copyPath recursively copies src to dst, returning the total bytes
// written. The budget parameter is the remaining byte allowance; when
// copyPath detects it has exceeded the budget it aborts early with an
// error so NewCheckpoint can clean up before returning.
func copyPath(src, dst string, srcInfo os.FileInfo, budget int64) (int64, error) {
	var total int64

	if !srcInfo.IsDir() {
		n, err := copyFile(src, dst, srcInfo.Mode())
		return n, err
	}

	err := filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return relErr
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		n, err := copyFile(path, target, info.Mode())
		total += n
		if err != nil {
			return err
		}
		if budget > 0 && total > budget {
			return fmt.Errorf("overnight: copy budget exceeded at %s", rel)
		}
		return nil
	})
	return total, err
}

type checkpointCloneFileFunc func(src, dst string, mode os.FileMode) (bytesWritten int64, cloned bool, err error)

var checkpointCloneFile checkpointCloneFileFunc = cloneFileForCheckpoint

// copyFile copies a single file, preserving mode. Returns bytes written.
func copyFile(src, dst string, mode os.FileMode) (int64, error) {
	if n, cloned, err := checkpointCloneFile(src, dst, mode); cloned || err != nil {
		return n, err
	}
	return copyFileBytes(src, dst, mode)
}

func copyFileBytes(src, dst string, mode os.FileMode) (int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer func() { _ = in.Close() }()

	// Ensure regular permissions at minimum; preserve src mode bits.
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode.Perm())
	if err != nil {
		return 0, err
	}
	n, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return n, copyErr
	}
	if closeErr != nil {
		return n, closeErr
	}
	return n, nil
}

// markerBody is the JSON schema of COMMIT-MARKER files.
type markerBody struct {
	State       string `json:"state"`
	IterationID string `json:"iteration_id"`
	StartedAt   string `json:"started_at"`
}

// writeMarker writes a commit marker file atomically via rename-from-temp.
func writeMarker(path, state, iterationID string, startedAt time.Time) error {
	body := markerBody{
		State:       state,
		IterationID: iterationID,
		StartedAt:   startedAt.UTC().Format(time.RFC3339Nano),
	}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// readMarkerState returns the marker state string, or empty if the marker
// is missing or malformed.
func readMarkerState(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var body markerBody
	if err := json.Unmarshal(data, &body); err != nil {
		return "", err
	}
	return body.State, nil
}

// rotatePrevDirs deletes prev.* snapshots beyond retention, oldest first.
func rotatePrevDirs(overnightDir string, retention int) error {
	entries, err := os.ReadDir(overnightDir)
	if err != nil {
		return err
	}
	type prevEntry struct {
		name    string
		modTime time.Time
	}
	var prevs []prevEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if !strings.HasPrefix(e.Name(), "prev.") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		prevs = append(prevs, prevEntry{name: e.Name(), modTime: info.ModTime()})
	}
	if len(prevs) <= retention {
		return nil
	}
	sort.Slice(prevs, func(i, j int) bool {
		return prevs[i].modTime.Before(prevs[j].modTime)
	})
	toRemove := len(prevs) - retention
	for i := 0; i < toRemove; i++ {
		_ = os.RemoveAll(filepath.Join(overnightDir, prevs[i].name))
	}
	return nil
}

// collectFrontmatter walks root, parsing frontmatter out of every .md file
// it finds, and returns a map of relative-path → set-of-keys.
func collectFrontmatter(root string) map[string]map[string]struct{} {
	out := make(map[string]map[string]struct{})
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return out
	}
	_ = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		keys := parseFrontmatterKeys(path)
		if keys == nil {
			return nil
		}
		out[filepath.ToSlash(rel)] = keys
		return nil
	})
	return out
}

// parseFrontmatterKeys reads the leading YAML frontmatter block (delimited
// by --- lines) from a markdown file and returns its top-level keys.
// Returns an empty set if the file has no frontmatter; returns nil only on
// read errors.
func parseFrontmatterKeys(path string) map[string]struct{} {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	keys := map[string]struct{}{}
	delim := []byte("---")
	// Trim leading whitespace/BOM-ish bytes; we require the file to start
	// with --- to be considered as having frontmatter.
	trimmed := bytes.TrimLeft(data, "\r\n ")
	if !bytes.HasPrefix(trimmed, delim) {
		return keys
	}
	// Drop the opening delimiter line.
	rest := trimmed[len(delim):]
	if idx := bytes.IndexByte(rest, '\n'); idx >= 0 {
		rest = rest[idx+1:]
	} else {
		return keys
	}
	// Find the closing delimiter (line starting with ---).
	closeIdx := bytes.Index(rest, []byte("\n---"))
	var block []byte
	if closeIdx < 0 {
		// No closing delim — treat the remainder as the block.
		block = rest
	} else {
		block = rest[:closeIdx]
	}
	var parsed map[string]any
	if err := yaml.Unmarshal(block, &parsed); err != nil {
		return keys
	}
	for k := range parsed {
		keys[k] = struct{}{}
	}
	return keys
}

// sortedKeys returns a stable slice of the keys in a set.
func sortedKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// VerifyMetadataRoundTripPostCommit checks that every learning-file
// frontmatter key present in the pre-REDUCE baseline (cp.PrevDir —
// the backup taken during Commit's swap) still exists in the
// post-Commit live tree. This is the Wave 4 Issue 8 / pm-V7 fix:
// the pre-commit VerifyMetadataRoundTrip compares staging vs live
// BEFORE the swap, so it catches reducer strips at the staging
// write layer. This function runs AFTER Commit and catches any
// late-stage corruption in the swap itself (e.g., a partial rename
// or a filesystem quirk that dropped content).
//
// Ratchet-forward semantics: failure here cannot unwind the commit
// (it already landed). The caller (RunLoop) logs a findings entry
// for the morning report and continues the iteration loop. The
// next iteration's pre-commit check will catch any cascading
// corruption.
//
// Walk direction: PrevDir (baseline) → LiveDir (post-promote).
// A key in PrevDir but missing from LiveDir is a silent strip.
// Extra keys in LiveDir that aren't in PrevDir are additions
// (e.g., harvest-promote imported a new file) — NOT strips,
// ignored.
//
// Legitimate deletions (defrag-prune, dedup merge) remove files
// entirely from LiveDir. Those are not flagged; only in-place key
// strips on files that exist in both trees count as failures.
func VerifyMetadataRoundTripPostCommit(cp *Checkpoint) MetadataIntegrityReport {
	report := MetadataIntegrityReport{Pass: true}
	if cp == nil {
		return report
	}

	prevRoot := filepath.Join(cp.PrevDir, "learnings")
	liveRoot := filepath.Join(cp.LiveDir, "learnings")

	prevMeta := collectFrontmatter(prevRoot)
	if len(prevMeta) == 0 {
		// No baseline to compare against (first iteration, or
		// no learnings subdir in the baseline). Trivially passes.
		return report
	}
	liveMeta := collectFrontmatter(liveRoot)

	// Stable iteration for deterministic report ordering.
	files := make([]string, 0, len(prevMeta))
	for f := range prevMeta {
		files = append(files, f)
	}
	sort.Strings(files)

	for _, rel := range files {
		prevKeys := prevMeta[rel]
		liveKeys, ok := liveMeta[rel]
		if !ok {
			// File was legitimately deleted by a REDUCE stage.
			// Not a metadata strip.
			continue
		}
		for _, key := range sortedKeys(prevKeys) {
			if _, present := liveKeys[key]; !present {
				report.StrippedFields = append(report.StrippedFields, StrippedField{
					File: filepath.ToSlash(filepath.Join("learnings", rel)),
					Key:  key,
				})
			}
		}
	}

	report.Pass = len(report.StrippedFields) == 0
	return report
}
