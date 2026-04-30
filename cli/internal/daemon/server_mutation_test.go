package daemon

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/cancel", bytes.NewBufferString(payload))
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
