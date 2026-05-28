// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/search"
)

// TestIntegration_TwoPhaseInjection tests the full two-phase injection lifecycle:
// Phase 1: ao inject --index-only produces compact table with IDs
// Phase 2: ao lookup retrieves full content by ID
func TestIntegration_TwoPhaseInjection(t *testing.T) {
	tmpDir := t.TempDir()
	learningsDir := filepath.Join(tmpDir, ".agents", "learnings")
	patternsDir := filepath.Join(tmpDir, ".agents", "patterns")
	os.MkdirAll(learningsDir, 0755)
	os.MkdirAll(patternsDir, 0755)

	// Create test learning with known content
	learningContent := `---
id: learn-test-auth
type: learning
created_at: "` + time.Now().Format(time.RFC3339) + `"
category: architecture
confidence: high
maturity: provisional
---
# Learning: Authentication Best Practices

## What We Learned

JWT tokens should be stored in httpOnly cookies, not localStorage.

## Why It Matters

Prevents XSS attacks from stealing auth tokens.
`
	os.WriteFile(filepath.Join(learningsDir, "test-auth-learning.md"), []byte(learningContent), 0644)

	// Create test pattern
	patternContent := `---
name: Council Judges
description: Multi-model validation with independent perspectives
---
# Pattern: Council Judges

Use specialized, independent judges for validation.
`
	os.WriteFile(filepath.Join(patternsDir, "council-judges.md"), []byte(patternContent), 0644)

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Phase 1: Test index-only output
	t.Run("phase1_index_output", func(t *testing.T) {
		knowledge := gatherKnowledge(tmpDir, &search.InjectOptions{}, "test-session", nil)
		indexOutput := renderKnowledgeIndex(knowledge)

		// Should contain table headers
		if !strings.Contains(indexOutput, "| ID | Title | Age | Score |") {
			t.Error("index output missing learnings table header")
		}

		// Should contain our test learning (ID defaults to filename)
		if !strings.Contains(indexOutput, "test-auth-learning.md") {
			t.Error("index output missing test learning filename ID")
		}

		// Should contain lookup instructions
		if !strings.Contains(indexOutput, "ao lookup") {
			t.Error("index output missing lookup instructions")
		}

		// Should be compact (under 1000 chars for 1 learning + 1 pattern)
		if len(indexOutput) > 1000 {
			t.Errorf("index output too large: %d chars (expected < 1000 for 2 items)", len(indexOutput))
		}
	})

	// Phase 2: Test lookup by ID (uses filename-based ID)
	t.Run("phase2_lookup_by_id", func(t *testing.T) {
		oldNoCite := lookupNoCite
		lookupNoCite = true
		defer func() { lookupNoCite = oldNoCite }()

		err := lookupByID(tmpDir, "test-auth-learning", nil)
		// Should find the learning (output goes to stdout, no error)
		if err != nil {
			t.Errorf("lookup by ID failed: %v", err)
		}
	})

	// Phase 2: Test lookup by query
	t.Run("phase2_lookup_by_query", func(t *testing.T) {
		// Set module-level vars for query mode
		oldQuery := lookupQuery
		oldLimit := lookupLimit
		oldNoCite := lookupNoCite
		lookupQuery = "authentication"
		lookupLimit = 3
		lookupNoCite = true
		defer func() {
			lookupQuery = oldQuery
			lookupLimit = oldLimit
			lookupNoCite = oldNoCite
		}()

		err := lookupByQuery(tmpDir, nil)
		if err != nil {
			t.Errorf("lookup by query failed: %v", err)
		}
	})

	// Verify index per-item cost is lower than full per-item cost
	t.Run("index_per_item_cost", func(t *testing.T) {
		// Create additional learnings to get a realistic comparison
		for i := 0; i < 8; i++ {
			content := "---\nid: learn-extra-" + strings.Repeat("x", 1) + "\nmaturity: provisional\n---\n# Learning " + string(rune('A'+i)) + "\n\nSome detailed content about topic " + string(rune('A'+i)) + " that takes up space in the full output but not in the index table.\n"
			os.WriteFile(filepath.Join(learningsDir, "extra-"+string(rune('a'+i))+".md"), []byte(content), 0644)
		}

		knowledge := gatherKnowledge(tmpDir, &search.InjectOptions{}, "test-session-2", nil)
		indexOutput := renderKnowledgeIndex(knowledge)
		fullOutput := formatKnowledgeMarkdown(knowledge)

		if len(indexOutput) >= len(fullOutput) {
			t.Errorf("index (%d chars) should be smaller than full output (%d chars) with %d learnings",
				len(indexOutput), len(fullOutput), len(knowledge.Learnings))
		}
	})
}

// TestIntegration_LookupNotFound verifies error handling for missing artifacts.
func TestIntegration_LookupNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".agents", "learnings"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, ".agents", "patterns"), 0755)

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	err := lookupByID(tmpDir, "nonexistent-learning", nil)
	if err == nil {
		t.Error("expected error for nonexistent ID")
	}
	if !strings.Contains(err.Error(), "no artifact found") {
		t.Errorf("unexpected error message: %v", err)
	}
}
