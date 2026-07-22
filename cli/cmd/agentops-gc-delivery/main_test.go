package main

import "testing"

func TestStatusObservationRequiresCallerAttestedCanonicalUTC(t *testing.T) {
	for _, value := range []string{"", "2026-07-22T03:00:00-04:00", "not-a-time"} {
		if _, err := statusObservation(value); err == nil {
			t.Fatalf("statusObservation(%q) succeeded", value)
		}
	}
	got, err := statusObservation("2026-07-22T03:00:00Z")
	if err != nil || got.UTC().Format("2006-01-02T15:04:05Z07:00") != "2026-07-22T03:00:00Z" {
		t.Fatalf("statusObservation = %v, %v", got, err)
	}
}
