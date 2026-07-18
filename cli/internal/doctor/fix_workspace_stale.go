package doctor

// Workspace stale-queue GC: fm-ws-stale-queue-dirs (auto-fixable).
//
// Land-queue lanes leave debris directories at the top level of `.agents/`:
// explicitly staled dirs (`*.stale-<timestamp>`) and retry-chain dirs
// (`*-retry<N>`). Once their newest content is older than the workspace GC
// TTL (workspaceGCTTL: 14d default, AO_DOCTOR_WS_TTL_DAYS override) they are
// dead weight. The detector flags them in ONE summarizing finding; the fixer
// quarantines each expired dir by renaming it to
// <RunDir>/quarantine/workspace/<dirName> (never a delete).
//
// The detector is PURE (stat + readdir via workspaceDirInventory). The fixer
// re-scans at fix time — it never trusts stale findings — and each move flows
// through the mutation chokepoint discipline via workspaceQuarantineRename
// (see that helper for why Mutate itself cannot take a directory path).

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// workspaceStaleQueueDirsID is the shared detector/fixer ID for workspace
// stale-queue GC.
const workspaceStaleQueueDirsID = "fm-ws-stale-queue-dirs"

func init() {
	RegisterDetector(workspaceStaleQueueDirsDetector{})
	RegisterFixer(workspaceStaleQueueDirsFixer{})
}

