// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"fmt"
	"strings"
	"time"
)

// renderKnowledgeIndex formats knowledge as a compact index table.
// This is the Phase 1 output for two-phase injection: lightweight discovery
// that costs ~200 tokens instead of ~800 for full content.
func renderKnowledgeIndex(k *injectedKnowledge) string {
	var sb strings.Builder
	sb.WriteString("## Knowledge Index (ao inject)\n\n")

	// Write predecessor context first (already compact)
	writePredecessorSection(&sb, k.Predecessor)

	hasContent := false

	// Learnings index
	if len(k.Learnings) > 0 {
		hasContent = true
		sb.WriteString("### Learnings\n\n")
		sb.WriteString("| ID | Title | Age | Score |\n")
		sb.WriteString("|----|-------|-----|-------|\n")
		for _, l := range k.Learnings {
			age := formatLookupAge(l.AgeWeeks)
			title := truncateText(l.Title, 60)
			fmt.Fprintf(&sb, "| %s | %s | %s | %.2f |\n",
				l.ID, title, age, l.CompositeScore)
		}
		sb.WriteString("\n")
	}

	// Patterns index
	if len(k.Patterns) > 0 {
		hasContent = true
		sb.WriteString("### Patterns\n\n")
		sb.WriteString("| Name | Description | Score |\n")
		sb.WriteString("|------|-------------|-------|\n")
		for _, p := range k.Patterns {
			desc := truncateText(p.Description, 60)
			fmt.Fprintf(&sb, "| %s | %s | %.2f |\n",
				p.Name, desc, p.CompositeScore)
		}
		sb.WriteString("\n")
	}

	if !hasContent && k.Predecessor == nil {
		sb.WriteString("*No prior knowledge found.*\n\n")
	}

	sb.WriteString("*Use `ao lookup <id>` to read full content. Use `ao lookup --query \"topic\"` to search.*\n")
	fmt.Fprintf(&sb, "*Last injection: %s*\n", k.Timestamp.Format(time.RFC3339))
	return sb.String()
}

// formatLookupAge is defined in lookup.go — shared by both inject index and lookup output.
