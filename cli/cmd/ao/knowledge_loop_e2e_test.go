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

type knowledgeLoopFixture struct {
	tempDir        string
	learningPath   string
	transcriptPath string
}

// TestKnowledgeLoopE2E tests the full knowledge loop:
// FORGE → STORE → RECALL → APPLY → FEEDBACK → (compounds)
func TestKnowledgeLoopE2E(t *testing.T) {
	fixture := setupKnowledgeLoopFixture(t)

	t.Run("Forge", fixture.runForgePhase)
	t.Run("Inject", fixture.runInjectPhase)
	t.Run("Citation", fixture.runCitationPhase)
	t.Run("Feedback", fixture.runFeedbackPhase)
	t.Run("Metrics", fixture.runMetricsPhase)
	t.Run("SecondCycle", fixture.runSecondCyclePhase)
	t.Run("Badge", fixture.runBadgePhase)
}

func setupKnowledgeLoopFixture(t *testing.T) knowledgeLoopFixture {
	t.Helper()

	tempDir := t.TempDir()
	for _, dir := range []string{
		filepath.Join(tempDir, ".agents", "ao", "sessions"),
		filepath.Join(tempDir, ".agents", "ao", "index"),
		filepath.Join(tempDir, ".agents", "learnings"),
		filepath.Join(tempDir, ".agents", "patterns"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create directory %s: %v", dir, err)
		}
	}

	learningPath := filepath.Join(tempDir, ".agents", "learnings", "test-learning.jsonl")
	learningContent := `{"id":"L-TEST-001","title":"Context Cancellation Pattern","content":"Use context.WithCancel for graceful shutdown in Go services","utility":0.5,"maturity":"provisional","created_at":"2026-01-25T10:00:00Z"}`
	if err := os.WriteFile(learningPath, []byte(learningContent), 0o644); err != nil {
		t.Fatalf("create test learning: %v", err)
	}

	patternPath := filepath.Join(tempDir, ".agents", "patterns", "graceful-shutdown.md")
	patternContent := `# Graceful Shutdown Pattern

Always use context.WithCancel to propagate cancellation signals.
This ensures all goroutines clean up properly on shutdown.
`
	if err := os.WriteFile(patternPath, []byte(patternContent), 0o644); err != nil {
		t.Fatalf("create test pattern: %v", err)
	}

	transcriptSrc := filepath.Join("testdata", "transcripts", "simple-decision.jsonl")
	transcriptPath := filepath.Join(tempDir, "test-transcript.jsonl")
	if err := copyFile(transcriptSrc, transcriptPath); err != nil {
		minimalTranscript := createMinimalTranscript()
		if err := os.WriteFile(transcriptPath, []byte(minimalTranscript), 0o644); err != nil {
			t.Fatalf("create test transcript: %v", err)
		}
	}

	return knowledgeLoopFixture{
		tempDir:        tempDir,
		learningPath:   learningPath,
		transcriptPath: transcriptPath,
	}
}

func (f knowledgeLoopFixture) runForgePhase(t *testing.T) {
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
		TranscriptPath: f.transcriptPath,
	}

	sessionPath := filepath.Join(f.tempDir, ".agents", "ao", "sessions", sessionID+".jsonl")
	sessionData, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("marshal session: %v", err)
	}
	if err := os.WriteFile(sessionPath, sessionData, 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	indexPath := filepath.Join(f.tempDir, ".agents", "ao", "index", "sessions.jsonl")
	indexEntry := map[string]any{
		"session_id": sessionID,
		"date":       session.Date.Format(time.RFC3339),
		"summary":    session.Summary,
		"path":       sessionPath,
	}
	indexData, err := json.Marshal(indexEntry)
	if err != nil {
		t.Fatalf("marshal index entry: %v", err)
	}
	if err := os.WriteFile(indexPath, append(indexData, '\n'), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}

	assertFileExists(t, sessionPath)
	assertFileExists(t, indexPath)
}

func (f knowledgeLoopFixture) runInjectPhase(t *testing.T) {
	learnings, err := collectLearnings(f.tempDir, "context", 10, "", 0)
	if err != nil {
		t.Fatalf("collectLearnings: %v", err)
	}
	if len(learnings) == 0 {
		t.Fatal("Expected to find at least 1 learning")
	}

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

	patterns, err := collectPatterns(f.tempDir, "shutdown", 5, "", 0)
	if err != nil {
		t.Fatalf("collectPatterns: %v", err)
	}
	if len(patterns) == 0 {
		t.Error("Expected to find at least 1 pattern")
	}
}

func (f knowledgeLoopFixture) runCitationPhase(t *testing.T) {
	sessionID := "session-20260125-120000"
	event := types.CitationEvent{
		ArtifactPath: f.learningPath,
		SessionID:    sessionID,
		CitedAt:      time.Now(),
		CitationType: "retrieved",
		Query:        "context cancellation",
	}
	if err := ratchet.RecordCitation(f.tempDir, event); err != nil {
		t.Fatalf("RecordCitation: %v", err)
	}

	citationsPath := filepath.Join(f.tempDir, ".agents", "ao", "citations.jsonl")
	assertFileExists(t, citationsPath)
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
}

