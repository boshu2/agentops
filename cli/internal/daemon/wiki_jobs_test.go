package daemon

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/boshu2/agentops/cli/internal/agentworker"
	"github.com/boshu2/agentops/cli/internal/wikiworker"
)

func TestWikiForgeJobSpecRoundTrip(t *testing.T) {
	spec := NewWikiForgeJobSpec("dream-1", ".agents/wiki/sources", []string{"session.jsonl"})
	job, err := spec.ToJobSpec("job-wiki-dream-1")
	if err != nil {
		t.Fatalf("ToJobSpec: %v", err)
	}
	got, err := WikiForgeJobSpecFromPayload(job.Payload)
	if err != nil {
		t.Fatalf("WikiForgeJobSpecFromPayload: %v", err)
	}
	if got.JobType != JobTypeWikiForge || got.WorkerKind != agentworker.WorkerKindCodex || got.Provider != agentworker.ProviderGasCity {
		t.Fatalf("spec roundtrip: %#v", got)
	}
}

func TestWikiForgeRunnerCompletesJobWithAgentWorkerSessionRefs(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	queue := NewQueue(store, QueueOptions{})
	spec := NewWikiForgeJobSpec("dream-1", ".agents/wiki/sources", []string{"session-a.jsonl"})
	jobSpec, err := spec.ToJobSpec("job-wiki-dream-1")
	if err != nil {
		t.Fatalf("ToJobSpec: %v", err)
	}
	if _, err := queue.SubmitJob(SubmitJobInput{
		RequestID:      "req-wiki-1",
		JobID:          jobSpec.ID,
		JobType:        jobSpec.Type,
		IdempotencyKey: "wiki.forge:dream-1",
		Payload:        jobSpec.Payload,
	}, QueueMutationOptions{}); err != nil {
		t.Fatalf("SubmitJob: %v", err)
	}

	worker := &fakeWikiForgeWorker{sessionID: "sess_wiki_1"}
	runner, err := NewWikiForgeRunner(store, WikiForgeRunnerOptions{Queue: queue, Worker: worker})
	if err != nil {
		t.Fatalf("NewWikiForgeRunner: %v", err)
	}
	result, err := runner.RunWikiForgeJob(context.Background(), "job-wiki-dream-1")
	if err != nil {
		t.Fatalf("RunWikiForgeJob: %v", err)
	}
	if result.Status != JobStatusCompleted {
		t.Fatalf("status: %s", result.Status)
	}
	if len(result.WorkerSessions) != 1 || result.WorkerSessions[0].Session.SessionID != "sess_wiki_1" {
		t.Fatalf("worker sessions: %#v", result.WorkerSessions)
	}
	refsPath := result.Artifacts["worker_session_refs"]
	data, err := os.ReadFile(refsPath)
	if err != nil {
		t.Fatalf("read refs artifact: %v", err)
	}
	var artifact struct {
		WorkerSessions []WikiWorkerSessionRef `json:"worker_sessions"`
	}
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatalf("decode refs artifact: %v", err)
	}
	if len(artifact.WorkerSessions) != 1 || artifact.WorkerSessions[0].Session.SessionID != "sess_wiki_1" {
		t.Fatalf("refs artifact: %#v", artifact)
	}
}

type fakeWikiForgeWorker struct {
	sessionID string
	requests  []wikiworker.ExtractionRequest
}

func (f *fakeWikiForgeWorker) RunExtractionWithRetry(_ context.Context, req wikiworker.ExtractionRequest, _ wikiworker.RetryOptions) (wikiworker.ExtractionResult, error) {
	f.requests = append(f.requests, req)
	return wikiworker.ExtractionResult{
		Session: agentworker.SessionRef{
			WorkerKind: req.Worker,
			Provider:   req.Provider,
			JobID:      req.JobID,
			AttemptID:  req.AttemptID,
			RequestID:  req.RequestID,
			SessionID:  f.sessionID,
			Status:     agentworker.StatusCompleted,
		},
		Terminal: agentworker.TerminalState{Status: agentworker.StatusCompleted},
	}, nil
}
