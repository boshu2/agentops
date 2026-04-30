// Package gascity contains AgentOps' narrow handwritten adapter contract for
// the public GasCity supervisor API.
package gascity

import "fmt"

// AdapterContractVersion pins the GasCity public API fixture set this adapter
// was written against.
const AdapterContractVersion = "gascity-openapi-2026-04-28"

// AdapterStrategy records the intentionally narrow first implementation
// strategy. Generated clients are deferred until these DTO fixtures stabilize.
const AdapterStrategy = "handwritten-narrow"

// MinSupportedSupervisorVersion is the minimum gc supervisor version expected
// by the AgentOps adapter.
const MinSupportedSupervisorVersion = "0.13.0"

// MutationHeader is required by GasCity on all mutation requests.
const MutationHeader = "X-GC-Request"

// RequestIDHeader is returned by GasCity on every response for correlation.
const RequestIDHeader = "X-GC-Request-Id"

// ValidateContractVersion rejects unknown adapter fixture versions.
func ValidateContractVersion(version string) error {
	if version != AdapterContractVersion {
		return fmt.Errorf("unsupported GasCity adapter contract %q", version)
	}
	return nil
}

// HealthResponse is the narrow DTO for GET /health.
type HealthResponse struct {
	OK     bool   `json:"ok,omitempty"`
	Status string `json:"status,omitempty"`
}

// ReadinessResponse is the narrow DTO for supervisor, provider, or city
// readiness probes.
//
// Two upstream shapes are accepted:
//
//   - Legacy / fixture shape (gascity ≤ 0.13.x): top-level {ready,status,degraded,providers}.
//   - gc v1.0.0+ shape: {items: {<name>: {status: configured|...}}} with no
//     top-level ready boolean. Callers should use IsReady / EffectiveStatus
//     rather than reading Ready directly.
type ReadinessResponse struct {
	Ready     bool                     `json:"ready"`
	Status    string                   `json:"status,omitempty"`
	Degraded  []string                 `json:"degraded,omitempty"`
	Providers []string                 `json:"providers,omitempty"`
	Items     map[string]ReadinessItem `json:"items,omitempty"`
}

