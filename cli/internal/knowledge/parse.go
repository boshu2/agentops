// Package knowledge holds pure helpers extracted from cmd/ao/knowledge*.go.
//
// These utilities parse topic packet markdown, frontmatter, and builder
// metadata. They have no dependency on cobra, package-level globals, or
// any cmd/ao state, which makes them easy to unit test in isolation.
package knowledge

import (
	"fmt"
	"os"
	"strings"

	"github.com/boshu2/agentops/cli/internal/wiki"
)

// ParseBuilderMetadata extracts key=value pairs from a builder's stdout.
// Lines without "=" are skipped. Returns nil when no pairs were found.
func ParseBuilderMetadata(output string) map[string]string {
	metadata := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key != "" && value != "" {
			metadata[key] = value
		}
	}
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

// ParseFrontmatter parses a leading YAML frontmatter block (--- ... ---) from
// the given text. Returns nil when no frontmatter is present or when parsing
// fails.
//
// Parsing is delegated to wiki.FrontmatterCodec; this wrapper preserves the
// historical nil-on-miss contract for existing callers.
func ParseFrontmatter(text string) map[string]any {
	doc := wiki.FrontmatterCodec{}.Decode(text)
	if !doc.HasFrontmatter {
		return nil
	}
	return doc.Fields
}

// FrontmatterString returns the trimmed string value for key, falling back to
// the provided default when the key is missing or empty.
func FrontmatterString(frontmatter map[string]any, key, fallback string) string {
	if frontmatter == nil {
		return fallback
	}
	if value, ok := frontmatter[key]; ok {
		if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
			return text
		}
	}
	return fallback
}

// ExtractBullets returns the list items found under the given markdown
// heading, stopping at the next "## " heading.
func ExtractBullets(text, heading string) []string {
	lines := strings.Split(text, "\n")
	items := make([]string, 0)
	inSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == heading {
			inSection = true
			continue
		}
		if !inSection {
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			break
		}
		if strings.HasPrefix(trimmed, "- ") {
			items = append(items, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
		}
	}
	return items
}

// FilterOpenGaps removes the "No open gaps recorded." sentinel and returns the
// remaining bullets unchanged.
func FilterOpenGaps(items []string) []string {
	filtered := make([]string, 0, len(items))
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), "No open gaps recorded.") {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

// DedupeStrings returns the unique trimmed strings in input order. Empty
// strings are dropped.
func DedupeStrings(items []string) []string {
	seen := make(map[string]bool, len(items))
	deduped := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		deduped = append(deduped, trimmed)
	}
	return deduped
}

// PathExists reports whether the given filesystem path can be stat'd.
func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
