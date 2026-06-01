package contextpacket

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// SummaryPriority defines what to preserve during summarization.
type SummaryPriority int

const (
	PriorityCritical SummaryPriority = iota // Always preserve
	PriorityHigh                            // Preserve if space allows
	PriorityMedium                          // Summarize
	PriorityLow                             // Can drop
)

// Context item type constants.
const (
	ItemTypeFailingTest     = "failing_test"
	ItemTypeFileChange      = "file_change"
	ItemTypeCriticalFinding = "critical_finding"
	ItemTypeHighFinding     = "high_finding"
	ItemTypeMediumFinding   = "medium_finding"
	ItemTypeLowFinding      = "low_finding"
	ItemTypeContext         = "context"
	ItemTypeExploration     = "exploration"
)

// ContextItem represents a piece of context to potentially summarize.
type ContextItem struct {
	// Type identifies the item (file_change, test_result, finding, etc).
	Type string `json:"type"`

	// Priority for preservation.
	Priority SummaryPriority `json:"priority"`

	// Content is the full text.
	Content string `json:"content"`

	// Summary is a condensed version.
	Summary string `json:"summary,omitempty"`

	// TokenEstimate for the full content.
	TokenEstimate int `json:"token_estimate"`

	// Metadata for the item.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// SummaryConfig configures the summarizer.
type SummaryConfig struct {
	// TargetUsage is the desired usage after summarization (default: 0.5).
	TargetUsage float64

	// PreserveFailingTests always keeps failing test details.
	PreserveFailingTests bool

	// PreserveFileChanges always keeps file change list.
	PreserveFileChanges bool

	// PreserveCriticalFindings always keeps CRITICAL findings.
	PreserveCriticalFindings bool

	// MaxSummaryLength for individual item summaries.
	MaxSummaryLength int
}

// DefaultSummaryConfig returns sensible defaults.
func DefaultSummaryConfig() SummaryConfig {
	return SummaryConfig{
		TargetUsage:              0.5,
		PreserveFailingTests:     true,
		PreserveFileChanges:      true,
		PreserveCriticalFindings: true,
		MaxSummaryLength:         200,
	}
}

// Summarizer handles progressive summarization.
type Summarizer struct {
	Config  SummaryConfig
	Tracker *BudgetTracker
}

// NewSummarizer creates a summarizer.
func NewSummarizer(tracker *BudgetTracker) *Summarizer {
	return &Summarizer{
		Config:  DefaultSummaryConfig(),
		Tracker: tracker,
	}
}

// SummarizeContext performs progressive summarization.
func (s *Summarizer) SummarizeContext(items []ContextItem) ([]ContextItem, SummarizationEvent) {
	tokensBefore := s.Tracker.EstimatedUsage

	// Calculate target tokens
	targetTokens := int(float64(s.Tracker.MaxTokens) * s.Config.TargetUsage)

	// Sort items by priority (lowest first - these get summarized/dropped first)
	sorted := s.sortByPriority(items)

	// Process items
	var result []ContextItem
	var preserved []string
	currentTokens := 0

	for _, item := range sorted {
		// Critical items always preserved as-is
		if item.Priority == PriorityCritical {
			result = append(result, item)
			currentTokens += item.TokenEstimate
			preserved = append(preserved, item.Type)
			continue
		}

		// Check if we have budget
		remainingBudget := targetTokens - currentTokens

		if item.TokenEstimate <= remainingBudget {
			// Item fits, keep as-is
			result = append(result, item)
			currentTokens += item.TokenEstimate
			preserved = append(preserved, item.Type)
		} else if item.Priority <= PriorityMedium {
			// Try to summarize
			summarized := s.summarizeItem(item, remainingBudget)
			if summarized.TokenEstimate > 0 {
				result = append(result, summarized)
				currentTokens += summarized.TokenEstimate
				preserved = append(preserved, item.Type+" (summarized)")
			}
			// Low priority items can be dropped if no space
		}
		// Low priority items are dropped if they don't fit
	}

	// Create event
	event := SummarizationEvent{
		Timestamp:        time.Now(),
		TokensBefore:     tokensBefore,
		TokensAfter:      currentTokens,
		TokensSaved:      tokensBefore - currentTokens,
		PreservedContext: preserved,
	}

	return result, event
}

// sortByPriority sorts items with highest priority first (lowest Priority value first).
func (s *Summarizer) sortByPriority(items []ContextItem) []ContextItem {
	// Create a copy to avoid modifying original
	sorted := make([]ContextItem, len(items))
	copy(sorted, items)

	// Use standard library sort for O(n log n) performance
	slices.SortFunc(sorted, func(a, b ContextItem) int {
		return cmp.Compare(a.Priority, b.Priority)
	})

	return sorted
}

// summarizeItem creates a summarized version of an item.
func (s *Summarizer) summarizeItem(item ContextItem, maxTokens int) ContextItem {
	// If item already has a summary, use it
	if item.Summary != "" {
		return ContextItem{
			Type:          item.Type,
			Priority:      item.Priority,
			Content:       item.Summary,
			TokenEstimate: EstimateTokens(item.Summary),
			Metadata:      item.Metadata,
		}
	}

	// Create a simple summary by truncating
	maxChars := maxTokens * 4 // Rough token-to-char conversion
	if maxChars > s.Config.MaxSummaryLength*4 {
		maxChars = s.Config.MaxSummaryLength * 4
	}

	summary := item.Content
	if maxChars >= 3 && len(summary) > maxChars {
		summary = summary[:maxChars-3] + "..."
	}

	return ContextItem{
		Type:          item.Type,
		Priority:      item.Priority,
		Content:       summary,
		TokenEstimate: EstimateTokens(summary),
		Metadata:      item.Metadata,
	}
}

// ClassifyItem determines the priority of a context item.
func (s *Summarizer) ClassifyItem(itemType, content string) SummaryPriority {
	base := baseItemPriority(itemType)
	if base == PriorityHigh && s.shouldPreserve(itemType) {
		return PriorityCritical
	}
	return base
}

// baseItemPriority returns the default priority for an item type (without config overrides).
func baseItemPriority(itemType string) SummaryPriority {
	switch itemType {
	case ItemTypeFailingTest, ItemTypeFileChange, ItemTypeCriticalFinding:
		return PriorityHigh
	case ItemTypeHighFinding:
		return PriorityHigh
	case ItemTypeMediumFinding:
		return PriorityMedium
	case ItemTypeLowFinding, ItemTypeContext, ItemTypeExploration:
		return PriorityLow
	default:
		return PriorityMedium
	}
}

// shouldPreserve checks whether the summarizer config escalates the given item type to critical.
func (s *Summarizer) shouldPreserve(itemType string) bool {
	switch itemType {
	case ItemTypeFailingTest:
		return s.Config.PreserveFailingTests
	case ItemTypeFileChange:
		return s.Config.PreserveFileChanges
	case ItemTypeCriticalFinding:
		return s.Config.PreserveCriticalFindings
	default:
		return false
	}
}

// CreateContextItem creates a classified context item.
func (s *Summarizer) CreateContextItem(itemType, content string, metadata map[string]string) ContextItem {
	return ContextItem{
		Type:          itemType,
		Priority:      s.ClassifyItem(itemType, content),
		Content:       content,
		TokenEstimate: EstimateTokens(content),
		Metadata:      metadata,
	}
}

// SummarizeState holds the state for resumption.
type SummarizeState struct {
	// SessionID for this state.
	SessionID string `json:"session_id"`

	// Timestamp when saved.
	Timestamp time.Time `json:"timestamp"`

	// FilesChanged during the session.
	FilesChanged []string `json:"files_changed"`

	// TestStatus at save time.
	TestStatus string `json:"test_status"`

	// FailingTests if any.
	FailingTests []string `json:"failing_tests,omitempty"`

	// CriticalFindings to preserve.
	CriticalFindings []string `json:"critical_findings,omitempty"`

	// CurrentTask being worked on.
	CurrentTask string `json:"current_task,omitempty"`

	// CompletedTasks list.
	CompletedTasks []string `json:"completed_tasks,omitempty"`

	// Notes for resumption.
	Notes string `json:"notes,omitempty"`
}

// SaveState persists state for resumption.
func (s *Summarizer) SaveState(baseDir string, state SummarizeState) error {
	dir := filepath.Join(baseDir, ".agents", "ao", "context")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	path := filepath.Join(dir, fmt.Sprintf("state-%s.json", state.SessionID))
	data, err := marshalJSON(state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// LoadState loads state for resumption.
func LoadState(baseDir, sessionID string) (*SummarizeState, error) {
	path := filepath.Join(baseDir, ".agents", "ao", "context", fmt.Sprintf("state-%s.json", sessionID))

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var state SummarizeState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// GenerateResumptionContext creates context for resuming work.
func (s *Summarizer) GenerateResumptionContext(state SummarizeState) string {
	var b strings.Builder
	b.WriteString("# Session Resumption Context\n\n")
	writeFilesSection(&b, state.FilesChanged)
	writeTestSection(&b, state.TestStatus, state.FailingTests)
	writeBulletSection(&b, "## Critical Findings", state.CriticalFindings)
	if state.CurrentTask != "" {
		b.WriteString("## Current Task\n")
		fmt.Fprintf(&b, "%s\n\n", state.CurrentTask)
	}
	writeChecklistSection(&b, "## Completed Tasks", state.CompletedTasks)
	if state.Notes != "" {
		b.WriteString("## Notes\n")
		b.WriteString(state.Notes)
		b.WriteString("\n")
	}
	return b.String()
}

// writeFilesSection writes the files-changed section.
func writeFilesSection(b *strings.Builder, files []string) {
	b.WriteString("## Files Changed\n")
	if len(files) > 0 {
		for _, f := range files {
			fmt.Fprintf(b, "- %s\n", f)
		}
	} else {
		b.WriteString("No files changed yet.\n")
	}
	b.WriteString("\n")
}

// writeTestSection writes the test-status section including any failing tests.
func writeTestSection(b *strings.Builder, status string, failing []string) {
	b.WriteString("## Test Status\n")
	fmt.Fprintf(b, "%s\n", status)
	if len(failing) > 0 {
		b.WriteString("\nFailing tests:\n")
		for _, t := range failing {
			fmt.Fprintf(b, "- %s\n", t)
		}
	}
	b.WriteString("\n")
}

// writeBulletSection writes an optional section with bullet items.
func writeBulletSection(b *strings.Builder, heading string, items []string) {
	if len(items) == 0 {
		return
	}
	b.WriteString(heading + "\n")
	for _, item := range items {
		fmt.Fprintf(b, "- %s\n", item)
	}
	b.WriteString("\n")
}

// writeChecklistSection writes an optional section with checklist items.
func writeChecklistSection(b *strings.Builder, heading string, items []string) {
	if len(items) == 0 {
		return
	}
	b.WriteString(heading + "\n")
	for _, item := range items {
		fmt.Fprintf(b, "- [x] %s\n", item)
	}
	b.WriteString("\n")
}
