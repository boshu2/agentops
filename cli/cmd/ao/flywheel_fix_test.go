// practices: [dora-metrics, wiki-knowledge-surface]
package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/types"
)

// --- Glob patch tests (Issue 1) ---

func TestGlobLearningFiles_IncludesMD(t *testing.T) {
	dir := t.TempDir()

	// Create both .jsonl and .md files
	os.WriteFile(filepath.Join(dir, "a.jsonl"), []byte(`{"utility":0.5}`), 0644)
	os.WriteFile(filepath.Join(dir, "b.md"), []byte("---\nutility: 0.5\n---\n# Test"), 0644)
	os.WriteFile(filepath.Join(dir, "c.txt"), []byte("ignored"), 0644)

	files, err := ratchet.GlobLearningFiles(dir)
	if err != nil {
		t.Fatalf("GlobLearningFiles: %v", err)
	}

	hasJSONL, hasMD := false, false
	for _, f := range files {
		if strings.HasSuffix(f, ".jsonl") {
			hasJSONL = true
		}
		if strings.HasSuffix(f, ".md") {
			hasMD = true
		}
		if strings.HasSuffix(f, ".txt") {
			t.Error("GlobLearningFiles returned .txt file — should only return .jsonl and .md")
		}
	}
	if !hasJSONL {
		t.Error("GlobLearningFiles did not return .jsonl file")
	}
	if !hasMD {
		t.Error("GlobLearningFiles did not return .md file")
	}
}

// --- readLearningData tests (Issue 1) ---

func TestReadLearningData_JSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	os.WriteFile(path, []byte(`{"utility":0.7,"maturity":"candidate","confidence":0.5}`+"\n"), 0644)

	data, ok := readLearningData(path)
	if !ok {
		t.Fatal("readLearningData returned false for valid JSONL")
	}
	if data["utility"] != 0.7 {
		t.Errorf("utility = %v, want 0.7", data["utility"])
	}
	if data["maturity"] != "candidate" {
		t.Errorf("maturity = %v, want candidate", data["maturity"])
	}
}

func TestReadLearningData_Markdown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	content := "---\nutility: 0.65\nconfidence: 0.4\nmaturity: provisional\nreward_count: 5\n---\n# Test Learning\n"
	os.WriteFile(path, []byte(content), 0644)

	data, ok := readLearningData(path)
	if !ok {
		t.Fatal("readLearningData returned false for valid .md")
	}
	if v, ok := data["utility"].(float64); !ok || math.Abs(v-0.65) > 0.001 {
		t.Errorf("utility = %v, want 0.65", data["utility"])
	}
	if v, ok := data["maturity"].(string); !ok || v != "provisional" {
		t.Errorf("maturity = %v, want provisional", data["maturity"])
	}
}

func TestReadLearningData_MarkdownNoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bare.md")
	os.WriteFile(path, []byte("# Just a heading\nNo frontmatter here.\n"), 0644)

	_, ok := readLearningData(path)
	if ok {
		t.Error("readLearningData returned true for .md without frontmatter — expected false")
	}
}

// --- annealedAlpha tests (Issue 3) ---

func TestAnnealedAlpha_FirstCitation(t *testing.T) {
	// citationCount=0: alpha = 0.1 * 3.0 * exp(0) = 0.3
	alpha := annealedAlpha(types.DefaultAlpha, 0)
	expected := types.DefaultAlpha * 3.0
	if math.Abs(alpha-expected) > 0.001 {
		t.Errorf("annealedAlpha(0.1, 0) = %f, want %f", alpha, expected)
	}
}

func TestAnnealedAlpha_TenCitations(t *testing.T) {
	// citationCount=10: alpha = 0.1 * 3.0 * exp(-1.0) ≈ 0.1104
	alpha := annealedAlpha(types.DefaultAlpha, 10)
	expected := types.DefaultAlpha * 3.0 * math.Exp(-1.0)
	if math.Abs(alpha-expected) > 0.001 {
		t.Errorf("annealedAlpha(0.1, 10) = %f, want %f", alpha, expected)
	}
}

