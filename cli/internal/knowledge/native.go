package knowledge

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Tokens splits text into lowercase alphanumeric tokens of length >= 3,
// deduplicated.
func Tokens(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
	tokens := make([]string, 0, len(fields))
	for _, field := range fields {
		if len(field) >= 3 {
			tokens = append(tokens, field)
		}
	}
	return DedupeStrings(tokens)
}

// HealthRank returns a numeric rank for topic health, higher is better.
func HealthRank(health string) int {
	switch strings.ToLower(strings.TrimSpace(health)) {
	case "healthy":
		return 2
	case "thin":
		return 1
	default:
		return 0
	}
}

// ChunkRank returns a numeric rank for chunk types, higher is more
// salient.
func ChunkRank(chunkType string) int {
	switch strings.ToLower(strings.TrimSpace(chunkType)) {
	case "decision":
		return 3
	case "pattern":
		return 2
	case "overview":
		return 1
	default:
		return 0
	}
}

// AppendCandidate appends a normalized candidate to items unless it is
// empty or already present.
func AppendCandidate(items []string, candidate string) []string {
	candidate = strings.Join(strings.Fields(strings.TrimSpace(candidate)), " ")
	if candidate == "" {
		return items
	}
	for _, item := range items {
		if item == candidate {
			return items
		}
	}
	return append(items, candidate)
}

// ContainsTopic reports whether topics already contains a topic with the
// given ID.
func ContainsTopic(topics []TopicDetail, topicID string) bool {
	for _, topic := range topics {
		if topic.ID == topicID {
			return true
		}
	}
	return false
}

// FieldValue extracts the value portion of a "- Field: value" markdown
// line, trimming surrounding backticks.
func FieldValue(line string) string {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return ""
	}
	value := strings.TrimSpace(parts[1])
	return strings.Trim(value, "`")
}

// BuilderGoal returns the value following --goal in args, or the empty
// string if not present.
func BuilderGoal(args []string) string {
	for idx := 0; idx < len(args); idx++ {
		if args[idx] != "--goal" {
			continue
		}
		if idx+1 < len(args) {
			return strings.TrimSpace(args[idx+1])
		}
	}
	return ""
}

// SlicesContain reports whether items contains target (exact match).
func SlicesContain(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

// StringSliceContainsFold reports whether items contains needle using a
// case-insensitive comparison.
func StringSliceContainsFold(items []string, needle string) bool {
	for _, item := range items {
		if strings.EqualFold(item, needle) {
			return true
		}
	}
	return false
}

// YesNo returns "yes" for true and "no" for false.
func YesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

// SectionText returns the collapsed-whitespace text under a markdown
// heading, stopping at the next "## " heading.
func SectionText(text, heading string) string {
	lines := strings.Split(text, "\n")
	var section []string
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
		section = append(section, strings.TrimSpace(line))
	}
	return strings.Join(strings.Fields(strings.Join(section, " ")), " ")
}

// FrontmatterStringList returns a deduped string list for key from
// frontmatter. Handles []string, []any, and scalar fallbacks.
func FrontmatterStringList(frontmatter map[string]any, key string) []string {
	if frontmatter == nil {
		return nil
	}
	raw, ok := frontmatter[key]
	if !ok {
		return nil
	}

	switch typed := raw.(type) {
	case []string:
		return DedupeStrings(typed)
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" && text != "<nil>" {
				values = append(values, text)
			}
		}
		return DedupeStrings(values)
	default:
		text := strings.TrimSpace(fmt.Sprint(typed))
		if text == "" || text == "<nil>" {
			return nil
		}
		return []string{text}
	}
}

// FrontmatterNestedInt extracts an int from a nested map under parent/key.
func FrontmatterNestedInt(frontmatter map[string]any, parent, key string) int {
	if frontmatter == nil {
		return 0
	}
	raw, ok := frontmatter[parent]
	if !ok {
		return 0
	}
	nested, ok := raw.(map[string]any)
	if !ok {
		return 0
	}
	value, ok := nested[key]
	if !ok {
		return 0
	}

	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		var parsed int
		_, _ = fmt.Sscanf(fmt.Sprint(typed), "%d", &parsed)
		return parsed
	}
}

