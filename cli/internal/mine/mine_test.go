package mine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Run orchestrator tests
// ---------------------------------------------------------------------------

func TestRun_DryRunSkipsSourcesAndWrites(t *testing.T) {
	tmp := t.TempDir()
	called := false
	opts := RunOpts{
		Sources:   []string{"events"},
		Window:    1 * time.Hour,
		OutputDir: filepath.Join(tmp, "mine-out"),
		DryRun:    true,
		MineEventsFn: func(cwd string, window time.Duration) (*EventsFindings, error) {
			called = true
			return &EventsFindings{}, nil
		},
	}

	report, err := Run(tmp, opts)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if report == nil {
		t.Fatal("Run returned nil report for dry run")
	}
	if called {
		t.Error("MineEventsFn should not be called on dry run")
	}
	// Dry run must not write the output directory.
	if _, err := os.Stat(opts.OutputDir); !os.IsNotExist(err) {
		t.Errorf("dry run should not create output dir, stat err = %v", err)
	}
	// Sources field should be populated even on dry run.
	if len(report.Sources) != 1 || report.Sources[0] != "events" {
		t.Errorf("Sources = %v, want [events]", report.Sources)
	}
}

func TestRun_ContextCanceledSkipsSourcesAndWrites(t *testing.T) {
	tmp := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	opts := RunOpts{
		Context:   ctx,
		Sources:   []string{"agents"},
		Window:    time.Hour,
		OutputDir: filepath.Join(tmp, "mine-out"),
	}

	report, err := Run(tmp, opts)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v, want context.Canceled", err)
	}
	if report == nil {
		t.Fatal("Run returned nil report")
	}
	if _, err := os.Stat(opts.OutputDir); !os.IsNotExist(err) {
		t.Fatalf("canceled run should not create output dir, stat err = %v", err)
	}
}

func TestRun_AgentsOnly_WritesReport(t *testing.T) {
	tmp := t.TempDir()
	// Seed an orphaned research file.
	researchDir := filepath.Join(tmp, ".agents", "research")
	if err := os.MkdirAll(researchDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(researchDir, "orphan.md"),
		[]byte("# Orphan"), 0o644); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(tmp, "mine-out")
	fixedTime := time.Date(2026, 4, 9, 14, 0, 0, 0, time.UTC)
	opts := RunOpts{
		Sources:   []string{"agents"},
		Window:    1 * time.Hour,
		OutputDir: outDir,
		Now:       func() time.Time { return fixedTime },
	}

	report, err := Run(tmp, opts)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if report.Agents == nil {
		t.Fatal("report.Agents should be populated")
	}
	if report.Agents.TotalResearch != 1 {
		t.Errorf("TotalResearch = %d, want 1", report.Agents.TotalResearch)
	}
	if len(report.Agents.OrphanedResearch) != 1 {
		t.Errorf("OrphanedResearch = %v, want 1 entry", report.Agents.OrphanedResearch)
	}

	// Expect the dated + latest JSON files.
	datedPath := filepath.Join(outDir, "2026-04-09-14.json")
	latestPath := filepath.Join(outDir, "latest.json")
	if _, err := os.Stat(datedPath); err != nil {
		t.Errorf("dated report missing: %v", err)
	}
	if _, err := os.Stat(latestPath); err != nil {
		t.Errorf("latest report missing: %v", err)
	}

	// The written JSON should round-trip to the same shape.
	raw, readErr := os.ReadFile(latestPath)
	if readErr != nil {
		t.Fatalf("read latest: %v", readErr)
	}
	var roundTrip Report
	if err := json.Unmarshal(raw, &roundTrip); err != nil {
		t.Fatalf("unmarshal latest: %v", err)
	}
	if roundTrip.Agents == nil || roundTrip.Agents.TotalResearch != 1 {
		t.Errorf("round-trip Agents mismatch: %+v", roundTrip.Agents)
	}
}

