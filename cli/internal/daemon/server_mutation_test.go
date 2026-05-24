package daemon

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMutationRouteRequiresPolicy(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := NewDaemonRouter(store, ServerOptions{Now: func() time.Time { return now }})
	resp := postJob(t, router, `{"request_id":"req-1","job_id":"job-rpi","job_type":"rpi.run"}`, "", "")
	if resp.Code != http.StatusForbidden {
		t.Fatalf("unauthorized POST status = %d, want 403", resp.Code)
	}
	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("unauthorized mutation wrote %d events, want 0", len(events))
	}
}

func TestMutationRouteAcceptsJobAfterLedgerAppend(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := mutationRouter(t, store, &now)
	resp := postJob(t, router, `{"request_id":"req-1","job_id":"job-rpi","job_type":"rpi.run","idempotency_key":"idem-1","payload":{"goal":"daemon"}}`, "secret-token", "")
	if resp.Code != http.StatusAccepted {
		t.Fatalf("authorized POST status = %d body=%s, want 202", resp.Code, resp.Body.String())
	}
	var body SubmitJobResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode mutation response: %v", err)
	}
	if !body.Accepted || body.JobID != "job-rpi" || body.RequestID != "req-1" {
		t.Fatalf("mutation response = %#v, want accepted job-rpi req-1", body)
	}
	if body.ProjectionStatus != ProjectionStatusCurrent || body.ProjectionLag.EventCount != 1 {
		t.Fatalf("projection ack fields = %#v, want current lag=1", body)
	}
	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 1 || events[0].EventType != EventJobAccepted {
		t.Fatalf("ledger events = %#v, want one accepted event", events)
	}
}

func TestMutationSubmitInvalidJobTypeReturnsBadRequest(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := mutationRouter(t, store, &now)
	resp := postJob(t, router, `{"request_id":"req-bad","job_id":"job-bad","job_type":"not-a-job"}`, "secret-token", "")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("invalid job_type status = %d body=%s, want 400", resp.Code, resp.Body.String())
	}
	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("invalid job_type wrote %d events, want 0", len(events))
	}
}

func TestMutationRouteAuditsScopedTokenName(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	policy := DefaultMutationPolicy("", []string{"/v1/jobs", "/v1/jobs/cancel"})
	policy.Tokens = []MutationToken{{
		Name:         "phone-readonly-submit",
		Token:        "phone-token",
		Capabilities: []MutationCapability{MutationCapabilitySubmitJob},
	}}
	router := mutationRouterWithPolicy(store, &now, policy)
	resp := postJob(t, router, `{"request_id":"req-1","job_id":"job-rpi","job_type":"rpi.run"}`, "phone-token", "")
	if resp.Code != http.StatusAccepted {
		t.Fatalf("scoped submit status = %d body=%s, want 202", resp.Code, resp.Body.String())
	}
	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 1 || events[0].Actor != "ao-http:phone-readonly-submit" {
		t.Fatalf("ledger events = %#v, want scoped actor", events)
	}
	resp = postCancel(t, router, `{"request_id":"req-cancel","job_id":"job-rpi"}`, "phone-token", "")
	if resp.Code != http.StatusForbidden {
		t.Fatalf("scoped cancel status = %d body=%s, want 403", resp.Code, resp.Body.String())
	}
}

func TestMutationCancelValidationStatusCodes(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	policy := DefaultMutationPolicy("secret-token", []string{
		"/v1/jobs/cancel",
	})
	router := mutationRouterWithPolicy(store, &now, policy)

	resp := postCancel(t, router, `{"request_id":"req-missing"}`, "secret-token", "")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("missing job_id status = %d body=%s, want 400", resp.Code, resp.Body.String())
	}
	resp = postCancel(t, router, `{"request_id":"req-missing-job","job_id":"does-not-exist"}`, "secret-token", "")
	if resp.Code != http.StatusNotFound {
		t.Fatalf("missing job status = %d body=%s, want 404", resp.Code, resp.Body.String())
	}
}

