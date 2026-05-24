// Package workspace_git is a real adapter for the WorkspacePort port,
// backed by `git worktree` subprocess calls.
//
// It absorbs the worktree-lifecycle coupling that the CLI previously reached
// by building its own exec.Command("git", "worktree", ...) at each callsite.
// Setup maps to `git worktree add`; Cleanup maps to `git worktree remove`
// (with optional branch delete + prune).
//
// Request contract (in addition to ports.WorkspacePort's WorkspaceID/Status
// contract):
//
//   - Path is the absolute or repo-relative worktree path. Required for Setup
//     and Cleanup (a worktree cannot be created or removed without a path).
//   - Metadata["branch"]   — branch name. Setup uses `add -b <branch>` when set,
//     plain `add <path>` otherwise. Cleanup deletes the branch when set.
//   - Metadata["base"]     — base ref for Setup's new branch (e.g. "origin/main").
//     Ignored when branch is empty.
//   - Metadata["force"]    — "true" makes Cleanup pass --force (default true; an
//     explicit "false" omits it).
//   - Metadata["prune"]    — "true" runs `git worktree prune` after Cleanup.
package workspace_git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// ErrMissingPath is returned when a WorkspaceRequest omits Path for an
// operation that requires it. Setup cannot create and Cleanup cannot remove a
// worktree without a path.
var ErrMissingPath = errors.New("workspace_git: workspace path required")

// Adapter satisfies ports.WorkspacePort using `git worktree` subprocess calls.
// RepoRoot, when set, becomes the working directory for every git invocation so
// the adapter operates against a known repository regardless of process cwd.
type Adapter struct {
	// RepoRoot is the directory git commands run in. Empty means the current
	// process working directory (git discovers the repo from there).
	RepoRoot string
}

// New returns an Adapter rooted at repoRoot. Pass "" to run git against the
// process working directory.
func New(repoRoot string) *Adapter {
	return &Adapter{RepoRoot: repoRoot}
}

// Compile-time interface check.
var _ ports.WorkspacePort = (*Adapter)(nil)

// Setup creates a git worktree for req. WorkspaceID and Path are required.
// When Metadata["branch"] is set, the worktree is created on a new branch
// (`git worktree add -b <branch> <path> [<base>]`); otherwise a plain
// `git worktree add <path>` is run.
func (a *Adapter) Setup(ctx context.Context, req ports.WorkspaceRequest) (ports.WorkspaceResult, error) {
	if err := ctx.Err(); err != nil {
		return ports.WorkspaceResult{}, err
	}
	if req.WorkspaceID == "" {
		return ports.WorkspaceResult{}, errors.New("workspace_git: Setup workspace id required")
	}
	if req.Path == "" {
		return ports.WorkspaceResult{}, fmt.Errorf("%w: Setup", ErrMissingPath)
	}

	branch := req.Metadata["branch"]
	base := req.Metadata["base"]

	args := []string{"worktree", "add"}
	if branch != "" {
		args = append(args, "-b", branch, req.Path)
		if base != "" {
			args = append(args, base)
		}
	} else {
		args = append(args, req.Path)
	}

	if out, err := a.run(ctx, args...); err != nil {
		return ports.WorkspaceResult{}, fmt.Errorf("workspace_git: git worktree add %s: %s: %w", req.Path, strings.TrimSpace(out), err)
	}

	return ports.WorkspaceResult{
		WorkspaceID: req.WorkspaceID,
		Path:        req.Path,
		Status:      "setup",
		Reason:      "git worktree created",
	}, nil
}

// Cleanup removes the git worktree for req. WorkspaceID and Path are required.
// It runs `git worktree remove [--force] <path>`; on failure it falls back to
// removing the directory directly so cleanup is best-effort and not blocked by
// an already-pruned worktree. When Metadata["branch"] is set it best-effort
// deletes the branch, and when Metadata["prune"] is "true" it prunes
// administrative worktree state afterward.
func (a *Adapter) Cleanup(ctx context.Context, req ports.WorkspaceRequest) (ports.WorkspaceResult, error) {
	if err := ctx.Err(); err != nil {
		return ports.WorkspaceResult{}, err
	}
	if req.WorkspaceID == "" {
		return ports.WorkspaceResult{}, errors.New("workspace_git: Cleanup workspace id required")
	}
	if req.Path == "" {
		return ports.WorkspaceResult{}, fmt.Errorf("%w: Cleanup", ErrMissingPath)
	}

	args := []string{"worktree", "remove"}
	if req.Metadata["force"] != "false" {
		args = append(args, "--force")
	}
	args = append(args, req.Path)

	reason := "git worktree removed"
	if out, err := a.run(ctx, args...); err != nil {
		// Fall back to a direct directory removal so an already-pruned
		// worktree does not leave the path on disk.
		if rmErr := os.RemoveAll(req.Path); rmErr != nil {
			return ports.WorkspaceResult{}, fmt.Errorf("workspace_git: git worktree remove %s: %s; manual rm: %w", req.Path, strings.TrimSpace(out), rmErr)
		}
		reason = "worktree directory removed (git worktree remove failed, fell back to rm)"
	}

	// Best-effort branch delete.
	if branch := req.Metadata["branch"]; branch != "" {
		_, _ = a.run(ctx, "branch", "-D", branch)
	}

	// Optional prune of administrative worktree state.
	if req.Metadata["prune"] == "true" {
		if out, err := a.run(ctx, "worktree", "prune"); err != nil {
			return ports.WorkspaceResult{}, fmt.Errorf("workspace_git: git worktree prune: %s: %w", strings.TrimSpace(out), err)
		}
	}

	return ports.WorkspaceResult{
		WorkspaceID: req.WorkspaceID,
		Path:        req.Path,
		Status:      "cleanup",
		Reason:      reason,
	}, nil
}

// run invokes git with args, returning combined output. The working directory
// is a.RepoRoot when set. GIT_DIR/GIT_WORK_TREE/GIT_COMMON_DIR are stripped from
// the environment so git rediscovers the repository from RepoRoot/cwd rather
// than inheriting a stale ambient pointer (mirrors cmd/ao's gitDiscoveryEnv).
func (a *Adapter) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if a.RepoRoot != "" {
		cmd.Dir = a.RepoRoot
	}
	cmd.Env = discoveryEnv()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// discoveryEnv returns the current environment with git repo-discovery
// overrides removed so git resolves the repo from the command's working
// directory.
func discoveryEnv() []string {
	src := os.Environ()
	env := make([]string, 0, len(src))
	for _, entry := range src {
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
