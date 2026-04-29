package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/boshu2/agentops/cli/internal/agentworker"
	"github.com/boshu2/agentops/cli/internal/wikiworker"
)

type WikiForgeExecutorOptions struct {
	Store         *Store
	Worker        WikiForgeWorker
	QuarantineDir string
}

type WikiForgeExecutor struct {
	store         *Store
	worker        WikiForgeWorker
	quarantineDir string
}

func NewWikiForgeExecutor(opts WikiForgeExecutorOptions) (*WikiForgeExecutor, error) {
	if opts.Store == nil {
		return nil, fmt.Errorf("wiki forge executor: store is required")
	}
	if opts.Worker == nil {
		return nil, fmt.Errorf("wiki forge executor: worker is required")
	}
	return &WikiForgeExecutor{
		store:         opts.Store,
		worker:        opts.Worker,
		quarantineDir: opts.QuarantineDir,
	}, nil
}

func (e *WikiForgeExecutor) JobTypes() []JobType {
	return []JobType{JobTypeWikiForge}
}

func (e *WikiForgeExecutor) RunJob(ctx context.Context, claim QueueClaim) (JobExecutionResult, error) {
	if claim.Job.JobType != JobTypeWikiForge {
		return JobExecutionResult{}, fmt.Errorf("wiki forge executor does not support job type %s", claim.Job.JobType)
	}
	spec, err := WikiForgeJobSpecFromPayload(claim.Job.Payload)
	if err != nil {
		return JobExecutionResult{}, err
	}
	refs := make([]WikiWorkerSessionRef, 0, len(spec.SourcePaths))
	for _, sourcePath := range spec.SourcePaths {
		result, err := e.worker.RunExtractionWithRetry(ctx, wikiworker.ExtractionRequest{
			Prompt:    wikiForgePrompt(sourcePath),
			JobID:     claim.Job.JobID,
			AttemptID: fmt.Sprintf("%d", claim.Job.Attempt),
			RequestID: claim.Job.RequestID,
			Worker:    spec.WorkerKind,
			Provider:  spec.Provider,
			Model:     spec.Model,
			CWD:       spec.CWD,
			Metadata: map[string]string{
				"title":         "wiki forge " + filepath.Base(sourcePath),
				"source_path":   sourcePath,
				"dream_run_id":  spec.DreamRunID,
				"daemon_job_id": claim.Job.JobID,
			},
		}, wikiworker.RetryOptions{
			MaxAttempts:   firstPositive(spec.MaxAttempts, 1),
			QuarantineDir: firstNonEmptyString(spec.QuarantineDir, e.quarantineDir, filepath.Join(e.store.root, ".agents", "quarantine", "agentworker")),
		})
		if err != nil {
			return JobExecutionResult{Artifacts: wikiForgeFailureArtifacts(result, err)}, err
		}
		refs = append(refs, WikiWorkerSessionRef{
			SourcePath: sourcePath,
			Session:    result.Session,
			Terminal:   result.Terminal,
		})
	}
	refsPath, err := writeWikiWorkerSessionRefs(e.store.root, claim.Job.JobID, refs)
	if err != nil {
		return JobExecutionResult{}, err
	}
	return JobExecutionResult{Artifacts: wikiForgeSuccessArtifacts(refsPath, refs)}, nil
}

func wikiForgeSuccessArtifacts(refsPath string, refs []WikiWorkerSessionRef) map[string]string {
	artifacts := map[string]string{"worker_session_refs": refsPath}
	if len(refs) == 0 {
		return artifacts
	}
	last := refs[len(refs)-1]
	artifacts["provider_request_id"] = last.Session.ProviderRequestID
	artifacts["session_id"] = last.Session.SessionID
	artifacts["event_cursor"] = last.Session.EventCursor
	artifacts["terminal_status"] = string(last.Terminal.Status)
	return artifacts
}

func wikiForgeFailureArtifacts(result wikiworker.ExtractionResult, err error) map[string]string {
	artifacts := map[string]string{}
	if result.Session.SessionID != "" {
		artifacts["provider_request_id"] = result.Session.ProviderRequestID
		artifacts["session_id"] = result.Session.SessionID
		artifacts["event_cursor"] = result.Session.EventCursor
	}
	if result.Terminal.Status != "" {
		artifacts["terminal_status"] = string(result.Terminal.Status)
	}
	var quarantineErr *wikiworker.QuarantineError
	if errors.As(err, &quarantineErr) {
		artifacts["quarantine_path"] = quarantineErr.Path
	}
	if len(artifacts) == 0 {
		return nil
	}
	return artifacts
}

