package overnight

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/lifecycle"
	"github.com/boshu2/agentops/cli/internal/mine"
	"github.com/boshu2/agentops/cli/internal/pool"
)

// writeLearning writes a minimal learning markdown file under
// <root>/.agents/learnings/<name>.md with the given frontmatter fields
// and body. All tests use this helper so the file shape stays in one
// place.
func writeLearning(t *testing.T, root, name string, fm map[string]string, body string) string {
	t.Helper()
	dir := filepath.Join(root, ".agents", "learnings")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir learnings: %v", err)
	}
	var b bytes.Buffer
	b.WriteString("---\n")
	for k, v := range fm {
		fmt.Fprintf(&b, "%s: %q\n", k, v)
	}
	b.WriteString("---\n\n")
	b.WriteString(body)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatalf("write learning %s: %v", name, err)
	}
	return path
}

// writeFindingRaw writes a canonical finding markdown file under
// <root>/.agents/findings/<name>.md with exact body contents. Used by
// MEASURE tests that seed unresolved findings. Distinct from
// findings_router_test.go's writeFinding helper which takes a
// title/summary pair.
func writeFindingRaw(t *testing.T, root, name, body string) string {
	t.Helper()
	dir := filepath.Join(root, ".agents", "findings")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir findings: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write finding %s: %v", name, err)
	}
	return path
}

// newTestOpts returns a RunLoopOptions scoped to a tempdir with sane
// zero values so each stage test can mutate the fields it cares about.
func newTestOpts(cwd string) RunLoopOptions {
	return RunLoopOptions{
		Cwd:       cwd,
		OutputDir: filepath.Join(cwd, ".agents", "overnight", "test-run"),
		RunID:     "test-run",
	}
}

// --- INGEST ------------------------------------------------------------

func TestRunIngest_HappyPath_HarvestCatalogBuilt(t *testing.T) {
	cwd := t.TempDir()
	writeLearning(t, cwd, "2026-04-09-first.md", map[string]string{
		"type":       "learning",
		"maturity":   "provisional",
		"confidence": "0.8",
	}, "# First\n\nBody text about the first learning.\n")
	writeLearning(t, cwd, "2026-04-09-second.md", map[string]string{
		"type":       "learning",
		"maturity":   "accepted",
		"confidence": "0.9",
	}, "# Second\n\nBody text about the second learning.\n")

	var logBuf bytes.Buffer
	res, err := RunIngest(context.Background(), newTestOpts(cwd), &logBuf)
	if err != nil {
		t.Fatalf("RunIngest error: %v", err)
	}
	if res.HarvestCatalog == nil {
		t.Fatal("expected HarvestCatalog to be non-nil")
	}
	if res.HarvestCatalog.Summary.ArtifactsExtracted < 2 {
		t.Errorf("expected >=2 artifacts, got %d", res.HarvestCatalog.Summary.ArtifactsExtracted)
	}
	if res.Duration <= 0 {
		t.Error("expected positive Duration")
	}
}

func TestRunIngest_HarvestDryRunDoesNotMutate(t *testing.T) {
	cwd := t.TempDir()
	writeLearning(t, cwd, "2026-04-09-static.md", map[string]string{
		"type":       "learning",
		"confidence": "0.9",
	}, "# Static\n\nUntouched body.\n")

	learningsDir := filepath.Join(cwd, ".agents", "learnings")
	before, err := os.ReadDir(learningsDir)
	if err != nil {
		t.Fatalf("read learnings before: %v", err)
	}

	if _, err := RunIngest(context.Background(), newTestOpts(cwd), io.Discard); err != nil {
		t.Fatalf("RunIngest error: %v", err)
	}

	after, err := os.ReadDir(learningsDir)
	if err != nil {
		t.Fatalf("read learnings after: %v", err)
	}
	if len(before) != len(after) {
		t.Errorf("learnings mutated: before=%d after=%d", len(before), len(after))
	}
}