func TestMutationCancelRESTAliasUsesPathJobID(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	queue := NewQueue(store, QueueOptions{Now: func() time.Time { return now }, LeaseDuration: time.Minute})
	if _, err := queue.SubmitJob(SubmitJobInput{RequestID: "req-submit", JobID: "job-rest-cancel", JobType: JobTypeRPIRun}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	policy := DefaultMutationPolicy("secret-token", []string{
		"/v1/jobs/*/cancel",
	})
	router := mutationRouterWithPolicy(store, &now, policy)

	resp := postCancelAt(t, router, "/v1/jobs/job-rest-cancel/cancel", "", "secret-token", "")
	if resp.Code != http.StatusAccepted {
		t.Fatalf("REST cancel alias status = %d body=%s, want 202", resp.Code, resp.Body.String())
	}
	var body CancelJobResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode cancel response: %v", err)
	}
	if !body.Cancelled || body.Job.JobID != "job-rest-cancel" || body.Job.Status != JobStatusCancelled {
		t.Fatalf("REST cancel response = %#v, want cancelled job-rest-cancel", body)
	}
}

func TestMutationCancelRESTAliasRejectsBodyJobIDMismatch(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	policy := DefaultMutationPolicy("secret-token", []string{
		"/v1/jobs/*/cancel",
	})
	router := mutationRouterWithPolicy(store, &now, policy)

	resp := postCancelAt(t, router, "/v1/jobs/job-rest-cancel/cancel", `{"job_id":"other-job"}`, "secret-token", "")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("mismatched REST cancel status = %d body=%s, want 400", resp.Code, resp.Body.String())
	}
}

func TestMutationAckFailpointBeforeAppendNoSideEffect(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := mutationRouter(t, store, &now)
	resp := postJob(t, router, `{"request_id":"req-1","job_id":"job-rpi","job_type":"rpi.run"}`, "secret-token", string(QueueFailpointBeforeAppend))
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("before-append failpoint status = %d body=%s, want 503", resp.Code, resp.Body.String())
	}
	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("before-append failpoint wrote %d events, want 0", len(events))
	}
}

func TestMutationAckFailpointAfterAppendBeforeAckRecoverable(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := mutationRouter(t, store, &now)
	payload := `{"request_id":"req-1","job_id":"job-rpi","job_type":"rpi.run","idempotency_key":"idem-ack"}`
	resp := postJob(t, router, payload, "secret-token", string(QueueFailpointAfterAppendBeforeAck))
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("after-append failpoint status = %d body=%s, want 503", resp.Code, resp.Body.String())
	}
	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger after lost ack: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("after-append failpoint wrote %d events, want durable accepted event", len(events))
	}

	resp = postJob(t, router, payload, "secret-token", "")
	if resp.Code != http.StatusAccepted {
		t.Fatalf("retry after lost ack status = %d body=%s, want 202", resp.Code, resp.Body.String())
	}
	events, err = store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger after retry: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("idempotent retry appended %d events, want still 1", len(events))
	}
}

func TestMutationRequestIDIsTraceOnlyWithoutIdempotencyKey(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := mutationRouter(t, store, &now)
	payload := `{"request_id":"req-reused","job_type":"rpi.run","payload":{"goal":"daemon"}}`

	first := postJob(t, router, payload, "secret-token", "")
	if first.Code != http.StatusAccepted {
		t.Fatalf("first submit status = %d body=%s, want 202", first.Code, first.Body.String())
	}
	second := postJob(t, router, payload, "secret-token", "")
	if second.Code != http.StatusAccepted {
		t.Fatalf("second submit status = %d body=%s, want 202", second.Code, second.Body.String())
	}

	var firstBody, secondBody SubmitJobResponse
	if err := json.Unmarshal(first.Body.Bytes(), &firstBody); err != nil {
		t.Fatalf("decode first response: %v", err)
	}
	if err := json.Unmarshal(second.Body.Bytes(), &secondBody); err != nil {
		t.Fatalf("decode second response: %v", err)
	}
	if firstBody.JobID == "" || secondBody.JobID == "" || firstBody.JobID == secondBody.JobID {
		t.Fatalf("request_id deduped unexpectedly: first=%#v second=%#v", firstBody, secondBody)
	}
	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("reused request_id wrote %d events, want 2 distinct accepted jobs", len(events))
	}
}

