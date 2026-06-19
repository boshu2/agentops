package rpi

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CollectSearchRoots returns the cwd plus any Git worktree roots attached to
// the same repository. Run-discovery (next-work proof classification, status)
// scans .agents/rpi state under every attached worktree, so the root set must
// span the whole worktree fan-out — not just the current directory.
//
// Migrated out of cmd/ao (rpi_status.go) so the keeper commands and the
// next-work-proof reader stay self-contained after the ao rpi command surface
// is removed (ADR-0009 teardown, age-3pdt). Logic is unchanged.
func CollectSearchRoots(cwd string) []string {
	roots := []string{}
	seen := make(map[string]struct{})

	TryAddSearchRoot(cwd, seen, &roots)

	if discovered := discoverGitWorktreeRoots(cwd); len(discovered) > 0 {
		for _, root := range discovered {
			TryAddSearchRoot(root, seen, &roots)
		}
		return roots
	}

	// Backward-compatible fallback: sibling *-rpi-* pattern.
	parent := filepath.Dir(cwd)
	pattern := filepath.Join(parent, "*-rpi-*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return roots
	}
	for _, m := range matches {
		TryAddSearchRoot(m, seen, &roots)
	}
	return roots
}

// TryAddSearchRoot normalizes and validates a path, then appends it to roots
// if it is a valid, unseen directory.
func TryAddSearchRoot(path string, seen map[string]struct{}, roots *[]string) {
	if path == "" {
		return
	}
	normalized := NormalizeSearchRootPath(path)
	if _, ok := seen[normalized]; ok {
		return
	}
	info, err := os.Stat(normalized)
	if err != nil || !info.IsDir() {
		return
	}
	stored := filepath.Clean(path)
	if abs, err := filepath.Abs(stored); err == nil {
		stored = filepath.Clean(abs)
	}
	seen[normalized] = struct{}{}
	*roots = append(*roots, stored)
}

// NormalizeSearchRootPath resolves symlinks and absolutizes a path so distinct
// spellings of the same directory dedupe to one search root.
func NormalizeSearchRootPath(path string) string {
	clean := filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(clean); err == nil && resolved != "" {
		return filepath.Clean(resolved)
	}
	if abs, err := filepath.Abs(clean); err == nil {
		return filepath.Clean(abs)
	}
	return clean
}

func discoverGitWorktreeRoots(cwd string) []string {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var roots []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}
		path := strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
		if path == "" {
			continue
		}
		roots = append(roots, path)
	}
	return roots
}
