package doctor

// Workspace subsystem: fm-ws-naming-drift (auto-fixable).
//
// Flags top-level `.agents` directories whose names are drifted spellings of a
// canonical directory (the workspaceCanonicalAliases registry: post-mortem ->
// postmortem, handoffs -> handoff, proof -> proofs, ...). Drifted names split
// one logical artifact family across two directories, so tooling that reads
// only the canonical name silently misses half the corpus.
//
// The detector emits ONE FINDING PER ALIAS DIRECTORY found — each is a
// distinct drift instance a user resolves separately — and every finding
// carries the detector's ID (the framework convention: groupFindingsByFixer
// matches findings to fixers by ID equality, so all fm-ws-naming-drift
// findings route to the one fixer in a single Fix call).
//
// The fixer merges each alias directory into its canonical sibling entry by
// entry under migration-owner discipline: an entry whose destination name
// already exists in the canonical directory, or an entry that is neither a
// regular file nor a directory (symlink, socket, ...), is NEVER moved and
// never overwritten — it is reported in FixResult.Skipped for a human to
// resolve. A fully emptied alias directory is quarantined via Rename into the
// run's quarantine directory (never removed).
//
// Detector is PURE (stat + readdir only). Every fixer disk write flows through
// the mutation chokepoint: regular files via Mutate, and directories via
// workspaceRenameDir — an in-package directory adaptation of Mutate that
// preserves the chokepoint's guarantees (per-path lock, write-scope check, op
// check, dry-run, fsync'd actions.jsonl journal). Directory renames need no
// byte backup to be reversible: undoOne reverses a Rename record with a bare
// rename-back and consults neither backups nor hashes.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// fmWorkspaceNamingDriftID is the shared detector/fixer ID for this failure mode.
const fmWorkspaceNamingDriftID = "fm-ws-naming-drift"

func init() {
	RegisterDetector(workspaceNamingDriftDetector{})
	RegisterFixer(workspaceNamingDriftFixer{})
}

// sortedDriftAliases returns the alias names of workspaceCanonicalAliases in
// deterministic sorted order, so findings and fix actions are stably ordered.
func sortedDriftAliases() []string {
	out := make([]string, 0, len(workspaceCanonicalAliases))
	for alias := range workspaceCanonicalAliases {
		out = append(out, alias)
	}
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------------
// FM: fm-ws-naming-drift (auto-fixable)
// ---------------------------------------------------------------------------

// workspaceNamingDriftDetector flags each top-level `.agents` directory whose
// name is a registered drifted alias of a canonical directory name.
type workspaceNamingDriftDetector struct{}

func (workspaceNamingDriftDetector) ID() string           { return fmWorkspaceNamingDriftID }
func (workspaceNamingDriftDetector) Subsystem() string    { return subsystemWorkspace }
func (workspaceNamingDriftDetector) Severity() string     { return "P3" }
func (workspaceNamingDriftDetector) EstimatedCostMS() int { return 4 }
func (workspaceNamingDriftDetector) OnlineRequired() bool { return false }
func (workspaceNamingDriftDetector) QuickPath() bool      { return true }
func (workspaceNamingDriftDetector) Describe() string {
	return "top-level .agents directory uses a drifted spelling of a canonical name"
}

func (d workspaceNamingDriftDetector) Detect(env *DetectEnv) ([]Finding, error) {
	inv, err := workspaceDirInventory(workspaceAgentsDir(env))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("doctor: inventory workspace: %w", err)
	}
	byName := make(map[string]workspaceDirInfo, len(inv))
	for _, info := range inv {
		byName[info.Name] = info
	}
	var findings []Finding
	// One finding PER alias dir found: each is a distinct drift instance the
	// user resolves separately. The alias is flagged regardless of whether the
	// canonical directory exists (Rename creates a missing canonical dir).
	for _, alias := range sortedDriftAliases() {
		info, ok := byName[alias]
		if !ok {
			// The inventory is symlink-safe: an alias name occupied by a
			// symlink or regular file is not a directory-naming drift.
			continue
		}
		canonical := workspaceCanonicalAliases[alias]
		findings = append(findings, Finding{
			ID:         d.ID(),
			Severity:   d.Severity(),
			Subsystem:  d.Subsystem(),
			Title:      fmt.Sprintf("workspace naming drift: .agents/%s -> .agents/%s", alias, canonical),
			Confidence: 1.0,
			Evidence: Evidence{
				File:  filepath.Join(".agents", alias),
				Query: fmt.Sprintf("%d transitive regular file(s) under .agents/%s; canonical directory is .agents/%s", info.FileCount, alias, canonical),
			},
			Remediation: Remediation{
				Command:          "ao doctor --fix --only " + d.ID(),
				ExplainCommand:   "ao doctor explain " + d.ID(),
				AutoFixable:      true,
				EstimatedActions: info.FileCount + 1,
			},
		})
	}
	return findings, nil
}

