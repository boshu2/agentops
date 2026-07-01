package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/search"
)

// TestLearningSurface_AliasesResolve confirms the relocated type aliases still
// point at the canonical internal/search types (the Phase-0 decouple must not
// change identity — survivors depend on these by name).
func TestLearningSurface_AliasesResolve(t *testing.T) {
	// Compile-time identity check that each local alias still points at the
	// canonical internal/search type: pass an alias-typed zero value into a
	// function whose parameter is the canonical type. This compiles iff the
	// alias resolves, and — unlike a `var _ search.Learning = l` annotation — it
	// is not a redundant-type var declaration, so staticcheck QF1011 (which fires
	// on identical alias types) does not apply.
	wantLearning := func(search.Learning) {}
	wantPattern := func(search.Pattern) {}
	wantKnowledgeFinding := func(search.KnowledgeFinding) {}
	wantSession := func(search.Session) {}
	var l learning
	var p pattern
	var f knowledgeFinding
	var s session
	wantLearning(l)
	wantPattern(p)
	wantKnowledgeFinding(f)
	wantSession(s)
}

// TestLearningSurface_HelpersWrapSearch confirms the relocated helpers behave as
// thin wrappers over internal/search (parse round-trips, freshness applies, glob
// finds .md files, canon dir derives from the canon constants).
func TestLearningSurface_HelpersWrapSearch(t *testing.T) {
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "x.md")
	body := "---\nid: surf-1\ntype: learning\nmaturity: provisional\n---\n\n# Learning: surf\n\nbody about widgets.\n"
	if err := os.WriteFile(mdPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	// globLearningFiles discovers the .md file.
	files := globLearningFiles(dir)
	if len(files) != 1 || filepath.Base(files[0]) != "x.md" {
		t.Fatalf("globLearningFiles = %v, want [x.md]", files)
	}

	// parseLearningFile round-trips the id.
	l, err := parseLearningFile(mdPath)
	if err != nil {
		t.Fatalf("parseLearningFile: %v", err)
	}
	if l.ID != "surf-1" {
		t.Errorf("parsed id = %q, want surf-1", l.ID)
	}

	// applyFreshnessScore sets a freshness score (does not panic, mutates in place).
	applyFreshnessScore(&l, mdPath, time.Now())
	if l.FreshnessScore < 0 {
		t.Errorf("freshness score = %f, want >= 0", l.FreshnessScore)
	}

	// canonLearningsDir derives from the canon.go constants.
	if filepath.Base(canonLearningsDir) != canonLearnings {
		t.Errorf("canonLearningsDir = %q, want suffix %q", canonLearningsDir, canonLearnings)
	}
}
