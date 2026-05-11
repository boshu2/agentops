// practices: [dora-metrics, lean-startup]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/types"
)

func TestUpdateJSONLUtility(t *testing.T) {
	// Create temp directory for test files
	tmpDir, err := os.MkdirTemp("", "feedback_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
	}()

	tests := []struct {
		name           string
		initialContent string
		reward         float64
		alpha          float64
		wantOldUtility float64
		wantNewUtility float64
	}{
		{
			name:           "initial utility (no utility field)",
			initialContent: `{"id":"L001","title":"Test Learning"}`,
			reward:         1.0,
			alpha:          0.1,
			wantOldUtility: 0.5,  // InitialUtility
			wantNewUtility: 0.55, // (1-0.1)*0.5 + 0.1*1.0
		},
		{
			name:           "existing utility positive reward",
			initialContent: `{"id":"L002","title":"Test","utility":0.6}`,
			reward:         1.0,
			alpha:          0.1,
			wantOldUtility: 0.6,
			wantNewUtility: 0.64, // (1-0.1)*0.6 + 0.1*1.0
		},
		{
			name:           "existing utility negative reward",
			initialContent: `{"id":"L003","title":"Test","utility":0.6}`,
			reward:         0.0,
			alpha:          0.1,
			wantOldUtility: 0.6,
			wantNewUtility: 0.54, // (1-0.1)*0.6 + 0.1*0.0
		},
		{
			name:           "partial reward",
			initialContent: `{"id":"L004","title":"Test","utility":0.5}`,
			reward:         0.75,
			alpha:          0.1,
			wantOldUtility: 0.5,
			wantNewUtility: 0.525, // (1-0.1)*0.5 + 0.1*0.75
		},
		{
			name:           "higher alpha faster learning",
			initialContent: `{"id":"L005","title":"Test","utility":0.5}`,
			reward:         1.0,
			alpha:          0.3,
			wantOldUtility: 0.5,
			wantNewUtility: 0.65, // (1-0.3)*0.5 + 0.3*1.0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write test file
			path := filepath.Join(tmpDir, tt.name+".jsonl")
			if err := os.WriteFile(path, []byte(tt.initialContent), 0644); err != nil {
				t.Fatal(err)
			}

			// Update utility
			oldUtility, newUtility, err := updateJSONLUtility(path, tt.reward, tt.alpha)
			if err != nil {
				t.Fatalf("updateJSONLUtility() error = %v", err)
			}

			// Check old utility
			if abs(oldUtility-tt.wantOldUtility) > 0.001 {
				t.Errorf("oldUtility = %v, want %v", oldUtility, tt.wantOldUtility)
			}

			// Check new utility
			if abs(newUtility-tt.wantNewUtility) > 0.001 {
				t.Errorf("newUtility = %v, want %v", newUtility, tt.wantNewUtility)
			}

			// Verify file was updated correctly
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			var data map[string]any
			if err := json.Unmarshal(content, &data); err != nil {
				t.Fatalf("failed to parse updated file: %v", err)
			}

			// Verify utility was written
			utility, ok := data["utility"].(float64)
			if !ok {
				t.Error("utility field not found in updated file")
			}
			if abs(utility-tt.wantNewUtility) > 0.001 {
				t.Errorf("file utility = %v, want %v", utility, tt.wantNewUtility)
			}

			// Verify last_reward was written
			lastReward, ok := data["last_reward"].(float64)
			if !ok {
				t.Error("last_reward field not found")
			}
			if abs(lastReward-tt.reward) > 0.001 {
				t.Errorf("last_reward = %v, want %v", lastReward, tt.reward)
			}

			// Verify reward_count was incremented
			rewardCount, ok := data["reward_count"].(float64)
			if !ok {
				t.Error("reward_count field not found")
			}
			if rewardCount != 1 {
				t.Errorf("reward_count = %v, want 1", rewardCount)
			}
		})
	}
}

