// practices: [microservices, sre]
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
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

func TestResolveAgentOpsDaemonClientMutationTokenPrecedence(t *testing.T) {
	cwd := t.TempDir()
	tokenFile := filepath.Join(cwd, "token")
	if err := os.WriteFile(tokenFile, []byte("file-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	activationTokenFile := filepath.Join(cwd, "activation-token")
	if err := os.WriteFile(activationTokenFile, []byte("activation-file-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeDaemonActivation(cwd, agentopsDaemonActivation{
		URL:       "http://127.0.0.1:8765",
		Address:   "127.0.0.1:8765",
		PID:       123,
		Ready:     true,
		StartedAt: "2026-05-01T00:00:00Z",
		Token:     "activation-token",
		TokenFile: activationTokenFile,
	}); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AGENTOPSD_TOKEN", "env-token")
	t.Setenv("AGENTOPS_DAEMON_TOKEN", "legacy-env-token")

	got, err := resolveAgentOpsDaemonClientMutationToken(cwd, "flag-token", tokenFile)
	if err != nil {
		t.Fatal(err)
	}
	if got != "flag-token" {
		t.Fatalf("flag precedence token = %q", got)
	}

	got, err = resolveAgentOpsDaemonClientMutationToken(cwd, "", tokenFile)
	if err != nil {
		t.Fatal(err)
	}
	if got != "file-token" {
		t.Fatalf("token-file precedence token = %q", got)
	}

	got, err = resolveAgentOpsDaemonClientMutationToken(cwd, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "env-token" {
		t.Fatalf("env precedence token = %q", got)
	}

	t.Setenv("AGENTOPSD_TOKEN", "")
	got, err = resolveAgentOpsDaemonClientMutationToken(cwd, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "legacy-env-token" {
		t.Fatalf("legacy env precedence token = %q", got)
	}

	t.Setenv("AGENTOPS_DAEMON_TOKEN", "")
	got, err = resolveAgentOpsDaemonClientMutationToken(cwd, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "activation-token" {
		t.Fatalf("activation fallback token = %q", got)
	}

	if err := writeDaemonActivation(cwd, agentopsDaemonActivation{
		URL:       "http://127.0.0.1:8765",
		Address:   "127.0.0.1:8765",
		PID:       123,
		Ready:     true,
		StartedAt: "2026-05-01T00:00:00Z",
		TokenFile: activationTokenFile,
	}); err != nil {
		t.Fatal(err)
	}
	got, err = resolveAgentOpsDaemonClientMutationToken(cwd, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "activation-file-token" {
		t.Fatalf("activation token-file fallback token = %q", got)
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
		t.Fatalf(
			"jobs = %#v, failure = %#v, artifacts = %#v, artifact refs = %#v, want completed wiki job",
			snapshot.Jobs,
			snapshot.Jobs[0].Failure,
			snapshot.Jobs[0].Artifacts,
			snapshot.Jobs[0].ArtifactRefs,
		)
	}
	if snapshot.Jobs[0].Artifacts["worker_session_refs"] == "" || snapshot.Jobs[0].Artifacts["session_id"] == "" {
		t.Fatalf("wiki artifacts = %#v, want worker session refs", snapshot.Jobs[0].Artifacts)
	}
}

func TestAgentOpsDaemonFakeExecutorPolicyCompletesRPIRunJob(t *testing.T) {
	cwd := t.TempDir()
	queue := daemonpkg.NewQueue(daemonpkg.NewStore(cwd), daemonpkg.QueueOptions{LeaseDuration: time.Minute})
	spec := daemonpkg.NewRPIRunJobSpec("run-daemon-fake", "validate daemon rpi run")
	spec.MaxPhase = 1
	jobSpec, err := spec.ToJobSpec("job-rpi-run")
	if err != nil {
		t.Fatalf("rpi run job spec: %v", err)
	}
	if _, err := queue.SubmitJob(daemonpkg.SubmitJobInput{
		RequestID: "req-rpi-run",
		JobID:     jobSpec.ID,
		JobType:   jobSpec.Type,
		Payload:   jobSpec.Payload,
	}, daemonpkg.QueueMutationOptions{}); err != nil {
		t.Fatalf("submit rpi run job: %v", err)
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
		t.Fatalf("result = %#v, want completed rpi.run job", result)
	}
	if result.Job.JobType != daemonpkg.JobTypeRPIRun {
		t.Fatalf("job type = %s, want %s", result.Job.JobType, daemonpkg.JobTypeRPIRun)
	}
	if got := result.Job.Artifacts["executor_policy"]; got != "fake" {
		t.Fatalf("executor_policy artifact = %q, want fake", got)
	}
	if got := result.Job.Artifacts["phase"]; got != "1" {
		t.Fatalf("phase artifact = %q, want 1", got)
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

func TestAgentOpsDaemonFakeExecutorPolicyClaimsFactoryAdmissionJob(t *testing.T) {
	cwd := t.TempDir()
	queue := daemonpkg.NewQueue(daemonpkg.NewStore(cwd), daemonpkg.QueueOptions{LeaseDuration: time.Minute})
	work := daemonpkg.FactoryWorkOrder{
		SchemaVersion: daemonpkg.FactoryAdmissionJobSpecSchemaVersion,
		WorkOrderID:   "factory-work-daemon-test",
		GeneratedAt:   "2026-05-04T23:30:00Z",
		ExpiresAt:     "2026-05-05T00:30:00Z",
		BaseSHA:       "abcdef123456",
		Target: daemonpkg.FactoryTarget{
			Type:    daemonpkg.FactoryTargetBead,
			ID:      "soc-ff7b.7",
			Summary: "Exercise daemon factory admission registration",
		},
		AllowedFiles: []string{
			"cli/internal/daemon/factory_admission_executor.go",
		},
		ValidationCommands: []string{
			"cd cli && go test ./internal/daemon -run FactoryAdmission",
		},
		LandingPolicy:         daemonpkg.FactoryLandingPolicyManualPR,
		DigestPolicy:          daemonpkg.FactoryDigestPolicyRequired,
		OpenPRBlockers:        []daemonpkg.FactoryOpenPRBlocker{},
		UnknownEvidencePolicy: daemonpkg.FactoryUnknownEvidenceBlock,
		MainCIBaseline: daemonpkg.FactoryMainCIBaseline{
			Status:     daemonpkg.FactoryCIStatusGreen,
			RunID:      "123",
			CheckedAt:  "2026-05-04T23:29:00Z",
			FailedJobs: []string{},
		},
	}
	spec := daemonpkg.NewFactoryAdmissionJobSpec("factory-run-daemon-test", work)
	jobSpec, err := spec.ToJobSpec("job-factory-admission")
	if err != nil {
		t.Fatalf("factory admission job spec: %v", err)
	}
	if _, err := queue.SubmitJob(daemonpkg.SubmitJobInput{
		RequestID: "req-factory-admission",
		JobID:     jobSpec.ID,
		JobType:   jobSpec.Type,
		Payload:   jobSpec.Payload,
	}, daemonpkg.QueueMutationOptions{}); err != nil {
		t.Fatalf("submit factory admission job: %v", err)
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
		t.Fatalf("result = %#v, want completed factory admission job", result)
	}
	if got := result.Job.Artifacts["allowed"]; got != "false" {
		t.Fatalf("allowed artifact = %q, want false because temp dir is not a git repo", got)
	}
	if result.Job.Artifacts["admission"] == "" {
		t.Fatalf("artifacts = %#v, want admission artifact", result.Job.Artifacts)
	}
}

func TestAgentOpsDaemonFakeFactoryLocalPilotHandoffCompletesChildRPI(t *testing.T) {
	cwd := t.TempDir()
	headSHA := initDaemonFactoryGitRepo(t, cwd)
	now := time.Date(2026, 5, 5, 2, 30, 0, 0, time.UTC)
	store := daemonpkg.NewStore(cwd)
	queue := daemonpkg.NewQueue(store, daemonpkg.QueueOptions{LeaseDuration: time.Minute})
	work := daemonpkg.FactoryWorkOrder{
		SchemaVersion: daemonpkg.FactoryAdmissionJobSpecSchemaVersion,
		WorkOrderID:   "factory-l3-rehearsal",
		GeneratedAt:   now.Add(-10 * time.Minute).Format(time.RFC3339),
		ExpiresAt:     now.Add(time.Hour).Format(time.RFC3339),
		BaseSHA:       headSHA,
		Target: daemonpkg.FactoryTarget{
			Type:    daemonpkg.FactoryTargetBead,
			ID:      "soc-ff7b.7",
			Summary: "Exercise daemon factory local pilot handoff",
		},
		AllowedFiles: []string{
			"cli/internal/daemon/factory_admission_executor.go",
		},
		ValidationCommands: []string{
			"cd cli && go test ./internal/daemon -run FactoryAdmission",
		},
		LandingPolicy:         daemonpkg.FactoryLandingPolicyManualPR,
		DigestPolicy:          daemonpkg.FactoryDigestPolicyRequired,
		OpenPRBlockers:        []daemonpkg.FactoryOpenPRBlocker{},
		UnknownEvidencePolicy: daemonpkg.FactoryUnknownEvidenceBlock,
		MainCIBaseline: daemonpkg.FactoryMainCIBaseline{
			Status:     daemonpkg.FactoryCIStatusGreen,
			RunID:      "25352104726",
			CheckedAt:  now.Add(-15 * time.Minute).Format(time.RFC3339),
			FailedJobs: []string{},
		},
	}
	spec := daemonpkg.NewFactoryLocalPilotJobSpec("factory-l3-run", work)
	spec.Mode = daemonpkg.FactoryAdmissionModeRPIHandoff
	spec.Handoff = daemonpkg.FactoryHandoff{
		Kind:                daemonpkg.FactoryHandoffRPI,
		ExecutionPacketPath: ".agents/rpi/runs/factory-l3-run/execution-packet.json",
		EpicID:              "soc-ff7b.7",
	}
	jobSpec, err := spec.ToJobSpec("job-factory-l3")
	if err != nil {
		t.Fatalf("factory local pilot spec: %v", err)
	}
	if _, err := queue.SubmitJob(daemonpkg.SubmitJobInput{
		RequestID: "req-factory-l3",
		JobID:     jobSpec.ID,
		JobType:   jobSpec.Type,
		Payload:   jobSpec.Payload,
	}, daemonpkg.QueueMutationOptions{}); err != nil {
		t.Fatalf("submit factory local pilot: %v", err)
	}

	supervisor, err := buildAgentOpsDaemonSupervisor(cwd, agentopsDaemonRunOptions{
		ExecutorPolicy: "fake",
		Now:            func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("build supervisor: %v", err)
	}
	admissionResult, err := supervisor.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run factory admission: %v", err)
	}
	if !admissionResult.Claimed || admissionResult.Job.Status != daemonpkg.JobStatusCompleted {
		t.Fatalf("admission result = %#v, want completed factory.local-pilot", admissionResult)
	}
	childJobID := admissionResult.Job.Artifacts["child_job_id"]
	if childJobID == "" {
		t.Fatalf("admission artifacts = %#v, want child_job_id", admissionResult.Job.Artifacts)
	}

	childResult, err := supervisor.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run child rpi job: %v", err)
	}
	if !childResult.Claimed || childResult.Job.JobID != childJobID || childResult.Job.Status != daemonpkg.JobStatusCompleted {
		t.Fatalf("child result = %#v, want completed child rpi job %s", childResult, childJobID)
	}
	if got := childResult.Job.Artifacts["executor_policy"]; got != "fake" {
		t.Fatalf("child executor_policy = %q, want fake", got)
	}

	projections, err := store.RebuildProjections(daemonpkg.ProjectionRebuildOptions{RebuiltAt: now})
	if err != nil {
		t.Fatalf("rebuild projections: %v", err)
	}
	if len(projections.Factory.Admissions) != 1 {
		t.Fatalf("admissions = %#v, want one admission", projections.Factory.Admissions)
	}
	admission := projections.Factory.Admissions[0]
	if !admission.Allowed || admission.ChildJobID != childJobID {
		t.Fatalf("admission projection = %#v, want allowed child handoff", admission)
	}
	if len(projections.Jobs) != 2 {
		t.Fatalf("queue projection jobs = %#v, want parent and child jobs", projections.Jobs)
	}
}

func TestAgentOpsDaemonFakeExecutorPolicyRoutesDreamRunJob(t *testing.T) {
	cwd := t.TempDir()
	queue := daemonpkg.NewQueue(daemonpkg.NewStore(cwd), daemonpkg.QueueOptions{LeaseDuration: time.Minute})
	if _, err := queue.SubmitJob(daemonpkg.SubmitJobInput{
		RequestID: "req-dream-invalid",
		JobID:     "job-dream-invalid",
		JobType:   daemonpkg.JobTypeDreamRun,
		Payload:   map[string]any{"schema_version": 1, "job_type": string(daemonpkg.JobTypeDreamRun)},
	}, daemonpkg.QueueMutationOptions{}); err != nil {
		t.Fatalf("submit dream job: %v", err)
	}

	supervisor, err := buildAgentOpsDaemonSupervisor(cwd, agentopsDaemonRunOptions{ExecutorPolicy: "fake"})
	if err != nil {
		t.Fatalf("build supervisor: %v", err)
	}
	result, err := supervisor.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run once: %v", err)
	}
	if !result.Claimed {
		t.Fatal("supervisor did not claim dream.run job under fake policy")
	}
	if result.Job.JobType != daemonpkg.JobTypeDreamRun || result.Job.Status != daemonpkg.JobStatusFailed {
		t.Fatalf("dream result = %#v, want failed dream.run job", result.Job)
	}
	if result.Job.Failure == nil || !strings.Contains(result.Job.Failure.Message, "dream_run_id is required") {
		t.Fatalf("dream failure = %#v, want payload validation failure", result.Job.Failure)
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

func TestAgentOpsDaemonMutationPathsIncludeRESTAliasAndSchedules(t *testing.T) {
	paths := map[string]bool{}
	for _, path := range agentOpsDaemonMutationPaths() {
		paths[path] = true
	}
	for _, want := range []string{
		"/v1/jobs/*/cancel",
		"/v1/schedules",
		"/v1/schedules/*",
	} {
		if !paths[want] {
			t.Fatalf("agentOpsDaemonMutationPaths missing %q: %v", want, agentOpsDaemonMutationPaths())
		}
	}
}

func TestAgentOpsDaemonCLIFallbackExecutorPolicyBuilds(t *testing.T) {
	if _, err := buildAgentOpsDaemonSupervisor(t.TempDir(), agentopsDaemonRunOptions{ExecutorPolicy: "cli-fallback"}); err != nil {
		t.Fatalf("cli-fallback executor policy: %v", err)
	}
}

func TestAgentOpsDaemonCLIFallbackExecutorPolicyCompletesRPIRunJob(t *testing.T) {
	cwd := t.TempDir()
	queue := daemonpkg.NewQueue(daemonpkg.NewStore(cwd), daemonpkg.QueueOptions{LeaseDuration: time.Minute})
	spec := daemonpkg.NewRPIRunJobSpec("run-daemon-cli", "validate daemon cli fallback rpi run")
	jobSpec, err := spec.ToJobSpec("job-rpi-cli")
	if err != nil {
		t.Fatalf("rpi run job spec: %v", err)
	}
	if _, err := queue.SubmitJob(daemonpkg.SubmitJobInput{
		RequestID: "req-rpi-cli",
		JobID:     jobSpec.ID,
		JobType:   jobSpec.Type,
		Payload:   jobSpec.Payload,
	}, daemonpkg.QueueMutationOptions{}); err != nil {
		t.Fatalf("submit rpi run job: %v", err)
	}

	// soc-bcrn.3.6 (E3.W4 sub-5a): the CLI-fallback executor now runs the
	// phased RPI engine in-process; tests inject a fake runner instead of
	// shelling out to scripts/ao-rpi-autonomous-cycle.sh.
	var runnerCalls int
	fakeRun := func(_ context.Context, req daemonpkg.RPIRunRequest) (daemonpkg.RPIRunResult, error) {
		runnerCalls++
		if req.Spec.RunID != "run-daemon-cli" || req.Spec.Goal == "" {
			t.Errorf("runner spec = %#v, want run-daemon-cli with non-empty goal", req.Spec)
		}
		if req.Root != cwd {
			t.Errorf("runner root = %q, want %q", req.Root, cwd)
		}
		return daemonpkg.RPIRunResult{Artifacts: map[string]string{
			"rpi_run_status": "completed",
			"runner_marker":  "in-process-ok",
		}}, nil
	}

	supervisor, err := buildAgentOpsDaemonSupervisor(cwd, agentopsDaemonRunOptions{
		ExecutorPolicy:        "cli-fallback",
		CLIFallbackRPIRunFunc: fakeRun,
	})
	if err != nil {
		t.Fatalf("build supervisor: %v", err)
	}
	result, err := supervisor.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run once: %v", err)
	}
	if !result.Claimed || result.Job.Status != daemonpkg.JobStatusCompleted {
		t.Fatalf("result = %#v, want completed rpi.run job", result)
	}
	if runnerCalls != 1 {
		t.Fatalf("runner called %d times, want 1", runnerCalls)
	}
	if got := result.Job.Artifacts["executor_policy"]; got != "in-process" {
		t.Fatalf("executor_policy artifact = %q, want in-process", got)
	}
	if got := result.Job.Artifacts["backend"]; got != "in-process" {
		t.Fatalf("backend artifact = %q, want in-process", got)
	}
	if got := result.Job.Artifacts["rpi_run_status"]; got != "completed" {
		t.Fatalf("rpi_run_status artifact = %q, want completed", got)
	}
	if got := result.Job.Artifacts["runner_marker"]; got != "in-process-ok" {
		t.Fatalf("runner_marker artifact = %q, want in-process-ok", got)
	}
}

// TestBuildSupervisorConfigFromSpec_MapsPolicyFields covers the spec→cfg
// translation introduced by soc-bcrn.3.8 (E3.W4 sub-5a-fix). Daemon-submitted
// rpi.run jobs must propagate gate, landing, BD-sync, failure, and
// kill-switch policy onto the supervisor config so the supervisor loop
// enforces the same semantics as `ao rpi loop --supervisor`.
func TestBuildSupervisorConfigFromSpec_MapsPolicyFields(t *testing.T) {
	root := t.TempDir()
	spec := daemonpkg.NewRPIRunJobSpec("run-cfg", "validate spec to cfg mapping")
	spec.GatePolicy = "required"
	spec.LandingPolicy = "commit"
	spec.LandingBranch = "release/v3"
	spec.MaxCycles = 3
	spec.BDSyncPolicy = "always"
	spec.FailurePolicy = "continue"
	spec.KillSwitchPath = filepath.Join(root, "custom-kill")

	cfg, err := buildSupervisorConfigFromSpec(spec, root)
	if err != nil {
		t.Fatalf("buildSupervisorConfigFromSpec: %v", err)
	}
	if cfg.GatePolicy != "required" {
		t.Errorf("GatePolicy = %q, want required", cfg.GatePolicy)
	}
	if cfg.LandingPolicy != "commit" {
		t.Errorf("LandingPolicy = %q, want commit", cfg.LandingPolicy)
	}
	if cfg.LandingBranch != "release/v3" {
		t.Errorf("LandingBranch = %q, want release/v3", cfg.LandingBranch)
	}
	if cfg.MaxCycles != 3 {
		t.Errorf("MaxCycles = %d, want 3", cfg.MaxCycles)
	}
	if cfg.BDSyncPolicy != "always" {
		t.Errorf("BDSyncPolicy = %q, want always", cfg.BDSyncPolicy)
	}
	if cfg.FailurePolicy != "continue" {
		t.Errorf("FailurePolicy = %q, want continue", cfg.FailurePolicy)
	}
	if cfg.KillSwitchPath != filepath.Join(root, "custom-kill") {
		t.Errorf("KillSwitchPath = %q, want absolute path under root", cfg.KillSwitchPath)
	}
	if cfg.LeaseEnabled {
		t.Errorf("LeaseEnabled = true, want false (daemon owns the queue-level lease)")
	}
}

// TestBuildSupervisorConfigFromSpec_DefaultsWhenSpecEmpty checks that a spec
// without any supervisor policy fields produces a cfg matching the
// supervisor's safe defaults (gate-policy=off, landing-policy=off,
// failure-policy=stop). Path defaults must still resolve relative to root.
func TestBuildSupervisorConfigFromSpec_DefaultsWhenSpecEmpty(t *testing.T) {
	root := t.TempDir()
	spec := daemonpkg.NewRPIRunJobSpec("run-default-cfg", "no policy fields")

	cfg, err := buildSupervisorConfigFromSpec(spec, root)
	if err != nil {
		t.Fatalf("buildSupervisorConfigFromSpec: %v", err)
	}
	if cfg.GatePolicy != "off" {
		t.Errorf("GatePolicy = %q, want off (default)", cfg.GatePolicy)
	}
	if cfg.LandingPolicy != "off" {
		t.Errorf("LandingPolicy = %q, want off (default)", cfg.LandingPolicy)
	}
	if cfg.FailurePolicy != "stop" {
		t.Errorf("FailurePolicy = %q, want stop (default)", cfg.FailurePolicy)
	}
	if cfg.LeaseEnabled {
		t.Errorf("LeaseEnabled = true, want false (daemon owns the queue-level lease)")
	}
	if !filepath.IsAbs(cfg.KillSwitchPath) {
		t.Errorf("KillSwitchPath = %q, want absolute (resolved against root)", cfg.KillSwitchPath)
	}
	if !strings.HasPrefix(cfg.KillSwitchPath, root) {
		t.Errorf("KillSwitchPath = %q, want prefix %q", cfg.KillSwitchPath, root)
	}
}

// TestBuildSupervisorConfigFromSpec_RejectsInvalidEnum guards against
// silently accepting a typo in a policy field. validateLoopConfigPolicies
// is the same enum gate the cobra path runs, so daemon submitters get the
// same diagnostics.
func TestBuildSupervisorConfigFromSpec_RejectsInvalidEnum(t *testing.T) {
	spec := daemonpkg.NewRPIRunJobSpec("run-bad", "invalid enum should fail")
	spec.GatePolicy = "ON" // not a valid value

	if _, err := buildSupervisorConfigFromSpec(spec, t.TempDir()); err == nil {
		t.Fatal("expected error for invalid GatePolicy")
	}
}

func TestAgentOpsDaemonPlansProjectionExecutorRegistration(t *testing.T) {
	cwd := t.TempDir()
	queue := daemonpkg.NewQueue(daemonpkg.NewStore(cwd), daemonpkg.QueueOptions{LeaseDuration: time.Minute})
	spec := daemonpkg.NewPlansProjectionJobSpec("soak-project", "soc", filepath.Join(cwd, ".agents", "plans", "soak"))
	specRaw, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal plans spec: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(specRaw, &payload); err != nil {
		t.Fatalf("unmarshal plans spec: %v", err)
	}
	if _, err := queue.SubmitJob(daemonpkg.SubmitJobInput{
		RequestID:      "req-plans-projection",
		JobID:          "job-plans-projection",
		JobType:        daemonpkg.JobTypePlansProjection,
		IdempotencyKey: spec.IdempotencyKey(),
		Payload:        payload,
	}, daemonpkg.QueueMutationOptions{}); err != nil {
		t.Fatalf("submit plans job: %v", err)
	}

	supervisor, err := buildAgentOpsDaemonSupervisor(cwd, agentopsDaemonRunOptions{
		ExecutorPolicy: "fake",
		PlansBdSource:  plansProjectionSoakBdSource{},
		Now:            fixedDaemonSoakNow,
	})
	if err != nil {
		t.Fatalf("build supervisor: %v", err)
	}
	result, err := supervisor.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run once: %v", err)
	}
	if !result.Claimed || result.Job.Status != daemonpkg.JobStatusCompleted {
		t.Fatalf("result = %#v, want completed plans.projection job", result)
	}
	if got := result.Job.Artifacts["manifest_count"]; got != "2" {
		t.Fatalf("manifest_count = %q, want 2", got)
	}
	manifestPath := result.Job.Artifacts["manifest_jsonl"]
	if manifestPath == "" {
		t.Fatalf("plans artifacts = %#v, want manifest_jsonl", result.Job.Artifacts)
	}
	if _, err := os.Stat(filepath.Clean(manifestPath)); err != nil {
		t.Fatalf("plans manifest %s: %v", manifestPath, err)
	}
}

func initDaemonFactoryGitRepo(t *testing.T, cwd string) string {
	t.Helper()
	runDaemonFactoryGit(t, cwd, "init", "-q")
	runDaemonFactoryGit(t, cwd, "config", "user.email", "test@example.com")
	runDaemonFactoryGit(t, cwd, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(cwd, "README.md"), []byte("factory rehearsal\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cwd, ".gitignore"), []byte(".agents/\n"), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
	runDaemonFactoryGit(t, cwd, "add", "README.md", ".gitignore")
	runDaemonFactoryGit(t, cwd, "commit", "-q", "-m", "init")
	return strings.TrimSpace(runDaemonFactoryGitOutput(t, cwd, "rev-parse", "HEAD"))
}

func runDaemonFactoryGit(t *testing.T, cwd string, args ...string) {
	t.Helper()
	_ = runDaemonFactoryGitOutput(t, cwd, args...)
}

func runDaemonFactoryGitOutput(t *testing.T, cwd string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
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

func TestAgentOpsDaemonAcceptsWikiBuildIntoReadModels(t *testing.T) {
	cwd := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	server, listener, activation, err := startAgentOpsDaemon(ctx, cwd, agentopsDaemonRunOptions{
		Addr:  "127.0.0.1:0",
		Token: "secret-token",
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

	response, err := postDaemonSubmitJob(context.Background(), activation.URL, "secret-token", daemonpkg.SubmitJobRequest{
		RequestID:      "req-wiki-build",
		JobID:          "job-wiki-build",
		JobType:        daemonpkg.JobTypeWikiBuild,
		IdempotencyKey: "wiki.build:read-model-test",
	})
	if err != nil {
		t.Fatalf("submit wiki.build: %v", err)
	}
	if !response.Accepted || response.JobID != "job-wiki-build" {
		t.Fatalf("submit response = %#v, want accepted wiki.build", response)
	}

	status, err := fetchDaemonStatus(context.Background(), activation.URL)
	if err != nil {
		t.Fatalf("fetch status: %v", err)
	}
	if job := findDaemonQueueJob(status.Queue.Jobs, "job-wiki-build"); job == nil || job.JobType != daemonpkg.JobTypeWikiBuild {
		t.Fatalf("queue jobs = %#v, want queued wiki.build", status.Queue.Jobs)
	}
	if len(status.Projections.Wiki.Jobs) != 1 || status.Projections.Wiki.Jobs[0].JobID != "job-wiki-build" {
		t.Fatalf("wiki projection jobs = %#v, want wiki.build job", status.Projections.Wiki.Jobs)
	}
	if len(status.Projections.OpenClaw.Resources.Wiki) != 1 || status.Projections.OpenClaw.Resources.Wiki[0].JobID != "job-wiki-build" {
		t.Fatalf("openclaw wiki resources = %#v, want wiki.build resource", status.Projections.OpenClaw.Resources.Wiki)
	}
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

func TestAgentopsdRegistersSkillInvokeExecutor(t *testing.T) {
	cwd := t.TempDir()
	queue := daemonpkg.NewQueue(daemonpkg.NewStore(cwd), daemonpkg.QueueOptions{LeaseDuration: time.Minute})
	if _, err := queue.SubmitJob(daemonpkg.SubmitJobInput{
		RequestID: "req-skill-invoke",
		JobID:     "job-skill-invoke",
		JobType:   daemonpkg.JobTypeSkillInvoke,
		Payload: map[string]any{
			"skill_name": "compile",
			"args":       []string{"--full"},
		},
	}, daemonpkg.QueueMutationOptions{}); err != nil {
		t.Fatalf("submit skill.invoke job: %v", err)
	}

	supervisor, err := buildAgentOpsDaemonSupervisor(cwd, agentopsDaemonRunOptions{
		ExecutorPolicy: "fake",
		SkillInvokeRunFunc: func(_ context.Context, req daemonpkg.SkillInvokeRequest) (daemonpkg.SkillInvokeResult, error) {
			if req.Spec.SkillName != "compile" {
				t.Fatalf("skill_name = %q, want compile", req.Spec.SkillName)
			}
			return daemonpkg.SkillInvokeResult{Artifacts: map[string]string{"runner": "fake-skill"}}, nil
		},
	})
	if err != nil {
		t.Fatalf("build supervisor: %v", err)
	}
	result, err := supervisor.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run once: %v", err)
	}
	if !result.Claimed || result.Job.JobType != daemonpkg.JobTypeSkillInvoke {
		t.Fatalf("result = %#v, want claimed skill.invoke job", result)
	}
	if result.Job.Status != daemonpkg.JobStatusCompleted {
		t.Fatalf("skill.invoke status = %s, want completed", result.Job.Status)
	}
	if result.Job.Artifacts["runner"] != "fake-skill" {
		t.Fatalf("artifacts = %#v, want fake runner marker", result.Job.Artifacts)
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

// TestAgentopsdLoadsScheduleFileOnStart verifies that --schedule-file=<path>
// causes the daemon to parse the file and persist its schedules into the
// Store before serving. (soc-8inr.17, amendment B5)
func TestAgentopsdLoadsScheduleFileOnStart(t *testing.T) {
	cwd := t.TempDir()
	schedulePath := filepath.Join(cwd, "custom-schedule.yaml")
	if err := os.WriteFile(schedulePath, []byte("schedules:\n  - name: alpha\n    cron: \"*/5 * * * *\"\n    job_type: llmwiki.loop\n"), 0o600); err != nil {
		t.Fatalf("write schedule file: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	server, listener, _, err := startAgentOpsDaemon(ctx, cwd, agentopsDaemonRunOptions{
		Addr:         "127.0.0.1:0",
		ScheduleFile: schedulePath,
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

	store := daemonpkg.NewStore(cwd)
	schedules, err := store.ListSchedules()
	if err != nil {
		t.Fatalf("ListSchedules: %v", err)
	}
	if len(schedules) != 1 {
		t.Fatalf("len(schedules) = %d, want 1 (got %#v)", len(schedules), schedules)
	}
	if schedules[0].Name != "alpha" {
		t.Fatalf("schedules[0].Name = %q, want %q", schedules[0].Name, "alpha")
	}
	if schedules[0].JobType != daemonpkg.JobTypeLLMWikiLoop {
		t.Fatalf("schedules[0].JobType = %q, want %q", schedules[0].JobType, daemonpkg.JobTypeLLMWikiLoop)
	}
}

// TestAgentopsdNoScheduleFile_BehavesIdenticallyToPreUpgrade verifies that
// when no --schedule-file flag is set AND no default .agents/schedule.yaml
// exists, the daemon starts cleanly with zero schedules — bit-identical to
// pre-upgrade behavior. (soc-8inr.17, binding test)
func TestAgentopsdNoScheduleFile_BehavesIdenticallyToPreUpgrade(t *testing.T) {
	cwd := t.TempDir()
	// Sanity: ensure default file does NOT exist.
	if _, err := os.Stat(filepath.Join(cwd, ".agents", "schedule.yaml")); !os.IsNotExist(err) {
		t.Fatalf("precondition: default schedule.yaml should not exist (err=%v)", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	server, listener, activation, err := startAgentOpsDaemon(ctx, cwd, agentopsDaemonRunOptions{
		Addr: "127.0.0.1:0",
		// ScheduleFile intentionally empty.
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

	if !activation.Ready || activation.URL == "" {
		t.Fatalf("activation = %#v, want ready URL", activation)
	}
	store := daemonpkg.NewStore(cwd)
	schedules, err := store.ListSchedules()
	if err != nil {
		t.Fatalf("ListSchedules: %v", err)
	}
	if len(schedules) != 0 {
		t.Fatalf("len(schedules) = %d, want 0 (no-schedule-file path must not load anything)", len(schedules))
	}
}

// TestAgentopsdMalformedScheduleFile_FailsClosed verifies that a malformed
// schedule.yaml causes the daemon start function to return an error
// (refuses to boot) rather than silently starting with zero schedules.
// Amendment C9 requirement.
func TestAgentopsdMalformedScheduleFile_FailsClosed(t *testing.T) {
	cwd := t.TempDir()
	schedulePath := filepath.Join(cwd, "malformed.yaml")
	// `schedules: not-a-list` — strict decoder should reject this because
	// the field is typed as a slice of structs.
	if err := os.WriteFile(schedulePath, []byte("schedules: not-a-list\n"), 0o600); err != nil {
		t.Fatalf("write malformed schedule file: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server, listener, _, err := startAgentOpsDaemon(ctx, cwd, agentopsDaemonRunOptions{
		Addr:         "127.0.0.1:0",
		ScheduleFile: schedulePath,
	})
	if err == nil {
		// Should not have returned a server.
		if listener != nil {
			_ = listener.Close()
		}
		if server != nil {
			_ = server.Close()
		}
		t.Fatalf("expected start to fail-closed on malformed schedule.yaml, got nil error")
	}
	msg := err.Error()
	if !strings.Contains(msg, schedulePath) {
		t.Fatalf("error message %q must contain schedule path %q", msg, schedulePath)
	}
	// Per amendment C9, the operator-facing message must mention "fix" or
	// "malformed" so it's actionable.
	if !strings.Contains(msg, "fix") && !strings.Contains(msg, "malformed") {
		t.Fatalf("error message %q must mention 'fix' or 'malformed' (amendment C9)", msg)
	}
}

// TestAgentopsdScheduleFileFlag_OverridesAutoDetect verifies that when both
// .agents/schedule.yaml and a flag-supplied path exist, the flag wins.
// (soc-8inr.17 precedence rule)
func TestAgentopsdScheduleFileFlag_OverridesAutoDetect(t *testing.T) {
	cwd := t.TempDir()
	// File A: default location, schedule named "alpha"
	defaultDir := filepath.Join(cwd, ".agents")
	if err := os.MkdirAll(defaultDir, 0o755); err != nil {
		t.Fatalf("mkdir .agents: %v", err)
	}
	if err := os.WriteFile(filepath.Join(defaultDir, "schedule.yaml"), []byte("schedules:\n  - name: alpha\n    cron: \"*/5 * * * *\"\n    job_type: llmwiki.loop\n"), 0o600); err != nil {
		t.Fatalf("write default schedule file: %v", err)
	}
	// File B: custom path, schedule named "beta"
	customPath := filepath.Join(cwd, "custom.yaml")
	if err := os.WriteFile(customPath, []byte("schedules:\n  - name: beta\n    cron: \"*/5 * * * *\"\n    job_type: llmwiki.loop\n"), 0o600); err != nil {
		t.Fatalf("write custom schedule file: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	server, listener, _, err := startAgentOpsDaemon(ctx, cwd, agentopsDaemonRunOptions{
		Addr:         "127.0.0.1:0",
		ScheduleFile: customPath, // flag wins
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

	store := daemonpkg.NewStore(cwd)
	schedules, err := store.ListSchedules()
	if err != nil {
		t.Fatalf("ListSchedules: %v", err)
	}
	if len(schedules) != 1 {
		t.Fatalf("len(schedules) = %d, want 1; flag must win over auto-detect (got %#v)", len(schedules), schedules)
	}
	if schedules[0].Name != "beta" {
		t.Fatalf("schedules[0].Name = %q, want %q (flag must win over auto-detect)", schedules[0].Name, "beta")
	}
}
