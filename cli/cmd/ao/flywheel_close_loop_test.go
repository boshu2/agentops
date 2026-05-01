package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

// writeTestMDLearning creates a markdown learning file with YAML front matter.
func writeTestMDLearning(t *testing.T, dir, filename string, fm map[string]string, body string) string {
	t.Helper()
	var sb strings.Builder
	sb.WriteString("---\n")
	for k, v := range fm {
		sb.WriteString(k + ": " + v + "\n")
	}
	sb.WriteString("---\n")
	sb.WriteString(body)
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("write test learning: %v", err)
	}
	return path
}

func TestApplyAllMaturityTransitions_NoLearningsDir(t *testing.T) {
	tmp := t.TempDir()
	// No .agents/learnings/ directory exists
	summary, err := applyAllMaturityTransitions(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Total != 0 {
		t.Errorf("Total = %d, want 0", summary.Total)
	}
	if summary.Applied != 0 {
		t.Errorf("Applied = %d, want 0", summary.Applied)
	}
	if len(summary.ChangedPaths) != 0 {
		t.Errorf("ChangedPaths = %v, want empty", summary.ChangedPaths)
	}
}

func TestApplyAllMaturityTransitions_EmptyDir(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	summary, err := applyAllMaturityTransitions(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Total != 0 {
		t.Errorf("Total = %d, want 0", summary.Total)
	}
	if summary.Applied != 0 {
		t.Errorf("Applied = %d, want 0", summary.Applied)
	}
}

func TestApplyAllMaturityTransitions_AppliesPromotion(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// provisional -> candidate requires utility >= 0.55 AND reward_count >= 3
	path := writeTestMDLearning(t, learningsDir, "test-learn.md", map[string]string{
		"id":           "test-learn",
		"maturity":     "provisional",
		"utility":      "0.85",
		"reward_count": "5",
		"confidence":   "0.8",
	}, "# Test Learning\nContent here.\n")

	summary, err := applyAllMaturityTransitions(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Total != 1 {
		t.Errorf("Total = %d, want 1", summary.Total)
	}
	if summary.Applied != 1 {
		t.Errorf("Applied = %d, want 1", summary.Applied)
	}

	// Verify the file now has maturity: candidate
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "maturity: candidate") {
		t.Errorf("expected maturity: candidate in file, got:\n%s", content)
	}
}

func TestApplyAllMaturityTransitions_AppliesAntiPattern(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// anti-pattern requires utility <= 0.2 AND harmful_count >= 3
	path := writeTestMDLearning(t, learningsDir, "bad-learn.md", map[string]string{
		"id":            "bad-learn",
		"maturity":      "provisional",
		"utility":       "0.1",
		"harmful_count": "6",
		"confidence":    "0.5",
	}, "# Bad Learning\nThis is harmful.\n")

	summary, err := applyAllMaturityTransitions(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Total != 1 {
		t.Errorf("Total = %d, want 1", summary.Total)
	}
	if summary.Applied != 1 {
		t.Errorf("Applied = %d, want 1", summary.Applied)
	}

	// Verify the file now has maturity: anti-pattern
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "maturity: anti-pattern") {
		t.Errorf("expected maturity: anti-pattern in file, got:\n%s", content)
	}
}

func TestApplyAllMaturityTransitions_MixedTransitions(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// 1. Should promote: provisional -> candidate (utility >= 0.55, reward_count >= 3)
	writeTestMDLearning(t, learningsDir, "good-learn.md", map[string]string{
		"id":           "good-learn",
		"maturity":     "provisional",
		"utility":      "0.85",
		"reward_count": "5",
		"confidence":   "0.8",
	}, "# Good Learning\nHigh utility content.\n")

	// 2. Should stay: provisional, mid-range utility, not enough rewards
	writeTestMDLearning(t, learningsDir, "mid-learn.md", map[string]string{
		"id":           "mid-learn",
		"maturity":     "provisional",
		"utility":      "0.5",
		"reward_count": "1",
		"confidence":   "0.6",
	}, "# Mid Learning\nAverage content.\n")

	// 3. Should become anti-pattern: utility <= 0.2, harmful_count >= 3
	writeTestMDLearning(t, learningsDir, "bad-learn.md", map[string]string{
		"id":            "bad-learn",
		"maturity":      "provisional",
		"utility":       "0.1",
		"harmful_count": "7",
		"confidence":    "0.3",
	}, "# Bad Learning\nHarmful content.\n")

	summary, err := applyAllMaturityTransitions(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Total should be 2: good-learn (promotes) and bad-learn (anti-pattern)
	// mid-learn does NOT transition so it is NOT in the scan results
	if summary.Total != 2 {
		t.Errorf("Total = %d, want 2", summary.Total)
	}
	if summary.Applied != 2 {
		t.Errorf("Applied = %d, want 2", summary.Applied)
	}
	if len(summary.ChangedPaths) != 2 {
		t.Errorf("ChangedPaths count = %d, want 2", len(summary.ChangedPaths))
	}

	// Verify good-learn is now candidate
	goodData, err := os.ReadFile(filepath.Join(learningsDir, "good-learn.md"))
	if err != nil {
		t.Fatalf("read good-learn: %v", err)
	}
	if !strings.Contains(string(goodData), "maturity: candidate") {
		t.Errorf("good-learn should be candidate, got:\n%s", string(goodData))
	}

	// Verify mid-learn is still provisional
	midData, err := os.ReadFile(filepath.Join(learningsDir, "mid-learn.md"))
	if err != nil {
		t.Fatalf("read mid-learn: %v", err)
	}
	if !strings.Contains(string(midData), "maturity: provisional") {
		t.Errorf("mid-learn should still be provisional, got:\n%s", string(midData))
	}

	// Verify bad-learn is now anti-pattern
	badData, err := os.ReadFile(filepath.Join(learningsDir, "bad-learn.md"))
	if err != nil {
		t.Fatalf("read bad-learn: %v", err)
	}
	if !strings.Contains(string(badData), "maturity: anti-pattern") {
		t.Errorf("bad-learn should be anti-pattern, got:\n%s", string(badData))
	}
}

func TestCloseLoop_AppliesAllTransitions(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create a .md learning with high utility that should promote from provisional to candidate.
	// Transition requires utility >= 0.55 AND reward_count >= 3.
	path := writeTestMDLearning(t, learningsDir, "test-learn-e2e-001.md", map[string]string{
		"id":            "test-learn-e2e-001",
		"maturity":      "provisional",
		"utility":       "0.85",
		"reward_count":  "5",
		"confidence":    "0.8",
		"helpful_count": "4",
		"harmful_count": "0",
	}, "# Test Learning\nThis learning has high utility and should promote to candidate.\n")

	summary, err := applyAllMaturityTransitions(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Applied < 1 {
		t.Errorf("Applied = %d, want >= 1", summary.Applied)
	}

	// Verify the file now contains maturity: candidate
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "maturity: candidate") {
		t.Errorf("expected maturity: candidate in file, got:\n%s", content)
	}
}

func TestCloseLoop_IntegrationEndToEnd(t *testing.T) {
	tmp := t.TempDir()

	// 1. Create the learning file
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatalf("mkdir learnings: %v", err)
	}

	learningPath := writeTestMDLearning(t, learningsDir, "e2e-test-learning.md", map[string]string{
		"id":             "e2e-test-learning",
		"utility":        "0.6000",
		"reward_count":   "3",
		"last_reward":    "0.70",
		"last_reward_at": "2026-02-24T00:00:00Z",
		"maturity":       "provisional",
		"helpful_count":  "2",
		"harmful_count":  "0",
		"confidence":     "0.3750",
	}, "# E2E Test Learning\nThis is an end-to-end test learning to verify the feedback loop.\n")

	// 2. Write a citation for this learning
	citation := types.CitationEvent{
		SessionID:     "e2e-test-session",
		ArtifactPath:  ".agents/learnings/e2e-test-learning.md",
		CitationType:  "applied",
		CitedAt:       time.Date(2026, 2, 24, 12, 0, 0, 0, time.UTC),
		FeedbackGiven: false,
	}
	citationData, err := json.Marshal(citation)
	if err != nil {
		t.Fatalf("marshal citation: %v", err)
	}
	citationsPath := filepath.Join(tmp, ".agents", "ao", "citations.jsonl")
	if err := os.MkdirAll(filepath.Dir(citationsPath), 0755); err != nil {
		t.Fatalf("mkdir citations: %v", err)
	}
	if err := os.WriteFile(citationsPath, append(citationData, '\n'), 0600); err != nil {
		t.Fatalf("write citations: %v", err)
	}

	// 3. Override HOME so computeSessionRewardForCloseLoop can't find real transcripts
	//    and falls back to 0.5 (neutral reward).
	t.Setenv("HOME", tmp)

	// 4. Call processCitationFeedback
	total, rewarded, skipped := processCitationFeedback(tmp)

	// 5. Verify return values
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if rewarded != 1 {
		t.Errorf("rewarded = %d, want 1", rewarded)
	}
	if skipped != 0 {
		t.Errorf("skipped = %d, want 0", skipped)
	}

	// 6. Verify utility updated via EMA with fallback reward=0.5
	//    Expected: (1-0.1)*0.6 + 0.1*0.5 = 0.54 + 0.05 = 0.59
	data, err := os.ReadFile(learningPath)
	if err != nil {
		t.Fatalf("read learning: %v", err)
	}
	content := string(data)

	// Parse utility from frontmatter
	// With annealed alpha (reward_count=3): alpha = 0.1 * 3.0 * exp(-0.3) ≈ 0.2222
	// new = 0.6 + 0.2222 * (0.5 - 0.6) ≈ 0.5778
	utilityVal := parseFrontMatterFloat(content, "utility")
	expectedAlpha := annealedAlpha(types.DefaultAlpha, 3)
	expectedUtility := 0.6 + expectedAlpha*(0.5-0.6)
	if math.Abs(utilityVal-expectedUtility) > 0.01 {
		t.Errorf("utility = %.4f, want approximately %.4f (tolerance 0.01)", utilityVal, expectedUtility)
	}

	// 7. Verify reward_count incremented from 3 to 4
	if !strings.Contains(content, "reward_count: 4") {
		t.Errorf("expected reward_count: 4 in file, got:\n%s", content)
	}

	// 8. Verify feedback.jsonl was written
	feedbackPath := filepath.Join(tmp, ".agents", "ao", "feedback.jsonl")
	feedbackData, err := os.ReadFile(feedbackPath)
	if err != nil {
		t.Fatalf("read feedback.jsonl: %v", err)
	}
	feedbackContent := string(feedbackData)
	if feedbackContent == "" {
		t.Fatal("feedback.jsonl is empty")
	}

	// Parse the first feedback event
	var fe FeedbackEvent
	if err := json.Unmarshal([]byte(strings.Split(feedbackContent, "\n")[0]), &fe); err != nil {
		t.Fatalf("parse feedback event: %v", err)
	}

	if fe.SessionID == "" {
		t.Error("feedback event session_id is empty")
	}
	if !strings.Contains(fe.ArtifactPath, "e2e-test-learning") {
		t.Errorf("feedback event artifact_path = %q, want to contain 'e2e-test-learning'", fe.ArtifactPath)
	}
	if math.Abs(fe.Reward-0.5) > 0.01 {
		t.Errorf("feedback event reward = %.4f, want approximately 0.5", fe.Reward)
	}
	if math.Abs(fe.UtilityBefore-0.6) > 0.01 {
		t.Errorf("feedback event utility_before = %.4f, want approximately 0.6", fe.UtilityBefore)
	}
	if math.Abs(fe.UtilityAfter-expectedUtility) > 0.01 {
		t.Errorf("feedback event utility_after = %.4f, want approximately %.4f", fe.UtilityAfter, expectedUtility)
	}
	if math.Abs(fe.Alpha-expectedAlpha) > 0.001 {
		t.Errorf("feedback event alpha = %.4f, want %.4f (annealed)", fe.Alpha, expectedAlpha)
	}
}

func TestCloseLoop_CitationFeedbackWithMaturityTransition(t *testing.T) {
	tmp := t.TempDir()

	// 1. Create a .md learning with utility=0.68, reward_count=2, maturity=provisional
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatalf("mkdir learnings: %v", err)
	}

	learningPath := writeTestMDLearning(t, learningsDir, "transition-test-learning.md", map[string]string{
		"id":            "transition-test-learning",
		"utility":       "0.68",
		"reward_count":  "2",
		"maturity":      "provisional",
		"helpful_count": "2",
		"harmful_count": "0",
		"confidence":    "0.3",
	}, "# Transition Test Learning\nThis learning will be manually pushed to candidate threshold.\n")

	// 2. Write a citation for it
	citation := types.CitationEvent{
		SessionID:     "transition-test-session",
		ArtifactPath:  ".agents/learnings/transition-test-learning.md",
		CitationType:  "applied",
		CitedAt:       time.Date(2026, 2, 24, 12, 0, 0, 0, time.UTC),
		FeedbackGiven: false,
	}
	citationData, err := json.Marshal(citation)
	if err != nil {
		t.Fatalf("marshal citation: %v", err)
	}
	citationsPath := filepath.Join(tmp, ".agents", "ao", "citations.jsonl")
	if err := os.MkdirAll(filepath.Dir(citationsPath), 0755); err != nil {
		t.Fatalf("mkdir citations: %v", err)
	}
	if err := os.WriteFile(citationsPath, append(citationData, '\n'), 0600); err != nil {
		t.Fatalf("write citations: %v", err)
	}

	// 3. Override HOME (no transcripts -> fallback reward 0.5)
	t.Setenv("HOME", tmp)

	// 4. Call processCitationFeedback — with annealed alpha (reward_count=2):
	// alpha = 0.1 * 3.0 * exp(-0.2) ≈ 0.2456
	// new = 0.68 + 0.2456 * (0.5 - 0.68) ≈ 0.6358
	total, rewarded, skipped := processCitationFeedback(tmp)
	if total != 1 || rewarded != 1 || skipped != 0 {
		t.Errorf("processCitationFeedback = (%d, %d, %d), want (1, 1, 0)", total, rewarded, skipped)
	}

	// Verify utility moved with annealed alpha (still below 0.55 threshold for promotion)
	expectedAlphaT := annealedAlpha(types.DefaultAlpha, 2)
	expectedUtilityT := 0.68 + expectedAlphaT*(0.5-0.68)
	data, err := os.ReadFile(learningPath)
	if err != nil {
		t.Fatalf("read learning: %v", err)
	}
	utilityAfterFeedback := parseFrontMatterFloat(string(data), "utility")
	if math.Abs(utilityAfterFeedback-expectedUtilityT) > 0.01 {
		t.Errorf("utility after feedback = %.4f, want approximately %.4f", utilityAfterFeedback, expectedUtilityT)
	}

	// 5. Now manually update utility to 0.75 and reward_count to 4
	//    (simulating multiple successful sessions that pushed utility above threshold)
	updatedContent := string(data)
	updatedContent = replaceFrontMatterField(updatedContent, "utility", "0.7500")
	updatedContent = replaceFrontMatterField(updatedContent, "reward_count", "4")
	if err := os.WriteFile(learningPath, []byte(updatedContent), 0644); err != nil {
		t.Fatalf("write updated learning: %v", err)
	}

	// 6. Call applyAllMaturityTransitions
	summary, err := applyAllMaturityTransitions(tmp)
	if err != nil {
		t.Fatalf("applyAllMaturityTransitions error: %v", err)
	}

	// 7. Verify the learning transitioned from provisional to candidate
	finalData, err := os.ReadFile(learningPath)
	if err != nil {
		t.Fatalf("read final learning: %v", err)
	}
	finalContent := string(finalData)
	if !strings.Contains(finalContent, "maturity: candidate") {
		t.Errorf("expected maturity: candidate after transition, got:\n%s", finalContent)
	}
	if summary.Applied < 1 {
		t.Errorf("Applied = %d, want >= 1", summary.Applied)
	}
}

// TestCloseLoop_CitationFeedbackBeforeMaturity verifies that citation feedback
// runs before maturity transitions, so reward_count bumps are visible to the
// maturity state machine within the same close-loop cycle.
func TestCloseLoop_CitationFeedbackBeforeMaturity(t *testing.T) {
	tmp := t.TempDir()

	// Create a learning with utility above threshold (0.55) but reward_count=2
	// (below MinFeedbackForPromotion=3). Citation feedback will bump reward_count
	// to 3, enabling promotion — but ONLY if citation runs before maturity.
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatalf("mkdir learnings: %v", err)
	}

	learningPath := writeTestMDLearning(t, learningsDir, "order-test.md", map[string]string{
		"id":            "order-test",
		"utility":       "0.7500",
		"reward_count":  "2",
		"maturity":      "provisional",
		"helpful_count": "2",
		"harmful_count": "0",
		"confidence":    "0.8",
	}, "# Order Test Learning\nVerifies citation feedback runs before maturity transitions.\n")

	// Write a citation event for this learning
	citation := types.CitationEvent{
		SessionID:     "order-test-session",
		ArtifactPath:  ".agents/learnings/order-test.md",
		CitationType:  "applied",
		CitedAt:       time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC),
		FeedbackGiven: false,
	}
	citationData, err := json.Marshal(citation)
	if err != nil {
		t.Fatalf("marshal citation: %v", err)
	}
	citationsPath := filepath.Join(tmp, ".agents", "ao", "citations.jsonl")
	if err := os.MkdirAll(filepath.Dir(citationsPath), 0755); err != nil {
		t.Fatalf("mkdir citations: %v", err)
	}
	if err := os.WriteFile(citationsPath, append(citationData, '\n'), 0600); err != nil {
		t.Fatalf("write citations: %v", err)
	}

	// Override HOME so computeSessionRewardForCloseLoop falls back to 0.5
	t.Setenv("HOME", tmp)

	// Run citation feedback first (matching new close-loop order)
	total, rewarded, _ := processCitationFeedback(tmp)
	if total != 1 || rewarded != 1 {
		t.Fatalf("processCitationFeedback = (%d, %d, ...), want (1, 1, ...)", total, rewarded)
	}

	// Verify reward_count was bumped to 3
	data, err := os.ReadFile(learningPath)
	if err != nil {
		t.Fatalf("read learning: %v", err)
	}
	if !strings.Contains(string(data), "reward_count: 3") {
		t.Fatalf("expected reward_count: 3 after citation, got:\n%s", string(data))
	}

	// Now run maturity transitions — should promote because reward_count=3 AND utility >= 0.55
	summary, err := applyAllMaturityTransitions(tmp)
	if err != nil {
		t.Fatalf("applyAllMaturityTransitions: %v", err)
	}
	if summary.Applied < 1 {
		t.Errorf("Applied = %d, want >= 1 (citation bump should enable promotion)", summary.Applied)
	}

	// Verify learning promoted to candidate
	finalData, err := os.ReadFile(learningPath)
	if err != nil {
		t.Fatalf("read final learning: %v", err)
	}
	if !strings.Contains(string(finalData), "maturity: candidate") {
		t.Errorf("expected maturity: candidate (citation feedback raised reward_count to 3), got:\n%s", string(finalData))
	}
}

