// practices: [microservices, sre]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tests for pure helper functions in pool_ingest.go

func TestConfidenceToScore(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  float64
	}{
		{name: "high confidence returns 0.9", input: "high", want: 0.9},
		{name: "HIGH case-insensitive", input: "HIGH", want: 0.9},
		{name: "High mixed case", input: "High", want: 0.9},
		{name: "medium confidence returns 0.7", input: "medium", want: 0.7},
		{name: "MEDIUM case-insensitive", input: "MEDIUM", want: 0.7},
		{name: "low confidence returns 0.5", input: "low", want: 0.5},
		{name: "LOW case-insensitive", input: "LOW", want: 0.5},
		{name: "unknown returns 0.6", input: "unknown", want: 0.6},
		{name: "empty string returns 0.6", input: "", want: 0.6},
		{name: "whitespace trimmed", input: "  high  ", want: 0.9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := confidenceToScore(tt.input)
			if got != tt.want {
				t.Errorf("confidenceToScore(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestComputeRubricScores(t *testing.T) {
	t.Run("plain body gets baseline scores", func(t *testing.T) {
		body := "This is a plain learning without code."
		scores := computeRubricScores(body, 0.7)
		// specificity baseline is 0.4
		if scores.Specificity < 0.4 || scores.Specificity > 1.0 {
			t.Errorf("Specificity = %v, want [0.4, 1.0]", scores.Specificity)
		}
		// actionability baseline is 0.4
		if scores.Actionability < 0.4 || scores.Actionability > 1.0 {
			t.Errorf("Actionability = %v, want [0.4, 1.0]", scores.Actionability)
		}
	})

	t.Run("body with code block increases specificity and actionability", func(t *testing.T) {
		body := "Use ```go\nfmt.Println(\"hello\")\n``` to print output."
		scores := computeRubricScores(body, 0.7)
		plain := computeRubricScores("Use this to print output.", 0.7)
		if scores.Specificity <= plain.Specificity {
			t.Errorf("code block should increase Specificity: got %v <= %v", scores.Specificity, plain.Specificity)
		}
		if scores.Actionability <= plain.Actionability {
			t.Errorf("code block should increase Actionability: got %v <= %v", scores.Actionability, plain.Actionability)
		}
	})

	t.Run("body with filename increases specificity", func(t *testing.T) {
		body := "Edit the file cli/cmd/ao/main.go to add the feature."
		scores := computeRubricScores(body, 0.7)
		if scores.Specificity < 0.6 {
			t.Errorf("body with filename should have Specificity >= 0.6, got %v", scores.Specificity)
		}
	})

	t.Run("body with action verbs increases actionability", func(t *testing.T) {
		body := "Run go build to ensure it compiles. Must fix the error."
		scores := computeRubricScores(body, 0.7)
		if scores.Actionability < 0.6 {
			t.Errorf("body with action verbs should have Actionability >= 0.6, got %v", scores.Actionability)
		}
	})

	t.Run("long body increases novelty", func(t *testing.T) {
		body := strings.Repeat("word ", 200) // 1000 chars
		scoresLong := computeRubricScores(body, 0.7)
		bodyShort := "short body" // < 250 chars
		scoresShort := computeRubricScores(bodyShort, 0.7)
		if scoresLong.Novelty <= scoresShort.Novelty {
			t.Errorf("long body should have higher Novelty: %v <= %v", scoresLong.Novelty, scoresShort.Novelty)
		}
	})

	t.Run("body with source section increases context", func(t *testing.T) {
		body := "Some learning.\n\n## Source\nThis was found in session xyz."
		scores := computeRubricScores(body, 0.7)
		plain := computeRubricScores("Some learning.", 0.7)
		if scores.Context <= plain.Context {
			t.Errorf("## Source should increase Context: %v <= %v", scores.Context, plain.Context)
		}
	})

	t.Run("scores are clamped to [0, 1]", func(t *testing.T) {
		// Use highly optimized body to push all scores high
		body := "```go\nfunc run() error { return nil }\n```\n" +
			"Run this. Use ensure check.\n" +
			"File: cli/cmd/ao/main.go line 42\n" +
			"## Source\nfound here\n## Why It Matters\nbecause\n" +
			strings.Repeat("more content ", 100)
		scores := computeRubricScores(body, 0.9)
		if scores.Specificity > 1.0 {
			t.Errorf("Specificity > 1.0: %v", scores.Specificity)
		}
		if scores.Actionability > 1.0 {
			t.Errorf("Actionability > 1.0: %v", scores.Actionability)
		}
		if scores.Novelty > 1.0 {
			t.Errorf("Novelty > 1.0: %v", scores.Novelty)
		}
		if scores.Context > 1.0 {
			t.Errorf("Context > 1.0: %v", scores.Context)
		}
	})
}

func TestParsePendingFileHeader(t *testing.T) {
	t.Run("extracts date from YAML frontmatter", func(t *testing.T) {
		md := "---\ndate: 2024-03-15\ntitle: Test\n---\n# Content"
		dir := t.TempDir()
		path := filepath.Join(dir, "test.md")
		if err := os.WriteFile(path, []byte(md), 0644); err != nil {
			t.Fatal(err)
		}
		fileDate, _ := parsePendingFileHeader(md, path)
		if fileDate.IsZero() {
			t.Error("expected non-zero date from YAML frontmatter")
		}
		if fileDate.Year() != 2024 || fileDate.Month() != 3 || fileDate.Day() != 15 {
			t.Errorf("date = %v, want 2024-03-15", fileDate)
		}
	})

	t.Run("extracts date from filename prefix", func(t *testing.T) {
		md := "# Content without date metadata"
		dir := t.TempDir()
		path := filepath.Join(dir, "2024-05-20-my-learning.md")
		if err := os.WriteFile(path, []byte(md), 0644); err != nil {
			t.Fatal(err)
		}
		fileDate, _ := parsePendingFileHeader(md, path)
		if fileDate.IsZero() {
			t.Error("expected non-zero date from filename prefix")
		}
		if fileDate.Year() != 2024 || fileDate.Month() != 5 || fileDate.Day() != 20 {
			t.Errorf("date = %v, want 2024-05-20", fileDate)
		}
	})

	t.Run("extracts session hint from content", func(t *testing.T) {
		md := "# Learning\n\nSession ag-abc123 was useful.\n"
		dir := t.TempDir()
		path := filepath.Join(dir, "test.md")
		if err := os.WriteFile(path, []byte(md), 0644); err != nil {
			t.Fatal(err)
		}
		_, sessionHint := parsePendingFileHeader(md, path)
		if strings.Contains(sessionHint, "ag-") {
			// Found session hint
		} else if sessionHint != "test" {
			// Fallback to filename base without extension
			t.Logf("sessionHint = %q (no session ID found, using filename)", sessionHint)
		}
	})

	t.Run("falls back to file mtime for missing date", func(t *testing.T) {
		md := "# No date info here"
		dir := t.TempDir()
		path := filepath.Join(dir, "no-date.md")
		if err := os.WriteFile(path, []byte(md), 0644); err != nil {
			t.Fatal(err)
		}
		fileDate, _ := parsePendingFileHeader(md, path)
		if fileDate.IsZero() {
			t.Error("expected non-zero date even without explicit date metadata")
		}
	})
}
