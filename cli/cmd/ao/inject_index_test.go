// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestRenderKnowledgeIndex_Empty(t *testing.T) {
	k := &injectedKnowledge{
		Timestamp: time.Date(2026, 2, 24, 12, 0, 0, 0, time.UTC),
	}
	output := renderKnowledgeIndex(k)
	if !strings.Contains(output, "No prior knowledge found") {
		t.Error("expected 'No prior knowledge found' for empty knowledge")
	}
	if !strings.Contains(output, "ao lookup") {
		t.Error("expected lookup hint in output")
	}
}

func TestRenderKnowledgeIndex_WithLearnings(t *testing.T) {
	k := &injectedKnowledge{
		Learnings: []learning{
			{ID: "learn-001", Title: "Test Learning", AgeWeeks: 0.5, CompositeScore: 0.85},
			{ID: "learn-002", Title: "Another Learning", AgeWeeks: 2.0, CompositeScore: 0.72},
		},
		Timestamp: time.Date(2026, 2, 24, 12, 0, 0, 0, time.UTC),
	}
	output := renderKnowledgeIndex(k)

	// Check table structure
	if !strings.Contains(output, "| ID | Title | Age | Score |") {
		t.Error("expected table header")
	}
	if !strings.Contains(output, "learn-001") {
		t.Error("expected learn-001 in output")
	}
	if !strings.Contains(output, "learn-002") {
		t.Error("expected learn-002 in output")
	}
	// Should NOT contain full summary text
	if strings.Contains(output, "No prior knowledge found") {
		t.Error("should not say 'No prior knowledge found' when learnings exist")
	}
}

func TestRenderKnowledgeIndex_WithPatterns(t *testing.T) {
	k := &injectedKnowledge{
		Patterns: []pattern{
			{Name: "Council Judges", Description: "Multi-model validation", CompositeScore: 0.91},
		},
		Timestamp: time.Date(2026, 2, 24, 12, 0, 0, 0, time.UTC),
	}
	output := renderKnowledgeIndex(k)
	if !strings.Contains(output, "Council Judges") {
		t.Error("expected pattern name in output")
	}
	if !strings.Contains(output, "| Name | Description | Score |") {
		t.Error("expected patterns table header")
	}
}

func TestRenderKnowledgeIndex_TokenBudget(t *testing.T) {
	// Index with 10 learnings + 5 patterns should be under 1500 chars (~375 tokens)
	k := &injectedKnowledge{
		Timestamp: time.Date(2026, 2, 24, 12, 0, 0, 0, time.UTC),
	}
	for i := 0; i < 10; i++ {
		k.Learnings = append(k.Learnings, learning{
			ID: fmt.Sprintf("learn-%03d", i), Title: fmt.Sprintf("Learning number %d", i),
			AgeWeeks: float64(i), CompositeScore: 0.9 - float64(i)*0.05,
		})
	}
	for i := 0; i < 5; i++ {
		k.Patterns = append(k.Patterns, pattern{
			Name: fmt.Sprintf("Pattern-%d", i), Description: fmt.Sprintf("Description %d", i),
			CompositeScore: 0.8 - float64(i)*0.1,
		})
	}
	output := renderKnowledgeIndex(k)
	// Target: under 1500 chars (375 tokens at 4 chars/token)
	if len(output) > 1500 {
		t.Errorf("index output too large: %d chars (target < 1500 for ~375 tokens)", len(output))
	}
}

// formatLookupAge tests are in lookup_test.go (single source of truth).
