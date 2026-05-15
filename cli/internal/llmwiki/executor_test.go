package llmwiki

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/daemon"
)

// fakeHandler is a test stub for StageHandler. It increments a counter on each
// Run call and returns a fixed StageResult; this lets tests assert how many
// times a stage handler was invoked across multiple Execute calls.
type fakeHandler struct {
	calls       int32
	lastAttempt int32
	stage       Stage
}

func (f *fakeHandler) Run(_ context.Context, _ string, attempt int) (StageResult, error) {
	atomic.AddInt32(&f.calls, 1)
	atomic.StoreInt32(&f.lastAttempt, int32(attempt))
	return StageResult{Stage: f.stage, Attempt: attempt}, nil
}

func mkClaim(jobType daemon.JobType, payload map[string]any, attempt int) daemon.QueueLease {
	return daemon.QueueLease{
		Job: daemon.QueueJobState{
			JobID:   "job-test",
			JobType: jobType,
			Attempt: attempt,
			Payload: payload,
		},
	}
}

func TestLLMWikiLoopExecutor_DeclaresLLMWikiLoopType(t *testing.T) {
	exec := &LLMWikiLoopExecutor{}
	got := exec.JobTypes()
	if len(got) != 1 || got[0] != daemon.JobTypeLLMWikiLoop {
		t.Fatalf("JobTypes() = %v, want [%s]", got, daemon.JobTypeLLMWikiLoop)
	}
}

