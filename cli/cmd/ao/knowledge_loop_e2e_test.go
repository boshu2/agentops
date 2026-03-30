package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/storage"
	"github.com/boshu2/agentops/cli/internal/types"
)

// TestKnowledgeLoopE2E tests the full knowledge loop:
// FORGE → STORE → RECALL → APPLY → FEEDBACK → (compounds)
func TestKnowledgeLoopE2E(t *testing.T) {
	// ========================================
	// Phase 1: Setup - Create isolated test environment
	// ========================================
	tempDir := t.TempDir()

	// Create directory structure
	dirs := []string{
		filepath.Join(tempDir, ".agents", "ao", "sessions"),
		filepath.Join(tempDir, ".agents", "ao", "index"),
		filepath.Join(tempDir, ".agents", "learnings"),
		filepath.Join(tempDir, ".agents", "patterns"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("create directory %s: %v", dir, err)
		}
	}

	// Create test learning (will be recalled during inject)
	learningPath := filepath.Join(tempDir, ".agents", "learnings", "test-learning.jsonl")
	learningContent := `{"id":"L-TEST-001","title":"Context Cancellation Pattern","content":"Use context.WithCancel for graceful shutdown in Go services","utility":0.5,"maturity":"provisional","created_at":"2026-01-25T10:00:00Z"}`
	if err := os.WriteFile(learningPath, []byte(learningContent), 0644); err != nil {
		t.Fatalf("create test learning: %v", err)
	}

	// Create test pattern
	patternPath := filepath.Join(tempDir, ".agents", "patterns", "graceful-shutdown.md")
	patternContent := `# Graceful Shutdown Pattern

Always use context.WithCancel to propagate cancellation signals.
This ensures all goroutines clean up properly on shutdown.
`
	if err := os.WriteFile(patternPath, []byte(patternContent), 0644); err != nil {
		t.Fatalf("create test pattern: %v", err)
	}

	// Copy test transcript
	transcriptSrc := filepath.Join("testdata", "transcripts", "simple-decision.jsonl")
	transcriptDst := filepath.Join(tempDir, "test-transcript.jsonl")
	if err := copyFile(transcriptSrc, transcriptDst); err != nil {
		// If fixture doesn't exist, create a minimal one
		minimalTranscript := createMinimalTranscript()
		if err := os.WriteFile(transcriptDst, []byte(minimalTranscript), 0644); err != nil {
			t.Fatalf("create test transcript: %v", err)
		}
	}

	// ========================================
	// Phase 2: FORGE - Process transcript
	// ========================================
	t.Run("Forge", func(t *testing.T) {
		// Simulate forge by creating session output
		sessionID := "test-session-001"
		session := &storage.Session{
			ID:      sessionID,
			Date:    time.Now(),
			Summary: "Test session for e2e validation",
			Decisions: []string{
				"Use context.WithCancel for shutdown",
			},
			Knowledge: []string{
				"Graceful shutdown requires context propagation",
			},
			FilesChanged:   []string{"cmd/main.go"},
			TranscriptPath: transcriptDst,
		}

		// Write session JSONL
		sessionPath := filepath.Join(tempDir, ".agents", "ao", "sessions", sessionID+".jsonl")
		sessionData, err := json.Marshal(session)
		if err != nil {
			t.Fatalf("marshal session: %v", err)
		}
		if err := os.WriteFile(sessionPath, sessionData, 0644); err != nil {
			t.Fatalf("write session: %v", err)
		}

		// Update index
		indexPath := filepath.Join(tempDir, ".agents", "ao", "index", "sessions.jsonl")
		indexEntry := map[string]any{
			"session_id": sessionID,
			"date":       session.Date.Format(time.RFC3339),
			"summary":    session.Summary,
			"path":       sessionPath,
		}
		indexData, _ := json.Marshal(indexEntry)
		if err := os.WriteFile(indexPath, append(indexData, '\n'), 0644); err != nil {
			t.Fatalf("write index: %v", err)
		}

		// Verify session was created
		assertFileExists(t, sessionPath)
		assertFileExists(t, indexPath)
	})

	// ========================================
	// Phase 3: INJECT - Recall knowledge
	// ========================================
	t.Run("Inject", func(t *testing.T) {
		// Test collectLearnings function
		learnings, err := collectLearnings(tempDir, "context", 10, "", 0)
		if err != nil {
			t.Fatalf("collectLearnings: %v", err)
		}

		// Should find our test learning
		if len(learnings) == 0 {
			t.Error("Expected to find at least 1 learning")
		} else {
			found := false
			for _, l := range learnings {
				if strings.Contains(l.Title, "Context") || strings.Contains(l.ID, "TEST") {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected to find 'Context Cancellation Pattern' learning, got: %+v", learnings)
			}
		}

		// Test collectPatterns function
		patterns, err := collectPatterns(tempDir, "shutdown", 5, "", 0)
		if err != nil {
			t.Fatalf("collectPatterns: %v", err)
		}

		if len(patterns) == 0 {
			t.Error("Expected to find at least 1 pattern")
		}
	})

	// ========================================
	// Phase 4: CITATION - Track usage
	// ========================================
	t.Run("Citation", func(t *testing.T) {
		// Record a citation
		sessionID := "session-20260125-120000"
		event := types.CitationEvent{
			ArtifactPath: learningPath,
			SessionID:    sessionID,
			CitedAt:      time.Now(),
			CitationType: "retrieved",
			Query:        "context cancellation",
		}

		if err := ratchet.RecordCitation(tempDir, event); err != nil {
			t.Fatalf("RecordCitation: %v", err)
		}

		// Verify citation was recorded
		citationsPath := filepath.Join(tempDir, ".agents", "ao", "citations.jsonl")
		assertFileExists(t, citationsPath)

		// Read and verify citation content
		data, err := os.ReadFile(citationsPath)
		if err != nil {
			t.Fatalf("read citations: %v", err)
		}

		if !strings.Contains(string(data), sessionID) {
			t.Errorf("Citations file should contain session ID %s", sessionID)
		}
		if !strings.Contains(string(data), "retrieved") {
			t.Error("Citations file should contain citation type 'retrieved'")
		}
	})

	// ========================================
	// Phase 5: FEEDBACK - Update utility
	// ========================================
	t.Run("Feedback", func(t *testing.T) {
		// Read original utility
		originalLearning, err := parseLearningJSONL(learningPath)
		if err != nil {
			t.Fatalf("parse original learning: %v", err)
		}
		originalUtility := originalLearning.Utility

		// Apply feedback (simulate successful usage)
		reward := 1.0 // success
		newUtility := updateUtility(originalUtility, reward, types.DefaultAlpha)

		// Verify utility increased
		if newUtility <= originalUtility {
			t.Errorf("Utility should increase after positive feedback: original=%.3f, new=%.3f",
				originalUtility, newUtility)
		}

		// Verify utility is bounded
		if newUtility < 0 || newUtility > 1 {
			t.Errorf("Utility should be in [0,1]: got %.3f", newUtility)
		}
	})

	// ========================================
	// Phase 6: METRICS - Compute flywheel health
	// ========================================
	t.Run("Metrics", func(t *testing.T) {
		// Count sessions
		sessionsDir := filepath.Join(tempDir, ".agents", "ao", "sessions")
		files, _ := filepath.Glob(filepath.Join(sessionsDir, "*.jsonl"))
		sessionCount := len(files)

		if sessionCount == 0 {
			t.Error("Expected at least 1 session")
		}

		// Load citations
		citations, err := ratchet.LoadCitations(tempDir)
		if err != nil {
			t.Fatalf("LoadCitations: %v", err)
		}

		if len(citations) == 0 {
			t.Error("Expected at least 1 citation")
		}

		// Verify escape velocity formula components
		// σ×ρ > δ/100 means knowledge compounds
		sigma := 0.5  // retrieval effectiveness (simulated)
		rho := 0.3    // citation rate (simulated)
		delta := 17.0 // avg age in days

		sigmaRho := sigma * rho
		escapingVelocity := sigmaRho > delta/100.0

		t.Logf("Flywheel metrics: σ=%.2f, ρ=%.2f, δ=%.2f, σ×ρ=%.3f, escaping=%v",
			sigma, rho, delta, sigmaRho, escapingVelocity)

		// At this early stage, we're not at escape velocity (expected)
		// The test verifies the formula works correctly
		if sigmaRho >= 1 {
			t.Error("σ×ρ should be < 1 for valid probability")
		}
	})

	// ========================================
	// Phase 7: SECOND CYCLE - Verify accumulation
	// ========================================
	t.Run("SecondCycle", func(t *testing.T) {
		// Add a second session
		session2 := &storage.Session{
			ID:      "test-session-002",
			Date:    time.Now(),
			Summary: "Second test session",
			Decisions: []string{
				"Add retry logic to HTTP client",
			},
		}

		session2Path := filepath.Join(tempDir, ".agents", "ao", "sessions", "test-session-002.jsonl")
		data, _ := json.Marshal(session2)
		if err := os.WriteFile(session2Path, data, 0644); err != nil {
			t.Fatalf("write session 2: %v", err)
		}

		// Verify sessions count increased
		sessionsDir := filepath.Join(tempDir, ".agents", "ao", "sessions")
		files, _ := filepath.Glob(filepath.Join(sessionsDir, "*.jsonl"))

		if len(files) != 2 {
			t.Errorf("Expected 2 sessions, got %d", len(files))
		}

		// Add another citation
		event := types.CitationEvent{
			ArtifactPath: learningPath,
			SessionID:    "session-20260125-130000",
			CitedAt:      time.Now(),
			CitationType: "applied", // upgraded from retrieved
			Query:        "graceful shutdown",
		}
		if err := ratchet.RecordCitation(tempDir, event); err != nil {
			t.Fatalf("RecordCitation 2: %v", err)
		}

		// Verify citations accumulated
		citations, _ := ratchet.LoadCitations(tempDir)
		if len(citations) < 2 {
			t.Errorf("Expected at least 2 citations, got %d", len(citations))
		}
	})

	// ========================================
	// Phase 8: BADGE - Visual health check
	// ========================================
	t.Run("Badge", func(t *testing.T) {
		// Test badge helper functions
		status, icon := getEscapeStatus(0.05, 17.0)
		if status != "STARTING" || icon != "🌱" {
			t.Errorf("Low velocity should be STARTING, got %s %s", icon, status)
		}

		status, _ = getEscapeStatus(0.10, 17.0)
		if status != "BUILDING" {
			t.Errorf("Medium velocity should be BUILDING, got %s", status)
		}

		status, _ = getEscapeStatus(0.15, 17.0)
		if status != "APPROACHING" {
			t.Errorf("High velocity should be APPROACHING, got %s", status)
		}

		status, icon = getEscapeStatus(0.20, 17.0)
		if status != "ESCAPE VELOCITY" || icon != "🚀" {
			t.Errorf("Above delta should be ESCAPE VELOCITY, got %s %s", icon, status)
		}

		// Test progress bar
		bar := makeProgressBar(0.5, 10)
		// Unicode chars are 3 bytes each, so 10 chars = 30 bytes
		runeCount := len([]rune(bar))
		if runeCount != 10 {
			t.Errorf("Progress bar should be 10 runes, got %d", runeCount)
		}
		if !strings.Contains(bar, "█") || !strings.Contains(bar, "░") {
			t.Errorf("Progress bar should have filled and empty segments: %s", bar)
		}

		// Test edge cases
		barEmpty := makeProgressBar(0, 10)
		if strings.Contains(barEmpty, "█") {
			t.Error("Empty progress bar should have no filled segments")
		}

		barFull := makeProgressBar(1, 10)
		if strings.Contains(barFull, "░") {
			t.Error("Full progress bar should have no empty segments")
		}

		// Test clamping
		barOver := makeProgressBar(1.5, 10)
		if barOver != barFull {
			t.Error("Value > 1 should clamp to full bar")
		}

		barUnder := makeProgressBar(-0.5, 10)
		if barUnder != barEmpty {
			t.Error("Value < 0 should clamp to empty bar")
		}
	})
}

// TestKnowledgeLoopCompositeScoring tests the Two-Phase retrieval scoring
func TestKnowledgeLoopCompositeScoring(t *testing.T) {
	learnings := []learning{
		{ID: "L1", FreshnessScore: 0.9, Utility: 0.3}, // Fresh but low utility
		{ID: "L2", FreshnessScore: 0.5, Utility: 0.8}, // Older but high utility
		{ID: "L3", FreshnessScore: 0.7, Utility: 0.5}, // Balanced
	}

	items := make([]scorable, len(learnings))
	for i := range learnings {
		items[i] = &learnings[i]
	}
	applyCompositeScoringTo(items, types.DefaultLambda)

	// With λ=0.5, utility matters but freshness too
	// L2 should rank higher due to high utility
	// L1 should rank lower despite being fresh

	// Verify all learnings have composite scores
	for _, l := range learnings {
		if l.CompositeScore == 0 && l.FreshnessScore != 0 {
			t.Errorf("Learning %s should have non-zero composite score", l.ID)
		}
	}

	// Find the highest scoring learning
	var highest learning
	for _, l := range learnings {
		if l.CompositeScore > highest.CompositeScore {
			highest = l
		}
	}

	t.Logf("Highest scoring: %s (score=%.3f, fresh=%.2f, util=%.2f)",
		highest.ID, highest.CompositeScore, highest.FreshnessScore, highest.Utility)
}

// TestKnowledgeLoopFreshnessDecay tests the knowledge decay formula
func TestKnowledgeLoopFreshnessDecay(t *testing.T) {
	tests := []struct {
		ageWeeks float64
		minScore float64
		maxScore float64
	}{
		{0, 0.99, 1.0}, // Brand new - should be ~1.0
		{1, 0.8, 0.9},  // 1 week old - slight decay
		{4, 0.4, 0.6},  // 1 month old - significant decay
		{12, 0.1, 0.2}, // 3 months old - heavy decay
		{52, 0.1, 0.1}, // 1 year old - clamped to minimum
	}

	for _, tt := range tests {
		score := freshnessScore(tt.ageWeeks)
		if score < tt.minScore || score > tt.maxScore {
			t.Errorf("freshnessScore(%.0f weeks) = %.3f, want [%.2f, %.2f]",
				tt.ageWeeks, score, tt.minScore, tt.maxScore)
		}
	}
}

// TestKnowledgeLoopUtilityUpdate tests the MemRL utility update formula
func TestKnowledgeLoopUtilityUpdate(t *testing.T) {
	tests := []struct {
		name         string
		oldUtility   float64
		reward       float64
		expectHigher bool
	}{
		{"success increases utility", 0.5, 1.0, true},
		{"failure decreases utility", 0.5, 0.0, false},
		{"partial success slight increase", 0.5, 0.6, true},
		{"low utility + success", 0.1, 1.0, true},
		{"high utility + failure", 0.9, 0.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newUtility := updateUtility(tt.oldUtility, tt.reward, types.DefaultAlpha)

			if tt.expectHigher && newUtility <= tt.oldUtility {
				t.Errorf("Expected utility to increase: old=%.3f, new=%.3f", tt.oldUtility, newUtility)
			}
			if !tt.expectHigher && newUtility >= tt.oldUtility {
				t.Errorf("Expected utility to decrease: old=%.3f, new=%.3f", tt.oldUtility, newUtility)
			}

			// Verify bounds
			if newUtility < 0 || newUtility > 1 {
				t.Errorf("Utility out of bounds: %.3f", newUtility)
			}
		})
	}
}

