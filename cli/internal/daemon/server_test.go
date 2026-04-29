package daemon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/openclaw"
)

func TestReadOnlyHealthReadyStatusEvents(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	queue := NewQueue(store, QueueOptions{Now: func() time.Time { return now }, LeaseDuration: time.Minute})
	if _, err := queue.SubmitJob(SubmitJobInput{RequestID: "req-submit", JobID: "job-rpi", JobType: JobTypeRPIRun}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	if _, err := queue.ClaimJob("job-rpi", "worker-1", QueueMutationOptions{}); err != nil {
		t.Fatalf("claim job: %v", err)
	}
	router := NewReadOnlyRouter(store, ServerOptions{Now: func() time.Time { return now }})

	var health ReadOnlyHealthResponse
	getJSON(t, router, "/health", &health)
	if health.Status != "ok" || !health.ReadOnly {
		t.Fatalf("health = %#v, want ok read-only", health)
	}

	var ready ReadOnlyReadyResponse
	getJSON(t, router, "/v1/ready", &ready)
	if !ready.Ready || ready.ProjectionLag.EventCount != 2 || ready.ProjectionLag.Degraded {
		t.Fatalf("ready = %#v, want ready with two events and no degraded flag", ready)
	}

	var status ReadOnlyStatusResponse
	getJSON(t, router, "/status", &status)
	if len(status.Queue.Jobs) != 1 || status.Queue.Jobs[0].Status != JobStatusRunning {
		t.Fatalf("status queue = %#v, want one running job", status.Queue.Jobs)
	}
	if len(status.Projections.RPI.Runs) != 1 {
		t.Fatalf("status RPI projection = %#v, want one run", status.Projections.RPI.Runs)
	}

	var events ReadOnlyEventsResponse
	getJSON(t, router, "/events", &events)
	if len(events.Events) != 2 || events.LastEventID == "" {
		t.Fatalf("events = %#v, want two events and last event id", events)
	}
}

func TestReadOnlyRoutesExposeDegradedStateWithoutQuarantineSideEffects(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	if err := os.MkdirAll(store.Dir(), 0700); err != nil {
		t.Fatalf("create store dir: %v", err)
	}
	valid := mustNewProjectionTestEvent(t, "evt-rpi-accepted", "req-rpi", "job-rpi", EventJobAccepted, JobTypeRPIRun, 0, nil)
	data := strings.Join([]string{mustLedgerLine(t, valid), "{not-json", ""}, "\n")
	if err := os.WriteFile(store.LedgerPath(), []byte(data), 0600); err != nil {
		t.Fatalf("write corrupt ledger fixture: %v", err)
	}
	router := NewReadOnlyRouter(store, ServerOptions{Now: func() time.Time { return now }})

	var ready ReadOnlyReadyResponse
	getJSON(t, router, "/ready", &ready)
	if ready.Ready {
		t.Fatalf("ready = true for corrupt replay: %#v", ready)
	}
	if ready.LedgerReplayStatus != SnapshotReplayCorrupt || ready.ProjectionStatus != ProjectionStatusDegraded {
		t.Fatalf("ready statuses = %#v, want corrupt/degraded", ready)
	}
	if _, err := os.Stat(store.QuarantineDir()); !os.IsNotExist(err) {
		t.Fatalf("read-only route created quarantine dir or unexpected stat error: %v", err)
	}
}

func TestReadOnlyRouterHasNoMutationEndpoints(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := NewReadOnlyRouter(store, ServerOptions{Now: func() time.Time { return now }})
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs", strings.NewReader(`{"job_type":"rpi.run"}`))
	req.Header.Set(DefaultMutationTokenHeader, "secret-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("POST /v1/jobs status = %d, want 404 absent mutation endpoint", resp.Code)
	}
	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("mutation route side effect wrote %d events, want 0", len(events))
	}
}

func TestEventsReadOnlyRejectsPost(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := NewReadOnlyRouter(store, ServerOptions{Now: func() time.Time { return now }})
	req := httptest.NewRequest(http.MethodPost, "/events", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /events status = %d, want 405", resp.Code)
	}
}

