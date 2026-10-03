// Package skillsapp holds the effectful skills use-case logic carved out of
// package main: repo/runtime skills-root resolution and the link/unlink
// filesystem sweeps. The command module in internal/commands/skills owns Cobra
// presentation and delegates every direct filesystem effect here.
package skillsapp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// repoRootMarkers are the distinctive agentops repo-root files that sit beside
// skills/. Shape is not identity: a bare skills/ directory exists in many
// places (cli/internal/skills is a Go package, and any repository may keep its
// own skills/), so the resolver only accepts a directory that also carries
// these markers.
var repoRootMarkers = []string{"registry.json", "PRODUCT.md"}

// ResolveSkillsRoot locates the agentops skills/ directory relative to the
// current working directory, walking up the tree until it finds a directory
// holding skills/ plus the repo-root markers. It falls back to the literal
// "skills" when no such root is found, which reads a cwd-local skills/ tree or
// produces a clear error from os.ReadDir.
func ResolveSkillsRoot() string {
	const skills = "skills"
	cwd, err := os.Getwd()
	if err != nil {
		return skills
	}
	dir := cwd
	for i := 0; i < 8; i++ {
		if s := filepath.Join(dir, skills); isDir(s) && hasRepoRootMarkers(dir) {
			return s
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return skills
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

// hasRepoRootMarkers reports whether dir carries every agentops repo-root
// marker as a regular (non-directory) entry.
func hasRepoRootMarkers(dir string) bool {
	for _, marker := range repoRootMarkers {
		if fi, err := os.Stat(filepath.Join(dir, marker)); err != nil || fi.IsDir() {
			return false
		}
	}
	return true
}

// ResolveRepoSkillsDir returns the ABSOLUTE agentops repo skills/ directory, or
// an error if the caller is not inside the repo. It relies on the resolver's
// real signal: ResolveSkillsRoot returns an absolute path ONLY when it located
// a directory holding skills/ AND the agentops repo-root markers walking up
// from cwd; its fallback returns the RELATIVE literal "skills". A mere
// existence check is not enough — running from an unrelated directory that
// happens to contain a stray skills/ subdir would pass os.Stat and scan/link
// that tree into ~/.claude/skills. Requiring an absolute, marker-verified path
// fails closed instead (cross-family refuter age-u031, codex-fresh-review).
func ResolveRepoSkillsDir() (string, error) {
	skillsDir := ResolveSkillsRoot()
	if !filepath.IsAbs(skillsDir) || !isDir(skillsDir) {
		return "", fmt.Errorf("could not locate the agentops repo skills/ tree (resolved %q; the repo root holds skills/, registry.json and PRODUCT.md) — run `ao skills link` from inside the agentops repo", skillsDir)
	}
	return skillsDir, nil
}

// runtimeSkillTarget pairs a runtime's install-detection dir with its
// user-level skills dir, both relative to $HOME. For most runtimes skills is
// <detect>/skills; Pi is the exception.
type runtimeSkillTarget struct {
	// detect is the dir under $HOME whose existence signals the runtime is
	// installed.
	detect string
	// skills is the dir under $HOME to link/unlink this runtime's skills into.
	skills string
}

// runtimeSkillTargets are the per-runtime user-level skill dirs `ao skills
// link` fans out into, in display order. AgentOps skills are identical across
// runtimes, so a default `ao skills link` links into EVERY runtime the user
// actually has installed — Claude, Codex (~/.codex/skills), AGY/Gemini
// (~/.gemini/skills), Cursor, and Pi (~/.pi/agent/skills) — not just Claude.
// Most runtimes are detected by their config dir existing under $HOME, with
// that same dir holding skills/. Pi is the exception: its USER-level skills
// dir is ~/.pi/agent/skills, detected by ~/.pi/agent existing — ~/.pi/skills
// is Pi's separate PROJECT-level dir. Ground truth: the npx `skills`
// installer's agent table (skills v1.7.0, dist/cli.mjs). The doctor adapter's
// runtimeSkillRoots mirrors this list.
var runtimeSkillTargets = []runtimeSkillTarget{
	{detect: ".claude", skills: filepath.Join(".claude", "skills")},
	{detect: ".codex", skills: filepath.Join(".codex", "skills")},
	{detect: ".gemini", skills: filepath.Join(".gemini", "skills")},
	{detect: ".cursor", skills: filepath.Join(".cursor", "skills")},
	{detect: filepath.Join(".pi", "agent"), skills: filepath.Join(".pi", "agent", "skills")},
}

// ResolveTargetDests returns the skills dirs to link into. An explicit dest wins
// as the single target. Otherwise it returns each runtimeSkillTargets entry's
// <home>/skills path for every runtime whose detect dir EXISTS under $HOME.
// The portable ~/.agents/skills root is always included, even in a fresh home
// with no runtime configuration yet.
func ResolveTargetDests(explicitDest string) ([]string, error) {
	if strings.TrimSpace(explicitDest) != "" {
		return []string{explicitDest}, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir for default --dest: %w", err)
	}
	dests := []string{filepath.Join(home, ".agents", "skills")}
	for _, rt := range runtimeSkillTargets {
		if isDir(filepath.Join(home, rt.detect)) {
			dests = append(dests, filepath.Join(home, rt.skills))
		}
	}
	return dests, nil
}
