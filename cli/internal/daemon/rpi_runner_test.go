package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/gascity"
	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
)

type fakeGasCityRPIClient struct {
	ready           gascity.ReadinessResponse
	readyErr        error
	session         gascity.Session
	createMeta      gascity.ResponseMeta
	submitMeta      gascity.ResponseMeta
	streamMeta      gascity.ResponseMeta
	transcriptMeta  gascity.ResponseMeta
	transcript      gascity.TranscriptResponse
	stream          *fakeGasCityRPIStream
	streamFactory   func(context.Context) GasCityRPIEventStream
	sessions        map[string]gascity.Session
	getErrs         map[string]error
	createCalls     []fakeGasCityRPICreateCall
	submitCalls     []fakeGasCityRPISubmitCall
	streamCalls     []fakeGasCityRPIStreamCall
	getCalls        []fakeGasCityRPIGetCall
	transcriptCalls []fakeGasCityRPITranscriptCall
}

type fakeGasCityRPICreateCall struct {
	cityName string
	req      gascity.SessionCreateRequest
}

type fakeGasCityRPISubmitCall struct {
	cityName  string
	sessionID string
	req       gascity.SessionSubmitRequest
}

type fakeGasCityRPIStreamCall struct {
	cityName string
	opts     gascity.EventStreamOptions
}

type fakeGasCityRPIGetCall struct {
	cityName  string
	sessionID string
	opts      gascity.SessionGetOptions
}

type fakeGasCityRPITranscriptCall struct {
	cityName  string
	sessionID string
	opts      gascity.TranscriptOptions
}

func (f *fakeGasCityRPIClient) CityReadiness(context.Context, string) (gascity.ReadinessResponse, error) {
	if f.readyErr != nil {
		return gascity.ReadinessResponse{}, f.readyErr
	}
	if f.ready.Status == "" && !f.ready.Ready {
		return gascity.ReadinessResponse{Ready: true, Status: "ready"}, nil
	}
	return f.ready, nil
}

func (f *fakeGasCityRPIClient) CreateSession(_ context.Context, cityName string, req gascity.SessionCreateRequest) (gascity.Session, gascity.ResponseMeta, error) {
	f.createCalls = append(f.createCalls, fakeGasCityRPICreateCall{cityName: cityName, req: req})
	session := f.session
	if session.ID == "" {
		session.ID = "sess_rpi_123"
	}
	if session.Alias == "" {
		session.Alias = req.Alias
	}
	meta := f.createMeta
	if meta.RequestID == "" {
		meta.RequestID = "req-create"
	}
	return session, meta, nil
}

func (f *fakeGasCityRPIClient) SubmitSession(_ context.Context, cityName string, sessionID string, req gascity.SessionSubmitRequest) (gascity.SessionSubmitResponse, gascity.ResponseMeta, error) {
	f.submitCalls = append(f.submitCalls, fakeGasCityRPISubmitCall{cityName: cityName, sessionID: sessionID, req: req})
	meta := f.submitMeta
	if meta.RequestID == "" {
		meta.RequestID = "req-submit"
	}
	return gascity.SessionSubmitResponse{Status: "queued", ID: sessionID, Queued: true, Intent: req.Intent}, meta, nil
}

func (f *fakeGasCityRPIClient) GetSession(_ context.Context, cityName string, sessionID string, opts gascity.SessionGetOptions) (gascity.Session, gascity.ResponseMeta, error) {
	f.getCalls = append(f.getCalls, fakeGasCityRPIGetCall{cityName: cityName, sessionID: sessionID, opts: opts})
	if err := f.getErrs[sessionID]; err != nil {
		return gascity.Session{}, gascity.ResponseMeta{RequestID: "req-get-" + sessionID}, err
	}
	if session, ok := f.sessions[sessionID]; ok {
		return session, gascity.ResponseMeta{RequestID: "req-get-" + sessionID}, nil
	}
	return gascity.Session{ID: sessionID, State: "running", Running: true}, gascity.ResponseMeta{RequestID: "req-get-" + sessionID}, nil
}

