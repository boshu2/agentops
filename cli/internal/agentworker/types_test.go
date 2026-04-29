package agentworker

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestAgentWorkerLifecycleFakeSession(t *testing.T) {
	ctx := context.Background()
	worker := newFakeLifecycleWorker()

	session, err := worker.Start(ctx, StartRequest{
		WorkerKind: WorkerKindCodex,
		Provider:   ProviderFake,
		JobID:      "job-1",
		AttemptID:  "attempt-1",
		RequestID:  "req-1",
		Prompt:     "extract wiki lessons",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	ref := session.Ref()
	if err := ref.Validate(); err != nil {
		t.Fatalf("ref validate: %v", err)
	}
	if ref.Status != StatusRunning {
		t.Fatalf("status: want running, got %s", ref.Status)
	}

	attached, err := worker.Attach(ctx, ref)
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	if attached.Ref().SessionID != ref.SessionID {
		t.Fatalf("attach session id: want %q, got %q", ref.SessionID, attached.Ref().SessionID)
	}

	if err := attached.Nudge(ctx, NudgeRequest{Message: "include decisions"}); err != nil {
		t.Fatalf("Nudge: %v", err)
	}

	events, err := attached.Stream(ctx, StreamOptions{})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	var sawNudge bool
	var terminal TerminalState
	for event := range events {
		if event.Type == EventNudged {
			sawNudge = true
		}
		if event.Type == EventTerminal {
			terminal = event.State
		}
	}
	if !sawNudge {
		t.Fatal("stream did not include nudge event")
	}
	if !terminal.Successful() {
		t.Fatalf("terminal: want completed, got %#v", terminal)
	}

	transcript, err := attached.Transcript(ctx)
	if err != nil {
		t.Fatalf("Transcript: %v", err)
	}
	if transcript.Text == "" || len(transcript.Messages) == 0 {
		t.Fatalf("transcript not populated: %#v", transcript)
	}

	artifacts, err := attached.Artifacts(ctx)
	if err != nil {
		t.Fatalf("Artifacts: %v", err)
	}
	if len(artifacts) != 1 || artifacts[0].ValidationStatus != "valid" {
		t.Fatalf("artifacts: %#v", artifacts)
	}

	if err := attached.Cancel(ctx, CancelRequest{Reason: "test cleanup"}); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	cancelled, err := attached.TerminalState(ctx)
	if err != nil {
		t.Fatalf("TerminalState: %v", err)
	}
	if cancelled.Status != StatusCancelled {
		t.Fatalf("cancel status: want cancelled, got %s", cancelled.Status)
	}
}

func TestTerminalClassification(t *testing.T) {
	tests := []struct {
		name        string
		observation string
		want        SessionStatus
		terminal    bool
		success     bool
	}{
		{name: "completed", observation: "completed with usable artifacts", want: StatusCompleted, terminal: true, success: true},
		{name: "failed artifact validation", observation: "completed but artifact validation failed", want: StatusFailed, terminal: true},
		{name: "cancelled", observation: "cancelled by AgentOps", want: StatusCancelled, terminal: true},
		{name: "lost", observation: "session ID previously known but provider cannot find it", want: StatusLost, terminal: true},
		{name: "provider unreachable", observation: "provider readiness unavailable before terminal state", want: StatusProviderUnreachable, terminal: true},
		{name: "stream disconnected", observation: "stream disconnected but REST reconciliation pending", want: StatusRunning, terminal: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyTerminalState(tt.observation)
			if got.Status != tt.want {
				t.Fatalf("status: want %s, got %#v", tt.want, got)
			}
			if got.Terminal() != tt.terminal {
				t.Fatalf("terminal: want %v, got %v", tt.terminal, got.Terminal())
			}
			if got.Successful() != tt.success {
				t.Fatalf("success: want %v, got %v", tt.success, got.Successful())
			}
		})
	}
}

func TestStartRequestValidate(t *testing.T) {
	if err := (StartRequest{}).Validate(); err == nil {
		t.Fatal("empty request should fail validation")
	}
	err := (StartRequest{
		WorkerKind: WorkerKindClaude,
		Provider:   ProviderGasCity,
		Prompt:     "build wiki page",
	}).Validate()
	if err != nil {
		t.Fatalf("valid request: %v", err)
	}
}

type fakeLifecycleWorker struct {
	sessions map[string]*fakeLifecycleSession
	nextID   int
}

func newFakeLifecycleWorker() *fakeLifecycleWorker {
	return &fakeLifecycleWorker{sessions: make(map[string]*fakeLifecycleSession)}
}

func (w *fakeLifecycleWorker) Start(_ context.Context, req StartRequest) (AgentSession, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	w.nextID++
	ref := SessionRef{
		WorkerKind: req.WorkerKind,
		Provider:   req.Provider,
		JobID:      req.JobID,
		AttemptID:  req.AttemptID,
		RequestID:  req.RequestID,
		SessionID:  fmt.Sprintf("fake-session-%d", w.nextID),
		Status:     StatusRunning,
	}
	session := &fakeLifecycleSession{
		ref: ref,
		events: []Event{
			{
				Cursor:  "1",
				At:      time.Unix(100, 0).UTC(),
				Type:    EventStarted,
				Message: req.Prompt,
				State:   TerminalState{Status: StatusRunning},
			},
		},
		transcript: Transcript{
			Text: "structured worker output",
			Messages: []TranscriptMessage{
				{Role: "user", Content: req.Prompt, At: time.Unix(100, 0).UTC()},
				{Role: "assistant", Content: "structured worker output", At: time.Unix(101, 0).UTC()},
			},
		},
		artifacts: []Artifact{
			{
				Kind:             "wiki-note",
				Path:             ".agents/wiki/sources/fake.md",
				JobID:            req.JobID,
				AttemptID:        req.AttemptID,
				SessionID:        ref.SessionID,
				ValidationStatus: "valid",
			},
		},
		terminal: TerminalState{Status: StatusCompleted},
	}
	session.events = append(session.events, Event{
		Cursor:  "2",
		At:      time.Unix(102, 0).UTC(),
		Type:    EventTerminal,
		State:   session.terminal,
		Message: "completed",
	})
	w.sessions[ref.SessionID] = session
	return session, nil
}

func (w *fakeLifecycleWorker) Attach(_ context.Context, ref SessionRef) (AgentSession, error) {
	session, ok := w.sessions[ref.SessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", ref.SessionID)
	}
	return session, nil
}

type fakeLifecycleSession struct {
	ref        SessionRef
	events     []Event
	transcript Transcript
	artifacts  []Artifact
	terminal   TerminalState
}

func (s *fakeLifecycleSession) Ref() SessionRef {
	return s.ref
}

func (s *fakeLifecycleSession) Nudge(_ context.Context, req NudgeRequest) error {
	s.events = append(s.events, Event{
		Cursor:  fmt.Sprintf("%d", len(s.events)+1),
		At:      time.Unix(103, 0).UTC(),
		Type:    EventNudged,
		Message: req.Message,
		State:   TerminalState{Status: StatusRunning},
	})
	return nil
}

func (s *fakeLifecycleSession) Cancel(_ context.Context, _ CancelRequest) error {
	s.terminal = TerminalState{Status: StatusCancelled, Reason: "test cleanup"}
	s.ref.Status = StatusCancelled
	return nil
}

func (s *fakeLifecycleSession) Stream(_ context.Context, _ StreamOptions) (<-chan Event, error) {
	ch := make(chan Event, len(s.events))
	for _, event := range s.events {
		ch <- event
	}
	close(ch)
	return ch, nil
}

func (s *fakeLifecycleSession) Transcript(_ context.Context) (Transcript, error) {
	return s.transcript, nil
}

func (s *fakeLifecycleSession) Artifacts(_ context.Context) ([]Artifact, error) {
	return s.artifacts, nil
}

func (s *fakeLifecycleSession) TerminalState(_ context.Context) (TerminalState, error) {
	return s.terminal, nil
}
