package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/types"
)

func TestResolveFeedbackLoopSessionID(t *testing.T) {
	t.Run("uses explicit flag", func(t *testing.T) {
		t.Setenv("CLAUDE_SESSION_ID", "env-session")
		got, err := resolveFeedbackLoopSessionID("flag-session")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "flag-session" {
			t.Errorf("got %q, want %q", got, "flag-session")
		}
	})

	t.Run("falls back to environment", func(t *testing.T) {
		t.Setenv("CLAUDE_SESSION_ID", "env-session")
		got, err := resolveFeedbackLoopSessionID("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "env-session" {
			t.Errorf("got %q, want %q", got, "env-session")
		}
	})

	t.Run("errors when no session provided", func(t *testing.T) {
		t.Setenv("CLAUDE_SESSION_ID", "")
		_, err := resolveFeedbackLoopSessionID("")
		if err == nil {
			t.Fatal("expected error for empty session sources")
		}
	})
}

func TestValidateFeedbackCitationType(t *testing.T) {
	t.Run("accepts valid values", func(t *testing.T) {
		for _, input := range []string{"retrieved", "applied", "all"} {
			got, err := validateFeedbackCitationType(input)
			if err != nil {
				t.Fatalf("validateFeedbackCitationType(%q): %v", input, err)
			}
			if got != input {
				t.Fatalf("got %q, want %q", got, input)
			}
		}
	})

	t.Run("rejects invalid values", func(t *testing.T) {
		_, err := validateFeedbackCitationType("typo")
		if err == nil {
			t.Fatal("expected error for invalid citation type")
		}
	})
}

func TestLoadSessionCitationsSupportsSessionAliases(t *testing.T) {
	tempDir := t.TempDir()
	timestampSession := "session-20260221-123456"
	uuidSession := "2d608ace-e8e4-4649-8ac0-70aeba0dcfee"

	entries := []types.CitationEvent{
		{
			ArtifactPath: filepath.Join(tempDir, ".agents", "learnings", "L-timestamp.jsonl"),
			SessionID:    timestampSession,
			CitedAt:      time.Now(),
			CitationType: "retrieved",
		},
		{
			ArtifactPath: filepath.Join(tempDir, ".agents", "learnings", "L-uuid.jsonl"),
			SessionID:    uuidSession,
			CitedAt:      time.Now(),
			CitationType: "retrieved",
		},
	}
	for _, entry := range entries {
		if err := ratchet.RecordCitation(tempDir, entry); err != nil {
			t.Fatalf("record citation: %v", err)
		}
	}

	gotTimestamp, err := loadSessionCitations(tempDir, "20260221-123456", "all")
	if err != nil {
		t.Fatalf("loadSessionCitations timestamp alias: %v", err)
	}
	if len(gotTimestamp) != 1 {
		t.Fatalf("expected 1 timestamp alias citation, got %d", len(gotTimestamp))
	}

	gotUUID, err := loadSessionCitations(tempDir, "session-uuid-2d608ace-e8e4-4649-8ac0-70aeba0dcfee", "all")
	if err != nil {
		t.Fatalf("loadSessionCitations uuid alias: %v", err)
	}
	if len(gotUUID) != 1 {
		t.Fatalf("expected 1 uuid alias citation, got %d", len(gotUUID))
	}
}

func TestWriteFeedbackEvents(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "feedback-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir) //nolint:errcheck // test cleanup

	events := []FeedbackEvent{
		{
			SessionID:     "session-20260125-120000",
			ArtifactPath:  "/path/to/learning1.jsonl",
			Reward:        0.85,
			UtilityBefore: 0.5,
			UtilityAfter:  0.535,
			Alpha:         0.1,
			RecordedAt:    time.Now(),
		},
		{
			SessionID:     "session-20260125-120000",
			ArtifactPath:  "/path/to/learning2.jsonl",
			Reward:        0.85,
			UtilityBefore: 0.6,
			UtilityAfter:  0.625,
			Alpha:         0.1,
			RecordedAt:    time.Now(),
		},
	}

	// Write events
	if err := writeFeedbackEvents(tempDir, events); err != nil {
		t.Fatalf("write feedback events: %v", err)
	}

	// Verify file exists
	feedbackPath := filepath.Join(tempDir, FeedbackFilePath)
	if _, err := os.Stat(feedbackPath); os.IsNotExist(err) {
		t.Fatal("feedback file not created")
	}

	// Read and verify content
	loaded, err := loadFeedbackEvents(tempDir)
	if err != nil {
		t.Fatalf("load feedback events: %v", err)
	}

	if len(loaded) != len(events) {
		t.Errorf("got %d events, want %d", len(loaded), len(events))
	}

	// Verify first event
	if loaded[0].SessionID != events[0].SessionID {
		t.Errorf("session ID mismatch: got %s, want %s", loaded[0].SessionID, events[0].SessionID)
	}
	if loaded[0].Reward != events[0].Reward {
		t.Errorf("reward mismatch: got %.2f, want %.2f", loaded[0].Reward, events[0].Reward)
	}
}

