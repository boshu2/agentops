package sessionapp

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// PruneAgentsOptions controls retention cleanup under a repository's .agents
// workspace. Execute defaults to false so a zero-value invocation is read-only.
// The implementation binds intermediate-directory traversal; it does not claim
// to make a final basename immutable against an adversarial replacement.
type PruneAgentsOptions struct {
	RepoRoot string
	Execute  bool
	Quiet    bool
	Stdout   io.Writer
	Now      func() time.Time
}

// PruneAgentsResult is the factual count produced by one retention pass.
type PruneAgentsResult struct {
	Files int
	Bytes int64
}

// pruneAgentsBeforeDeleteTestHook is an in-process package test seam. It fires
// after the final current-path identity check and before the descriptor-rooted
// delete. Production has no environment or command-controlled race callback.
var pruneAgentsBeforeDeleteTestHook func(relativePath string)

type pruneAgentsRunner struct {
	opts   PruneAgentsOptions
	root   *os.Root
	now    time.Time
	result PruneAgentsResult
}

type pruneCandidate struct {
	name string
	info os.FileInfo
}

// PruneAgents applies the retention policy historically exposed by
// scripts/prune-agents.sh. All mutation is relative to an already-open os.Root
// for the target's parent directory. A renamed or symlink-swapped intermediate
// path therefore cannot redirect deletion to another tree; parent identity is
// checked immediately before and after each mutation, and observed drift makes
// the run fail closed. The final child name is checked but not locked, so this
// is not a claim of resistance to a final-entry replacement or unobserved ABA.
func PruneAgents(opts PruneAgentsOptions) (PruneAgentsResult, error) {
	if strings.TrimSpace(opts.RepoRoot) == "" {
		return PruneAgentsResult{}, fmt.Errorf("prune agents: repository root is required")
	}
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	root, err := os.OpenRoot(opts.RepoRoot)
	if err != nil {
		return PruneAgentsResult{}, fmt.Errorf("prune agents: open repository root: %w", err)
	}
	defer func() { _ = root.Close() }()

	runner := &pruneAgentsRunner{opts: opts, root: root, now: opts.Now()}
	if err := runner.validateCanonicalHandoffChain(); err != nil {
		return runner.result, err
	}
	if !opts.Quiet {
		if opts.Execute {
			fmt.Fprintln(opts.Stdout, "=== EXECUTE MODE — files will be deleted ===")
		} else {
			fmt.Fprintln(opts.Stdout, "=== DRY RUN — no files will be deleted (pass --execute to delete) ===")
		}
		fmt.Fprintln(opts.Stdout)
	}

	steps := []func() error{
		func() error { return runner.pruneKeepNewest(".agents/council", 30, "council") },
		runner.pruneLegacyScannerDirs,
		func() error {
			return runner.pruneOlderThan(".agents/knowledge/pending", 14, "*.md", "knowledge/pending")
		},
		func() error {
			return runner.pruneOlderThan(".agents/rpi", 30, "phase-*-summary-*", "rpi/phase-summaries")
		},
		func() error { return runner.pruneKeepNewest(".agents/ao/sessions", 50, "ao/sessions") },
		func() error {
			if err := runner.validateCanonicalHandoffChain(); err != nil {
				return err
			}
			return runner.pruneKeepNewest(".agents/ao/handoff", 10, "ao/handoff")
		},
		func() error {
			if err := runner.pruneOlderThan(".agents/opencode-tests", 7, "*.log", "opencode-tests"); err != nil {
				return err
			}
			return runner.pruneOlderThan(".agents/opencode-tests", 7, "*.txt", "opencode-tests/summaries")
		},
		func() error { return runner.pruneKeepNewest(".agents/ao/subagent-outputs", 50, "ao/subagent-outputs") },
		runner.pruneLocalCIRuns,
		func() error {
			if err := runner.pruneKeepNewest(".agents/vibe", 20, "vibe"); err != nil {
				return err
			}
			return runner.pruneKeepNewest(".agents/vibecheck", 20, "vibecheck")
		},
		func() error { return runner.pruneKeepNewest(".agents/brainstorm", 10, "brainstorm") },
		func() error {
			return runner.pruneOlderThan(".agents/compaction-snapshots", 7, "*.md", "compaction-snapshots")
		},
		func() error { return runner.pruneKeepNewest(".agents/swarm", 10, "swarm") },
		runner.pruneStatusDashboards,
		runner.pruneArchivedWorktrees,
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return runner.result, err
		}
		if !opts.Quiet {
			fmt.Fprintln(opts.Stdout)
		}
	}

	fmt.Fprintln(opts.Stdout, "========================================")
	if opts.Execute {
		fmt.Fprintln(opts.Stdout, "PRUNE COMPLETE")
		fmt.Fprintf(opts.Stdout, "Files deleted: %d\n", runner.result.Files)
	} else {
		fmt.Fprintln(opts.Stdout, "DRY RUN COMPLETE")
		fmt.Fprintf(opts.Stdout, "Files that would be deleted: %d\n", runner.result.Files)
	}
	if !opts.Quiet {
		fmt.Fprintln(opts.Stdout)
		fmt.Fprintln(opts.Stdout, "Protected directories (never pruned):")
		fmt.Fprintln(opts.Stdout, "  handoff/ mto-handoff/ learnings/ patterns/ plans/ research/ retros/")
	}
	return runner.result, nil
}

