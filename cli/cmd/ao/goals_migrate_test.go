// practices: [dora-metrics, lean-startup]
package main

import (
	"testing"

	"github.com/boshu2/agentops/cli/internal/goals"
)

func TestDirectivesFromPillars_NoPillars(t *testing.T) {
	gs := []goals.Goal{
		{ID: "g1", Check: "true", Weight: 1},
		{ID: "g2", Check: "true", Weight: 1},
	}
	dirs := directivesFromPillars(gs)
	if len(dirs) != 1 {
		t.Fatalf("expected 1 default directive, got %d", len(dirs))
	}
	if dirs[0].Title != "Improve project quality" {
		t.Errorf("default directive title = %q", dirs[0].Title)
	}
	if dirs[0].Number != 1 {
		t.Errorf("default directive number = %d, want 1", dirs[0].Number)
	}
}

func TestDirectivesFromPillars_WithPillars(t *testing.T) {
	gs := []goals.Goal{
		{ID: "g1", Pillar: "reliability", Check: "true", Weight: 1},
		{ID: "g2", Pillar: "security", Check: "true", Weight: 1},
		{ID: "g3", Pillar: "reliability", Check: "true", Weight: 1}, // duplicate
	}
	dirs := directivesFromPillars(gs)
	if len(dirs) != 2 {
		t.Fatalf("expected 2 directives (deduped), got %d", len(dirs))
	}
	if dirs[0].Number != 1 || dirs[1].Number != 2 {
		t.Errorf("numbering wrong: %d, %d", dirs[0].Number, dirs[1].Number)
	}
	if dirs[0].Title != "Strengthen reliability" {
		t.Errorf("dirs[0].Title = %q", dirs[0].Title)
	}
	if dirs[1].Title != "Strengthen security" {
		t.Errorf("dirs[1].Title = %q", dirs[1].Title)
	}
}

func TestDirectivesFromPillars_EmptyGoals(t *testing.T) {
	dirs := directivesFromPillars([]goals.Goal{})
	if len(dirs) != 1 {
		t.Fatalf("expected 1 default directive for empty goals, got %d", len(dirs))
	}
	if dirs[0].Title != "Improve project quality" {
		t.Errorf("default directive title = %q", dirs[0].Title)
	}
}

func TestDirectivesFromPillars_MixedEmptyAndFilled(t *testing.T) {
	gs := []goals.Goal{
		{ID: "g1", Pillar: "", Check: "true", Weight: 1},
		{ID: "g2", Pillar: "performance", Check: "true", Weight: 1},
		{ID: "g3", Pillar: "", Check: "true", Weight: 1},
	}
	dirs := directivesFromPillars(gs)
	if len(dirs) != 1 {
		t.Fatalf("expected 1 directive (only filled pillars), got %d", len(dirs))
	}
	if dirs[0].Title != "Strengthen performance" {
		t.Errorf("dirs[0].Title = %q", dirs[0].Title)
	}
}
