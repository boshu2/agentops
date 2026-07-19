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

// ResolveSkillsRoots locates the skills/ and skills-codex/ directories relative
// to the current working directory, walking up the tree until both are found.
// Falls back to literal "skills" / "skills-codex" if not found, which produces a
// clear error from os.ReadDir.
func ResolveSkillsRoots() (string, string) {
	const skills = "skills"
	const codex = "skills-codex"
	cwd, err := os.Getwd()
	if err != nil {
		return skills, codex
	}
	dir := cwd
	for i := 0; i < 8; i++ {
		s := filepath.Join(dir, skills)
		c := filepath.Join(dir, codex)
		if isDir(s) && isDir(c) {
			return s, c
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return skills, codex
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

// ResolveRepoSkillsDir returns the ABSOLUTE agentops repo skills/ directory, or
// an error if the caller is not inside the repo. It relies on the resolver's
// real signal: ResolveSkillsRoots returns absolute paths ONLY when it located a
// directory holding BOTH skills/ and skills-codex/ (the agentops structure)
// walking up from cwd; its fallback returns the RELATIVE literal "skills". A
// mere existence check is not enough — running from an unrelated directory that
// happens to contain a stray skills/ subdir would pass os.Stat and scan/link
// that tree. Requiring an absolute, pair-verified path fails closed instead
// (cross-family refuter age-u031, codex-fresh-review, two rounds).
func ResolveRepoSkillsDir() (string, error) {
	skillsDir, codexDir := ResolveSkillsRoots()
	if !filepath.IsAbs(skillsDir) || !isDir(skillsDir) || !isDir(codexDir) {
		return "", fmt.Errorf("could not locate the agentops repo skills/ tree (resolved %q) — run `ao skills link` from inside the agentops repo", skillsDir)
	}
	// Shape is not identity: a skills/+skills-codex/ pair could exist outside
	// agentops. Require distinctive agentops repo-root markers (siblings of
	// skills/) so the command never scans a look-alike tree into
	// ~/.claude/skills (cross-family refuter age-u031, codex-fresh-review, 3
	// rounds — this is the terminal identity check).
	root := filepath.Dir(skillsDir)
	for _, marker := range []string{"registry.json", "PRODUCT.md"} {
		if fi, err := os.Stat(filepath.Join(root, marker)); err != nil || fi.IsDir() {
			return "", fmt.Errorf("resolved %q is not the agentops repo root (missing %s) — run `ao skills link` from inside the agentops repo", root, marker)
		}
	}
	return skillsDir, nil
}

// runtimeConfigDirs are the per-agent config dirs whose skills/ subdir is that
// runtime's live tier. AgentOps skills are identical across runtimes, so a
// default `ao skills link` links into EVERY runtime the user actually has
// installed — Claude, Codex (~/.codex/skills), AGY/Gemini (~/.gemini/skills),
// Cursor, and Pi (~/.pi/skills) — not just Claude. Detection is by the config
// dir existing under $HOME. Order is display order.
var runtimeConfigDirs = []string{".claude", ".codex", ".gemini", ".cursor", ".pi"}

// ResolveTargetDests returns the skills dirs to link into. An explicit dest wins
// as the single target. Otherwise it returns <home>/<rt>/skills for every
// runtime config dir that EXISTS under $HOME. The portable ~/.agents/skills root
// is always included, even in a fresh home with no runtime configuration yet.
func ResolveTargetDests(explicitDest string) ([]string, error) {
	if strings.TrimSpace(explicitDest) != "" {
		return []string{explicitDest}, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir for default --dest: %w", err)
	}
	dests := []string{filepath.Join(home, ".agents", "skills")}
	for _, rt := range runtimeConfigDirs {
		if isDir(filepath.Join(home, rt)) {
			dests = append(dests, filepath.Join(home, rt, "skills"))
		}
	}
	return dests, nil
}
