package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/lifecycle"
	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/types"
)

// FeedbackEvent records a feedback loop closure event.
// Alias for lifecycle.FeedbackLoopEvent.
type FeedbackEvent = lifecycle.FeedbackLoopEvent

// FeedbackFilePath is the relative path to the feedback log.
const FeedbackFilePath = lifecycle.FeedbackLoopFilePath

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

var validFeedbackCitationTypes = lifecycle.ValidFeedbackLoopCitationTypes

func init() {
	feedbackLoopCmd.Hidden = false
	feedbackLoopCmd.GroupID = "workflow"
	rootCmd.AddCommand(feedbackLoopCmd)
	feedbackLoopCmd.Flags().StringVar(&feedbackLoopSessionID, "session", "", "Session ID to process")
	feedbackLoopCmd.Flags().Float64Var(&feedbackLoopReward, "reward", -1, "Override reward value (0.0-1.0); -1 = compute from transcript")
	feedbackLoopCmd.Flags().StringVar(&feedbackLoopTranscript, "transcript", "", "Path to transcript for reward computation")
	feedbackLoopCmd.Flags().Float64Var(&feedbackLoopAlpha, "alpha", types.DefaultAlpha, "EMA learning rate")
	feedbackLoopCmd.Flags().StringVar(&feedbackLoopCitationType, "citation-type", "retrieved", "Filter citations by type (retrieved, applied, all)")
	_ = feedbackLoopCmd.RegisterFlagCompletionFunc("citation-type", staticCompletionFunc("retrieved", "applied", "all"))
}

// annealedAlpha computes a decaying learning rate.
// Delegates to lifecycle.AnnealedAlpha.
func annealedAlpha(baseAlpha float64, citationCount int) float64 {
	return lifecycle.AnnealedAlpha(baseAlpha, citationCount)
}

// getLearningRewardCount reads the reward_count from a learning file.
func getLearningRewardCount(path string) int {
	if strings.HasSuffix(path, ".jsonl") {
		data, ok := readLearningJSONLData(path)
		if !ok {
			return 0
		}
		if rc, ok := data["reward_count"].(float64); ok {
			return int(rc)
		}
		return 0
	}
	fields, err := parseFrontmatterFields(path, "reward_count")
	if err != nil {
		return 0
	}
	if rc, err := strconv.Atoi(fields["reward_count"]); err == nil {
		return rc
	}
	return 0
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

	sessionCitations := make([]types.CitationEvent, 0, len(allCitations))
	for _, c := range allCitations {
		c.SessionID = canonicalSessionID(c.SessionID)
		c.ArtifactPath = canonicalArtifactPath(cwd, c.ArtifactPath)
		c.CitationType = canonicalCitationType(c.CitationType)
		c.MetricNamespace = canonicalMetricNamespace(c.MetricNamespace)
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
		c.MetricNamespace = canonicalMetricNamespace(c.MetricNamespace)
		key := citationFeedbackNamespaceKey(baseDir, c.ArtifactPath, c.MetricNamespace)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, c)
		}
	}
	return unique
}

// processUniqueCitations updates learning utilities and returns feedback events.
func processUniqueCitations(cwd, sessionID, transcriptPath string, citations []types.CitationEvent, reward, alpha float64) ([]FeedbackEvent, int, int) {
	events := make([]FeedbackEvent, 0, len(citations))
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

		metricNamespace := canonicalMetricNamespace(citation.MetricNamespace)
		if !isPrimaryMetricNamespace(metricNamespace) {
			currentUtility := parseUtilityFromFile(learningPath)
			events = append(events, FeedbackEvent{
				SessionID:       canonicalSessionID(sessionID),
				ArtifactPath:    learningPath,
				WorkspacePath:   canonicalWorkspacePath(cwd, cwd),
				CitationType:    canonicalCitationType(citation.CitationType),
				MetricNamespace: metricNamespace,
				Decision:        "audited",
				Reason:          "non-primary-namespace",
				Reward:          0,
				UtilityBefore:   currentUtility,
				UtilityAfter:    currentUtility,
				Alpha:           0,
				RecordedAt:      time.Now(),
				TranscriptPath:  transcriptPath,
			})
			failedCount++
			continue
		}

		oldUtility, newUtility, err := updateLearningUtility(learningPath, reward, alpha)
		if err != nil {
			VerbosePrintf("Warning: failed to update %s: %v\n", learningPath, err)
			failedCount++
			continue
		}

		event := FeedbackEvent{
			SessionID:       canonicalSessionID(sessionID),
			ArtifactPath:    learningPath,
			WorkspacePath:   canonicalWorkspacePath(cwd, cwd),
			CitationType:    canonicalCitationType(citation.CitationType),
			MetricNamespace: metricNamespace,
			Decision:        "rewarded",
			Reason:          "feedback-loop-manual",
			Reward:          reward,
			UtilityBefore:   oldUtility,
			UtilityAfter:    newUtility,
			Alpha:           alpha,
			RecordedAt:      time.Now(),
			TranscriptPath:  transcriptPath,
		}
		events = append(events, event)
		updatedCount++

		VerbosePrintf("Updated %s: %.3f → %.3f (reward=%.2f)\n",
			filepath.Base(learningPath), oldUtility, newUtility, reward)
	}

	return events, updatedCount, failedCount
}

