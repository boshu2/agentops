package llm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func makeNote(idx int, intent, summary string, entities []string) ChunkNote {
	return ChunkNote{
		Index:              idx,
		Intent:             intent,
		Summary:            summary,
		Entities:           entities,
		AssistantCondensed: "assistant did the thing",
	}
}

func TestSessionPage_Render_IncludesFullFrontmatter(t *testing.T) {
	page := SessionPage{
		Frontmatter: SessionFrontmatter{
			Type:          "session",
			SessionID:     "abc123-session",
			SourceJSONL:   "/fixture/path.jsonl",
			Workspace:     "/Users/bo/repo",
			StartedAt:     time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC),
			EndedAt:       time.Date(2026, 4, 11, 13, 0, 0, 0, time.UTC),
			Turns:         5,
			TokensIn:      1000,
			TokensOut:     500,
			ModelPrimary:  "claude-sonnet-4-6",
			Status:        "draft",
			Tier:          1,
			Model:         "gemma2:9b",
			ModelDigest:   "sha256:ff02c3",
			Confidence:    0.75,
			IngestedAt:    time.Date(2026, 4, 11, 14, 0, 0, 0, time.UTC),
			IngestedBy:    "ao-forge-tier1",
			ParentSession: "",
			FirstSeen:     time.Date(2026, 4, 11, 14, 0, 0, 0, time.UTC),
		},
		Notes: []ChunkNote{
			makeNote(0, "fix bug", "fixed a bug", []string{"file:/a.go"}),
		},
	}
	out := page.Render()

	// Frontmatter delimiters
	if !strings.HasPrefix(out, "---\n") {
		t.Errorf("output should start with --- frontmatter delimiter")
	}

	// Every one of the 17 required fields must be present.
	required := []string{
		"type: session",
		"session_id: abc123-session",
		"source_jsonl: /fixture/path.jsonl",
		"workspace: /Users/bo/repo",
		"started_at: 2026-04-11T12:00:00Z",
		"ended_at: 2026-04-11T13:00:00Z",
		"turns: 5",
		"tokens_in: 1000",
		"tokens_out: 500",
		"model_primary: claude-sonnet-4-6",
		"status: draft",
		"tier: 1",
		"model: gemma2:9b",
		"model_digest: sha256:ff02c3",
		"confidence: 0.75",
		"ingested_at: 2026-04-11T14:00:00Z",
		"ingested_by: ao-forge-tier1",
		"first_seen: 2026-04-11T14:00:00Z",
	}
	for _, f := range required {
		if !strings.Contains(out, f) {
			t.Errorf("frontmatter missing %q\noutput:\n%s", f, out)
		}
	}

	// Chunk note should render
	if !strings.Contains(out, "### fix bug") && !strings.Contains(out, "fix bug") {
		t.Errorf("rendered output missing chunk intent")
	}
	if !strings.Contains(out, "[[file:/a.go]]") {
		t.Errorf("rendered output missing entity wikilink")
	}
}

func TestSessionPage_Render_SkipsEmptyOptionalFields(t *testing.T) {
	// parent_session, reviewed_at, reviewed_by are optional — when empty they
	// should be omitted from the frontmatter (not rendered as ""/0).
	page := SessionPage{
		Frontmatter: SessionFrontmatter{
			Type:         "session",
			SessionID:    "x",
			SourceJSONL:  "/x.jsonl",
			Workspace:    "/x",
			StartedAt:    time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
			EndedAt:      time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
			Turns:        1,
			TokensIn:     1,
			TokensOut:    1,
			ModelPrimary: "claude",
			Status:       "draft",
			Tier:         1,
			Model:        "gemma2:9b",
			ModelDigest:  "sha256:x",
			Confidence:   0.5,
			IngestedAt:   time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
			IngestedBy:   "ao-forge-tier1",
			FirstSeen:    time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
		},
	}
	out := page.Render()
	if strings.Contains(out, "parent_session:") {
		t.Errorf("empty parent_session should be omitted")
	}
	if strings.Contains(out, "reviewed_at:") {
		t.Errorf("empty reviewed_at should be omitted")
	}
	if strings.Contains(out, "reviewed_by:") {
		t.Errorf("empty reviewed_by should be omitted")
	}
}

