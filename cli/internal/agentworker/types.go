// Package agentworker defines the shared runtime contract for headless agent
// providers such as GasCity-backed worker sessions.
package agentworker

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// WorkerKind identifies the agent runtime the session executes.
type WorkerKind string

// Provider identifies the transport/provider that owns the session.
type Provider string

const (
	ProviderGasCity Provider = "gascity"
)

// SessionStatus is the AgentOps status vocabulary for worker sessions.
type SessionStatus string

const (
	StatusStarting            SessionStatus = "starting"
	StatusRunning             SessionStatus = "running"
	StatusWaiting             SessionStatus = "waiting"
	StatusCompleted           SessionStatus = "completed"
	StatusFailed              SessionStatus = "failed"
	StatusCancelled           SessionStatus = "cancelled"
	StatusLost                SessionStatus = "lost"
	StatusProviderUnreachable SessionStatus = "provider_unreachable"
	StatusUnknown             SessionStatus = "unknown"
)

// Terminal returns true when the status is final and must not be reconciled as
// an in-flight stream state.
func (s SessionStatus) Terminal() bool {
	switch s {
	case StatusCompleted, StatusFailed, StatusCancelled, StatusLost, StatusProviderUnreachable:
		return true
	default:
		return false
	}
}

// Successful returns true only for completed sessions. Lost and provider
// unreachable are intentionally non-success terminal states.
func (s SessionStatus) Successful() bool {
	return s == StatusCompleted
}

// TerminalState is the classified end state of a provider session.
type TerminalState struct {
	Status      SessionStatus `json:"status"`
	FailureCode string        `json:"failure_code,omitempty"`
	Reason      string        `json:"reason,omitempty"`
}

// Terminal reports whether the contained status is final.
func (s TerminalState) Terminal() bool {
	return s.Status.Terminal()
}

// Successful reports whether the contained status is completed.
func (s TerminalState) Successful() bool {
	return s.Status.Successful()
}

// ClassifyTerminalState maps provider observations onto the AgentOps status
// vocabulary. The mapping is deliberately conservative: ambiguous provider or
// stream failures classify as non-success.
func ClassifyTerminalState(observation string) TerminalState {
	raw := strings.TrimSpace(strings.ToLower(observation))
	switch {
	case raw == "":
		return TerminalState{Status: StatusUnknown, Reason: "empty provider observation"}
	case raw == string(StatusCompleted), raw == "complete", raw == "succeeded", raw == "success", raw == "done", strings.Contains(raw, "completed with usable artifacts"):
		return TerminalState{Status: StatusCompleted}
	case raw == string(StatusCancelled), raw == "canceled", strings.Contains(raw, "cancel"):
		return TerminalState{Status: StatusCancelled}
	case raw == string(StatusLost), strings.Contains(raw, "not found"), strings.Contains(raw, "missing session"), strings.Contains(raw, "previously known"):
		return TerminalState{Status: StatusLost, FailureCode: string(StatusLost), Reason: observation}
	case raw == string(StatusProviderUnreachable), strings.Contains(raw, "provider unreachable"), strings.Contains(raw, "provider readiness unavailable"), strings.Contains(raw, "connection refused"):
		return TerminalState{Status: StatusProviderUnreachable, FailureCode: string(StatusProviderUnreachable), Reason: observation}
	case raw == string(StatusRunning), raw == "stream disconnected", strings.Contains(raw, "reconciliation pending"):
		return TerminalState{Status: StatusRunning, Reason: observation}
	case raw == string(StatusStarting):
		return TerminalState{Status: StatusStarting}
	case raw == string(StatusWaiting):
		return TerminalState{Status: StatusWaiting}
	case raw == string(StatusFailed), raw == "failure", raw == "error", strings.Contains(raw, "artifact validation failed"), strings.Contains(raw, "failed"):
		return TerminalState{Status: StatusFailed, FailureCode: string(StatusFailed), Reason: observation}
	default:
		return TerminalState{Status: StatusUnknown, Reason: observation}
	}
}