// workspaceExpiredStaleDirs returns the top-level directories under base whose
// NAME classifies as stale/retry debris (isWorkspaceStaleDirName) AND whose
// newest content mtime is older than the GC TTL relative to now. An empty
// matched directory (no regular files, zero NewestMTime) counts as expired:
// there is nothing fresh in it by definition. Non-matching or young
// directories are simply not selected. Pure: stat + readdir only.
func workspaceExpiredStaleDirs(base string, now time.Time) ([]workspaceDirInfo, error) {
	inv, err := workspaceDirInventory(base)
	if err != nil {
		return nil, err
	}
	cutoff := now.Add(-workspaceGCTTL())
	var out []workspaceDirInfo
	for _, d := range inv {
		if !isWorkspaceStaleDirName(d.Name) {
			continue
		}
		if d.NewestMTime.IsZero() || d.NewestMTime.Before(cutoff) {
			out = append(out, d)
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Detector
// ---------------------------------------------------------------------------

// workspaceStaleQueueDirsDetector flags expired stale/retry land-queue debris
// directories at the top level of `.agents/`.
type workspaceStaleQueueDirsDetector struct{}

func (workspaceStaleQueueDirsDetector) ID() string           { return workspaceStaleQueueDirsID }
func (workspaceStaleQueueDirsDetector) Subsystem() string    { return subsystemWorkspace }
func (workspaceStaleQueueDirsDetector) Severity() string     { return "P3" }
func (workspaceStaleQueueDirsDetector) EstimatedCostMS() int { return 10 }
func (workspaceStaleQueueDirsDetector) OnlineRequired() bool { return false }
func (workspaceStaleQueueDirsDetector) QuickPath() bool      { return true }
func (workspaceStaleQueueDirsDetector) Describe() string {
	return "expired stale/retry land-queue dirs under .agents awaiting quarantine"
}

func (d workspaceStaleQueueDirsDetector) Detect(env *DetectEnv) ([]Finding, error) {
	base := workspaceAgentsDir(env)
	if _, err := os.Stat(base); err != nil {
		return nil, nil
	}
	expired, err := workspaceExpiredStaleDirs(base, time.Now())
	if err != nil {
		return nil, fmt.Errorf("doctor: inventory %s: %w", base, err)
	}
	if len(expired) == 0 {
		return nil, nil
	}
	ttlDays := int(workspaceGCTTL() / day)
	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      fmt.Sprintf("%d stale/retry workspace dir(s) past the %dd GC TTL", len(expired), ttlDays),
		Confidence: 1.0,
		Evidence: Evidence{
			File:  ".agents",
			Query: fmt.Sprintf(`find .agents -mindepth 1 -maxdepth 1 -type d \( -name '*.stale-*' -o -name '*-retry[0-9]*' \) -mtime +%d`, ttlDays),
		},
		Remediation: Remediation{
			Command:          "ao doctor --fix --only " + d.ID(),
			ExplainCommand:   "ao doctor explain " + d.ID(),
			AutoFixable:      true,
			EstimatedActions: len(expired),
		},
	}}, nil
}

// ---------------------------------------------------------------------------
// Fixer
// ---------------------------------------------------------------------------

// workspaceStaleQueueDirsFixer quarantines each expired stale/retry directory
// via a chokepoint-disciplined Rename into <RunDir>/quarantine/workspace/. It
// re-scans at fix time so a stale finding can never select a directory that
// has since gone young, been renamed, or disappeared.
type workspaceStaleQueueDirsFixer struct{}

func (workspaceStaleQueueDirsFixer) ID() string { return workspaceStaleQueueDirsID }
func (workspaceStaleQueueDirsFixer) Preconditions() []string {
	return []string{
		".agents exists and is a directory",
		"directory name matches the stale/retry debris pattern",
		"newest content mtime is older than the workspace GC TTL (an empty dir counts as expired)",
	}
}
func (workspaceStaleQueueDirsFixer) WritesTo() []string { return []string{".agents"} }
func (workspaceStaleQueueDirsFixer) Ops() []string      { return []string{"Rename"} }
func (workspaceStaleQueueDirsFixer) Reversible() bool   { return true }
func (workspaceStaleQueueDirsFixer) Idempotent() bool   { return true }
func (workspaceStaleQueueDirsFixer) AutoFixable() bool  { return true }

func (f workspaceStaleQueueDirsFixer) Fix(ctx *MutateContext, env *DetectEnv, _ []Finding) (FixResult, error) {
	res := FixResult{FixerID: f.ID(), FindingIDs: []string{f.ID()}}
	base := workspaceAgentsDir(env)
	if info, err := os.Stat(base); err != nil || !info.IsDir() {
		// Nothing to GC; the finding (if any) is stale.
		res.Fixed = true
		return res, nil
	}
	expired, err := workspaceExpiredStaleDirs(base, time.Now())
	if err != nil {
		res.Err = fmt.Errorf("doctor: %s: re-scan %s: %w", f.ID(), base, err)
		return res, res.Err
	}
	for _, d := range expired {
		dest := workspaceQuarantineDest(ctx, d.Name)
		if err := workspaceQuarantineRename(ctx, base, d.Name, dest); err != nil {
			res.Err = fmt.Errorf("doctor: %s: quarantine %s: %w", f.ID(), d.Name, err)
			return res, res.Err
		}
		res.ActionsTaken++
	}
	res.Fixed = true
	return res, nil
}

// workspaceQuarantineRename moves the workspace DIRECTORY <base>/<name> to
// dest with the full mutation-chokepoint discipline. Mutate itself cannot
// take a directory path: its before/after hashing does os.ReadFile(path),
// which fails with EISDIR on a directory. This helper is the directory
// analogue of Mutate for the Rename op only — the same precedent as the
// knowledge fixer's createDirViaRename and the workspace empty-dirs
// quarantineEmptyDir, which bridge chokepoint gaps in-file. It reuses
// Mutate's own primitives step for step: per-path lock, precondition checks,
// atomic execute via executeAtomic, and an fsync'd actions.jsonl line via
// appendAction. The source is gated by structural containment (name must be
// a single plain path element under the fixed workspace root, matching the
// fixer's WritesTo declaration of ".agents"); the destination (under the run
// dir) goes through EnsureInScope, and the op through EnsureOpAllowed.
// Hashes are the empty-content hash (directories have no byte content — the
// same value Mutate records for an absent file). No backup copy is taken: a
// Rename's undo path never consults backups (undoOne reverses it with
// os.Rename, which handles directories), and the quarantined tree IS the
// preserved copy, byte for byte.
func workspaceQuarantineRename(ctx *MutateContext, base, name, dest string) error {
	op := Rename{To: dest}

	// Structural containment: name must be a single, plain path element.
	if name != filepath.Base(name) || name == "." || name == ".." || name == "" {
		return fmt.Errorf("doctor: invalid workspace dir name %q (refused_unsafe)", name)
	}
	path := filepath.Join(base, name)

	// Step 1 — per-path advisory lock (skipped in dry-run, matching Mutate).
	if !ctx.DryRun {
		guard, err := ctx.Locks.Acquire(path)
		if err != nil {
			return err
		}
		defer func() { _ = guard.Release() }()
	}

	// Step 2 — before-state: the source must still exist and be a directory.
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("doctor: stat %s: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("doctor: %s is not a directory (refused_unsafe)", path)
	}
	emptyHash := sha256Hex(nil)

	// Step 3 — preconditions: in-scope destination + executable op. (The
	// source is gated by the structural containment check above.)
	if err := EnsureInScope(ctx.Capabilities, ctx.RepoRoot, ctx.HomeDir, dest); err != nil {
		return err
	}
	if err := EnsureOpAllowed(ctx.Capabilities, op); err != nil {
		return err
	}

	// Step 4 — backup: intentionally none (see the function comment).

	// Step 5/6 — plan + atomic execute.
	startedNS := time.Since(ctx.start).Nanoseconds()
	if ctx.DryRun {
		fmt.Fprintf(os.Stderr, "[dry-run] would mutate %s: %s\n", path, DescribeOp(op))
		return nil
	}
	if err := executeAtomic(path, op); err != nil {
		return fmt.Errorf("doctor: execute %s on %s: %w", op.kind(), path, err)
	}

	// Step 7/8 — record the action line, fsync'd.
	rel, relErr := filepath.Rel(ctx.RepoRoot, path)
	if relErr != nil {
		rel = path
	}
	return ctx.appendAction(ActionRecord{
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
	})
}
