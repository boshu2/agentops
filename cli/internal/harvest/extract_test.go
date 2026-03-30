package harvest

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractArtifacts_ParsesFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	agentsDir := filepath.Join(tmp, ".agents")
	learningsDir := filepath.Join(agentsDir, "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `---
title: Retry Logic Matters
confidence: 0.9
scope: global
date: "2026-03-15"
summary: Always use exponential backoff
---
# Retry Logic Matters

When retrying HTTP calls, use exponential backoff with jitter.
`
	if err := os.WriteFile(filepath.Join(learningsDir, "2026-03-15-retry-logic.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	rig := RigInfo{
		Path:    agentsDir,
		Project: "agentops",
		Crew:    "nami",
		Rig:     "agentops-nami",
	}
	opts := WalkOptions{
		MaxFileSize: 1048576,
		IncludeDirs: []string{"learnings"},
	}

	artifacts, err := ExtractArtifacts(rig, opts)
	if err != nil {
		t.Fatalf("ExtractArtifacts failed: %v", err)
	}

	if len(artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(artifacts))
	}

	a := artifacts[0]
	if a.Title != "Retry Logic Matters" {
		t.Errorf("title = %q, want %q", a.Title, "Retry Logic Matters")
	}
	if a.Confidence != 0.9 {
		t.Errorf("confidence = %v, want 0.9", a.Confidence)
	}
	if a.Scope != "global" {
		t.Errorf("scope = %q, want %q", a.Scope, "global")
	}
	if a.Date != "2026-03-15" {
		t.Errorf("date = %q, want %q", a.Date, "2026-03-15")
	}
	if a.Type != "learning" {
		t.Errorf("type = %q, want %q", a.Type, "learning")
	}
	if a.SourceRig != "agentops-nami" {
		t.Errorf("source_rig = %q, want %q", a.SourceRig, "agentops-nami")
	}
	if a.Summary != "Always use exponential backoff" {
		t.Errorf("summary = %q, want %q", a.Summary, "Always use exponential backoff")
	}
	if a.ID != "learning-2026-03-15-retry-logic-matters" {
		t.Errorf("id = %q, want %q", a.ID, "learning-2026-03-15-retry-logic-matters")
	}
	if a.ContentHash == "" {
		t.Error("content_hash is empty")
	}
}

func TestExtractArtifacts_DefaultConfidence(t *testing.T) {
	tmp := t.TempDir()
	agentsDir := filepath.Join(tmp, ".agents")
	patternsDir := filepath.Join(agentsDir, "patterns")
	if err := os.MkdirAll(patternsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `---
title: Circuit Breaker
date: "2026-01-10"
---
Use circuit breakers for external service calls.
`
	if err := os.WriteFile(filepath.Join(patternsDir, "circuit-breaker.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	rig := RigInfo{
		Path:    agentsDir,
		Project: "myproject",
		Crew:    "worker",
		Rig:     "myproject-worker",
	}
	opts := WalkOptions{
		MaxFileSize: 1048576,
		IncludeDirs: []string{"patterns"},
	}

	artifacts, err := ExtractArtifacts(rig, opts)
	if err != nil {
		t.Fatalf("ExtractArtifacts failed: %v", err)
	}

	if len(artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(artifacts))
	}

	if artifacts[0].Confidence != 0.3 {
		t.Errorf("confidence = %v, want 0.3 (default)", artifacts[0].Confidence)
	}
	if artifacts[0].Scope != "project:myproject" {
		t.Errorf("scope = %q, want %q", artifacts[0].Scope, "project:myproject")
	}
	if artifacts[0].Type != "pattern" {
		t.Errorf("type = %q, want %q", artifacts[0].Type, "pattern")
	}
}

func TestExtractArtifacts_SkipsLargeFiles(t *testing.T) {
	tmp := t.TempDir()
	agentsDir := filepath.Join(tmp, ".agents")
	researchDir := filepath.Join(agentsDir, "research")
	if err := os.MkdirAll(researchDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Small file (should be included).
	small := "---\ntitle: Small\n---\nSmall content.\n"
	if err := os.WriteFile(filepath.Join(researchDir, "small.md"), []byte(small), 0o644); err != nil {
		t.Fatal(err)
	}

	// Large file (should be skipped).
	large := strings.Repeat("x", 2048)
	if err := os.WriteFile(filepath.Join(researchDir, "large.md"), []byte(large), 0o644); err != nil {
		t.Fatal(err)
	}

	rig := RigInfo{
		Path:    agentsDir,
		Project: "test",
		Crew:    "test",
		Rig:     "test-test",
	}
	opts := WalkOptions{
		MaxFileSize: 1024, // 1KB limit
		IncludeDirs: []string{"research"},
	}

	artifacts, err := ExtractArtifacts(rig, opts)
	if err != nil {
		t.Fatalf("ExtractArtifacts failed: %v", err)
	}

	if len(artifacts) != 1 {
		t.Fatalf("expected 1 artifact (large file skipped), got %d", len(artifacts))
	}
	if artifacts[0].Title != "Small" {
		t.Errorf("expected small file, got title %q", artifacts[0].Title)
	}
}

func TestExtractArtifacts_ComputesContentHash(t *testing.T) {
	tmp := t.TempDir()
	agentsDir := filepath.Join(tmp, ".agents")
	learningsDir := filepath.Join(agentsDir, "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	body := "This is the body content for hashing."
	content := "---\ntitle: Hash Test\ndate: \"2026-01-01\"\n---\n" + body
	if err := os.WriteFile(filepath.Join(learningsDir, "hash-test.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	rig := RigInfo{
		Path:    agentsDir,
		Project: "test",
		Crew:    "test",
		Rig:     "test-test",
	}
	opts := WalkOptions{
		MaxFileSize: 1048576,
		IncludeDirs: []string{"learnings"},
	}

	artifacts, err := ExtractArtifacts(rig, opts)
	if err != nil {
		t.Fatalf("ExtractArtifacts failed: %v", err)
	}

	if len(artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(artifacts))
	}

	// Compute expected hash using the same normalization logic.
	// The body after frontmatter parsing will be "\nThis is the body content for hashing.\n"
	// (there's a newline after the closing ---).
	rawBody := "\n" + body
	normalized := strings.ToLower(strings.TrimSpace(rawBody))
	normalized = strings.ReplaceAll(normalized, "#", "")
	normalized = strings.ReplaceAll(normalized, "*", "")
	normalized = strings.ReplaceAll(normalized, "`", "")
	normalized = strings.ReplaceAll(normalized, "---", "")
	normalized = strings.Join(strings.Fields(normalized), " ")
	h := sha256.Sum256([]byte(normalized))
	expected := hex.EncodeToString(h[:])

	if artifacts[0].ContentHash != expected {
		t.Errorf("content_hash = %q, want %q", artifacts[0].ContentHash, expected)
	}
}

