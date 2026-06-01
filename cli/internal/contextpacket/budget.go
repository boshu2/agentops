// Package contextpacket provides context budget tracking and progressive summarization.
package contextpacket

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// marshalJSON is the JSON marshalling function used by Save and SaveState.
// It is a package-level variable so tests can inject failures.
var marshalJSON = json.MarshalIndent

// Thresholds for context budget management.
const (
	// OptimalThreshold is the ideal context usage (40%).
	OptimalThreshold = 0.40

	// WarningThreshold triggers auto-batch warnings (60%).
	WarningThreshold = 0.60

	// SummarizationThreshold triggers progressive summarization (80%).
	SummarizationThreshold = 0.80

	// CriticalThreshold requires immediate action (90%).
	CriticalThreshold = 0.90

	// DefaultMaxTokens is the assumed max context window.
	DefaultMaxTokens = 200000
)

// BudgetStatus represents the current budget state.
type BudgetStatus string

const (
	StatusOptimal  BudgetStatus = "OPTIMAL"
	StatusWarning  BudgetStatus = "WARNING"
	StatusCritical BudgetStatus = "CRITICAL"
)

// BudgetTracker tracks context budget across a session.
type BudgetTracker struct {
	// SessionID identifies this session.
	SessionID string `json:"session_id"`

	// MaxTokens is the context window size.
	MaxTokens int `json:"max_tokens"`

	// EstimatedUsage is the current token estimate.
	EstimatedUsage int `json:"estimated_usage"`

	// Checkpoints recorded during the session.
	Checkpoints []Checkpoint `json:"checkpoints"`

	// SummarizationEvents records when summarization occurred.
	SummarizationEvents []SummarizationEvent `json:"summarization_events"`

	// StartedAt is when tracking started.
	StartedAt time.Time `json:"started_at"`

	// LastUpdated is when the tracker was last updated.
	LastUpdated time.Time `json:"last_updated"`
}

// Checkpoint records a point-in-time state.
type Checkpoint struct {
	// ID uniquely identifies this checkpoint.
	ID string `json:"id"`

	// Timestamp of the checkpoint.
	Timestamp time.Time `json:"timestamp"`

	// TokenUsage at checkpoint.
	TokenUsage int `json:"token_usage"`

	// PercentUsage at checkpoint.
	PercentUsage float64 `json:"percent_usage"`

	// Description of what was completed.
	Description string `json:"description"`

	// FilesChanged since last checkpoint.
	FilesChanged []string `json:"files_changed,omitempty"`

	// TestStatus at checkpoint (passing, failing, none).
	TestStatus string `json:"test_status,omitempty"`
}

// SummarizationEvent records when context was summarized.
type SummarizationEvent struct {
	// Timestamp of summarization.
	Timestamp time.Time `json:"timestamp"`

	// TokensBefore usage before summarization.
	TokensBefore int `json:"tokens_before"`

	// TokensAfter usage after summarization.
	TokensAfter int `json:"tokens_after"`

	// TokensSaved by summarization.
	TokensSaved int `json:"tokens_saved"`

	// PreservedContext describes what was kept.
	PreservedContext []string `json:"preserved_context"`
}

// NewBudgetTracker creates a new tracker for a session.
func NewBudgetTracker(sessionID string) *BudgetTracker {
	return &BudgetTracker{
		SessionID:   sessionID,
		MaxTokens:   DefaultMaxTokens,
		StartedAt:   time.Now(),
		LastUpdated: time.Now(),
	}
}

// GetUsagePercent returns the current usage as a percentage.
func (b *BudgetTracker) GetUsagePercent() float64 {
	if b.MaxTokens == 0 {
		return 0
	}
	return float64(b.EstimatedUsage) / float64(b.MaxTokens)
}

// GetStatus returns the current budget status.
func (b *BudgetTracker) GetStatus() BudgetStatus {
	usage := b.GetUsagePercent()
	switch {
	case usage >= SummarizationThreshold:
		return StatusCritical
	case usage >= WarningThreshold:
		return StatusWarning
	default:
		return StatusOptimal
	}
}

// NeedsSummarization returns true if progressive summarization is needed.
func (b *BudgetTracker) NeedsSummarization() bool {
	return b.GetUsagePercent() >= SummarizationThreshold
}

// NeedsCheckpoint returns true if a checkpoint should be created.
func (b *BudgetTracker) NeedsCheckpoint() bool {
	return b.GetUsagePercent() >= WarningThreshold
}

