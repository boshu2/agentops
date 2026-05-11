// practices: [microservices, dora-metrics]
package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

// ===========================================================================
// batch_forge.go helpers
// ===========================================================================

func TestBatchForge_humanSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero bytes", 0, "0 B"},
		{"small bytes", 50, "50 B"},
		{"exactly 1023", 1023, "1023 B"},
		{"exactly 1 KB", 1024, "1.0 KB"},
		{"1.5 KB", 1536, "1.5 KB"},
		{"1 MB", 1024 * 1024, "1.0 MB"},
		{"2.5 MB", int64(2.5 * 1024 * 1024), "2.5 MB"},
		{"1 GB", 1024 * 1024 * 1024, "1.0 GB"},
		{"large file", 5 * 1024 * 1024 * 1024, "5.0 GB"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := humanSize(tc.bytes)
			if got != tc.want {
				t.Errorf("humanSize(%d) = %q, want %q", tc.bytes, got, tc.want)
			}
		})
	}
}

func TestBatchForge_normalizeForDedup(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		same bool
	}{
		{"exact duplicates", "hello world", "hello world", true},
		{"case insensitive", "Hello World", "hello world", true},
		{"extra whitespace", "hello   world", "hello world", true},
		{"trailing ellipsis", "hello world...", "hello world", true},
		{"leading/trailing space", "  hello world  ", "hello world", true},
		{"different content", "hello", "world", false},
		{"prefix not matching full", "hello world extended", "hello world", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			keyA := normalizeForDedup(tc.a)
			keyB := normalizeForDedup(tc.b)
			if (keyA == keyB) != tc.same {
				t.Errorf("normalizeForDedup(%q) == normalizeForDedup(%q) = %v, want %v",
					tc.a, tc.b, keyA == keyB, tc.same)
			}
		})
	}

	// Key should be a hex-encoded SHA256 (64 chars)
	t.Run("key is hex sha256", func(t *testing.T) {
		key := normalizeForDedup("test")
		if len(key) != 64 {
			t.Errorf("normalizeForDedup key length = %d, want 64 (SHA256 hex)", len(key))
		}
	})
}

func TestBatchForge_dedupSimilar(t *testing.T) {
	tests := []struct {
		name  string
		items []string
		want  int
	}{
		{"nil input", nil, 0},
		{"empty input", []string{}, 0},
		{"no duplicates", []string{"alpha", "beta", "gamma"}, 3},
		{"exact duplicates", []string{"foo", "foo", "bar"}, 2},
		{"case duplicates", []string{"Hello", "hello", "HELLO"}, 1},
		{"whitespace duplicates", []string{"a  b", "a b"}, 1},
		{"ellipsis duplicates", []string{"learning about go...", "learning about go"}, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := dedupSimilar(tc.items)
			if tc.want == 0 {
				if got != nil {
					t.Errorf("dedupSimilar() = %v, want nil", got)
				}
				return
			}
			if len(got) != tc.want {
				t.Errorf("dedupSimilar() len = %d, want %d; items: %v", len(got), tc.want, got)
			}
		})
	}
}

func TestBatchForge_filterUnforgedTranscripts(t *testing.T) {
	now := time.Now()
	transcripts := []transcriptCandidate{
		{path: "/a.jsonl", modTime: now, size: 200},
		{path: "/b.jsonl", modTime: now, size: 300},
		{path: "/c.jsonl", modTime: now, size: 400},
	}

	tests := []struct {
		name        string
		forgedSet   map[string]bool
		wantLen     int
		wantSkipped int
	}{
		{"none forged", map[string]bool{}, 3, 0},
		{"all forged", map[string]bool{"/a.jsonl": true, "/b.jsonl": true, "/c.jsonl": true}, 0, 3},
		{"partial forged", map[string]bool{"/b.jsonl": true}, 2, 1},
		{"nil forged set", nil, 3, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			unforged, skipped := filterUnforgedTranscripts(transcripts, tc.forgedSet)
			if len(unforged) != tc.wantLen {
				t.Errorf("unforged len = %d, want %d", len(unforged), tc.wantLen)
			}
			if skipped != tc.wantSkipped {
				t.Errorf("skipped = %d, want %d", skipped, tc.wantSkipped)
			}
		})
	}
}

