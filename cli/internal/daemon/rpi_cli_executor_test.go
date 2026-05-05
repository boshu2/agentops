package daemon

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNewRPICLIExecutorRequiresRoot(t *testing.T) {
	if _, err := NewRPICLIExecutor(RPICLIExecutorOptions{}); err == nil {
		t.Fatal("expected error when root is empty")
	}
}

func TestRPICLIExecutorJobTypesCoversRPIRunOnly(t *testing.T) {
	exec, err := NewRPICLIExecutor(RPICLIExecutorOptions{Root: t.TempDir()})
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}
	if got, want := exec.JobTypes(), []JobType{JobTypeRPIRun}; !reflect.DeepEqual(got, want) {
		t.Fatalf("JobTypes = %v, want %v", got, want)
	}
}

func TestRPICLIExecutorRejectsRPIPhaseJob(t *testing.T) {
	exec, err := NewRPICLIExecutor(RPICLIExecutorOptions{Root: t.TempDir()})
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}
	claim := QueueClaim{Job: QueueJobState{JobID: "job-phase", JobType: JobTypeRPIPhase}}
	_, runErr := exec.RunJob(context.Background(), claim)
	if runErr == nil || !strings.Contains(runErr.Error(), "does not support") {
		t.Fatalf("RunJob error = %v, want unsupported type", runErr)
	}
}

func TestRPICLIExecutorMapsRunPayloadToSafeWrapperArgs(t *testing.T) {
	root := t.TempDir()
	script := writeExecutableRPITestScript(t, root, "exit 0\n")
	var got RPICLICommand
	exec, err := NewRPICLIExecutor(RPICLIExecutorOptions{
		Root:       root,
		ScriptPath: script,
		CommandRunner: func(_ context.Context, command RPICLICommand) error {
			got = command
			return nil
		},
	})
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}
	spec := NewRPIRunJobSpec("run-cli", "land soc-ff7b.3")
	spec.Backend = RPIBackendGasCityAPI
	spec.EpicID = "soc-ff7b"
	spec.ExecutionPacketPath = ".agents/rpi/runs/run-cli/execution-packet.json"
	spec.PhaseTimeout = "5m0s"
	jobSpec, err := spec.ToJobSpec("job-rpi-cli")
	if err != nil {
		t.Fatalf("job spec: %v", err)
	}
	claim := QueueClaim{
		Job: QueueJobState{
			JobID:     jobSpec.ID,
			RequestID: "req-rpi-cli",
			JobType:   jobSpec.Type,
			Payload:   jobSpec.Payload,
		},
	}
	result, err := exec.RunJob(context.Background(), claim)
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	wantArgs := []string{
		"--goal", "land soc-ff7b.3",
		"--max-cycles", "1",
		"--gate-policy", "required",
		"--landing-policy", "off",
		"--failure-policy", "stop",
		"--phase-timeout", "5m0s",
	}
	if !reflect.DeepEqual(got.Args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", got.Args, wantArgs)
	}
	if got.Root != root || got.Script != script {
		t.Fatalf("command root/script = %q/%q, want %q/%q", got.Root, got.Script, root, script)
	}
	for _, want := range []string{
		"AGENTOPS_DAEMON_JOB_ID=job-rpi-cli",
		"AGENTOPS_DAEMON_REQUEST_ID=req-rpi-cli",
		"AGENTOPS_DAEMON_RPI_RUN_ID=run-cli",
		"AGENTOPS_DAEMON_RPI_BACKEND=gc-cli-fallback",
		"AGENTOPS_DAEMON_RPI_EPIC_ID=soc-ff7b",
		"AGENTOPS_DAEMON_RPI_EXECUTION_PACKET=.agents/rpi/runs/run-cli/execution-packet.json",
	} {
		if !containsString(got.Env, want) {
			t.Fatalf("env = %#v, missing %q", got.Env, want)
		}
	}
	if result.Artifacts["executor_policy"] != "cli-fallback" {
		t.Fatalf("artifacts = %#v, want cli-fallback executor policy", result.Artifacts)
	}
	if result.Artifacts["landing_policy"] != "off" {
		t.Fatalf("artifacts = %#v, want landing_policy off", result.Artifacts)
	}
}