// parseFrontMatterFloat extracts a float value from YAML front matter in a markdown string.
func parseFrontMatterFloat(content, field string) float64 {
	var val float64
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, field+":") {
			_, _ = fmt.Sscanf(line, field+": %f", &val)
			return val
		}
	}
	return val
}

// replaceFrontMatterField replaces a front matter field value in a markdown string.
func replaceFrontMatterField(content, field, newValue string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, field+":") {
			lines[i] = fmt.Sprintf("%s: %s", field, newValue)
			break
		}
	}
	return strings.Join(lines, "\n")
}

func TestOutputFlywheelCloseLoopResult_JSON(t *testing.T) {
	oldOutput := output
	output = "json"
	defer func() { output = oldOutput }()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	res := flywheelCloseLoopResult{
		Ingest: poolIngestResult{
			Added:            3,
			FilesScanned:     5,
			SkippedExisting:  1,
			SkippedMalformed: 0,
		},
		AutoPromote: poolAutoPromotePromoteResult{
			Promoted:  2,
			Threshold: "24h",
		},
		MemoryPromoted: 1,
	}
	res.AntiPattern.Eligible = 4
	res.AntiPattern.Promoted = 1
	res.Store.Indexed = 10
	res.Store.Categorize = true
	res.CitationFeedback.Processed = 8
	res.CitationFeedback.Rewarded = 3
	res.CitationFeedback.Skipped = 5

	err := outputFlywheelCloseLoopResult(res)
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputFlywheelCloseLoopResult: %v", err)
	}

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	r.Close()

	var parsed flywheelCloseLoopResult
	if err := json.Unmarshal(buf[:n], &parsed); err != nil {
		t.Fatalf("parse JSON: %v\n%s", err, string(buf[:n]))
	}
	if parsed.Ingest.Added != 3 {
		t.Errorf("Ingest.Added = %d, want 3", parsed.Ingest.Added)
	}
	if parsed.MemoryPromoted != 1 {
		t.Errorf("MemoryPromoted = %d, want 1", parsed.MemoryPromoted)
	}
}