// SessionRef is the durable identity for a worker session.
type SessionRef struct {
	WorkerKind        WorkerKind    `json:"worker_kind"`
	Provider          Provider      `json:"provider"`
	JobID             string        `json:"job_id,omitempty"`
	AttemptID         string        `json:"attempt_id,omitempty"`
	RequestID         string        `json:"request_id,omitempty"`
	ProviderRequestID string        `json:"provider_request_id,omitempty"`
	SessionID         string        `json:"session_id"`
	EventCursor       string        `json:"event_cursor,omitempty"`
	Status            SessionStatus `json:"status"`
}

// Validate checks the minimal fields required for a durable worker reference.
func (r SessionRef) Validate() error {
	if r.WorkerKind == "" {
		return fmt.Errorf("worker_kind is required")
	}
	if r.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	if strings.TrimSpace(r.SessionID) == "" {
		return fmt.Errorf("session_id is required")
	}
	return nil
}

// StartRequest describes a new worker session request.
type StartRequest struct {
	WorkerKind WorkerKind        `json:"worker_kind"`
	Provider   Provider          `json:"provider"`
	JobID      string            `json:"job_id,omitempty"`
	AttemptID  string            `json:"attempt_id,omitempty"`
	RequestID  string            `json:"request_id,omitempty"`
	Model      string            `json:"model,omitempty"`
	CWD        string            `json:"cwd,omitempty"`
	Prompt     string            `json:"prompt"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// Validate checks the fields every AgentWorker implementation needs before it
// can start a session.
func (r StartRequest) Validate() error {
	if r.WorkerKind == "" {
		return fmt.Errorf("worker_kind is required")
	}
	if r.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	if strings.TrimSpace(r.Prompt) == "" {
		return fmt.Errorf("prompt is required")
	}
	return nil
}

// NudgeRequest is an additional prompt/control message sent to a live session.
type NudgeRequest struct {
	Message  string            `json:"message"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// CancelRequest asks a provider to cooperatively stop a session.
type CancelRequest struct {
	Reason string `json:"reason,omitempty"`
}

// StreamOptions controls event replay.
type StreamOptions struct {
	AfterCursor string `json:"after_cursor,omitempty"`
}

// EventType classifies worker stream frames.
type EventType string

const (
	EventStarted  EventType = "started"
	EventOutput   EventType = "output"
	EventNudged   EventType = "nudged"
	EventArtifact EventType = "artifact"
	EventTerminal EventType = "terminal"
)

// Event is a replayable worker stream frame.
type Event struct {
	Cursor   string            `json:"cursor"`
	At       time.Time         `json:"at"`
	Type     EventType         `json:"type"`
	Message  string            `json:"message,omitempty"`
	State    TerminalState     `json:"state,omitempty"`
	Artifact *Artifact         `json:"artifact,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Artifact describes an output produced by a worker session.
type Artifact struct {
	Kind             string            `json:"kind"`
	Path             string            `json:"path,omitempty"`
	URI              string            `json:"uri,omitempty"`
	MIME             string            `json:"mime,omitempty"`
	JobID            string            `json:"job_id,omitempty"`
	AttemptID        string            `json:"attempt_id,omitempty"`
	SessionID        string            `json:"session_id,omitempty"`
	ValidationStatus string            `json:"validation_status,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

// Transcript is durable output history for a worker session.
type Transcript struct {
	Text       string              `json:"text"`
	SourcePath string              `json:"source_path,omitempty"`
	Messages   []TranscriptMessage `json:"messages,omitempty"`
}

// TranscriptMessage is one durable conversation/output message.
type TranscriptMessage struct {
	Role    string    `json:"role"`
	Content string    `json:"content"`
	At      time.Time `json:"at,omitempty"`
}

// AgentWorker starts new sessions and attaches to durable existing sessions.
type AgentWorker interface {
	Start(ctx context.Context, req StartRequest) (AgentSession, error)
	Attach(ctx context.Context, ref SessionRef) (AgentSession, error)
}

// AgentSession is the durable handle for one accepted worker attempt.
type AgentSession interface {
	Ref() SessionRef
	Nudge(ctx context.Context, req NudgeRequest) error
	Cancel(ctx context.Context, req CancelRequest) error
	Stream(ctx context.Context, opts StreamOptions) (<-chan Event, error)
	Transcript(ctx context.Context) (Transcript, error)
	Artifacts(ctx context.Context) ([]Artifact, error)
	TerminalState(ctx context.Context) (TerminalState, error)
}
