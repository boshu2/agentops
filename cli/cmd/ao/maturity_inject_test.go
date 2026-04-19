package main

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/search"
	"github.com/boshu2/agentops/cli/internal/types"
)

// ---------------------------------------------------------------------------
// inject_scoring.go — freshnessScore
// ---------------------------------------------------------------------------

func TestMaturity_freshnessScore(t *testing.T) {
	tests := []struct {
		name     string
		ageWeeks float64
		wantMin  float64
		wantMax  float64
	}{
		{"zero age returns 1.0", 0, 1.0, 1.0},
		{"one week decayed", 1, 0.83, 0.86},
		{"four weeks decayed", 4, 0.50, 0.52},
		{"very old clamped to 0.1", 100, 0.1, 0.1},
		{"twenty weeks", 20, 0.1, 0.1},
		{"half week", 0.5, 0.91, 0.93},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := freshnessScore(tt.ageWeeks)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("freshnessScore(%f) = %f, want [%f, %f]", tt.ageWeeks, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// inject_scoring.go — applyCompositeScoringTo
// ---------------------------------------------------------------------------

func TestMaturity_applyCompositeScoringTo(t *testing.T) {
	t.Run("empty slice is no-op", func(t *testing.T) {
		applyCompositeScoringTo(nil, 0.5)
		applyCompositeScoringTo([]scorable{}, 0.5)
		// Should not panic
	})

	t.Run("single item gets raw weighted score", func(t *testing.T) {
		l := &learning{FreshnessScore: 0.8, Utility: 0.6}
		applyCompositeScoringTo([]scorable{l}, 0.5)
		// With N < 3, uses raw weighted score: freshness + lambda*utility = 0.8 + 0.5*0.6 = 1.1
		expected := 0.8 + 0.5*0.6
		if math.Abs(l.CompositeScore-expected) > 0.01 {
			t.Errorf("single item composite = %f, expected %f", l.CompositeScore, expected)
		}
	})

	t.Run("higher freshness and utility rank higher", func(t *testing.T) {
		high := &learning{FreshnessScore: 0.9, Utility: 0.9}
		low := &learning{FreshnessScore: 0.1, Utility: 0.1}
		mid := &learning{FreshnessScore: 0.5, Utility: 0.5}
		applyCompositeScoringTo([]scorable{high, low, mid}, 0.5)
		if high.CompositeScore <= mid.CompositeScore {
			t.Errorf("high (%f) should rank above mid (%f)", high.CompositeScore, mid.CompositeScore)
		}
		if mid.CompositeScore <= low.CompositeScore {
			t.Errorf("mid (%f) should rank above low (%f)", mid.CompositeScore, low.CompositeScore)
		}
	})

	t.Run("pattern type works too", func(t *testing.T) {
		p := &pattern{FreshnessScore: 0.7, Utility: 0.8}
		applyCompositeScoringTo([]scorable{p}, 1.0)
		// Single item → raw weighted score (N < 3 bypass); verify no panic with pattern type.
		_ = p.CompositeScore // use the variable to prove scoring ran
	})

	t.Run("lambda zero ignores utility", func(t *testing.T) {
		fresh := &learning{FreshnessScore: 0.9, Utility: 0.1}
		useful := &learning{FreshnessScore: 0.1, Utility: 0.9}
		applyCompositeScoringTo([]scorable{fresh, useful}, 0.0)
		if fresh.CompositeScore <= useful.CompositeScore {
			t.Errorf("lambda=0: fresh (%f) should rank above useful (%f)",
				fresh.CompositeScore, useful.CompositeScore)
		}
	})

	t.Run("large lambda favors utility", func(t *testing.T) {
		fresh := &learning{FreshnessScore: 0.9, Utility: 0.1}
		useful := &learning{FreshnessScore: 0.1, Utility: 0.9}
		applyCompositeScoringTo([]scorable{fresh, useful}, 10.0)
		if useful.CompositeScore <= fresh.CompositeScore {
			t.Errorf("lambda=10: useful (%f) should rank above fresh (%f)",
				useful.CompositeScore, fresh.CompositeScore)
		}
	})
}

// ---------------------------------------------------------------------------
// inject.go — truncateText
// ---------------------------------------------------------------------------

func TestInject_truncateText(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short string unchanged", "hello", 10, "hello"},
		{"exact length unchanged", "hello", 5, "hello"},
		{"truncated with ellipsis", "hello world", 8, "hello..."},
		{"very short max", "abcdef", 4, "a..."},
		{"empty string", "", 10, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateText(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateText(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// inject.go — trimToCharBudget
// ---------------------------------------------------------------------------

func TestInject_trimToCharBudget(t *testing.T) {
	t.Run("under budget returns as-is", func(t *testing.T) {
		input := "short text"
		got := trimToCharBudget(input, 100)
		if got != input {
			t.Errorf("expected unchanged output, got %q", got)
		}
	})

	t.Run("over budget truncates with marker", func(t *testing.T) {
		lines := make([]string, 100)
		for i := range lines {
			lines[i] = "This is a line of text that is reasonably long."
		}
		input := strings.Join(lines, "\n")
		got := trimToCharBudget(input, 200)
		if len(got) > 250 { // budget + truncation marker
			t.Errorf("output too long: %d chars", len(got))
		}
		if !strings.Contains(got, "[truncated to fit token budget]") {
			t.Error("expected truncation marker")
		}
	})

	t.Run("exactly at budget returns as-is", func(t *testing.T) {
		input := strings.Repeat("a", 50)
		got := trimToCharBudget(input, 50)
		if got != input {
			t.Errorf("expected unchanged output at exact budget")
		}
	})
}

// ---------------------------------------------------------------------------
// inject.go — formatKnowledgeMarkdown
// ---------------------------------------------------------------------------

func TestInject_formatKnowledgeMarkdown(t *testing.T) {
	t.Run("empty knowledge shows no-prior-knowledge message", func(t *testing.T) {
		k := &injectedKnowledge{
			Timestamp: time.Date(2026, 2, 20, 12, 0, 0, 0, time.UTC),
		}
		got := formatKnowledgeMarkdown(k)
		if !strings.Contains(got, "No prior knowledge found") {
			t.Error("expected no-prior-knowledge message")
		}
		if !strings.Contains(got, "2026-02-20") {
			t.Error("expected timestamp in output")
		}
	})

	t.Run("with all sections", func(t *testing.T) {
		k := &injectedKnowledge{
			Learnings: []learning{{ID: "L1", Title: "Test"}},
			Patterns:  []pattern{{Name: "P1", Description: "desc"}},
			Sessions:  []session{{Date: "2026-02-20", Summary: "work"}},
			Timestamp: time.Date(2026, 2, 20, 12, 0, 0, 0, time.UTC),
		}
		got := formatKnowledgeMarkdown(k)
		if strings.Contains(got, "No prior knowledge found") {
			t.Error("should NOT show no-prior-knowledge when data exists")
		}
		if !strings.Contains(got, "Injected Knowledge") {
			t.Error("expected header")
		}
	})
}

// ---------------------------------------------------------------------------
// inject.go — findAgentsSubdir
// ---------------------------------------------------------------------------

func TestInject_findAgentsSubdir(t *testing.T) {
	t.Run("finds in current dir", func(t *testing.T) {
		tmp := t.TempDir()
		agentsDir := filepath.Join(tmp, ".agents", "learnings")
		if err := os.MkdirAll(agentsDir, 0755); err != nil {
			t.Fatal(err)
		}
		got := findAgentsSubdir(tmp, "learnings")
		if got != agentsDir {
			t.Errorf("expected %q, got %q", agentsDir, got)
		}
	})

	t.Run("walks up to rig root", func(t *testing.T) {
		tmp := t.TempDir()
		// Create .beads marker at root to stop traversal
		if err := os.MkdirAll(filepath.Join(tmp, ".beads"), 0755); err != nil {
			t.Fatal(err)
		}
		agentsDir := filepath.Join(tmp, ".agents", "patterns")
		if err := os.MkdirAll(agentsDir, 0755); err != nil {
			t.Fatal(err)
		}
		subDir := filepath.Join(tmp, "sub", "deep")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatal(err)
		}
		got := findAgentsSubdir(subDir, "patterns")
		if got != agentsDir {
			t.Errorf("expected %q, got %q", agentsDir, got)
		}
	})

	t.Run("returns empty when not found", func(t *testing.T) {
		tmp := t.TempDir()
		// Create rig root marker to stop traversal
		if err := os.MkdirAll(filepath.Join(tmp, ".beads"), 0755); err != nil {
			t.Fatal(err)
		}
		got := findAgentsSubdir(tmp, "nonexistent")
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}

// ---------------------------------------------------------------------------
// ratchet.go — statusIcon
// ---------------------------------------------------------------------------

func TestRatchet_statusIcon(t *testing.T) {
	tests := []struct {
		status ratchet.StepStatus
		want   string
	}{
		{ratchet.StatusLocked, "\u2713"},          // checkmark
		{ratchet.StatusSkipped, "\u2298"},         // circled slash
		{ratchet.StatusInProgress, "\u25d0"},      // half circle
		{ratchet.StatusPending, "\u25cb"},         // open circle
		{ratchet.StepStatus("unknown"), "\u25cb"}, // default
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := statusIcon(tt.status)
			if got != tt.want {
				t.Errorf("statusIcon(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ratchet.go — truncate
// ---------------------------------------------------------------------------

func TestRatchet_truncate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		max   int
		want  string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"truncated", "hello world", 8, "hello..."},
		{"empty", "", 5, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.max)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ratchet_validate.go — buildValidateOptions
// ---------------------------------------------------------------------------

func TestRatchet_buildValidateOptions(t *testing.T) {
	t.Run("default not lenient", func(t *testing.T) {
		origLenient := ratchetLenient
		origDays := ratchetLenientDays
		defer func() {
			ratchetLenient = origLenient
			ratchetLenientDays = origDays
		}()

		ratchetLenient = false
		ratchetLenientDays = 90
		opts := buildValidateOptions()
		if opts.Lenient {
			t.Error("expected Lenient=false")
		}
		if opts.LenientExpiryDate != nil {
			t.Error("expected nil expiry date when not lenient")
		}
	})

	t.Run("lenient with expiry", func(t *testing.T) {
		origLenient := ratchetLenient
		origDays := ratchetLenientDays
		defer func() {
			ratchetLenient = origLenient
			ratchetLenientDays = origDays
		}()

		ratchetLenient = true
		ratchetLenientDays = 30
		opts := buildValidateOptions()
		if !opts.Lenient {
			t.Error("expected Lenient=true")
		}
		if opts.LenientExpiryDate == nil {
			t.Fatal("expected non-nil expiry date")
		}
		// Expiry should be ~30 days from now
		expected := time.Now().AddDate(0, 0, 30)
		diff := opts.LenientExpiryDate.Sub(expected)
		if diff > time.Minute || diff < -time.Minute {
			t.Errorf("expiry date off: got %v, expected ~%v", opts.LenientExpiryDate, expected)
		}
	})
}

// ---------------------------------------------------------------------------
// ratchet_validate.go — formatValidationStatus, formatLenientInfo
// ---------------------------------------------------------------------------

func TestRatchet_formatValidationStatus(t *testing.T) {
	t.Run("valid result", func(t *testing.T) {
		var buf bytes.Buffer
		allValid := true
		formatValidationStatus(&buf, &ratchet.ValidationResult{Valid: true}, &allValid)
		if !strings.Contains(buf.String(), "VALID") {
			t.Error("expected VALID in output")
		}
		if !allValid {
			t.Error("allValid should remain true")
		}
	})

	t.Run("invalid result", func(t *testing.T) {
		var buf bytes.Buffer
		allValid := true
		formatValidationStatus(&buf, &ratchet.ValidationResult{Valid: false}, &allValid)
		if !strings.Contains(buf.String(), "INVALID") {
			t.Error("expected INVALID in output")
		}
		if allValid {
			t.Error("allValid should be false")
		}
	})
}

func TestRatchet_formatLenientInfo(t *testing.T) {
	t.Run("not lenient writes nothing", func(t *testing.T) {
		var buf bytes.Buffer
		formatLenientInfo(&buf, &ratchet.ValidationResult{Lenient: false})
		if buf.Len() != 0 {
			t.Errorf("expected no output, got %q", buf.String())
		}
	})

	t.Run("lenient without expiry", func(t *testing.T) {
		var buf bytes.Buffer
		formatLenientInfo(&buf, &ratchet.ValidationResult{Lenient: true})
		if !strings.Contains(buf.String(), "LENIENT") {
			t.Error("expected LENIENT in output")
		}
	})

	t.Run("lenient with expiry and expiring soon", func(t *testing.T) {
		var buf bytes.Buffer
		expiry := "2026-03-01"
		formatLenientInfo(&buf, &ratchet.ValidationResult{
			Lenient:             true,
			LenientExpiryDate:   &expiry,
			LenientExpiringSoon: true,
		})
		out := buf.String()
		if !strings.Contains(out, "2026-03-01") {
			t.Error("expected expiry date")
		}
		if !strings.Contains(out, "Expiring soon") {
			t.Error("expected expiring soon warning")
		}
	})
}

// ---------------------------------------------------------------------------
// ratchet_validate.go — outputValidationResult
// ---------------------------------------------------------------------------

func TestRatchet_outputValidationResult(t *testing.T) {
	t.Run("text output", func(t *testing.T) {
		origOutput := output
		output = "table"
		defer func() { output = origOutput }()

		var buf bytes.Buffer
		allValid := true
		result := &ratchet.ValidationResult{Valid: true}
		err := outputValidationResult(&buf, "test.md", result, &allValid)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(buf.String(), "test.md") {
			t.Error("expected filename in output")
		}
	})

	t.Run("json output", func(t *testing.T) {
		origOutput := output
		output = "json"
		defer func() { output = origOutput }()

		var buf bytes.Buffer
		allValid := true
		result := &ratchet.ValidationResult{Valid: true}
		err := outputValidationResult(&buf, "test.md", result, &allValid)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasPrefix(strings.TrimSpace(buf.String()), "{") {
			t.Error("expected JSON output")
		}
	})
}

// ---------------------------------------------------------------------------
// ratchet_status.go — outputRatchetStatus YAML
// ---------------------------------------------------------------------------

func TestRatchet_outputRatchetStatusYAML(t *testing.T) {
	origOutput := output
	output = "yaml"
	defer func() { output = origOutput }()

	data := &ratchetStatusOutput{
		ChainID: "yaml-chain",
		Started: "2026-01-15T10:00:00Z",
		Steps:   []ratchetStepInfo{},
		Path:    "/tmp/chain.jsonl",
	}

	var buf bytes.Buffer
	err := outputRatchetStatus(&buf, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "yaml-chain") {
		t.Error("expected chain ID in YAML output")
	}
}

func TestRatchet_outputRatchetStatusTableWithCycle(t *testing.T) {
	origOutput := output
	output = "table"
	defer func() { output = origOutput }()

	data := &ratchetStatusOutput{
		ChainID: "cycle-chain",
		Started: "2026-01-15T10:00:00Z",
		Steps: []ratchetStepInfo{
			{Step: ratchet.StepResearch, Status: ratchet.StatusLocked, Cycle: 2, ParentEpic: "ag-parent"},
		},
		Path: "/tmp/chain.jsonl",
	}

	var buf bytes.Buffer
	err := outputRatchetStatus(&buf, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Cycle: 2") {
		t.Error("expected Cycle in output")
	}
	if !strings.Contains(out, "Parent: ag-parent") {
		t.Error("expected Parent in output")
	}
}

// ---------------------------------------------------------------------------
// inject_learnings.go — jsonFloat
// ---------------------------------------------------------------------------

func TestInject_jsonFloat(t *testing.T) {
	tests := []struct {
		name       string
		data       map[string]any
		key        string
		defaultVal float64
		want       float64
	}{
		{"present positive", map[string]any{"x": 0.8}, "x", 0.5, 0.8},
		{"present zero returns default", map[string]any{"x": 0.0}, "x", 0.5, 0.5},
		{"present negative returns default", map[string]any{"x": -0.5}, "x", 0.5, 0.5},
		{"missing returns default", map[string]any{}, "x", 0.3, 0.3},
		{"wrong type returns default", map[string]any{"x": "hello"}, "x", 0.5, 0.5},
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
// inject_learnings.go — jsonTimeField
// ---------------------------------------------------------------------------

func TestInject_jsonTimeField(t *testing.T) {
	t.Run("parses first valid key", func(t *testing.T) {
		data := map[string]any{
			"last_decay_at": "2026-02-20T12:00:00Z",
		}
		got := jsonTimeField(data, "last_decay_at", "last_reward_at")
		if got.IsZero() {
			t.Error("expected non-zero time")
		}
		if got.Year() != 2026 || got.Month() != 2 || got.Day() != 20 {
			t.Errorf("parsed time = %v, expected 2026-02-20", got)
		}
	})

	t.Run("falls back to second key", func(t *testing.T) {
		data := map[string]any{
			"last_reward_at": "2026-01-15T10:00:00Z",
		}
		got := jsonTimeField(data, "missing_key", "last_reward_at")
		if got.IsZero() {
			t.Error("expected non-zero time from fallback key")
		}
	})

	t.Run("returns zero for missing keys", func(t *testing.T) {
		data := map[string]any{"other": "value"}
		got := jsonTimeField(data, "a", "b")
		if !got.IsZero() {
			t.Errorf("expected zero time, got %v", got)
		}
	})

	t.Run("returns zero for invalid format", func(t *testing.T) {
		data := map[string]any{"date": "not-a-date"}
		got := jsonTimeField(data, "date")
		if !got.IsZero() {
			t.Errorf("expected zero time for invalid format, got %v", got)
		}
	})

	t.Run("returns zero for empty string", func(t *testing.T) {
		data := map[string]any{"date": ""}
		got := jsonTimeField(data, "date")
		if !got.IsZero() {
			t.Errorf("expected zero time for empty string, got %v", got)
		}
	})
}

// ---------------------------------------------------------------------------
// inject_learnings.go — computeDecayedConfidence
// ---------------------------------------------------------------------------

func TestInject_computeDecayedConfidence(t *testing.T) {
	tests := []struct {
		name       string
		confidence float64
		weeks      float64
		wantMin    float64
		wantMax    float64
	}{
		{"no decay at zero weeks", 0.8, 0, 0.79, 0.81},
		{"some decay at 2 weeks", 0.8, 2, 0.5, 0.7},
		{"clamped to 0.1 at high weeks", 0.8, 100, 0.1, 0.1},
		{"already low confidence decays further", 0.15, 5, 0.1, 0.1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeDecayedConfidence(tt.confidence, tt.weeks)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("computeDecayedConfidence(%f, %f) = %f, want [%f, %f]",
					tt.confidence, tt.weeks, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// inject_learnings.go — writeDecayFields
// ---------------------------------------------------------------------------

func TestInject_writeDecayFields(t *testing.T) {
	t.Run("sets all fields", func(t *testing.T) {
		data := map[string]any{}
		now := time.Date(2026, 2, 20, 12, 0, 0, 0, time.UTC)
		writeDecayFields(data, 0.42, now)

		if data["confidence"] != 0.42 {
			t.Errorf("confidence = %v, want 0.42", data["confidence"])
		}
		if data["last_decay_at"] != now.Format(time.RFC3339) {
			t.Errorf("last_decay_at = %v", data["last_decay_at"])
		}
		if data["decay_count"] != 1.0 {
			t.Errorf("decay_count = %v, want 1.0", data["decay_count"])
		}
	})

	t.Run("increments existing decay_count", func(t *testing.T) {
		data := map[string]any{"decay_count": 3.0}
		writeDecayFields(data, 0.5, time.Now())
		if data["decay_count"] != 4.0 {
			t.Errorf("decay_count = %v, want 4.0", data["decay_count"])
		}
	})
}

// ---------------------------------------------------------------------------
// inject_learnings.go — parseFrontMatter
// ---------------------------------------------------------------------------

func TestInject_parseFrontMatter(t *testing.T) {
	tests := []struct {
		name        string
		lines       []string
		wantIdx     int
		wantSuper   string
		wantUtility float64
		wantHasUtil bool
	}{
		{
			name:    "no frontmatter",
			lines:   []string{"# Title", "Content"},
			wantIdx: 0,
		},
		{
			name:    "empty lines",
			lines:   []string{},
			wantIdx: 0,
		},
		{
			name:      "valid frontmatter with superseded_by",
			lines:     []string{"---", "superseded_by: L-new", "---", "# Title"},
			wantIdx:   3,
			wantSuper: "L-new",
		},
		{
			name:      "superseded-by with dash",
			lines:     []string{"---", "superseded-by: L-other", "---"},
			wantIdx:   3,
			wantSuper: "L-other",
		},
		{
			name:        "utility parsed",
			lines:       []string{"---", "utility: 0.85", "---"},
			wantIdx:     3,
			wantUtility: 0.85,
			wantHasUtil: true,
		},
		{
			name:    "unclosed frontmatter",
			lines:   []string{"---", "utility: 0.5", "content"},
			wantIdx: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, idx := parseFrontMatter(tt.lines)
			if idx != tt.wantIdx {
				t.Errorf("idx = %d, want %d", idx, tt.wantIdx)
			}
			if fm.SupersededBy != tt.wantSuper {
				t.Errorf("SupersededBy = %q, want %q", fm.SupersededBy, tt.wantSuper)
			}
			if tt.wantHasUtil {
				if !fm.HasUtility {
					t.Error("expected HasUtility=true")
				}
				if math.Abs(fm.Utility-tt.wantUtility) > 0.01 {
					t.Errorf("Utility = %f, want %f", fm.Utility, tt.wantUtility)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// inject_learnings.go — isSuperseded
// ---------------------------------------------------------------------------

func TestInject_isSuperseded(t *testing.T) {
	tests := []struct {
		name string
		fm   frontMatter
		want bool
	}{
		{"empty superseded_by", frontMatter{SupersededBy: ""}, false},
		{"null superseded_by", frontMatter{SupersededBy: "null"}, false},
		{"tilde superseded_by", frontMatter{SupersededBy: "~"}, false},
		{"valid superseded_by", frontMatter{SupersededBy: "L-new"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSuperseded(tt.fm)
			if got != tt.want {
				t.Errorf("isSuperseded(%v) = %v, want %v", tt.fm, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// inject_learnings.go — extractSummary
// ---------------------------------------------------------------------------

func TestInject_extractSummary(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		startIdx int
		want     string
	}{
		{
			name:     "first paragraph",
			lines:    []string{"# Title", "", "This is the summary.", "More text."},
			startIdx: 0,
			want:     "This is the summary. More text.",
		},
		{
			name:     "skips headings and blanks",
			lines:    []string{"", "# Heading", "", "Content here."},
			startIdx: 0,
			want:     "Content here.",
		},
		{
			name:     "empty lines only",
			lines:    []string{"", "", ""},
			startIdx: 0,
			want:     "",
		},
		{
			name:     "starts after frontmatter",
			lines:    []string{"---", "key: val", "---", "Summary text."},
			startIdx: 3,
			want:     "Summary text.",
		},
		{
			name:     "stops at heading in paragraph",
			lines:    []string{"First line.", "# Second heading"},
			startIdx: 0,
			want:     "First line.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSummary(tt.lines, tt.startIdx)
			if !strings.HasPrefix(got, tt.want) && tt.want != "" {
				t.Errorf("extractSummary() = %q, want prefix %q", got, tt.want)
			}
			if tt.want == "" && got != "" {
				t.Errorf("extractSummary() = %q, want empty", got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// inject_learnings.go — parseLearningBody
// ---------------------------------------------------------------------------

func TestInject_parseLearningBody(t *testing.T) {
	t.Run("extracts title and ID", func(t *testing.T) {
		lines := []string{"# My Learning", "ID: L-042", "Some content."}
		// parseLearningBody only replaces ID if l.ID == filepath.Base(l.Source)
		l := &learning{ID: "default.md", Source: "/path/to/default.md"}
		parseLearningBody(lines, 0, l)
		if l.Title != "My Learning" {
			t.Errorf("Title = %q, want %q", l.Title, "My Learning")
		}
		if l.ID != "L-042" {
			t.Errorf("ID = %q, want %q", l.ID, "L-042")
		}
	})

	t.Run("lowercase id: also works", func(t *testing.T) {
		lines := []string{"id: low-001"}
		l := &learning{ID: "default.md", Source: "/path/to/default.md"}
		parseLearningBody(lines, 0, l)
		if l.ID != "low-001" {
			t.Errorf("ID = %q, want %q", l.ID, "low-001")
		}
	})

	t.Run("does not override existing title", func(t *testing.T) {
		lines := []string{"# New Title"}
		l := &learning{Source: "/path/to/file.md", Title: "Existing"}
		parseLearningBody(lines, 0, l)
		if l.Title != "Existing" {
			t.Errorf("Title = %q, should not override", l.Title)
		}
	})
}

// ---------------------------------------------------------------------------
// inject_learnings.go — populateLearningFromJSON
// ---------------------------------------------------------------------------

func TestInject_populateLearningFromJSON(t *testing.T) {
	t.Run("all fields", func(t *testing.T) {
		data := map[string]any{
			"id":      "L-100",
			"title":   "Test Title",
			"summary": "Test Summary",
			"utility": 0.75,
		}
		l := &learning{}
		populateLearningFromJSON(data, l)
		if l.ID != "L-100" {
			t.Errorf("ID = %q", l.ID)
		}
		if l.Title != "Test Title" {
			t.Errorf("Title = %q", l.Title)
		}
		if l.Summary != "Test Summary" {
			t.Errorf("Summary = %q", l.Summary)
		}
		if l.Utility != 0.75 {
			t.Errorf("Utility = %f", l.Utility)
		}
	})

	t.Run("content fallback for summary", func(t *testing.T) {
		data := map[string]any{
			"content": "Content as summary",
		}
		l := &learning{}
		populateLearningFromJSON(data, l)
		if l.Summary != "Content as summary" {
			t.Errorf("Summary = %q, expected content fallback", l.Summary)
		}
	})

	t.Run("summary takes priority over content", func(t *testing.T) {
		data := map[string]any{
			"summary": "Real summary",
			"content": "Content text",
		}
		l := &learning{}
		populateLearningFromJSON(data, l)
		if l.Summary != "Real summary" {
			t.Errorf("Summary = %q, expected summary to take priority", l.Summary)
		}
	})

	t.Run("zero utility not set", func(t *testing.T) {
		data := map[string]any{
			"utility": 0.0,
		}
		l := &learning{Utility: types.InitialUtility}
		populateLearningFromJSON(data, l)
		if l.Utility != types.InitialUtility {
			t.Errorf("Utility = %f, expected unchanged default", l.Utility)
		}
	})
}

// ---------------------------------------------------------------------------
// inject_patterns.go — enrichPatternFreshness
// ---------------------------------------------------------------------------

func TestInject_enrichPatternFreshness(t *testing.T) {
	t.Run("with real file", func(t *testing.T) {
		tmp := t.TempDir()
		fpath := filepath.Join(tmp, "pattern.md")
		if err := os.WriteFile(fpath, []byte("# Pattern"), 0644); err != nil {
			t.Fatal(err)
		}
		p := &pattern{}
		enrichPatternFreshness(p, fpath, time.Now())
		if p.FreshnessScore <= 0 || p.FreshnessScore > 1.0 {
			t.Errorf("FreshnessScore = %f, expected (0, 1]", p.FreshnessScore)
		}
		if p.Utility != types.InitialUtility {
			t.Errorf("Utility = %f, expected default %f", p.Utility, types.InitialUtility)
		}
	})

	t.Run("nonexistent file gets default", func(t *testing.T) {
		p := &pattern{}
		enrichPatternFreshness(p, "/nonexistent/file.md", time.Now())
		if p.FreshnessScore != 0.5 {
			t.Errorf("FreshnessScore = %f, expected 0.5 for missing file", p.FreshnessScore)
		}
	})

	t.Run("preserves existing utility", func(t *testing.T) {
		tmp := t.TempDir()
		fpath := filepath.Join(tmp, "pattern.md")
		if err := os.WriteFile(fpath, []byte("# Pattern"), 0644); err != nil {
			t.Fatal(err)
		}
		p := &pattern{Utility: 0.9}
		enrichPatternFreshness(p, fpath, time.Now())
		if p.Utility != 0.9 {
			t.Errorf("Utility = %f, expected 0.9 (preserved)", p.Utility)
		}
	})
}

// ---------------------------------------------------------------------------
// maturity.go — readLearningJSONLData
// ---------------------------------------------------------------------------

func TestMaturity_readLearningJSONLData(t *testing.T) {
	t.Run("valid JSONL file", func(t *testing.T) {
		tmp := t.TempDir()
		fpath := filepath.Join(tmp, "learning.jsonl")
		content := `{"id":"L-001","utility":0.8,"maturity":"candidate"}`
		if err := os.WriteFile(fpath, []byte(content+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		data, ok := readLearningJSONLData(fpath)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if data["id"] != "L-001" {
			t.Errorf("id = %v", data["id"])
		}
	})

	t.Run("empty file returns false", func(t *testing.T) {
		tmp := t.TempDir()
		fpath := filepath.Join(tmp, "empty.jsonl")
		if err := os.WriteFile(fpath, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
		_, ok := readLearningJSONLData(fpath)
		if ok {
			t.Error("expected ok=false for empty file")
		}
	})

	t.Run("invalid JSON returns false", func(t *testing.T) {
		tmp := t.TempDir()
		fpath := filepath.Join(tmp, "bad.jsonl")
		if err := os.WriteFile(fpath, []byte("not json\n"), 0644); err != nil {
			t.Fatal(err)
		}
		_, ok := readLearningJSONLData(fpath)
		if ok {
			t.Error("expected ok=false for invalid JSON")
		}
	})

	t.Run("nonexistent file returns false", func(t *testing.T) {
		_, ok := readLearningJSONLData("/nonexistent/file.jsonl")
		if ok {
			t.Error("expected ok=false for nonexistent file")
		}
	})
}

// ---------------------------------------------------------------------------
// inject_sessions.go — parseJSONLSessionSummary
// ---------------------------------------------------------------------------

func TestInject_parseJSONLSessionSummary(t *testing.T) {
	t.Run("valid summary", func(t *testing.T) {
		tmp := t.TempDir()
		fpath := filepath.Join(tmp, "session.jsonl")
		content := `{"summary":"Fixed authentication bug in OAuth module"}`
		if err := os.WriteFile(fpath, []byte(content+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		summary, err := parseJSONLSessionSummary(fpath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(summary, "Fixed authentication") {
			t.Errorf("summary = %q", summary)
		}
	})

	t.Run("no summary field", func(t *testing.T) {
		tmp := t.TempDir()
		fpath := filepath.Join(tmp, "session.jsonl")
		content := `{"other":"value"}`
		if err := os.WriteFile(fpath, []byte(content+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		summary, err := parseJSONLSessionSummary(fpath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if summary != "" {
			t.Errorf("expected empty summary, got %q", summary)
		}
	})

	t.Run("invalid JSON returns empty", func(t *testing.T) {
		tmp := t.TempDir()
		fpath := filepath.Join(tmp, "session.jsonl")
		if err := os.WriteFile(fpath, []byte("not json\n"), 0644); err != nil {
			t.Fatal(err)
		}
		summary, err := parseJSONLSessionSummary(fpath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if summary != "" {
			t.Errorf("expected empty summary, got %q", summary)
		}
	})

	t.Run("long summary gets truncated", func(t *testing.T) {
		tmp := t.TempDir()
		fpath := filepath.Join(tmp, "session.jsonl")
		long := strings.Repeat("x", 200)
		content := `{"summary":"` + long + `"}`
		if err := os.WriteFile(fpath, []byte(content+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		summary, err := parseJSONLSessionSummary(fpath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(summary) > 153 { // 150 + "..."
			t.Errorf("summary too long: %d chars", len(summary))
		}
	})
}

// ---------------------------------------------------------------------------
// inject_sessions.go — parseMarkdownSessionSummary
// ---------------------------------------------------------------------------

func TestInject_parseMarkdownSessionSummary(t *testing.T) {
	t.Run("extracts first content line", func(t *testing.T) {
		tmp := t.TempDir()
		fpath := filepath.Join(tmp, "session.md")
		content := "# Session\n\nWorked on feature X.\nMore details."
		if err := os.WriteFile(fpath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		summary, err := parseMarkdownSessionSummary(fpath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(summary, "Worked on feature X") {
			t.Errorf("summary = %q", summary)
		}
	})

	t.Run("skips frontmatter delimiters", func(t *testing.T) {
		tmp := t.TempDir()
		fpath := filepath.Join(tmp, "session.md")
		content := "---\ntitle: session\n---\n# Title\n\nActual content."
		if err := os.WriteFile(fpath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		summary, err := parseMarkdownSessionSummary(fpath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if summary == "---" || summary == "" {
			t.Errorf("summary = %q, should skip frontmatter", summary)
		}
	})

	t.Run("empty file returns empty", func(t *testing.T) {
		tmp := t.TempDir()
		fpath := filepath.Join(tmp, "empty.md")
		if err := os.WriteFile(fpath, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
		summary, err := parseMarkdownSessionSummary(fpath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if summary != "" {
			t.Errorf("expected empty summary, got %q", summary)
		}
	})
}

// ---------------------------------------------------------------------------
// maturity.go — reportEvictionCandidates
// ---------------------------------------------------------------------------

func TestMaturity_reportEvictionCandidates(t *testing.T) {
	t.Run("no candidates", func(t *testing.T) {
		shouldArchive, err := reportEvictionCandidates([]string{"a.jsonl"}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if shouldArchive {
			t.Error("expected shouldArchive=false with no candidates")
		}
	})

	t.Run("with candidates in text mode", func(t *testing.T) {
		origOutput := output
		output = "table"
		defer func() { output = origOutput }()

		candidates := []evictionCandidate{
			{Name: "old.jsonl", Utility: 0.1, Confidence: 0.05, Maturity: "provisional", LastCited: "never"},
		}
		shouldArchive, err := reportEvictionCandidates([]string{"a.jsonl", "b.jsonl"}, candidates)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !shouldArchive {
			t.Error("expected shouldArchive=true with candidates")
		}
	})

	t.Run("json mode", func(t *testing.T) {
		origOutput := output
		output = "json"
		defer func() { output = origOutput }()

		candidates := []evictionCandidate{
			{Name: "old.jsonl", Utility: 0.1, Confidence: 0.05, Maturity: "provisional", LastCited: "never"},
		}
		shouldArchive, err := reportEvictionCandidates([]string{"a.jsonl"}, candidates)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if shouldArchive {
			t.Error("expected shouldArchive=false in json mode")
		}
	})
}

// ---------------------------------------------------------------------------
// maturity.go — collectEvictionCandidates
// ---------------------------------------------------------------------------

func TestMaturity_collectEvictionCandidates(t *testing.T) {
	tmp := t.TempDir()

	// Create eligible JSONL file
	eligible := filepath.Join(tmp, "eligible.jsonl")
	if err := os.WriteFile(eligible, []byte(`{"utility":0.1,"confidence":0.05,"maturity":"provisional"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create non-eligible JSONL file (utility too high)
	nonEligible := filepath.Join(tmp, "good.jsonl")
	if err := os.WriteFile(nonEligible, []byte(`{"utility":0.8,"confidence":0.9,"maturity":"established"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	files := []string{eligible, nonEligible}
	cutoff := time.Now().AddDate(0, 0, -90)
	lastCited := map[string]time.Time{}

	candidates := collectEvictionCandidates(tmp, files, lastCited, cutoff)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].Name != "eligible.jsonl" {
		t.Errorf("expected eligible.jsonl, got %q", candidates[0].Name)
	}
}

// ---------------------------------------------------------------------------
// maturity.go — archiveExpiredLearnings
// ---------------------------------------------------------------------------

func TestMaturity_archiveExpiredLearnings(t *testing.T) {
	t.Run("dry run does not move files", func(t *testing.T) {
		origDryRun := dryRun
		dryRun = true
		defer func() { dryRun = origDryRun }()

		tmp := t.TempDir()
		learningsDir := filepath.Join(tmp, ".agents", "learnings")
		if err := os.MkdirAll(learningsDir, 0755); err != nil {
			t.Fatal(err)
		}
		fpath := filepath.Join(learningsDir, "expired.md")
		if err := os.WriteFile(fpath, []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}

		err := archiveExpiredLearnings(tmp, learningsDir, []string{"expired.md"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// File should still exist
		if _, err := os.Stat(fpath); os.IsNotExist(err) {
			t.Error("file should not be moved in dry-run mode")
		}
	})

	t.Run("moves files to archive", func(t *testing.T) {
		origDryRun := dryRun
		dryRun = false
		defer func() { dryRun = origDryRun }()

		tmp := t.TempDir()
		learningsDir := filepath.Join(tmp, ".agents", "learnings")
		if err := os.MkdirAll(learningsDir, 0755); err != nil {
			t.Fatal(err)
		}
		fpath := filepath.Join(learningsDir, "expired.md")
		if err := os.WriteFile(fpath, []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}

		err := archiveExpiredLearnings(tmp, learningsDir, []string{"expired.md"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Source should be gone
		if _, err := os.Stat(fpath); !os.IsNotExist(err) {
			t.Error("source file should be moved")
		}
		// Archive should exist
		archivePath := filepath.Join(tmp, ".agents", "archive", "learnings", "expired.md")
		if _, err := os.Stat(archivePath); os.IsNotExist(err) {
			t.Error("archived file should exist")
		}
	})
}

// ---------------------------------------------------------------------------
// maturity.go — archiveEvictionCandidates
// ---------------------------------------------------------------------------

func TestMaturity_archiveEvictionCandidates(t *testing.T) {
	t.Run("dry run does not move files", func(t *testing.T) {
		origDryRun := dryRun
		dryRun = true
		defer func() { dryRun = origDryRun }()

		tmp := t.TempDir()
		candidates := []evictionCandidate{
			{Path: "/nonexistent/path.jsonl", Name: "path.jsonl"},
		}
		err := archiveEvictionCandidates(tmp, candidates)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("moves files to archive", func(t *testing.T) {
		origDryRun := dryRun
		dryRun = false
		defer func() { dryRun = origDryRun }()

		tmp := t.TempDir()
		learningsDir := filepath.Join(tmp, "learnings")
		if err := os.MkdirAll(learningsDir, 0755); err != nil {
			t.Fatal(err)
		}
		fpath := filepath.Join(learningsDir, "evict.jsonl")
		if err := os.WriteFile(fpath, []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}

		candidates := []evictionCandidate{
			{Path: fpath, Name: "evict.jsonl"},
		}
		err := archiveEvictionCandidates(tmp, candidates)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		archivePath := filepath.Join(tmp, ".agents", "archive", "learnings", "evict.jsonl")
		if _, err := os.Stat(archivePath); os.IsNotExist(err) {
			t.Error("archived file should exist")
		}
	})
}

// ---------------------------------------------------------------------------
// inject_learnings.go — parseFrontMatterLine
// ---------------------------------------------------------------------------

func TestInject_parseFrontMatterLine(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantSuper string
		wantUtil  float64
		wantHas   bool
	}{
		{"superseded_by colon", "superseded_by: L-new", "L-new", 0, false},
		{"superseded-by dash", "superseded-by: L-other", "L-other", 0, false},
		{"utility valid", "utility: 0.85", "", 0.85, true},
		{"utility zero not set", "utility: 0.0", "", 0, false},
		{"utility invalid", "utility: notanumber", "", 0, false},
		{"unrelated line", "title: My Title", "", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := frontMatter{}
			parseFrontMatterLine(tt.line, &fm)
			if fm.SupersededBy != tt.wantSuper {
				t.Errorf("SupersededBy = %q, want %q", fm.SupersededBy, tt.wantSuper)
			}
			if tt.wantHas {
				if !fm.HasUtility {
					t.Error("expected HasUtility=true")
				}
				if math.Abs(fm.Utility-tt.wantUtil) > 0.01 {
					t.Errorf("Utility = %f, want %f", fm.Utility, tt.wantUtil)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// inject.go — printInjectDryRun (smoke test for coverage)
// ---------------------------------------------------------------------------

func TestInject_printInjectDryRun(t *testing.T) {
	origMaxTokens := injectMaxTokens
	injectMaxTokens = 2000
	defer func() { injectMaxTokens = origMaxTokens }()

	out, _ := captureStdout(t, func() error {
		printInjectDryRun(&search.InjectOptions{MaxTokens: injectMaxTokens})
		printInjectDryRun(&search.InjectOptions{MaxTokens: injectMaxTokens, Query: "some query"})
		return nil
	})
	if !strings.Contains(out, "2000") {
		t.Errorf("expected max tokens in output, got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// maturity.go — displayMaturityDistribution (smoke test)
// ---------------------------------------------------------------------------

func TestMaturity_displayMaturityDistribution(t *testing.T) {
	dist := &ratchet.MaturityDistribution{
		Provisional: 3,
		Candidate:   2,
		Established: 1,
		AntiPattern: 0,
		Total:       6,
	}
	out, _ := captureStdout(t, func() error { displayMaturityDistribution(dist); return nil })
	for _, want := range []string{"3", "2", "1"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

// ---------------------------------------------------------------------------
// maturity.go — displayMaturityResult (smoke test)
// ---------------------------------------------------------------------------

func TestMaturity_displayMaturityResult(t *testing.T) {
	t.Run("without transition", func(t *testing.T) {
		r := &ratchet.MaturityTransitionResult{
			LearningID:   "L001",
			OldMaturity:  "provisional",
			Transitioned: false,
			Utility:      0.5,
			Confidence:   0.8,
			RewardCount:  3,
			HelpfulCount: 2,
			HarmfulCount: 1,
			Reason:       "no change needed",
		}
		displayMaturityResult(r, false)
	})

	t.Run("with transition applied", func(t *testing.T) {
		r := &ratchet.MaturityTransitionResult{
			LearningID:   "L002",
			OldMaturity:  "provisional",
			NewMaturity:  "candidate",
			Transitioned: true,
			Utility:      0.8,
			Confidence:   0.9,
			RewardCount:  5,
			HelpfulCount: 4,
			HarmfulCount: 0,
			Reason:       "positive feedback",
		}
		displayMaturityResult(r, true)
	})

	t.Run("with transition not applied", func(t *testing.T) {
		r := &ratchet.MaturityTransitionResult{
			LearningID:   "L003",
			OldMaturity:  "candidate",
			NewMaturity:  "established",
			Transitioned: true,
			Utility:      0.9,
			Confidence:   0.95,
			RewardCount:  10,
			HelpfulCount: 8,
			HarmfulCount: 1,
			Reason:       "proven value",
		}
		displayMaturityResult(r, false)
	})
}
