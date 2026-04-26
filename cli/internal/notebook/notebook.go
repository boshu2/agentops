// Package notebook provides pure functions for parsing and updating MEMORY.md
// session notebooks.
package notebook

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PendingEntry represents one entry from pending.jsonl (forge output).
type PendingEntry struct {
	SessionID string    `json:"session_id"`
	Summary   string    `json:"summary"`
	Decisions []string  `json:"decisions,omitempty"`
	Knowledge []string  `json:"knowledge,omitempty"`
	QueuedAt  time.Time `json:"queued_at"`
}

// Section represents a parsed section of MEMORY.md.
type Section struct {
	Heading string
	Lines   []string
}

// SessionEntry is used to unmarshal session JSONL files from .agents/ao/sessions/.
type SessionEntry struct {
	SessionID string    `json:"session_id"`
	Date      time.Time `json:"date"`
	Summary   string    `json:"summary,omitempty"`
	Decisions []string  `json:"decisions,omitempty"`
	Knowledge []string  `json:"knowledge,omitempty"`
}

// ToPendingEntry converts a SessionEntry to a PendingEntry.
func (s *SessionEntry) ToPendingEntry() *PendingEntry {
	return &PendingEntry{
		SessionID: s.SessionID,
		Summary:   s.Summary,
		Decisions: s.Decisions,
		Knowledge: s.Knowledge,
		QueuedAt:  s.Date,
	}
}

// FindMemoryFile locates the Claude Code project MEMORY.md for the given cwd.
func FindMemoryFile(cwd string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}

	normalizedPath := strings.ReplaceAll(cwd, "/", "-")
	memoryFile := filepath.Join(homeDir, ".claude", "projects", normalizedPath, "memory", "MEMORY.md")
	if _, err := os.Stat(memoryFile); err == nil {
		return memoryFile, nil
	}

	projectsDir := filepath.Join(homeDir, ".claude", "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return "", fmt.Errorf("no MEMORY.md found")
	}

	lastComponent := filepath.Base(cwd)
	suffix := "-" + lastComponent
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), suffix) {
			candidate := filepath.Join(projectsDir, e.Name(), "memory", "MEMORY.md")
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("no MEMORY.md found for %s", cwd)
}

// ReadLatestPendingEntry reads the most recent entry from pending.jsonl.
func ReadLatestPendingEntry(cwd string) (*PendingEntry, error) {
	pendingPath := filepath.Join(cwd, ".agents", "ao", "pending.jsonl")
	f, err := os.Open(pendingPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var latest *PendingEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry PendingEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		latest = &entry
	}

	return latest, scanner.Err()
}

// ResolveSource reads session data using the specified source strategy.
func ResolveSource(cwd string, source string) (*PendingEntry, error) {
	switch source {
	case "sessions":
		return ReadLatestSessionEntry(cwd)
	case "pending":
		return ReadLatestPendingEntry(cwd)
	case "auto":
		entry, err := ReadLatestSessionEntry(cwd)
		if entry != nil && err == nil {
			return entry, nil
		}
		return ReadLatestPendingEntry(cwd)
	default:
		return nil, fmt.Errorf("unknown source: %s (use auto, sessions, or pending)", source)
	}
}

// ReadLatestSessionEntry reads the most recent session from .agents/ao/sessions/*.jsonl.
func ReadLatestSessionEntry(cwd string) (*PendingEntry, error) {
	sessionsDir := filepath.Join(cwd, ".agents", "ao", "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return nil, err
	}

	var latest string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) != ".jsonl" {
			continue
		}
		if e.Name() > latest {
			latest = e.Name()
		}
	}
	if latest == "" {
		return nil, fmt.Errorf("no session files found in %s", sessionsDir)
	}

	return ReadSessionFile(filepath.Join(sessionsDir, latest))
}

// ReadSessionByID finds and reads a specific session by ID prefix.
func ReadSessionByID(cwd string, id string) (*PendingEntry, error) {
	sessionsDir := filepath.Join(cwd, ".agents", "ao", "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return nil, err
	}

	var matches []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".jsonl" {
			continue
		}
		if strings.Contains(e.Name(), id) {
			matches = append(matches, e.Name())
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("session %s not found", id)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("ambiguous session ID %s matches %d files", id, len(matches))
	}
	return ReadSessionFile(filepath.Join(sessionsDir, matches[0]))
}

// ReadSessionFile reads a single session JSONL file.
func ReadSessionFile(path string) (*PendingEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)
	if scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			return nil, fmt.Errorf("empty session file: %s", path)
		}
		var entry SessionEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("parse session file %s: %w", path, err)
		}
		return entry.ToPendingEntry(), nil
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read session file %s: %w", path, err)
	}
	return nil, fmt.Errorf("empty session file: %s", path)
}

// ParseSections reads MEMORY.md and splits it into sections.
func ParseSections(path string) ([]Section, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseSectionsFromString(string(data)), nil
}

// ParseSectionsFromString splits markdown content into sections by ## headings.
func ParseSectionsFromString(content string) []Section {
	trimmed := strings.TrimRight(content, "\n")
	if trimmed == "" {
		return nil
	}
	lines := strings.Split(trimmed, "\n")
	var sections []Section
	var current *Section

	for i, line := range lines {
		prevBlank := i == 0 || strings.TrimSpace(lines[i-1]) == ""
		if (strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "# ")) && prevBlank {
			if current != nil {
				sections = append(sections, *current)
			}
			current = &Section{
				Heading: line,
				Lines:   nil,
			}
		} else if current != nil {
			current.Lines = append(current.Lines, line)
		} else {
			if len(sections) == 0 {
				current = &Section{Heading: "", Lines: nil}
			}
			if current != nil {
				current.Lines = append(current.Lines, line)
			}
		}
	}
	if current != nil {
		sections = append(sections, *current)
	}

	return sections
}

