package wikiworker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/agentworker"
)

func TestWikiWorkerRunsGasCityCodexAgentWorker(t *testing.T) {
	agent := &fakeWikiAgentWorker{
		transcript: validWorkerEnvelope("codex", validExtractionPayload()),
		terminal:   agentworker.TerminalState{Status: agentworker.StatusCompleted},
	}
	worker, err := NewWorker(agent)
	if err != nil {
		t.Fatalf("NewWorker: %v", err)
	}

	result, err := worker.RunExtraction(context.Background(), ExtractionRequest{
		Prompt:    "extract wiki",
		JobID:     "wiki.forge:1",
		AttemptID: "attempt-1",
		RequestID: "req-1",
		Worker:    agentworker.WorkerKindCodex,
		Provider:  agentworker.ProviderGasCity,
		Model:     "codex-headless",
	})
	if err != nil {
		t.Fatalf("RunExtraction: %v", err)
	}
	if agent.started.WorkerKind != agentworker.WorkerKindCodex || agent.started.Provider != agentworker.ProviderGasCity {
		t.Fatalf("started request: %#v", agent.started)
	}
	if result.Extraction.Title != "GasCity worker extracts wiki" {
		t.Fatalf("title: %q", result.Extraction.Title)
	}
	if !result.Terminal.Successful() {
		t.Fatalf("terminal: %#v", result.Terminal)
	}
}

func TestWikiWorkerRejectsClaudeTerminalFailure(t *testing.T) {
	agent := &fakeWikiAgentWorker{
		transcript: validWorkerEnvelope("claude", validExtractionPayload()),
		terminal: agentworker.TerminalState{
			Status:      agentworker.StatusLost,
			FailureCode: string(agentworker.StatusLost),
			Reason:      "session missing after acceptance",
		},
	}
	worker, err := NewWorker(agent)
	if err != nil {
		t.Fatalf("NewWorker: %v", err)
	}

	result, err := worker.RunExtraction(context.Background(), ExtractionRequest{
		Prompt:   "extract wiki",
		Worker:   agentworker.WorkerKindClaude,
		Provider: agentworker.ProviderGasCity,
	})
	if err == nil {
		t.Fatal("RunExtraction should fail for lost Claude session")
	}
	if result.Terminal.Status != agentworker.StatusLost {
		t.Fatalf("terminal: %#v", result.Terminal)
	}
}

func TestWikiWorkerRejectsInvalidGasCityWorkerOutput(t *testing.T) {
	agent := &fakeWikiAgentWorker{
		transcript: `{"schema_version":1,"refusal":"nope"}`,
		terminal:   agentworker.TerminalState{Status: agentworker.StatusCompleted},
	}
	worker, err := NewWorker(agent)
	if err != nil {
		t.Fatalf("NewWorker: %v", err)
	}

	_, err = worker.RunExtraction(context.Background(), ExtractionRequest{Prompt: "extract wiki"})
	if err == nil || !strings.Contains(err.Error(), "refusal") {
		t.Fatalf("want refusal error, got %v", err)
	}
}

func TestWikiWorkerRetrySucceedsBeforeQuarantine(t *testing.T) {
	quarantineDir := filepath.Join(t.TempDir(), "quarantine")
	agent := &fakeWikiAgentWorker{
		transcripts: []string{
			`{"schema_version":1,"refusal":"not enough context"}`,
			validWorkerEnvelope("codex", validExtractionPayload()),
		},
		terminal: agentworker.TerminalState{Status: agentworker.StatusCompleted},
	}
	worker, err := NewWorker(agent)
	if err != nil {
		t.Fatalf("NewWorker: %v", err)
	}

	result, err := worker.RunExtractionWithRetry(context.Background(), ExtractionRequest{
		Prompt:   "extract wiki",
		JobID:    "wiki.forge:retry",
		Worker:   agentworker.WorkerKindCodex,
		Provider: agentworker.ProviderGasCity,
	}, RetryOptions{MaxAttempts: 2, QuarantineDir: quarantineDir})
	if err != nil {
		t.Fatalf("RunExtractionWithRetry: %v", err)
	}
	if result.Extraction.Title == "" {
		t.Fatalf("missing extraction: %#v", result)
	}
	if agent.startCount != 2 {
		t.Fatalf("start count: want 2, got %d", agent.startCount)
	}
	entries, err := os.ReadDir(quarantineDir)
	if err == nil && len(entries) != 0 {
		t.Fatalf("quarantine should be empty, got %d entries", len(entries))
	}
}