func (runner *pruneAgentsRunner) pruneKeepNewest(relativeDir string, keep int, label string) error {
	dir, exists, err := runner.openRealDir(relativeDir)
	if err != nil || !exists {
		return err
	}
	defer func() { _ = dir.Close() }()
	candidates, err := regularChildren(dir)
	if err != nil {
		return fmt.Errorf("prune agents: inspect %s: %w", runner.display(relativeDir), err)
	}
	if len(candidates) <= keep {
		if !runner.opts.Quiet {
			fmt.Fprintf(runner.opts.Stdout, "[%s] %d files — within limit (%d). Nothing to prune.\n", label, len(candidates), keep)
		}
		return nil
	}
	sortNewestFirst(candidates)
	toDelete := candidates[keep:]
	if !runner.opts.Quiet {
		fmt.Fprintf(runner.opts.Stdout, "[%s] %d files — keeping newest %d, pruning %d\n", label, len(candidates), keep, len(toDelete))
	}
	for _, candidate := range toDelete {
		if err := runner.pruneFile(dir, relativeDir, candidate); err != nil {
			return err
		}
	}
	return nil
}

func (runner *pruneAgentsRunner) pruneOlderThan(relativeDir string, days int, pattern, label string) error {
	dir, exists, err := runner.openRealDir(relativeDir)
	if err != nil || !exists {
		return err
	}
	defer func() { _ = dir.Close() }()
	children, err := regularChildren(dir)
	if err != nil {
		return fmt.Errorf("prune agents: inspect %s: %w", runner.display(relativeDir), err)
	}
	cutoff := runner.now.Add(-time.Duration(days+1) * 24 * time.Hour)
	var candidates []pruneCandidate
	for _, candidate := range children {
		matched, matchErr := filepath.Match(pattern, candidate.name)
		if matchErr != nil {
			return fmt.Errorf("prune agents: invalid retention pattern %q: %w", pattern, matchErr)
		}
		if matched && !candidate.info.ModTime().After(cutoff) {
			candidates = append(candidates, candidate)
		}
	}
	if len(candidates) == 0 {
		if !runner.opts.Quiet {
			fmt.Fprintf(runner.opts.Stdout, "[%s] No files older than %dd matching '%s'. Nothing to prune.\n", label, days, pattern)
		}
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].name < candidates[j].name })
	if !runner.opts.Quiet {
		fmt.Fprintf(runner.opts.Stdout, "[%s] %d files older than %dd\n", label, len(candidates), days)
	}
	for _, candidate := range candidates {
		if err := runner.pruneFile(dir, relativeDir, candidate); err != nil {
			return err
		}
	}
	return nil
}

