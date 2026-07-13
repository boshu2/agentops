package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeScorecardPremortemCheck(t *testing.T, root, dir, id, content string) string {
	t.Helper()
	path := filepath.Join(root, ".agents", dir, id+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func prepareScorecardRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".agents", "rpi"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".agents", "rpi", "next-work.jsonl"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestLoadStigmergicScorecard_MergesPremortemDirectoriesByID(t *testing.T) {
	t.Run("canonical only counts", func(t *testing.T) {
		root := prepareScorecardRoot(t)
		writeScorecardPremortemCheck(t, root, "premortem-checks", "canonical", "canonical")
		scorecard, err := loadStigmergicScorecard(root)
		if err != nil {
			t.Fatal(err)
		}
		if scorecard.PreMortemChecks != 1 {
			t.Fatalf("PreMortemChecks = %d, want 1 canonical check", scorecard.PreMortemChecks)
		}
	})

	t.Run("legacy fills missing canonical IDs", func(t *testing.T) {
		root := prepareScorecardRoot(t)
		writeScorecardPremortemCheck(t, root, "premortem-checks", "canonical", "canonical")
		writeScorecardPremortemCheck(t, root, "pre-mortem-checks", "legacy", "legacy")
		scorecard, err := loadStigmergicScorecard(root)
		if err != nil {
			t.Fatal(err)
		}
		if scorecard.PreMortemChecks != 2 {
			t.Fatalf("PreMortemChecks = %d, want canonical plus legacy-fill IDs", scorecard.PreMortemChecks)
		}
	})

	t.Run("equal same ID counts once", func(t *testing.T) {
		root := prepareScorecardRoot(t)
		writeScorecardPremortemCheck(t, root, "premortem-checks", "equal", "same bytes")
		writeScorecardPremortemCheck(t, root, "pre-mortem-checks", "equal", "same bytes")
		scorecard, err := loadStigmergicScorecard(root)
		if err != nil {
			t.Fatal(err)
		}
		if scorecard.PreMortemChecks != 1 {
			t.Fatalf("PreMortemChecks = %d, want equal same-ID check counted once", scorecard.PreMortemChecks)
		}
	})

	t.Run("conflicting same ID fails and names both paths", func(t *testing.T) {
		root := prepareScorecardRoot(t)
		canonical := writeScorecardPremortemCheck(t, root, "premortem-checks", "conflict", "canonical bytes")
		legacy := writeScorecardPremortemCheck(t, root, "pre-mortem-checks", "conflict", "legacy bytes")
		_, err := loadStigmergicScorecard(root)
		if err == nil {
			t.Fatal("loadStigmergicScorecard silently chose one conflicting same-ID check")
		}
		for _, path := range []string{canonical, legacy} {
			if !strings.Contains(err.Error(), path) {
				t.Errorf("conflict error %q does not name %s", err, path)
			}
		}
	})
}

func TestLoadStatusFlywheelBrief_RejectsPremortemDirectoryConflict(t *testing.T) {
	root := prepareScorecardRoot(t)
	canonical := writeScorecardPremortemCheck(t, root, "premortem-checks", "conflict", "canonical bytes")
	legacy := writeScorecardPremortemCheck(t, root, "pre-mortem-checks", "conflict", "legacy bytes")
	_, err := loadStatusFlywheelBrief(root)
	if err == nil {
		t.Fatal("status silently discarded a canonical/legacy premortem conflict")
	}
	for _, path := range []string{canonical, legacy} {
		if !strings.Contains(err.Error(), path) {
			t.Errorf("status conflict error %q does not name %s", err, path)
		}
	}
}
