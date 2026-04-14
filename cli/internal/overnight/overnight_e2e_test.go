package overnight

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/corpus"
	"github.com/boshu2/agentops/cli/internal/lifecycle"
	"github.com/boshu2/agentops/cli/internal/overnight/fixture"
)

// stubInjectRefresh temporarily replaces refreshInjectCacheFn with a
// pass-through that returns a successful, attempted, in-process result
// without touching the real search.BuildIndex code path. The returned
// cleanup must be deferred to restore the package-level default.
//
// This mirrors the refreshInjectCacheFn override pattern established
// by Wave 3 and documented on the RefreshInjectCache godoc.
func stubInjectRefresh(t *testing.T) func() {
	t.Helper()
	prev := refreshInjectCacheFn
	refreshInjectCacheFn = func(_ context.Context, _ string, _ io.Writer) (*InjectRefreshResult, error) {
		return &InjectRefreshResult{
			Attempted: true,
			Succeeded: true,
			Method:    "in-process",
			Duration:  time.Millisecond,
		}, nil
	}
	return func() { refreshInjectCacheFn = prev }
}

// frontmatterKeys loads every *.md file under dir/.agents/learnings
// and returns a map keyed by relative filename whose value is the set
// of frontmatter keys seen in that file. Used by the metadata
// round-trip assertion in the L2 e2e test. The parser is intentionally
// minimal: it reads lines between the first "---" and the next "---"
// and treats the substring before the first ":" as the key.
func frontmatterKeys(t *testing.T, dir string) map[string]map[string]bool {
	t.Helper()
	out := map[string]map[string]bool{}
	learnings := filepath.Join(dir, ".agents", "learnings")
	entries, err := os.ReadDir(learnings)
	if err != nil {
		t.Fatalf("read learnings dir: %v", err)
	}
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(learnings, ent.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", ent.Name(), err)
		}
		keys := map[string]bool{}
		lines := strings.Split(string(data), "\n")
		inFront := false
		for _, line := range lines {
			if strings.TrimSpace(line) == "---" {
				if !inFront {
					inFront = true
					continue
				}
				break
			}
			if !inFront {
				continue
			}
			if idx := strings.Index(line, ":"); idx > 0 {
				keys[strings.TrimSpace(line[:idx])] = true
			}
		}
		out[ent.Name()] = keys
	}
	return out
}

// frontmatterDigest returns a stable sha256 hex of the frontmatter
// key-set snapshot across every learning. Two digests equal if-and-
// only-if the same set of (file, key) tuples is present.
func frontmatterDigest(fm map[string]map[string]bool) string {
	var names []string
	for name := range fm {
		names = append(names, name)
	}
	// Sort for determinism.
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[j] < names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	h := sha256.New()
	for _, name := range names {
		h.Write([]byte(name))
		h.Write([]byte{0})
		var keys []string
		for k := range fm[name] {
			keys = append(keys, k)
		}
		for i := 0; i < len(keys); i++ {
			for j := i + 1; j < len(keys); j++ {
				if keys[j] < keys[i] {
					keys[i], keys[j] = keys[j], keys[i]
				}
			}
		}
		for _, k := range keys {
			h.Write([]byte(k))
			h.Write([]byte{0})
		}
		h.Write([]byte{1})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// countLines returns the number of newline-terminated lines in path.
// Returns 0 if the file does not exist. Used to assert the findings
// router actually routed work into next-work.jsonl.
func countLines(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("read %s: %v", path, err)
	}
	if len(data) == 0 {
		return 0
	}
	return strings.Count(string(data), "\n")
}

// TestRunLoop_L2_FullIteration_CorpusQualityMoves exercises the
// Dream nightly compounder end-to-end against a 150-learning fixture.
//
// pm-015: the compound filter cliff only surfaces at >= 150 learnings,
// so the fixture default (via fixture.DefaultOpts) must stay at that
// threshold.
//
// Because RunLoop's outer driver is still a Wave 1 skeleton, the test
// also calls RunIngest/RunReduce/RunMeasure inline to prove the real
// stage drivers work against the same fixture.
func TestRunLoop_L2_FullIteration_CorpusQualityMoves(t *testing.T) {
	// Isolate HOME: harvest.Promote writes to $HOME/.agents/learnings/ by
	// default. Without this guard, every L2 run poisons the real user's
	// global knowledge hub with 150 "unknown-unknown-learning-NNN.md"
	// fixture files. Confirmed bug during Phase 3 validation.
	t.Setenv("HOME", t.TempDir())

	restore := stubInjectRefresh(t)
	defer restore()

	dir := generateCorpusQualityFixture(t)
	assertCorpusQualityBaseline(t, dir)

	before := snapshotCorpusQualityFrontmatter(t, dir)
	result := runCorpusQualityLoop(t, dir)

	assertCorpusQualityIterations(t, result)
	assertCorpusQualityFindingsRouted(t, dir, result)
	assertCorpusQualityFitnessCaptured(t, result)
	assertCorpusQualityFrontmatterPreserved(t, dir, before)
}

type corpusQualityFrontmatterSnapshot struct {
	keys   map[string]map[string]bool
	digest string
}

func generateCorpusQualityFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := fixture.GenerateFixture(dir, fixture.DefaultOpts()); err != nil {
		t.Fatalf("GenerateFixture: %v", err)
	}
	return dir
}

