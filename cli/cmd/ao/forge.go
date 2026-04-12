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
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/config"
	"github.com/boshu2/agentops/cli/internal/forge"
	"github.com/boshu2/agentops/cli/internal/formatter"
	"github.com/boshu2/agentops/cli/internal/llm"
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
	forgeTier        int
	forgeTier1Model  string
	forgeLLMEndpoint string
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

var (
	sessionUUIDPattern             = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	sessionClaudeTranscriptPattern = regexp.MustCompile(`(ses_[A-Za-z0-9]+)`)
	errTranscriptHasNoChatMessages = errors.New("transcript has no chat messages")
)

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

var (
	forgeReviewDryRun bool
)

var forgeReviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Tier 2 structural review of draft session pages",
	Long: `Review draft session pages in .agents/ao/sessions/ and promote
qualifying pages to status:reviewed. Uses structural quality checks
(section presence, confidence threshold) in v1; LLM-based review
is planned for v2.

Pass --eval to dry-run the same structural gate against a labeled JSON
manifest without mutating session pages. Eval case expected values are
"promote" or "skip".

Pass --reviewer-model to add a configured local LLM reviewer after the
structural gate. The reviewer uses the same Ollama readiness checks as Tier 1.

Examples:
  ao forge review
  ao forge review --dry-run
  ao forge review --reviewer-model gemma2:9b
  ao forge review --eval .agents/rpi/forge-review-eval.json --json`,
	RunE: runForgeReview,
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
	forgeCmd.GroupID = "knowledge"
	rootCmd.AddCommand(forgeCmd)
	forgeCmd.AddCommand(forgeTranscriptCmd)
	forgeCmd.AddCommand(forgeMarkdownCmd)
	forgeCmd.AddCommand(forgeReviewCmd)
	forgeReviewCmd.Flags().BoolVar(&forgeReviewDryRun, "dry-run", false, "Show what would be promoted without writing")
	forgeReviewCmd.Flags().String("eval", "", "Evaluate review decisions against a labeled JSON manifest without writing")
	forgeReviewCmd.Flags().String("reviewer-endpoint", "", "Ollama HTTP endpoint for --reviewer-model (default: $AGENTOPS_LLM_ENDPOINT or http://localhost:11434)")
	forgeReviewCmd.Flags().String("reviewer-model", "", "LLM model tag for Tier 2 reviewer decisions (e.g. gemma2:9b)")
	forgeReviewCmd.Flags().String("sessions-dir", "", "Directory containing session pages (default: .agents/ao/sessions)")

	// Transcript flags
	forgeTranscriptCmd.Flags().BoolVar(&forgeLastSession, "last-session", false, "Process only the most recent transcript")
	forgeTranscriptCmd.Flags().BoolVar(&forgeQuiet, "quiet", false, "Suppress all output (for hooks)")
	forgeTranscriptCmd.Flags().BoolVar(&forgeQueue, "queue", false, "Queue session for learning extraction at next session start")
	forgeTranscriptCmd.Flags().IntVar(&forgeTier, "tier", 0, "Tier 1 transcript processing: enqueue to configured Dream worker, otherwise use local LLM with --model")
	forgeTranscriptCmd.Flags().StringVar(&forgeTier1Model, "model", "", "LLM model tag for --tier=1 (e.g. gemma2:9b)")
	forgeTranscriptCmd.Flags().StringVar(&forgeLLMEndpoint, "llm-endpoint", "", "Ollama HTTP endpoint for --tier=1 (default: $AGENTOPS_LLM_ENDPOINT or http://localhost:11434)")

	// Markdown flags
	forgeMarkdownCmd.Flags().BoolVar(&forgeMdQuiet, "quiet", false, "Suppress all output (for hooks)")
	forgeMarkdownCmd.Flags().BoolVar(&forgeMdQueue, "queue", false, "Queue for learning extraction at next session start")
}

