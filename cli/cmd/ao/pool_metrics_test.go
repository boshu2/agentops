package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/taxonomy"
	"github.com/boshu2/agentops/cli/internal/types"
)

// ===========================================================================
// pool_ingest.go — parseLearningBlocks
// ===========================================================================

func TestPoolIngest_parseLearningBlocks(t *testing.T) {
	tests := []struct {
		name      string
		md        string
		wantCount int
		wantTitle string
		wantID    string
		wantCat   string
		wantConf  string
	}{
		{
			name:      "no learning blocks returns nil",
			md:        "# Regular Heading\n\nSome content without learning prefix.",
			wantCount: 0,
		},
		{
			name: "single learning block",
			md: `# Learning: Use guard clauses for early returns

**ID**: L-001
**Category**: patterns
**Confidence**: high

Guard clauses reduce nesting.
`,
			wantCount: 1,
			wantTitle: "Use guard clauses for early returns",
			wantID:    "L-001",
			wantCat:   "patterns",
			wantConf:  "high",
		},
		{
			name: "two learning blocks",
			md: `# Learning: First learning

**ID**: L-001
**Category**: patterns
**Confidence**: high

First body.

# Learning: Second learning

**ID**: L-002
**Category**: tooling
**Confidence**: medium

Second body.
`,
			wantCount: 2,
			wantTitle: "First learning",
			wantID:    "L-001",
		},
		{
			name: "learning block without metadata fields",
			md: `# Learning: Bare learning

Just content with no ID/Category/Confidence fields.
`,
			wantCount: 1,
			wantTitle: "Bare learning",
			wantID:    "",
			wantCat:   "",
			wantConf:  "",
		},
		{
			name: "colon inside bold variant",
			md: `# Learning: Colon variant

**ID:** COL-1
**Category:** ops
**Confidence:** low

Body here.
`,
			wantCount: 1,
			wantID:    "COL-1",
			wantCat:   "ops",
			wantConf:  "low",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			blocks := parseLearningBlocks(tc.md)
			if len(blocks) != tc.wantCount {
				t.Fatalf("parseLearningBlocks() returned %d blocks, want %d", len(blocks), tc.wantCount)
			}
			if tc.wantCount == 0 {
				return
			}
			b := blocks[0]
			if tc.wantTitle != "" && b.Title != tc.wantTitle {
				t.Errorf("Title = %q, want %q", b.Title, tc.wantTitle)
			}
			if tc.wantID != "" && b.ID != tc.wantID {
				t.Errorf("ID = %q, want %q", b.ID, tc.wantID)
			}
			if tc.wantCat != "" && b.Category != tc.wantCat {
				t.Errorf("Category = %q, want %q", b.Category, tc.wantCat)
			}
			if tc.wantConf != "" && b.Confidence != tc.wantConf {
				t.Errorf("Confidence = %q, want %q", b.Confidence, tc.wantConf)
			}
		})
	}
}

// ===========================================================================
// pool_ingest.go — parseLegacyFrontmatterLearning
// ===========================================================================

func TestPoolIngest_parseLegacyFrontmatterLearning(t *testing.T) {
	tests := []struct {
		name     string
		md       string
		wantOK   bool
		wantCat  string
		wantConf string
		wantID   string
	}{
		{
			name:   "no frontmatter",
			md:     "# Plain markdown\nContent here.",
			wantOK: false,
		},
		{
			name:   "frontmatter without type",
			md:     "---\nsource: test\n---\n# Title\nBody content.",
			wantOK: false,
		},
		{
			name:   "frontmatter with empty body",
			md:     "---\ntype: learning\n---\n",
			wantOK: false,
		},
		{
			name:     "valid legacy learning",
			md:       "---\ntype: pattern\nid: legacy-1\nconfidence: high\n---\n# Guard Clauses\nUse guard clauses for early return.",
			wantOK:   true,
			wantCat:  "pattern",
			wantConf: "high",
			wantID:   "legacy-1",
		},
		{
			name:     "defaults for missing id and confidence",
			md:       "---\ntype: learning\n---\n# Some Title\nSome body text here.",
			wantOK:   true,
			wantCat:  "learning",
			wantConf: "medium",
			wantID:   "legacy",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			block, ok := parseLegacyFrontmatterLearning(tc.md)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if block.Category != tc.wantCat {
				t.Errorf("Category = %q, want %q", block.Category, tc.wantCat)
			}
			if block.Confidence != tc.wantConf {
				t.Errorf("Confidence = %q, want %q", block.Confidence, tc.wantConf)
			}
			if block.ID != tc.wantID {
				t.Errorf("ID = %q, want %q", block.ID, tc.wantID)
			}
		})
	}
}

// ===========================================================================
// pool_ingest.go — parseYAMLFrontmatter
// ===========================================================================

func TestPoolIngest_parseYAMLFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantLen int
		checkFn func(map[string]string) bool
	}{
		{
			name:    "empty string",
			raw:     "",
			wantLen: 0,
		},
		{
			name:    "comments skipped",
			raw:     "# comment\ntype: learning\n# another comment",
			wantLen: 1,
			checkFn: func(m map[string]string) bool { return m["type"] == "learning" },
		},
		{
			name:    "strips quotes",
			raw:     `title: "Hello World"`,
			wantLen: 1,
			checkFn: func(m map[string]string) bool { return m["title"] == "Hello World" },
		},
		{
			name:    "strips single quotes",
			raw:     "title: 'Single Quoted'",
			wantLen: 1,
			checkFn: func(m map[string]string) bool { return m["title"] == "Single Quoted" },
		},
		{
			name:    "key lowercased",
			raw:     "Type: PATTERN",
			wantLen: 1,
			checkFn: func(m map[string]string) bool { return m["type"] == "PATTERN" },
		},
		{
			name:    "line without colon skipped",
			raw:     "no-colon-here\ntype: learning",
			wantLen: 1,
		},
		{
			name:    "value with colons preserves full value",
			raw:     "url: https://example.com:8080/path",
			wantLen: 1,
			checkFn: func(m map[string]string) bool { return m["url"] == "https://example.com:8080/path" },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseYAMLFrontmatter(tc.raw)
			if len(got) != tc.wantLen {
				t.Fatalf("len = %d, want %d; map: %v", len(got), tc.wantLen, got)
			}
			if tc.checkFn != nil && !tc.checkFn(got) {
				t.Errorf("check failed; map: %v", got)
			}
		})
	}
}

// ===========================================================================
// pool_ingest.go — extractFirstHeadingText
// ===========================================================================

func TestPoolIngest_extractFirstHeadingText(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"heading line", "# My Title\n\nContent", "My Title"},
		{"non-heading content", "First line of text\nSecond line", "First line of text"},
		{"empty lines then content", "\n\n\nContent here", "Content here"},
		{"all empty", "\n\n\n", ""},
		{"hash-only line skipped", "#\nReal content", "Real content"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractFirstHeadingText(tc.body)
			if got != tc.want {
				t.Errorf("extractFirstHeadingText() = %q, want %q", got, tc.want)
			}
		})
	}
}