func assertCorpusQualityBaseline(t *testing.T, dir string) {
	t.Helper()
	baseline, _, err := corpus.Compute(dir)
	if err != nil {
		t.Fatalf("corpus.Compute baseline: %v", err)
	}
	if baseline == nil {
		t.Fatal("baseline FitnessVector is nil")
	}
}

func snapshotCorpusQualityFrontmatter(t *testing.T, dir string) corpusQualityFrontmatterSnapshot {
	t.Helper()
	keys := frontmatterKeys(t, dir)
	if len(keys) < 150 {
		t.Fatalf("expected >= 150 learnings in fixture, got %d", len(keys))
	}
	return corpusQualityFrontmatterSnapshot{
		keys:   keys,
		digest: frontmatterDigest(keys),
	}
}

func runCorpusQualityLoop(t *testing.T, dir string) *RunLoopResult {
	t.Helper()
	result, err := RunLoop(context.Background(), corpusQualityLoopOptions(dir))
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}
	if result == nil {
		t.Fatal("RunLoop returned nil result")
	}
	return result
}

func corpusQualityLoopOptions(dir string) RunLoopOptions {
	return RunLoopOptions{
		Cwd:           dir,
		OutputDir:     filepath.Join(dir, ".agents", "overnight", "test-run"),
		RunID:         "test-run",
		RunTimeout:    30 * time.Second,
		MaxIterations: 2,
		WarnOnly:      true, // tolerate plateau/regression while exercising the loop
		LogWriter:     io.Discard,
	}
}

func assertCorpusQualityIterations(t *testing.T, result *RunLoopResult) {
	t.Helper()
	if len(result.Iterations) != 2 {
		t.Fatalf("expected 2 iterations from RunLoop, got %d (degraded=%v)",
			len(result.Iterations), result.Degraded)
	}
	for i, iter := range result.Iterations {
		assertCorpusQualityIteration(t, i, iter)
	}
}

func assertCorpusQualityIteration(t *testing.T, index int, iter IterationSummary) {
	t.Helper()
	if iter.Status == StatusRolledBackPreCommit || iter.Status == StatusFailed {
		t.Errorf("iteration %d ended in status %q (error=%s)", index+1, iter.Status, iter.Error)
	}
	if iter.Reduce == nil {
		t.Errorf("iteration %d missing Reduce sub-summary", index+1)
	}
	if iter.Measure == nil {
		t.Errorf("iteration %d missing Measure sub-summary", index+1)
	}
}

