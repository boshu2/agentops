// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import "strings"

// sessionTypeBoost returns a scoring multiplier for learnings matching the
// current session type. Exact match: 1.3x, related: 1.15x, otherwise 1.0x.
func sessionTypeBoost(l learning, sessionType string) float64 {
	if sessionType == "" {
		return 1.0
	}
	if l.SessionType == sessionType {
		return 1.3
	}
	if isRelatedSessionType(l.SessionType, sessionType) {
		return 1.15
	}
	return 1.0
}

func isRelatedSessionType(a, b string) bool {
	related := map[string][]string{
		"career":     {"coaching", "interview"},
		"coaching":   {"career", "interview"},
		"debug":      {"debugging", "troubleshoot"},
		"debugging":  {"debug", "troubleshoot"},
		"research":   {"brainstorm", "explore"},
		"brainstorm": {"research", "explore"},
	}
	for _, r := range related[a] {
		if r == b {
			return true
		}
	}
	return false
}

// detectSessionTypeFromGoal infers session type from predecessor handoff goal text.
func detectSessionTypeFromGoal(goal string) string {
	goal = strings.ToLower(goal)
	switch {
	case strings.Contains(goal, "career") || strings.Contains(goal, "interview") ||
		strings.Contains(goal, "resume") || strings.Contains(goal, "job"):
		return "career"
	case strings.Contains(goal, "debug") || strings.Contains(goal, "fix") ||
		strings.Contains(goal, "broken"):
		return "debug"
	case strings.Contains(goal, "research") || strings.Contains(goal, "explore"):
		return "research"
	case strings.Contains(goal, "brainstorm") || strings.Contains(goal, "design"):
		return "brainstorm"
	default:
		return "implement"
	}
}
