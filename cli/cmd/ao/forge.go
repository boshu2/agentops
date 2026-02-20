package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/formatter"
	"github.com/boshu2/agentops/cli/internal/parser"
	"github.com/boshu2/agentops/cli/internal/search"
	"github.com/boshu2/agentops/cli/internal/storage"
	"github.com/boshu2/agentops/cli/internal/types"
)

var (
	forgeLastSession bool
	forgeQuiet       bool
	forgeQueue       bool
	forgeMdQuiet     bool
	forgeMdQueue     bool
)

const (
	// SnippetMaxLength is the maximum length for extracted text snippets.
	SnippetMaxLength = 200

	// SummaryMaxLength is the maximum length for session summaries.
	SummaryMaxLength = 100

	// CharsPerToken is the rough estimate of characters per token.
	// Used for approximate token counting from file size.
	CharsPerToken = 4
)

// issueIDPattern matches beads issue IDs like "ol-0001", "at-v123", "gt-abc-def".
var issueIDPattern = regexp.MustCompile(`\b([a-z]{2,3})-([a-z0-9]{3,7}(?:-[a-z0-9]+)?)\b`)

var forgeCmd = &cobra.Command{
	Use:   "forge",
	Short: "Extract knowledge from sources",
	Long: `The forge command extracts knowledge candidates from various sources.

Currently supported forges:
  transcript    Extract from Claude Code JSONL transcripts
  markdown      Extract from markdown files (.md)

Example:
  ao forge transcript ~/.claude/projects/**/*.jsonl
  ao forge markdown .agents/learnings/*.md`,
}

var forgeTranscriptCmd = &cobra.Command{
	Use:   "transcript <path-or-glob>",
	Short: "Extract knowledge from Claude Code transcripts",
	Long: `Parse Claude Code JSONL transcript files and extract knowledge candidates.

The transcript forge identifies:
  - Decisions: Architectural choices with rationale
  - Solutions: Working fixes for problems
  - Learnings: Insights gained from experience
  - Failures: What didn't work and why
  - References: Pointers to useful resources

Examples:
  ao forge transcript session.jsonl
  ao forge transcript ~/.claude/projects/**/*.jsonl
  ao forge transcript /path/to/*.jsonl --output candidates.json
  ao forge transcript --last-session              # Process most recent transcript
  ao forge transcript --last-session --quiet      # Silent mode for hooks`,
	Args: func(cmd *cobra.Command, args []string) error {
		lastSession, _ := cmd.Flags().GetBool("last-session")
		if !lastSession && len(args) < 1 {
			return fmt.Errorf("requires at least 1 arg(s), only received %d (or use --last-session)", len(args))
		}
		return nil
	},
	RunE: runForgeTranscript,
}

var forgeMarkdownCmd = &cobra.Command{
	Use:   "markdown <path-or-glob>",
	Short: "Extract knowledge from markdown files",
	Long: `Parse markdown (.md) files and extract knowledge candidates.

The markdown forge splits files by headings and runs the same extraction
patterns used for transcripts (decisions, solutions, learnings, failures,
references).

Examples:
  ao forge markdown .agents/learnings/*.md
  ao forge markdown docs/**/*.md
  ao forge markdown session-notes.md --quiet`,
	Args: cobra.MinimumNArgs(1),
	RunE: runForgeMarkdown,
}

func init() {
	rootCmd.AddCommand(forgeCmd)
	forgeCmd.AddCommand(forgeTranscriptCmd)
	forgeCmd.AddCommand(forgeMarkdownCmd)

	// Transcript flags
	forgeTranscriptCmd.Flags().BoolVar(&forgeLastSession, "last-session", false, "Process only the most recent transcript")
	forgeTranscriptCmd.Flags().BoolVar(&forgeQuiet, "quiet", false, "Suppress all output (for hooks)")
	forgeTranscriptCmd.Flags().BoolVar(&forgeQueue, "queue", false, "Queue session for learning extraction at next session start")

	// Markdown flags
	forgeMarkdownCmd.Flags().BoolVar(&forgeMdQuiet, "quiet", false, "Suppress all output (for hooks)")
	forgeMarkdownCmd.Flags().BoolVar(&forgeMdQueue, "queue", false, "Queue for learning extraction at next session start")
}