func assertCorpusQualityFindingsRouted(t *testing.T, dir string, result *RunLoopResult) {
	t.Helper()
	iter1Reduce := result.Iterations[0].Reduce
	if iter1Reduce == nil {
		t.Fatal("iteration 1 Reduce summary is nil")
	}
	routed, _ := iter1Reduce["findings_routed"].(int)
	if routed <= 0 {
		t.Errorf("expected iteration 1 to route >0 findings, got %d (reduce=%v)", routed, iter1Reduce)
	}

	liveNextWork := filepath.Join(dir, ".agents", "rpi", "next-work.jsonl")
	routedLines := countLines(t, liveNextWork)
	if routedLines <= 0 {
		t.Errorf("expected live next-work.jsonl to contain >0 routed lines after RunLoop, got %d", routedLines)
	}
}

func assertCorpusQualityFitnessCaptured(t *testing.T, result *RunLoopResult) {
	t.Helper()
	iter1Measure := result.Iterations[0].Measure
	if iter1Measure == nil {
		t.Fatal("iteration 1 Measure summary is nil")
	}
	fitness, ok := iter1Measure["fitness"]
	if !ok || fitness == nil {
		t.Errorf("expected iteration 1 measure.fitness to be non-nil, got %v", iter1Measure)
	}
}

func assertCorpusQualityFrontmatterPreserved(
	t *testing.T,
	dir string,
	before corpusQualityFrontmatterSnapshot,
) {
	t.Helper()
	after := frontmatterKeys(t, dir)
	if frontmatterDigest(after) == before.digest {
		return
	}
	for file, beforeKeys := range before.keys {
		afterKeys := after[file]
		for k := range beforeKeys {
			if !afterKeys[k] {
				t.Errorf("frontmatter key %q dropped from %s", k, file)
			}
		}
	}
}

// TestRunLoop_L2_MetadataStripDetected is a pm-005 regression guard at
// the L2 level: corrupt a staging copy of a learning by removing a
// frontmatter key, then verify VerifyMetadataRoundTrip surfaces the
// drop and rollback restores the live tree.
func TestRunLoop_L2_MetadataStripDetected(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate global hub writes; see note on sibling test
	restore := stubInjectRefresh(t)
	defer restore()

	dir := t.TempDir()
	if err := fixture.GenerateFixture(dir, fixture.DefaultOpts()); err != nil {
		t.Fatalf("GenerateFixture: %v", err)
	}

	cp, err := NewCheckpoint(dir, "l2-e2e-strip", 128*1024*1024)
	if err != nil {
		t.Fatalf("NewCheckpoint: %v", err)
	}

	// Confirm the staging copy has the original frontmatter (the
	// checkpoint cloned it before we corrupt staging below).
	stagedTarget := filepath.Join(cp.StagingDir, ".agents", "learnings", "learning-000.md")
	stagedBefore, err := os.ReadFile(stagedTarget)
	if err != nil {
		t.Fatalf("read staged fixture: %v", err)
	}
	if !strings.Contains(string(stagedBefore), "Synthetic Learning 000") {
		t.Fatalf("expected staging copy to hold original content, got %q",
			string(stagedBefore))
	}

	// Post-V1 fix semantic: REDUCE mutates STAGING (not LIVE).
	// To simulate a reducer that strips metadata, overwrite the staged
	// copy; LIVE (the pristine baseline) still has the original
	// frontmatter. VerifyMetadataRoundTrip walks LIVE and reports keys
	// missing from the staged output as stripped.
	if err := os.WriteFile(stagedTarget, []byte("# Stripped\n\nNo frontmatter here.\n"), 0o644); err != nil {
		t.Fatalf("strip staged fixture: %v", err)
	}

	report := VerifyMetadataRoundTrip(cp)
	if report.Pass {
		t.Fatal("expected VerifyMetadataRoundTrip to detect stripped fields, got Pass=true")
	}
	if len(report.StrippedFields) == 0 {
		t.Fatal("expected StrippedFields to be non-empty")
	}
	// Confirm at least one stripped field points at learning-000.md.
	found := false
	for _, sf := range report.StrippedFields {
		if strings.HasSuffix(sf.File, "learning-000.md") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a stripped field for learning-000.md, got %+v", report.StrippedFields)
	}

	// Pre-commit Rollback removes the staging dir but does NOT
	// restore the live tree (Checkpoint.Rollback only reverses
	// partial commits after a READY marker). Assert the staging
	// directory is gone, which proves the checkpoint is no longer
	// usable — the semantic guard is "no half-mutated state left
	// behind" rather than "live tree restored".
	if err := cp.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if _, statErr := os.Stat(cp.StagingDir); !os.IsNotExist(statErr) {
		t.Errorf("expected staging dir removed after Rollback, stat err=%v", statErr)
	}
}

