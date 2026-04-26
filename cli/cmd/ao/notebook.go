package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/notebook"
	"github.com/spf13/cobra"
)

var (
	notebookMemoryFile string
	notebookQuiet      bool
	notebookMaxLines   int
	notebookSource     string
	notebookSessionID  string
)

// notebookCmd is the parent for notebook-related subcommands.
var notebookCmd = &cobra.Command{
	Use:   "notebook",
	Short: "Manage the session notebook (MEMORY.md)",
}

// notebookUpdateCmd distills session insights into MEMORY.md.
var notebookUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update MEMORY.md with latest session insights",
	Long: `Reads the most recent session data and updates MEMORY.md with a "Last Session"
section containing summary, decisions, and next steps.

Source priority (--source auto): session artifacts (.agents/ao/sessions/*.jsonl)
first, then pending.jsonl as fallback. Use --source sessions or --source pending
to force a specific source.

Stays within the configured line budget (default 190) by pruning stale entries
from the longest sections first.`,
	RunE: runNotebookUpdate,
}

func init() {
	notebookCmd.GroupID = "config"
	rootCmd.AddCommand(notebookCmd)
	notebookUpdateCmd.Flags().StringVar(&notebookMemoryFile, "memory-file", "", "Path to MEMORY.md (auto-detected if omitted)")
	notebookUpdateCmd.Flags().BoolVar(&notebookQuiet, "quiet", false, "Suppress output (for hooks)")
	notebookUpdateCmd.Flags().IntVar(&notebookMaxLines, "max-lines", 190, "Maximum lines in MEMORY.md")
	notebookUpdateCmd.Flags().StringVar(&notebookSource, "source", "auto", "Source: auto|sessions|pending")
	notebookUpdateCmd.Flags().StringVar(&notebookSessionID, "session", "", "Specific session ID to update from")

	notebookCmd.AddCommand(notebookUpdateCmd)
}

// Type aliases preserve in-package identifiers for existing tests.
type pendingEntry = notebook.PendingEntry
type notebookSection = notebook.Section
type sessionEntry = notebook.SessionEntry

func runNotebookUpdate(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	memoryFile, ok := resolveNotebookMemoryFile(cwd)
	if !ok {
		return nil
	}

	entry := resolveNotebookEntry(cwd)
	if entry == nil {
		return nil
	}

	cursorPath := filepath.Join(cwd, ".agents", "ao", "notebook-cursor.json")
	if lastID, _ := readNotebookCursor(cursorPath); lastID == entry.SessionID && entry.SessionID != "" {
		VerbosePrintf("Session %s already processed — skipping.\n", entry.SessionID)
		return nil
	}

	sections, err := parseNotebookSections(memoryFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("parse MEMORY.md: %w", err)
	}

	lastSession := buildLastSessionSection(entry)
	sections = upsertLastSession(sections, lastSession)
	sections = pruneNotebook(sections, notebookMaxLines)

	content := renderNotebook(sections)
	if err := atomicWriteFile(memoryFile, []byte(content), 0600); err != nil {
		return fmt.Errorf("write MEMORY.md: %w", err)
	}

	if entry.SessionID != "" {
		_ = writeNotebookCursor(cursorPath, entry.SessionID)
	}

	if !notebookQuiet {
		lineCount := strings.Count(content, "\n")
		fmt.Printf("Updated %s (%d lines)\n", memoryFile, lineCount)
	}

	return nil
}

// resolveNotebookMemoryFile honors the --memory-file override or, when absent,
// searches upward for a MEMORY.md. Returns ok=false when no MEMORY.md can be
// located so the caller can short-circuit with the standard "skipping" message.
func resolveNotebookMemoryFile(cwd string) (string, bool) {
	if notebookMemoryFile != "" {
		return notebookMemoryFile, true
	}
	memoryFile, err := findMemoryFile(cwd)
	if err != nil {
		if !notebookQuiet {
			fmt.Println("No MEMORY.md found — skipping notebook update.")
		}
		return "", false
	}
	return memoryFile, true
}

// resolveNotebookEntry returns the pending session entry to render, honoring
// --session-id when set and falling back to the configured source. Returns nil
// when no entry is found so the caller can short-circuit; the caller does not
// need to distinguish "not found" from a read error here — both end the run.
func resolveNotebookEntry(cwd string) *pendingEntry {
	if notebookSessionID != "" {
		entry, err := readSessionByID(cwd, notebookSessionID)
		if err != nil || entry == nil {
			if !notebookQuiet {
				VerbosePrintf("Session %s not found.\n", notebookSessionID)
			}
			return nil
		}
		return entry
	}
	entry, err := resolveNotebookSource(cwd, notebookSource)
	if err != nil || entry == nil {
		if !notebookQuiet {
			VerbosePrintf("No session data — nothing to update.\n")
		}
		return nil
	}
	return entry
}

// Thin wrappers delegating to internal/notebook.

func findMemoryFile(cwd string) (string, error) { return notebook.FindMemoryFile(cwd) }

func readLatestPendingEntry(cwd string) (*pendingEntry, error) {
	return notebook.ReadLatestPendingEntry(cwd)
}

func resolveNotebookSource(cwd string, source string) (*pendingEntry, error) {
	return notebook.ResolveSource(cwd, source)
}

func readLatestSessionEntry(cwd string) (*pendingEntry, error) {
	return notebook.ReadLatestSessionEntry(cwd)
}

func readSessionByID(cwd string, id string) (*pendingEntry, error) {
	return notebook.ReadSessionByID(cwd, id)
}

func readSessionFile(path string) (*pendingEntry, error) {
	return notebook.ReadSessionFile(path)
}

func parseNotebookSections(path string) ([]notebookSection, error) {
	return notebook.ParseSections(path)
}

func parseSectionsFromString(content string) []notebookSection {
	return notebook.ParseSectionsFromString(content)
}

func categorizeKnowledge(items []string) (worked, next, other []string) {
	return notebook.CategorizeKnowledge(items)
}

func appendBulletSection(lines []string, heading string, items []string, maxLen int) []string {
	return notebook.AppendBulletSection(lines, heading, items, maxLen, truncateText)
}

func buildLastSessionSection(entry *pendingEntry) notebookSection {
	return notebook.BuildLastSessionSection(entry, truncateText)
}

func upsertLastSession(sections []notebookSection, lastSession notebookSection) []notebookSection {
	return notebook.UpsertLastSession(sections, lastSession)
}

func pruneNotebook(sections []notebookSection, maxLines int) []notebookSection {
	return notebook.Prune(sections, maxLines)
}

func totalLines(sections []notebookSection) int { return notebook.TotalLines(sections) }

func renderNotebook(sections []notebookSection) string { return notebook.Render(sections) }

func readNotebookCursor(path string) (string, error) { return notebook.ReadCursor(path) }

func writeNotebookCursor(path string, sessionID string) error {
	return notebook.WriteCursor(path, sessionID)
}
