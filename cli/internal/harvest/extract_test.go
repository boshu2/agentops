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

	artifacts, warnings := ExtractArtifacts(rig, opts)
	if len(warnings) != 0 {
		t.Fatalf("ExtractArtifacts warnings = %#v, want none", warnings)
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

	artifacts, warnings := ExtractArtifacts(rig, opts)
	if len(warnings) != 0 {
		t.Fatalf("ExtractArtifacts warnings = %#v, want none", warnings)
	}

	if len(artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(artifacts))
	}

	if artifacts[0].Confidence != 0.5 {
		t.Errorf("confidence = %v, want 0.5 (default)", artifacts[0].Confidence)
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

	artifacts, warnings := ExtractArtifacts(rig, opts)
	if len(warnings) != 0 {
		t.Fatalf("ExtractArtifacts warnings = %#v, want none", warnings)
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

	artifacts, warnings := ExtractArtifacts(rig, opts)
	if len(warnings) != 0 {
		t.Fatalf("ExtractArtifacts warnings = %#v, want none", warnings)
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

func TestExtractArtifacts_ContinuesAfterMalformedFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	agentsDir := filepath.Join(tmp, ".agents")
	learningsDir := filepath.Join(agentsDir, "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	validContent := `---
title: Good Learning
confidence: 0.8
---

# Good Learning

This file should still be harvested even when another file is malformed.
`
	if err := os.WriteFile(filepath.Join(learningsDir, "2026-04-10-good.md"), []byte(validContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Truly malformed: tab character in indentation is unsalvageable.
	// (Mid-value colon cases like "Broken: value" are now salvaged by
	// salvageFrontmatterYAML — see TestParseFrontmatter_SalvagesMidValueColons.)
	invalidContent := "---\nkey:\n\tvalue_with_tab: broken\n---\n"
	if err := os.WriteFile(filepath.Join(learningsDir, "2026-04-10-bad.md"), []byte(invalidContent), 0o644); err != nil {
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

	artifacts, warnings := ExtractArtifacts(rig, opts)
	if len(artifacts) != 1 {
		t.Fatalf("expected 1 valid artifact, got %d", len(artifacts))
	}
	if artifacts[0].Title != "Good Learning" {
		t.Fatalf("unexpected artifact title %q", artifacts[0].Title)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %#v", warnings)
	}
	if warnings[0].Stage != "parse_frontmatter" {
		t.Fatalf("warning stage = %q, want parse_frontmatter", warnings[0].Stage)
	}
	if !strings.Contains(warnings[0].Path, "2026-04-10-bad.md") {
		t.Fatalf("warning path = %q, want bad.md", warnings[0].Path)
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
			name:     "string confidence high maps to 0.9",
			input:    map[string]any{"confidence": "high"},
			wantConf: 0.9,
			hasConf:  true,
		},
		{
			name:     "string confidence medium maps to 0.6",
			input:    map[string]any{"confidence": "medium"},
			wantConf: 0.6,
			hasConf:  true,
		},
		{
			name:     "string confidence low maps to 0.3",
			input:    map[string]any{"confidence": "low"},
			wantConf: 0.3,
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

// TestParseFrontmatter_SalvagesMidValueColons guards the 2026-04-15
// regression: authoring mistake of an unquoted scalar whose value contains
// ": " (colon-space) causes yaml to read the second key as a nested
// mapping. Harvest must salvage by quoting the scalar and retrying.
func TestParseFrontmatter_SalvagesMidValueColons(t *testing.T) {
	cases := []struct {
		name    string
		content string
		wantKey string
		wantVal string
	}{
		{
			name: "description with Solution: mid-value",
			content: `---
name: geoint-edge — bake GDIT CA into mountpoint-s3 image
description: On edge the CSI driver can't mount https://s3.gdit.demo. Solution: layer the CA into the trust store.
---

Body here.`,
			wantKey: "description",
			wantVal: "On edge the CSI driver can't mount https://s3.gdit.demo. Solution: layer the CA into the trust store.",
		},
		{
			name: "name with em-dash and mid-value colon",
			content: `---
name: foo bar — Note: this is a name
description: short
---

Body.`,
			wantKey: "name",
			wantVal: "foo bar — Note: this is a name",
		},
		{
			name: "multiple lines with embedded colons",
			content: `---
name: a: b
description: c: d
---

Body.`,
			wantKey: "name",
			wantVal: "a: b",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fm, _, err := parseFrontmatter(tc.content)
			if err != nil {
				t.Fatalf("parseFrontmatter unexpected error: %v", err)
			}
			got, ok := fm[tc.wantKey].(string)
			if !ok {
				t.Fatalf("key %q missing or not string in %+v", tc.wantKey, fm)
			}
			if got != tc.wantVal {
				t.Errorf("fm[%q] = %q, want %q", tc.wantKey, got, tc.wantVal)
			}
		})
	}
}

// TestParseFrontmatter_ValidDocIsUnchanged ensures the salvage pass does
// not rewrite a document that already parses cleanly (false-positive guard).
func TestParseFrontmatter_ValidDocIsUnchanged(t *testing.T) {
	content := `---
name: "already quoted: good"
description: plain scalar no colon
tags:
  - one
  - two
---

Body.`
	fm, _, err := parseFrontmatter(content)
	if err != nil {
		t.Fatalf("parseFrontmatter: %v", err)
	}
	if fm["name"] != "already quoted: good" {
		t.Errorf("name = %v, want 'already quoted: good'", fm["name"])
	}
	if fm["description"] != "plain scalar no colon" {
		t.Errorf("description = %v", fm["description"])
	}
	tags, ok := fm["tags"].([]any)
	if !ok || len(tags) != 2 {
		t.Errorf("tags not preserved: %v", fm["tags"])
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