func TestBatchForge_accumulateSuccess(t *testing.T) {
	var acc batchForgeAccumulator

	// Accumulate a success
	acc.accumulate(true, []string{"d1", "d2"}, []string{"k1"}, "/path/a")
	if acc.processed != 1 {
		t.Errorf("processed = %d, want 1", acc.processed)
	}
	if acc.failed != 0 {
		t.Errorf("failed = %d, want 0", acc.failed)
	}
	if acc.totalDecisions != 2 {
		t.Errorf("totalDecisions = %d, want 2", acc.totalDecisions)
	}
	if acc.totalKnowledge != 1 {
		t.Errorf("totalKnowledge = %d, want 1", acc.totalKnowledge)
	}
	if len(acc.allKnowledge) != 1 {
		t.Errorf("allKnowledge len = %d, want 1", len(acc.allKnowledge))
	}
	if len(acc.allDecisions) != 2 {
		t.Errorf("allDecisions len = %d, want 2", len(acc.allDecisions))
	}
	if len(acc.processedPaths) != 1 || acc.processedPaths[0] != "/path/a" {
		t.Errorf("processedPaths = %v, want [/path/a]", acc.processedPaths)
	}

	// Accumulate a failure
	acc.accumulate(false, nil, nil, "")
	if acc.failed != 1 {
		t.Errorf("failed = %d, want 1", acc.failed)
	}
	if acc.processed != 1 {
		t.Errorf("processed should still be 1, got %d", acc.processed)
	}

	// Accumulate another success
	acc.accumulate(true, []string{"d3"}, []string{"k2", "k3"}, "/path/b")
	if acc.processed != 2 {
		t.Errorf("processed = %d, want 2", acc.processed)
	}
	if acc.totalDecisions != 3 {
		t.Errorf("totalDecisions = %d, want 3", acc.totalDecisions)
	}
	if acc.totalKnowledge != 3 {
		t.Errorf("totalKnowledge = %d, want 3", acc.totalKnowledge)
	}
}

func TestBatchForge_loadForgedIndex(t *testing.T) {
	t.Run("nonexistent file returns empty set", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "forged.jsonl")
		set, err := loadForgedIndex(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(set) != 0 {
			t.Errorf("expected empty set, got %d entries", len(set))
		}
	})

	t.Run("valid entries", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "forged.jsonl")

		r1 := ForgedRecord{Path: "/transcript/a.jsonl", ForgedAt: time.Now(), Session: "s1"}
		r2 := ForgedRecord{Path: "/transcript/b.jsonl", ForgedAt: time.Now(), Session: "s2"}
		d1, _ := json.Marshal(r1)
		d2, _ := json.Marshal(r2)
		content := string(d1) + "\n" + string(d2) + "\n"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		set, err := loadForgedIndex(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(set) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(set))
		}
		if !set["/transcript/a.jsonl"] {
			t.Error("missing /transcript/a.jsonl")
		}
		if !set["/transcript/b.jsonl"] {
			t.Error("missing /transcript/b.jsonl")
		}
	})

	t.Run("skips malformed lines and blank lines", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "forged.jsonl")

		r := ForgedRecord{Path: "/good.jsonl", ForgedAt: time.Now()}
		d, _ := json.Marshal(r)
		content := string(d) + "\nnot-json\n\n{\"bad\": true}\n"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		set, err := loadForgedIndex(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Only the first valid line and the last (which has "path":"" -> empty key) are parsed
		if !set["/good.jsonl"] {
			t.Error("missing /good.jsonl")
		}
	})
}

func TestBatchForge_appendForgedRecord(t *testing.T) {
	t.Run("creates file and appends", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "sub", "forged.jsonl")

		r := ForgedRecord{Path: "/test.jsonl", ForgedAt: time.Now(), Session: "s1"}
		if err := appendForgedRecord(path, r); err != nil {
			t.Fatalf("first append: %v", err)
		}

		// Append a second record
		r2 := ForgedRecord{Path: "/test2.jsonl", ForgedAt: time.Now(), Session: "s2"}
		if err := appendForgedRecord(path, r2); err != nil {
			t.Fatalf("second append: %v", err)
		}

		// Read back and verify
		set, err := loadForgedIndex(path)
		if err != nil {
			t.Fatalf("loadForgedIndex: %v", err)
		}
		if len(set) != 2 {
			t.Errorf("expected 2 entries, got %d", len(set))
		}
	})
}