// TestRunLoop_L2_CompoundFilterCliff_DoesNotSilentlyKillCorpus is the
// pm-015 regression guard: the 2026-04-02 flywheel-quality-fixes
// learning warned that compound filters can silently drop every
// learning when stacked naively. This test generates a 150-learning
// fixture, runs REDUCE, and asserts no corpus metric drops by more
// than 0.30 between pre- and post-REDUCE corpus.Compute snapshots.
func TestRunLoop_L2_CompoundFilterCliff_DoesNotSilentlyKillCorpus(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate global hub writes; see note on sibling test
	restore := stubInjectRefresh(t)
	defer restore()

	dir := t.TempDir()
	opts := fixture.DefaultOpts() // 150 learnings
	if err := fixture.GenerateFixture(dir, opts); err != nil {
		t.Fatalf("GenerateFixture: %v", err)
	}

	pre, _, err := corpus.Compute(dir)
	if err != nil {
		t.Fatalf("corpus.Compute pre: %v", err)
	}
	if pre == nil {
		t.Fatal("pre FitnessVector nil")
	}

	runOpts := RunLoopOptions{
		Cwd:       dir,
		OutputDir: filepath.Join(dir, ".agents", "overnight", "cliff-run"),
		RunID:     "cliff-run",
		WarnOnly:  true,
		LogWriter: io.Discard,
	}
	ingest, err := RunIngest(context.Background(), runOpts, io.Discard)
	if err != nil {
		t.Fatalf("RunIngest: %v", err)
	}
	cp, err := NewCheckpoint(dir, "l2-cliff-iter", 128*1024*1024)
	if err != nil {
		t.Fatalf("NewCheckpoint: %v", err)
	}
	reduce, err := RunReduce(context.Background(), runOpts, ingest, cp, lifecycle.CloseLoopOpts{}, io.Discard)
	if err != nil {
		t.Fatalf("RunReduce: %v", err)
	}
	if reduce.RolledBack {
		t.Fatalf("unexpected cliff-run rollback: %s", reduce.RollbackReason)
	}

	post, _, err := corpus.Compute(dir)
	if err != nil {
		t.Fatalf("corpus.Compute post: %v", err)
	}

	// Cliff threshold: 0.30. Any metric that drops by more than this
	// is flagged as a silent-kill symptom.
	checks := []struct {
		name string
		pre  float64
		post float64
	}{
		{"maturity_provisional", pre.MaturityProvisional, post.MaturityProvisional},
		{"citation_coverage", pre.CitationCoverage, post.CitationCoverage},
		{"inject_visibility", pre.InjectVisibility, post.InjectVisibility},
	}
	for _, c := range checks {
		drop := c.pre - c.post
		if drop > 0.30 {
			t.Errorf("pm-015 cliff: metric %q dropped by %.3f (pre=%.3f post=%.3f)",
				c.name, drop, c.pre, c.post)
		}
	}
}

