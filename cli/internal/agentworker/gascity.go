package agentworker

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/gascity"
)

const defaultGasCityTranscriptFormat = "conversation"

// GasCityClient is the narrow GasCity surface AgentWorker needs.
type GasCityClient interface {
	CityReadiness(context.Context, string) (gascity.ReadinessResponse, error)
	CreateSession(context.Context, string, gascity.SessionCreateRequest) (gascity.Session, gascity.ResponseMeta, error)
	GetSession(context.Context, string, string, gascity.SessionGetOptions) (gascity.Session, gascity.ResponseMeta, error)
	SubmitSession(context.Context, string, string, gascity.SessionSubmitRequest) (gascity.SessionSubmitResponse, gascity.ResponseMeta, error)
	StreamCityEvents(context.Context, string, gascity.EventStreamOptions) (GasCityEventStream, gascity.ResponseMeta, error)
	SessionTranscript(context.Context, string, string, gascity.TranscriptOptions) (gascity.TranscriptResponse, gascity.ResponseMeta, error)
}

// GasCityEventStream is the event-stream subset AgentWorker consumes.
type GasCityEventStream interface {
	NextEvent() (gascity.EventStreamFrame, error)
	Close() error
}

// GasCityClientAdapter adapts the public gascity.Client to GasCityClient.
type GasCityClientAdapter struct {
	Client *gascity.Client
}

func (a GasCityClientAdapter) CityReadiness(ctx context.Context, cityName string) (gascity.ReadinessResponse, error) {
	return a.Client.CityReadiness(ctx, cityName)
}

func (a GasCityClientAdapter) CreateSession(ctx context.Context, cityName string, req gascity.SessionCreateRequest) (gascity.Session, gascity.ResponseMeta, error) {
	return a.Client.CreateSession(ctx, cityName, req)
}

func (a GasCityClientAdapter) GetSession(ctx context.Context, cityName string, id string, opts gascity.SessionGetOptions) (gascity.Session, gascity.ResponseMeta, error) {
	return a.Client.GetSession(ctx, cityName, id, opts)
}

func (a GasCityClientAdapter) SubmitSession(ctx context.Context, cityName string, id string, req gascity.SessionSubmitRequest) (gascity.SessionSubmitResponse, gascity.ResponseMeta, error) {
	return a.Client.SubmitSession(ctx, cityName, id, req)
}

func (a GasCityClientAdapter) StreamCityEvents(ctx context.Context, cityName string, opts gascity.EventStreamOptions) (GasCityEventStream, gascity.ResponseMeta, error) {
	stream, meta, err := a.Client.StreamCityEvents(ctx, cityName, opts)
	if err != nil {
		return nil, meta, err
	}
	return gasCityEventStreamAdapter{stream: stream}, meta, nil
}

func (a GasCityClientAdapter) SessionTranscript(ctx context.Context, cityName string, id string, opts gascity.TranscriptOptions) (gascity.TranscriptResponse, gascity.ResponseMeta, error) {
	return a.Client.SessionTranscript(ctx, cityName, id, opts)
}

type gasCityEventStreamAdapter struct {
	stream *gascity.EventStream
}

func (s gasCityEventStreamAdapter) NextEvent() (gascity.EventStreamFrame, error) {
	return s.stream.Recv()
}

func (s gasCityEventStreamAdapter) Close() error {
	return s.stream.Close()
}

// GasCityWorkerOptions configures a GasCity-backed AgentWorker.
type GasCityWorkerOptions struct {
	Client           GasCityClient
	CityName         string
	TranscriptFormat string
	AliasFunc        func(StartRequest) string
}

// GasCityWorker drives Codex/Claude AgentWorker sessions through GasCity.
type GasCityWorker struct {
	client           GasCityClient
	cityName         string
	transcriptFormat string
	aliasFunc        func(StartRequest) string
}

// NewGasCityWorker constructs a GasCity-backed AgentWorker.
func NewGasCityWorker(opts GasCityWorkerOptions) (*GasCityWorker, error) {
	if opts.Client == nil {
		return nil, fmt.Errorf("gascity client is required")
	}
	cityName := strings.TrimSpace(opts.CityName)
	if cityName == "" {
		return nil, fmt.Errorf("gascity city name is required")
	}
	format := strings.TrimSpace(opts.TranscriptFormat)
	if format == "" {
		format = defaultGasCityTranscriptFormat
	}
	aliasFunc := opts.AliasFunc
	if aliasFunc == nil {
		aliasFunc = defaultGasCityAlias
	}
	return &GasCityWorker{
		client:           opts.Client,
		cityName:         cityName,
		transcriptFormat: format,
		aliasFunc:        aliasFunc,
	}, nil
}