// UpdateUsage updates the estimated token usage.
func (b *BudgetTracker) UpdateUsage(tokens int) {
	b.EstimatedUsage = tokens
	b.LastUpdated = time.Now()
}

// AddTokens adds to the estimated usage.
func (b *BudgetTracker) AddTokens(tokens int) {
	b.EstimatedUsage += tokens
	b.LastUpdated = time.Now()
}

// CreateCheckpoint records the current state.
func (b *BudgetTracker) CreateCheckpoint(id, description string, filesChanged []string, testStatus string) Checkpoint {
	cp := Checkpoint{
		ID:           id,
		Timestamp:    time.Now(),
		TokenUsage:   b.EstimatedUsage,
		PercentUsage: b.GetUsagePercent(),
		Description:  description,
		FilesChanged: filesChanged,
		TestStatus:   testStatus,
	}
	b.Checkpoints = append(b.Checkpoints, cp)
	b.LastUpdated = time.Now()
	return cp
}

// GetLastCheckpoint returns the most recent checkpoint.
func (b *BudgetTracker) GetLastCheckpoint() *Checkpoint {
	if len(b.Checkpoints) == 0 {
		return nil
	}
	return &b.Checkpoints[len(b.Checkpoints)-1]
}

// RecordSummarization records a summarization event.
func (b *BudgetTracker) RecordSummarization(tokensBefore, tokensAfter int, preserved []string) {
	event := SummarizationEvent{
		Timestamp:        time.Now(),
		TokensBefore:     tokensBefore,
		TokensAfter:      tokensAfter,
		TokensSaved:      tokensBefore - tokensAfter,
		PreservedContext: preserved,
	}
	b.SummarizationEvents = append(b.SummarizationEvents, event)
	b.EstimatedUsage = tokensAfter
	b.LastUpdated = time.Now()
}

// GetRecommendation returns advice based on current status.
func (b *BudgetTracker) GetRecommendation() string {
	usage := b.GetUsagePercent()

	switch {
	case usage >= CriticalThreshold:
		return "CRITICAL: Context nearly full. Summarize immediately or start new session."
	case usage >= SummarizationThreshold:
		return "HIGH: Trigger progressive summarization. Preserve file changes and test status."
	case usage >= WarningThreshold:
		return "MEDIUM: Create checkpoint. Consider batching remaining work."
	default:
		return "OPTIMAL: Continue work. Context budget healthy."
	}
}

// Save persists the tracker to disk.
func (b *BudgetTracker) Save(baseDir string) error {
	dir := filepath.Join(baseDir, ".agents", "ao", "context")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	path := filepath.Join(dir, fmt.Sprintf("budget-%s.json", b.SessionID))
	data, err := marshalJSON(b, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// Load loads a tracker from disk.
func Load(baseDir, sessionID string) (*BudgetTracker, error) {
	path := filepath.Join(baseDir, ".agents", "ao", "context", fmt.Sprintf("budget-%s.json", sessionID))

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var b BudgetTracker
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}

	return &b, nil
}

// BudgetReport summarizes budget status.
type BudgetReport struct {
	SessionID          string       `json:"session_id"`
	Status             BudgetStatus `json:"status"`
	UsagePercent       float64      `json:"usage_percent"`
	TokensUsed         int          `json:"tokens_used"`
	TokensRemaining    int          `json:"tokens_remaining"`
	CheckpointCount    int          `json:"checkpoint_count"`
	SummarizationCount int          `json:"summarization_count"`
	Recommendation     string       `json:"recommendation"`
}

// GetReport returns a summary report.
func (b *BudgetTracker) GetReport() BudgetReport {
	return BudgetReport{
		SessionID:          b.SessionID,
		Status:             b.GetStatus(),
		UsagePercent:       b.GetUsagePercent() * 100,
		TokensUsed:         b.EstimatedUsage,
		TokensRemaining:    b.MaxTokens - b.EstimatedUsage,
		CheckpointCount:    len(b.Checkpoints),
		SummarizationCount: len(b.SummarizationEvents),
		Recommendation:     b.GetRecommendation(),
	}
}

// EstimateTokens estimates tokens from text length.
// Uses rough 4 chars per token approximation.
func EstimateTokens(text string) int {
	return len(text) / 4
}

// EstimateFileTokens estimates tokens for reading a file.
func EstimateFileTokens(path string) int {
	info, err := os.Stat(path)
	if err != nil {
		return 1000 // Default estimate
	}
	return int(info.Size()) / 4
}