func TestRun_EventsCallbackInvoked(t *testing.T) {
	tmp := t.TempDir()
	wantFindings := &EventsFindings{
		RunsScanned: 3,
		TotalEvents: 42,
	}
	var gotCwd string
	var gotWindow time.Duration
	opts := RunOpts{
		Sources:   []string{"events"},
		Window:    2 * time.Hour,
		OutputDir: filepath.Join(tmp, "mine-out"),
		MineEventsFn: func(cwd string, window time.Duration) (*EventsFindings, error) {
			gotCwd = cwd
			gotWindow = window
			return wantFindings, nil
		},
	}

	report, err := Run(tmp, opts)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if gotCwd != tmp {
		t.Errorf("callback cwd = %q, want %q", gotCwd, tmp)
	}
	if gotWindow != 2*time.Hour {
		t.Errorf("callback window = %v, want 2h", gotWindow)
	}
	if report.Events == nil {
		t.Fatal("report.Events should be set")
	}
	if report.Events.RunsScanned != 3 || report.Events.TotalEvents != 42 {
		t.Errorf("Events = %+v, want RunsScanned=3 TotalEvents=42", report.Events)
	}
}

func TestRun_EventsCallbackNilIsNoop(t *testing.T) {
	tmp := t.TempDir()
	opts := RunOpts{
		Sources:      []string{"events"},
		Window:       1 * time.Hour,
		OutputDir:    filepath.Join(tmp, "mine-out"),
		MineEventsFn: nil, // no callback
	}

	report, err := Run(tmp, opts)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if report.Events != nil {
		t.Errorf("report.Events should be nil when callback missing, got %+v", report.Events)
	}
}

func TestRun_EventsCallbackErrorIsSoftFail(t *testing.T) {
	tmp := t.TempDir()
	var errBuf bytes.Buffer
	opts := RunOpts{
		Sources:   []string{"events"},
		Window:    1 * time.Hour,
		OutputDir: filepath.Join(tmp, "mine-out"),
		ErrOut:    &errBuf,
		MineEventsFn: func(cwd string, window time.Duration) (*EventsFindings, error) {
			return nil, errors.New("boom")
		},
	}

	report, err := Run(tmp, opts)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if report.Events != nil {
		t.Errorf("report.Events should stay nil on callback error")
	}
	if !strings.Contains(errBuf.String(), "warning: events source: boom") {
		t.Errorf("expected warning on ErrOut, got %q", errBuf.String())
	}
}

func TestRun_QuietSuppressesWarnings(t *testing.T) {
	tmp := t.TempDir()
	var errBuf bytes.Buffer
	opts := RunOpts{
		Sources:   []string{"events"},
		Window:    1 * time.Hour,
		OutputDir: filepath.Join(tmp, "mine-out"),
		Quiet:     true,
		ErrOut:    &errBuf,
		MineEventsFn: func(cwd string, window time.Duration) (*EventsFindings, error) {
			return nil, errors.New("boom")
		},
	}
	if _, err := Run(tmp, opts); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if errBuf.Len() != 0 {
		t.Errorf("Quiet should suppress warnings, got %q", errBuf.String())
	}
}