func TestCounterDirectionFromFeedback(t *testing.T) {
	tests := []struct {
		name            string
		reward          float64
		explicitHelpful bool
		explicitHarmful bool
		wantHelpful     bool
		wantHarmful     bool
	}{
		{
			name:            "explicit helpful wins",
			reward:          0.0,
			explicitHelpful: true,
			explicitHarmful: false,
			wantHelpful:     true,
			wantHarmful:     false,
		},
		{
			name:            "explicit harmful wins",
			reward:          1.0,
			explicitHelpful: false,
			explicitHarmful: true,
			wantHelpful:     false,
			wantHarmful:     true,
		},
		{
			name:            "high reward implied helpful",
			reward:          0.95,
			explicitHelpful: false,
			explicitHarmful: false,
			wantHelpful:     true,
			wantHarmful:     false,
		},
		{
			name:            "low reward implied harmful",
			reward:          0.05,
			explicitHelpful: false,
			explicitHarmful: false,
			wantHelpful:     false,
			wantHarmful:     true,
		},
		{
			name:            "mid reward neutral",
			reward:          0.5,
			explicitHelpful: false,
			explicitHarmful: false,
			wantHelpful:     false,
			wantHarmful:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotHelpful, gotHarmful := counterDirectionFromFeedback(tt.reward, tt.explicitHelpful, tt.explicitHarmful)
			if gotHelpful != tt.wantHelpful || gotHarmful != tt.wantHarmful {
				t.Errorf("counterDirectionFromFeedback(%v, %v, %v) = (%v, %v), want (%v, %v)",
					tt.reward, tt.explicitHelpful, tt.explicitHarmful,
					gotHelpful, gotHarmful, tt.wantHelpful, tt.wantHarmful)
			}
		})
	}
}

func TestUpdateJSONLUtilityImpliedCounters(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "counter.jsonl")
	content := `{"id":"L100","title":"Counter Test","utility":0.5}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	origHelpful := feedbackHelpful
	origHarmful := feedbackHarmful
	feedbackHelpful = false
	feedbackHarmful = false
	t.Cleanup(func() {
		feedbackHelpful = origHelpful
		feedbackHarmful = origHarmful
	})

	if _, _, err := updateJSONLUtility(path, 0.95, 0.1); err != nil {
		t.Fatalf("high-reward update failed: %v", err)
	}
	if _, _, err := updateJSONLUtility(path, 0.05, 0.1); err != nil {
		t.Fatalf("low-reward update failed: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if got := int(data["helpful_count"].(float64)); got != 1 {
		t.Errorf("helpful_count = %d, want 1", got)
	}
	if got := int(data["harmful_count"].(float64)); got != 1 {
		t.Errorf("harmful_count = %d, want 1", got)
	}
}

func TestUpdateMarkdownUtility(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "feedback_md_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
	}()

	tests := []struct {
		name           string
		initialContent string
		reward         float64
		alpha          float64
		wantOldUtility float64
		wantNewUtility float64
	}{
		{
			name: "no front matter",
			initialContent: `# Test Learning

This is the content.`,
			reward:         1.0,
			alpha:          0.1,
			wantOldUtility: types.InitialUtility,
			wantNewUtility: 0.55,
		},
		{
			name: "existing front matter without utility",
			initialContent: `---
id: L001
---
# Test Learning`,
			reward:         0.0,
			alpha:          0.1,
			wantOldUtility: types.InitialUtility,
			wantNewUtility: 0.45,
		},
		{
			name: "existing front matter with utility",
			initialContent: `---
utility: 0.7
---
# Test Learning`,
			reward:         1.0,
			alpha:          0.1,
			wantOldUtility: 0.7,
			wantNewUtility: 0.73,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, tt.name+".md")
			if err := os.WriteFile(path, []byte(tt.initialContent), 0644); err != nil {
				t.Fatal(err)
			}

			oldUtility, newUtility, err := updateMarkdownUtility(path, tt.reward, tt.alpha)
			if err != nil {
				t.Fatalf("updateMarkdownUtility() error = %v", err)
			}

			if abs(oldUtility-tt.wantOldUtility) > 0.001 {
				t.Errorf("oldUtility = %v, want %v", oldUtility, tt.wantOldUtility)
			}
			if abs(newUtility-tt.wantNewUtility) > 0.001 {
				t.Errorf("newUtility = %v, want %v", newUtility, tt.wantNewUtility)
			}
		})
	}
}

