package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/agentworker"
	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
	"github.com/boshu2/agentops/cli/internal/wikiworker"
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

func TestDaemonRunWorkerOnceCompletesWikiForgeFakeJob(t *testing.T) {
	cwd := t.TempDir()
	queue := daemonpkg.NewQueue(daemonpkg.NewStore(cwd), daemonpkg.QueueOptions{LeaseDuration: time.Minute})
	sourcePath := cwd + "/session-a.jsonl"
	if err := os.WriteFile(sourcePath, []byte("decision: fake wiki forge jobs write session refs\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	spec := daemonpkg.NewWikiForgeJobSpec("dream-1", ".agents/wiki/sources", []string{sourcePath})
	jobSpec, err := spec.ToJobSpec("job-wiki")
	if err != nil {
		t.Fatalf("wiki job spec: %v", err)
	}
	if _, err := queue.SubmitJob(daemonpkg.SubmitJobInput{
		RequestID: "req-wiki",
		JobID:     jobSpec.ID,
		JobType:   jobSpec.Type,
		Payload:   jobSpec.Payload,
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
		t.Fatalf("jobs = %#v, want completed wiki job", snapshot.Jobs)
	}
	if snapshot.Jobs[0].Artifacts["worker_session_refs"] == "" || snapshot.Jobs[0].Artifacts["session_id"] == "" {
		t.Fatalf("wiki artifacts = %#v, want worker session refs", snapshot.Jobs[0].Artifacts)
	}
}

func TestAgentOpsDaemonFakeExecutorPolicyCompletesRPIPhaseJob(t *testing.T) {
	cwd := t.TempDir()
	queue := daemonpkg.NewQueue(daemonpkg.NewStore(cwd), daemonpkg.QueueOptions{LeaseDuration: time.Minute})
	phaseSpec := daemonpkg.NewRPIPhaseJobSpec("run-daemon-fake", "validate daemon rpi executor", 2)
	jobSpec, err := phaseSpec.ToJobSpec("job-rpi-phase")
	if err != nil {
		t.Fatalf("rpi phase job spec: %v", err)
	}
	if _, err := queue.SubmitJob(daemonpkg.SubmitJobInput{
		RequestID: "req-rpi-phase",
		JobID:     jobSpec.ID,
		JobType:   jobSpec.Type,
		Payload:   jobSpec.Payload,
	}, daemonpkg.QueueMutationOptions{}); err != nil {
		t.Fatalf("submit rpi phase job: %v", err)
	}

	supervisor, err := buildAgentOpsDaemonSupervisor(cwd, agentopsDaemonRunOptions{ExecutorPolicy: "fake"})
	if err != nil {
		t.Fatalf("build supervisor: %v", err)
	}
	result, err := supervisor.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run once: %v", err)
	}
	if !result.Claimed || result.Job.Status != daemonpkg.JobStatusCompleted {
		t.Fatalf("result = %#v, want completed rpi phase job", result)
	}
	if got := result.Job.Artifacts["executor_policy"]; got != "fake" {
		t.Fatalf("executor_policy artifact = %q, want fake", got)
	}
	if got := result.Job.Artifacts["phase"]; got != "2" {
		t.Fatalf("phase artifact = %q, want 2", got)
	}
}

func TestAgentOpsDaemonGasCityExecutorPolicyRequiresConfig(t *testing.T) {
	if _, err := buildAgentOpsDaemonSupervisor(t.TempDir(), agentopsDaemonRunOptions{ExecutorPolicy: "gascity"}); err == nil {
		t.Fatal("gascity executor policy without endpoint/city succeeded")
	}
}

func TestResolveAgentOpsDaemonMutationPolicySupportsScopedTokenFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	data := `{"tokens":[{"name":"phone-readonly-submit","token":"phone-token","capabilities":["submit_job"]},{"name":"bushido-admin","token":"admin-token","capabilities":["admin"],"local_only":true}]}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write scoped token file: %v", err)
	}
	policy, err := resolveAgentOpsDaemonMutationPolicy("", path)
	if err != nil {
		t.Fatalf("resolve policy: %v", err)
	}
	if len(policy.Tokens) != 2 || policy.Tokens[0].Name != "phone-readonly-submit" || policy.Token != "" {
		t.Fatalf("policy = %#v", policy)
	}
	if policy.PathCapabilities["/v1/jobs"] != daemonpkg.MutationCapabilitySubmitJob {
		t.Fatalf("path capabilities = %#v", policy.PathCapabilities)
	}
}

func TestAgentOpsDaemonCLIFallbackExecutorPolicyBuilds(t *testing.T) {
	if _, err := buildAgentOpsDaemonSupervisor(t.TempDir(), agentopsDaemonRunOptions{ExecutorPolicy: "cli-fallback"}); err != nil {
		t.Fatalf("cli-fallback executor policy: %v", err)
	}
}

func TestProviderOverrideWikiForgeWorkerForcesCLIFallback(t *testing.T) {
	inner := &recordingWikiForgeWorker{}
	worker := providerOverrideWikiForgeWorker{
		inner:    inner,
		provider: agentworker.ProviderCLIFallback,
	}
	if _, err := worker.RunExtractionWithRetry(context.Background(), wikiworker.ExtractionRequest{
		Provider: agentworker.ProviderGasCity,
		Prompt:   "prompt",
	}, wikiworker.RetryOptions{}); err != nil {
		t.Fatalf("RunExtractionWithRetry: %v", err)
	}
	if inner.req.Provider != agentworker.ProviderCLIFallback {
		t.Fatalf("provider = %q, want cli-fallback", inner.req.Provider)
	}
}

func TestAgentOpsDaemonWorkerFlagsRegistered(t *testing.T) {
	for _, flag := range []string{"workers", "worker-once", "worker-timeout", "worker-memory-max-bytes", "worker-cgroup-root", "executor-policy", "gascity-endpoint", "gascity-city", "gascity-token", "gascity-token-file"} {
		if daemonRunCmd.Flags().Lookup(flag) == nil {
			t.Fatalf("daemon run missing --%s flag", flag)
		}
	}
}

type recordingWikiForgeWorker struct {
	req wikiworker.ExtractionRequest
}

func (w *recordingWikiForgeWorker) RunExtractionWithRetry(_ context.Context, req wikiworker.ExtractionRequest, _ wikiworker.RetryOptions) (wikiworker.ExtractionResult, error) {
	w.req = req
	return wikiworker.ExtractionResult{
		Terminal: agentworker.TerminalState{Status: agentworker.StatusCompleted},
	}, nil
}

// TestAgentopsdRegistersLLMWikiLoopExecutor verifies that the daemon's
// supervisor registers an executor for JobTypeLLMWikiLoop under each policy.
// The executors map is unexported, so we assert via behavior: submit an
// llmwiki.loop job, run RunOnce, and confirm the supervisor claimed and
// executed it (rather than returning ErrNoClaimableJobs).
func TestAgentopsdRegistersLLMWikiLoopExecutor(t *testing.T) {
	cases := []struct {
		name   string
		policy string
		opts   agentopsDaemonRunOptions
	}{
		{name: "fake", policy: "fake"},
		{
			name:   "gascity",
			policy: "gascity",
			opts: agentopsDaemonRunOptions{
				GasCityEndpoint: "http://127.0.0.1:0",
				GasCityCity:     "test-city",
			},
		},
		{name: "cli-fallback", policy: "cli-fallback"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cwd := t.TempDir()
			vault := filepath.Join(cwd, "vault")
			if err := os.MkdirAll(vault, 0o755); err != nil {
				t.Fatalf("mkdir vault: %v", err)
			}
			queue := daemonpkg.NewQueue(daemonpkg.NewStore(cwd), daemonpkg.QueueOptions{LeaseDuration: time.Minute})
			if _, err := queue.SubmitJob(daemonpkg.SubmitJobInput{
				RequestID: "req-llmwiki",
				JobID:     "job-llmwiki",
				JobType:   daemonpkg.JobTypeLLMWikiLoop,
				Payload: map[string]any{
					"vault":  vault,
					"stages": []string{"lint"},
				},
			}, daemonpkg.QueueMutationOptions{}); err != nil {
				t.Fatalf("submit llmwiki job: %v", err)
			}

			opts := tc.opts
			opts.ExecutorPolicy = tc.policy
			supervisor, err := buildAgentOpsDaemonSupervisor(cwd, opts)
			if err != nil {
				t.Fatalf("build supervisor (%s): %v", tc.policy, err)
			}
			result, err := supervisor.RunOnce(context.Background())
			if err != nil {
				t.Fatalf("run once (%s): %v", tc.policy, err)
			}
			if !result.Claimed {
				t.Fatalf("supervisor did not claim llmwiki.loop job under policy %s", tc.policy)
			}
			if result.Job.JobType != daemonpkg.JobTypeLLMWikiLoop {
				t.Fatalf("claimed job type = %s, want %s", result.Job.JobType, daemonpkg.JobTypeLLMWikiLoop)
			}
			if result.Job.Status != daemonpkg.JobStatusCompleted {
				t.Fatalf("llmwiki.loop status = %s, want completed (artifacts=%v)", result.Job.Status, result.Job.Artifacts)
			}
			if got := result.Job.Artifacts["stage"]; got != "lint" {
				t.Fatalf("stage artifact = %q, want lint", got)
			}
		})
	}
}

// TestLLMWikiHarvestAdapter_PromoteDryRun is an L1 sanity check on the
// adapter that bridges PromoteStage to harvest.Promote. With dry-run set
// and no source artifacts, the adapter must succeed without writing to
// destDir.
func TestLLMWikiHarvestAdapter_PromoteDryRun(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	count, err := llmwikiHarvestAdapter{}.Promote(src, dst, true)
	if err != nil {
		t.Fatalf("adapter promote dry-run: %v", err)
	}
	if count != 0 {
		t.Fatalf("dry-run promoted count = %d, want 0", count)
	}
	entries, err := os.ReadDir(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("dry-run wrote %d entries to destDir, want 0", len(entries))
	}
}

func TestLLMWikiHarvestAdapter_RequiresArgs(t *testing.T) {
	if _, err := (llmwikiHarvestAdapter{}).Promote("", "/tmp/x", true); err == nil {
		t.Fatal("expected error when sourceDir empty")
	}
	if _, err := (llmwikiHarvestAdapter{}).Promote("/tmp/x", "", true); err == nil {
		t.Fatal("expected error when destDir empty")
	}
}
