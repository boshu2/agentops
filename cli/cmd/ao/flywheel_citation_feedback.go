package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/resolver"
	"github.com/boshu2/agentops/cli/internal/types"
)

// processCitationFeedback reads unprocessed citations from .agents/ao/citations.jsonl,
// applies positive MemRL feedback for each cited learning, and marks them as processed.
// Returns (total processed, rewarded count, skipped count).
func processCitationFeedback(cwd string) (int, int, int) {
	return processCitationFeedbackWithOptions(cwd, citationFeedbackOptions{
		MutateArtifacts: true,
	})
}

type citationFeedbackOptions struct {
	MutateArtifacts bool
}

func processCitationFeedbackWithOptions(cwd string, opts citationFeedbackOptions) (int, int, int) {
	citationsPath := filepath.Join(cwd, ratchet.CitationsFilePath)
	citations, err := ratchet.LoadCitations(cwd)
	if err != nil || len(citations) == 0 {
		return 0, 0, 0
	}

	unique := deduplicateCitationFeedbackTargets(cwd, citations)
	if len(unique) == 0 {
		return 0, 0, 0
	}

	reward, err := computeSessionRewardForCloseLoop(cwd)
	if err != nil {
		reward = types.InitialUtility
	}

	res := resolver.NewFileResolver(cwd)
	sessionID := canonicalSessionID("")
	mutateArtifacts := opts.MutateArtifacts && !GetDryRun()

	var rewarded, skipped int
	var feedbackEvents []FeedbackEvent
	for _, c := range unique {
		result := evaluateCitationFeedback(cwd, c, sessionID, reward, mutateArtifacts, res)
		if result.event != nil {
			feedbackEvents = append(feedbackEvents, *result.event)
		}
		rewarded += result.rewarded
		skipped += result.skipped
	}

	if !GetDryRun() && len(feedbackEvents) > 0 {
		_ = writeFeedbackEvents(cwd, feedbackEvents)
	}

	markCitationsFeedbackGiven(cwd, citationsPath, citations, feedbackEvents)

	return len(unique), rewarded, skipped
}

// citationFeedbackResult is the per-citation outcome consumed by
// processCitationFeedbackWithOptions: an optional event to append and the
// rewarded/skipped counter deltas.
type citationFeedbackResult struct {
	event    *FeedbackEvent
	rewarded int
	skipped  int
}

func evaluateCitationFeedback(
	cwd string,
	c types.CitationEvent,
	sessionID string,
	reward float64,
	mutateArtifacts bool,
	res *resolver.FileResolver,
) citationFeedbackResult {
	citationType := effectiveCitationFeedbackType(c.CitationType)
	decision, reason, rewardable := classifyCitationFeedback(citationType)
	metricNamespace := canonicalMetricNamespace(c.MetricNamespace)

	if !isPrimaryMetricNamespace(metricNamespace) {
		return nonPrimaryNamespaceCitationFeedback(cwd, c, citationType, metricNamespace, sessionID, res)
	}
	if isFindingArtifactPath(cwd, c.ArtifactPath) {
		return findingArtifactCitationFeedback(cwd, c, citationType, metricNamespace, decision, reason, rewardable, sessionID, mutateArtifacts)
	}
	return learningArtifactCitationFeedback(cwd, c, citationType, metricNamespace, decision, reason, rewardable, sessionID, reward, mutateArtifacts, res)
}

func nonPrimaryNamespaceCitationFeedback(
	cwd string,
	c types.CitationEvent,
	citationType, metricNamespace, sessionID string,
	res *resolver.FileResolver,
) citationFeedbackResult {
	artifactPath := canonicalArtifactPath(cwd, c.ArtifactPath)
	currentUtility := 0.0
	if !isFindingArtifactPath(cwd, c.ArtifactPath) {
		learningID := extractLearningID(c.ArtifactPath)
		if path, err := res.Resolve(learningID); err == nil {
			artifactPath = path
			currentUtility = parseUtilityFromFile(path)
		}
	}
	event := FeedbackEvent{
		SessionID:       sessionID,
		ArtifactPath:    artifactPath,
		CitationType:    citationType,
		MetricNamespace: metricNamespace,
		Decision:        "audited",
		Reason:          "non-primary-namespace",
		Reward:          0,
		UtilityBefore:   currentUtility,
		UtilityAfter:    currentUtility,
		Alpha:           0,
		RecordedAt:      time.Now(),
	}
	return citationFeedbackResult{event: &event, skipped: 1}
}