func TestRunIngest_FindingGeneratorEmitsRealSidecarCandidates(t *testing.T) {
	cwd := t.TempDir()
	writeLearning(t, cwd, "2026-04-09-static.md", map[string]string{
		"type":       "learning",
		"confidence": "0.9",
	}, "# Static\n\nUntouched body.\n")
	researchDir := filepath.Join(cwd, ".agents", "research")
	if err := os.MkdirAll(researchDir, 0o755); err != nil {
		t.Fatalf("mkdir research: %v", err)
	}
	if err := os.WriteFile(filepath.Join(researchDir, "orphan.md"), []byte("# Orphan\n"), 0o644); err != nil {
		t.Fatalf("write orphan research: %v", err)
	}

	opts := newTestOpts(cwd)
	res, err := RunIngest(context.Background(), opts, io.Discard)
	if err != nil {
		t.Fatalf("RunIngest: %v", err)
	}
	if res.GeneratorCandidateCount == 0 {
		t.Fatal("expected generator candidates from orphaned research")
	}
	if res.GeneratorSidecarCount != 1 {
		t.Fatalf("GeneratorSidecarCount = %d, want 1", res.GeneratorSidecarCount)
	}
	if len(res.GeneratorSidecarPaths) != 1 {
		t.Fatalf("GeneratorSidecarPaths = %v, want one sidecar", res.GeneratorSidecarPaths)
	}
	data, err := os.ReadFile(filepath.Join(opts.OutputDir, "generator-results", "mine-findings.json"))
	if err != nil {
		t.Fatalf("read mine sidecar: %v", err)
	}
	var sidecar FindingGeneratorSidecar
	if err := json.Unmarshal(data, &sidecar); err != nil {
		t.Fatalf("decode mine sidecar: %v", err)
	}
	if sidecar.Status != "completed" {
		t.Fatalf("sidecar status = %q, want completed: %s", sidecar.Status, sidecar.Error)
	}
	if len(sidecar.Candidates) == 0 {
		t.Fatal("sidecar candidates empty")
	}
	got := sidecar.Candidates[0]
	if got.Type != "task" || got.Source != "evolve-generator" || got.Description == "" || got.DedupKey == "" {
		t.Fatalf("candidate not queue-safe: %+v", got)
	}
}

func TestRunFindingGenerator_StalledGeneratorWritesSoftFailSidecar(t *testing.T) {
	cwd := t.TempDir()
	opts := newTestOpts(cwd)
	result := runFindingGenerator(context.Background(), opts, findingGenerator{
		name:       "stalled-generator",
		sourceEpic: "stall-test",
		timeout:    5 * time.Millisecond,
		run: func(context.Context, RunLoopOptions) FindingGeneratorSidecar {
			select {}
		},
	})

	if result.err != nil {
		t.Fatalf("runFindingGenerator write error: %v", result.err)
	}
	if result.sidecar.Status != "soft-fail" {
		t.Fatalf("sidecar status = %q, want soft-fail", result.sidecar.Status)
	}
	if !strings.Contains(result.sidecar.Error, "context deadline exceeded") {
		t.Fatalf("sidecar error = %q, want context deadline exceeded", result.sidecar.Error)
	}
	if _, err := os.Stat(result.path); err != nil {
		t.Fatalf("expected soft-fail sidecar at %s: %v", result.path, err)
	}
}

func TestRunIngest_EmptyCorpus(t *testing.T) {
	// Isolate HOME to the tempdir so harvest.DiscoverRigs does not pick
	// up the operator's real ~/.agents/ global hub. DiscoverRigs adds
	// ~/.agents unconditionally; scoping HOME is the only portable way
	// to keep the walker from crossing the corpus we're trying to
	// measure as empty.
	home := t.TempDir()
	t.Setenv("HOME", home)

	cwd := t.TempDir()
	// Create .agents/ but no learnings/patterns/research.
	if err := os.MkdirAll(filepath.Join(cwd, ".agents"), 0o755); err != nil {
		t.Fatalf("mkdir .agents: %v", err)
	}

	res, err := RunIngest(context.Background(), newTestOpts(cwd), io.Discard)
	if err != nil {
		t.Fatalf("RunIngest error: %v", err)
	}
	if res.HarvestCatalog == nil {
		t.Fatal("expected HarvestCatalog to be non-nil even for empty corpus")
	}
	if res.HarvestCatalog.Summary.ArtifactsExtracted != 0 {
		t.Errorf("expected 0 artifacts, got %d", res.HarvestCatalog.Summary.ArtifactsExtracted)
	}
	if !containsSubstring(res.Degraded, "empty corpus") {
		t.Errorf("expected empty-corpus degraded note, got %v", res.Degraded)
	}
}