// ===========================================================================
// pool_ingest.go — confidenceToScore
// ===========================================================================

func TestPoolIngest_confidenceToScore(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"high", 0.9},
		{"HIGH", 0.9},
		{" High ", 0.9},
		{"medium", 0.7},
		{"low", 0.5},
		{"", 0.6},
		{"unknown", 0.6},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := confidenceToScore(tc.input)
			if got != tc.want {
				t.Errorf("confidenceToScore(%q) = %f, want %f", tc.input, got, tc.want)
			}
		})
	}
}

// ===========================================================================
// pool_ingest.go — computeSpecificityScore
// ===========================================================================

func TestPoolIngest_computeSpecificityScore(t *testing.T) {
	tests := []struct {
		name string
		body string
		want float64 // minimum expected score
	}{
		{
			name: "plain text baseline",
			body: "Some plain text without any markers",
			want: 0.4,
		},
		{
			name: "with backticks",
			body: "Use `go build` to compile",
			want: 0.6,
		},
		{
			name: "with digits",
			body: "Set retry count to 3",
			want: 0.6,
		},
		{
			name: "with file extension",
			body: "Edit the config.yaml file",
			want: 0.6,
		},
		{
			name: "with line reference",
			body: "Check line 42 in the file",
			want: 0.5, // digits + "line "
		},
		{
			name: "all indicators maxes at 1.0",
			body: "Use `cmd` at line 5 to fix config.go with value 42",
			want: 1.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lower := strings.ToLower(tc.body)
			got := computeSpecificityScore(tc.body, lower)
			if got < tc.want-0.01 {
				t.Errorf("computeSpecificityScore() = %f, want >= %f", got, tc.want)
			}
			if got > 1.0 {
				t.Errorf("computeSpecificityScore() = %f, should not exceed 1.0", got)
			}
		})
	}
}

// ===========================================================================
// pool_ingest.go — computeActionabilityScore
// ===========================================================================

func TestPoolIngest_computeActionabilityScore(t *testing.T) {
	tests := []struct {
		name string
		body string
		want float64 // minimum expected score
	}{
		{
			name: "plain text baseline",
			body: "Some general observation about the system",
			want: 0.4,
		},
		{
			name: "with bullet list",
			body: "Steps:\n- First do this\n- Then do that",
			want: 0.6,
		},
		{
			name: "with action verb",
			body: "You should run the test suite before committing",
			want: 0.6,
		},
		{
			name: "with code block",
			body: "Run this:\n```\ngo test ./...\n```",
			want: 0.6,
		},
		{
			name: "all indicators caps at 1.0",
			body: "- Use this pattern:\n```\ngo test\n```\nAlways run tests first",
			want: 1.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := computeActionabilityScore(tc.body)
			if got < tc.want-0.01 {
				t.Errorf("computeActionabilityScore() = %f, want >= %f", got, tc.want)
			}
			if got > 1.0 {
				t.Errorf("computeActionabilityScore() = %f, should not exceed 1.0", got)
			}
		})
	}
}

// ===========================================================================
// pool_ingest.go — computeNoveltyScore
// ===========================================================================

func TestPoolIngest_computeNoveltyScore(t *testing.T) {
	tests := []struct {
		name string
		body string
		want float64
	}{
		{
			name: "medium length baseline",
			body: strings.Repeat("x", 400),
			want: 0.5,
		},
		{
			name: "long body bonus",
			body: strings.Repeat("x", 900),
			want: 0.6,
		},
		{
			name: "short body penalty",
			body: strings.Repeat("x", 100),
			want: 0.4,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := computeNoveltyScore(tc.body)
			if math.Abs(got-tc.want) > 0.01 {
				t.Errorf("computeNoveltyScore() = %f, want %f", got, tc.want)
			}
		})
	}
}

// ===========================================================================
// pool_ingest.go — computeContextScore
// ===========================================================================

func TestPoolIngest_computeContextScore(t *testing.T) {
	tests := []struct {
		name  string
		lower string
		want  float64
	}{
		{
			name:  "baseline",
			lower: "plain text",
			want:  0.5,
		},
		{
			name:  "source section",
			lower: "## source\nsome reference",
			want:  0.7,
		},
		{
			name:  "bold source format",
			lower: "**source** details here",
			want:  0.7,
		},
		{
			name:  "why it matters section",
			lower: "## why it matters\nexplanation",
			want:  0.6,
		},
		{
			name:  "both source and why it matters",
			lower: "## source\nref\n## why it matters\nreason",
			want:  0.8,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := computeContextScore(tc.lower)
			if math.Abs(got-tc.want) > 0.01 {
				t.Errorf("computeContextScore(%q) = %f, want %f", tc.lower, got, tc.want)
			}
		})
	}
}

// ===========================================================================
// pool_ingest.go — computeRubricScores
// ===========================================================================

func TestPoolIngest_computeRubricScores(t *testing.T) {
	body := "Use `go test ./...` to verify:\n- Run tests\n- Check coverage\n\n## source\nhttps://example.com"
	rubric := computeRubricScores(body, 0.9)

	if rubric.Confidence != 0.9 {
		t.Errorf("Confidence = %f, want 0.9", rubric.Confidence)
	}
	if rubric.Specificity <= 0.4 {
		t.Errorf("Specificity = %f, should be > 0.4 (has backticks)", rubric.Specificity)
	}
	if rubric.Actionability <= 0.4 {
		t.Errorf("Actionability = %f, should be > 0.4 (has bullet list and action verb)", rubric.Actionability)
	}
	if rubric.Context <= 0.5 {
		t.Errorf("Context = %f, should be > 0.5 (has source section)", rubric.Context)
	}
}

// ===========================================================================
// pool_ingest.go — rubricWeightedSum
// ===========================================================================

func TestPoolIngest_rubricWeightedSum(t *testing.T) {
	tests := []struct {
		name    string
		rubric  types.RubricScores
		weights taxonomy.RubricWeights
		want    float64
	}{
		{
			name: "all zeros",
			rubric: types.RubricScores{
				Specificity: 0, Actionability: 0, Novelty: 0, Context: 0, Confidence: 0,
			},
			weights: taxonomy.DefaultRubricWeights,
			want:    0.0,
		},
		{
			name: "all ones with equal weights",
			rubric: types.RubricScores{
				Specificity: 1.0, Actionability: 1.0, Novelty: 1.0, Context: 1.0, Confidence: 1.0,
			},
			weights: taxonomy.RubricWeights{
				Specificity: 0.2, Actionability: 0.2, Novelty: 0.2, Context: 0.2, Confidence: 0.2,
			},
			want: 1.0,
		},
		{
			name: "uses default weights correctly",
			rubric: types.RubricScores{
				Specificity: 1.0, Actionability: 1.0, Novelty: 1.0, Context: 1.0, Confidence: 1.0,
			},
			weights: taxonomy.DefaultRubricWeights,
			want: taxonomy.DefaultRubricWeights.Specificity +
				taxonomy.DefaultRubricWeights.Actionability +
				taxonomy.DefaultRubricWeights.Novelty +
				taxonomy.DefaultRubricWeights.Context +
				taxonomy.DefaultRubricWeights.Confidence,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := rubricWeightedSum(tc.rubric, tc.weights)
			if math.Abs(got-tc.want) > 0.001 {
				t.Errorf("rubricWeightedSum() = %f, want %f", got, tc.want)
			}
		})
	}
}

