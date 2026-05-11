// practices: [microservices, team-topologies]
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func sanitizeGitProcessEnv() error {
	for _, key := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR"} {
		if err := os.Unsetenv(key); err != nil {
			return fmt.Errorf("unset %s: %w", key, err)
		}
	}
	return nil
}

func repairSharedCoreWorktreeConfig(cwd string) error {
	if strings.TrimSpace(cwd) == "" {
		return nil
	}

	commonGitDir, err := gitCommonDir(cwd)
	if err != nil {
		return nil
	}
	sharedConfigPath := filepath.Join(commonGitDir, "config")

	sharedCoreWorktree, err := gitOutputFromConfigFile(sharedConfigPath, "--get", "core.worktree")
	if err != nil || strings.TrimSpace(sharedCoreWorktree) == "" {
		return nil
	}

	worktrees, err := listGitWorktrees(cwd)
	if err != nil {
		return fmt.Errorf("inspect git worktrees: %w", err)
	}
	if len(worktrees) <= 1 {
		return nil
	}

	if err := runGitWithConfigFile(sharedConfigPath, "extensions.worktreeConfig", "true"); err != nil {
		return fmt.Errorf("enable worktree config: %w", err)
	}

	for _, worktreePath := range worktrees {
		if err := runGitInDir(worktreePath, "config", "--worktree", "core.worktree", worktreePath); err != nil {
			return fmt.Errorf("set worktree-local core.worktree for %s: %w", worktreePath, err)
		}
	}

	if err := runGitWithConfigFile(sharedConfigPath, "--unset-all", "core.worktree"); err != nil {
		return fmt.Errorf("remove shared core.worktree: %w", err)
	}

	return nil
}

func listGitWorktrees(cwd string) ([]string, error) {
	out, err := gitOutputInDir(cwd, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}

	var worktrees []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}
		worktreePath, err := filepath.Abs(strings.TrimSpace(strings.TrimPrefix(line, "worktree ")))
		if err != nil {
			return nil, err
		}
		worktrees = append(worktrees, worktreePath)
	}
	return worktrees, nil
}

func gitOutputInDir(cwd string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	cmd.Env = gitDiscoveryEnv()
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitCommonDir(cwd string) (string, error) {
	out, err := gitOutputInDir(cwd, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(out) {
		return filepath.Clean(out), nil
	}
	return filepath.Abs(filepath.Join(cwd, out))
}

func gitOutputFromConfigFile(configPath string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"config", "--file", configPath}, args...)...)
	cmd.Env = gitDiscoveryEnv()
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func runGitInDir(cwd string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	cmd.Env = gitDiscoveryEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func runGitWithConfigFile(configPath string, args ...string) error {
	cmd := exec.Command("git", append([]string{"config", "--file", configPath}, args...)...)
	cmd.Env = gitDiscoveryEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git config --file %s %s: %w (%s)", configPath, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
