package contextpacket

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/search"
)

// NormalizeAssemblePhase normalizes a phase string to one of the canonical phases.
func NormalizeAssemblePhase(phase string) string {
	switch strings.ToLower(strings.TrimSpace(phase)) {
	case "", "task":
		return "task"
	case "startup", "start":
		return "startup"
	case "planning", "plan":
		return "planning"
	case "pre-mortem", "premortem", "pre_mortem":
		return "pre-mortem"
	case "validation", "validate":
		return "validation"
	default:
		return "task"
	}
}

// ContextBundleLimit returns how many items to include based on a character budget.
func ContextBundleLimit(budget int) int {
	switch {
	case budget >= 18000:
		return 6
	case budget >= 12000:
		return 5
	case budget >= 7000:
		return 4
	default:
		return 3
	}
}

// CompactText collapses all whitespace in a string to single spaces and trims edges.
func CompactText(input string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(input)), " ")
}

// FirstNonEmpty returns the first non-blank string from the arguments.
func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// JoinBullet formats a title and details into a single truncated bullet string.
func JoinBullet(title, details string) string {
	title = CompactText(title)
	details = CompactText(details)
	switch {
	case title == "" && details == "":
		return ""
	case details == "":
		return search.TruncateText(title, 220)
	case title == "":
		return search.TruncateText(details, 220)
	default:
		return search.TruncateText(title+" - "+details, 220)
	}
}

// StringBullets compacts and truncates a slice of raw strings into bullet text.
func StringBullets(items []string) []string {
	bullets := make([]string, 0, len(items))
	for _, item := range items {
		item = CompactText(item)
		if item == "" {
			continue
		}
		bullets = append(bullets, search.TruncateText(item, 220))
	}
	return bullets
}

// LearningBullets formats a slice of learnings into bullet strings.
func LearningBullets(items []search.Learning) []string {
	bullets := make([]string, 0, len(items))
	for _, item := range items {
		title := FirstNonEmpty(item.Title, item.ID)
		summary := CompactText(FirstNonEmpty(item.Summary, item.BodyText))
		bullets = append(bullets, JoinBullet(title, summary))
	}
	return bullets
}

// PatternBullets formats a slice of patterns into bullet strings.
func PatternBullets(items []search.Pattern) []string {
	bullets := make([]string, 0, len(items))
	for _, item := range items {
		name := FirstNonEmpty(item.Name, filepath.Base(item.FilePath))
		description := CompactText(item.Description)
		bullets = append(bullets, JoinBullet(name, description))
	}
	return bullets
}

// FindingBullets formats a slice of findings into bullet strings with severity prefixes.
func FindingBullets(items []search.KnowledgeFinding) []string {
	bullets := make([]string, 0, len(items))
	for _, item := range items {
		title := FirstNonEmpty(item.Title, item.ID)
		summary := CompactText(item.Summary)
		bullet := JoinBullet(title, summary)
		if sev := strings.TrimSpace(item.Severity); sev != "" {
			bullet = fmt.Sprintf("[%s] %s", strings.ToUpper(sev), bullet)
		}
		bullets = append(bullets, bullet)
	}
	return bullets
}

// SessionBullets formats a slice of sessions into bullet strings.
func SessionBullets(items []search.Session) []string {
	bullets := make([]string, 0, len(items))
	for _, item := range items {
		title := strings.TrimSpace(item.Date)
		summary := CompactText(item.Summary)
		bullets = append(bullets, JoinBullet(title, summary))
	}
	return bullets
}

// PrioritizeFindings reorders findings so preferred IDs come first, then truncates to limit.
func PrioritizeFindings(findings []search.KnowledgeFinding, preferredIDs []string, limit int) []search.KnowledgeFinding {
	if len(findings) == 0 {
		return nil
	}

	ordered := make([]search.KnowledgeFinding, 0, len(findings))
	seen := make(map[string]bool, len(findings))
	for _, id := range preferredIDs {
		for _, finding := range findings {
			if finding.ID != id || seen[finding.ID] {
				continue
			}
			ordered = append(ordered, finding)
			seen[finding.ID] = true
			break
		}
	}
	for _, finding := range findings {
		if seen[finding.ID] {
			continue
		}
		ordered = append(ordered, finding)
		seen[finding.ID] = true
	}
	if limit > 0 && len(ordered) > limit {
		ordered = ordered[:limit]
	}
	return ordered
}

// LimitLearnings returns a defensive copy of items truncated to limit.
func LimitLearnings(items []search.Learning, limit int) []search.Learning {
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return append([]search.Learning(nil), items...)
}

// LimitPatterns returns a defensive copy of items truncated to limit.
func LimitPatterns(items []search.Pattern, limit int) []search.Pattern {
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return append([]search.Pattern(nil), items...)
}

// LimitSessions returns a defensive copy of items truncated to limit.
func LimitSessions(items []search.Session, limit int) []search.Session {
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return append([]search.Session(nil), items...)
}