func TestOutputFlywheelCloseLoopResult_Table(t *testing.T) {
	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	oldQuiet := flywheelCloseLoopQuiet
	flywheelCloseLoopQuiet = false
	defer func() { flywheelCloseLoopQuiet = oldQuiet }()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	res := flywheelCloseLoopResult{
		Ingest: poolIngestResult{Added: 2, FilesScanned: 4},
		AutoPromote: poolAutoPromotePromoteResult{
			Promoted:  1,
			Threshold: "24h",
		},
		MemoryPromoted: 0,
	}
	res.Store.Indexed = 5
	res.CitationFeedback.Processed = 3
	res.CitationFeedback.Rewarded = 1
	res.CitationFeedback.Skipped = 2

	err := outputFlywheelCloseLoopResult(res)
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputFlywheelCloseLoopResult: %v", err)
	}

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	r.Close()
	out := string(buf[:n])

	if !strings.Contains(out, "Flywheel Close-Loop Summary") {
		t.Errorf("expected header in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Pool ingest: added=2") {
		t.Errorf("expected ingest line, got:\n%s", out)
	}
	if !strings.Contains(out, "Auto-promote: promoted=1") {
		t.Errorf("expected auto-promote line, got:\n%s", out)
	}
}

func TestOutputFlywheelCloseLoopResult_Quiet(t *testing.T) {
	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	oldQuiet := flywheelCloseLoopQuiet
	flywheelCloseLoopQuiet = true
	defer func() { flywheelCloseLoopQuiet = oldQuiet }()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputFlywheelCloseLoopResult(flywheelCloseLoopResult{})
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputFlywheelCloseLoopResult: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	r.Close()
	out := string(buf[:n])

	if out != "" {
		t.Errorf("expected empty output in quiet mode, got:\n%s", out)
	}
}

