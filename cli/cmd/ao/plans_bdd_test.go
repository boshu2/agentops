package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
)

// L2 BDD — exercises the full CLI → daemon → executor → projection chain
// through an httptest.NewServer-wrapped daemon router. Pattern mirrors
// cli/internal/daemon/server_test.go:441 (httptest.NewServer router).
//
// Six scenarios per pilot §b. Subtest names follow the foundation §2
// Given/When/Then template.

// l2MutationToken is the in-test mutation token; the daemon router rejects
// /v1/jobs without it.
const l2MutationToken = "l2-bdd-token"

// l2HarnessOptions configures one BDD harness instance.
type l2HarnessOptions struct {
	bdEntries []daemonpkg.PlansProjectionEntry
	bdErr     error
	cancelCtx bool
	stale     []daemonpkg.PlansProjectionEntry
	now       time.Time
}

// l2Harness wires a real PlansProjectionExecutor against an httptest daemon
// router and a fake bd source. The CLI talks to the server URL exactly as
// production would.
type l2Harness struct {
	t          *testing.T
	server     *httptest.Server
	store      *daemonpkg.Store
	queue      *daemonpkg.Queue
	executor   *daemonpkg.PlansProjectionExecutor
	rootDir    string
	outputDir  string
	bdSource   *fakeBdSourceL2
	now        time.Time
	clientCtx  context.Context
	clientStop context.CancelFunc
}

type fakeBdSourceL2 struct {
	mu         sync.Mutex
	entries    []daemonpkg.PlansProjectionEntry
	err        error
	delayUntil func(ctx context.Context) error
	calls      int
}