// CategorizeKnowledge splits knowledge items into worked/next/other buckets by prefix.
func CategorizeKnowledge(items []string) (worked, next, other []string) {
	for _, k := range items {
		kLower := strings.ToLower(k)
		switch {
		case strings.HasPrefix(kLower, "next:") || strings.HasPrefix(kLower, "next ") ||
			strings.HasPrefix(kLower, "todo:") || strings.HasPrefix(kLower, "todo ") ||
			strings.HasPrefix(kLower, "follow-up:") || strings.HasPrefix(kLower, "follow-up "):
			next = append(next, k)
		case strings.HasPrefix(kLower, "worked:") || strings.HasPrefix(kLower, "worked ") ||
			strings.HasPrefix(kLower, "success:") || strings.HasPrefix(kLower, "success ") ||
			strings.HasPrefix(kLower, "resolved:") || strings.HasPrefix(kLower, "resolved "):
			worked = append(worked, k)
		default:
			other = append(other, k)
		}
	}
	return
}

// Truncate shortens s to at most n runes, appending "..." if truncated.
func Truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	if n <= 3 {
		return "..."[:n]
	}
	return string(runes[:n-3]) + "..."
}

// AppendBulletSection appends a heading + indented bullet list.
func AppendBulletSection(lines []string, heading string, items []string, maxLen int, truncate func(string, int) string) []string {
	if len(items) == 0 {
		return lines
	}
	lines = append(lines, heading)
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("  - %s", truncate(item, maxLen)))
	}
	return lines
}

// BuildLastSessionSection creates the "Last Session" content from a pending entry.
func BuildLastSessionSection(entry *PendingEntry, truncate func(string, int) string) Section {
	var lines []string

	date := entry.QueuedAt.Format("2006-01-02")
	if entry.QueuedAt.IsZero() {
		date = time.Now().Format("2006-01-02")
	}

	lines = append(lines, fmt.Sprintf("- **Date:** %s", date))

	if entry.Summary != "" {
		lines = append(lines, fmt.Sprintf("- **Summary:** %s", truncate(entry.Summary, 200)))
	}

	if len(entry.Decisions) > 0 {
		lines = append(lines, "- **Key decisions:**")
		for _, d := range entry.Decisions {
			lines = append(lines, fmt.Sprintf("  - %s", truncate(d, 150)))
		}
	}

	worked, next, other := CategorizeKnowledge(entry.Knowledge)
	lines = AppendBulletSection(lines, "- **What worked:**", worked, 150, truncate)
	lines = AppendBulletSection(lines, "- **Next:**", next, 150, truncate)
	lines = AppendBulletSection(lines, "- **Insights:**", other, 150, truncate)

	lines = append(lines, "")

	return Section{
		Heading: "## Last Session",
		Lines:   lines,
	}
}

// UpsertLastSession replaces or inserts the "Last Session" section.
func UpsertLastSession(sections []Section, lastSession Section) []Section {
	for i, s := range sections {
		if s.Heading == "## Last Session" {
			sections[i] = lastSession
			result := sections[:i+1]
			for _, rem := range sections[i+1:] {
				if rem.Heading != "## Last Session" {
					result = append(result, rem)
				}
			}
			return result
		}
	}

	if len(sections) == 0 {
		return []Section{lastSession}
	}

	result := make([]Section, 0, len(sections)+1)
	result = append(result, sections[0])
	result = append(result, lastSession)
	result = append(result, sections[1:]...)
	return result
}

// Prune trims the longest sections to stay under the line budget.
func Prune(sections []Section, maxLines int) []Section {
	const maxIterations = 100
	iteration := 0
	for TotalLines(sections) > maxLines && iteration < maxIterations {
		iteration++
		longestIdx := -1
		longestLen := 0
		for i, s := range sections {
			if s.Heading == "" || s.Heading == "## Last Session" {
				continue
			}
			if strings.HasPrefix(s.Heading, "# ") && !strings.HasPrefix(s.Heading, "## ") {
				continue
			}
			contentLen := len(s.Lines)
			if contentLen > longestLen {
				longestLen = contentLen
				longestIdx = i
			}
		}

		if longestIdx < 0 || longestLen == 0 {
			break
		}

		lines := sections[longestIdx].Lines
		for j := len(lines) - 1; j >= 0; j-- {
			if strings.TrimSpace(lines[j]) != "" {
				sections[longestIdx].Lines = append(lines[:j], lines[j+1:]...)
				break
			}
		}
	}

	return sections
}

// TotalLines counts the total lines across all sections.
func TotalLines(sections []Section) int {
	count := 0
	for _, s := range sections {
		if s.Heading != "" {
			count++
		}
		count += len(s.Lines)
	}
	return count
}

// Render produces the final MEMORY.md content from sections.
func Render(sections []Section) string {
	var b strings.Builder
	for _, s := range sections {
		if s.Heading != "" {
			b.WriteString(s.Heading)
			b.WriteString("\n")
		}
		for _, line := range s.Lines {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// ReadCursor reads the last-processed session ID from the cursor file.
func ReadCursor(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var cursor struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(data, &cursor); err != nil {
		return "", err
	}
	return cursor.SessionID, nil
}

// WriteCursor writes the last-processed session ID to the cursor file.
func WriteCursor(path string, sessionID string) error {
	data, err := json.Marshal(struct {
		SessionID string `json:"session_id"`
		UpdatedAt string `json:"updated_at"`
	}{
		SessionID: sessionID,
		UpdatedAt: time.Now().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0600)
}
