package doctor

// Workspace subsystem: fm-ws-naming-drift (auto-fixable).
//
// Flags top-level `.agents` directories whose names are drifted spellings of a
// canonical directory (the workspaceCanonicalAliases registry: post-mortem ->
// postmortem, handoffs -> ao/handoff, proof -> proofs, ...). Drifted names split
// one logical artifact family across two directories, so tooling that reads
// only the canonical name silently misses half the corpus.
//
// The detector emits ONE FINDING PER ALIAS DIRECTORY found — each is a
// distinct drift instance a user resolves separately — and every finding
// carries the detector's ID (the framework convention: groupFindingsByFixer
// matches findings to fixers by ID equality, so all fm-ws-naming-drift
// findings route to the one fixer in a single Fix call).
//
// The fixer merges each alias directory into its canonical destination entry by
// entry under migration-owner discipline: an entry whose destination name
// already exists in the canonical directory, or an entry that is neither a
// regular file nor a directory (symlink, socket, ...), is NEVER moved and
// never overwritten — it is reported in FixResult.Skipped for a human to
// resolve. A fully emptied alias directory is quarantined via Rename into the
// run's quarantine directory (never removed).
//
// Detector is PURE (stat + readdir only). Every fixer disk write flows through
// the mutation chokepoint discipline: regular files via
// workspaceFileMoveNoClobber (a link+remove adaptation of Mutate's Rename
// whose execute is kernel-atomic no-clobber), and directories via
// workspaceRenameDir — an in-package directory adaptation of Mutate that
// preserves the chokepoint's guarantees (per-path lock, write-scope check, op
// check, dry-run, fsync'd actions.jsonl journal). Directory renames need no
// byte backup to be reversible: undoOne reverses a Rename record with a bare
// rename-back and consults neither backups nor hashes.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// fmWorkspaceNamingDriftID is the shared detector/fixer ID for this failure mode.
const fmWorkspaceNamingDriftID = "fm-ws-naming-drift"

// workspaceAliasMutationTestHook deterministically opens the final
// preflight-to-mutation race window in package tests. Production leaves it
// nil; all real mutation remains rooted by an os.Root descriptor.
var workspaceAliasMutationTestHook func()

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
// name is a registered drifted alias of a canonical path.
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
	base := workspaceAgentsDir(env)
	// Lstat guard: a symlinked `.agents` root would make the inventory (and
	// then the fixer's renames) traverse a tree outside the repo.
	if !workspaceRealDir(base) {
		return nil, nil
	}
	inv, err := workspaceDirInventory(base)
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
// destination entry by entry, skipping (never overwriting) destination collisions
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
	// Never mutate through a `.agents` root that is not a real directory: a
	// symlinked root would make every merge rename act on a tree OUTSIDE the
	// repo while the lexical scope check still passes.
	exists, err := workspaceRequireRealAgentsDir(base)
	if err != nil {
		res.Err = fmt.Errorf("doctor: %s: %w", f.ID(), err)
		return res, res.Err
	}
	if !exists {
		res.Fixed = true // no workspace, nothing drifted
		return res, nil
	}
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

