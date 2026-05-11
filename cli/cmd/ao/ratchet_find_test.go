// practices: [dora-metrics, continuous-delivery]
package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

func TestRunRatchetFind_LocatorFindsArtifact(t *testing.T) {
	tmp := t.TempDir()
	setupAgentsDir(t, tmp)

	// Create a research artifact
	researchDir := filepath.Join(tmp, ".agents", "research")
	if err := os.WriteFile(filepath.Join(researchDir, "findings.md"), []byte("# Findings"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	locator, err := ratchet.NewLocator(tmp)
	if err != nil {
		t.Fatalf("NewLocator: %v", err)
	}

	result, err := locator.Find("research/*.md")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	if len(result.Matches) == 0 {
		t.Error("expected at least one match, got 0")
	}

	// Verify our file is among the crew-level matches
	found := false
	for _, m := range result.Matches {
		if m.Location == string(ratchet.LocationCrew) && filepath.Base(m.Path) == "findings.md" {
			found = true
			break
		}
	}
	if !found {
		t.Error("crew-level findings.md not in matches")
	}
}

func TestRunRatchetFind_NoCrewMatches(t *testing.T) {
	tmp := t.TempDir()
	setupAgentsDir(t, tmp)

	locator, err := ratchet.NewLocator(tmp)
	if err != nil {
		t.Fatalf("NewLocator: %v", err)
	}

	// Use a unique pattern that won't exist anywhere
	result, err := locator.Find("nonexistent-unique-8f3a/*.xyz")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	if len(result.Matches) != 0 {
		t.Errorf("expected 0 matches for nonexistent pattern, got %d", len(result.Matches))
	}
}

func TestRunRatchetFind_CrewLevelMultipleMatches(t *testing.T) {
	tmp := t.TempDir()
	setupAgentsDir(t, tmp)

	researchDir := filepath.Join(tmp, ".agents", "research")
	files := []string{"topic-a.md", "topic-b.md", "topic-c.md"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(researchDir, f), []byte("# "+f), 0644); err != nil {
			t.Fatalf("write file %s: %v", f, err)
		}
	}

	locator, err := ratchet.NewLocator(tmp)
	if err != nil {
		t.Fatalf("NewLocator: %v", err)
	}

	result, err := locator.Find("research/*.md")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	// Count only crew-level matches from our temp dir
	crewMatches := 0
	for _, m := range result.Matches {
		if m.Location == string(ratchet.LocationCrew) {
			crewMatches++
		}
	}

	if crewMatches != 3 {
		t.Errorf("expected 3 crew-level matches, got %d", crewMatches)
	}
}

func TestRunRatchetFind_PatternPreserved(t *testing.T) {
	tmp := t.TempDir()
	setupAgentsDir(t, tmp)

	locator, err := ratchet.NewLocator(tmp)
	if err != nil {
		t.Fatalf("NewLocator: %v", err)
	}

	pattern := "specs/*-v2.md"
	result, err := locator.Find(pattern)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	if result.Pattern != pattern {
		t.Errorf("Pattern = %q, want %q", result.Pattern, pattern)
	}
}

func TestRunRatchetFind_FindFirstReturnsHighestPriority(t *testing.T) {
	tmp := t.TempDir()
	setupAgentsDir(t, tmp)

	// Create artifact at crew level
	researchDir := filepath.Join(tmp, ".agents", "research")
	if err := os.WriteFile(filepath.Join(researchDir, "topic.md"), []byte("# Topic"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	locator, err := ratchet.NewLocator(tmp)
	if err != nil {
		t.Fatalf("NewLocator: %v", err)
	}

	path, loc, err := locator.FindFirst("research/*.md")
	if err != nil {
		t.Fatalf("FindFirst: %v", err)
	}

	if path == "" {
		t.Error("FindFirst returned empty path")
	}
	// Crew is highest priority and our artifact should be found there
	if loc != ratchet.LocationCrew {
		t.Errorf("location = %q, want %q", loc, ratchet.LocationCrew)
	}
}

func TestRunRatchetFind_FindResultStructure(t *testing.T) {
	tmp := t.TempDir()
	setupAgentsDir(t, tmp)

	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.WriteFile(filepath.Join(learningsDir, "insight.md"), []byte("# Insight"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	locator, err := ratchet.NewLocator(tmp)
	if err != nil {
		t.Fatalf("NewLocator: %v", err)
	}

	result, err := locator.Find("learnings/*.md")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	// Verify result structure
	if result.Pattern != "learnings/*.md" {
		t.Errorf("Pattern = %q", result.Pattern)
	}
	if result.Matches == nil {
		t.Error("Matches should not be nil")
	}
	if result.Warnings == nil {
		t.Error("Warnings should not be nil (should be empty slice)")
	}
}
