package gascity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadinessResponseIsReadyBothShapes(t *testing.T) {
	cases := []struct {
		name      string
		json      string
		wantReady bool
		wantStat  string
	}{
		{
			name:      "legacy_ready_true",
			json:      `{"ready":true,"status":"ready"}`,
			wantReady: true,
			wantStat:  "ready",
		},
		{
			name:      "legacy_ready_false",
			json:      `{"ready":false,"status":"degraded","degraded":["claude"]}`,
			wantReady: false,
			wantStat:  "degraded",
		},
		{
			name:      "gc_v1_items_all_configured",
			json:      `{"items":{"claude":{"status":"configured"},"codex":{"status":"configured"}}}`,
			wantReady: true,
			wantStat:  "ready",
		},
		{
			name:      "gc_v1_items_partial",
			json:      `{"items":{"claude":{"status":"configured"},"gemini":{"status":"not_installed"}}}`,
			wantReady: true,
			wantStat:  "partial (1 configured, 1 missing)",
		},
		{
			name:      "gc_v1_items_degraded_blocks",
			json:      `{"items":{"claude":{"status":"configured"},"codex":{"status":"degraded"}}}`,
			wantReady: false,
			wantStat:  "partial (1 configured, 1 missing)",
		},
		{
			name:      "empty",
			json:      `{}`,
			wantReady: false,
			wantStat:  "no readiness data",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var resp ReadinessResponse
			if err := json.Unmarshal([]byte(tc.json), &resp); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got := resp.IsReady(); got != tc.wantReady {
				t.Fatalf("IsReady() = %v, want %v (response=%#v)", got, tc.wantReady, resp)
			}
			if got := resp.EffectiveStatus(); got != tc.wantStat {
				t.Fatalf("EffectiveStatus() = %q, want %q", got, tc.wantStat)
			}
		})
	}
}

func TestContractVersion(t *testing.T) {
	if err := ValidateContractVersion(AdapterContractVersion); err != nil {
		t.Fatalf("current contract rejected: %v", err)
	}
	if err := ValidateContractVersion("gascity-openapi-1970-01-01"); err == nil {
		t.Fatal("unknown contract version accepted")
	}
	if AdapterStrategy != "handwritten-narrow" {
		t.Fatalf("adapter strategy drifted: %q", AdapterStrategy)
	}
}

func TestDTOFixturesRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		file string
		ptr  any
	}{
		{name: "health", file: "health.json", ptr: &HealthResponse{}},
		{name: "readiness", file: "readiness.json", ptr: &ReadinessResponse{}},
		{name: "city response", file: "city-response.json", ptr: &CityResponse{}},
		{name: "session", file: "session.json", ptr: &Session{}},
		{name: "session submit request", file: "session-submit-request.json", ptr: &SessionSubmitRequest{}},
		{name: "transcript", file: "transcript.json", ptr: &TranscriptResponse{}},
		{name: "wire event", file: "wire-event.json", ptr: &WireEvent{}},
		{name: "tagged wire event", file: "tagged-wire-event.json", ptr: &TaggedWireEvent{}},
		{name: "event stream envelope", file: "event-stream-envelope.json", ptr: &EventStreamEnvelope{}},
		{name: "heartbeat", file: "heartbeat.json", ptr: &HeartbeatEvent{}},
		{name: "problem details", file: "problem-details.json", ptr: &ProblemDetails{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", tc.file))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, tc.ptr); err != nil {
				t.Fatalf("unmarshal fixture: %v", err)
			}
			encoded, err := json.Marshal(tc.ptr)
			if err != nil {
				t.Fatalf("marshal fixture: %v", err)
			}
			roundTrip := reflect.New(reflect.TypeOf(tc.ptr).Elem()).Interface()
			if err := json.Unmarshal(encoded, roundTrip); err != nil {
				t.Fatalf("unmarshal round trip: %v", err)
			}
			if !reflect.DeepEqual(tc.ptr, roundTrip) {
				t.Fatalf("round trip mismatch:\noriginal: %#v\nroundtrip: %#v", tc.ptr, roundTrip)
			}
		})
	}
}