func TestUpdateMarkdownUtility_TracksHelpfulCount(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "helpful.md")
	content := `---
utility: 0.5
reward_count: 2
---
# Test Learning`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	origHelpful := feedbackHelpful
	origHarmful := feedbackHarmful
	feedbackHelpful = false
	feedbackHarmful = false
	t.Cleanup(func() {
		feedbackHelpful = origHelpful
		feedbackHarmful = origHarmful
	})

	// reward=0.95 is above 0.8 threshold → implied helpful
	if _, _, err := updateMarkdownUtility(path, 0.95, 0.1); err != nil {
		t.Fatalf("updateMarkdownUtility() error = %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "helpful_count: 1") {
		t.Errorf("expected helpful_count: 1 in front matter, got:\n%s", text)
	}
	// Should NOT contain harmful_count since reward was high
	if strings.Contains(text, "harmful_count:") {
		t.Errorf("unexpected harmful_count in front matter for helpful reward, got:\n%s", text)
	}
}

func TestUpdateMarkdownUtility_TracksHarmfulCount(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "harmful.md")
	content := `---
utility: 0.5
reward_count: 2
---
# Test Learning`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	origHelpful := feedbackHelpful
	origHarmful := feedbackHarmful
	feedbackHelpful = false
	feedbackHarmful = false
	t.Cleanup(func() {
		feedbackHelpful = origHelpful
		feedbackHarmful = origHarmful
	})

	// reward=0.05 is below 0.2 threshold → implied harmful
	if _, _, err := updateMarkdownUtility(path, 0.05, 0.1); err != nil {
		t.Fatalf("updateMarkdownUtility() error = %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "harmful_count: 1") {
		t.Errorf("expected harmful_count: 1 in front matter, got:\n%s", text)
	}
	// Should NOT contain helpful_count since reward was low
	if strings.Contains(text, "helpful_count:") {
		t.Errorf("unexpected helpful_count in front matter for harmful reward, got:\n%s", text)
	}
}

func TestUpdateMarkdownUtility_TracksConfidence(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "confidence.md")
	content := `---
utility: 0.5
reward_count: 4
---
# Test Learning`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	origHelpful := feedbackHelpful
	origHarmful := feedbackHarmful
	feedbackHelpful = false
	feedbackHarmful = false
	t.Cleanup(func() {
		feedbackHelpful = origHelpful
		feedbackHarmful = origHarmful
	})

	if _, _, err := updateMarkdownUtility(path, 0.5, 0.1); err != nil {
		t.Fatalf("updateMarkdownUtility() error = %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "confidence:") {
		t.Fatalf("expected confidence field in front matter, got:\n%s", text)
	}

	// reward_count was 4, now 5. confidence = 1 - 1/(1+5/5.0) = 1 - 1/2 = 0.5
	// Parse the confidence value from the file
	lines := strings.Split(text, "\n")
	var confidence float64
	found := false
	for _, line := range lines {
		if strings.HasPrefix(line, "confidence:") {
			if _, err := fmt.Sscanf(line, "confidence: %f", &confidence); err == nil {
				found = true
			}
			break
		}
	}
	if !found {
		t.Fatalf("could not parse confidence from front matter")
	}
	// Expected: 1 - 1/(1 + 5/5.0) = 0.5
	if abs(confidence-0.5) > 0.01 {
		t.Errorf("confidence = %.4f, want ~0.5", confidence)
	}
}