func runForgeReview(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	sessionsDir, err := resolveForgeReviewSessionsDir(cmd, cwd)
	if err != nil {
		return err
	}
	reviewer, err := resolveForgeReviewReviewer(cmd)
	if err != nil {
		return err
	}
	evalPath, err := cmd.Flags().GetString("eval")
	if err != nil {
		return err
	}
	if strings.TrimSpace(evalPath) != "" {
		report, err := llm.EvaluateReviewDraftSessions(llm.ReviewEvalOptions{
			SessionsDir:  sessionsDir,
			ManifestPath: evalPath,
			Reviewer:     reviewer,
		})
		if err != nil {
			return err
		}
		return writeForgeReviewEvalReport(cmd.OutOrStdout(), report)
	}

	result, err := llm.ReviewDraftSessions(llm.ReviewOptions{
		SessionsDir: sessionsDir,
		DryRun:      forgeReviewDryRun,
		Quiet:       forgeQuiet,
		Reviewer:    reviewer,
	})
	if err != nil {
		return err
	}
	if !forgeQuiet {
		return writeForgeReviewResult(cmd.OutOrStdout(), result)
	}
	return nil
}

func resolveForgeReviewSessionsDir(cmd *cobra.Command, cwd string) (string, error) {
	sessionsDir, err := cmd.Flags().GetString("sessions-dir")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(sessionsDir) == "" {
		sessionsDir = filepath.Join(cwd, ".agents", "ao", "sessions")
	}
	if filepath.IsAbs(sessionsDir) {
		return filepath.Clean(sessionsDir), nil
	}
	return filepath.Clean(filepath.Join(cwd, sessionsDir)), nil
}

func resolveForgeReviewReviewer(cmd *cobra.Command) (llm.PageReviewer, error) {
	model, err := cmd.Flags().GetString("reviewer-model")
	if err != nil {
		return nil, err
	}
	endpoint, err := cmd.Flags().GetString("reviewer-endpoint")
	if err != nil {
		return nil, err
	}
	model = strings.TrimSpace(model)
	endpoint = strings.TrimSpace(endpoint)
	if model == "" {
		if endpoint != "" {
			return nil, fmt.Errorf("--reviewer-endpoint requires --reviewer-model")
		}
		return nil, nil
	}

	gen, err := llm.NewOllamaClient(llm.OllamaOptions{
		Endpoint: endpoint,
		Model:    model,
	})
	if err != nil {
		return nil, fmt.Errorf("build Tier 2 reviewer: %w", err)
	}
	return llm.NewGeneratorReviewer(gen), nil
}

type forgeReviewJSONResult struct {
	Reviewed      int      `json:"reviewed"`
	Skipped       int      `json:"skipped"`
	Errors        int      `json:"errors"`
	ErrorMessages []string `json:"error_messages,omitempty"`
}

