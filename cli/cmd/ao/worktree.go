// practices: [microservices, team-topologies]
package main

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	worktreeGCStaleAfter   time.Duration
	worktreeGCPrune        bool
	worktreeGCCleanTmux    bool
	worktreeGCIncludeDirty bool
)

var tmuxRPISessionPattern = regexp.MustCompile(`^ao-rpi-(.+)-p[1-3]$`)

type worktreeGCCandidate struct {
	RunID     string
	Path      string
	Reference time.Time
	Dirty     bool
}

type tmuxSessionMeta struct {
	Name      string
	RunID     string
	CreatedAt time.Time
}

// worktreeCmd is the parent for worktree maintenance subcommands.
var worktreeCmd = &cobra.Command{
	Use:   "worktree",
	Short: "Worktree maintenance utilities",
}

func init() {
	worktreeGCCmd := &cobra.Command{
		Use:   "gc",
		Short: "Garbage-collect stale RPI worktrees and orphaned tmux sessions",
		Long: `Safely remove stale AgentOps RPI worktrees and orphaned tmux sessions.

Safety defaults:
  - Only considers worktrees/sessions older than --stale-after
  - Skips worktrees with uncommitted changes
  - Skips runs that still appear active`,
		RunE: runWorktreeGC,
	}

	worktreeGCCmd.Flags().DurationVar(&worktreeGCStaleAfter, "stale-after", 24*time.Hour, "Only clean worktrees/sessions older than this age")
	worktreeGCCmd.Flags().BoolVar(&worktreeGCPrune, "prune", true, "Run 'git worktree prune' after cleanup")
	worktreeGCCmd.Flags().BoolVar(&worktreeGCCleanTmux, "clean-tmux", true, "Clean stale ao-rpi tmux sessions without active run/worktree")
	worktreeGCCmd.Flags().BoolVar(&worktreeGCIncludeDirty, "include-dirty", false, "Also clean worktrees with uncommitted changes (unsafe)")

	worktreeCmd.AddCommand(worktreeGCCmd)
	worktreeCmd.Hidden = true
	worktreeCmd.GroupID = "config"
	rootCmd.AddCommand(worktreeCmd)
}

// finalizeWorktreeGC handles pruning and prints the summary message.
func finalizeWorktreeGC(repoRoot string, candidateCount, removed, killedSessions, tmuxCandidates int) {
	if worktreeGCPrune && !GetDryRun() {
		if err := pruneWorktrees(repoRoot); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: git worktree prune failed: %v\n", err)
		}
	}

	if GetDryRun() {
		fmt.Printf("[dry-run] Worktree GC complete. worktree_candidates=%d tmux_candidates=%d\n", candidateCount, tmuxCandidates)
		return
	}
	fmt.Printf("Worktree GC complete. removed=%d tmux_killed=%d\n", removed, killedSessions)
}

func runWorktreeGC(cmd *cobra.Command, args []string) error {
	if worktreeGCStaleAfter <= 0 {
		return fmt.Errorf("--stale-after must be > 0")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	repoRoot, err := resolveRepoRoot(cwd)
	if err != nil {
		return err
	}

	now := time.Now()
	activeRuns := discoverActiveRPIRuns(repoRoot)
	candidates, liveWorktreeRuns, skippedDirty, err := findStaleRPISiblingWorktrees(
		repoRoot,
		now,
		worktreeGCStaleAfter,
		activeRuns,
		worktreeGCIncludeDirty,
	)
	if err != nil {
		return err
	}

	for _, path := range skippedDirty {
		fmt.Printf("Skipped dirty worktree: %s\n", path)
	}

	removed := gcWorktreeCandidates(candidates, liveWorktreeRuns, repoRoot, now)

	killedSessions, tmuxCandidates := 0, 0
	if worktreeGCCleanTmux {
		killedSessions, tmuxCandidates = gcTmuxSessions(now, activeRuns, liveWorktreeRuns)
	}

	finalizeWorktreeGC(repoRoot, len(candidates), removed, killedSessions, tmuxCandidates)
	return nil
}

// gcWorktreeCandidates removes stale worktrees, printing dry-run or result messages.
func gcWorktreeCandidates(candidates []worktreeGCCandidate, liveWorktreeRuns map[string]bool, repoRoot string, now time.Time) int {
	if len(candidates) == 0 {
		fmt.Println("No stale RPI worktrees found.")
		return 0
	}
	removed := 0
	for _, candidate := range candidates {
		age := now.Sub(candidate.Reference).Truncate(time.Second)
		if GetDryRun() {
			fmt.Printf("[dry-run] Would remove worktree: %s (run=%s age=%s dirty=%t)\n",
				candidate.Path, candidate.RunID, age, candidate.Dirty)
			continue
		}
		if err := removeOrphanedWorktree(repoRoot, candidate.Path, candidate.RunID); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove worktree %s: %v\n", candidate.Path, err)
			continue
		}
		fmt.Printf("Removed worktree: %s (run=%s age=%s)\n", candidate.Path, candidate.RunID, age)
		removed++
		delete(liveWorktreeRuns, candidate.RunID)
	}
	return removed
}