func resolveTranscriptFiles(args []string, quiet bool) ([]string, error) {
	if forgeLastSession {
		lastFile, err := findLastSession()
		if err != nil {
			if quiet {
				return nil, nil // Silent fail for hooks
			}
			return nil, fmt.Errorf("find last session: %w", err)
		}
		return []string{lastFile}, nil
	}

	return collectFilesFromPatterns(args, nil)
}

func resolveMarkdownFiles(args []string) ([]string, error) {
	return collectFilesFromPatterns(args, func(path string) bool {
		return filepath.Ext(path) == ".md"
	})
}

func collectFilesFromPatterns(patterns []string, matchFilter func(string) bool) ([]string, error) {
	var files []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
		}

		if len(matches) == 0 {
			// Treat as literal path.
			if _, err := os.Stat(pattern); err == nil {
				files = append(files, pattern)
			}
			continue
		}

		for _, match := range matches {
			if matchFilter == nil || matchFilter(match) {
				files = append(files, match)
			}
		}
	}

	return files, nil
}

func handleForgeDryRun(w io.Writer, quiet bool, files []string, noun string) bool {
	if !GetDryRun() || quiet {
		return false
	}

	fmt.Fprintf(w, "[dry-run] Would process %d %s\n", len(files), noun)
	for _, path := range files {
		fmt.Fprintf(w, "  - %s\n", path)
	}

	return true
}

func noFilesError(quiet bool, msg string) error {
	if quiet {
		return nil // Silent fail for hooks
	}
	return errors.New(msg)
}

func initForgeStorage() (cwd, baseDir string, fs *storage.FileStorage, err error) {
	cwd, err = os.Getwd()
	if err != nil {
		return "", "", nil, fmt.Errorf("get working directory: %w", err)
	}

	baseDir = filepath.Join(cwd, storage.DefaultBaseDir)
	fs = storage.NewFileStorage(
		storage.WithBaseDir(baseDir),
		storage.WithFormatters(
			formatter.NewMarkdownFormatter(),
			formatter.NewJSONLFormatter(),
		),
	)

	if err := fs.Init(); err != nil {
		return "", "", nil, fmt.Errorf("initialize storage: %w", err)
	}

	return cwd, baseDir, fs, nil
}

func forgeWarnf(quiet bool, format string, args ...any) {
	if quiet {
		return
	}
	fmt.Fprintf(os.Stderr, format, args...)
}

type forgeTotals struct {
	sessions  int
	decisions int
	knowledge int
}

func (t *forgeTotals) addSession(session *storage.Session) {
	t.sessions++
	t.decisions += len(session.Decisions)
	t.knowledge += len(session.Knowledge)
}

func writeSessionIndex(fs *storage.FileStorage, session *storage.Session, sessionPath string) error {
	indexEntry := &storage.IndexEntry{
		SessionID:   session.ID,
		Date:        session.Date,
		SessionPath: sessionPath,
		Summary:     session.Summary,
	}
	return fs.WriteIndex(indexEntry)
}

func writeSessionProvenance(fs *storage.FileStorage, sessionID, sessionPath, sourcePath, sourceType string, includeSessionID bool) error {
	provRecord := &storage.ProvenanceRecord{
		ID:           fmt.Sprintf("prov-%s", sessionID[:7]),
		ArtifactPath: sessionPath,
		ArtifactType: "session",
		SourcePath:   sourcePath,
		SourceType:   sourceType,
		CreatedAt:    time.Now(),
	}
	if includeSessionID {
		provRecord.SessionID = sessionID
	}
	return fs.WriteProvenance(provRecord)
}

func printForgeSummary(w io.Writer, totals forgeTotals, baseDir, noun string) {
	fmt.Fprintf(w, "\n✓ Processed %d %s\n", totals.sessions, noun)
	fmt.Fprintf(w, "  Decisions: %d\n", totals.decisions)
	fmt.Fprintf(w, "  Knowledge: %d\n", totals.knowledge)
	fmt.Fprintf(w, "  Output: %s\n", baseDir)
}

