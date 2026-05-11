// practices: [wiki-knowledge-surface, event-sourcing-cqrs]
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var (
	memorySyncQuiet      bool
	memorySyncMaxEntries int
	memorySyncOutput     string
)

const (
	memoryBlockStart = "<!-- ao:memory:start -->"
	memoryBlockEnd   = "<!-- ao:memory:end -->"
)

// memoryCmd is the parent for memory-related subcommands.
var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Manage repo-root MEMORY.md for cross-runtime access",
}

var memorySyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync session history to repo-root MEMORY.md",
	Long: `Write recent session history to a repo-root MEMORY.md with managed block markers.

This enables cross-runtime access (Codex, OpenCode) where Claude Code's
auto-load mechanism (~/.claude/projects/.../MEMORY.md) is unavailable.

Content outside the managed block markers is preserved. Content inside
the markers is replaced on each sync.`,
	RunE: runMemorySync,
}

func init() {
	memoryCmd.GroupID = "config"
	rootCmd.AddCommand(memoryCmd)
	memorySyncCmd.Flags().BoolVar(&memorySyncQuiet, "quiet", false, "Suppress output")
	memorySyncCmd.Flags().IntVar(&memorySyncMaxEntries, "max-entries", 10, "Maximum session entries to keep")
	memorySyncCmd.Flags().StringVar(&memorySyncOutput, "output-file", "", "Output path (default: MEMORY.md in repo root)")

	memoryCmd.AddCommand(memorySyncCmd)
}

func runMemorySync(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Determine output path
	outputPath := memorySyncOutput
	if outputPath == "" {
		root := findGitRoot(cwd)
		if root == "" {
			root = cwd
		}
		outputPath = filepath.Join(root, "MEMORY.md")
	}

	return syncMemory(cwd, outputPath, memorySyncMaxEntries, memorySyncQuiet)
}

// syncMemory is the testable core of memory sync, free of Cobra global state.
func syncMemory(cwd, outputPath string, maxEntries int, quiet bool) error {
	// Read recent sessions
	entries, err := readNLatestSessionEntries(cwd, maxEntries)
	if err != nil {
		return fmt.Errorf("read sessions: %w", err)
	}
	if len(entries) == 0 {
		if !quiet {
			VerbosePrintf("No session data available for memory sync\n")
		}
		return nil
	}

	// Read existing file (or start fresh)
	var existing string
	if data, err := os.ReadFile(outputPath); err == nil {
		existing = string(data)
	}

	// Parse managed block
	before, managed, after := parseManagedBlock(existing)

	// Build new managed content from sessions, deduplicating against existing.
	// IDs are truncated to 7 chars (matching formatMemoryEntry output).
	existingIDs := extractSessionIDs(managed)
	var newEntries []string
	for _, entry := range entries {
		id := entry.SessionID
		if len(id) > 7 {
			id = id[:7]
		}
		if id != "" && existingIDs[id] {
			continue // already present
		}
		newEntries = append(newEntries, formatMemoryEntry(entry))
	}

	// Merge: new entries first (newest), then existing entries
	existingEntries := extractEntryLines(managed)
	allEntries := append(newEntries, existingEntries...)

	// Trim to max
	if len(allEntries) > maxEntries {
		allEntries = allEntries[:maxEntries]
	}

	// Build output
	managedContent := buildManagedBlock(allEntries)
	content := assembleManagedFile(before, managedContent, after)

	// Atomic write
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	if err := atomicWriteFile(outputPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("write %s: %w", outputPath, err)
	}

	if !quiet {
		fmt.Printf("Synced %d session(s) to %s\n", len(allEntries), outputPath)
	}
	return nil
}

