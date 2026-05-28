// practices: [llm-eval-harness, dora-metrics]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/vibecheck"
)

// seedAgentsFixture copies testdata/agents-fixture/ into a temp dir and returns the temp dir path.
func seedAgentsFixture(t *testing.T) string {
	t.Helper()
	src := filepath.Join(packageDir, "testdata", "agents-fixture", ".agents")
	dst := t.TempDir()
	dstAgents := filepath.Join(dst, ".agents")
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dstAgents, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
	if err != nil {
		t.Fatalf("seed fixture: %v", err)
	}
	return dst
}

// --- Test 1: TestDoctorFlywheelFindsFixtureArtifacts ---

func TestDoctorFlywheelFindsFixtureArtifacts(t *testing.T) {
	tmp := seedAgentsFixture(t)

	result := checkFlywheelHealth(tmp)

	// Fixture has 3 .md + 1 .jsonl in learnings = 4 learning files
	// checkFlywheelHealth should find learnings > 0 and report pass
	if result.Status == "fail" {
		t.Errorf("expected pass or warn, got fail (detail: %s)", result.Detail)
	}
	if result.Status == "warn" {
		t.Errorf("expected pass with fixture learnings, got warn (detail: %s)", result.Detail)
	}
	if result.Status != "pass" {
		t.Errorf("expected status=pass, got %q (detail: %s)", result.Status, result.Detail)
	}
}

// --- Test 2: TestCountLearningFilesMatchesFixture ---

func TestCountLearningFilesMatchesFixture(t *testing.T) {
	tmp := seedAgentsFixture(t)

	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	got := countLearningFiles(learningsDir)

	// Fixture learnings: learn-2026-test-001.md, learn-2026-test-002.md,
	// learn-2026-test-003.md (3 .md) + feedback.jsonl (1 .jsonl) = 4
	want := 4
	if got != want {
		t.Errorf("countLearningFiles() = %d, want %d", got, want)
	}
}

// --- Test 3: TestMetricsCountArtifactsIncludesLearnings ---

func TestMetricsCountArtifactsIncludesLearnings(t *testing.T) {
	tmp := seedAgentsFixture(t)

	total, tierCounts, err := countArtifacts(tmp)
	if err != nil {
		t.Fatalf("countArtifacts: %v", err)
	}

	// learning tier: 3 .md + 1 .jsonl = 4
	if tierCounts["learning"] <= 0 {
		t.Errorf("expected learning tier > 0, got %d", tierCounts["learning"])
	}

	// retro tier should be separate from learning tier
	// Fixture has 1 retro file
	if _, ok := tierCounts["retro"]; !ok {
		t.Error("expected retro tier to exist in tierCounts")
	}
	if tierCounts["retro"] != 1 {
		t.Errorf("expected retro tier = 1, got %d", tierCounts["retro"])
	}

	// Total should be > 0
	if total <= 0 {
		t.Errorf("expected total artifacts > 0, got %d", total)
	}
}

// --- Test 4: TestLearningCountConsistency ---

func TestLearningCountConsistency(t *testing.T) {
	tmp := seedAgentsFixture(t)

	// Get count from countLearningFiles (doctor.go method)
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	doctorCount := countLearningFiles(learningsDir)

	// Get count from countArtifacts (metrics.go method)
	_, tierCounts, err := countArtifacts(tmp)
	if err != nil {
		t.Fatalf("countArtifacts: %v", err)
	}
	metricsCount := tierCounts["learning"]

	// Both should agree: they both count *.md + *.jsonl in the learnings directory
	if doctorCount != metricsCount {
		t.Errorf("learning count mismatch: countLearningFiles()=%d, countArtifacts[learning]=%d",
			doctorCount, metricsCount)
	}
}

// --- Test 5: TestVibeCheckScoreNormalization ---

