// practices: [dora-metrics, wiki-knowledge-surface]
package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestCloseLoopArchivedPendingReplayIsStable(t *testing.T) {
	oldDryRun := dryRun
	dryRun = false
	t.Cleanup(func() { dryRun = oldDryRun })

	tmp := t.TempDir()
	sourceID := "2026-04-27-96469d5-6"
	pendingFile := writeArchivedShapePendingLearning(t, tmp, sourceID)

	first, err := performFlywheelCloseLoop(tmp, filepath.Join(".agents", "knowledge", "pending"), 0, true)
	if err != nil {
		t.Fatalf("first close-loop: %v", err)
	}
	if first.Ingest.Added != 1 {
		t.Fatalf("first ingest Added=%d, want 1 (res=%+v)", first.Ingest.Added, first.Ingest)
	}
	wantCandidateID := "pend-" + sourceID
	if len(first.Ingest.AddedIDs) != 1 || first.Ingest.AddedIDs[0] != wantCandidateID {
		t.Fatalf("first AddedIDs=%v, want [%s]", first.Ingest.AddedIDs, wantCandidateID)
	}
	assertPendingMovedToProcessed(t, tmp, pendingFile)
	assertNoAmplifiedReplayID(t, tmp, sourceID)
	firstIDs := collectCloseLoopGeneratedIDs(t, tmp)

	second, err := performFlywheelCloseLoop(tmp, filepath.Join(".agents", "knowledge", "pending"), 0, true)
	if err != nil {
		t.Fatalf("second close-loop: %v", err)
	}
	if second.Ingest.Added != 0 || second.Ingest.CandidatesFound != 0 {
		t.Fatalf("second close-loop re-ingested pending source: %+v", second.Ingest)
	}
	assertNoAmplifiedReplayID(t, tmp, sourceID)
	secondIDs := collectCloseLoopGeneratedIDs(t, tmp)
	if strings.Join(secondIDs, "\n") != strings.Join(firstIDs, "\n") {
		t.Fatalf("generated ID set changed after replay\nfirst:\n%s\nsecond:\n%s",
			strings.Join(firstIDs, "\n"), strings.Join(secondIDs, "\n"))
	}
}

func writeArchivedShapePendingLearning(t *testing.T, tmp, sourceID string) string {
	t.Helper()
	pendingDir := filepath.Join(tmp, ".agents", "knowledge", "pending")
	if err := os.MkdirAll(pendingDir, 0o755); err != nil {
		t.Fatalf("mkdir pending: %v", err)
	}
	path := filepath.Join(pendingDir, sourceID+".md")
	content := strings.Join([]string{
		"---",
		"date: 2026-04-27",
		"type: learning",
		"source: 96469d52-3777-401e-9961-02a44012fff0",
		"---",
		"",
		"# Learning: anti-pattern pre-flight and design brief rules",
		"",
		"**ID**: " + sourceID,
		"**Category**: learning",
		"**Confidence**: medium",
		"",
		"Anti-pattern pre-flight, design briefs for rewrites, issue granularity rules, operationalization heuristics, conformance checks, and schema strictness pre-flight.",
		"",
		"## Source",
		"",
		"- **Session**: 96469d52-3777-401e-9961-02a44012fff0",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write pending: %v", err)
	}
	return path
}

func assertNoAmplifiedReplayID(t *testing.T, tmp, sourceID string) {
	t.Helper()
	repeated := sourceID + "-" + sourceID
	for _, item := range collectCloseLoopGeneratedIDs(t, tmp) {
		if strings.Contains(item, repeated) {
			t.Fatalf("generated amplified replay ID containing %q: %s", repeated, item)
		}
	}
}

func collectCloseLoopGeneratedIDs(t *testing.T, tmp string) []string {
	t.Helper()
	var ids []string
	roots := []string{
		filepath.Join(tmp, ".agents", "pool"),
		filepath.Join(tmp, ".agents", "learnings"),
		filepath.Join(tmp, ".agents", "patterns"),
	}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Ext(path) != ".md" {
				return err
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			rel, relErr := filepath.Rel(tmp, path)
			if relErr != nil {
				return relErr
			}
			ids = append(ids, rel)
			for _, line := range strings.Split(string(data), "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "id: ") || strings.HasPrefix(trimmed, "**ID**:") {
					ids = append(ids, rel+"|"+trimmed)
				}
			}
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			t.Fatalf("walk generated IDs under %s: %v", root, err)
		}
	}
	sort.Strings(ids)
	return ids
}
