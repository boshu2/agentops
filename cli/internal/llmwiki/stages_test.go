package llmwiki

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func mkVaultWithRaw(t *testing.T, files map[string]string) string {
	t.Helper()
	vault := t.TempDir()
	rawDir := filepath.Join(vault, "raw")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatalf("mkdir raw: %v", err)
	}
	for name, body := range files {
		path := filepath.Join(rawDir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write raw file %s: %v", name, err)
		}
	}
	return vault
}

func fixedNow() time.Time {
	return time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
}

// ---------------------------------------------------------------------------
// IngestStage
// ---------------------------------------------------------------------------

func TestIngestStage_AtomicWrite(t *testing.T) {
	vault := mkVaultWithRaw(t, map[string]string{
		"foo.md": "raw foo body\n",
	})
	stage := &IngestStage{Now: fixedNow}

	result, err := stage.Run(context.Background(), vault, 1)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Stage != StageIngest {
		t.Errorf("result.Stage = %s, want %s", result.Stage, StageIngest)
	}
	if len(result.ArtifactsPath) != 1 {
		t.Fatalf("ArtifactsPath = %v, want 1 entry", result.ArtifactsPath)
	}

	dest := filepath.Join(vault, "wiki", "sources", "foo.md")
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("expected %s to exist: %v", dest, err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if !strings.Contains(string(data), "attempt: 1") {
		t.Errorf("written file missing 'attempt: 1' frontmatter: %s", data)
	}
	if !strings.HasPrefix(string(data), "---\n") {
		t.Errorf("written file missing frontmatter delimiter")
	}

	// Atomic-write contract: no .tmp-* leftovers in the destination dir.
	entries, err := os.ReadDir(filepath.Join(vault, "wiki", "sources"))
	if err != nil {
		t.Fatalf("read sources dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Errorf("found temp file leftover: %s", e.Name())
		}
	}
}

func TestIngestStage_AttemptTwoDoesNotDoubleWriteSource(t *testing.T) {
	vault := mkVaultWithRaw(t, map[string]string{
		"foo.md": "raw body\n",
	})
	stage := &IngestStage{Now: fixedNow}

	if _, err := stage.Run(context.Background(), vault, 1); err != nil {
		t.Fatalf("first run: %v", err)
	}
	dest := filepath.Join(vault, "wiki", "sources", "foo.md")
	first, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read after first run: %v", err)
	}

	// Second run with attempt=2: handler must detect existing valid
	// artifact and skip.
	result, err := stage.Run(context.Background(), vault, 2)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if !result.Skipped {
		t.Errorf("expected Skipped=true on re-run with valid artifact, got %+v", result)
	}
	second, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read after second run: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("file contents changed on no-op re-run\nbefore: %s\nafter: %s", first, second)
	}
}