func TestMutationIdempotencyKeyDedupsRetryWithoutJobID(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := mutationRouter(t, store, &now)
	firstPayload := `{"request_id":"req-first","job_type":"rpi.run","idempotency_key":"idem-retry","payload":{"goal":"daemon"}}`
	secondPayload := `{"request_id":"req-second","job_type":"rpi.run","idempotency_key":"idem-retry","payload":{"goal":"daemon"}}`

	first := postJob(t, router, firstPayload, "secret-token", "")
	if first.Code != http.StatusAccepted {
		t.Fatalf("first submit status = %d body=%s, want 202", first.Code, first.Body.String())
	}
	second := postJob(t, router, secondPayload, "secret-token", "")
	if second.Code != http.StatusAccepted {
		t.Fatalf("second submit status = %d body=%s, want 202", second.Code, second.Body.String())
	}

	var firstBody, secondBody SubmitJobResponse
	if err := json.Unmarshal(first.Body.Bytes(), &firstBody); err != nil {
		t.Fatalf("decode first response: %v", err)
	}
	if err := json.Unmarshal(second.Body.Bytes(), &secondBody); err != nil {
		t.Fatalf("decode second response: %v", err)
	}
	if firstBody.JobID == "" || secondBody.JobID != firstBody.JobID {
		t.Fatalf("idempotency key did not dedup retry: first=%#v second=%#v", firstBody, secondBody)
	}
	if secondBody.IdempotencyKey != "idem-retry" {
		t.Fatalf("second idempotency_key = %q, want idem-retry", secondBody.IdempotencyKey)
	}
	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("idempotent retry appended %d events, want still 1", len(events))
	}
}

func TestMutationProjectionFailpointStillAcknowledgesAcceptedJob(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := mutationRouter(t, store, &now)
	resp := postJob(t, router, `{"request_id":"req-1","job_id":"job-rpi","job_type":"rpi.run"}`, "secret-token", "projection_rebuild")
	if resp.Code != http.StatusAccepted {
		t.Fatalf("projection failpoint status = %d body=%s, want 202", resp.Code, resp.Body.String())
	}
	var body SubmitJobResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode mutation response: %v", err)
	}
	if body.ProjectionStatus != ProjectionStatusDegraded {
		t.Fatalf("projection status = %q, want degraded", body.ProjectionStatus)
	}
	if len(body.DegradedReasons) == 0 {
		t.Fatalf("missing degraded reason in projection failpoint response: %#v", body)
	}
	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("projection failpoint wrote %d events, want accepted ledger event", len(events))
	}
}

func mutationRouter(t *testing.T, store *Store, now *time.Time) http.Handler {
	t.Helper()
	return mutationRouterWithPolicy(store, now, DefaultMutationPolicy("secret-token", []string{
		"/v1/jobs",
		"/jobs",
	}))
}

func mutationRouterWithPolicy(store *Store, now *time.Time, policy MutationPolicy) http.Handler {
	return NewDaemonRouter(store, ServerOptions{
		Now:            func() time.Time { return *now },
		MutationPolicy: policy,
	})
}

func postJob(t *testing.T, handler http.Handler, payload, token, failpoint string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs", bytes.NewBufferString(payload))
	req.RemoteAddr = "127.0.0.1:51111"
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set(DefaultMutationTokenHeader, token)
	}
	if failpoint != "" {
		req.Header.Set("X-AgentOps-Failpoint", failpoint)
	}
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}