func writeForgeReviewResult(w io.Writer, result *llm.ReviewResult) error {
	if GetOutput() == "json" {
		out := forgeReviewJSONResult{
			Reviewed: result.Reviewed,
			Skipped:  result.Skipped,
			Errors:   len(result.Errors),
		}
		for _, err := range result.Errors {
			out.ErrorMessages = append(out.ErrorMessages, err.Error())
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	fmt.Fprintf(w, "Reviewed: %d, Skipped: %d, Errors: %d\n",
		result.Reviewed, result.Skipped, len(result.Errors))
	return nil
}

func writeForgeReviewEvalReport(w io.Writer, report *llm.ReviewEvalReport) error {
	if GetOutput() == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	fmt.Fprintln(w, "AO Forge Review Eval")
	fmt.Fprintln(w, "====================")
	fmt.Fprintf(w, "Eval set:     %s\n", report.ID)
	fmt.Fprintf(w, "Manifest:     %s\n", report.ManifestPath)
	fmt.Fprintf(w, "Sessions dir: %s\n", report.SessionsDir)
	fmt.Fprintf(w, "Cases:        %d\n", report.Cases)
	fmt.Fprintf(w, "Passed:       %d\n", report.Passed)
	fmt.Fprintf(w, "Failed:       %d\n", report.Failed)
	if report.Errors > 0 {
		fmt.Fprintf(w, "Errors:       %d\n", report.Errors)
	}
	fmt.Fprintf(w, "Accuracy:     %.0f%%\n", report.Accuracy*100)
	for _, result := range report.Results {
		status := "PASS"
		if !result.Passed {
			status = "FAIL"
		}
		fmt.Fprintf(w, "  %-5s %s expected=%s actual=%s path=%s\n",
			result.ID, status, result.Expected, result.Actual, result.Path)
		if result.ErrorMessage != "" {
			fmt.Fprintf(w, "        error=%s\n", result.ErrorMessage)
		}
	}
	return nil
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
	return forge.CollectFilesFromPatterns(patterns, matchFilter)
}

func handleForgeDryRun(w io.Writer, quiet bool, files []string, noun string) bool {
	if !GetDryRun() || quiet {
		return false
	}

	_, _ = fmt.Fprintf(w, "[dry-run] Would process %d %s\n", len(files), noun)
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
		ID:            fmt.Sprintf("prov-%s", sessionID[:7]),
		ArtifactPath:  sessionPath,
		WorkspacePath: workspacePathFromAgentArtifactPath(sessionPath),
		ArtifactType:  "session",
		SourcePath:    sourcePath,
		SourceType:    sourceType,
		CreatedAt:     time.Now(),
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

	if forgeTier == 1 {
		return runForgeTier1(w, files)
	}

	if !forgeQuiet {
		VerbosePrintf("Processing %d transcript file(s)...\n", len(files))
	}

	cwd, baseDir, fs, err := initForgeStorage()
	if err != nil {
		return err
	}

	p := parser.NewParser()
	p.MaxContentLength = 0

	extractor := parser.NewExtractor()

	totals := forgeTotals{}

	for _, filePath := range files {
		session, err := processTranscript(filePath, p, extractor, forgeQuiet, w)
		if err != nil {
			if errors.Is(err, errTranscriptHasNoChatMessages) {
				VerbosePrintf("  - skipped %s (no chat messages)\n", filepath.Base(filePath))
				continue
			}
			forgeWarnf(forgeQuiet, "Warning: failed to process %s: %v\n", filePath, err)
			continue
		}

		forgeTranscriptFile(fs, session, filePath, baseDir, cwd, &totals)
	}

	if !forgeQuiet {
		printForgeSummary(w, totals, baseDir, "session(s)")
	}

	return nil
}

func forgeTranscriptFile(fs *storage.FileStorage, session *storage.Session, filePath, baseDir, cwd string, totals *forgeTotals) {
	sessionPath, err := fs.WriteSession(session)
	if err != nil {
		forgeWarnf(forgeQuiet, "Warning: failed to write session for %s: %v\n", filePath, err)
		return
	}

	if err := writeSessionIndex(fs, session, sessionPath); err != nil {
		forgeWarnf(forgeQuiet, "Warning: failed to index session: %v\n", err)
	}

	if err := writeSessionProvenance(fs, session.ID, sessionPath, filePath, "transcript", true); err != nil {
		forgeWarnf(forgeQuiet, "Warning: failed to write provenance: %v\n", err)
	}

	updateSearchIndexForFile(baseDir, sessionPath, forgeQuiet)
	totals.addSession(session)

	if !forgeQuiet {
		VerbosePrintf("  ✓ %s → %s\n", filepath.Base(filePath), filepath.Base(sessionPath))
	}

	if forgeQueue {
		if err := queueForExtraction(session, sessionPath, filePath, cwd); err != nil {
			forgeWarnf(forgeQuiet, "Warning: failed to queue for extraction: %v\n", err)
		}
	}

	// Auto-write pending learnings for close-loop ingestion (bridges forge→pool)
	if n, err := writePendingLearnings(session, cwd); err != nil {
		forgeWarnf(forgeQuiet, "Warning: failed to write pending learnings: %v\n", err)
	} else if n > 0 && !forgeQuiet {
		VerbosePrintf("  → %d pending learning(s) written\n", n)
	}
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
	state := forge.NewTranscriptState()

	consumeTranscriptMessages(msgCh, session, extractor, state, quiet, w, totalLines)

	if err := drainParseErrors(errCh); err != nil {
		return nil, err
	}

	if session.Date.IsZero() {
		session.Date = info.ModTime().UTC()
	}
	finalizeTranscriptSession(session, state, fileSize)
	if state.ChatMessages == 0 {
		return nil, errTranscriptHasNoChatMessages
	}

	return session, nil
}

func consumeTranscriptMessages(msgCh <-chan types.TranscriptMessage, session *storage.Session, extractor *parser.Extractor, state *transcriptState, quiet bool, w io.Writer, totalLines int) {
	lineCount := 0
	lastProgress := 0

	for msg := range msgCh {
		lineCount++
		reportProgress(quiet, w, lineCount, totalLines, &lastProgress)
		updateSessionMeta(session, msg)
		if forge.IsConversationMessage(msg.Type, msg.Role) {
			state.ChatMessages++
		}
		extractMessageKnowledge(msg, extractor, state)
		extractMessageRefs(msg, session, state)
	}

	if !quiet {
		fmt.Fprintf(w, "\r%s\r", "                                                    ")
	}
}

func reportProgress(quiet bool, w io.Writer, lineCount, totalLines int, lastProgress *int) {
	if quiet || lineCount-*lastProgress < 1000 {
		return
	}
	pct := 0
	if totalLines > 0 {
		pct = lineCount * 100 / totalLines
	}
	fmt.Fprintf(w, "\r[forge] Processing... %d/%d (%d%%)  ", lineCount, totalLines, pct)
	*lastProgress = lineCount
}

func drainParseErrors(errCh <-chan error) error {
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

func finalizeTranscriptSession(session *storage.Session, state *transcriptState, fileSize int64) {
	forge.FinalizeTranscriptSession(
		&session.Summary,
		&session.Decisions,
		&session.Knowledge,
		&session.FilesChanged,
		&session.Issues,
		&session.Tokens.Total,
		&session.Tokens.Estimated,
		&session.SessionType,
		state,
		session.Date,
		fileSize,
	)
}

// detectSessionTypeFromContent infers session type from forged content.
func detectSessionTypeFromContent(summary string, knowledge, decisions []string) string {
	return forge.DetectSessionTypeFromContent(summary, knowledge, decisions)
}

// transcriptState is a local alias for the extracted forge.TranscriptState.
type transcriptState = forge.TranscriptState

// initSession creates a new session with default values.
func initSession(filePath string) *storage.Session {
	return &storage.Session{
		ID:             inferSessionIDFromPath(filePath),
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
			state.Decisions = append(state.Decisions, text)
		case types.KnowledgeTypeSolution, types.KnowledgeTypeLearning,
			types.KnowledgeTypeFailure, types.KnowledgeTypeReference:
			state.Knowledge = append(state.Knowledge, text)
		}
	}
}

// extractMessageRefs extracts file paths and issue IDs from a message.
func extractMessageRefs(msg types.TranscriptMessage, session *storage.Session, state *transcriptState) {
	for _, tool := range msg.Tools {
		if tool.Name != "" && tool.Name != "tool_result" {
			session.ToolCalls[tool.Name]++
		}
		forge.ExtractFilePathsFromTool(tool.Input, state)
	}
	forge.ExtractIssueRefs(msg.Content, state)
}

func extractToolRefs(tools []types.ToolCall, session *storage.Session, state *transcriptState) {
	for _, tool := range tools {
		if tool.Name != "" && tool.Name != "tool_result" {
			session.ToolCalls[tool.Name]++
		}
		forge.ExtractFilePathsFromTool(tool.Input, state)
	}
}

func extractFilePathsFromTool(tool types.ToolCall, state *transcriptState) {
	forge.ExtractFilePathsFromTool(tool.Input, state)
}

func isConversationMessage(msg types.TranscriptMessage) bool {
	return forge.IsConversationMessage(msg.Type, msg.Role)
}

func inferSessionIDFromPath(filePath string) string {
	return forge.InferSessionIDFromPath(filePath)
}

func extractIssueRefs(content string, state *transcriptState) {
	forge.ExtractIssueRefs(content, state)
}

// generateSummary creates a session summary from extracted content.
func generateSummary(decisions, knowledge []string, date time.Time) string {
	return forge.GenerateSummary(decisions, knowledge, date)
}

// countLines quickly counts lines in a file.
func countLines(path string) int { return forge.CountLines(path) }

// extractSnippet extracts a text snippet around a match.
func extractSnippet(content string, startIdx, maxLen int) string {
	return forge.ExtractSnippet(content, startIdx, maxLen)
}

func lastSpaceIndex(s string) int { return forge.LastSpaceIndex(s) }

// extractIssueIDs finds issue IDs like "ol-0001", "at-v123" in content.
func extractIssueIDs(content string) []string { return forge.ExtractIssueIDs(content) }

// truncateString limits a string to maxLen characters.
func truncateString(s string, maxLen int) string { return forge.TruncateString(s, maxLen) }

// dedup removes duplicates from a string slice.
func dedup(items []string) []string { return forge.Dedup(items) }

// queueForExtraction adds a session to the pending extraction queue.
func queueForExtraction(session *storage.Session, sessionPath, transcriptPath, cwd string) error {
	pendingDir := filepath.Join(cwd, storage.DefaultBaseDir)
	if err := os.MkdirAll(pendingDir, 0750); err != nil {
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

	f, err := os.OpenFile(pendingPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
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

		forgeMarkdownFile(fs, session, filePath, baseDir, cwd, &totals)
	}

	if !forgeMdQuiet {
		printForgeSummary(w, totals, baseDir, "markdown file(s)")
	}

	return nil
}

func forgeMarkdownFile(fs *storage.FileStorage, session *storage.Session, filePath, baseDir, cwd string, totals *forgeTotals) {
	sessionPath, err := fs.WriteSession(session)
	if err != nil {
		forgeWarnf(forgeMdQuiet, "Warning: failed to write session for %s: %v\n", filePath, err)
		return
	}

	if err := writeSessionIndex(fs, session, sessionPath); err != nil {
		forgeWarnf(forgeMdQuiet, "Warning: failed to index session: %v\n", err)
	}

	if err := writeSessionProvenance(fs, session.ID, sessionPath, filePath, "markdown", false); err != nil {
		forgeWarnf(forgeMdQuiet, "Warning: failed to write provenance: %v\n", err)
	}

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

	// Auto-write pending learnings for close-loop ingestion (bridges forge→pool)
	if n, err := writePendingLearnings(session, cwd); err != nil {
		forgeWarnf(forgeMdQuiet, "Warning: failed to write pending learnings: %v\n", err)
	} else if n > 0 && !forgeMdQuiet {
		VerbosePrintf("  → %d pending learning(s) written\n", n)
	}
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

	state := forge.NewTranscriptState()

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

	session.Summary = generateSummary(state.Decisions, state.Knowledge, session.Date)
	session.Decisions = dedup(state.Decisions)
	session.Knowledge = dedup(state.Knowledge)
	session.Issues = state.Issues

	return session, nil
}

// splitMarkdownSections splits markdown content by heading boundaries.
func splitMarkdownSections(content string) []string {
	return forge.SplitMarkdownSections(content)
}

// updateSearchIndexForFile loads the search index (if it exists), updates the
// entry for the given file path, and saves it back. If no index exists yet
// this is a no-op -- the user can create one with `ao store rebuild`.
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

type fileWithTime struct {
	path    string
	modTime time.Time
}

func isTranscriptCandidate(path string, info os.FileInfo, projectsDir string) bool {
	return forge.IsTranscriptCandidate(path, info, projectsDir)
}

func collectTranscriptCandidates(projectsDir string) ([]fileWithTime, error) {
	pkgCands, err := forge.CollectTranscriptCandidates(projectsDir)
	if err != nil {
		return nil, err
	}
	out := make([]fileWithTime, len(pkgCands))
	for i, c := range pkgCands {
		out[i] = fileWithTime{path: c.Path, modTime: c.ModTime}
	}
	return out, nil
}

// runMinePassAdapter is a thin cobra-side adapter over forge.RunMinePass.
// The cobra command flow (runForgeTranscript) processes raw Claude JSONL
// transcripts from ~/.claude/projects/ and is independent of RunMinePass,
// which mines already-forged session files under .agents/sessions/. This
// adapter exists so non-cobra callers (Dream INGEST, tests) can exercise
// the in-process entry point from the cmd/ao package without duplicating
// the forge.MineOpts construction logic. It also anchors the planned
// extraction pattern (mirror of lifecycle.ExecuteCloseLoop) for future
// refactors that may fold the raw-transcript flow into the same entry.
func runMinePassAdapter(cwd string, sessionsDir string, sinceTime time.Time, quiet bool) (*forge.MineReport, error) {
	return forge.RunMinePass(cwd, forge.MineOpts{
		SessionsDir: sessionsDir,
		SinceTime:   sinceTime,
		Quiet:       quiet,
	})
}

// runForgeTier1 dispatches --tier=1. A configured Dream worker queue wins; when
// absent, the local-LLM fallback remains available through cli/internal/llm.
func runForgeTier1(w io.Writer, files []string) error {
	if handled, err := runForgeTier1ViaCuratorQueue(w, files); handled || err != nil {
		return err
	}
	if forgeTier1Model == "" {
		return fmt.Errorf("--tier=1 requires --model (e.g. --model=gemma2:9b)")
	}
	cwd, _ := os.Getwd()
	outDir := filepath.Join(cwd, ".agents", "ao", "sessions")
	_, err := llm.RunForgeTier1(llm.Tier1Options{
		SourcePaths: files,
		OutputDir:   outDir,
		Model:       forgeTier1Model,
		Endpoint:    forgeLLMEndpoint,
		Quiet:       forgeQuiet,
		Writer:      w,
		Workspace:   cwd,
	})
	return err
}

func runForgeTier1ViaCuratorQueue(w io.Writer, files []string) (bool, error) {
	result, handled, err := enqueueForgeTier1ToCuratorQueue(files)
	if !handled || err != nil {
		return handled, err
	}
	if w == nil {
		w = io.Discard
	}
	if !forgeQuiet {
		fmt.Fprintf(w, "Queued %d transcript file(s) for Dream curator Tier 1: %s\n", result.JobsQueued, result.QueueDir)
	}
	return true, nil
}

type curatorQueueEnqueueResult struct {
	QueueDir   string
	JobsQueued int
}

func enqueueForgeTier1ToCuratorQueue(files []string) (curatorQueueEnqueueResult, bool, error) {
	workerDir := configuredDreamCuratorWorkerDir()
	if workerDir == "" {
		return curatorQueueEnqueueResult{}, false, nil
	}
	queueDir := filepath.Join(workerDir, "queue")
	if err := os.MkdirAll(queueDir, 0o755); err != nil {
		return curatorQueueEnqueueResult{}, true, fmt.Errorf("create Dream curator queue dir: %w", err)
	}

	for _, sourcePath := range files {
		job := curatorJob{
			ID:         buildCuratorID("ingest-claude-session"),
			Kind:       "ingest-claude-session",
			CreatedAt:  time.Now().UTC().Format(time.RFC3339),
			MaxRetries: 1,
			Source: &curatorJobSource{
				Path:       sourcePath,
				ChunkStart: 0,
				ChunkEnd:   1,
			},
		}
		if err := writeJSONAtomic(filepath.Join(queueDir, job.ID+".json"), job); err != nil {
			return curatorQueueEnqueueResult{}, true, fmt.Errorf("enqueue Dream curator job for %s: %w", sourcePath, err)
		}
	}

	return curatorQueueEnqueueResult{QueueDir: queueDir, JobsQueued: len(files)}, true, nil
}

func configuredDreamCuratorWorkerDir() string {
	resolved := config.Resolve("", "", false)
	workerDir, _ := resolved.DreamCuratorWorkerDir.Value.(string)
	return expandConfiguredPath(workerDir)
}
