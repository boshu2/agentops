package openclaw_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/daemon"
	"github.com/boshu2/agentops/cli/internal/openclaw"
)

func TestExternalStyleOpenClawClientReadsSnapshot(t *testing.T) {
	now := time.Date(2026, 4, 28, 21, 0, 0, 0, time.UTC)
	store := daemon.NewStore(t.TempDir())
	seedOpenClawFixtureDaemon(t, store, now)
	server := httptest.NewServer(daemon.NewDaemonRouter(store, daemon.ServerOptions{
		Now: func() time.Time { return now },
		MutationPolicy: daemon.DefaultMutationPolicy("secret-token", []string{
			openclaw.TriggerJobsPath,
		}),
	}))
	defer server.Close()

	client := externalOpenClawClient{baseURL: server.URL, httpClient: server.Client()}
	snapshot, err := client.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snapshot.SchemaVersion != openclaw.ConsumerSnapshotSchemaVersion || snapshot.SnapshotID == "snap_empty" {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	if len(snapshot.Resources.Runs) != 1 || len(snapshot.Resources.Wiki) != 1 {
		t.Fatalf("snapshot resources = %#v", snapshot.Resources)
	}
	if len(snapshot.Resources.Wiki[0].Provenance) == 0 {
		t.Fatalf("wiki resource missing provenance: %#v", snapshot.Resources.Wiki[0])
	}

	runs, err := client.Runs()
	if err != nil {
		t.Fatalf("runs: %v", err)
	}
	if len(runs.Runs) != 1 || runs.Runs[0].ResourceKind != openclaw.ResourceKindRun {
		t.Fatalf("runs response = %#v", runs)
	}
	wiki, err := client.Wiki()
	if err != nil {
		t.Fatalf("wiki: %v", err)
	}
	if len(wiki.Wiki) != 1 || wiki.Wiki[0].JobID != "job-wiki" {
		t.Fatalf("wiki response = %#v", wiki)
	}
	health, err := client.Health()
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if health.Status != "ok" || !health.Ready || health.ResourceCounts.Jobs != 2 {
		t.Fatalf("health response = %#v", health)
	}
}

type externalOpenClawClient struct {
	baseURL    string
	httpClient *http.Client
}

func (c externalOpenClawClient) Snapshot() (openclaw.ConsumerSnapshot, error) {
	var out openclaw.ConsumerSnapshot
	err := c.getJSON("/openclaw/v1/snapshot/latest", &out)
	return out, err
}

func (c externalOpenClawClient) Runs() (openclaw.RunsResponse, error) {
	var out openclaw.RunsResponse
	err := c.getJSON("/openclaw/v1/runs", &out)
	return out, err
}

func (c externalOpenClawClient) Wiki() (openclaw.WikiResponse, error) {
	var out openclaw.WikiResponse
	err := c.getJSON("/openclaw/v1/wiki", &out)
	return out, err
}

func (c externalOpenClawClient) Health() (openclaw.HealthResponse, error) {
	var out openclaw.HealthResponse
	err := c.getJSON("/openclaw/v1/health", &out)
	return out, err
}

func (c externalOpenClawClient) getJSON(path string, out any) error {
	httpClient := c.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Get(c.baseURL + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s status %d", path, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func seedOpenClawFixtureDaemon(t *testing.T, store *daemon.Store, now time.Time) {
	t.Helper()
	queue := daemon.NewQueue(store, daemon.QueueOptions{Now: func() time.Time { return now }, LeaseDuration: time.Minute})
	if _, err := queue.SubmitJob(daemon.SubmitJobInput{
		RequestID: "req-rpi",
		JobID:     "job-rpi",
		JobType:   daemon.JobTypeRPIRun,
	}, daemon.QueueMutationOptions{}); err != nil {
		t.Fatalf("submit rpi job: %v", err)
	}
	if _, err := queue.SubmitJob(daemon.SubmitJobInput{
		RequestID: "req-wiki",
		JobID:     "job-wiki",
		JobType:   daemon.JobTypeWikiForge,
	}, daemon.QueueMutationOptions{}); err != nil {
		t.Fatalf("submit wiki job: %v", err)
	}
	claim, err := queue.ClaimJob("job-wiki", "wiki-worker", daemon.QueueMutationOptions{})
	if err != nil {
		t.Fatalf("claim wiki job: %v", err)
	}
	if _, err := queue.CompleteJob(daemon.CompleteJobInput{
		JobID:      "job-wiki",
		RequestID:  "req-wiki-complete",
		ClaimToken: claim.ClaimToken,
		LeaseEpoch: claim.LeaseEpoch,
		Actor:      "wiki-worker",
		Artifacts: map[string]string{
			"worker_session_refs": ".agents/daemon/wiki/job-wiki-worker-sessions.json",
		},
	}, daemon.QueueMutationOptions{}); err != nil {
		t.Fatalf("complete wiki job: %v", err)
	}
}