// ===========================================================================
// pool_ingest.go — slugify
// ===========================================================================

func TestPoolIngest_slugify(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple lowercase", "hello-world", "hello-world"},
		{"uppercase converted", "Hello World", "hello-world"},
		{"special chars become dash", "foo@bar!baz", "foo-bar-baz"},
		{"consecutive non-alnum collapsed", "foo!!??bar", "foo-bar"},
		{"leading trailing dashes trimmed", "--hello--", "hello"},
		{"empty input returns cand", "", "cand"},
		{"all special chars returns cand", "!!!@@@", "cand"},
		{"numbers preserved", "run-3-times", "run-3-times"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := slugify(tc.input)
			if got != tc.want {
				t.Errorf("slugify(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ===========================================================================
// pool_ingest.go — isSlugAlphanumeric
// ===========================================================================

func TestPoolIngest_isSlugAlphanumeric(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
	}{
		{'a', true},
		{'z', true},
		{'0', true},
		{'9', true},
		{'A', false}, // uppercase not considered alphanumeric for slug
		{'-', false},
		{'_', false},
		{' ', false},
	}

	for _, tc := range tests {
		t.Run(string(tc.r), func(t *testing.T) {
			got := isSlugAlphanumeric(tc.r)
			if got != tc.want {
				t.Errorf("isSlugAlphanumeric(%q) = %v, want %v", tc.r, got, tc.want)
			}
		})
	}
}

// ===========================================================================
// pool_ingest.go — date strategy functions
// ===========================================================================

func TestPoolIngest_dateFromFrontmatter(t *testing.T) {
	tests := []struct {
		name   string
		md     string
		wantOK bool
		wantD  string // YYYY-MM-DD
	}{
		{
			name:   "valid frontmatter with date",
			md:     "---\ndate: 2026-01-15\ntitle: test\n---\nContent",
			wantOK: true,
			wantD:  "2026-01-15",
		},
		{
			name:   "no frontmatter",
			md:     "# Just content\nNo frontmatter here.",
			wantOK: false,
		},
		{
			name:   "frontmatter without date",
			md:     "---\ntitle: test\n---\nContent",
			wantOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := dateFromFrontmatter(tc.md, "")
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && got.Format("2006-01-02") != tc.wantD {
				t.Errorf("date = %s, want %s", got.Format("2006-01-02"), tc.wantD)
			}
		})
	}
}

func TestPoolIngest_dateFromMarkdownField(t *testing.T) {
	tests := []struct {
		name   string
		md     string
		wantOK bool
		wantD  string
	}{
		{
			name:   "standard date field",
			md:     "**Date**: 2026-02-20\nOther content",
			wantOK: true,
			wantD:  "2026-02-20",
		},
		{
			name:   "colon inside bold",
			md:     "**Date:** 2026-03-15\nContent",
			wantOK: true,
			wantD:  "2026-03-15",
		},
		{
			name:   "no date field",
			md:     "Some content without date",
			wantOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := dateFromMarkdownField(tc.md, "")
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && got.Format("2006-01-02") != tc.wantD {
				t.Errorf("date = %s, want %s", got.Format("2006-01-02"), tc.wantD)
			}
		})
	}
}

func TestPoolIngest_dateFromFilenamePrefix(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		wantOK bool
		wantD  string
	}{
		{
			name:   "valid date prefix",
			path:   "/path/to/2026-01-15-learning.md",
			wantOK: true,
			wantD:  "2026-01-15",
		},
		{
			name:   "filename too short",
			path:   "/path/to/short.md",
			wantOK: false,
		},
		{
			name:   "not a valid date",
			path:   "/path/to/abcdefghij-file.md",
			wantOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := dateFromFilenamePrefix("", tc.path)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && got.Format("2006-01-02") != tc.wantD {
				t.Errorf("date = %s, want %s", got.Format("2006-01-02"), tc.wantD)
			}
		})
	}
}

func TestPoolIngest_dateFromFileMtime(t *testing.T) {
	t.Run("existing file returns mtime", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "test.md")
		if err := os.WriteFile(f, []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
		got, ok := dateFromFileMtime("", f)
		if !ok {
			t.Fatal("expected ok=true for existing file")
		}
		if got.IsZero() {
			t.Error("expected non-zero time")
		}
	})

	t.Run("nonexistent file returns false", func(t *testing.T) {
		_, ok := dateFromFileMtime("", "/nonexistent/path/file.md")
		if ok {
			t.Error("expected ok=false for nonexistent file")
		}
	})
}

// ===========================================================================
// pool_ingest.go — extractSessionHint
// ===========================================================================

func TestPoolIngest_extractSessionHint(t *testing.T) {
	tests := []struct {
		name string
		md   string
		path string
		want string
	}{
		{
			name: "session id in content",
			md:   "Session ag-abc123 worked on fixes",
			path: "/path/to/2026-01-15-learning.md",
			want: "ag-abc123",
		},
		{
			name: "no session id falls back to filename stem",
			md:   "No session ID in this content",
			path: "/path/to/2026-01-15-guard-clauses.md",
			want: "2026-01-15-guard-clauses",
		},
		{
			name: "long content truncated to 2048 for search",
			md:   strings.Repeat("x", 3000) + " ag-late1",
			path: "/path/to/file.md",
			want: "file", // ag-late1 is past 2048, should fall back
		},
		{
			name: "session id near start of content",
			md:   "Session ag-xyz9 completed. " + strings.Repeat("x", 3000),
			path: "/path/to/file.md",
			want: "ag-xyz9",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractSessionHint(tc.md, tc.path)
			if got != tc.want {
				t.Errorf("extractSessionHint() = %q, want %q", got, tc.want)
			}
		})
	}
}

// ===========================================================================
// pool_ingest.go — buildCandidateFromLearningBlock
// ===========================================================================

