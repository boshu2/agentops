package gates

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Scope selects which git diff defines the change set.
type Scope string

const (
	// ScopeHead is the files in the HEAD commit.
	ScopeHead Scope = "head"
	// ScopeStaged is staged (cached) changes.
	ScopeStaged Scope = "staged"
	// ScopeWorktree is staged + unstaged changes vs HEAD.
	ScopeWorktree Scope = "worktree"
	// ScopeUpstream is the diff vs the upstream merge base (CI default).
	ScopeUpstream Scope = "upstream"
)

// ChangedFilesPort reports the set of changed files (repo-relative, deduped)
// for a scope.
type ChangedFilesPort interface {
	Changed(ctx context.Context, scope Scope) ([]string, error)
}

// gitRunner runs a git subcommand in a repo and returns stdout. It is a field
// so tests can inject a fake without shelling out.
type gitRunner func(ctx context.Context, repoRoot string, args ...string) (string, error)

// GitChangedFiles is the production ChangedFilesPort backed by git.
type GitChangedFiles struct {
	RepoRoot string
	run      gitRunner
}

// NewGitChangedFiles returns a ChangedFilesPort rooted at repoRoot.
func NewGitChangedFiles(repoRoot string) *GitChangedFiles {
	return &GitChangedFiles{RepoRoot: repoRoot}
}

func (g *GitChangedFiles) exec(ctx context.Context, args ...string) (string, error) {
	if g.run != nil {
		return g.run(ctx, g.RepoRoot, args...)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.RepoRoot
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

// Changed returns the deduped repo-relative paths changed for the scope.
func (g *GitChangedFiles) Changed(ctx context.Context, scope Scope) ([]string, error) {
	args, err := scopeArgs(scope)
	if err != nil {
		return nil, err
	}
	out, err := g.exec(ctx, args...)
	if err != nil {
		return nil, err
	}
	return dedupeLines(out), nil
}

// scopeArgs maps a Scope to its git diff arguments.
func scopeArgs(scope Scope) ([]string, error) {
	switch scope {
	case ScopeHead:
		return []string{"show", "--name-only", "--pretty=format:", "HEAD"}, nil
	case ScopeStaged:
		return []string{"diff", "--name-only", "--cached"}, nil
	case ScopeWorktree:
		return []string{"diff", "--name-only", "HEAD"}, nil
	case ScopeUpstream:
		return []string{"diff", "--name-only", "@{upstream}...HEAD"}, nil
	default:
		return nil, fmt.Errorf("gates: unknown scope %q", scope)
	}
}

// dedupeLines splits git output into trimmed, non-empty, deduplicated lines,
// preserving first-seen order.
func dedupeLines(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, ln := range strings.Split(s, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || seen[ln] {
			continue
		}
		seen[ln] = true
		out = append(out, ln)
	}
	return out
}

var _ ChangedFilesPort = (*GitChangedFiles)(nil)