func TestSelectStage_PrefersIngestWhenRawHasNew(t *testing.T) {
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "raw"), 0o755); err != nil {
		t.Fatalf("mkdir raw: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "wiki"), 0o755); err != nil {
		t.Fatalf("mkdir wiki: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "raw", "foo.md"), []byte("raw"), 0o644); err != nil {
		t.Fatalf("write raw/foo.md: %v", err)
	}
	got := SelectStage(vault, 24, time.Now())
	if got != StageIngest {
		t.Fatalf("SelectStage = %q, want %q", got, StageIngest)
	}
}

func TestSelectStage_PrefersLintWhenStale(t *testing.T) {
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "raw"), 0o755); err != nil {
		t.Fatalf("mkdir raw: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "wiki"), 0o755); err != nil {
		t.Fatalf("mkdir wiki: %v", err)
	}
	// raw/ is empty → no Ingest preference.
	// wiki/.last-lint exists but is old → Lint should win.
	sentinel := filepath.Join(vault, "wiki", ".last-lint")
	if err := os.WriteFile(sentinel, []byte("x"), 0o644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}
	old := time.Now().Add(-72 * time.Hour)
	if err := os.Chtimes(sentinel, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	got := SelectStage(vault, 24, time.Now())
	if got != StageLint {
		t.Fatalf("SelectStage = %q, want %q", got, StageLint)
	}
}

func TestSelectStage_DefaultsToIngestWhenNothingDue(t *testing.T) {
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "raw"), 0o755); err != nil {
		t.Fatalf("mkdir raw: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "wiki"), 0o755); err != nil {
		t.Fatalf("mkdir wiki: %v", err)
	}
	// Fresh lint sentinel; empty raw/. Conservative default is StageIngest.
	if err := os.WriteFile(filepath.Join(vault, "wiki", ".last-lint"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write last-lint: %v", err)
	}
	got := SelectStage(vault, 24, time.Now())
	if got != StageIngest {
		t.Fatalf("SelectStage = %q, want %q", got, StageIngest)
	}
}

func TestLLMWikiLoopExecutor_AttemptCounterPropagates(t *testing.T) {
	// The executor's per-stage idempotency contract is "the handler must be
	// idempotent." For this PR, the executor's responsibility is to pass the
	// QueueJobState.Attempt through to the handler so re-claims after crash
	// can detect retry attempts. Here we verify the handler is invoked twice
	// across two Execute calls and the attempt parameter increments.
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "raw"), 0o755); err != nil {
		t.Fatalf("mkdir raw: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "wiki"), 0o755); err != nil {
		t.Fatalf("mkdir wiki: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "raw", "src.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("seed raw: %v", err)
	}

	h := &fakeHandler{stage: StageIngest}
	exec := &LLMWikiLoopExecutor{Ingest: h}

	payload := map[string]any{"vault": vault}
	if _, err := exec.RunJob(context.Background(), mkClaim(daemon.JobTypeLLMWikiLoop, payload, 1)); err != nil {
		t.Fatalf("RunJob attempt 1: %v", err)
	}
	if _, err := exec.RunJob(context.Background(), mkClaim(daemon.JobTypeLLMWikiLoop, payload, 2)); err != nil {
		t.Fatalf("RunJob attempt 2: %v", err)
	}

	if got := atomic.LoadInt32(&h.calls); got != 2 {
		t.Fatalf("handler.calls = %d, want 2", got)
	}
	if got := atomic.LoadInt32(&h.lastAttempt); got != 2 {
		t.Fatalf("handler.lastAttempt = %d, want 2 (executor must propagate attempt)", got)
	}
}

func TestLLMWikiLoopExecutor_NilHandlerSkipsCleanly(t *testing.T) {
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "raw"), 0o755); err != nil {
		t.Fatalf("mkdir raw: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "wiki"), 0o755); err != nil {
		t.Fatalf("mkdir wiki: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "raw", "src.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("seed raw: %v", err)
	}
	// Ingest is nil → executor must record skip, not error.
	exec := &LLMWikiLoopExecutor{}

	res, err := exec.RunJob(context.Background(), mkClaim(daemon.JobTypeLLMWikiLoop, map[string]any{"vault": vault}, 1))
	if err != nil {
		t.Fatalf("RunJob with nil handler returned error: %v", err)
	}
	if res.Artifacts["skipped"] != "true" {
		t.Fatalf("artifacts[skipped] = %q, want %q", res.Artifacts["skipped"], "true")
	}
	if res.Artifacts["skip_reason"] != "handler-not-wired" {
		t.Fatalf("artifacts[skip_reason] = %q, want %q", res.Artifacts["skip_reason"], "handler-not-wired")
	}
	if res.Artifacts["stage"] != string(StageIngest) {
		t.Fatalf("artifacts[stage] = %q, want %q", res.Artifacts["stage"], StageIngest)
	}
}

func TestLLMWikiLoopExecutor_StageWhitelistRespected(t *testing.T) {
	// Vault state would prefer Ingest (raw/ has new content), but the spec
	// pins Stages=[Lint] — executor must run Lint, not Ingest.
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "raw"), 0o755); err != nil {
		t.Fatalf("mkdir raw: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "wiki"), 0o755); err != nil {
		t.Fatalf("mkdir wiki: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "raw", "src.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("seed raw: %v", err)
	}

	ingest := &fakeHandler{stage: StageIngest}
	lint := &fakeHandler{stage: StageLint}
	exec := &LLMWikiLoopExecutor{Ingest: ingest, Lint: lint}

	payload := map[string]any{
		"vault":  vault,
		"stages": []string{"lint"},
	}
	if _, err := exec.RunJob(context.Background(), mkClaim(daemon.JobTypeLLMWikiLoop, payload, 1)); err != nil {
		t.Fatalf("RunJob: %v", err)
	}

	if got := atomic.LoadInt32(&lint.calls); got != 1 {
		t.Fatalf("lint.calls = %d, want 1", got)
	}
	if got := atomic.LoadInt32(&ingest.calls); got != 0 {
		t.Fatalf("ingest.calls = %d, want 0 (whitelist excluded)", got)
	}
}

func TestLLMWikiLoopExecutor_RejectsWrongJobType(t *testing.T) {
	exec := &LLMWikiLoopExecutor{}
	_, err := exec.RunJob(context.Background(), mkClaim(daemon.JobTypeWikiBuild, map[string]any{"vault": "/tmp"}, 1))
	if err == nil {
		t.Fatal("expected error for non-llmwiki job type, got nil")
	}
}

func TestLLMWikiLoopExecutor_RejectsEmptyPayload(t *testing.T) {
	exec := &LLMWikiLoopExecutor{}
	_, err := exec.RunJob(context.Background(), mkClaim(daemon.JobTypeLLMWikiLoop, nil, 1))
	if err == nil {
		t.Fatal("expected error for empty payload, got nil")
	}
}

func TestLLMWikiLoopExecutor_RejectsMissingVault(t *testing.T) {
	exec := &LLMWikiLoopExecutor{}
	_, err := exec.RunJob(context.Background(), mkClaim(daemon.JobTypeLLMWikiLoop, map[string]any{"stages": []string{"lint"}}, 1))
	if err == nil {
		t.Fatal("expected error for missing vault, got nil")
	}
}

func TestLLMWikiLoopExecutor_NowInjectable(t *testing.T) {
	// Using a frozen Now() isolates the test from real wall-clock drift.
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "raw"), 0o755); err != nil {
		t.Fatalf("mkdir raw: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "wiki"), 0o755); err != nil {
		t.Fatalf("mkdir wiki: %v", err)
	}
	sentinel := filepath.Join(vault, "wiki", ".last-lint")
	if err := os.WriteFile(sentinel, []byte("x"), 0o644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}
	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(sentinel, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	frozen := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC) // 96h later
	lint := &fakeHandler{stage: StageLint}
	exec := &LLMWikiLoopExecutor{Lint: lint, Now: func() time.Time { return frozen }}
	if _, err := exec.RunJob(context.Background(), mkClaim(daemon.JobTypeLLMWikiLoop, map[string]any{"vault": vault, "lint_interval_hours": 24}, 1)); err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if got := atomic.LoadInt32(&lint.calls); got != 1 {
		t.Fatalf("lint.calls = %d, want 1 (frozen Now should drive Lint selection)", got)
	}
}