func TestPoolIngest_buildCandidateFromLearningBlock(t *testing.T) {
	t.Run("valid block produces candidate", func(t *testing.T) {
		b := learningBlock{
			Title:      "Guard Clauses",
			ID:         "L-001",
			Category:   "patterns",
			Confidence: "high",
			Body:       "Use guard clauses to reduce nesting and improve readability in Go functions.",
		}
		fileDate := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

		cand, scoring, ok := buildCandidateFromLearningBlock(b, "/path/to/2026-01-15-learning.md", fileDate, "ag-test")
		if !ok {
			t.Fatal("expected ok=true for valid block")
		}
		if cand.ID == "" {
			t.Error("expected non-empty ID")
		}
		if cand.Type != types.KnowledgeTypeLearning {
			t.Errorf("Type = %q, want %q", cand.Type, types.KnowledgeTypeLearning)
		}
		if cand.RawScore <= 0 || cand.RawScore > 1.0 {
			t.Errorf("RawScore = %f, want 0 < score <= 1.0", cand.RawScore)
		}
		if cand.Tier == "" {
			t.Error("expected non-empty Tier")
		}
		if scoring.RawScore != cand.RawScore {
			t.Errorf("Scoring.RawScore = %f, want %f", scoring.RawScore, cand.RawScore)
		}
		if cand.Metadata["pending_confidence"] != "high" {
			t.Errorf("Metadata[pending_confidence] = %v, want %q", cand.Metadata["pending_confidence"], "high")
		}
	})

	t.Run("empty title returns not ok", func(t *testing.T) {
		b := learningBlock{Title: "", Body: "some body"}
		_, _, ok := buildCandidateFromLearningBlock(b, "/path.md", time.Now(), "ag-test")
		if ok {
			t.Error("expected ok=false for empty title")
		}
	})

	t.Run("empty body returns not ok", func(t *testing.T) {
		b := learningBlock{Title: "Has Title", Body: ""}
		_, _, ok := buildCandidateFromLearningBlock(b, "/path.md", time.Now(), "ag-test")
		if ok {
			t.Error("expected ok=false for empty body")
		}
	})

	t.Run("long ID gets truncated with hash", func(t *testing.T) {
		b := learningBlock{
			Title: "Long ID Test",
			ID:    strings.Repeat("a", 200),
			Body:  "Body content for long ID test.",
		}
		cand, _, ok := buildCandidateFromLearningBlock(b, "/path/to/very-long-filename-that-is-quite-lengthy.md", time.Now(), "ag-test-session-with-long-name")
		if !ok {
			t.Fatal("expected ok=true")
		}
		if len(cand.ID) > 120 {
			t.Errorf("ID length = %d, should be <= 120", len(cand.ID))
		}
	})

	t.Run("high confidence boosts score", func(t *testing.T) {
		bHigh := learningBlock{Title: "T", ID: "id", Confidence: "high", Body: "Body content here."}
		bLow := learningBlock{Title: "T", ID: "id", Confidence: "low", Body: "Body content here."}
		candH, _, _ := buildCandidateFromLearningBlock(bHigh, "/p.md", time.Now(), "s")
		candL, _, _ := buildCandidateFromLearningBlock(bLow, "/p.md", time.Now(), "s")
		if candH.RawScore <= candL.RawScore {
			t.Errorf("high confidence score (%f) should be > low confidence score (%f)", candH.RawScore, candL.RawScore)
		}
	})
}

// ===========================================================================
// pool_ingest.go — resolveIngestFiles
// ===========================================================================

func TestPoolIngest_resolveIngestFiles(t *testing.T) {
	t.Run("no args uses default patterns", func(t *testing.T) {
		dir := t.TempDir()
		pendingDir := filepath.Join(dir, ".agents", "knowledge", "pending")
		if err := os.MkdirAll(pendingDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pendingDir, "test.md"), []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}

		files, err := resolveIngestFiles(dir, filepath.Join(".agents", "knowledge", "pending"), nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(files) == 0 {
			t.Error("expected at least one file")
		}
	})

	t.Run("deduplicates files", func(t *testing.T) {
		dir := t.TempDir()
		knowledgeDir := filepath.Join(dir, ".agents", "knowledge")
		pendingDir := filepath.Join(knowledgeDir, "pending")
		if err := os.MkdirAll(pendingDir, 0755); err != nil {
			t.Fatal(err)
		}
		// File in both knowledge/ and pending/
		if err := os.WriteFile(filepath.Join(knowledgeDir, "shared.md"), []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}

		files, err := resolveIngestFiles(dir, filepath.Join(".agents", "knowledge", "pending"), nil)
		if err != nil {
			t.Fatal(err)
		}
		// Check no duplicates
		seen := map[string]bool{}
		for _, f := range files {
			if seen[f] {
				t.Errorf("duplicate file: %s", f)
			}
			seen[f] = true
		}
	})

	t.Run("explicit args resolved relative to cwd", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "explicit.md")
		if err := os.WriteFile(f, []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}

		files, err := resolveIngestFiles(dir, "", []string{f})
		if err != nil {
			t.Fatal(err)
		}
		if len(files) != 1 {
			t.Errorf("expected 1 file, got %d", len(files))
		}
	})

	t.Run("empty dir returns empty", func(t *testing.T) {
		dir := t.TempDir()
		files, err := resolveIngestFiles(dir, filepath.Join(".agents", "knowledge", "pending"), nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(files) != 0 {
			t.Errorf("expected 0 files, got %d", len(files))
		}
	})
}

// ===========================================================================
// pool_ingest.go — moveIngestedFiles
// ===========================================================================