func TestBatchForge_findPendingTranscripts(t *testing.T) {
	t.Run("specific dir with valid files", func(t *testing.T) {
		dir := t.TempDir()

		// Create a valid JSONL file
		valid := filepath.Join(dir, "transcript.jsonl")
		if err := os.WriteFile(valid, make([]byte, 200), 0644); err != nil {
			t.Fatal(err)
		}

		// Too small
		small := filepath.Join(dir, "tiny.jsonl")
		if err := os.WriteFile(small, make([]byte, 50), 0644); err != nil {
			t.Fatal(err)
		}

		// Wrong extension
		wrong := filepath.Join(dir, "data.json")
		if err := os.WriteFile(wrong, make([]byte, 200), 0644); err != nil {
			t.Fatal(err)
		}

		// Subagents dir should be skipped
		subagentsDir := filepath.Join(dir, "subagents")
		if err := os.MkdirAll(subagentsDir, 0755); err != nil {
			t.Fatal(err)
		}
		skipped := filepath.Join(subagentsDir, "sub.jsonl")
		if err := os.WriteFile(skipped, make([]byte, 200), 0644); err != nil {
			t.Fatal(err)
		}

		candidates, err := findPendingTranscripts(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(candidates) != 1 {
			t.Fatalf("expected 1 candidate, got %d", len(candidates))
		}
		if candidates[0].path != valid {
			t.Errorf("candidate path = %q, want %q", candidates[0].path, valid)
		}
	})

	t.Run("empty dir returns nil", func(t *testing.T) {
		dir := t.TempDir()
		candidates, err := findPendingTranscripts(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(candidates) != 0 {
			t.Errorf("expected 0 candidates, got %d", len(candidates))
		}
	})
}

func TestBatchForge_resolveSearchDirs(t *testing.T) {
	t.Run("specific dir returns it", func(t *testing.T) {
		dirs, err := resolveSearchDirs("/specific/dir")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(dirs) != 1 || dirs[0] != "/specific/dir" {
			t.Errorf("dirs = %v, want [/specific/dir]", dirs)
		}
	})

	t.Run("empty dir uses home", func(t *testing.T) {
		dirs, err := resolveSearchDirs("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// May return nil if ~/.claude/projects doesn't exist, or a path if it does
		if len(dirs) > 0 {
			if !strings.Contains(dirs[0], ".claude") {
				t.Errorf("dirs = %v, expected .claude path", dirs)
			}
		}
	})
}

// ===========================================================================
// feedback.go helpers
// ===========================================================================

func TestFeedback_resolveReward(t *testing.T) {
	tests := []struct {
		name    string
		helpful bool
		harmful bool
		reward  float64
		alpha   float64
		want    float64
		wantErr string
	}{
		{"helpful shortcut", true, false, -1, 0.1, 1.0, ""},
		{"harmful shortcut", false, true, -1, 0.1, 0.0, ""},
		{"explicit reward", false, false, 0.75, 0.1, 0.75, ""},
		{"both helpful and harmful", true, true, -1, 0.1, 0, "cannot use both"},
		{"no reward specified", false, false, -1, 0.1, 0, "must provide"},
		{"reward over 1.0", false, false, 1.5, 0.1, 0, "must be between"},
		{"alpha zero", false, false, 0.5, 0.0, 0, "alpha must be between"},
		{"alpha negative", false, false, 0.5, -0.1, 0, "alpha must be between"},
		{"alpha over 1", false, false, 0.5, 1.5, 0, "alpha must be between"},
		{"alpha exactly 1", false, false, 0.5, 1.0, 0.5, ""},
		{"reward exactly 0", false, false, 0.0, 0.1, 0.0, ""},
		{"reward exactly 1", false, false, 1.0, 0.1, 1.0, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveReward(tc.helpful, tc.harmful, tc.reward, tc.alpha)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %q, want containing %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("resolveReward() = %f, want %f", got, tc.want)
			}
		})
	}
}

func TestFeedback_classifyFeedbackType(t *testing.T) {
	tests := []struct {
		name    string
		helpful bool
		harmful bool
		want    string
	}{
		{"helpful", true, false, "helpful"},
		{"harmful", false, true, "harmful"},
		{"custom", false, false, "custom"},
		{"both flags (edge case)", true, true, "helpful"}, // helpful checked first
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyFeedbackType(tc.helpful, tc.harmful)
			if got != tc.want {
				t.Errorf("classifyFeedbackType(%v, %v) = %q, want %q",
					tc.helpful, tc.harmful, got, tc.want)
			}
		})
	}
}

func TestFeedback_counterDirectionFromFeedback(t *testing.T) {
	tests := []struct {
		name     string
		reward   float64
		helpful  bool
		harmful  bool
		wantHelp bool
		wantHarm bool
	}{
		{"explicit helpful", 0.5, true, false, true, false},
		{"explicit harmful", 0.5, false, true, false, true},
		{"implied helpful (high reward)", 0.9, false, false, true, false},
		{"implied harmful (low reward)", 0.1, false, false, false, true},
		{"neutral reward", 0.5, false, false, false, false},
		{"boundary helpful", 0.8, false, false, true, false},
		{"boundary harmful", 0.2, false, false, false, true},
		{"just below helpful threshold", 0.79, false, false, false, false},
		{"just above harmful threshold", 0.21, false, false, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			help, harm := counterDirectionFromFeedback(tc.reward, tc.helpful, tc.harmful)
			if help != tc.wantHelp {
				t.Errorf("helpful = %v, want %v", help, tc.wantHelp)
			}
			if harm != tc.wantHarm {
				t.Errorf("harmful = %v, want %v", harm, tc.wantHarm)
			}
		})
	}
}

