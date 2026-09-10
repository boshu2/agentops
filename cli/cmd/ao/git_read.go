package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// gitDiscoveryEnv strips GIT_DIR/GIT_WORK_TREE/GIT_COMMON_DIR/GIT_INDEX_FILE
// from the environment so git discovery resolves the repository from the
// working directory, never from a leaked parent-process override. A hook-leaked
// GIT_DIR is not just a read hazard: a "scoped" `git -C <dir> config ...` under
// it writes the LEAKED repo's shared config (age-gate-scripts-worktree-gitdir-p62wo).
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
		case strings.HasPrefix(entry, "GIT_INDEX_FILE="):
			continue
		default:
			env = append(env, entry)
		}
	}
	return env
}

// gitChangedFiles includes tracked and untracked work for handoff evidence.
// A failed observation is distinct from a successfully observed clean tree.
func gitChangedFiles(cwd string, limit int) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()
	command := exec.CommandContext(ctx, "git", "status", "--porcelain=v1", "-z", "--untracked-files=all")
	command.Dir = cwd
	command.Env = gitDiscoveryEnv()
	out, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("observe Git status: %w", err)
	}
	files, err := parseGitStatus(string(out))
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(files) > limit {
		files = files[:limit]
	}
	return files, nil
}

func parseGitStatus(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}
	if !strings.HasSuffix(raw, "\x00") {
		return nil, fmt.Errorf("unterminated Git status record")
	}
	records := strings.Split(strings.TrimSuffix(raw, "\x00"), "\x00")
	var paths []string
	for i := 0; i < len(records); i++ {
		record := records[i]
		if len(record) < 4 || record[2] != ' ' {
			return nil, fmt.Errorf("invalid Git status record")
		}
		paths = append(paths, record[3:])
		// Porcelain -z emits a rename/copy destination followed by the
		// original path as a separate NUL-delimited field without a status.
		if strings.ContainsAny(record[:2], "RC") {
			i++
			if i == len(records) || records[i] == "" {
				return nil, fmt.Errorf("missing Git rename/copy source")
			}
			paths = append(paths, records[i])
		}
	}
	return paths, nil
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