func findingArtifactCitationFeedback(
	cwd string,
	c types.CitationEvent,
	citationType, metricNamespace, decision, reason string,
	rewardable bool,
	sessionID string,
	mutateArtifacts bool,
) citationFeedbackResult {
	if !rewardable {
		return citationFeedbackResult{skipped: 1}
	}
	path := normalizeArtifactPath(cwd, c.ArtifactPath)
	citedAt := c.CitedAt
	if citedAt.IsZero() {
		citedAt = time.Now()
	}
	if mutateArtifacts {
		if err := updateFindingCitationFields(path, citedAt); err != nil {
			return citationFeedbackResult{skipped: 1}
		}
		return citationFeedbackResult{rewarded: 1}
	}
	event := FeedbackEvent{
		SessionID:       sessionID,
		ArtifactPath:    path,
		CitationType:    citationType,
		MetricNamespace: metricNamespace,
		Decision:        decision,
		Reason:          reason,
		Reward:          citationEventConfidence(c),
		UtilityBefore:   0,
		UtilityAfter:    0,
		Alpha:           0,
		RecordedAt:      time.Now(),
	}
	return citationFeedbackResult{event: &event, rewarded: 1}
}

func learningArtifactCitationFeedback(
	cwd string,
	c types.CitationEvent,
	citationType, metricNamespace, decision, reason string,
	rewardable bool,
	sessionID string,
	reward float64,
	mutateArtifacts bool,
	res *resolver.FileResolver,
) citationFeedbackResult {
	learningID := extractLearningID(c.ArtifactPath)
	path, err := res.Resolve(learningID)
	if err != nil {
		event := FeedbackEvent{
			SessionID:       sessionID,
			ArtifactPath:    canonicalArtifactPath(cwd, c.ArtifactPath),
			CitationType:    citationType,
			MetricNamespace: metricNamespace,
			Decision:        "skipped",
			Reason:          "artifact-not-resolved",
			RecordedAt:      time.Now(),
		}
		return citationFeedbackResult{event: &event, skipped: 1}
	}

	if !rewardable {
		currentUtility := parseUtilityFromFile(path)
		event := FeedbackEvent{
			SessionID:       sessionID,
			ArtifactPath:    path,
			CitationType:    citationType,
			MetricNamespace: metricNamespace,
			Decision:        decision,
			Reason:          reason,
			Reward:          0,
			UtilityBefore:   currentUtility,
			UtilityAfter:    currentUtility,
			Alpha:           0,
			RecordedAt:      time.Now(),
		}
		return citationFeedbackResult{event: &event, skipped: 1}
	}

	rewardCount := getLearningRewardCount(path)
	alpha := annealedAlpha(types.DefaultAlpha, rewardCount)
	if !mutateArtifacts {
		currentUtility := parseUtilityFromFile(path)
		event := FeedbackEvent{
			SessionID:       sessionID,
			ArtifactPath:    path,
			CitationType:    citationType,
			MetricNamespace: metricNamespace,
			Decision:        decision,
			Reason:          reason,
			Reward:          reward,
			UtilityBefore:   currentUtility,
			UtilityAfter:    currentUtility,
			Alpha:           0,
			RecordedAt:      time.Now(),
		}
		return citationFeedbackResult{event: &event, rewarded: 1}
	}

	oldUtility, newUtility, err := updateLearningUtility(path, reward, alpha)
	if err != nil {
		currentUtility := parseUtilityFromFile(path)
		event := FeedbackEvent{
			SessionID:       sessionID,
			ArtifactPath:    path,
			CitationType:    citationType,
			MetricNamespace: metricNamespace,
			Decision:        "skipped",
			Reason:          "utility-update-failed",
			Reward:          0,
			UtilityBefore:   currentUtility,
			UtilityAfter:    currentUtility,
			Alpha:           0,
			RecordedAt:      time.Now(),
		}
		return citationFeedbackResult{event: &event, skipped: 1}
	}

	event := FeedbackEvent{
		SessionID:       sessionID,
		ArtifactPath:    path,
		CitationType:    citationType,
		MetricNamespace: metricNamespace,
		Decision:        decision,
		Reason:          reason,
		Reward:          reward,
		UtilityBefore:   oldUtility,
		UtilityAfter:    newUtility,
		Alpha:           alpha,
		RecordedAt:      time.Now(),
	}
	return citationFeedbackResult{event: &event, rewarded: 1}
}

