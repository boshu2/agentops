// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
)

func TestOvernightDaemonSubmitQueueOnly(t *testing.T) {
	cwd := t.TempDir()
	now := time.Date(2026, 4, 28, 13, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithCancel(context.Background())
	server, listener, activation, err := startAgentOpsDaemon(ctx, cwd, agentopsDaemonRunOptions{
		Addr:  "127.0.0.1:0",
		Token: "secret-token",
		Now:   func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("start daemon: %v", err)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(listener) }()
	t.Cleanup(func() {
		cancel()
		err := <-errCh
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("daemon serve returned unexpected error: %v", err)
		}
	})

	restoreMaxIterations := overnightMaxIterations
	overnightMaxIterations = 3
	t.Cleanup(func() { overnightMaxIterations = restoreMaxIterations })

	summary := testOvernightDaemonSummary(cwd)
	result, err := submitOvernightDaemon(context.Background(), cwd, summary, overnightDaemonModeOptions{
		Enabled: true,
		URL:     activation.URL,
		Token:   "secret-token",
	})
	if err != nil {
		t.Fatalf("submitOvernightDaemon: %v", err)
	}
	if !result.Accepted || result.JobID != "job-dream-daemon-dream-run" {
		t.Fatalf("submit result = %#v", result)
	}

	events, err := daemonpkg.NewStore(cwd).ReadLedger()
	if err != nil {
		t.Fatalf("read daemon ledger: %v", err)
	}
	if len(events) != 1 || events[0].EventType != daemonpkg.EventJobAccepted {
		t.Fatalf("ledger events = %#v, want one accepted event", events)
	}
	payload, ok := events[0].Payload["job_payload"].(map[string]any)
	if !ok {
		t.Fatalf("ledger payload = %#v, want job_payload", events[0].Payload)
	}
	spec, err := daemonpkg.DreamRunJobSpecFromPayload(payload)
	if err != nil {
		t.Fatalf("DreamRunJobSpecFromPayload: %v", err)
	}
	if spec.DreamRunID != "daemon-dream-run" ||
		spec.OutputDir != summary.OutputDir ||
		spec.Goal != "daemon dream goal" ||
		spec.MaxIterations != 3 ||
		spec.ExecutionTimeout != "1h0m0s" {
		t.Fatalf("dream run spec = %#v", spec)
	}
	applyOvernightDaemonSubmitResult(&summary, result)
	if summary.Mode != "dream.daemon-queue" || summary.Status != "queued" {
		t.Fatalf("queue-only summary mode/status = %s/%s", summary.Mode, summary.Status)
	}
}

func TestOvernightDaemonUnreadyRefusesWithoutFallback(t *testing.T) {
	cwd := t.TempDir()
	summary := testOvernightDaemonSummary(cwd)
	handled, err := maybeSubmitOvernightDaemon(context.Background(), cwd, &summary, time.Now().UTC(), overnightDaemonModeOptions{
		Enabled: true,
		URL:     "http://127.0.0.1:1",
	})
	if err == nil {
		t.Fatal("daemon unready submit succeeded")
	}
	if !handled {
		t.Fatal("daemon unready without fallback should be handled as an error")
	}
}

func TestOvernightDaemonFallbackPreservesOneShotPath(t *testing.T) {
	cwd := t.TempDir()
	summary := testOvernightDaemonSummary(cwd)
	handled, err := maybeSubmitOvernightDaemon(context.Background(), cwd, &summary, time.Now().UTC(), overnightDaemonModeOptions{
		Enabled:  true,
		URL:      "http://127.0.0.1:1",
		Fallback: true,
	})
	if err != nil {
		t.Fatalf("fallback returned error: %v", err)
	}
	if handled {
		t.Fatal("fallback should return handled=false so the one-shot path can continue")
	}
	if len(summary.Degraded) != 1 || !strings.Contains(summary.Degraded[0], "falling back to one-shot") {
		t.Fatalf("summary degraded = %v, want one-shot fallback note", summary.Degraded)
	}
}

func TestOvernightDaemonWaitCompleted(t *testing.T) {
	cwd := t.TempDir()
	server, errCh, activation, cancel := startOvernightDaemonTestServer(t, cwd)
	defer stopOvernightDaemonTestServer(t, server, errCh, cancel)
	summary := testOvernightDaemonSummary(cwd)
	completeDaemonDreamJobAsync(t, cwd, "job-dream-"+summary.RunID, daemonpkg.JobStatusCompleted)

	handled, err := maybeSubmitOvernightDaemon(context.Background(), cwd, &summary, time.Now().UTC(), overnightDaemonModeOptions{
		Enabled: true,
		URL:     activation.URL,
		Token:   "secret-token",
		Wait:    true,
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("maybeSubmitOvernightDaemon wait completed: %v", err)
	}
	if !handled {
		t.Fatal("daemon wait completed should handle the command")
	}
	if summary.Mode != "dream.daemon-run" || summary.Status != string(daemonpkg.JobStatusCompleted) {
		t.Fatalf("summary mode/status = %s/%s, want daemon-run/completed", summary.Mode, summary.Status)
	}
	if summary.Artifacts["summary_json"] == "" || summary.Artifacts["overnight_log"] == "" {
		t.Fatalf("summary artifacts = %#v, want terminal daemon artifacts", summary.Artifacts)
	}
}

func TestOvernightDaemonWaitTimeoutDoesNotMarkSuccess(t *testing.T) {
	cwd := t.TempDir()
	server, errCh, activation, cancel := startOvernightDaemonTestServer(t, cwd)
	defer stopOvernightDaemonTestServer(t, server, errCh, cancel)
	summary := testOvernightDaemonSummary(cwd)

	handled, err := maybeSubmitOvernightDaemon(context.Background(), cwd, &summary, time.Now().UTC(), overnightDaemonModeOptions{
		Enabled: true,
		URL:     activation.URL,
		Token:   "secret-token",
		Wait:    true,
		Timeout: 10 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("daemon wait timeout returned nil error")
	}
	if !handled {
		t.Fatal("daemon wait timeout should still handle the command")
	}
	if summary.Mode != "dream.daemon-run" || summary.Status == "done" || summary.Status == string(daemonpkg.JobStatusCompleted) {
		t.Fatalf("summary mode/status = %s/%s, want daemon-run non-success", summary.Mode, summary.Status)
	}
}

func TestOvernightDaemonWaitFailed(t *testing.T) {
	cwd := t.TempDir()
	server, errCh, activation, cancel := startOvernightDaemonTestServer(t, cwd)
	defer stopOvernightDaemonTestServer(t, server, errCh, cancel)
	summary := testOvernightDaemonSummary(cwd)
	completeDaemonDreamJobAsync(t, cwd, "job-dream-"+summary.RunID, daemonpkg.JobStatusFailed)

	handled, err := maybeSubmitOvernightDaemon(context.Background(), cwd, &summary, time.Now().UTC(), overnightDaemonModeOptions{
		Enabled: true,
		URL:     activation.URL,
		Token:   "secret-token",
		Wait:    true,
		Timeout: 2 * time.Second,
	})
	if err == nil {
		t.Fatal("daemon failed job returned nil error")
	}
	if !handled {
		t.Fatal("daemon failed wait should handle the command")
	}
	if summary.Mode != "dream.daemon-run" || summary.Status != string(daemonpkg.JobStatusFailed) {
		t.Fatalf("summary mode/status = %s/%s, want daemon-run/failed", summary.Mode, summary.Status)
	}
	if summary.Artifacts["failure_report"] == "" {
		t.Fatalf("summary artifacts = %#v, want failure_report", summary.Artifacts)
	}
}

func TestDreamDaemonRestartRecoveryIntegration(t *testing.T) {
	cwd := t.TempDir()
	seedDreamReadyCrash(t, cwd, "iter-ready-1")

	ctx1, cancel1 := context.WithCancel(context.Background())
	server1, listener1, activation1, err := startAgentOpsDaemon(ctx1, cwd, agentopsDaemonRunOptions{
		Addr:  "127.0.0.1:0",
		Token: "secret-token",
	})
	if err != nil {
		t.Fatalf("start first daemon: %v", err)
	}
	errCh1 := make(chan error, 1)
	go func() { errCh1 <- server1.Serve(listener1) }()

	restored := filepath.Join(cwd, ".agents", "learnings", "restored.md")
	if data, err := os.ReadFile(restored); err != nil || string(data) != "restored" {
		t.Fatalf("startup recovery did not restore learning: data=%q err=%v", data, err)
	}
	if _, err := os.Stat(filepath.Join(cwd, ".agents", "overnight", "COMMIT-MARKER.iter-ready-1")); !os.IsNotExist(err) {
		t.Fatalf("startup recovery did not remove READY marker: %v", err)
	}

	summary := testOvernightDaemonSummary(cwd)
	if _, err := submitOvernightDaemon(context.Background(), cwd, summary, overnightDaemonModeOptions{
		Enabled: true,
		URL:     activation1.URL,
		Token:   "secret-token",
	}); err != nil {
		t.Fatalf("submit dream run: %v", err)
	}

	cancel1()
	if err := <-errCh1; err != nil && !errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("first daemon serve returned unexpected error: %v", err)
	}

	ctx2, cancel2 := context.WithCancel(context.Background())
	server2, listener2, activation2, err := startAgentOpsDaemon(ctx2, cwd, agentopsDaemonRunOptions{
		Addr:  "127.0.0.1:0",
		Token: "secret-token",
	})
	if err != nil {
		t.Fatalf("restart daemon: %v", err)
	}
	errCh2 := make(chan error, 1)
	go func() { errCh2 <- server2.Serve(listener2) }()
	t.Cleanup(func() {
		cancel2()
		err := <-errCh2
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("second daemon serve returned unexpected error: %v", err)
		}
	})

	status, err := fetchDaemonStatus(context.Background(), activation2.URL)
	if err != nil {
		t.Fatalf("fetch restarted daemon status: %v", err)
	}
	if !status.Ready {
		t.Fatalf("restarted daemon not ready: %#v", status)
	}
	job := findDaemonQueueJob(status.Queue.Jobs, "job-dream-daemon-dream-run")
	if job == nil {
		t.Fatalf("restarted daemon queue missing dream job: %#v", status.Queue.Jobs)
	}
	if job.Status != daemonpkg.JobStatusQueued || job.JobType != daemonpkg.JobTypeDreamRun {
		t.Fatalf("dream job after restart = %#v, want queued dream.run", job)
	}
}

func testOvernightDaemonSummary(cwd string) overnightSummary {
	settings := overnightSettings{
		OutputDir:     filepath.Join(cwd, ".agents", "overnight", "daemon-dream-run"),
		RunTimeoutRaw: "1h",
		RunTimeout:    time.Hour,
	}
	summary := newOvernightStartSummary(cwd, settings, time.Date(2026, 4, 28, 13, 0, 0, 0, time.UTC))
	summary.RunID = "daemon-dream-run"
	summary.Goal = "daemon dream goal"
	return summary
}

func seedDreamReadyCrash(t *testing.T, cwd, iter string) {
	t.Helper()
	prevLearnings := filepath.Join(cwd, ".agents", "overnight", "prev."+iter, "learnings")
	if err := os.MkdirAll(prevLearnings, 0o755); err != nil {
		t.Fatalf("mkdir prev learnings: %v", err)
	}
	if err := os.WriteFile(filepath.Join(prevLearnings, "restored.md"), []byte("restored"), 0o644); err != nil {
		t.Fatalf("write prev learning: %v", err)
	}
	stagingDir := filepath.Join(cwd, ".agents", "overnight", "staging", iter)
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		t.Fatalf("mkdir staging: %v", err)
	}
	markerPath := filepath.Join(cwd, ".agents", "overnight", "COMMIT-MARKER."+iter)
	body := []byte(`{"state":"READY","iteration_id":"` + iter + `","started_at":"2026-04-28T13:00:00Z"}`)
	if err := os.WriteFile(markerPath, body, 0o644); err != nil {
		t.Fatalf("write ready marker: %v", err)
	}
}

