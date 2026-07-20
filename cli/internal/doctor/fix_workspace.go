package doctor

// Workspace subsystem shared base.
//
// This file holds the helpers the workspace hygiene detectors and fixers are
// built on: the `.agents` root resolver, a symlink-safe directory inventory,
// the canonical-alias registry for drifted directory spellings, stale/retry
// name classification, the quarantine Rename destination, and the GC TTL.
// It registers NO detectors and NO fixers; those live in sibling files.
//
// Helpers here are PURE reads (stat + readdir only). Any future fixer disk
// write flows through Mutate; "deletion" of a workspace directory is a Rename
// into the run's quarantine/ directory (see workspaceQuarantineDest), never a
// remove.

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"
)

// subsystemWorkspace is the Subsystem() name shared by all workspace
// detectors and fixers.
const subsystemWorkspace = "workspace"

// workspaceAgentsDir returns the workspace hygiene root, <RepoRoot>/.agents.
func workspaceAgentsDir(env *DetectEnv) string {
	return filepath.Join(env.RepoRoot, ".agents")
}

// workspaceRealDir Lstats path and reports whether it is a REAL directory:
// present, and not a symlink (even a symlink to a directory) or any other
// non-directory entry. Every workspace detector that consumes the `.agents`
// root must gate on this before reading it: ReadDir/WalkDir on a symlinked
// root silently traverse the symlink target, so detectors would inventory —
// and fixers would then rename — directories OUTSIDE the repository while the
// lexical EnsureInScope check still passes.
func workspaceRealDir(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink == 0 && info.IsDir()
}

// workspaceRequireRealAgentsDir is the fixer-side counterpart of
// workspaceRealDir for the `.agents` root: an absent root reports
// (false, nil) — nothing to fix, a no-op success for idempotent fixers —
// while a root that exists but is not a real directory (symlink, regular
// file, ...) refuses with a refused_unsafe error so no fixer ever mutates
// through it.
func workspaceRequireRealAgentsDir(base string) (exists bool, err error) {
	info, lerr := os.Lstat(base)
	if lerr != nil {
		if os.IsNotExist(lerr) {
			return false, nil
		}
		return false, fmt.Errorf("doctor: lstat workspace root %s: %w", base, lerr)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return false, fmt.Errorf("doctor: workspace root %s is not a real directory (refused_unsafe)", base)
	}
	return true, nil
}

// workspaceDirInfo describes one immediate subdirectory of the workspace root.
type workspaceDirInfo struct {
	// Name is the directory's base name.
	Name string
	// FileCount is the transitive count of regular files under the directory.
	FileCount int
	// ByteSize is the transitive sum of regular-file sizes in bytes.
	ByteSize int64
	// NewestMTime is the newest regular-file modification time under the
	// directory; zero when the directory contains no regular files.
	NewestMTime time.Time
	// OtherEntries is the transitive count of entries that are neither
	// regular files nor directories (symlinks, sockets, FIFOs, ...). A dir
	// with FileCount==0 but OtherEntries>0 is NOT empty.
	OtherEntries int
	// WalkErrs is the count of subpaths that could not be read (permission
	// errors, races, stat failures). WalkErrs>0 means the inventory is
	// incomplete: FileCount/ByteSize/NewestMTime are lower bounds, so
	// consumers must treat the directory's content as UNKNOWN, never as
	// provably empty or provably expired.
	WalkErrs int
}