func deduplicateCitationFeedbackTargets(cwd string, citations []types.CitationEvent) []types.CitationEvent {
	type indexedCitation struct {
		order int
		event types.CitationEvent
	}

	byKey := make(map[string]indexedCitation)
	order := make([]string, 0, len(citations))
	for _, citation := range citations {
		if citation.FeedbackGiven {
			continue
		}
		citation.ArtifactPath = canonicalArtifactPath(cwd, citation.ArtifactPath)
		citation.CitationType = canonicalCitationType(citation.CitationType)
		citation.MetricNamespace = canonicalMetricNamespace(citation.MetricNamespace)
		key := citationFeedbackNamespaceKey(cwd, citation.ArtifactPath, citation.MetricNamespace)
		current, exists := byKey[key]
		if !exists {
			byKey[key] = indexedCitation{order: len(order), event: citation}
			order = append(order, key)
			continue
		}
		if preferCitationFeedbackEvidence(current.event, citation) {
			byKey[key] = indexedCitation{order: current.order, event: citation}
		}
	}

	unique := make([]types.CitationEvent, 0, len(byKey))
	for _, key := range order {
		unique = append(unique, byKey[key].event)
	}
	return unique
}

func preferCitationFeedbackEvidence(current, candidate types.CitationEvent) bool {
	currentRank := citationFeedbackEvidenceRank(effectiveCitationFeedbackType(current.CitationType))
	candidateRank := citationFeedbackEvidenceRank(effectiveCitationFeedbackType(candidate.CitationType))
	if candidateRank != currentRank {
		return candidateRank > currentRank
	}
	if current.CitedAt.IsZero() {
		return true
	}
	if candidate.CitedAt.IsZero() {
		return false
	}
	return candidate.CitedAt.After(current.CitedAt)
}

func citationFeedbackEvidenceRank(citationType string) int {
	switch citationType {
	case "applied":
		return 3
	case "reference":
		return 2
	case "retrieved":
		return 1
	default:
		return 0
	}
}

func effectiveCitationFeedbackType(citationType string) string {
	citationType = canonicalCitationType(citationType)
	if citationType == "" {
		return "reference"
	}
	return citationType
}

const highConfidenceCitationThreshold = 0.7

func citationConfidenceScore(citationType string) float64 {
	switch effectiveCitationFeedbackType(citationType) {
	case "applied":
		return 0.9
	case "reference":
		return 0.7
	case "retrieved":
		return 0.5
	default:
		return 0
	}
}

func citationEventConfidence(citation types.CitationEvent) float64 {
	if citation.MatchConfidence > 0 {
		return normalizeCitationMatchConfidence(citation.MatchConfidence)
	}
	return citationConfidenceScore(citation.CitationType)
}

func citationIsHighConfidence(citationType string) bool {
	return citationConfidenceScore(citationType) >= highConfidenceCitationThreshold
}

func citationEventIsHighConfidence(citation types.CitationEvent) bool {
	return citationEventConfidence(citation) >= highConfidenceCitationThreshold
}

func classifyCitationFeedback(citationType string) (decision, reason string, rewardable bool) {
	switch citationType {
	case "applied":
		return "rewarded", "artifact-applied", true
	case "reference":
		return "rewarded", "manual-reference", true
	case "retrieved":
		return "skipped", "retrieved-no-artifact-evidence", false
	default:
		return "skipped", "unsupported-citation-type", false
	}
}

func updateFindingCitationFields(path string, citedAt time.Time) error {
	finding, err := parseFindingFile(path)
	if err != nil {
		return err
	}
	hitCount := finding.HitCount + 1
	return updateFindingFrontMatter(path, map[string]string{
		"hit_count":  fmt.Sprintf("%d", hitCount),
		"last_cited": citedAt.UTC().Format(time.RFC3339),
	})
}