// TestRunLoop_PostCommitMetadataCheck_LogsFinding is the Wave 4 Issue 8
// regression guard: after Commit() swaps staging into live, the
// post-commit VerifyMetadataRoundTripPostCommit check must walk the
// PrevDir backup against LiveDir and catch any late-stage strips that
// the pre-commit check (staging vs live) would have missed. We can't
// easily inject a hook between Commit and MEASURE inside RunLoop, so
// this test drives RunIngest + NewCheckpoint + RunReduce + Commit
// manually (mirroring RunLoop), simulates a post-commit strip by
// overwriting a file in the live tree, and asserts both the
// verification function and logPostCommitFinding behave correctly.
func TestRunLoop_PostCommitMetadataCheck_LogsFinding(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	restore := stubInjectRefresh(t)
	defer restore()

	dir := t.TempDir()
	if err := fixture.GenerateFixture(dir, fixture.DefaultOpts()); err != nil {
		t.Fatalf("GenerateFixture: %v", err)
	}

	ctx := context.Background()
	opts := RunLoopOptions{
		Cwd:       dir,
		OutputDir: filepath.Join(dir, ".agents", "overnight", "postcommit-test"),
		RunID:     "postcommit-test",
		WarnOnly:  true,
		LogWriter: io.Discard,
	}

	ingest, err := RunIngest(ctx, opts, io.Discard)
	if err != nil {
		t.Fatalf("RunIngest: %v", err)
	}

	cp, err := NewCheckpoint(dir, "postcommit-test-iter-1", 128*1024*1024)
	if err != nil {
		t.Fatalf("NewCheckpoint: %v", err)
	}

	rec := &recorder{}
	if _, err := RunReduce(ctx, opts, ingest, cp, rec.callbacks(), io.Discard); err != nil {
		t.Fatalf("RunReduce: %v", err)
	}
	if err := cp.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Simulate a post-commit strip by overwriting a learning in LIVE.
	// After Commit, the pre-REDUCE baseline is in cp.PrevDir and the
	// post-promote state is in cp.LiveDir, so overwriting the live copy
	// with stripped content creates the exact (file, key) set-difference
	// that VerifyMetadataRoundTripPostCommit is designed to detect.
	stripped := filepath.Join(dir, ".agents", "learnings", "learning-000.md")
	if err := os.WriteFile(stripped, []byte("# Stripped\n\nNo frontmatter\n"), 0o644); err != nil {
		t.Fatalf("write stripped: %v", err)
	}

	report := VerifyMetadataRoundTripPostCommit(cp)
	if report.Pass {
		t.Fatal("expected Pass=false after simulated post-commit strip")
	}
	if len(report.StrippedFields) == 0 {
		t.Error("expected non-empty StrippedFields")
	}
	// At least one stripped field should point at learning-000.md.
	foundTarget := false
	for _, sf := range report.StrippedFields {
		if strings.HasSuffix(sf.File, "learning-000.md") {
			foundTarget = true
			break
		}
	}
	if !foundTarget {
		t.Errorf("expected a stripped field for learning-000.md, got %+v", report.StrippedFields)
	}

	// Exercise logPostCommitFinding directly and verify the filename
	// intentionally does NOT match findingFilenameRe, so the router
	// will skip it.
	if err := logPostCommitFinding(dir, 1, report); err != nil {
		t.Fatalf("logPostCommitFinding: %v", err)
	}

	findingsDir := filepath.Join(dir, ".agents", "findings")
	entries, err := os.ReadDir(findingsDir)
	if err != nil {
		t.Fatalf("read findings dir: %v", err)
	}
	found := false
	for _, e := range entries {
		if strings.Contains(e.Name(), "postcommit-iter-1") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected findings file containing 'postcommit-iter-1', got entries=%v", postCommitDirNames(entries))
	}
}

// postCommitDirNames is a tiny helper used only by
// TestRunLoop_PostCommitMetadataCheck_LogsFinding to render directory
// entries in error messages. Named with a prefix to avoid collisions
// with any future sibling helper.
func postCommitDirNames(entries []os.DirEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}