// workspaceDirInventory inventories the immediate subdirectories of base,
// returning one workspaceDirInfo per directory in deterministic name order.
// Symlinks are never followed: a top-level symlink (even to a directory) is
// not inventoried, and symlinks encountered during the walk count only
// toward OtherEntries. Unreadable entries are skipped, not fatal — but each
// skip is tallied in WalkErrs so consumers can tell "empty" from "could not
// read". A missing or unreadable base itself is the only error case.
func workspaceDirInventory(base string) ([]workspaceDirInfo, error) {
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, err
	}
	out := make([]workspaceDirInfo, 0, len(entries))
	for _, e := range entries {
		// os.ReadDir reports lstat-style types, so a symlink to a directory
		// is ModeSymlink here (not IsDir) and gets skipped — symlink-safe.
		if !e.IsDir() {
			continue
		}
		info := workspaceDirInfo{Name: e.Name()}
		// WalkDir never follows symlinks; the error callback path tolerates
		// permission failures by skipping the offending subtree, but every
		// skipped subpath is counted in WalkErrs (unknown ≠ empty).
		_ = filepath.WalkDir(filepath.Join(base, e.Name()), func(_ string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				info.WalkErrs++
				if d != nil && d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if !d.Type().IsRegular() {
				// Symlink, socket, FIFO, device, ...: invisible to the
				// regular-file tallies but NOT ignorable content.
				info.OtherEntries++
				return nil
			}
			fi, infoErr := d.Info()
			if infoErr != nil {
				info.WalkErrs++
				return nil
			}
			info.FileCount++
			info.ByteSize += fi.Size()
			if fi.ModTime().After(info.NewestMTime) {
				info.NewestMTime = fi.ModTime()
			}
			return nil
		})
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// workspaceCanonicalAliases maps drifted spellings of top-level `.agents`
// directory names to their canonical names. The drift detector consumes this
// table verbatim: a key present as a top-level directory is a drift finding
// whose remediation is a merge/rename into the value directory.
var workspaceCanonicalAliases = map[string]string{
	"post-mortem":      "postmortem",
	"post-mortems":     "postmortem",
	"pre-mortem":       "pre-mortem-checks",
	"pre-mortems":      "pre-mortem-checks",
	"premortem-checks": "pre-mortem-checks",
	"handoffs":         "handoff",
	"mto-handoff":      "handoff",
	"retros":           "retro",
	"proof":            "proofs",
	"test":             "tests",
}

// workspaceStaleSuffixRe matches an explicit stale marker: a trailing
// `.stale-<timestamp>` suffix as written by land-queue staling
// (e.g. `land-queue-age-h433.19.stale-20260711T221032Z`).
var workspaceStaleSuffixRe = regexp.MustCompile(`\.stale-\d{8}T\d{6}Z$`)

// workspaceRetryChainRe matches a retry-chain directory: a trailing
// `-retry<N>` suffix, optionally itself staled
// (e.g. `land-queue-age-h433.22-native-retry2`).
var workspaceRetryChainRe = regexp.MustCompile(`-retry\d+(\.stale-\d{8}T\d{6}Z)?$`)

// workspaceStaleNameKind classifies a top-level workspace directory name.
// explicitStale reports a `.stale-<timestamp>` suffix; retryChain reports a
// `-retry<N>` suffix (possibly combined with a stale suffix). Both can be
// true at once for a staled retry directory.
func workspaceStaleNameKind(name string) (explicitStale, retryChain bool) {
	return workspaceStaleSuffixRe.MatchString(name), workspaceRetryChainRe.MatchString(name)
}

// isWorkspaceStaleDirName reports whether a top-level directory name is
// GC-eligible debris: explicitly staled, a retry-chain dir, or both.
func isWorkspaceStaleDirName(name string) bool {
	explicitStale, retryChain := workspaceStaleNameKind(name)
	return explicitStale || retryChain
}

// workspaceQuarantineDest returns the Rename target for quarantining a
// top-level workspace directory: <RunDir>/quarantine/workspace/<dirName>.
// This mirrors the run-quarantine scheme every other fixer uses (paths under
// <RunDir>/quarantine/, cf. RunArtifact.QuarantineDir and the knowledge
// fixer's staged-mkdir path): there is no deletion op, so a workspace GC
// fixer "deletes" via Mutate with Rename{To: workspaceQuarantineDest(...)}.
// Rename's executeAtomic MkdirAll's the destination parent.
func workspaceQuarantineDest(ctx *MutateContext, dirName string) string {
	return filepath.Join(ctx.RunDir, "quarantine", subsystemWorkspace, dirName)
}

// Workspace GC TTL configuration.
const (
	// workspaceTTLEnvVar overrides the workspace GC TTL, in whole days.
	workspaceTTLEnvVar = "AO_DOCTOR_WS_TTL_DAYS"
	// workspaceTTLDefaultDays is the default workspace GC TTL in days.
	workspaceTTLDefaultDays = 14
	// workspaceTTLMaxDays caps the env override at 10 years. The cap is a
	// correctness guard, not just taste: an unbounded day count overflows
	// time.Duration (day counts above ~106751 wrap the int64 nanosecond
	// representation negative), which would push the GC cutoff into the far
	// FUTURE and make every stale-named directory look expired at once.
	workspaceTTLMaxDays = 3650
)

// workspaceGCTTL returns the workspace GC TTL: stale/retry directories whose
// newest content is older than this are GC candidates. Defaults to 14 days;
// AO_DOCTOR_WS_TTL_DAYS overrides with an integer day count in
// [1, workspaceTTLMaxDays], and any invalid or out-of-range value falls back
// to the default (never trusted — see workspaceTTLMaxDays for why).
func workspaceGCTTL() time.Duration {
	if raw := os.Getenv(workspaceTTLEnvVar); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 1 && n <= workspaceTTLMaxDays {
			return time.Duration(n) * day
		}
	}
	return workspaceTTLDefaultDays * day
}