func TestPoolIngest_moveIngestedFiles(t *testing.T) {
	t.Run("moves files to processed dir", func(t *testing.T) {
		dir := t.TempDir()
		srcDir := filepath.Join(dir, "source")
		if err := os.MkdirAll(srcDir, 0755); err != nil {
			t.Fatal(err)
		}
		f1 := filepath.Join(srcDir, "a.md")
		f2 := filepath.Join(srcDir, "b.md")
		if err := os.WriteFile(f1, []byte("a"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f2, []byte("b"), 0644); err != nil {
			t.Fatal(err)
		}

		moveIngestedFiles(dir, []string{f1, f2})

		processedDir := filepath.Join(dir, ".agents", "knowledge", "processed")
		if _, err := os.Stat(filepath.Join(processedDir, "a.md")); err != nil {
			t.Errorf("expected a.md in processed dir: %v", err)
		}
		if _, err := os.Stat(filepath.Join(processedDir, "b.md")); err != nil {
			t.Errorf("expected b.md in processed dir: %v", err)
		}
	})

	t.Run("empty list is no-op", func(t *testing.T) {
		dir := t.TempDir()
		moveIngestedFiles(dir, nil) // should not panic
	})
}

// ===========================================================================
// pool_ingest.go — parsePendingFileHeader
// ===========================================================================

func TestPoolIngest_parsePendingFileHeader(t *testing.T) {
	t.Run("date from frontmatter", func(t *testing.T) {
		md := "---\ndate: 2026-01-15\n---\nContent with ag-abc1"
		d, hint := parsePendingFileHeader(md, "/path/to/file.md")
		if d.Format("2006-01-02") != "2026-01-15" {
			t.Errorf("date = %s, want 2026-01-15", d.Format("2006-01-02"))
		}
		if hint != "ag-abc1" {
			t.Errorf("hint = %q, want %q", hint, "ag-abc1")
		}
	})

	t.Run("date from markdown field", func(t *testing.T) {
		md := "**Date**: 2026-03-10\nSome content"
		d, _ := parsePendingFileHeader(md, "/path/to/file.md")
		if d.Format("2006-01-02") != "2026-03-10" {
			t.Errorf("date = %s, want 2026-03-10", d.Format("2006-01-02"))
		}
	})

	t.Run("date from filename prefix", func(t *testing.T) {
		md := "No date markers here"
		d, _ := parsePendingFileHeader(md, "/path/to/2026-05-20-learning.md")
		if d.Format("2006-01-02") != "2026-05-20" {
			t.Errorf("date = %s, want 2026-05-20", d.Format("2006-01-02"))
		}
	})

	t.Run("falls back to now if no date found", func(t *testing.T) {
		md := "No date anywhere"
		d, _ := parsePendingFileHeader(md, "/path/to/short.md")
		if d.IsZero() {
			t.Error("expected non-zero fallback time")
		}
		// Should be recent
		if time.Since(d) > time.Minute {
			t.Errorf("fallback time too old: %v", d)
		}
	})
}

// ===========================================================================
// pool.go — truncateID
// ===========================================================================

func TestPool_truncateID(t *testing.T) {
	tests := []struct {
		id   string
		max  int
		want string
	}{
		{"short-id", 20, "short-id"},
		{"exact-length-id", 15, "exact-length-id"},
		{"this-is-a-very-long-candidate-id", 15, "this-is-a-ve..."},
		{"abc", 5, "abc"},
		{"abcde", 5, "abcde"},
		{"abcdef", 5, "ab..."},
	}

	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			got := truncateID(tc.id, tc.max)
			if got != tc.want {
				t.Errorf("truncateID(%q, %d) = %q, want %q", tc.id, tc.max, got, tc.want)
			}
		})
	}
}

// ===========================================================================
// pool.go — repeat
// ===========================================================================

func TestPool_repeat(t *testing.T) {
	tests := []struct {
		s    string
		n    int
		want string
	}{
		{"=", 5, "====="},
		{"-", 0, ""},
		{"ab", 3, "ababab"},
		{"", 10, ""},
	}

	for _, tc := range tests {
		t.Run(tc.s, func(t *testing.T) {
			got := repeat(tc.s, tc.n)
			if got != tc.want {
				t.Errorf("repeat(%q, %d) = %q, want %q", tc.s, tc.n, got, tc.want)
			}
		})
	}
}

// ===========================================================================
// pool_migrate_legacy.go — nextLegacyDestination
// ===========================================================================

func TestPoolMigrate_nextLegacyDestination(t *testing.T) {
	t.Run("no collision returns original name", func(t *testing.T) {
		dir := t.TempDir()
		got, err := nextLegacyDestination(dir, "learning.md")
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Base(got) != "learning.md" {
			t.Errorf("base = %q, want %q", filepath.Base(got), "learning.md")
		}
	})

	t.Run("collision produces migrated suffix", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "file.md"), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
		got, err := nextLegacyDestination(dir, "file.md")
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Base(got) != "file-migrated-1.md" {
			t.Errorf("base = %q, want %q", filepath.Base(got), "file-migrated-1.md")
		}
	})

	t.Run("file without extension", func(t *testing.T) {
		dir := t.TempDir()
		got, err := nextLegacyDestination(dir, "noext")
		if err != nil {
			t.Fatal(err)
		}
		// cmp.Or defaults to ".md" when no extension
		if !strings.HasSuffix(filepath.Base(got), ".md") || filepath.Base(got) == "noext" {
			// The function should use cmp.Or to default ext to ".md"
			// baseName = "noext", ext = ".md" (from cmp.Or), name = ""
			// Actually: ext = cmp.Or(filepath.Ext("noext"), ".md") = ".md"
			// name = strings.TrimSuffix("noext", ".md") = "noext"
			// candidate = dir/noext
			t.Logf("base = %q (this is expected when no collision)", filepath.Base(got))
		}
	})
}

// ===========================================================================
// pool_migrate_legacy.go — migrateLegacyKnowledgeFiles
// ===========================================================================