func runForgeTranscript(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()

	files, err := resolveTranscriptFiles(args, forgeQuiet)
	if err != nil {
		return err
	}

	if handleForgeDryRun(w, forgeQuiet, files, "file(s)") {
		return nil
	}

	if len(files) == 0 {
		return noFilesError(forgeQuiet, "no files found matching patterns")
	}

	if !forgeQuiet {
		VerbosePrintf("Processing %d transcript file(s)...\n", len(files))
	}

	cwd, baseDir, fs, err := initForgeStorage()
	if err != nil {
		return err
	}

	// Create parser with no truncation for full content extraction
	p := parser.NewParser()
	p.MaxContentLength = 0 // No truncation

	// Create extractor for knowledge identification
	extractor := parser.NewExtractor()

	// Process each file
	totals := forgeTotals{}

	for _, filePath := range files {
		session, err := processTranscript(filePath, p, extractor, forgeQuiet, w)
		if err != nil {
			forgeWarnf(forgeQuiet, "Warning: failed to process %s: %v\n", filePath, err)
			continue
		}

		// Write session
		sessionPath, err := fs.WriteSession(session)
		if err != nil {
			forgeWarnf(forgeQuiet, "Warning: failed to write session for %s: %v\n", filePath, err)
			continue
		}

		if err := writeSessionIndex(fs, session, sessionPath); err != nil {
			forgeWarnf(forgeQuiet, "Warning: failed to index session: %v\n", err)
		}

		if err := writeSessionProvenance(fs, session.ID, sessionPath, filePath, "transcript", true); err != nil {
			forgeWarnf(forgeQuiet, "Warning: failed to write provenance: %v\n", err)
		}

		// Update search index with the new session file
		updateSearchIndexForFile(baseDir, sessionPath, forgeQuiet)

		totals.addSession(session)

		if !forgeQuiet {
			VerbosePrintf("  ✓ %s → %s\n", filepath.Base(filePath), filepath.Base(sessionPath))
		}

		// Queue for extraction if requested
		if forgeQueue {
			if err := queueForExtraction(session, sessionPath, filePath, cwd); err != nil {
				forgeWarnf(forgeQuiet, "Warning: failed to queue for extraction: %v\n", err)
			}
		}
	}

	if !forgeQuiet {
		printForgeSummary(w, totals, baseDir, "session(s)")
	}

	return nil
}

// processTranscript parses a transcript and extracts session data.
func processTranscript(filePath string, p *parser.Parser, extractor *parser.Extractor, quiet bool, w io.Writer) (session *storage.Session, err error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	fileSize := info.Size()
	totalLines := countLines(filePath)

	if _, err := f.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("seek file: %w", err)
	}

	msgCh, errCh := p.ParseChannel(f)
	session = initSession(filePath)
	state := &transcriptState{
		seenFiles:  make(map[string]bool),
		seenIssues: make(map[string]bool),
	}

	lineCount := 0
	lastProgress := 0

	for msg := range msgCh {
		lineCount++

		// Progress output every 1000 lines
		if !quiet && lineCount-lastProgress >= 1000 {
			pct := 0
			if totalLines > 0 {
				pct = lineCount * 100 / totalLines
			}
			fmt.Fprintf(w, "\r[forge] Processing... %d/%d (%d%%)  ", lineCount, totalLines, pct)
			lastProgress = lineCount
		}

		updateSessionMeta(session, msg)
		extractMessageKnowledge(msg, extractor, state)
		extractMessageRefs(msg, session, state)
	}

	if !quiet {
		fmt.Fprintf(w, "\r%s\r", "                                                    ")
	}

	select {
	case err := <-errCh:
		if err != nil {
			return nil, err
		}
	default:
	}

	session.Summary = generateSummary(state.decisions, state.knowledge, session.Date)
	session.Decisions = dedup(state.decisions)
	session.Knowledge = dedup(state.knowledge)
	session.FilesChanged = state.filesChanged
	session.Issues = state.issues
	session.Tokens = storage.TokenUsage{
		Total:     int(fileSize / CharsPerToken),
		Estimated: true,
	}

	return session, nil
}

// transcriptState holds accumulated state during transcript processing.
type transcriptState struct {
	decisions    []string
	knowledge    []string
	filesChanged []string
	issues       []string
	seenFiles    map[string]bool
	seenIssues   map[string]bool
}

// initSession creates a new session with default values.
func initSession(filePath string) *storage.Session {
	return &storage.Session{
		TranscriptPath: filePath,
		ToolCalls:      make(map[string]int),
	}
}

