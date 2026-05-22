package daemon

import (
	"testing"
	"time"
)

func TestPhaseLatencyHistogram(t *testing.T) {
	base := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	events := []LedgerEvent{
		mustTelemetryEvent(t, "evt-a-accepted", "job-a", EventJobAccepted, base, JobTypeRPIPhase, map[string]any{"job_payload": map[string]any{"phase_name": "discovery"}}),
		mustTelemetryEvent(t, "evt-a-completed", "job-a", EventJobCompleted, base.Add(10*time.Second), "", nil),
		mustTelemetryEvent(t, "evt-b-accepted", "job-b", EventJobAccepted, base.Add(time.Minute), JobTypeRPIPhase, map[string]any{"job_payload": map[string]any{"phase_name": "discovery"}}),
		mustTelemetryEvent(t, "evt-b-completed", "job-b", EventJobCompleted, base.Add(time.Minute+21*time.Second), "", nil),
		mustTelemetryEvent(t, "evt-c-accepted", "job-c", EventJobAccepted, base.Add(2*time.Minute), JobTypeRPIPhase, map[string]any{"job_payload": map[string]any{"phase_name": "implementation"}}),
		mustTelemetryEvent(t, "evt-c-completed", "job-c", EventJobCompleted, base.Add(2*time.Minute+3*time.Second), "", nil),
	}

	telemetry := BuildLedgerTelemetry(events, base.Add(3*time.Minute), DefaultTelemetryWindow)
	histograms := map[string]PhaseLatencyHistogram{}
	for _, hist := range telemetry.PhaseLatency {
		histograms[hist.PhaseName] = hist
	}
	discovery := histograms["discovery"]
	if discovery.Count != 2 || discovery.P50Millis != int64((10*time.Second).Milliseconds()) || discovery.P99Millis != int64((21*time.Second).Milliseconds()) {
		t.Fatalf("discovery histogram = %#v, want count=2 p50=10s p99=21s", discovery)
	}
	implementation := histograms["implementation"]
	if implementation.Count != 1 || implementation.P50Millis != (3*time.Second).Milliseconds() || implementation.P99Millis != (3*time.Second).Milliseconds() {
		t.Fatalf("implementation histogram = %#v, want count=1 p50/p99=3s", implementation)
	}
}

func TestWorkerKindDistribution(t *testing.T) {
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	events := []LedgerEvent{
		mustTelemetryEvent(t, "evt-codex-new", "job-codex-new", EventJobAccepted, now.Add(-time.Hour), JobTypeWikiForge, map[string]any{"job_payload": map[string]any{"worker_kind": "codex"}}),
		mustTelemetryEvent(t, "evt-claude-new", "job-claude-new", EventJobAccepted, now.Add(-2*time.Hour), JobTypeWikiForge, map[string]any{"job_payload": map[string]any{"worker_kind": "claude"}}),
		mustTelemetryEvent(t, "evt-codex-old", "job-codex-old", EventJobAccepted, now.Add(-48*time.Hour), JobTypeWikiForge, map[string]any{"job_payload": map[string]any{"worker_kind": "codex"}}),
	}

	telemetry := BuildLedgerTelemetry(events, now, DefaultTelemetryWindow)
	counts := map[string]int{}
	for _, dist := range telemetry.WorkerKindDistribution {
		counts[dist.WorkerKind] = dist.Count
	}
	if counts["codex"] != 1 || counts["claude"] != 1 {
		t.Fatalf("worker counts = %#v, want codex=1 claude=1", counts)
	}
}

func TestFailureRateByJobType(t *testing.T) {
	base := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	events := []LedgerEvent{
		mustTelemetryEvent(t, "evt-rpi-ok-accepted", "job-rpi-ok", EventJobAccepted, base, JobTypeRPIRun, nil),
		mustTelemetryEvent(t, "evt-rpi-ok-completed", "job-rpi-ok", EventJobCompleted, base.Add(time.Second), "", nil),
		mustTelemetryEvent(t, "evt-rpi-fail-accepted", "job-rpi-fail", EventJobAccepted, base, JobTypeRPIRun, nil),
		mustTelemetryEvent(t, "evt-rpi-fail-failed", "job-rpi-fail", EventJobFailed, base.Add(time.Second), "", nil),
		mustTelemetryEvent(t, "evt-wiki-cancel-accepted", "job-wiki-cancel", EventJobAccepted, base, JobTypeWikiForge, nil),
		mustTelemetryEvent(t, "evt-wiki-cancel-cancelled", "job-wiki-cancel", EventJobCancelled, base.Add(time.Second), "", nil),
	}

	telemetry := BuildLedgerTelemetry(events, base.Add(2*time.Second), DefaultTelemetryWindow)
	rates := map[JobType]JobTypeFailureRateSummary{}
	for _, rate := range telemetry.FailureRates {
		rates[rate.JobType] = rate
	}
	rpi := rates[JobTypeRPIRun]
	if rpi.TerminalCount != 2 || rpi.FailedCount != 1 || rpi.FailureRate != 0.5 {
		t.Fatalf("rpi failure rate = %#v, want 1/2", rpi)
	}
	wiki := rates[JobTypeWikiForge]
	if wiki.TerminalCount != 1 || wiki.FailedCount != 0 || wiki.FailureRate != 0 {
		t.Fatalf("wiki failure rate = %#v, want 0/1", wiki)
	}
}

func mustTelemetryEvent(t *testing.T, eventID, jobID string, eventType EventType, occurredAt time.Time, jobType JobType, payload map[string]any) LedgerEvent {
	t.Helper()
	event, err := NewLedgerEvent(LedgerEventInput{
		EventID:    eventID,
		RequestID:  RequestID("req-" + eventID),
		JobID:      jobID,
		EventType:  eventType,
		OccurredAt: occurredAt,
		Actor:      "test",
		JobType:    jobType,
		Payload:    payload,
	})
	if err != nil {
		t.Fatalf("NewLedgerEvent(%s): %v", eventID, err)
	}
	return event
}
