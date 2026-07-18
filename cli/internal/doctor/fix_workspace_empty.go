package doctor

// Workspace subsystem: fm-ws-empty-dirs (auto-fixable).
//
// Flags empty immediate subdirectories of the workspace root `.agents/` —
// directories holding zero transitive regular files (truly empty, or holding
// only empty substructure). These are abandoned scaffolding from crashed or
// half-finished lanes: they clutter inventory output and mislead tooling that
// treats directory presence as a signal.
//
// Three name classes are deliberately NOT claimed here:
//
//   - "ao" — the structured knowledge-store root. Its (empty) substructure is
//     the knowledge subsystem's contract (fm-knowledge-missing-substructure).
//   - canonical directory names (the VALUES of workspaceCanonicalAliases:
//     postmortem, pre-mortem-checks, handoff, retro, proofs, tests) — these may
//     legitimately sit empty awaiting their first write.
//   - stale/retry-named directories (isWorkspaceStaleDirName) — owned by the
//     workspace GC failure mode (fm-ws-stale-queue-dirs); no double-claim.
//
// The detector is PURE (stat + readdir only). The fixer's only user-state
// disk write is a Rename into the run's quarantine directory — never a
// remove — performed by quarantineEmptyDir, the directory adaptation of the
// Mutate eight-step discipline (Mutate itself is byte-oriented and cannot
// hash or back up a directory).

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// fmWorkspaceEmptyDirsID is the shared detector/fixer ID for this failure mode.
const fmWorkspaceEmptyDirsID = "fm-ws-empty-dirs"

func init() {
	RegisterDetector(workspaceEmptyDirsDetector{})
	RegisterFixer(workspaceEmptyDirsFixer{})
}

// workspaceEmptyDirClaimed reports whether name is excluded from the
// empty-dirs failure mode because another owner claims it: the knowledge
// store root, a canonical directory name, or a stale/retry-named directory.
func workspaceEmptyDirClaimed(name string) bool {
	if name == "ao" {
		return true
	}
	for _, canonical := range workspaceCanonicalAliases {
		if name == canonical {
			return true
		}
	}
	return isWorkspaceStaleDirName(name)
}