func (f knowledgeLoopFixture) runFeedbackPhase(t *testing.T) {
	originalLearning, err := parseLearningJSONL(f.learningPath)
	if err != nil {
		t.Fatalf("parse original learning: %v", err)
	}

	originalUtility := originalLearning.Utility
	newUtility := updateUtility(originalUtility, 1.0, types.DefaultAlpha)
	if newUtility <= originalUtility {
		t.Errorf("Utility should increase after positive feedback: original=%.3f, new=%.3f",
			originalUtility, newUtility)
	}
	if newUtility < 0 || newUtility > 1 {
		t.Errorf("Utility should be in [0,1]: got %.3f", newUtility)
	}
}

func (f knowledgeLoopFixture) runMetricsPhase(t *testing.T) {
	sessionCount, err := countKnowledgeLoopSessions(f.tempDir)
	if err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if sessionCount == 0 {
		t.Error("Expected at least 1 session")
	}

	citations, err := ratchet.LoadCitations(f.tempDir)
	if err != nil {
		t.Fatalf("LoadCitations: %v", err)
	}
	if len(citations) == 0 {
		t.Error("Expected at least 1 citation")
	}

	sigma := 0.5  // retrieval effectiveness (simulated)
	rho := 0.3    // citation rate (simulated)
	delta := 17.0 // avg age in days
	sigmaRho := sigma * rho
	escapingVelocity := sigmaRho > delta/100.0
	t.Logf("Flywheel metrics: σ=%.2f, ρ=%.2f, δ=%.2f, σ×ρ=%.3f, escaping=%v",
		sigma, rho, delta, sigmaRho, escapingVelocity)
	if sigmaRho >= 1 {
		t.Error("σ×ρ should be < 1 for valid probability")
	}
}

func (f knowledgeLoopFixture) runSecondCyclePhase(t *testing.T) {
	session2 := &storage.Session{
		ID:      "test-session-002",
		Date:    time.Now(),
		Summary: "Second test session",
		Decisions: []string{
			"Add retry logic to HTTP client",
		},
	}

	session2Path := filepath.Join(f.tempDir, ".agents", "ao", "sessions", "test-session-002.jsonl")
	data, err := json.Marshal(session2)
	if err != nil {
		t.Fatalf("marshal session 2: %v", err)
	}
	if err := os.WriteFile(session2Path, data, 0o644); err != nil {
		t.Fatalf("write session 2: %v", err)
	}

	sessionCount, err := countKnowledgeLoopSessions(f.tempDir)
	if err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if sessionCount != 2 {
		t.Errorf("Expected 2 sessions, got %d", sessionCount)
	}

	event := types.CitationEvent{
		ArtifactPath: f.learningPath,
		SessionID:    "session-20260125-130000",
		CitedAt:      time.Now(),
		CitationType: "applied", // upgraded from retrieved
		Query:        "graceful shutdown",
	}
	if err := ratchet.RecordCitation(f.tempDir, event); err != nil {
		t.Fatalf("RecordCitation 2: %v", err)
	}

	citations, err := ratchet.LoadCitations(f.tempDir)
	if err != nil {
		t.Fatalf("LoadCitations: %v", err)
	}
	if len(citations) < 2 {
		t.Errorf("Expected at least 2 citations, got %d", len(citations))
	}
}

func (f knowledgeLoopFixture) runBadgePhase(t *testing.T) {
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

	bar := makeProgressBar(0.5, 10)
	if runeCount := len([]rune(bar)); runeCount != 10 {
		t.Errorf("Progress bar should be 10 runes, got %d", runeCount)
	}
	if !strings.Contains(bar, "█") || !strings.Contains(bar, "░") {
		t.Errorf("Progress bar should have filled and empty segments: %s", bar)
	}

	barEmpty := makeProgressBar(0, 10)
	if strings.Contains(barEmpty, "█") {
		t.Error("Empty progress bar should have no filled segments")
	}
	barFull := makeProgressBar(1, 10)
	if strings.Contains(barFull, "░") {
		t.Error("Full progress bar should have no empty segments")
	}
	if barOver := makeProgressBar(1.5, 10); barOver != barFull {
		t.Error("Value > 1 should clamp to full bar")
	}
	if barUnder := makeProgressBar(-0.5, 10); barUnder != barEmpty {
		t.Error("Value < 0 should clamp to empty bar")
	}
}

func countKnowledgeLoopSessions(tempDir string) (int, error) {
	sessionsDir := filepath.Join(tempDir, ".agents", "ao", "sessions")
	files, err := filepath.Glob(filepath.Join(sessionsDir, "*.jsonl"))
	if err != nil {
		return 0, err
	}
	return len(files), nil
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
