package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// recurrenceE2EToken is the secret used by every test in this file. Tests in
// this file run with the loopback-only mutation policy so authorization
// resolves identically to the production daemon path.
const recurrenceE2EToken = "secret-token"

// e2eTickAt is the deterministic wall-clock time used for tick boundaries.
// It lands exactly on the "*/5 * * * *" cron grid so the supervisor fires
// immediately on the first tick (refreshSchedules uses Next(now-1ns)).
var e2eTickAt = time.Date(2026, 5, 1, 0, 5, 0, 0, time.UTC)

// newE2EHarness wires Store + Queue + RecurrenceSupervisor + an authenticated
// httptest.Server registering the schedules routes. The test's *time.Time
// pointer drives both the queue's clock and the server's clock.
func newE2EHarness(t *testing.T, now *time.Time) (*Store, *Queue, *RecurrenceSupervisor, *httptest.Server) {
	t.Helper()
	store := NewStore(t.TempDir())
	queue := NewQueue(store, QueueOptions{
		LeaseDuration: 5 * time.Minute,
		MaxAttempts:   3,
		Now:           func() time.Time { return *now },
	})
	policy := DefaultMutationPolicy(recurrenceE2EToken, []string{
		"/v1/jobs",
		"/jobs",
		"/v1/schedules",
		"/v1/schedules/*",
	})
	router := NewDaemonRouter(store, ServerOptions{
		Now:            func() time.Time { return *now },
		MutationPolicy: policy,
	})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	sup := NewRecurrenceSupervisor(store, queue, NewFakeClock(*now))
	return store, queue, sup, server
}

