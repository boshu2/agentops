package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestDaemonActivationSmokeReadyStatus(t *testing.T) {
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
	go func() {
		errCh <- server.Serve(listener)
	}()
	t.Cleanup(func() {
		cancel()
		err := <-errCh
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("daemon serve returned unexpected error: %v", err)
		}
	})

	if activation.URL == "" || !activation.Ready {
		t.Fatalf("activation = %#v, want ready URL", activation)
	}
	if _, err := readDaemonActivation(cwd); err != nil {
		t.Fatalf("read activation file: %v", err)
	}
	ready, err := fetchDaemonReady(context.Background(), activation.URL)
	if err != nil {
		t.Fatalf("fetch ready: %v", err)
	}
	if !ready.Ready {
		t.Fatalf("ready response = %#v, want ready", ready)
	}
	status, err := fetchDaemonStatus(context.Background(), activation.URL)
	if err != nil {
		t.Fatalf("fetch status: %v", err)
	}
	if !status.Ready || status.ProjectionLag.EventCount != 0 {
		t.Fatalf("status response = %#v, want ready empty daemon", status)
	}
}

func TestDaemonReadyCommandUsesActivationFile(t *testing.T) {
	cwd := t.TempDir()
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithCancel(context.Background())
	server, listener, activation, err := startAgentOpsDaemon(ctx, cwd, agentopsDaemonRunOptions{
		Addr: "127.0.0.1:0",
		Now:  func() time.Time { return now },
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

	oldProjectDir := testProjectDir
	oldOutput := output
	oldURL := daemonURL
	testProjectDir = cwd
	output = "table"
	daemonURL = ""
	t.Cleanup(func() {
		testProjectDir = oldProjectDir
		output = oldOutput
		daemonURL = oldURL
	})

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runAgentOpsDaemonReadyCommand(cmd, nil); err != nil {
		t.Fatalf("ready command: %v", err)
	}
	if !strings.Contains(out.String(), activation.URL) {
		t.Fatalf("ready output %q does not contain activation URL %q", out.String(), activation.URL)
	}
}

func TestDaemonLifecycleDryRunCommand(t *testing.T) {
	// Covers ao daemon service install.
	cwd := t.TempDir()
	oldProjectDir := testProjectDir
	oldDryRun := dryRun
	oldAddr := daemonAddr
	oldExecutable := daemonServiceExecutable
	testProjectDir = cwd
	dryRun = true
	daemonAddr = "127.0.0.1:9876"
	daemonServiceExecutable = "/usr/local/bin/ao"
	t.Cleanup(func() {
		testProjectDir = oldProjectDir
		dryRun = oldDryRun
		daemonAddr = oldAddr
		daemonServiceExecutable = oldExecutable
	})

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runAgentOpsDaemonServiceInstallCommand(cmd, nil); err != nil {
		t.Fatalf("service install dry-run: %v", err)
	}
	got := out.String()
	for _, needle := range []string{`"service_name": "agentopsd"`, `"dry_run": true`, "127.0.0.1:9876"} {
		if !strings.Contains(got, needle) {
			t.Fatalf("service dry-run output missing %q:\n%s", needle, got)
		}
	}
}

func TestDaemonRunRejectsUnsafeActivationBind(t *testing.T) {
	_, _, _, err := startAgentOpsDaemon(context.Background(), t.TempDir(), agentopsDaemonRunOptions{Addr: "0.0.0.0:8765"})
	if err == nil {
		t.Fatal("unsafe daemon bind succeeded")
	}
}