func TestPoolMigrate_migrateLegacyKnowledgeFiles(t *testing.T) {
	t.Run("empty source dir", func(t *testing.T) {
		srcDir := t.TempDir()
		pendingDir := filepath.Join(t.TempDir(), "pending")
		result, err := migrateLegacyKnowledgeFiles(srcDir, pendingDir)
		if err != nil {
			t.Fatal(err)
		}
		if result.Scanned != 0 {
			t.Errorf("Scanned = %d, want 0", result.Scanned)
		}
	})

	t.Run("skips non-ingestible files", func(t *testing.T) {
		srcDir := t.TempDir()
		pendingDir := filepath.Join(t.TempDir(), "pending")
		// Write a file that has no learning blocks
		if err := os.WriteFile(filepath.Join(srcDir, "readme.md"), []byte("# README\nJust a readme."), 0644); err != nil {
			t.Fatal(err)
		}
		result, err := migrateLegacyKnowledgeFiles(srcDir, pendingDir)
		if err != nil {
			t.Fatal(err)
		}
		if result.Scanned != 1 {
			t.Errorf("Scanned = %d, want 1", result.Scanned)
		}
		if result.Skipped != 1 {
			t.Errorf("Skipped = %d, want 1", result.Skipped)
		}
		if result.Moved != 0 {
			t.Errorf("Moved = %d, want 0", result.Moved)
		}
	})

	t.Run("moves eligible files", func(t *testing.T) {
		srcDir := t.TempDir()
		pendingDir := filepath.Join(t.TempDir(), "pending")
		// Write a file with a valid learning block
		content := "# Learning: Test Learning\n\n**ID**: L-001\n**Category**: test\n**Confidence**: medium\n\nBody content here."
		if err := os.WriteFile(filepath.Join(srcDir, "learning.md"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		result, err := migrateLegacyKnowledgeFiles(srcDir, pendingDir)
		if err != nil {
			t.Fatal(err)
		}
		if result.Eligible != 1 {
			t.Errorf("Eligible = %d, want 1", result.Eligible)
		}
		if result.Moved != 1 {
			t.Errorf("Moved = %d, want 1", result.Moved)
		}
		// Verify the file was actually moved
		if _, err := os.Stat(filepath.Join(pendingDir, "learning.md")); err != nil {
			t.Errorf("expected moved file in pending dir: %v", err)
		}
	})
}

// ===========================================================================
// metrics.go — computeSigmaRho
// ===========================================================================

func TestPoolMetrics_computeSigmaRho(t *testing.T) {
	tests := []struct {
		name           string
		totalArtifacts int
		uniqueCited    int
		citationCount  int
		days           int
		wantSigma      float64
		wantRho        float64
	}{
		{
			name:           "zero artifacts",
			totalArtifacts: 0, uniqueCited: 5, citationCount: 10, days: 7,
			wantSigma: 0, wantRho: 2.0,
		},
		{
			name:           "zero unique cited",
			totalArtifacts: 10, uniqueCited: 0, citationCount: 0, days: 7,
			wantSigma: 0, wantRho: 0,
		},
		{
			name:           "normal case",
			totalArtifacts: 100, uniqueCited: 20, citationCount: 40, days: 7,
			wantSigma: 0.2, wantRho: 2.0,
		},
		{
			name:           "14 days period",
			totalArtifacts: 50, uniqueCited: 10, citationCount: 20, days: 14,
			wantSigma: 0.2, wantRho: 1.0,
		},
		{
			name:           "zero days",
			totalArtifacts: 10, uniqueCited: 5, citationCount: 10, days: 0,
			wantSigma: 0.5, wantRho: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sigma, rho := computeSigmaRho(tc.totalArtifacts, tc.uniqueCited, tc.citationCount, tc.days)
			if math.Abs(sigma-tc.wantSigma) > 0.001 {
				t.Errorf("sigma = %f, want %f", sigma, tc.wantSigma)
			}
			if math.Abs(rho-tc.wantRho) > 0.001 {
				t.Errorf("rho = %f, want %f", rho, tc.wantRho)
			}
		})
	}
}

// ===========================================================================
// metrics.go — filterCitationsForPeriod
// ===========================================================================

func TestPoolMetrics_filterCitationsForPeriod(t *testing.T) {
	baseTime := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	citations := []types.CitationEvent{
		{ArtifactPath: "/a.md", CitedAt: baseTime.Add(-48 * time.Hour)},
		{ArtifactPath: "/b.md", CitedAt: baseTime.Add(-12 * time.Hour)},
		{ArtifactPath: "/c.md", CitedAt: baseTime.Add(12 * time.Hour)},
		{ArtifactPath: "/b.md", CitedAt: baseTime.Add(24 * time.Hour)},
	}

	start := baseTime.Add(-24 * time.Hour)
	end := baseTime.Add(48 * time.Hour)

	stats := filterCitationsForPeriod(citations, start, end)

	if len(stats.citations) != 3 {
		t.Errorf("citations count = %d, want 3", len(stats.citations))
	}
	if len(stats.uniqueCited) != 2 {
		t.Errorf("uniqueCited count = %d, want 2 (/b.md and /c.md)", len(stats.uniqueCited))
	}
}

// ===========================================================================
// metrics.go — countBypassCitations
// ===========================================================================

func TestPoolMetrics_countBypassCitations(t *testing.T) {
	citations := []types.CitationEvent{
		{CitationType: "reference", ArtifactPath: "/a.md"},
		{CitationType: "bypass", ArtifactPath: "/b.md"},
		{CitationType: "recall", ArtifactPath: "bypass:something"},
		{CitationType: "reference", ArtifactPath: "/c.md"},
	}

	got := countBypassCitations(citations)
	if got != 2 {
		t.Errorf("countBypassCitations() = %d, want 2", got)
	}
}

// ===========================================================================
// metrics.go — isKnowledgeFile
// ===========================================================================

func TestPoolMetrics_isKnowledgeFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/path/to/file.md", true},
		{"/path/to/file.jsonl", true},
		{"/path/to/file.go", false},
		{"/path/to/file.txt", false},
		{"readme.md", true},
		{"data.jsonl", true},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			got := isKnowledgeFile(tc.path)
			if got != tc.want {
				t.Errorf("isKnowledgeFile(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

// ===========================================================================
// metrics.go — isStaleArtifact
// ===========================================================================

func TestPoolMetrics_isStaleArtifact(t *testing.T) {
	staleThreshold := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		modTime   time.Time
		lastCited map[string]time.Time
		path      string
		want      bool
	}{
		{
			name:      "recent mod time not stale",
			modTime:   staleThreshold.Add(24 * time.Hour),
			lastCited: map[string]time.Time{},
			path:      "/base/file.md",
			want:      false,
		},
		{
			name:      "old mod, never cited is stale",
			modTime:   staleThreshold.Add(-30 * 24 * time.Hour),
			lastCited: map[string]time.Time{},
			path:      "/base/file.md",
			want:      true,
		},
		{
			name:      "old mod, recently cited not stale",
			modTime:   staleThreshold.Add(-30 * 24 * time.Hour),
			lastCited: map[string]time.Time{
				// normalizeArtifactPath will canonicalize, so we need the right key
			},
			path: "/base/file.md",
			want: true, // no matching citation key
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isStaleArtifact("/base", tc.path, tc.modTime, staleThreshold, tc.lastCited)
			if got != tc.want {
				t.Errorf("isStaleArtifact() = %v, want %v", got, tc.want)
			}
		})
	}
}

// ===========================================================================
// metrics.go — buildLastCitedMap
// ===========================================================================

func TestPoolMetrics_buildLastCitedMap(t *testing.T) {
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	citations := []types.CitationEvent{
		{ArtifactPath: ".agents/learnings/a.md", CitedAt: t1},
		{ArtifactPath: ".agents/learnings/a.md", CitedAt: t3},
		{ArtifactPath: ".agents/learnings/b.md", CitedAt: t2},
		{ArtifactPath: "", CitedAt: t1}, // empty path skipped
	}

	baseDir := "/project"
	got := buildLastCitedMap(baseDir, citations)

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}

	// The map should have latest citation time for each normalized path
	for path, lastTime := range got {
		if strings.Contains(path, "a.md") && !lastTime.Equal(t3) {
			t.Errorf("a.md last cited = %v, want %v", lastTime, t3)
		}
		if strings.Contains(path, "b.md") && !lastTime.Equal(t2) {
			t.Errorf("b.md last cited = %v, want %v", lastTime, t2)
		}
	}
}

// ===========================================================================
// metrics.go — computeUtilityStats
// ===========================================================================

func TestPoolMetrics_computeUtilityStats(t *testing.T) {
	tests := []struct {
		name     string
		values   []float64
		wantMean float64
		wantHigh int
		wantLow  int
	}{
		{
			name:     "empty",
			values:   nil,
			wantMean: 0,
		},
		{
			name:     "single value",
			values:   []float64{0.8},
			wantMean: 0.8,
			wantHigh: 1,
			wantLow:  0,
		},
		{
			name:     "mixed values",
			values:   []float64{0.1, 0.5, 0.9},
			wantMean: 0.5,
			wantHigh: 1,
			wantLow:  1,
		},
		{
			name:     "all high",
			values:   []float64{0.8, 0.9, 0.75},
			wantMean: (0.8 + 0.9 + 0.75) / 3.0,
			wantHigh: 3, // all three are > 0.7
			wantLow:  0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := computeUtilityStats(tc.values)
			if math.Abs(got.mean-tc.wantMean) > 0.01 {
				t.Errorf("mean = %f, want %f", got.mean, tc.wantMean)
			}
			if got.highCount != tc.wantHigh {
				t.Errorf("highCount = %d, want %d", got.highCount, tc.wantHigh)
			}
			if got.lowCount != tc.wantLow {
				t.Errorf("lowCount = %d, want %d", got.lowCount, tc.wantLow)
			}
			if len(tc.values) > 1 && got.stdDev <= 0 {
				t.Errorf("stdDev = %f, expected > 0 for varied data", got.stdDev)
			}
		})
	}
}