// workspaceDirRename is the single directory analogue of Mutate for the
// Rename op, shared by every workspace fixer. Mutate itself is file-shaped —
// its before-hash read and verbatim backup only work on regular files, so a
// directory rename adapts its eight-step discipline here: the same per-path
// advisory lock, the same EnsureInScope/EnsureOpAllowed preconditions (source
// AND destination; `.agents` is a canonical write scope), the same dry-run
// transparency, the same atomic execute via the chokepoint's executeAtomic
// (which MkdirAll's the destination parent), and the same fsync'd
// actions.jsonl record.
//
// A directory Rename needs no byte backup or content hash to be reversible:
// engine.undoOne reverses a Rename record with a bare rename-back and never
// consults backups or hashes, and the renamed tree IS the preserved copy,
// byte for byte. Hash fields record the empty-content hash for journal-shape
// consistency (the same value Mutate records for an absent file).
//
// verify, when non-nil, runs under the per-path lock after the directory
// check and before any preconditions — fixers use it to re-validate state
// that may have changed since their scan (e.g. "still empty"). A verify
// error aborts the rename with no mutation.
//
// TOCTOU bound (external writers): the per-path lock is in-process advisory
// only, so for a writer OUTSIDE this process a verify→rename window exists
// and cannot be eliminated here — full closure would need filesystem
// transactions or stop-the-world exclusivity, both out of scope by design.
// The consequence is bounded, not destructive: content that lands inside the
// directory during the window is MOVED with it to the journaled rename
// destination (quarantine), remains restorable via `doctor undo`, and is
// never deleted.
func workspaceDirRename(ctx *MutateContext, path, dest string, verify func(path string) error) error {
	op := Rename{To: dest}

	// Step 1 — per-path advisory lock (skipped in dry-run, matching Mutate).
	if !ctx.DryRun {
		guard, err := ctx.Locks.Acquire(path)
		if err != nil {
			return err
		}
		defer func() { _ = guard.Release() }()
	}

	// Step 2 — before-state: the source must exist and be a directory (never
	// follow a symlink into pretending it is one).
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("doctor: lstat %s: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("doctor: rename-dir %s: not a directory (refused_unsafe)", path)
	}
	if verify != nil {
		if err := verify(path); err != nil {
			return err
		}
	}

	// Step 3 — preconditions: both endpoints in scope, op executable.
	if err := EnsureInScope(ctx.Capabilities, ctx.RepoRoot, ctx.HomeDir, path); err != nil {
		return err
	}
	if err := EnsureInScope(ctx.Capabilities, ctx.RepoRoot, ctx.HomeDir, dest); err != nil {
		return err
	}
	if err := EnsureOpAllowed(ctx.Capabilities, op); err != nil {
		return err
	}

	// Step 4 — backup: intentionally none (see the function comment).

	// Step 5/6 — dry-run transparency, then atomic execute.
	startedNS := time.Since(ctx.start).Nanoseconds()
	if ctx.DryRun {
		fmt.Fprintf(os.Stderr, "[dry-run] would mutate %s: %s\n", path, DescribeOp(op))
		return nil
	}
	if err := executeAtomic(path, op); err != nil {
		return fmt.Errorf("doctor: execute Rename on %s: %w", path, err)
	}

	// Step 7/8 — fsync'd action record.
	//
	// Execute-before-journal parity: this order matches the file-shaped Mutate
	// chokepoint (execute, then journal). A CRASH between the execute above
	// and the append below is therefore undo-blind for BOTH shapes — the
	// journal never sees the mutation — and the rename destination tree
	// (quarantine) is the manual recovery path: the run dir's receipts list
	// what moved, and nothing is ever deleted.
	rel, relErr := filepath.Rel(ctx.RepoRoot, path)
	if relErr != nil {
		rel = path
	}
	emptyHash := sha256Hex(nil)
	if wrote, aerr := workspaceAppendAction(ctx, ActionRecord{
		Path:         rel,
		Op:           op.kind(),
		BeforeHash:   emptyHash,
		AfterHash:    emptyHash,
		BeforeMode:   fmt.Sprintf("%o", info.Mode().Perm()),
		StartedAtNS:  startedNS,
		FinishedAtNS: time.Since(ctx.start).Nanoseconds(),
		RunID:        ctx.RunID,
		FixerID:      ctx.FixerID,
		OK:           true,
		RenameTo:     dest,
		Existed:      true,
	}); aerr != nil {
		if !wrote {
			// WRITE-stage failure: the record definitely did not land as a
			// parseable journal line, so `doctor undo` would be blind to the
			// rename. Compensate with a rename-back so disk state matches the
			// (empty) journal, and report both the journal error and the
			// compensation outcome.
			if backErr := os.Rename(dest, path); backErr != nil {
				return fmt.Errorf("doctor: journal Rename of %s: %w; compensating rename-back FAILED (%w) — directory left at %s and is NOT recorded in actions.jsonl", path, aerr, backErr, dest)
			}
			return fmt.Errorf("doctor: journal Rename of %s: %w (compensated: directory renamed back to its original path; no mutation recorded)", path, aerr)
		}
		// SYNC-stage failure: the write succeeded and only the fsync failed,
		// so the record has PROBABLY been persisted. Compensating here would
		// desync disk from the journal — a later `doctor undo` would replay an
		// unnecessary reverse rename over the restored path. Leave the rename
		// in place (state and record are consistent) and surface the
		// journal-durability uncertainty instead.
		return fmt.Errorf("doctor: journal Rename of %s: record written but not durably synced (%w) — rename left in place at %s; actions.jsonl durability is uncertain until the next successful sync", path, aerr, dest)
	}
	return nil
}