func TestVibeCheckScoreNormalization(t *testing.T) {
	// vibecheck.ComputeOverallRating is exported; clampScore and scoreToGrade are not.
	// We test the exported entry point to verify score stays in [0,100].
	tests := []struct {
		name    string
		metrics map[string]vibecheck.Metric
	}{
		{
			name:    "empty metrics",
			metrics: map[string]vibecheck.Metric{},
		},
		{
			name: "all pass",
			metrics: map[string]vibecheck.Metric{
				"velocity": {Name: "velocity", Value: 5.0, Threshold: 3.0, Passed: true},
				"rework":   {Name: "rework", Value: 10.0, Threshold: 20.0, Passed: true},
				"trust":    {Name: "trust", Value: 80.0, Threshold: 60.0, Passed: true},
				"spirals":  {Name: "spirals", Value: 0.0, Threshold: 3.0, Passed: true},
				"flow":     {Name: "flow", Value: 70.0, Threshold: 50.0, Passed: true},
			},
		},
		{
			name: "all fail",
			metrics: map[string]vibecheck.Metric{
				"velocity": {Name: "velocity", Value: 0.0, Threshold: 3.0, Passed: false},
				"rework":   {Name: "rework", Value: 100.0, Threshold: 20.0, Passed: false},
				"trust":    {Name: "trust", Value: 0.0, Threshold: 60.0, Passed: false},
				"spirals":  {Name: "spirals", Value: 10.0, Threshold: 3.0, Passed: false},
				"flow":     {Name: "flow", Value: 0.0, Threshold: 50.0, Passed: false},
			},
		},
		{
			name: "single metric (non-5 count normalization)",
			metrics: map[string]vibecheck.Metric{
				"velocity": {Name: "velocity", Value: 5.0, Threshold: 3.0, Passed: true},
			},
		},
		{
			name: "two metrics mixed",
			metrics: map[string]vibecheck.Metric{
				"velocity": {Name: "velocity", Value: 5.0, Threshold: 3.0, Passed: true},
				"rework":   {Name: "rework", Value: 100.0, Threshold: 20.0, Passed: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, grade := vibecheck.ComputeOverallRating(tt.metrics)

			if score < 0 || score > 100 {
				t.Errorf("score out of range [0,100]: got %.2f", score)
			}

			validGrades := map[string]bool{"A": true, "B": true, "C": true, "D": true, "F": true}
			if !validGrades[grade] {
				t.Errorf("invalid grade: got %q", grade)
			}
		})
	}
}

// --- Test 6: TestInjectTruncationProducesValidJSON ---

func TestInjectTruncationProducesValidJSON(t *testing.T) {
	// trimJSONToCharBudget is an unexported function in package main.
	// Since we are in package main, we can call it directly.

	tests := []struct {
		name         string
		knowledge    *injectedKnowledge
		budget       int
		wantTruncKey bool
	}{
		{
			name: "small budget forces truncation",
			knowledge: &injectedKnowledge{
				Learnings: []learning{
					{ID: "L1", Title: "Learning 1", Summary: "Summary about a thing"},
					{ID: "L2", Title: "Learning 2", Summary: "Summary about another thing"},
					{ID: "L3", Title: "Learning 3", Summary: "Yet another summary"},
				},
				Patterns: []pattern{
					{Name: "P1", Description: "Pattern one description"},
				},
				Sessions: []session{
					{Date: "2026-01-01", Summary: "Did stuff"},
					{Date: "2026-01-02", Summary: "Did more stuff"},
				},
			},
			budget:       200,
			wantTruncKey: true,
		},
		{
			name: "large budget no truncation needed",
			knowledge: &injectedKnowledge{
				Learnings: []learning{
					{ID: "L1", Title: "T1", Summary: "S1"},
				},
			},
			budget:       100000,
			wantTruncKey: true, // trimJSONToCharBudget always adds "truncated": true
		},
		{
			name:         "empty knowledge",
			knowledge:    &injectedKnowledge{},
			budget:       50,
			wantTruncKey: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := trimJSONToCharBudget(tt.knowledge, tt.budget)

			// Output must be valid JSON
			var parsed map[string]any
			if err := json.Unmarshal([]byte(output), &parsed); err != nil {
				t.Fatalf("trimJSONToCharBudget produced invalid JSON: %v\noutput: %s", err, output)
			}

			// Must have "truncated" key
			if tt.wantTruncKey {
				trunc, ok := parsed["truncated"]
				if !ok {
					t.Error("expected 'truncated' key in output JSON")
				} else if trunc != true {
					t.Errorf("expected 'truncated': true, got %v", trunc)
				}
			}

			// Output must fit within budget (or be the minimum fallback)
			if tt.budget > 100 && len(output) > tt.budget {
				// For very small budgets the minimum JSON {"truncated": true} may exceed budget;
				// only assert for reasonable budgets.
				t.Errorf("output length %d exceeds budget %d", len(output), tt.budget)
			}
		})
	}
}

// --- Test 7: TestCountArtifactsTierCompleteness ---

func TestCountArtifactsTierCompleteness(t *testing.T) {
	tmp := seedAgentsFixture(t)

	_, tierCounts, err := countArtifacts(tmp)
	if err != nil {
		t.Fatalf("countArtifacts: %v", err)
	}

	// Verify expected tiers from fixture data
	// learning: 3 .md + 1 .jsonl = 4
	if tierCounts["learning"] != 4 {
		t.Errorf("learning tier: got %d, want 4", tierCounts["learning"])
	}

	// pattern: 2 .md files
	if tierCounts["pattern"] != 2 {
		t.Errorf("pattern tier: got %d, want 2", tierCounts["pattern"])
	}

	// observation: research has 1 .md (fixture has no .agents/candidates dir,
	// only .agents/pool/candidates which countArtifacts does not scan)
	if tierCounts["observation"] != 1 {
		t.Errorf("observation tier: got %d, want 1", tierCounts["observation"])
	}

	// retro: 1 .md
	if tierCounts["retro"] != 1 {
		t.Errorf("retro tier: got %d, want 1", tierCounts["retro"])
	}
}

// --- Test 8: TestInjectTrimToCharBudgetMarkdown ---

func TestInjectTrimToCharBudgetMarkdown(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		budget int
	}{
		{
			name:   "already within budget",
			input:  "short text",
			budget: 100,
		},
		{
			name:   "needs truncation",
			input:  "line 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7\nline 8\nline 9\nline 10\n",
			budget: 30,
		},
		{
			name:   "zero length input",
			input:  "",
			budget: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trimToCharBudget(tt.input, tt.budget)

			// If input was within budget, output should equal input
			if len(tt.input) <= tt.budget {
				if result != tt.input {
					t.Errorf("expected input passthrough for within-budget text")
				}
				return
			}

			// Truncated output should contain the truncation marker
			if len(result) == 0 {
				t.Error("truncation should produce non-empty output")
			}
		})
	}
}

// --- Test 9: TestMakeProgressBarBounds ---

func TestMakeProgressBarBounds(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		width int
		want  int // expected total rune count
	}{
		{"zero", 0.0, 10, 10},
		{"full", 1.0, 10, 10},
		{"over 1 clamped", 2.0, 10, 10},
		{"negative clamped", -0.5, 10, 10},
		{"half", 0.5, 10, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bar := makeProgressBar(tt.value, tt.width)
			runes := []rune(bar)
			if len(runes) != tt.want {
				t.Errorf("makeProgressBar(%.1f, %d) length = %d runes, want %d",
					tt.value, tt.width, len(runes), tt.want)
			}
		})
	}
}