func postCancel(t *testing.T, handler http.Handler, payload, token, failpoint string) *httptest.ResponseRecorder {
	t.Helper()
	return postCancelAt(t, handler, "/v1/jobs/cancel", payload, token, failpoint)
}

func postCancelAt(t *testing.T, handler http.Handler, path, payload, token, failpoint string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(payload))
	req.RemoteAddr = "127.0.0.1:51111"
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set(DefaultMutationTokenHeader, token)
	}
	if failpoint != "" {
		req.Header.Set("X-AgentOps-Failpoint", failpoint)
	}
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}

// TestSubmitJobRejectsBodyOverMaxBytes asserts that a POST larger than
// MaxJobSubmissionBytes is short-circuited to 413 before any ledger write
// happens. Regression for the daemon crash where an unbounded POST body
// landed in ledger.jsonl as a multi-megabyte event line, then panicked the
// daemon at next replay with "bufio.Scanner: token too long".
func TestSubmitJobRejectsBodyOverMaxBytes(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := mutationRouter(t, store, &now)

	// Build a valid-shape submission with a goal-string padded past the cap.
	prefix := `{"request_id":"req-big","job_id":"job-big","job_type":"dream.run","payload":{"schema_version":1,"job_type":"dream.run","dream_run_id":"big","mode":"daemon","output_dir":".x","goal":"`
	suffix := `"}}`
	pad := bytes.Repeat([]byte("x"), MaxJobSubmissionBytes+1024)
	body := prefix + string(pad) + suffix

	resp := postJob(t, router, body, "secret-token", "")
	if resp.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized POST status = %d, want 413; body=%s", resp.Code, resp.Body.String())
	}
	var responseBody map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &responseBody); err != nil {
		t.Fatalf("decode 413 body: %v", err)
	}
	if got := responseBody["max_bytes"]; got == nil {
		t.Fatalf("413 body missing max_bytes hint: %s", resp.Body.String())
	}

	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("oversized POST wrote %d events, want 0 (size-cap must short-circuit before ledger append)", len(events))
	}
}

// TestCancelJobRejectsBodyOverMaxBytes mirrors the submit-side cap on the
// cancel route — the cap lives on every mutation entry-point because any
// of them can append to the ledger.
func TestCancelJobRejectsBodyOverMaxBytes(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	policy := DefaultMutationPolicy("secret-token", []string{
		"/v1/jobs",
		"/v1/jobs/cancel",
		"/jobs",
		"/jobs/cancel",
	})
	router := mutationRouterWithPolicy(store, &now, policy)

	prefix := `{"request_id":"req-big","job_id":"job-big","reason":"`
	suffix := `"}`
	pad := bytes.Repeat([]byte("x"), MaxJobSubmissionBytes+1024)
	body := prefix + string(pad) + suffix

	resp := postCancel(t, router, body, "secret-token", "")
	if resp.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized cancel status = %d, want 413; body=%s", resp.Code, resp.Body.String())
	}
}

// TestSubmitJobRejectsOverlongIdempotencyKey asserts that an idempotency key
// longer than MaxIdempotencyKeyBytes is rejected with 400 before any ledger
// write. Without the bound an unbounded key is stored verbatim in the
// JobAccepted ledger event payload and echoed into every job projection,
// bloating the append-only ledger for the job's lifetime (soc-scg3h).
func TestSubmitJobRejectsOverlongIdempotencyKey(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := mutationRouter(t, store, &now)

	// Key one byte over the cap; the body itself stays well under
	// MaxJobSubmissionBytes so this exercises the key bound, not the body cap.
	overlong := strings.Repeat("k", MaxIdempotencyKeyBytes+1)
	payload := `{"request_id":"req-idem","job_id":"job-idem","job_type":"rpi.run","idempotency_key":"` + overlong + `"}`

	resp := postJob(t, router, payload, "secret-token", "")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("overlong idempotency key status = %d, want 400; body=%s", resp.Code, resp.Body.String())
	}
	var responseBody map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &responseBody); err != nil {
		t.Fatalf("decode 400 body: %v", err)
	}
	if got, ok := responseBody["max_bytes"].(float64); !ok || int(got) != MaxIdempotencyKeyBytes {
		t.Fatalf("400 body max_bytes = %v, want %d", responseBody["max_bytes"], MaxIdempotencyKeyBytes)
	}

	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("overlong idempotency key wrote %d events, want 0 (bound must reject before ledger append)", len(events))
	}
}