func TestWriteSessionPage_AtomicRename(t *testing.T) {
	dir := t.TempDir()
	page := SessionPage{
		Frontmatter: SessionFrontmatter{
			Type:         "session",
			SessionID:    "atomic-test",
			SourceJSONL:  "/x.jsonl",
			Workspace:    "/x",
			StartedAt:    time.Now().UTC(),
			EndedAt:      time.Now().UTC(),
			Turns:        1,
			ModelPrimary: "claude",
			Status:       "draft",
			Tier:         1,
			Model:        "gemma2:9b",
			ModelDigest:  "sha256:x",
			Confidence:   0.5,
			IngestedAt:   time.Now().UTC(),
			IngestedBy:   "ao-forge-tier1",
			FirstSeen:    time.Now().UTC(),
		},
	}
	path := filepath.Join(dir, "atomic-test.md")
	if err := WriteSessionPage(path, page); err != nil {
		t.Fatalf("WriteSessionPage: %v", err)
	}
	// File should exist and contain the frontmatter.
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(b), "session_id: atomic-test") {
		t.Errorf("written file missing session_id")
	}
	// No temp file should be left behind.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
}

func TestWriteSessionPage_OverwriteAtomically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "overwrite.md")
	if err := os.WriteFile(path, []byte("old content"), 0644); err != nil {
		t.Fatalf("pre-write: %v", err)
	}
	page := SessionPage{
		Frontmatter: SessionFrontmatter{
			Type:         "session",
			SessionID:    "overwrite-test",
			SourceJSONL:  "/x.jsonl",
			Workspace:    "/x",
			StartedAt:    time.Now().UTC(),
			EndedAt:      time.Now().UTC(),
			Turns:        1,
			ModelPrimary: "claude",
			Status:       "draft",
			Tier:         1,
			Model:        "gemma2:9b",
			ModelDigest:  "sha256:x",
			Confidence:   0.5,
			IngestedAt:   time.Now().UTC(),
			IngestedBy:   "ao-forge-tier1",
			FirstSeen:    time.Now().UTC(),
		},
	}
	if err := WriteSessionPage(path, page); err != nil {
		t.Fatalf("WriteSessionPage: %v", err)
	}
	b, _ := os.ReadFile(path)
	if strings.Contains(string(b), "old content") {
		t.Errorf("overwrite failed, old content still present")
	}
	if !strings.Contains(string(b), "overwrite-test") {
		t.Errorf("new content missing session_id")
	}
}

func TestBuildPage_FromChunksAndNotes(t *testing.T) {
	notes := []ChunkNote{
		makeNote(0, "first", "first summary", []string{"file:/a.go"}),
		{Index: 1, Skipped: true},
		makeNote(2, "third", "third summary", []string{"bead:na-1"}),
	}
	meta := SessionMeta{
		SessionID:    "m1",
		SourceJSONL:  "/fix.jsonl",
		Workspace:    "/w",
		StartedAt:    time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
		EndedAt:      time.Date(2026, 4, 11, 1, 0, 0, 0, time.UTC),
		Turns:        3,
		TokensIn:     100,
		TokensOut:    50,
		ModelPrimary: "claude",
		Model:        "gemma2:9b",
		ModelDigest:  "sha256:d",
		IngestedBy:   "ao-forge-tier1",
	}
	page := BuildPage(meta, notes)
	if page.Frontmatter.Status != "draft" {
		t.Errorf("Status should default to 'draft', got %q", page.Frontmatter.Status)
	}
	if page.Frontmatter.Tier != 1 {
		t.Errorf("Tier should default to 1, got %d", page.Frontmatter.Tier)
	}
	if page.Frontmatter.Confidence <= 0 || page.Frontmatter.Confidence > 1 {
		t.Errorf("Confidence should be in (0,1], got %v", page.Frontmatter.Confidence)
	}
	// 1 of 3 chunks was skipped, so non-skipped count is 2. Confidence
	// should reflect that (skipped chunks lower confidence).
	if len(page.Notes) != len(notes) {
		t.Errorf("Notes: want %d, got %d", len(notes), len(page.Notes))
	}
}
