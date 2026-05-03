package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Section name constants for context filtering.
const (
	sectionHistory = "HISTORY"
	sectionIntel   = "INTEL"
	sectionTask    = "TASK"
)

// validIntelScopes is the set of allowed intel_scope values.
var validIntelScopes = map[string]bool{
	"none":  true,
	"topic": true,
	"full":  true,
}

// validSectionNames is the set of allowed section names in context declarations.
var validSectionNames = map[string]bool{
	sectionHistory: true,
	sectionIntel:   true,
	sectionTask:    true,
}

// ContextDeclaration represents a skill's context window policy.
type ContextDeclaration struct {
	Window     string         `yaml:"window"` // isolated, fork, inherit
	Sections   *SectionFilter `yaml:"sections"`
	Intent     *IntentConfig  `yaml:"intent"`
	IntelScope string         `yaml:"intel_scope"`
}

// SectionFilter controls which knowledge sections to include or exclude.
type SectionFilter struct {
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
}

// IntentConfig declares the skill's intent mode.
type IntentConfig struct {
	Mode string `yaml:"mode"` // questions, task, none
}

// parseContextDeclaration reads a skill's SKILL.md frontmatter and parses the context field.
// Returns nil, nil if no context field exists (meaning: use defaults).
func parseContextDeclaration(skillName string) (*ContextDeclaration, error) {
	skillPath, err := resolveSkillPath(skillName)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(skillPath)
	if err != nil {
		return nil, fmt.Errorf("read SKILL.md: %w", err)
	}

	frontmatter, err := extractFrontmatter(string(data))
	if err != nil {
		return nil, err
	}
	if frontmatter == "" {
		return nil, nil
	}

	return parseContextFromFrontmatter([]byte(frontmatter))
}

// extractFrontmatter pulls YAML content between --- markers from a markdown file.
// Uses line-oriented parsing to avoid false matches on --- appearing in YAML string values.
func extractFrontmatter(content string) (string, error) {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", nil
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n"), nil
		}
	}
	return "", nil
}

// parseContextFromFrontmatter parses the context field from YAML frontmatter bytes.
// Handles both string form (context: fork) and object form (full struct).
func parseContextFromFrontmatter(frontmatter []byte) (*ContextDeclaration, error) {
	// Parse into a generic node tree so we can detect whether context is scalar vs mapping
	var doc yaml.Node
	if err := yaml.Unmarshal(frontmatter, &doc); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}

	// doc is a Document node; its first child is the mapping
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, nil
	}
	mapping := doc.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return nil, nil
	}

	// Walk the mapping key/value pairs to find "context"
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		keyNode := mapping.Content[i]
		valNode := mapping.Content[i+1]

		if keyNode.Value != "context" {
			continue
		}

		// String form: context: fork
		if valNode.Kind == yaml.ScalarNode {
			if err := validateWindow(valNode.Value); err != nil {
				return nil, err
			}
			return &ContextDeclaration{Window: valNode.Value}, nil
		}

		// Object form: context: { window: ..., sections: ..., ... }
		if valNode.Kind == yaml.MappingNode {
			var decl ContextDeclaration
			if err := valNode.Decode(&decl); err != nil {
				return nil, fmt.Errorf("decode context declaration: %w", err)
			}
			if decl.Window != "" {
				if err := validateWindow(decl.Window); err != nil {
					return nil, err
				}
			}
			if decl.IntelScope != "" {
				if err := validateIntelScope(decl.IntelScope); err != nil {
					return nil, err
				}
			}
			if err := validateSectionNames(decl.Sections); err != nil {
				return nil, err
			}
			return &decl, nil
		}

		return nil, fmt.Errorf("unexpected context node kind: %d", valNode.Kind)
	}

	// No context field found
	return nil, nil
}

// validateWindow checks that the window value is one of the allowed values.
func validateWindow(w string) error {
	switch w {
	case "isolated", "fork", "inherit":
		return nil
	default:
		return fmt.Errorf("invalid context.window %q: must be isolated, fork, or inherit", w)
	}
}

