package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/formatter"
	"github.com/boshu2/agentops/cli/internal/parser"
	"github.com/boshu2/agentops/cli/internal/storage"
	"github.com/boshu2/agentops/cli/internal/types"
)

// sessionCmd is the parent command for session operations.
var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Session lifecycle operations",
	Long: `Session lifecycle operations.

Commands:
  close    Forge transcript, extract learnings, measure flywheel delta

Examples:
  ao session close
  ao session close --session abc123
  ao session close --dry-run
  ao session close --json`,
}

var sessionCloseSessionID string
var sessionCloseAutoExtract bool

// autoExtractContinuationPrefixes rejects content starting with mid-sentence
// fragments or operational chatter. Add new entries grouped by category:
//   - session chatter: "till ", "still ", "will "
//   - doc structure: "see also", "key learnings"
//   - mid-thought: "choosing the", "selected ", "anti-pattern"
var autoExtractContinuationPrefixes = []string{
	// session chatter
	"till ", "still ", "will ", "let me ",
	// doc structure
	"see also", "key learnings", "key insight",
	// mid-thought continuations
	"choosing the", "selected ", "anti-pattern",
	"and ", "but ", "or ", "however ", "therefore ",
	"additionally ", "furthermore ",
	// list/reference orphans
	"- ", "* ", "the ", "a ", "an ", "this ", "that ", "these ",
	// operational noise
	"going with", "picked up", "resolved by",
}

// autoExtractSkillPatterns rejects content that parrots skill doc structure.
// Keep in sync when adding new skills referenced in auto-extract sources.
var autoExtractSkillPatterns = []string{
	"## Phase", "### Step", "```bash\n#", "```validation",
	"skills/council/SKILL.md", "skills/research/SKILL.md",
	"skills/plan/SKILL.md",
}

var sessionCloseCmd = &cobra.Command{
	Use:   "close",
	Short: "Forge transcript, extract learnings, measure flywheel impact",
	Long: `Close a session by forging its transcript, extracting learnings,
measuring the flywheel delta, and reporting impact.

Pipeline:
  1. Find transcript (--session flag or most recent)
  2. Forge: extract knowledge from transcript
  3. Extract: queue and process learnings
  4. Measure: compute flywheel metrics delta
  5. Report: summarize impact

Examples:
  ao session close                        # Close most recent session
  ao session close --session abc123       # Close specific session
  ao session close --dry-run              # Preview what would happen
  ao session close --json                # Structured output`,
	RunE: runSessionClose,
}

func init() {
	sessionCmd.GroupID = "workflow"
	rootCmd.AddCommand(sessionCmd)
	sessionCmd.AddCommand(sessionCloseCmd)

	sessionCloseCmd.Flags().StringVar(&sessionCloseSessionID, "session", "", "Session ID to close (default: most recent transcript)")
	sessionCloseCmd.Flags().BoolVar(&sessionCloseAutoExtract, "auto-extract", false,
		"Extract lightweight learnings (quality-filtered) and write handoff artifact")
}

// SessionCloseResult holds the result of a session close operation.
type SessionCloseResult struct {
	SessionID          string  `json:"session_id"`
	Transcript         string  `json:"transcript"`
	Decisions          int     `json:"decisions"`
	Knowledge          int     `json:"knowledge"`
	FilesChanged       int     `json:"files_changed"`
	Issues             int     `json:"issues"`
	VelocityDelta      float64 `json:"velocity_delta"`
	Status             string  `json:"status"`
	Message            string  `json:"message"`
	LearningsExtracted int     `json:"learnings_extracted"`
	LearningsRejected  int     `json:"learnings_rejected,omitempty"`
	HandoffWritten     string  `json:"handoff_written,omitempty"`
}

func runSessionClose(cmd *cobra.Command, args []string) error {
	// Step 1: Find transcript
	transcriptPath, usedFallback, err := resolveTranscript(sessionCloseSessionID)
	if err != nil {
		return fmt.Errorf("find transcript: %w", err)
	}

	if usedFallback {
		fmt.Fprintf(os.Stderr, "No --session specified, using most recent transcript\n")
	}

	// Step 2: Dry-run check
	if GetDryRun() {
		result := SessionCloseResult{
			SessionID:  sessionCloseSessionID,
			Transcript: transcriptPath,
			Message:    "dry-run: would forge, extract, and measure flywheel delta",
		}
		return outputCloseResult(result)
	}

	// Step 3: Forge, extract, measure, build result
	return forgeExtractAndReport(transcriptPath, sessionCloseAutoExtract)
}

