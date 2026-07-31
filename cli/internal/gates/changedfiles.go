package gates

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// gitDiscoveryEnvVars is the canonical set of git repository-discovery
// environment variables. Git injects these into hook-launched processes
// (GIT_DIR points at the triggering repo's gitdir, GIT_WORK_TREE at its
// worktree, etc.). A subprocess that inherits them resolves git operations
// against the WRONG repository even when cmd.Dir names the intended one —
// GIT_DIR overrides cwd-based discovery. This is the exact 7-var set scrubbed
// by scripts/lib/repo-root.sh so the Go and shell paths stay in lockstep.
var gitDiscoveryEnvVars = []string{
	"GIT_DIR",
	"GIT_WORK_TREE",
	"GIT_INDEX_FILE",
	"GIT_PREFIX",
	"GIT_OBJECT_DIRECTORY",
	"GIT_COMMON_DIR",
	"GIT_NAMESPACE",
}

// ScrubbedGitEnv returns a copy of the current process environment with git's
// hook-injected repository-discovery variables (gitDiscoveryEnvVars) removed.
// Every git subprocess in the gates package MUST set cmd.Env to this so a
// leaked GIT_DIR cannot route the changed-set computation at the wrong repo —
// a leaked GIT_DIR silently skips blocking checks (SECURITY-MED). cmd.Dir then
// resolves the intended repo via normal cwd-based discovery.
func ScrubbedGitEnv() []string {
	return scrubGitEnv(os.Environ())
}

// scrubGitEnv filters gitDiscoveryEnvVars out of a KEY=VALUE environment slice.
func scrubGitEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if isGitDiscoveryVar(kv) {
			continue
		}
		out = append(out, kv)
	}
	return out
}

func isGitDiscoveryVar(kv string) bool {
	for _, name := range gitDiscoveryEnvVars {
		if strings.HasPrefix(kv, name+"=") {
			return true
		}
	}
	return false
}

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

// ErrUnbornHead is the sentinel returned when a scope needs a commit to
// compare against but the repository has none yet (an "unborn HEAD": `git
// init` with no commit, or a fresh orphan branch). Without it the caller sees
// git's raw plumbing failure — "git show --name-only --pretty=format: HEAD:
// exit status 128" — which reads as a broken tool rather than the ordinary,
// self-inflicted state it is. Callers match with errors.Is.
var ErrUnbornHead = errors.New("no commits yet (unborn HEAD)")

// scopeNeedsHead reports whether a scope's git invocation resolves HEAD and so
// cannot work before the first commit. `staged` is deliberately excluded: `git
// diff --cached` compares the index against the empty tree when HEAD is
// unborn, which is exactly the pre-first-commit escape hatch the friendly
// message points at.
func scopeNeedsHead(scope Scope) bool {
	switch scope {
	case ScopeHead, ScopeWorktree, ScopeUpstream:
		return true
	default:
		_, isRange := ScopeRange(scope)
		return isRange
	}
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
	cmd.Env = ScrubbedGitEnv()
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
		if unborn := g.unbornHeadError(ctx, scope); unborn != nil {
			return nil, unborn
		}
		return nil, err
	}
	return dedupeLines(out), nil
}

// unbornHeadError translates a failed change-set computation into the friendly
// ErrUnbornHead message when — and only when — the cause really is "this repo
// has no commits yet". It runs only on the failure path, so the happy path
// keeps its single git invocation.
//
// Both probes matter. `rev-parse --git-dir` separates "not a git repository at
// all" (keep git's own error; the remedy is different) from "a repository
// without commits". `rev-parse --verify HEAD` then confirms HEAD is
// unresolvable rather than the scope's revision being bad (e.g. a range whose
// base does not exist), which must keep its own error too.
func (g *GitChangedFiles) unbornHeadError(ctx context.Context, scope Scope) error {
	if !scopeNeedsHead(scope) {
		return nil
	}
	if _, err := g.exec(ctx, "rev-parse", "--git-dir"); err != nil {
		return nil
	}
	if _, err := g.exec(ctx, "rev-parse", "--verify", "HEAD"); err == nil {
		return nil
	}
	return fmt.Errorf("%w: scope %q needs a commit to compare against — make an initial commit, or run with an explicit scope such as --scope staged after 'git add'", ErrUnbornHead, scope)
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
