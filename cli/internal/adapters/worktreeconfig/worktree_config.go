// practices: [microservices, team-topologies]
package worktreeconfig

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SanitizeGitProcessEnv removes repository-discovery variables that can pin
// Git commands to the wrong worktree when ao starts inside nested sessions.
func SanitizeGitProcessEnv() error {
	for _, key := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR"} {
		if err := os.Unsetenv(key); err != nil {
			return fmt.Errorf("unset %s: %w", key, err)
		}
	}
	return nil
}

// RepairSharedCoreWorktreeConfig migrates a shared core.worktree setting into
// per-worktree config when a linked worktree has inherited a stale top-level.
func RepairSharedCoreWorktreeConfig(cwd string) error {
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

// gitSafetyFlags neutralizes repo-local git config that can execute code when git runs in
// an untrusted cwd (the root pre-run runs git in the user's repo on every ao invocation):
// core.fsmonitor + core.hooksPath are the config-driven exec vectors. Passed as -c overrides
// so the repo's own .git/config values can't fire. (age-a9iv.4 residual.)
func gitSafetyFlags() []string {
	return []string{"-c", "core.fsmonitor=", "-c", "core.hooksPath=/dev/null"}
}

func gitOutputInDir(cwd string, args ...string) (string, error) {
	cmd := exec.Command(trustedGitBin(cwd), append(gitSafetyFlags(), args...)...)
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
	cmd := exec.Command(trustedGitBin(processCwd()), append(gitSafetyFlags(), append([]string{"config", "--file", configPath}, args...)...)...)
	cmd.Env = gitDiscoveryEnv()
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func runGitInDir(cwd string, args ...string) error {
	cmd := exec.Command(trustedGitBin(cwd), append(gitSafetyFlags(), args...)...)
	cmd.Dir = cwd
	cmd.Env = gitDiscoveryEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func runGitWithConfigFile(configPath string, args ...string) error {
	cmd := exec.Command(trustedGitBin(processCwd()), append(gitSafetyFlags(), append([]string{"config", "--file", configPath}, args...)...)...)
	cmd.Env = gitDiscoveryEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git config --file %s %s: %w (%s)", configPath, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// trustedGitBin resolves git to an ABSOLUTE path on a PATH that excludes every dir the repo
// containing cwd could control — "", ".", relative entries, and any absolute dir inside the
// ENCLOSING REPO ROOT (so $REPO_ROOT/bin is excluded even when ao runs from a subdir). ao's
// root pre-run runs git in the user's cwd before any command, so a repo-planted git on PATH
// would otherwise execute (SECURITY, age-a9iv.4). Returns "" when no trusted git is found —
// callers FAIL CLOSED (the worktree repair is best-effort and simply skips); it NEVER falls
// back to bare "git" (which Go would re-resolve on the unsanitized PATH, running the planted
// git a real system git would have shadowed).
func trustedGitBin(cwd string) string {
	sep := string(os.PathListSeparator)
	rootReal := enclosingRepoRoot(cwd)
	for _, dir := range strings.Split(os.Getenv("PATH"), sep) {
		if dir == "" || dir == "." || !filepath.IsAbs(dir) {
			continue
		}
		if rootReal != "" {
			dr := dir
			if r, err := filepath.EvalSymlinks(dir); err == nil {
				dr = r
			}
			if rel, err := filepath.Rel(rootReal, dr); err == nil &&
				(rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel))) {
				continue
			}
		}
		cand := filepath.Join(dir, "git")
		if info, err := os.Stat(cand); err == nil && !info.IsDir() && info.Mode().Perm()&0o111 != 0 {
			return cand
		}
	}
	return ""
}

// enclosingRepoRoot returns the symlink-resolved root of the git working tree containing
// start (the nearest ancestor with a `.git` entry), found in PURE Go — no binary run. When
// start is not inside a repo it returns start's own realpath, so at minimum the cwd subtree
// is treated as untrusted for git resolution.
func enclosingRepoRoot(start string) string {
	base := start
	if r, err := filepath.EvalSymlinks(start); err == nil {
		base = r
	} else if abs, err := filepath.Abs(start); err == nil {
		base = abs
	}
	cur := base
	for {
		if _, err := os.Stat(filepath.Join(cur, ".git")); err == nil {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return base
		}
		cur = parent
	}
}

// processCwd returns the process working directory (the repo ao was launched in), used as
// the exclude-root for git resolution in the config-file helpers that take no cwd.
func processCwd() string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return ""
}

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
		case strings.HasPrefix(entry, "GIT_CONFIG_"):
			// Drop env-injected config (GIT_CONFIG_COUNT/KEY/VALUE/GLOBAL/SYSTEM) so a
			// poisoned environment can't add config-driven exec to the pre-run git ops.
			continue
		default:
			env = append(env, entry)
		}
	}
	return env
}