func TestOpenClawReadOnlyEndpoints(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	queue := NewQueue(store, QueueOptions{Now: func() time.Time { return now }, LeaseDuration: time.Minute})
	if _, err := queue.SubmitJob(SubmitJobInput{RequestID: "req-rpi", JobID: "job-rpi", JobType: JobTypeRPIRun}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit rpi job: %v", err)
	}
	if _, err := queue.SubmitJob(SubmitJobInput{RequestID: "req-dream", JobID: "job-dream", JobType: JobTypeDreamRun}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit dream job: %v", err)
	}
	if _, err := queue.SubmitJob(SubmitJobInput{RequestID: "req-wiki", JobID: "job-wiki", JobType: JobTypeWikiForge}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit wiki job: %v", err)
	}
	claim, err := queue.ClaimJob("job-wiki", "wiki-worker", QueueMutationOptions{})
	if err != nil {
		t.Fatalf("claim wiki job: %v", err)
	}
	if _, err := queue.CompleteJob(CompleteJobInput{
		JobID:      "job-wiki",
		RequestID:  "req-wiki-complete",
		ClaimToken: claim.ClaimToken,
		LeaseEpoch: claim.LeaseEpoch,
		Actor:      "wiki-worker",
		Artifacts:  map[string]string{"worker_session_refs": ".agents/daemon/wiki/job-wiki-worker-sessions.json"},
	}, QueueMutationOptions{}); err != nil {
		t.Fatalf("complete wiki job: %v", err)
	}
	router := NewReadOnlyRouter(store, ServerOptions{Now: func() time.Time { return now }})

	var snapshot openclaw.ConsumerSnapshot
	getJSON(t, router, "/openclaw/v1/snapshot/latest", &snapshot)
	if snapshot.SchemaVersion != openclaw.ConsumerSnapshotSchemaVersion {
		t.Fatalf("snapshot schema = %d", snapshot.SchemaVersion)
	}
	if snapshot.SnapshotID == "snap_empty" || snapshot.Source.LastEventID == "" {
		t.Fatalf("snapshot source = %#v id=%q", snapshot.Source, snapshot.SnapshotID)
	}
	if len(snapshot.Resources.Runs) != 2 {
		t.Fatalf("snapshot runs = %d, want rpi + dream", len(snapshot.Resources.Runs))
	}
	if len(snapshot.Resources.Jobs) != 3 {
		t.Fatalf("snapshot jobs = %d, want all jobs", len(snapshot.Resources.Jobs))
	}
	if len(snapshot.Resources.Wiki) != 1 || snapshot.Resources.Wiki[0].ResourceKind != openclaw.ResourceKindWiki {
		t.Fatalf("snapshot wiki = %#v", snapshot.Resources.Wiki)
	}
	if !hasOpenClawProvenance(snapshot.Resources.Wiki[0].Provenance, "source-event", "daemon-ledger-event", "") {
		t.Fatalf("wiki missing source-event provenance: %#v", snapshot.Resources.Wiki[0].Provenance)
	}
	if !hasOpenClawProvenance(snapshot.Resources.Wiki[0].Provenance, "artifact", "artifact", "worker_session_refs") {
		t.Fatalf("wiki missing artifact provenance: %#v", snapshot.Resources.Wiki[0].Provenance)
	}

	var runs openclaw.RunsResponse
	getJSON(t, router, "/openclaw/v1/runs", &runs)
	if len(runs.Runs) != 2 || runs.Runs[0].ResourceKind != openclaw.ResourceKindRun {
		t.Fatalf("runs response = %#v", runs)
	}
	var jobs openclaw.JobsResponse
	getJSON(t, router, "/openclaw/v1/jobs", &jobs)
	if len(jobs.Jobs) != 3 || jobs.Jobs[0].ResourceKind != openclaw.ResourceKindJob {
		t.Fatalf("jobs response = %#v", jobs)
	}
	var wiki openclaw.WikiResponse
	getJSON(t, router, "/openclaw/v1/wiki", &wiki)
	if len(wiki.Wiki) != 1 || wiki.Wiki[0].JobID != "job-wiki" {
		t.Fatalf("wiki response = %#v", wiki)
	}
	var health openclaw.HealthResponse
	getJSON(t, router, "/openclaw/v1/health", &health)
	if health.Status != "ok" || !health.Ready || health.ResourceCounts.Jobs != 3 {
		t.Fatalf("health response = %#v", health)
	}
}

