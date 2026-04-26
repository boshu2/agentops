package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

func TestApplyConfidenceDecay_WritesBack(t *testing.T) {
	// Create temp JSONL file with confidence=0.8 and last_decay_at 2 weeks ago
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test-learning.jsonl")

	twoWeeksAgo := time.Now().Add(-14 * 24 * time.Hour)
	data := map[string]any{
		"id":            "test-1",
		"title":         "Test Learning",
		"summary":       "A test learning for decay",
		"confidence":    0.8,
		"last_decay_at": twoWeeksAgo.Format(time.RFC3339),
		"utility":       0.5,
	}
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filePath, jsonBytes, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Call applyConfidenceDecay
	l := learning{
		ID:      "test-1",
		Title:   "Test Learning",
		Source:  filePath,
		Utility: 0.5,
	}
	now := time.Now()
	_ = applyConfidenceDecay(l, filePath, now)

	// Read back and verify
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(content, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	// Verify confidence was decayed
	newConf, ok := result["confidence"].(float64)
	if !ok {
		t.Fatal("confidence not found in result")
	}

	// Expected: 0.8 * exp(-2 * 0.1) = 0.8 * exp(-0.2) ≈ 0.654
	expectedConf := 0.8 * math.Exp(-2.0*types.ConfidenceDecayRate)
	if math.Abs(newConf-expectedConf) > 0.01 {
		t.Errorf("confidence = %f, want ~%f", newConf, expectedConf)
	}

	// Verify last_decay_at was updated
	lastDecay, ok := result["last_decay_at"].(string)
	if !ok || lastDecay == "" {
		t.Fatal("last_decay_at not found or empty")
	}
	parsedDecay, err := time.Parse(time.RFC3339, lastDecay)
	if err != nil {
		t.Fatalf("parse last_decay_at: %v", err)
	}
	if now.Sub(parsedDecay) > time.Second {
		t.Errorf("last_decay_at = %v, want ~%v", parsedDecay, now)
	}

	// Verify decay_count = 1
	decayCount, ok := result["decay_count"].(float64)
	if !ok {
		t.Fatal("decay_count not found in result")
	}
	if decayCount != 1 {
		t.Errorf("decay_count = %f, want 1", decayCount)
	}
}

func TestApplyConfidenceDecay_SkipsMarkdown(t *testing.T) {
	// Create a temp .md file
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test-learning.md")
	originalContent := "# Test Learning\n\nSome content here.\n"
	if err := os.WriteFile(filePath, []byte(originalContent), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	l := learning{
		ID:      "test-md",
		Title:   "Test Learning",
		Source:  filePath,
		Utility: 0.5,
	}
	now := time.Now()
	result := applyConfidenceDecay(l, filePath, now)

	// Verify file was not modified
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(content) != originalContent {
		t.Errorf("markdown file was modified: got %q, want %q", string(content), originalContent)
	}

	// Verify learning was returned unchanged
	if result.Utility != 0.5 {
		t.Errorf("utility changed for markdown file: got %f, want 0.5", result.Utility)
	}
}

func TestApplyConfidenceDecay_ClampsMinimum(t *testing.T) {
	// Create temp JSONL with very old last_decay_at to force confidence below 0.1
	dir := t.TempDir()
	filePath := filepath.Join(dir, "old-learning.jsonl")

	// 100 weeks ago — decay factor = exp(-100 * 0.1) ≈ 0.0000454
	// 0.5 * 0.0000454 ≈ 0.0000227, should clamp to 0.1
	longAgo := time.Now().Add(-100 * 7 * 24 * time.Hour)
	data := map[string]any{
		"id":            "test-clamp",
		"title":         "Very Old Learning",
		"confidence":    0.5,
		"last_decay_at": longAgo.Format(time.RFC3339),
		"utility":       0.5,
	}
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filePath, jsonBytes, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	l := learning{
		ID:      "test-clamp",
		Title:   "Very Old Learning",
		Source:  filePath,
		Utility: 0.5,
	}
	now := time.Now()
	_ = applyConfidenceDecay(l, filePath, now)

	// Read back and verify confidence is clamped to 0.1
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(content, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	newConf, ok := result["confidence"].(float64)
	if !ok {
		t.Fatal("confidence not found in result")
	}
	if newConf != 0.1 {
		t.Errorf("confidence = %f, want 0.1 (clamped minimum)", newConf)
	}
}

// TestParseConfidenceField pins the default-on-missing behavior so a learning
// file without a confidence frontmatter field still gets the 0.5 baseline that
// the decay calculation expects.
func TestParseConfidenceField(t *testing.T) {
	cases := []struct {
		name   string
		fields map[string]string
		want   float64
	}{
		{"missing field returns default 0.5", map[string]string{}, 0.5},
		{"empty string returns default", map[string]string{"confidence": ""}, 0.5},
		{"unparseable returns default", map[string]string{"confidence": "abc"}, 0.5},
		{"zero collapses to default", map[string]string{"confidence": "0"}, 0.5},
		{"negative collapses to default", map[string]string{"confidence": "-0.4"}, 0.5},
		{"valid float passes through", map[string]string{"confidence": "0.75"}, 0.75},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseConfidenceField(tc.fields)
			if got != tc.want {
				t.Errorf("parseConfidenceField(%v) = %v, want %v", tc.fields, got, tc.want)
			}
		})
	}
}

// TestMostRecentInteraction asserts the helper picks the latest of
// last_decay_at / last_reward_at, returns zero on absence, and ignores
// unparseable timestamps without confusing later parses.
func TestMostRecentInteraction(t *testing.T) {
	earlier := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	later := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	t.Run("both present, returns later", func(t *testing.T) {
		got := mostRecentInteraction(map[string]string{
			"last_decay_at":  earlier.Format(time.RFC3339),
			"last_reward_at": later.Format(time.RFC3339),
		})
		if !got.Equal(later) {
			t.Errorf("got %v, want %v", got, later)
		}
	})

	t.Run("only decay present", func(t *testing.T) {
		got := mostRecentInteraction(map[string]string{
			"last_decay_at": later.Format(time.RFC3339),
		})
		if !got.Equal(later) {
			t.Errorf("got %v, want %v", got, later)
		}
	})

	t.Run("neither present returns zero time", func(t *testing.T) {
		got := mostRecentInteraction(map[string]string{})
		if !got.IsZero() {
			t.Errorf("expected zero time, got %v", got)
		}
	})

	t.Run("unparseable values skipped, valid still wins", func(t *testing.T) {
		got := mostRecentInteraction(map[string]string{
			"last_decay_at":  "not-a-timestamp",
			"last_reward_at": later.Format(time.RFC3339),
		})
		if !got.Equal(later) {
			t.Errorf("got %v, want %v", got, later)
		}
	})
}

// TestFindFrontmatterEnd documents the lookup semantics: returns the closing
// `---` after line 0, or -1 when no closer is present.
func TestFindFrontmatterEnd(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  int
	}{
		{"empty slice", []string{}, -1},
		{"only opener", []string{"---", "key: value"}, -1},
		{"opener + closer", []string{"---", "key: value", "---", "body"}, 2},
		{"closer with whitespace", []string{"---", "key: value", "  ---  ", "body"}, 2},
		{"first closer wins on multiple", []string{"---", "k: v", "---", "x: y", "---"}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := findFrontmatterEnd(tc.lines)
			if got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}
