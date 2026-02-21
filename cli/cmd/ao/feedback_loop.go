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

	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/types"
)

// FeedbackEvent records a feedback loop closure event.
type FeedbackEvent struct {
	SessionID      string    `json:"session_id"`
	ArtifactPath   string    `json:"artifact_path"`
	Reward         float64   `json:"reward"`
	UtilityBefore  float64   `json:"utility_before"`
	UtilityAfter   float64   `json:"utility_after"`
	Alpha          float64   `json:"alpha"`
	RecordedAt     time.Time `json:"recorded_at"`
	TranscriptPath string    `json:"transcript_path,omitempty"`
}

// FeedbackFilePath is the relative path to the feedback log.
const FeedbackFilePath = ".agents/ao/feedback.jsonl"

var feedbackLoopCmd = &cobra.Command{
	Use:   "feedback-loop",
	Short: "Close the MemRL feedback loop for a session",
	Long: `Automatically close the MemRL feedback loop by updating utilities of cited learnings.

This command:
1. Reads citations for the session from .agents/ao/citations.jsonl
2. Computes reward from session outcome (or uses --reward override)
3. Updates utility of each cited learning via EMA rule
4. Logs feedback events to .agents/ao/feedback.jsonl

The feedback loop enables knowledge to compound:
- High-utility learnings surface more often
- Learnings that correlate with success get reinforced
- Learnings that don't help slowly decay

Examples:
  ao feedback-loop --session session-20260125-120000
  ao feedback-loop --session abc123 --reward 0.85
  ao feedback-loop --transcript ~/.claude/projects/*/abc.jsonl`,
	RunE: runFeedbackLoop,
}

var (
	feedbackLoopSessionID    string
	feedbackLoopReward       float64
	feedbackLoopTranscript   string
	feedbackLoopAlpha        float64
	feedbackLoopCitationType string
)

func init() {
	rootCmd.AddCommand(feedbackLoopCmd)
	feedbackLoopCmd.Flags().StringVar(&feedbackLoopSessionID, "session", "", "Session ID to process")
	feedbackLoopCmd.Flags().Float64Var(&feedbackLoopReward, "reward", -1, "Override reward value (0.0-1.0); -1 = compute from transcript")
	feedbackLoopCmd.Flags().StringVar(&feedbackLoopTranscript, "transcript", "", "Path to transcript for reward computation")
	feedbackLoopCmd.Flags().Float64Var(&feedbackLoopAlpha, "alpha", types.DefaultAlpha, "EMA learning rate")
	feedbackLoopCmd.Flags().StringVar(&feedbackLoopCitationType, "citation-type", "retrieved", "Filter citations by type (retrieved, applied, all)")
}

// loadSessionCitations loads and filters citations for a session.
func loadSessionCitations(cwd, sessionID, citationType string) ([]types.CitationEvent, error) {
	allCitations, err := ratchet.LoadCitations(cwd)
	if err != nil {
		return nil, fmt.Errorf("load citations: %w", err)
	}

	targetAliases := make(map[string]bool)
	for _, alias := range sessionIDAliases(sessionID) {
		targetAliases[alias] = true
	}

	var sessionCitations []types.CitationEvent
	for _, c := range allCitations {
		c.SessionID = canonicalSessionID(c.SessionID)
		c.ArtifactPath = canonicalArtifactPath(cwd, c.ArtifactPath)
		if !targetAliases[c.SessionID] {
			continue
		}
		if citationType != "all" && c.CitationType != citationType {
			continue
		}
		sessionCitations = append(sessionCitations, c)
	}
	return sessionCitations, nil
}

// computeRewardFromTranscript derives reward from transcript analysis.
func computeRewardFromTranscript(transcriptPath, sessionID string) (float64, error) {
	if transcriptPath == "" {
		homeDir, _ := os.UserHomeDir()
		transcriptsDir := filepath.Join(homeDir, ".claude", "projects")
		transcriptPath = findTranscriptForSession(transcriptsDir, sessionID)
		if transcriptPath == "" {
			transcriptPath = findMostRecentTranscript(transcriptsDir)
		}
	}
	if transcriptPath == "" {
		return 0, fmt.Errorf("no transcript found; use --reward to specify manually")
	}
	outcome, err := analyzeTranscript(transcriptPath, sessionID)
	if err != nil {
		return 0, fmt.Errorf("analyze transcript: %w", err)
	}
	VerbosePrintf("Computed reward %.2f from transcript %s\n", outcome.Reward, transcriptPath)
	return outcome.Reward, nil
}