// resolveReward determines the reward value, either from the flag or from transcript analysis.
func resolveFeedbackReward(flagReward float64, transcriptPath, sessionID string) (float64, error) {
	if reward, ok := lifecycle.ResolveFeedbackLoopReward(flagReward); ok {
		return reward, nil
	}
	return computeRewardFromTranscript(transcriptPath, sessionID)
}

func validateFeedbackCitationType(citationType string) (string, error) {
	return lifecycle.ValidateFeedbackLoopCitationType(citationType)
}

func runFeedbackLoop(cmd *cobra.Command, args []string) error {
	sessionID, err := resolveFeedbackLoopSessionID(feedbackLoopSessionID)
	if err != nil {
		return err
	}
	citationType, err := validateFeedbackCitationType(feedbackLoopCitationType)
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

	sessionCitations, err := loadSessionCitations(cwd, sessionID, citationType)
	if err != nil {
		return err
	}
	if len(sessionCitations) == 0 {
		fmt.Printf("No citations found for session %s\n", sessionID)
		return nil
	}

	reward, err := resolveFeedbackReward(feedbackLoopReward, feedbackLoopTranscript, sessionID)
	if err != nil {
		return err
	}

	uniqueCitations := deduplicateCitations(cwd, sessionCitations)
	feedbackEvents, updatedCount, failedCount := processUniqueCitations(
		cwd, sessionID, feedbackLoopTranscript, uniqueCitations, reward, feedbackLoopAlpha,
	)

	if err := writeFeedbackEvents(cwd, feedbackEvents); err != nil {
		VerbosePrintf("Warning: failed to write feedback log: %v\n", err)
	}
	if err := markCitationFeedback(cwd, sessionID, reward, feedbackEvents); err != nil {
		VerbosePrintf("Warning: failed to mark citation feedback metadata: %v\n", err)
	}

	return outputFeedbackSummary(sessionID, reward, len(sessionCitations), len(uniqueCitations), updatedCount, failedCount, feedbackEvents)
}

