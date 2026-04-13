package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"strings"

	"github.com/boshu2/agentops/cli/internal/pool"
	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/taxonomy"
	"github.com/boshu2/agentops/cli/internal/types"
)

func TestParseLearningBlocks(t *testing.T) {
	md := `# Learnings: ag-xyz — Something

**Date:** 2026-01-01

# Learning: First Title

**ID**: L1
**Category**: process
**Confidence**: high

## What We Learned

Do the thing.

# Learning: Second Title

**ID**: L2
**Category**: architecture
**Confidence**: medium

## What We Learned

Do the other thing.
`

	blocks := parseLearningBlocks(md)
	if len(blocks) != 2 {
		t.Fatalf("blocks=%d, want 2", len(blocks))
	}
	if blocks[0].Title != "First Title" || blocks[0].ID != "L1" || blocks[0].Category != "process" || blocks[0].Confidence != "high" {
		t.Fatalf("block0=%+v", blocks[0])
	}
	if blocks[1].Title != "Second Title" || blocks[1].ID != "L2" || blocks[1].Category != "architecture" || blocks[1].Confidence != "medium" {
		t.Fatalf("block1=%+v", blocks[1])
	}
}

func TestParseLearningBlocksLegacyFrontmatter(t *testing.T) {
	md := `---
type: learning
source: manual
date: 2026-02-20
---

# Fix shell PATH mismatch for ao detection

Ensure command checks run in the same shell context as runtime.
`

	blocks := parseLearningBlocks(md)
	if len(blocks) != 1 {
		t.Fatalf("blocks=%d, want 1", len(blocks))
	}
	if blocks[0].Category != "learning" {
		t.Fatalf("category=%q, want learning", blocks[0].Category)
	}
	if blocks[0].Confidence != "medium" {
		t.Fatalf("confidence=%q, want medium default", blocks[0].Confidence)
	}
	if blocks[0].Title == "" {
		t.Fatal("expected non-empty title")
	}
}

