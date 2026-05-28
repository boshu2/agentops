// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/search"
)

const (
	// maxPredecessorTokens is the token budget reserved for predecessor context.
	maxPredecessorTokens = 200
	// maxPredecessorChars is the character budget (tokens * 4).
	maxPredecessorChars = maxPredecessorTokens * InjectCharsPerToken
)

// predecessorContext — canonical definition in internal/search/types.go.
type predecessorContext = search.PredecessorContext

// parsePredecessorFile reads a handoff file and extracts structured predecessor context.
// Supports three handoff formats:
//   - Explicit handoff (/handoff skill): has ## headers like Accomplishments, Pause Point, Blockers
//   - Auto-handoff (stop hook): prose summary, may have ## headers
//   - Pre-compact snapshot: has Branch, Status, Teams sections
//
// Falls back to first-N-paragraphs when no recognized headers are found.
func parsePredecessorFile(path string) *predecessorContext {
	data, err := os.ReadFile(path)
	if err != nil {
		VerbosePrintf("Warning: failed to read predecessor file: %v\n", err)
		return nil
	}

	content := string(data)
	if strings.TrimSpace(content) == "" {
		return nil
	}

	ctx := &predecessorContext{}

	// Extract structured sections by header
	sections := extractSections(content)

	// Map sections to context fields
	ctx.WorkingOn = findSection(sections, "working on", "topic", "task", "hook", "bead")
	ctx.Progress = findSection(sections, "accomplishments", "progress", "completed", "done", "what happened")
	ctx.Blocker = findSection(sections, "blockers", "blocker", "blocked", "issues", "problems")
	ctx.NextStep = findSection(sections, "next step", "next steps", "continuation", "pause point", "resume")

	// If no structured headers found, use first paragraphs as raw summary
	if ctx.Progress == "" && ctx.NextStep == "" && ctx.Blocker == "" {
		ctx.RawSummary = extractFirstParagraphs(content, 3)
	}

	// Derive working-on from filename if not in content
	if ctx.WorkingOn == "" {
		ctx.WorkingOn = deriveTopicFromPath(path)
	}

	// Derive session age from file modification time
	if info, err := os.Stat(path); err == nil {
		ctx.SessionAge = formatAgeSince(info.ModTime())
	}

	// Truncate all fields to fit token budget
	truncatePredecessor(ctx)

	return ctx
}

// Thin wrappers — canonical definitions in internal/search/predecessor.go.
func extractSections(content string) map[string]string { return search.ExtractSections(content) }
func findSection(sections map[string]string, candidates ...string) string {
	return search.FindSection(sections, candidates...)
}
func extractFirstParagraphs(content string, maxParas int) string {
	return search.ExtractFirstParagraphs(content, maxParas)
}
func deriveTopicFromPath(path string) string { return search.DeriveTopicFromPath(path) }
func truncatePredecessor(ctx *predecessorContext) {
	search.TruncatePredecessor(ctx, maxPredecessorChars)
}

func formatAgeSince(t time.Time) string {
	age := time.Since(t)
	if age < time.Minute {
		return fmt.Sprintf("%ds ago", int(age.Seconds()))
	}
	if age < time.Hour {
		return fmt.Sprintf("%dm ago", int(age.Minutes()))
	}
	if age < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(age.Hours()))
	}
	return t.Format("Jan 2")
}
