package daemon

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/openclaw"
)

func TestJobSpecV0GoldenSubmitIsDurableAndIdempotent(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := jobSpecV0MutationRouter(store, &now)
	payload := readJobSpecV0Fixture(t, "submit-request.json")

	resp := postJobSpecV0(t, router, "/v1/jobs", payload, "secret-token")
	if resp.Code != http.StatusAccepted {
		t.Fatalf("POST /v1/jobs status = %d body=%s, want 202", resp.Code, resp.Body.String())
	}
	assertJobSpecV0JSONFixture(t, "submit-response.json", resp.Body.Bytes())

	var body SubmitJobResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode submit response: %v", err)
	}
	if body.LastEventID != "evt_job_accepted_job-golden-rpi_000001" {
		t.Fatalf("last_event_id = %q", body.LastEventID)
	}

	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 1 || events[0].EventType != EventJobAccepted || events[0].JobID != "job-golden-rpi" {
		t.Fatalf("ledger events = %#v, want one accepted job-golden-rpi event", events)
	}
	if got := events[0].Payload["idempotency_key"]; got != "idem-golden-rpi" {
		t.Fatalf("ledger idempotency_key = %#v", got)
	}
	jobPayload, ok := events[0].Payload["job_payload"].(map[string]any)
	if !ok || jobPayload["goal"] != "ship daemon conformance" {
		t.Fatalf("ledger job_payload = %#v", events[0].Payload["job_payload"])
	}

	resp = postJobSpecV0(t, router, "/v1/jobs", payload, "secret-token")
	if resp.Code != http.StatusAccepted {
		t.Fatalf("idempotent retry status = %d body=%s, want 202", resp.Code, resp.Body.String())
	}
	events, err = store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger after retry: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("idempotent retry appended %d events, want still 1", len(events))
	}
}