// updateSessionMeta updates session ID and date from a message.
func updateSessionMeta(session *storage.Session, msg types.TranscriptMessage) {
	if session.ID == "" && msg.SessionID != "" {
		session.ID = msg.SessionID
	}
	if session.Date.IsZero() || (!msg.Timestamp.IsZero() && msg.Timestamp.Before(session.Date)) {
		session.Date = msg.Timestamp
	}
}

// extractMessageKnowledge extracts decisions and knowledge from message content.
func extractMessageKnowledge(msg types.TranscriptMessage, extractor *parser.Extractor, state *transcriptState) {
	if msg.Content == "" {
		return
	}
	results := extractor.Extract(msg)
	for _, result := range results {
		text := extractSnippet(msg.Content, result.StartIndex, SnippetMaxLength)
		switch result.Type {
		case types.KnowledgeTypeDecision:
			state.decisions = append(state.decisions, text)
		case types.KnowledgeTypeSolution, types.KnowledgeTypeLearning:
			state.knowledge = append(state.knowledge, text)
		}
	}
}

// extractMessageRefs extracts file paths and issue IDs from a message.
func extractMessageRefs(msg types.TranscriptMessage, session *storage.Session, state *transcriptState) {
	extractToolRefs(msg.Tools, session, state)
	extractIssueRefs(msg.Content, state)
}

// extractToolRefs extracts tool calls and file paths from tool invocations.
func extractToolRefs(tools []types.ToolCall, session *storage.Session, state *transcriptState) {
	for _, tool := range tools {
		if tool.Name != "" && tool.Name != "tool_result" {
			session.ToolCalls[tool.Name]++
		}
		extractFilePathsFromTool(tool, state)
	}
}

// extractFilePathsFromTool extracts file paths from a tool's input parameters.
func extractFilePathsFromTool(tool types.ToolCall, state *transcriptState) {
	if tool.Input == nil {
		return
	}
	if fp, ok := tool.Input["file_path"].(string); ok && !state.seenFiles[fp] {
		state.filesChanged = append(state.filesChanged, fp)
		state.seenFiles[fp] = true
	}
	if fp, ok := tool.Input["path"].(string); ok && !state.seenFiles[fp] {
		state.filesChanged = append(state.filesChanged, fp)
		state.seenFiles[fp] = true
	}
}

// extractIssueRefs extracts issue IDs from message content.
func extractIssueRefs(content string, state *transcriptState) {
	ids := extractIssueIDs(content)
	for _, id := range ids {
		if !state.seenIssues[id] {
			state.issues = append(state.issues, id)
			state.seenIssues[id] = true
		}
	}
}

// generateSummary creates a session summary from extracted content.
func generateSummary(decisions, knowledge []string, date time.Time) string {
	if len(decisions) > 0 {
		return truncateString(decisions[0], SummaryMaxLength)
	}
	if len(knowledge) > 0 {
		return truncateString(knowledge[0], SummaryMaxLength)
	}
	return fmt.Sprintf("Session from %s", date.Format("2006-01-02"))
}

// countLines quickly counts lines in a file.
func countLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer func() {
		_ = f.Close() //nolint:errcheck // read-only line count, close error non-fatal
	}()

	buf := make([]byte, 64*1024)
	count := 0

	for {
		n, err := f.Read(buf)
		if n > 0 {
			for _, b := range buf[:n] {
				if b == '\n' {
					count++
				}
			}
		}
		if err != nil {
			break
		}
	}

	return count
}

// extractSnippet extracts a text snippet around a match.
func extractSnippet(content string, startIdx, maxLen int) string {
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx >= len(content) {
		return ""
	}

	end := startIdx + maxLen
	if end > len(content) {
		end = len(content)
	}

	snippet := content[startIdx:end]

	// Trim to word boundary
	if end < len(content) {
		if idx := lastSpaceIndex(snippet); idx > maxLen/2 {
			snippet = snippet[:idx]
		}
		snippet += "..."
	}

	return snippet
}

func lastSpaceIndex(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ' ' {
			return i
		}
	}
	return -1
}

// extractIssueIDs finds issue IDs like "ol-0001", "at-v123" in content.
func extractIssueIDs(content string) []string {
	matches := issueIDPattern.FindAllString(content, -1)
	if len(matches) == 0 {
		return nil
	}
	return matches
}