// ReadinessItem is one entry in the gc v1.0.0+ items-shape readiness response.
type ReadinessItem struct {
	Name        string `json:"name,omitempty"`
	Kind        string `json:"kind,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Status      string `json:"status"`
	Detail      string `json:"detail,omitempty"`
}

// IsReady reports whether the readiness response signals a usable city / scope.
// It accepts either the legacy top-level Ready bool or the gc v1.0.0+ items map
// (ready when at least one item is "configured" or "ready" and none are
// "degraded"/"error"). Empty responses report not ready.
func (r ReadinessResponse) IsReady() bool {
	if r.Ready {
		return true
	}
	if len(r.Items) == 0 {
		return false
	}
	hasConfigured := false
	for _, item := range r.Items {
		switch item.Status {
		case "configured", "ready":
			hasConfigured = true
		case "degraded", "error", "unavailable":
			return false
		}
	}
	return hasConfigured
}

// EffectiveStatus returns a human-readable status string, preferring the
// legacy Status field but synthesising from Items when only the gc v1.0.0+
// shape is present.
func (r ReadinessResponse) EffectiveStatus() string {
	if r.Status != "" {
		return r.Status
	}
	if len(r.Items) == 0 {
		return "no readiness data"
	}
	configured, missing := 0, 0
	for _, item := range r.Items {
		switch item.Status {
		case "configured", "ready":
			configured++
		default:
			missing++
		}
	}
	if configured > 0 && missing == 0 {
		return "ready"
	}
	if configured > 0 {
		return fmt.Sprintf("partial (%d configured, %d missing)", configured, missing)
	}
	return "not ready"
}

// CityCreateRequest is the request body for POST /v0/city.
type CityCreateRequest struct {
	Dir              string `json:"dir"`
	Provider         string `json:"provider"`
	BootstrapProfile string `json:"bootstrap_profile,omitempty"`
}

// CityResponse is the response body for city create/register style operations.
type CityResponse struct {
	OK   bool   `json:"ok"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// CityInfo is one item in GET /v0/cities.
type CityInfo struct {
	Name            string   `json:"name"`
	Path            string   `json:"path"`
	Running         bool     `json:"running"`
	Status          string   `json:"status,omitempty"`
	Error           string   `json:"error,omitempty"`
	PhasesCompleted []string `json:"phases_completed,omitempty"`
}

// CityListResponse is the response body for GET /v0/cities.
type CityListResponse struct {
	Items []CityInfo `json:"items,omitempty"`
	Total int        `json:"total"`
}

// CityGetResponse is the response body for GET /v0/city/{cityName}.
type CityGetResponse struct {
	Name            string `json:"name"`
	Path            string `json:"path"`
	AgentCount      int    `json:"agent_count"`
	RigCount        int    `json:"rig_count"`
	Suspended       bool   `json:"suspended"`
	UptimeSec       int    `json:"uptime_sec"`
	Version         string `json:"version,omitempty"`
	Provider        string `json:"provider,omitempty"`
	SessionTemplate string `json:"session_template,omitempty"`
}

// SessionCreateRequest is the request body for POST /v0/city/{cityName}/sessions.
type SessionCreateRequest struct {
	Kind      string            `json:"kind,omitempty"`
	Name      string            `json:"name,omitempty"`
	Alias     string            `json:"alias,omitempty"`
	Message   string            `json:"message,omitempty"`
	Async     bool              `json:"async,omitempty"`
	Options   map[string]string `json:"options,omitempty"`
	ProjectID string            `json:"project_id,omitempty"`
	Title     string            `json:"title,omitempty"`
}

// SubmissionCapabilities describes the semantic submit modes a session accepts.
type SubmissionCapabilities struct {
	SupportsFollowUp     bool `json:"supports_follow_up"`
	SupportsInterruptNow bool `json:"supports_interrupt_now"`
}

// Session describes the fields AgentOps needs from a GasCity session.
type Session struct {
	ID                     string                  `json:"id,omitempty"`
	Kind                   string                  `json:"kind,omitempty"`
	Template               string                  `json:"template,omitempty"`
	State                  string                  `json:"state,omitempty"`
	Status                 string                  `json:"status,omitempty"`
	Reason                 string                  `json:"reason,omitempty"`
	Title                  string                  `json:"title,omitempty"`
	Alias                  string                  `json:"alias,omitempty"`
	Provider               string                  `json:"provider,omitempty"`
	DisplayName            string                  `json:"display_name,omitempty"`
	SessionName            string                  `json:"session_name,omitempty"`
	CreatedAt              string                  `json:"created_at,omitempty"`
	LastActive             string                  `json:"last_active,omitempty"`
	Attached               bool                    `json:"attached,omitempty"`
	Running                bool                    `json:"running,omitempty"`
	ActiveBead             string                  `json:"active_bead,omitempty"`
	LastOutput             string                  `json:"last_output,omitempty"`
	Model                  string                  `json:"model,omitempty"`
	ContextPct             *int                    `json:"context_pct,omitempty"`
	ContextWindow          *int                    `json:"context_window,omitempty"`
	Activity               string                  `json:"activity,omitempty"`
	Rig                    string                  `json:"rig,omitempty"`
	Pool                   string                  `json:"pool,omitempty"`
	ConfiguredNamedSession bool                    `json:"configured_named_session,omitempty"`
	SubmissionCapabilities *SubmissionCapabilities `json:"submission_capabilities,omitempty"`
	Options                map[string]string       `json:"options,omitempty"`
	Metadata               map[string]string       `json:"metadata,omitempty"`
	Closed                 bool                    `json:"closed,omitempty"`
}

// SessionListParams controls GET /v0/city/{cityName}/sessions query parameters.
type SessionListParams struct {
	State    string
	Template string
	Cursor   string
	Limit    int
	Peek     bool
}

// SessionListResponse is the response body for GET /v0/city/{cityName}/sessions.
type SessionListResponse struct {
	Items         []Session `json:"items,omitempty"`
	Total         int       `json:"total"`
	NextCursor    string    `json:"next_cursor,omitempty"`
	Partial       bool      `json:"partial,omitempty"`
	PartialErrors []string  `json:"partial_errors,omitempty"`
}

// SessionGetOptions controls GET /v0/city/{cityName}/session/{id}.
type SessionGetOptions struct {
	Peek bool
}

// SessionSubmitRequest is the request body for POST /v0/city/{cityName}/session/{id}/submit.
type SessionSubmitRequest struct {
	Message string `json:"message"`
	Intent  string `json:"intent,omitempty"`
}

// SessionSubmitResponse is the response body for session submit.
type SessionSubmitResponse struct {
	Status string `json:"status"`
	ID     string `json:"id"`
	Queued bool   `json:"queued"`
	Intent string `json:"intent"`
}

// TranscriptOptions controls GET /v0/city/{cityName}/session/{id}/transcript.
type TranscriptOptions struct {
	Format string
	Tail   *int
	Before string
}

// TranscriptResponse is the transcript/result evidence response shape consumed
// by RPI and worker jobs.
type TranscriptResponse struct {
	ID         string                `json:"id,omitempty"`
	SessionID  string                `json:"session_id,omitempty"`
	Template   string                `json:"template,omitempty"`
	Provider   string                `json:"provider,omitempty"`
	Format     string                `json:"format,omitempty"`
	Turns      []TranscriptEntry     `json:"turns,omitempty"`
	Messages   []map[string]any      `json:"messages,omitempty"`
	Pagination *TranscriptPagination `json:"pagination,omitempty"`
	Artifacts  []TranscriptArtifact  `json:"artifacts,omitempty"`
}

// TranscriptEntry is one display turn in a transcript response.
type TranscriptEntry struct {
	Role      string `json:"role"`
	Text      string `json:"text,omitempty"`
	Content   string `json:"content,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

// TranscriptPagination describes a paginated transcript response.
type TranscriptPagination struct {
	HasOlderMessages       bool   `json:"has_older_messages"`
	ReturnedMessageCount   int    `json:"returned_message_count"`
	TotalCompactions       int    `json:"total_compactions"`
	TotalMessageCount      int    `json:"total_message_count"`
	TruncatedBeforeMessage string `json:"truncated_before_message,omitempty"`
}

// TranscriptArtifact is one artifact reference returned with transcript
// evidence.
type TranscriptArtifact struct {
	Path string `json:"path"`
	Kind string `json:"kind,omitempty"`
}

// WireEvent is the city-scoped event line shape from the public event API.
type WireEvent struct {
	Seq     int64          `json:"seq"`
	Type    string         `json:"type"`
	Subject string         `json:"subject,omitempty"`
	Actor   string         `json:"actor,omitempty"`
	Message string         `json:"message,omitempty"`
	TS      string         `json:"ts,omitempty"`
	Payload map[string]any `json:"payload,omitempty"`
}

// TaggedWireEvent is the supervisor-scoped event line shape.
type TaggedWireEvent struct {
	City string `json:"city"`
	WireEvent
}

// EventListParams controls event list query parameters.
type EventListParams struct {
	Type   string
	Actor  string
	Since  string
	Index  string
	Wait   string
	Cursor string
	Limit  int
}

// EventListResponse is the city-scoped event list response body.
type EventListResponse struct {
	Items         []WireEvent `json:"items,omitempty"`
	Total         int         `json:"total"`
	NextCursor    string      `json:"next_cursor,omitempty"`
	Partial       bool        `json:"partial,omitempty"`
	PartialErrors []string    `json:"partial_errors,omitempty"`
}

// TaggedEventListResponse is the supervisor-scoped event list response body.
type TaggedEventListResponse struct {
	Items []TaggedWireEvent `json:"items,omitempty"`
	Total int               `json:"total"`
}

// EventEmitRequest is the request body for POST /v0/city/{cityName}/events.
type EventEmitRequest struct {
	Type    string `json:"type"`
	Actor   string `json:"actor"`
	Subject string `json:"subject,omitempty"`
	Message string `json:"message,omitempty"`
}

// EventEmitResponse is the response body for POST /v0/city/{cityName}/events.
type EventEmitResponse struct {
	Status string `json:"status"`
}

// EventStreamEnvelope is the semantic event payload emitted by SSE streams.
type EventStreamEnvelope struct {
	Seq     int64          `json:"seq"`
	Type    string         `json:"type"`
	Subject string         `json:"subject,omitempty"`
	Actor   string         `json:"actor,omitempty"`
	Message string         `json:"message,omitempty"`
	TS      string         `json:"ts,omitempty"`
	Payload map[string]any `json:"payload,omitempty"`
}

// TaggedEventStreamEnvelope is the supervisor-scoped SSE event payload.
type TaggedEventStreamEnvelope struct {
	City string `json:"city"`
	EventStreamEnvelope
}

// HeartbeatEvent is the heartbeat payload emitted by SSE streams.
type HeartbeatEvent struct {
	Timestamp string `json:"timestamp,omitempty"`
	TS        string `json:"ts,omitempty"`
}

// EventStreamOptions controls event stream reconnect parameters.
type EventStreamOptions struct {
	LastEventID string
	AfterSeq    string
	AfterCursor string
}

// EventStreamScope selects city or supervisor reconnect semantics.
type EventStreamScope string

const (
	EventStreamScopeCity       EventStreamScope = "city"
	EventStreamScopeSupervisor EventStreamScope = "supervisor"
)

// EventStreamFrame is one parsed SSE frame from a GasCity event stream.
type EventStreamFrame struct {
	ID          string
	Event       string
	Retry       int
	RawData     []byte
	Heartbeat   *HeartbeatEvent
	CityEvent   *EventStreamEnvelope
	TaggedEvent *TaggedEventStreamEnvelope
}

const (
	TerminalStatusUnknown                   = "unknown"
	TerminalStatusRunning                   = "running"
	TerminalStatusCompleted                 = "completed"
	TerminalStatusFailed                    = "failed"
	TerminalStatusCancelled                 = "cancelled"
	TerminalStatusLost                      = "lost"
	TerminalStatusProviderUnreachable       = "provider_unreachable"
	TerminalStatusEventStreamUnavailable    = "event_stream_unavailable"
	TerminalStatusTerminalWithoutTranscript = "terminal_without_transcript"
)

// TerminalStateInput is the evidence set used to classify a GasCity-backed
// worker/session state.
type TerminalStateInput struct {
	EventType              string
	EventPayload           map[string]any
	SessionState           string
	SessionStatus          string
	SessionMissing         bool
	ProviderUnreachable    bool
	EventStreamUnavailable bool
	TranscriptRequired     bool
	TranscriptAvailable    bool
	TranscriptUnavailable  bool
}

// TerminalClassification is the normalized AgentOps status for a GasCity
// worker/session.
type TerminalClassification struct {
	Status   string
	Terminal bool
	Degraded bool
	Reason   string
}

// ProblemDetails is the RFC 9457 error body returned by GasCity.
type ProblemDetails struct {
	Type     string         `json:"type,omitempty"`
	Title    string         `json:"title,omitempty"`
	Status   int            `json:"status,omitempty"`
	Detail   string         `json:"detail,omitempty"`
	Instance string         `json:"instance,omitempty"`
	Errors   map[string]any `json:"errors,omitempty"`
}