func TestLoadFeedbackEventsEmpty(t *testing.T) {
	// Create temp directory without feedback file
	tempDir, err := os.MkdirTemp("", "feedback-empty-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir) //nolint:errcheck // test cleanup

	// Load from non-existent file should return empty slice
	events, err := loadFeedbackEvents(tempDir)
	if err == nil {
		t.Error("expected error for non-existent file")
	}
	if len(events) != 0 {
		t.Errorf("expected empty slice, got %d events", len(events))
	}
}

func TestCanonicalSessionID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		pattern  string // regex pattern to match if exact match not expected
	}{
		{
			name:    "empty generates timestamp",
			input:   "",
			pattern: `^session-\d{8}-\d{6}$`,
		},
		{
			name:     "already canonical",
			input:    "session-20260125-120000",
			expected: "session-20260125-120000",
		},
		{
			name:     "UUID format",
			input:    "2d608ace-e8e4-4649-8ac0-70aeba0dcfee",
			expected: "session-uuid-2d608ace-e8e4-4649-8ac0-70aeba0dcfee",
		},
		{
			name:     "custom ID preserved",
			input:    "my-custom-session",
			expected: "my-custom-session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := canonicalSessionID(tt.input)

			if tt.expected != "" {
				if result != tt.expected {
					t.Errorf("got %q, want %q", result, tt.expected)
				}
			} else if tt.pattern != "" {
				// Pattern match for generated IDs
				if !matchPattern(result, tt.pattern) {
					t.Errorf("result %q doesn't match pattern %s", result, tt.pattern)
				}
			}
		})
	}
}

func matchPattern(s, pattern string) bool {
	// Simple pattern match without regexp for testing
	// Just check basic format
	if pattern == `^session-\d{8}-\d{6}$` {
		if len(s) != 23 { // "session-YYYYMMDD-HHMMSS" = 23 chars
			return false
		}
		if s[:8] != "session-" {
			return false
		}
		return true
	}
	return false
}