// fixOneAlias merges one alias directory into its canonical destination. An absent
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
	canonicalName := workspaceCanonicalAliases[alias]
	canonicalDir := filepath.Join(base, canonicalName)
	// Destination-tree guard: every existing component below `.agents` must be
	// a real directory. Checking only the leaf is insufficient for nested
	// canonical paths such as ao/handoff: Lstat on that leaf follows a symlinked
	// ao parent and can make an external directory look safe. Missing components
	// are fine (the move creates them); unknown or non-directory components make
	// the whole alias unsafe.
	if unsafeReason := workspaceCanonicalPathUnsafe(base, canonicalName); unsafeReason != "" {
		res.Skipped = append(res.Skipped, fmt.Sprintf("%s: %s", aliasRel, unsafeReason))
		return nil
	}
	entries, err := os.ReadDir(aliasPath)
	if err != nil {
		return fmt.Errorf("doctor: %s: read %s: %w", f.ID(), aliasRel, err)
	}
	for _, e := range entries {
		src := filepath.Join(aliasPath, e.Name())
		dest := filepath.Join(canonicalDir, e.Name())
		srcRel := filepath.Join(aliasRel, e.Name())
		if !e.Type().IsRegular() && !e.IsDir() {
			// Symlink or other special file: moving it could silently change
			// what it resolves to. Leave it in place.
			res.Skipped = append(res.Skipped, fmt.Sprintf("%s: not a regular file or directory; resolve by hand", srcRel))
			continue
		}
		if e.IsDir() {
			unsafeEntry, inspectErr := workspaceDirectoryTreeSpecial(src)
			if inspectErr != nil {
				return fmt.Errorf("doctor: %s: inspect %s: %w", f.ID(), srcRel, inspectErr)
			}
			if unsafeEntry != "" {
				res.Skipped = append(res.Skipped, fmt.Sprintf("%s: nested special entry %s; resolve by hand", srcRel, unsafeEntry))
				continue
			}
		}
		skipReason, moveErr := f.moveEntryNoClobber(ctx, src, dest, e.IsDir())
		if moveErr != nil {
			return fmt.Errorf("doctor: %s: move %s: %w", f.ID(), srcRel, moveErr)
		}
		if skipReason != "" {
			// Refused (destination collision, or destination state unknown).
			// Never overwrite, never guess — leave the entry and report it.
			res.Skipped = append(res.Skipped, fmt.Sprintf("%s: %s", srcRel, skipReason))
			continue
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

// workspaceCanonicalPathUnsafe validates an alias migration destination
// without following symlinks. base itself is checked by Fix before this helper
// runs; this walks each existing relative component with Lstat so a nested
// destination cannot escape through an intermediate symlink. An empty return
// means the existing prefix is safe. The first absent component ends the walk,
// because no deeper component can exist without traversing that missing path.
func workspaceCanonicalPathUnsafe(base, canonicalName string) string {
	clean := filepath.Clean(canonicalName)
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Sprintf("canonical path .agents/%s is invalid; resolve by hand", canonicalName)
	}

	current := base
	for _, component := range strings.Split(clean, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				return ""
			}
			rel, relErr := filepath.Rel(base, current)
			if relErr != nil {
				rel = clean
			}
			return fmt.Sprintf("cannot verify canonical path component .agents/%s (%v); resolve by hand", rel, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			rel, relErr := filepath.Rel(base, current)
			if relErr != nil {
				rel = clean
			}
			return fmt.Sprintf("canonical path component .agents/%s is not a real directory; resolve by hand", rel)
		}
	}
	return ""
}

// errWorkspaceDriftDestExists is the sentinel raised by the under-lock
// destination recheck when a same-named entry appeared at the destination
// after the caller's scan. It is classified as a collision (Skipped), never
// an overwrite.
var errWorkspaceDriftDestExists = errors.New("destination appeared since scan; refusing to overwrite (refused_unsafe)")

