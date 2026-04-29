package gascity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

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