// workspaceAppendAction is the journal-append seam used by the workspace
// directory/file chokepoint adapters. It is a package var ONLY so tests can
// simulate the write-succeeded/fsync-failed stage deterministically (forcing a
// real fsync failure while the write succeeds is not portable); production
// always points at workspaceAppendActionStaged.
var workspaceAppendAction = workspaceAppendActionStaged

// workspaceAppendActionStaged appends one action record to the run's
// actions.jsonl exactly as MutateContext.appendAction does (same mutex, same
// handle, same fsync) but reports WHICH stage failed, so callers can pick the
// right recovery:
//
//   - wrote == false: the record was definitely not persisted as a parseable
//     line (marshal or write failed; at worst a partial, unparseable trailing
//     fragment reached the file, which the journal reader rejects). The
//     mutation is journal-invisible — compensating is safe and correct.
//   - wrote == true with a non-nil error: the write succeeded and only the
//     fsync failed. The line is in the OS page cache and has probably been (or
//     will be) persisted, so the caller must NOT compensate by reversing the
//     mutation — the on-disk journal likely records it.
func workspaceAppendActionStaged(ctx *MutateContext, rec ActionRecord) (wrote bool, err error) {
	line, merr := json.Marshal(rec)
	if merr != nil {
		return false, fmt.Errorf("doctor: marshal action record: %w", merr)
	}
	line = append(line, '\n')
	ctx.actionsMu.Lock()
	defer ctx.actionsMu.Unlock()
	if _, werr := ctx.actionsFile.Write(line); werr != nil {
		return false, fmt.Errorf("doctor: append actions.jsonl: %w", werr)
	}
	if serr := ctx.actionsFile.Sync(); serr != nil {
		return true, fmt.Errorf("doctor: sync actions.jsonl: %w", serr)
	}
	return true, nil
}