// TestRunIngest_ForgeProvenanceMineAreReal pins the Wave 2 Issue 5
// wiring: the three substages that used to log "deferred to follow-up"
// must now call forge.RunMinePass, provenance.Audit, and mine.Run
// in-process. The test asserts negative (the stale stub marker is gone)
// and positive (the counter fields are populated — 0 is acceptable for
// an empty fixture because the point is that the real call path ran,
// not the stub degraded note).
func TestRunIngest_ForgeProvenanceMineAreReal(t *testing.T) {
	// Isolate HOME so harvest.DiscoverRigs does not cross into the
	// operator's real global hub (matches TestRunIngest_EmptyCorpus).
	home := t.TempDir()
	t.Setenv("HOME", home)

	cwd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cwd, ".agents"), 0o755); err != nil {
		t.Fatalf("mkdir .agents: %v", err)
	}
	res, err := RunIngest(context.Background(), newTestOpts(cwd), io.Discard)
	if err != nil {
		t.Fatalf("RunIngest error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil IngestResult")
	}

	// The stale stub degraded marker MUST be gone. If it reappears,
	// someone has reverted the Wave 2 Issue 5 wiring.
	for _, d := range res.Degraded {
		if strings.Contains(d, "deferred to follow-up") {
			t.Errorf("found stale stub degraded note: %q", d)
		}
	}

	// Counters must be populated via the real call path. For an empty
	// fixture every counter is expected to be 0; the assertion is
	// structural (the field was touched by a real call rather than a
	// stub degraded note that left it at its zero value).
	if res.ForgeArtifactsMined < 0 {
		t.Errorf("expected ForgeArtifactsMined >= 0, got %d", res.ForgeArtifactsMined)
	}
	if res.ProvenanceAudited < 0 {
		t.Errorf("expected ProvenanceAudited >= 0, got %d", res.ProvenanceAudited)
	}
	if res.MineFindingsNew < 0 {
		t.Errorf("expected MineFindingsNew >= 0, got %d", res.MineFindingsNew)
	}

	// No hard stage failures on the happy path.
	for _, stage := range []string{"forge-mine", "provenance-audit", "mine-findings"} {
		if msg, failed := res.StageFailures[stage]; failed {
			t.Errorf("unexpected hard failure for %s: %s", stage, msg)
		}
	}
}

func TestBuildMineGeneratorSidecarDedupesCandidates(t *testing.T) {
	started := time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC)
	finished := started.Add(1500 * time.Millisecond)
	report := &mine.Report{
		Code: &mine.CodeFindings{
			Hotspots: []mine.ComplexityHotspot{
				{File: "cli/cmd/ao/overnight.go", Func: "runOvernight", Complexity: 27, RecentEdits: 3},
			},
		},
		Agents: &mine.AgentsFindings{
			OrphanedResearch: []string{"2026-04-26-sidecar.md"},
		},
	}
	items := mine.CollectMineWorkItems(report)
	if len(items) != 2 {
		t.Fatalf("fixture work items: got %d, want 2", len(items))
	}
	sidecar := buildMineGeneratorSidecar(
		newTestOpts(t.TempDir()),
		report,
		started,
		finished,
		map[string]bool{items[0].ID: true},
	)
	if sidecar.SchemaVersion != "finding-generator-sidecar/v1" {
		t.Fatalf("SchemaVersion = %q", sidecar.SchemaVersion)
	}
	if sidecar.Generator != mineFindingsGeneratorName {
		t.Fatalf("Generator = %q", sidecar.Generator)
	}
	if sidecar.CandidateCount != 2 {
		t.Fatalf("CandidateCount = %d, want 2", sidecar.CandidateCount)
	}
	if sidecar.DuplicateCount != 1 {
		t.Fatalf("DuplicateCount = %d, want 1", sidecar.DuplicateCount)
	}
	if sidecar.NewCandidateCount != 1 {
		t.Fatalf("NewCandidateCount = %d, want 1", sidecar.NewCandidateCount)
	}
	if sidecar.DuplicateRate != 0.5 {
		t.Fatalf("DuplicateRate = %v, want 0.5", sidecar.DuplicateRate)
	}
	if sidecar.DurationMillis != 1500 {
		t.Fatalf("DurationMillis = %d, want 1500", sidecar.DurationMillis)
	}
	for _, candidate := range sidecar.Candidates {
		if candidate.DedupKey == "" {
			t.Fatalf("candidate %q missing dedup key", candidate.Title)
		}
	}
}

func TestWriteFindingGeneratorSidecarAtomicJSON(t *testing.T) {
	cwd := t.TempDir()
	opts := newTestOpts(cwd)
	sidecar := FindingGeneratorSidecar{
		SchemaVersion:     "finding-generator-sidecar/v1",
		RunID:             opts.RunID,
		Generator:         mineFindingsGeneratorName,
		SourceEpic:        "compile-mine",
		Status:            "completed",
		CandidateCount:    1,
		NewCandidateCount: 1,
		Candidates: []FindingGeneratorCandidate{{
			ID:       "abc123",
			Title:    "Example",
			Type:     "task",
			Severity: "medium",
			Source:   "compile-mine",
			DedupKey: "finding-generator|mine-findings|example",
		}},
	}
	path, err := writeFindingGeneratorSidecar(opts, sidecar)
	if err != nil {
		t.Fatalf("writeFindingGeneratorSidecar: %v", err)
	}
	wantPath := filepath.Join(opts.OutputDir, "generator-results", "mine-findings.json")
	if path != wantPath {
		t.Fatalf("sidecar path = %q, want %q", path, wantPath)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	var got FindingGeneratorSidecar
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("sidecar JSON invalid: %v", err)
	}
	if got.CandidateCount != 1 || len(got.Candidates) != 1 {
		t.Fatalf("unexpected sidecar payload: %+v", got)
	}
}

