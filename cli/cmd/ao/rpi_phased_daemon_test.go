package main

import (
	"context"
	"errors"
	"net/http"
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