func TestIngestStage_AttemptTwoOverwritesCorruptArtifact(t *testing.T) {
	vault := mkVaultWithRaw(t, map[string]string{
		"foo.md": "raw body\n",
	})
	sourcesDir := filepath.Join(vault, "wiki", "sources")
	if err := os.MkdirAll(sourcesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Pre-stage a corrupt artifact (no frontmatter).
	dest := filepath.Join(sourcesDir, "foo.md")
	if err := os.WriteFile(dest, []byte("garbage no frontmatter\n"), 0o644); err != nil {
		t.Fatalf("seed corrupt: %v", err)
	}

	stage := &IngestStage{Now: fixedNow}
	result, err := stage.Run(context.Background(), vault, 2)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Skipped {
		t.Errorf("expected overwrite, got Skipped=true")
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if !strings.Contains(string(data), "attempt: 2") {
		t.Errorf("expected attempt: 2 in overwritten file, got: %s", data)
	}
}

func TestIngestStage_CtxCancellationStopsWritesMidLoop(t *testing.T) {
	files := make(map[string]string, 5)
	for i := 0; i < 5; i++ {
		files[fmt.Sprintf("f%d.md", i)] = fmt.Sprintf("body %d\n", i)
	}
	vault := mkVaultWithRaw(t, files)

	// Cancel immediately before Run starts: zero artifacts should be
	// written, and Run should return ctx.Err() without panicking.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	stage := &IngestStage{Now: fixedNow}
	_, err := stage.Run(ctx, vault, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	sourcesDir := filepath.Join(vault, "wiki", "sources")
	entries, statErr := os.ReadDir(sourcesDir)
	if statErr != nil && !os.IsNotExist(statErr) {
		t.Fatalf("unexpected stat error: %v", statErr)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			t.Errorf("file written despite cancelled ctx: %s", e.Name())
		}
	}
}

func TestIngestStage_NoRawDirReturnsSkipped(t *testing.T) {
	vault := t.TempDir()
	stage := &IngestStage{Now: fixedNow}
	result, err := stage.Run(context.Background(), vault, 1)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Skipped || result.SkipReason != "no-raw-dir" {
		t.Errorf("expected Skipped+no-raw-dir, got %+v", result)
	}
}

// ---------------------------------------------------------------------------
// QueryStage
// ---------------------------------------------------------------------------

func setupQueryPending(t *testing.T, vault, query string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(vault, "wiki"), 0o755); err != nil {
		t.Fatalf("mkdir wiki: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(vault, "wiki", ".query-pending.json"),
		[]byte(query), 0o644,
	); err != nil {
		t.Fatalf("write pending: %v", err)
	}
}

func TestQueryStage_AtomicWrite(t *testing.T) {
	vault := t.TempDir()
	setupQueryPending(t, vault, "what is fitness gradient")
	stage := &QueryStage{Now: fixedNow}

	result, err := stage.Run(context.Background(), vault, 1)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(result.ArtifactsPath) != 1 {
		t.Fatalf("ArtifactsPath = %v, want 1", result.ArtifactsPath)
	}
	dest := result.ArtifactsPath[0]
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "attempt: 1") {
		t.Errorf("missing 'attempt: 1' in: %s", data)
	}
	if !strings.Contains(string(data), "query_key:") {
		t.Errorf("missing 'query_key:' in: %s", data)
	}
}

func TestQueryStage_AttemptTwoDoesNotDoubleWrite(t *testing.T) {
	vault := t.TempDir()
	setupQueryPending(t, vault, "hello world")
	stage := &QueryStage{Now: fixedNow}

	first, err := stage.Run(context.Background(), vault, 1)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if first.Skipped {
		t.Fatalf("first run unexpectedly skipped: %+v", first)
	}

	second, err := stage.Run(context.Background(), vault, 2)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if !second.Skipped {
		t.Errorf("expected Skipped=true on second call, got %+v", second)
	}
}

func TestQueryStage_NoPendingReturnsSkipped(t *testing.T) {
	vault := t.TempDir()
	stage := &QueryStage{Now: fixedNow}
	result, err := stage.Run(context.Background(), vault, 1)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Skipped || result.SkipReason != "no-pending-query" {
		t.Errorf("expected Skipped+no-pending-query, got %+v", result)
	}
}

// ---------------------------------------------------------------------------
// LintStage
// ---------------------------------------------------------------------------

func TestLintStage_OverwriteIsTheContract(t *testing.T) {
	vault := t.TempDir()
	// Seed a few sources so the lint findings have something to count.
	srcDir := filepath.Join(vault, "wiki", "sources")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir sources: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "a.md"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write a: %v", err)
	}

	stage := &LintStage{Now: fixedNow}

	first, err := stage.Run(context.Background(), vault, 1)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if len(first.ArtifactsPath) != 1 {
		t.Fatalf("first ArtifactsPath = %v, want 1", first.ArtifactsPath)
	}
	dest := first.ArtifactsPath[0]
	firstData, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read first: %v", err)
	}
	if !strings.Contains(string(firstData), "attempt: 1") {
		t.Errorf("missing 'attempt: 1' in first lint: %s", firstData)
	}

	// Add another source so the second lint diverges in content.
	if err := os.WriteFile(filepath.Join(srcDir, "b.md"), []byte("b"), 0o644); err != nil {
		t.Fatalf("write b: %v", err)
	}

	second, err := stage.Run(context.Background(), vault, 2)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if len(second.ArtifactsPath) != 1 {
		t.Fatalf("second ArtifactsPath = %v, want 1", second.ArtifactsPath)
	}
	if second.ArtifactsPath[0] != dest {
		t.Errorf("lint dest changed across runs: %s vs %s", second.ArtifactsPath[0], dest)
	}
	secondData, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read second: %v", err)
	}
	if !strings.Contains(string(secondData), "attempt: 2") {
		t.Errorf("missing 'attempt: 2' in second lint: %s", secondData)
	}
	if !strings.Contains(string(secondData), "2 files") {
		t.Errorf("expected updated count '2 files' in second lint: %s", secondData)
	}
}

func TestLintStage_CtxCancelledReturnsErrEarly(t *testing.T) {
	vault := t.TempDir()
	stage := &LintStage{Now: fixedNow}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := stage.Run(ctx, vault, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// PromoteStage
// ---------------------------------------------------------------------------

type fakePromoter struct {
	calls     int
	gotSource string
	gotDest   string
	gotDryRun bool
	retCount  int
	retErr    error
}

func (f *fakePromoter) Promote(sourceDir, destDir string, dryRun bool) (int, error) {
	f.calls++
	f.gotSource = sourceDir
	f.gotDest = destDir
	f.gotDryRun = dryRun
	return f.retCount, f.retErr
}

func TestPromoteStage_DelegatesToHarvest(t *testing.T) {
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "wiki"), 0o755); err != nil {
		t.Fatalf("mkdir wiki: %v", err)
	}
	pending := filepath.Join(vault, "wiki", ".promote-pending.json")
	body := `{"source_dir":"/tmp/src","dest_dir":"/tmp/dst"}`
	if err := os.WriteFile(pending, []byte(body), 0o644); err != nil {
		t.Fatalf("write pending: %v", err)
	}

	fp := &fakePromoter{retCount: 3}
	stage := &PromoteStage{Harvest: fp, DryRun: true}

	result, err := stage.Run(context.Background(), vault, 1)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if fp.calls != 1 {
		t.Errorf("expected 1 promoter call, got %d", fp.calls)
	}
	if fp.gotSource != "/tmp/src" || fp.gotDest != "/tmp/dst" {
		t.Errorf("source/dest not propagated: %s -> %s", fp.gotSource, fp.gotDest)
	}
	if !fp.gotDryRun {
		t.Errorf("expected DryRun=true to propagate")
	}
	if len(result.ArtifactsPath) != 1 {
		t.Errorf("expected 1 artifact line, got %v", result.ArtifactsPath)
	}
}

func TestPromoteStage_NoPendingReturnsSkipped(t *testing.T) {
	vault := t.TempDir()
	fp := &fakePromoter{}
	stage := &PromoteStage{Harvest: fp}

	result, err := stage.Run(context.Background(), vault, 1)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Skipped {
		t.Errorf("expected Skipped, got %+v", result)
	}
	if fp.calls != 0 {
		t.Errorf("promoter called despite no pending file: %d calls", fp.calls)
	}
}

func TestPromoteStage_NilHarvestErrors(t *testing.T) {
	vault := t.TempDir()
	stage := &PromoteStage{}
	_, err := stage.Run(context.Background(), vault, 1)
	if err == nil {
		t.Fatal("expected error when HarvestPromoter is nil")
	}
}