// gcTmuxSessions finds and kills stale RPI tmux sessions, returning (killed, candidates) counts.
func gcTmuxSessions(now time.Time, activeRuns, liveWorktreeRuns map[string]bool) (killed, candidates int) {
	staleSessions, err := findStaleRPITmuxSessions(now, worktreeGCStaleAfter, activeRuns, liveWorktreeRuns)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to inspect tmux sessions: %v\n", err)
		return 0, 0
	}
	candidates = len(staleSessions)
	for _, sess := range staleSessions {
		age := now.Sub(sess.CreatedAt).Truncate(time.Second)
		if GetDryRun() {
			fmt.Printf("[dry-run] Would kill tmux session: %s (run=%s age=%s)\n", sess.Name, sess.RunID, age)
			continue
		}
		if err := killTmuxSession(sess.Name); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to kill tmux session %s: %v\n", sess.Name, err)
			continue
		}
		fmt.Printf("Killed tmux session: %s (run=%s age=%s)\n", sess.Name, sess.RunID, age)
		killed++
	}
	return killed, candidates
}

func resolveRepoRoot(cwd string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = cwd
	cmd.Env = gitDiscoveryEnv()
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("resolve git repo root: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func discoverActiveRPIRuns(repoRoot string) map[string]bool {
	activeRuns := make(map[string]bool)
	for _, run := range discoverRPIRuns(repoRoot) {
		if run.RunID != "" && run.IsActive {
			activeRuns[run.RunID] = true
		}
	}
	return activeRuns
}

func findStaleRPISiblingWorktrees(repoRoot string, now time.Time, staleAfter time.Duration, activeRuns map[string]bool, includeDirty bool) (
	[]worktreeGCCandidate,
	map[string]bool,
	[]string,
	error,
) {
	paths, err := findRPISiblingWorktreePaths(repoRoot)
	if err != nil {
		return nil, nil, nil, err
	}

	liveWorktreeRuns := make(map[string]bool)
	var candidates []worktreeGCCandidate
	var skippedDirty []string

	for _, path := range paths {
		runID := runIDFromWorktreePath(repoRoot, path)
		if runID == "" {
			continue
		}
		liveWorktreeRuns[runID] = true
		if activeRuns[runID] {
			continue
		}

		reference := worktreeReferenceTime(path)
		if now.Sub(reference) < staleAfter {
			continue
		}

		dirty, err := isWorktreeDirty(path)
		if err != nil {
			// Be conservative: if git status fails, do not delete.
			continue
		}
		if dirty && !includeDirty {
			skippedDirty = append(skippedDirty, path)
			continue
		}

		candidates = append(candidates, worktreeGCCandidate{
			RunID:     runID,
			Path:      path,
			Reference: reference,
			Dirty:     dirty,
		})
	}

	slices.SortFunc(candidates, func(a, b worktreeGCCandidate) int { return cmp.Compare(a.Path, b.Path) })
	sort.Strings(skippedDirty)
	return candidates, liveWorktreeRuns, skippedDirty, nil
}

// findRPISiblingWorktreePaths returns sibling worktree paths that one of
// AgentOps's orchestrators may have created. Multiple naming schemes are
// scanned because different orchestrators choose different layouts:
//
//	<repo>-rpi-<id>             ao rpi phased worktrees
//	<repo>-w<N>-<slice>         agentops:crank wave isolation (sibling form)
//	<repo>-d-<slice>            agentops:crank discovery slice (sibling form)
//	<repo>-worktrees/wave-<x>   agentops:crank documented path
func findRPISiblingWorktreePaths(repoRoot string) ([]string, error) {
	base := filepath.Base(repoRoot)
	parent := filepath.Dir(repoRoot)
	patterns := []string{
		filepath.Join(parent, base+"-rpi-*"),
		filepath.Join(parent, base+"-w[0-9]*-*"),
		filepath.Join(parent, base+"-d-*"),
		filepath.Join(parent, base+"-worktrees", "wave-*"),
	}

	seen := make(map[string]struct{})
	var paths []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil || !info.IsDir() {
				continue
			}
			if _, ok := seen[match]; ok {
				continue
			}
			seen[match] = struct{}{}
			paths = append(paths, match)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// waveSiblingPattern matches the per-worker suffix for crank wave worktrees,
// e.g. "w3-zeus" or "w12-formula". Used by runIDFromWorktreePath.
var waveSiblingPattern = regexp.MustCompile(`^w[0-9]+-.+$`)

// runIDFromWorktreePath extracts a stable run identifier from a managed
// worktree path. Returns "" if the path doesn't match any known orchestrator
// naming scheme.
func runIDFromWorktreePath(repoRoot, worktreePath string) string {
	parent := filepath.Dir(repoRoot)
	base := filepath.Base(repoRoot)

	// Wave-pack pattern: <parent>/<base>-worktrees/wave-<slug>
	if filepath.Dir(worktreePath) == filepath.Join(parent, base+"-worktrees") {
		name := filepath.Base(worktreePath)
		if strings.HasPrefix(name, "wave-") && len(name) > len("wave-") {
			return name
		}
		return ""
	}

	// Sibling patterns: <parent>/<base>-{rpi|w<N>|d}-<...>
	name := filepath.Base(worktreePath)
	repoPrefix := base + "-"
	if !strings.HasPrefix(name, repoPrefix) {
		return ""
	}
	suffix := strings.TrimPrefix(name, repoPrefix)
	if suffix == "" {
		return ""
	}

	if strings.HasPrefix(suffix, "rpi-") {
		id := strings.TrimPrefix(suffix, "rpi-")
		if id == "" {
			return ""
		}
		return id
	}
	if strings.HasPrefix(suffix, "d-") && len(suffix) > len("d-") {
		return suffix
	}
	if waveSiblingPattern.MatchString(suffix) {
		return suffix
	}
	return ""
}

func worktreeReferenceTime(worktreePath string) time.Time {
	best := time.Time{}
	candidates := []string{
		filepath.Join(worktreePath, ".agents", "rpi", "phased-state.json"),
		filepath.Join(worktreePath, ".agents", "rpi", "live-status.md"),
		worktreePath,
	}
	for _, path := range candidates {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.ModTime().After(best) {
			best = info.ModTime()
		}
	}
	if best.IsZero() {
		return time.Unix(0, 0)
	}
	return best
}

// runtimeOnlyPathPrefixes are paths that orchestrators (ao, bd,
// agentops:crank) modify as runtime state, not as user work-in-progress.
// Dirty entries confined to these prefixes don't represent meaningful
// uncommitted changes and shouldn't block worktree gc.
var runtimeOnlyPathPrefixes = []string{
	".beads/",
	".agents/ao/sessions/",
	".agents/ao/baselines/",
	".agents/ao/goals/baselines/",
	".agents/crank/",
	".agents/rpi/",
}

func isWorktreeDirty(worktreePath string) (bool, error) {
	// -uall expands untracked directories so each file appears with its
	// full path. Required for hasNonRuntimeDirtyEntries to apply the
	// runtime-prefix filter accurately (e.g. `.agents/ao/sessions/x.jsonl`
	// instead of just `.agents/`).
	cmd := exec.Command("git", "-C", worktreePath, "status", "--porcelain", "-uall")
	cmd.Env = gitDiscoveryEnv()
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git status --porcelain in %s: %w", worktreePath, err)
	}
	return hasNonRuntimeDirtyEntries(string(out)), nil
}

// hasNonRuntimeDirtyEntries reports whether git-status-porcelain output
// contains at least one entry whose path is NOT confined to a runtime-only
// directory. Empty/blank output is treated as clean.
func hasNonRuntimeDirtyEntries(porcelainOutput string) bool {
	for _, line := range strings.Split(porcelainOutput, "\n") {
		path := porcelainEntryPath(line)
		if path == "" {
			continue
		}
		if !isRuntimeOnlyPath(path) {
			return true
		}
	}
	return false
}

// porcelainEntryPath extracts the affected path from one line of git
// status --porcelain output. Returns "" for blank or malformed lines.
// For renames ("R  old -> new"), returns the destination path.
func porcelainEntryPath(line string) string {
	// Format: "XY path" — XY is two status chars, then a space, then path.
	if len(line) < 4 {
		return ""
	}
	rest := line[3:]
	if idx := strings.Index(rest, " -> "); idx >= 0 {
		rest = rest[idx+4:]
	}
	return strings.TrimSpace(rest)
}

func isRuntimeOnlyPath(path string) bool {
	for _, prefix := range runtimeOnlyPathPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func findStaleRPITmuxSessions(now time.Time, staleAfter time.Duration, activeRuns, liveWorktreeRuns map[string]bool) ([]tmuxSessionMeta, error) {
	sessions, err := listRPITmuxSessions()
	if err != nil {
		return nil, err
	}

	var stale []tmuxSessionMeta
	for _, sess := range sessions {
		if !shouldCleanupRPITmuxSession(sess.RunID, sess.CreatedAt, now, staleAfter, activeRuns, liveWorktreeRuns) {
			continue
		}
		stale = append(stale, sess)
	}
	slices.SortFunc(stale, func(a, b tmuxSessionMeta) int { return cmp.Compare(a.Name, b.Name) })
	return stale, nil
}

func shouldCleanupRPITmuxSession(runID string, createdAt, now time.Time, staleAfter time.Duration, activeRuns, liveWorktreeRuns map[string]bool) bool {
	if runID == "" {
		return false
	}
	if activeRuns[runID] {
		return false
	}
	if liveWorktreeRuns[runID] {
		return false
	}
	if now.Sub(createdAt) < staleAfter {
		return false
	}
	return true
}

func listRPITmuxSessions() ([]tmuxSessionMeta, error) {
	if _, err := exec.LookPath("tmux"); err != nil {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "tmux", "list-sessions", "-F", "#{session_name}\t#{session_created}")
	out, err := cmd.Output()
	if err != nil {
		// Fail-open: treat as no sessions.
		return nil, nil
	}
	return parseTmuxSessionListOutput(string(out)), nil
}

func parseTmuxSessionListOutput(output string) []tmuxSessionMeta {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var sessions []tmuxSessionMeta
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 2 {
			continue
		}
		sessionName := strings.TrimSpace(fields[0])
		runID, ok := parseRPITmuxSessionRunID(sessionName)
		if !ok {
			continue
		}
		createdEpoch, err := strconv.ParseInt(strings.TrimSpace(fields[1]), 10, 64)
		if err != nil {
			continue
		}
		sessions = append(sessions, tmuxSessionMeta{
			Name:      sessionName,
			RunID:     runID,
			CreatedAt: time.Unix(createdEpoch, 0),
		})
	}
	return sessions
}

func parseRPITmuxSessionRunID(sessionName string) (string, bool) {
	matches := tmuxRPISessionPattern.FindStringSubmatch(sessionName)
	if len(matches) != 2 {
		return "", false
	}
	runID := strings.TrimSpace(matches[1])
	if runID == "" {
		return "", false
	}
	return runID, true
}

func killTmuxSession(sessionName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tmux", "kill-session", "-t", sessionName)
	return cmd.Run()
}
