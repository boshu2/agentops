package daemon

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

// TestAgentUpdate_EventTypesRegistered guards the four agent-update event
// type constants against accidental rename/removal and pins the wire-level
// strings that downstream consumers (transcripts, projections) depend on.
func TestAgentUpdate_EventTypesRegistered(t *testing.T) {
	cases := []struct {
		name string
		got  EventType
		want string
	}{
		{"phase_start", EventAgentUpdatePhaseStart, "agent_update.phase_start"},
		{"phase_complete", EventAgentUpdatePhaseComplete, "agent_update.phase_complete"},
		{"criterion_verdict", EventAgentUpdateCriterionVerdict, "agent_update.criterion_verdict"},
		{"phase_handoff", EventAgentUpdatePhaseHandoff, "agent_update.phase_handoff"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.got) == "" {
				t.Fatalf("event type %s is empty", tc.name)
			}
			if string(tc.got) != tc.want {
				t.Fatalf("event type %s: got %q, want %q", tc.name, tc.got, tc.want)
			}
			if err := ValidateEventType(tc.got); err != nil {
				t.Fatalf("ValidateEventType(%q) returned error: %v", tc.got, err)
			}
		})
	}
}

func TestAgentUpdatePhaseStart_RoundTrip(t *testing.T) {
	in := AgentUpdatePhaseStart{
		PhaseName: "research",
		RunID:     "run-abc",
		Timestamp: "2026-05-07T12:34:56.789Z",
		Metadata:  map[string]any{"operator": "bo", "lane": "primary"},
	}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal phase_start: %v", err)
	}
	var out AgentUpdatePhaseStart
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal phase_start: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n in: %+v\nout: %+v", in, out)
	}
}

func TestAgentUpdatePhaseComplete_RoundTrip(t *testing.T) {
	in := AgentUpdatePhaseComplete{
		PhaseName:  "implement",
		RunID:      "run-abc",
		Timestamp:  "2026-05-07T12:45:00Z",
		Status:     "success",
		DurationMs: 12345,
		Artifacts:  map[string]string{"log": ".agents/daemon/rpi/runs/run-abc/job-x/rpi-run.log"},
	}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal phase_complete: %v", err)
	}
	var out AgentUpdatePhaseComplete
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal phase_complete: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n in: %+v\nout: %+v", in, out)
	}
}

func TestAgentUpdateCriterionVerdict_RoundTrip(t *testing.T) {
	in := AgentUpdateCriterionVerdict{
		CriterionID:  "ac-y0ct.1",
		Status:       "PASS",
		EvidencePath: ".agents/evidence/ac-y0ct.1.txt",
		Notes:        "schema + types compile cleanly",
		RunID:        "run-abc",
		Timestamp:    "2026-05-07T12:50:00Z",
	}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal criterion_verdict: %v", err)
	}
	var out AgentUpdateCriterionVerdict
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal criterion_verdict: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n in: %+v\nout: %+v", in, out)
	}
}

func TestAgentUpdatePhaseHandoff_RoundTrip(t *testing.T) {
	in := AgentUpdatePhaseHandoff{
		FromPhase:  "plan",
		ToPhase:    "implement",
		RunID:      "run-abc",
		Timestamp:  "2026-05-07T12:55:00Z",
		PacketPath: ".agents/daemon/rpi/runs/run-abc/packet.json",
	}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal phase_handoff: %v", err)
	}
	var out AgentUpdatePhaseHandoff
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal phase_handoff: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n in: %+v\nout: %+v", in, out)
	}
}

// TestAgentUpdatePhaseStartEvent_VersionStamped verifies the constructor
// stamps agent_update_version=1 and the right EventType onto the
// LedgerEventInput payload so downstream NewLedgerEvent flows preserve it.
func TestAgentUpdatePhaseStartEvent_VersionStamped(t *testing.T) {
	evt := NewAgentUpdatePhaseStartEvent(AgentUpdatePhaseStart{
		PhaseName: "research",
		RunID:     "run-abc",
		Timestamp: "2026-05-07T12:34:56Z",
	})
	if evt.EventType != EventAgentUpdatePhaseStart {
		t.Fatalf("EventType: got %q, want %q", evt.EventType, EventAgentUpdatePhaseStart)
	}
	v, ok := evt.Payload["agent_update_version"]
	if !ok {
		t.Fatalf("payload missing agent_update_version key: %#v", evt.Payload)
	}
	got, ok := v.(int)
	if !ok {
		t.Fatalf("agent_update_version should be int, got %T (%v)", v, v)
	}
	if got != AgentUpdateVersion {
		t.Fatalf("agent_update_version: got %d, want %d", got, AgentUpdateVersion)
	}
	if got != 1 {
		t.Fatalf("agent_update_version pinned wire-value 1, got %d", got)
	}
	// Spot-check the per-payload fields propagated.
	if evt.Payload["phase_name"] != "research" {
		t.Fatalf("phase_name: got %v, want %q", evt.Payload["phase_name"], "research")
	}
	if evt.Payload["run_id"] != "run-abc" {
		t.Fatalf("run_id: got %v, want %q", evt.Payload["run_id"], "run-abc")
	}
}

// TestAgentUpdate_TimestampDefaulted verifies that each constructor stamps a
// non-empty RFC 3339 nano timestamp when the caller passes an empty Timestamp,
// across all four payload shapes.
func TestAgentUpdate_TimestampDefaulted(t *testing.T) {
	cases := []struct {
		name string
		got  LedgerEventInput
	}{
		{
			name: "phase_start",
			got: NewAgentUpdatePhaseStartEvent(AgentUpdatePhaseStart{
				PhaseName: "research",
				RunID:     "run-abc",
			}),
		},
		{
			name: "phase_complete",
			got: NewAgentUpdatePhaseCompleteEvent(AgentUpdatePhaseComplete{
				PhaseName: "research",
				RunID:     "run-abc",
				Status:    "success",
			}),
		},
		{
			name: "criterion_verdict",
			got: NewAgentUpdateCriterionVerdictEvent(AgentUpdateCriterionVerdict{
				CriterionID: "ac-y0ct.1",
				Status:      "PASS",
				RunID:       "run-abc",
			}),
		},
		{
			name: "phase_handoff",
			got: NewAgentUpdatePhaseHandoffEvent(AgentUpdatePhaseHandoff{
				FromPhase: "plan",
				ToPhase:   "implement",
				RunID:     "run-abc",
			}),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts, ok := tc.got.Payload["timestamp"].(string)
			if !ok {
				t.Fatalf("timestamp missing or wrong type: %#v", tc.got.Payload["timestamp"])
			}
			if ts == "" {
				t.Fatalf("timestamp default should be non-empty for %s", tc.name)
			}
			if _, err := time.Parse(time.RFC3339Nano, ts); err != nil {
				t.Fatalf("timestamp %q does not parse as RFC3339Nano: %v", ts, err)
			}
		})
	}
}
