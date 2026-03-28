package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/storage"
)

// writePendingLearnings writes forge-extracted knowledge as markdown files
// to .agents/knowledge/pending/ for pool ingestion by close-loop.
// This bridges the gap between forge output and pool ingest input.
func writePendingLearnings(session *storage.Session, baseDir string) (int, error) {
	if session == nil {
		return 0, nil
	}

	// Collect all items: knowledge + decisions
	type item struct {
		text     string
		category string
	}
	var items []item
	for _, k := range session.Knowledge {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		items = append(items, item{text: k, category: inferCategory(k)})
	}
	for _, d := range session.Decisions {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		items = append(items, item{text: d, category: "decision"})
	}

	if len(items) == 0 {
		return 0, nil
	}

	pendingDir := filepath.Join(baseDir, ".agents", "knowledge", "pending")
	if err := os.MkdirAll(pendingDir, 0700); err != nil {
		return 0, fmt.Errorf("create pending dir: %w", err)
	}

	// Guard against zero date — use current date as fallback
	dateStr := session.Date.Format("2006-01-02")
	if session.Date.IsZero() {
		dateStr = time.Now().Format("2006-01-02")
	}

	// Guard against empty session ID — generate a short hash from content
	sessionShort := session.ID
	if sessionShort == "" {
		sessionShort = fmt.Sprintf("anon%d", time.Now().UnixNano()%100000)
	}
	if len(sessionShort) > 7 {
		sessionShort = sessionShort[:7]
	}
	// Sanitize session ID to prevent path traversal
	sessionShort = sanitizePathComponent(sessionShort)

	written := 0
	for i, it := range items {
		title := pendingTitle(it.text)
		id := fmt.Sprintf("%s-%s-%d", dateStr, sessionShort, i+1)
		filename := fmt.Sprintf("%s-%s-%d.md", dateStr, sessionShort, i+1)

		safeText := escapeFrontmatterText(it.text)
		safeSessionID := sanitizePathComponent(session.ID)

		// Carry research provenance into pending learnings so closure metrics
		// can trace learnings back to .agents/research/ sources (ag-73u.4).
		researchSources := renderResearchSourcesFrontmatter(gatherResearchSources(it.text))

		content := fmt.Sprintf(`---
date: %s
type: %s
source: %s
%s---

# Learning: %s

**ID**: %s
**Category**: %s
**Confidence**: medium

%s

## Source

- **Session**: %s
`, dateStr, it.category, safeSessionID, researchSources, title, id, it.category, safeText, safeSessionID)

		path := filepath.Join(pendingDir, filename)
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			return written, fmt.Errorf("write %s: %w", filename, err)
		}
		written++
	}

	return written, nil
}

// inferCategory guesses the knowledge type from content.
func inferCategory(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "decided") || strings.Contains(lower, "decision") ||
		strings.Contains(lower, "chose") || strings.Contains(lower, "selected"):
		return "decision"
	case strings.Contains(lower, "fix") || strings.Contains(lower, "solved") ||
		strings.Contains(lower, "workaround") || strings.Contains(lower, "solution"):
		return "solution"
	case strings.Contains(lower, "failed") || strings.Contains(lower, "broke") ||
		strings.Contains(lower, "bug") || strings.Contains(lower, "error"):
		return "failure"
	default:
		return "learning"
	}
}

// sanitizePathComponent removes path traversal characters from a string
// intended for use as a filename component.
func sanitizePathComponent(s string) string {
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "\\", "-")
	s = strings.ReplaceAll(s, "..", "-")
	s = strings.ReplaceAll(s, "\x00", "")
	return s
}

// escapeFrontmatterText indents any bare "---" lines in text to prevent
// YAML frontmatter delimiter injection.
func escapeFrontmatterText(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			lines[i] = "  ---"
		}
	}
	return strings.Join(lines, "\n")
}

// pendingTitle takes the first meaningful line of text as a title for a pending learning.
func pendingTitle(text string) string {
	line := text
	if idx := strings.IndexByte(line, '\n'); idx != -1 {
		line = line[:idx]
	}
	line = strings.TrimSpace(line)
	line = strings.TrimLeft(line, "# -•*")
	line = strings.TrimSpace(line)
	if len(line) > 80 {
		line = line[:77] + "..."
	}
	if line == "" {
		line = "Extracted knowledge"
	}
	return line
}