func TestUpdateMarkdownUtility_PreservesExistingFrontMatter(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "preserve.md")
	content := `---
id: test-001
category: debugging
utility: 0.5
reward_count: 2
---
# Test Learning

Some body content.`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	origHelpful := feedbackHelpful
	origHarmful := feedbackHarmful
	feedbackHelpful = false
	feedbackHarmful = false
	t.Cleanup(func() {
		feedbackHelpful = origHelpful
		feedbackHarmful = origHarmful
	})

	if _, _, err := updateMarkdownUtility(path, 0.8, 0.1); err != nil {
		t.Fatalf("updateMarkdownUtility() error = %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)

	// Verify original fields are preserved
	if !strings.Contains(text, "id: test-001") {
		t.Errorf("expected id: test-001 preserved, got:\n%s", text)
	}
	if !strings.Contains(text, "category: debugging") {
		t.Errorf("expected category: debugging preserved, got:\n%s", text)
	}
	// Verify body content is preserved
	if !strings.Contains(text, "Some body content.") {
		t.Errorf("expected body content preserved, got:\n%s", text)
	}
	// Verify new MemRL fields were added
	if !strings.Contains(text, "confidence:") {
		t.Errorf("expected confidence field added, got:\n%s", text)
	}
	if !strings.Contains(text, "last_decay_at:") {
		t.Errorf("expected last_decay_at field added, got:\n%s", text)
	}
}

func TestFindLearningFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "find_learning_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
	}()

	// Create .agents/learnings directory
	learningsDir := filepath.Join(tmpDir, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}
	patternsDir := filepath.Join(tmpDir, ".agents", "patterns")
	if err := os.MkdirAll(patternsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test files
	testFiles := []string{"L001.jsonl", "L002.md", "learning-003.jsonl"}
	for _, name := range testFiles {
		path := filepath.Join(learningsDir, name)
		if err := os.WriteFile(path, []byte(`{"id":"test"}`), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(patternsDir, "retry-backoff.md"), []byte("# Retry Backoff"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		learningID string
		wantFile   string
		wantErr    bool
	}{
		{
			name:       "find by ID (jsonl)",
			learningID: "L001",
			wantFile:   "L001.jsonl",
			wantErr:    false,
		},
		{
			name:       "find by ID (md)",
			learningID: "L002",
			wantFile:   "L002.md",
			wantErr:    false,
		},
		{
			name:       "find by partial match",
			learningID: "003",
			wantFile:   "learning-003.jsonl",
			wantErr:    false,
		},
		{
			name:       "find pattern by name",
			learningID: "retry-backoff",
			wantFile:   "retry-backoff.md",
			wantErr:    false,
		},
		{
			name:       "not found",
			learningID: "nonexistent",
			wantFile:   "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := findLearningFile(tmpDir, tt.learningID)
			if (err != nil) != tt.wantErr {
				t.Errorf("findLearningFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantFile != "" && filepath.Base(path) != tt.wantFile {
				t.Errorf("findLearningFile() = %v, want %v", filepath.Base(path), tt.wantFile)
			}
		})
	}
}

func TestNeedsUtilityMigration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "migration_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
	}()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "no utility field",
			content: `{"id":"L001","title":"Test"}`,
			want:    true,
		},
		{
			name:    "utility is zero",
			content: `{"id":"L002","utility":0}`,
			want:    true,
		},
		{
			name:    "has utility",
			content: `{"id":"L003","utility":0.5}`,
			want:    false,
		},
		{
			name:    "has high utility",
			content: `{"id":"L004","utility":0.9}`,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, tt.name+".jsonl")
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			got, err := needsUtilityMigration(path)
			if err != nil {
				t.Fatalf("needsUtilityMigration() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("needsUtilityMigration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAddUtilityField(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "add_utility_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
	}()

	content := `{"id":"L001","title":"Test Learning"}`
	path := filepath.Join(tmpDir, "test.jsonl")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := addUtilityField(path); err != nil {
		t.Fatalf("addUtilityField() error = %v", err)
	}

	// Read and verify
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	utility, ok := result["utility"].(float64)
	if !ok {
		t.Fatal("utility field not added")
	}
	if abs(utility-types.InitialUtility) > 0.001 {
		t.Errorf("utility = %v, want %v", utility, types.InitialUtility)
	}
}

// abs returns the absolute value of a float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// ---------------------------------------------------------------------------
// printFeedbackJSON (0%)
// ---------------------------------------------------------------------------

func TestPrintFeedbackJSON(t *testing.T) {
	// printFeedbackJSON writes to os.Stdout. Capture it.
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	printErr := printFeedbackJSON("L001", "/path/to/L001.jsonl", "helpful", 0.5, 0.55, 1.0, 0.1)

	_ = w.Close()
	os.Stdout = origStdout

	if printErr != nil {
		t.Fatalf("printFeedbackJSON() error = %v", printErr)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("expected valid JSON, got error: %v\nOutput: %s", err, output)
	}
	if result["learning_id"] != "L001" {
		t.Errorf("learning_id = %v, want L001", result["learning_id"])
	}
	if result["feedback_type"] != "helpful" {
		t.Errorf("feedback_type = %v, want helpful", result["feedback_type"])
	}
	if result["old_utility"].(float64) != 0.5 {
		t.Errorf("old_utility = %v, want 0.5", result["old_utility"])
	}
	if result["new_utility"].(float64) != 0.55 {
		t.Errorf("new_utility = %v, want 0.55", result["new_utility"])
	}
}

// ---------------------------------------------------------------------------
// runFeedback (0%) — dry-run path
// ---------------------------------------------------------------------------

func TestRunFeedback_DryRun(t *testing.T) {
	tmp := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prevWD) })

	// Create a learning file
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(learningsDir, "L001.jsonl"), []byte(`{"id":"L001","utility":0.5}`), 0644); err != nil {
		t.Fatal(err)
	}

	// Save/restore globals
	origDryRun := dryRun
	origReward := feedbackReward
	origAlpha := feedbackAlpha
	origHelpful := feedbackHelpful
	origHarmful := feedbackHarmful
	dryRun = true
	feedbackReward = -1
	feedbackAlpha = 0.1
	feedbackHelpful = true
	feedbackHarmful = false
	t.Cleanup(func() {
		dryRun = origDryRun
		feedbackReward = origReward
		feedbackAlpha = origAlpha
		feedbackHelpful = origHelpful
		feedbackHarmful = origHarmful
	})

	err = runFeedback(feedbackCmd, []string{"L001"})
	if err != nil {
		t.Fatalf("runFeedback() dry-run error = %v", err)
	}
}