func TestPerformFlywheelCloseLoop_GeneratesSkillDrafts(t *testing.T) {
	tmp := t.TempDir()

	patternsDir := filepath.Join(tmp, ".agents", "patterns")
	sessionsDir := filepath.Join(tmp, ".agents", "ao", "sessions")
	if err := os.MkdirAll(patternsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	patternPath := filepath.Join(patternsDir, "2026-04-09-test-pattern.md")
	if err := os.WriteFile(patternPath, []byte("---\nconfidence: 0.9\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"sess1.jsonl", "sess2.md", "sess3.jsonl"} {
		path := filepath.Join(sessionsDir, name)
		if err := os.WriteFile(path, []byte("used 2026-04-09-test-pattern.md\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result, err := performFlywheelCloseLoop(tmp, filepath.Join(".agents", "knowledge", "pending"), 0, true)
	if err != nil {
		t.Fatalf("performFlywheelCloseLoop: %v", err)
	}
	if result.Ingest.Added != 0 {
		t.Fatalf("expected no ingest work, got %+v", result.Ingest)
	}

	draftPath := filepath.Join(tmp, ".agents", "skill-drafts", "test", "SKILL.md")
	if _, err := os.Stat(draftPath); err != nil {
		t.Fatalf("expected generated skill draft at %s: %v", draftPath, err)
	}
}

// ---------------------------------------------------------------------------
// ingestPendingFilesToPool — pending file lifecycle (mol-qwx4 regression)
// ---------------------------------------------------------------------------

// writeRegressionPendingLearning writes a parseable pending learning under
// .agents/knowledge/pending/<name>.md with content unique to <name>.
func writeRegressionPendingLearning(t *testing.T, tmp, name string) string {
	t.Helper()
	pendingDir := filepath.Join(tmp, ".agents", "knowledge", "pending")
	if err := os.MkdirAll(pendingDir, 0o755); err != nil {
		t.Fatalf("mkdir pending: %v", err)
	}
	pendingFile := filepath.Join(pendingDir, name+".md")
	body := "# Learnings: ag-" + name + "\n\n" +
		"**Date:** 2026-01-01\n\n" +
		"# Learning: " + name + " title\n\n" +
		"**ID**: L-" + name + "\n" +
		"**Category**: process\n" +
		"**Confidence**: high\n\n" +
		"## What We Learned\n\n" +
		"Body content for " + name + " regression test.\n\n" +
		"## Source\n\nSession: ag-" + name + "\n"
	if err := os.WriteFile(pendingFile, []byte(body), 0o600); err != nil {
		t.Fatalf("write pending file: %v", err)
	}
	return pendingFile
}

func TestIngestPendingFilesToPool_MovesProcessedOnSuccess(t *testing.T) {
	tmp := t.TempDir()
	pendingFile := writeRegressionPendingLearning(t, tmp, "movetest")

	res, err := ingestPendingFilesToPool(tmp, []string{pendingFile})
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("Added=%d, want 1 (res=%+v)", res.Added, res)
	}

	// Source pending file should have been moved out of pending/.
	if _, err := os.Stat(pendingFile); !os.IsNotExist(err) {
		t.Errorf("pending file still present after ingest: err=%v", err)
	}

	// And present under processed/.
	dst := filepath.Join(tmp, ".agents", "knowledge", "processed", filepath.Base(pendingFile))
	if _, err := os.Stat(dst); err != nil {
		t.Errorf("expected processed file at %s: %v", dst, err)
	}
}

func TestIngestPendingFilesToPool_SecondRunIsIdempotent(t *testing.T) {
	tmp := t.TempDir()
	pendingFile := writeRegressionPendingLearning(t, tmp, "idem")

	res1, err := ingestPendingFilesToPool(tmp, []string{pendingFile})
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	if res1.Added != 1 {
		t.Fatalf("first ingest Added=%d, want 1", res1.Added)
	}

	// Second pass simulates a recurring close-loop with the same source
	// directory. If lifecycle is correct, the file is no longer at the
	// pending path, so a glob-based pass would miss it. Even if a stale path
	// is passed in directly, the read should fail and Added must be 0.
	if _, err := os.Stat(pendingFile); !os.IsNotExist(err) {
		t.Fatalf("pending file should have been moved, still at %s", pendingFile)
	}

	// Re-resolve from the pending dir the way close-loop actually does.
	resolved, err := resolveIngestFiles(tmp, filepath.Join(".agents", "knowledge", "pending"), nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	res2, err := ingestPendingFilesToPool(tmp, resolved)
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	if res2.Added != 0 {
		t.Errorf("second ingest Added=%d, want 0 (res=%+v)", res2.Added, res2)
	}
	if res2.CandidatesFound != 0 {
		t.Errorf("second ingest CandidatesFound=%d, want 0 (file should not be re-discovered)", res2.CandidatesFound)
	}
}

func TestIngestPendingFilesToPool_DryRunDoesNotMove(t *testing.T) {
	tmp := t.TempDir()
	pendingFile := writeRegressionPendingLearning(t, tmp, "dryrun")

	dryRun = true
	defer func() { dryRun = false }()

	res, err := ingestPendingFilesToPool(tmp, []string{pendingFile})
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("Added=%d, want 1 (dry-run still counts)", res.Added)
	}

	// Pending file must remain in place under dry-run.
	if _, err := os.Stat(pendingFile); err != nil {
		t.Errorf("pending file should remain under dry-run, got: %v", err)
	}
	processedPath := filepath.Join(tmp, ".agents", "knowledge", "processed", filepath.Base(pendingFile))
	if _, err := os.Stat(processedPath); !os.IsNotExist(err) {
		t.Errorf("processed file should NOT be created under dry-run, found: %v", err)
	}
}

func TestIngestPendingFilesToPool_ReadErrorDoesNotMove(t *testing.T) {
	tmp := t.TempDir()

	// Path that does not exist — os.ReadFile will fail and the function
	// must increment Errors and skip the file (no move attempt).
	pendingDir := filepath.Join(tmp, ".agents", "knowledge", "pending")
	if err := os.MkdirAll(pendingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(pendingDir, "missing.md")

	res, err := ingestPendingFilesToPool(tmp, []string{missing})
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if res.Errors != 1 {
		t.Errorf("Errors=%d, want 1", res.Errors)
	}
	if res.Added != 0 {
		t.Errorf("Added=%d, want 0", res.Added)
	}

	// processed/ dir may be created lazily, but the missing file must not
	// appear under it (we never had it to move in the first place).
	stale := filepath.Join(tmp, ".agents", "knowledge", "processed", "missing.md")
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("missing file should not appear in processed/, got: %v", err)
	}
}