func (runner *pruneAgentsRunner) pruneFile(dir *os.Root, relativeDir string, candidate pruneCandidate) error {
	relativePath := filepath.Join(relativeDir, candidate.name)
	runner.result.Files++
	runner.result.Bytes += candidate.info.Size()
	if !runner.opts.Execute {
		if !runner.opts.Quiet {
			fmt.Fprintf(runner.opts.Stdout, "  would delete: %s (%s)\n", runner.display(relativePath), numfmtSize(candidate.info.Size()))
		}
		return nil
	}
	if err := runner.removeAnchored(dir, relativeDir, candidate.name, candidate.info, false); err != nil {
		return err
	}
	if !runner.opts.Quiet {
		fmt.Fprintf(runner.opts.Stdout, "  deleted: %s (%s)\n", runner.display(relativePath), numfmtSize(candidate.info.Size()))
	}
	return nil
}

func (runner *pruneAgentsRunner) pruneLegacyScannerDirs() error {
	for _, relativeDir := range []string{".agents/tooling", ".agents/security"} {
		parentRel, name := filepath.Split(relativeDir)
		parentRel = filepath.Clean(parentRel)
		parent, exists, err := runner.openRealDir(parentRel)
		if err != nil || !exists {
			return err
		}
		info, err := parent.Lstat(name)
		if errors.Is(err, os.ErrNotExist) {
			_ = parent.Close()
			continue
		}
		if err != nil {
			_ = parent.Close()
			return fmt.Errorf("prune agents: inspect %s: %w", runner.display(relativeDir), err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			_ = parent.Close()
			return runner.unsafe("legacy scanner path is not a real directory: %s", runner.display(relativeDir))
		}
		dir, err := parent.OpenRoot(name)
		if err != nil {
			_ = parent.Close()
			return runner.unsafe("open legacy scanner directory %s: %v", runner.display(relativeDir), err)
		}
		count, err := countRegularTree(dir)
		_ = dir.Close()
		if err != nil {
			_ = parent.Close()
			return fmt.Errorf("prune agents: count %s: %w", runner.display(relativeDir), err)
		}
		if count == 0 {
			_ = parent.Close()
			continue
		}
		if !runner.opts.Quiet {
			fmt.Fprintf(runner.opts.Stdout, "[legacy] %s has %d files (scanner output moved to $TMPDIR)\n", runner.display(relativeDir), count)
		}
		runner.result.Files++
		if !runner.opts.Execute {
			if !runner.opts.Quiet {
				fmt.Fprintf(runner.opts.Stdout, "  would delete: %s/ (%d files)\n", runner.display(relativeDir), count)
			}
			_ = parent.Close()
			continue
		}
		if err := runner.removeAnchored(parent, parentRel, name, info, true); err != nil {
			_ = parent.Close()
			return err
		}
		if err := runner.ensureOpenedDirCurrent(parentRel, parent); err != nil {
			_ = parent.Close()
			return err
		}
		if err := parent.Mkdir(name, 0o755); err != nil {
			_ = parent.Close()
			return fmt.Errorf("prune agents: recreate %s: %w", runner.display(relativeDir), err)
		}
		_ = parent.Close()
		if !runner.opts.Quiet {
			fmt.Fprintf(runner.opts.Stdout, "  deleted: %s/ (%d files)\n", runner.display(relativeDir), count)
		}
	}
	return nil
}

func (runner *pruneAgentsRunner) pruneLocalCIRuns() error {
	const relativeDir = ".agents/releases/local-ci"
	dir, exists, err := runner.openRealDir(relativeDir)
	if err != nil || !exists {
		return err
	}
	defer func() { _ = dir.Close() }()
	directories, err := directoryChildren(dir)
	if err != nil {
		return fmt.Errorf("prune agents: inspect %s: %w", runner.display(relativeDir), err)
	}
	const keep = 3
	if len(directories) <= keep {
		if !runner.opts.Quiet {
			fmt.Fprintf(runner.opts.Stdout, "[releases/local-ci] %d runs — within limit (%d). Nothing to prune.\n", len(directories), keep)
		}
		return nil
	}
	sortNewestFirst(directories)
	toDelete := directories[keep:]
	if !runner.opts.Quiet {
		fmt.Fprintf(runner.opts.Stdout, "[releases/local-ci] %d runs — keeping newest %d, pruning %d\n", len(directories), keep, len(toDelete))
	}
	for _, candidate := range toDelete {
		sizeKB := int64(0)
		child, openErr := dir.OpenRoot(candidate.name)
		if openErr == nil {
			if bytes, walkErr := regularTreeBytes(child); walkErr == nil {
				sizeKB = (bytes + 1023) / 1024
			}
			_ = child.Close()
		}
		runner.result.Files++
		relativePath := filepath.Join(relativeDir, candidate.name)
		if !runner.opts.Execute {
			if !runner.opts.Quiet {
				fmt.Fprintf(runner.opts.Stdout, "  would delete: %s (~%dKB)\n", runner.display(relativePath), sizeKB)
			}
			continue
		}
		if err := runner.removeAnchored(dir, relativeDir, candidate.name, candidate.info, true); err != nil {
			return err
		}
		if !runner.opts.Quiet {
			fmt.Fprintf(runner.opts.Stdout, "  deleted: %s (~%dKB)\n", runner.display(relativePath), sizeKB)
		}
	}
	return nil
}

func (runner *pruneAgentsRunner) pruneStatusDashboards() error {
	const relativeDir = ".agents"
	dir, exists, err := runner.openRealDir(relativeDir)
	if err != nil || !exists {
		return err
	}
	defer func() { _ = dir.Close() }()
	children, err := regularChildren(dir)
	if err != nil {
		return fmt.Errorf("prune agents: inspect %s: %w", runner.display(relativeDir), err)
	}
	var candidates []pruneCandidate
	for _, candidate := range children {
		if strings.HasPrefix(candidate.name, "status-dashboard") {
			candidates = append(candidates, candidate)
		}
	}
	if len(candidates) <= 5 {
		return nil
	}
	sortNewestFirst(candidates)
	toDelete := candidates[5:]
	if !runner.opts.Quiet {
		fmt.Fprintf(runner.opts.Stdout, "[status-dashboards] %d files — keeping newest 5, pruning %d\n", len(candidates), len(toDelete))
	}
	for _, candidate := range toDelete {
		runner.result.Files++
		relativePath := filepath.Join(relativeDir, candidate.name)
		if !runner.opts.Execute {
			if !runner.opts.Quiet {
				fmt.Fprintf(runner.opts.Stdout, "  would delete: %s\n", runner.display(relativePath))
			}
			continue
		}
		if err := runner.removeAnchored(dir, relativeDir, candidate.name, candidate.info, false); err != nil {
			return err
		}
		if !runner.opts.Quiet {
			fmt.Fprintf(runner.opts.Stdout, "  deleted: %s\n", runner.display(relativePath))
		}
	}
	return nil
}

func (runner *pruneAgentsRunner) pruneArchivedWorktrees() error {
	const relativeDir = ".agents/archived-worktrees"
	dir, exists, err := runner.openRealDir(relativeDir)
	if err != nil || !exists {
		return err
	}
	defer func() { _ = dir.Close() }()
	directories, err := directoryChildren(dir)
	if err != nil {
		return fmt.Errorf("prune agents: inspect %s: %w", runner.display(relativeDir), err)
	}
	cutoff := runner.now.Add(-8 * 24 * time.Hour)
	var candidates []pruneCandidate
	for _, candidate := range directories {
		if !candidate.info.ModTime().After(cutoff) {
			candidates = append(candidates, candidate)
		}
	}
	if len(candidates) == 0 {
		if !runner.opts.Quiet {
			fmt.Fprintln(runner.opts.Stdout, "[archived-worktrees] No directories older than 7d. Nothing to prune.")
		}
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].name < candidates[j].name })
	if !runner.opts.Quiet {
		fmt.Fprintf(runner.opts.Stdout, "[archived-worktrees] %d directories older than 7d\n", len(candidates))
	}
	for _, candidate := range candidates {
		runner.result.Files++
		relativePath := filepath.Join(relativeDir, candidate.name)
		if !runner.opts.Execute {
			if !runner.opts.Quiet {
				fmt.Fprintf(runner.opts.Stdout, "  would delete: %s\n", runner.display(relativePath))
			}
			continue
		}
		if err := runner.removeAnchored(dir, relativeDir, candidate.name, candidate.info, true); err != nil {
			return err
		}
		if !runner.opts.Quiet {
			fmt.Fprintf(runner.opts.Stdout, "  deleted: %s\n", runner.display(relativePath))
		}
	}
	return nil
}