// workspaceEmptyDirCandidates inventories base and returns the names of
// immediate subdirectories holding zero transitive regular files, excluding
// claimed names, in deterministic (name-sorted) order. A missing base yields
// nil candidates and a nil error — nothing to flag.
func workspaceEmptyDirCandidates(base string) ([]string, error) {
	inv, err := workspaceDirInventory(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, info := range inv {
		if info.FileCount != 0 {
			continue
		}
		if workspaceEmptyDirClaimed(info.Name) {
			continue
		}
		out = append(out, info.Name)
	}
	return out, nil
}

// quarantineEmptyDir renames the verified-empty directory base/name to dest
// with the same eight-step discipline as Mutate (per-path lock, hashes,
// preconditions, dry-run transparency, atomic execute through the chokepoint's
// executeAtomic, fsync'd actions.jsonl record). Mutate itself is byte-oriented
// — its before-hash read and verbatim backup only work on regular files — so
// this directory adaptation lives here.
//
// Scope: the static write-scope list cannot enumerate arbitrary top-level
// `.agents/<name>` directories, so the source is gated STRUCTURALLY — name
// must be a bare path element and base/name a still-empty immediate
// subdirectory of the workspace root — which is strictly narrower than a
// `.agents` scope entry. The destination (under the run dir) goes through
// EnsureInScope, and the op through EnsureOpAllowed.
//
// Because the tree holds zero regular files there are no bytes to hash or back
// up: both hashes record the empty hash, and the "backup" mirrors the empty
// subtree shape under the run's backups/ dir. Undo of a Rename record replays
// the rename in reverse (engine.undoOne), which is directory-safe, so the
// quarantine remains fully reversible.
func quarantineEmptyDir(ctx *MutateContext, base, name, dest string) (bool, error) {
	// Structural containment: name must be a single, plain path element.
	if name != filepath.Base(name) || name == "." || name == ".." || name == "" {
		return false, fmt.Errorf("doctor: invalid workspace dir name %q (refused_unsafe)", name)
	}
	path := filepath.Join(base, name)

	// Step 1 — per-path advisory lock.
	if !ctx.DryRun {
		guard, err := ctx.Locks.Acquire(path)
		if err != nil {
			return false, err
		}
		defer func() { _ = guard.Release() }()
	}

	// Step 2 — before state. Refuse anything but a directory; verify the tree
	// still holds zero transitive regular files (a file may have landed since
	// the caller's scan).
	info, err := os.Stat(path)
	if err != nil {
		return false, fmt.Errorf("doctor: stat %s: %w", path, err)
	}
	if !info.IsDir() {
		return false, fmt.Errorf("doctor: %s is not a directory (refused_unsafe)", path)
	}
	baseInv, err := workspaceDirInventory(base)
	if err != nil {
		return false, fmt.Errorf("doctor: re-inventory %s: %w", base, err)
	}
	stillEmpty := false
	for _, di := range baseInv {
		if di.Name == name {
			stillEmpty = di.FileCount == 0
			break
		}
	}
	if !stillEmpty {
		return false, fmt.Errorf("doctor: %s gained files since scan; refusing to quarantine (refused_unsafe)", path)
	}
	emptyHash := sha256Hex(nil)
	beforeMode := fmt.Sprintf("%o", info.Mode().Perm())

	// Step 3 — preconditions: in-scope destination + executable op. (The
	// source is gated by the structural containment check above.)
	op := Rename{To: dest}
	if err := EnsureInScope(ctx.Capabilities, ctx.RepoRoot, ctx.HomeDir, dest); err != nil {
		return false, err
	}
	if err := EnsureOpAllowed(ctx.Capabilities, op); err != nil {
		return false, err
	}

	rel, relErr := filepath.Rel(ctx.RepoRoot, path)
	if relErr != nil {
		rel = path
	}

	// Step 5/6 — dry-run transparency, then atomic execute through the
	// chokepoint's executor. (Step 4, verbatim backup, degenerates to
	// mirroring the empty subtree shape; Rename undo never reads backups.)
	startedNS := time.Since(ctx.start).Nanoseconds()
	if ctx.DryRun {
		fmt.Fprintf(os.Stderr, "[dry-run] would mutate %s: %s\n", path, DescribeOp(op))
		return true, nil
	}
	if err := mirrorEmptyTree(path, filepath.Join(ctx.RunDir, "backups", rel)); err != nil {
		return false, fmt.Errorf("doctor: backup %s: %w", path, err)
	}
	if err := executeAtomic(path, op); err != nil {
		return false, fmt.Errorf("doctor: execute Rename on %s: %w", path, err)
	}

	// Step 7/8 — after hash (path is gone; empty) + fsync'd action record.
	rec := ActionRecord{
		Path:         rel,
		Op:           op.kind(),
		BeforeHash:   emptyHash,
		AfterHash:    emptyHash,
		BeforeMode:   beforeMode,
		StartedAtNS:  startedNS,
		FinishedAtNS: time.Since(ctx.start).Nanoseconds(),
		RunID:        ctx.RunID,
		FixerID:      ctx.FixerID,
		OK:           true,
		RenameTo:     dest,
		Existed:      true,
	}
	if err := ctx.appendAction(rec); err != nil {
		return false, err
	}
	return true, nil
}

// mirrorEmptyTree recreates src's directory structure (which holds no regular
// files) under dst. It is the backup-step analogue for an empty tree: shape
// only, since there are no bytes to copy.
func mirrorEmptyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || !d.IsDir() {
			return nil //nolint:nilerr // best-effort mirror; unreadable subtree skipped
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return nil //nolint:nilerr // unmappable entry contributes nothing
		}
		return os.MkdirAll(filepath.Join(dst, rel), 0o755)
	})
}

// ---------------------------------------------------------------------------
// FM: fm-ws-empty-dirs (auto-fixable)
// ---------------------------------------------------------------------------

