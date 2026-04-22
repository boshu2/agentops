package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/harvest"
	"github.com/boshu2/agentops/cli/internal/pool"
	"github.com/boshu2/agentops/cli/internal/types"
)

// TestFlywheelE2E_CreateHarvestPromoteRetrieveInject validates the full flywheel loop:
// create learning → harvest extract → catalog + promote → retrieve → quality gate.
func TestFlywheelE2E_CreateHarvestPromoteRetrieveInject(t *testing.T) {
	fixture := setupFlywheelE2ECanary(t)
	artifacts := extractFlywheelE2ECanaryArtifacts(t, fixture)
	assertFlywheelE2ECanaryArtifact(t, artifacts[0])
	promotedPath := promoteFlywheelE2ECanaryArtifact(t, fixture, artifacts)

	assertFlywheelE2EPromotedMetadata(t, promotedPath)
	assertFlywheelE2EOriginalLearningRetrievable(t, fixture.learningFile)
	assertFlywheelE2EQualityGate(t, fixture.learningFile)
}

type flywheelE2ECanaryFixture struct {
	rigBase      string
	learningFile string
	promoteDir   string
}

func setupFlywheelE2ECanary(t *testing.T) flywheelE2ECanaryFixture {
	t.Helper()

	t.Setenv("HOME", t.TempDir()) // isolate HOME per Wave 1 check-home-isolation gate
	tmpDir := t.TempDir()
	rigBase := filepath.Join(tmpDir, "testproject", "crew", "testcrew")
	learningsDir := filepath.Join(rigBase, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatalf("creating learnings dir: %v", err)
	}

	learningContent := `---
type: learning
maturity: established
utility: 0.8
confidence: high
date: 2026-04-02
---

# Learning: Flywheel E2E Canary Validation

This is a canary learning created specifically for end-to-end flywheel validation.
It contains the unique token flywheel-canary-e2e-test that retrieval should match on.
The content is long enough to pass the quality gate minimum of 50 characters.
`
	learningFile := filepath.Join(learningsDir, "2026-04-02-flywheel-canary.md")
	if err := os.WriteFile(learningFile, []byte(learningContent), 0o644); err != nil {
		t.Fatalf("writing learning file: %v", err)
	}

	return flywheelE2ECanaryFixture{
		rigBase:      rigBase,
		learningFile: learningFile,
		promoteDir:   filepath.Join(tmpDir, "global-learnings"),
	}
}

func extractFlywheelE2ECanaryArtifacts(t *testing.T, fixture flywheelE2ECanaryFixture) []harvest.Artifact {
	t.Helper()

	rig := harvest.RigInfo{
		Path:    filepath.Join(fixture.rigBase, ".agents"),
		Project: "testproject",
		Crew:    "testcrew",
		Rig:     "testproject-testcrew",
	}
	opts := harvest.WalkOptions{
		MaxFileSize: 1048576,
		IncludeDirs: []string{"learnings"},
	}

	artifacts, warnings := harvest.ExtractArtifacts(rig, opts)
	if len(warnings) != 0 {
		t.Fatalf("ExtractArtifacts warnings: %#v", warnings)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(artifacts))
	}

	return artifacts
}

func assertFlywheelE2ECanaryArtifact(t *testing.T, art harvest.Artifact) {
	t.Helper()

	if art.Type != "learning" {
		t.Errorf("expected type=learning, got %q", art.Type)
	}
	// "high" string → 0.9 via toFloat64WithDefault
	if art.Confidence != 0.9 {
		t.Errorf("expected confidence=0.9 (from 'high'), got %g", art.Confidence)
	}
	if art.SourceRig != "testproject-testcrew" {
		t.Errorf("expected source_rig=testproject-testcrew, got %q", art.SourceRig)
	}
}

func promoteFlywheelE2ECanaryArtifact(t *testing.T, fixture flywheelE2ECanaryFixture, artifacts []harvest.Artifact) string {
	t.Helper()

	catalog := harvest.BuildCatalog(artifacts, 0.5)
	if len(catalog.Promoted) != 1 {
		t.Fatalf("expected 1 promoted artifact, got %d", len(catalog.Promoted))
	}

	if err := os.MkdirAll(fixture.promoteDir, 0o755); err != nil {
		t.Fatalf("creating promotion dir: %v", err)
	}

	promoted, err := harvest.Promote(catalog, fixture.promoteDir, false)
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if promoted != 1 {
		t.Fatalf("expected 1 promoted file, got %d", promoted)
	}

	// Find the promoted file
	promotedFiles, err := filepath.Glob(filepath.Join(fixture.promoteDir, "learning", "*.md"))
	if err != nil || len(promotedFiles) == 0 {
		t.Fatalf("no promoted files found in %s/learning/", fixture.promoteDir)
	}

	return promotedFiles[0]
}