// NewFakeWikiAgentWorker returns an in-memory AgentWorker for deterministic
// daemon wiki executor tests and fake foreground policy.
func NewFakeWikiAgentWorker() agentworker.AgentWorker {
	return fakeWikiAgentWorker{}
}

type fakeWikiAgentWorker struct{}

func (fakeWikiAgentWorker) Start(_ context.Context, req agentworker.StartRequest) (agentworker.AgentSession, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	return &fakeWikiAgentSession{req: req}, nil
}

func (fakeWikiAgentWorker) Attach(_ context.Context, ref agentworker.SessionRef) (agentworker.AgentSession, error) {
	if err := ref.Validate(); err != nil {
		return nil, err
	}
	return &fakeWikiAgentSession{ref: ref}, nil
}

type fakeWikiAgentSession struct {
	req agentworker.StartRequest
	ref agentworker.SessionRef
}

func (s *fakeWikiAgentSession) Ref() agentworker.SessionRef {
	if s.ref.SessionID != "" {
		return s.ref
	}
	return agentworker.SessionRef{
		WorkerKind:        s.req.WorkerKind,
		Provider:          agentworker.ProviderFake,
		JobID:             s.req.JobID,
		AttemptID:         s.req.AttemptID,
		RequestID:         s.req.RequestID,
		ProviderRequestID: "fake-wiki-request-" + sanitizeWikiArtifactName(s.req.JobID),
		SessionID:         "fake-wiki-session-" + sanitizeWikiArtifactName(s.req.JobID),
		EventCursor:       "fake-cursor-terminal",
		Status:            agentworker.StatusCompleted,
	}
}

func (s *fakeWikiAgentSession) Nudge(context.Context, agentworker.NudgeRequest) error {
	return nil
}

func (s *fakeWikiAgentSession) Cancel(context.Context, agentworker.CancelRequest) error {
	return nil
}

func (s *fakeWikiAgentSession) Stream(context.Context, agentworker.StreamOptions) (<-chan agentworker.Event, error) {
	ch := make(chan agentworker.Event, 1)
	ch <- agentworker.Event{
		Cursor: "fake-cursor-terminal",
		At:     time.Now().UTC(),
		Type:   agentworker.EventTerminal,
		State:  agentworker.TerminalState{Status: agentworker.StatusCompleted},
	}
	close(ch)
	return ch, nil
}

func (s *fakeWikiAgentSession) Transcript(context.Context) (agentworker.Transcript, error) {
	payload := map[string]any{
		"schema_version": 1,
		"title":          "Fake daemon wiki extraction",
		"summary":        "The fake AgentWorker produced deterministic wiki extraction output.",
		"entities":       []string{"AgentOps", "AgentWorker"},
		"concepts":       []string{"daemon wiki executor"},
		"decisions":      []string{"Use AgentWorker for daemon wiki jobs"},
		"open_questions": []string{},
		"work_phase":     "implement",
	}
	envelope := map[string]any{
		"schema_version": 1,
		"session":        s.Ref(),
		"status":         string(agentworker.StatusCompleted),
		"payload":        payload,
		"artifacts": []map[string]string{{
			"kind":              "wiki-note",
			"path":              ".agents/wiki/sources/fake-daemon-wiki.md",
			"validation_status": "valid",
		}},
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return agentworker.Transcript{}, err
	}
	return agentworker.Transcript{Text: string(data)}, nil
}

func (s *fakeWikiAgentSession) Artifacts(context.Context) ([]agentworker.Artifact, error) {
	return []agentworker.Artifact{{
		Kind:             "wiki-note",
		Path:             ".agents/wiki/sources/fake-daemon-wiki.md",
		SessionID:        s.Ref().SessionID,
		ValidationStatus: "valid",
	}}, nil
}

func (s *fakeWikiAgentSession) TerminalState(context.Context) (agentworker.TerminalState, error) {
	return agentworker.TerminalState{Status: agentworker.StatusCompleted}, nil
}
