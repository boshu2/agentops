package rpi

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// FindingIDPattern matches finding IDs in free text (e.g. f-2026-04-02-1).
var FindingIDPattern = regexp.MustCompile(`\bf-\d{4}-\d{2}-\d{2}-\d+\b`)

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

// ExtractFindingIDs returns unique finding IDs (f-YYYY-MM-DD-N) found in text.
func ExtractFindingIDs(text string) []string {
	return UniqueStringsPreserveOrder(FindingIDPattern.FindAllString(text, -1))
}

// ExtractBulletItemsAfterMarker extracts bullet items from markdown text after a marker line.
// Stops at the next heading or non-bullet, non-empty line.
func ExtractBulletItemsAfterMarker(text, marker string) []string {
	lines := strings.Split(text, "\n")
	marker = strings.TrimSpace(marker)
	items := []string{}
	capturing := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !capturing {
			if trimmed == marker || strings.HasPrefix(trimmed, marker+" ") {
				capturing = true
				continue
			}
			continue
		}
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "# ") {
			break
		}
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			items = append(items, strings.TrimSpace(trimmed[2:]))
			continue
		}
		if len(items) > 0 {
			break
		}
	}

	return UniqueStringsPreserveOrder(items)
}

// ExtractMarkdownListItemsUnderHeading extracts bullet items under a heading.
// Stops at the next heading or non-bullet, non-empty line.
func ExtractMarkdownListItemsUnderHeading(text, heading string) []string {
	lines := strings.Split(text, "\n")
	heading = strings.TrimSpace(heading)
	items := []string{}
	capturing := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !capturing {
			if trimmed == heading {
				capturing = true
			}
			continue
		}
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "# ") {
			break
		}
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			items = append(items, strings.TrimSpace(trimmed[2:]))
			continue
		}
		if len(items) > 0 {
			break
		}
	}

	return UniqueStringsPreserveOrder(items)
}

// TruncateRunes truncates s to at most cap runes and appends "..." if truncated.
// Safe for multi-byte UTF-8 characters.
func TruncateRunes(s string, cap int) string {
	// Fast path: len(s) is an upper bound on the rune count, so any string
	// whose byte length already fits the cap needs no rune conversion. This
	// avoids a []rune allocation on the common short-ASCII case without
	// changing behavior for multi-byte inputs that exceed the byte limit.
	if len(s) <= cap {
		return s
	}
	runes := []rune(s)
	if len(runes) <= cap {
		return s
	}
	return string(runes[:cap]) + "..."
}

// FormatVerdicts renders a sorted verdict line from a map.
// Returns empty string if verdicts is nil or empty.
func FormatVerdicts(verdicts map[string]string) string {
	if len(verdicts) == 0 {
		return ""
	}
	keys := make([]string, 0, len(verdicts))
	for k := range verdicts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %s", k, verdicts[k]))
	}
	return fmt.Sprintf("Verdict: %s\n", strings.Join(parts, ", "))
}

// RenderHandoffField renders a labeled field line.
// For string values: returns "Label: value\n" or "" if empty.
// For []string values: returns "Label: a, b, c\n" or "" if empty.
func RenderHandoffField(label string, value interface{}) string {
	switch v := value.(type) {
	case string:
		if v == "" {
			return ""
		}
		return fmt.Sprintf("%s: %s\n", label, v)
	case []string:
		if len(v) == 0 {
			return ""
		}
		return fmt.Sprintf("%s: %s\n", label, strings.Join(v, ", "))
	}
	return ""
}

// FieldAllowed checks whether a field should be included in handoff context.
// Returns true if handoffFields is empty (backward compat) or the field is listed.
func FieldAllowed(handoffFields []string, field string) bool {
	if len(handoffFields) == 0 {
		return true
	}
	for _, f := range handoffFields {
		if f == field {
			return true
		}
	}
	return false
}

// ResolveNarrativeCap returns the narrative character cap.
// narrativeCap=0 means "omit narrative" when handoffFields is set (least-privilege).
// When handoffFields is empty (no manifest), default to 1000 for backward compat.
func ResolveNarrativeCap(narrativeCap int, handoffFields []string) int {
	if narrativeCap > 0 {
		return narrativeCap
	}
	if len(handoffFields) == 0 {
		return 1000
	}
	return 0
}

// RenderDegradationWarnings writes context degradation warnings for handoffs.
// phaseNumbers is a list of phase numbers that have context degradation.
func RenderDegradationWarnings(sb *strings.Builder, degradedPhases []int) {
	for _, phase := range degradedPhases {
		fmt.Fprintf(sb, "⚠️ CONTEXT DEGRADATION: Phase %d handoff was missing — context may be incomplete\n\n", phase-1)
	}
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
