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
	var eventsAfter ReadOnlyEventsResponse
	getJSON(t, router, "/v1/events?since="+events.Events[0].EventID, &eventsAfter)
	if len(eventsAfter.Events) != 1 || eventsAfter.Events[0].EventID != events.Events[1].EventID {
		t.Fatalf("events after = %#v, want only %s", eventsAfter, events.Events[1].EventID)
	}
}

func TestReadOnlyServerLoadsProjectionSnapshotOnReadState(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	// N=2 events get folded into a snapshot.
	for _, evt := range []LedgerEvent{
		mustNewProjectionTestEvent(t, "evt-001", "req-1", "job-rpi", EventJobAccepted, JobTypeRPIRun, 0, nil),
		mustNewProjectionTestEvent(t, "evt-002", "req-2", "job-rpi", EventJobClaimed, "", 1, nil),
	} {
		if _, err := store.AppendLedgerEvent(evt); err != nil {
			t.Fatalf("append base event: %v", err)
		}
	}
	baseSet, err := store.RebuildProjections(ProjectionRebuildOptions{RebuiltAt: now})
	if err != nil {
		t.Fatalf("rebuild base: %v", err)
	}
	if _, err := store.WriteProjectionSnapshot(baseSet); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
	// M=1 new event arrives after the snapshot.
	if _, err := store.AppendLedgerEvent(mustNewProjectionTestEvent(t, "evt-003", "req-3", "job-rpi", EventJobCompleted, "", 2, nil)); err != nil {
		t.Fatalf("append delta event: %v", err)
	}

	router := NewReadOnlyRouter(store, ServerOptions{Now: func() time.Time { return now }})
	var status ReadOnlyStatusResponse
	getJSON(t, router, "/status", &status)
	if status.Projections.LastEventID != "evt-003" {
		t.Fatalf("LastEventID after delta replay = %q, want evt-003", status.Projections.LastEventID)
	}
	if len(status.Projections.RPI.Runs) != 1 || status.Projections.RPI.Runs[0].Status != JobStatusCompleted {
		t.Fatalf("RPI run status = %#v, want completed", status.Projections.RPI.Runs)
	}
	if status.Projections.RPI.Runs[0].LastEventID != "evt-003" {
		t.Fatalf("RPI run LastEventID = %q, want evt-003", status.Projections.RPI.Runs[0].LastEventID)
	}
}