// removeAnchored performs the actual mutation through the already-open parent
// directory descriptor. The current parent path is checked before the test hook
// and after the mutation. Even if an intermediate component changes in between,
// the delete remains bound to the opened directory and cannot follow the new
// path; the post-check converts observed drift into a non-zero result. The
// SameFile check below observes the final child name at one instant; it does not
// lock that directory entry or claim resistance to a final-name ABA.
func (runner *pruneAgentsRunner) removeAnchored(parent *os.Root, parentRel, name string, expected os.FileInfo, recursive bool) error {
	if err := runner.ensureOpenedDirCurrent(parentRel, parent); err != nil {
		return err
	}
	current, err := parent.Lstat(name)
	if err != nil {
		return runner.unsafe("target changed before deletion: %s: %v", runner.display(filepath.Join(parentRel, name)), err)
	}
	if current.Mode()&os.ModeSymlink != 0 || !os.SameFile(expected, current) || current.IsDir() != expected.IsDir() {
		return runner.unsafe("target changed identity before deletion: %s", runner.display(filepath.Join(parentRel, name)))
	}
	if pruneAgentsBeforeDeleteTestHook != nil {
		pruneAgentsBeforeDeleteTestHook(filepath.Join(parentRel, name))
	}
	if recursive {
		err = parent.RemoveAll(name)
	} else {
		err = parent.Remove(name)
	}
	if err != nil {
		return runner.unsafe("descriptor-rooted deletion failed for %s: %v", runner.display(filepath.Join(parentRel, name)), err)
	}
	if err := runner.ensureOpenedDirCurrent(parentRel, parent); err != nil {
		return err
	}
	return nil
}

