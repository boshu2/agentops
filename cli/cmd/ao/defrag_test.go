package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFindOrphanLearnings_StaleUnreferenced(t *testing.T) {
	tmp := t.TempDir()

	// Create learnings dir with an old file
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	oldFile := filepath.Join(learningsDir, "old-learning.md")
	if err := os.WriteFile(oldFile, []byte("# Old learning\nSome content"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Set mtime to 60 days ago
	oldTime := time.Now().AddDate(0, 0, -60)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	// Create empty patterns and research dirs (no references)
	if err := os.MkdirAll(filepath.Join(tmp, ".agents", "patterns"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, ".agents", "research"), 0o750); err != nil {
		t.Fatal(err)
	}

	result, err := findOrphanLearnings(tmp, 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalLearnings != 1 {
		t.Errorf("TotalLearnings = %d, want 1", result.TotalLearnings)
	}
	if result.StaleCount != 1 {
		t.Errorf("StaleCount = %d, want 1", result.StaleCount)
	}
	if len(result.Orphans) != 1 {
		t.Fatalf("Orphans count = %d, want 1", len(result.Orphans))
	}
	expected := filepath.Join(".agents", "learnings", "old-learning.md")
	if result.Orphans[0] != expected {
		t.Errorf("Orphans[0] = %q, want %q", result.Orphans[0], expected)
	}
}

func TestFindOrphanLearnings_StaleButReferenced(t *testing.T) {
	tmp := t.TempDir()

	// Create learnings dir with an old file
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	oldFile := filepath.Join(learningsDir, "referenced-learning.md")
	if err := os.WriteFile(oldFile, []byte("# Referenced learning"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().AddDate(0, 0, -60)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	// Create a pattern file that references the learning
	patternsDir := filepath.Join(tmp, ".agents", "patterns")
	if err := os.MkdirAll(patternsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	patternContent := "# Pattern\nSee [referenced-learning.md](../learnings/referenced-learning.md)\n"
	if err := os.WriteFile(filepath.Join(patternsDir, "pattern.md"), []byte(patternContent), 0o600); err != nil {
		t.Fatal(err)
	}

	// Create research dir
	if err := os.MkdirAll(filepath.Join(tmp, ".agents", "research"), 0o750); err != nil {
		t.Fatal(err)
	}

	result, err := findOrphanLearnings(tmp, 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.StaleCount != 1 {
		t.Errorf("StaleCount = %d, want 1", result.StaleCount)
	}
	if len(result.Orphans) != 0 {
		t.Errorf("Orphans = %v, want empty (file is referenced)", result.Orphans)
	}
}

func TestFindOrphanLearnings_NoLearningsDir(t *testing.T) {
	tmp := t.TempDir()

	result, err := findOrphanLearnings(tmp, 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalLearnings != 0 {
		t.Errorf("TotalLearnings = %d, want 0", result.TotalLearnings)
	}
}

func TestFindDuplicateLearnings_NearDuplicate(t *testing.T) {
	tmp := t.TempDir()

	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o750); err != nil {
		t.Fatal(err)
	}

	// Two nearly identical files
	content := "This is a learning about how to handle errors in Go programs effectively and safely"
	if err := os.WriteFile(filepath.Join(learningsDir, "a.md"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	// Slightly different version of same content
	content2 := "This is a learning about how to handle errors in Go programs effectively and well"
	if err := os.WriteFile(filepath.Join(learningsDir, "b.md"), []byte(content2), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := findDuplicateLearnings(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Checked != 2 {
		t.Errorf("Checked = %d, want 2", result.Checked)
	}
	if len(result.DuplicatePairs) != 1 {
		t.Fatalf("DuplicatePairs count = %d, want 1", len(result.DuplicatePairs))
	}
	pair := result.DuplicatePairs[0]
	if pair[0] != "a.md" || pair[1] != "b.md" {
		t.Errorf("DuplicatePairs[0] = %v, want [a.md, b.md]", pair)
	}
}

func TestFindDuplicateLearnings_Distinct(t *testing.T) {
	tmp := t.TempDir()

	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o750); err != nil {
		t.Fatal(err)
	}

	// Two completely different files
	if err := os.WriteFile(filepath.Join(learningsDir, "x.md"),
		[]byte("# Go Error Handling\nAlways wrap errors with fmt.Errorf and %w verb"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(learningsDir, "y.md"),
		[]byte("# Python Virtual Environments\nUse venv module for isolated package management"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := findDuplicateLearnings(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Checked != 2 {
		t.Errorf("Checked = %d, want 2", result.Checked)
	}
	if len(result.DuplicatePairs) != 0 {
		t.Errorf("DuplicatePairs = %v, want empty", result.DuplicatePairs)
	}
}

func TestSweepOscillatingGoals_Oscillating(t *testing.T) {
	tmp := t.TempDir()

	evolveDir := filepath.Join(tmp, ".agents", "evolve")
	if err := os.MkdirAll(evolveDir, 0o750); err != nil {
		t.Fatal(err)
	}

	// Create a cycle history where "logging" oscillates improved/fail 4 times
	lines := []cycleRecord{
		{Cycle: 1, Target: "logging", Result: "improved"},
		{Cycle: 2, Target: "logging", Result: "fail"},
		{Cycle: 3, Target: "logging", Result: "improved"},
		{Cycle: 4, Target: "logging", Result: "fail"},
		{Cycle: 5, Target: "logging", Result: "improved"},
	}

	histPath := filepath.Join(evolveDir, "cycle-history.jsonl")
	f, err := os.Create(histPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, rec := range lines {
		data, _ := json.Marshal(rec)
		f.Write(data)
		f.Write([]byte("\n"))
	}
	f.Close()

	result, err := sweepOscillatingGoals(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.OscillatingGoals) != 1 {
		t.Fatalf("OscillatingGoals count = %d, want 1", len(result.OscillatingGoals))
	}
	goal := result.OscillatingGoals[0]
	if goal.Target != "logging" {
		t.Errorf("Target = %q, want %q", goal.Target, "logging")
	}
	if goal.AlternationCount != 4 {
		t.Errorf("AlternationCount = %d, want 4", goal.AlternationCount)
	}
	if goal.LastCycle != 5 {
		t.Errorf("LastCycle = %d, want 5", goal.LastCycle)
	}
}

func TestSweepOscillatingGoals_Stable(t *testing.T) {
	tmp := t.TempDir()

	evolveDir := filepath.Join(tmp, ".agents", "evolve")
	if err := os.MkdirAll(evolveDir, 0o750); err != nil {
		t.Fatal(err)
	}

	// All improved — no oscillation
	lines := []cycleRecord{
		{Cycle: 1, Target: "tests", Result: "improved"},
		{Cycle: 2, Target: "tests", Result: "improved"},
		{Cycle: 3, Target: "tests", Result: "improved"},
	}

	histPath := filepath.Join(evolveDir, "cycle-history.jsonl")
	f, err := os.Create(histPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, rec := range lines {
		data, _ := json.Marshal(rec)
		f.Write(data)
		f.Write([]byte("\n"))
	}
	f.Close()

	result, err := sweepOscillatingGoals(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.OscillatingGoals) != 0 {
		t.Errorf("OscillatingGoals = %v, want empty", result.OscillatingGoals)
	}
}

func TestSweepOscillatingGoals_NoHistory(t *testing.T) {
	tmp := t.TempDir()

	// No cycle-history.jsonl file at all
	result, err := sweepOscillatingGoals(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.OscillatingGoals != nil {
		t.Errorf("OscillatingGoals = %v, want nil", result.OscillatingGoals)
	}
}

func TestRunDefrag_DryRun(t *testing.T) {
	tmp := t.TempDir()

	// Create learnings dir with an old orphan file
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, ".agents", "patterns"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, ".agents", "research"), 0o750); err != nil {
		t.Fatal(err)
	}

	orphanFile := filepath.Join(learningsDir, "orphan.md")
	if err := os.WriteFile(orphanFile, []byte("# Orphan"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().AddDate(0, 0, -60)
	if err := os.Chtimes(orphanFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	// Simulate dry-run by calling findOrphanLearnings (the function
	// doesn't delete — that's done in runDefrag based on isDryRun)
	result, err := findOrphanLearnings(tmp, 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Orphans) != 1 {
		t.Fatalf("Orphans count = %d, want 1", len(result.Orphans))
	}

	// Verify file still exists (dry-run should not delete)
	if _, err := os.Stat(orphanFile); os.IsNotExist(err) {
		t.Error("orphan file was deleted during dry-run")
	}

	// Verify Deleted is nil (not set during dry-run)
	if result.Deleted != nil {
		t.Errorf("Deleted = %v, want nil in dry-run", result.Deleted)
	}
}

func TestWriteDefragReport_CreatesLatest(t *testing.T) {
	tmp := t.TempDir()

	outDir := filepath.Join(tmp, "defrag-output")

	// Temporarily set defragQuiet so printDefragSummary is suppressed in tests
	origQuiet := defragQuiet
	defragQuiet = true
	defer func() { defragQuiet = origQuiet }()

	report := &DefragReport{
		Timestamp: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		DryRun:    true,
		Prune: &PruneResult{
			TotalLearnings: 10,
			StaleCount:     3,
			Orphans:        []string{".agents/learnings/old.md"},
		},
	}

	if err := writeDefragReport(outDir, report, io.Discard); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check dated file exists
	datedPath := filepath.Join(outDir, "2026-03-01.json")
	if _, err := os.Stat(datedPath); os.IsNotExist(err) {
		t.Error("dated report file not created")
	}

	// Check latest.json exists
	latestPath := filepath.Join(outDir, "latest.json")
	if _, err := os.Stat(latestPath); os.IsNotExist(err) {
		t.Error("latest.json not created")
	}

	// Verify content can be parsed back
	data, err := os.ReadFile(latestPath)
	if err != nil {
		t.Fatalf("read latest.json: %v", err)
	}

	var parsed DefragReport
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal latest.json: %v", err)
	}

	if !parsed.DryRun {
		t.Error("parsed.DryRun = false, want true")
	}
	if parsed.Prune == nil {
		t.Fatal("parsed.Prune is nil")
	}
	if parsed.Prune.TotalLearnings != 10 {
		t.Errorf("parsed.Prune.TotalLearnings = %d, want 10", parsed.Prune.TotalLearnings)
	}
}

func TestBuildTrigrams(t *testing.T) {
	tg := buildTrigrams("abcde")
	// Should have: "abc", "bcd", "cde"
	if len(tg) != 3 {
		t.Errorf("trigram count = %d, want 3", len(tg))
	}
	for _, expected := range []string{"abc", "bcd", "cde"} {
		if !tg[expected] {
			t.Errorf("missing trigram %q", expected)
		}
	}
}

func TestTrigramOverlap_Identical(t *testing.T) {
	a := buildTrigrams("hello world")
	b := buildTrigrams("hello world")
	overlap := trigramOverlap(a, b)
	if overlap != 1.0 {
		t.Errorf("overlap = %f, want 1.0", overlap)
	}
}

func TestTrigramOverlap_Empty(t *testing.T) {
	a := buildTrigrams("")
	b := buildTrigrams("")
	overlap := trigramOverlap(a, b)
	if overlap != 0 {
		t.Errorf("overlap = %f, want 0", overlap)
	}
}

func TestIsHashNamed(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"2026-02-23-4556c2b4.md", true},
		{"2026-02-24-d26c5b4e.md", true},
		{"2026-02-25-b64c8555.md", true},
		{"2026-02-23-cli-skill-audit-retro.md", false},
		{"2026-02-24-tdd-hardening.md", false},
		{"2026-02-24-the-seed-post-mortem.md", false},
		{"plain.md", false},
		{"2026-02-24-toolongname.md", false}, // 12 chars after date prefix
		{"2026-02-24-ABCDEF12.md", false},    // uppercase hex — not a match
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isHashNamed(tt.name)
			if got != tt.want {
				t.Errorf("isHashNamed(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestFindDuplicateLearnings_DedupApply(t *testing.T) {
	tmp := t.TempDir()

	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o750); err != nil {
		t.Fatal(err)
	}

	content := "This is a learning about how to handle errors in Go programs effectively and safely"
	hashFile := "2026-03-01-a1b2c3d4.md"
	namedFile := "2026-03-01-my-learning.md"
	if err := os.WriteFile(filepath.Join(learningsDir, hashFile), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(learningsDir, namedFile), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := findDuplicateLearnings(tmp)
	if err != nil {
		t.Fatalf("findDuplicateLearnings: %v", err)
	}
	if len(result.DuplicatePairs) != 1 {
		t.Fatalf("DuplicatePairs count = %d, want 1", len(result.DuplicatePairs))
	}

	// Simulate the apply path from runDefrag.
	for _, pair := range result.DuplicatePairs {
		keep, del := pair[0], pair[1]
		if isHashNamed(pair[0]) && !isHashNamed(pair[1]) {
			keep, del = pair[1], pair[0]
		}
		_ = keep
		p := filepath.Join(tmp, ".agents", "learnings", del)
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			t.Fatalf("remove: %v", err)
		}
		result.Deleted = append(result.Deleted, del)
	}

	// Hash-named file should be deleted, named file should survive.
	if _, err := os.Stat(filepath.Join(learningsDir, hashFile)); !os.IsNotExist(err) {
		t.Errorf("hash-named file %q should have been deleted", hashFile)
	}
	if _, err := os.Stat(filepath.Join(learningsDir, namedFile)); os.IsNotExist(err) {
		t.Errorf("named file %q should have been kept", namedFile)
	}
	if len(result.Deleted) != 1 || result.Deleted[0] != hashFile {
		t.Errorf("Deleted = %v, want [%s]", result.Deleted, hashFile)
	}
}

func TestDefragOutputDirFlag(t *testing.T) {
	resetCommandState(t)
	// Verify the flag is named "output-dir", not "output"
	cmd := defragCmd
	f := cmd.Flags().Lookup("output-dir")
	if f == nil {
		t.Fatal("expected --output-dir flag, not found")
	}
	// Also verify "output" is NOT a registered local flag on defrag
	if old := cmd.LocalFlags().Lookup("output"); old != nil {
		t.Error("--output flag should be renamed to --output-dir")
	}
}

func TestWriteDefragReport_JSONOutput(t *testing.T) {
	tmp := t.TempDir()
	outDir := filepath.Join(tmp, "defrag-output")

	// Save and restore global state
	origQuiet := defragQuiet
	origOutput := output
	defragQuiet = true
	output = "json"
	defer func() {
		defragQuiet = origQuiet
		output = origOutput
	}()

	report := &DefragReport{
		Timestamp: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		DryRun:    true,
		Prune: &PruneResult{
			TotalLearnings: 5,
			StaleCount:     2,
			Orphans:        []string{".agents/learnings/stale.md"},
		},
	}

	var buf bytes.Buffer
	err := writeDefragReport(outDir, report, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stdout := buf.String()

	// Verify JSON was written to the writer
	var parsed DefragReport
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nGot: %s", err, stdout)
	}
	if !parsed.DryRun {
		t.Error("parsed.DryRun = false, want true")
	}
	if parsed.Prune == nil || parsed.Prune.TotalLearnings != 5 {
		t.Errorf("parsed.Prune.TotalLearnings = %v, want 5", parsed.Prune)
	}
}

func TestWriteDefragReport_TextOutputNotJSON(t *testing.T) {
	tmp := t.TempDir()
	outDir := filepath.Join(tmp, "defrag-output")

	// Save and restore global state
	origQuiet := defragQuiet
	origOutput := output
	defragQuiet = false
	output = "table"
	defer func() {
		defragQuiet = origQuiet
		output = origOutput
	}()

	report := &DefragReport{
		Timestamp: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		DryRun:    true,
		Prune: &PruneResult{
			TotalLearnings: 5,
			StaleCount:     2,
		},
	}

	var buf bytes.Buffer
	err := writeDefragReport(outDir, report, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stdout := buf.String()

	// Should contain human-readable summary, not JSON
	if !strings.Contains(stdout, "Defrag report:") {
		t.Errorf("expected human-readable summary, got: %s", stdout)
	}
}

func TestCountAlternations(t *testing.T) {
	tests := []struct {
		name    string
		records []cycleRecord
		want    int
	}{
		{
			name:    "empty",
			records: nil,
			want:    0,
		},
		{
			name: "no alternation",
			records: []cycleRecord{
				{Result: "improved"},
				{Result: "improved"},
				{Result: "improved"},
			},
			want: 0,
		},
		{
			name: "three alternations",
			records: []cycleRecord{
				{Result: "improved"},
				{Result: "fail"},
				{Result: "improved"},
				{Result: "fail"},
			},
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countAlternations(tt.records)
			if got != tt.want {
				t.Errorf("countAlternations = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestExecutePrune_FindsOrphans(t *testing.T) {
	tmp := t.TempDir()

	// Create learnings dir with an old orphan file
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, ".agents", "patterns"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, ".agents", "research"), 0o750); err != nil {
		t.Fatal(err)
	}

	orphanFile := filepath.Join(learningsDir, "stale-orphan.md")
	if err := os.WriteFile(orphanFile, []byte("# Stale orphan learning\nNot referenced anywhere"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().AddDate(0, 0, -60)
	if err := os.Chtimes(orphanFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	// Also add a fresh file that should NOT be orphaned (not stale)
	freshFile := filepath.Join(learningsDir, "fresh-learning.md")
	if err := os.WriteFile(freshFile, []byte("# Fresh learning"), 0o600); err != nil {
		t.Fatal(err)
	}

	// isDryRun=false so orphans get deleted
	result, err := executePrune(tmp, false, 30)
	if err != nil {
		t.Fatalf("executePrune: %v", err)
	}

	if result.TotalLearnings != 2 {
		t.Errorf("TotalLearnings = %d, want 2", result.TotalLearnings)
	}
	if result.StaleCount != 1 {
		t.Errorf("StaleCount = %d, want 1", result.StaleCount)
	}
	if len(result.Orphans) != 1 {
		t.Fatalf("Orphans count = %d, want 1", len(result.Orphans))
	}
	expectedOrphan := filepath.Join(".agents", "learnings", "stale-orphan.md")
	if result.Orphans[0] != expectedOrphan {
		t.Errorf("Orphans[0] = %q, want %q", result.Orphans[0], expectedOrphan)
	}
	// Orphan should have been deleted
	if len(result.Deleted) != 1 {
		t.Fatalf("Deleted count = %d, want 1", len(result.Deleted))
	}
	if result.Deleted[0] != expectedOrphan {
		t.Errorf("Deleted[0] = %q, want %q", result.Deleted[0], expectedOrphan)
	}
	// File should no longer exist on disk
	if _, err := os.Stat(orphanFile); !os.IsNotExist(err) {
		t.Error("orphan file should have been deleted from disk")
	}
	// Fresh file should still exist
	if _, err := os.Stat(freshFile); os.IsNotExist(err) {
		t.Error("fresh file should NOT have been deleted")
	}
}

func TestExecutePrune_DryRun(t *testing.T) {
	tmp := t.TempDir()

	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, ".agents", "patterns"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, ".agents", "research"), 0o750); err != nil {
		t.Fatal(err)
	}

	orphanFile := filepath.Join(learningsDir, "dry-run-orphan.md")
	if err := os.WriteFile(orphanFile, []byte("# Orphan in dry-run test"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().AddDate(0, 0, -45)
	if err := os.Chtimes(orphanFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	// isDryRun=true — should report but NOT delete
	result, err := executePrune(tmp, true, 30)
	if err != nil {
		t.Fatalf("executePrune dry-run: %v", err)
	}

	// Orphan should be identified
	if len(result.Orphans) != 1 {
		t.Fatalf("Orphans count = %d, want 1", len(result.Orphans))
	}
	// Deleted should be empty in dry-run
	if len(result.Deleted) != 0 {
		t.Errorf("Deleted = %v, want empty in dry-run", result.Deleted)
	}
	// File must still exist on disk
	if _, err := os.Stat(orphanFile); os.IsNotExist(err) {
		t.Error("orphan file was deleted during dry-run — should have been preserved")
	}
}

func TestExecuteDedup_FindsDuplicates(t *testing.T) {
	tmp := t.TempDir()

	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o750); err != nil {
		t.Fatal(err)
	}

	// Two files with identical content but different names
	content := "This is a learning about how to handle errors in Go programs effectively and safely"
	hashFile := "2026-03-01-a1b2c3d4.md"
	namedFile := "2026-03-01-error-handling.md"
	if err := os.WriteFile(filepath.Join(learningsDir, hashFile), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(learningsDir, namedFile), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	// isDryRun=false — should delete the hash-named duplicate
	result, err := executeDedup(tmp, false)
	if err != nil {
		t.Fatalf("executeDedup: %v", err)
	}

	if result.Checked != 2 {
		t.Errorf("Checked = %d, want 2", result.Checked)
	}
	// After apply, DuplicatePairs is cleared
	if result.DuplicatePairs != nil {
		t.Errorf("DuplicatePairs should be nil after apply, got %v", result.DuplicatePairs)
	}
	// Hash-named file should have been deleted
	if len(result.Deleted) != 1 {
		t.Fatalf("Deleted count = %d, want 1", len(result.Deleted))
	}
	if result.Deleted[0] != hashFile {
		t.Errorf("Deleted[0] = %q, want %q", result.Deleted[0], hashFile)
	}
	// Hash file gone from disk
	if _, err := os.Stat(filepath.Join(learningsDir, hashFile)); !os.IsNotExist(err) {
		t.Error("hash-named duplicate should have been deleted from disk")
	}
	// Named file preserved
	if _, err := os.Stat(filepath.Join(learningsDir, namedFile)); os.IsNotExist(err) {
		t.Error("named file should have been kept")
	}
}

func TestExecuteDedup_DryRun(t *testing.T) {
	tmp := t.TempDir()

	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o750); err != nil {
		t.Fatal(err)
	}

	content := "This is a learning about how to handle errors in Go programs effectively and safely"
	fileA := "2026-03-01-dup-a.md"
	fileB := "2026-03-01-dup-b.md"
	if err := os.WriteFile(filepath.Join(learningsDir, fileA), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(learningsDir, fileB), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	// isDryRun=true — report but do NOT remove
	result, err := executeDedup(tmp, true)
	if err != nil {
		t.Fatalf("executeDedup dry-run: %v", err)
	}

	if result.Checked != 2 {
		t.Errorf("Checked = %d, want 2", result.Checked)
	}
	// Duplicates should be reported
	if len(result.DuplicatePairs) != 1 {
		t.Fatalf("DuplicatePairs count = %d, want 1", len(result.DuplicatePairs))
	}
	// Nothing should be deleted in dry-run
	if len(result.Deleted) != 0 {
		t.Errorf("Deleted = %v, want empty in dry-run", result.Deleted)
	}
	// Both files must still exist on disk
	if _, err := os.Stat(filepath.Join(learningsDir, fileA)); os.IsNotExist(err) {
		t.Error("fileA was deleted during dry-run")
	}
	if _, err := os.Stat(filepath.Join(learningsDir, fileB)); os.IsNotExist(err) {
		t.Error("fileB was deleted during dry-run")
	}
}

func TestDefrag_NoFlags_DefaultsAll(t *testing.T) {
	// When no mode flags are set, runDefrag should enable all three modes.
	dir := chdirTemp(t)

	// Seed minimal .agents/ structure so defrag has something to scan.
	agentsDir := filepath.Join(dir, ".agents", "learnings")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Reset flags to simulate bare "ao defrag" (no mode flags).
	defragPrune = false
	defragDedup = false
	defragOscillationSweep = false
	defragQuiet = true
	defer func() {
		defragPrune = false
		defragDedup = false
		defragOscillationSweep = false
		defragQuiet = false
	}()

	err := runDefrag(defragCmd, nil)
	if err != nil {
		t.Fatalf("runDefrag with no flags: %v", err)
	}

	// After runDefrag, all three should have been enabled.
	if !defragPrune {
		t.Error("expected defragPrune to be true after no-flags invocation")
	}
	if !defragDedup {
		t.Error("expected defragDedup to be true after no-flags invocation")
	}
	if !defragOscillationSweep {
		t.Error("expected defragOscillationSweep to be true after no-flags invocation")
	}
}

// ---------------------------------------------------------------------------
// trigramOverlap
// ---------------------------------------------------------------------------

func TestTrigramOverlap_BothEmpty(t *testing.T) {
	got := trigramOverlap(map[string]bool{}, map[string]bool{})
	if got != 0 {
		t.Errorf("trigramOverlap(empty, empty) = %v, want 0", got)
	}
}

func TestTrigramOverlap_NoOverlap(t *testing.T) {
	a := map[string]bool{"abc": true, "bcd": true}
	b := map[string]bool{"xyz": true, "yzw": true}
	got := trigramOverlap(a, b)
	if got != 0 {
		t.Errorf("trigramOverlap(disjoint) = %v, want 0", got)
	}
}

func TestTrigramOverlap_PartialOverlap(t *testing.T) {
	a := map[string]bool{"abc": true, "bcd": true, "cde": true}
	b := map[string]bool{"abc": true, "xyz": true}
	// intersection=1, union=3+2-1=4 => 0.25
	got := trigramOverlap(a, b)
	if got != 0.25 {
		t.Errorf("trigramOverlap(partial) = %v, want 0.25", got)
	}
}

func TestTrigramOverlap_OneEmpty(t *testing.T) {
	a := map[string]bool{"abc": true}
	got := trigramOverlap(a, map[string]bool{})
	if got != 0 {
		t.Errorf("trigramOverlap(a, empty) = %v, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// sweepOscillatingGoals
// ---------------------------------------------------------------------------

func TestSweepOscillatingGoals_NoHistoryFile(t *testing.T) {
	tmp := t.TempDir()
	result, err := sweepOscillatingGoals(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.OscillatingGoals) != 0 {
		t.Errorf("expected no oscillating goals for missing file, got %d", len(result.OscillatingGoals))
	}
}

func TestSweepOscillatingGoals_DetectsOscillation(t *testing.T) {
	tmp := t.TempDir()
	histDir := filepath.Join(tmp, ".agents", "evolve")
	if err := os.MkdirAll(histDir, 0755); err != nil {
		t.Fatal(err)
	}
	// goal-a alternates 4 times (>= 3 threshold)
	// goal-b only 1 alternation (below threshold)
	content := `{"cycle":1,"target":"goal-a","result":"improved"}
{"cycle":2,"target":"goal-a","result":"regressed"}
{"cycle":3,"target":"goal-a","result":"improved"}
{"cycle":4,"target":"goal-a","result":"regressed"}
{"cycle":5,"target":"goal-a","result":"improved"}
{"cycle":1,"target":"goal-b","result":"improved"}
{"cycle":2,"target":"goal-b","result":"regressed"}
`
	if err := os.WriteFile(filepath.Join(histDir, "cycle-history.jsonl"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := sweepOscillatingGoals(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.OscillatingGoals) != 1 {
		t.Fatalf("expected 1 oscillating goal, got %d", len(result.OscillatingGoals))
	}
	if result.OscillatingGoals[0].Target != "goal-a" {
		t.Errorf("expected goal-a, got %q", result.OscillatingGoals[0].Target)
	}
	if result.OscillatingGoals[0].AlternationCount != 4 {
		t.Errorf("expected 4 alternations, got %d", result.OscillatingGoals[0].AlternationCount)
	}
	if result.OscillatingGoals[0].LastCycle != 5 {
		t.Errorf("expected last cycle 5, got %d", result.OscillatingGoals[0].LastCycle)
	}
}

func TestSweepOscillatingGoals_SkipsMalformedLines(t *testing.T) {
	tmp := t.TempDir()
	histDir := filepath.Join(tmp, ".agents", "evolve")
	if err := os.MkdirAll(histDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `not valid json
{"cycle":1,"target":"","result":"improved"}
{"cycle":1,"target":"goal-x","result":"improved"}

{"cycle":2,"target":"goal-x","result":"regressed"}
`
	if err := os.WriteFile(filepath.Join(histDir, "cycle-history.jsonl"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := sweepOscillatingGoals(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// goal-x only has 1 alternation, not enough
	if len(result.OscillatingGoals) != 0 {
		t.Errorf("expected 0 oscillating goals, got %d", len(result.OscillatingGoals))
	}
}
