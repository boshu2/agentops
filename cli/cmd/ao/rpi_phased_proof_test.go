// practices: [agile-manifesto, dora-metrics]
package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

// ---------------------------------------------------------------------------
// Flywheel Proof: end-to-end verification that knowledge compounds
// ---------------------------------------------------------------------------

// TestFlywheelProof_InjectRetrievesCrossSession verifies that a learning file
// seeded in one "session" (directory) is discovered and parsed in a subsequent
// call to processLearningFile, simulating cross-session retrieval.
func TestFlywheelProof_InjectRetrievesCrossSession(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Session 1: seed a learning with frontmatter
	content := "---\nutility: 0.8\nsource_bead: test-proof\nsource_phase: validate\nmaturity: provisional\n---\n# Proof Learning\n\nFlywheel compounding test content verifying that learnings persist across sessions and are retrievable.\n"
	path := filepath.Join(learningsDir, "proof-learning-1.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Session 2: process the learning (simulating inject retrieval)
	l, ok := processLearningFile(path, nil, time.Now())
	if !ok {
		t.Fatal("processLearningFile returned ok=false; expected the learning to be included")
	}

	if l.SourceBead != "test-proof" {
		t.Errorf("SourceBead = %q, want %q", l.SourceBead, "test-proof")
	}
	if l.SourcePhase != "validate" {
		t.Errorf("SourcePhase = %q, want %q", l.SourcePhase, "validate")
	}
	if l.Utility <= 0 {
		t.Errorf("Utility = %f, want positive value", l.Utility)
	}
	if l.Title != "Proof Learning" {
		t.Errorf("Title = %q, want %q", l.Title, "Proof Learning")
	}
}

// TestFlywheelProof_QualityGateFilters verifies that the quality gate penalizes
// unsourced learnings (no source_bead) per the C2.1 contract: 0.7x soft penalty.
// The penalty is 0.7x (not 0.3x) to avoid a utility cliff that would cause most
// unsourced learnings to fail the >0.3 quality gate threshold.
func TestFlywheelProof_QualityGateFilters(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Learning WITH source_bead
	sourcedContent := "---\nutility: 0.8\nsource_bead: ag-proof\nsource_phase: implement\nmaturity: provisional\n---\n# Sourced Learning\n\nThis learning has provenance and demonstrates that sourced learnings keep their utility score intact.\n"
	sourcedPath := filepath.Join(learningsDir, "sourced.md")
	if err := os.WriteFile(sourcedPath, []byte(sourcedContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Learning WITHOUT source_bead (still has maturity for quality gate)
	unsourcedContent := "---\nutility: 0.8\nmaturity: provisional\n---\n# Unsourced Learning\n\nThis learning has no provenance and should receive a utility penalty during scoring.\n"
	unsourcedPath := filepath.Join(learningsDir, "unsourced.md")
	if err := os.WriteFile(unsourcedPath, []byte(unsourcedContent), 0644); err != nil {
		t.Fatal(err)
	}

	now := time.Now()

	sourced, okS := processLearningFile(sourcedPath, nil, now)
	if !okS {
		t.Fatal("processLearningFile returned ok=false for sourced learning")
	}

	unsourced, okU := processLearningFile(unsourcedPath, nil, now)
	if !okU {
		t.Fatal("processLearningFile returned ok=false for unsourced learning")
	}

	// Sourced learning should keep its utility (0.8)
	if sourced.Utility != 0.8 {
		t.Errorf("sourced Utility = %f, want 0.8", sourced.Utility)
	}

	// Unsourced learning gets 0.7x soft penalty: 0.8 * 0.7 = 0.56
	// (Changed from 0.3x hard penalty to avoid cliffing learnings below the >0.3 quality gate threshold)
	expectedUnsourced := 0.8 * 0.7
	const tolerance = 0.001
	if unsourced.Utility < expectedUnsourced-tolerance || unsourced.Utility > expectedUnsourced+tolerance {
		t.Errorf("unsourced Utility = %f, want ~%f (0.7x soft penalty)", unsourced.Utility, expectedUnsourced)
	}

	// Sourced must have strictly higher utility
	if sourced.Utility <= unsourced.Utility {
		t.Errorf("sourced Utility (%f) should be > unsourced Utility (%f)", sourced.Utility, unsourced.Utility)
	}
}

// TestFlywheelProof_DecayReducesScore verifies that the freshness scoring
// function produces lower scores for older learnings, proving the decay
// mechanism works. Uses freshnessScore directly since file-mtime-based
// decay is the primary mechanism for .md learnings.
func TestFlywheelProof_DecayReducesScore(t *testing.T) {
	// Fresh learning (0 weeks old) should score near 1.0
	freshScore := freshnessScore(0)
	if freshScore < 0.99 {
		t.Errorf("freshnessScore(0) = %f, want ~1.0", freshScore)
	}

	// 8-week-old learning should score lower
	oldScore := freshnessScore(8)
	if oldScore >= freshScore {
		t.Errorf("freshnessScore(8) = %f should be less than freshnessScore(0) = %f", oldScore, freshScore)
	}

	// Very old learning (52 weeks) should approach minimum
	veryOldScore := freshnessScore(52)
	if veryOldScore > 0.15 {
		t.Errorf("freshnessScore(52) = %f, expected near minimum (0.1)", veryOldScore)
	}

	// Also verify through processLearningFile with actual file timestamps.
	// Create two files: one recent, one with an old mtime.
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := "---\nutility: 0.8\nsource_bead: decay-test\nmaturity: provisional\n---\n# Decay Test\n\nContent for decay test validates that older learnings receive lower freshness scores than recent ones.\n"

	recentPath := filepath.Join(learningsDir, "recent.md")
	if err := os.WriteFile(recentPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	oldPath := filepath.Join(learningsDir, "old.md")
	if err := os.WriteFile(oldPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	// Set old file mtime to 60 days ago
	oldTime := time.Now().Add(-60 * 24 * time.Hour)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	now := time.Now()

	recent, okR := processLearningFile(recentPath, nil, now)
	if !okR {
		t.Fatal("processLearningFile returned ok=false for recent file")
	}

	old, okO := processLearningFile(oldPath, nil, now)
	if !okO {
		t.Fatal("processLearningFile returned ok=false for old file")
	}

	if recent.FreshnessScore <= old.FreshnessScore {
		t.Errorf("recent FreshnessScore (%f) should be > old FreshnessScore (%f)",
			recent.FreshnessScore, old.FreshnessScore)
	}

	// Verify default utility is set when no explicit utility in frontmatter
	// (both files have explicit utility=0.8, but verify the mechanism works)
	tmpNoUtil := t.TempDir()
	noUtilDir := filepath.Join(tmpNoUtil, ".agents", "learnings")
	if err := os.MkdirAll(noUtilDir, 0755); err != nil {
		t.Fatal(err)
	}
	noUtilContent := "---\nsource_bead: no-util\nmaturity: provisional\n---\n# No Utility Learning\n\nContent without utility field should receive the default InitialUtility value from the system.\n"
	noUtilPath := filepath.Join(noUtilDir, "no-util.md")
	if err := os.WriteFile(noUtilPath, []byte(noUtilContent), 0644); err != nil {
		t.Fatal(err)
	}
	noUtil, okNU := processLearningFile(noUtilPath, nil, now)
	if !okNU {
		t.Fatal("processLearningFile returned ok=false for no-utility file")
	}
	if noUtil.Utility != types.InitialUtility {
		t.Errorf("Utility = %f, want %f (InitialUtility default)", noUtil.Utility, types.InitialUtility)
	}
}
