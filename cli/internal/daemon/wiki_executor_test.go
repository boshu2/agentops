package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/agentworker"
	"github.com/boshu2/agentops/cli/internal/wikiworker"
)

func TestWikiForgeExecutorCompletesJobWithAgentWorkerSessionRefs(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	queue := newTestQueue(t, &now, QueueOptions{LeaseDuration: time.Minute})
	sourcePath := filepath.Join(store.root, "session-a.jsonl")
	if err := os.WriteFile(sourcePath, []byte("decision: restore fake baseline after smoke\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	spec := NewWikiForgeJobSpec("dream-1", ".agents/wiki/sources", []string{sourcePath})
	spec.Provider = agentworker.Provider("fake")
	jobSpec, err := spec.ToJobSpec("job-wiki")
	if err != nil {
		t.Fatalf("ToJobSpec: %v", err)
	}
	if _, err := queue.SubmitJob(SubmitJobInput{
		RequestID: "req-wiki",
		JobID:     jobSpec.ID,
		JobType:   jobSpec.Type,
		Payload:   jobSpec.Payload,
	}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	executor, err := NewWikiForgeExecutor(WikiForgeExecutorOptions{Store: store, Worker: successfulWikiForgeWorker{}})
	if err != nil {
		t.Fatalf("NewWikiForgeExecutor: %v", err)
	}
	supervisor := newTestSupervisor(t, queue, executor)

	result, err := supervisor.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if result.Job.Status != JobStatusCompleted {
		t.Fatalf("job status = %q, want completed", result.Job.Status)
	}
	for _, key := range []string{"worker_session_refs", "provider_request_id", "session_id", "event_cursor", "terminal_status"} {
		if result.Job.Artifacts[key] == "" {
			t.Fatalf("missing artifact %q in %#v", key, result.Job.Artifacts)
		}
	}
	if result.Job.Artifacts["terminal_status"] != string(agentworker.StatusCompleted) {
		t.Fatalf("terminal_status = %q", result.Job.Artifacts["terminal_status"])
	}
	ref := result.Job.ArtifactRefs["worker_session_refs"]
	if err := ref.Validate(); err != nil {
		t.Fatalf("worker_session_refs ref invalid: %v", err)
	}
	if result.Job.Artifacts["worker_session_refs"] != ref.Path {
		t.Fatalf("compat worker_session_refs = %q, want %q", result.Job.Artifacts["worker_session_refs"], ref.Path)
	}
}

// TestWikiForgeExecutor_ExpandsDirectorySourcePaths verifies that listing a
// directory in source_paths walks its top-level files. Starter schedules
// commonly point at .agents/sessions/ rather than enumerating each transcript
// by hand; the executor must expand the directory before reading.
func TestWikiForgeExecutor_ExpandsDirectorySourcePaths(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	queue := newTestQueue(t, &now, QueueOptions{LeaseDuration: time.Minute})
	srcDir := filepath.Join(store.root, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir srcDir: %v", err)
	}
	for _, name := range []string{"a.jsonl", "b.jsonl"} {
		if err := os.WriteFile(filepath.Join(srcDir, name), []byte("decision: noop\n"), 0o644); err != nil {
			t.Fatalf("write source %s: %v", name, err)
		}
	}
	spec := NewWikiForgeJobSpec("dream-1", ".agents/wiki/sources", []string{srcDir})
	spec.Provider = agentworker.Provider("fake")
	jobSpec, err := spec.ToJobSpec("job-wiki-dir")
	if err != nil {
		t.Fatalf("ToJobSpec: %v", err)
	}
	if _, err := queue.SubmitJob(SubmitJobInput{
		RequestID: "req-wiki-dir",
		JobID:     jobSpec.ID,
		JobType:   jobSpec.Type,
		Payload:   jobSpec.Payload,
	}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	executor, err := NewWikiForgeExecutor(WikiForgeExecutorOptions{Store: store, Worker: successfulWikiForgeWorker{}})
	if err != nil {
		t.Fatalf("NewWikiForgeExecutor: %v", err)
	}
	supervisor := newTestSupervisor(t, queue, executor)
	result, err := supervisor.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if result.Job.Status != JobStatusCompleted {
		t.Fatalf("job status = %q, want completed (failure: %+v)", result.Job.Status, result.Job.Failure)
	}
}

func TestWikiForgeExecutorInvalidOutputFailsWithQuarantineArtifact(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	queue := newTestQueue(t, &now, QueueOptions{LeaseDuration: time.Minute})
	sourcePath := filepath.Join(store.root, "session-a.jsonl")
	if err := os.WriteFile(sourcePath, []byte("decision: quarantine invalid worker output\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	spec := NewWikiForgeJobSpec("dream-1", ".agents/wiki/sources", []string{sourcePath})
	jobSpec, err := spec.ToJobSpec("job-wiki")
	if err != nil {
		t.Fatalf("ToJobSpec: %v", err)
	}
	if _, err := queue.SubmitJob(SubmitJobInput{
		RequestID: "req-wiki",
		JobID:     jobSpec.ID,
		JobType:   jobSpec.Type,
		Payload:   jobSpec.Payload,
	}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	executor, err := NewWikiForgeExecutor(WikiForgeExecutorOptions{
		Store:  store,
		Worker: failingWikiForgeWorker{quarantinePath: ".agents/quarantine/wiki.json"},
	})
	if err != nil {
		t.Fatalf("NewWikiForgeExecutor: %v", err)
	}
	supervisor := newTestSupervisor(t, queue, executor)

	result, err := supervisor.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if result.Job.Status != JobStatusFailed {
		t.Fatalf("job status = %q, want failed", result.Job.Status)
	}
	if result.Job.Artifacts["quarantine_path"] != ".agents/quarantine/wiki.json" {
		t.Fatalf("artifacts = %#v, want quarantine path", result.Job.Artifacts)
	}
	if result.Job.Artifacts["terminal_status"] != string(agentworker.StatusCompleted) {
		t.Fatalf("terminal artifact = %#v", result.Job.Artifacts)
	}
}

// TestWikiExecutor_RespectsCtxCancelBeforeRun guards against the regression
// described in bd soc-58q5.12: RunJob must observe context cancellation
// issued mid-claim before doing any work (payload parse, source expansion,
// or worker spawn). Without the early ctx.Err() guard, a cancel issued
// between QueueClaim and execution start is silently ignored until an
// underlying call happens to block.
func TestWikiExecutor_RespectsCtxCancelBeforeRun(t *testing.T) {
	store := NewStore(t.TempDir())
	sourcePath := filepath.Join(store.root, "session-cancel.jsonl")
	if err := os.WriteFile(sourcePath, []byte("decision: should not be read\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	spec := NewWikiForgeJobSpec("dream-cancel", ".agents/wiki/sources", []string{sourcePath})
	spec.Provider = agentworker.Provider("fake")
	jobSpec, err := spec.ToJobSpec("job-wiki-cancel")
	if err != nil {
		t.Fatalf("ToJobSpec: %v", err)
	}
	worker := &recordingWikiForgeWorker{}
	executor, err := NewWikiForgeExecutor(WikiForgeExecutorOptions{Store: store, Worker: worker})
	if err != nil {
		t.Fatalf("NewWikiForgeExecutor: %v", err)
	}

	claim := QueueClaim{
		Job: QueueJobState{
			JobID:     jobSpec.ID,
			JobType:   jobSpec.Type,
			RequestID: "req-cancel",
			Status:    JobStatusRunning,
			Attempt:   1,
			Payload:   jobSpec.Payload,
		},
		ClaimToken: "claim-cancel",
		LeaseEpoch: 1,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := executor.RunJob(ctx, claim)
	if err == nil {
		t.Fatalf("RunJob: expected error from canceled ctx, got nil (result=%+v)", result)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunJob: err = %v, want context.Canceled", err)
	}
	if worker.calls != 0 {
		t.Fatalf("worker.RunExtractionWithRetry called %d times, want 0 (no side effects)", worker.calls)
	}
	if len(result.Artifacts) != 0 {
		t.Fatalf("result.Artifacts = %#v, want empty (no side effects)", result.Artifacts)
	}
	if len(result.ArtifactRefs) != 0 {
		t.Fatalf("result.ArtifactRefs = %#v, want empty (no side effects)", result.ArtifactRefs)
	}
	// No worker_session_refs file should have been written under the store
	// root for this job — the cancel must short-circuit before any disk write.
	refsGlob := filepath.Join(store.root, ".agents", "**", jobSpec.ID+"*")
	matches, _ := filepath.Glob(refsGlob)
	if len(matches) != 0 {
		t.Fatalf("found unexpected artifacts on disk: %v", matches)
	}
}

type recordingWikiForgeWorker struct {
	calls int
}

func (r *recordingWikiForgeWorker) RunExtractionWithRetry(_ context.Context, req wikiworker.ExtractionRequest, _ wikiworker.RetryOptions) (wikiworker.ExtractionResult, error) {
	r.calls++
	return wikiworker.ExtractionResult{
		Session: agentworker.SessionRef{
			WorkerKind:        req.Worker,
			Provider:          req.Provider,
			JobID:             req.JobID,
			AttemptID:         req.AttemptID,
			RequestID:         req.RequestID,
			ProviderRequestID: "provider-request",
			SessionID:         "session-recording",
			EventCursor:       "cursor-recording",
			Status:            agentworker.StatusCompleted,
		},
		Terminal: agentworker.TerminalState{Status: agentworker.StatusCompleted},
	}, nil
}

type successfulWikiForgeWorker struct{}

func (successfulWikiForgeWorker) RunExtractionWithRetry(_ context.Context, req wikiworker.ExtractionRequest, _ wikiworker.RetryOptions) (wikiworker.ExtractionResult, error) {
	result := wikiworker.ExtractionResult{
		Session: agentworker.SessionRef{
			WorkerKind:        req.Worker,
			Provider:          req.Provider,
			JobID:             req.JobID,
			AttemptID:         req.AttemptID,
			RequestID:         req.RequestID,
			ProviderRequestID: "provider-request",
			SessionID:         "session-valid",
			EventCursor:       "cursor-valid",
			Status:            agentworker.StatusCompleted,
		},
		Terminal: agentworker.TerminalState{Status: agentworker.StatusCompleted},
	}
	return result, nil
}

type failingWikiForgeWorker struct {
	quarantinePath string
}

func (f failingWikiForgeWorker) RunExtractionWithRetry(_ context.Context, req wikiworker.ExtractionRequest, _ wikiworker.RetryOptions) (wikiworker.ExtractionResult, error) {
	result := wikiworker.ExtractionResult{
		Session: agentworker.SessionRef{
			WorkerKind:        req.Worker,
			Provider:          req.Provider,
			JobID:             req.JobID,
			AttemptID:         req.AttemptID,
			RequestID:         req.RequestID,
			ProviderRequestID: "provider-request",
			SessionID:         "session-invalid",
			EventCursor:       "cursor-invalid",
			Status:            agentworker.StatusCompleted,
		},
		Terminal: agentworker.TerminalState{Status: agentworker.StatusCompleted},
	}
	return result, &wikiworker.QuarantineError{Path: f.quarantinePath, Err: errors.New("invalid output")}
}