// truncateString limits a string to maxLen characters.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// dedup removes duplicates from a string slice.
func dedup(items []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

// queueForExtraction adds a session to the pending extraction queue.
func queueForExtraction(session *storage.Session, sessionPath, transcriptPath, cwd string) error {
	pendingDir := filepath.Join(cwd, storage.DefaultBaseDir)
	if err := os.MkdirAll(pendingDir, 0755); err != nil {
		return fmt.Errorf("create pending dir: %w", err)
	}

	pendingPath := filepath.Join(pendingDir, "pending.jsonl")

	// Create pending extraction record
	pending := struct {
		SessionID      string    `json:"session_id"`
		SessionPath    string    `json:"session_path"`
		TranscriptPath string    `json:"transcript_path"`
		Summary        string    `json:"summary"`
		Decisions      []string  `json:"decisions,omitempty"`
		Knowledge      []string  `json:"knowledge,omitempty"`
		QueuedAt       time.Time `json:"queued_at"`
	}{
		SessionID:      session.ID,
		SessionPath:    sessionPath,
		TranscriptPath: transcriptPath,
		Summary:        session.Summary,
		Decisions:      session.Decisions,
		Knowledge:      session.Knowledge,
		QueuedAt:       time.Now(),
	}

	data, err := json.Marshal(pending)
	if err != nil {
		return fmt.Errorf("marshal pending: %w", err)
	}

	f, err := os.OpenFile(pendingPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open pending file: %w", err)
	}
	defer func() {
		_ = f.Close() //nolint:errcheck // write complete, close best-effort
	}()

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write pending: %w", err)
	}

	return nil
}

func runForgeMarkdown(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()

	files, err := resolveMarkdownFiles(args)
	if err != nil {
		return err
	}

	if handleForgeDryRun(w, forgeMdQuiet, files, "markdown file(s)") {
		return nil
	}

	if len(files) == 0 {
		return noFilesError(forgeMdQuiet, "no markdown files found matching patterns")
	}

	if !forgeMdQuiet {
		VerbosePrintf("Processing %d markdown file(s)...\n", len(files))
	}

	cwd, baseDir, fs, err := initForgeStorage()
	if err != nil {
		return err
	}

	extractor := parser.NewExtractor()

	totals := forgeTotals{}

	for _, filePath := range files {
		session, err := processMarkdown(filePath, extractor, forgeMdQuiet)
		if err != nil {
			forgeWarnf(forgeMdQuiet, "Warning: failed to process %s: %v\n", filePath, err)
			continue
		}

		sessionPath, err := fs.WriteSession(session)
		if err != nil {
			forgeWarnf(forgeMdQuiet, "Warning: failed to write session for %s: %v\n", filePath, err)
			continue
		}

		if err := writeSessionIndex(fs, session, sessionPath); err != nil {
			forgeWarnf(forgeMdQuiet, "Warning: failed to index session: %v\n", err)
		}

		if err := writeSessionProvenance(fs, session.ID, sessionPath, filePath, "markdown", false); err != nil {
			forgeWarnf(forgeMdQuiet, "Warning: failed to write provenance: %v\n", err)
		}

		// Update search index with the new session file
		updateSearchIndexForFile(baseDir, sessionPath, forgeMdQuiet)

		totals.addSession(session)

		if !forgeMdQuiet {
			VerbosePrintf("  ✓ %s → %s\n", filepath.Base(filePath), filepath.Base(sessionPath))
		}

		if forgeMdQueue {
			if err := queueForExtraction(session, sessionPath, filePath, cwd); err != nil {
				forgeWarnf(forgeMdQuiet, "Warning: failed to queue for extraction: %v\n", err)
			}
		}
	}

	if !forgeMdQuiet {
		printForgeSummary(w, totals, baseDir, "markdown file(s)")
	}

	return nil
}

