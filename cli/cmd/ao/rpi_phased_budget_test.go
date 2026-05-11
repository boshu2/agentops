// practices: [agile-manifesto, dora-metrics]
package main

import (
	"strings"
	"testing"
)

func TestEstimateTokens_ShortText(t *testing.T) {
	tokens := estimateTokens("hello")
	// "hello" = 5 chars, (5+3)/4 = 2 tokens
	if tokens < 1 || tokens > 3 {
		t.Errorf("estimateTokens(\"hello\") = %d, want ~2", tokens)
	}
}

func TestEstimateTokens_LongText(t *testing.T) {
	text := strings.Repeat("abcd", 1000) // 4000 chars
	tokens := estimateTokens(text)
	// 4000 chars / 4 = ~1000 tokens
	if tokens < 900 || tokens > 1100 {
		t.Errorf("estimateTokens(4000 chars) = %d, want ~1000", tokens)
	}
}

func TestEstimateTokens_Empty(t *testing.T) {
	tokens := estimateTokens("")
	if tokens != 0 {
		t.Errorf("estimateTokens(\"\") = %d, want 0", tokens)
	}
}

func TestTruncateToTokenBudget_UnderBudget(t *testing.T) {
	text := "This is a short text."
	result := truncateToTokenBudget(text, 1000)
	if result != text {
		t.Errorf("expected text unchanged, got %q", result)
	}
}

func TestTruncateToTokenBudget_OverBudget(t *testing.T) {
	// Create text that exceeds budget. 400 chars = ~100 tokens.
	text := "First sentence. " + strings.Repeat("More content here. ", 25)
	result := truncateToTokenBudget(text, 10) // 10 tokens = ~40 chars
	if len(result) >= len(text) {
		t.Errorf("expected truncation, got same length %d", len(result))
	}
	// Should find a sentence boundary or add "..."
	if !strings.HasSuffix(result, ".") && !strings.HasSuffix(result, "...") {
		t.Errorf("expected sentence boundary or ..., got suffix %q", result[len(result)-5:])
	}
}

func TestTruncateToTokenBudget_ZeroBudget(t *testing.T) {
	text := "No truncation expected."
	result := truncateToTokenBudget(text, 0)
	if result != text {
		t.Errorf("zero budget should pass through, got %q", result)
	}
}

func TestApplyContextBudget_Reports(t *testing.T) {
	text := strings.Repeat("word ", 500) // ~2500 chars = ~625 tokens
	result, info := applyContextBudget(text, 100)

	if !info.WasTruncated {
		t.Error("expected WasTruncated=true")
	}
	if info.OriginalTokens < 500 {
		t.Errorf("OriginalTokens = %d, expected >= 500", info.OriginalTokens)
	}
	if info.TruncatedTokens <= 0 {
		t.Errorf("TruncatedTokens = %d, expected > 0", info.TruncatedTokens)
	}
	if len(result) >= len(text) {
		t.Error("expected result to be shorter than input")
	}
}

func TestApplyContextBudget_ZeroBudget(t *testing.T) {
	text := "Pass through."
	result, info := applyContextBudget(text, 0)
	if result != text {
		t.Errorf("zero budget should pass through, got %q", result)
	}
	if info.WasTruncated {
		t.Error("expected WasTruncated=false for zero budget")
	}
}

func TestEstimateTokens_UsesCharsPerTokenConstant(t *testing.T) {
	// Verify estimateTokens uses InjectCharsPerToken, not a hardcoded value.
	text := strings.Repeat("a", InjectCharsPerToken*10)
	tokens := estimateTokens(text)
	if tokens != 10 {
		t.Errorf("estimateTokens(%d chars) = %d, want %d (InjectCharsPerToken=%d)",
			len(text), tokens, 10, InjectCharsPerToken)
	}
}

func TestTruncateToTokenBudget_UsesCharsPerTokenConstant(t *testing.T) {
	// Verify truncateToTokenBudget converts tokens→chars via InjectCharsPerToken.
	// 10 tokens * InjectCharsPerToken chars = the char budget.
	text := strings.Repeat("x", InjectCharsPerToken*20) // 20 tokens worth
	result := truncateToTokenBudget(text, 10)           // 10 token budget
	// Expected char limit = 10 * InjectCharsPerToken
	expectedMaxLen := 10*InjectCharsPerToken + 3 // +3 for possible "..."
	if len(result) > expectedMaxLen {
		t.Errorf("truncated len=%d, expected <= %d (10 tokens * %d chars/token + 3)",
			len(result), expectedMaxLen, InjectCharsPerToken)
	}
}

func TestApplyContextBudget_UnderBudget(t *testing.T) {
	text := "Short text."
	result, info := applyContextBudget(text, 10000)
	if result != text {
		t.Errorf("under-budget text should pass through, got %q", result)
	}
	if info.WasTruncated {
		t.Error("expected WasTruncated=false when under budget")
	}
}