func resolveFeedbackLoopSessionID(sessionFlag string) (string, error) {
	id, err := lifecycle.ResolveFeedbackLoopSessionID(sessionFlag)
	if err != nil {
		return "", err
	}
	return canonicalSessionID(id), nil
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
		key := citationFeedbackNamespaceKey(baseDir, event.ArtifactPath, event.MetricNamespace)
		eventByPath[key] = event
	}

	updated := 0
	now := time.Now()
	for i := range citations {
		citations[i].SessionID = canonicalSessionID(citations[i].SessionID)
		citations[i].ArtifactPath = canonicalArtifactPath(baseDir, citations[i].ArtifactPath)
		citations[i].MetricNamespace = canonicalMetricNamespace(citations[i].MetricNamespace)
		if !targetAliases[citations[i].SessionID] {
			continue
		}
		citations[i].FeedbackGiven = true
		citations[i].FeedbackReward = reward
		citations[i].FeedbackAt = now
		if event, ok := eventByPath[citationFeedbackNamespaceKey(baseDir, citations[i].ArtifactPath, citations[i].MetricNamespace)]; ok {
			citations[i].FeedbackReward = event.Reward
			citations[i].FeedbackAt = event.RecordedAt
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
	if err := os.MkdirAll(filepath.Dir(citationsPath), 0750); err != nil {
		return fmt.Errorf("create citations directory: %w", err)
	}

	tmpPath := citationsPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create citations temp file: %w", err)
	}

	enc := json.NewEncoder(f)
	for _, citation := range citations {
		citation = normalizeCitationEventForRuntime(baseDir, citation)
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
		result := map[string]any{
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
		fmt.Print(lifecycle.FormatFeedbackLoopSummaryText(lifecycle.FeedbackLoopSummary{
			SessionID:      sessionID,
			Reward:         reward,
			TotalCitations: totalCitations,
			UniqueCount:    uniqueCount,
			UpdatedCount:   updatedCount,
			FailedCount:    failedCount,
		}))
	}

	return nil
}

// writeFeedbackEvents appends feedback events to the feedback log.
func writeFeedbackEvents(baseDir string, events []FeedbackEvent) error {
	return lifecycle.WriteFeedbackLoopEvents(baseDir, events)
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
	batchFeedbackCmd.Hidden = true
	batchFeedbackCmd.GroupID = "knowledge"
	rootCmd.AddCommand(batchFeedbackCmd)
	batchFeedbackCmd.Flags().IntVar(&batchFeedbackDays, "days", 7, "Process sessions from the last N days")
	batchFeedbackCmd.Flags().IntVar(&batchFeedbackMaxSessions, "max-sessions", 0, "Process at most N sessions per run (0 = no limit)")
	batchFeedbackCmd.Flags().Float64Var(&batchFeedbackReward, "reward", -1, "Override reward value for all sessions (0.0-1.0); -1 = compute from transcript")
	batchFeedbackCmd.Flags().DurationVar(&batchFeedbackMaxRuntime, "max-runtime", 0, "Maximum wall-clock runtime for this invocation (0 disables)")
}

func runBatchFeedback(cmd *cobra.Command, args []string) error {
	if err := validateBatchFeedbackFlags(); err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	sessionCitations, sessionLatestCitation, err := discoverUnprocessedSessions(cwd)
	if err != nil {
		return err
	}
	if len(sessionCitations) == 0 {
		fmt.Println("No unprocessed sessions found")
		return nil
	}

	sessionIDs := sortAndCapSessions(sessionCitations, sessionLatestCitation)
	candidateCount := len(sessionCitations)

	if GetDryRun() {
		reportBatchFeedbackDryRun(sessionIDs, candidateCount, sessionCitations)
		return nil
	}

	processed := executeBatchFeedbackSessions(cmd, sessionIDs)
	fmt.Printf("\nProcessed %d/%d sessions (candidates: %d)\n", processed, len(sessionIDs), candidateCount)
	return nil
}

// validateBatchFeedbackFlags checks the batch-feedback command flags.
func validateBatchFeedbackFlags() error {
	return lifecycle.ValidateBatchFeedbackFlags(batchFeedbackMaxSessions, batchFeedbackReward, batchFeedbackMaxRuntime)
}

// discoverUnprocessedSessions finds sessions with citations but no feedback.
func discoverUnprocessedSessions(cwd string) (map[string][]types.CitationEvent, map[string]time.Time, error) {
	citations, err := ratchet.LoadCitations(cwd)
	if err != nil {
		return nil, nil, fmt.Errorf("load citations: %w", err)
	}

	processedSessions := buildProcessedSessionSet(cwd)
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

	return sessionCitations, sessionLatestCitation, nil
}

// buildProcessedSessionSet loads existing feedback and returns a set of session
// IDs that already have feedback.
func buildProcessedSessionSet(cwd string) map[string]bool {
	existingFeedback, err := loadFeedbackEvents(cwd)
	if err != nil && !os.IsNotExist(err) {
		VerbosePrintf("Warning: failed to load feedback: %v\n", err)
	}
	processed := make(map[string]bool, len(existingFeedback))
	for _, f := range existingFeedback {
		processed[canonicalSessionID(f.SessionID)] = true
	}
	return processed
}

// sortAndCapSessions sorts session IDs by latest citation (newest first) and
// caps the list by batchFeedbackMaxSessions.
func sortAndCapSessions(sessionCitations map[string][]types.CitationEvent, sessionLatestCitation map[string]time.Time) []string {
	return lifecycle.SortAndCapSessions(sessionCitations, sessionLatestCitation, batchFeedbackMaxSessions)
}

// reportBatchFeedbackDryRun prints what would be processed without making changes.
func reportBatchFeedbackDryRun(sessionIDs []string, candidateCount int, sessionCitations map[string][]types.CitationEvent) {
	fmt.Printf("[dry-run] Would process %d/%d sessions:\n", len(sessionIDs), candidateCount)
	for _, sessionID := range sessionIDs {
		citations := sessionCitations[sessionID]
		fmt.Printf("  - %s (%d citations)\n", sessionID, len(citations))
	}
}

// executeBatchFeedbackSessions processes each session's feedback loop, respecting
// the runtime budget. Returns the count of successfully processed sessions.
func executeBatchFeedbackSessions(cmd *cobra.Command, sessionIDs []string) int {
	startedAt := time.Now()
	processed := 0
	for _, sessionID := range sessionIDs {
		if batchFeedbackMaxRuntime > 0 && time.Since(startedAt) >= batchFeedbackMaxRuntime {
			VerbosePrintf("Batch feedback runtime budget reached after %s\n", time.Since(startedAt).Truncate(time.Millisecond))
			break
		}

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
	return processed
}

// loadFeedbackEvents reads all feedback events from the log.
func loadFeedbackEvents(baseDir string) ([]FeedbackEvent, error) {
	return lifecycle.LoadFeedbackLoopEvents(baseDir)
}
