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

	// ScopeRangePrefix marks an explicit revision-range scope written
	// "range:<base>..<head>". It routes on `git diff --name-only <base>..<head>`.
	// Landing loops need it because a detached landing worktree has no upstream:
	// the upstream/head fallbacks then see only the tip commit, so a c1+c2 train
	// whose tests land in c2 falsely fails a co-change gate at c1. An explicit
	// range spans the whole train.
	ScopeRangePrefix = "range:"
)

// ScopeRange reports whether scope is a range scope ("range:<base>..<head>")
// and returns the "<base>..<head>" revision spec verbatim. Callers validate the
// spec with ValidateRangeSpec before use.
func ScopeRange(scope Scope) (spec string, ok bool) {
	s := string(scope)
	if !strings.HasPrefix(s, ScopeRangePrefix) {
		return "", false
	}
	return strings.TrimPrefix(s, ScopeRangePrefix), true
}

// ValidateRangeSpec checks a range scope's "<base>..<head>" spec is well-formed
// enough to hand to `git diff`: non-empty and containing the ".." range
// operator. It rejects a bare rev (e.g. "range:HEAD"), which git would
// otherwise interpret as a diff against the working tree — a silent footgun.
func ValidateRangeSpec(spec string) error {
	if strings.TrimSpace(spec) == "" {
		return fmt.Errorf("gates: range scope requires <base>..<head>, got empty range")
	}
	if !strings.Contains(spec, "..") {
		return fmt.Errorf("gates: range scope %q must be a <base>..<head> revision range", spec)
	}
	return nil
}

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
		if spec, ok := ScopeRange(scope); ok {
			if err := ValidateRangeSpec(spec); err != nil {
				return nil, err
			}
			return []string{"diff", "--name-only", spec}, nil
		}
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