// workspaceFileMoveNoClobber moves the regular file path to dest through the
// Mutate eight-step discipline (per-path lock, before-hash, scope/op
// preconditions on both endpoints, verbatim backup, dry-run transparency,
// journal) but executes via os.Link + os.Remove instead of os.Rename:
// link(2) fails with EEXIST when the destination exists, so link+remove is
// the no-clobber implementation of Rename for regular files — the kernel
// itself refuses the overwrite, closing the lstat→rename destination race
// that a check-then-rename sequence merely narrows. An EEXIST is reported as
// (collided=true, nil) with nothing moved.
//
// The action is journaled with the same record shape Mutate writes for a
// Rename (Op "Rename", RenameTo=dest, before-hash of the source content,
// empty after-hash at the vacated source path): it IS the move it performed,
// only executed differently. engine.undoOne replays a Rename record with a
// reverse os.Rename(RenameTo, Path), which works identically after
// link+remove — the source is gone and the destination exists, exactly as
// after a plain rename.
func workspaceFileMoveNoClobber(ctx *MutateContext, path, dest string) (collided bool, err error) {
	op := Rename{To: dest}

	// Step 1 — per-path advisory lock on the source (callers moving toward a
	// contended destination hold the destination lock; distinct paths, so no
	// self-deadlock).
	if !ctx.DryRun {
		guard, lerr := ctx.Locks.Acquire(path)
		if lerr != nil {
			return false, lerr
		}
		defer func() { _ = guard.Release() }()
	}

	// Step 2 — before-state: the source must be a regular file (never follow
	// a symlink into pretending it is one).
	info, lerr := os.Lstat(path)
	if lerr != nil {
		return false, fmt.Errorf("doctor: lstat %s: %w", path, lerr)
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("doctor: move %s: not a regular file (refused_unsafe)", path)
	}
	beforeBytes, rerr := os.ReadFile(path)
	if rerr != nil {
		return false, fmt.Errorf("doctor: read %s: %w", path, rerr)
	}
	beforeHash := sha256Hex(beforeBytes)

	// Step 3 — preconditions: both endpoints in scope, op executable.
	if err := EnsureInScope(ctx.Capabilities, ctx.RepoRoot, ctx.HomeDir, path); err != nil {
		return false, err
	}
	if err := EnsureInScope(ctx.Capabilities, ctx.RepoRoot, ctx.HomeDir, dest); err != nil {
		return false, err
	}
	if err := EnsureOpAllowed(ctx.Capabilities, op); err != nil {
		return false, err
	}

	// Step 4 — verbatim backup (same as Mutate for an existing file).
	if !ctx.DryRun {
		rel, relErr := filepath.Rel(ctx.RepoRoot, path)
		if relErr != nil {
			rel = filepath.Base(path)
		}
		backup := filepath.Join(ctx.RunDir, "backups", rel)
		if err := copyVerbatim(path, backup); err != nil {
			return false, fmt.Errorf("doctor: backup %s: %w", path, err)
		}
		if err := cmpStrict(path, backup); err != nil {
			return false, err
		}
	}

	// Step 5/6 — dry-run transparency, then atomic no-clobber execute.
	startedNS := time.Since(ctx.start).Nanoseconds()
	if ctx.DryRun {
		fmt.Fprintf(os.Stderr, "[dry-run] would mutate %s: %s\n", path, DescribeOp(op))
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return false, fmt.Errorf("doctor: mkdir %s: %w", filepath.Dir(dest), err)
	}
	if err := os.Link(path, dest); err != nil {
		if os.IsExist(err) {
			return true, nil // destination appeared — collision, nothing moved
		}
		return false, fmt.Errorf("doctor: link %s -> %s: %w", path, dest, err)
	}
	if err := os.Remove(path); err != nil {
		// The link landed but the source could not be unlinked, leaving two
		// paths to one inode. Compensate by removing the new link so the move
		// stays all-or-nothing.
		if unlinkErr := os.Remove(dest); unlinkErr != nil {
			return false, fmt.Errorf("doctor: remove source %s after link: %w; compensating removal of %s ALSO failed (%w) — file is hard-linked at both paths and NOT recorded in actions.jsonl", path, err, dest, unlinkErr)
		}
		return false, fmt.Errorf("doctor: remove source %s after link: %w (compensated: link at %s removed; nothing moved)", path, err, dest)
	}

	// Step 7/8 — fsync'd action record; same execute-before-journal parity and
	// crash exposure as workspaceDirRename (see the comment there), and the
	// same staged write/sync recovery split.
	rel, relErr := filepath.Rel(ctx.RepoRoot, path)
	if relErr != nil {
		rel = path
	}
	if wrote, aerr := workspaceAppendAction(ctx, ActionRecord{
		Path:         rel,
		Op:           op.kind(),
		BeforeHash:   beforeHash,
		AfterHash:    sha256Hex(nil), // the source path is empty after the move, matching Mutate's read-back
		BeforeMode:   fmt.Sprintf("%o", info.Mode().Perm()),
		StartedAtNS:  startedNS,
		FinishedAtNS: time.Since(ctx.start).Nanoseconds(),
		RunID:        ctx.RunID,
		FixerID:      ctx.FixerID,
		OK:           true,
		RenameTo:     dest,
		Existed:      true,
	}); aerr != nil {
		if !wrote {
			// WRITE-stage failure: record definitely not persisted; move the
			// file back so disk matches the (empty) journal.
			if backErr := os.Rename(dest, path); backErr != nil {
				return false, fmt.Errorf("doctor: journal Rename of %s: %w; compensating move-back FAILED (%w) — file left at %s and is NOT recorded in actions.jsonl", path, aerr, backErr, dest)
			}
			return false, fmt.Errorf("doctor: journal Rename of %s: %w (compensated: file moved back to its original path; no mutation recorded)", path, aerr)
		}
		// SYNC-stage failure: record probably persisted; leave the move in
		// place so state and journal stay consistent (see workspaceDirRename).
		return false, fmt.Errorf("doctor: journal Rename of %s: record written but not durably synced (%w) — move left in place at %s; actions.jsonl durability is uncertain until the next successful sync", path, aerr, dest)
	}
	return false, nil
}

// workspaceQuarantineDirByName validates name as a bare path element (a
// structural-containment guard on top of the scope check), then renames
// base/name to dest through workspaceDirRename.
func workspaceQuarantineDirByName(ctx *MutateContext, base, name, dest string, verify func(path string) error) error {
	if name != filepath.Base(name) || name == "." || name == ".." || name == "" {
		return fmt.Errorf("doctor: invalid workspace dir name %q (refused_unsafe)", name)
	}
	return workspaceDirRename(ctx, filepath.Join(base, name), dest, verify)
}