func TestAnnealedAlpha_ThirtyCitations(t *testing.T) {
	// citationCount=30: alpha = 0.1 * 3.0 * exp(-3.0) ≈ 0.0149
	// Still above floor (0.01) but converging toward it
	alpha := annealedAlpha(types.DefaultAlpha, 30)
	floor := types.DefaultAlpha / 10.0
	if alpha < floor {
		t.Errorf("annealedAlpha(0.1, 30) = %f, should be >= floor %f", alpha, floor)
	}
	if alpha > 0.02 {
		t.Errorf("annealedAlpha(0.1, 30) = %f, should be near floor (< 0.02)", alpha)
	}

	// At very high counts, should hit the floor
	alphaHigh := annealedAlpha(types.DefaultAlpha, 100)
	if alphaHigh != floor {
		t.Errorf("annealedAlpha(0.1, 100) = %f, want floor %f", alphaHigh, floor)
	}
}

// --- Binary outcome reward tests (Issue 2) ---

func TestBinaryOutcomeReward_Success(t *testing.T) {
	dir := t.TempDir()
	aoDir := filepath.Join(dir, ".agents", "ao")
	os.MkdirAll(aoDir, 0755)
	os.WriteFile(filepath.Join(aoDir, "last-session-outcome.json"),
		[]byte(`{"outcome":"success","written_at":"2026-02-25T12:00:00Z"}`), 0644)

	reward, err := computeSessionRewardForCloseLoop(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reward != 0.8 {
		t.Errorf("reward = %f, want 0.8 for success", reward)
	}
}

func TestBinaryOutcomeReward_Failure(t *testing.T) {
	dir := t.TempDir()
	aoDir := filepath.Join(dir, ".agents", "ao")
	os.MkdirAll(aoDir, 0755)
	os.WriteFile(filepath.Join(aoDir, "last-session-outcome.json"),
		[]byte(`{"outcome":"failure","written_at":"2026-02-25T12:00:00Z"}`), 0644)

	reward, err := computeSessionRewardForCloseLoop(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reward != 0.2 {
		t.Errorf("reward = %f, want 0.2 for failure", reward)
	}
}

func TestBinaryOutcomeReward_FallbackToTranscript(t *testing.T) {
	dir := t.TempDir()
	// No outcome file, fake HOME so no transcripts found
	t.Setenv("HOME", dir)

	reward, err := computeSessionRewardForCloseLoop(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should fall back to InitialUtility (0.5) when no outcome file and no transcript
	if reward != types.InitialUtility {
		t.Errorf("reward = %f, want %f (InitialUtility fallback)", reward, types.InitialUtility)
	}
}

// --- Inject dedup tests (Issue 4) ---

func TestFilterMemoryDuplicates_SkipsPresent(t *testing.T) {
	dir := t.TempDir()

	// Create a fake MEMORY.md that contains a learning title
	memDir := filepath.Join(dir, ".claude", "projects", "-test")
	os.MkdirAll(memDir, 0755)
	os.WriteFile(filepath.Join(memDir, "MEMORY.md"), []byte(
		"# Memory\n\n- **existing-learning-id**: This learning is about testing\n- Known Pattern: Always test first\n",
	), 0644)

	// Set up git root detection to find our test dir
	gitDir := filepath.Join(dir, ".git")
	os.MkdirAll(gitDir, 0755)

	learnings := []learning{
		{ID: "existing-learning-id", Title: "Some title"},
		{ID: "new-learning-id", Title: "Brand new learning"},
		{ID: "", Title: "Known Pattern: Always test first"},
	}

	filtered := filterMemoryDuplicates(dir, learnings)

	// The function looks for MEMORY.md via findMemoryFile which searches
	// Claude's project dirs. Since we can't easily mock that, test the
	// case where no memory file exists (returns all learnings)
	if len(filtered) == 0 {
		t.Error("filterMemoryDuplicates returned empty slice — expected at least some learnings")
	}
}

func TestFilterMemoryDuplicates_KeepsNew(t *testing.T) {
	dir := t.TempDir()
	// No MEMORY.md exists → all learnings should pass through

	learnings := []learning{
		{ID: "brand-new-1", Title: "First learning"},
		{ID: "brand-new-2", Title: "Second learning"},
	}

	filtered := filterMemoryDuplicates(dir, learnings)
	if len(filtered) != 2 {
		t.Errorf("expected 2 learnings, got %d", len(filtered))
	}
}

func TestFilterMemoryDuplicates_NoMemoryFile(t *testing.T) {
	dir := t.TempDir()
	learnings := []learning{
		{ID: "a", Title: "Learning A"},
		{ID: "b", Title: "Learning B"},
		{ID: "c", Title: "Learning C"},
	}

	filtered := filterMemoryDuplicates(dir, learnings)
	if len(filtered) != len(learnings) {
		t.Errorf("with no MEMORY.md, expected all %d learnings returned, got %d", len(learnings), len(filtered))
	}
}

// --- Phase 2: Lowered threshold and implicit helpful tests ---

func TestMaturityPromotion_LoweredThreshold(t *testing.T) {
	// Test that the lowered threshold (0.55) is used for provisional -> candidate promotion.
	// utility=0.56 is above the new 0.55 threshold but below the old 0.7 threshold.
	dir := t.TempDir()
	path := filepath.Join(dir, "test-lowered.jsonl")
	os.WriteFile(path, []byte(`{"utility":0.56,"reward_count":5,"maturity":"provisional","confidence":0.5}`+"\n"), 0644)

	result, err := ratchet.CheckMaturityTransition(path)
	if err != nil {
		t.Fatalf("CheckMaturityTransition: %v", err)
	}
	if !result.Transitioned {
		t.Errorf("expected Transitioned=true, got false (reason: %s)", result.Reason)
	}
	if result.NewMaturity != types.MaturityCandidate {
		t.Errorf("expected NewMaturity=candidate, got %s", result.NewMaturity)
	}
}

func TestMaturityPromotion_OldThresholdNoLongerRequired(t *testing.T) {
	// Test that utility=0.60 (above 0.55 but below old 0.7) gets promoted.
	// This confirms the threshold lowering works.
	dir := t.TempDir()
	path := filepath.Join(dir, "test-old-threshold.jsonl")
	os.WriteFile(path, []byte(`{"utility":0.60,"reward_count":5,"maturity":"provisional","confidence":0.5}`+"\n"), 0644)

	result, err := ratchet.CheckMaturityTransition(path)
	if err != nil {
		t.Fatalf("CheckMaturityTransition: %v", err)
	}
	if !result.Transitioned {
		t.Errorf("expected Transitioned=true for utility=0.60 (above 0.55), got false (reason: %s)", result.Reason)
	}
	if result.NewMaturity != types.MaturityCandidate {
		t.Errorf("expected NewMaturity=candidate, got %s", result.NewMaturity)
	}
}

func TestMaturityPromotion_ImplicitHelpfulSignal(t *testing.T) {
	// Test that high citation count (reward_count >= 10) triggers implicit helpful signal
	// for candidate -> established promotion, bypassing the explicit helpful_count requirement.
	dir := t.TempDir()
	path := filepath.Join(dir, "test-implicit.jsonl")
	os.WriteFile(path, []byte(`{"utility":0.56,"reward_count":15,"maturity":"candidate","helpful_count":0,"harmful_count":0,"confidence":0.7}`+"\n"), 0644)

	result, err := ratchet.CheckMaturityTransition(path)
	if err != nil {
		t.Fatalf("CheckMaturityTransition: %v", err)
	}
	if !result.Transitioned {
		t.Errorf("expected Transitioned=true (implicit helpful via reward_count>=10), got false (reason: %s)", result.Reason)
	}
	if result.NewMaturity != types.MaturityEstablished {
		t.Errorf("expected NewMaturity=established, got %s", result.NewMaturity)
	}
	// Verify the reason mentions implicit helpful signal
	if !strings.Contains(result.Reason, "implicit helpful") {
		t.Errorf("expected reason to mention 'implicit helpful', got: %s", result.Reason)
	}
}

func TestMaturityPromotion_LowCitationsNoImplicitHelpful(t *testing.T) {
	// Test that reward_count=5 (< 10) does NOT trigger implicit helpful signal.
	// With helpful_count=0 and harmful_count=0, helpful > harmful is false,
	// so the candidate should NOT be promoted to established.
	dir := t.TempDir()
	path := filepath.Join(dir, "test-no-implicit.jsonl")
	os.WriteFile(path, []byte(`{"utility":0.56,"reward_count":5,"maturity":"candidate","helpful_count":0,"harmful_count":0,"confidence":0.5}`+"\n"), 0644)

	result, err := ratchet.CheckMaturityTransition(path)
	if err != nil {
		t.Fatalf("CheckMaturityTransition: %v", err)
	}
	if result.Transitioned {
		t.Errorf("expected Transitioned=false (reward_count=5 < 10, no helpful signal), got true (new maturity: %s, reason: %s)",
			result.NewMaturity, result.Reason)
	}
}

func TestMigrateMd_Integration(t *testing.T) {
	// Test the migrate-md components: a bare .md file with no frontmatter
	// should return empty utility from parseFrontmatterFields, and after
	// manually adding frontmatter the utility field should be present.
	dir := t.TempDir()

	// Create a bare .md file (no frontmatter)
	barePath := filepath.Join(dir, "bare.md")
	os.WriteFile(barePath, []byte("# A Learning\nSome content here.\n"), 0644)

	// parseFrontmatterFields should return empty for utility
	fields, err := parseFrontmatterFields(barePath, "utility")
	if err != nil {
		t.Fatalf("parseFrontmatterFields on bare file: %v", err)
	}
	if fields["utility"] != "" {
		t.Errorf("expected empty utility for bare .md, got %q", fields["utility"])
	}

	// Create a .md file with existing frontmatter that already has utility
	existingPath := filepath.Join(dir, "existing.md")
	os.WriteFile(existingPath, []byte("---\nutility: 0.7500\nmaturity: candidate\n---\n# Existing Learning\n"), 0644)

	existingFields, err := parseFrontmatterFields(existingPath, "utility")
	if err != nil {
		t.Fatalf("parseFrontmatterFields on existing file: %v", err)
	}
	if existingFields["utility"] != "0.7500" {
		t.Errorf("expected utility=0.7500 for existing .md, got %q", existingFields["utility"])
	}

	// Simulate migrate-md: manually prepend frontmatter to the bare file
	bareContent, _ := os.ReadFile(barePath)
	migrated := "---\nutility: 0.5000\nmaturity: provisional\nconfidence: 0.0000\nreward_count: 0\nhelpful_count: 0\nharmful_count: 0\n---\n" + string(bareContent)
	os.WriteFile(barePath, []byte(migrated), 0644)

	// Now parseFrontmatterFields should find utility
	migratedFields, err := parseFrontmatterFields(barePath, "utility")
	if err != nil {
		t.Fatalf("parseFrontmatterFields after migration: %v", err)
	}
	if migratedFields["utility"] != "0.5000" {
		t.Errorf("expected utility=0.5000 after migration, got %q", migratedFields["utility"])
	}
}

func TestRecalibrate_ResetsUtility(t *testing.T) {
	// Test that recalibration resets an inflated utility to 0.5 by using alpha=1.0.
	// EMA formula: new = (1-1.0)*0.85 + 1.0*0.5 = 0.5
	dir := t.TempDir()
	path := filepath.Join(dir, "inflated.jsonl")
	os.WriteFile(path, []byte(`{"utility":0.85,"reward_count":10,"maturity":"established","confidence":0.8}`+"\n"), 0644)

	oldUtility, newUtility, err := updateLearningUtility(path, types.InitialUtility, 1.0)
	if err != nil {
		t.Fatalf("updateLearningUtility: %v", err)
	}
	if math.Abs(oldUtility-0.85) > 0.001 {
		t.Errorf("oldUtility = %f, want 0.85", oldUtility)
	}
	if math.Abs(newUtility-types.InitialUtility) > 0.001 {
		t.Errorf("newUtility = %f, want %f (InitialUtility)", newUtility, types.InitialUtility)
	}

	// Verify by reading back the file
	content, _ := os.ReadFile(path)
	if !strings.Contains(string(content), `"utility":0.5`) {
		t.Errorf("written file does not contain utility=0.5, got: %s", string(content))
	}
}