// extractLearningID derives a learning ID from an artifact path.
// Handles both relative (".agents/learnings/abc.md") and absolute
// ("/home/user/repo/.agents/learnings/abc.md") paths.
func extractLearningID(artifactPath string) string {
	for _, marker := range []string{"/.agents/learnings/", "/.agents/patterns/", ".agents/learnings/", ".agents/patterns/"} {
		if idx := strings.Index(artifactPath, marker); idx >= 0 {
			return artifactPath[idx+len(marker):]
		}
	}
	return filepath.Base(artifactPath)
}

// markCitationsFeedbackGiven rewrites citations.jsonl with FeedbackGiven=true for all entries.
func markCitationsFeedbackGiven(cwd, citationsPath string, citations []types.CitationEvent, feedbackEvents []FeedbackEvent) {
	if GetDryRun() {
		return
	}

	feedbackByPath := make(map[string]FeedbackEvent, len(feedbackEvents))
	for _, event := range feedbackEvents {
		feedbackByPath[citationFeedbackNamespaceKey(cwd, event.ArtifactPath, event.MetricNamespace)] = event
	}

	var lines []string
	for _, c := range citations {
		c.ArtifactPath = canonicalArtifactPath(cwd, c.ArtifactPath)
		c.CitationType = canonicalCitationType(c.CitationType)
		c.MetricNamespace = canonicalMetricNamespace(c.MetricNamespace)
		c.FeedbackGiven = true
		if event, ok := feedbackByPath[citationFeedbackNamespaceKey(cwd, c.ArtifactPath, c.MetricNamespace)]; ok {
			c.FeedbackReward = event.Reward
			c.UtilityBefore = event.UtilityBefore
			c.UtilityAfter = event.UtilityAfter
			c.FeedbackAt = event.RecordedAt
		}
		data, err := json.Marshal(c)
		if err != nil {
			continue
		}
		lines = append(lines, string(data))
	}

	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(citationsPath, []byte(content), 0600); err != nil {
		VerbosePrintf("Warning: failed to write citations feedback: %v\n", err)
	}
}

// computeSessionRewardForCloseLoop checks for a binary session outcome file first,
// then falls back to transcript analysis.
func computeSessionRewardForCloseLoop(cwd string) (float64, error) {
	outcomePath := filepath.Join(cwd, ".agents", "ao", "last-session-outcome.json")
	if data, err := os.ReadFile(outcomePath); err == nil {
		var outcome struct {
			Outcome string `json:"outcome"`
		}
		if json.Unmarshal(data, &outcome) == nil {
			switch outcome.Outcome {
			case "success":
				return 0.8, nil
			case "failure":
				return 0.2, nil
			case "abandoned":
				return 0.4, nil
			}
		}
	}

	homeDir, _ := os.UserHomeDir()
	if homeDir == "" {
		return types.InitialUtility, nil
	}
	transcriptsDir := filepath.Join(homeDir, ".claude", "projects")
	transcriptPath := findMostRecentTranscript(transcriptsDir)
	if transcriptPath == "" {
		return types.InitialUtility, nil
	}
	outcome, err := analyzeTranscript(transcriptPath, "")
	if err != nil {
		return types.InitialUtility, nil
	}
	return outcome.Reward, nil
}

// promoteCitedLearnings reads the feedback log and attempts maturity promotion
// on each learning that received citation feedback. This ensures learnings whose
// utility was just bumped by citation feedback get promoted in the same close-loop
// cycle rather than waiting for the next run.
// Returns the number of learnings that transitioned.
func promoteCitedLearnings(cwd string, quiet bool) int {
	if GetDryRun() {
		return 0
	}

	feedbackPath := filepath.Join(cwd, FeedbackFilePath)
	data, err := os.ReadFile(feedbackPath)
	if err != nil {
		return 0
	}

	seen := make(map[string]bool)
	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var evt FeedbackEvent
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			continue
		}
		if evt.Decision != "" && evt.Decision != "rewarded" {
			continue
		}
		if evt.ArtifactPath == "" || seen[evt.ArtifactPath] {
			continue
		}
		seen[evt.ArtifactPath] = true
		paths = append(paths, evt.ArtifactPath)
	}

	promoted := 0
	for _, p := range paths {
		result, err := ratchet.ApplyMaturityTransition(p)
		if err != nil {
			continue
		}
		if result.Transitioned {
			promoted++
			if !quiet {
				fmt.Fprintf(os.Stderr, "  maturity: %s → %s (%s)\n", result.OldMaturity, result.NewMaturity, filepath.Base(p))
			}
		}
	}
	if promoted > 0 && !quiet {
		fmt.Fprintf(os.Stderr, "Promoted %d learnings\n", promoted)
	}
	return promoted
}