func TestFeedback_updateFrontMatterFields(t *testing.T) {
	tests := []struct {
		name   string
		lines  []string
		fields map[string]string
		check  func([]string) bool
		desc   string
	}{
		{
			name:  "update existing field",
			lines: []string{"utility: 0.5", "title: test"},
			fields: map[string]string{
				"utility": "0.7500",
			},
			check: func(result []string) bool {
				for _, l := range result {
					if l == "utility: 0.7500" {
						return true
					}
				}
				return false
			},
			desc: "should contain 'utility: 0.7500'",
		},
		{
			name:  "add missing field",
			lines: []string{"title: test"},
			fields: map[string]string{
				"utility": "0.5000",
			},
			check: func(result []string) bool {
				return len(result) == 2 // title + new utility
			},
			desc: "should have 2 lines",
		},
		{
			name:   "empty lines and fields",
			lines:  []string{},
			fields: map[string]string{},
			check: func(result []string) bool {
				return len(result) == 0
			},
			desc: "should be empty",
		},
		{
			name:  "preserves non-matching lines",
			lines: []string{"author: test", "date: today"},
			fields: map[string]string{
				"utility": "0.5",
			},
			check: func(result []string) bool {
				return len(result) == 3 // author, date, utility
			},
			desc: "should have 3 lines",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := updateFrontMatterFields(tc.lines, tc.fields)
			if !tc.check(result) {
				t.Errorf("updateFrontMatterFields: %s; got %v", tc.desc, result)
			}
		})
	}
}

func TestFeedback_incrementRewardCount(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  string
	}{
		{"no reward_count", []string{"title: test"}, "1"},
		{"existing count 0", []string{"reward_count: 0"}, "1"},
		{"existing count 5", []string{"reward_count: 5"}, "6"},
		{"empty lines", []string{}, "1"},
		{"count among others", []string{"utility: 0.5", "reward_count: 3", "confidence: 0.8"}, "4"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := incrementRewardCount(tc.lines)
			if got != tc.want {
				t.Errorf("incrementRewardCount() = %q, want %q", got, tc.want)
			}
		})
	}
}

// Tests for probeWithExtensions, probeDirect, probeGlob, searchDirsForLearning,
// buildAgentsDirs, isInSet have been moved to cli/internal/resolver/resolver_test.go
// as part of the LearningResolver extraction.

func TestFeedback_parseJSONLFirstLine(t *testing.T) {
	t.Run("valid JSONL", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.jsonl")
		if err := os.WriteFile(path, []byte(`{"utility":0.5,"id":"L001"}`+"\nsecond line\n"), 0644); err != nil {
			t.Fatal(err)
		}
		lines, data, err := parseJSONLFirstLine(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(lines) != 3 {
			t.Errorf("lines len = %d, want 3", len(lines))
		}
		if data["id"] != "L001" {
			t.Errorf("data[id] = %v, want L001", data["id"])
		}
	})

	t.Run("empty file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "empty.jsonl")
		if err := os.WriteFile(path, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
		_, _, err := parseJSONLFirstLine(path)
		if err == nil {
			t.Fatal("expected error for empty file")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "bad.jsonl")
		if err := os.WriteFile(path, []byte("not json\n"), 0644); err != nil {
			t.Fatal(err)
		}
		_, _, err := parseJSONLFirstLine(path)
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		_, _, err := parseJSONLFirstLine("/nonexistent/file.jsonl")
		if err == nil {
			t.Fatal("expected error for nonexistent file")
		}
	})
}