// Start creates a GasCity session, submits the initial prompt, and returns a
// durable AgentSession handle.
func (w *GasCityWorker) Start(ctx context.Context, req StartRequest) (AgentSession, error) {
	if req.Provider == "" {
		req.Provider = ProviderGasCity
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if err := validateGasCityWorkerKind(req.WorkerKind); err != nil {
		return nil, err
	}
	ready, err := w.client.CityReadiness(ctx, w.cityName)
	if err != nil {
		return nil, fmt.Errorf("gascity city readiness for %q: %w", w.cityName, err)
	}
	if !ready.Ready {
		status := strings.TrimSpace(ready.Status)
		if status == "" {
			status = "not ready"
		}
		return nil, fmt.Errorf("gascity city %q not ready: %s", w.cityName, status)
	}

	alias := w.aliasFunc(req)
	session, createMeta, err := w.client.CreateSession(ctx, w.cityName, gascity.SessionCreateRequest{
		Kind:    "agent",
		Name:    string(req.WorkerKind),
		Alias:   alias,
		Async:   true,
		Options: gasCitySessionOptions(req),
		Title:   firstNonEmpty(req.Metadata["title"], req.JobID, string(req.WorkerKind)+" worker"),
	})
	if err != nil {
		return nil, fmt.Errorf("gascity create %s session: %w", req.WorkerKind, err)
	}
	sessionID := strings.TrimSpace(session.ID)
	if sessionID == "" {
		return nil, fmt.Errorf("gascity create %s session returned empty session id", req.WorkerKind)
	}
	_, submitMeta, err := w.client.SubmitSession(ctx, w.cityName, sessionID, gascity.SessionSubmitRequest{
		Message: req.Prompt,
		Intent:  "follow_up",
	})
	if err != nil {
		return nil, fmt.Errorf("gascity submit %s session %q: %w", req.WorkerKind, sessionID, err)
	}

	ref := gasCitySessionRef(req, session, createMeta.RequestID, StatusRunning)
	ref.SessionID = sessionID
	ref.ProviderRequestID = firstNonEmpty(submitMeta.RequestID, createMeta.RequestID)
	if ref.EventCursor == "" {
		ref.EventCursor = gascity.CursorFromFrame(gascity.EventStreamFrame{})
	}
	return &GasCitySession{
		client:           w.client,
		cityName:         w.cityName,
		transcriptFormat: w.transcriptFormat,
		ref:              ref,
		alias:            alias,
	}, nil
}

// Attach reconnects to an existing GasCity session by durable AgentWorker ref.
func (w *GasCityWorker) Attach(ctx context.Context, ref SessionRef) (AgentSession, error) {
	if err := ref.Validate(); err != nil {
		return nil, err
	}
	session, meta, err := w.client.GetSession(ctx, w.cityName, ref.SessionID, gascity.SessionGetOptions{Peek: true})
	if err != nil {
		return nil, err
	}
	if meta.RequestID != "" {
		ref.ProviderRequestID = meta.RequestID
	}
	ref.Status = mapGasCityTerminalState(gascity.ClassifyTerminalState(gascity.TerminalStateInput{
		SessionState:  session.State,
		SessionStatus: session.Status,
	})).Status
	return &GasCitySession{
		client:           w.client,
		cityName:         w.cityName,
		transcriptFormat: w.transcriptFormat,
		ref:              ref,
		alias:            session.Alias,
	}, nil
}

// GasCitySession is a durable AgentSession backed by one GasCity session.
type GasCitySession struct {
	client           GasCityClient
	cityName         string
	transcriptFormat string
	ref              SessionRef
	alias            string
}

func (s *GasCitySession) Ref() SessionRef {
	return s.ref
}

func (s *GasCitySession) Nudge(ctx context.Context, req NudgeRequest) error {
	message := strings.TrimSpace(req.Message)
	if message == "" {
		return fmt.Errorf("nudge message is required")
	}
	_, meta, err := s.client.SubmitSession(ctx, s.cityName, s.ref.SessionID, gascity.SessionSubmitRequest{
		Message: message,
		Intent:  "follow_up",
	})
	if meta.RequestID != "" {
		s.ref.ProviderRequestID = meta.RequestID
	}
	return err
}

func (s *GasCitySession) Cancel(ctx context.Context, req CancelRequest) error {
	message := strings.TrimSpace(req.Reason)
	if message == "" {
		message = "cancel requested by AgentOps"
	}
	_, meta, err := s.client.SubmitSession(ctx, s.cityName, s.ref.SessionID, gascity.SessionSubmitRequest{
		Message: message,
		Intent:  "interrupt_now",
	})
	if meta.RequestID != "" {
		s.ref.ProviderRequestID = meta.RequestID
	}
	if err == nil {
		s.ref.Status = StatusCancelled
	}
	return err
}

func (s *GasCitySession) Stream(ctx context.Context, opts StreamOptions) (<-chan Event, error) {
	stream, meta, err := s.client.StreamCityEvents(ctx, s.cityName, gascity.EventStreamOptions{AfterCursor: opts.AfterCursor})
	if err != nil {
		return nil, err
	}
	if meta.RequestID != "" {
		s.ref.ProviderRequestID = meta.RequestID
	}
	ch := make(chan Event)
	go func() {
		defer close(ch)
		defer stream.Close()
		for {
			frame, err := stream.NextEvent()
			if err != nil {
				return
			}
			event := frame.CityEvent
			if !gasCityEventMatchesSession(event, s.ref.SessionID, s.alias) {
				continue
			}
			workerEvent := gasCityFrameToAgentEvent(frame)
			if workerEvent.Cursor != "" {
				s.ref.EventCursor = workerEvent.Cursor
			}
			if workerEvent.State.Status != "" {
				s.ref.Status = workerEvent.State.Status
			}
			select {
			case <-ctx.Done():
				return
			case ch <- workerEvent:
			}
			if workerEvent.State.Terminal() {
				return
			}
		}
	}()
	return ch, nil
}

func (s *GasCitySession) Transcript(ctx context.Context) (Transcript, error) {
	transcript, meta, err := s.client.SessionTranscript(ctx, s.cityName, s.ref.SessionID, gascity.TranscriptOptions{
		Format: s.transcriptFormat,
	})
	if meta.RequestID != "" {
		s.ref.ProviderRequestID = meta.RequestID
	}
	if err != nil {
		return Transcript{}, err
	}
	return convertGasCityTranscript(transcript), nil
}

func (s *GasCitySession) Artifacts(ctx context.Context) ([]Artifact, error) {
	transcript, meta, err := s.client.SessionTranscript(ctx, s.cityName, s.ref.SessionID, gascity.TranscriptOptions{
		Format: s.transcriptFormat,
	})
	if meta.RequestID != "" {
		s.ref.ProviderRequestID = meta.RequestID
	}
	if err != nil {
		return nil, err
	}
	return convertGasCityArtifacts(s.ref, transcript.Artifacts), nil
}

func (s *GasCitySession) TerminalState(ctx context.Context) (TerminalState, error) {
	session, meta, err := s.client.GetSession(ctx, s.cityName, s.ref.SessionID, gascity.SessionGetOptions{Peek: true})
	if meta.RequestID != "" {
		s.ref.ProviderRequestID = meta.RequestID
	}
	if err != nil {
		return terminalStateForGasCityError(err), err
	}
	state := mapGasCityTerminalState(gascity.ClassifyTerminalState(gascity.TerminalStateInput{
		SessionState:  session.State,
		SessionStatus: session.Status,
	}))
	s.ref.Status = state.Status
	return state, nil
}

func validateGasCityWorkerKind(kind WorkerKind) error {
	switch kind {
	case WorkerKindCodex, WorkerKindClaude:
		return nil
	default:
		return fmt.Errorf("unsupported gascity worker kind %q", kind)
	}
}

func gasCitySessionOptions(req StartRequest) map[string]string {
	options := map[string]string{
		"agentops.worker_kind": string(req.WorkerKind),
	}
	if req.Model != "" {
		options["agentops.model"] = req.Model
	}
	if req.CWD != "" {
		options["agentops.cwd"] = req.CWD
	}
	if req.JobID != "" {
		options["agentops.job_id"] = req.JobID
	}
	if req.AttemptID != "" {
		options["agentops.attempt_id"] = req.AttemptID
	}
	if req.RequestID != "" {
		options["agentops.request_id"] = req.RequestID
	}
	for key, value := range req.Metadata {
		if strings.TrimSpace(key) == "" {
			continue
		}
		options[key] = value
	}
	return options
}

func gasCitySessionRef(req StartRequest, session gascity.Session, requestID string, status SessionStatus) SessionRef {
	return SessionRef{
		WorkerKind:        req.WorkerKind,
		Provider:          ProviderGasCity,
		JobID:             req.JobID,
		AttemptID:         req.AttemptID,
		RequestID:         req.RequestID,
		ProviderRequestID: requestID,
		SessionID:         session.ID,
		Status:            status,
	}
}

func gasCityFrameToAgentEvent(frame gascity.EventStreamFrame) Event {
	event := frame.CityEvent
	state := mapGasCityTerminalState(gascity.ClassifyTerminalState(gascity.TerminalStateInput{
		EventType:    event.Type,
		EventPayload: event.Payload,
	}))
	eventType := EventOutput
	if state.Terminal() {
		eventType = EventTerminal
	} else if strings.Contains(strings.ToLower(event.Type), "start") {
		eventType = EventStarted
	}
	return Event{
		Cursor:  gascity.CursorFromFrame(frame),
		At:      parseGasCityEventTime(event.TS),
		Type:    eventType,
		Message: event.Message,
		State:   state,
		Metadata: map[string]string{
			"gascity_event_type": event.Type,
		},
	}
}

func mapGasCityTerminalState(classification gascity.TerminalClassification) TerminalState {
	status := StatusRunning
	switch classification.Status {
	case gascity.TerminalStatusCompleted:
		status = StatusCompleted
	case gascity.TerminalStatusFailed:
		status = StatusFailed
	case gascity.TerminalStatusCancelled:
		status = StatusCancelled
	case gascity.TerminalStatusLost:
		status = StatusLost
	case gascity.TerminalStatusProviderUnreachable:
		status = StatusProviderUnreachable
	case gascity.TerminalStatusTerminalWithoutTranscript:
		status = StatusFailed
	case gascity.TerminalStatusEventStreamUnavailable:
		status = StatusRunning
	case gascity.TerminalStatusUnknown:
		status = StatusUnknown
	case gascity.TerminalStatusRunning:
		status = StatusRunning
	default:
		status = ClassifyTerminalState(classification.Status).Status
	}
	return TerminalState{
		Status:      status,
		FailureCode: failureCodeForAgentWorkerStatus(status),
		Reason:      classification.Reason,
	}
}

func terminalStateForGasCityError(err error) TerminalState {
	var apiErr *gascity.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
		return TerminalState{Status: StatusLost, FailureCode: string(StatusLost), Reason: "session missing after acceptance"}
	}
	if gascity.IsAPIUnavailable(err) {
		return TerminalState{Status: StatusProviderUnreachable, FailureCode: string(StatusProviderUnreachable), Reason: err.Error()}
	}
	return TerminalState{Status: StatusFailed, FailureCode: string(StatusFailed), Reason: err.Error()}
}

