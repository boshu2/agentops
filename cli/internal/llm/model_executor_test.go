package llm

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/agentworker"
)

func TestAgentWorkerGeneratorLifecycleReturnsTranscript(t *testing.T) {
	worker := &fakeModelWorker{
		transcript: "worker json response",
		state:      agentworker.TerminalState{Status: agentworker.StatusCompleted},
	}
	gen, err := NewAgentWorkerGenerator(AgentWorkerGeneratorOptions{
		Worker:        worker,
		WorkerKind:    agentworker.WorkerKindCodex,
		Provider:      agentworker.ProviderFake,
		Model:         "codex-headless",
		JobID:         "wiki.forge:1",
		AttemptID:     "attempt-1",
		RequestID:     "req-1",
		ContextBudget: 32000,
		Digest:        "fake:digest",
	})
	if err != nil {
		t.Fatalf("NewAgentWorkerGenerator: %v", err)
	}

	got, err := gen.Generate("extract lessons")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got != "worker json response" {
		t.Fatalf("Generate: want transcript, got %q", got)
	}
	if worker.started.Prompt != "extract lessons" {
		t.Fatalf("start prompt: %q", worker.started.Prompt)
	}
	if worker.started.JobID != "wiki.forge:1" || worker.started.AttemptID != "attempt-1" {
		t.Fatalf("start ownership not preserved: %#v", worker.started)
	}
	if gen.ModelName() != "codex-headless" {
		t.Fatalf("model name: %s", gen.ModelName())
	}
	if gen.Digest() != "fake:digest" {
		t.Fatalf("digest: %s", gen.Digest())
	}
	if gen.ContextBudget() != 32000 {
		t.Fatalf("context budget: %d", gen.ContextBudget())
	}
}

func TestAgentWorkerGeneratorTerminalFailureReturnsError(t *testing.T) {
	worker := &fakeModelWorker{
		transcript: "not usable",
		state: agentworker.TerminalState{
			Status:      agentworker.StatusLost,
			FailureCode: string(agentworker.StatusLost),
			Reason:      "session ID previously known but provider cannot find it",
		},
	}
	gen, err := NewAgentWorkerGenerator(AgentWorkerGeneratorOptions{
		Worker:     worker,
		WorkerKind: agentworker.WorkerKindClaude,
		Provider:   agentworker.ProviderFake,
		Model:      "claude-headless",
	})
	if err != nil {
		t.Fatalf("NewAgentWorkerGenerator: %v", err)
	}

	_, err = gen.Generate("extract lessons")
	if err == nil {
		t.Fatal("Generate should fail for lost terminal state")
	}
	if !strings.Contains(err.Error(), "lost") {
		t.Fatalf("error should classify lost state, got %v", err)
	}
}

func TestAgentWorkerGeneratorUsesTerminalStateAfterStreamReplay(t *testing.T) {
	worker := &fakeModelWorker{
		transcript:      "fallback terminal transcript",
		state:           agentworker.TerminalState{Status: agentworker.StatusCompleted},
		omitTerminalEvt: true,
	}
	gen, err := NewAgentWorkerGenerator(AgentWorkerGeneratorOptions{
		Worker:     worker,
		WorkerKind: agentworker.WorkerKindCodex,
		Provider:   agentworker.ProviderFake,
		Model:      "codex-headless",
	})
	if err != nil {
		t.Fatalf("NewAgentWorkerGenerator: %v", err)
	}

	got, err := gen.Generate("extract lessons")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got != "fallback terminal transcript" {
		t.Fatalf("Generate: %q", got)
	}
	if !worker.terminalQueried {
		t.Fatal("TerminalState should be queried when stream has no terminal event")
	}
}

type fakeModelWorker struct {
	started         agentworker.StartRequest
	transcript      string
	state           agentworker.TerminalState
	omitTerminalEvt bool
	terminalQueried bool
}

func (w *fakeModelWorker) Start(_ context.Context, req agentworker.StartRequest) (agentworker.AgentSession, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	w.started = req
	return &fakeModelSession{worker: w}, nil
}

func (w *fakeModelWorker) Attach(_ context.Context, ref agentworker.SessionRef) (agentworker.AgentSession, error) {
	if err := ref.Validate(); err != nil {
		return nil, err
	}
	return &fakeModelSession{worker: w, ref: ref}, nil
}

type fakeModelSession struct {
	worker *fakeModelWorker
	ref    agentworker.SessionRef
}

func (s *fakeModelSession) Ref() agentworker.SessionRef {
	if s.ref.SessionID != "" {
		return s.ref
	}
	return agentworker.SessionRef{
		WorkerKind: s.worker.started.WorkerKind,
		Provider:   s.worker.started.Provider,
		JobID:      s.worker.started.JobID,
		AttemptID:  s.worker.started.AttemptID,
		RequestID:  s.worker.started.RequestID,
		SessionID:  "fake-model-session",
		Status:     agentworker.StatusRunning,
	}
}

func (s *fakeModelSession) Nudge(_ context.Context, _ agentworker.NudgeRequest) error {
	return nil
}

func (s *fakeModelSession) Cancel(_ context.Context, _ agentworker.CancelRequest) error {
	s.worker.state = agentworker.TerminalState{Status: agentworker.StatusCancelled}
	return nil
}

func (s *fakeModelSession) Stream(_ context.Context, _ agentworker.StreamOptions) (<-chan agentworker.Event, error) {
	ch := make(chan agentworker.Event, 2)
	ch <- agentworker.Event{
		Cursor:  "1",
		At:      time.Unix(200, 0).UTC(),
		Type:    agentworker.EventStarted,
		Message: s.worker.started.Prompt,
		State:   agentworker.TerminalState{Status: agentworker.StatusRunning},
	}
	if !s.worker.omitTerminalEvt {
		ch <- agentworker.Event{
			Cursor: "2",
			At:     time.Unix(201, 0).UTC(),
			Type:   agentworker.EventTerminal,
			State:  s.worker.state,
		}
	}
	close(ch)
	return ch, nil
}

func (s *fakeModelSession) Transcript(_ context.Context) (agentworker.Transcript, error) {
	if s.worker.transcript == "error" {
		return agentworker.Transcript{}, fmt.Errorf("transcript failed")
	}
	return agentworker.Transcript{Text: s.worker.transcript}, nil
}

func (s *fakeModelSession) Artifacts(_ context.Context) ([]agentworker.Artifact, error) {
	return nil, nil
}

func (s *fakeModelSession) TerminalState(_ context.Context) (agentworker.TerminalState, error) {
	s.worker.terminalQueried = true
	return s.worker.state, nil
}
