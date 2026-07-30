package gcmaintainer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/skillsapp"
)

// resolveSkillsSource picks the AgentOps skills directory the Codex sinks link
// to: the explicit override, an agentops checkout enclosing the working
// directory, or an installed skills root that carries every required skill.
// The shipped `ao` binary therefore works without a repo checkout.
func resolveSkillsSource(explicit string) (string, error) {
	if explicit != "" {
		if !isDir(explicit) {
			return "", fmt.Errorf("skills source directory does not exist: %s", explicit)
		}
		source, err := canonical(explicit)
		if err != nil {
			return "", fmt.Errorf("resolve skills source: %w", err)
		}
		if missing := missingRequiredSkills(source); len(missing) > 0 {
			return "", fmt.Errorf("skills source %s is missing required skills: %s", source, strings.Join(missing, " "))
		}
		return source, nil
	}
	if repoSkills, err := skillsapp.ResolveRepoSkillsDir(); err == nil {
		return canonical(repoSkills)
	}
	home, err := os.UserHomeDir()
	if err == nil {
		for _, root := range []string{
			filepath.Join(home, ".agents", "skills"),
			filepath.Join(home, ".claude", "skills"),
		} {
			if isDir(root) && len(missingRequiredSkills(root)) == 0 {
				return canonical(root)
			}
		}
	}
	return "", fmt.Errorf("cannot resolve an AgentOps skills source: no agentops checkout encloses the working directory and no installed skills root carries %s; pass --skills-source",
		strings.Join(requiredSkills, " "))
}

// missingRequiredSkills lists required skills that root does not expose as a
// directory (or symlink to one) holding SKILL.md.
func missingRequiredSkills(root string) []string {
	var missing []string
	for _, skill := range requiredSkills {
		if !isRegularFile(filepath.Join(root, skill, "SKILL.md")) {
			missing = append(missing, skill)
		}
	}
	return missing
}

func (o *ops) skillSinks() []string {
	return []string{
		filepath.Join(o.city, ".codex", "skills"),
		filepath.Join(o.rig, ".codex", "skills"),
	}
}

// checkSkillLinkConflicts refuses, before any mutation, a required-skill path
// in a Codex sink that exists but is not this operation's skills source.
func (o *ops) checkSkillLinkConflicts() error {
	for _, sink := range o.skillSinks() {
		for _, skill := range requiredSkills {
			target := filepath.Join(sink, skill)
			if _, err := os.Lstat(target); err != nil {
				continue
			}
			if !isRegularFile(filepath.Join(target, "SKILL.md")) {
				return fmt.Errorf("refusing conflicting AgentOps skill path: %s", target)
			}
			if err := o.verifySkillResolves(sink, skill); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkSkillLinks requires every required skill to be visible in both Codex
// sinks and to resolve to the active skills source.
func (o *ops) checkSkillLinks() error {
	for _, sink := range o.skillSinks() {
		for _, skill := range requiredSkills {
			if !isRegularFile(filepath.Join(sink, skill, "SKILL.md")) {
				return fmt.Errorf("AgentOps skill is not visible at %s", filepath.Join(sink, skill))
			}
			if err := o.verifySkillResolves(sink, skill); err != nil {
				return err
			}
		}
	}
	return nil
}

func (o *ops) verifySkillResolves(sink, skill string) error {
	target := filepath.Join(sink, skill)
	actual, err := canonical(filepath.Join(target, "SKILL.md"))
	if err != nil {
		return fmt.Errorf("refusing conflicting AgentOps skill path: %s", target)
	}
	expected, err := canonical(filepath.Join(o.skillsSource, skill, "SKILL.md"))
	if err != nil {
		return fmt.Errorf("skills source is missing %s/SKILL.md: %w", skill, err)
	}
	if actual != expected {
		return fmt.Errorf("AgentOps skill does not resolve to the active skills source: %s", target)
	}
	return nil
}

// linkSkills makes every skill in the resolved source visible in the city and
// rig Codex sinks. Missing entries gain a symlink to the canonical source
// skill directory; existing entries are never replaced (required-skill
// conflicts were refused earlier, and the final link check judges the result).
func (o *ops) linkSkills() error {
	entries, err := os.ReadDir(o.skillsSource)
	if err != nil {
		return fmt.Errorf("read skills source %s: %w", o.skillsSource, err)
	}
	for _, sink := range o.skillSinks() {
		if err := os.MkdirAll(sink, 0o755); err != nil {
			return fmt.Errorf("create skills sink %s: %w", sink, err)
		}
	}
	for _, entry := range entries {
		name := entry.Name()
		// Follow symlinked skill dirs so an installed live tier works as a source.
		if !isRegularFile(filepath.Join(o.skillsSource, name, "SKILL.md")) {
			continue
		}
		src, err := canonical(filepath.Join(o.skillsSource, name))
		if err != nil {
			continue
		}
		for _, sink := range o.skillSinks() {
			target := filepath.Join(sink, name)
			if _, err := os.Lstat(target); err == nil {
				continue
			}
			if err := os.Symlink(src, target); err != nil {
				return fmt.Errorf("link skill %s into %s: %w", name, sink, err)
			}
		}
	}
	return nil
}