// ---------------------------------------------------------------------------
// runFeedback (0%) — actual update path (text output)
// ---------------------------------------------------------------------------

func TestRunFeedback_ActualUpdate(t *testing.T) {
	tmp := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prevWD) })

	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(learningsDir, "L002.jsonl"), []byte(`{"id":"L002","utility":0.5}`), 0644); err != nil {
		t.Fatal(err)
	}

	origDryRun := dryRun
	origOutput := output
	origReward := feedbackReward
	origAlpha := feedbackAlpha
	origHelpful := feedbackHelpful
	origHarmful := feedbackHarmful
	dryRun = false
	output = "table"
	feedbackReward = -1
	feedbackAlpha = 0.1
	feedbackHelpful = false
	feedbackHarmful = true
	t.Cleanup(func() {
		dryRun = origDryRun
		output = origOutput
		feedbackReward = origReward
		feedbackAlpha = origAlpha
		feedbackHelpful = origHelpful
		feedbackHarmful = origHarmful
	})

	err = runFeedback(feedbackCmd, []string{"L002"})
	if err != nil {
		t.Fatalf("runFeedback() error = %v", err)
	}

	// Verify the file was updated
	raw, err := os.ReadFile(filepath.Join(learningsDir, "L002.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data["utility"] == nil {
		t.Error("expected utility field after update")
	}
}

// ---------------------------------------------------------------------------
// runFeedback — validation error (both helpful + harmful)
// ---------------------------------------------------------------------------

