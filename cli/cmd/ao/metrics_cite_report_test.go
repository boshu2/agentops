package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
	"github.com/spf13/cobra"
)

func writeCitationsJSONL(t *testing.T, dir string, events []types.CitationEvent) {
	t.Helper()
	citPath := filepath.Join(dir, ".agents", "ao")
	if err := os.MkdirAll(citPath, 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(citPath, "citations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, e := range events {
		if err := enc.Encode(e); err != nil {
			t.Fatal(err)
		}
	}
}

func sampleCitations(now time.Time) []types.CitationEvent {
	return []types.CitationEvent{
		{ArtifactPath: "/tmp/test/.agents/learnings/a.md", WorkspacePath: "/tmp/test", SessionID: "s1", CitedAt: now.AddDate(0, 0, -5), FeedbackGiven: true},
		{ArtifactPath: "/tmp/test/.agents/learnings/a.md", WorkspacePath: "/tmp/test", SessionID: "s2", CitedAt: now.AddDate(0, 0, -3), FeedbackGiven: false},
		{ArtifactPath: "/tmp/test/.agents/learnings/b.md", WorkspacePath: "/tmp/other", SessionID: "s1", CitedAt: now.AddDate(0, 0, -2), FeedbackGiven: true},
		{ArtifactPath: "/tmp/test/.agents/patterns/c.md", WorkspacePath: "/tmp/test", SessionID: "s3", CitedAt: now.AddDate(0, 0, -1), FeedbackGiven: false},
	}
}

func TestCiteReportHumanOutput(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	citations := sampleCitations(now)
	writeCitationsJSONL(t, dir, citations)

	// Create learnings dir with an uncited file
	learningsDir := filepath.Join(dir, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(learningsDir, "uncited.md"), []byte("# Uncited"), 0o644); err != nil {
		t.Fatal(err)
	}

	start := now.AddDate(0, 0, -30)
	stats := filterCitationsForPeriod(citations, start, now)
	report := buildCiteReport(dir, stats.citations, citations, 30, start, now)

	if report.TotalCitations != 4 {
		t.Errorf("expected 4 total citations, got %d", report.TotalCitations)
	}
	if report.UniqueArtifacts != 3 {
		t.Errorf("expected 3 unique artifacts, got %d", report.UniqueArtifacts)
	}
	if report.UniqueSessions != 3 {
		t.Errorf("expected 3 unique sessions, got %d", report.UniqueSessions)
	}
	if report.UniqueWorkspaces != 2 {
		t.Errorf("expected 2 unique workspaces, got %d", report.UniqueWorkspaces)
	}
	if report.DedupedCitations != 4 {
		t.Errorf("expected 4 deduped citations, got %d", report.DedupedCitations)
	}
	// a.md is cited by s1 and s2 => hit count = 1
	if report.HitCount != 1 {
		t.Errorf("expected hit count 1, got %d", report.HitCount)
	}
	if report.FeedbackGiven != 2 {
		t.Errorf("expected 2 feedback given, got %d", report.FeedbackGiven)
	}
	if len(report.UncitedLearnings) != 1 {
		t.Errorf("expected 1 uncited learning, got %d", len(report.UncitedLearnings))
	}
}

func TestCiteReportJSON(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeCitationsJSONL(t, dir, sampleCitations(now))

	start := now.AddDate(0, 0, -30)
	citations := sampleCitations(now)
	stats := filterCitationsForPeriod(citations, start, now)
	report := buildCiteReport(dir, stats.citations, citations, 30, start, now)

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("failed to marshal report: %v", err)
	}

	var parsed citeReportData
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal report: %v", err)
	}
	if parsed.TotalCitations != 4 {
		t.Errorf("expected 4 total citations in JSON, got %d", parsed.TotalCitations)
	}
	if parsed.Staleness == nil {
		t.Error("expected staleness map in JSON output")
	}
}

func TestCiteReportEmpty(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	start := now.AddDate(0, 0, -30)

	// No citations at all
	var empty []types.CitationEvent
	report := buildCiteReport(dir, empty, empty, 30, start, now)

	if report.TotalCitations != 0 {
		t.Errorf("expected 0 total citations, got %d", report.TotalCitations)
	}
	if report.UniqueArtifacts != 0 {
		t.Errorf("expected 0 unique artifacts, got %d", report.UniqueArtifacts)
	}
	if report.HitRate != 0 {
		t.Errorf("expected 0 hit rate, got %f", report.HitRate)
	}
}