// ParseChunks parses a chunk bundle markdown body into a list of chunk
// states from the "## Knowledge Chunks" section.
func ParseChunks(text string) []ChunkState {
	lines := strings.Split(text, "\n")
	chunks := make([]ChunkState, 0, 8)
	inSection := false
	var current *ChunkState

	flush := func() {
		if current == nil {
			return
		}
		if strings.TrimSpace(current.ID) != "" || strings.TrimSpace(current.Claim) != "" {
			chunks = append(chunks, *current)
		}
		current = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "## Knowledge Chunks":
			inSection = true
		case !inSection:
			continue
		case strings.HasPrefix(trimmed, "## "):
			flush()
			return chunks
		case strings.HasPrefix(trimmed, "### "):
			flush()
			current = &ChunkState{}
		case current == nil:
			continue
		case strings.HasPrefix(trimmed, "- Chunk ID:"):
			current.ID = FieldValue(trimmed)
		case strings.HasPrefix(trimmed, "- Type:"):
			current.Type = FieldValue(trimmed)
		case strings.HasPrefix(trimmed, "- Confidence:"):
			current.Confidence = FieldValue(trimmed)
		case strings.HasPrefix(trimmed, "- Claim:"):
			current.Claim = FieldValue(trimmed)
		}
	}
	flush()
	return chunks
}

// WhenToUse renders the "When To Use" blurb for a topic playbook.
func WhenToUse(topic TopicDetail) string {
	phrases := make([]string, 0, 2)
	for _, item := range topic.Aliases {
		phrases = append(phrases, strings.ToLower(strings.TrimSpace(item)))
		if len(phrases) == 2 {
			break
		}
	}
	if len(phrases) == 0 {
		phrases = append(phrases, strings.ToLower(topic.Title))
	}
	return fmt.Sprintf("Use this playbook when the task is primarily about %s and you need a bounded operator loop instead of freeform exploration.", strings.Join(phrases, ", "))
}

// PrimitiveDescriptions returns the canonical operator primitive
// descriptions used when rendering playbooks.
func PrimitiveDescriptions() map[string]string {
	return map[string]string{
		"fitness gradient":       "Defines what better and worse look like for this topic.",
		"stateful environment":   "Captures the durable context, artifacts, and rules that carry continuity.",
		"replaceable actors":     "Keeps execution bound to narrow, swappable workers instead of one special actor.",
		"stigmergic traces":      "Uses durable traces such as packets, citations, logs, and handoffs for coordination.",
		"selection gates":        "Applies explicit checks that decide what is allowed to survive.",
		"evolutionary promotion": "Promotes validated patterns back into reusable defaults.",
		"governance":             "Shapes boundaries, ownership, and allowed moves for the operator loop.",
	}
}

// PrimitivesForTopic picks the operator primitives most relevant to a
// topic based on keyword hits.
func PrimitivesForTopic(topic TopicDetail) []string {
	corpus := strings.ToLower(strings.Join(append(append(append([]string{topic.Title, topic.Summary}, topic.Aliases...), topic.KeyDecisions...), topic.RepeatedPatterns...), " "))
	keywords := map[string][]string{
		"fitness gradient":       {"goal", "fitness", "validation", "acceptance", "test", "review", "gate"},
		"stateful environment":   {"context", "memory", "environment", "packet", "state", "control plane", "knowledge"},
		"replaceable actors":     {"actor", "agent", "worker", "handoff", "owner", "ownership", "swarm"},
		"stigmergic traces":      {"trace", "provenance", "citation", "handoff", "log", "queue", "artifact"},
		"selection gates":        {"gate", "validation", "check", "policy", "proof", "review", "premortem"},
		"evolutionary promotion": {"promotion", "promote", "retro", "learning", "flywheel", "reuse", "playbook"},
		"governance":             {"governance", "scope", "boundary", "operator", "policy", "constraint"},
	}
	type scored struct {
		primitive string
		score     int
	}
	var scoredPrimitives []scored
	for primitive, hints := range keywords {
		score := 0
		for _, hint := range hints {
			if strings.Contains(corpus, hint) {
				score++
			}
		}
		if score > 0 {
			scoredPrimitives = append(scoredPrimitives, scored{primitive: primitive, score: score})
		}
	}
	sort.Slice(scoredPrimitives, func(i, j int) bool {
		if scoredPrimitives[i].score != scoredPrimitives[j].score {
			return scoredPrimitives[i].score > scoredPrimitives[j].score
		}
		return scoredPrimitives[i].primitive < scoredPrimitives[j].primitive
	})
	selected := make([]string, 0, 3)
	selected = append(selected, "stateful environment")
	for _, item := range scoredPrimitives {
		if StringSliceContainsFold(selected, item.primitive) {
			continue
		}
		selected = append(selected, item.primitive)
		if len(selected) == 3 {
			break
		}
	}
	if len(selected) < 2 {
		selected = append(selected, "selection gates")
	}
	return DedupeStrings(selected)
}