func TestRunFeedback_ValidationError(t *testing.T) {
	origReward := feedbackReward
	origAlpha := feedbackAlpha
	origHelpful := feedbackHelpful
	origHarmful := feedbackHarmful
	feedbackReward = -1
	feedbackAlpha = 0.1
	feedbackHelpful = true
	feedbackHarmful = true
	t.Cleanup(func() {
		feedbackReward = origReward
		feedbackAlpha = origAlpha
		feedbackHelpful = origHelpful
		feedbackHarmful = origHarmful
	})

	err := runFeedback(feedbackCmd, []string{"L999"})
	if err == nil {
		t.Error("expected error for both helpful+harmful")
	}
	if !strings.Contains(err.Error(), "cannot use both") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runFeedback — learning not found
// ---------------------------------------------------------------------------

func TestRunFeedback_LearningNotFound(t *testing.T) {
	tmp := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prevWD) })

	origReward := feedbackReward
	origAlpha := feedbackAlpha
	origHelpful := feedbackHelpful
	origHarmful := feedbackHarmful
	feedbackReward = 1.0
	feedbackAlpha = 0.1
	feedbackHelpful = false
	feedbackHarmful = false
	t.Cleanup(func() {
		feedbackReward = origReward
		feedbackAlpha = origAlpha
		feedbackHelpful = origHelpful
		feedbackHarmful = origHarmful
	})

	err = runFeedback(feedbackCmd, []string{"NONEXISTENT"})
	if err == nil {
		t.Error("expected error for missing learning")
	}
	if !strings.Contains(err.Error(), "find learning") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// runMigrate (0%)
// ---------------------------------------------------------------------------

func TestRunMigrate_UnknownMigration(t *testing.T) {
	err := runMigrate(migrateCmd, []string{"unknown"})
	if err == nil {
		t.Error("expected error for unknown migration")
	}
	if !strings.Contains(err.Error(), "unknown migration") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunMigrate_MemRL_NoLearnings(t *testing.T) {
	tmp := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prevWD) })

	err = runMigrate(migrateCmd, []string{"memrl"})
	if err != nil {
		t.Fatalf("runMigrate() error = %v", err)
	}
}

func TestRunMigrate_MemRL_WithFiles(t *testing.T) {
	tmp := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prevWD) })

	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}
	// File without utility (should be migrated)
	if err := os.WriteFile(filepath.Join(learningsDir, "L001.jsonl"), []byte(`{"id":"L001"}`), 0644); err != nil {
		t.Fatal(err)
	}
	// File with utility (should be skipped)
	if err := os.WriteFile(filepath.Join(learningsDir, "L002.jsonl"), []byte(`{"id":"L002","utility":0.7}`), 0644); err != nil {
		t.Fatal(err)
	}

	origDryRun := dryRun
	dryRun = false
	t.Cleanup(func() { dryRun = origDryRun })

	err = runMigrate(migrateCmd, []string{"memrl"})
	if err != nil {
		t.Fatalf("runMigrate() error = %v", err)
	}

	// Verify L001 was migrated
	raw, readErr := os.ReadFile(filepath.Join(learningsDir, "L001.jsonl"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if _, ok := data["utility"]; !ok {
		t.Error("expected utility field added to L001.jsonl")
	}
}

func TestRunMigrate_MemRL_DryRun(t *testing.T) {
	tmp := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prevWD) })

	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(learningsDir, "L001.jsonl"), []byte(`{"id":"L001"}`), 0644); err != nil {
		t.Fatal(err)
	}

	origDryRun := dryRun
	dryRun = true
	t.Cleanup(func() { dryRun = origDryRun })

	err = runMigrate(migrateCmd, []string{"memrl"})
	if err != nil {
		t.Fatalf("runMigrate() dry-run error = %v", err)
	}

	// Verify file was NOT changed
	raw, readErr := os.ReadFile(filepath.Join(learningsDir, "L001.jsonl"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if _, ok := data["utility"]; ok {
		t.Error("expected utility NOT added in dry-run mode")
	}
}

// ---------------------------------------------------------------------------
// migrateJSONLFiles — exercise dry-run and error paths
// ---------------------------------------------------------------------------

func TestMigrateJSONLFiles(t *testing.T) {
	tmp := t.TempDir()

	// Valid file needing migration
	f1 := filepath.Join(tmp, "needs.jsonl")
	if err := os.WriteFile(f1, []byte(`{"id":"L1"}`), 0644); err != nil {
		t.Fatal(err)
	}
	// Valid file already migrated
	f2 := filepath.Join(tmp, "done.jsonl")
	if err := os.WriteFile(f2, []byte(`{"id":"L2","utility":0.8}`), 0644); err != nil {
		t.Fatal(err)
	}
	// Invalid JSON file
	f3 := filepath.Join(tmp, "bad.jsonl")
	if err := os.WriteFile(f3, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("actual migration", func(t *testing.T) {
		migrated, skipped := migrateJSONLFiles([]string{f1, f2, f3}, false)
		if migrated != 1 {
			t.Errorf("migrated = %d, want 1", migrated)
		}
		if skipped != 1 {
			t.Errorf("skipped = %d, want 1", skipped)
		}
	})

	t.Run("dry-run migration", func(t *testing.T) {
		// Reset f1 to need migration again
		if err := os.WriteFile(f1, []byte(`{"id":"L1"}`), 0644); err != nil {
			t.Fatal(err)
		}
		migrated, skipped := migrateJSONLFiles([]string{f1, f2}, true)
		if migrated != 1 {
			t.Errorf("migrated = %d, want 1 in dry-run", migrated)
		}
		if skipped != 1 {
			t.Errorf("skipped = %d, want 1", skipped)
		}
	})
}

// ---------------------------------------------------------------------------
// classifyFeedbackType
// ---------------------------------------------------------------------------

func TestClassifyFeedbackType(t *testing.T) {
	if got := classifyFeedbackType(true, false); got != "helpful" {
		t.Errorf("got %q, want helpful", got)
	}
	if got := classifyFeedbackType(false, true); got != "harmful" {
		t.Errorf("got %q, want harmful", got)
	}
	if got := classifyFeedbackType(false, false); got != "custom" {
		t.Errorf("got %q, want custom", got)
	}
}

// ---------------------------------------------------------------------------
// parseJSONLFirstLine — error paths
// ---------------------------------------------------------------------------

func TestParseJSONLFirstLine_Empty(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "empty.jsonl")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	_, _, err := parseJSONLFirstLine(path)
	if err == nil {
		t.Error("expected error for empty JSONL file")
	}
}

func TestParseJSONLFirstLine_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "bad.jsonl")
	if err := os.WriteFile(path, []byte("not json\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, _, err := parseJSONLFirstLine(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// rebuildWithFrontMatter
// ---------------------------------------------------------------------------

func TestRebuildWithFrontMatter(t *testing.T) {
	fm := []string{"id: test", "utility: 0.5"}
	body := []string{"# Test", "", "Content here."}
	result := rebuildWithFrontMatter(fm, body)
	if !strings.HasPrefix(result, "---\n") {
		t.Error("expected front matter opening ---")
	}
	if !strings.Contains(result, "id: test") {
		t.Error("expected id field")
	}
	if !strings.Contains(result, "# Test") {
		t.Error("expected body content")
	}
}

// ---------------------------------------------------------------------------
// needsUtilityMigration — empty file
// ---------------------------------------------------------------------------

func TestNeedsUtilityMigration_EmptyFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "empty.jsonl")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := needsUtilityMigration(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("empty file should not need migration")
	}
}