func TestJobSpecV0StatusAndEventsReflectQueueTransitionReplay(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	queue := NewQueue(store, QueueOptions{Now: func() time.Time { return now }, LeaseDuration: time.Minute})
	if _, err := queue.SubmitJob(SubmitJobInput{
		RequestID:      "req-status-submit",
		JobID:          "job-status-rpi",
		JobType:        JobTypeRPIRun,
		IdempotencyKey: "idem-status-rpi",
		Payload:        map[string]any{"goal": "status replay"},
	}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	claim, err := queue.ClaimJob("job-status-rpi", "worker-golden", QueueMutationOptions{})
	if err != nil {
		t.Fatalf("claim job: %v", err)
	}
	now = now.Add(30 * time.Second)
	if _, err := queue.Heartbeat(HeartbeatInput{
		JobID:      claim.Job.JobID,
		RequestID:  "req-status-heartbeat",
		ClaimToken: claim.ClaimToken,
		LeaseEpoch: claim.LeaseEpoch,
		Actor:      "worker-golden",
		Artifacts:  map[string]string{"heartbeat": "seen"},
	}, QueueMutationOptions{}); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	now = now.Add(30 * time.Second)
	if _, err := queue.CompleteJob(CompleteJobInput{
		JobID:      claim.Job.JobID,
		RequestID:  "req-status-complete",
		ClaimToken: claim.ClaimToken,
		LeaseEpoch: claim.LeaseEpoch,
		Actor:      "worker-golden",
		Artifacts:  map[string]string{"summary": ".agents/rpi/runs/job-status-rpi/summary.md"},
	}, QueueMutationOptions{}); err != nil {
		t.Fatalf("complete job: %v", err)
	}

	router := NewReadOnlyRouter(store, ServerOptions{Now: func() time.Time { return now }})
	var status ReadOnlyStatusResponse
	getJSON(t, router, "/v1/status", &status)
	if !status.Ready || status.ProjectionLag.EventCount != 4 || status.ProjectionLag.Degraded {
		t.Fatalf("status projection = %#v, want ready with four non-degraded events", status.ProjectionLag)
	}
	if len(status.Queue.Jobs) != 1 {
		t.Fatalf("queue jobs = %#v, want one job", status.Queue.Jobs)
	}
	job := status.Queue.Jobs[0]
	if job.Status != JobStatusCompleted || job.LastEventID != status.ProjectionLag.LastEventID {
		t.Fatalf("job status/last event = %#v, lag=%#v", job, status.ProjectionLag)
	}
	if job.Artifacts["summary"] != ".agents/rpi/runs/job-status-rpi/summary.md" || job.Artifacts["heartbeat"] != "seen" {
		t.Fatalf("job artifacts = %#v", job.Artifacts)
	}
	if got := strings.Join(job.RequestIDs, ","); got != "req-status-submit,req-status-heartbeat,req-status-complete" {
		t.Fatalf("request ids = %q", got)
	}

	var events ReadOnlyEventsResponse
	getJSON(t, router, "/v1/events", &events)
	wantTypes := []EventType{EventJobAccepted, EventJobClaimed, EventJobHeartbeat, EventJobCompleted}
	if len(events.Events) != len(wantTypes) {
		t.Fatalf("event count = %d, want %d", len(events.Events), len(wantTypes))
	}
	for i, want := range wantTypes {
		if events.Events[i].EventType != want {
			t.Fatalf("events[%d].event_type = %q, want %q", i, events.Events[i].EventType, want)
		}
	}
	if events.LastEventID != status.ProjectionLag.LastEventID {
		t.Fatalf("events last_event_id = %q, status last_event_id = %q", events.LastEventID, status.ProjectionLag.LastEventID)
	}
}

func TestJobSpecV0CancelOutcomesAreDurableAndTerminalIdempotent(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := jobSpecV0MutationRouter(store, &now)
	submit := []byte(`{"request_id":"req-cancel-submit","job_id":"job-cancel-rpi","job_type":"rpi.run"}`)
	resp := postJobSpecV0(t, router, "/v1/jobs", submit, "secret-token")
	if resp.Code != http.StatusAccepted {
		t.Fatalf("submit status = %d body=%s, want 202", resp.Code, resp.Body.String())
	}

	cancel := []byte(`{"request_id":"req-cancel","job_id":"job-cancel-rpi","reason":"operator requested"}`)
	resp = postJobSpecV0(t, router, "/v1/jobs/cancel", cancel, "secret-token")
	if resp.Code != http.StatusAccepted {
		t.Fatalf("cancel status = %d body=%s, want 202", resp.Code, resp.Body.String())
	}
	var body CancelJobResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode cancel response: %v", err)
	}
	if !body.Cancelled || body.Outcome != CancelJobOutcomeCancelled || body.Job.Status != JobStatusCancelled {
		t.Fatalf("cancel response = %#v", body)
	}
	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 2 || events[1].EventType != EventJobCancelled {
		t.Fatalf("ledger events = %#v, want accepted then cancelled", events)
	}
	if got := events[1].Payload["reason"]; got != "operator requested" {
		t.Fatalf("cancel reason = %#v", got)
	}

	cancelAgain := []byte(`{"request_id":"req-cancel-again","job_id":"job-cancel-rpi","reason":"second request"}`)
	resp = postJobSpecV0(t, router, "/v1/jobs/cancel", cancelAgain, "secret-token")
	if resp.Code != http.StatusAccepted {
		t.Fatalf("repeat cancel status = %d body=%s, want 202", resp.Code, resp.Body.String())
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode repeat cancel response: %v", err)
	}
	if !body.Cancelled || body.Outcome != CancelJobOutcomeAlreadyTerminalCancelled {
		t.Fatalf("repeat cancel response = %#v", body)
	}
	events, err = store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger after repeat cancel: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("repeat cancel appended %d events, want still 2", len(events))
	}
}