// forgeExtractAndReport runs the forge/extract/measure pipeline and outputs
// the session close result. Extracted from runSessionClose to reduce CC.
func forgeExtractAndReport(transcriptPath string, autoExtract bool) error {
	cwd, err := resolveProjectDir()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	result, err := forgeExtractReportWithOptions(transcriptPath, cwd, autoExtract, true)
	if err != nil {
		return err
	}
	return outputCloseResult(result)
}

func forgeExtractReport(transcriptPath, cwd string, autoExtract bool) (SessionCloseResult, error) {
	return forgeExtractReportWithOptions(transcriptPath, cwd, autoExtract, true)
}

func forgeExtractReportWithOptions(transcriptPath, cwd string, autoExtract, processPending bool) (SessionCloseResult, error) {
	preMetrics, err := computeMetrics(cwd, 7)
	if err != nil {
		VerbosePrintf("Warning: pre-forge metrics: %v\n", err)
	}

	session, err := forgeTranscriptForClose(transcriptPath, cwd)
	if err != nil {
		return SessionCloseResult{}, fmt.Errorf("forge transcript: %w", err)
	}

	VerbosePrintf("Forged session %s: %d decisions, %d knowledge items\n",
		session.ID, len(session.Decisions), len(session.Knowledge))

	extractCount, err := extractForClose(session, transcriptPath, cwd, processPending)
	if err != nil {
		VerbosePrintf("Warning: extraction: %v\n", err)
	} else {
		VerbosePrintf("Extraction processed %d entries\n", extractCount)
	}

	postMetrics, err := computeMetrics(cwd, 7)
	if err != nil {
		VerbosePrintf("Warning: post-forge metrics: %v\n", err)
	}
	populateGoldenSignals(cwd, 7, postMetrics)

	velocityDelta := computeVelocityDelta(preMetrics, postMetrics)
	status := classifyFlywheelStatus(postMetrics)

	var extractResult autoExtractResult
	var handoffPath string

	if autoExtract {
		extractResult, err = writeAutoExtractedLearnings(cwd, session.Decisions, session.Knowledge)
		if err != nil {
			VerbosePrintf("Warning: auto-extract learnings: %v\n", err)
		}

		artifact := &handoffArtifact{
			SchemaVersion: 1,
			ID:            fmt.Sprintf("auto-%s", session.ID[:minInt(7, len(session.ID))]),
			CreatedAt:     time.Now().UTC().Format(time.RFC3339),
			Type:          "auto",
			Summary:       fmt.Sprintf("Auto-extracted from session %s", session.ID),
			DecisionsMade: session.Decisions,
		}
		handoffPath, err = writeHandoffArtifact(cwd, artifact)
		if err != nil {
			VerbosePrintf("Warning: auto-extract handoff: %v\n", err)
		}
	}

	return SessionCloseResult{
		SessionID:          session.ID,
		Transcript:         transcriptPath,
		Decisions:          len(session.Decisions),
		Knowledge:          len(session.Knowledge),
		FilesChanged:       len(session.FilesChanged),
		Issues:             len(session.Issues),
		VelocityDelta:      velocityDelta,
		Status:             status,
		Message:            fmt.Sprintf("Session closed: %d decisions, %d learnings extracted", len(session.Decisions), len(session.Knowledge)),
		LearningsExtracted: extractResult.written,
		LearningsRejected:  extractResult.rejected,
		HandoffWritten:     handoffPath,
	}, nil
}

// computeVelocityDelta returns the velocity change between pre and post metrics.
// Returns 0.0 if either measurement is nil.
func computeVelocityDelta(pre, post *types.FlywheelMetrics) float64 {
	if pre == nil || post == nil {
		return 0.0
	}
	return post.Velocity - pre.Velocity
}

// classifyFlywheelStatus returns a human-readable flywheel status label.
func classifyFlywheelStatus(post *types.FlywheelMetrics) string {
	if post == nil {
		return "unknown"
	}
	return strings.ToLower(strings.ReplaceAll(post.HealthStatus(), " ", "-"))
}

// resolveTranscript finds the transcript path from a session ID or fallback.
// Returns the path, whether fallback was used, and any error.
func resolveTranscript(sessionID string) (string, bool, error) {
	if sessionID != "" {
		path, err := findTranscriptBySessionID(sessionID)
		if err != nil {
			return "", false, err
		}
		return path, false, nil
	}

	// Fallback: most recently modified transcript
	path, err := findLastSession()
	if err != nil {
		return "", false, fmt.Errorf("no transcript found: %w", err)
	}
	return path, true, nil
}