func TestOpenClawJobsReflectTerminalStatus(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	queue := NewQueue(store, QueueOptions{Now: func() time.Time { return now }, LeaseDuration: time.Minute})
	if _, err := queue.SubmitJob(SubmitJobInput{
		RequestID: "req-openclaw-terminal",
		JobID:     "job-openclaw-terminal",
		JobType:   JobTypeOpenClawSnapshot,
	}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit openclaw job: %v", err)
	}
	claim, err := queue.ClaimJob("job-openclaw-terminal", "worker", QueueMutationOptions{})
	if err != nil {
		t.Fatalf("claim openclaw job: %v", err)
	}
	completed, err := queue.CompleteJob(CompleteJobInput{
		JobID:      claim.Job.JobID,
		RequestID:  "req-openclaw-terminal-complete",
		ClaimToken: claim.ClaimToken,
		LeaseEpoch: claim.LeaseEpoch,
		Actor:      "worker",
		Artifacts:  map[string]string{"snapshot_status": "validated"},
	}, QueueMutationOptions{})
	if err != nil {
		t.Fatalf("complete openclaw job: %v", err)
	}
	router := NewReadOnlyRouter(store, ServerOptions{Now: func() time.Time { return now }})

	var jobs openclaw.JobsResponse
	getJSON(t, router, "/openclaw/v1/jobs", &jobs)
	if len(jobs.Jobs) != 1 {
		t.Fatalf("OpenClaw jobs = %#v, want one job", jobs.Jobs)
	}
	if jobs.Jobs[0].JobID != completed.JobID || jobs.Jobs[0].Status != string(completed.Status) {
		t.Fatalf("OpenClaw job = %#v, want status %s for %s", jobs.Jobs[0], completed.Status, completed.JobID)
	}
	if jobs.Jobs[0].Artifacts["snapshot_status"] != "validated" {
		t.Fatalf("OpenClaw artifacts = %#v, want terminal artifacts", jobs.Jobs[0].Artifacts)
	}
}

func hasOpenClawProvenance(links []openclaw.ProvenanceLink, rel, kind, artifact string) bool {
	for _, link := range links {
		if link.Rel != rel || link.Kind != kind {
			continue
		}
		if artifact != "" && link.Artifact != artifact {
			continue
		}
		return true
	}
	return false
}

func TestOpenClawReadOnlyEndpointsRejectMutationMethods(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := NewReadOnlyRouter(store, ServerOptions{Now: func() time.Time { return now }})
	req := httptest.NewRequest(http.MethodPost, "/openclaw/v1/jobs", strings.NewReader(`{"job_type":"rpi.run"}`))
	req.Header.Set(DefaultMutationTokenHeader, "secret-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /openclaw/v1/jobs status = %d, want 405", resp.Code)
	}
	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("OpenClaw read endpoint wrote %d ledger events, want 0", len(events))
	}
}

func TestOpenClawTriggerRequiresAuthAndHasNoSideEffect(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := openClawTriggerRouter(t, store, &now)
	resp := postOpenClawTrigger(t, router, `{"request_id":"req-oc","job_id":"job-oc","job_type":"openclaw.snapshot"}`, "", "")
	if resp.Code != http.StatusForbidden {
		t.Fatalf("unauthorized trigger status = %d, want 403", resp.Code)
	}
	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("unauthorized trigger wrote %d events, want 0", len(events))
	}
}

func TestOpenClawTriggerRequiresReadyDaemon(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	if err := os.MkdirAll(store.Dir(), 0700); err != nil {
		t.Fatalf("create store dir: %v", err)
	}
	if err := os.WriteFile(store.LedgerPath(), []byte("{not-json\n"), 0600); err != nil {
		t.Fatalf("write corrupt ledger: %v", err)
	}
	router := openClawTriggerRouter(t, store, &now)
	resp := postOpenClawTrigger(t, router, `{"request_id":"req-oc","job_id":"job-oc","job_type":"openclaw.snapshot"}`, "secret-token", "")
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("unready trigger status = %d body=%s, want 503", resp.Code, resp.Body.String())
	}
	replay, err := store.ReplayLedgerReadOnly()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(replay.Events) != 0 {
		t.Fatalf("unready trigger wrote %d valid events, want 0", len(replay.Events))
	}
}

func TestOpenClawTriggerAcceptsAllowlistedJobAfterLedgerAck(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := openClawTriggerRouter(t, store, &now)
	resp := postOpenClawTrigger(t, router, `{"request_id":"req-oc","job_id":"job-oc","job_type":"openclaw.snapshot","idempotency_key":"oc-1","payload":{"reason":"refresh"}}`, "secret-token", "")
	if resp.Code != http.StatusAccepted {
		t.Fatalf("authorized trigger status = %d body=%s, want 202", resp.Code, resp.Body.String())
	}
	var body openclaw.TriggerJobResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode trigger response: %v", err)
	}
	if !body.Accepted || body.JobID != "job-oc" || body.JobType != string(JobTypeOpenClawSnapshot) || body.LastEventID == "" {
		t.Fatalf("trigger response = %#v", body)
	}
	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 1 || events[0].EventType != EventJobAccepted || events[0].JobID != "job-oc" {
		t.Fatalf("ledger events = %#v, want accepted job-oc", events)
	}
}

