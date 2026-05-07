package daemon

import "time"

// AgentUpdateVersion is the wire-version stamped onto every agent-update event
// payload. Consumers (projections, transcripts, downstream judges) use it to
// pin parsing to the schema in schemas/agent-update.schema.json.
//
// soc-y0ct.1 (UW7-1 of soc-bcrn): introduced the agent-update protocol surface.
const AgentUpdateVersion = 1

// AgentUpdatePhaseStart payloads mark the beginning of a named RPI phase.
// Mirrors $defs/phase_start in schemas/agent-update.schema.json.
type AgentUpdatePhaseStart struct {
	PhaseName string         `json:"phase_name"`
	RunID     string         `json:"run_id"`
	Timestamp string         `json:"timestamp"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// AgentUpdatePhaseComplete payloads mark the terminal boundary of a phase.
// Status is one of "success" | "failure" | "timeout".
// Mirrors $defs/phase_complete in schemas/agent-update.schema.json.
type AgentUpdatePhaseComplete struct {
	PhaseName  string            `json:"phase_name"`
	RunID      string            `json:"run_id"`
	Timestamp  string            `json:"timestamp"`
	Status     string            `json:"status"`
	DurationMs int64             `json:"duration_ms,omitempty"`
	Artifacts  map[string]string `json:"artifacts,omitempty"`
}

// AgentUpdateCriterionVerdict payloads carry a per-criterion judgement.
// Status is one of "PASS" | "FAIL" | "SKIP".
// Mirrors $defs/criterion_verdict in schemas/agent-update.schema.json.
type AgentUpdateCriterionVerdict struct {
	CriterionID  string `json:"criterion_id"`
	Status       string `json:"status"`
	EvidencePath string `json:"evidence_path,omitempty"`
	Notes        string `json:"notes,omitempty"`
	RunID        string `json:"run_id"`
	Timestamp    string `json:"timestamp"`
}

// AgentUpdatePhaseHandoff payloads describe a transition between two phases of
// the same run.
// Mirrors $defs/phase_handoff in schemas/agent-update.schema.json.
type AgentUpdatePhaseHandoff struct {
	FromPhase  string `json:"from_phase"`
	ToPhase    string `json:"to_phase"`
	RunID      string `json:"run_id"`
	Timestamp  string `json:"timestamp"`
	PacketPath string `json:"packet_path,omitempty"`
}

// nowAgentUpdateTimestamp returns the canonical agent-update timestamp format
// (RFC 3339 nano, UTC). Centralised so constructors stay consistent.
func nowAgentUpdateTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// agentUpdateBasePayload returns the common envelope (currently just the
// version stamp). Each constructor layers payload-specific keys on top.
func agentUpdateBasePayload() map[string]any {
	return map[string]any{
		"agent_update_version": AgentUpdateVersion,
	}
}

// NewAgentUpdatePhaseStartEvent builds a LedgerEventInput for an agent-update
// phase_start payload. An empty Timestamp is defaulted to time.Now().UTC() in
// RFC 3339 nano format so callers can leave it blank for "now".
func NewAgentUpdatePhaseStartEvent(p AgentUpdatePhaseStart) LedgerEventInput {
	if p.Timestamp == "" {
		p.Timestamp = nowAgentUpdateTimestamp()
	}
	payload := agentUpdateBasePayload()
	payload["phase_name"] = p.PhaseName
	payload["run_id"] = p.RunID
	payload["timestamp"] = p.Timestamp
	if p.Metadata != nil {
		payload["metadata"] = p.Metadata
	}
	return LedgerEventInput{
		EventType: EventAgentUpdatePhaseStart,
		Payload:   payload,
	}
}

// NewAgentUpdatePhaseCompleteEvent builds a LedgerEventInput for an
// agent-update phase_complete payload. An empty Timestamp is defaulted to
// time.Now().UTC() in RFC 3339 nano format.
func NewAgentUpdatePhaseCompleteEvent(p AgentUpdatePhaseComplete) LedgerEventInput {
	if p.Timestamp == "" {
		p.Timestamp = nowAgentUpdateTimestamp()
	}
	payload := agentUpdateBasePayload()
	payload["phase_name"] = p.PhaseName
	payload["run_id"] = p.RunID
	payload["timestamp"] = p.Timestamp
	payload["status"] = p.Status
	if p.DurationMs != 0 {
		payload["duration_ms"] = p.DurationMs
	}
	if p.Artifacts != nil {
		payload["artifacts"] = p.Artifacts
	}
	return LedgerEventInput{
		EventType: EventAgentUpdatePhaseComplete,
		Payload:   payload,
	}
}

// NewAgentUpdateCriterionVerdictEvent builds a LedgerEventInput for an
// agent-update criterion_verdict payload. An empty Timestamp is defaulted to
// time.Now().UTC() in RFC 3339 nano format.
func NewAgentUpdateCriterionVerdictEvent(p AgentUpdateCriterionVerdict) LedgerEventInput {
	if p.Timestamp == "" {
		p.Timestamp = nowAgentUpdateTimestamp()
	}
	payload := agentUpdateBasePayload()
	payload["criterion_id"] = p.CriterionID
	payload["status"] = p.Status
	if p.EvidencePath != "" {
		payload["evidence_path"] = p.EvidencePath
	}
	if p.Notes != "" {
		payload["notes"] = p.Notes
	}
	payload["run_id"] = p.RunID
	payload["timestamp"] = p.Timestamp
	return LedgerEventInput{
		EventType: EventAgentUpdateCriterionVerdict,
		Payload:   payload,
	}
}

// NewAgentUpdatePhaseHandoffEvent builds a LedgerEventInput for an agent-update
// phase_handoff payload. An empty Timestamp is defaulted to time.Now().UTC() in
// RFC 3339 nano format.
func NewAgentUpdatePhaseHandoffEvent(p AgentUpdatePhaseHandoff) LedgerEventInput {
	if p.Timestamp == "" {
		p.Timestamp = nowAgentUpdateTimestamp()
	}
	payload := agentUpdateBasePayload()
	payload["from_phase"] = p.FromPhase
	payload["to_phase"] = p.ToPhase
	payload["run_id"] = p.RunID
	payload["timestamp"] = p.Timestamp
	if p.PacketPath != "" {
		payload["packet_path"] = p.PacketPath
	}
	return LedgerEventInput{
		EventType: EventAgentUpdatePhaseHandoff,
		Payload:   payload,
	}
}