// workspaceEmptyDirsDetector flags unclaimed empty top-level workspace
// directories in one summarizing finding.
type workspaceEmptyDirsDetector struct{}

func (workspaceEmptyDirsDetector) ID() string           { return fmWorkspaceEmptyDirsID }
func (workspaceEmptyDirsDetector) Subsystem() string    { return subsystemWorkspace }
func (workspaceEmptyDirsDetector) Severity() string     { return "P4" }
func (workspaceEmptyDirsDetector) EstimatedCostMS() int { return 4 }
func (workspaceEmptyDirsDetector) OnlineRequired() bool { return false }
func (workspaceEmptyDirsDetector) QuickPath() bool      { return true }
func (workspaceEmptyDirsDetector) Describe() string {
	return "empty unclaimed top-level .agents directories (abandoned scaffolding)"
}

func (d workspaceEmptyDirsDetector) Detect(env *DetectEnv) ([]Finding, error) {
	candidates, err := workspaceEmptyDirCandidates(workspaceAgentsDir(env))
	if err != nil {
		return nil, fmt.Errorf("doctor: inventory workspace: %w", err)
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      fmt.Sprintf("%d empty workspace dir(s): %s", len(candidates), strings.Join(candidates, ", ")),
		Confidence: 1.0,
		Evidence: Evidence{
			File:  ".agents",
			Query: "find .agents -mindepth 1 -maxdepth 1 -type d -empty (plus dirs with only empty subdirs)",
		},
		Remediation: Remediation{
			Command:          "ao doctor --fix --only " + d.ID(),
			ExplainCommand:   "ao doctor explain " + d.ID(),
			AutoFixable:      true,
			EstimatedActions: len(candidates),
		},
	}}, nil
}

// workspaceEmptyDirsFixer quarantines each still-empty qualifying directory via
// a Rename into <RunDir>/quarantine/workspace/. It re-scans at fix time, so a
// directory that gained files between detect and fix is naturally skipped.
type workspaceEmptyDirsFixer struct{}

func (workspaceEmptyDirsFixer) ID() string { return fmWorkspaceEmptyDirsID }
func (workspaceEmptyDirsFixer) Preconditions() []string {
	return []string{
		".agents exists and is a directory",
		"directory still holds zero transitive regular files at fix time",
	}
}
func (workspaceEmptyDirsFixer) WritesTo() []string { return []string{".agents"} }
func (workspaceEmptyDirsFixer) Ops() []string      { return []string{"Rename"} }
func (workspaceEmptyDirsFixer) Reversible() bool   { return true }
func (workspaceEmptyDirsFixer) Idempotent() bool   { return true }
func (workspaceEmptyDirsFixer) AutoFixable() bool  { return true }

func (f workspaceEmptyDirsFixer) Fix(ctx *MutateContext, env *DetectEnv, _ []Finding) (FixResult, error) {
	res := FixResult{FixerID: f.ID(), FindingIDs: []string{f.ID()}}
	base := workspaceAgentsDir(env)
	candidates, err := workspaceEmptyDirCandidates(base)
	if err != nil {
		res.Err = fmt.Errorf("doctor: %s: inventory workspace: %w", f.ID(), err)
		return res, res.Err
	}
	if len(candidates) == 0 {
		res.Fixed = true
		return res, nil
	}
	for _, name := range candidates {
		ok, err := quarantineEmptyDir(ctx, base, name, workspaceQuarantineDest(ctx, name))
		if err != nil {
			res.Err = fmt.Errorf("doctor: %s: quarantine %s: %w", f.ID(), name, err)
			return res, res.Err
		}
		if ok {
			res.ActionsTaken++
		}
	}
	if !ctx.DryRun {
		remaining, err := workspaceEmptyDirCandidates(base)
		if err != nil {
			res.Err = fmt.Errorf("doctor: %s: post-fix inventory: %w", f.ID(), err)
			return res, res.Err
		}
		if len(remaining) != 0 {
			res.Err = fmt.Errorf("doctor: %s: fix did not eliminate the finding", f.ID())
			return res, res.Err
		}
	}
	res.Fixed = true
	return res, nil
}
