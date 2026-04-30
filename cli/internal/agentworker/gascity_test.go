package agentworker

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/boshu2/agentops/cli/internal/gascity"
)

func TestGasCityAgentWorkerStartsCodexSessionAndStreamsTerminal(t *testing.T) {
	fake := &fakeGasCityWorkerClient{
		ready:      gascity.ReadinessResponse{Ready: true, Status: "ready"},
		session:    gascity.Session{ID: "sess_codex", Alias: "alias-codex", State: "running"},
		createMeta: gascity.ResponseMeta{RequestID: "req-create"},
		submitMeta: gascity.ResponseMeta{RequestID: "req-submit"},
		stream: &fakeGasCityWorkerStream{frames: []gascity.EventStreamFrame{
			{
				ID: "1",
				CityEvent: &gascity.EventStreamEnvelope{
					Seq:     1,
					Type:    "session.output",
					Subject: "sess_codex",
					Message: "working",
					TS:      "2026-04-28T10:00:00Z",
				},
			},
			{
				ID: "2",
				CityEvent: &gascity.EventStreamEnvelope{
					Seq:     2,
					Type:    "session.completed",
					Subject: "sess_codex",
					Payload: map[string]any{"status": "completed"},
					TS:      "2026-04-28T10:00:01Z",
				},
			},
		}},
		transcript: gascity.TranscriptResponse{
			Turns: []gascity.TranscriptEntry{
				{Role: "assistant", Text: "structured output", Timestamp: "2026-04-28T10:00:02Z"},
			},
			Artifacts: []gascity.TranscriptArtifact{{Kind: "wiki-note", Path: ".agents/wiki/sources/codex.md"}},
		},
	}
	worker, err := NewGasCityWorker(GasCityWorkerOptions{Client: fake, CityName: "agentops"})
	if err != nil {
		t.Fatalf("NewGasCityWorker: %v", err)
	}

	session, err := worker.Start(context.Background(), StartRequest{
		WorkerKind: WorkerKind("codex"),
		JobID:      "wiki.forge:1",
		AttemptID:  "attempt-1",
		RequestID:  "req-1",
		Model:      "codex-headless",
		CWD:        "/repo",
		Prompt:     "extract wiki lessons",
		Metadata:   map[string]string{"title": "wiki forge"},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(fake.createCalls) != 1 || fake.createCalls[0].req.Name != "codex" {
		t.Fatalf("create calls: %#v", fake.createCalls)
	}
	createReq := fake.createCalls[0].req
	if len(createReq.Options) != 0 {
		t.Fatalf("create options should not carry AgentOps metadata: %#v", createReq.Options)
	}
	if createReq.Title != "wiki forge" {
		t.Fatalf("create title = %q, want wiki forge", createReq.Title)
	}
	if createReq.Message != "extract wiki lessons" {
		t.Fatalf("create message = %q, want prompt", createReq.Message)
	}
	if createReq.Alias != "agentworker-wiki-forge-1" {
		t.Fatalf("create alias = %q, want agentworker-wiki-forge-1", createReq.Alias)
	}
	if len(fake.submitCalls) != 0 {
		t.Fatalf("start should use create-time message, got submit calls: %#v", fake.submitCalls)
	}
	if session.Ref().Provider != ProviderGasCity || session.Ref().SessionID != "sess_codex" {
		t.Fatalf("session ref: %#v", session.Ref())
	}

	events, err := session.Stream(context.Background(), StreamOptions{})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	var terminal TerminalState
	for event := range events {
		if event.Type == EventTerminal {
			terminal = event.State
		}
	}
	if !terminal.Successful() {
		t.Fatalf("terminal: %#v", terminal)
	}

	transcript, err := session.Transcript(context.Background())
	if err != nil {
		t.Fatalf("Transcript: %v", err)
	}
	if transcript.Text != "ASSISTANT:\nstructured output" {
		t.Fatalf("transcript: %q", transcript.Text)
	}
	artifacts, err := session.Artifacts(context.Background())
	if err != nil {
		t.Fatalf("Artifacts: %v", err)
	}
	if len(artifacts) != 1 || artifacts[0].SessionID != "sess_codex" {
		t.Fatalf("artifacts: %#v", artifacts)
	}
}

func TestGasCityAgentWorkerUsesConfiguredTemplateName(t *testing.T) {
	fake := &fakeGasCityWorkerClient{
		ready:   gascity.ReadinessResponse{Ready: true, Status: "ready"},
		session: gascity.Session{ID: "sess_worker", Alias: "alias-worker", State: "running"},
	}
	worker, err := NewGasCityWorker(GasCityWorkerOptions{
		Client:       fake,
		CityName:     "agentops",
		TemplateName: "agentops-worker",
	})
	if err != nil {
		t.Fatalf("NewGasCityWorker: %v", err)
	}
	if _, err := worker.Start(context.Background(), StartRequest{
		WorkerKind: WorkerKind("codex"),
		JobID:      "wiki.forge:1",
		AttemptID:  "attempt-1",
		RequestID:  "req-1",
		Prompt:     "extract wiki lessons",
	}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(fake.createCalls) != 1 || fake.createCalls[0].req.Name != "agentops-worker" {
		t.Fatalf("create calls: %#v", fake.createCalls)
	}
}

func TestGasCityAgentWorkerClaudeTerminalLost(t *testing.T) {
	fake := &fakeGasCityWorkerClient{
		ready:   gascity.ReadinessResponse{Ready: true, Status: "ready"},
		getErr:  fakeGasCityWorkerNotFound("sess_claude"),
		session: gascity.Session{ID: "sess_claude"},
	}
	worker, err := NewGasCityWorker(GasCityWorkerOptions{Client: fake, CityName: "agentops"})
	if err != nil {
		t.Fatalf("NewGasCityWorker: %v", err)
	}

	session, err := worker.Attach(context.Background(), SessionRef{
		WorkerKind: WorkerKind("claude"),
		Provider:   ProviderGasCity,
		SessionID:  "sess_claude",
		Status:     StatusRunning,
	})
	if err == nil {
		t.Fatalf("Attach should surface missing session, got %#v", session)
	}

	state := terminalStateForGasCityError(err)
	if state.Status != StatusLost || !state.Terminal() {
		t.Fatalf("lost terminal state: %#v", state)
	}
}

func TestGasCityAgentWorkerRejectsUnreadyCity(t *testing.T) {
	fake := &fakeGasCityWorkerClient{
		ready: gascity.ReadinessResponse{Ready: false, Status: "starting"},
	}
	worker, err := NewGasCityWorker(GasCityWorkerOptions{Client: fake, CityName: "agentops"})
	if err != nil {
		t.Fatalf("NewGasCityWorker: %v", err)
	}

	_, err = worker.Start(context.Background(), StartRequest{
		WorkerKind: WorkerKind("codex"),
		Prompt:     "extract",
	})
	if err == nil {
		t.Fatal("Start should reject unready city")
	}
}

type fakeGasCityWorkerClient struct {
	ready       gascity.ReadinessResponse
	readyErr    error
	session     gascity.Session
	createMeta  gascity.ResponseMeta
	submitMeta  gascity.ResponseMeta
	streamMeta  gascity.ResponseMeta
	transMeta   gascity.ResponseMeta
	getErr      error
	stream      *fakeGasCityWorkerStream
	transcript  gascity.TranscriptResponse
	createCalls []fakeGasCityWorkerCreateCall
	submitCalls []fakeGasCityWorkerSubmitCall
}

type fakeGasCityWorkerCreateCall struct {
	cityName string
	req      gascity.SessionCreateRequest
}

type fakeGasCityWorkerSubmitCall struct {
	cityName  string
	sessionID string
	req       gascity.SessionSubmitRequest
}

func (f *fakeGasCityWorkerClient) CityReadiness(context.Context, string) (gascity.ReadinessResponse, error) {
	if f.readyErr != nil {
		return gascity.ReadinessResponse{}, f.readyErr
	}
	if f.ready.Status == "" && !f.ready.Ready {
		return gascity.ReadinessResponse{Ready: true, Status: "ready"}, nil
	}
	return f.ready, nil
}

func (f *fakeGasCityWorkerClient) CreateSession(_ context.Context, cityName string, req gascity.SessionCreateRequest) (gascity.Session, gascity.ResponseMeta, error) {
	f.createCalls = append(f.createCalls, fakeGasCityWorkerCreateCall{cityName: cityName, req: req})
	session := f.session
	if session.ID == "" {
		session.ID = "sess_worker"
	}
	meta := f.createMeta
	if meta.RequestID == "" {
		meta.RequestID = "req-create"
	}
	return session, meta, nil
}

func (f *fakeGasCityWorkerClient) GetSession(_ context.Context, _ string, sessionID string, _ gascity.SessionGetOptions) (gascity.Session, gascity.ResponseMeta, error) {
	if f.getErr != nil {
		return gascity.Session{}, gascity.ResponseMeta{RequestID: "req-get"}, f.getErr
	}
	session := f.session
	if session.ID == "" {
		session.ID = sessionID
	}
	return session, gascity.ResponseMeta{RequestID: "req-get"}, nil
}

func (f *fakeGasCityWorkerClient) SubmitSession(_ context.Context, cityName string, sessionID string, req gascity.SessionSubmitRequest) (gascity.SessionSubmitResponse, gascity.ResponseMeta, error) {
	f.submitCalls = append(f.submitCalls, fakeGasCityWorkerSubmitCall{cityName: cityName, sessionID: sessionID, req: req})
	meta := f.submitMeta
	if meta.RequestID == "" {
		meta.RequestID = "req-submit"
	}
	return gascity.SessionSubmitResponse{Status: "queued", ID: sessionID, Queued: true, Intent: req.Intent}, meta, nil
}

func (f *fakeGasCityWorkerClient) CloseSession(context.Context, string, string) (gascity.ResponseMeta, error) {
	return gascity.ResponseMeta{RequestID: "req-close"}, nil
}

func (f *fakeGasCityWorkerClient) StreamCityEvents(context.Context, string, gascity.EventStreamOptions) (GasCityEventStream, gascity.ResponseMeta, error) {
	stream := f.stream
	if stream == nil {
		stream = &fakeGasCityWorkerStream{}
	}
	meta := f.streamMeta
	if meta.RequestID == "" {
		meta.RequestID = "req-stream"
	}
	return stream, meta, nil
}

func (f *fakeGasCityWorkerClient) SessionTranscript(context.Context, string, string, gascity.TranscriptOptions) (gascity.TranscriptResponse, gascity.ResponseMeta, error) {
	meta := f.transMeta
	if meta.RequestID == "" {
		meta.RequestID = "req-transcript"
	}
	return f.transcript, meta, nil
}

type fakeGasCityWorkerStream struct {
	frames []gascity.EventStreamFrame
	index  int
	closed bool
}

func (s *fakeGasCityWorkerStream) NextEvent() (gascity.EventStreamFrame, error) {
	if s.index >= len(s.frames) {
		return gascity.EventStreamFrame{}, io.EOF
	}
	frame := s.frames[s.index]
	s.index++
	return frame, nil
}

func (s *fakeGasCityWorkerStream) Close() error {
	s.closed = true
	return nil
}

func fakeGasCityWorkerNotFound(sessionID string) error {
	return &gascity.APIError{
		Method:     http.MethodGet,
		Path:       "/v0/city/agentops/session/" + sessionID,
		StatusCode: http.StatusNotFound,
		RequestID:  "req-get",
	}
}