// forgeTranscriptForClose runs the forge pipeline on a transcript.
func forgeTranscriptForClose(transcriptPath, cwd string) (*storage.Session, error) {
	baseDir := filepath.Join(cwd, storage.DefaultBaseDir)
	fs := storage.NewFileStorage(
		storage.WithBaseDir(baseDir),
		storage.WithFormatters(
			formatter.NewMarkdownFormatter(),
			formatter.NewJSONLFormatter(),
		),
	)

	if err := fs.Init(); err != nil {
		return nil, fmt.Errorf("initialize storage: %w", err)
	}

	p := parser.NewParser()
	p.MaxContentLength = 0

	extractor := parser.NewExtractor()

	session, err := processTranscript(transcriptPath, p, extractor, true, os.Stdout)
	if err != nil {
		return nil, fmt.Errorf("process transcript: %w", err)
	}

	sessionPath, err := fs.WriteSession(session)
	if err != nil {
		return nil, fmt.Errorf("write session: %w", err)
	}

	// Write index
	indexEntry := &storage.IndexEntry{
		SessionID:   session.ID,
		Date:        session.Date,
		SessionPath: sessionPath,
		Summary:     session.Summary,
	}
	if err := fs.WriteIndex(indexEntry); err != nil {
		VerbosePrintf("Warning: failed to index session: %v\n", err)
	}

	// Write provenance
	provRecord := &storage.ProvenanceRecord{
		ID:            fmt.Sprintf("prov-%s", session.ID[:7]),
		ArtifactPath:  sessionPath,
		WorkspacePath: canonicalWorkspacePath(cwd, cwd),
		ArtifactType:  "session",
		SourcePath:    transcriptPath,
		SourceType:    "transcript",
		SessionID:     session.ID,
		CreatedAt:     time.Now(),
	}
	if err := fs.WriteProvenance(provRecord); err != nil {
		VerbosePrintf("Warning: failed to write provenance: %v\n", err)
	}

	return session, nil
}

// extractForClose queues the session for extraction and processes it.
func extractForClose(session *storage.Session, transcriptPath, cwd string, processPending bool) (int, error) {
	// Queue session for extraction
	if err := queueForExtraction(session, "", transcriptPath, cwd); err != nil {
		return 0, fmt.Errorf("queue for extraction: %w", err)
	}

	if !processPending {
		return 0, nil
	}

	// Read and process pending extractions
	pendingPath := filepath.Join(cwd, storage.DefaultBaseDir, "pending.jsonl")
	pending, err := readPendingExtractions(pendingPath)
	if err != nil {
		return 0, fmt.Errorf("read pending: %w", err)
	}

	if len(pending) == 0 {
		return 0, nil
	}

	// Process all pending (includes the one we just queued)
	if err := runExtractAll(pendingPath, pending, cwd); err != nil {
		return 0, fmt.Errorf("extract: %w", err)
	}

	return len(pending), nil
}

// outputCloseResult formats and prints the session close result.
func outputCloseResult(result SessionCloseResult) error {
	switch GetOutput() {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)

	default:
		printCloseTable(result)
	}

	return nil
}

// printCloseTable prints a formatted table of session close results.
func printCloseTable(r SessionCloseResult) {
	fmt.Println()
	fmt.Println("  Session Close Summary")
	fmt.Println("  =====================")
	fmt.Println()

	if r.SessionID != "" {
		displayID := r.SessionID
		if len(displayID) > 12 {
			displayID = displayID[:12]
		}
		fmt.Printf("  Session:       %s\n", displayID)
	}
	fmt.Printf("  Transcript:    %s\n", shortenPath(r.Transcript))
	fmt.Printf("  Decisions:     %d\n", r.Decisions)
	fmt.Printf("  Knowledge:     %d\n", r.Knowledge)
	fmt.Printf("  Files changed: %d\n", r.FilesChanged)
	if r.Issues > 0 {
		fmt.Printf("  Issues:        %d\n", r.Issues)
	}
	if r.LearningsRejected > 0 {
		fmt.Printf("  Rejected:      %d (run with --verbose to see why)\n", r.LearningsRejected)
	}
	fmt.Println()

	// Flywheel impact
	sign := "+"
	if r.VelocityDelta < 0 {
		sign = ""
	}
	fmt.Printf("  Flywheel:      %s (velocity %s%.3f)\n", r.Status, sign, r.VelocityDelta)
	fmt.Println()
	fmt.Printf("  %s\n", r.Message)
	fmt.Println()
}

