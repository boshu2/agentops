package contextpacket

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
)

// FormatHistoryEntry formats a cycle-history JSON entry into a numbered Markdown section.
func FormatHistoryEntry(entry map[string]interface{}, index int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "### Entry %d\n", index)

	type historyField struct {
		label   string
		primary string
		aliases []string
	}

	fields := []historyField{
		{label: "timestamp", primary: "timestamp"},
		{label: "cycle", primary: "cycle"},
		{label: "target", primary: "target", aliases: []string{"goal_id"}},
		{label: "goal_ids", primary: "goal_ids"},
		{label: "result", primary: "result", aliases: []string{"status"}},
		{label: "sha", primary: "sha"},
		{label: "canonical_sha", primary: "canonical_sha"},
		{label: "log_sha", primary: "log_sha"},
		{label: "goals_passing", primary: "goals_passing"},
		{label: "goals_total", primary: "goals_total"},
		{label: "summary", primary: "summary"},
		{label: "error", primary: "error"},
	}

	for _, field := range fields {
		if v, ok := LookupHistoryField(entry, field.primary, field.aliases...); ok && v != nil {
			fmt.Fprintf(&sb, "- **%s**: %v\n", field.label, FormatHistoryValue(v))
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

// LookupHistoryField looks up a field in history data by primary key then aliases.
func LookupHistoryField(entry map[string]interface{}, primary string, aliases ...string) (interface{}, bool) {
	if v, ok := entry[primary]; ok && v != nil {
		return v, true
	}
	for _, alias := range aliases {
		if v, ok := entry[alias]; ok && v != nil {
			return v, true
		}
	}
	return nil, false
}

// FormatHistoryValue formats a history value for display, joining slices with commas.
func FormatHistoryValue(value interface{}) interface{} {
	switch v := value.(type) {
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, fmt.Sprintf("%v", item))
		}
		return strings.Join(parts, ", ")
	case []string:
		return strings.Join(v, ", ")
	default:
		return value
	}
}

// ExtractIntelJSONContent extracts displayable content from an intel JSON blob.
func ExtractIntelJSONContent(data []byte) string {
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return strings.TrimSpace(string(data))
	}

	for _, key := range []string{"content", "pattern", "summary", "description", "title"} {
		if v, ok := decoded[key]; ok && v != nil {
			s := strings.TrimSpace(fmt.Sprintf("%v", v))
			if s != "" {
				return s
			}
		}
	}
	return strings.TrimSpace(string(data))
}

// FormatTaskSection formats a task description into a Markdown TASK section.
func FormatTaskSection(task string, budget int) string {
	var sb strings.Builder
	sb.WriteString("## TASK\n\n")
	sb.WriteString(task)
	sb.WriteString("\n")
	return TruncateToCharBudget(sb.String(), budget)
}

// TruncateToCharBudget truncates content to fit within a character budget,
// preferring to break at a newline boundary.
func TruncateToCharBudget(content string, budget int) string {
	if budget <= 0 {
		return ""
	}
	runes := []rune(content)
	if len(runes) <= budget {
		return content
	}
	// Truncate at budget, try to break at a newline.
	truncated := string(runes[:budget])
	lastNL := strings.LastIndex(truncated, "\n")
	if lastNL > budget/2 {
		truncated = truncated[:lastNL+1]
	}
	return truncated + "\n... [truncated to fit budget]\n"
}

// RedactHighEntropy replaces long tokens (>30 chars) with high Shannon entropy (>4.5 bits/char).
func RedactHighEntropy(content string) (string, int) {
	redactions := 0
	// Find words/tokens > 30 chars that look like secrets.
	wordRe := regexp.MustCompile(`\S{31,}`)
	content = wordRe.ReplaceAllStringFunc(content, func(match string) string {
		if ShannonEntropy(match) > 4.5 {
			redactions++
			return "[REDACTED: high-entropy]"
		}
		return match
	})
	return content, redactions
}

// ShannonEntropy calculates the Shannon entropy (bits per character) of a string.
func ShannonEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	freq := make(map[rune]int)
	for _, r := range s {
		freq[r]++
	}
	length := float64(len([]rune(s)))
	entropy := 0.0
	for _, count := range freq {
		p := float64(count) / length
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}
	return entropy
}

// AssembledSection holds a named section of a context briefing.
type AssembledSection struct {
	Name       string `json:"name"`
	CharCount  int    `json:"char_count"`
	Redactions int    `json:"redactions"`
	Content    string `json:"-"`
}

// ComposeBriefingMarkdown composes a full briefing document from assembled sections.
func ComposeBriefingMarkdown(sections []AssembledSection) string {
	var sb strings.Builder
	sb.WriteString("# Context Briefing\n\n")
	fmt.Fprintf(&sb, "_Generated: %s_\n\n", time.Now().UTC().Format(time.RFC3339))

	for _, s := range sections {
		sb.WriteString(s.Content)
		sb.WriteString("\n")
	}

	return sb.String()
}
