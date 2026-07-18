package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// gitDiscoveryEnv strips GIT_DIR/GIT_WORK_TREE/GIT_COMMON_DIR from the
// environment so read-only git discovery resolves the repository from the
// working directory, never from a leaked parent-process override.
func gitDiscoveryEnv() []string {
	env := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		switch {
		case strings.HasPrefix(entry, "GIT_DIR="):
			continue
		case strings.HasPrefix(entry, "GIT_WORK_TREE="):
			continue
		case strings.HasPrefix(entry, "GIT_COMMON_DIR="):
			continue
		default:
			env = append(env, entry)
		}
	}
	return env
}

// gitChangedFiles lists worktree-modified paths (read-only, bounded) for
// handoff evidence. Returns nil when git is unavailable or the tree is clean.
func gitChangedFiles(cwd string, limit int) []string {
	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()
	command := exec.CommandContext(ctx, "git", "diff", "--name-only", "HEAD")
	command.Dir = cwd
	command.Env = gitDiscoveryEnv()
	out, err := command.Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if limit > 0 && len(lines) > limit {
		lines = lines[:limit]
	}
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			result = append(result, line)
		}
	}
	return result
}

// resolveRepoRoot is read-only discovery. AgentOps does not mutate Git state.
func resolveRepoRoot(cwd string) (string, error) {
	command := exec.Command("git", "rev-parse", "--show-toplevel")
	command.Dir = cwd
	command.Env = gitDiscoveryEnv()
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("resolve git repo root: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// getCurrentBranch is optional read-only metadata for handoff artifacts.
func getCurrentBranch(cwd string) (string, error) {
	command := exec.Command("git", "branch", "--show-current")
	command.Dir = cwd
	command.Env = gitDiscoveryEnv()
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("read current branch: %w", err)
	}
	branch := strings.TrimSpace(string(output))
	if branch == "" {
		return "", fmt.Errorf("detached HEAD")
	}
	return branch, nil
}