// readNLatestSessionEntries reads up to N most recent sessions sorted newest-first.
func readNLatestSessionEntries(cwd string, maxCount int) ([]*pendingEntry, error) {
	sessionsDir := filepath.Join(cwd, ".agents", "ao", "sessions")
	dirEntries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return nil, err
	}

	// Collect .jsonl filenames and sort descending (newest first)
	var jsonlFiles []string
	for _, e := range dirEntries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".jsonl" {
			continue
		}
		jsonlFiles = append(jsonlFiles, e.Name())
	}
	sort.Sort(sort.Reverse(sort.StringSlice(jsonlFiles)))

	// Cap at maxCount
	if len(jsonlFiles) > maxCount {
		jsonlFiles = jsonlFiles[:maxCount]
	}

	var results []*pendingEntry
	var skipped int
	for _, name := range jsonlFiles {
		entry, err := readSessionFile(filepath.Join(sessionsDir, name))
		if err != nil {
			skipped++
			continue
		}
		results = append(results, entry)
	}
	if skipped > 0 {
		VerbosePrintf("warning: skipped %d unreadable session file(s)\n", skipped)
	}
	return results, nil
}

// parseManagedBlock splits content into (before-markers, inside-markers, after-markers).
// If no markers exist, before = entire content, managed and after are empty.
func parseManagedBlock(content string) (before, managed, after string) {
	startIdx := strings.Index(content, memoryBlockStart)
	if startIdx == -1 {
		return content, "", ""
	}
	endIdx := strings.Index(content, memoryBlockEnd)
	if endIdx == -1 {
		return content, "", ""
	}
	if endIdx <= startIdx {
		return content, "", ""
	}
	// Ambiguous: multiple markers found — treat as no block to avoid data loss
	if strings.Count(content, memoryBlockStart) > 1 || strings.Count(content, memoryBlockEnd) > 1 {
		return content, "", ""
	}

	before = content[:startIdx]
	managed = content[startIdx+len(memoryBlockStart) : endIdx]
	after = content[endIdx+len(memoryBlockEnd):]
	return before, managed, after
}

// extractSessionIDs pulls session IDs from existing managed block content.
// Expects entries formatted as: - **[date]** (sessionID) summary...
func extractSessionIDs(managed string) map[string]bool {
	ids := make(map[string]bool)
	for _, line := range strings.Split(managed, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- **[") {
			continue
		}
		// Find (sessionID) pattern
		openParen := strings.Index(line, "(")
		closeParen := strings.Index(line, ")")
		if openParen != -1 && closeParen > openParen {
			id := line[openParen+1 : closeParen]
			if id != "" {
				ids[id] = true
			}
		}
	}
	return ids
}

// extractEntryLines pulls existing entry lines from managed block content.
func extractEntryLines(managed string) []string {
	var lines []string
	for _, line := range strings.Split(managed, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- **[") {
			lines = append(lines, line)
		}
	}
	return lines
}

// formatMemoryEntry creates a single-line markdown entry from session data.
func formatMemoryEntry(entry *pendingEntry) string {
	date := entry.QueuedAt.Format("2006-01-02")
	id := entry.SessionID
	if len(id) > 7 {
		id = id[:7]
	}

	summary := entry.Summary
	if summary == "" {
		summary = "Session recorded"
	}
	// Strip newlines from summary
	summary = strings.ReplaceAll(summary, "\n", " ")
	// Truncate long summaries (uses rune-safe truncateText from inject.go)
	summary = truncateText(summary, 200)

	return fmt.Sprintf("- **[%s]** (%s) %s", date, id, summary)
}

// buildManagedBlock wraps entry lines in managed markers.
func buildManagedBlock(entries []string) string {
	if len(entries) == 0 {
		return memoryBlockStart + "\n" + memoryBlockEnd
	}
	var b strings.Builder
	b.WriteString(memoryBlockStart)
	b.WriteString("\n")
	for _, e := range entries {
		b.WriteString(e)
		b.WriteString("\n")
	}
	b.WriteString(memoryBlockEnd)
	return b.String()
}

// assembleManagedFile combines before, managed block, and after into a complete file.
func assembleManagedFile(before, managed, after string) string {
	// If no existing content, create a minimal file with the block
	if before == "" && after == "" {
		return "# Memory\n\n" + managed + "\n"
	}

	// Ensure before ends with a newline before the block
	before = strings.TrimRight(before, "\n") + "\n\n"

	// Ensure after starts clean
	after = strings.TrimLeft(after, "\n")
	if after != "" {
		after = "\n" + after
	}

	return before + managed + after
}

// findGitRoot walks up from cwd to find .git directory.
func findGitRoot(cwd string) string {
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
