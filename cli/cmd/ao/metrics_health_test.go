package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
	"github.com/spf13/cobra"
)

func setupHealthTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Create standard directories
	for _, sub := range []string{
		".agents/learnings",
		".agents/patterns",
		".agents/constraints",
		".agents/ao",
	} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func writeHealthCitations(t *testing.T, dir string, events []types.CitationEvent) {
	t.Helper()
	citDir := filepath.Join(dir, ".agents", "ao")
	if err := os.MkdirAll(citDir, 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(citDir, "citations.jsonl"))
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

func TestMetricsHealth_EmptyInputPaths(t *testing.T) {
	dir := t.TempDir()

	hm, err := computeHealthMetrics(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if hm.Sigma != 0 {
		t.Errorf("expected sigma=0, got %f", hm.Sigma)
	}
	if hm.Rho != 0 {
		t.Errorf("expected rho=0, got %f", hm.Rho)
	}
	if hm.Delta != 0 {
		t.Errorf("expected delta=0, got %f", hm.Delta)
	}
	if hm.EscapeVelocity {
		t.Error("expected escape_velocity=false for empty input path")
	}
	if hm.KnowledgeStock.Total != 0 {
		t.Errorf("expected knowledge_stock.total=0, got %d", hm.KnowledgeStock.Total)
	}
	if hm.LoopDominance.Dominant != "B1" {
		t.Errorf("expected loop_dominance.dominant=B1, got %s", hm.LoopDominance.Dominant)
	}
}

func TestMetricsHealth_DirtyArtifactPaths(t *testing.T) {
	dir := setupHealthTestDir(t)
	now := time.Now()

	learningsDir := filepath.Join(dir, ".agents", "learnings")
	patternsDir := filepath.Join(dir, ".agents", "patterns")
	if err := os.WriteFile(filepath.Join(learningsDir, "kept.md"), []byte("# Kept"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(learningsDir, "junk.txt"), []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(patternsDir, "pattern.jsonl"), []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}

	writeHealthCitations(t, dir, []types.CitationEvent{
		{
			ArtifactPath: filepath.Join(dir, ".agents", "learnings", "kept.md"),
			SessionID:    "session-1",
			CitedAt:      now.Add(-time.Hour),
			CitationType: "reference",
		},
		{
			ArtifactPath: filepath.Join(dir, ".agents", "learnings", "missing.md"),
			SessionID:    "session-1",
			CitedAt:      now.Add(-2 * time.Hour),
			CitationType: "reference",
		},
		{
			ArtifactPath: filepath.Join(learningsDir, "junk.txt"),
			SessionID:    "session-2",
			CitedAt:      now.Add(-3 * time.Hour),
			CitationType: "reference",
		},
		{
			ArtifactPath: filepath.Join(dir, ".agents", "evolve", "ignore-me.md"),
			SessionID:    "session-3",
			CitedAt:      now.Add(-4 * time.Hour),
			CitationType: "reference",
		},
	})

	hm, err := computeHealthMetrics(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Total retrievable artifacts: kept.md + pattern.jsonl = 2.
	// Citations include kept.md, missing.md, and junk.txt from retrievable dirs,
	// plus one non-retrievable path in .agents/evolve.
	if hm.Sigma < 0.99 || hm.Sigma > 1.01 {
		t.Errorf("expected sigma~1.0 from retrievable citations, got %f", hm.Sigma)
	}
	if hm.Rho < 0.99 || hm.Rho > 1.01 {
		t.Errorf("expected rho~1.0 with all surfaced artifacts evidenced, got %f", hm.Rho)
	}
}

func TestMetricsHealth_EmptyRepo(t *testing.T) {
	dir := t.TempDir()
	// No .agents/ directory at all

	hm, err := computeHealthMetrics(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if hm.Sigma != 0 {
		t.Errorf("expected sigma=0, got %f", hm.Sigma)
	}
	if hm.Rho != 0 {
		t.Errorf("expected rho=0, got %f", hm.Rho)
	}
	if hm.Delta != 0 {
		t.Errorf("expected delta=0, got %f", hm.Delta)
	}
	if hm.EscapeVelocity {
		t.Error("expected escape_velocity=false for empty repo")
	}
	if hm.KnowledgeStock.Total != 0 {
		t.Errorf("expected knowledge_stock.total=0, got %d", hm.KnowledgeStock.Total)
	}
	if hm.LoopDominance.Dominant != "B1" {
		t.Errorf("expected loop_dominance.dominant=B1, got %s", hm.LoopDominance.Dominant)
	}
}

func TestMetricsHealth_WithCitations(t *testing.T) {
	dir := setupHealthTestDir(t)
	now := time.Now()

	// Create some learnings files
	learningsDir := filepath.Join(dir, ".agents", "learnings")
	for _, name := range []string{"a.md", "b.md", "c.md", "d.md"} {
		if err := os.WriteFile(filepath.Join(learningsDir, name), []byte("# Learning"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Create citations: 2 of 4 learnings cited across 2 sessions
	citations := []types.CitationEvent{
		{
			ArtifactPath: filepath.Join(dir, ".agents", "learnings", "a.md"),
			SessionID:    "session-1",
			CitedAt:      now.AddDate(0, 0, -2),
			CitationType: "reference",
		},
		{
			ArtifactPath: filepath.Join(dir, ".agents", "learnings", "a.md"),
			SessionID:    "session-2",
			CitedAt:      now.AddDate(0, 0, -1),
			CitationType: "applied",
		},
		{
			ArtifactPath: filepath.Join(dir, ".agents", "learnings", "b.md"),
			SessionID:    "session-1",
			CitedAt:      now.AddDate(0, 0, -2),
			CitationType: "reference",
		},
	}
	writeHealthCitations(t, dir, citations)

	hm, err := computeHealthMetrics(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// sigma = unique cited (a.md, b.md = 2) / total retrievable (4 learnings + 0 patterns = 4) = 0.5
	if hm.Sigma < 0.49 || hm.Sigma > 0.51 {
		t.Errorf("expected sigma~0.5, got %f", hm.Sigma)
	}

	// rho = evidence-backed surfaced (a.md, b.md = 2) / surfaced (2) = 1.0
	if hm.Rho < 0.99 || hm.Rho > 1.01 {
		t.Errorf("expected rho~1.0, got %f", hm.Rho)
	}

	// Knowledge stock
	if hm.KnowledgeStock.Learnings != 4 {
		t.Errorf("expected 4 learnings, got %d", hm.KnowledgeStock.Learnings)
	}
}

func TestMetricsHealth_FindingsParticipateInStockAndRetrieval(t *testing.T) {
	dir := setupHealthTestDir(t)
	now := time.Now()

	learningsDir := filepath.Join(dir, ".agents", "learnings")
	findingsDir := filepath.Join(dir, ".agents", SectionFindings)
	if err := os.MkdirAll(findingsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(learningsDir, "learn.md"), []byte("# L"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(findingsDir, "finding.md"), []byte("# F"), 0o644); err != nil {
		t.Fatal(err)
	}

	writeHealthCitations(t, dir, []types.CitationEvent{
		{
			ArtifactPath: filepath.Join(dir, ".agents", SectionFindings, "finding.md"),
			SessionID:    "s1",
			CitedAt:      now.Add(-time.Hour),
		},
	})

	hm, err := computeHealthMetrics(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hm.KnowledgeStock.Findings != 1 {
		t.Fatalf("expected 1 finding, got %d", hm.KnowledgeStock.Findings)
	}
	if hm.KnowledgeStock.Total != 2 {
		t.Fatalf("expected total knowledge stock 2, got %d", hm.KnowledgeStock.Total)
	}
	if hm.Sigma < 0.49 || hm.Sigma > 0.51 {
		t.Fatalf("expected sigma~0.5 with 1 cited finding / 2 retrievable artifacts, got %f", hm.Sigma)
	}
}

func TestMetricsHealth_EscapeVelocity_Positive(t *testing.T) {
	dir := setupHealthTestDir(t)
	now := time.Now()

	// Create 2 very recent learnings (low delta => low threshold)
	learningsDir := filepath.Join(dir, ".agents", "learnings")
	for _, name := range []string{"x.md", "y.md"} {
		if err := os.WriteFile(filepath.Join(learningsDir, name), []byte("# L"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Cite both heavily across multiple sessions => high sigma, high rho
	var citations []types.CitationEvent
	for i := 0; i < 5; i++ {
		citations = append(citations, types.CitationEvent{
			ArtifactPath: filepath.Join(dir, ".agents", "learnings", "x.md"),
			SessionID:    "s" + string(rune('1'+i)),
			CitedAt:      now.Add(-time.Duration(i) * time.Hour),
		})
		citations = append(citations, types.CitationEvent{
			ArtifactPath: filepath.Join(dir, ".agents", "learnings", "y.md"),
			SessionID:    "s" + string(rune('1'+i)),
			CitedAt:      now.Add(-time.Duration(i) * time.Hour),
		})
	}
	writeHealthCitations(t, dir, citations)

	hm, err := computeHealthMetrics(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// sigma should be 1.0 (both cited / both exist)
	// rho should be high (10 citations / 2 artifacts = 5.0)
	// delta should be very low (files just created, ~0 days)
	// So sigma*rho (1.0 * 5.0 = 5.0) > delta/100 (~0) => compounding
	if !hm.EscapeVelocity {
		t.Errorf("expected escape_velocity=true (compounding), got false; sigma=%f rho=%f delta=%f sigmaRho=%f threshold=%f",
			hm.Sigma, hm.Rho, hm.Delta, hm.Sigma*hm.Rho, hm.Delta/100.0)
	}
}

func TestMetricsHealth_EscapeVelocity_Negative(t *testing.T) {
	dir := setupHealthTestDir(t)

	// Create 10 old learnings with no citations at all => high delta, zero sigma*rho
	learningsDir := filepath.Join(dir, ".agents", "learnings")
	for i := 0; i < 10; i++ {
		name := filepath.Join(learningsDir, "old-"+string(rune('a'+i))+".md")
		if err := os.WriteFile(name, []byte("# Old Learning"), 0o644); err != nil {
			t.Fatal(err)
		}
		// Backdate the file to 60 days ago
		oldTime := time.Now().AddDate(0, 0, -60)
		if err := os.Chtimes(name, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}
	}

	// No citations => sigma=0, rho=0
	hm, err := computeHealthMetrics(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// sigma*rho = 0 < delta/100 (~0.6) => decaying
	if hm.EscapeVelocity {
		t.Errorf("expected escape_velocity=false (decaying), got true; sigma=%f rho=%f delta=%f",
			hm.Sigma, hm.Rho, hm.Delta)
	}
	if hm.Delta < 50 {
		t.Errorf("expected delta >= 50 days for 60-day-old files, got %f", hm.Delta)
	}
}

func TestMetricsHealth_JSONOutput(t *testing.T) {
	dir := setupHealthTestDir(t)
	now := time.Now()

	// Create a learning and cite it
	learningsDir := filepath.Join(dir, ".agents", "learnings")
	if err := os.WriteFile(filepath.Join(learningsDir, "test.md"), []byte("# Test"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeHealthCitations(t, dir, []types.CitationEvent{
		{
			ArtifactPath: filepath.Join(dir, ".agents", "learnings", "test.md"),
			SessionID:    "s1",
			CitedAt:      now.AddDate(0, 0, -1),
		},
	})

	// Save and restore global state
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	oldOutput := output
	output = "json"
	defer func() { output = oldOutput }()

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := runMetricsHealth(cmd, nil); err != nil {
		t.Fatalf("runMetricsHealth failed: %v", err)
	}

	// Parse JSON output
	var parsed healthMetrics
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("expected valid JSON, got: %q (%v)", out.String(), err)
	}

	// Verify schema fields are present
	if parsed.Sigma < 0 {
		t.Error("expected sigma >= 0")
	}
	if parsed.KnowledgeStock.Total < 1 {
		t.Errorf("expected knowledge_stock.total >= 1, got %d", parsed.KnowledgeStock.Total)
	}
	if parsed.LoopDominance.Dominant == "" {
		t.Error("expected loop_dominance.dominant to be non-empty")
	}

	// Verify escape_velocity field exists (JSON will have it even if false)
	raw := make(map[string]json.RawMessage)
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
		t.Fatalf("failed to parse raw JSON: %v", err)
	}
	if _, ok := raw["escape_velocity"]; !ok {
		t.Error("expected escape_velocity field in JSON output")
	}
	if _, ok := raw["knowledge_stock"]; !ok {
		t.Error("expected knowledge_stock field in JSON output")
	}
	if _, ok := raw["loop_dominance"]; !ok {
		t.Error("expected loop_dominance field in JSON output")
	}
}

func TestMetricsHealth_PrintPath(t *testing.T) {
	dir := setupHealthTestDir(t)
	now := time.Now()

	learningsDir := filepath.Join(dir, ".agents", "learnings")
	if err := os.WriteFile(filepath.Join(learningsDir, "print.md"), []byte("# Print"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeHealthCitations(t, dir, []types.CitationEvent{
		{
			ArtifactPath: filepath.Join(learningsDir, "print.md"),
			SessionID:    "s1",
			CitedAt:      now,
		},
	})

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := runMetricsHealth(cmd, nil); err != nil {
		t.Fatalf("runMetricsHealth failed: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Flywheel Health") {
		t.Fatalf("expected table output containing Flywheel Health, got %q", got)
	}
	if !strings.Contains(got, "LOOP DOMINANCE:") {
		t.Fatalf("expected LOOP DOMINANCE section, got %q", got)
	}
	if strings.HasPrefix(strings.TrimSpace(got), "{") {
		t.Fatalf("expected text output, got JSON-like output %q", got)
	}
}

func TestMetricsHealth_TablelessRun(t *testing.T) {
	dir := setupHealthTestDir(t)
	learningsDir := filepath.Join(dir, ".agents", "learnings")
	if err := os.WriteFile(filepath.Join(learningsDir, "print.jsonl"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	oldOutput := output
	defer func() { output = oldOutput }()
	cmd := &cobra.Command{}

	output = "json"
	var jsonOut bytes.Buffer
	cmd.SetOut(&jsonOut)
	if err := runMetricsHealth(cmd, nil); err != nil {
		t.Fatalf("runMetricsHealth json failed: %v", err)
	}
	raw := make(map[string]json.RawMessage)
	if err := json.Unmarshal(jsonOut.Bytes(), &raw); err != nil {
		t.Fatalf("expected valid JSON output: %v", err)
	}
	if _, ok := raw["escape_velocity"]; !ok {
		t.Fatalf("expected escape_velocity field in JSON output")
	}

	output = "table"
	var tableOut bytes.Buffer
	cmd.SetOut(&tableOut)
	if err := runMetricsHealth(cmd, nil); err != nil {
		t.Fatalf("runMetricsHealth table failed: %v", err)
	}
	if !strings.Contains(tableOut.String(), "RETRIEVAL:") {
		t.Fatalf("expected table output, got: %q", tableOut.String())
	}
}

func TestMetricsHealth_LoadCycleHistory(t *testing.T) {
	dir := t.TempDir()
	evolveDir := filepath.Join(dir, ".agents", "evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	historyPath := filepath.Join(evolveDir, "cycle-history.jsonl")
	data := `{"cycle":1,"status":"pass"}` + "\n" + "invalid-json" + "\n" + `{"cycle":2,"status":"fail"}` + "\n"
	if err := os.WriteFile(historyPath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := loadCycleHistory(dir)
	if len(entries) != 2 {
		t.Fatalf("expected 2 parseable entries, got %d", len(entries))
	}
}

func TestMetricsHealth_LoadCycleHistoryMissingFile(t *testing.T) {
	if got := loadCycleHistory(t.TempDir()); got != nil {
		t.Fatalf("expected nil history for missing file, got %v", got)
	}
}

func TestComputeLoopDominance_StaleUsesNinetyDayWindow(t *testing.T) {
	dir := setupHealthTestDir(t)
	learningsDir := filepath.Join(dir, ".agents", "learnings")

	oldPath := filepath.Join(learningsDir, "old-60d.md")
	if err := os.WriteFile(oldPath, []byte("# Old learning"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().AddDate(0, 0, -60)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	citations := []types.CitationEvent{
		{
			ArtifactPath: filepath.Join(dir, ".agents", "learnings", "recent.md"),
			SessionID:    "session-1",
			CitedAt:      time.Now(),
			CitationType: "reference",
		},
	}
	writeHealthCitations(t, dir, citations)

	ld := computeLoopDominance(dir, citations)
	if ld.B1 != 0 {
		t.Fatalf("expected B1=0 for 60-day-old uncited learning with 90-day stale threshold, got %f", ld.B1)
	}
}

func TestComputeLoopDominance_StaleAndNewBoundaries(t *testing.T) {
	dir := setupHealthTestDir(t)
	learningsDir := filepath.Join(dir, ".agents", "learnings")
	now := time.Now()

	recent1 := filepath.Join(learningsDir, "recent-1.md")
	recent2 := filepath.Join(learningsDir, "recent-2.md")
	stale1 := filepath.Join(learningsDir, "stale-1.md")
	stale2 := filepath.Join(learningsDir, "stale-2.md")
	stale3 := filepath.Join(learningsDir, "stale-3.md")

	for _, path := range []string{recent1, recent2, stale1, stale2, stale3} {
		if err := os.WriteFile(path, []byte("# artifact"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{stale1, stale2, stale3} {
		staleTime := now.AddDate(0, 0, -120)
		if err := os.Chtimes(path, staleTime, staleTime); err != nil {
			t.Fatal(err)
		}
	}

	citations := []types.CitationEvent{
		{
			ArtifactPath: recent1,
			SessionID:    "session-1",
			CitedAt:      now,
		},
		{
			ArtifactPath: recent2,
			SessionID:    "session-2",
			CitedAt:      now,
		},
	}

	ldMixed := computeLoopDominance(dir, citations)
	if ldMixed.R1 != 1 {
		t.Fatalf("expected R1=1 for 2 recent artifacts over 2 sessions, got %f", ldMixed.R1)
	}
	if ldMixed.B1 != 1.5 {
		t.Fatalf("expected B1=1.5 for mixed stale/new boundary case, got %f", ldMixed.B1)
	}
	if ldMixed.Dominant != "B1" {
		t.Fatalf("expected dominant=B1, got %s", ldMixed.Dominant)
	}

	ldZero := computeLoopDominance(dir, nil)
	if ldZero.Dominant != "B1" {
		t.Fatalf("expected zero-session dominant=B1, got %s", ldZero.Dominant)
	}
	if ldZero.R1 != 0 || ldZero.B1 != 0 {
		t.Fatalf("expected zero-session R1/B1=0, got R1=%f B1=%f", ldZero.R1, ldZero.B1)
	}
}

func TestComputeHealthSigmaRho_AllEmptySessionIDs(t *testing.T) {
	// When all citations have empty session IDs, lastNSessions returns empty,
	// so computeHealthSigmaRho should return (0, 0).
	dir := t.TempDir()
	citations := []types.CitationEvent{
		{ArtifactPath: "learnings/foo.md", SessionID: "", CitedAt: time.Now()},
		{ArtifactPath: "learnings/bar.md", SessionID: "", CitedAt: time.Now()},
	}
	sigma, rho := computeHealthSigmaRho(dir, citations)
	if sigma != 0 {
		t.Errorf("expected sigma=0 for empty-session citations, got %f", sigma)
	}
	if rho != 0 {
		t.Errorf("expected rho=0 for empty-session citations, got %f", rho)
	}
}

func TestLastNSessions_AllEmptySessionIDs(t *testing.T) {
	citations := []types.CitationEvent{
		{SessionID: "", CitedAt: time.Now()},
		{SessionID: "", CitedAt: time.Now()},
	}
	result := lastNSessions(citations, 5)
	if len(result) != 0 {
		t.Errorf("expected 0 sessions for all-empty IDs, got %d", len(result))
	}
}

func TestLastNSessions_MoreSessionsThanN(t *testing.T) {
	now := time.Now()
	citations := []types.CitationEvent{
		{SessionID: "s1", CitedAt: now.Add(-3 * time.Hour)},
		{SessionID: "s2", CitedAt: now.Add(-2 * time.Hour)},
		{SessionID: "s3", CitedAt: now.Add(-1 * time.Hour)},
	}
	result := lastNSessions(citations, 2)
	if len(result) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(result))
	}
	// Most recent first
	if result[0] != "s3" {
		t.Errorf("expected first session='s3', got %q", result[0])
	}
	if result[1] != "s2" {
		t.Errorf("expected second session='s2', got %q", result[1])
	}
}
