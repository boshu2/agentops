package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// fakePlansBdSource is the L1 test double for PlansBdSource. Returns the
// pre-loaded `entries` payload, optionally erroring on demand. Tracks the
// (project_id, prefix, ctx) seen by the last QueryEpics call.
type fakePlansBdSource struct {
	mu          sync.Mutex
	entries     []PlansProjectionEntry
	err         error
	calls       int
	delayUntil  func(ctx context.Context) error
	lastProject string
	lastPrefix  string
}

func (f *fakePlansBdSource) QueryEpics(ctx context.Context, projectID, issuePrefix string) ([]PlansProjectionEntry, error) {
	f.mu.Lock()
	f.calls++
	f.lastProject = projectID
	f.lastPrefix = issuePrefix
	delay := f.delayUntil
	err := f.err
	entries := append([]PlansProjectionEntry(nil), f.entries...)
	f.mu.Unlock()
	if delay != nil {
		if delayErr := delay(ctx); delayErr != nil {
			return nil, delayErr
		}
	}
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// claimForPlansSpec returns a QueueLease with the plans.projection spec
// payload-encoded the same way Queue.SubmitJob does.
func claimForPlansSpec(t *testing.T, jobID string, spec PlansProjectionJobSpec) QueueLease {
	t.Helper()
	raw, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal spec: %v", err)
	}
	return QueueLease{Job: QueueJobState{
		JobID:   jobID,
		JobType: JobTypePlansProjection,
		Payload: payload,
	}}
}

// L0 — constructor-validation contract tests (F-PM-4).
// Mirrors cli/internal/daemon/rpi_executor_test.go:29-40.
func TestNewPlansProjectionExecutorRequiresStoreAndBdSource(t *testing.T) {
	if _, err := NewPlansProjectionExecutor(PlansProjectionExecutorOptions{}); err == nil {
		t.Fatalf("expected error when Store and BdSource are missing")
	}
	if _, err := NewPlansProjectionExecutor(PlansProjectionExecutorOptions{Store: NewStore(t.TempDir())}); err == nil {
		t.Fatalf("expected error when BdSource is missing")
	}
	if _, err := NewPlansProjectionExecutor(PlansProjectionExecutorOptions{BdSource: &fakePlansBdSource{}}); err == nil {
		t.Fatalf("expected error when Store is missing")
	}
}