func TestFeedback_updateJSONLUtility(t *testing.T) {
	// Save and restore package-level flags
	origHelpful := feedbackHelpful
	origHarmful := feedbackHarmful
	defer func() {
		feedbackHelpful = origHelpful
		feedbackHarmful = origHarmful
	}()
	feedbackHelpful = true
	feedbackHarmful = false

	t.Run("updates utility in JSONL", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "learning.jsonl")
		data := map[string]any{"id": "L001", "utility": 0.5}
		jsonData, _ := json.Marshal(data)
		if err := os.WriteFile(path, append(jsonData, '\n'), 0644); err != nil {
			t.Fatal(err)
		}

		oldU, newU, err := updateJSONLUtility(path, 1.0, 0.1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if oldU != 0.5 {
			t.Errorf("oldUtility = %f, want 0.5", oldU)
		}
		// newU = (1-0.1)*0.5 + 0.1*1.0 = 0.45 + 0.1 = 0.55
		expected := 0.55
		if math.Abs(newU-expected) > 0.001 {
			t.Errorf("newUtility = %f, want ~%f", newU, expected)
		}

		// Verify file was updated
		content, _ := os.ReadFile(path)
		var updated map[string]any
		lines := strings.Split(string(content), "\n")
		if err := json.Unmarshal([]byte(lines[0]), &updated); err != nil {
			t.Fatalf("parse updated: %v", err)
		}
		if updated["utility"].(float64) < 0.54 || updated["utility"].(float64) > 0.56 {
			t.Errorf("written utility = %v, want ~0.55", updated["utility"])
		}
	})

	t.Run("default utility when missing", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "no-utility.jsonl")
		data := map[string]any{"id": "L002"}
		jsonData, _ := json.Marshal(data)
		if err := os.WriteFile(path, append(jsonData, '\n'), 0644); err != nil {
			t.Fatal(err)
		}

		oldU, _, err := updateJSONLUtility(path, 1.0, 0.1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if oldU != types.InitialUtility {
			t.Errorf("oldUtility = %f, want InitialUtility %f", oldU, types.InitialUtility)
		}
	})
}

func TestFeedback_updateMarkdownUtility(t *testing.T) {
	t.Run("with existing front matter", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "learning.md")
		content := "---\ntitle: Test\nutility: 0.5\nreward_count: 2\n---\n# Content\n\nBody text.\n"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		oldU, newU, err := updateMarkdownUtility(path, 1.0, 0.1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if oldU != 0.5 {
			t.Errorf("oldUtility = %f, want 0.5", oldU)
		}
		expected := 0.55
		if math.Abs(newU-expected) > 0.001 {
			t.Errorf("newUtility = %f, want ~%f", newU, expected)
		}

		// Verify file was updated
		updated, _ := os.ReadFile(path)
		if !strings.Contains(string(updated), "utility: 0.5500") {
			t.Errorf("file should contain updated utility, got: %s", string(updated))
		}
		if !strings.Contains(string(updated), "reward_count: 3") {
			t.Errorf("file should contain incremented reward_count, got: %s", string(updated))
		}
	})

	t.Run("without front matter adds it", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "plain.md")
		content := "# Just Content\n\nNo front matter here.\n"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		oldU, newU, err := updateMarkdownUtility(path, 0.8, 0.1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if oldU != types.InitialUtility {
			t.Errorf("oldUtility = %f, want InitialUtility", oldU)
		}
		expected := (1-0.1)*types.InitialUtility + 0.1*0.8
		if math.Abs(newU-expected) > 0.001 {
			t.Errorf("newUtility = %f, want ~%f", newU, expected)
		}

		// Verify front matter was added
		updated, _ := os.ReadFile(path)
		s := string(updated)
		if !strings.HasPrefix(s, "---\n") {
			t.Error("file should start with front matter delimiter")
		}
		if !strings.Contains(s, "reward_count: 1") {
			t.Error("file should contain reward_count: 1")
		}
		if !strings.Contains(s, "# Just Content") {
			t.Error("file should preserve original content")
		}
	})
}

