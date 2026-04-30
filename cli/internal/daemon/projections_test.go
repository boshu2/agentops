package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRequestIDModelRequiresLedgerCorrelation(t *testing.T) {
	if err := ValidateRequestID("req_20260428_000001"); err != nil {
		t.Fatalf("valid request id rejected: %v", err)
	}
	if err := ValidateRequestID("gc-request-123"); err != nil {
		t.Fatalf("provider-shaped request id rejected: %v", err)
	}
	for _, value := range []string{"", "   ", "req with spaces", "req\nbad"} {
		if err := ValidateRequestID(value); err == nil {
			t.Fatalf("invalid request id %q accepted", value)
		}
	}

	event, err := NewLedgerEvent(LedgerEventInput{
		EventID:           "evt-1",
		RequestID:         RequestID("req-1"),
		JobID:             "job-1",
		EventType:         EventJobAccepted,
		OccurredAt:        projectionTestTime(t, 0),
		Actor:             "ao",
		JobType:           JobTypeRPIRun,
		ProjectionTargets: []ProjectionName{ProjectionRPIRegistry, ProjectionOpenClaw},
	})
	if err != nil {
		t.Fatalf("create ledger event: %v", err)
	}
	if event.RequestID != "req-1" {
		t.Fatalf("RequestID = %q, want req-1", event.RequestID)
	}
	if got := event.Payload["job_type"]; got != string(JobTypeRPIRun) {
		t.Fatalf("payload job_type = %#v, want %q", got, JobTypeRPIRun)
	}
	targets := projectionTargetsFromPayload(event.Payload)
	if len(targets) != 2 || targets[0] != ProjectionRPIRegistry || targets[1] != ProjectionOpenClaw {
		t.Fatalf("projection targets = %#v, want rpi/openclaw", targets)
	}
}

func TestProjectionRebuildsRpiDreamWikiAndOpenClawFromLedger(t *testing.T) {
	events := []LedgerEvent{
		mustNewProjectionTestEvent(t, "evt-rpi-accepted", "req-rpi-1", "job-rpi", EventJobAccepted, JobTypeRPIRun, 0, nil),
		mustNewProjectionTestEvent(t, "evt-rpi-completed", "req-rpi-2", "job-rpi", EventJobCompleted, "", 1, map[string]any{
			"artifacts": map[string]string{"summary": ".agents/rpi/runs/job-rpi/phase-3-summary.md"},
		}),
		mustNewProjectionTestEvent(t, "evt-dream-accepted", "req-dream-1", "job-dream", EventJobAccepted, JobTypeDreamRun, 2, nil),
		mustNewProjectionTestEvent(t, "evt-dream-claimed", "req-dream-2", "job-dream", EventJobClaimed, "", 3, nil),
		mustNewProjectionTestEvent(t, "evt-wiki-accepted", "req-wiki-1", "job-wiki", EventJobAccepted, JobTypeWikiForge, 4, nil),
		mustNewProjectionTestEvent(t, "evt-wiki-failed", "req-wiki-2", "job-wiki", EventJobFailed, "", 5, map[string]any{
			"failure_code": string(FailureProviderUnreachable),
			"message":      "worker unavailable",
			"retryable":    true,
		}),
	}

	projections, err := RebuildProjections(events, ProjectionRebuildOptions{
		RebuiltAt:    projectionTestTime(t, 10),
		SourceLedger: ".agents/daemon/ledger.jsonl",
	})
	if err != nil {
		t.Fatalf("rebuild projections: %v", err)
	}
	if projections.LastEventID != "evt-wiki-failed" {
		t.Fatalf("LastEventID = %q, want evt-wiki-failed", projections.LastEventID)
	}
	if len(projections.RPI.Runs) != 1 || projections.RPI.Runs[0].Status != JobStatusCompleted {
		t.Fatalf("RPI projection = %#v, want one completed run", projections.RPI.Runs)
	}
	if projections.RPI.Runs[0].RequestID != "req-rpi-1" || len(projections.RPI.Runs[0].RequestIDs) != 2 {
		t.Fatalf("RPI request ids = %#v, first=%q", projections.RPI.Runs[0].RequestIDs, projections.RPI.Runs[0].RequestID)
	}
	if len(projections.Dream.Runs) != 1 || projections.Dream.Runs[0].Status != JobStatusRunning {
		t.Fatalf("Dream projection = %#v, want one running run", projections.Dream.Runs)
	}
	if len(projections.Wiki.Jobs) != 1 || projections.Wiki.Jobs[0].Status != JobStatusFailed {
		t.Fatalf("Wiki projection = %#v, want one failed job", projections.Wiki.Jobs)
	}
	if projections.Wiki.Jobs[0].Failure == nil || projections.Wiki.Jobs[0].Failure.Code != FailureProviderUnreachable {
		t.Fatalf("Wiki failure = %#v, want provider_unreachable", projections.Wiki.Jobs[0].Failure)
	}
	if len(projections.OpenClaw.Resources.Runs) != 2 {
		t.Fatalf("OpenClaw runs = %d, want RPI + Dream", len(projections.OpenClaw.Resources.Runs))
	}
	if len(projections.OpenClaw.Resources.Jobs) != 3 {
		t.Fatalf("OpenClaw jobs = %d, want all daemon jobs", len(projections.OpenClaw.Resources.Jobs))
	}
	if len(projections.OpenClaw.Resources.Wiki) != 1 {
		t.Fatalf("OpenClaw wiki = %d, want wiki job", len(projections.OpenClaw.Resources.Wiki))
	}
	if projections.Manifests[ProjectionOpenClaw].Status != ProjectionStatusCurrent {
		t.Fatalf("OpenClaw manifest status = %q, want current", projections.Manifests[ProjectionOpenClaw].Status)
	}
}

