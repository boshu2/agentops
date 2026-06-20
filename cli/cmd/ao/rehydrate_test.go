package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPickLatestHandoff finds the most-recent handoff artifact (timestamped
// names sort lexicographically, so the max name is newest).
func TestPickLatestHandoff(t *testing.T) {
	dir := t.TempDir()
	hdir := filepath.Join(dir, ".agents", "handoff")
	if err := os.MkdirAll(hdir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"handoff-20260615T100000Z.json",
		"handoff-20260615T153000Z.json", // newest
		"handoff-20260614T090000Z.json",
		"not-a-handoff.txt",
	} {
		if err := os.WriteFile(filepath.Join(hdir, name), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := pickLatestHandoff(dir)
	if err != nil {
		t.Fatalf("pickLatestHandoff: %v", err)
	}
	if filepath.Base(got) != "handoff-20260615T153000Z.json" {
		t.Errorf("picked %q, want the newest handoff-20260615T153000Z.json", filepath.Base(got))
	}
}

// TestPickLatestHandoff_None returns an error (not a panic) when there is no
// handoff to rehydrate from.
func TestPickLatestHandoff_None(t *testing.T) {
	dir := t.TempDir()
	if _, err := pickLatestHandoff(dir); err == nil {
		t.Error("expected an error when no handoff artifacts exist")
	}
}

// TestRenderRehydrateBrief asserts the brief carries the lane's restore-essentials:
// goal, active bead (+ resolved br show), reservations to re-acquire, continuation, commits.
func TestRenderRehydrateBrief(t *testing.T) {
	a := &handoffArtifact{
		Goal:         "operability lane",
		Continuation: "Resume ag-7k2g2 (run resolved bead lookup for ag-7k2g2).",
		Summary:      "did things",
		State: &handoffState{
			ActiveBead:    "ag-7k2g2",
			Reservations:  []string{"cli/ [EmeraldJaguar]"},
			RecentCommits: []string{"ff4102167 fix(handoff): rehydrate-completeness"},
		},
	}
	brief := renderRehydrateBrief(a)
	for _, want := range []string{
		"operability lane", // goal
		"ag-7k2g2",         // active bead
		`BEADS_DIR="$(ao beads dir)" br show ag-7k2g2 --json`, // the resume verb
		"cli/ [EmeraldJaguar]",                                // reservation to re-acquire
		"Resume ag-7k2g2",                                     // continuation / next action
		"ff4102167",                                           // recent commit
	} {
		if !strings.Contains(brief, want) {
			t.Errorf("brief missing %q\n--- got ---\n%s", want, brief)
		}
	}
}

// TestRenderRehydrateBrief_Sparse: a thin handoff (no active bead) still renders
// a usable brief and doesn't invent fields.
func TestRenderRehydrateBrief_Sparse(t *testing.T) {
	a := &handoffArtifact{Goal: "x", State: &handoffState{}}
	brief := renderRehydrateBrief(a)
	if brief == "" {
		t.Fatal("brief should never be empty")
	}
	if strings.Contains(brief, "br show ") {
		t.Errorf("no active bead → must not emit a br show line, got:\n%s", brief)
	}
}
