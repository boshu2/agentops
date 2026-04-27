package main

import (
	"fmt"
	"io"
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

type notebookUpdateOptions struct {
	Cwd        string
	MemoryFile string
	Quiet      bool
	MaxLines   int
	Source     string
	SessionID  string
	Writer     io.Writer
}

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
	writer := io.Writer(os.Stdout)
	if cmd != nil {
		writer = cmd.OutOrStdout()
	}

	return runNotebookUpdateWithOptions(notebookUpdateOptions{
		Cwd:        cwd,
		MemoryFile: notebookMemoryFile,
		Quiet:      notebookQuiet,
		MaxLines:   notebookMaxLines,
		Source:     notebookSource,
		SessionID:  notebookSessionID,
		Writer:     writer,
	})
}

func runNotebookUpdateWithOptions(opts notebookUpdateOptions) error {
	opts, err := normalizeNotebookUpdateOptions(opts)
	if err != nil {
		return err
	}

	memoryFile, ok := resolveNotebookMemoryFile(opts)
	if !ok {
		return nil
	}

	entry, ok := resolveNotebookUpdateEntry(opts)
	if !ok {
		return nil
	}

	if notebookEntryAlreadyProcessed(opts, entry) {
		return nil
	}

	return writeNotebookUpdate(opts, memoryFile, entry)
}

func normalizeNotebookUpdateOptions(opts notebookUpdateOptions) (notebookUpdateOptions, error) {
	if opts.Cwd == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return opts, fmt.Errorf("get working directory: %w", err)
		}
		opts.Cwd = cwd
	}
	if opts.Writer == nil {
		opts.Writer = os.Stdout
	}
	if opts.MaxLines <= 0 {
		opts.MaxLines = 190
	}
	if opts.Source == "" {
		opts.Source = "auto"
	}
	return opts, nil
}

func resolveNotebookMemoryFile(opts notebookUpdateOptions) (string, bool) {
	if opts.MemoryFile != "" {
		return opts.MemoryFile, true
	}

	memoryFile, err := findMemoryFile(opts.Cwd)
	if err != nil {
		if !opts.Quiet {
			fmt.Fprintln(opts.Writer, "No MEMORY.md found — skipping notebook update.")
		}
		return "", false
	}
	return memoryFile, true
}

func resolveNotebookUpdateEntry(opts notebookUpdateOptions) (*pendingEntry, bool) {
	if opts.SessionID != "" {
		entry, err := readSessionByID(opts.Cwd, opts.SessionID)
		if err != nil || entry == nil {
			if !opts.Quiet {
				notebookVerboseFprintf(opts.Writer, "Session %s not found.\n", opts.SessionID)
			}
			return nil, false
		}
		return entry, true
	}

	entry, err := resolveNotebookSource(opts.Cwd, opts.Source)
	if err != nil || entry == nil {
		if !opts.Quiet {
			notebookVerboseFprintf(opts.Writer, "No session data — nothing to update.\n")
		}
		return nil, false
	}
	return entry, true
}

func notebookEntryAlreadyProcessed(opts notebookUpdateOptions, entry *pendingEntry) bool {
	cursorPath := filepath.Join(opts.Cwd, ".agents", "ao", "notebook-cursor.json")
	if lastID, _ := readNotebookCursor(cursorPath); lastID == entry.SessionID && entry.SessionID != "" {
		notebookVerboseFprintf(opts.Writer, "Session %s already processed — skipping.\n", entry.SessionID)
		return true
	}
	return false
}

func writeNotebookUpdate(opts notebookUpdateOptions, memoryFile string, entry *pendingEntry) error {
	sections, err := parseNotebookSections(memoryFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("parse MEMORY.md: %w", err)
	}

	lastSession := buildLastSessionSection(entry)
	sections = upsertLastSession(sections, lastSession)
	sections = pruneNotebook(sections, opts.MaxLines)

	content := renderNotebook(sections)
	if err := atomicWriteFile(memoryFile, []byte(content), 0600); err != nil {
		return fmt.Errorf("write MEMORY.md: %w", err)
	}

	if entry.SessionID != "" {
		cursorPath := filepath.Join(opts.Cwd, ".agents", "ao", "notebook-cursor.json")
		_ = writeNotebookCursor(cursorPath, entry.SessionID)
	}

	if !opts.Quiet {
		lineCount := strings.Count(content, "\n")
		fmt.Fprintf(opts.Writer, "Updated %s (%d lines)\n", memoryFile, lineCount)
	}

	return nil
}

func notebookVerboseFprintf(w io.Writer, format string, args ...any) {
	if verbose {
		fmt.Fprintf(w, format, args...)
	}
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