func TestProjectionReplayContentAddressedArtifacts(t *testing.T) {
	ref := ArtifactRef{
		Path:      ".agents/handoffs/sha256/aa/bb/" + strings.Repeat("a", 64),
		SHA256:    strings.Repeat("a", 64),
		Size:      42,
		WrittenAt: projectionTestTime(t, 2).Format(time.RFC3339Nano),
	}
	events := []LedgerEvent{
		mustNewProjectionTestEvent(t, "evt-wiki-accepted", "req-wiki-1", "job-wiki", EventJobAccepted, JobTypeWikiForge, 0, nil),
		mustNewProjectionTestEvent(t, "evt-wiki-completed", "req-wiki-2", "job-wiki", EventJobCompleted, "", 1, map[string]any{
			"artifact_refs": map[string]ArtifactRef{"worker_session_refs": ref},
		}),
	}
	projections, err := RebuildProjections(events, ProjectionRebuildOptions{RebuiltAt: projectionTestTime(t, 10)})
	if err != nil {
		t.Fatalf("rebuild projections: %v", err)
	}
	if len(projections.Wiki.Jobs) != 1 {
		t.Fatalf("wiki jobs = %#v", projections.Wiki.Jobs)
	}
	job := projections.Wiki.Jobs[0]
	if got := job.ArtifactRefs["worker_session_refs"]; got != ref {
		t.Fatalf("artifact ref = %#v, want %#v", got, ref)
	}
	if got := job.Artifacts["worker_session_refs"]; got != ref.Path {
		t.Fatalf("compat artifact path = %q, want %q", got, ref.Path)
	}
	if got := projections.OpenClaw.Resources.Wiki[0].ArtifactRefs["worker_session_refs"]; got != ref {
		t.Fatalf("openclaw artifact ref = %#v, want %#v", got, ref)
	}
}

func TestProjectionReplayFromStoreCarriesRequestIDsAndDegradesOnCorruptLedger(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.AppendLedgerEvent(mustNewProjectionTestEvent(t, "evt-rpi-accepted", "req-rpi-1", "job-rpi", EventJobAccepted, JobTypeRPIPhase, 0, nil)); err != nil {
		t.Fatalf("append accepted: %v", err)
	}
	if _, err := store.AppendLedgerEvent(mustNewProjectionTestEvent(t, "evt-rpi-heartbeat", "req-rpi-2", "job-rpi", EventJobHeartbeat, "", 1, nil)); err != nil {
		t.Fatalf("append heartbeat: %v", err)
	}
	file, err := os.OpenFile(store.LedgerPath(), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatalf("open ledger for corrupt fixture append: %v", err)
	}
	if _, err := file.WriteString("{not-json\n"); err != nil {
		t.Fatalf("append corrupt fixture line: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close corrupt fixture file: %v", err)
	}

	projections, err := store.RebuildProjections(ProjectionRebuildOptions{RebuiltAt: projectionTestTime(t, 10)})
	if err != nil {
		t.Fatalf("rebuild projections from store: %v", err)
	}
	if len(projections.RPI.Runs) != 1 {
		t.Fatalf("RPI runs = %d, want 1", len(projections.RPI.Runs))
	}
	run := projections.RPI.Runs[0]
	if run.Status != JobStatusRunning {
		t.Fatalf("run status = %q, want running", run.Status)
	}
	if got := strings.Join(run.RequestIDs, ","); got != "req-rpi-1,req-rpi-2" {
		t.Fatalf("request ids = %q, want req-rpi-1,req-rpi-2", got)
	}
	if projections.Manifests[ProjectionOpenClaw].Status != ProjectionStatusDegraded {
		t.Fatalf("OpenClaw manifest status = %q, want degraded", projections.Manifests[ProjectionOpenClaw].Status)
	}
	if projections.OpenClaw.Status != ProjectionStatusDegraded {
		t.Fatalf("OpenClaw snapshot status = %q, want degraded", projections.OpenClaw.Status)
	}
	if len(projections.DegradedReasons) == 0 {
		t.Fatal("projection set missing degraded reason for corrupt replay")
	}
	if _, err := os.Stat(filepath.Join(store.QuarantineDir(), "ledger-line-000003.json")); err != nil {
		t.Fatalf("corrupt replay did not quarantine line 3: %v", err)
	}
}

func mustNewProjectionTestEvent(t *testing.T, eventID, requestID, jobID string, eventType EventType, jobType JobType, minuteOffset int, payload map[string]any) LedgerEvent {
	t.Helper()
	event, err := NewLedgerEvent(LedgerEventInput{
		EventID:    eventID,
		RequestID:  RequestID(requestID),
		JobID:      jobID,
		EventType:  eventType,
		OccurredAt: projectionTestTime(t, minuteOffset),
		Actor:      "test",
		JobType:    jobType,
		Payload:    payload,
	})
	if err != nil {
		t.Fatalf("new ledger event %s: %v", eventID, err)
	}
	return event
}

func projectionTestTime(t *testing.T, minuteOffset int) time.Time {
	t.Helper()
	base, err := time.Parse(time.RFC3339, fixedLedgerTime)
	if err != nil {
		t.Fatalf("parse fixed ledger time: %v", err)
	}
	return base.Add(time.Duration(minuteOffset) * time.Minute)
}
