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
	sourcePath := filepath.Join(t.TempDir(), "session-a.jsonl")
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

func TestWikiForgeExecutorInvalidOutputFailsWithQuarantineArtifact(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	queue := newTestQueue(t, &now, QueueOptions{LeaseDuration: time.Minute})
	sourcePath := filepath.Join(t.TempDir(), "session-a.jsonl")
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