// processMarkdown parses a markdown file and extracts session data.
func processMarkdown(filePath string, extractor *parser.Extractor, quiet bool) (*storage.Session, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	content := string(data)
	if len(content) == 0 {
		return nil, fmt.Errorf("empty file")
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}

	// Generate a deterministic session ID from file path
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(filePath)))
	sessionID := fmt.Sprintf("md-%s", hash[:12])

	session := &storage.Session{
		ID:             sessionID,
		Date:           info.ModTime(),
		TranscriptPath: filePath,
		ToolCalls:      make(map[string]int),
		Tokens: storage.TokenUsage{
			Total:     len(content) / CharsPerToken,
			Estimated: true,
		},
	}

	// Split by headings (## or #) into sections
	sections := splitMarkdownSections(content)

	state := &transcriptState{
		seenFiles:  make(map[string]bool),
		seenIssues: make(map[string]bool),
	}

	for i, section := range sections {
		if len(section) == 0 {
			continue
		}

		// Create a synthetic message for the extractor
		msg := types.TranscriptMessage{
			Content:      section,
			Role:         "assistant",
			SessionID:    sessionID,
			Timestamp:    info.ModTime(),
			MessageIndex: i,
		}

		extractMessageKnowledge(msg, extractor, state)
		extractIssueRefs(section, state)
	}

	session.Summary = generateSummary(state.decisions, state.knowledge, session.Date)
	session.Decisions = dedup(state.decisions)
	session.Knowledge = dedup(state.knowledge)
	session.Issues = state.issues

	return session, nil
}

// splitMarkdownSections splits markdown content by heading boundaries.
// Returns sections including their heading line.
func splitMarkdownSections(content string) []string {
	lines := strings.Split(content, "\n")
	var sections []string
	var current []string

	for _, line := range lines {
		if (strings.HasPrefix(line, "# ") || strings.HasPrefix(line, "## ")) && len(current) > 0 {
			sections = append(sections, strings.Join(current, "\n"))
			current = nil
		}
		current = append(current, line)
	}
	if len(current) > 0 {
		sections = append(sections, strings.Join(current, "\n"))
	}

	// If no headings found, treat entire content as one section
	if len(sections) == 0 {
		sections = []string{content}
	}

	return sections
}

// updateSearchIndexForFile loads the search index (if it exists), updates the
// entry for the given file path, and saves it back. If no index exists yet
// this is a no-op -- the user can create one with `ao search --rebuild-index`.
func updateSearchIndexForFile(baseDir, filePath string, quiet bool) {
	idxPath := filepath.Join(baseDir, "index.jsonl")
	if _, err := os.Stat(idxPath); os.IsNotExist(err) {
		return // no index yet -- nothing to update
	}

	idx, err := search.LoadIndex(idxPath)
	if err != nil {
		if !quiet {
			fmt.Fprintf(os.Stderr, "Warning: failed to load search index: %v\n", err)
		}
		return
	}

	if err := search.UpdateIndex(idx, filePath); err != nil {
		if !quiet {
			fmt.Fprintf(os.Stderr, "Warning: failed to update search index for %s: %v\n", filePath, err)
		}
		return
	}

	if err := search.SaveIndex(idx, idxPath); err != nil {
		if !quiet {
			fmt.Fprintf(os.Stderr, "Warning: failed to save search index: %v\n", err)
		}
	}
}

// findLastSession finds the most recently modified transcript file.
func findLastSession() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}

	projectsDir := filepath.Join(homeDir, ".claude", "projects")
	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		return "", fmt.Errorf("no Claude projects directory found at %s", projectsDir)
	}

	// Find all main session transcripts (exclude subagents)
	type fileWithTime struct {
		path    string
		modTime time.Time
	}
	var candidates []fileWithTime

	err = filepath.Walk(projectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip directories named "subagents"
		if info.IsDir() && info.Name() == "subagents" {
			return filepath.SkipDir
		}

		// Only consider .jsonl files at depth 2 (project/session.jsonl)
		if !info.IsDir() && filepath.Ext(path) == ".jsonl" {
			rel, _ := filepath.Rel(projectsDir, path)
			depth := len(filepath.SplitList(rel))
			// Accept files directly in project dirs (depth ~2)
			if depth <= 3 && info.Size() > 100 { // Skip tiny files
				candidates = append(candidates, fileWithTime{
					path:    path,
					modTime: info.ModTime(),
				})
			}
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk projects directory: %w", err)
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no transcript files found in %s", projectsDir)
	}

	// Sort by modification time, most recent first
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].modTime.After(candidates[j].modTime)
	})

	return candidates[0].path, nil
}