func startOvernightDaemonTestServer(t *testing.T, cwd string) (*http.Server, <-chan error, agentopsDaemonActivation, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	server, listener, activation, err := startAgentOpsDaemon(ctx, cwd, agentopsDaemonRunOptions{
		Addr:  "127.0.0.1:0",
		Token: "secret-token",
	})
	if err != nil {
		cancel()
		t.Fatalf("start daemon: %v", err)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(listener) }()
	return server, errCh, activation, cancel
}

func stopOvernightDaemonTestServer(t *testing.T, _ *http.Server, errCh <-chan error, cancel context.CancelFunc) {
	t.Helper()
	cancel()
	err := <-errCh
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("daemon serve returned unexpected error: %v", err)
	}
}

func completeDaemonDreamJobAsync(t *testing.T, cwd, jobID string, terminal daemonpkg.JobStatus) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- completeDaemonDreamJobWhenAccepted(cwd, jobID, terminal) }()
	t.Cleanup(func() {
		if err := <-done; err != nil {
			t.Errorf("complete daemon dream job: %v", err)
		}
	})
}

func completeDaemonDreamJobWhenAccepted(cwd, jobID string, terminal daemonpkg.JobStatus) error {
	queue := daemonpkg.NewQueue(daemonpkg.NewStore(cwd), daemonpkg.QueueOptions{LeaseDuration: time.Minute})
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		claim, err := queue.ClaimJob(jobID, "test-worker", daemonpkg.QueueMutationOptions{})
		if err != nil {
			if errors.Is(err, daemonpkg.ErrJobNotFound) || errors.Is(err, daemonpkg.ErrNoClaimableJobs) || errors.Is(err, daemonpkg.ErrJobAlreadyClaimed) {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			return err
		}
		artifacts := map[string]string{
			"summary_json":     filepath.Join(cwd, ".agents", "overnight", "daemon-dream-run", "summary.json"),
			"summary_markdown": filepath.Join(cwd, ".agents", "overnight", "daemon-dream-run", "summary.md"),
			"overnight_log":    filepath.Join(cwd, ".agents", "overnight", "daemon-dream-run", "overnight.log"),
		}
		switch terminal {
		case daemonpkg.JobStatusCompleted:
			_, err = queue.CompleteJob(daemonpkg.CompleteJobInput{
				JobID:      claim.Job.JobID,
				RequestID:  daemonpkg.RequestID("req-test-complete"),
				ClaimToken: claim.ClaimToken,
				LeaseEpoch: claim.LeaseEpoch,
				Actor:      "test-worker",
				Artifacts:  artifacts,
			}, daemonpkg.QueueMutationOptions{})
		case daemonpkg.JobStatusFailed:
			artifacts["failure_report"] = filepath.Join(cwd, ".agents", "overnight", "daemon-dream-run", "failure-report.md")
			_, err = queue.FailJob(daemonpkg.FailJobInput{
				JobID:      claim.Job.JobID,
				RequestID:  daemonpkg.RequestID("req-test-fail"),
				ClaimToken: claim.ClaimToken,
				LeaseEpoch: claim.LeaseEpoch,
				Actor:      "test-worker",
				Failure: daemonpkg.JobFailure{
					Code:    daemonpkg.FailureRequestRejected,
					Message: "dream failed",
				},
				Artifacts: artifacts,
			}, daemonpkg.QueueMutationOptions{})
		default:
			err = nil
		}
		return err
	}
	return errors.New("timed out waiting for daemon dream job to be accepted")
}

func findDaemonQueueJob(jobs []daemonpkg.QueueJobState, id string) *daemonpkg.QueueJobState {
	for i := range jobs {
		if jobs[i].JobID == id {
			return &jobs[i]
		}
	}
	return nil
}