func failureCodeForAgentWorkerStatus(status SessionStatus) string {
	switch status {
	case StatusFailed, StatusCancelled, StatusLost, StatusProviderUnreachable:
		return string(status)
	default:
		return ""
	}
}

func gasCityEventMatchesSession(event *gascity.EventStreamEnvelope, sessionID, alias string) bool {
	if event == nil {
		return false
	}
	subject := strings.TrimSpace(event.Subject)
	if subject == "" {
		return false
	}
	return subject == sessionID || (alias != "" && subject == alias)
}

func convertGasCityTranscript(transcript gascity.TranscriptResponse) Transcript {
	messages := make([]TranscriptMessage, 0, len(transcript.Turns))
	var b strings.Builder
	for _, turn := range transcript.Turns {
		text := firstNonEmpty(turn.Text, turn.Content)
		if strings.TrimSpace(text) == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		if turn.Role != "" {
			b.WriteString(strings.ToUpper(turn.Role))
			b.WriteString(":\n")
		}
		b.WriteString(text)
		messages = append(messages, TranscriptMessage{
			Role:    turn.Role,
			Content: text,
			At:      parseGasCityEventTime(turn.Timestamp),
		})
	}
	return Transcript{
		Text:     b.String(),
		Messages: messages,
	}
}

func convertGasCityArtifacts(ref SessionRef, artifacts []gascity.TranscriptArtifact) []Artifact {
	out := make([]Artifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		out = append(out, Artifact{
			Kind:             artifact.Kind,
			Path:             artifact.Path,
			JobID:            ref.JobID,
			AttemptID:        ref.AttemptID,
			SessionID:        ref.SessionID,
			ValidationStatus: "pending",
		})
	}
	return out
}

func parseGasCityEventTime(raw string) time.Time {
	if raw == "" {
		return time.Now().UTC()
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Now().UTC()
	}
	return t.UTC()
}

func defaultGasCityAlias(req StartRequest) string {
	if alias := strings.TrimSpace(req.Metadata["session_alias"]); alias != "" {
		return alias
	}
	seed := firstNonEmpty(req.JobID, req.RequestID, string(req.WorkerKind))
	return "agentworker-" + sanitizeAlias(seed)
}

func sanitizeAlias(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