// RenderPlaybooksIndex renders the markdown table index for playbook
// rows.
func RenderPlaybooksIndex(rows []PlaybookRow) string {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Health != rows[j].Health {
			return HealthRank(rows[i].Health) > HealthRank(rows[j].Health)
		}
		return rows[i].Topic < rows[j].Topic
	})

	var b strings.Builder
	b.WriteString("# Playbook Candidates\n\n")
	b.WriteString("Candidate operator playbooks promoted from topic packets.\n\n")
	b.WriteString("| Topic | Health | Canonical |\n")
	b.WriteString("|---|---|---|\n")
	for _, row := range rows {
		canonical := "no"
		if row.Canonical {
			canonical = "yes"
		}
		fmt.Fprintf(&b, "| [%s](%s) | `%s` | `%s` |\n", row.Topic, row.Path, row.Health, canonical)
	}
	return b.String()
}

// RenderBeliefBook renders the belief book markdown given the collected
// content sections.
func RenderBeliefBook(outputPath, sourcePath string, coreBeliefs, operatingPrinciples, thinTopics, sourceSurfaces []string) string {
	datePrefix := time.Now().Format("2006-01-02")
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "id: knowledge-book-of-beliefs-%s\n", datePrefix)
	b.WriteString("type: principle-book\n")
	fmt.Fprintf(&b, "date: %s\n", datePrefix)
	if sourcePath != "" {
		fmt.Fprintf(&b, "source: %s\n", sourcePath)
	}
	b.WriteString("status: generated\n")
	b.WriteString("---\n\n")
	b.WriteString("# Book Of Beliefs\n\n")
	b.WriteString("Cross-domain operating beliefs promoted from the `.agents` corpus.\n\n")
	b.WriteString("## Canonical Vocabulary\n\n")
	for _, primitive := range []string{
		"fitness gradient",
		"stateful environment",
		"replaceable actors",
		"stigmergic traces",
		"selection gates",
		"evolutionary promotion",
		"governance",
	} {
		fmt.Fprintf(&b, "- %s\n", primitive)
	}
	b.WriteString("## Core Beliefs\n\n")
	for idx, belief := range coreBeliefs {
		fmt.Fprintf(&b, "%d. %s\n", idx+1, belief)
	}
	if len(coreBeliefs) == 0 {
		b.WriteString("1. Promoted topic packets have not surfaced stable beliefs yet.\n")
	}
	b.WriteString("\n## Operating Principles\n\n")
	for _, principle := range operatingPrinciples {
		fmt.Fprintf(&b, "- %s\n", principle)
	}
	if len(operatingPrinciples) == 0 {
		b.WriteString("- No operating principles surfaced from the current topic packets.\n")
	}
	b.WriteString("\n## Translation Map\n\n")
	for _, item := range []string{
		"context is the control plane -> stateful environment + governance",
		"distributed cognition -> replaceable actors coordinating through a shared environment",
		"stigmergy -> stigmergic traces in the environment",
		"flywheel -> evolutionary promotion after selection gates",
	} {
		fmt.Fprintf(&b, "- %s\n", item)
	}
	b.WriteString("\n## Thin-Topic Cautions\n\n")
	if len(thinTopics) == 0 {
		b.WriteString("- None surfaced\n")
	} else {
		for _, title := range thinTopics {
			fmt.Fprintf(&b, "- %s\n", title)
		}
	}
	b.WriteString("\n## Source Surfaces\n\n")
	for _, source := range sourceSurfaces {
		fmt.Fprintf(&b, "- `%s`\n", source)
	}
	if len(sourceSurfaces) == 0 {
		b.WriteString("- No index surfaces were found.\n")
	}
	b.WriteString("\n## Refresh Command\n\n")
	b.WriteString("`ao knowledge beliefs`\n")
	return b.String()
}