func TestOpenClawTriggerRejectsNonAllowlistedJobType(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := openClawTriggerRouter(t, store, &now)
	resp := postOpenClawTrigger(t, router, `{"request_id":"req-stage","job_id":"job-stage","job_type":"dream.stage"}`, "secret-token", "")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("non-allowlisted trigger status = %d body=%s, want 400", resp.Code, resp.Body.String())
	}
	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("non-allowlisted trigger wrote %d events, want 0", len(events))
	}
}

func TestDaemonJobsCancelStaticRouteUsesMutationPolicy(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	queue := NewQueue(store, QueueOptions{Now: func() time.Time { return now }, LeaseDuration: time.Minute})
	if _, err := queue.SubmitJob(SubmitJobInput{RequestID: "req-cancel", JobID: "job-cancel", JobType: JobTypeRPIRun}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	router := NewDaemonRouter(store, ServerOptions{
		Now: func() time.Time { return now },
		MutationPolicy: DefaultMutationPolicy("secret-token", []string{
			"/jobs/cancel",
			"/v1/jobs/cancel",
		}),
	})

	for _, tc := range []struct {
		name       string
		method     string
		path       string
		token      string
		origin     string
		remoteAddr string
		wantStatus int
	}{
		{name: "missing token", method: http.MethodPost, path: "/v1/jobs/cancel", remoteAddr: "127.0.0.1:51111", wantStatus: http.StatusForbidden},
		{name: "bad token", method: http.MethodPost, path: "/v1/jobs/cancel", token: "wrong", remoteAddr: "127.0.0.1:51111", wantStatus: http.StatusForbidden},
		{name: "bad method", method: http.MethodGet, path: "/v1/jobs/cancel", token: "secret-token", remoteAddr: "127.0.0.1:51111", wantStatus: http.StatusMethodNotAllowed},
		{name: "cross origin", method: http.MethodPost, path: "/v1/jobs/cancel", token: "secret-token", origin: "https://example.com", remoteAddr: "127.0.0.1:51111", wantStatus: http.StatusForbidden},
		{name: "non local remote", method: http.MethodPost, path: "/v1/jobs/cancel", token: "secret-token", remoteAddr: "192.0.2.1:51111", wantStatus: http.StatusForbidden},
		{name: "bad path", method: http.MethodPost, path: "/v1/jobs/job-cancel/cancel", token: "secret-token", remoteAddr: "127.0.0.1:51111", wantStatus: http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"job_id":"job-cancel","reason":"operator"}`))
			req.RemoteAddr = tc.remoteAddr
			if tc.token != "" {
				req.Header.Set(DefaultMutationTokenHeader, tc.token)
			}
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			if resp.Code != tc.wantStatus {
				t.Fatalf("%s status = %d body=%s, want %d", tc.name, resp.Code, resp.Body.String(), tc.wantStatus)
			}
			snapshot, err := queue.Snapshot()
			if err != nil {
				t.Fatalf("snapshot: %v", err)
			}
			if snapshot.Jobs[0].Status != JobStatusQueued {
				t.Fatalf("unauthorized cancel mutated status to %q", snapshot.Jobs[0].Status)
			}
		})
	}

	resp := postDaemonCancel(t, router, `{"request_id":"req-cancel-op","job_id":"job-cancel","reason":"operator"}`, "secret-token", "/v1/jobs/cancel")
	if resp.Code != http.StatusAccepted {
		t.Fatalf("authorized cancel status = %d body=%s, want 202", resp.Code, resp.Body.String())
	}
	var body CancelJobResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode cancel response: %v", err)
	}
	if !body.Cancelled || body.Outcome != CancelJobOutcomeCancelled || body.Job.Status != JobStatusCancelled {
		t.Fatalf("cancel response = %#v, want cancelled terminal job", body)
	}
}

func openClawTriggerRouter(t *testing.T, store *Store, now *time.Time) http.Handler {
	t.Helper()
	return NewDaemonRouter(store, ServerOptions{
		Now: func() time.Time { return *now },
		MutationPolicy: DefaultMutationPolicy("secret-token", []string{
			openclaw.TriggerJobsPath,
		}),
	})
}

func postOpenClawTrigger(t *testing.T, handler http.Handler, payload, token, failpoint string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, openclaw.TriggerJobsPath, strings.NewReader(payload))
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

func postDaemonCancel(t *testing.T, handler http.Handler, payload, token, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(payload))
	req.RemoteAddr = "127.0.0.1:51111"
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set(DefaultMutationTokenHeader, token)
	}
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}

func getJSON(t *testing.T, handler http.Handler, target string, out any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d body=%s", target, resp.Code, resp.Body.String())
	}
	if err := json.Unmarshal(resp.Body.Bytes(), out); err != nil {
		t.Fatalf("decode GET %s response: %v\nbody=%s", target, err, resp.Body.String())
	}
}