// deduplicateCitations returns unique citations by artifact path.
func deduplicateCitations(baseDir string, citations []types.CitationEvent) []types.CitationEvent {
	seen := make(map[string]bool)
	var unique []types.CitationEvent
	for _, c := range citations {
		c.ArtifactPath = canonicalArtifactPath(baseDir, c.ArtifactPath)
		key := canonicalArtifactKey(baseDir, c.ArtifactPath)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, c)
		}
	}
	return unique
}

// processUniqueCitations updates learning utilities and returns feedback events.
func processUniqueCitations(cwd, sessionID, transcriptPath string, citations []types.CitationEvent, reward, alpha float64) ([]FeedbackEvent, int, int) {
	var events []FeedbackEvent
	updatedCount, failedCount := 0, 0

	for _, citation := range citations {
		artifactPath := canonicalArtifactPath(cwd, citation.ArtifactPath)
		learningPath, err := findLearningFile(cwd, filepath.Base(artifactPath))
		if err != nil {
			if _, statErr := os.Stat(artifactPath); statErr == nil {
				learningPath = artifactPath
			} else {
				VerbosePrintf("Warning: learning not found for %s: %v\n", artifactPath, err)
				failedCount++
				continue
			}
		}

		oldUtility, newUtility, err := updateLearningUtility(learningPath, reward, alpha)
		if err != nil {
			VerbosePrintf("Warning: failed to update %s: %v\n", learningPath, err)
			failedCount++
			continue
		}

		event := FeedbackEvent{
			SessionID:      canonicalSessionID(sessionID),
			ArtifactPath:   learningPath,
			Reward:         reward,
			UtilityBefore:  oldUtility,
			UtilityAfter:   newUtility,
			Alpha:          alpha,
			RecordedAt:     time.Now(),
			TranscriptPath: transcriptPath,
		}
		events = append(events, event)
		updatedCount++

		VerbosePrintf("Updated %s: %.3f → %.3f (reward=%.2f)\n",
			filepath.Base(learningPath), oldUtility, newUtility, reward)
	}

	return events, updatedCount, failedCount
}

func runFeedbackLoop(cmd *cobra.Command, args []string) error {
	sessionID, err := resolveFeedbackLoopSessionID(feedbackLoopSessionID)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	if GetDryRun() {
		fmt.Printf("[dry-run] Would process feedback loop for session: %s\n", sessionID)
		return nil
	}

	// Load and filter citations
	sessionCitations, err := loadSessionCitations(cwd, sessionID, feedbackLoopCitationType)
	if err != nil {
		return err
	}
	if len(sessionCitations) == 0 {
		fmt.Printf("No citations found for session %s\n", sessionID)
		return nil
	}

	// Determine reward
	reward := feedbackLoopReward
	if reward < 0 || reward > 1 {
		reward, err = computeRewardFromTranscript(feedbackLoopTranscript, sessionID)
		if err != nil {
			return err
		}
	}

	// Process citations
	uniqueCitations := deduplicateCitations(cwd, sessionCitations)
	feedbackEvents, updatedCount, failedCount := processUniqueCitations(
		cwd, sessionID, feedbackLoopTranscript, uniqueCitations, reward, feedbackLoopAlpha,
	)

	// Write feedback events to log
	if err := writeFeedbackEvents(cwd, feedbackEvents); err != nil {
		VerbosePrintf("Warning: failed to write feedback log: %v\n", err)
	}
	if err := markCitationFeedback(cwd, sessionID, reward, feedbackEvents); err != nil {
		VerbosePrintf("Warning: failed to mark citation feedback metadata: %v\n", err)
	}

	// Output summary
	return outputFeedbackSummary(sessionID, reward, len(sessionCitations), len(uniqueCitations), updatedCount, failedCount, feedbackEvents)
}

func resolveFeedbackLoopSessionID(sessionFlag string) (string, error) {
	candidate := strings.TrimSpace(sessionFlag)
	if candidate == "" {
		candidate = strings.TrimSpace(os.Getenv("CLAUDE_SESSION_ID"))
	}
	if candidate == "" {
		return "", fmt.Errorf("--session is required (or set CLAUDE_SESSION_ID)")
	}
	return canonicalSessionID(candidate), nil
}