func (runner *pruneAgentsRunner) ensureOpenedDirCurrent(relativeDir string, opened *os.Root) error {
	current, exists, err := runner.openRealDir(relativeDir)
	if err != nil {
		return err
	}
	if !exists {
		return runner.unsafe("mutation parent disappeared: %s", runner.display(relativeDir))
	}
	defer func() { _ = current.Close() }()
	openedInfo, err := opened.Stat(".")
	if err != nil {
		return runner.unsafe("inspect opened mutation parent %s: %v", runner.display(relativeDir), err)
	}
	currentInfo, err := current.Stat(".")
	if err != nil || !os.SameFile(openedInfo, currentInfo) {
		return runner.unsafe("mutation parent changed identity: %s", runner.display(relativeDir))
	}
	return nil
}

func (runner *pruneAgentsRunner) validateCanonicalHandoffChain() error {
	current := runner.root
	owned := false
	currentRel := ""
	for _, component := range []string{".agents", "ao", "handoff"} {
		currentRel = filepath.Join(currentRel, component)
		before, err := current.Lstat(component)
		if errors.Is(err, os.ErrNotExist) {
			if owned {
				_ = current.Close()
			}
			return nil
		}
		if err != nil {
			if owned {
				_ = current.Close()
			}
			return runner.unsafe("inspect canonical handoff path component %s: %v", runner.display(currentRel), err)
		}
		if before.Mode()&os.ModeSymlink != 0 {
			if owned {
				_ = current.Close()
			}
			return runner.unsafe("canonical handoff path component is a symlink: %s", runner.display(currentRel))
		}
		if !before.IsDir() {
			if owned {
				_ = current.Close()
			}
			return runner.unsafe("canonical handoff path component is not a directory: %s", runner.display(currentRel))
		}
		next, err := current.OpenRoot(component)
		if err != nil {
			if owned {
				_ = current.Close()
			}
			return runner.unsafe("open canonical handoff path component %s: %v", runner.display(currentRel), err)
		}
		openedInfo, openedErr := next.Stat(".")
		after, afterErr := current.Lstat(component)
		if openedErr != nil || afterErr != nil || after.Mode()&os.ModeSymlink != 0 || !after.IsDir() || !os.SameFile(before, openedInfo) || !os.SameFile(after, openedInfo) {
			_ = next.Close()
			if owned {
				_ = current.Close()
			}
			return runner.unsafe("canonical handoff path component changed identity while opening: %s", runner.display(currentRel))
		}
		if owned {
			_ = current.Close()
		}
		current = next
		owned = true
	}
	if owned {
		_ = current.Close()
	}
	return nil
}

