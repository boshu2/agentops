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

func TestOvernightDaemonReadySubmitsRunJob(t *testing.T) {
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
		spec.MaxIterations != 3 {
		t.Fatalf("dream run spec = %#v", spec)
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

func findDaemonQueueJob(jobs []daemonpkg.QueueJobState, id string) *daemonpkg.QueueJobState {
	for i := range jobs {
		if jobs[i].JobID == id {
			return &jobs[i]
		}
	}
	return nil
}
