// practices: [agile-manifesto, dora-metrics]
package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
)

func TestRPIPhasedDaemonReadySubmitsRunJob(t *testing.T) {
	cwd := t.TempDir()
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
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

	opts := defaultPhasedEngineOptions()
	opts.WorkingDir = cwd
	opts.DaemonSubmit = true
	opts.DaemonURL = activation.URL
	opts.DaemonToken = "secret-token"
	opts.RunID = "daemon-ready-run"
	opts.PhaseTimeout = 5 * time.Minute
	handled, err := maybeSubmitRPIPhasedDaemon(context.Background(), opts, []string{"ship daemon"})
	if err != nil {
		t.Fatalf("daemon submit: %v", err)
	}
	if !handled {
		t.Fatal("daemon submit was not handled")
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
		t.Fatalf("accepted payload = %#v, want job_payload map", events[0].Payload)
	}
	if payload["phase_timeout"] != "5m0s" {
		t.Fatalf("phase_timeout payload = %#v, want 5m0s", payload["phase_timeout"])
	}
	projection, err := daemonpkg.RebuildRPIRegistryProjection(events)
	if err != nil {
		t.Fatalf("rebuild RPI registry projection: %v", err)
	}
	if len(projection.States) != 1 || projection.States[0].RunID != "daemon-ready-run" {
		t.Fatalf("RPI registry projection = %#v, want daemon-ready-run", projection.States)
	}
}

func TestRPIPhasedDaemonUnreadyRefusesWithoutFallback(t *testing.T) {
	opts := defaultPhasedEngineOptions()
	opts.WorkingDir = t.TempDir()
	opts.DaemonSubmit = true
	opts.DaemonURL = "http://127.0.0.1:1"
	handled, err := maybeSubmitRPIPhasedDaemon(context.Background(), opts, []string{"ship daemon"})
	if err == nil {
		t.Fatal("daemon unready submit succeeded")
	}
	if !handled {
		t.Fatal("daemon unready without fallback should be handled as an error")
	}
}

func TestRPIPhasedDaemonFallbackPreservesForegroundPath(t *testing.T) {
	opts := defaultPhasedEngineOptions()
	opts.WorkingDir = t.TempDir()
	opts.DaemonSubmit = true
	opts.DaemonURL = "http://127.0.0.1:1"
	opts.DaemonFallback = true
	handled, err := maybeSubmitRPIPhasedDaemon(context.Background(), opts, []string{"ship daemon"})
	if err != nil {
		t.Fatalf("fallback returned error: %v", err)
	}
	if handled {
		t.Fatal("fallback should return handled=false so foreground execution can continue")
	}
}

func TestRPIPhasedDaemonDisabledDoesNothing(t *testing.T) {
	opts := defaultPhasedEngineOptions()
	opts.WorkingDir = t.TempDir()
	handled, err := maybeSubmitRPIPhasedDaemon(context.Background(), opts, []string{"ship daemon"})
	if err != nil {
		t.Fatalf("disabled daemon submit returned error: %v", err)
	}
	if handled {
		t.Fatal("disabled daemon submit should not handle the run")
	}
}

func TestEnsureDaemonSubmitRetryIdempotencyKeyGeneratesStableKey(t *testing.T) {
	first := daemonpkg.SubmitJobRequest{
		RequestID: "req-retry",
		JobID:     "job-retry",
		JobType:   daemonpkg.JobTypeRPIRun,
		Payload:   map[string]any{"goal": "ship daemon"},
	}
	second := first
	if err := ensureDaemonSubmitRetryIdempotencyKey(&first); err != nil {
		t.Fatalf("first idempotency key: %v", err)
	}
	if err := ensureDaemonSubmitRetryIdempotencyKey(&second); err != nil {
		t.Fatalf("second idempotency key: %v", err)
	}
	if first.IdempotencyKey == "" {
		t.Fatal("expected generated idempotency key")
	}
	if !strings.HasPrefix(first.IdempotencyKey, "cli-submit:rpi.run:") {
		t.Fatalf("idempotency key = %q, want cli-submit:rpi.run prefix", first.IdempotencyKey)
	}
	if second.IdempotencyKey != first.IdempotencyKey {
		t.Fatalf("generated keys differ: %q vs %q", first.IdempotencyKey, second.IdempotencyKey)
	}

	explicit := daemonpkg.SubmitJobRequest{
		JobType:        daemonpkg.JobTypeRPIRun,
		IdempotencyKey: "rpi.run:explicit",
	}
	if err := ensureDaemonSubmitRetryIdempotencyKey(&explicit); err != nil {
		t.Fatalf("explicit idempotency key: %v", err)
	}
	if explicit.IdempotencyKey != "rpi.run:explicit" {
		t.Fatalf("explicit idempotency key was overwritten: %q", explicit.IdempotencyKey)
	}
}