func assertFlywheelE2EPromotedMetadata(t *testing.T, promotedPath string) {
	t.Helper()

	promotedContent, err := os.ReadFile(promotedPath)
	if err != nil {
		t.Fatalf("reading promoted file: %v", err)
	}
	pc := string(promotedContent)

	// Verify metadata preservation in promoted file
	if !strings.Contains(pc, "promoted_from:") {
		t.Error("promoted file missing promoted_from header")
	}
	if !strings.Contains(pc, "maturity: established") {
		t.Error("promoted file lost maturity metadata")
	}
	if !strings.Contains(pc, "utility: 0.8") {
		t.Error("promoted file lost utility metadata")
	}
}

func assertFlywheelE2EOriginalLearningRetrievable(t *testing.T, learningFile string) {
	t.Helper()

	// Note: promoted files are intentionally skipped by inject (isPromoted → Superseded=true)
	// to avoid double-counting. The inject pipeline reads local .agents/learnings/, not the
	// global promoted store. So we validate retrieval against the original source file.
	tokens := queryTokens(strings.ToLower("flywheel canary e2e"))
	now := time.Now()
	l, ok := processLearningFile(learningFile, tokens, now)
	if !ok {
		t.Fatalf("processLearningFile returned false for canary learning (maturity=%s, utility=%.3f, body=%d chars)",
			l.Maturity, l.Utility, len(l.BodyText))
	}
	if l.Title == "" {
		t.Error("parsed learning has empty title")
	}
}

func assertFlywheelE2EQualityGate(t *testing.T, learningFile string) {
	t.Helper()

	// Note: learnings without source_bead get a 0.3x utility penalty in processLearningFile,
	// so we check the gate BEFORE that penalty (which is applied during scoring, not parsing).
	// The quality gate itself checks the raw parsed values.
	rawL, err := parseLearningFile(learningFile)
	if err != nil {
		t.Fatalf("parseLearningFile: %v", err)
	}
	if !passesQualityGate(rawL) {
		t.Errorf("canary learning failed quality gate (maturity=%s, utility=%.3f)", rawL.Maturity, rawL.Utility)
	}
}

// TestFlywheelE2E_StringConfidenceMapping validates that string confidence values
// survive through the harvest pipeline: "high"→0.9, "medium"→0.6, "low"→0.3.
func TestFlywheelE2E_StringConfidenceMapping(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate HOME per Wave 1 check-home-isolation gate
	tmpDir := t.TempDir()
	rigBase := filepath.Join(tmpDir, "conftest", "crew", "confcrew")
	learningsDir := filepath.Join(rigBase, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatalf("creating dir: %v", err)
	}

	cases := []struct {
		name       string
		confidence string
		expected   float64
	}{
		{"high", "high", 0.9},
		{"medium", "medium", 0.6},
		{"low", "low", 0.3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			content := "---\ntype: learning\nconfidence: " + tc.confidence + "\nmaturity: provisional\nutility: 0.5\ndate: 2026-04-02\n---\n\n# Confidence Test: " + tc.name + "\n\nThis learning tests that string confidence values map correctly through harvest.\n"
			file := filepath.Join(learningsDir, "2026-04-02-conf-"+tc.name+".md")
			if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
				t.Fatalf("writing file: %v", err)
			}
		})
	}

	rig := harvest.RigInfo{
		Path:    filepath.Join(rigBase, ".agents"),
		Project: "conftest",
		Crew:    "confcrew",
		Rig:     "conftest-confcrew",
	}
	opts := harvest.WalkOptions{
		MaxFileSize: 1048576,
		IncludeDirs: []string{"learnings"},
	}

	artifacts, warnings := harvest.ExtractArtifacts(rig, opts)
	if len(warnings) != 0 {
		t.Fatalf("ExtractArtifacts warnings: %#v", warnings)
	}
	if len(artifacts) != 3 {
		t.Fatalf("expected 3 artifacts, got %d", len(artifacts))
	}

	for _, art := range artifacts {
		var expected float64
		switch {
		case strings.Contains(art.Title, "high"):
			expected = 0.9
		case strings.Contains(art.Title, "medium"):
			expected = 0.6
		case strings.Contains(art.Title, "low"):
			expected = 0.3
		default:
			t.Errorf("unexpected artifact title: %s", art.Title)
			continue
		}
		if art.Confidence != expected {
			t.Errorf("artifact %q: expected confidence=%g, got %g", art.Title, expected, art.Confidence)
		}
	}
}

