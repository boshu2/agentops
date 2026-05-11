// practices: [agile-manifesto, dora-metrics]
package main

import (
	"strings"
)

// NOTE: estimateTokens is defined in context.go, InjectCharsPerToken in inject.go — reused here.

// truncateToTokenBudget truncates text to fit within a token budget.
// Finds a safe boundary at the last sentence ending (. ! ?) before the budget.
// Falls back to character boundary with "..." if no sentence boundary found.
func truncateToTokenBudget(text string, maxTokens int) string {
	if maxTokens <= 0 || estimateTokens(text) <= maxTokens {
		return text
	}

	// Convert token budget to approximate character limit
	charLimit := maxTokens * InjectCharsPerToken
	if charLimit >= len(text) {
		return text
	}

	candidate := text[:charLimit]

	// Find last sentence boundary
	lastDot := strings.LastIndexAny(candidate, ".!?")
	if lastDot > 0 && lastDot > charLimit/2 {
		// Only use sentence boundary if it preserves at least half the budget
		return candidate[:lastDot+1]
	}

	// No good sentence boundary — truncate at char limit
	return candidate + "..."
}

// contextBudgetResult reports what was kept vs truncated.
type contextBudgetResult struct {
	OriginalTokens  int  `json:"original_tokens"`
	BudgetTokens    int  `json:"budget_tokens"`
	TruncatedTokens int  `json:"truncated_tokens"`
	WasTruncated    bool `json:"was_truncated"`
}

// applyContextBudget enforces a token budget on assembled context.
// Returns the (possibly truncated) context and a report of what happened.
// Zero budget means no truncation (pass-through).
func applyContextBudget(context string, maxTokens int) (string, contextBudgetResult) {
	original := estimateTokens(context)
	result := contextBudgetResult{
		OriginalTokens: original,
		BudgetTokens:   maxTokens,
	}

	if maxTokens <= 0 || original <= maxTokens {
		return context, result
	}

	truncated := truncateToTokenBudget(context, maxTokens)
	result.TruncatedTokens = original - estimateTokens(truncated)
	result.WasTruncated = true
	return truncated, result
}