// TestSubmitJobAcceptsMaxLengthIdempotencyKey is the boundary companion: a key
// exactly at MaxIdempotencyKeyBytes must still be accepted (202) and append one
// ledger event, so the bound is behavior-preserving for valid requests.
func TestSubmitJobAcceptsMaxLengthIdempotencyKey(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := mutationRouter(t, store, &now)

	maxKey := strings.Repeat("k", MaxIdempotencyKeyBytes)
	payload := `{"request_id":"req-idem-ok","job_id":"job-idem-ok","job_type":"rpi.run","idempotency_key":"` + maxKey + `"}`

	resp := postJob(t, router, payload, "secret-token", "")
	if resp.Code != http.StatusAccepted {
		t.Fatalf("max-length idempotency key status = %d, want 202; body=%s", resp.Code, resp.Body.String())
	}
	var body SubmitJobResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode 202 body: %v", err)
	}
	if !body.Accepted || body.IdempotencyKey != maxKey {
		t.Fatalf("response = %#v, want accepted with the max-length key echoed back", body)
	}

	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 1 || events[0].EventType != EventJobAccepted {
		t.Fatalf("ledger events = %#v, want one accepted event", events)
	}
}

// TestOpenClawTriggerRejectsOverlongIdempotencyKey mirrors the submit-side key
// bound on the OpenClaw trigger route — both accept caller-supplied keys that
// flow into the ledger, so the bound lives on both entry-points (soc-scg3h).
func TestOpenClawTriggerRejectsOverlongIdempotencyKey(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := openClawTriggerRouter(t, store, &now)

	overlong := strings.Repeat("k", MaxIdempotencyKeyBytes+1)
	payload := `{"request_id":"req-oc","job_id":"job-oc","job_type":"openclaw.snapshot","idempotency_key":"` + overlong + `"}`

	resp := postOpenClawTrigger(t, router, payload, "secret-token", "")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("overlong trigger idempotency key status = %d, want 400; body=%s", resp.Code, resp.Body.String())
	}
	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("overlong trigger idempotency key wrote %d events, want 0", len(events))
	}
}

// TestPostScheduleRejectsBodyOverMaxBytes asserts the schedule create route
// shares the body-size cap with the other mutation routes: an oversized body
// must return 413, not be fully buffered and decoded into a 500/OOM. Regression
// for the one decode path that previously lacked MaxBytesReader (soc-scg3h).
func TestPostScheduleRejectsBodyOverMaxBytes(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := schedulesRouter(t, store, &now)

	// Valid-shaped template with the cron string padded past the cap.
	prefix := `{"name":"big","job_type":"rpi.run","cron":"`
	suffix := `"}`
	pad := strings.Repeat("x", MaxJobSubmissionBytes+1024)
	body := prefix + pad + suffix

	resp := postSchedule(t, router, body, "secret-token")
	if resp.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized schedule status = %d, want 413 (not 500); body=%s", resp.Code, resp.Body.String())
	}
	var responseBody map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &responseBody); err != nil {
		t.Fatalf("decode 413 body: %v", err)
	}
	if got := responseBody["max_bytes"]; got == nil {
		t.Fatalf("413 body missing max_bytes hint: %s", resp.Body.String())
	}
}