// postScheduleE2E POSTs a schedule template to the daemon test server. Empty
// token omits the auth header so the call exercises the unauthorized path.
func postScheduleE2E(t *testing.T, server *httptest.Server, body string, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/schedules", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set(DefaultMutationTokenHeader, token)
	}
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("post schedule: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// deleteScheduleE2E issues DELETE /v1/schedules/{name} against the test server.
func deleteScheduleE2E(t *testing.T, server *httptest.Server, name, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, server.URL+"/v1/schedules/"+name, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if token != "" {
		req.Header.Set(DefaultMutationTokenHeader, token)
	}
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("delete schedule: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// getSchedulesE2E issues a token-less GET to confirm the read-only route.
func getSchedulesE2E(t *testing.T, server *httptest.Server) *http.Response {
	t.Helper()
	resp, err := server.Client().Get(server.URL + "/v1/schedules")
	if err != nil {
		t.Fatalf("get schedules: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// TestRecurrence_E2EHappyPath wires Store + Queue + RecurrenceSupervisor + HTTP
// server and drives a full POST → tick → submit → claim → terminal sequence,
// then advances the FakeClock to the next cron boundary and confirms a second
// job is enqueued (default backpressure allows new submissions even with
// completed prior runs).
func TestRecurrence_E2EHappyPath(t *testing.T) {
	t.Parallel()
	now := e2eTickAt
	store, queue, sup, server := newE2EHarness(t, &now)

	body := `{"name":"wiki-loop","cron":"*/5 * * * *","job_type":"wiki.forge","payload":{"source_paths":[".agents/sessions"],"output_dir":".agents/wiki/forge"}}`
	resp := postScheduleE2E(t, server, body, recurrenceE2EToken)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /v1/schedules status = %d, want 201", resp.StatusCode)
	}

	// Tick exactly at the cron boundary. refreshSchedules computes
	// Next(now-1ns) so a freshly-installed schedule fires immediately.
	if err := sup.tick(context.Background(), now); err != nil {
		t.Fatalf("first tick: %v", err)
	}

	jobs := realQueueJobs(t, queue)
	if len(jobs) != 1 {
		t.Fatalf("after first tick: want 1 job; got %d", len(jobs))
	}
	subID := submissionID("wiki-loop", now)
	if jobs[0].IdempotencyKey != subID {
		t.Fatalf("idempotency key = %q want %q", jobs[0].IdempotencyKey, subID)
	}
	if !ledgerHasFiredEvent(t, store, "wiki-loop", subID) {
		t.Fatalf("ledger missing schedule.fired event for subID=%s", subID)
	}

	// Worker claims and completes the job (terminal write).
	claim, err := queue.ClaimJob(jobs[0].JobID, "test-worker", QueueMutationOptions{})
	if err != nil {
		t.Fatalf("ClaimJob: %v", err)
	}
	if _, err := queue.CompleteJob(CompleteJobInput{
		JobID:      jobs[0].JobID,
		ClaimToken: claim.ClaimToken,
		LeaseEpoch: claim.LeaseEpoch,
		Actor:      "test-worker",
	}, QueueMutationOptions{}); err != nil {
		t.Fatalf("CompleteJob: %v", err)
	}

	// Advance to the next cron tick (5 minutes). Default backpressure has
	// SkipIfRunning=false and MaxQueueDepth=0, so the supervisor must fire
	// a second submission after the prior job reached terminal status.
	now = now.Add(5 * time.Minute)
	if err := sup.tick(context.Background(), now); err != nil {
		t.Fatalf("second tick: %v", err)
	}
	jobs2 := realQueueJobs(t, queue)
	if len(jobs2) != 2 {
		t.Fatalf("after second tick: want 2 jobs total; got %d", len(jobs2))
	}
	subID2 := submissionID("wiki-loop", now)
	if subID2 == subID {
		t.Fatalf("submission_id collided across tick boundaries")
	}
	if !ledgerHasFiredEvent(t, store, "wiki-loop", subID2) {
		t.Fatalf("ledger missing schedule.fired event for second tick subID=%s", subID2)
	}
}

// TestRecurrence_E2EBackpressureSkipsAndResumes wires the same harness with
// SkipIfRunning=true. With an in-flight job the first tick must record a
// skipped event (no new submission). After completing the in-flight job a
// follow-up tick fires normally.
func TestRecurrence_E2EBackpressureSkipsAndResumes(t *testing.T) {
	t.Parallel()
	now := e2eTickAt
	store, queue, sup, server := newE2EHarness(t, &now)

	body := `{"name":"wiki-loop","cron":"*/5 * * * *","job_type":"wiki.forge","payload":{"source_paths":[".agents/sessions"],"output_dir":".agents/wiki/forge"},"backpressure":{"skip_if_running":true}}`
	resp := postScheduleE2E(t, server, body, recurrenceE2EToken)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /v1/schedules status = %d, want 201", resp.StatusCode)
	}

	// Pre-seed a running job tagged with the schedule name. This is what
	// triggers the SkipIfRunning backpressure path.
	preSeedRunningJob(t, queue, "wiki-loop", "preseed-running", JobTypeWikiForge, now)

	if err := sup.tick(context.Background(), now); err != nil {
		t.Fatalf("first tick: %v", err)
	}
	scheduleJobs := 0
	for _, j := range realQueueJobs(t, queue) {
		if jobBelongsToSchedule(j, "wiki-loop") {
			scheduleJobs++
		}
	}
	if scheduleJobs != 1 {
		t.Fatalf("first tick must skip — want 1 schedule-tagged job (preseed); got %d", scheduleJobs)
	}
	reason := lastSkipReason(t, store, "wiki-loop")
	if !contains(reason, "in-flight") {
		t.Fatalf("expected skipped reason containing in-flight; got %q", reason)
	}

	// Complete the in-flight job. Claim with the existing-claim path is not
	// needed: the pre-seed used preSeedRunningJob which already produced a
	// claim. We re-claim via ClaimJob is forbidden, but the snapshot exposed
	// the claim token via the latest event; for this test the simplest path
	// is to fail-out the job to terminal status using a fresh claim token
	// shape — instead, we use the daemon's CancelJob helper to drive the
	// job to terminal (cancelled is a terminal state so backpressure
	// releases).
	if _, err := queue.CancelJob(CancelJobInput{
		JobID:  "preseed-running",
		Actor:  "test",
		Reason: "release-backpressure",
	}, QueueMutationOptions{}); err != nil {
		t.Fatalf("CancelJob: %v", err)
	}

	// Advance to the next cron boundary. With the prior job terminal, the
	// supervisor must fire a fresh submission.
	now = now.Add(5 * time.Minute)
	if err := sup.tick(context.Background(), now); err != nil {
		t.Fatalf("second tick: %v", err)
	}
	subID2 := submissionID("wiki-loop", now)
	if !ledgerHasFiredEvent(t, store, "wiki-loop", subID2) {
		t.Fatalf("ledger missing schedule.fired event after backpressure cleared (subID=%s)", subID2)
	}
}

// TestRecurrence_E2EAuthEnforcedOnSchedulesEndpoint runs the parity test for
// /v1/schedules: POST without token returns 403, POST with token returns 201,
// and GET (read-only) succeeds without a token.
func TestRecurrence_E2EAuthEnforcedOnSchedulesEndpoint(t *testing.T) {
	t.Parallel()
	now := e2eTickAt
	_, _, _, server := newE2EHarness(t, &now)

	body := `{"name":"auth-test","cron":"*/5 * * * *","job_type":"wiki.forge","payload":{"source_paths":[".agents/sessions"],"output_dir":".agents/wiki/forge"}}`

	// 1. POST without token → 403.
	resp := postScheduleE2E(t, server, body, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("unauthorized POST status = %d, want 403", resp.StatusCode)
	}

	// 2. POST with valid token → 201.
	resp = postScheduleE2E(t, server, body, recurrenceE2EToken)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("authorized POST status = %d, want 201", resp.StatusCode)
	}

	// 3. GET without token → 200 (read-only route bypasses mutation auth).
	resp = getSchedulesE2E(t, server)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("token-less GET status = %d, want 200", resp.StatusCode)
	}
	var listed ListSchedulesResponse
	if err := json.NewDecoder(resp.Body).Decode(&listed); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	if len(listed.Schedules) != 1 || listed.Schedules[0].Name != "auth-test" {
		t.Fatalf("listed schedules = %#v, want exactly auth-test", listed.Schedules)
	}

	// 4. DELETE without token → 403 (mutation route).
	resp = deleteScheduleE2E(t, server, "auth-test", "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("unauthorized DELETE status = %d, want 403", resp.StatusCode)
	}
}

// TestRecurrence_E2EDeleteScheduleStopsFiring posts a schedule, ticks once,
// deletes the schedule, advances time, and ticks again. The second tick must
// not fire because refreshSchedules drops deleted schedules from the cache.
func TestRecurrence_E2EDeleteScheduleStopsFiring(t *testing.T) {
	t.Parallel()
	now := e2eTickAt
	store, queue, sup, server := newE2EHarness(t, &now)

	body := `{"name":"ephemeral","cron":"*/5 * * * *","job_type":"wiki.forge","payload":{"source_paths":[".agents/sessions"],"output_dir":".agents/wiki/forge"}}`
	if resp := postScheduleE2E(t, server, body, recurrenceE2EToken); resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST status = %d, want 201", resp.StatusCode)
	}

	if err := sup.tick(context.Background(), now); err != nil {
		t.Fatalf("first tick: %v", err)
	}
	subID1 := submissionID("ephemeral", now)
	if !ledgerHasFiredEvent(t, store, "ephemeral", subID1) {
		t.Fatalf("first tick did not fire schedule")
	}
	if got := len(realQueueJobs(t, queue)); got != 1 {
		t.Fatalf("after first tick: want 1 job; got %d", got)
	}

	// Delete the schedule.
	resp := deleteScheduleE2E(t, server, "ephemeral", recurrenceE2EToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE status = %d, want 200", resp.StatusCode)
	}

	// Advance past the next cron boundary; the supervisor must NOT fire
	// because refreshSchedules drops the now-absent schedule from its cache.
	now = now.Add(5 * time.Minute)
	if err := sup.tick(context.Background(), now); err != nil {
		t.Fatalf("second tick: %v", err)
	}
	subID2 := submissionID("ephemeral", now)
	if ledgerHasFiredEvent(t, store, "ephemeral", subID2) {
		t.Fatalf("second tick fired schedule.fired for deleted schedule (subID=%s)", subID2)
	}
	if got := len(realQueueJobs(t, queue)); got != 1 {
		t.Fatalf("after delete + second tick: want still 1 job; got %d", got)
	}
}