func TestResolveIngestFilesDefaultIncludesLegacyKnowledge(t *testing.T) {
	tmp := t.TempDir()
	pendingDir := filepath.Join(tmp, ".agents", "knowledge", "pending")
	rootKnowledge := filepath.Join(tmp, ".agents", "knowledge")
	if err := os.MkdirAll(pendingDir, 0o700); err != nil {
		t.Fatalf("mkdir pending: %v", err)
	}
	if err := os.MkdirAll(rootKnowledge, 0o700); err != nil {
		t.Fatalf("mkdir knowledge: %v", err)
	}

	pendingFile := filepath.Join(pendingDir, "2026-02-20-a.md")
	legacyFile := filepath.Join(rootKnowledge, "2026-02-20-b.md")
	if err := os.WriteFile(pendingFile, []byte("# Learning: A"), 0o600); err != nil {
		t.Fatalf("write pending: %v", err)
	}
	if err := os.WriteFile(legacyFile, []byte("# Learning: B"), 0o600); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	files, err := resolveIngestFiles(tmp, filepath.Join(".agents", "knowledge", "pending"), nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	seen := make(map[string]bool)
	for _, f := range files {
		seen[f] = true
	}
	if !seen[pendingFile] {
		t.Fatalf("missing pending file in default ingest set: %s", pendingFile)
	}
	if !seen[legacyFile] {
		t.Fatalf("missing legacy file in default ingest set: %s", legacyFile)
	}
}

func TestIngestAutoPromoteAndIndex(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	pendingDir := filepath.Join(tmp, ".agents", "knowledge", "pending")
	if err := os.MkdirAll(pendingDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	pendingFile := filepath.Join(pendingDir, "2026-01-01-ag-xyz-learnings.md")
	if err := os.WriteFile(pendingFile, []byte(`# Learnings: ag-xyz — Something

**Date:** 2026-01-01

# Learning: First Title

**ID**: L1
**Category**: process
**Confidence**: high

## What We Learned

Run command -v ao first.

## Source

Session: ag-xyz
`), 0600); err != nil {
		t.Fatalf("write pending: %v", err)
	}

	ingRes, err := ingestPendingFilesToPool(tmp, []string{pendingFile})
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if ingRes.Added != 1 {
		t.Fatalf("added=%d, want 1 (res=%+v)", ingRes.Added, ingRes)
	}

	p := pool.NewPool(tmp)
	entries, err := p.List(pool.ListOptions{Status: types.PoolStatusPending})
	if err != nil {
		t.Fatalf("list pool: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("pool entries=%d, want 1", len(entries))
	}

	// Auto-promotion now requires citation evidence.
	if err := ratchet.RecordCitation(tmp, types.CitationEvent{
		ArtifactPath: entries[0].FilePath,
		SessionID:    "session-ingest-test",
		CitedAt:      time.Now(),
		CitationType: "retrieved",
		Query:        "session hygiene",
	}); err != nil {
		t.Fatalf("record citation: %v", err)
	}

	// With a 1h threshold, a 2026-01-01 AddedAt should be eligible.
	autoRes, err := autoPromoteAndPromoteToArtifacts(p, time.Hour, true)
	if err != nil {
		t.Fatalf("auto-promote: %v", err)
	}
	if autoRes.Promoted != 1 || len(autoRes.Artifacts) != 1 {
		t.Fatalf("autoRes=%+v", autoRes)
	}

	if _, err := os.Stat(autoRes.Artifacts[0]); err != nil {
		t.Fatalf("artifact missing: %v", err)
	}

	// Ensure artifact landed in .agents/learnings.
	if filepath.Base(filepath.Dir(autoRes.Artifacts[0])) != "learnings" {
		t.Fatalf("artifact dir=%s, want learnings", filepath.Dir(autoRes.Artifacts[0]))
	}

	indexed, indexPath, err := storeIndexUpsert(tmp, autoRes.Artifacts, true)
	if err != nil {
		t.Fatalf("store index: %v", err)
	}
	if indexed != 1 {
		t.Fatalf("indexed=%d, want 1", indexed)
	}
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("index missing: %v", err)
	}

	f, err := os.Open(indexPath)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		t.Fatalf("expected index entry line")
	}
	var ie IndexEntry
	if err := json.Unmarshal(sc.Bytes(), &ie); err != nil {
		t.Fatalf("unmarshal index: %v", err)
	}
	if ie.Category != "process" {
		t.Fatalf("index category=%q, want %q", ie.Category, "process")
	}
}

// ---------------------------------------------------------------------------
// slugify
// ---------------------------------------------------------------------------

func TestPoolIngestCoverage_Slugify(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "simple lowercase", input: "hello-world", want: "hello-world"},
		{name: "uppercase converted", input: "Hello-World", want: "hello-world"},
		{name: "spaces become dashes", input: "hello world", want: "hello-world"},
		{name: "special chars become dashes", input: "hello@world#2024", want: "hello-world-2024"},
		{name: "consecutive special chars single dash", input: "hello!!!world", want: "hello-world"},
		{name: "leading special trimmed", input: "---hello", want: "hello"},
		{name: "trailing special trimmed", input: "hello---", want: "hello"},
		{name: "empty returns cand", input: "", want: "cand"},
		{name: "only special chars returns cand", input: "!@#$%", want: "cand"},
		{name: "digits preserved", input: "test123abc", want: "test123abc"},
		{name: "long string kept", input: strings.Repeat("a", 200), want: strings.Repeat("a", 200)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slugify(tt.input)
			if got != tt.want {
				t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// isSlugAlphanumeric
// ---------------------------------------------------------------------------

func TestPoolIngestCoverage_IsSlugAlphanumeric(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
	}{
		{'a', true},
		{'z', true},
		{'0', true},
		{'9', true},
		{'A', false}, // uppercase not alphanumeric in slug context
		{'-', false},
		{' ', false},
		{'_', false},
	}
	for _, tt := range tests {
		got := pool.IsSlugAlphanumeric(tt.r)
		if got != tt.want {
			t.Errorf("isSlugAlphanumeric(%q) = %v, want %v", tt.r, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// computeSpecificityScore
// ---------------------------------------------------------------------------

func TestPoolIngestCoverage_ComputeSpecificityScore(t *testing.T) {
	t.Run("baseline plain text", func(t *testing.T) {
		body := "Plain text without any code or numbers."
		lower := strings.ToLower(body)
		score := computeSpecificityScore(body, lower)
		if score < 0.4 || score > 0.5 {
			t.Errorf("baseline score = %v, want ~0.4", score)
		}
	})

	t.Run("backtick boosts score", func(t *testing.T) {
		body := "Use `go build` to compile."
		lower := strings.ToLower(body)
		score := computeSpecificityScore(body, lower)
		if score < 0.6 {
			t.Errorf("backtick score = %v, want >= 0.6", score)
		}
	})

	t.Run("digits boost score", func(t *testing.T) {
		body := "The value should be 42."
		lower := strings.ToLower(body)
		score := computeSpecificityScore(body, lower)
		if score < 0.6 {
			t.Errorf("digits score = %v, want >= 0.6", score)
		}
	})

	t.Run("filename boosts score", func(t *testing.T) {
		body := "Edit cli/cmd/ao/main.go to fix."
		lower := strings.ToLower(body)
		score := computeSpecificityScore(body, lower)
		if score < 0.6 {
			t.Errorf("filename score = %v, want >= 0.6", score)
		}
	})

	t.Run("line reference boosts score", func(t *testing.T) {
		body := "See line 42 for the issue."
		lower := strings.ToLower(body)
		score := computeSpecificityScore(body, lower)
		if score < 0.6 {
			t.Errorf("line ref score = %v, want >= 0.6", score)
		}
	})

	t.Run("all boosts capped at 1.0", func(t *testing.T) {
		body := "```go\nfmt.Println(\"hello\")\n```\nValue is 42 on line 5 in main.go"
		lower := strings.ToLower(body)
		score := computeSpecificityScore(body, lower)
		if score > 1.0 {
			t.Errorf("score = %v, want <= 1.0", score)
		}
	})
}

// ---------------------------------------------------------------------------
// computeActionabilityScore
// ---------------------------------------------------------------------------

func TestPoolIngestCoverage_ComputeActionabilityScore(t *testing.T) {
	t.Run("baseline plain text", func(t *testing.T) {
		score := computeActionabilityScore("Plain text here.")
		if score < 0.4 || score > 0.5 {
			t.Errorf("baseline = %v, want ~0.4", score)
		}
	})

	t.Run("list items boost", func(t *testing.T) {
		score := computeActionabilityScore("Steps:\n- Step one\n- Step two\n")
		if score < 0.6 {
			t.Errorf("list score = %v, want >= 0.6", score)
		}
	})

	t.Run("action verbs boost", func(t *testing.T) {
		score := computeActionabilityScore("Run go build to ensure it compiles.")
		if score < 0.6 {
			t.Errorf("verbs score = %v, want >= 0.6", score)
		}
	})

	t.Run("code block boosts", func(t *testing.T) {
		score := computeActionabilityScore("Use this:\n```\ngo test ./...\n```\n")
		if score < 0.6 {
			t.Errorf("code block score = %v, want >= 0.6", score)
		}
	})

	t.Run("capped at 1.0", func(t *testing.T) {
		body := "- Run this. Must fix.\n```\ncommand\n```\n- Add check.\n"
		score := computeActionabilityScore(body)
		if score > 1.0 {
			t.Errorf("score = %v, want <= 1.0", score)
		}
	})
}

// ---------------------------------------------------------------------------
// computeNoveltyScore
// ---------------------------------------------------------------------------

func TestPoolIngestCoverage_ComputeNoveltyScore(t *testing.T) {
	t.Run("medium length baseline", func(t *testing.T) {
		body := strings.Repeat("word ", 60) // ~300 chars
		score := computeNoveltyScore(body)
		if score < 0.4 || score > 0.6 {
			t.Errorf("baseline = %v, want ~0.5", score)
		}
	})

	t.Run("long body boosts", func(t *testing.T) {
		body := strings.Repeat("word ", 200) // ~1000 chars
		score := computeNoveltyScore(body)
		if score < 0.5 {
			t.Errorf("long body = %v, want >= 0.5", score)
		}
	})

	t.Run("short body penalized", func(t *testing.T) {
		score := computeNoveltyScore("short")
		if score > 0.5 {
			t.Errorf("short body = %v, want <= 0.5", score)
		}
	})

	t.Run("clamped to [0, 1]", func(t *testing.T) {
		score := computeNoveltyScore("")
		if score < 0.0 || score > 1.0 {
			t.Errorf("score = %v, want in [0, 1]", score)
		}
	})
}

// ---------------------------------------------------------------------------
// computeContextScore
// ---------------------------------------------------------------------------

func TestPoolIngestCoverage_ComputeContextScore(t *testing.T) {
	t.Run("baseline", func(t *testing.T) {
		score := computeContextScore("plain content")
		if score < 0.4 || score > 0.6 {
			t.Errorf("baseline = %v, want ~0.5", score)
		}
	})

	t.Run("source section boosts", func(t *testing.T) {
		score := computeContextScore("content\n## source\nsession xyz")
		if score < 0.6 {
			t.Errorf("source section = %v, want >= 0.6", score)
		}
	})

	t.Run("bold source boosts", func(t *testing.T) {
		score := computeContextScore("**source**: session xyz")
		if score < 0.6 {
			t.Errorf("bold source = %v, want >= 0.6", score)
		}
	})

	t.Run("why it matters boosts", func(t *testing.T) {
		score := computeContextScore("## why it matters\nimportant stuff")
		if score < 0.5 {
			t.Errorf("why it matters = %v, want >= 0.5", score)
		}
	})

	t.Run("capped at 1.0", func(t *testing.T) {
		score := computeContextScore("## source\n**source**: x\n## why it matters\ny")
		if score > 1.0 {
			t.Errorf("score = %v, want <= 1.0", score)
		}
	})
}

// ---------------------------------------------------------------------------
// rubricWeightedSum
// ---------------------------------------------------------------------------

func TestPoolIngestCoverage_RubricWeightedSum(t *testing.T) {
	rubric := types.RubricScores{
		Specificity:   1.0,
		Actionability: 1.0,
		Novelty:       1.0,
		Context:       1.0,
		Confidence:    1.0,
	}
	w := taxonomy.DefaultRubricWeights
	sum := rubricWeightedSum(rubric, w)
	// Sum of all weights should be ~1.0
	expectedSum := w.Specificity + w.Actionability + w.Novelty + w.Context + w.Confidence
	if sum < expectedSum-0.01 || sum > expectedSum+0.01 {
		t.Errorf("rubricWeightedSum = %v, want ~%v", sum, expectedSum)
	}

	// All zeros
	zero := types.RubricScores{}
	if rubricWeightedSum(zero, w) != 0.0 {
		t.Error("expected 0 for zero rubric")
	}
}

// ---------------------------------------------------------------------------
// buildCandidateFromLearningBlock
// ---------------------------------------------------------------------------

func TestPoolIngestCoverage_BuildCandidateFromLearningBlock(t *testing.T) {
	fixture := newPoolIngestCandidateFixture()

	t.Run("valid block produces candidate", func(t *testing.T) {
		block := learningBlock{
			Title:      "Test Learning",
			ID:         "L1",
			Category:   "process",
			Confidence: "high",
			Body:       "## What We Learned\n\nRun go test before commit.\n",
		}
		cand, scoring := fixture.mustBuildCandidate(t, block)
		fixture.assertValidLearningCandidate(t, cand, scoring)
	})

	t.Run("empty title returns not ok", func(t *testing.T) {
		fixture.assertRejected(t, learningBlock{Title: "", Body: "some body"}, "empty title")
	})

	t.Run("empty body returns not ok", func(t *testing.T) {
		fixture.assertRejected(t, learningBlock{Title: "Title", Body: ""}, "empty body")
	})

	t.Run("whitespace-only title returns not ok", func(t *testing.T) {
		fixture.assertRejected(t, learningBlock{Title: "   ", Body: "some body"}, "whitespace-only title")
	})

	t.Run("stub 'no significant learnings' body returns not ok", func(t *testing.T) {
		block := learningBlock{
			Title: "Session Summary",
			ID:    "L-stub",
			Body:  "No significant learnings from this session.",
		}
		fixture.assertRejected(t, block, "stub 'no significant learnings' body")
	})

	t.Run("decision category sets KnowledgeTypeDecision", func(t *testing.T) {
		block := learningBlock{
			Title:      "Always Use Conventional Commits",
			ID:         "L-dec",
			Category:   "decision",
			Confidence: "high",
			Body:       "We decided to always use conventional commits for consistency.",
		}
		cand, _ := fixture.mustBuildCandidate(t, block)
		assertCandidateType(t, cand, types.KnowledgeTypeDecision)
	})

	t.Run("high confidence boosts raw score", func(t *testing.T) {
		high := learningBlock{Title: "Test", ID: "L1", Confidence: "high", Body: "Some body content here."}
		low := learningBlock{Title: "Test", ID: "L2", Confidence: "low", Body: "Some body content here."}
		candHigh, _ := fixture.mustBuildCandidate(t, high)
		candLow, _ := fixture.mustBuildCandidate(t, low)
		assertRawScoreGreater(t, candHigh, candLow)
	})

	t.Run("long ID gets truncated with hash", func(t *testing.T) {
		longID := strings.Repeat("x", 200)
		block := learningBlock{Title: "Test", ID: longID, Confidence: "medium", Body: "Some learning body."}
		cand, _ := fixture.mustBuildCandidate(t, block)
		assertCandidateIDMaxLength(t, cand, 120)
	})

	t.Run("metadata populated", func(t *testing.T) {
		block := learningBlock{Title: "Test", ID: "L1", Category: "process", Confidence: "high", Body: "Body text."}
		cand, _ := fixture.mustBuildCandidate(t, block)
		assertCandidateMetadata(t, cand, map[string]string{
			"pending_category":   "process",
			"pending_confidence": "high",
			"pending_title":      "Test",
		})
	})
}

type poolIngestCandidateFixture struct {
	fileDate    time.Time
	srcPath     string
	sessionHint string
}

func newPoolIngestCandidateFixture() poolIngestCandidateFixture {
	return poolIngestCandidateFixture{
		fileDate:    time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		srcPath:     "/test/file.md",
		sessionHint: "ag-xyz",
	}
}

func (f poolIngestCandidateFixture) build(block learningBlock) (types.Candidate, types.Scoring, bool) {
	return buildCandidateFromLearningBlock(block, f.srcPath, f.fileDate, f.sessionHint)
}

func (f poolIngestCandidateFixture) mustBuildCandidate(t *testing.T, block learningBlock) (types.Candidate, types.Scoring) {
	t.Helper()

	cand, scoring, ok := f.build(block)
	if !ok {
		t.Fatal("expected ok=true")
	}
	return cand, scoring
}

func (f poolIngestCandidateFixture) assertRejected(t *testing.T, block learningBlock, reason string) {
	t.Helper()

	_, _, ok := f.build(block)
	if ok {
		t.Fatalf("expected ok=false for %s", reason)
	}
}

func (f poolIngestCandidateFixture) assertValidLearningCandidate(t *testing.T, cand types.Candidate, scoring types.Scoring) {
	t.Helper()

	if cand.ID == "" {
		t.Error("expected non-empty ID")
	}
	assertCandidateType(t, cand, types.KnowledgeTypeLearning)
	if cand.RawScore <= 0 || cand.RawScore > 1.0 {
		t.Errorf("RawScore = %v, want (0, 1]", cand.RawScore)
	}
	if scoring.RawScore != cand.RawScore {
		t.Errorf("scoring.RawScore = %v, want %v", scoring.RawScore, cand.RawScore)
	}
	if scoring.GateRequired && cand.Tier == "" {
		t.Error("if gate required, tier should be set")
	}
	if cand.Maturity != types.MaturityProvisional {
		t.Errorf("Maturity = %q, want %q", cand.Maturity, types.MaturityProvisional)
	}
	if cand.ExtractedAt != f.fileDate {
		t.Errorf("ExtractedAt = %v, want %v", cand.ExtractedAt, f.fileDate)
	}
}

func assertCandidateType(t *testing.T, cand types.Candidate, want types.KnowledgeType) {
	t.Helper()

	if cand.Type != want {
		t.Errorf("Type = %q, want %q", cand.Type, want)
	}
}

func assertRawScoreGreater(t *testing.T, high types.Candidate, low types.Candidate) {
	t.Helper()

	if high.RawScore <= low.RawScore {
		t.Errorf("high confidence score %v should be > low confidence score %v", high.RawScore, low.RawScore)
	}
}

func assertCandidateIDMaxLength(t *testing.T, cand types.Candidate, maxLen int) {
	t.Helper()

	if len(cand.ID) > maxLen {
		t.Errorf("ID length = %d, want <= %d", len(cand.ID), maxLen)
	}
}

func assertCandidateMetadata(t *testing.T, cand types.Candidate, want map[string]string) {
	t.Helper()

	if cand.Metadata == nil {
		t.Fatal("expected non-nil Metadata")
	}
	for key, wantValue := range want {
		if cand.Metadata[key] != wantValue {
			t.Errorf("%s = %v, want %q", key, cand.Metadata[key], wantValue)
		}
	}
}

// ---------------------------------------------------------------------------
// parseLearningBlocks edge cases
// ---------------------------------------------------------------------------

func TestPoolIngestCoverage_ParseLearningBlocks(t *testing.T) {
	t.Run("no learning headers returns nil", func(t *testing.T) {
		md := "# Some Other Document\n\nNo learning blocks here.\n"
		blocks := parseLearningBlocks(md)
		if len(blocks) != 0 {
			t.Errorf("expected 0 blocks, got %d", len(blocks))
		}
	})

	t.Run("single block", func(t *testing.T) {
		md := "# Learning: Single Block\n\n**ID**: L1\n**Category**: process\n**Confidence**: medium\n\nContent.\n"
		blocks := parseLearningBlocks(md)
		if len(blocks) != 1 {
			t.Fatalf("expected 1 block, got %d", len(blocks))
		}
		if blocks[0].Title != "Single Block" {
			t.Errorf("Title = %q, want %q", blocks[0].Title, "Single Block")
		}
		if blocks[0].ID != "L1" {
			t.Errorf("ID = %q, want %q", blocks[0].ID, "L1")
		}
	})

	t.Run("three blocks", func(t *testing.T) {
		md := "# Learning: A\n\n**ID**: L1\n\nBody A.\n\n# Learning: B\n\n**ID**: L2\n\nBody B.\n\n# Learning: C\n\n**ID**: L3\n\nBody C.\n"
		blocks := parseLearningBlocks(md)
		if len(blocks) != 3 {
			t.Errorf("expected 3 blocks, got %d", len(blocks))
		}
	})
}

// ---------------------------------------------------------------------------
// parseYAMLFrontmatter
// ---------------------------------------------------------------------------

func TestPoolIngestCoverage_ParseYAMLFrontmatter(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want map[string]string
	}{
		{
			name: "simple key-value pairs",
			raw:  "type: learning\nsource: manual\ndate: 2026-01-01",
			want: map[string]string{"type": "learning", "source": "manual", "date": "2026-01-01"},
		},
		{
			name: "quoted values stripped",
			raw:  `type: "learning"`,
			want: map[string]string{"type": "learning"},
		},
		{
			name: "single quoted values stripped",
			raw:  `type: 'learning'`,
			want: map[string]string{"type": "learning"},
		},
		{
			name: "comment lines skipped",
			raw:  "# comment\ntype: learning",
			want: map[string]string{"type": "learning"},
		},
		{
			name: "empty lines skipped",
			raw:  "\n\ntype: learning\n\n",
			want: map[string]string{"type": "learning"},
		},
		{
			name: "no colon lines skipped",
			raw:  "nocolon",
			want: map[string]string{},
		},
		{
			name: "empty string",
			raw:  "",
			want: map[string]string{},
		},
		{
			name: "keys lowercased",
			raw:  "Type: LEARNING",
			want: map[string]string{"type": "LEARNING"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseYAMLFrontmatter(tt.raw)
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("key %q = %q, want %q", k, got[k], v)
				}
			}
			if len(got) != len(tt.want) {
				t.Errorf("len = %d, want %d; got %v", len(got), len(tt.want), got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractFirstHeadingText
// ---------------------------------------------------------------------------

func TestPoolIngestCoverage_ExtractFirstHeadingText(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "heading with hash", body: "# My Title\n\nContent", want: "My Title"},
		{name: "plain first line", body: "First line\nSecond line", want: "First line"},
		{name: "empty lines then content", body: "\n\n\nContent", want: "Content"},
		{name: "all empty", body: "\n\n\n", want: ""},
		{name: "hash only", body: "#\n", want: ""},
		{name: "multi-hash heading strips one hash", body: "## Subtitle", want: "# Subtitle"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractFirstHeadingText(tt.body)
			if got != tt.want {
				t.Errorf("extractFirstHeadingText = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseLegacyFrontmatterLearning
// ---------------------------------------------------------------------------

func TestPoolIngestCoverage_ParseLegacyFrontmatterLearning(t *testing.T) {
	t.Run("valid legacy learning", func(t *testing.T) {
		md := "---\ntype: learning\nsource: manual\ndate: 2026-02-20\n---\n\n# Fix PATH issue\n\nDetails here.\n"
		block, ok := parseLegacyFrontmatterLearning(md)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if block.Category != "learning" {
			t.Errorf("Category = %q, want %q", block.Category, "learning")
		}
		if block.Confidence != "medium" {
			t.Errorf("Confidence = %q, want %q", block.Confidence, "medium")
		}
		if block.Title == "" {
			t.Error("expected non-empty title")
		}
	})

	t.Run("no frontmatter returns false", func(t *testing.T) {
		_, ok := parseLegacyFrontmatterLearning("# Just a document\n\nContent.\n")
		if ok {
			t.Error("expected ok=false without frontmatter")
		}
	})

	t.Run("no type field returns false", func(t *testing.T) {
		md := "---\nsource: manual\n---\n\n# Title\n\nContent.\n"
		_, ok := parseLegacyFrontmatterLearning(md)
		if ok {
			t.Error("expected ok=false without type field")
		}
	})

	t.Run("empty body after frontmatter returns false", func(t *testing.T) {
		md := "---\ntype: learning\n---\n"
		_, ok := parseLegacyFrontmatterLearning(md)
		if ok {
			t.Error("expected ok=false with empty body")
		}
	})

	t.Run("body with no heading text returns false", func(t *testing.T) {
		md := "---\ntype: learning\n---\n\n\n\n"
		_, ok := parseLegacyFrontmatterLearning(md)
		if ok {
			t.Error("expected ok=false with no heading text")
		}
	})

	t.Run("id from frontmatter", func(t *testing.T) {
		md := "---\ntype: learning\nid: custom-id\n---\n\n# Title\n\nBody.\n"
		block, ok := parseLegacyFrontmatterLearning(md)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if block.ID != "custom-id" {
			t.Errorf("ID = %q, want %q", block.ID, "custom-id")
		}
	})

	t.Run("confidence from frontmatter", func(t *testing.T) {
		md := "---\ntype: learning\nconfidence: high\n---\n\n# Title\n\nBody.\n"
		block, ok := parseLegacyFrontmatterLearning(md)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if block.Confidence != "high" {
			t.Errorf("Confidence = %q, want %q", block.Confidence, "high")
		}
	})
}

// ---------------------------------------------------------------------------
// dateFromFrontmatter
// ---------------------------------------------------------------------------

func TestPoolIngestCoverage_DateFromFrontmatter(t *testing.T) {
	t.Run("valid date", func(t *testing.T) {
		md := "---\ndate: 2026-03-15\n---\n# Content"
		d, ok := dateFromFrontmatter(md, "")
		if !ok {
			t.Fatal("expected ok=true")
		}
		if d.Year() != 2026 || d.Month() != 3 || d.Day() != 15 {
			t.Errorf("date = %v, want 2026-03-15", d)
		}
	})

	t.Run("no frontmatter returns false", func(t *testing.T) {
		_, ok := dateFromFrontmatter("# No frontmatter", "")
		if ok {
			t.Error("expected ok=false")
		}
	})

	t.Run("frontmatter without date returns false", func(t *testing.T) {
		md := "---\ntype: learning\n---\n# Content"
		_, ok := dateFromFrontmatter(md, "")
		if ok {
			t.Error("expected ok=false")
		}
	})
}

// ---------------------------------------------------------------------------
// dateFromMarkdownField
// ---------------------------------------------------------------------------

func TestPoolIngestCoverage_DateFromMarkdownField(t *testing.T) {
	t.Run("valid date field", func(t *testing.T) {
		md := "# Doc\n\n**Date**: 2026-05-20\n\nContent"
		d, ok := dateFromMarkdownField(md, "")
		if !ok {
			t.Fatal("expected ok=true")
		}
		if d.Year() != 2026 || d.Month() != 5 || d.Day() != 20 {
			t.Errorf("date = %v, want 2026-05-20", d)
		}
	})

	t.Run("no date field returns false", func(t *testing.T) {
		_, ok := dateFromMarkdownField("# No date here", "")
		if ok {
			t.Error("expected ok=false")
		}
	})
}

// ---------------------------------------------------------------------------
// dateFromFilenamePrefix
// ---------------------------------------------------------------------------

func TestPoolIngestCoverage_DateFromFilenamePrefix(t *testing.T) {
	t.Run("valid prefix", func(t *testing.T) {
		d, ok := dateFromFilenamePrefix("", "/path/2026-01-15-test.md")
		if !ok {
			t.Fatal("expected ok=true")
		}
		if d.Year() != 2026 || d.Month() != 1 || d.Day() != 15 {
			t.Errorf("date = %v, want 2026-01-15", d)
		}
	})

	t.Run("short filename returns false", func(t *testing.T) {
		_, ok := dateFromFilenamePrefix("", "/path/test.md")
		if ok {
			t.Error("expected ok=false for short filename")
		}
	})

	t.Run("non-date prefix returns false", func(t *testing.T) {
		_, ok := dateFromFilenamePrefix("", "/path/not-a-date-x.md")
		if ok {
			t.Error("expected ok=false for non-date prefix")
		}
	})
}

// ---------------------------------------------------------------------------
// dateFromFileMtime
// ---------------------------------------------------------------------------

func TestPoolIngestCoverage_DateFromFileMtime(t *testing.T) {
	t.Run("existing file returns mtime", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "test.md")
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
		d, ok := dateFromFileMtime("", path)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if d.IsZero() {
			t.Error("expected non-zero date")
		}
	})

	t.Run("nonexistent file returns false", func(t *testing.T) {
		_, ok := dateFromFileMtime("", "/nonexistent/file.md")
		if ok {
			t.Error("expected ok=false for nonexistent file")
		}
	})
}

// ---------------------------------------------------------------------------
// extractSessionHint
// ---------------------------------------------------------------------------

func TestPoolIngestCoverage_ExtractSessionHint(t *testing.T) {
	t.Run("finds session ID in content", func(t *testing.T) {
		md := "Session ag-abc123 was productive.\n"
		hint := extractSessionHint(md, "/test/file.md")
		if hint != "ag-abc123" {
			t.Errorf("hint = %q, want %q", hint, "ag-abc123")
		}
	})

	t.Run("no session ID falls back to filename", func(t *testing.T) {
		md := "No session ID here.\n"
		hint := extractSessionHint(md, "/test/my-learning.md")
		if hint != "my-learning" {
			t.Errorf("hint = %q, want %q", hint, "my-learning")
		}
	})

	t.Run("long content only checks first 2KB", func(t *testing.T) {
		md := strings.Repeat("x", 3000) + "ag-late"
		hint := extractSessionHint(md, "/test/file.md")
		// Should not find "ag-late" past 2KB
		if hint == "ag-late" {
			t.Error("should not find session hint beyond 2KB")
		}
	})
}

// ---------------------------------------------------------------------------
// resolveIngestFiles
// ---------------------------------------------------------------------------

func TestPoolIngestCoverage_ResolveIngestFiles(t *testing.T) {
	tmp := t.TempDir()

	// Create files
	dir := filepath.Join(tmp, ".agents", "knowledge", "pending")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.md"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("with explicit args", func(t *testing.T) {
		files, err := resolveIngestFiles(tmp, "", []string{filepath.Join(dir, "a.md")})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 1 {
			t.Errorf("expected 1 file, got %d", len(files))
		}
	})

	t.Run("deduplicates", func(t *testing.T) {
		path := filepath.Join(dir, "a.md")
		files, err := resolveIngestFiles(tmp, "", []string{path, path})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 1 {
			t.Errorf("expected 1 file (deduped), got %d", len(files))
		}
	})

	t.Run("relative paths resolved", func(t *testing.T) {
		files, err := resolveIngestFiles(tmp, "", []string{".agents/knowledge/pending/a.md"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 1 {
			t.Errorf("expected 1 file, got %d", len(files))
		}
	})
}

// ---------------------------------------------------------------------------
// moveIngestedFiles
// ---------------------------------------------------------------------------

func TestPoolIngestCoverage_MoveIngestedFiles(t *testing.T) {
	tmp := t.TempDir()

	// Create source files
	srcDir := filepath.Join(tmp, ".agents", "knowledge", "pending")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	srcFile := filepath.Join(srcDir, "test.md")
	if err := os.WriteFile(srcFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	moveIngestedFiles(tmp, []string{srcFile})

	// Source should be gone
	if _, err := os.Stat(srcFile); !os.IsNotExist(err) {
		t.Error("expected source file to be moved")
	}

	// Should be in processed dir
	dstFile := filepath.Join(tmp, ".agents", "knowledge", "processed", "test.md")
	if _, err := os.Stat(dstFile); err != nil {
		t.Errorf("expected file in processed dir: %v", err)
	}
}

// ---------------------------------------------------------------------------
// outputPoolIngestResult (smoke test)
// ---------------------------------------------------------------------------

func TestPoolIngestCoverage_OutputPoolIngestResult(t *testing.T) {
	t.Run("text output", func(t *testing.T) {
		res := poolIngestResult{
			FilesScanned:     3,
			CandidatesFound:  5,
			Added:            2,
			SkippedExisting:  1,
			SkippedMalformed: 1,
			Errors:           1,
		}
		if err := outputPoolIngestResult(res); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("text output no skips no errors", func(t *testing.T) {
		res := poolIngestResult{
			FilesScanned:    1,
			CandidatesFound: 1,
			Added:           1,
		}
		if err := outputPoolIngestResult(res); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// inferKnowledgeType
// ---------------------------------------------------------------------------

func TestInferKnowledgeType(t *testing.T) {
	tests := []struct {
		name     string
		block    learningBlock
		wantType types.KnowledgeType
	}{
		// Category exact-match cases
		{name: "category decision", block: learningBlock{Category: "decision", Body: "some text"}, wantType: types.KnowledgeTypeDecision},
		{name: "category pattern", block: learningBlock{Category: "pattern", Body: "some text"}, wantType: types.KnowledgeTypeDecision},
		{name: "category architectural-decision", block: learningBlock{Category: "architectural-decision", Body: "x"}, wantType: types.KnowledgeTypeDecision},
		{name: "category convention", block: learningBlock{Category: "convention", Body: "x"}, wantType: types.KnowledgeTypeDecision},
		{name: "category failure", block: learningBlock{Category: "failure", Body: "x"}, wantType: types.KnowledgeTypeFailure},
		{name: "category anti-pattern", block: learningBlock{Category: "anti-pattern", Body: "x"}, wantType: types.KnowledgeTypeFailure},
		{name: "category antipattern", block: learningBlock{Category: "antipattern", Body: "x"}, wantType: types.KnowledgeTypeFailure},
		{name: "category postmortem", block: learningBlock{Category: "postmortem", Body: "x"}, wantType: types.KnowledgeTypeFailure},
		{name: "category solution", block: learningBlock{Category: "solution", Body: "x"}, wantType: types.KnowledgeTypeSolution},
		{name: "category fix", block: learningBlock{Category: "fix", Body: "x"}, wantType: types.KnowledgeTypeSolution},
		{name: "category workaround", block: learningBlock{Category: "workaround", Body: "x"}, wantType: types.KnowledgeTypeSolution},
		{name: "category reference", block: learningBlock{Category: "reference", Body: "x"}, wantType: types.KnowledgeTypeReference},
		{name: "category doc", block: learningBlock{Category: "doc", Body: "x"}, wantType: types.KnowledgeTypeReference},
		{name: "category documentation", block: learningBlock{Category: "documentation", Body: "x"}, wantType: types.KnowledgeTypeReference},
		// Category case-insensitive
		{name: "category Decision uppercase", block: learningBlock{Category: "Decision", Body: "x"}, wantType: types.KnowledgeTypeDecision},
		{name: "category FAILURE uppercase", block: learningBlock{Category: "FAILURE", Body: "x"}, wantType: types.KnowledgeTypeFailure},
		// Signal-based: decision (needs >= 2 signals)
		{name: "decision signals 2", block: learningBlock{Category: "", Body: "we always prefer convention over configuration"}, wantType: types.KnowledgeTypeDecision},
		{name: "decision signals 1 not enough", block: learningBlock{Category: "", Body: "we always use X"}, wantType: types.KnowledgeTypeLearning},
		// Signal-based: failure (needs >= 2 signals)
		{name: "failure signals 2", block: learningBlock{Category: "", Body: "the deploy failed and we found the root cause"}, wantType: types.KnowledgeTypeFailure},
		{name: "failure signals 1 not enough", block: learningBlock{Category: "", Body: "the deploy failed"}, wantType: types.KnowledgeTypeLearning},
		// Fallback to learning
		{name: "empty category and body", block: learningBlock{Category: "", Body: ""}, wantType: types.KnowledgeTypeLearning},
		{name: "unknown category", block: learningBlock{Category: "process", Body: "run tests first"}, wantType: types.KnowledgeTypeLearning},
		{name: "generic content", block: learningBlock{Category: "", Body: "use go test before commit"}, wantType: types.KnowledgeTypeLearning},
		// Category takes priority over signals
		{name: "category overrides signals", block: learningBlock{Category: "reference", Body: "we always prefer this convention"}, wantType: types.KnowledgeTypeReference},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferKnowledgeType(tt.block)
			if got != tt.wantType {
				t.Errorf("inferKnowledgeType(%q, %q) = %q, want %q", tt.block.Category, tt.block.Body, got, tt.wantType)
			}
		})
	}
}