func TestApplyJSONLRewardFields_IncrementExistingCounters(t *testing.T) {
	// When data already has helpful_count/harmful_count as float64 (from JSON),
	// the function should increment them rather than reset to 1.
	feedbackHelpful = true
	feedbackHarmful = false
	defer func() { feedbackHelpful = false }()

	data := map[string]any{
		"utility":       0.7,
		"reward_count":  float64(3),
		"helpful_count": float64(5),
	}
	applyJSONLRewardFields(data, 0.7, 0.8, 0.9)

	if data["helpful_count"] != 6 {
		t.Errorf("helpful_count = %v, want 6", data["helpful_count"])
	}
	if data["reward_count"] != 4 {
		t.Errorf("reward_count = %v, want 4", data["reward_count"])
	}
	if data["utility"] != 0.8 {
		t.Errorf("utility = %v, want 0.8", data["utility"])
	}
}

func TestApplyJSONLRewardFields_IncrementHarmfulCounter(t *testing.T) {
	feedbackHelpful = false
	feedbackHarmful = true
	defer func() { feedbackHarmful = false }()

	data := map[string]any{
		"utility":       0.5,
		"harmful_count": float64(2),
	}
	applyJSONLRewardFields(data, 0.5, 0.3, 0.1)

	if data["harmful_count"] != 3 {
		t.Errorf("harmful_count = %v, want 3", data["harmful_count"])
	}
}