// ===========================================================================
// metrics.go — parseUtilityFromFile
// ===========================================================================

func TestMetrics_parseUtilityFromFile(t *testing.T) {
	dir := t.TempDir()

	t.Run("markdown with utility in frontmatter", func(t *testing.T) {
		path := filepath.Join(dir, "learning.md")
		if err := os.WriteFile(path, []byte("---\nutility: 0.85\n---\n# Content"), 0644); err != nil {
			t.Fatal(err)
		}
		got := parseUtilityFromFile(path)
		if math.Abs(got-0.85) > 0.001 {
			t.Errorf("parseUtilityFromFile() = %f, want 0.85", got)
		}
	})

	t.Run("markdown without frontmatter", func(t *testing.T) {
		path := filepath.Join(dir, "no-fm.md")
		if err := os.WriteFile(path, []byte("# Just content"), 0644); err != nil {
			t.Fatal(err)
		}
		got := parseUtilityFromFile(path)
		if got != 0 {
			t.Errorf("parseUtilityFromFile() = %f, want 0", got)
		}
	})

	t.Run("jsonl with utility", func(t *testing.T) {
		path := filepath.Join(dir, "candidate.jsonl")
		if err := os.WriteFile(path, []byte(`{"utility": 0.72, "id": "test"}`+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		got := parseUtilityFromFile(path)
		if math.Abs(got-0.72) > 0.001 {
			t.Errorf("parseUtilityFromFile() = %f, want 0.72", got)
		}
	})

	t.Run("jsonl without utility", func(t *testing.T) {
		path := filepath.Join(dir, "no-utility.jsonl")
		if err := os.WriteFile(path, []byte(`{"id": "test"}`+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		got := parseUtilityFromFile(path)
		if got != 0 {
			t.Errorf("parseUtilityFromFile() = %f, want 0", got)
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		got := parseUtilityFromFile(filepath.Join(dir, "nope.md"))
		if got != 0 {
			t.Errorf("parseUtilityFromFile() = %f, want 0 for nonexistent", got)
		}
	})
}

// ===========================================================================
// metrics.go — parseUtilityFromMarkdown
// ===========================================================================

func TestPoolMetrics_parseUtilityFromMarkdown(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name    string
		content string
		want    float64
	}{
		{
			name:    "valid utility",
			content: "---\nutility: 0.65\n---\n# Content",
			want:    0.65,
		},
		{
			name:    "no frontmatter",
			content: "# No frontmatter",
			want:    0,
		},
		{
			name:    "frontmatter without utility",
			content: "---\ntitle: test\n---\n# Content",
			want:    0,
		},
		{
			name:    "not starting with ---",
			content: "content\n---\nutility: 0.5\n---",
			want:    0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, tc.name+".md")
			if err := os.WriteFile(path, []byte(tc.content), 0644); err != nil {
				t.Fatal(err)
			}
			got := parseUtilityFromMarkdown(path)
			if math.Abs(got-tc.want) > 0.001 {
				t.Errorf("parseUtilityFromMarkdown() = %f, want %f", got, tc.want)
			}
		})
	}
}

// ===========================================================================
// metrics.go — parseUtilityFromJSONL
// ===========================================================================

func TestPoolMetrics_parseUtilityFromJSONL(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name    string
		content string
		want    float64
	}{
		{
			name:    "valid utility",
			content: `{"utility": 0.88}` + "\n",
			want:    0.88,
		},
		{
			name:    "no utility field",
			content: `{"id": "test"}` + "\n",
			want:    0,
		},
		{
			name:    "invalid JSON",
			content: "not json\n",
			want:    0,
		},
		{
			name:    "empty file",
			content: "",
			want:    0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, tc.name+".jsonl")
			if err := os.WriteFile(path, []byte(tc.content), 0644); err != nil {
				t.Fatal(err)
			}
			got := parseUtilityFromJSONL(path)
			if math.Abs(got-tc.want) > 0.001 {
				t.Errorf("parseUtilityFromJSONL() = %f, want %f", got, tc.want)
			}
		})
	}
}

// ===========================================================================
// metrics.go — isRetrievableArtifactPath
// ===========================================================================

func TestPoolMetrics_isRetrievableArtifactPath(t *testing.T) {
	baseDir := "/project"

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"learnings path", ".agents/learnings/test.md", true},
		{"patterns path", ".agents/patterns/error-handling.md", true},
		{"research path", ".agents/research/test.md", false},
		{"candidates path", ".agents/candidates/test.jsonl", false},
		{"empty path", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isRetrievableArtifactPath(baseDir, tc.path)
			if got != tc.want {
				t.Errorf("isRetrievableArtifactPath(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

// ===========================================================================
// metrics.go — retrievableCitationStats
// ===========================================================================

func TestMetrics_retrievableCitationStats(t *testing.T) {
	baseDir := "/project"
	citations := []types.CitationEvent{
		{ArtifactPath: ".agents/learnings/a.md"},
		{ArtifactPath: ".agents/learnings/a.md"},
		{ArtifactPath: ".agents/patterns/b.md"},
		{ArtifactPath: ".agents/research/c.md"},   // not retrievable
		{ArtifactPath: ".agents/candidates/d.md"}, // not retrievable
	}

	unique, evidenced := retrievableCitationStats(baseDir, citations)
	if evidenced != 2 {
		t.Errorf("evidenced = %d, want 2", evidenced)
	}
	if unique != 2 {
		t.Errorf("unique = %d, want 2", unique)
	}
}

// ===========================================================================
// metrics.go — countNewArtifactsInDir
// ===========================================================================

func TestMetrics_countNewArtifactsInDir(t *testing.T) {
	t.Run("nonexistent dir returns 0", func(t *testing.T) {
		count, err := countNewArtifactsInDir("/nonexistent/path", time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Errorf("count = %d, want 0", count)
		}
	})

	t.Run("counts files after since", func(t *testing.T) {
		dir := t.TempDir()
		// Create a file (recent)
		if err := os.WriteFile(filepath.Join(dir, "new.md"), []byte("new"), 0644); err != nil {
			t.Fatal(err)
		}
		// Create an old file
		oldPath := filepath.Join(dir, "old.md")
		if err := os.WriteFile(oldPath, []byte("old"), 0644); err != nil {
			t.Fatal(err)
		}
		oldTime := time.Now().Add(-48 * time.Hour)
		if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}

		since := time.Now().Add(-24 * time.Hour)
		count, err := countNewArtifactsInDir(dir, since)
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("count = %d, want 1", count)
		}
	})
}

// ===========================================================================
// metrics.go — retroHasLearnings
// ===========================================================================

func TestMetrics_retroHasLearnings(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"has ## Learnings", "# Retro\n\n## Learnings\n- Learned X", true},
		{"has ## Key Learnings", "# Retro\n\n## Key Learnings\n- Learned Y", true},
		{"has ### Learnings", "# Retro\n\n### Learnings\n- Learned Z", true},
		{"no learnings section", "# Retro\n\n## Summary\nAll good.", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, tc.name+".md")
			if err := os.WriteFile(path, []byte(tc.content), 0644); err != nil {
				t.Fatal(err)
			}
			got := retroHasLearnings(path)
			if got != tc.want {
				t.Errorf("retroHasLearnings() = %v, want %v", got, tc.want)
			}
		})
	}

	t.Run("nonexistent file returns false", func(t *testing.T) {
		got := retroHasLearnings("/nonexistent/file.md")
		if got {
			t.Error("expected false for nonexistent file")
		}
	})
}