// workspaceNamingDriftFixer merges each alias directory into its canonical
// sibling entry by entry, skipping (never overwriting) destination collisions
// and non-regular non-directory oddities, then quarantines the emptied alias
// directory. It re-scans the disk at fix time rather than trusting findings.
type workspaceNamingDriftFixer struct{}

func (workspaceNamingDriftFixer) ID() string { return fmWorkspaceNamingDriftID }
func (workspaceNamingDriftFixer) Preconditions() []string {
	return []string{
		".agents exists and is a directory",
		"alias path is a real directory (not a symlink or regular file)",
		"moved entries have no same-named entry in the canonical directory",
	}
}
func (workspaceNamingDriftFixer) WritesTo() []string { return []string{".agents"} }
func (workspaceNamingDriftFixer) Ops() []string      { return []string{"Rename"} }
func (workspaceNamingDriftFixer) Reversible() bool   { return true }
func (workspaceNamingDriftFixer) Idempotent() bool   { return true }
func (workspaceNamingDriftFixer) AutoFixable() bool  { return true }

func (f workspaceNamingDriftFixer) Fix(ctx *MutateContext, env *DetectEnv, _ []Finding) (FixResult, error) {
	res := FixResult{FixerID: f.ID(), FindingIDs: []string{f.ID()}}
	base := workspaceAgentsDir(env)
	for _, alias := range sortedDriftAliases() {
		if err := f.fixOneAlias(ctx, base, alias, &res); err != nil {
			res.Err = err
			return res, err
		}
	}
	// Migration-owner discipline: a non-empty Skipped means units were
	// deliberately refused as ambiguous — the fixer is not fully done.
	res.Fixed = len(res.Skipped) == 0
	return res, nil
}

