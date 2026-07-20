// Package workflowsapp holds the effectful workflows use-case logic for the
// `ao workflows` command family: canonical-checkout resolution, target-project
// resolution, and the link/unlink filesystem sweeps. The command module in
// internal/commands/workflows owns Cobra presentation and delegates every
// direct filesystem effect here, mirroring the skills / skillsapp split.
//
// Workflows are a CLAUDE-ONLY runtime adapter (the same doctrine as
// skills-codex/ being Codex-only): the canonical source is the checkout's
// top-level workflows/*.js scripts, and the install target is the
// project-local .claude/workflows/ directory where the Claude Code harness
// resolves named workflows. There is deliberately NO multi-runtime fan-out.
package workflowsapp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// resolveCheckoutRoot locates the agentops checkout root by walking up from
// the current working directory, using the same identity discipline as
// skillsapp.ResolveRepoSkillsDir: the root must hold BOTH skills/ and
// skills-codex/ directories AND the distinctive agentops repo-root marker
// files (registry.json, PRODUCT.md). Shape is not identity — requiring the
// full marker set means the command never treats a look-alike tree as the
// canonical checkout (cross-family refuter age-u031 lineage).
func resolveCheckoutRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	dir := cwd
	for i := 0; i < 8; i++ {
		if isDir(filepath.Join(dir, "skills")) &&
			isDir(filepath.Join(dir, "skills-codex")) &&
			isFile(filepath.Join(dir, "registry.json")) &&
			isFile(filepath.Join(dir, "PRODUCT.md")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("could not locate the agentops checkout walking up from %q (need skills/, skills-codex/, registry.json, PRODUCT.md at the root) — run `ao workflows link` from inside the agentops repo", cwd)
}

// ResolveRepoWorkflowsDir returns the ABSOLUTE workflows/ directory of the
// agentops checkout containing the current working directory — the canonical
// source of the Claude-harness workflow scripts. It fails closed with a clear
// error when the caller is not inside the checkout, or when the checkout has
// no workflows/ directory yet.
func ResolveRepoWorkflowsDir() (string, error) {
	root, err := resolveCheckoutRoot()
	if err != nil {
		return "", err
	}
	workflowsDir := filepath.Join(root, "workflows")
	if !isDir(workflowsDir) {
		return "", fmt.Errorf("agentops checkout %s has no workflows/ directory — nothing to link (workflows/*.js is the canonical source for the Claude-harness workflow scripts)", root)
	}
	return workflowsDir, nil
}

// ResolveTargetDir returns the directory workflow links install into. An
// explicit --into dir wins as the single target. Otherwise the target is the
// project-local .claude/workflows/ directory of the CURRENT working
// directory's git top-level — where the Claude Code harness resolves named
// workflows. The directory need not exist yet; the link sweep creates it
// lazily. Fails closed when cwd is not inside a git repository, because
// guessing a target for a symlink install is worse than asking for --into.
func ResolveTargetDir(explicitInto string) (string, error) {
	if v := strings.TrimSpace(explicitInto); v != "" {
		return v, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	top, err := gitTopLevel(cwd)
	if err != nil {
		return "", fmt.Errorf("resolve the target project: %s is not inside a git repository (workflows install into <git-root>/.claude/workflows) — pass --into <dir> to choose the target explicitly", cwd)
	}
	return filepath.Join(top, ".claude", "workflows"), nil
}

// gitTopLevel returns the git repository top-level for dir, or an error when
// dir is not inside a repo or git is unavailable.
func gitTopLevel(dir string) (string, error) {
	git, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git not found on PATH: %w", err)
	}
	out, err := exec.Command(git, "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel in %s: %w", dir, err)
	}
	top := strings.TrimSpace(string(out))
	if top == "" {
		return "", fmt.Errorf("git rev-parse --show-toplevel returned empty output for %s", dir)
	}
	return top, nil
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