// qualitySignalEntry represents a single line from session-quality.jsonl.
type qualitySignalEntry struct {
	Timestamp  string `json:"timestamp"`
	SignalType string `json:"signal_type"`
	Detail     string `json:"detail"`
	SessionID  string `json:"session_id"`
}

// processQualitySignalFeedback reads correction signals from
// .agents/signals/session-quality.jsonl and applies negative utility
// adjustments to skills that were loaded during the given session.
// Returns the number of correction signals found for the session.
func processQualitySignalFeedback(cwd, sessionID string, mutateArtifacts bool) (int, error) {
	signalsPath := filepath.Join(cwd, ".agents", "signals", "session-quality.jsonl")
	f, err := os.Open(signalsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("open quality signals: %w", err)
	}
	defer f.Close()

	// Count correction signals for this session.
	var correctionCount int
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry qualitySignalEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if entry.SessionID == sessionID && entry.SignalType == "correction" {
			correctionCount++
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("read quality signals: %w", err)
	}
	if correctionCount == 0 {
		return 0, nil
	}

	// Compute penalty: -0.1 per correction, capped at -0.3.
	penalty := -0.1 * float64(correctionCount)
	if penalty < -0.3 {
		penalty = -0.3
	}

	// Load citations for this session and filter for skill_loaded entries.
	citations, err := ratchet.LoadCitations(cwd)
	if err != nil {
		return correctionCount, fmt.Errorf("load citations for quality feedback: %w", err)
	}

	res := resolver.NewFileResolver(cwd)
	appliedMutate := mutateArtifacts && !GetDryRun()

	var feedbackEvents []FeedbackEvent
	for _, c := range citations {
		if c.CitationType != "skill_loaded" {
			continue
		}

		learningID := extractLearningID(c.ArtifactPath)
		path, resolveErr := res.Resolve(learningID)
		if resolveErr != nil {
			continue
		}

		if !appliedMutate {
			currentUtility := parseUtilityFromFile(path)
			event := FeedbackEvent{
				SessionID:       sessionID,
				ArtifactPath:    path,
				CitationType:    "skill_loaded",
				MetricNamespace: "quality-signal",
				Decision:        "penalized",
				Reason:          fmt.Sprintf("quality-correction-count-%d", correctionCount),
				Reward:          penalty,
				UtilityBefore:   currentUtility,
				UtilityAfter:    currentUtility,
				Alpha:           0,
				RecordedAt:      time.Now(),
			}
			feedbackEvents = append(feedbackEvents, event)
			continue
		}

		rewardCount := getLearningRewardCount(path)
		alpha := annealedAlpha(types.DefaultAlpha, rewardCount)
		oldUtility, newUtility, updateErr := updateLearningUtility(path, penalty, alpha)
		if updateErr != nil {
			continue
		}

		event := FeedbackEvent{
			SessionID:       sessionID,
			ArtifactPath:    path,
			CitationType:    "skill_loaded",
			MetricNamespace: "quality-signal",
			Decision:        "penalized",
			Reason:          fmt.Sprintf("quality-correction-count-%d", correctionCount),
			Reward:          penalty,
			UtilityBefore:   oldUtility,
			UtilityAfter:    newUtility,
			Alpha:           alpha,
			RecordedAt:      time.Now(),
		}
		feedbackEvents = append(feedbackEvents, event)
	}

	if !GetDryRun() && len(feedbackEvents) > 0 {
		_ = writeFeedbackEvents(cwd, feedbackEvents)
	}

	return correctionCount, nil
}