func TestReadOnlyServerSkipsCorruptSnapshotAndDegradesGracefully(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	if _, err := store.AppendLedgerEvent(mustNewProjectionTestEvent(t, "evt-001", "req-1", "job-rpi", EventJobAccepted, JobTypeRPIRun, 0, nil)); err != nil {
		t.Fatalf("append event: %v", err)
	}
	// Plant a snapshot file with the wrong schema_version on disk.
	if err := os.MkdirAll(store.ProjectionSnapshotDir(), 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	stale := []byte(`{"schema_version":99,"last_event_id":"evt-001","jobs":[]}` + "\n")
	if err := os.WriteFile(store.ProjectionSnapshotDir()+"/snapshot-99999999T999999.999999999Z.json", stale, 0600); err != nil {
		t.Fatalf("plant stale snapshot: %v", err)
	}

	router := NewReadOnlyRouter(store, ServerOptions{Now: func() time.Time { return now }})
	var status ReadOnlyStatusResponse
	getJSON(t, router, "/status", &status)
	if status.Projections.LastEventID != "evt-001" {
		t.Fatalf("LastEventID after fallback = %q, want evt-001", status.Projections.LastEventID)
	}
	if len(status.Projections.DegradedReasons) == 0 {
		t.Fatal("expected degraded reason from stale snapshot fallback")
	}
	foundStaleReason := false
	for _, r := range status.Projections.DegradedReasons {
		if strings.Contains(r, "ignored stale projection snapshot") {
			foundStaleReason = true
			break
		}
	}
	if !foundStaleReason {
		t.Fatalf("degraded reasons = %#v, want stale-snapshot reason", status.Projections.DegradedReasons)
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
	refsArtifact := ArtifactRef{
		Path:      ".agents/handoffs/sha256/aa/bb/" + strings.Repeat("a", 64),
		SHA256:    strings.Repeat("a", 64),
		Size:      128,
		WrittenAt: now.Format(time.RFC3339Nano),
	}
	if _, err := queue.CompleteJob(CompleteJobInput{
		JobID:        "job-wiki",
		RequestID:    "req-wiki-complete",
		ClaimToken:   claim.ClaimToken,
		LeaseEpoch:   claim.LeaseEpoch,
		Actor:        "wiki-worker",
		ArtifactRefs: map[string]ArtifactRef{"worker_session_refs": refsArtifact},
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
	if got := snapshot.Resources.Wiki[0].ArtifactRefs["worker_session_refs"].SHA256; got != refsArtifact.SHA256 {
		t.Fatalf("snapshot artifact ref sha = %q, want %q", got, refsArtifact.SHA256)
	}
	if got := snapshot.Resources.Wiki[0].Artifacts["worker_session_refs"]; got != refsArtifact.Path {
		t.Fatalf("snapshot compat artifact path = %q, want %q", got, refsArtifact.Path)
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

// schedulesRouter builds a daemon router with a default-token mutation policy
// suitable for schedule-route tests.
func schedulesRouter(t *testing.T, store *Store, now *time.Time) http.Handler {
	t.Helper()
	return NewDaemonRouter(store, ServerOptions{
		Now:            func() time.Time { return *now },
		MutationPolicy: DefaultMutationPolicy("secret-token", nil),
	})
}

func postSchedule(t *testing.T, handler http.Handler, payload, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/schedules", strings.NewReader(payload))
	req.RemoteAddr = "127.0.0.1:51111"
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set(DefaultMutationTokenHeader, token)
	}
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}

func deleteSchedule(t *testing.T, handler http.Handler, name, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, "/v1/schedules/"+name, nil)
	req.RemoteAddr = "127.0.0.1:51111"
	if token != "" {
		req.Header.Set(DefaultMutationTokenHeader, token)
	}
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}

// TestRoute_PostSchedules_Creates posts a valid RecurringJobTemplate and
// asserts the schedule lands in the store.
func TestRoute_PostSchedules_Creates(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := schedulesRouter(t, store, &now)

	payload := `{"name":"daily-llmwiki","cron":"0 3 * * *","job_type":"llmwiki.loop","backpressure":{"skip_if_running":true}}`
	resp := postSchedule(t, router, payload, "secret-token")
	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /v1/schedules status = %d body=%s, want 201", resp.Code, resp.Body.String())
	}
	var body CreateScheduleResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Name != "daily-llmwiki" {
		t.Fatalf("response name = %q, want daily-llmwiki", body.Name)
	}
	saved, err := store.ListSchedules()
	if err != nil {
		t.Fatalf("ListSchedules: %v", err)
	}
	if len(saved) != 1 || saved[0].Name != "daily-llmwiki" || saved[0].Cron != "0 3 * * *" || saved[0].JobType != JobTypeLLMWikiLoop {
		t.Fatalf("ListSchedules = %#v, want one daily-llmwiki schedule", saved)
	}
}

// TestRoute_PostSchedules_DuplicateNameReturns409 verifies the name-uniqueness
// contract bubbles up as 409.
func TestRoute_PostSchedules_DuplicateNameReturns409(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := schedulesRouter(t, store, &now)

	payload := `{"name":"dup","cron":"0 3 * * *","job_type":"llmwiki.loop"}`
	resp := postSchedule(t, router, payload, "secret-token")
	if resp.Code != http.StatusCreated {
		t.Fatalf("first POST status = %d body=%s, want 201", resp.Code, resp.Body.String())
	}
	resp = postSchedule(t, router, payload, "secret-token")
	if resp.Code != http.StatusConflict {
		t.Fatalf("duplicate POST status = %d body=%s, want 409", resp.Code, resp.Body.String())
	}
}

// TestRoute_PostSchedules_MalformedJSONReturns400 ensures the daemon
// surfaces a clear 400 on invalid JSON (see amendment A2 fail-loud posture).
func TestRoute_PostSchedules_MalformedJSONReturns400(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := schedulesRouter(t, store, &now)

	resp := postSchedule(t, router, `{not json`, "secret-token")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("malformed JSON status = %d body=%s, want 400", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "error") {
		t.Fatalf("400 body missing error field: %s", resp.Body.String())
	}
}

// TestRoute_PostSchedules_MissingFieldsReturns400 covers required fields.
func TestRoute_PostSchedules_MissingFieldsReturns400(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := schedulesRouter(t, store, &now)

	cases := []struct {
		name    string
		payload string
	}{
		{"missing name", `{"cron":"0 3 * * *","job_type":"llmwiki.loop"}`},
		{"missing cron", `{"name":"x","job_type":"llmwiki.loop"}`},
		{"missing job_type", `{"name":"x","cron":"0 3 * * *"}`},
		{"invalid cron", `{"name":"x","cron":"not-a-cron","job_type":"llmwiki.loop"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := postSchedule(t, router, tc.payload, "secret-token")
			if resp.Code != http.StatusBadRequest {
				t.Fatalf("status = %d body=%s, want 400", resp.Code, resp.Body.String())
			}
		})
	}
}

// TestRoute_GetSchedules_ReturnsList saves two schedules directly via the
// store and verifies GET surfaces both. GET is read-only and bypasses auth.
func TestRoute_GetSchedules_ReturnsList(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := schedulesRouter(t, store, &now)

	if err := store.SaveSchedule(RecurringJobTemplate{Name: "alpha", Cron: "0 3 * * *", JobType: JobTypeLLMWikiLoop}); err != nil {
		t.Fatalf("save alpha: %v", err)
	}
	if err := store.SaveSchedule(RecurringJobTemplate{Name: "beta", Cron: "0 4 * * *", JobType: JobTypeWikiBuild}); err != nil {
		t.Fatalf("save beta: %v", err)
	}
	var body ListSchedulesResponse
	getJSON(t, router, "/v1/schedules", &body)
	if len(body.Schedules) != 2 {
		t.Fatalf("schedules = %#v, want 2", body.Schedules)
	}
	names := map[string]bool{}
	for _, s := range body.Schedules {
		names[s.Name] = true
	}
	if !names["alpha"] || !names["beta"] {
		t.Fatalf("names = %v, want alpha+beta", names)
	}
}

// TestRoute_DeleteSchedule_Removes saves a schedule, deletes via HTTP, and
// asserts the next ListSchedules omits it.
func TestRoute_DeleteSchedule_Removes(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := schedulesRouter(t, store, &now)

	if err := store.SaveSchedule(RecurringJobTemplate{Name: "gone-soon", Cron: "0 5 * * *", JobType: JobTypeLLMWikiLoop}); err != nil {
		t.Fatalf("save: %v", err)
	}
	resp := deleteSchedule(t, router, "gone-soon", "secret-token")
	if resp.Code != http.StatusOK {
		t.Fatalf("DELETE status = %d body=%s, want 200", resp.Code, resp.Body.String())
	}
	saved, err := store.ListSchedules()
	if err != nil {
		t.Fatalf("ListSchedules: %v", err)
	}
	for _, s := range saved {
		if s.Name == "gone-soon" {
			t.Fatalf("schedule still present after DELETE: %#v", saved)
		}
	}
}

// TestRoute_DeleteSchedule_AuthRequired asserts DELETE rejects an
// unauthenticated request with 403 (the registerMutationRoute wrapper).
func TestRoute_DeleteSchedule_AuthRequired(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	router := schedulesRouter(t, store, &now)

	if err := store.SaveSchedule(RecurringJobTemplate{Name: "guarded", Cron: "0 5 * * *", JobType: JobTypeLLMWikiLoop}); err != nil {
		t.Fatalf("save: %v", err)
	}
	resp := deleteSchedule(t, router, "guarded", "")
	if resp.Code != http.StatusForbidden {
		t.Fatalf("DELETE without token status = %d body=%s, want 403", resp.Code, resp.Body.String())
	}
	// The schedule must still be there — auth rejected the request before any
	// store mutation could land.
	saved, err := store.ListSchedules()
	if err != nil {
		t.Fatalf("ListSchedules: %v", err)
	}
	found := false
	for _, s := range saved {
		if s.Name == "guarded" {
			found = true
		}
	}
	if !found {
		t.Fatalf("unauthorized DELETE removed the schedule: %#v", saved)
	}
}