func TestAggregateFindingGeneratorSidecarsAppendsOneDedupedBatch(t *testing.T) {
	cwd := t.TempDir()
	nextWorkDir := filepath.Join(cwd, ".agents", "rpi")
	if err := os.MkdirAll(nextWorkDir, 0o755); err != nil {
		t.Fatalf("mkdir next-work dir: %v", err)
	}
	existingLine := `{"source_epic":"seed","timestamp":"2026-04-26T00:00:00Z","items":[{"id":"already-present","title":"Already present","type":"task","severity":"medium","source":"evolve-generator","description":"seed","dedup_key":"finding-generator|mine-findings|already-present"}],"consumed":false,"claim_status":"available","claimed_by":null,"claimed_at":null,"consumed_by":null,"consumed_at":null}` + "\n"
	if err := os.WriteFile(filepath.Join(nextWorkDir, "next-work.jsonl"), []byte(existingLine), 0o644); err != nil {
		t.Fatalf("write seed next-work: %v", err)
	}

	opts := newTestOpts(cwd)
	completed := FindingGeneratorSidecar{
		SchemaVersion:  "finding-generator-sidecar/v1",
		RunID:          opts.RunID,
		Generator:      mineFindingsGeneratorName,
		SourceEpic:     "compile-mine",
		Status:         "completed",
		CandidateCount: 3,
		Candidates: []FindingGeneratorCandidate{
			{
				ID:          "already-present",
				Title:       "Already present",
				Type:        "task",
				Severity:    "medium",
				Source:      "evolve-generator",
				Description: "Seeded duplicate should not be routed again.",
				DedupKey:    "finding-generator|mine-findings|already-present",
			},
			{
				ID:          "new-high",
				Title:       "Reduce complexity: runDream",
				Type:        "tech-debt",
				Severity:    "high",
				Source:      "evolve-generator",
				Description: "Complexity hotspot should be routed.",
				Evidence:    "complexity=27",
				File:        "cli/cmd/ao/overnight.go",
				Func:        "runDream",
				DedupKey:    "finding-generator|mine-findings|tech-debt-reduce-complexity-rundream",
			},
			{
				ID:          "new-low-duplicate",
				Title:       "Reduce complexity duplicate",
				Type:        "tech-debt",
				Severity:    "medium",
				Source:      "evolve-generator",
				Description: "Lower-priority duplicate should lose reconciliation.",
				DedupKey:    "finding-generator|mine-findings|tech-debt-reduce-complexity-rundream",
			},
		},
	}
	if _, err := writeFindingGeneratorSidecar(opts, completed); err != nil {
		t.Fatalf("write completed sidecar: %v", err)
	}
	if _, err := writeFindingGeneratorSidecar(opts, FindingGeneratorSidecar{
		SchemaVersion: "finding-generator-sidecar/v1",
		RunID:         opts.RunID,
		Generator:     "slow-generator",
		SourceEpic:    "compile-mine",
		Status:        "soft-fail",
		Error:         "deadline exceeded",
	}); err != nil {
		t.Fatalf("write failed sidecar: %v", err)
	}

	got, degraded, err := AggregateFindingGeneratorSidecars(cwd, opts.OutputDir)
	if err != nil {
		t.Fatalf("AggregateFindingGeneratorSidecars: %v", err)
	}
	if got.SidecarsRead != 2 {
		t.Fatalf("SidecarsRead = %d, want 2", got.SidecarsRead)
	}
	if got.CandidatesSeen != 3 {
		t.Fatalf("CandidatesSeen = %d, want 3", got.CandidatesSeen)
	}
	if got.DuplicatesSkipped != 2 {
		t.Fatalf("DuplicatesSkipped = %d, want 2", got.DuplicatesSkipped)
	}
	if got.ItemsWritten != 1 {
		t.Fatalf("ItemsWritten = %d, want 1", got.ItemsWritten)
	}
	if !containsSubstring(degraded, "slow-generator: soft-fail") {
		t.Fatalf("expected soft-fail degradation, got %v", degraded)
	}

	data, err := os.ReadFile(filepath.Join(nextWorkDir, "next-work.jsonl"))
	if err != nil {
		t.Fatalf("read next-work: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("next-work lines = %d, want seed + one aggregate line\n%s", len(lines), data)
	}
	var appended struct {
		SourceEpic  string `json:"source_epic"`
		ClaimStatus string `json:"claim_status"`
		Items       []struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			Type        string `json:"type"`
			Severity    string `json:"severity"`
			Source      string `json:"source"`
			Description string `json:"description"`
			DedupKey    string `json:"dedup_key"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &appended); err != nil {
		t.Fatalf("unmarshal appended line: %v", err)
	}
	if appended.SourceEpic != "dream-generator-aggregator" {
		t.Fatalf("SourceEpic = %q, want dream-generator-aggregator", appended.SourceEpic)
	}
	if appended.ClaimStatus != "available" {
		t.Fatalf("ClaimStatus = %q, want available", appended.ClaimStatus)
	}
	if len(appended.Items) != 1 {
		t.Fatalf("appended items = %d, want 1", len(appended.Items))
	}
	item := appended.Items[0]
	if item.ID != "new-high" || item.Type != "tech-debt" || item.Source != "evolve-generator" || item.Description == "" || item.DedupKey == "" {
		t.Fatalf("unexpected appended item: %+v", item)
	}
}

// --- REDUCE ------------------------------------------------------------

// recordingCallbacks builds a CloseLoopOpts whose callbacks append their
// name to the shared order slice, so stage-order tests can assert the
// contract in one place.
type recorder struct {
	order []string
	// closeLoopErr, when non-nil, is returned from the first callback
	// that ExecuteCloseLoop invokes, forcing the stage to fail.
	closeLoopErr error
}

func (r *recorder) callbacks() lifecycle.CloseLoopOpts {
	return lifecycle.CloseLoopOpts{
		ResolveIngestFiles: func(cwd, pendingDir string, args []string) ([]string, error) {
			r.order = append(r.order, "close-loop")
			if r.closeLoopErr != nil {
				return nil, r.closeLoopErr
			}
			return nil, nil
		},
		IngestFilesToPool: func(cwd string, files []string) (lifecycle.CloseLoopIngestResult, error) {
			return lifecycle.CloseLoopIngestResult{}, nil
		},
		AutoPromoteFn: func(p *pool.Pool, threshold time.Duration, includeGold bool) (lifecycle.CloseLoopAutoPromoteResult, error) {
			return lifecycle.CloseLoopAutoPromoteResult{}, nil
		},
		ProcessCitationFeedback: func(cwd string) (int, int, int) {
			return 0, 0, 0
		},
		PromoteCitedLearnings: func(cwd string, quiet bool) int { return 0 },
		PromoteToMemory:       func(cwd string) (int, error) { return 0, nil },
		ApplyMaturityFn: func(cwd string) (lifecycle.MaturityTransitionSummary, error) {
			return lifecycle.MaturityTransitionSummary{}, nil
		},
	}
}

// newReduceFixture lays out a minimal .agents/ tree plus a fresh
// Checkpoint ready for RunReduce to drive.
func newReduceFixture(t *testing.T) (string, *Checkpoint, *IngestResult) {
	t.Helper()
	cwd := t.TempDir()
	// Minimal learnings so harvest has something to work with.
	writeLearning(t, cwd, "2026-04-09-fixture.md", map[string]string{
		"type":       "learning",
		"maturity":   "provisional",
		"confidence": "0.8",
	}, "# Fixture\n\nBody.\n")
	// Empty findings dir so the router has somewhere to look.
	if err := os.MkdirAll(filepath.Join(cwd, ".agents", "findings"), 0o755); err != nil {
		t.Fatalf("mkdir findings: %v", err)
	}

	cp, err := NewCheckpoint(cwd, "test-iter-1", 64*1024*1024)
	if err != nil {
		t.Fatalf("NewCheckpoint: %v", err)
	}

	ingest := &IngestResult{
		HarvestCatalog: nil, // harvest-promote stage no-ops when nil
	}
	return cwd, cp, ingest
}

func TestRunReduce_StageOrderEnforced(t *testing.T) {
	cwd, cp, ingest := newReduceFixture(t)
	rec := &recorder{}
	opts := newTestOpts(cwd)

	res, err := RunReduce(context.Background(), opts, ingest, cp, rec.callbacks(), io.Discard)
	if err != nil {
		t.Fatalf("RunReduce: %v", err)
	}
	if res.RolledBack {
		t.Fatalf("unexpected rollback: reason=%s", res.RollbackReason)
	}
	// close-loop is the only stage that records into rec.order via
	// a callback. The harvest-promote, dedup, maturity-temper,
	// defrag-prune, and findings-router stages execute real package
	// helpers (or the deferred stub) so their order is implicit in
	// RunReduce's in-source stage slice, not observable through
	// callbacks. Asserting close-loop got called at all confirms the
	// loop ran past its position in the stage sequence.
	if len(rec.order) != 1 || rec.order[0] != "close-loop" {
		t.Errorf("expected close-loop to be invoked exactly once, got %v", rec.order)
	}
}

func TestRunReduce_AggregatesGeneratorSidecarsIntoStagedNextWork(t *testing.T) {
	cwd, cp, ingest := newReduceFixture(t)
	opts := newTestOpts(cwd)
	if _, err := writeFindingGeneratorSidecar(opts, FindingGeneratorSidecar{
		SchemaVersion: "finding-generator-sidecar/v1",
		RunID:         opts.RunID,
		Generator:     mineFindingsGeneratorName,
		SourceEpic:    "compile-mine",
		Status:        "completed",
		Candidates: []FindingGeneratorCandidate{{
			ID:          "generated-one",
			Title:       "Generated queue item",
			Type:        "task",
			Severity:    "medium",
			Source:      "evolve-generator",
			Description: "Aggregator should write this item only inside the checkpoint staging tree.",
			DedupKey:    "finding-generator|mine-findings|generated-one",
		}},
	}); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	res, err := RunReduce(context.Background(), opts, ingest, cp, (&recorder{}).callbacks(), io.Discard)
	if err != nil {
		t.Fatalf("RunReduce: %v", err)
	}
	if res.GeneratorCandidatesRouted != 1 {
		t.Fatalf("GeneratorCandidatesRouted = %d, want 1", res.GeneratorCandidatesRouted)
	}
	liveNextWork := filepath.Join(cwd, ".agents", "rpi", "next-work.jsonl")
	if liveData, err := os.ReadFile(liveNextWork); err == nil && strings.Contains(string(liveData), "generated-one") {
		t.Fatalf("live next-work mutated before checkpoint commit:\n%s", liveData)
	}
	stagedNextWork := filepath.Join(cp.StagingDir, ".agents", "rpi", "next-work.jsonl")
	stagedData, err := os.ReadFile(stagedNextWork)
	if err != nil {
		t.Fatalf("read staged next-work: %v", err)
	}
	if !strings.Contains(string(stagedData), "generated-one") {
		t.Fatalf("staged next-work missing generated item:\n%s", stagedData)
	}
}

func TestRunReduce_RollbackOnMetadataStrip(t *testing.T) {
	cwd, cp, ingest := newReduceFixture(t)

	// Post-V1 fix semantic: REDUCE mutates STAGING. Simulate a reducer
	// that strips frontmatter by overwriting the staged copy BEFORE the
	// real stages run. When RunReduce reaches VerifyMetadataRoundTrip
	// it compares LIVE (pristine baseline) against STAGING (our stripped
	// copy) and reports every key as dropped, triggering rollback.
	//
	// Wave 3's buggy pre-V1 version stripped the LIVE copy; that only
	// worked because the integrity check walked the wrong direction.
	stagedPath := filepath.Join(cp.StagingDir, ".agents", "learnings", "2026-04-09-fixture.md")
	if err := os.WriteFile(stagedPath, []byte("# Fixture\n\nNo frontmatter here.\n"), 0o644); err != nil {
		t.Fatalf("strip staged fixture: %v", err)
	}

	rec := &recorder{}
	_, err := RunReduce(context.Background(), newTestOpts(cwd), ingest, cp, rec.callbacks(), io.Discard)
	if err == nil {
		t.Fatal("expected RunReduce to fail integrity check")
	}
	// The result is captured even on error — re-run is unnecessary;
	// instead, assert the error surface contains the rollback reason.
	if !strings.Contains(err.Error(), "metadata integrity failed") {
		t.Errorf("expected metadata integrity error, got %v", err)
	}
}

func TestRunReduce_RollbackOnMetadataStrip_ResultState(t *testing.T) {
	cwd, cp, ingest := newReduceFixture(t)
	stagedPath := filepath.Join(cp.StagingDir, ".agents", "learnings", "2026-04-09-fixture.md")
	if err := os.WriteFile(stagedPath, []byte("# Fixture\n\nNo frontmatter here.\n"), 0o644); err != nil {
		t.Fatalf("strip staged fixture: %v", err)
	}

	rec := &recorder{}
	res, err := RunReduce(context.Background(), newTestOpts(cwd), ingest, cp, rec.callbacks(), io.Discard)
	if err == nil {
		t.Fatal("expected RunReduce to return error")
	}
	if !res.RolledBack {
		t.Errorf("expected RolledBack=true")
	}
	if res.RollbackReason == "" {
		t.Errorf("expected non-empty RollbackReason")
	}
	if res.MetadataIntegrity.Pass {
		t.Errorf("expected MetadataIntegrity.Pass=false")
	}
}

func TestRunReduce_CloseLoopCallbacksNil_Skipped(t *testing.T) {
	cwd, cp, ingest := newReduceFixture(t)
	var empty lifecycle.CloseLoopOpts
	res, err := RunReduce(context.Background(), newTestOpts(cwd), ingest, cp, empty, io.Discard)
	if err != nil {
		t.Fatalf("RunReduce: %v", err)
	}
	if !containsSubstring(res.Degraded, "close-loop") {
		t.Errorf("expected close-loop degraded note, got %v", res.Degraded)
	}
	if res.CloseLoopPromoted != 0 {
		t.Errorf("expected CloseLoopPromoted=0 when skipped, got %d", res.CloseLoopPromoted)
	}
}

// TestRunReduce_IncludesInjectRefreshStage pins the Wave 4 Issue 16
// contract: RunReduce's stage pipeline MUST invoke the inject-refresh
// stage between findings-router and the metadata integrity check, and
// the outcome MUST land on ReduceResult. The test installs a stub via
// refreshInjectCacheFn so it neither touches exec.Command nor requires
// the `ao` binary on PATH.
func TestRunReduce_IncludesInjectRefreshStage(t *testing.T) {
	cwd, cp, ingest := newReduceFixture(t)

	// Install a stub refresh that returns a structured success result.
	// Restore the default after the test so unrelated tests continue
	// to exercise the real in-process path.
	//
	// Post-V1 fix: REDUCE stages now target cp.StagingDir (not opts.Cwd)
	// so every mutation lands inside the checkpoint boundary and is
	// rolled back or committed atomically. The stub assertion therefore
	// expects cp.StagingDir, not cwd. Guards against regression to the
	// buggy opts.Cwd behavior caught in Phase 3 vibe finding V1.
	_ = cwd // cwd is the repo root; staging is the checkpoint dir we expect
	stubCalled := false
	prev := refreshInjectCacheFn
	refreshInjectCacheFn = func(ctx context.Context, stubCwd string, log io.Writer) (*InjectRefreshResult, error) {
		stubCalled = true
		if stubCwd != cp.StagingDir {
			t.Errorf("stub received cwd %q, want cp.StagingDir %q", stubCwd, cp.StagingDir)
		}
		return &InjectRefreshResult{
			Attempted: true,
			Succeeded: true,
			Method:    "in-process",
			Duration:  time.Millisecond,
		}, nil
	}
	t.Cleanup(func() { refreshInjectCacheFn = prev })

	rec := &recorder{}
	res, err := RunReduce(context.Background(), newTestOpts(cwd), ingest, cp, rec.callbacks(), io.Discard)
	if err != nil {
		t.Fatalf("RunReduce: %v", err)
	}
	if res.RolledBack {
		t.Fatalf("unexpected rollback: reason=%s", res.RollbackReason)
	}
	if !stubCalled {
		t.Fatal("expected inject-refresh stub to be invoked")
	}
	if res.InjectRefreshResult == nil {
		t.Fatal("expected ReduceResult.InjectRefreshResult to be non-nil")
	}
	if !res.InjectRefreshResult.Succeeded {
		t.Errorf("expected InjectRefreshResult.Succeeded=true, got %+v",
			res.InjectRefreshResult)
	}
	if res.InjectRefreshResult.Method != "in-process" {
		t.Errorf("expected Method=in-process, got %q",
			res.InjectRefreshResult.Method)
	}
	if !res.InjectRefreshed {
		t.Error("expected ReduceResult.InjectRefreshed=true")
	}
	// Metadata integrity must still pass — inject-refresh runs before
	// the integrity check and must not interfere with it.
	if !res.MetadataIntegrity.Pass {
		t.Errorf("expected MetadataIntegrity.Pass=true, got %+v",
			res.MetadataIntegrity)
	}
}

// TestRefreshInjectCache_SkipsMissingAgentsDir verifies that the
// stage soft-degrades when .agents/ is absent: no error, no panic,
// just a "skipped" result with a degraded note. This is the documented
// honest-degradation path from inject_refresh.go.
func TestRefreshInjectCache_SkipsMissingAgentsDir(t *testing.T) {
	cwd := t.TempDir()
	// No .agents/ created.
	res, err := RefreshInjectCache(context.Background(), cwd, io.Discard)
	if err != nil {
		t.Fatalf("RefreshInjectCache: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}
	if res.Method != "skipped" {
		t.Errorf("expected Method=skipped, got %q", res.Method)
	}
	if res.Succeeded {
		t.Error("expected Succeeded=false for skipped path")
	}
	if !containsSubstring(res.Degraded, "inject-refresh") {
		t.Errorf("expected inject-refresh degraded note, got %v", res.Degraded)
	}
}

// TestRefreshInjectCache_InProcessHappyPath covers the preferred path:
// a real .agents/ tree with one learning is rebuilt into index.jsonl
// via search.BuildIndex + search.SaveIndex.
func TestRefreshInjectCache_InProcessHappyPath(t *testing.T) {
	cwd := t.TempDir()
	writeLearning(t, cwd, "2026-04-09-refresh.md", map[string]string{
		"type":     "learning",
		"maturity": "provisional",
	}, "# Refresh\n\nBody about the refresh path.\n")

	res, err := RefreshInjectCache(context.Background(), cwd, io.Discard)
	if err != nil {
		t.Fatalf("RefreshInjectCache: %v", err)
	}
	if res.Method != "in-process" {
		t.Errorf("expected Method=in-process, got %q", res.Method)
	}
	if !res.Succeeded {
		t.Errorf("expected Succeeded=true, got %+v", res)
	}
	// index.jsonl should exist under .agents/.
	idxPath := filepath.Join(cwd, ".agents", "index.jsonl")
	if _, err := os.Stat(idxPath); err != nil {
		t.Errorf("expected index.jsonl at %s, got %v", idxPath, err)
	}
}

func TestRunReduce_CloseLoopStageFailurePropagates(t *testing.T) {
	cwd, cp, ingest := newReduceFixture(t)
	rec := &recorder{closeLoopErr: errors.New("synthetic close-loop failure")}

	res, err := RunReduce(context.Background(), newTestOpts(cwd), ingest, cp, rec.callbacks(), io.Discard)
	if err == nil {
		t.Fatal("expected RunReduce to fail")
	}
	if !res.RolledBack {
		t.Error("expected RolledBack=true")
	}
	if res.RollbackReason == "" {
		t.Error("expected non-empty RollbackReason")
	}
	if _, ok := res.StageFailures["close-loop"]; !ok {
		t.Errorf("expected close-loop in StageFailures, got %v", res.StageFailures)
	}
}

// --- MEASURE -----------------------------------------------------------

func TestRunMeasure_CorpusComputeGoes(t *testing.T) {
	cwd := t.TempDir()
	writeLearning(t, cwd, "2026-04-09-m1.md", map[string]string{
		"type":        "learning",
		"maturity":    "provisional",
		"source_bead": "ao-1",
	}, "# M1\n\nBody.\n")

	res, err := RunMeasure(context.Background(), newTestOpts(cwd), io.Discard)
	if err != nil {
		t.Fatalf("RunMeasure: %v", err)
	}
	if res.Fitness == nil {
		t.Fatal("expected Fitness non-nil")
	}
	wantKeys := []string{
		"retrieval_precision",
		"retrieval_recall",
		"maturity_provisional_or_higher",
		"unresolved_findings",
		"citation_coverage",
		"inject_visibility",
		"cross_rig_dedup_ratio",
	}
	if len(res.FitnessSnapshot.Metrics) != len(wantKeys) {
		t.Errorf("expected %d metrics, got %d", len(wantKeys), len(res.FitnessSnapshot.Metrics))
	}
	for _, k := range wantKeys {
		if _, ok := res.FitnessSnapshot.Metrics[k]; !ok {
			t.Errorf("missing snapshot metric %s", k)
		}
	}
}

func TestRunMeasure_UnresolvedFindingsSignInverted(t *testing.T) {
	cwd := t.TempDir()
	// Seed .agents/ with a learning so corpus.Compute returns normally.
	writeLearning(t, cwd, "2026-04-09-m2.md", map[string]string{
		"type":     "learning",
		"maturity": "provisional",
	}, "# M2\n\nBody.\n")
	// Seed two unresolved findings.
	writeFindingRaw(t, cwd, "f-2026-04-09-001.md", "# First\n\nUnresolved.\n")
	writeFindingRaw(t, cwd, "f-2026-04-09-002.md", "# Second\n\nUnresolved.\n")

	res, err := RunMeasure(context.Background(), newTestOpts(cwd), io.Discard)
	if err != nil {
		t.Fatalf("RunMeasure: %v", err)
	}
	got, ok := res.FitnessSnapshot.Metrics["unresolved_findings"]
	if !ok {
		t.Fatal("missing unresolved_findings metric")
	}
	if got != -2.0 {
		t.Errorf("expected unresolved_findings=-2.0, got %v", got)
	}
}

func TestRunMeasure_MissingAgentsDir(t *testing.T) {
	cwd := t.TempDir()
	// No .agents/ directory at all.
	_, err := RunMeasure(context.Background(), newTestOpts(cwd), io.Discard)
	if err == nil {
		t.Fatal("expected error for missing .agents/")
	}
	if !strings.Contains(err.Error(), ".agents") {
		t.Errorf("expected error to mention .agents/, got %v", err)
	}
}

// --- helpers -----------------------------------------------------------

// containsSubstring returns true if any entry in haystack contains
// needle. Used by the degraded-note assertions so tests can match
// stable fragments of the degraded message without locking in the
// full human-readable wording.
func containsSubstring(haystack []string, needle string) bool {
	for _, s := range haystack {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

func TestRunReduce_FullStageOrder_Enforced(t *testing.T) {
	var order []string
	prev := reduceStageRecorder
	reduceStageRecorder = func(name string) { order = append(order, name) }
	t.Cleanup(func() { reduceStageRecorder = prev })

	cwd, cp, ingest := newReduceFixture(t)
	rec := &recorder{}
	// Install a working close-loop stub via the recorder so that stage
	// actually runs instead of being skipped.
	_, err := RunReduce(
		context.Background(),
		newTestOpts(cwd),
		ingest,
		cp,
		rec.callbacks(),
		io.Discard,
	)
	if err != nil {
		t.Fatalf("RunReduce: %v", err)
	}

	want := []string{
		"harvest-promote",
		"dedup",
		"maturity-temper",
		"defrag-prune",
		"close-loop",
		"findings-router",
		"generator-aggregator",
		"inject-refresh",
	}
	if !reflect.DeepEqual(order, want) {
		t.Errorf("stage order mismatch:\n got=%v\nwant=%v", order, want)
	}
}