// fixOneAlias merges one alias directory into its canonical sibling. An absent
// alias is a no-op (idempotency); a non-directory alias path is recorded in
// Skipped and left alone. Skipped entries stay in place, and the alias dir is
// quarantined only once it holds nothing at all.
func (f workspaceNamingDriftFixer) fixOneAlias(ctx *MutateContext, base, alias string, res *FixResult) error {
	aliasPath := filepath.Join(base, alias)
	aliasRel := filepath.Join(".agents", alias)
	fi, err := os.Lstat(aliasPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // nothing to do — already merged or never drifted
		}
		return fmt.Errorf("doctor: %s: lstat %s: %w", f.ID(), aliasRel, err)
	}
	if fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
		// Never guess: an alias name occupied by a symlink or regular file is
		// not the drift this fixer owns. Leave it for a human.
		res.Skipped = append(res.Skipped, fmt.Sprintf("%s: alias path is not a real directory; resolve by hand", aliasRel))
		return nil
	}
	canonicalDir := filepath.Join(base, workspaceCanonicalAliases[alias])
	entries, err := os.ReadDir(aliasPath)
	if err != nil {
		return fmt.Errorf("doctor: %s: read %s: %w", f.ID(), aliasRel, err)
	}
	for _, e := range entries {
		src := filepath.Join(aliasPath, e.Name())
		dest := filepath.Join(canonicalDir, e.Name())
		srcRel := filepath.Join(aliasRel, e.Name())
		if _, lerr := os.Lstat(dest); lerr == nil {
			// Destination name already exists: both old and new form present.
			// Never overwrite, never guess — leave the entry and report it.
			res.Skipped = append(res.Skipped, fmt.Sprintf("%s: same name already exists in .agents/%s; resolve by hand", srcRel, workspaceCanonicalAliases[alias]))
			continue
		}
		var moveErr error
		switch {
		case e.Type().IsRegular():
			_, moveErr = Mutate(ctx, src, Rename{To: dest})
		case e.IsDir():
			moveErr = workspaceRenameDir(ctx, src, dest)
		default:
			// Symlink or other special file: moving it could silently change
			// what it resolves to. Leave it in place.
			res.Skipped = append(res.Skipped, fmt.Sprintf("%s: not a regular file or directory; resolve by hand", srcRel))
			continue
		}
		if moveErr != nil {
			return fmt.Errorf("doctor: %s: move %s: %w", f.ID(), srcRel, moveErr)
		}
		res.ActionsTaken++
	}
	if ctx.DryRun {
		return nil // post-state is unobservable in dry-run; nothing moved
	}
	remaining, err := os.ReadDir(aliasPath)
	if err != nil {
		return fmt.Errorf("doctor: %s: re-read %s: %w", f.ID(), aliasRel, err)
	}
	if len(remaining) != 0 {
		res.Skipped = append(res.Skipped, fmt.Sprintf("%s: %d skipped entr(y/ies) remain; alias dir left in place", aliasRel, len(remaining)))
		return nil
	}
	if err := workspaceRenameDir(ctx, aliasPath, workspaceQuarantineDest(ctx, alias)); err != nil {
		return fmt.Errorf("doctor: %s: quarantine %s: %w", f.ID(), aliasRel, err)
	}
	res.ActionsTaken++
	return nil
}

// workspaceRenameDir renames a directory through the mutation chokepoint's
// guarantees. Mutate itself is file-shaped (it hashes and backs up the path's
// bytes, which fails with EISDIR on a directory), so this helper adapts its
// steps for a directory: the same per-path lock, the same write-scope and op
// preconditions, the same dry-run behavior, the same atomic execute
// (executeAtomic MkdirAll's the destination parent), and the same fsync'd
// actions.jsonl record. A directory Rename needs no byte backup or content
// hash to be reversible: undoOne reverses a Rename record with a bare
// rename-back and never consults backups or hashes for it. Hash fields are
// recorded as the empty-content hash for journal-shape consistency.
func workspaceRenameDir(ctx *MutateContext, path string, to string) error {
	if !ctx.DryRun {
		guard, err := ctx.Locks.Acquire(path)
		if err != nil {
			return err
		}
		defer func() { _ = guard.Release() }()
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("doctor: lstat %s: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("doctor: rename-dir %s: not a directory", path)
	}
	op := Rename{To: to}
	if err := EnsureInScope(ctx.Capabilities, ctx.RepoRoot, ctx.HomeDir, path); err != nil {
		return err
	}
	if err := EnsureOpAllowed(ctx.Capabilities, op); err != nil {
		return err
	}
	startedNS := time.Since(ctx.start).Nanoseconds()
	if ctx.DryRun {
		fmt.Fprintf(os.Stderr, "[dry-run] would mutate %s: %s\n", path, DescribeOp(op))
		return nil
	}
	if err := executeAtomic(path, op); err != nil {
		return fmt.Errorf("doctor: execute Rename on %s: %w", path, err)
	}
	rel, relErr := filepath.Rel(ctx.RepoRoot, path)
	if relErr != nil {
		rel = path
	}
	emptyHash := sha256Hex(nil)
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
		RenameTo:     to,
		Existed:      true,
	})
}