func TestJobSpecV0LeaseExpiryRetryWaitingAndRetryExhaustion(t *testing.T) {
	now := projectionTestTime(t, 0)
	queue := newTestQueue(t, &now, QueueOptions{LeaseDuration: time.Minute, MaxAttempts: 2})
	if _, err := queue.SubmitJob(SubmitJobInput{RequestID: "req-retry-submit", JobID: "job-retry", JobType: JobTypeWikiForge}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	first, err := queue.ClaimJob("job-retry", "worker-1", QueueMutationOptions{})
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if first.Job.Attempt != 1 || first.Job.Status != JobStatusRunning {
		t.Fatalf("first claim = %#v, want attempt 1 running", first.Job)
	}

	now = now.Add(2 * time.Minute)
	snapshot, err := queue.Snapshot()
	if err != nil {
		t.Fatalf("snapshot after first expiry: %v", err)
	}
	expired, err := snapshot.jobByID("job-retry")
	if err != nil {
		t.Fatalf("lookup expired job: %v", err)
	}
	if expired.Status != JobStatusRetryWaiting || expired.RetryExhausted {
		t.Fatalf("expired job = %#v, want retry_waiting without exhaustion", expired)
	}
	second, err := queue.ClaimJob("job-retry", "worker-2", QueueMutationOptions{})
	if err != nil {
		t.Fatalf("second claim: %v", err)
	}
	if second.Job.Attempt != 2 || second.Job.LeaseEpoch != 2 || second.ClaimToken == first.ClaimToken {
		t.Fatalf("second claim = %#v, first=%#v", second, first)
	}

	now = now.Add(2 * time.Minute)
	snapshot, err = queue.Snapshot()
	if err != nil {
		t.Fatalf("snapshot after retry cap expiry: %v", err)
	}
	capped, err := snapshot.jobByID("job-retry")
	if err != nil {
		t.Fatalf("lookup capped job: %v", err)
	}
	if capped.Status != JobStatusRetryWaiting || !capped.RetryExhausted {
		t.Fatalf("capped job before final claim = %#v, want retry_waiting retry_exhausted", capped)
	}
	if _, err := queue.ClaimNext("worker-3", QueueMutationOptions{}); !errors.Is(err, ErrNoClaimableJobs) {
		t.Fatalf("claim after retry cap error = %v, want ErrNoClaimableJobs", err)
	}
	snapshot, err = queue.Snapshot()
	if err != nil {
		t.Fatalf("snapshot after retry exhausted append: %v", err)
	}
	failed, err := snapshot.jobByID("job-retry")
	if err != nil {
		t.Fatalf("lookup failed job: %v", err)
	}
	if failed.Status != JobStatusFailed || failed.Failure == nil || failed.Failure.Code != FailureRetryExhausted {
		t.Fatalf("failed job = %#v, want retry_exhausted failure", failed)
	}
}

func TestJobSpecV0OpenClawTriggerIsAllowlistedMutationSurface(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := jobSpecV0MutationRouter(store, &now)

	allowed := []byte(`{"request_id":"req-oc","job_id":"job-oc","job_type":"openclaw.snapshot","idempotency_key":"oc-refresh","payload":{"reason":"refresh"}}`)
	resp := postJobSpecV0(t, router, openclaw.TriggerJobsPath, allowed, "secret-token")
	if resp.Code != http.StatusAccepted {
		t.Fatalf("allowlisted trigger status = %d body=%s, want 202", resp.Code, resp.Body.String())
	}
	var accepted openclaw.TriggerJobResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &accepted); err != nil {
		t.Fatalf("decode trigger response: %v", err)
	}
	if !accepted.Accepted || accepted.JobType != string(JobTypeOpenClawSnapshot) || accepted.Status != string(JobStatusQueued) {
		t.Fatalf("trigger response = %#v", accepted)
	}

	blocked := []byte(`{"request_id":"req-rpi-phase","job_id":"job-rpi-phase","job_type":"rpi.phase"}`)
	resp = postJobSpecV0(t, router, openclaw.TriggerJobsPath, blocked, "secret-token")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("non-allowlisted trigger status = %d body=%s, want 400", resp.Code, resp.Body.String())
	}
	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 1 || events[0].JobID != "job-oc" {
		t.Fatalf("trigger ledger events = %#v, want only job-oc accepted event", events)
	}
}

func jobSpecV0MutationRouter(store *Store, now *time.Time) http.Handler {
	return NewDaemonRouter(store, ServerOptions{
		Now:            func() time.Time { return *now },
		MutationPolicy: DefaultMutationPolicy("secret-token", nil),
	})
}

func postJobSpecV0(t *testing.T, handler http.Handler, target string, payload []byte, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, bytes.NewReader(payload))
	req.RemoteAddr = "127.0.0.1:51111"
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set(DefaultMutationTokenHeader, token)
	}
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}

func readJobSpecV0Fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/jobspec-v0/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func assertJobSpecV0JSONFixture(t *testing.T, name string, got []byte) {
	t.Helper()
	var wantValue any
	if err := json.Unmarshal(readJobSpecV0Fixture(t, name), &wantValue); err != nil {
		t.Fatalf("decode fixture %s: %v", name, err)
	}
	var gotValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("decode actual JSON for %s: %v\nbody=%s", name, err, string(got))
	}
	if !reflect.DeepEqual(wantValue, gotValue) {
		wantPretty, _ := json.MarshalIndent(wantValue, "", "  ")
		gotPretty, _ := json.MarshalIndent(gotValue, "", "  ")
		t.Fatalf("JSON mismatch for %s\nwant:\n%s\ngot:\n%s", name, wantPretty, gotPretty)
	}
}