func TestRun_EmitWorkItemsWritesJSONL(t *testing.T) {
	tmp := t.TempDir()
	// Seed an orphaned research file to produce a work item.
	researchDir := filepath.Join(tmp, ".agents", "research")
	if err := os.MkdirAll(researchDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(researchDir, "stale.md"),
		[]byte("# stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := RunOpts{
		Sources:       []string{"agents"},
		Window:        1 * time.Hour,
		OutputDir:     filepath.Join(tmp, "mine-out"),
		EmitWorkItems: true,
	}

	if _, err := Run(tmp, opts); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	nextWorkPath := filepath.Join(tmp, ".agents", "rpi", "next-work.jsonl")
	data, err := os.ReadFile(nextWorkPath)
	if err != nil {
		t.Fatalf("read next-work.jsonl: %v", err)
	}
	if !strings.Contains(string(data), "stale.md") {
		t.Errorf("expected stale.md in work items, got: %s", string(data))
	}
	if !strings.Contains(string(data), "compile-mine") {
		t.Errorf("expected source_epic compile-mine, got: %s", string(data))
	}
}

func TestRun_DefaultsNowToTimeNow(t *testing.T) {
	tmp := t.TempDir()
	before := time.Now().UTC()
	opts := RunOpts{
		Sources:   []string{"agents"},
		Window:    1 * time.Hour,
		OutputDir: filepath.Join(tmp, "mine-out"),
	}
	report, err := Run(tmp, opts)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	after := time.Now().UTC().Add(1 * time.Second)
	if report.Timestamp.Before(before) || report.Timestamp.After(after) {
		t.Errorf("Timestamp = %v, want between %v and %v", report.Timestamp, before, after)
	}
}

// ---------------------------------------------------------------------------
// MineAgentsDir (moved helper) tests
// ---------------------------------------------------------------------------

func TestMineAgentsDir_NoResearchDir(t *testing.T) {
	tmp := t.TempDir()
	findings, err := MineAgentsDir(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if findings.TotalResearch != 0 || len(findings.OrphanedResearch) != 0 {
		t.Errorf("expected empty findings, got %+v", findings)
	}
}

func TestMineAgentsDir_OrphanedVsReferenced(t *testing.T) {
	tmp := t.TempDir()
	researchDir := filepath.Join(tmp, ".agents", "research")
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(researchDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(researchDir, "ref.md"), []byte("#"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(researchDir, "orphan.md"), []byte("#"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(learningsDir, "lesson.md"),
		[]byte("see ref.md"), 0o644); err != nil {
		t.Fatal(err)
	}

	findings, err := MineAgentsDir(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if findings.TotalResearch != 2 {
		t.Errorf("TotalResearch = %d, want 2", findings.TotalResearch)
	}
	if len(findings.OrphanedResearch) != 1 || findings.OrphanedResearch[0] != "orphan.md" {
		t.Errorf("OrphanedResearch = %v, want [orphan.md]", findings.OrphanedResearch)
	}
}

// ---------------------------------------------------------------------------
// CollectMineWorkItems tests
// ---------------------------------------------------------------------------

func TestCollectMineWorkItems_Empty(t *testing.T) {
	items := CollectMineWorkItems(&Report{})
	if len(items) != 0 {
		t.Errorf("items = %d, want 0 for empty report", len(items))
	}
}

func TestCollectMineWorkItems_HotspotsAndOrphans(t *testing.T) {
	r := &Report{
		Code: &CodeFindings{
			Hotspots: []ComplexityHotspot{
				{File: "a.go", Func: "big", Complexity: 25, RecentEdits: 10},
			},
		},
		Agents: &AgentsFindings{
			OrphanedResearch: []string{"old.md"},
		},
	}
	items := CollectMineWorkItems(r)
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	// Find hotspot item
	var hot, orphan *WorkItemEmit
	for i := range items {
		switch items[i].Type {
		case "refactor":
			hot = &items[i]
		case "knowledge-gap":
			orphan = &items[i]
		}
	}
	if hot == nil {
		t.Fatal("missing refactor work item from hotspot")
	}
	if hot.Severity != "high" {
		t.Errorf("hot.Severity = %q, want high", hot.Severity)
	}
	if hot.File != "a.go" || hot.Func != "big" {
		t.Errorf("hot file/func = %q/%q", hot.File, hot.Func)
	}
	if orphan == nil {
		t.Fatal("missing knowledge-gap work item from orphan")
	}
	if orphan.Severity != "medium" {
		t.Errorf("orphan.Severity = %q, want medium", orphan.Severity)
	}
}

// ---------------------------------------------------------------------------
// EmitWorkItems deduplication test
// ---------------------------------------------------------------------------

func TestEmitWorkItems_DedupesSecondCall(t *testing.T) {
	tmp := t.TempDir()
	r := &Report{
		Timestamp: time.Now().UTC(),
		Code: &CodeFindings{
			Hotspots: []ComplexityHotspot{
				{File: "a.go", Func: "bigFunc", Complexity: 30, RecentEdits: 5},
			},
		},
	}
	if err := EmitWorkItems(tmp, r); err != nil {
		t.Fatalf("first emit: %v", err)
	}
	if err := EmitWorkItems(tmp, r); err != nil {
		t.Fatalf("second emit: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(tmp, ".agents", "rpi", "next-work.jsonl"))
	if err != nil {
		t.Fatalf("read jsonl: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Errorf("lines = %d, want 1 (dedup should prevent re-emit)", len(lines))
	}
}

// ---------------------------------------------------------------------------
// ParseWindow / SplitSources smoke tests (light — cmd/ao has fuller coverage)
// ---------------------------------------------------------------------------

func TestParseWindow_Smoke(t *testing.T) {
	d, err := ParseWindow("7d")
	if err != nil {
		t.Fatalf("ParseWindow: %v", err)
	}
	if d != 7*24*time.Hour {
		t.Errorf("7d = %v, want 168h", d)
	}
}

func TestSplitSources_Smoke(t *testing.T) {
	got, err := SplitSources("git,agents")
	if err != nil {
		t.Fatalf("SplitSources: %v", err)
	}
	if len(got) != 2 || got[0] != "git" || got[1] != "agents" {
		t.Errorf("SplitSources = %v", got)
	}
}