// qualifyAutoExtract returns true if the content is substantial enough
// to be a useful learning. Rejects fragments, skill doc parrots, and
// operational chatter.
func qualifyAutoExtract(content string) bool {
	trimmed := strings.TrimSpace(content)

	// Minimum word count
	words := strings.Fields(trimmed)
	if len(words) < 25 {
		return false
	}

	// No mid-sentence starts (lowercase or continuation words)
	lower := strings.ToLower(trimmed)
	for _, prefix := range autoExtractContinuationPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return false
		}
	}

	// No skill doc structure parroting
	for _, pat := range autoExtractSkillPatterns {
		if strings.Contains(trimmed, pat) {
			return false
		}
	}

	// Must contain at least one sentence-ending punctuation.
	// Use []rune to avoid byte/rune offset mismatch on non-ASCII content.
	runes := []rune(trimmed)
	sentenceEnders := 0
	for i, ch := range runes {
		if ch == '.' || ch == '!' || ch == '?' {
			if i == len(runes)-1 || runes[i+1] == ' ' || runes[i+1] == '\n' {
				sentenceEnders++
			}
		}
	}
	return sentenceEnders >= 2
}

// autoExtractResult holds counts from writeAutoExtractedLearnings.
type autoExtractResult struct {
	written  int
	rejected int
}

// writeAutoExtractedLearnings writes each decision and knowledge item as a
// lightweight learning file to .agents/learnings/. Returns written and rejected counts.
func writeAutoExtractedLearnings(cwd string, decisions []string, knowledge []string) (autoExtractResult, error) {
	var result autoExtractResult
	dir := filepath.Join(cwd, ".agents", "learnings")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return result, fmt.Errorf("create learnings dir: %w", err)
	}

	datePrefix := time.Now().Format("2006-01-02")

	type item struct {
		category string
		content  string
	}

	items := make([]item, 0, len(decisions)+len(knowledge))
	for _, d := range decisions {
		items = append(items, item{"decision", d})
	}
	for _, k := range knowledge {
		items = append(items, item{"knowledge", k})
	}

	for _, it := range items {
		if !qualifyAutoExtract(it.content) {
			VerbosePrintf("Skipped low-quality auto-extract: %s\n",
				truncate(it.content, 60))
			result.rejected++
			continue
		}

		// Dedup check: skip if first 80 chars match an existing learning from today
		existingFiles, _ := filepath.Glob(filepath.Join(dir, datePrefix+"-auto-*.md"))
		isDuplicate := false
		for _, ef := range existingFiles {
			body, readErr := os.ReadFile(ef)
			if readErr != nil {
				continue
			}
			existingContent := extractBodyAfterFrontmatter(string(body))
			if len(existingContent) > 80 && len(it.content) > 80 &&
				existingContent[:80] == it.content[:80] {
				isDuplicate = true
				break
			}
		}
		if isDuplicate {
			VerbosePrintf("Skipped duplicate auto-extract: %s\n",
				truncate(it.content, 60))
			result.rejected++
			continue
		}

		slug := slugify(it.content)
		if len(slug) > 40 {
			slug = slug[:40]
		}
		filename := fmt.Sprintf("%s-auto-%s.md", datePrefix, slug)
		target := filepath.Join(dir, filename)

		// Escape bare "---" lines in body to prevent YAML multi-document parsing issues.
		safeBody := strings.ReplaceAll(it.content, "\n---\n", "\n- - -\n")
		researchSources := renderResearchSourcesFrontmatter(gatherResearchSources(it.content))
		content := fmt.Sprintf("---\ntype: learning\nsource: auto-extract\nconfidence: medium\nmaturity: provisional\n%scategory: %s\n---\n\n%s\n", researchSources, it.category, safeBody)
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return result, fmt.Errorf("write learning %s: %w", filename, err)
		}
		result.written++
	}

	return result, nil
}

// extractBodyAfterFrontmatter returns content after YAML frontmatter (--- delimited).
func extractBodyAfterFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---\n") {
		return content
	}
	end := strings.Index(content[4:], "\n---\n")
	if end < 0 {
		return content
	}
	return strings.TrimSpace(content[4+end+5:])
}

func gatherResearchSources(texts ...string) []string {
	seen := make(map[string]bool)
	var refs []string
	for _, text := range texts {
		for _, ref := range extractResearchRefsFromText(text) {
			if seen[ref] {
				continue
			}
			seen[ref] = true
			refs = append(refs, ref)
		}
	}
	sort.Strings(refs)
	return refs
}

func renderResearchSourcesFrontmatter(refs []string) string {
	if len(refs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("research_sources:\n")
	for _, ref := range refs {
		fmt.Fprintf(&b, "  - %q\n", ref)
	}
	return b.String()
}

// minInt returns the smaller of two ints.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// shortenPath returns a display-friendly version of a file path.
func shortenPath(p string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if strings.HasPrefix(p, homeDir) {
		return "~" + p[len(homeDir):]
	}
	return p
}