func markCitationFeedback(baseDir, sessionID string, reward float64, events []FeedbackEvent) error {
	citations, err := ratchet.LoadCitations(baseDir)
	if err != nil {
		return fmt.Errorf("load citations for feedback mark: %w", err)
	}
	if len(citations) == 0 {
		return nil
	}

	targetAliases := make(map[string]bool)
	for _, alias := range sessionIDAliases(sessionID) {
		targetAliases[alias] = true
	}

	eventByPath := make(map[string]FeedbackEvent, len(events))
	for _, event := range events {
		key := canonicalArtifactKey(baseDir, event.ArtifactPath)
		eventByPath[key] = event
	}

	updated := 0
	now := time.Now()
	for i := range citations {
		citations[i].SessionID = canonicalSessionID(citations[i].SessionID)
		citations[i].ArtifactPath = canonicalArtifactPath(baseDir, citations[i].ArtifactPath)
		if !targetAliases[citations[i].SessionID] {
			continue
		}
		citations[i].FeedbackGiven = true
		citations[i].FeedbackReward = reward
		citations[i].FeedbackAt = now
		if event, ok := eventByPath[canonicalArtifactKey(baseDir, citations[i].ArtifactPath)]; ok {
			citations[i].UtilityBefore = event.UtilityBefore
			citations[i].UtilityAfter = event.UtilityAfter
		}
		updated++
	}
	if updated == 0 {
		return nil
	}

	return writeCitations(baseDir, citations)
}

func writeCitations(baseDir string, citations []types.CitationEvent) error {
	citationsPath := filepath.Join(baseDir, ratchet.CitationsFilePath)
	if err := os.MkdirAll(filepath.Dir(citationsPath), 0755); err != nil {
		return fmt.Errorf("create citations directory: %w", err)
	}

	tmpPath := citationsPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create citations temp file: %w", err)
	}

	enc := json.NewEncoder(f)
	for _, citation := range citations {
		if err := enc.Encode(citation); err != nil {
			_ = f.Close()
			return fmt.Errorf("write citation event: %w", err)
		}
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close citations temp file: %w", err)
	}
	if err := os.Rename(tmpPath, citationsPath); err != nil {
		return fmt.Errorf("replace citations file: %w", err)
	}
	return nil
}

// outputFeedbackSummary outputs the feedback loop results.
func outputFeedbackSummary(sessionID string, reward float64, totalCitations, uniqueCount, updatedCount, failedCount int, events []FeedbackEvent) error {
	switch GetOutput() {
	case "json":
		result := map[string]interface{}{
			"session_id": sessionID,
			"reward":     reward,
			"citations":  totalCitations,
			"unique":     uniqueCount,
			"updated":    updatedCount,
			"failed":     failedCount,
			"feedback":   events,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)

	default:
		fmt.Printf("Feedback Loop Complete\n")
		fmt.Printf("======================\n")
		fmt.Printf("Session:     %s\n", sessionID)
		fmt.Printf("Reward:      %.2f\n", reward)
		fmt.Printf("Citations:   %d (%d unique)\n", totalCitations, uniqueCount)
		fmt.Printf("Updated:     %d\n", updatedCount)
		if failedCount > 0 {
			fmt.Printf("Failed:      %d\n", failedCount)
		}
	}

	return nil
}

// writeFeedbackEvents appends feedback events to the feedback log.
func writeFeedbackEvents(baseDir string, events []FeedbackEvent) error {
	if len(events) == 0 {
		return nil
	}

	feedbackPath := filepath.Join(baseDir, FeedbackFilePath)

	// Create parent directory
	if err := os.MkdirAll(filepath.Dir(feedbackPath), 0755); err != nil {
		return fmt.Errorf("create feedback directory: %w", err)
	}

	// Open for append
	f, err := os.OpenFile(feedbackPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open feedback file: %w", err)
	}
	defer f.Close() //nolint:errcheck // write-only file, Close error non-actionable

	// Write each event as JSONL
	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			continue
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			return fmt.Errorf("write feedback event: %w", err)
		}
	}

	return nil
}

// batchFeedbackCmd processes feedback for multiple sessions.
var batchFeedbackCmd = &cobra.Command{
	Use:   "batch-feedback",
	Short: "Process feedback loop for all recent sessions",
	Long: `Process feedback loop for all sessions that have citations but no feedback.

Scans .agents/ao/citations.jsonl for sessions without corresponding
entries in .agents/ao/feedback.jsonl and processes them.

Examples:
  ao batch-feedback
  ao batch-feedback --days 7
  ao batch-feedback --dry-run`,
	RunE: runBatchFeedback,
}

var batchFeedbackDays int
var batchFeedbackMaxSessions int
var batchFeedbackReward float64
var batchFeedbackMaxRuntime time.Duration

func init() {
	rootCmd.AddCommand(batchFeedbackCmd)
	batchFeedbackCmd.Flags().IntVar(&batchFeedbackDays, "days", 7, "Process sessions from the last N days")
	batchFeedbackCmd.Flags().IntVar(&batchFeedbackMaxSessions, "max-sessions", 0, "Process at most N sessions per run (0 = no limit)")
	batchFeedbackCmd.Flags().Float64Var(&batchFeedbackReward, "reward", -1, "Override reward value for all sessions (0.0-1.0); -1 = compute from transcript")
	batchFeedbackCmd.Flags().DurationVar(&batchFeedbackMaxRuntime, "max-runtime", 0, "Maximum wall-clock runtime for this invocation (0 disables)")
}

