package rpi

import (
	"fmt"
	"strings"
)

// The handoff/markdown rendering engine (finding-ID extraction, bullet/list
// parsing, verdict/field rendering, degradation warnings) was deleted with the rpi
// engine (age-tlj6); only the three helpers still consumed externally remain:
// UniqueStringsPreserveOrder + CompiledChecklistSummaryFromContent (cmd/ao engine
// shared) and StripMarkdownFrontmatter (the latter's dependency).

// UniqueStringsPreserveOrder deduplicates strings while preserving first-seen order.
// Empty/whitespace-only strings are dropped.
func UniqueStringsPreserveOrder(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

// StripMarkdownFrontmatter removes YAML frontmatter (--- delimited) from markdown content.
func StripMarkdownFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---\n") {
		return content
	}
	lines := strings.Split(content, "\n")
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[i+1:], "\n")
		}
	}
	return content
}

// CompiledChecklistSummaryFromContent builds a checklist summary from file content and ID.
func CompiledChecklistSummaryFromContent(id, body string) string {
	body = StripMarkdownFrontmatter(body)
	lines := strings.Split(body, "\n")
	items := []string{}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			continue
		case strings.HasPrefix(trimmed, "#"):
			continue
		case strings.HasPrefix(trimmed, "Prevent this known failure mode"):
			continue
		case strings.HasPrefix(trimmed, "- "):
			item := strings.TrimSpace(trimmed[2:])
			if strings.HasPrefix(item, "Source:") {
				continue
			}
			items = append(items, item)
		default:
			if len(items) == 0 {
				items = append(items, trimmed)
			}
		}
		if len(items) >= 3 {
			break
		}
	}

	if len(items) == 0 {
		return id
	}
	return fmt.Sprintf("%s — %s", id, strings.Join(items, " | "))
}