func TestIntegrationFeedbackLoop(t *testing.T) {
	// Create temp directory with full structure
	tempDir, err := os.MkdirTemp("", "feedback-integration-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir) //nolint:errcheck // test cleanup

	// Create .agents/learnings/ directory
	learningsDir := filepath.Join(tempDir, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatalf("create learnings dir: %v", err)
	}

	// Create .agents/ao/ directory for citations
	aoDir := filepath.Join(tempDir, ".agents", "ao")
	if err := os.MkdirAll(aoDir, 0755); err != nil {
		t.Fatalf("create ao dir: %v", err)
	}

	// Create a test learning
	learningData := map[string]any{
		"id":      "L-test-001",
		"title":   "Test Learning",
		"content": "This is a test learning",
		"utility": 0.5,
	}
	learningJSON, err := json.Marshal(learningData)
	if err != nil {
		t.Fatalf("marshal learning data: %v", err)
	}
	learningPath := filepath.Join(learningsDir, "L-test-001.jsonl")
	if err := os.WriteFile(learningPath, learningJSON, 0644); err != nil {
		t.Fatalf("write learning: %v", err)
	}

	// Create a citation for the learning
	sessionID := "session-20260125-120000"
	citation := types.CitationEvent{
		ArtifactPath: learningPath,
		SessionID:    sessionID,
		CitedAt:      time.Now(),
		CitationType: "retrieved",
	}
	if err := ratchet.RecordCitation(tempDir, citation); err != nil {
		t.Fatalf("record citation: %v", err)
	}

	// Verify citation was recorded
	citations, err := ratchet.LoadCitations(tempDir)
	if err != nil {
		t.Fatalf("load citations: %v", err)
	}
	if len(citations) != 1 {
		t.Fatalf("expected 1 citation, got %d", len(citations))
	}

	// Verify the citation has correct session ID
	if citations[0].SessionID != sessionID {
		t.Errorf("citation session ID mismatch: got %s, want %s", citations[0].SessionID, sessionID)
	}
}

func TestFeedbackFilePath(t *testing.T) {
	// Verify the feedback file path is correct
	expected := ".agents/ao/feedback.jsonl"
	if FeedbackFilePath != expected {
		t.Errorf("FeedbackFilePath = %q, want %q", FeedbackFilePath, expected)
	}
}

func TestMarkCitationFeedback(t *testing.T) {
	tempDir := t.TempDir()
	sessionID := "session-target"
	artifact1 := filepath.Join(tempDir, ".agents", "learnings", "L1.jsonl")
	artifact2 := filepath.Join(tempDir, ".agents", "patterns", "P1.md")

	citations := []types.CitationEvent{
		{ArtifactPath: artifact1, SessionID: sessionID, CitedAt: time.Now(), CitationType: "retrieved"},
		{ArtifactPath: artifact2, SessionID: sessionID, CitedAt: time.Now(), CitationType: "retrieved"},
		{ArtifactPath: artifact1, SessionID: "other-session", CitedAt: time.Now(), CitationType: "retrieved"},
	}
	for _, c := range citations {
		if err := ratchet.RecordCitation(tempDir, c); err != nil {
			t.Fatalf("record citation: %v", err)
		}
	}

	events := []FeedbackEvent{
		{
			SessionID:     sessionID,
			ArtifactPath:  artifact1,
			Reward:        0.9,
			UtilityBefore: 0.5,
			UtilityAfter:  0.54,
			RecordedAt:    time.Now(),
		},
	}

	if err := markCitationFeedback(tempDir, sessionID, 0.9, events); err != nil {
		t.Fatalf("markCitationFeedback failed: %v", err)
	}

	updated, err := ratchet.LoadCitations(tempDir)
	if err != nil {
		t.Fatalf("load citations: %v", err)
	}
	if len(updated) != 3 {
		t.Fatalf("expected 3 citations, got %d", len(updated))
	}

	targetMarked := 0
	otherMarked := 0
	for _, c := range updated {
		if c.SessionID == sessionID {
			if !c.FeedbackGiven {
				t.Fatal("expected target session citation to be marked feedback_given")
			}
			if c.FeedbackReward != 0.9 {
				t.Fatalf("FeedbackReward = %f, want 0.9", c.FeedbackReward)
			}
			targetMarked++
		} else if c.FeedbackGiven {
			otherMarked++
		}
		if c.SessionID == sessionID && c.ArtifactPath == artifact1 {
			if c.UtilityBefore != 0.5 || c.UtilityAfter != 0.54 {
				t.Fatalf("utility metadata mismatch: before=%f after=%f", c.UtilityBefore, c.UtilityAfter)
			}
		}
	}
	if targetMarked != 2 {
		t.Fatalf("expected 2 target citations marked, got %d", targetMarked)
	}
	if otherMarked != 0 {
		t.Fatalf("expected 0 non-target citations marked, got %d", otherMarked)
	}
}

func TestComputeRewardFromTranscriptPrefersSessionMatch(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	base := filepath.Join(tempHome, ".claude", "projects", "proj1")
	if err := os.MkdirAll(base, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	nonMatchPath := filepath.Join(base, "newer.jsonl")
	nonMatchContent := `{"type":"user","sessionId":"other-session"}
{"type":"tool_result","content":"PASSED 10 tests"}
{"type":"tool_result","content":"[main abc123] feat: test"}
{"type":"tool_result","content":"Enumerating objects: 2, done.\nWriting objects: 100% (2/2), done."}`
	if err := os.WriteFile(nonMatchPath, []byte(nonMatchContent), 0644); err != nil {
		t.Fatalf("write non-match transcript: %v", err)
	}

	matchPath := filepath.Join(base, "older-match.jsonl")
	matchContent := `{"type":"user","sessionId":"target-session"}
{"type":"assistant","message":{"content":"hello"}}`
	if err := os.WriteFile(matchPath, []byte(matchContent), 0644); err != nil {
		t.Fatalf("write match transcript: %v", err)
	}

	reward, err := computeRewardFromTranscript("", "target-session")
	if err != nil {
		t.Fatalf("computeRewardFromTranscript failed: %v", err)
	}
	if reward > 0.4 {
		t.Fatalf("expected low reward from matched transcript, got %.2f", reward)
	}
}

func TestFeedbackLoop_loadSessionCitations_filterByType(t *testing.T) {
	tmp := t.TempDir()
	sessionID := "session-20260201-120000"

	// Write citations with different types
	for _, entry := range []types.CitationEvent{
		{ArtifactPath: filepath.Join(tmp, "L1.md"), SessionID: sessionID, CitedAt: time.Now(), CitationType: "retrieved"},
		{ArtifactPath: filepath.Join(tmp, "L2.md"), SessionID: sessionID, CitedAt: time.Now(), CitationType: "applied"},
		{ArtifactPath: filepath.Join(tmp, "L3.md"), SessionID: sessionID, CitedAt: time.Now(), CitationType: "retrieved"},
	} {
		if err := ratchet.RecordCitation(tmp, entry); err != nil {
			t.Fatalf("record citation: %v", err)
		}
	}

	// Filter by "retrieved"
	retrieved, err := loadSessionCitations(tmp, sessionID, "retrieved")
	if err != nil {
		t.Fatalf("loadSessionCitations (retrieved): %v", err)
	}
	if len(retrieved) != 2 {
		t.Errorf("expected 2 retrieved citations, got %d", len(retrieved))
	}

	// Filter by "all"
	all, err := loadSessionCitations(tmp, sessionID, "all")
	if err != nil {
		t.Fatalf("loadSessionCitations (all): %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 total citations, got %d", len(all))
	}
}

func TestFeedbackLoop_loadSessionCitations_wrongSession(t *testing.T) {
	tmp := t.TempDir()

	entry := types.CitationEvent{
		ArtifactPath: filepath.Join(tmp, "L1.md"),
		SessionID:    "session-other",
		CitedAt:      time.Now(),
		CitationType: "retrieved",
	}
	if err := ratchet.RecordCitation(tmp, entry); err != nil {
		t.Fatalf("record citation: %v", err)
	}

	citations, err := loadSessionCitations(tmp, "session-different", "all")
	if err != nil {
		t.Fatalf("loadSessionCitations: %v", err)
	}
	if len(citations) != 0 {
		t.Errorf("expected 0 citations for wrong session, got %d", len(citations))
	}
}

func TestFeedbackLoop_computeRewardFromTranscript_explicitPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create a minimal transcript
	transcriptPath := filepath.Join(tmp, "transcript.jsonl")
	content := `{"type":"user","sessionId":"test-session"}
{"type":"assistant","message":{"content":"hello"}}
`
	if err := os.WriteFile(transcriptPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	reward, err := computeRewardFromTranscript(transcriptPath, "test-session")
	if err != nil {
		t.Fatalf("computeRewardFromTranscript: %v", err)
	}
	// Reward should be between 0 and 1
	if reward < 0 || reward > 1 {
		t.Errorf("reward %f out of range [0, 1]", reward)
	}
}

// ===========================================================================
// feedback_loop.go — processUniqueCitations (zero coverage)
// ===========================================================================

func TestFeedbackLoop_processUniqueCitations_noFiles(t *testing.T) {
	tmp := t.TempDir()

	citations := []types.CitationEvent{
		{ArtifactPath: filepath.Join(tmp, "nonexistent.md"), SessionID: "s1", CitedAt: time.Now()},
	}

	events, updated, failed := processUniqueCitations(tmp, "s1", "", citations, 0.8, 0.1)
	if updated != 0 {
		t.Errorf("expected 0 updated, got %d", updated)
	}
	if failed != 1 {
		t.Errorf("expected 1 failed, got %d", failed)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestFeedbackLoop_processUniqueCitations_withLearning(t *testing.T) {
	tmp := t.TempDir()

	// Create a learning file
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}
	learningPath := filepath.Join(learningsDir, "L-test-001.jsonl")
	data := map[string]any{"id": "L-test-001", "utility": 0.5}
	jsonData, _ := json.Marshal(data)
	if err := os.WriteFile(learningPath, append(jsonData, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	// Save and restore feedback flags for counter update in updateJSONLUtility
	origHelpful := feedbackHelpful
	origHarmful := feedbackHarmful
	defer func() {
		feedbackHelpful = origHelpful
		feedbackHarmful = origHarmful
	}()
	feedbackHelpful = false
	feedbackHarmful = false

	citations := []types.CitationEvent{
		{ArtifactPath: learningPath, SessionID: "s1", CitedAt: time.Now()},
	}

	events, updated, failed := processUniqueCitations(tmp, "s1", "", citations, 0.9, 0.1)
	if updated != 1 {
		t.Errorf("expected 1 updated, got %d", updated)
	}
	if failed != 0 {
		t.Errorf("expected 0 failed, got %d", failed)
	}
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}
	if len(events) > 0 {
		if events[0].Reward != 0.9 {
			t.Errorf("event reward = %f, want 0.9", events[0].Reward)
		}
	}
}

// NOTE: TestFeedbackLoop_processUniqueCitations_skipsRetrievedEvidence and
// TestFeedbackLoop_processUniqueCitations_skipsLowConfidenceReference removed —
// they test confidence-gated skipping behavior that exists on the ag-wfo3 branch
// but wasn't merged into main's processUniqueCitations. Re-add when the full
// confidence-gated feedback loop lands.

func _removedTestFeedbackLoop_processUniqueCitations_skipsRetrievedEvidence(t *testing.T) {
	tmp := t.TempDir()

	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}
	learningPath := filepath.Join(learningsDir, "L-low-confidence.jsonl")
	data := map[string]any{"id": "L-low-confidence", "utility": 0.5}
	jsonData, _ := json.Marshal(data)
	if err := os.WriteFile(learningPath, append(jsonData, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	citations := []types.CitationEvent{
		{
			ArtifactPath: learningPath,
			SessionID:    "s1",
			CitedAt:      time.Now(),
			CitationType: "retrieved",
		},
	}

	events, updated, failed := processUniqueCitations(tmp, "s1", "", citations, 0.9, 0.1)
	if updated != 0 {
		t.Fatalf("expected 0 updated, got %d", updated)
	}
	if failed != 0 {
		t.Fatalf("expected 0 failed, got %d", failed)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 feedback event, got %d", len(events))
	}
	if events[0].Decision != "skipped" {
		t.Fatalf("expected skipped decision, got %q", events[0].Decision)
	}
	if events[0].Reason != "retrieved-no-artifact-evidence" {
		t.Fatalf("expected retrieved-no-artifact-evidence, got %q", events[0].Reason)
	}

	updatedData, err := os.ReadFile(learningPath)
	if err != nil {
		t.Fatalf("read learning: %v", err)
	}
	if !strings.Contains(string(updatedData), `"utility":0.5`) {
		t.Fatalf("expected utility to remain unchanged, got %s", string(updatedData))
	}
}

func _removedTestFeedbackLoop_processUniqueCitations_skipsLowConfidenceReference(t *testing.T) {
	tmp := t.TempDir()

	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}
	learningPath := filepath.Join(learningsDir, "L-low-confidence-reference.jsonl")
	data := map[string]any{"id": "L-low-confidence-reference", "utility": 0.5}
	jsonData, _ := json.Marshal(data)
	if err := os.WriteFile(learningPath, append(jsonData, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	citations := []types.CitationEvent{
		{
			ArtifactPath:    learningPath,
			SessionID:       "s1",
			CitedAt:         time.Now(),
			CitationType:    "reference",
			MatchConfidence: 0.5,
		},
	}

	events, updated, failed := processUniqueCitations(tmp, "s1", "", citations, 0.9, 0.1)
	if updated != 0 {
		t.Fatalf("expected 0 updated, got %d", updated)
	}
	if failed != 0 {
		t.Fatalf("expected 0 failed, got %d", failed)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 feedback event, got %d", len(events))
	}
	if events[0].Reason != "low-confidence-evidence" {
		t.Fatalf("expected low-confidence-evidence, got %q", events[0].Reason)
	}
}

// ===========================================================================
// feedback_loop.go — outputFeedbackSummary (zero coverage)
// ===========================================================================

func TestFeedbackLoop_outputFeedbackSummary_tableFormat(t *testing.T) {
	origOutput := output
	defer func() { output = origOutput }()
	output = "table"

	events := []FeedbackEvent{
		{SessionID: "s1", Reward: 0.8, ArtifactPath: "/a.md", UtilityBefore: 0.5, UtilityAfter: 0.53},
	}
	err := outputFeedbackSummary("s1", 0.8, 3, 2, 1, 0, events)
	if err != nil {
		t.Fatalf("outputFeedbackSummary (table): %v", err)
	}
}

func TestFeedbackLoop_outputFeedbackSummary_jsonFormat(t *testing.T) {
	origOutput := output
	defer func() { output = origOutput }()
	output = "json"

	events := []FeedbackEvent{
		{SessionID: "s1", Reward: 0.9, ArtifactPath: "/a.md"},
	}
	err := outputFeedbackSummary("s1", 0.9, 2, 1, 1, 0, events)
	if err != nil {
		t.Fatalf("outputFeedbackSummary (json): %v", err)
	}
}

// ===========================================================================
// feedback_loop.go — discoverUnprocessedSessions (zero coverage)
// ===========================================================================

func TestFeedbackLoop_discoverUnprocessedSessions_noCitations(t *testing.T) {
	tmp := t.TempDir()

	origDays := batchFeedbackDays
	defer func() { batchFeedbackDays = origDays }()
	batchFeedbackDays = 7

	sessions, latest, err := discoverUnprocessedSessions(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
	if len(latest) != 0 {
		t.Errorf("expected 0 latest, got %d", len(latest))
	}
}

func TestFeedbackLoop_discoverUnprocessedSessions_withCitations(t *testing.T) {
	tmp := t.TempDir()

	origDays := batchFeedbackDays
	defer func() { batchFeedbackDays = origDays }()
	batchFeedbackDays = 7

	// Create some citations
	for _, entry := range []types.CitationEvent{
		{ArtifactPath: "/a.md", SessionID: "session-1", CitedAt: time.Now(), CitationType: "retrieved"},
		{ArtifactPath: "/b.md", SessionID: "session-1", CitedAt: time.Now(), CitationType: "retrieved"},
		{ArtifactPath: "/c.md", SessionID: "session-2", CitedAt: time.Now(), CitationType: "retrieved"},
	} {
		if err := ratchet.RecordCitation(tmp, entry); err != nil {
			t.Fatalf("record citation: %v", err)
		}
	}

	sessions, latest, err := discoverUnprocessedSessions(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
	if len(latest) != 2 {
		t.Errorf("expected 2 latest entries, got %d", len(latest))
	}
}

func TestFeedbackLoop_discoverUnprocessedSessions_excludesProcessed(t *testing.T) {
	tmp := t.TempDir()

	origDays := batchFeedbackDays
	defer func() { batchFeedbackDays = origDays }()
	batchFeedbackDays = 7

	// Create a citation
	entry := types.CitationEvent{
		ArtifactPath: "/a.md",
		SessionID:    "session-processed",
		CitedAt:      time.Now(),
		CitationType: "retrieved",
	}
	if err := ratchet.RecordCitation(tmp, entry); err != nil {
		t.Fatalf("record citation: %v", err)
	}

	// Create matching feedback (marks session as processed)
	feedbackDir := filepath.Join(tmp, ".agents", "ao")
	if err := os.MkdirAll(feedbackDir, 0755); err != nil {
		t.Fatal(err)
	}
	fbEvent := FeedbackEvent{SessionID: "session-processed", Reward: 0.8}
	fbData, _ := json.Marshal(fbEvent)
	fbPath := filepath.Join(feedbackDir, "feedback.jsonl")
	if err := os.WriteFile(fbPath, append(fbData, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	sessions, _, err := discoverUnprocessedSessions(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 unprocessed sessions (already processed), got %d", len(sessions))
	}
}

// ===========================================================================
// feedback_loop.go — reportBatchFeedbackDryRun (zero coverage)
// ===========================================================================

func TestFeedbackLoop_reportBatchFeedbackDryRun(t *testing.T) {
	sessionIDs := []string{"s1", "s2"}
	sessionCitations := map[string][]types.CitationEvent{
		"s1": {{ArtifactPath: "/a.md"}},
		"s2": {{ArtifactPath: "/b.md"}, {ArtifactPath: "/c.md"}},
	}
	out, _ := captureStdout(t, func() error {
		reportBatchFeedbackDryRun(sessionIDs, 3, sessionCitations)
		return nil
	})
	for _, want := range []string{"s1", "s2", "dry-run"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestFeedbackLoop_reportBatchFeedbackDryRun_empty(t *testing.T) {
	out, _ := captureStdout(t, func() error {
		reportBatchFeedbackDryRun(nil, 0, nil)
		return nil
	})
	// Empty input should produce header but no session details
	if strings.Contains(out, "s1") {
		t.Errorf("empty input should not reference sessions:\n%s", out)
	}
}