func runBatchFeedback(cmd *cobra.Command, args []string) error {
	if batchFeedbackMaxSessions < 0 {
		return fmt.Errorf("--max-sessions must be >= 0")
	}
	if batchFeedbackReward != -1 && (batchFeedbackReward < 0 || batchFeedbackReward > 1) {
		return fmt.Errorf("--reward must be between 0.0 and 1.0, or -1 to auto-compute")
	}
	if batchFeedbackMaxRuntime < 0 {
		return fmt.Errorf("--max-runtime must be >= 0")
	}

	startedAt := time.Now()

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Load all citations
	citations, err := ratchet.LoadCitations(cwd)
	if err != nil {
		return fmt.Errorf("load citations: %w", err)
	}

	// Load existing feedback
	existingFeedback, err := loadFeedbackEvents(cwd)
	if err != nil && !os.IsNotExist(err) {
		VerbosePrintf("Warning: failed to load feedback: %v\n", err)
	}

	// Build set of sessions that already have feedback
	processedSessions := make(map[string]bool)
	for _, f := range existingFeedback {
		processedSessions[canonicalSessionID(f.SessionID)] = true
	}

	// Find sessions with citations but no feedback
	since := time.Now().AddDate(0, 0, -batchFeedbackDays)
	sessionCitations := make(map[string][]types.CitationEvent)
	sessionLatestCitation := make(map[string]time.Time)

	for _, c := range citations {
		if c.CitedAt.Before(since) {
			continue
		}
		sessionKey := canonicalSessionID(c.SessionID)
		c.SessionID = sessionKey
		if processedSessions[sessionKey] {
			continue
		}
		sessionCitations[sessionKey] = append(sessionCitations[sessionKey], c)
		if latest, ok := sessionLatestCitation[sessionKey]; !ok || c.CitedAt.After(latest) {
			sessionLatestCitation[sessionKey] = c.CitedAt
		}
	}

	if len(sessionCitations) == 0 {
		fmt.Println("No unprocessed sessions found")
		return nil
	}

	sessionIDs := make([]string, 0, len(sessionCitations))
	for sessionID := range sessionCitations {
		sessionIDs = append(sessionIDs, sessionID)
	}
	sort.Slice(sessionIDs, func(i, j int) bool {
		ti := sessionLatestCitation[sessionIDs[i]]
		tj := sessionLatestCitation[sessionIDs[j]]
		if ti.Equal(tj) {
			return sessionIDs[i] < sessionIDs[j]
		}
		return ti.After(tj)
	})

	candidateCount := len(sessionIDs)
	if batchFeedbackMaxSessions > 0 && len(sessionIDs) > batchFeedbackMaxSessions {
		sessionIDs = sessionIDs[:batchFeedbackMaxSessions]
	}

	if GetDryRun() {
		fmt.Printf("[dry-run] Would process %d/%d sessions:\n", len(sessionIDs), candidateCount)
		for _, sessionID := range sessionIDs {
			citations := sessionCitations[sessionID]
			fmt.Printf("  - %s (%d citations)\n", sessionID, len(citations))
		}
		return nil
	}

	// Process each session
	processed := 0
	for _, sessionID := range sessionIDs {
		if batchFeedbackMaxRuntime > 0 && time.Since(startedAt) >= batchFeedbackMaxRuntime {
			VerbosePrintf("Batch feedback runtime budget reached after %s\n", time.Since(startedAt).Truncate(time.Millisecond))
			break
		}

		// Set flags and run feedback loop
		feedbackLoopSessionID = sessionID
		feedbackLoopReward = batchFeedbackReward
		feedbackLoopTranscript = ""

		fmt.Printf("Processing session %s...\n", sessionID)
		if err := runFeedbackLoop(cmd, nil); err != nil {
			VerbosePrintf("Warning: failed to process %s: %v\n", sessionID, err)
			continue
		}
		processed++
	}

	fmt.Printf("\nProcessed %d/%d sessions (candidates: %d)\n", processed, len(sessionIDs), candidateCount)
	return nil
}

// loadFeedbackEvents reads all feedback events from the log.
func loadFeedbackEvents(baseDir string) ([]FeedbackEvent, error) {
	feedbackPath := filepath.Join(baseDir, FeedbackFilePath)

	f, err := os.Open(feedbackPath)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read-only file, Close error non-actionable

	var events []FeedbackEvent
	decoder := json.NewDecoder(f)
	for decoder.More() {
		var event FeedbackEvent
		if err := decoder.Decode(&event); err != nil {
			continue // Skip malformed lines
		}
		events = append(events, event)
	}

	return events, nil
}