func (runner *pruneAgentsRunner) openRealDir(relativeDir string) (*os.Root, bool, error) {
	clean := filepath.Clean(relativeDir)
	if clean == "." || clean == "" || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, false, runner.unsafe("invalid repository-relative directory: %s", relativeDir)
	}
	current := runner.root
	owned := false
	currentRel := ""
	for _, component := range strings.Split(clean, string(filepath.Separator)) {
		currentRel = filepath.Join(currentRel, component)
		before, err := current.Lstat(component)
		if errors.Is(err, os.ErrNotExist) {
			if owned {
				_ = current.Close()
			}
			return nil, false, nil
		}
		if err != nil {
			if owned {
				_ = current.Close()
			}
			return nil, false, runner.unsafe("inspect directory component %s: %v", runner.display(currentRel), err)
		}
		if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
			if owned {
				_ = current.Close()
			}
			return nil, false, runner.unsafe("directory component is not a real directory: %s", runner.display(currentRel))
		}
		next, err := current.OpenRoot(component)
		if err != nil {
			if owned {
				_ = current.Close()
			}
			return nil, false, runner.unsafe("open directory component %s: %v", runner.display(currentRel), err)
		}
		openedInfo, openedErr := next.Stat(".")
		after, afterErr := current.Lstat(component)
		if openedErr != nil || afterErr != nil || after.Mode()&os.ModeSymlink != 0 || !after.IsDir() || !os.SameFile(before, openedInfo) || !os.SameFile(after, openedInfo) {
			_ = next.Close()
			if owned {
				_ = current.Close()
			}
			return nil, false, runner.unsafe("directory component changed identity while opening: %s", runner.display(currentRel))
		}
		if owned {
			_ = current.Close()
		}
		current = next
		owned = true
	}
	return current, true, nil
}

func regularChildren(root *os.Root) ([]pruneCandidate, error) {
	return typedChildren(root, false)
}

func directoryChildren(root *os.Root) ([]pruneCandidate, error) {
	return typedChildren(root, true)
}

func typedChildren(root *os.Root, directories bool) ([]pruneCandidate, error) {
	dir, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	entries, readErr := dir.ReadDir(-1)
	closeErr := dir.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	var candidates []pruneCandidate
	for _, entry := range entries {
		info, err := root.Lstat(entry.Name())
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if directories && info.IsDir() {
			candidates = append(candidates, pruneCandidate{name: entry.Name(), info: info})
		}
		if !directories && info.Mode().IsRegular() {
			candidates = append(candidates, pruneCandidate{name: entry.Name(), info: info})
		}
	}
	return candidates, nil
}

func sortNewestFirst(candidates []pruneCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].info.ModTime().Equal(candidates[j].info.ModTime()) {
			return candidates[i].name > candidates[j].name
		}
		return candidates[i].info.ModTime().After(candidates[j].info.ModTime())
	})
}

func countRegularTree(root *os.Root) (int, error) {
	count := 0
	err := fs.WalkDir(root.FS(), ".", func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			count++
		}
		return nil
	})
	return count, err
}

func regularTreeBytes(root *os.Root) (int64, error) {
	var total int64
	err := fs.WalkDir(root.FS(), ".", func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	return total, err
}

func (runner *pruneAgentsRunner) display(relativePath string) string {
	return filepath.Join(runner.opts.RepoRoot, relativePath)
}

func (runner *pruneAgentsRunner) unsafe(format string, args ...any) error {
	return fmt.Errorf("prune agents: refusing to prune: "+format, args...)
}

func numfmtSize(bytes int64) string {
	switch {
	case bytes >= 1<<30:
		return fmt.Sprintf("%dGB", bytes/(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%dMB", bytes/(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%dKB", bytes/(1<<10))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
