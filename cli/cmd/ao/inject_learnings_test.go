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

// ---------------------------------------------------------------------------
// sanitizeSourcePhase
// ---------------------------------------------------------------------------

func TestInjectLearnings_sanitizeSourcePhase(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"research", "research", "research"},
		{"plan", "plan", "plan"},
		{"implement", "implement", "implement"},
		{"validate", "validate", "validate"},
		{"uppercase", "RESEARCH", "research"},
		{"mixed case", "Plan", "plan"},
		{"with spaces", "  validate  ", "validate"},
		{"invalid", "build", ""},
		{"empty", "", ""},
		{"random", "foobar", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeSourcePhase(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeSourcePhase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseFrontMatter
// ---------------------------------------------------------------------------

func TestInjectLearnings_parseFrontMatter(t *testing.T) {
	t.Run("basic frontmatter", func(t *testing.T) {
		lines := []string{
			"---",
			"superseded_by: L99",
			"utility: 0.7",
			"source_bead: ag-abc",
			"source_phase: plan",
			"---",
			"# Content",
		}
		fm, endIdx := parseFrontMatter(lines)
		if fm.SupersededBy != "L99" {
			t.Errorf("SupersededBy = %q, want %q", fm.SupersededBy, "L99")
		}
		if fm.Utility != 0.7 {
			t.Errorf("Utility = %f, want 0.7", fm.Utility)
		}
		if !fm.HasUtility {
			t.Error("expected HasUtility = true")
		}
		if fm.SourceBead != "ag-abc" {
			t.Errorf("SourceBead = %q, want %q", fm.SourceBead, "ag-abc")
		}
		if fm.SourcePhase != "plan" {
			t.Errorf("SourcePhase = %q, want %q", fm.SourcePhase, "plan")
		}
		if endIdx != 6 {
			t.Errorf("endIdx = %d, want 6", endIdx)
		}
	})

	t.Run("no frontmatter", func(t *testing.T) {
		lines := []string{"# Just content", "Some text"}
		fm, endIdx := parseFrontMatter(lines)
		if endIdx != 0 {
			t.Errorf("endIdx = %d, want 0", endIdx)
		}
		if fm.HasUtility {
			t.Error("expected HasUtility = false")
		}
	})

	t.Run("empty lines", func(t *testing.T) {
		fm, endIdx := parseFrontMatter(nil)
		if endIdx != 0 {
			t.Errorf("endIdx = %d, want 0", endIdx)
		}
		if fm.HasUtility {
			t.Error("expected HasUtility = false")
		}
	})

	t.Run("unclosed frontmatter returns 0", func(t *testing.T) {
		lines := []string{"---", "title: Test", "no closing"}
		_, endIdx := parseFrontMatter(lines)
		if endIdx != 0 {
			t.Errorf("endIdx = %d, want 0 for unclosed frontmatter", endIdx)
		}
	})

	t.Run("stability field", func(t *testing.T) {
		lines := []string{"---", "stability: experimental", "---"}
		fm, _ := parseFrontMatter(lines)
		if fm.Stability != "experimental" {
			t.Errorf("Stability = %q, want %q", fm.Stability, "experimental")
		}
	})

	t.Run("stability stable", func(t *testing.T) {
		lines := []string{"---", "stability: stable", "---"}
		fm, _ := parseFrontMatter(lines)
		if fm.Stability != "stable" {
			t.Errorf("Stability = %q, want %q", fm.Stability, "stable")
		}
	})

	t.Run("stability absent defaults empty", func(t *testing.T) {
		lines := []string{"---", "utility: 0.5", "---"}
		fm, _ := parseFrontMatter(lines)
		if fm.Stability != "" {
			t.Errorf("Stability = %q, want empty string for absent field", fm.Stability)
		}
	})

	t.Run("promoted_to field", func(t *testing.T) {
		lines := []string{"---", "promoted_to: ~/.agents/learnings/global.md", "---"}
		fm, _ := parseFrontMatter(lines)
		if fm.PromotedTo != "~/.agents/learnings/global.md" {
			t.Errorf("PromotedTo = %q, want path", fm.PromotedTo)
		}
	})

	t.Run("hyphenated field names", func(t *testing.T) {
		lines := []string{"---", "superseded-by: L88", "promoted-to: global", "source-bead: xx-123", "source-phase: research", "---"}
		fm, _ := parseFrontMatter(lines)
		if fm.SupersededBy != "L88" {
			t.Errorf("SupersededBy = %q, want L88", fm.SupersededBy)
		}
		if fm.PromotedTo != "global" {
			t.Errorf("PromotedTo = %q, want global", fm.PromotedTo)
		}
		if fm.SourceBead != "xx-123" {
			t.Errorf("SourceBead = %q, want xx-123", fm.SourceBead)
		}
		if fm.SourcePhase != "research" {
			t.Errorf("SourcePhase = %q, want research", fm.SourcePhase)
		}
	})

	t.Run("invalid utility is not set", func(t *testing.T) {
		lines := []string{"---", "utility: not-a-number", "---"}
		fm, _ := parseFrontMatter(lines)
		if fm.HasUtility {
			t.Error("expected HasUtility = false for non-numeric utility")
		}
	})

	t.Run("zero utility is not set", func(t *testing.T) {
		lines := []string{"---", "utility: 0", "---"}
		fm, _ := parseFrontMatter(lines)
		if fm.HasUtility {
			t.Error("expected HasUtility = false for zero utility")
		}
	})
}

// ---------------------------------------------------------------------------
// isSuperseded / isPromoted
// ---------------------------------------------------------------------------

func TestInjectLearnings_isSuperseded(t *testing.T) {
	tests := []struct {
		name string
		fm   frontMatter
		want bool
	}{
		{"empty", frontMatter{}, false},
		{"set", frontMatter{SupersededBy: "L99"}, true},
		{"null string", frontMatter{SupersededBy: "null"}, false},
		{"tilde", frontMatter{SupersededBy: "~"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSuperseded(tt.fm)
			if got != tt.want {
				t.Errorf("isSuperseded() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInjectLearnings_isPromoted(t *testing.T) {
	tests := []struct {
		name string
		fm   frontMatter
		want bool
	}{
		{"empty", frontMatter{}, false},
		{"set", frontMatter{PromotedTo: "/global/learn.md"}, true},
		{"null string", frontMatter{PromotedTo: "null"}, false},
		{"tilde", frontMatter{PromotedTo: "~"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPromoted(tt.fm)
			if got != tt.want {
				t.Errorf("isPromoted() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseLearningBody
// ---------------------------------------------------------------------------

func TestInjectLearnings_parseLearningBody(t *testing.T) {
	t.Run("extracts title and id", func(t *testing.T) {
		l := learning{ID: "filename.md", Source: "filename.md"}
		lines := []string{
			"# Authentication Patterns",
			"",
			"ID: L42",
			"",
			"Some content",
		}
		parseLearningBody(lines, 0, &l)
		if l.Title != "Authentication Patterns" {
			t.Errorf("Title = %q, want %q", l.Title, "Authentication Patterns")
		}
		if l.ID != "L42" {
			t.Errorf("ID = %q, want %q", l.ID, "L42")
		}
	})

	t.Run("does not overwrite title", func(t *testing.T) {
		l := learning{ID: "filename.md", Source: "filename.md", Title: "Existing"}
		lines := []string{"# New Title", "id: L10"}
		parseLearningBody(lines, 0, &l)
		if l.Title != "Existing" {
			t.Errorf("Title = %q, should not overwrite %q", l.Title, "Existing")
		}
	})

	t.Run("does not overwrite explicit ID", func(t *testing.T) {
		l := learning{ID: "explicit-id", Source: "other.md"}
		lines := []string{"ID: should-not-replace"}
		parseLearningBody(lines, 0, &l)
		if l.ID != "explicit-id" {
			t.Errorf("ID = %q, should not overwrite", l.ID)
		}
	})

	t.Run("lowercase id prefix", func(t *testing.T) {
		l := learning{ID: "test.md", Source: "test.md"}
		lines := []string{"id: L99"}
		parseLearningBody(lines, 0, &l)
		if l.ID != "L99" {
			t.Errorf("ID = %q, want L99", l.ID)
		}
	})
}

// ---------------------------------------------------------------------------
// parseLearningFile
// ---------------------------------------------------------------------------

func TestInjectLearnings_parseLearningFile_Markdown(t *testing.T) {
	tmp := t.TempDir()

	t.Run("basic markdown learning", func(t *testing.T) {
		content := "---\nutility: 0.6\nsource_bead: ag-001\nsource_phase: implement\n---\n# My Learning\n\nID: L123\n\nActual content here.\n"
		path := filepath.Join(tmp, "learn.md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		l, err := parseLearningFile(path)
		if err != nil {
			t.Fatalf("parseLearningFile: %v", err)
		}
		if l.ID != "L123" {
			t.Errorf("ID = %q, want L123", l.ID)
		}
		if l.Title != "My Learning" {
			t.Errorf("Title = %q, want 'My Learning'", l.Title)
		}
		if l.Utility != 0.6 {
			t.Errorf("Utility = %f, want 0.6", l.Utility)
		}
		if l.SourceBead != "ag-001" {
			t.Errorf("SourceBead = %q, want ag-001", l.SourceBead)
		}
		if l.SourcePhase != "implement" {
			t.Errorf("SourcePhase = %q, want implement", l.SourcePhase)
		}
		if l.Summary == "" {
			t.Error("expected non-empty summary")
		}
		if l.Superseded {
			t.Error("should not be superseded")
		}
	})

	t.Run("superseded learning", func(t *testing.T) {
		content := "---\nsuperseded_by: L99\n---\n# Old\n"
		path := filepath.Join(tmp, "old.md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		l, err := parseLearningFile(path)
		if err != nil {
			t.Fatalf("parseLearningFile: %v", err)
		}
		if !l.Superseded {
			t.Error("expected Superseded = true")
		}
	})

	t.Run("promoted learning", func(t *testing.T) {
		content := "---\npromoted_to: /global/learn.md\n---\n# Promoted\n"
		path := filepath.Join(tmp, "promoted.md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		l, err := parseLearningFile(path)
		if err != nil {
			t.Fatalf("parseLearningFile: %v", err)
		}
		if !l.Superseded {
			t.Error("expected Superseded = true for promoted learning")
		}
	})

	t.Run("title from filename when no heading", func(t *testing.T) {
		content := "Just some text without a heading.\n"
		path := filepath.Join(tmp, "no-heading.md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		l, err := parseLearningFile(path)
		if err != nil {
			t.Fatalf("parseLearningFile: %v", err)
		}
		if l.Title != "no-heading" {
			t.Errorf("Title = %q, want 'no-heading' (from filename)", l.Title)
		}
	})
}

// ---------------------------------------------------------------------------
// populateLearningFromJSON
// ---------------------------------------------------------------------------

func TestInjectLearnings_populateLearningFromJSON(t *testing.T) {
	data := map[string]any{
		"id":           "L42",
		"title":        "Test Title",
		"summary":      "Test summary content",
		"utility":      0.85,
		"source_bead":  "ag-xyz",
		"source_phase": "validate",
		"maturity":     "established",
	}

	l := &learning{ID: "default"}
	populateLearningFromJSON(data, l)

	if l.ID != "L42" {
		t.Errorf("ID = %q, want L42", l.ID)
	}
	if l.Title != "Test Title" {
		t.Errorf("Title = %q, want 'Test Title'", l.Title)
	}
	if l.Summary != "Test summary content" {
		t.Errorf("Summary = %q, want 'Test summary content'", l.Summary)
	}
	if l.Utility != 0.85 {
		t.Errorf("Utility = %f, want 0.85", l.Utility)
	}
	if l.SourceBead != "ag-xyz" {
		t.Errorf("SourceBead = %q, want ag-xyz", l.SourceBead)
	}
	if l.SourcePhase != "validate" {
		t.Errorf("SourcePhase = %q, want validate", l.SourcePhase)
	}
	if l.Maturity != "established" {
		t.Errorf("Maturity = %q, want established", l.Maturity)
	}
}

func TestInjectLearnings_populateLearningFromJSON_ContentFallback(t *testing.T) {
	data := map[string]any{
		"id":      "L1",
		"content": "This is the content used as summary",
	}

	l := &learning{}
	populateLearningFromJSON(data, l)

	if l.Summary == "" {
		t.Error("expected summary from content field fallback")
	}
}

func TestInjectLearnings_populateLearningFromJSON_InvalidPhase(t *testing.T) {
	data := map[string]any{
		"id":           "L1",
		"source_phase": "invalid-phase",
	}

	l := &learning{}
	populateLearningFromJSON(data, l)

	if l.SourcePhase != "" {
		t.Errorf("SourcePhase = %q, want empty for invalid phase", l.SourcePhase)
	}
}

// ---------------------------------------------------------------------------
// jsonFloat
// ---------------------------------------------------------------------------

func TestInjectLearnings_jsonFloat(t *testing.T) {
	tests := []struct {
		name       string
		data       map[string]any
		key        string
		defaultVal float64
		want       float64
	}{
		{"present positive", map[string]any{"conf": 0.8}, "conf", 0.5, 0.8},
		{"missing key", map[string]any{}, "conf", 0.5, 0.5},
		{"zero value", map[string]any{"conf": 0.0}, "conf", 0.5, 0.5}, // zero is not > 0
		{"negative value", map[string]any{"conf": -0.5}, "conf", 0.5, 0.5},
		{"wrong type", map[string]any{"conf": "text"}, "conf", 0.5, 0.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jsonFloat(tt.data, tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("jsonFloat() = %f, want %f", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// jsonTimeField
// ---------------------------------------------------------------------------

func TestInjectLearnings_jsonTimeField(t *testing.T) {
	t.Run("finds first valid key", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Second)
		data := map[string]any{
			"last_decay_at":  now.Format(time.RFC3339),
			"last_reward_at": now.Add(-time.Hour).Format(time.RFC3339),
		}
		got := jsonTimeField(data, "last_decay_at", "last_reward_at")
		if got.IsZero() {
			t.Error("expected non-zero time")
		}
		if !got.Equal(now) {
			t.Errorf("got %v, want %v", got, now)
		}
	})

	t.Run("falls through to second key", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Second)
		data := map[string]any{
			"last_reward_at": now.Format(time.RFC3339),
		}
		got := jsonTimeField(data, "last_decay_at", "last_reward_at")
		if got.IsZero() {
			t.Error("expected non-zero time from second key")
		}
	})

	t.Run("returns zero when no keys match", func(t *testing.T) {
		data := map[string]any{"other": "value"}
		got := jsonTimeField(data, "missing1", "missing2")
		if !got.IsZero() {
			t.Errorf("expected zero time, got %v", got)
		}
	})

	t.Run("returns zero for unparseable time", func(t *testing.T) {
		data := map[string]any{"last_decay_at": "not-a-time"}
		got := jsonTimeField(data, "last_decay_at")
		if !got.IsZero() {
			t.Errorf("expected zero time for unparseable, got %v", got)
		}
	})
}

// ---------------------------------------------------------------------------
// computeDecayedConfidence
// ---------------------------------------------------------------------------

func TestInjectLearnings_computeDecayedConfidence(t *testing.T) {
	tests := []struct {
		name       string
		confidence float64
		weeks      float64
		wantMin    float64
		wantMax    float64
	}{
		{
			name:       "zero weeks no decay",
			confidence: 0.8,
			weeks:      0,
			wantMin:    0.8,
			wantMax:    0.8,
		},
		{
			name:       "two weeks mild decay",
			confidence: 0.8,
			weeks:      2,
			wantMin:    0.6,
			wantMax:    0.7,
		},
		{
			name:       "very old clamps to 0.1",
			confidence: 0.5,
			weeks:      100,
			wantMin:    0.1,
			wantMax:    0.1,
		},
		{
			name:       "low confidence still clamps",
			confidence: 0.15,
			weeks:      50,
			wantMin:    0.1,
			wantMax:    0.1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeDecayedConfidence(tt.confidence, tt.weeks)
			if got < tt.wantMin-0.01 || got > tt.wantMax+0.01 {
				t.Errorf("computeDecayedConfidence(%f, %f) = %f, want [%f, %f]",
					tt.confidence, tt.weeks, got, tt.wantMin, tt.wantMax)
			}
		})
	}

	t.Run("mathematical correctness", func(t *testing.T) {
		// 0.8 * exp(-2 * 0.1) = 0.8 * exp(-0.2)
		got := computeDecayedConfidence(0.8, 2)
		expected := 0.8 * math.Exp(-2.0*types.ConfidenceDecayRate)
		if math.Abs(got-expected) > 0.001 {
			t.Errorf("got %f, want %f", got, expected)
		}
	})
}

// ---------------------------------------------------------------------------
// writeDecayFields
// ---------------------------------------------------------------------------

func TestInjectLearnings_writeDecayFields(t *testing.T) {
	data := map[string]any{
		"confidence": 0.8,
	}
	now := time.Now()

	writeDecayFields(data, 0.65, now)

	if data["confidence"] != 0.65 {
		t.Errorf("confidence = %v, want 0.65", data["confidence"])
	}
	if data["decay_count"] != 1.0 {
		t.Errorf("decay_count = %v, want 1.0", data["decay_count"])
	}
	if _, ok := data["last_decay_at"].(string); !ok {
		t.Error("expected last_decay_at string")
	}

	// Second call increments decay count
	writeDecayFields(data, 0.5, now)
	if data["decay_count"] != 2.0 {
		t.Errorf("decay_count = %v, want 2.0 after second call", data["decay_count"])
	}
}

// ---------------------------------------------------------------------------
// globLearningFiles
// ---------------------------------------------------------------------------

func TestInjectLearnings_globLearningFiles(t *testing.T) {
	tmp := t.TempDir()
	nested := filepath.Join(tmp, "jren-platform")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test files
	for _, name := range []string{"a.md", "b.md", "c.jsonl", "d.txt"} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(nested, "nested.md"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	files := globLearningFiles(tmp)
	if len(files) != 4 { // a.md, b.md, c.jsonl, nested.md — NOT d.txt
		t.Errorf("expected 4 learning files, got %d: %v", len(files), files)
	}

	for _, f := range files {
		ext := filepath.Ext(f)
		if ext != ".md" && ext != ".jsonl" {
			t.Errorf("unexpected file extension: %s", f)
		}
	}
}

func TestInjectLearnings_globLearningFiles_EmptyDir(t *testing.T) {
	tmp := t.TempDir()
	files := globLearningFiles(tmp)
	if len(files) != 0 {
		t.Errorf("expected 0 files in empty dir, got %d", len(files))
	}
}

// ---------------------------------------------------------------------------
// processLearningFile
// ---------------------------------------------------------------------------

func TestInjectLearnings_processLearningFile_SkipsSuperseded(t *testing.T) {
	tmp := t.TempDir()
	content := "---\nsuperseded_by: L99\n---\n# Old\n"
	path := filepath.Join(tmp, "old.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, ok := processLearningFile(path, nil, time.Now())
	if ok {
		t.Error("expected ok=false for superseded learning")
	}
}

func TestInjectLearnings_processLearningFile_FiltersByQuery(t *testing.T) {
	tmp := t.TempDir()
	content := "---\nmaturity: provisional\n---\n# Authentication Patterns\n\nUse JWT for auth tokens in stateless APIs. Always validate the signature server-side and check expiry claims.\n"
	path := filepath.Join(tmp, "auth.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Matching query
	_, ok := processLearningFile(path, []string{"authentication"}, time.Now())
	if !ok {
		t.Error("expected ok=true for matching query")
	}

	// Non-matching query (not in title, summary, or body)
	_, ok = processLearningFile(path, []string{"kubernetes"}, time.Now())
	if ok {
		t.Error("expected ok=false for non-matching query")
	}
}

func TestInjectLearnings_processLearningFile_BodyTextSearch(t *testing.T) {
	tmp := t.TempDir()
	// Title says "Auth Patterns" but body mentions "flywheel" — query should match body
	content := "---\nmaturity: provisional\n---\n# Auth Patterns\n\nUse flywheel compounding for knowledge retention.\n"
	path := filepath.Join(tmp, "auth.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Query matches body but not title/summary
	l, ok := processLearningFile(path, []string{"flywheel"}, time.Now())
	if !ok {
		t.Error("expected ok=true when query matches body text")
	}
	if l.BodyText == "" {
		t.Error("expected BodyText to be populated")
	}
}

func TestInjectLearnings_processLearningFile_TokenOverlapQuery(t *testing.T) {
	tmp := t.TempDir()
	content := "---\nmaturity: provisional\nsource_bead: test\nutility: 0.8\n---\n# Git Hooks Authoring Guide\n\nConfiguring hooks for CI pipelines.\n"
	path := filepath.Join(tmp, "hooks.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// "hook authoring" should match — both tokens appear in title
	_, ok := processLearningFile(path, queryTokens("hook authoring"), time.Now())
	if !ok {
		t.Error("expected ok=true: both 'hook' and 'authoring' appear in title")
	}

	// Single token that appears as substring should match
	_, ok = processLearningFile(path, queryTokens("hooks"), time.Now())
	if !ok {
		t.Error("expected ok=true: 'hooks' appears in title")
	}

	// "CI pipeline" — both "ci" and "pipeline" appear in body
	_, ok = processLearningFile(path, queryTokens(strings.ToLower("CI pipeline")), time.Now())
	if !ok {
		t.Error("expected ok=true: both 'ci' and 'pipeline' appear in body")
	}

	// Partial match — one of two tokens present — matches with OR fallback but lower score
	l, ok := processLearningFile(path, queryTokens("hook migration"), time.Now())
	if !ok {
		t.Error("expected ok=true: 'hook' found (OR fallback for partial matches)")
	}
	// Partial match should have lower utility than full match due to ratio penalty
	fullL, _ := processLearningFile(path, queryTokens("hook authoring"), time.Now())
	if ok && l.Utility >= fullL.Utility {
		t.Errorf("partial match utility (%.3f) should be less than full match (%.3f)", l.Utility, fullL.Utility)
	}

	// Completely unrelated tokens should not match
	_, ok = processLearningFile(path, queryTokens("database migration"), time.Now())
	if ok {
		t.Error("expected ok=false: no token overlap with hooks content")
	}
}

func TestQueryTokens(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"hook authoring", []string{"hook", "authoring"}},
		{"CI pipeline", []string{"ci", "pipeline"}},
		{"a b cd efg", []string{"cd", "efg"}},
		{"", nil},
	}
	for _, tt := range tests {
		got := queryTokens(strings.ToLower(tt.input))
		if len(got) == 0 && len(tt.want) == 0 {
			continue
		}
		if len(got) != len(tt.want) {
			t.Errorf("queryTokens(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("queryTokens(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestMatchesQuery(t *testing.T) {
	title := "Hook Authoring Guide"
	summary := "Best practices"
	body := "Configuring hooks for CI pipelines"

	// All tokens present → match
	if !matchesQuery([]string{"hook", "authoring"}, title, summary, body) {
		t.Error("expected match: both tokens present")
	}

	// One token present, one missing → partial match (OR fallback)
	if !matchesQuery([]string{"hook", "database"}, title, summary, body) {
		t.Error("expected partial match: 'hook' is present (OR fallback)")
	}
	// Verify partial match has lower ratio than full match
	fullRatio := matchRatio([]string{"hook", "authoring"}, title, summary, body)
	partialRatio := matchRatio([]string{"hook", "database"}, title, summary, body)
	if partialRatio >= fullRatio {
		t.Errorf("partial ratio (%.2f) should be less than full ratio (%.2f)", partialRatio, fullRatio)
	}

	// No tokens present → no match
	if matchesQuery([]string{"database", "redis"}, title, summary, body) {
		t.Error("expected no match: neither token present")
	}

	// Empty tokens → match everything
	if !matchesQuery(nil, title, summary, body) {
		t.Error("expected match: nil tokens should match all")
	}

	// 2-char token "ci" should work
	if !matchesQuery([]string{"ci", "pipeline"}, title, summary, body) {
		t.Error("expected match: both 'ci' and 'pipeline' present")
	}
}

func TestInjectLearnings_passesQualityGate_EmptyMaturityDefaultsProvisional(t *testing.T) {
	// Empty maturity should default to provisional and pass the gate
	l := learning{Maturity: "", Utility: 0.5}
	if !passesQualityGate(l) {
		t.Error("expected empty maturity to default to provisional and pass quality gate")
	}

	// Draft maturity should still fail
	l.Maturity = "draft"
	if passesQualityGate(l) {
		t.Error("expected draft maturity to fail quality gate")
	}

	// Explicit provisional should still pass
	l.Maturity = "provisional"
	if !passesQualityGate(l) {
		t.Error("expected provisional maturity to pass quality gate")
	}

	// Empty maturity with low utility should still fail (utility gate)
	l = learning{Maturity: "", Utility: 0.1}
	if passesQualityGate(l) {
		t.Error("expected empty maturity with low utility to fail quality gate")
	}
}

func TestInjectLearnings_processLearningFile_SetsDefaultUtility(t *testing.T) {
	tmp := t.TempDir()
	content := "---\nsource_bead: test-fixture\nmaturity: provisional\n---\n# Test Learning for Default Utility\n\nWhen a learning has no explicit utility field, the system should assign InitialUtility as default.\n"
	path := filepath.Join(tmp, "test.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	l, ok := processLearningFile(path, nil, time.Now())
	if !ok {
		t.Fatal("expected ok=true")
	}
	if l.Utility != types.InitialUtility {
		t.Errorf("Utility = %f, want %f (InitialUtility)", l.Utility, types.InitialUtility)
	}
}

// ---------------------------------------------------------------------------
// applyFreshnessScore
// ---------------------------------------------------------------------------

func TestInjectLearnings_applyFreshnessScore(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "learn.md")
	if err := os.WriteFile(path, []byte("# Test"), 0644); err != nil {
		t.Fatal(err)
	}

	l := &learning{}
	now := time.Now()
	applyFreshnessScore(l, path, now)

	// File was just created, so freshness should be near 1.0
	if l.FreshnessScore < 0.9 {
		t.Errorf("FreshnessScore = %f, expected near 1.0 for fresh file", l.FreshnessScore)
	}
}

func TestInjectLearnings_applyFreshnessScore_NonexistentFile(t *testing.T) {
	l := &learning{}
	applyFreshnessScore(l, "/nonexistent/file.md", time.Now())

	if l.FreshnessScore != 0.5 {
		t.Errorf("FreshnessScore = %f, want 0.5 for nonexistent file", l.FreshnessScore)
	}
}

// ---------------------------------------------------------------------------
// parseFrontMatterLine
// ---------------------------------------------------------------------------

func TestInjectLearnings_parseFrontMatterLine(t *testing.T) {
	t.Run("superseded_by", func(t *testing.T) {
		fm := &frontMatter{}
		parseFrontMatterLine("superseded_by: L99", fm)
		if fm.SupersededBy != "L99" {
			t.Errorf("SupersededBy = %q, want L99", fm.SupersededBy)
		}
	})

	t.Run("promoted_to", func(t *testing.T) {
		fm := &frontMatter{}
		parseFrontMatterLine("promoted_to: /global/learn.md", fm)
		if fm.PromotedTo != "/global/learn.md" {
			t.Errorf("PromotedTo = %q, want path", fm.PromotedTo)
		}
	})

	t.Run("utility valid", func(t *testing.T) {
		fm := &frontMatter{}
		parseFrontMatterLine("utility: 0.75", fm)
		if fm.Utility != 0.75 || !fm.HasUtility {
			t.Errorf("Utility = %f, HasUtility = %v", fm.Utility, fm.HasUtility)
		}
	})

	t.Run("utility invalid", func(t *testing.T) {
		fm := &frontMatter{}
		parseFrontMatterLine("utility: abc", fm)
		if fm.HasUtility {
			t.Error("expected HasUtility = false for invalid value")
		}
	})

	t.Run("source_bead", func(t *testing.T) {
		fm := &frontMatter{}
		parseFrontMatterLine("source_bead: ag-xyz", fm)
		if fm.SourceBead != "ag-xyz" {
			t.Errorf("SourceBead = %q, want ag-xyz", fm.SourceBead)
		}
	})

	t.Run("source_phase", func(t *testing.T) {
		fm := &frontMatter{}
		parseFrontMatterLine("source_phase: implement", fm)
		if fm.SourcePhase != "implement" {
			t.Errorf("SourcePhase = %q, want implement", fm.SourcePhase)
		}
	})

	t.Run("unrecognized line is noop", func(t *testing.T) {
		fm := &frontMatter{}
		parseFrontMatterLine("unknown_field: value", fm)
		// Should not set any fields
		if fm.SupersededBy != "" || fm.PromotedTo != "" || fm.HasUtility || fm.SourceBead != "" || fm.SourcePhase != "" {
			t.Error("unrecognized line should not set any fields")
		}
	})
}

// ---------------------------------------------------------------------------
// rankLearnings
// ---------------------------------------------------------------------------

func TestInjectLearnings_rankLearnings(t *testing.T) {
	learnings := []learning{
		{ID: "low", FreshnessScore: 0.2, Utility: 0.2},
		{ID: "high", FreshnessScore: 0.9, Utility: 0.9},
		{ID: "mid", FreshnessScore: 0.5, Utility: 0.5},
	}

	rankLearnings(learnings)

	// After ranking, items should be sorted by composite score descending
	if learnings[0].ID != "high" {
		t.Errorf("first item should be 'high', got %q", learnings[0].ID)
	}
	if learnings[len(learnings)-1].ID != "low" {
		t.Errorf("last item should be 'low', got %q", learnings[len(learnings)-1].ID)
	}

	// Verify descending order (composite scores can be negative due to z-score normalization)
	for i := 1; i < len(learnings); i++ {
		if learnings[i].CompositeScore > learnings[i-1].CompositeScore {
			t.Errorf("learnings not sorted descending: [%d]=%f > [%d]=%f",
				i, learnings[i].CompositeScore, i-1, learnings[i-1].CompositeScore)
		}
	}
}

// ---------------------------------------------------------------------------
// collectLearnings with global dir
// ---------------------------------------------------------------------------

func TestLocalLearningDedupeSetsTracksAbsPathsAndLowerTitles(t *testing.T) {
	file := filepath.Join(t.TempDir(), "learning.md")
	paths, titles := localLearningDedupeSets([]string{file}, []learning{{Title: "Case Sensitive Title"}})

	abs, err := filepath.Abs(file)
	if err != nil {
		t.Fatal(err)
	}
	if !paths[abs] {
		t.Fatalf("local path %q was not tracked: %#v", abs, paths)
	}
	if !titles["case sensitive title"] {
		t.Fatalf("lowercase title was not tracked: %#v", titles)
	}
}

func TestInjectLearnings_collectLearnings_WithGlobalDir(t *testing.T) {
	// Create local learnings
	localDir := t.TempDir()
	localLearnings := filepath.Join(localDir, ".agents", "learnings")
	if err := os.MkdirAll(localLearnings, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localLearnings, "local.md"), []byte("---\nmaturity: provisional\n---\n# Local Learning\n\nLocal content for testing that local learnings are discovered and included in results correctly.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create global learnings in separate dir
	globalDir := t.TempDir()
	globalNamespace := filepath.Join(globalDir, "jren-platform")
	if err := os.MkdirAll(globalNamespace, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalNamespace, "global.md"), []byte("---\nmaturity: provisional\n---\n# Global Learning\n\nGlobal content for testing cross-rig knowledge retrieval and the global weight penalty.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	learnings, err := collectLearnings(localDir, "", 10, globalDir, 0.8)
	if err != nil {
		t.Fatalf("collectLearnings: %v", err)
	}

	if len(learnings) < 2 {
		t.Errorf("expected at least 2 learnings (local + global), got %d", len(learnings))
	}

	// Check that global learning has Global flag set
	hasGlobal := false
	for _, l := range learnings {
		if l.Global {
			hasGlobal = true
		}
	}
	if !hasGlobal {
		t.Error("expected at least one global learning")
	}
}

func TestInjectLearnings_collectLearnings_GlobalWeightPenalty(t *testing.T) {
	// Create local learning
	localDir := t.TempDir()
	localLearnings := filepath.Join(localDir, ".agents", "learnings")
	if err := os.MkdirAll(localLearnings, 0755); err != nil {
		t.Fatal(err)
	}

	localData := map[string]any{"id": "local-1", "title": "Local", "summary": "Local learning", "utility": 0.8, "maturity": "provisional"}
	b, _ := json.Marshal(localData)
	if err := os.WriteFile(filepath.Join(localLearnings, "local.jsonl"), b, 0644); err != nil {
		t.Fatal(err)
	}

	// Create global learning with same utility
	globalDir := t.TempDir()
	globalData := map[string]any{"id": "global-1", "title": "Global", "summary": "Global learning", "utility": 0.8, "maturity": "provisional"}
	bg, _ := json.Marshal(globalData)
	globalNamespace := filepath.Join(globalDir, "jren-platform")
	if err := os.MkdirAll(globalNamespace, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalNamespace, "global.jsonl"), bg, 0644); err != nil {
		t.Fatal(err)
	}

	learnings, err := collectLearnings(localDir, "", 10, globalDir, 0.5)
	if err != nil {
		t.Fatalf("collectLearnings: %v", err)
	}

	// The global learning should have a lower composite score due to weight penalty
	var localScore, globalScore float64
	for _, l := range learnings {
		if l.Global {
			globalScore = l.CompositeScore
		} else {
			localScore = l.CompositeScore
		}
	}

	if globalScore >= localScore {
		t.Errorf("global score (%f) should be less than local score (%f) due to weight penalty",
			globalScore, localScore)
	}
}

func TestInjectLearnings_collectLearnings_SectionRollupEvidence(t *testing.T) {
	dir := t.TempDir()
	learningsDir := filepath.Join(dir, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `---
maturity: provisional
utility: 0.9
source_bead: ag-9qm
---
# Flywheel Recovery

Recovery plan for retrieval experiments.

## Shadow Rollback

Shadow rollback keeps the primary lane clean during retrieval experiments.

## Metrics Audit

Metrics audit compares shadow and primary before promotion.`
	if err := os.WriteFile(filepath.Join(learningsDir, "recovery.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := collectLearnings(dir, "shadow metrics rollback", 10, "", 0)
	if err != nil {
		t.Fatalf("collectLearnings: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 rolled-up result, got %d", len(results))
	}

	got := results[0]
	if got.SectionHeading != "Shadow Rollback" {
		t.Fatalf("SectionHeading = %q, want %q", got.SectionHeading, "Shadow Rollback")
	}
	if got.SectionLocator == "" {
		t.Fatal("expected non-empty SectionLocator")
	}
	if !strings.Contains(strings.ToLower(got.MatchedSnippet), "shadow rollback") {
		t.Fatalf("MatchedSnippet = %q, want shadow rollback evidence", got.MatchedSnippet)
	}
	if got.MatchProvenance != "section-rollup" {
		t.Fatalf("MatchProvenance = %q, want section-rollup", got.MatchProvenance)
	}
	if got.MatchConfidence <= (2.0 / 3.0) {
		t.Fatalf("MatchConfidence = %.3f, want corroborating-section bonus over 0.667", got.MatchConfidence)
	}
}

// ---------------------------------------------------------------------------
// validPhases map
// ---------------------------------------------------------------------------

func TestInjectLearnings_validPhases(t *testing.T) {
	expected := []string{"research", "plan", "implement", "validate"}
	for _, phase := range expected {
		if !validPhases[phase] {
			t.Errorf("expected %q in validPhases", phase)
		}
	}
	if validPhases["build"] {
		t.Error("'build' should not be a valid phase")
	}
}

// ---------------------------------------------------------------------------
// extractSummary
// ---------------------------------------------------------------------------

func TestInjectLearnings_extractSummary_OnlySkippableLines(t *testing.T) {
	// All lines are headings, empty, or frontmatter delimiters — no content to extract
	lines := []string{"---", "---", "", "# Heading Only", ""}
	got := extractSummary(lines, 0)
	if got != "" {
		t.Errorf("expected empty summary for only skippable lines, got %q", got)
	}
}

func TestInjectLearnings_extractSummary_FrontmatterContentIncluded(t *testing.T) {
	// Note: extractSummary treats frontmatter key/value lines as content
	// (it only skips "---" delimiters, not frontmatter field lines)
	lines := []string{"---", "title: test", "---"}
	got := extractSummary(lines, 0)
	if got == "" {
		t.Error("expected non-empty summary (frontmatter fields are treated as content)")
	}
}

func TestInjectLearnings_extractSummary_LongParagraph(t *testing.T) {
	long := strings.Repeat("word ", 60)
	lines := []string{"# Title", "", long}
	got := extractSummary(lines, 0)
	if len(got) > 200 {
		t.Errorf("summary too long: %d chars", len(got))
	}
}

// ---------------------------------------------------------------------------
// TestInjectLearnings_StabilityWeight
// ---------------------------------------------------------------------------

func TestInjectLearnings_StabilityWeight(t *testing.T) {
	dir := t.TempDir()

	writeFile := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		return p
	}

	// Experimental learning — same base utility but marked experimental.
	expPath := writeFile("experimental.md", `---
title: Experimental Learning
utility: 0.8
stability: experimental
---
# Experimental Learning

This is a test learning with experimental stability.
`)

	// Stable learning — same base utility, default stability.
	stablePath := writeFile("stable.md", `---
title: Stable Learning
utility: 0.8
stability: stable
---
# Stable Learning

This is a test learning with stable stability.
`)

	now := time.Now()
	tokens := queryTokens("learning")

	expL, ok := processLearningFile(expPath, tokens, now)
	if !ok {
		t.Fatalf("processLearningFile(%s) returned ok=false", expPath)
	}
	stableL, ok := processLearningFile(stablePath, tokens, now)
	if !ok {
		t.Fatalf("processLearningFile(%s) returned ok=false", stablePath)
	}

	// Experimental should have lower utility than stable (0.7x multiplier).
	if expL.Utility >= stableL.Utility {
		t.Errorf("experimental utility (%v) should be < stable utility (%v)", expL.Utility, stableL.Utility)
	}

	// Verify the ratio is approximately 0.7.
	ratio := expL.Utility / stableL.Utility
	if ratio < 0.65 || ratio > 0.75 {
		t.Errorf("utility ratio = %v, want ~0.7 (experimental vs stable)", ratio)
	}
}