func TestWikiWorkerQuarantineAfterRetryCap(t *testing.T) {
	quarantineDir := filepath.Join(t.TempDir(), "quarantine")
	agent := &fakeWikiAgentWorker{
		transcripts: []string{
			`{"schema_version":1,"refusal":"bad first output"}`,
			`{"schema_version":1,"refusal":"bad second output"}`,
		},
		terminal: agentworker.TerminalState{Status: agentworker.StatusCompleted},
	}
	worker, err := NewWorker(agent)
	if err != nil {
		t.Fatalf("NewWorker: %v", err)
	}

	_, err = worker.RunExtractionWithRetry(context.Background(), ExtractionRequest{
		Prompt:    "extract wiki",
		JobID:     "wiki.forge:quarantine",
		AttemptID: "attempt-7",
		RequestID: "req-7",
		Worker:    agentworker.WorkerKindCodex,
		Provider:  agentworker.ProviderGasCity,
	}, RetryOptions{
		MaxAttempts:   2,
		QuarantineDir: quarantineDir,
		Writer: agentworker.QuarantineWriter{
			Now: func() time.Time { return time.Date(2026, 4, 28, 13, 0, 0, 0, time.UTC) },
		},
	})
	var quarantineErr *QuarantineError
	if !errors.As(err, &quarantineErr) {
		t.Fatalf("want QuarantineError, got %v", err)
	}
	data, readErr := os.ReadFile(quarantineErr.Path)
	if readErr != nil {
		t.Fatalf("read quarantine: %v", readErr)
	}
	var record agentworker.QuarantineRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("decode quarantine: %v", err)
	}
	if record.JobID != "wiki.forge:quarantine" || record.AttemptID != "attempt-7" || record.RequestID != "req-7" {
		t.Fatalf("record refs: %#v", record)
	}
	if record.Session.SessionID != "wiki-session" || record.Attempts != 2 {
		t.Fatalf("record session/attempts: %#v", record)
	}
	if !strings.Contains(record.RawOutput, "bad second output") {
		t.Fatalf("record raw output: %q", record.RawOutput)
	}
}

type fakeWikiAgentWorker struct {
	started     agentworker.StartRequest
	startCount  int
	transcript  string
	transcripts []string
	terminal    agentworker.TerminalState
}

func (f *fakeWikiAgentWorker) Start(_ context.Context, req agentworker.StartRequest) (agentworker.AgentSession, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	f.started = req
	f.startCount++
	return &fakeWikiAgentSession{worker: f, attempt: f.startCount}, nil
}

func (f *fakeWikiAgentWorker) Attach(_ context.Context, ref agentworker.SessionRef) (agentworker.AgentSession, error) {
	if err := ref.Validate(); err != nil {
		return nil, err
	}
	return &fakeWikiAgentSession{worker: f, ref: ref}, nil
}

type fakeWikiAgentSession struct {
	worker  *fakeWikiAgentWorker
	ref     agentworker.SessionRef
	attempt int
}

func (s *fakeWikiAgentSession) Ref() agentworker.SessionRef {
	if s.ref.SessionID != "" {
		return s.ref
	}
	return agentworker.SessionRef{
		WorkerKind: s.worker.started.WorkerKind,
		Provider:   s.worker.started.Provider,
		JobID:      s.worker.started.JobID,
		AttemptID:  s.worker.started.AttemptID,
		RequestID:  s.worker.started.RequestID,
		SessionID:  "wiki-session",
		Status:     s.worker.terminal.Status,
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
	ch <- agentworker.Event{Type: agentworker.EventTerminal, State: s.worker.terminal}
	close(ch)
	return ch, nil
}

func (s *fakeWikiAgentSession) Transcript(context.Context) (agentworker.Transcript, error) {
	if len(s.worker.transcripts) > 0 {
		index := s.attempt - 1
		if index < 0 {
			index = 0
		}
		if index >= len(s.worker.transcripts) {
			index = len(s.worker.transcripts) - 1
		}
		return agentworker.Transcript{Text: s.worker.transcripts[index]}, nil
	}
	return agentworker.Transcript{Text: s.worker.transcript}, nil
}

func (s *fakeWikiAgentSession) Artifacts(context.Context) ([]agentworker.Artifact, error) {
	return []agentworker.Artifact{{
		Kind:             "wiki-note",
		Path:             ".agents/wiki/sources/session.md",
		SessionID:        s.Ref().SessionID,
		ValidationStatus: "valid",
	}}, nil
}

func (s *fakeWikiAgentSession) TerminalState(context.Context) (agentworker.TerminalState, error) {
	return s.worker.terminal, nil
}

func validWorkerEnvelope(kind string, payload string) string {
	return fmt.Sprintf(`{
		"schema_version": 1,
		"session": {
			"worker_kind": %q,
			"provider": "gascity",
			"session_id": "wiki-session",
			"status": "completed"
		},
		"status": "completed",
		"payload": %s,
		"artifacts": [{"kind":"wiki-note","path":".agents/wiki/sources/session.md","validation_status":"valid"}]
	}`, kind, payload)
}

func validExtractionPayload() string {
	return `{
		"schema_version": 1,
		"title": "GasCity worker extracts wiki",
		"summary": "The wiki worker validates structured GasCity output before persisting it.",
		"entities": ["GasCity", "AgentWorker"],
		"concepts": ["provider-neutral wiki extraction"],
		"decisions": ["Validate worker output before wiki writes"],
		"open_questions": [],
		"work_phase": "implement"
	}`
}
