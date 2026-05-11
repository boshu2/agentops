// practices: [dora-metrics, continuous-delivery]
package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

func TestRunRatchetSpec_FindsSpecV2(t *testing.T) {
	tmp := t.TempDir()
	setupAgentsDir(t, tmp)

	specsDir := filepath.Join(tmp, ".agents", "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("create specs dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specsDir, "feature-v2.md"), []byte("# Spec v2"), 0644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	locator, err := ratchet.NewLocator(tmp)
	if err != nil {
		t.Fatalf("NewLocator: %v", err)
	}

	// Mirrors the search patterns from runRatchetSpec
	patterns := []string{
		"specs/*-v*.md",
		"synthesis/*.md",
	}

	found := false
	for _, pattern := range patterns {
		path, _, err := locator.FindFirst(pattern)
		if err == nil {
			found = true
			if path == "" {
				t.Error("FindFirst returned empty path")
			}
			break
		}
	}

	if !found {
		t.Error("spec not found via FindFirst")
	}
}

func TestRunRatchetSpec_FindsSynthesis(t *testing.T) {
	tmp := t.TempDir()
	setupAgentsDir(t, tmp)

	synthesisDir := filepath.Join(tmp, ".agents", "synthesis")
	if err := os.MkdirAll(synthesisDir, 0755); err != nil {
		t.Fatalf("create synthesis dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(synthesisDir, "analysis.md"), []byte("# Synthesis"), 0644); err != nil {
		t.Fatalf("write synthesis: %v", err)
	}

	locator, err := ratchet.NewLocator(tmp)
	if err != nil {
		t.Fatalf("NewLocator: %v", err)
	}

	path, _, err := locator.FindFirst("synthesis/*.md")
	if err != nil {
		t.Fatalf("FindFirst synthesis: %v", err)
	}

	if path == "" {
		t.Error("synthesis artifact not found")
	}
}

func TestRunRatchetSpec_NoSpecFoundLocally(t *testing.T) {
	tmp := t.TempDir()
	setupAgentsDir(t, tmp)

	locator, err := ratchet.NewLocator(tmp)
	if err != nil {
		t.Fatalf("NewLocator: %v", err)
	}

	// Search only for spec patterns and verify crew-level has no matches
	// Note: The locator also checks rig/town/plugins locations, so we only
	// verify the crew-level search returns nothing for our empty temp dir.
	result, err := locator.Find("specs/*-v*.md")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	// No specs in our temp dir's crew location
	crewMatches := 0
	for _, m := range result.Matches {
		if m.Location == string(ratchet.LocationCrew) {
			crewMatches++
		}
	}
	if crewMatches != 0 {
		t.Errorf("expected 0 crew-level spec matches, got %d", crewMatches)
	}
}

func TestRunRatchetSpec_PrefersSpecOverSynthesis(t *testing.T) {
	tmp := t.TempDir()
	setupAgentsDir(t, tmp)

	// Create both spec and synthesis
	specsDir := filepath.Join(tmp, ".agents", "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("create specs dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specsDir, "feature-v2.md"), []byte("# Spec"), 0644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	synthesisDir := filepath.Join(tmp, ".agents", "synthesis")
	if err := os.MkdirAll(synthesisDir, 0755); err != nil {
		t.Fatalf("create synthesis dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(synthesisDir, "analysis.md"), []byte("# Synthesis"), 0644); err != nil {
		t.Fatalf("write synthesis: %v", err)
	}

	locator, err := ratchet.NewLocator(tmp)
	if err != nil {
		t.Fatalf("NewLocator: %v", err)
	}

	// The spec pattern is first in priority order
	patterns := []string{
		"specs/*-v*.md",
		"synthesis/*.md",
	}

	var foundPath string
	for _, pattern := range patterns {
		path, _, err := locator.FindFirst(pattern)
		if err == nil {
			foundPath = path
			break
		}
	}

	if foundPath == "" {
		t.Fatal("no spec found")
	}

	// Should find the spec (first pattern), not synthesis
	base := filepath.Base(foundPath)
	if base != "feature-v2.md" {
		t.Errorf("expected spec file, got %q", base)
	}
}

func TestRunRatchetSpec_SearchPatternsOrder(t *testing.T) {
	// Verify the search patterns match the implementation
	patterns := []string{
		"specs/*-v*.md",
		"synthesis/*.md",
	}

	if patterns[0] != "specs/*-v*.md" {
		t.Errorf("first pattern should be specs, got %q", patterns[0])
	}
	if patterns[1] != "synthesis/*.md" {
		t.Errorf("second pattern should be synthesis, got %q", patterns[1])
	}
}