func TestNormalizeFrontmatter_StandardizesFields(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		wantType string
		wantConf float64
		hasType  bool
		hasConf  bool
	}{
		{
			name:     "category becomes type",
			input:    map[string]any{"category": "bug-fix"},
			wantType: "bug-fix",
			hasType:  true,
		},
		{
			name:     "score becomes confidence as float64",
			input:    map[string]any{"score": 85},
			wantConf: 85.0,
			hasConf:  true,
		},
		{
			name:     "existing type not overwritten by category",
			input:    map[string]any{"type": "original", "category": "replacement"},
			wantType: "original",
			hasType:  true,
		},
		{
			name:     "existing confidence not overwritten by score",
			input:    map[string]any{"confidence": 0.7, "score": 99},
			wantConf: 0.7,
			hasConf:  true,
		},
		{
			name:  "nil input returns empty map",
			input: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeFrontmatter(tt.input)
			if result == nil {
				t.Fatal("NormalizeFrontmatter returned nil")
			}

			if tt.hasType {
				got, ok := result["type"]
				if !ok {
					t.Fatal("expected 'type' field in result")
				}
				if got != tt.wantType {
					t.Errorf("type = %v, want %v", got, tt.wantType)
				}
				// category should be removed
				if _, hasCat := result["category"]; hasCat {
					t.Error("category field should be removed after normalization")
				}
			}

			if tt.hasConf {
				got, ok := result["confidence"]
				if !ok {
					t.Fatal("expected 'confidence' field in result")
				}
				gotF, ok := got.(float64)
				if !ok {
					t.Fatalf("confidence is %T, want float64", got)
				}
				if gotF != tt.wantConf {
					t.Errorf("confidence = %v, want %v", gotF, tt.wantConf)
				}
				// score should be removed
				if _, hasScore := result["score"]; hasScore {
					t.Error("score field should be removed after normalization")
				}
			}
		})
	}
}

func TestParseFrontmatter_NoDelimiters(t *testing.T) {
	content := "Just plain markdown\nWith no frontmatter."
	fm, body, err := parseFrontmatter(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fm) != 0 {
		t.Errorf("expected empty frontmatter, got %v", fm)
	}
	if body != content {
		t.Errorf("body should equal original content")
	}
}

func TestSingularType(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"learnings", "learning"},
		{"patterns", "pattern"},
		{"research", "research"},
	}
	for _, tt := range tests {
		got := singularType(tt.input)
		if got != tt.want {
			t.Errorf("singularType(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestToSlug(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"Retry Logic Matters", "retry-logic-matters"},
		{"  Multiple   Spaces  ", "multiple-spaces"},
		{"Special!@#Characters", "specialcharacters"},
	}
	for _, tt := range tests {
		got := toSlug(tt.input)
		if got != tt.want {
			t.Errorf("toSlug(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractTitle_FallbackToHeading(t *testing.T) {
	fm := map[string]any{}
	body := "\nSome intro text.\n\n# My Heading\n\nBody content."
	got := extractTitle(fm, body, "fallback.md")
	if got != "My Heading" {
		t.Errorf("extractTitle = %q, want %q", got, "My Heading")
	}
}

func TestExtractTitle_FallbackToFilename(t *testing.T) {
	fm := map[string]any{}
	body := "No headings here, just text."
	got := extractTitle(fm, body, "2026-03-15-my-doc.md")
	if got != "my-doc" {
		t.Errorf("extractTitle = %q, want %q", got, "my-doc")
	}
}