func TestCiteReportDaysFilter(t *testing.T) {
	now := time.Now()
	citations := []types.CitationEvent{
		{ArtifactPath: "recent.md", SessionID: "s1", CitedAt: now.AddDate(0, 0, -3)},
		{ArtifactPath: "old.md", SessionID: "s2", CitedAt: now.AddDate(0, 0, -20)},
		{ArtifactPath: "ancient.md", SessionID: "s3", CitedAt: now.AddDate(0, 0, -45)},
	}

	// 7-day window: only recent.md
	start7 := now.AddDate(0, 0, -7)
	stats7 := filterCitationsForPeriod(citations, start7, now)
	if len(stats7.citations) != 1 {
		t.Errorf("7-day filter: expected 1, got %d", len(stats7.citations))
	}

	// 30-day window: recent.md + old.md
	start30 := now.AddDate(0, 0, -30)
	stats30 := filterCitationsForPeriod(citations, start30, now)
	if len(stats30.citations) != 2 {
		t.Errorf("30-day filter: expected 2, got %d", len(stats30.citations))
	}

	// 60-day window: all three
	start60 := now.AddDate(0, 0, -60)
	stats60 := filterCitationsForPeriod(citations, start60, now)
	if len(stats60.citations) != 3 {
		t.Errorf("60-day filter: expected 3, got %d", len(stats60.citations))
	}
}

func TestRunMetricsCiteReport_RespectsGlobalOutputJSON(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeCitationsJSONL(t, dir, sampleCitations(now))

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	oldOutput := output
	output = "json"
	defer func() { output = oldOutput }()

	cmd := &cobra.Command{}
	cmd.Flags().Int("days", 30, "Period in days")
	cmd.Flags().Bool("json", false, "Output as JSON")

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := runMetricsCiteReport(cmd, nil); err != nil {
		t.Fatalf("runMetricsCiteReport failed: %v", err)
	}

	var parsed citeReportData
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("expected JSON output, got: %q (%v)", out.String(), err)
	}
	if parsed.TotalCitations == 0 {
		t.Fatalf("expected citation report data, got zero citations")
	}
}

func TestRunMetricsCiteReport_EmptyWithJSONOutput(t *testing.T) {
	dir := t.TempDir()

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	oldOutput := output
	output = "json"
	defer func() { output = oldOutput }()

	cmd := &cobra.Command{}
	cmd.Flags().Int("days", 30, "Period in days")
	cmd.Flags().Bool("json", false, "Output as JSON")

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := runMetricsCiteReport(cmd, nil); err != nil {
		t.Fatalf("runMetricsCiteReport failed: %v", err)
	}

	var parsed citeReportData
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("expected JSON output for empty dataset, got: %q (%v)", out.String(), err)
	}
	if parsed.TotalCitations != 0 {
		t.Fatalf("expected zero citations, got %d", parsed.TotalCitations)
	}
	if parsed.Staleness["90d"] != 0 {
		t.Fatalf("expected zero stale artifacts for empty dataset, got %d", parsed.Staleness["90d"])
	}
}

func TestPrintCiteReport_FullReport(t *testing.T) {
	now := time.Now()
	r := citeReportData{
		TotalCitations:  42,
		UniqueArtifacts: 10,
		UniqueSessions:  5,
		HitRate:         0.6,
		HitCount:        6,
		TopArtifacts: []artifactCount{
			{Path: "learnings/a.md", Count: 15},
			{Path: "learnings/b.md", Count: 8},
		},
		UncitedLearnings: []string{"uncited1.md", "uncited2.md"},
		Staleness:        map[string]int{"30d": 2, "60d": 5, "90d": 7},
		FeedbackTotal:    20,
		FeedbackGiven:    12,
		FeedbackRate:     0.6,
		Days:             30,
		PeriodStart:      now.AddDate(0, 0, -30),
		PeriodEnd:        now,
	}

	var buf bytes.Buffer
	printCiteReport(&buf, r)
	out := buf.String()

	checks := []string{
		"Citation Report",
		"Total citations:     42",
		"Unique artifacts:    10",
		"Unique sessions:     5",
		"Hit rate",
		"60%",
		"TOP CITED ARTIFACTS:",
		"learnings/a.md (15)",
		"learnings/b.md (8)",
		"UNCITED LEARNINGS:",
		"uncited1.md",
		"uncited2.md",
		"STALENESS:",
		"Not cited in 30d: 2",
		"Not cited in 60d: 5",
		"Not cited in 90d: 7",
		"FEEDBACK:",
		"Closure rate: 60%",
	}
	for _, want := range checks {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestPrintCiteReport_EmptyTopAndUncited(t *testing.T) {
	now := time.Now()
	r := citeReportData{
		TotalCitations:  0,
		UniqueArtifacts: 0,
		UniqueSessions:  0,
		Staleness:       map[string]int{"30d": 0, "60d": 0, "90d": 0},
		Days:            7,
		PeriodStart:     now.AddDate(0, 0, -7),
		PeriodEnd:       now,
	}

	var buf bytes.Buffer
	printCiteReport(&buf, r)
	out := buf.String()

	if bytes.Contains([]byte(out), []byte("TOP CITED ARTIFACTS:")) {
		t.Error("expected no TOP CITED section for empty artifacts")
	}
	if bytes.Contains([]byte(out), []byte("UNCITED LEARNINGS:")) {
		t.Error("expected no UNCITED section for empty list")
	}
	if !bytes.Contains([]byte(out), []byte("Total citations:     0")) {
		t.Error("expected zero total in output")
	}
}
