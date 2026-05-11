// practices: [dora-metrics, lean-startup]
package main

import (
	"testing"
)

// Tests for pure helper functions in feedback.go

func TestIncrementRewardCount(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  string
	}{
		{
			name:  "no reward_count line starts at 1",
			lines: []string{"title: test", "utility: 0.5"},
			want:  "1",
		},
		{
			name:  "existing reward_count 0 increments to 1",
			lines: []string{"reward_count: 0"},
			want:  "1",
		},
		{
			name:  "existing reward_count 5 increments to 6",
			lines: []string{"title: test", "reward_count: 5", "utility: 0.7"},
			want:  "6",
		},
		{
			name:  "empty lines starts at 1",
			lines: []string{},
			want:  "1",
		},
		{
			name:  "malformed reward_count line treats as 0",
			lines: []string{"reward_count: abc"},
			want:  "1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := incrementRewardCount(tt.lines)
			if got != tt.want {
				t.Errorf("incrementRewardCount(%v) = %q, want %q", tt.lines, got, tt.want)
			}
		})
	}
}

func TestUpdateFrontMatterFields(t *testing.T) {
	t.Run("updates existing field", func(t *testing.T) {
		lines := []string{"utility: 0.5000", "title: test"}
		fields := map[string]string{"utility": "0.7000"}
		result := updateFrontMatterFields(lines, fields)
		found := false
		for _, line := range result {
			if line == "utility: 0.7000" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected utility updated, got: %v", result)
		}
	})

	t.Run("adds missing field", func(t *testing.T) {
		lines := []string{"title: test"}
		fields := map[string]string{"utility": "0.5000"}
		result := updateFrontMatterFields(lines, fields)
		found := false
		for _, line := range result {
			if line == "utility: 0.5000" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected utility added, got: %v", result)
		}
	})

	t.Run("preserves unrelated fields", func(t *testing.T) {
		lines := []string{"title: test", "category: learning"}
		fields := map[string]string{"utility": "0.5000"}
		result := updateFrontMatterFields(lines, fields)
		foundTitle := false
		foundCategory := false
		for _, line := range result {
			if line == "title: test" {
				foundTitle = true
			}
			if line == "category: learning" {
				foundCategory = true
			}
		}
		if !foundTitle || !foundCategory {
			t.Errorf("expected unrelated fields preserved, got: %v", result)
		}
	})

	t.Run("empty lines with new field adds field", func(t *testing.T) {
		lines := []string{}
		fields := map[string]string{"utility": "0.5000"}
		result := updateFrontMatterFields(lines, fields)
		if len(result) != 1 {
			t.Errorf("expected 1 line, got %d: %v", len(result), result)
		}
	})
}