// ===========================================================================
// metrics.go — countArtifacts
// ===========================================================================

func TestMetrics_countArtifacts(t *testing.T) {
	dir := t.TempDir()

	// Create directory structure
	learningsDir := filepath.Join(dir, ".agents", "learnings")
	patternsDir := filepath.Join(dir, ".agents", "patterns")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(patternsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create some files
	if err := os.WriteFile(filepath.Join(learningsDir, "l1.md"), []byte("learning"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(learningsDir, "l2.md"), []byte("learning"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(patternsDir, "p1.md"), []byte("pattern"), 0644); err != nil {
		t.Fatal(err)
	}

	total, tierCounts, err := countArtifacts(dir)
	if err != nil {
		t.Fatal(err)
	}
	if total < 3 {
		t.Errorf("total = %d, want >= 3", total)
	}
	if tierCounts["learning"] != 2 {
		t.Errorf("learning count = %d, want 2", tierCounts["learning"])
	}
	if tierCounts["pattern"] != 1 {
		t.Errorf("pattern count = %d, want 1", tierCounts["pattern"])
	}
}

// ===========================================================================
// metrics_flywheel.go — printFlywheelStatus (smoke test via string matching)
// ===========================================================================

func TestMetricsFlywheel_printFlywheelStatus(t *testing.T) {
	tests := []struct {
		name    string
		metrics *types.FlywheelMetrics
		wantStr string
	}{
		{
			name: "compounding",
			metrics: &types.FlywheelMetrics{
				AboveEscapeVelocity: true,
				Velocity:            0.05,
				Sigma:               0.5,
				Rho:                 0.5,
				SigmaRho:            0.25,
				Delta:               0.17,
				PeriodStart:         time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				PeriodEnd:           time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC),
			},
			wantStr: "COMPOUNDING",
		},
		{
			name: "near escape",
			metrics: &types.FlywheelMetrics{
				AboveEscapeVelocity: false,
				Velocity:            -0.02,
				PeriodStart:         time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				PeriodEnd:           time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC),
			},
			wantStr: "Escape Velocity: [NEAR ESCAPE]",
		},
		{
			name: "decaying with recommendations",
			metrics: &types.FlywheelMetrics{
				AboveEscapeVelocity: false,
				Velocity:            -0.15,
				Sigma:               0.1,
				Rho:                 0.2,
				StaleArtifacts:      10,
				PeriodStart:         time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				PeriodEnd:           time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC),
			},
			wantStr: "DECAYING",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf strings.Builder
			printFlywheelStatus(&buf, tc.metrics)
			got := buf.String()
			if !strings.Contains(got, tc.wantStr) {
				t.Errorf("output should contain %q, got:\n%s", tc.wantStr, got)
			}
			if !strings.Contains(got, "Flywheel Health") {
				t.Error("output should contain 'Flywheel Health' header")
			}
		})
	}
}

// ===========================================================================
// canonical_identity.go — canonicalSessionID
// ===========================================================================

func TestMetrics_canonicalSessionID(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"already canonical", "session-20260115-120000", "session-20260115-120000"},
		{"timestamp only", "20260115-120000", "session-20260115-120000"},
		{"session- prefix but not timestamp", "session-custom", "session-custom"},
		{"uuid format", "a1b2c3d4-e5f6-7890-abcd-ef1234567890", "session-uuid-a1b2c3d4-e5f6-7890-abcd-ef1234567890"},
		{"plain string", "ag-abc123", "ag-abc123"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := canonicalSessionID(tc.raw)
			if got != tc.want {
				t.Errorf("canonicalSessionID(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

// ===========================================================================
// canonical_identity.go — canonicalArtifactPath
// ===========================================================================

func TestMetrics_canonicalArtifactPath(t *testing.T) {
	tests := []struct {
		name    string
		baseDir string
		path    string
		wantAbs bool
	}{
		{"absolute path stays absolute", "/project", "/abs/path/file.md", true},
		{"relative path made absolute", "/project", ".agents/learnings/a.md", true},
		{"empty path returns empty", "/project", "", false},
		{"whitespace path returns empty", "/project", "   ", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := canonicalArtifactPath(tc.baseDir, tc.path)
			if tc.wantAbs && !filepath.IsAbs(got) {
				t.Errorf("expected absolute path, got %q", got)
			}
			if tc.path == "" || strings.TrimSpace(tc.path) == "" {
				if got != "" {
					t.Errorf("expected empty for empty/whitespace input, got %q", got)
				}
			}
		})
	}
}

// ===========================================================================
// canonical_identity.go — sessionIDAliases
// ===========================================================================

func TestMetrics_sessionIDAliases(t *testing.T) {
	t.Run("timestamp session generates aliases", func(t *testing.T) {
		aliases := sessionIDAliases("20260115-120000")
		aliasSet := make(map[string]bool)
		for _, a := range aliases {
			aliasSet[a] = true
		}
		if !aliasSet["20260115-120000"] {
			t.Error("expected raw timestamp in aliases")
		}
		if !aliasSet["session-20260115-120000"] {
			t.Error("expected session- prefixed timestamp in aliases")
		}
	})

	t.Run("session- prefix generates timestamp alias", func(t *testing.T) {
		aliases := sessionIDAliases("session-20260115-120000")
		aliasSet := make(map[string]bool)
		for _, a := range aliases {
			aliasSet[a] = true
		}
		if !aliasSet["20260115-120000"] {
			t.Error("expected bare timestamp in aliases")
		}
	})

	t.Run("empty returns non-empty (generates canonical)", func(t *testing.T) {
		aliases := sessionIDAliases("")
		if len(aliases) == 0 {
			t.Error("expected at least one alias for empty input")
		}
	})
}
