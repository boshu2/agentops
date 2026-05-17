package lifecycle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

// FeedbackLoopEvent records a feedback loop closure event.
type FeedbackLoopEvent struct {
	SessionID       string    `json:"session_id"`
	ArtifactPath    string    `json:"artifact_path"`
	ArtifactKind    string    `json:"artifact_kind,omitempty"`
	WorkspacePath   string    `json:"workspace_path,omitempty"`
	CitationType    string    `json:"citation_type,omitempty"`
	MetricNamespace string    `json:"metric_namespace,omitempty"`
	Reward          float64   `json:"reward"`
	UtilityBefore   float64   `json:"utility_before"`
	UtilityAfter    float64   `json:"utility_after"`
	Alpha           float64   `json:"alpha"`
	RecordedAt      time.Time `json:"recorded_at"`
	TranscriptPath  string    `json:"transcript_path,omitempty"`
	Decision        string    `json:"decision,omitempty"`
	Reason          string    `json:"reason,omitempty"`
}

// FeedbackLoopFilePath is the relative path to the feedback log.
const FeedbackLoopFilePath = ".agents/ao/feedback.jsonl"

// ValidFeedbackLoopCitationTypes is the set of allowed citation type filter values.
var ValidFeedbackLoopCitationTypes = map[string]bool{
	"retrieved": true,
	"applied":   true,
	"all":       true,
}

// ValidateFeedbackLoopCitationType checks that citationType is an allowed value.
func ValidateFeedbackLoopCitationType(citationType string) (string, error) {
	candidate := strings.TrimSpace(citationType)
	if ValidFeedbackLoopCitationTypes[candidate] {
		return candidate, nil
	}
	return "", fmt.Errorf("invalid --citation-type %q (valid: retrieved, applied, all)", candidate)
}

// ResolveFeedbackLoopSessionID resolves the session ID from flag or env.
func ResolveFeedbackLoopSessionID(sessionFlag string) (string, error) {
	candidate := strings.TrimSpace(sessionFlag)
	if candidate == "" {
		candidate = strings.TrimSpace(os.Getenv("CLAUDE_SESSION_ID"))
	}
	if candidate == "" {
		return "", fmt.Errorf("--session is required (or set CLAUDE_SESSION_ID)")
	}
	return candidate, nil
}

// ResolveFeedbackLoopReward determines whether the flag reward is valid, or signals transcript computation.
func ResolveFeedbackLoopReward(flagReward float64) (float64, bool) {
	if flagReward >= 0 && flagReward <= 1 {
		return flagReward, true
	}
	return -1, false
}

// LoadFeedbackLoopEvents reads all feedback events from the log.
func LoadFeedbackLoopEvents(baseDir string) ([]FeedbackLoopEvent, error) {
	feedbackPath := filepath.Join(baseDir, FeedbackLoopFilePath)

	f, err := os.Open(feedbackPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }() //nolint:errcheck // read-only file, Close error non-actionable

	var events []FeedbackLoopEvent
	decoder := json.NewDecoder(f)
	for decoder.More() {
		var event FeedbackLoopEvent
		if err := decoder.Decode(&event); err != nil {
			continue // Skip malformed lines
		}
		events = append(events, event)
	}

	return events, nil
}

// ValidateBatchFeedbackFlags checks the batch-feedback command flags.
func ValidateBatchFeedbackFlags(maxSessions int, reward float64, maxRuntime time.Duration) error {
	if maxSessions < 0 {
		return fmt.Errorf("--max-sessions must be >= 0")
	}
	if reward != -1 && (reward < 0 || reward > 1) {
		return fmt.Errorf("--reward must be between 0.0 and 1.0, or -1 to auto-compute")
	}
	if maxRuntime < 0 {
		return fmt.Errorf("--max-runtime must be >= 0")
	}
	return nil
}

// SortAndCapSessions sorts session IDs by latest citation (newest first) and
// caps the list by maxSessions. Pass 0 for maxSessions to disable the cap.
func SortAndCapSessions(sessionCitations map[string][]types.CitationEvent, sessionLatestCitation map[string]time.Time, maxSessions int) []string {
	sessionIDs := make([]string, 0, len(sessionCitations))
	for sessionID := range sessionCitations {
		sessionIDs = append(sessionIDs, sessionID)
	}
	// Sort newest-first by latest citation, break ties lexically.
	for i := 0; i < len(sessionIDs); i++ {
		for j := i + 1; j < len(sessionIDs); j++ {
			ta := sessionLatestCitation[sessionIDs[i]]
			tb := sessionLatestCitation[sessionIDs[j]]
			if tb.After(ta) || (tb.Equal(ta) && sessionIDs[j] < sessionIDs[i]) {
				sessionIDs[i], sessionIDs[j] = sessionIDs[j], sessionIDs[i]
			}
		}
	}
	if maxSessions > 0 && len(sessionIDs) > maxSessions {
		sessionIDs = sessionIDs[:maxSessions]
	}
	return sessionIDs
}

// FeedbackLoopSummary holds the computed summary values for display.
type FeedbackLoopSummary struct {
	SessionID      string
	Reward         float64
	TotalCitations int
	UniqueCount    int
	UpdatedCount   int
	FailedCount    int
}

// FormatFeedbackLoopSummaryText returns the human-readable summary string.
func FormatFeedbackLoopSummaryText(s FeedbackLoopSummary) string {
	text := "Feedback Loop Complete\n"
	text += "======================\n"
	text += fmt.Sprintf("Session:     %s\n", s.SessionID)
	text += fmt.Sprintf("Reward:      %.2f\n", s.Reward)
	text += fmt.Sprintf("Citations:   %d (%d unique)\n", s.TotalCitations, s.UniqueCount)
	text += fmt.Sprintf("Updated:     %d\n", s.UpdatedCount)
	if s.FailedCount > 0 {
		text += fmt.Sprintf("Failed:      %d\n", s.FailedCount)
	}
	return text
}

// WriteFeedbackLoopEvents appends feedback events to the feedback log.
func WriteFeedbackLoopEvents(baseDir string, events []FeedbackLoopEvent) error {
	if len(events) == 0 {
		return nil
	}

	feedbackPath := filepath.Join(baseDir, FeedbackLoopFilePath)

	if err := os.MkdirAll(filepath.Dir(feedbackPath), 0750); err != nil {
		return fmt.Errorf("create feedback directory: %w", err)
	}

	f, err := os.OpenFile(feedbackPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open feedback file: %w", err)
	}
	defer func() { _ = f.Close() }() //nolint:errcheck // write-only file, Close error non-actionable

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