func (f *fakeGasCityRPIClient) StreamCityEvents(ctx context.Context, cityName string, opts gascity.EventStreamOptions) (GasCityRPIEventStream, gascity.ResponseMeta, error) {
	f.streamCalls = append(f.streamCalls, fakeGasCityRPIStreamCall{cityName: cityName, opts: opts})
	var stream GasCityRPIEventStream
	if f.streamFactory != nil {
		stream = f.streamFactory(ctx)
	}
	if stream == nil && f.stream != nil {
		stream = f.stream
	}
	if stream == nil {
		stream = &fakeGasCityRPIStream{frames: []gascity.EventStreamFrame{{
			ID: "1",
			CityEvent: &gascity.EventStreamEnvelope{
				Seq:     1,
				Type:    "session.completed",
				Subject: "sess_rpi_123",
				Payload: map[string]any{"status": "completed"},
			},
		}}}
	}
	meta := f.streamMeta
	if meta.RequestID == "" {
		meta.RequestID = "req-stream"
	}
	return stream, meta, nil
}

func fakeGasCityNotFound(sessionID string) error {
	return &gascity.APIError{
		Method:     http.MethodGet,
		Path:       "/v0/city/agentops/session/" + sessionID,
		StatusCode: http.StatusNotFound,
		RequestID:  "req-get-" + sessionID,
		Problem:    &gascity.ProblemDetails{Status: http.StatusNotFound, Title: "not found"},
	}
}

func (f *fakeGasCityRPIClient) SessionTranscript(_ context.Context, cityName string, sessionID string, opts gascity.TranscriptOptions) (gascity.TranscriptResponse, gascity.ResponseMeta, error) {
	f.transcriptCalls = append(f.transcriptCalls, fakeGasCityRPITranscriptCall{cityName: cityName, sessionID: sessionID, opts: opts})
	transcript := f.transcript
	if transcript.ID == "" {
		transcript.ID = "tx_rpi_123"
	}
	if transcript.Format == "" {
		transcript.Format = opts.Format
	}
	if len(transcript.Turns) == 0 {
		transcript.Turns = []gascity.TranscriptEntry{{Role: "assistant", Text: "done"}}
	}
	meta := f.transcriptMeta
	if meta.RequestID == "" {
		meta.RequestID = "req-transcript"
	}
	return transcript, meta, nil
}

type fakeGasCityRPIStream struct {
	frames []gascity.EventStreamFrame
	index  int
	closed bool
	err    error
}

func (s *fakeGasCityRPIStream) NextEvent() (gascity.EventStreamFrame, error) {
	if s.err != nil {
		return gascity.EventStreamFrame{}, s.err
	}
	if s.index >= len(s.frames) {
		return gascity.EventStreamFrame{}, io.EOF
	}
	frame := s.frames[s.index]
	s.index++
	return frame, nil
}

func (s *fakeGasCityRPIStream) Close() error {
	s.closed = true
	return nil
}

type contextDoneGasCityRPIStream struct {
	ctx    context.Context
	closed bool
}

func (s *contextDoneGasCityRPIStream) NextEvent() (gascity.EventStreamFrame, error) {
	<-s.ctx.Done()
	return gascity.EventStreamFrame{}, s.ctx.Err()
}

func (s *contextDoneGasCityRPIStream) Close() error {
	s.closed = true
	return nil
}