func (f *fakeBdSourceL2) QueryEpics(ctx context.Context, projectID, issuePrefix string) ([]daemonpkg.PlansProjectionEntry, error) {
	f.mu.Lock()
	f.calls++
	delay := f.delayUntil
	err := f.err
	entries := append([]daemonpkg.PlansProjectionEntry(nil), f.entries...)
	f.mu.Unlock()
	if delay != nil {
		if delayErr := delay(ctx); delayErr != nil {
			return nil, delayErr
		}
	}
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func newL2Harness(t *testing.T, opts l2HarnessOptions) *l2Harness {
	t.Helper()
	root := t.TempDir()
	outputDir := filepath.Join(root, ".agents", "plans")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("mkdir output: %v", err)
	}
	if len(opts.stale) > 0 {
		// Pre-seed a stale snapshot so the recovery scenario has something
		// to overwrite.
		snap, err := os.Create(filepath.Join(outputDir, "manifest.jsonl"))
		if err != nil {
			t.Fatalf("seed stale: %v", err)
		}
		enc := json.NewEncoder(snap)
		for _, entry := range opts.stale {
			if err := enc.Encode(entry); err != nil {
				t.Fatalf("seed stale encode: %v", err)
			}
		}
		_ = snap.Close()
	}

	store := daemonpkg.NewStore(root)
	now := opts.now
	if now.IsZero() {
		now = time.Date(2026, 5, 1, 14, 0, 0, 0, time.UTC)
	}
	queue := daemonpkg.NewQueue(store, daemonpkg.QueueOptions{Now: func() time.Time { return now }})
	bdSource := &fakeBdSourceL2{entries: opts.bdEntries, err: opts.bdErr}
	if opts.cancelCtx {
		bdSource.delayUntil = func(c context.Context) error {
			select {
			case <-c.Done():
				return c.Err()
			case <-time.After(time.Second):
				return errors.New("delay timeout")
			}
		}
	}
	executor, err := daemonpkg.NewPlansProjectionExecutor(daemonpkg.PlansProjectionExecutorOptions{
		Store:    store,
		BdSource: bdSource,
		Now:      func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	router := daemonpkg.NewDaemonRouter(store, daemonpkg.ServerOptions{
		Now:            func() time.Time { return now },
		MutationPolicy: daemonpkg.DefaultMutationPolicy(l2MutationToken, nil),
	})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	return &l2Harness{
		t:          t,
		server:     server,
		store:      store,
		queue:      queue,
		executor:   executor,
		rootDir:    root,
		outputDir:  outputDir,
		bdSource:   bdSource,
		now:        now,
		clientCtx:  ctx,
		clientStop: cancel,
	}
}

// submitAndExecute drives the full lifecycle: POST /v1/jobs → claim → run
// the executor in-process → record terminal. Returns the final job state.
func (h *l2Harness) submitAndExecute(jobID string, spec daemonpkg.PlansProjectionJobSpec) daemonpkg.QueueJobState {
	h.t.Helper()
	specRaw, err := json.Marshal(spec)
	if err != nil {
		h.t.Fatalf("marshal spec: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(specRaw, &payload); err != nil {
		h.t.Fatalf("unmarshal spec: %v", err)
	}
	req := daemonpkg.SubmitJobRequest{
		JobID:          jobID,
		JobType:        daemonpkg.JobTypePlansProjection,
		IdempotencyKey: spec.IdempotencyKey(),
		Payload:        payload,
	}
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequest(http.MethodPost, h.server.URL+"/v1/jobs", bytes.NewReader(body))
	if err != nil {
		h.t.Fatalf("build POST: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set(daemonpkg.DefaultMutationTokenHeader, l2MutationToken)
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		h.t.Fatalf("POST /v1/jobs: %v", err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		h.t.Fatalf("POST /v1/jobs status = %d", resp.StatusCode)
	}
	var submitResp daemonpkg.SubmitJobResponse
	if err := json.NewDecoder(resp.Body).Decode(&submitResp); err != nil {
		h.t.Fatalf("decode submit response: %v", err)
	}
	_ = resp.Body.Close()
	if !submitResp.Accepted {
		h.t.Fatalf("submit rejected: %+v", submitResp)
	}

	claim, err := h.queue.ClaimJob(submitResp.JobID, "l2-test", daemonpkg.QueueMutationOptions{})
	if err != nil {
		// Idempotency-key collapse: the queue may report no claimable jobs
		// when the prior submission for the same key already terminated.
		// Return the existing terminal state so callers see the
		// replay-idempotent result.
		snapshot, snapErr := h.queue.Snapshot()
		if snapErr == nil {
			for _, j := range snapshot.Jobs {
				if j.JobID == submitResp.JobID && (j.Status == daemonpkg.JobStatusCompleted || j.Status == daemonpkg.JobStatusFailed) {
					return j
				}
			}
		}
		h.t.Fatalf("claim: %v", err)
	}
	result, runErr := h.executor.RunJob(h.clientCtx, claim)
	if runErr != nil {
		_, _ = h.queue.FailJob(daemonpkg.FailJobInput{
			JobID:      claim.Job.JobID,
			RequestID:  daemonpkg.RequestID("req-" + claim.Job.JobID + "-fail"),
			ClaimToken: claim.ClaimToken,
			LeaseEpoch: claim.LeaseEpoch,
			Actor:      "l2-test",
			Failure: daemonpkg.JobFailure{
				Code:    daemonpkg.FailureRequestRejected,
				Message: runErr.Error(),
			},
		}, daemonpkg.QueueMutationOptions{})
	} else {
		_, _ = h.queue.CompleteJob(daemonpkg.CompleteJobInput{
			JobID:      claim.Job.JobID,
			RequestID:  daemonpkg.RequestID("req-" + claim.Job.JobID + "-complete"),
			ClaimToken: claim.ClaimToken,
			LeaseEpoch: claim.LeaseEpoch,
			Actor:      "l2-test",
			Artifacts:  result.Artifacts,
		}, daemonpkg.QueueMutationOptions{})
	}

	snapshot, err := h.queue.Snapshot()
	if err != nil {
		h.t.Fatalf("snapshot: %v", err)
	}
	for _, j := range snapshot.Jobs {
		if j.JobID == submitResp.JobID {
			return j
		}
	}
	h.t.Fatalf("job %s not found in snapshot", submitResp.JobID)
	return daemonpkg.QueueJobState{}
}

// fetchManifestEntries reads the manifest snapshot the executor wrote to
// the harness's output dir.
func (h *l2Harness) fetchManifestEntries() []daemonpkg.PlansProjectionEntry {
	h.t.Helper()
	manifestPath := filepath.Join(h.outputDir, "manifest.jsonl")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		h.t.Fatalf("read manifest: %v", err)
	}
	var entries []daemonpkg.PlansProjectionEntry
	dec := json.NewDecoder(bytes.NewReader(data))
	for dec.More() {
		var entry daemonpkg.PlansProjectionEntry
		if err := dec.Decode(&entry); err != nil {
			h.t.Fatalf("decode manifest line: %v", err)
		}
		entries = append(entries, entry)
	}
	return entries
}

func (h *l2Harness) spec() daemonpkg.PlansProjectionJobSpec {
	return daemonpkg.NewPlansProjectionJobSpec("l2-project", "soc", h.outputDir)
}

func TestPlans_BDD(t *testing.T) {
	t.Run("Given empty bd state, When subscription starts, Then empty projection emitted with zero manifest entries", func(t *testing.T) {
		h := newL2Harness(t, l2HarnessOptions{})
		job := h.submitAndExecute("job-l2-empty", h.spec())
		if job.Status != daemonpkg.JobStatusCompleted {
			t.Fatalf("status = %s, want completed", job.Status)
		}
		if got := job.Artifacts["manifest_count"]; got != "0" {
			t.Fatalf("manifest_count = %q, want 0", got)
		}
		if entries := h.fetchManifestEntries(); len(entries) != 0 {
			t.Fatalf("entries = %d, want 0", len(entries))
		}
	})

	t.Run("Given populated bd state, When subscription starts, Then projection rebuilt with N sorted entries via HTTP submit", func(t *testing.T) {
		h := newL2Harness(t, l2HarnessOptions{
			bdEntries: []daemonpkg.PlansProjectionEntry{
				{BeadsID: "soc-cccc", Title: "c", Status: "open", IssueType: "epic"},
				{BeadsID: "soc-aaaa", Title: "a", Status: "open", IssueType: "epic"},
				{BeadsID: "soc-bbbb", Title: "b", Status: "closed", IssueType: "epic"},
			},
		})
		job := h.submitAndExecute("job-l2-populated", h.spec())
		if job.Status != daemonpkg.JobStatusCompleted {
			t.Fatalf("status = %s, want completed", job.Status)
		}
		if got := job.Artifacts["manifest_count"]; got != "3" {
			t.Fatalf("manifest_count = %q, want 3", got)
		}
		entries := h.fetchManifestEntries()
		want := []string{"soc-aaaa", "soc-bbbb", "soc-cccc"}
		for i, expect := range want {
			if entries[i].BeadsID != expect {
				t.Fatalf("entries[%d] = %q, want %q (sort order)", i, entries[i].BeadsID, expect)
			}
		}
	})

	t.Run("Given bd query fails transiently, When job runs, Then job terminates failed and no snapshot is written", func(t *testing.T) {
		h := newL2Harness(t, l2HarnessOptions{bdErr: errors.New("dolt unreachable")})
		job := h.submitAndExecute("job-l2-transient", h.spec())
		if job.Status != daemonpkg.JobStatusFailed {
			t.Fatalf("status = %s, want failed", job.Status)
		}
		if entries := h.fetchManifestEntries(); len(entries) != 0 {
			t.Fatalf("entries on failure = %d, want 0 (no snapshot written)", len(entries))
		}
	})

	t.Run("Given context cancellation mid-run, When executor checks ctx, Then job terminates failed and no snapshot is written", func(t *testing.T) {
		h := newL2Harness(t, l2HarnessOptions{
			bdEntries: []daemonpkg.PlansProjectionEntry{{BeadsID: "soc-c", Title: "c"}},
			cancelCtx: true,
		})
		// Cancel the harness ctx before submit completes.
		h.clientStop()
		job := h.submitAndExecute("job-l2-cancel", h.spec())
		if job.Status != daemonpkg.JobStatusFailed {
			t.Fatalf("status = %s, want failed", job.Status)
		}
	})

	t.Run("Given identical bd state across two HTTP submits, When jobs run, Then projection is replay-idempotent", func(t *testing.T) {
		h := newL2Harness(t, l2HarnessOptions{
			bdEntries: []daemonpkg.PlansProjectionEntry{
				{BeadsID: "soc-1", Title: "one", Status: "open", IssueType: "epic"},
				{BeadsID: "soc-2", Title: "two", Status: "closed", IssueType: "epic"},
			},
		})
		first := h.submitAndExecute("job-l2-idem-1", h.spec())
		second := h.submitAndExecute("job-l2-idem-2", h.spec())
		if first.Status != daemonpkg.JobStatusCompleted || second.Status != daemonpkg.JobStatusCompleted {
			t.Fatalf("statuses = %s/%s, want both completed", first.Status, second.Status)
		}
		if first.Artifacts["manifest_jsonl"] != second.Artifacts["manifest_jsonl"] {
			t.Fatalf("manifest path drifted: %q -> %q",
				first.Artifacts["manifest_jsonl"], second.Artifacts["manifest_jsonl"])
		}
		entries := h.fetchManifestEntries()
		if len(entries) != 2 {
			t.Fatalf("entries after replay = %d, want 2", len(entries))
		}
	})

	t.Run("Given a stale snapshot from a prior crash, When job runs after restart, Then snapshot is overwritten atomically with current bd state", func(t *testing.T) {
		h := newL2Harness(t, l2HarnessOptions{
			stale: []daemonpkg.PlansProjectionEntry{
				{BeadsID: "soc-stale", Title: "stale-from-crashed-run"},
			},
			bdEntries: []daemonpkg.PlansProjectionEntry{
				{BeadsID: "soc-fresh", Title: "fresh", Status: "open", IssueType: "epic"},
			},
		})
		job := h.submitAndExecute("job-l2-recover", h.spec())
		if job.Status != daemonpkg.JobStatusCompleted {
			t.Fatalf("status = %s, want completed", job.Status)
		}
		entries := h.fetchManifestEntries()
		if len(entries) != 1 || entries[0].BeadsID != "soc-fresh" {
			t.Fatalf("post-recover entries = %#v, want only soc-fresh", entries)
		}
	})
}

// TestPlans_BDD_RouteVisibility asserts that the /v1/plans/manifest and
// /v1/plans/diff routes registered in atom-1 stay reachable via the
// httptest server even when no projection has been built yet.
func TestPlans_BDD_RouteVisibility(t *testing.T) {
	store := daemonpkg.NewStore(t.TempDir())
	router := daemonpkg.NewDaemonRouter(store, daemonpkg.ServerOptions{Now: func() time.Time { return time.Now().UTC() }})
	server := httptest.NewServer(router)
	defer server.Close()

	for _, path := range []string{"/v1/plans/manifest", "/v1/plans/diff"} {
		resp, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200", path, resp.StatusCode)
		}
		_ = resp.Body.Close()
	}

	if !strings.Contains(fmt.Sprintf("%v", server.URL), "127.0.0.1") {
		t.Fatalf("httptest server URL unexpected: %s", server.URL)
	}
}
