package main

import (
	"fmt"
	"os/exec"
	"strings"
)

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