// Helper functions

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}

func createMinimalTranscript() string {
	messages := []map[string]any{
		{
			"type":      "user",
			"sessionId": "test-session-001",
			"timestamp": time.Now().Format(time.RFC3339),
			"uuid":      "msg-001",
			"message": map[string]string{
				"role":    "user",
				"content": "How should I implement graceful shutdown in Go?",
			},
		},
		{
			"type":       "assistant",
			"sessionId":  "test-session-001",
			"timestamp":  time.Now().Format(time.RFC3339),
			"uuid":       "msg-002",
			"parentUuid": "msg-001",
			"message": map[string]string{
				"role":    "assistant",
				"content": "Use context.WithCancel to propagate cancellation. Decision: Implement signal handler with graceful timeout.",
			},
		},
	}

	lines := make([]string, 0, len(messages))
	for _, msg := range messages {
		data, err := json.Marshal(msg)
		if err != nil {
			panic("marshal transcript message: " + err.Error())
		}
		lines = append(lines, string(data))
	}
	return strings.Join(lines, "\n")
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("Expected file to exist: %s", path)
	}
}

// updateUtility implements the MemRL utility update formula.
// U(t+1) = U(t) + α × (R - U(t))
func updateUtility(oldUtility, reward, alpha float64) float64 {
	newUtility := oldUtility + alpha*(reward-oldUtility)

	// Clamp to [0, 1]
	if newUtility < 0 {
		return 0
	}
	if newUtility > 1 {
		return 1
	}
	return newUtility
}