// moveEntryNoClobber moves one alias-directory entry to its canonical
// destination WITHOUT ever overwriting. It takes the destination path's
// advisory lock, checks the destination, then routes the move through the
// chokepoint discipline:
//
//   - Regular files go through workspaceFileMoveNoClobber, whose link+remove
//     execute is kernel-atomic no-clobber: os.Link fails with EEXIST if the
//     destination exists (or appears at ANY point, external writers
//     included), so for files the lstat→rename overwrite window is closed
//     for real, not merely narrowed.
//   - Directories go through workspaceDirRename with a verify hook that
//     re-checks destination non-existence under the SOURCE lock. Doctor's
//     advisory locks are in-process only, so for an EXTERNAL writer a
//     verify→rename window remains — but its worst case is bounded: POSIX
//     rename refuses a non-empty destination directory (ENOTEMPTY), so the
//     only thing the rename can silently replace is an EMPTY directory that
//     appeared in the window. No content is lost, and the move is journaled
//     and undoable.
//
// A non-empty skipReason means the entry was refused (destination collision,
// or a destination whose state could not be verified) and nothing moved; the
// caller records it in FixResult.Skipped.
func (workspaceNamingDriftFixer) moveEntryNoClobber(ctx *MutateContext, src, dest string, isDir bool) (skipReason string, err error) {
	collisionReason := fmt.Sprintf("same name already exists in .agents/%s; resolve by hand", filepath.Base(filepath.Dir(dest)))
	if !ctx.DryRun {
		guard, lerr := ctx.Locks.Acquire(dest)
		if lerr != nil {
			return "", lerr
		}
		defer func() { _ = guard.Release() }()
	}
	if _, lerr := os.Lstat(dest); lerr == nil {
		return collisionReason, nil
	} else if !os.IsNotExist(lerr) {
		// Permission or I/O failure: the destination's state is UNKNOWN, and
		// unknown is never "absent". Refuse the move like a collision.
		return fmt.Sprintf("cannot verify destination in .agents/%s (%v); resolve by hand", filepath.Base(filepath.Dir(dest)), lerr), nil
	}
	if isDir {
		unsafeEntry, inspectErr := workspaceDirectoryTreeSpecial(src)
		if inspectErr != nil {
			return "", inspectErr
		}
		if unsafeEntry != "" {
			return fmt.Sprintf("nested special entry %s; resolve by hand", unsafeEntry), nil
		}
		verify := func(string) error {
			unsafeEntry, inspectErr := workspaceDirectoryTreeSpecial(src)
			if inspectErr != nil {
				return inspectErr
			}
			if unsafeEntry != "" {
				return fmt.Errorf("doctor: directory %s gained nested special entry %s (refused_unsafe)", src, unsafeEntry)
			}
			if _, lerr := os.Lstat(dest); lerr == nil {
				return errWorkspaceDriftDestExists
			} else if !os.IsNotExist(lerr) {
				return fmt.Errorf("doctor: lstat %s: %w: %w", dest, lerr, errWorkspaceDriftDestExists)
			}
			return nil
		}
		if workspaceAliasMutationTestHook != nil {
			workspaceAliasMutationTestHook()
		}
		if moveErr := workspaceDirRename(ctx, src, dest, verify); moveErr != nil {
			if errors.Is(moveErr, errWorkspaceDriftDestExists) {
				return collisionReason, nil
			}
			return "", moveErr
		}
		return "", nil
	}
	if workspaceAliasMutationTestHook != nil {
		workspaceAliasMutationTestHook()
	}
	collided, moveErr := workspaceFileMoveNoClobber(ctx, src, dest)
	if moveErr != nil {
		return "", moveErr
	}
	if collided {
		return collisionReason, nil
	}
	return "", nil
}

// workspaceDirectoryTreeSpecial returns the first nested path whose type is
// neither a real directory nor a regular file. It walks through
// descriptor-anchored roots and revalidates directory identities around each
// open, so a symlink cannot be smuggled into a directory-shaped alias entry.
func workspaceDirectoryTreeSpecial(path string) (string, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
		return filepath.Base(path), nil
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = root.Close() }()
	opened, err := root.Stat(".")
	if err != nil {
		return "", err
	}
	after, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if after.Mode()&os.ModeSymlink != 0 || !after.IsDir() || !os.SameFile(before, opened) || !os.SameFile(after, opened) {
		return filepath.Base(path), nil
	}
	return workspaceRootTreeSpecial(root, "")
}

func workspaceRootTreeSpecial(root *os.Root, prefix string) (string, error) {
	dir, err := root.Open(".")
	if err != nil {
		return "", err
	}
	entries, readErr := dir.ReadDir(-1)
	closeErr := dir.Close()
	if readErr != nil {
		return "", readErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	for _, entry := range entries {
		name := entry.Name()
		display := filepath.Join(prefix, name)
		before, err := root.Lstat(name)
		if err != nil {
			return "", err
		}
		if before.Mode()&os.ModeSymlink != 0 {
			return display, nil
		}
		if before.Mode().IsRegular() {
			continue
		}
		if !before.IsDir() {
			return display, nil
		}
		child, err := root.OpenRoot(name)
		if err != nil {
			return "", err
		}
		opened, openedErr := child.Stat(".")
		after, afterErr := root.Lstat(name)
		if openedErr != nil || afterErr != nil || after.Mode()&os.ModeSymlink != 0 || !after.IsDir() || !os.SameFile(before, opened) || !os.SameFile(after, opened) {
			_ = child.Close()
			return display, nil
		}
		unsafeEntry, walkErr := workspaceRootTreeSpecial(child, display)
		_ = child.Close()
		if walkErr != nil || unsafeEntry != "" {
			return unsafeEntry, walkErr
		}
	}
	return "", nil
}

// workspaceRenameDir renames a directory through the shared workspace
// directory-rename chokepoint adapter (workspaceDirRename): same per-path
// lock, scope/op preconditions on both endpoints, dry-run transparency,
// atomic execute, and fsync'd actions.jsonl record.
func workspaceRenameDir(ctx *MutateContext, path string, to string) error {
	return workspaceDirRename(ctx, path, to, nil)
}