func TestPlansProjectionExecutorJobTypesCoversPlansProjection(t *testing.T) {
	exec, err := NewPlansProjectionExecutor(PlansProjectionExecutorOptions{
		Store:    NewStore(t.TempDir()),
		BdSource: &fakePlansBdSource{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	types := exec.JobTypes()
	if len(types) != 1 || types[0] != JobTypePlansProjection {
		t.Fatalf("JobTypes = %v, want [plans.projection]", types)
	}
}

// L1 BDD — 6 scenarios per pilot §b. Subtest names use the
// Given/When/Then template from foundation §2.
func TestPlansProjectionExecutor(t *testing.T) {
	now := time.Date(2026, 5, 1, 14, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	t.Run("Given empty bd state, When subscription starts, Then empty projection emitted with zero manifest entries", func(t *testing.T) {
		assertPlansProjectionExecutorEmptyState(t, clock)
	})

	t.Run("Given populated bd state, When subscription starts, Then manifest snapshot rebuilt with N sorted entries", func(t *testing.T) {
		assertPlansProjectionExecutorPopulatedState(t, clock, now)
	})

	t.Run("Given bd query fails transiently, When executor runs, Then error surfaces and no snapshot is written", func(t *testing.T) {
		assertPlansProjectionExecutorTransientFailure(t, clock)
	})

	t.Run("Given context cancellation mid-run, When executor checks ctx, Then ctx.Err() is returned without writing snapshot", func(t *testing.T) {
		assertPlansProjectionExecutorCancellation(t, clock)
	})

	t.Run("Given identical bd state across runs, When executor runs twice, Then second run is replay-idempotent (same entries written)", func(t *testing.T) {
		assertPlansProjectionExecutorIdempotent(t, clock, now)
	})

	t.Run("Given a stale snapshot from a prior crash, When executor runs after restart, Then snapshot is overwritten atomically with current bd state", func(t *testing.T) {
		assertPlansProjectionExecutorRecoversStaleSnapshot(t, clock, now)
	})
}

func assertPlansProjectionExecutorEmptyState(t *testing.T, clock func() time.Time) {
	t.Helper()
	dir := t.TempDir()
	outDir := filepath.Join(dir, "plans")
	exec := newPlansProjectionExecutorForTest(t, dir, &fakePlansBdSource{entries: nil}, clock)
	result, runErr := runPlansProjectionJob(t, exec, "job-empty", outDir, context.Background())
	if runErr != nil {
		t.Fatalf("run: %v", runErr)
	}
	if got := result.Artifacts["manifest_count"]; got != "0" {
		t.Fatalf("manifest_count = %q, want 0", got)
	}
	manifestPath := result.Artifacts["manifest_jsonl"]
	if manifestPath == "" {
		t.Fatalf("manifest_jsonl artifact missing: %#v", result.Artifacts)
	}
	readBack := loadManifestEntries(t, manifestPath)
	if len(readBack) != 0 {
		t.Fatalf("manifest entries = %d, want 0", len(readBack))
	}
}

func assertPlansProjectionExecutorPopulatedState(t *testing.T, clock func() time.Time, now time.Time) {
	t.Helper()
	dir := t.TempDir()
	outDir := filepath.Join(dir, "plans")
	entries := []PlansProjectionEntry{
		{BeadsID: "soc-zzz", Title: "z-issue", Status: "open", IssueType: "epic", UpdatedAt: now},
		{BeadsID: "soc-aaa", Title: "a-issue", Status: "open", IssueType: "epic", UpdatedAt: now},
		{BeadsID: "soc-mmm", Title: "m-issue", Status: "closed", IssueType: "epic", UpdatedAt: now},
	}
	exec := newPlansProjectionExecutorForTest(t, dir, &fakePlansBdSource{entries: entries}, clock)
	result, runErr := runPlansProjectionJob(t, exec, "job-populated", outDir, context.Background())
	if runErr != nil {
		t.Fatalf("run: %v", runErr)
	}
	if got := result.Artifacts["manifest_count"]; got != "3" {
		t.Fatalf("manifest_count = %q, want 3", got)
	}
	readBack := loadManifestEntries(t, result.Artifacts["manifest_jsonl"])
	assertPlansProjectionEntriesSorted(t, readBack, []string{"soc-aaa", "soc-mmm", "soc-zzz"})
	snapshotPath := result.Artifacts["projection_snapshot"]
	if snapshotPath == "" {
		t.Fatalf("projection_snapshot artifact missing: %#v", result.Artifacts)
	}
	if _, err := os.Stat(snapshotPath); err != nil {
		t.Fatalf("projection snapshot stat: %v", err)
	}
	snapshot, latestPath, err := NewStore(dir).LoadLatestProjectionSnapshot()
	if err != nil {
		t.Fatalf("load latest projection snapshot: %v", err)
	}
	if latestPath != snapshotPath {
		t.Fatalf("latest projection snapshot = %q, want artifact path %q", latestPath, snapshotPath)
	}
	assertPlansProjectionEntriesSorted(t, snapshot.Plans.Entries, []string{"soc-aaa", "soc-mmm", "soc-zzz"})
}

func assertPlansProjectionExecutorTransientFailure(t *testing.T, clock func() time.Time) {
	t.Helper()
	dir := t.TempDir()
	outDir := filepath.Join(dir, "plans")
	exec := newPlansProjectionExecutorForTest(t, dir, &fakePlansBdSource{err: errors.New("dolt unreachable")}, clock)
	_, runErr := runPlansProjectionJob(t, exec, "job-transient", outDir, context.Background())
	if runErr == nil {
		t.Fatalf("expected error when bd source fails")
	}
	assertNoPlansManifest(t, outDir, "bd query failure")
}

func assertPlansProjectionExecutorCancellation(t *testing.T, clock func() time.Time) {
	t.Helper()
	dir := t.TempDir()
	outDir := filepath.Join(dir, "plans")
	ctx, cancel := context.WithCancel(context.Background())
	bdSource := &fakePlansBdSource{
		entries: []PlansProjectionEntry{{BeadsID: "soc-x", Title: "x", Status: "open", IssueType: "epic"}},
		delayUntil: func(c context.Context) error {
			cancel()
			select {
			case <-c.Done():
				return c.Err()
			case <-time.After(time.Second):
				return errors.New("delay timeout")
			}
		},
	}
	exec := newPlansProjectionExecutorForTest(t, dir, bdSource, clock)
	_, runErr := runPlansProjectionJob(t, exec, "job-cancel", outDir, ctx)
	if !errors.Is(runErr, context.Canceled) {
		t.Fatalf("run error = %v, want context.Canceled", runErr)
	}
	assertNoPlansManifest(t, outDir, "cancellation")
}

func assertPlansProjectionExecutorIdempotent(t *testing.T, clock func() time.Time, now time.Time) {
	t.Helper()
	dir := t.TempDir()
	outDir := filepath.Join(dir, "plans")
	bdSource := &fakePlansBdSource{entries: []PlansProjectionEntry{
		{BeadsID: "soc-1", Title: "one", Status: "open", IssueType: "epic", UpdatedAt: now},
		{BeadsID: "soc-2", Title: "two", Status: "closed", IssueType: "epic", UpdatedAt: now},
	}}
	exec := newPlansProjectionExecutorForTest(t, dir, bdSource, clock)
	first, runErr := runPlansProjectionJob(t, exec, "job-idem", outDir, context.Background())
	if runErr != nil {
		t.Fatalf("first run: %v", runErr)
	}
	second, runErr := runPlansProjectionJob(t, exec, "job-idem", outDir, context.Background())
	if runErr != nil {
		t.Fatalf("second run: %v", runErr)
	}
	if first.Artifacts["manifest_jsonl"] != second.Artifacts["manifest_jsonl"] {
		t.Fatalf("manifest path drifted across runs: %q -> %q",
			first.Artifacts["manifest_jsonl"], second.Artifacts["manifest_jsonl"])
	}
	readBack := loadManifestEntries(t, second.Artifacts["manifest_jsonl"])
	if len(readBack) != 2 {
		t.Fatalf("entries after second run = %d, want 2 (idempotent)", len(readBack))
	}
	if bdSource.calls != 2 {
		t.Fatalf("bd query calls = %d, want 2 (each run queries once)", bdSource.calls)
	}
}

func assertPlansProjectionExecutorRecoversStaleSnapshot(t *testing.T, clock func() time.Time, now time.Time) {
	t.Helper()
	dir := t.TempDir()
	outDir := filepath.Join(dir, "plans")
	stale := seedStalePlansManifest(t, outDir)
	bdSource := &fakePlansBdSource{entries: []PlansProjectionEntry{
		{BeadsID: "soc-fresh", Title: "fresh", Status: "open", IssueType: "epic", UpdatedAt: now},
	}}
	exec := newPlansProjectionExecutorForTest(t, dir, bdSource, clock)
	_, runErr := runPlansProjectionJob(t, exec, "job-recover", outDir, context.Background())
	if runErr != nil {
		t.Fatalf("run: %v", runErr)
	}
	readBack := loadManifestEntries(t, stale)
	if len(readBack) != 1 || readBack[0].BeadsID != "soc-fresh" {
		t.Fatalf("post-recover entries = %#v, want exactly soc-fresh", readBack)
	}
}

func newPlansProjectionExecutorForTest(
	t *testing.T,
	dir string,
	bdSource PlansBdSource,
	clock func() time.Time,
) *PlansProjectionExecutor {
	t.Helper()
	exec, err := NewPlansProjectionExecutor(PlansProjectionExecutorOptions{
		Store:    NewStore(dir),
		BdSource: bdSource,
		Now:      clock,
	})
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}
	return exec
}

func runPlansProjectionJob(
	t *testing.T,
	exec *PlansProjectionExecutor,
	jobID, outDir string,
	ctx context.Context,
) (JobExecutionResult, error) {
	t.Helper()
	spec := NewPlansProjectionJobSpec("proj-1", "soc", outDir)
	claim := claimForPlansSpec(t, jobID, spec)
	return exec.RunJob(ctx, claim)
}

func assertPlansProjectionEntriesSorted(t *testing.T, entries []PlansProjectionEntry, want []string) {
	t.Helper()
	if len(entries) != len(want) {
		t.Fatalf("manifest entries = %d, want %d", len(entries), len(want))
	}
	for i, expected := range want {
		if entries[i].BeadsID != expected {
			t.Fatalf("entries[%d].BeadsID = %q, want %q", i, entries[i].BeadsID, expected)
		}
		if entries[i].Checksum == "" {
			t.Fatalf("entries[%d].Checksum is empty (executor should fill)", i)
		}
	}
}

func assertNoPlansManifest(t *testing.T, outDir, reason string) {
	t.Helper()
	if _, statErr := os.Stat(filepath.Join(outDir, "manifest.jsonl")); statErr == nil {
		t.Fatalf("manifest.jsonl should not exist on %s", reason)
	}
}

func seedStalePlansManifest(t *testing.T, outDir string) string {
	t.Helper()
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("seed mkdir: %v", err)
	}
	stale := filepath.Join(outDir, "manifest.jsonl")
	if err := os.WriteFile(stale, []byte(`{"beads_id":"soc-stale","title":"stale"}`+"\n"), 0o644); err != nil {
		t.Fatalf("seed stale: %v", err)
	}
	return stale
}

// TestDaemonPlansProjection_RebuildAndValidate exercises the projection
// helper triple in isolation.
func TestDaemonPlansProjection_RebuildAndValidate(t *testing.T) {
	t.Run("RebuildDaemonPlansProjection on empty events returns empty projection with current schema", func(t *testing.T) {
		projection, err := RebuildDaemonPlansProjection(nil)
		if err != nil {
			t.Fatalf("rebuild: %v", err)
		}
		if projection.SchemaVersion != DaemonPlansProjectionSchemaVersion {
			t.Fatalf("schema_version = %d, want %d", projection.SchemaVersion, DaemonPlansProjectionSchemaVersion)
		}
		if len(projection.Entries) != 0 {
			t.Fatalf("entries = %d, want 0", len(projection.Entries))
		}
	})
	t.Run("ValidateDaemonPlansProjection rejects duplicate beads_id", func(t *testing.T) {
		projection := DaemonPlansProjection{
			SchemaVersion: DaemonPlansProjectionSchemaVersion,
			Entries: []PlansProjectionEntry{
				{BeadsID: "soc-1"}, {BeadsID: "soc-1"},
			},
		}
		if err := ValidateDaemonPlansProjection(projection); err == nil {
			t.Fatalf("expected duplicate beads_id error")
		}
	})
	t.Run("ValidateDaemonPlansProjection rejects empty beads_id", func(t *testing.T) {
		projection := DaemonPlansProjection{
			SchemaVersion: DaemonPlansProjectionSchemaVersion,
			Entries:       []PlansProjectionEntry{{BeadsID: ""}},
		}
		if err := ValidateDaemonPlansProjection(projection); err == nil {
			t.Fatalf("expected missing beads_id error")
		}
	})
}

func loadManifestEntries(t *testing.T, path string) []PlansProjectionEntry {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest %q: %v", path, err)
	}
	var entries []PlansProjectionEntry
	dec := json.NewDecoder(bytes.NewReader(data))
	for dec.More() {
		var entry PlansProjectionEntry
		if err := dec.Decode(&entry); err != nil {
			t.Fatalf("decode manifest line: %v", err)
		}
		entries = append(entries, entry)
	}
	return entries
}