// validateSectionNames checks that all section names in include/exclude lists are valid.
func validateSectionNames(sf *SectionFilter) error {
	if sf == nil {
		return nil
	}
	for _, s := range sf.Include {
		if !validSectionNames[s] {
			return fmt.Errorf("invalid section name %q in sections.include: must be HISTORY, INTEL, or TASK", s)
		}
	}
	for _, s := range sf.Exclude {
		if !validSectionNames[s] {
			return fmt.Errorf("invalid section name %q in sections.exclude: must be HISTORY, INTEL, or TASK", s)
		}
	}
	return nil
}

// validateIntelScope checks that the intel_scope value is one of the allowed values.
func validateIntelScope(scope string) error {
	if validIntelScopes[scope] {
		return nil
	}
	return fmt.Errorf("invalid context.intel_scope %q: must be none, topic, or full", scope)
}

// resolveSkillPath finds the SKILL.md for a given skill name.
// Search order: local repo, installed skills, plugin cache.
func resolveSkillPath(skillName string) (string, error) {
	// Reject path traversal characters to prevent directory escape
	if strings.ContainsAny(skillName, "/\\.") {
		return "", fmt.Errorf("invalid skill name %q: must not contain path separators or dots", skillName)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	// 1. Local repo: skills/<name>/SKILL.md
	local := filepath.Join(cwd, "skills", skillName, "SKILL.md")
	if _, err := os.Stat(local); err == nil {
		return local, nil
	}

	// 2. Installed: ~/.claude/skills/<name>/SKILL.md
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	installed := filepath.Join(home, ".claude", "skills", skillName, "SKILL.md")
	if _, err := os.Stat(installed); err == nil {
		return installed, nil
	}

	// 3. Plugin cache: ~/.claude/plugins/cache/agentops-marketplace/agentops/*/skills/<name>/SKILL.md
	cachePattern := filepath.Join(home, ".claude", "plugins", "cache", "agentops-marketplace", "agentops", "*", "skills", skillName, "SKILL.md")
	matches, _ := filepath.Glob(cachePattern)
	if len(matches) > 0 {
		sort.Strings(matches)
		return matches[len(matches)-1], nil // latest version (lexicographically highest)
	}

	return "", fmt.Errorf("skill %q not found", skillName)
}

// applyContextFilter filters knowledge based on the context declaration.
// Mutates the knowledge struct in place and returns it.
func applyContextFilter(knowledge *injectedKnowledge, decl *ContextDeclaration) *injectedKnowledge {
	if decl == nil {
		return knowledge
	}

	// Apply section filters
	if decl.Sections != nil {
		// Section→Field mapping:
		//   HISTORY → Sessions
		//   INTEL   → Learnings, Patterns
		//   TASK    → BeadID, Predecessor
		if len(decl.Sections.Include) > 0 {
			// Allowlist mode: zero sections NOT in the include list.
			// Include takes precedence over exclude when both are specified.
			allowed := make(map[string]bool, len(decl.Sections.Include))
			for _, s := range decl.Sections.Include {
				allowed[s] = true
			}
			if !allowed[sectionHistory] {
				knowledge.Sessions = nil
			}
			if !allowed[sectionIntel] {
				knowledge.Learnings = nil
				knowledge.Patterns = nil
			}
			if !allowed[sectionTask] {
				knowledge.BeadID = ""
				knowledge.Predecessor = nil
			}
		} else {
			// Exclude mode: zero sections in the exclude list.
			for _, section := range decl.Sections.Exclude {
				switch section {
				case sectionHistory:
					knowledge.Sessions = nil
				case sectionIntel:
					knowledge.Learnings = nil
					knowledge.Patterns = nil
				case sectionTask:
					knowledge.BeadID = ""
					knowledge.Predecessor = nil
				}
			}
		}
	}

	// Apply intel_scope
	// "topic" and "full" are declaration-only in v1 — no runtime difference.
	// Future versions may use "topic" to restrict learnings to query-matched intel only.
	if decl.IntelScope == "none" {
		knowledge.Learnings = nil
		knowledge.Patterns = nil
	}

	// intent.mode is declaration-only in v1 — do NOT filter on it.
	// Skills declare their intent mode so upstream orchestrators (e.g. hooks)
	// can adapt behavior, but the inject pipeline does not act on it yet.

	return knowledge
}