func TestFeedback_updateLearningUtility_dispatch(t *testing.T) {
	// Save and restore package-level flags
	origHelpful := feedbackHelpful
	origHarmful := feedbackHarmful
	defer func() {
		feedbackHelpful = origHelpful
		feedbackHarmful = origHarmful
	}()
	feedbackHelpful = false
	feedbackHarmful = false

	t.Run("dispatches to JSONL", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.jsonl")
		data := map[string]any{"id": "L001", "utility": 0.5}
		jsonData, _ := json.Marshal(data)
		if err := os.WriteFile(path, append(jsonData, '\n'), 0644); err != nil {
			t.Fatal(err)
		}

		_, _, err := updateLearningUtility(path, 0.8, 0.1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("dispatches to markdown", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.md")
		if err := os.WriteFile(path, []byte("# Test\nContent"), 0644); err != nil {
			t.Fatal(err)
		}

		_, _, err := updateLearningUtility(path, 0.8, 0.1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestFeedback_needsUtilityMigration(t *testing.T) {
	t.Run("needs migration - no utility", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.jsonl")
		if err := os.WriteFile(path, []byte(`{"id":"L001"}`+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		needs, err := needsUtilityMigration(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !needs {
			t.Error("expected to need migration")
		}
	})

	t.Run("no migration needed - has utility", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.jsonl")
		if err := os.WriteFile(path, []byte(`{"id":"L001","utility":0.5}`+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		needs, err := needsUtilityMigration(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if needs {
			t.Error("expected to NOT need migration")
		}
	})

	t.Run("empty file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "empty.jsonl")
		if err := os.WriteFile(path, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
		needs, err := needsUtilityMigration(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if needs {
			t.Error("expected empty file to not need migration")
		}
	})
}

func TestFeedback_addUtilityField(t *testing.T) {
	t.Run("adds utility to existing JSONL", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.jsonl")
		if err := os.WriteFile(path, []byte(`{"id":"L001"}`+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := addUtilityField(path); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		content, _ := os.ReadFile(path)
		var data map[string]any
		lines := strings.Split(string(content), "\n")
		if err := json.Unmarshal([]byte(lines[0]), &data); err != nil {
			t.Fatalf("parse: %v", err)
		}
		if data["utility"] != types.InitialUtility {
			t.Errorf("utility = %v, want %f", data["utility"], types.InitialUtility)
		}
	})

	t.Run("empty file is no-op", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "empty.jsonl")
		if err := os.WriteFile(path, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
		if err := addUtilityField(path); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestFeedback_migrateJSONLFiles(t *testing.T) {
	dir := t.TempDir()

	// File needing migration
	f1 := filepath.Join(dir, "needs.jsonl")
	if err := os.WriteFile(f1, []byte(`{"id":"L001"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// File not needing migration
	f2 := filepath.Join(dir, "ok.jsonl")
	if err := os.WriteFile(f2, []byte(`{"id":"L002","utility":0.5}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("migrates and skips correctly", func(t *testing.T) {
		migrated, skipped := migrateJSONLFiles([]string{f1, f2}, false)
		if migrated != 1 {
			t.Errorf("migrated = %d, want 1", migrated)
		}
		if skipped != 1 {
			t.Errorf("skipped = %d, want 1", skipped)
		}
	})

	t.Run("dry-run does not write", func(t *testing.T) {
		dir2 := t.TempDir()
		f3 := filepath.Join(dir2, "dry.jsonl")
		if err := os.WriteFile(f3, []byte(`{"id":"L003"}`+"\n"), 0644); err != nil {
			t.Fatal(err)
		}

		migrated, _ := migrateJSONLFiles([]string{f3}, true)
		if migrated != 1 {
			t.Errorf("migrated = %d, want 1 (dry-run count)", migrated)
		}

		// Verify file was NOT modified
		content, _ := os.ReadFile(f3)
		if strings.Contains(string(content), "utility") {
			t.Error("dry-run should not modify file")
		}
	})
}

func TestFeedback_findLearningFile(t *testing.T) {
	t.Run("finds in .agents/learnings", func(t *testing.T) {
		dir := t.TempDir()
		learningsDir := filepath.Join(dir, ".agents", "learnings")
		if err := os.MkdirAll(learningsDir, 0755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(learningsDir, "L001.md")
		if err := os.WriteFile(path, []byte("# L001"), 0644); err != nil {
			t.Fatal(err)
		}

		found, err := findLearningFile(dir, "L001")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found != path {
			t.Errorf("found = %q, want %q", found, path)
		}
	})

	t.Run("not found returns error", func(t *testing.T) {
		dir := t.TempDir()
		_, err := findLearningFile(dir, "NONEXISTENT")
		if err == nil {
			t.Fatal("expected error for nonexistent learning")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("error = %q, want containing 'not found'", err.Error())
		}
	})
}

// ===========================================================================
// feedback_loop.go helpers
// ===========================================================================

func TestFeedbackLoop_deduplicateCitations(t *testing.T) {
	baseDir := t.TempDir()
	citations := []types.CitationEvent{
		{ArtifactPath: "learnings/L001.md"},
		{ArtifactPath: "learnings/L001.md"}, // duplicate
		{ArtifactPath: "learnings/L002.md"},
	}

	unique := deduplicateCitations(baseDir, citations)
	if len(unique) != 2 {
		t.Errorf("deduplicateCitations() len = %d, want 2", len(unique))
	}
}

func TestFeedbackLoop_resolveFeedbackLoopSessionID(t *testing.T) {
	tests := []struct {
		name      string
		flag      string
		envVar    string
		wantErr   bool
		wantEmpty bool
	}{
		{"from flag", "session-20260125-120000", "", false, false},
		{"empty flag and env", "", "", true, false},
		{"whitespace flag", "   ", "", true, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envVar != "" {
				t.Setenv("CLAUDE_SESSION_ID", tc.envVar)
			} else {
				t.Setenv("CLAUDE_SESSION_ID", "")
			}
			got, err := resolveFeedbackLoopSessionID(tc.flag)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == "" {
				t.Error("expected non-empty session ID")
			}
		})
	}

	t.Run("from env var", func(t *testing.T) {
		t.Setenv("CLAUDE_SESSION_ID", "session-from-env")
		got, err := resolveFeedbackLoopSessionID("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "session-from-env" {
			t.Errorf("got = %q, want 'session-from-env'", got)
		}
	})
}

func TestFeedbackLoop_resolveFeedbackReward(t *testing.T) {
	tests := []struct {
		name       string
		flagReward float64
		wantReward float64
		wantErr    bool
	}{
		{"valid override", 0.8, 0.8, false},
		{"zero override", 0.0, 0.0, false},
		{"one override", 1.0, 1.0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveFeedbackReward(tc.flagReward, "", "")
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantReward {
				t.Errorf("reward = %f, want %f", got, tc.wantReward)
			}
		})
	}
}

func TestFeedbackLoop_validateBatchFeedbackFlags(t *testing.T) {
	// Save and restore package-level vars
	origMax := batchFeedbackMaxSessions
	origReward := batchFeedbackReward
	origRuntime := batchFeedbackMaxRuntime
	defer func() {
		batchFeedbackMaxSessions = origMax
		batchFeedbackReward = origReward
		batchFeedbackMaxRuntime = origRuntime
	}()

	tests := []struct {
		name        string
		maxSessions int
		reward      float64
		maxRuntime  time.Duration
		wantErr     string
	}{
		{"all valid", 5, -1, time.Minute, ""},
		{"all defaults", 0, -1, 0, ""},
		{"negative max-sessions", -1, -1, 0, "--max-sessions"},
		{"reward too low", 0, -0.5, 0, "--reward"},
		{"reward too high", 0, 1.5, 0, "--reward"},
		{"valid reward zero", 0, 0.0, 0, ""},
		{"valid reward one", 0, 1.0, 0, ""},
		{"negative max-runtime", 0, -1, -time.Second, "--max-runtime"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			batchFeedbackMaxSessions = tc.maxSessions
			batchFeedbackReward = tc.reward
			batchFeedbackMaxRuntime = tc.maxRuntime

			err := validateBatchFeedbackFlags()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error containing %q", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %q, want containing %q", err.Error(), tc.wantErr)
				}
			}
		})
	}
}

func TestFeedbackLoop_sortAndCapSessions(t *testing.T) {
	// Save and restore package-level var
	origMax := batchFeedbackMaxSessions
	defer func() { batchFeedbackMaxSessions = origMax }()

	now := time.Now()
	sessionCitations := map[string][]types.CitationEvent{
		"s1": {{ArtifactPath: "a.md"}},
		"s2": {{ArtifactPath: "b.md"}},
		"s3": {{ArtifactPath: "c.md"}},
	}
	sessionLatest := map[string]time.Time{
		"s1": now.Add(-3 * time.Hour),
		"s2": now.Add(-1 * time.Hour),
		"s3": now.Add(-2 * time.Hour),
	}

	t.Run("sorted by latest citation descending", func(t *testing.T) {
		batchFeedbackMaxSessions = 0 // no limit
		result := sortAndCapSessions(sessionCitations, sessionLatest)
		if len(result) != 3 {
			t.Fatalf("expected 3 sessions, got %d", len(result))
		}
		// s2 is newest, then s3, then s1
		if result[0] != "s2" {
			t.Errorf("first = %q, want s2 (newest)", result[0])
		}
		if result[1] != "s3" {
			t.Errorf("second = %q, want s3", result[1])
		}
		if result[2] != "s1" {
			t.Errorf("third = %q, want s1 (oldest)", result[2])
		}
	})

	t.Run("capped by maxSessions", func(t *testing.T) {
		batchFeedbackMaxSessions = 2
		result := sortAndCapSessions(sessionCitations, sessionLatest)
		if len(result) != 2 {
			t.Fatalf("expected 2 sessions (capped), got %d", len(result))
		}
	})
}

func TestFeedbackLoop_writeFeedbackEvents(t *testing.T) {
	t.Run("empty events is no-op", func(t *testing.T) {
		dir := t.TempDir()
		err := writeFeedbackEvents(dir, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// File should not be created
		path := filepath.Join(dir, FeedbackFilePath)
		if _, err := os.Stat(path); err == nil {
			t.Error("expected no file for empty events")
		}
	})

	t.Run("writes events", func(t *testing.T) {
		dir := t.TempDir()
		events := []FeedbackEvent{
			{SessionID: "s1", Reward: 0.8, ArtifactPath: "/a.md"},
			{SessionID: "s1", Reward: 0.8, ArtifactPath: "/b.md"},
		}
		err := writeFeedbackEvents(dir, events)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Read back
		loaded, err := loadFeedbackEvents(dir)
		if err != nil {
			t.Fatalf("loadFeedbackEvents: %v", err)
		}
		if len(loaded) != 2 {
			t.Errorf("loaded %d events, want 2", len(loaded))
		}
	})
}

func TestFeedbackLoop_loadFeedbackEvents(t *testing.T) {
	t.Run("nonexistent file returns error", func(t *testing.T) {
		dir := t.TempDir()
		_, err := loadFeedbackEvents(dir)
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("valid events", func(t *testing.T) {
		dir := t.TempDir()
		feedbackDir := filepath.Join(dir, ".agents", "ao")
		if err := os.MkdirAll(feedbackDir, 0755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(feedbackDir, "feedback.jsonl")

		e1 := FeedbackEvent{SessionID: "s1", Reward: 0.8, ArtifactPath: "/a.md"}
		e2 := FeedbackEvent{SessionID: "s2", Reward: 0.5, ArtifactPath: "/b.md"}
		d1, _ := json.Marshal(e1)
		d2, _ := json.Marshal(e2)
		if err := os.WriteFile(path, append(append(d1, '\n'), append(d2, '\n')...), 0644); err != nil {
			t.Fatal(err)
		}

		events, err := loadFeedbackEvents(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(events) != 2 {
			t.Errorf("loaded %d events, want 2", len(events))
		}
		if events[0].SessionID != "s1" {
			t.Errorf("events[0].SessionID = %q, want s1", events[0].SessionID)
		}
	})
}

func TestFeedbackLoop_buildProcessedSessionSet(t *testing.T) {
	t.Run("no feedback file returns empty set", func(t *testing.T) {
		dir := t.TempDir()
		set := buildProcessedSessionSet(dir)
		if len(set) != 0 {
			t.Errorf("expected empty set, got %d entries", len(set))
		}
	})

	t.Run("returns set of canonical session IDs", func(t *testing.T) {
		dir := t.TempDir()
		feedbackDir := filepath.Join(dir, ".agents", "ao")
		if err := os.MkdirAll(feedbackDir, 0755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(feedbackDir, "feedback.jsonl")

		e1 := FeedbackEvent{SessionID: "session-20260125-120000", Reward: 0.8}
		e2 := FeedbackEvent{SessionID: "session-20260126-130000", Reward: 0.5}
		d1, _ := json.Marshal(e1)
		d2, _ := json.Marshal(e2)
		if err := os.WriteFile(path, append(append(d1, '\n'), append(d2, '\n')...), 0644); err != nil {
			t.Fatal(err)
		}

		set := buildProcessedSessionSet(dir)
		if len(set) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(set))
		}
		if !set["session-20260125-120000"] {
			t.Error("missing session-20260125-120000")
		}
	})
}

func TestFeedbackLoop_FeedbackEventSerialization(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	event := FeedbackEvent{
		SessionID:      "session-20260125-120000",
		ArtifactPath:   "/path/to/learning.md",
		Reward:         0.85,
		UtilityBefore:  0.5,
		UtilityAfter:   0.535,
		Alpha:          0.1,
		RecordedAt:     now,
		TranscriptPath: "/path/to/transcript.jsonl",
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded FeedbackEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.SessionID != event.SessionID {
		t.Errorf("SessionID = %q, want %q", decoded.SessionID, event.SessionID)
	}
	if decoded.Reward != event.Reward {
		t.Errorf("Reward = %f, want %f", decoded.Reward, event.Reward)
	}
	if decoded.TranscriptPath != event.TranscriptPath {
		t.Errorf("TranscriptPath = %q, want %q", decoded.TranscriptPath, event.TranscriptPath)
	}
}

func TestBatchForge_BatchForgeResultSerialization(t *testing.T) {
	result := BatchForgeResult{
		Forged:    5,
		Skipped:   3,
		Failed:    1,
		Extracted: 2,
		Paths:     []string{"/a", "/b", "/c", "/d", "/e"},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded BatchForgeResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Forged != 5 {
		t.Errorf("Forged = %d, want 5", decoded.Forged)
	}
	if decoded.Skipped != 3 {
		t.Errorf("Skipped = %d, want 3", decoded.Skipped)
	}
	if decoded.Failed != 1 {
		t.Errorf("Failed = %d, want 1", decoded.Failed)
	}
	if decoded.Extracted != 2 {
		t.Errorf("Extracted = %d, want 2", decoded.Extracted)
	}
	if len(decoded.Paths) != 5 {
		t.Errorf("Paths len = %d, want 5", len(decoded.Paths))
	}
}

func TestBatchForge_ForgedRecordSerialization(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	record := ForgedRecord{
		Path:     "/transcript/test.jsonl",
		ForgedAt: now,
		Session:  "session-123",
	}

	data, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ForgedRecord
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Path != record.Path {
		t.Errorf("Path = %q, want %q", decoded.Path, record.Path)
	}
	if decoded.Session != record.Session {
		t.Errorf("Session = %q, want %q", decoded.Session, record.Session)
	}
}