// TestFlywheelE2E_MetadataPreservation verifies maturity and utility survive
// the full harvest → promote pipeline without being stripped or defaulted.
func TestFlywheelE2E_MetadataPreservation(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate HOME per Wave 1 check-home-isolation gate
	tmpDir := t.TempDir()
	rigBase := filepath.Join(tmpDir, "metarig", "crew", "metacrew")
	learningsDir := filepath.Join(rigBase, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatalf("creating dir: %v", err)
	}

	content := `---
type: learning
maturity: candidate
utility: 0.75
confidence: 0.85
date: 2026-04-02
source_bead: ag-test-123
---

# Learning: Metadata Preservation Canary

This learning has specific maturity=candidate and utility=0.75 values
that must survive through harvest extraction and promotion to the global store.
`
	file := filepath.Join(learningsDir, "2026-04-02-meta-canary.md")
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	rig := harvest.RigInfo{
		Path:    filepath.Join(rigBase, ".agents"),
		Project: "metarig",
		Crew:    "metacrew",
		Rig:     "metarig-metacrew",
	}
	artifacts, warnings := harvest.ExtractArtifacts(rig, harvest.WalkOptions{
		MaxFileSize: 1048576,
		IncludeDirs: []string{"learnings"},
	})
	if len(warnings) != 0 {
		t.Fatalf("ExtractArtifacts warnings: %#v", warnings)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(artifacts))
	}
	if artifacts[0].Confidence != 0.85 {
		t.Errorf("extraction lost confidence: expected 0.85, got %g", artifacts[0].Confidence)
	}

	catalog := harvest.BuildCatalog(artifacts, 0.5)
	promoteDir := filepath.Join(tmpDir, "meta-global")
	promoted, err := harvest.Promote(catalog, promoteDir, false)
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if promoted != 1 {
		t.Fatalf("expected 1 promoted, got %d", promoted)
	}

	promotedFiles, _ := filepath.Glob(filepath.Join(promoteDir, "learning", "*.md"))
	if len(promotedFiles) == 0 {
		t.Fatal("no promoted files found")
	}
	body, _ := os.ReadFile(promotedFiles[0])
	text := string(body)

	if !strings.Contains(text, "maturity: candidate") {
		t.Error("promoted file lost maturity=candidate")
	}
	if !strings.Contains(text, "utility: 0.75") {
		t.Error("promoted file lost utility=0.75")
	}
	if !strings.Contains(text, "source_bead: ag-test-123") {
		t.Error("promoted file lost source_bead")
	}
}

// TestFlywheelE2E_GarbageRejection verifies that low-quality learnings are
// rejected by the quality gate: short body, missing metadata, low utility.
func TestFlywheelE2E_GarbageRejection(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate HOME per Wave 1 check-home-isolation gate
	cases := []struct {
		name     string
		learning learning
		passes   bool
	}{
		{
			name: "short body rejected",
			learning: learning{
				Title:    "Too Short",
				Maturity: "established",
				Utility:  0.8,
				BodyText: "tiny",
			},
			passes: false,
		},
		{
			name: "draft maturity rejected",
			learning: learning{
				Title:    "Draft Learning",
				Maturity: "draft",
				Utility:  0.8,
				BodyText: "This is a long enough body text that should pass the length check easily.",
			},
			passes: false,
		},
		{
			name: "low utility rejected",
			learning: learning{
				Title:    "Low Utility",
				Maturity: "established",
				Utility:  0.2,
				BodyText: "This learning has low utility and should be filtered out by the quality gate.",
			},
			passes: false,
		},
		{
			name: "good learning passes",
			learning: learning{
				Title:    "Good Learning",
				Maturity: "established",
				Utility:  0.8,
				BodyText: "This is a well-formed learning with sufficient body text and proper metadata values.",
			},
			passes: true,
		},
		{
			name: "empty maturity defaults to provisional and passes",
			learning: learning{
				Title:    "Legacy Learning",
				Maturity: "",
				Utility:  0.8,
				BodyText: "This legacy learning has no maturity set but should default to provisional and pass.",
			},
			passes: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := passesQualityGate(tc.learning)
			if got != tc.passes {
				t.Errorf("passesQualityGate(%q): got %v, want %v (maturity=%s, utility=%.2f, body=%d chars)",
					tc.name, got, tc.passes, tc.learning.Maturity, tc.learning.Utility, len(tc.learning.BodyText))
			}
		})
	}
}

