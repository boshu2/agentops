package ratchet

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDraftSlug(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"strips date prefix", "/tmp/2026-04-09-test-pattern.md", "test"},
		{"strips -pattern suffix", "/tmp/2026-01-01-my-cool-pattern.md", "my-cool"},
		{"no date prefix", "/tmp/my-skill.md", "my-skill"},
		{"underscores to hyphens", "/tmp/2026-01-01-my_cool_thing.md", "my-cool-thing"},
		{"lowercases", "/tmp/2026-01-01-MyPattern.md", "mypattern"},
		{"pattern alone stays", "/tmp/2026-01-01-pattern.md", "pattern"},
		{"actually empty", "/tmp/2026-01-01-.md", "generated-skill-draft"},
		{"plain name", "/tmp/testing.md", "testing"},
		{"trims leading/trailing hyphens", "/tmp/2026-01-01--extra--pattern.md", "extra"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := draftSlug(tt.path)
			if got != tt.want {
				t.Errorf("draftSlug(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestSuggestDraftTier(t *testing.T) {
	tests := []struct {
		slug string
		want string
	}{
		{"research-methods", "knowledge"},
		{"knowledge-base", "knowledge"},
		{"trace-analysis", "knowledge"},
		{"status-report", "session"},
		{"handoff-protocol", "session"},
		{"recover-state", "session"},
		{"release-process", "product"},
		{"readme-update", "product"},
		{"product-launch", "product"},
		{"build-optimization", "execution"},
		{"testing-harness", "execution"},
		{"unknown-thing", "execution"},
	}
	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			got := suggestDraftTier(tt.slug)
			if got != tt.want {
				t.Errorf("suggestDraftTier(%q) = %q, want %q", tt.slug, got, tt.want)
			}
		})
	}
}

func TestRenderSkillDraft(t *testing.T) {
	got := renderSkillDraft("/tmp/patterns/test.md", "test-skill", "execution")
	if !strings.Contains(got, "name: test-skill") {
		t.Error("missing skill name in frontmatter")
	}
	if !strings.Contains(got, "tier: execution") {
		t.Error("missing tier in frontmatter")
	}
	if !strings.Contains(got, "# test-skill") {
		t.Error("missing heading")
	}
	if !strings.Contains(got, "/tmp/patterns/test.md") {
		t.Error("missing pattern path reference")
	}
	if !strings.Contains(got, "skill_api_version: 1") {
		t.Error("missing skill_api_version")
	}
}

func TestSkillDraftResult_EmptyPatterns(t *testing.T) {
	baseDir := t.TempDir()
	patternsDir := filepath.Join(baseDir, ".agents", "patterns")
	if err := os.MkdirAll(patternsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := GenerateSkillDrafts(baseDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Evaluated != 0 || result.Generated != 0 {
		t.Errorf("expected zero evaluated/generated for empty patterns dir, got %+v", result)
	}
}

func TestGenerateSkillDrafts_RequiresThreeSessionRefs(t *testing.T) {
	baseDir := t.TempDir()
	patternsDir := filepath.Join(baseDir, ".agents", "patterns")
	sessionsDir := filepath.Join(baseDir, ".agents", "ao", "sessions")
	if err := os.MkdirAll(patternsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	patternPath := filepath.Join(patternsDir, "2026-04-09-test-pattern.md")
	if err := os.WriteFile(patternPath, []byte("---\nconfidence: 0.9\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	writeSessionRef := func(name string) {
		t.Helper()
		path := filepath.Join(sessionsDir, name)
		if err := os.WriteFile(path, []byte("used 2026-04-09-test-pattern.md\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeSessionRef("sess1.jsonl")
	writeSessionRef("sess2.md")

	result, err := GenerateSkillDrafts(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	if result.Generated != 0 {
		t.Fatalf("expected no drafts with two session refs, got %+v", result)
	}

	writeSessionRef("sess3.jsonl")

	result, err = GenerateSkillDrafts(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	if result.Generated != 1 {
		t.Fatalf("expected one draft with three session refs, got %+v", result)
	}
	if len(result.Paths) != 1 {
		t.Fatalf("expected one draft path, got %+v", result.Paths)
	}

	draftPath := result.Paths[0]
	data, err := os.ReadFile(draftPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "name: test") {
		t.Fatalf("expected draft frontmatter name to derive from pattern slug, got:\n%s", text)
	}
	if !strings.Contains(text, "Draft generated from recurring pattern evidence") {
		t.Fatalf("expected generated draft description, got:\n%s", text)
	}

	evidencePath := filepath.Join(filepath.Dir(draftPath), "evidence.json")
	evidenceBytes, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}

	var evidence skillDraftEvidence
	if err := json.Unmarshal(evidenceBytes, &evidence); err != nil {
		t.Fatal(err)
	}
	if evidence.SessionRefs != 3 {
		t.Fatalf("expected session_refs=3, got %+v", evidence)
	}
	if evidence.PatternPath != patternPath {
		t.Fatalf("expected evidence pattern path %q, got %+v", patternPath, evidence)
	}
}
