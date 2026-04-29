package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
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

func TestDaemonRunWorkerOnceCompletesFakeJob(t *testing.T) {
	cwd := t.TempDir()
	queue := daemonpkg.NewQueue(daemonpkg.NewStore(cwd), daemonpkg.QueueOptions{LeaseDuration: time.Minute})
	if _, err := queue.SubmitJob(daemonpkg.SubmitJobInput{
		RequestID: "req-openclaw",
		JobID:     "job-openclaw",
		JobType:   daemonpkg.JobTypeOpenClawSnapshot,
	}, daemonpkg.QueueMutationOptions{}); err != nil {
		t.Fatalf("submit job: %v", err)
	}

	prevProjectDir := testProjectDir
	prevAddr := daemonAddr
	prevToken := daemonToken
	prevTokenFile := daemonTokenFile
	prevWorkers := daemonWorkers
	prevWorkerOnce := daemonWorkerOnce
	prevExecutorPolicy := daemonExecutorPolicy
	testProjectDir = cwd
	daemonAddr = "127.0.0.1:0"
	daemonToken = "secret-token"
	daemonTokenFile = ""
	daemonWorkers = 1
	daemonWorkerOnce = true
	daemonExecutorPolicy = "fake"
	t.Cleanup(func() {
		testProjectDir = prevProjectDir
		daemonAddr = prevAddr
		daemonToken = prevToken
		daemonTokenFile = prevTokenFile
		daemonWorkers = prevWorkers
		daemonWorkerOnce = prevWorkerOnce
		daemonExecutorPolicy = prevExecutorPolicy
	})

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runAgentOpsDaemonCommand(cmd, nil); err != nil {
		t.Fatalf("daemon run worker once: %v", err)
	}
	snapshot, err := queue.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snapshot.Jobs) != 1 || snapshot.Jobs[0].Status != daemonpkg.JobStatusCompleted {
		t.Fatalf("jobs = %#v, want completed openclaw job", snapshot.Jobs)
	}
	if !strings.Contains(out.String(), "agentopsd ready:") {
		t.Fatalf("output %q missing ready line", out.String())
	}
}

func TestAgentOpsDaemonWorkerFlagsRegistered(t *testing.T) {
	for _, flag := range []string{"workers", "worker-once", "executor-policy"} {
		if daemonRunCmd.Flags().Lookup(flag) == nil {
			t.Fatalf("daemon run missing --%s flag", flag)
		}
	}
}