func TestRPIRunner_RunNextRPIJobThroughGasCityExecutor(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	queue := NewQueue(store, QueueOptions{
		LeaseDuration: time.Minute,
		Actor:         "test-daemon",
	})
	spec := NewRPIRunJobSpec("run-123", "Build daemon RPI")
	spec.MaxPhase = 1
	spec.GasCityCityName = "agentops"
	jobSpec, err := spec.ToJobSpec("job-rpi-run")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := queue.SubmitJob(SubmitJobInput{
		RequestID:      "req-rpi-run",
		JobID:          jobSpec.ID,
		JobType:        jobSpec.Type,
		IdempotencyKey: "rpi-run-123",
		Payload:        jobSpec.Payload,
	}, QueueMutationOptions{}); err != nil {
		t.Fatalf("SubmitJob: %v", err)
	}

	fake := &fakeGasCityRPIClient{
		ready:          gascity.ReadinessResponse{Ready: true, Status: "ready"},
		session:        gascity.Session{ID: "sess_rpi_123"},
		createMeta:     gascity.ResponseMeta{RequestID: "req-create-123"},
		submitMeta:     gascity.ResponseMeta{RequestID: "req-submit-123"},
		streamMeta:     gascity.ResponseMeta{RequestID: "req-stream-123"},
		transcriptMeta: gascity.ResponseMeta{RequestID: "req-transcript-123"},
		transcript: gascity.TranscriptResponse{
			ID:       "tx_rpi_123",
			Format:   "conversation",
			Turns:    []gascity.TranscriptEntry{{Role: "assistant", Text: "phase done"}},
			Messages: []map[string]any{{"role": "assistant", "content": "phase done"}},
			Artifacts: []gascity.TranscriptArtifact{{
				Path: ".agents/rpi/phase-1-summary.md",
				Kind: "summary",
			}},
		},
	}
	runner, err := NewRPIRunner(store, RPIRunnerOptions{
		Queue:    queue,
		Executor: GasCityRPIPhaseExecutor{Client: fake, PhaseTimeout: time.Second},
		Actor:    "test-rpi-runner",
	})
	if err != nil {
		t.Fatalf("NewRPIRunner: %v", err)
	}

	result, err := runner.RunNextRPIJob(context.Background())
	if err != nil {
		t.Fatalf("RunNextRPIJob: %v", err)
	}
	if result.Status != JobStatusCompleted {
		t.Fatalf("status = %q, want completed", result.Status)
	}
	if len(fake.createCalls) != 1 || fake.createCalls[0].cityName != "agentops" {
		t.Fatalf("create calls = %#v", fake.createCalls)
	}
	if got, want := fake.createCalls[0].req.Alias, "rpi-run-123-p1"; got != want {
		t.Fatalf("session alias = %q, want %q", got, want)
	}
	if len(fake.submitCalls) != 1 || fake.submitCalls[0].req.Intent != "follow_up" {
		t.Fatalf("submit calls = %#v", fake.submitCalls)
	}
	if len(fake.streamCalls) != 1 || len(fake.transcriptCalls) != 1 {
		t.Fatalf("stream calls = %d transcript calls = %d", len(fake.streamCalls), len(fake.transcriptCalls))
	}
	if got := result.Artifacts["phase_1_gascity_evidence"]; got == "" {
		t.Fatalf("missing gascity evidence artifact: %#v", result.Artifacts)
	} else if _, err := os.Stat(got); err != nil {
		t.Fatalf("evidence artifact not written: %v", err)
	}

	snapshot, err := queue.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(snapshot.Jobs) != 1 || snapshot.Jobs[0].Status != JobStatusCompleted {
		t.Fatalf("snapshot jobs = %#v", snapshot.Jobs)
	}
	statePath := filepath.Join(root, ".agents", "rpi", "runs", "run-123", cliRPI.PhasedStateFile)
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read registry state: %v", err)
	}
	var state cliRPI.RunRegistryState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("decode registry state: %v", err)
	}
	if state.TerminalStatus != "completed" || state.Backend != string(RPIBackendGasCityAPI) {
		t.Fatalf("registry state = %#v", state)
	}
}

func TestGasCityRPIPhaseExecutorCompletesFromSessionTerminalEvidence(t *testing.T) {
	root := t.TempDir()
	fake := &fakeGasCityRPIClient{
		ready:   gascity.ReadinessResponse{Ready: true, Status: "ready"},
		session: gascity.Session{ID: "sess_rpi_123"},
		stream:  &fakeGasCityRPIStream{},
		sessions: map[string]gascity.Session{
			"sess_rpi_123": {ID: "sess_rpi_123", State: "closed", Status: "completed"},
		},
	}
	exec := GasCityRPIPhaseExecutor{Client: fake, CityName: "agentops"}
	result, err := exec.ExecuteRPIPhase(context.Background(), RPIPhaseExecutionRequest{
		Root:      root,
		RunID:     "run-rest-terminal",
		Goal:      "prove REST terminal evidence",
		Phase:     1,
		PhaseName: "discovery",
		Prompt:    "finish",
	})
	if err != nil {
		t.Fatalf("ExecuteRPIPhase: %v", err)
	}
	if result.Status != gascity.TerminalStatusCompleted {
		t.Fatalf("status = %q, want completed", result.Status)
	}
	if len(fake.getCalls) == 0 {
		t.Fatal("expected GetSession fallback when stream ended without terminal event")
	}
	if result.EvidencePath == "" {
		t.Fatalf("result = %#v, want evidence path", result)
	}
	if _, err := os.Stat(result.EvidencePath); err != nil {
		t.Fatalf("evidence path %s: %v", result.EvidencePath, err)
	}
}

