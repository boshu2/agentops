package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

func TestExtractLearningID_RelativePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{".agents/learnings/2026-02-24-abc.md", "2026-02-24-abc.md"},
		{".agents/patterns/auth-pattern.md", "auth-pattern.md"},
		{".agents/learnings/nested/deep.md", "nested/deep.md"},
		{"random/path/file.md", "file.md"}, // fallback to Base
	}
	for _, tt := range tests {
		got := extractLearningID(tt.input)
		if got != tt.want {
			t.Errorf("extractLearningID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractLearningID_AbsolutePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/home/user/repo/.agents/learnings/2026-02-24-abc.md", "2026-02-24-abc.md"},
		{"/tmp/work/.agents/patterns/go-pattern.md", "go-pattern.md"},
	}
	for _, tt := range tests {
		got := extractLearningID(tt.input)
		if got != tt.want {
			t.Errorf("extractLearningID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCitationConfidenceBuckets(t *testing.T) {
	tests := []struct {
		name string
		typ  string
		want float64
		high bool
	}{
		{name: "retrieved", typ: "retrieved", want: 0.5, high: false},
		{name: "reference", typ: "reference", want: 0.7, high: true},
		{name: "applied", typ: "applied", want: 0.9, high: true},
		{name: "blank defaults high", typ: "", want: 0.7, high: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := citationConfidenceScore(tt.typ); got != tt.want {
				t.Fatalf("citationConfidenceScore(%q) = %f, want %f", tt.typ, got, tt.want)
			}
			if got := citationIsHighConfidence(tt.typ); got != tt.high {
				t.Fatalf("citationIsHighConfidence(%q) = %v, want %v", tt.typ, got, tt.high)
			}
		})
	}
}

func TestCitationEventConfidence_PrefersRecordedMatchConfidence(t *testing.T) {
	citation := types.CitationEvent{
		CitationType:    "reference",
		MatchConfidence: 0.5,
	}
	if got := citationEventConfidence(citation); got != 0.5 {
		t.Fatalf("citationEventConfidence(reference, 0.5) = %f, want 0.5 (recorded confidence preferred over type)", got)
	}
	if citationEventIsHighConfidence(citation) {
		t.Fatal("expected low recorded confidence to suppress reward")
	}

	citation.MatchConfidence = 0.91
	if !citationEventIsHighConfidence(citation) {
		t.Fatal("expected high recorded confidence to allow reward")
	}
}

func TestMarkCitationsFeedbackGiven_WritesAllTrue(t *testing.T) {
	tmp := t.TempDir()
	citationsPath := filepath.Join(tmp, "citations.jsonl")

	citations := []types.CitationEvent{
		{ArtifactPath: ".agents/learnings/a.md", FeedbackGiven: false},
		{ArtifactPath: ".agents/learnings/b.md", FeedbackGiven: false},
		{ArtifactPath: ".agents/learnings/c.md", FeedbackGiven: true}, // already processed
	}

	markCitationsFeedbackGiven(tmp, citationsPath, citations, nil)

	data, err := os.ReadFile(citationsPath)
	if err != nil {
		t.Fatalf("failed to read citations: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	for i, line := range lines {
		var c types.CitationEvent
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			t.Fatalf("line %d: unmarshal error: %v", i, err)
		}
		if !c.FeedbackGiven {
			t.Errorf("line %d: expected FeedbackGiven=true, got false", i)
		}
	}
}

func TestMarkCitationsFeedbackGiven_EmptyList(t *testing.T) {
	tmp := t.TempDir()
	citationsPath := filepath.Join(tmp, "citations.jsonl")

	markCitationsFeedbackGiven(tmp, citationsPath, nil, nil)

	data, err := os.ReadFile(citationsPath)
	if err != nil {
		t.Fatalf("failed to read citations: %v", err)
	}

	// Should just have a trailing newline
	if strings.TrimSpace(string(data)) != "" {
		t.Errorf("expected empty content, got %q", string(data))
	}
}

func TestProcessCitationFeedback_NoCitationsFile(t *testing.T) {
	tmp := t.TempDir()
	total, rewarded, skipped := processCitationFeedback(tmp)
	if total != 0 || rewarded != 0 || skipped != 0 {
		t.Errorf("expected (0,0,0), got (%d,%d,%d)", total, rewarded, skipped)
	}
}

func TestProcessCitationFeedback_AllAlreadyProcessed(t *testing.T) {
	tmp := t.TempDir()
	aoDir := filepath.Join(tmp, ".agents", "ao")
	if err := os.MkdirAll(aoDir, 0755); err != nil {
		t.Fatal(err)
	}

	citations := []types.CitationEvent{
		{ArtifactPath: ".agents/learnings/a.md", FeedbackGiven: true},
		{ArtifactPath: ".agents/learnings/b.md", FeedbackGiven: true},
	}

	var lines []string
	for _, c := range citations {
		data, _ := json.Marshal(c)
		lines = append(lines, string(data))
	}
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(aoDir, "citations.jsonl"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	total, rewarded, skipped := processCitationFeedback(tmp)
	if total != 0 || rewarded != 0 || skipped != 0 {
		t.Errorf("expected (0,0,0) for all-processed, got (%d,%d,%d)", total, rewarded, skipped)
	}
}

func TestProcessCitationFeedback_UsesAdaptiveReward(t *testing.T) {
	// Setup: temp dir with citations and a learning file.
	// Override HOME so computeSessionRewardForCloseLoop finds no transcripts,
	// causing reward to fall back to 0.5 (InitialUtility).
	// With EMA: new = (1-0.1)*0.5 + 0.1*0.5 = 0.5 (unchanged).
	// If the old hardcoded 1.0 were used: new = (1-0.1)*0.5 + 0.1*1.0 = 0.55.
	tmp := t.TempDir()

	// Isolate from real transcripts by overriding HOME
	fakeHome := filepath.Join(tmp, "fakehome")
	if err := os.MkdirAll(fakeHome, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", fakeHome)

	// Create .agents/ao/ for citations
	aoDir := filepath.Join(tmp, ".agents", "ao")
	if err := os.MkdirAll(aoDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create .agents/learnings/ with a learning file (JSONL format)
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}
	learningPath := filepath.Join(learningsDir, "test-learning.jsonl")
	learningContent := `{"id":"test-learning","title":"Test Learning","utility":0.5}`
	if err := os.WriteFile(learningPath, []byte(learningContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Write an unprocessed citation pointing at the learning
	citations := []types.CitationEvent{
		{ArtifactPath: ".agents/learnings/test-learning.jsonl", FeedbackGiven: false},
	}
	var citationLines []string
	for _, c := range citations {
		data, _ := json.Marshal(c)
		citationLines = append(citationLines, string(data))
	}
	citationsContent := strings.Join(citationLines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(aoDir, "citations.jsonl"), []byte(citationsContent), 0600); err != nil {
		t.Fatal(err)
	}

	total, rewarded, skipped := processCitationFeedback(tmp)
	if total != 1 {
		t.Errorf("expected total=1, got %d", total)
	}
	if rewarded != 1 {
		t.Errorf("expected rewarded=1, got %d", rewarded)
	}
	if skipped != 0 {
		t.Errorf("expected skipped=0, got %d", skipped)
	}

	// Verify the learning's utility was updated with fallback reward (0.5), NOT 1.0.
	// EMA with reward=0.5, alpha=0.1, old=0.5: new = 0.9*0.5 + 0.1*0.5 = 0.5
	updatedData, err := os.ReadFile(learningPath)
	if err != nil {
		t.Fatalf("failed to read updated learning: %v", err)
	}

	var parsed map[string]any
	firstLine := strings.Split(string(updatedData), "\n")[0]
	if err := json.Unmarshal([]byte(firstLine), &parsed); err != nil {
		t.Fatalf("failed to parse updated learning: %v", err)
	}

	utility, ok := parsed["utility"].(float64)
	if !ok {
		t.Fatal("utility field not found in updated learning")
	}
	// With fallback reward 0.5, utility should remain 0.5 (unchanged).
	// With old hardcoded 1.0, it would be 0.55.
	if utility > 0.501 || utility < 0.499 {
		t.Errorf("utility = %f, want ~0.5 (fallback adaptive reward); if 0.55, hardcoded 1.0 is still in use", utility)
	}
}

func TestDeduplicateCitationFeedbackTargets_SeparatesMetricNamespaces(t *testing.T) {
	tmp := t.TempDir()
	citations := []types.CitationEvent{
		{ArtifactPath: ".agents/learnings/test-learning.jsonl", CitationType: "reference", MetricNamespace: "primary"},
		{ArtifactPath: ".agents/learnings/test-learning.jsonl", CitationType: "reference", MetricNamespace: "shadow"},
	}

	unique := deduplicateCitationFeedbackTargets(tmp, citations)
	if len(unique) != 2 {
		t.Fatalf("expected 2 deduped citations, got %d", len(unique))
	}
}

func TestProcessCitationFeedback_ShadowNamespaceAuditOnly(t *testing.T) {
	tmp := t.TempDir()

	fakeHome := filepath.Join(tmp, "fakehome")
	if err := os.MkdirAll(fakeHome, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", fakeHome)

	aoDir := filepath.Join(tmp, ".agents", "ao")
	if err := os.MkdirAll(aoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	learningPath := filepath.Join(learningsDir, "shadow-learning.jsonl")
	if err := os.WriteFile(learningPath, []byte(`{"id":"shadow-learning","title":"Shadow Learning","utility":0.6}`), 0o644); err != nil {
		t.Fatal(err)
	}

	citations := []types.CitationEvent{
		{
			ArtifactPath:    ".agents/learnings/shadow-learning.jsonl",
			CitationType:    "reference",
			MetricNamespace: "shadow",
			FeedbackGiven:   false,
		},
	}
	var citationLines []string
	for _, c := range citations {
		data, _ := json.Marshal(c)
		citationLines = append(citationLines, string(data))
	}
	if err := os.WriteFile(filepath.Join(aoDir, "citations.jsonl"), []byte(strings.Join(citationLines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	total, rewarded, skipped := processCitationFeedback(tmp)
	if total != 1 || rewarded != 0 || skipped != 1 {
		t.Fatalf("expected (1,0,1), got (%d,%d,%d)", total, rewarded, skipped)
	}

	updatedData, err := os.ReadFile(learningPath)
	if err != nil {
		t.Fatalf("failed to read updated learning: %v", err)
	}
	var parsed map[string]any
	firstLine := strings.Split(string(updatedData), "\n")[0]
	if err := json.Unmarshal([]byte(firstLine), &parsed); err != nil {
		t.Fatalf("failed to parse updated learning: %v", err)
	}
	if utility := parsed["utility"].(float64); utility != 0.6 {
		t.Fatalf("utility = %f, want unchanged 0.6", utility)
	}

	feedbackData, err := os.ReadFile(filepath.Join(aoDir, "feedback.jsonl"))
	if err != nil {
		t.Fatalf("failed to read feedback log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(feedbackData)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 feedback event, got %d", len(lines))
	}

	var event FeedbackEvent
	if err := json.Unmarshal([]byte(lines[0]), &event); err != nil {
		t.Fatalf("failed to parse feedback event: %v", err)
	}
	if event.Decision != "audited" {
		t.Fatalf("decision = %q, want audited", event.Decision)
	}
	if event.Reason != "non-primary-namespace" {
		t.Fatalf("reason = %q, want non-primary-namespace", event.Reason)
	}
	if event.MetricNamespace != "shadow" {
		t.Fatalf("metric namespace = %q, want shadow", event.MetricNamespace)
	}
}

func TestProcessCitationFeedback_WritesFeedbackEvents(t *testing.T) {
	fixture := setupCitationFeedbackEventsFixture(t)
	assertCitationFeedbackProcessCounts(t, fixture.tmp)
	event := readCitationFeedbackEvent(t, fixture.feedbackPath)
	assertCitationFeedbackEvent(t, event)
}

type citationFeedbackEventsFixture struct {
	tmp          string
	feedbackPath string
}

func setupCitationFeedbackEventsFixture(t *testing.T) citationFeedbackEventsFixture {
	t.Helper()

	tmp := t.TempDir()
	setupCitationFeedbackFakeHome(t, tmp)

	aoDir := filepath.Join(tmp, ".agents", "ao")
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	mkdirCitationFeedbackDir(t, aoDir)
	mkdirCitationFeedbackDir(t, learningsDir)

	writeCitationFeedbackLearning(t, learningsDir)
	writeCitationFeedbackCitation(t, aoDir)

	return citationFeedbackEventsFixture{
		tmp:          tmp,
		feedbackPath: filepath.Join(aoDir, "feedback.jsonl"),
	}
}

func setupCitationFeedbackFakeHome(t *testing.T, tmp string) {
	t.Helper()

	fakeHome := filepath.Join(tmp, "fakehome")
	if err := os.MkdirAll(fakeHome, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", fakeHome)
}

func mkdirCitationFeedbackDir(t *testing.T, dir string) {
	t.Helper()

	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
}

func writeCitationFeedbackLearning(t *testing.T, learningsDir string) {
	t.Helper()

	learningPath := filepath.Join(learningsDir, "fb-test.jsonl")
	if err := os.WriteFile(learningPath, []byte(`{"id":"fb-test","title":"Feedback Test","utility":0.6}`), 0644); err != nil {
		t.Fatal(err)
	}
}

func writeCitationFeedbackCitation(t *testing.T, aoDir string) {
	t.Helper()

	citation := types.CitationEvent{
		ArtifactPath:  ".agents/learnings/fb-test.jsonl",
		CitationType:  "applied",
		FeedbackGiven: false,
	}
	data, err := json.Marshal(citation)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(aoDir, "citations.jsonl"), append(data, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}

func assertCitationFeedbackProcessCounts(t *testing.T, tmp string) {
	t.Helper()

	total, rewarded, _ := processCitationFeedback(tmp)
	if total != 1 || rewarded != 1 {
		t.Fatalf("expected (1,1,_), got (%d,%d,_)", total, rewarded)
	}
}

func readCitationFeedbackEvent(t *testing.T, feedbackPath string) FeedbackEvent {
	t.Helper()

	feedbackData, err := os.ReadFile(feedbackPath)
	if err != nil {
		t.Fatalf("feedback.jsonl not created: %v", err)
	}

	feedbackLines := strings.Split(strings.TrimSpace(string(feedbackData)), "\n")
	if len(feedbackLines) == 0 {
		t.Fatal("feedback.jsonl is empty")
	}

	var event FeedbackEvent
	if err := json.Unmarshal([]byte(feedbackLines[0]), &event); err != nil {
		t.Fatalf("failed to parse FeedbackEvent: %v", err)
	}
	return event
}

func assertCitationFeedbackEvent(t *testing.T, event FeedbackEvent) {
	t.Helper()

	if event.SessionID == "" {
		t.Error("FeedbackEvent.SessionID is empty")
	}
	if event.ArtifactPath == "" {
		t.Error("FeedbackEvent.ArtifactPath is empty")
	}
	if event.Decision != "rewarded" {
		t.Errorf("FeedbackEvent.Decision = %q, want rewarded", event.Decision)
	}
	if event.Reason != "artifact-applied" {
		t.Errorf("FeedbackEvent.Reason = %q, want artifact-applied", event.Reason)
	}
	if event.UtilityBefore < 0.001 {
		t.Errorf("FeedbackEvent.UtilityBefore = %f, expected non-zero", event.UtilityBefore)
	}
	// For a new learning (0 reward_count), annealed alpha = DefaultAlpha * 3.0
	expectedAlpha := annealedAlpha(types.DefaultAlpha, 0)
	if event.Alpha != expectedAlpha {
		t.Errorf("FeedbackEvent.Alpha = %f, want %f (annealed)", event.Alpha, expectedAlpha)
	}
	if event.RecordedAt.IsZero() {
		t.Error("FeedbackEvent.RecordedAt is zero")
	}
	// Reward should be the fallback (0.5) since no transcript exists with fake HOME
	if event.Reward < 0.499 || event.Reward > 0.501 {
		t.Errorf("FeedbackEvent.Reward = %f, want ~0.5 (fallback)", event.Reward)
	}
}

func TestProcessCitationFeedback_RetrievedCitationIsSkipped(t *testing.T) {
	tmp := t.TempDir()
	aoDir := filepath.Join(tmp, ".agents", "ao")
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(aoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	learningPath := filepath.Join(learningsDir, "retrieved-only.jsonl")
	if err := os.WriteFile(learningPath, []byte(`{"id":"retrieved-only","title":"Retrieved Only","utility":0.6}`), 0o644); err != nil {
		t.Fatal(err)
	}

	citation := types.CitationEvent{
		ArtifactPath:  ".agents/learnings/retrieved-only.jsonl",
		CitationType:  "retrieved",
		FeedbackGiven: false,
	}
	data, err := json.Marshal(citation)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(aoDir, "citations.jsonl"), append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	total, rewarded, skipped := processCitationFeedback(tmp)
	if total != 1 || rewarded != 0 || skipped != 1 {
		t.Fatalf("expected (1,0,1), got (%d,%d,%d)", total, rewarded, skipped)
	}

	updatedData, err := os.ReadFile(learningPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updatedData), `"utility":0.6`) {
		t.Fatalf("expected utility to remain unchanged, got %s", string(updatedData))
	}

	feedbackData, err := os.ReadFile(filepath.Join(aoDir, "feedback.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var event FeedbackEvent
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(feedbackData))), &event); err != nil {
		t.Fatal(err)
	}
	if event.Decision != "skipped" {
		t.Fatalf("expected skipped decision, got %q", event.Decision)
	}
	if event.Reason != "retrieved-no-artifact-evidence" {
		t.Fatalf("expected retrieved-no-artifact-evidence, got %q", event.Reason)
	}
	if event.UtilityBefore != event.UtilityAfter {
		t.Fatalf("expected unchanged utility in skipped event, got before=%f after=%f", event.UtilityBefore, event.UtilityAfter)
	}
}

// NOTE: Removed — tests confidence-gated skipping in processCitationFeedback
// which exists on ag-wfo3 branch but wasn't merged into main's feedback loop.
func _removedTestProcessCitationFeedback_LowConfidenceReferenceIsSkipped(t *testing.T) {
	tmp := t.TempDir()
	aoDir := filepath.Join(tmp, ".agents", "ao")
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(aoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	learningPath := filepath.Join(learningsDir, "low-confidence-reference.jsonl")
	if err := os.WriteFile(learningPath, []byte(`{"id":"low-confidence-reference","title":"Low Confidence Reference","utility":0.6}`), 0o644); err != nil {
		t.Fatal(err)
	}

	citation := types.CitationEvent{
		ArtifactPath:    ".agents/learnings/low-confidence-reference.jsonl",
		CitationType:    "reference",
		MatchConfidence: 0.5,
		MatchProvenance: "lookup:query",
		FeedbackGiven:   false,
	}
	data, err := json.Marshal(citation)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(aoDir, "citations.jsonl"), append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	total, rewarded, skipped := processCitationFeedback(tmp)
	if total != 1 || rewarded != 0 || skipped != 1 {
		t.Fatalf("expected (1,0,1), got (%d,%d,%d)", total, rewarded, skipped)
	}

	feedbackData, err := os.ReadFile(filepath.Join(aoDir, "feedback.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var event FeedbackEvent
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(feedbackData))), &event); err != nil {
		t.Fatal(err)
	}
	if event.Reason != "low-confidence-evidence" {
		t.Fatalf("expected low-confidence-evidence, got %q", event.Reason)
	}
}

func TestProcessCitationFeedback_PrefersAppliedEvidenceOverRetrieved(t *testing.T) {
	tmp := t.TempDir()
	aoDir := filepath.Join(tmp, ".agents", "ao")
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(aoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	learningPath := filepath.Join(learningsDir, "mixed-evidence.jsonl")
	if err := os.WriteFile(learningPath, []byte(`{"id":"mixed-evidence","title":"Mixed Evidence","utility":0.5}`), 0o644); err != nil {
		t.Fatal(err)
	}

	citations := []types.CitationEvent{
		{ArtifactPath: ".agents/learnings/mixed-evidence.jsonl", CitationType: "retrieved", FeedbackGiven: false, CitedAt: time.Now().Add(-time.Minute)},
		{ArtifactPath: ".agents/learnings/mixed-evidence.jsonl", CitationType: "applied", FeedbackGiven: false, CitedAt: time.Now()},
	}
	var lines []string
	for _, citation := range citations {
		data, err := json.Marshal(citation)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, string(data))
	}
	if err := os.WriteFile(filepath.Join(aoDir, "citations.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	total, rewarded, skipped := processCitationFeedback(tmp)
	if total != 1 || rewarded != 1 || skipped != 0 {
		t.Fatalf("expected applied evidence to win, got (%d,%d,%d)", total, rewarded, skipped)
	}
}

func TestComputeSessionRewardForCloseLoop_NoTranscript(t *testing.T) {
	// Override HOME to an empty temp dir so no transcripts are found.
	fakeHome := filepath.Join(t.TempDir(), "emptyhome")
	if err := os.MkdirAll(fakeHome, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", fakeHome)

	// With no outcome file and no transcript, should return InitialUtility (neutral fallback)
	reward, err := computeSessionRewardForCloseLoop(t.TempDir())
	if err != nil {
		t.Errorf("expected nil error for graceful fallback, got: %v", err)
	}
	if reward != 0.5 {
		t.Errorf("expected InitialUtility (0.5) as fallback, got: %v", reward)
	}
}

func TestFlywheelCitationFeedback_FindingUpdatesCitationFields(t *testing.T) {
	tmp := t.TempDir()
	aoDir := filepath.Join(tmp, ".agents", "ao")
	findingsDir := filepath.Join(tmp, ".agents", SectionFindings)
	if err := os.MkdirAll(aoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(findingsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	findingPath := filepath.Join(findingsDir, "f-cited.md")
	if err := os.WriteFile(findingPath, []byte(`---
id: f-cited
title: Cited finding
status: active
hit_count: 1
---

# Cited finding

Summary.
`), 0o644); err != nil {
		t.Fatal(err)
	}

	citation := types.CitationEvent{
		ArtifactPath: filepath.Join(".agents", SectionFindings, "f-cited.md"),
		SessionID:    "session-1",
		CitedAt:      time.Date(2026, 3, 10, 1, 0, 0, 0, time.UTC),
	}
	data, err := json.Marshal(citation)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(aoDir, "citations.jsonl"), append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	total, rewarded, skipped := processCitationFeedback(tmp)
	if total != 1 || rewarded != 1 || skipped != 0 {
		t.Fatalf("expected (1,1,0), got (%d,%d,%d)", total, rewarded, skipped)
	}

	content, err := os.ReadFile(findingPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, "hit_count: 2") {
		t.Fatalf("expected updated hit_count, got:\n%s", text)
	}
	if !strings.Contains(text, "last_cited: 2026-03-10T01:00:00Z") {
		t.Fatalf("expected updated last_cited, got:\n%s", text)
	}
}

func TestPromoteCitedLearnings_NoFeedbackFile(t *testing.T) {
	dir := t.TempDir()
	count := promoteCitedLearnings(dir, true)
	if count != 0 {
		t.Errorf("expected 0 promoted for missing feedback file, got %d", count)
	}
}

func TestPromoteCitedLearnings_DryRunReturnsZero(t *testing.T) {
	dir := t.TempDir()
	feedbackDir := filepath.Join(dir, ".agents", "ao")
	if err := os.MkdirAll(feedbackDir, 0o755); err != nil {
		t.Fatal(err)
	}

	evt := FeedbackEvent{
		ArtifactPath: filepath.Join(dir, ".agents", "learnings", "test.md"),
		Reward:       1.0,
	}
	data, _ := json.Marshal(evt)
	if err := os.WriteFile(filepath.Join(feedbackDir, "feedback.jsonl"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	count := promoteCitedLearnings(dir, true)
	if count != 0 {
		t.Errorf("expected 0 in dry-run mode, got %d", count)
	}
}

func TestPromoteCitedLearnings_EmptyFeedbackFile(t *testing.T) {
	dir := t.TempDir()
	feedbackDir := filepath.Join(dir, ".agents", "ao")
	if err := os.MkdirAll(feedbackDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(feedbackDir, "feedback.jsonl"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	count := promoteCitedLearnings(dir, true)
	if count != 0 {
		t.Errorf("expected 0 for empty feedback, got %d", count)
	}
}

func TestPromoteCitedLearnings_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	feedbackDir := filepath.Join(dir, ".agents", "ao")
	if err := os.MkdirAll(feedbackDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(feedbackDir, "feedback.jsonl"), []byte("not json\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	count := promoteCitedLearnings(dir, true)
	if count != 0 {
		t.Errorf("expected 0 for malformed JSON, got %d", count)
	}
}
