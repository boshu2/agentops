package search

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/notebook"
)

const (
	// InjectCharsPerToken is the approximate characters per token (conservative estimate).
	InjectCharsPerToken = 4

)

// ResortLearnings re-sorts learnings by CompositeScore descending.
func ResortLearnings(learnings []Learning) {
	slices.SortFunc(learnings, func(a, b Learning) int {
		if a.CompositeScore > b.CompositeScore {
			return -1
		}
		if a.CompositeScore < b.CompositeScore {
			return 1
		}
		return 0
	})
}

// FilterMemoryDuplicates removes learnings whose Title or ID already appears in MEMORY.md.
func FilterMemoryDuplicates(cwd string, learnings []Learning) []Learning {
	memoryFile, err := notebook.FindMemoryFile(cwd)
	if err != nil {
		return learnings
	}
	content, err := os.ReadFile(memoryFile)
	if err != nil {
		return learnings
	}
	memoryText := string(content)

	filtered := make([]Learning, 0, len(learnings))
	for _, l := range learnings {
		if l.ID != "" && strings.Contains(memoryText, l.ID) {
			continue
		}
		if l.Title != "" && strings.Contains(memoryText, l.Title) {
			continue
		}
		filtered = append(filtered, l)
	}
	return filtered
}

// FindAgentsSubdir walks up from startDir looking for .agents/<subdir>/.
func FindAgentsSubdir(startDir, subdir string) string {
	dir := startDir
	for {
		candidate := filepath.Join(dir, ".agents", subdir)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}

		markers := []string{".beads", "crew", "polecats"}
		atRigRoot := false
		for _, marker := range markers {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				atRigRoot = true
				break
			}
		}
		if atRigRoot {
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// FormatKnowledgeMarkdown renders knowledge as markdown.
func FormatKnowledgeMarkdown(k *InjectedKnowledge, compactText func(string) string) string {
	var sb strings.Builder
	sb.WriteString("## Injected Knowledge (ao inject)\n\n")
	WritePredecessorSection(&sb, k.Predecessor)
	WriteLearningsSection(&sb, k.Learnings, compactText)
	WritePatternsSection(&sb, k.Patterns)
	WriteSessionsSection(&sb, k.Sessions)
	if k.Predecessor == nil && len(k.Learnings) == 0 && len(k.Patterns) == 0 && len(k.Sessions) == 0 {
		sb.WriteString("*No prior knowledge found.*\n\n")
	}
	fmt.Fprintf(&sb, "*Last injection: %s*\n", k.Timestamp.Format(time.RFC3339))
	return sb.String()
}

// RenderKnowledge formats the knowledge struct into the requested output format.
func RenderKnowledge(knowledge *InjectedKnowledge, format string, compactText func(string) string) (string, error) {
	if format == "json" {
		data, err := json.MarshalIndent(knowledge, "", "  ")
		if err != nil {
			return "", fmt.Errorf("marshal json: %w", err)
		}
		return string(data), nil
	}
	return FormatKnowledgeMarkdown(knowledge, compactText), nil
}

// WriteLearningsSection renders a learnings block.
func WriteLearningsSection(sb *strings.Builder, learnings []Learning, compactText func(string) string) {
	if len(learnings) == 0 {
		return
	}
	sb.WriteString("### Recent Learnings\n")
	for _, l := range learnings {
		text := l.Title
		if l.Summary != "" {
			text = l.Summary
		}
		if l.SectionHeading != "" {
			text += fmt.Sprintf(" (match: %s", l.SectionHeading)
			if l.MatchedSnippet != "" {
				snippet := l.MatchedSnippet
				if compactText != nil {
					snippet = compactText(snippet)
				}
				text += fmt.Sprintf(" -> %s", snippet)
			}
			text += ")"
		}
		fmt.Fprintf(sb, "- **%s**: %s\n", l.ID, text)
	}
	sb.WriteString("\n")
}

// WritePatternsSection renders an active-patterns block.
func WritePatternsSection(sb *strings.Builder, patterns []Pattern) {
	if len(patterns) == 0 {
		return
	}
	sb.WriteString("### Active Patterns\n")
	for _, p := range patterns {
		if p.Description != "" {
			fmt.Fprintf(sb, "- **%s**: %s\n", p.Name, p.Description)
		} else {
			fmt.Fprintf(sb, "- **%s**\n", p.Name)
		}
	}
	sb.WriteString("\n")
}

// WriteSessionsSection renders recent-sessions block.
func WriteSessionsSection(sb *strings.Builder, sessions []Session) {
	if len(sessions) == 0 {
		return
	}
	sb.WriteString("### Recent Sessions\n")
	for _, s := range sessions {
		fmt.Fprintf(sb, "- [%s] %s\n", s.Date, s.Summary)
	}
	sb.WriteString("\n")
}

// WritePredecessorSection renders the predecessor context block.
func WritePredecessorSection(sb *strings.Builder, pred *PredecessorContext) {
	if pred == nil {
		return
	}
	sb.WriteString("### Predecessor Context")
	if pred.SessionAge != "" {
		fmt.Fprintf(sb, " (%s ago)", pred.SessionAge)
	}
	sb.WriteString("\n")
	if pred.WorkingOn != "" {
		fmt.Fprintf(sb, "- **Working on:** %s\n", pred.WorkingOn)
	}
	if pred.Progress != "" {
		fmt.Fprintf(sb, "- **Progress:** %s\n", pred.Progress)
	}
	if pred.Blocker != "" {
		fmt.Fprintf(sb, "- **Blocker:** %s\n", pred.Blocker)
	}
	if pred.NextStep != "" {
		fmt.Fprintf(sb, "- **Next step:** %s\n", pred.NextStep)
	}
	if pred.RawSummary != "" && pred.Progress == "" {
		fmt.Fprintf(sb, "- %s\n", pred.RawSummary)
	}
	sb.WriteString("\n")
}

// TrimJSONToCharBudget progressively removes items until the JSON fits the budget.
func TrimJSONToCharBudget(knowledge *InjectedKnowledge, budget int) string {
	trimmed := *knowledge
	trimmed.Learnings = append([]Learning(nil), knowledge.Learnings...)
	trimmed.Patterns = append([]Pattern(nil), knowledge.Patterns...)
	trimmed.Sessions = append([]Session(nil), knowledge.Sessions...)

	type truncatedKnowledge struct {
		InjectedKnowledge
		Truncated bool `json:"truncated"`
	}

	tryMarshal := func() string {
		tk := truncatedKnowledge{InjectedKnowledge: trimmed, Truncated: true}
		data, err := json.MarshalIndent(tk, "", "  ")
		if err != nil {
			return "{\"truncated\": true}"
		}
		return string(data)
	}

	for len(trimmed.Sessions) > 0 {
		if out := tryMarshal(); len(out) <= budget {
			return out
		}
		trimmed.Sessions = trimmed.Sessions[:len(trimmed.Sessions)-1]
	}
	for len(trimmed.Patterns) > 0 {
		if out := tryMarshal(); len(out) <= budget {
			return out
		}
		trimmed.Patterns = trimmed.Patterns[:len(trimmed.Patterns)-1]
	}
	for len(trimmed.Learnings) > 0 {
		if out := tryMarshal(); len(out) <= budget {
			return out
		}
		trimmed.Learnings = trimmed.Learnings[:len(trimmed.Learnings)-1]
	}

	return tryMarshal()
}

// ReadFlaggedQualityPaths reads .agents/defrag/quality-report.json and returns the
// list of flagged paths (absolute), or an error if the report is missing or invalid.
func ReadFlaggedQualityPaths(cwd string) ([]string, error) {
	reportPath := filepath.Join(cwd, ".agents", "defrag", "quality-report.json")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no quality report found at %s", reportPath)
		}
		return nil, fmt.Errorf("read quality report: %w", err)
	}

	var report struct {
		FlaggedPaths []string `json:"flagged_paths"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("parse quality report: %w", err)
	}

	out := make([]string, 0, len(report.FlaggedPaths))
	for _, p := range report.FlaggedPaths {
		if filepath.IsAbs(p) {
			out = append(out, p)
		} else {
			out = append(out, filepath.Join(cwd, p))
		}
	}
	return out, nil
}

// TrimToCharBudget truncates markdown output at a line boundary to fit the char budget.
func TrimToCharBudget(output string, budget int) string {
	if len(output) <= budget {
		return output
	}

	lines := strings.Split(output, "\n")
	var result strings.Builder
	for _, line := range lines {
		if result.Len()+len(line)+1 > budget-50 {
			break
		}
		result.WriteString(line)
		result.WriteString("\n")
	}

	result.WriteString("\n*[truncated to fit token budget]*\n")
	return result.String()
}