// TestFlywheelE2E_CitationPromotionPipeline validates that the signal-based
// promotion criteria work end-to-end: a candidate with sufficient citations
// AND utility promotes, while candidates lacking either are rejected.
func TestFlywheelE2E_CitationPromotionPipeline(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate HOME per Wave 1 check-home-isolation gate
	minAge := 24 * time.Hour

	cases := []struct {
		name       string
		entry      pool.PoolEntry
		citations  map[string]int
		wantPass   bool
		wantReason string // substring expected in skip reason
	}{
		{
			name: "full pipeline pass — 3 citations, utility 0.7",
			entry: pool.PoolEntry{
				PoolEntry: types.PoolEntry{
					Candidate: types.Candidate{
						ID:      "pipeline-pass",
						Content: "learning with strong signal from multiple sessions",
						Utility: 0.7,
					},
				},
				Age:       72 * time.Hour,
				AgeString: "72h",
			},
			citations: map[string]int{"pipeline-pass": 3},
			wantPass:  true,
		},
		{
			name: "rejected — 1 citation below minimum 2",
			entry: pool.PoolEntry{
				PoolEntry: types.PoolEntry{
					Candidate: types.Candidate{
						ID:      "pipeline-low-cite",
						Content: "learning with only one citation",
						Utility: 0.8,
					},
				},
				Age:       72 * time.Hour,
				AgeString: "72h",
			},
			citations:  map[string]int{"pipeline-low-cite": 1},
			wantPass:   false,
			wantReason: "insufficient citations",
		},
		{
			name: "rejected — utility 0.3 below 0.5 threshold",
			entry: pool.PoolEntry{
				PoolEntry: types.PoolEntry{
					Candidate: types.Candidate{
						ID:      "pipeline-low-util",
						Content: "learning with low utility despite citations",
						Utility: 0.3,
					},
				},
				Age:       72 * time.Hour,
				AgeString: "72h",
			},
			citations:  map[string]int{"pipeline-low-util": 5},
			wantPass:   false,
			wantReason: "utility too low",
		},
		{
			name: "rejected — both citations and utility insufficient",
			entry: pool.PoolEntry{
				PoolEntry: types.PoolEntry{
					Candidate: types.Candidate{
						ID:      "pipeline-both-low",
						Content: "learning with no signal at all",
						Utility: 0.2,
					},
				},
				Age:       72 * time.Hour,
				AgeString: "72h",
			},
			citations:  map[string]int{},
			wantPass:   false,
			wantReason: "insufficient citations", // citations checked first
		},
		{
			name: "boundary — exactly 2 citations and utility 0.5",
			entry: pool.PoolEntry{
				PoolEntry: types.PoolEntry{
					Candidate: types.Candidate{
						ID:      "pipeline-boundary",
						Content: "learning at exact promotion boundary",
						Utility: 0.5,
					},
				},
				Age:       72 * time.Hour,
				AgeString: "72h",
			},
			citations: map[string]int{"pipeline-boundary": 2},
			wantPass:  true,
		},
		{
			name: "boundary — utility 0.49 just below threshold",
			entry: pool.PoolEntry{
				PoolEntry: types.PoolEntry{
					Candidate: types.Candidate{
						ID:      "pipeline-just-under",
						Content: "learning just below utility threshold",
						Utility: 0.49,
					},
				},
				Age:       72 * time.Hour,
				AgeString: "72h",
			},
			citations:  map[string]int{"pipeline-just-under": 5},
			wantPass:   false,
			wantReason: "utility too low",
		},
	}

	promoted := map[string]bool{}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reason := checkPromotionCriteria("/tmp", tc.entry, minAge, tc.citations, promoted, true)
			if tc.wantPass {
				if reason != "" {
					t.Errorf("expected promotion, got skip: %q", reason)
				}
			} else {
				if reason == "" {
					t.Errorf("expected rejection containing %q, got promotion", tc.wantReason)
				} else if !strings.Contains(reason, tc.wantReason) {
					t.Errorf("skip reason %q does not contain %q", reason, tc.wantReason)
				}
			}
		})
	}
}