func TestRPICLIExecutorRejectsPartialPhaseRun(t *testing.T) {
	root := t.TempDir()
	script := writeExecutableRPITestScript(t, root, "exit 0\n")
	exec, err := NewRPICLIExecutor(RPICLIExecutorOptions{Root: root, ScriptPath: script})
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}
	spec := NewRPIRunJobSpec("run-partial", "partial phase should block")
	spec.MaxPhase = 1
	jobSpec, err := spec.ToJobSpec("job-rpi-partial")
	if err != nil {
		t.Fatalf("job spec: %v", err)
	}
	result, runErr := exec.RunJob(context.Background(), QueueClaim{Job: QueueJobState{
		JobID:   jobSpec.ID,
		JobType: jobSpec.Type,
		Payload: jobSpec.Payload,
	}})
	if runErr == nil || !strings.Contains(runErr.Error(), "full rpi.run cycles") {
		t.Fatalf("RunJob error = %v, want partial run rejection", runErr)
	}
	if result.Artifacts["executor_policy"] != "cli-fallback" {
		t.Fatalf("artifacts = %#v, want cli-fallback artifacts on rejection", result.Artifacts)
	}
}

func TestRPICLIExecutorRunsScriptAndWritesLog(t *testing.T) {
	root := t.TempDir()
	script := writeExecutableRPITestScript(t, root, "echo wrapper-ok\n")
	exec, err := NewRPICLIExecutor(RPICLIExecutorOptions{Root: root, ScriptPath: script})
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}
	spec := NewRPIRunJobSpec("run-cli-log", "prove cli fallback")
	jobSpec, err := spec.ToJobSpec("job-rpi-cli-log")
	if err != nil {
		t.Fatalf("job spec: %v", err)
	}
	result, err := exec.RunJob(context.Background(), QueueClaim{Job: QueueJobState{
		JobID:   jobSpec.ID,
		JobType: jobSpec.Type,
		Payload: jobSpec.Payload,
	}})
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	logPath := filepath.Join(root, filepath.FromSlash(result.Artifacts["rpi_cli_log"]))
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(data), "wrapper-ok") {
		t.Fatalf("log = %q, want wrapper output", data)
	}
}

func TestRPICLIExecutorReturnsArtifactsWhenScriptFails(t *testing.T) {
	root := t.TempDir()
	script := writeExecutableRPITestScript(t, root, "echo wrapper-failed\nexit 7\n")
	exec, err := NewRPICLIExecutor(RPICLIExecutorOptions{Root: root, ScriptPath: script})
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}
	spec := NewRPIRunJobSpec("run-cli-fail", "prove failure artifacts")
	jobSpec, err := spec.ToJobSpec("job-rpi-cli-fail")
	if err != nil {
		t.Fatalf("job spec: %v", err)
	}
	result, runErr := exec.RunJob(context.Background(), QueueClaim{Job: QueueJobState{
		JobID:   jobSpec.ID,
		JobType: jobSpec.Type,
		Payload: jobSpec.Payload,
	}})
	if runErr == nil {
		t.Fatal("expected script failure")
	}
	if result.Artifacts["rpi_cli_log"] == "" {
		t.Fatalf("artifacts = %#v, want log path on failure", result.Artifacts)
	}
}

func writeExecutableRPITestScript(t *testing.T, root, body string) string {
	t.Helper()
	script := filepath.Join(root, "scripts", "ao-rpi-autonomous-cycle.sh")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatalf("mkdir scripts: %v", err)
	}
	content := "#!/usr/bin/env bash\nset -euo pipefail\n" + body
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return script
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