func TestGasCityRPIPhaseExecutorInterruptsSessionOnPhaseTimeout(t *testing.T) {
	fake := &fakeGasCityRPIClient{
		ready:   gascity.ReadinessResponse{Ready: true, Status: "ready"},
		session: gascity.Session{ID: "sess_rpi_timeout"},
		streamFactory: func(ctx context.Context) GasCityRPIEventStream {
			return &contextDoneGasCityRPIStream{ctx: ctx}
		},
		sessions: map[string]gascity.Session{
			"sess_rpi_timeout": {ID: "sess_rpi_timeout", State: "running", Status: "running"},
		},
	}
	exec := GasCityRPIPhaseExecutor{Client: fake, CityName: "agentops", PhaseTimeout: time.Millisecond}
	_, err := exec.ExecuteRPIPhase(context.Background(), RPIPhaseExecutionRequest{
		Root:      t.TempDir(),
		RunID:     "run-timeout",
		Goal:      "bound daemon phase",
		Phase:     1,
		PhaseName: "discovery",
		Prompt:    "wait",
	})
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("ExecuteRPIPhase error = %v, want deadline exceeded", err)
	}
	if len(fake.submitCalls) < 2 {
		t.Fatalf("submit calls = %#v, want initial submit plus interrupt", fake.submitCalls)
	}
	last := fake.submitCalls[len(fake.submitCalls)-1]
	if last.sessionID != "sess_rpi_timeout" || last.req.Intent != "interrupt_now" {
		t.Fatalf("last submit = %#v, want interrupt_now for timeout session", last)
	}
}

func TestRPIRunner_FailsUnreadyGasCityAsProviderUnreachable(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	queue := NewQueue(store, QueueOptions{LeaseDuration: time.Minute})
	spec := NewRPIPhaseJobSpec("run-unready", "Handle provider outage", 2)
	spec.GasCityCityName = "agentops"
	jobSpec, err := spec.ToJobSpec("job-phase-unready")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := queue.SubmitJob(SubmitJobInput{
		RequestID: "req-unready",
		JobID:     jobSpec.ID,
		JobType:   jobSpec.Type,
		Payload:   jobSpec.Payload,
	}, QueueMutationOptions{}); err != nil {
		t.Fatalf("SubmitJob: %v", err)
	}
	fake := &fakeGasCityRPIClient{ready: gascity.ReadinessResponse{Ready: false, Status: "booting"}}
	runner, err := NewRPIRunner(store, RPIRunnerOptions{
		Queue:    queue,
		Executor: GasCityRPIPhaseExecutor{Client: fake},
	})
	if err != nil {
		t.Fatalf("NewRPIRunner: %v", err)
	}

	result, err := runner.RunNextRPIJob(context.Background())
	if err == nil {
		t.Fatal("RunNextRPIJob error = nil, want provider failure")
	}
	if result.Status != JobStatusFailed || result.Failure == nil {
		t.Fatalf("result = %#v", result)
	}
	if result.Failure.Code != FailureProviderUnreachable || !result.Failure.Retryable {
		t.Fatalf("failure = %#v", result.Failure)
	}
	if len(fake.createCalls) != 0 {
		t.Fatalf("create should not be called when city is unready: %#v", fake.createCalls)
	}
}

func TestRPIRunner_SkipsNonRPIJobs(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	queue := NewQueue(store, QueueOptions{LeaseDuration: time.Minute})
	if _, err := queue.SubmitJob(SubmitJobInput{
		RequestID: "req-dream",
		JobID:     "job-dream",
		JobType:   JobTypeDreamRun,
		Payload: map[string]any{
			"schema_version": 1,
			"job_type":       string(JobTypeDreamRun),
		},
	}, QueueMutationOptions{}); err != nil {
		t.Fatalf("SubmitJob: %v", err)
	}
	runner, err := NewRPIRunner(store, RPIRunnerOptions{
		Queue:    queue,
		Executor: GasCityRPIPhaseExecutor{Client: &fakeGasCityRPIClient{}},
	})
	if err != nil {
		t.Fatalf("NewRPIRunner: %v", err)
	}
	if _, err := runner.RunNextRPIJob(context.Background()); !errors.Is(err, ErrNoRPIJobs) {
		t.Fatalf("RunNextRPIJob err = %v, want ErrNoRPIJobs", err)
	}
	snapshot, err := queue.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snapshot.Jobs[0].Status != JobStatusQueued {
		t.Fatalf("dream job status = %q, want queued", snapshot.Jobs[0].Status)
	}

	if _, err := runner.RunRPIJob(context.Background(), "job-dream"); !errors.Is(err, ErrNoRPIJobs) {
		t.Fatalf("RunRPIJob err = %v, want ErrNoRPIJobs", err)
	}
	snapshot, err = queue.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot after explicit run: %v", err)
	}
	if snapshot.Jobs[0].Status != JobStatusQueued {
		t.Fatalf("dream job status after explicit run = %q, want queued", snapshot.Jobs[0].Status)
	}
}
