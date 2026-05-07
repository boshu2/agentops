package daemon

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// TestRPIRunExecutor_RunsViaFunc verifies the executor dispatches to the
// injected RPIRunFunc with a faithful RPIRunRequest and merges runner-emitted
// artifacts on top of the executor defaults.
func TestRPIRunExecutor_RunsViaFunc(t *testing.T) {
	root := t.TempDir()
	var got RPIRunRequest
	exec, err := NewRPIRunExecutor(RPIRunExecutorOptions{
		Root: root,
		Run: func(_ context.Context, req RPIRunRequest) (RPIRunResult, error) {
			got = req
			return RPIRunResult{Artifacts: map[string]string{
				"rpi_run_status": "completed",
				"goal":           "runner-overridden",
			}}, nil
		},
	})
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}
	spec := NewRPIRunJobSpec("run-in-process", "validate in-process executor")
	spec.EpicID = "soc-bcrn"
	spec.ExecutionPacketPath = ".agents/rpi/runs/run-in-process/execution-packet.json"
	jobSpec, err := spec.ToJobSpec("job-rpi-run")
	if err != nil {
		t.Fatalf("job spec: %v", err)
	}
	claim := QueueClaim{Job: QueueJobState{
		JobID:     jobSpec.ID,
		RequestID: "req-rpi-run",
		JobType:   jobSpec.Type,
		Payload:   jobSpec.Payload,
	}}
	result, err := exec.RunJob(context.Background(), claim)
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if got.Spec.RunID != "run-in-process" || got.Spec.Goal != "validate in-process executor" {
		t.Fatalf("runner spec = %#v, want run-in-process/validate in-process executor", got.Spec)
	}
	if got.Spec.EpicID != "soc-bcrn" {
		t.Fatalf("runner spec EpicID = %q, want soc-bcrn", got.Spec.EpicID)
	}
	if got.Root != root {
		t.Fatalf("runner root = %q, want %q", got.Root, root)
	}
	if got.Claim.Job.JobID != "job-rpi-run" {
		t.Fatalf("runner claim JobID = %q, want job-rpi-run", got.Claim.Job.JobID)
	}
	if result.Artifacts["executor_policy"] != "in-process" {
		t.Fatalf("artifacts = %#v, want executor_policy in-process", result.Artifacts)
	}
	if result.Artifacts["backend"] != "in-process" {
		t.Fatalf("artifacts = %#v, want backend in-process", result.Artifacts)
	}
	if result.Artifacts["requested_backend"] != string(RPIBackendGasCityAPI) {
		t.Fatalf("artifacts = %#v, want requested_backend gascity-api", result.Artifacts)
	}
	if result.Artifacts["rpi_run_status"] != "completed" {
		t.Fatalf("artifacts = %#v, want rpi_run_status completed (runner-emitted)", result.Artifacts)
	}
	if result.Artifacts["goal"] != "runner-overridden" {
		t.Fatalf("artifacts = %#v, want goal=runner-overridden (runner artifacts merged on top)", result.Artifacts)
	}
	if result.Artifacts["run_id"] != "run-in-process" {
		t.Fatalf("artifacts = %#v, want run_id=run-in-process", result.Artifacts)
	}
	if !strings.Contains(result.Artifacts["rpi_run_log"], "run-in-process") || !strings.Contains(result.Artifacts["rpi_run_log"], "job-rpi-run") {
		t.Fatalf("artifacts rpi_run_log = %q, want path containing run/job IDs", result.Artifacts["rpi_run_log"])
	}
	if got, want := exec.JobTypes(), []JobType{JobTypeRPIRun}; !reflect.DeepEqual(got, want) {
		t.Fatalf("JobTypes = %v, want %v", got, want)
	}
}

// TestRPIRunExecutor_RejectsWrongJobType ensures non-rpi.run claims surface a
// type-mismatch error and do not dispatch to the runner.
func TestRPIRunExecutor_RejectsWrongJobType(t *testing.T) {
	called := false
	exec, err := NewRPIRunExecutor(RPIRunExecutorOptions{
		Root: t.TempDir(),
		Run: func(context.Context, RPIRunRequest) (RPIRunResult, error) {
			called = true
			return RPIRunResult{}, nil
		},
	})
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}
	claim := QueueClaim{Job: QueueJobState{JobID: "job-phase", JobType: JobTypeRPIPhase}}
	_, runErr := exec.RunJob(context.Background(), claim)
	if runErr == nil || !strings.Contains(runErr.Error(), "does not support") {
		t.Fatalf("RunJob error = %v, want unsupported type", runErr)
	}
	if called {
		t.Fatal("runner should not be invoked for wrong job type")
	}
}

// TestRPIRunExecutor_PropagatesRunnerError confirms runner errors are returned
// to the supervisor along with the executor-default artifacts (so the
// terminal record retains the run-log path even on failure).
func TestRPIRunExecutor_PropagatesRunnerError(t *testing.T) {
	wantErr := errors.New("runner blew up")
	exec, err := NewRPIRunExecutor(RPIRunExecutorOptions{
		Root: t.TempDir(),
		Run: func(context.Context, RPIRunRequest) (RPIRunResult, error) {
			return RPIRunResult{Artifacts: map[string]string{"partial": "yes"}}, wantErr
		},
	})
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}
	spec := NewRPIRunJobSpec("run-fail", "prove failure path")
	jobSpec, err := spec.ToJobSpec("job-rpi-fail")
	if err != nil {
		t.Fatalf("job spec: %v", err)
	}
	result, runErr := exec.RunJob(context.Background(), QueueClaim{Job: QueueJobState{
		JobID:   jobSpec.ID,
		JobType: jobSpec.Type,
		Payload: jobSpec.Payload,
	}})
	if !errors.Is(runErr, wantErr) {
		t.Fatalf("RunJob error = %v, want %v", runErr, wantErr)
	}
	if result.Artifacts["executor_policy"] != "in-process" {
		t.Fatalf("artifacts = %#v, want in-process executor policy on failure", result.Artifacts)
	}
	if result.Artifacts["partial"] != "yes" {
		t.Fatalf("artifacts = %#v, want partial=yes (runner artifacts merged on failure)", result.Artifacts)
	}
}

// TestRPIRunExecutor_ValidateFullRun confirms partial-phase specs are
// rejected before the runner is invoked, with executor-default artifacts
// returned alongside the validation error.
func TestRPIRunExecutor_ValidateFullRun(t *testing.T) {
	called := false
	exec, err := NewRPIRunExecutor(RPIRunExecutorOptions{
		Root: t.TempDir(),
		Run: func(context.Context, RPIRunRequest) (RPIRunResult, error) {
			called = true
			return RPIRunResult{}, nil
		},
	})
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
		t.Fatalf("RunJob error = %v, want full-run rejection", runErr)
	}
	if called {
		t.Fatal("runner should not be invoked for partial-run spec")
	}
	if result.Artifacts["executor_policy"] != "in-process" {
		t.Fatalf("artifacts = %#v, want in-process artifacts on validation failure", result.Artifacts)
	}
}

// TestNewRPIRunExecutor_RequiresFields rejects construction with a nil runner
// or empty root.
func TestNewRPIRunExecutor_RequiresFields(t *testing.T) {
	if _, err := NewRPIRunExecutor(RPIRunExecutorOptions{Root: t.TempDir()}); err == nil {
		t.Fatal("expected error when Run is nil")
	}
	if _, err := NewRPIRunExecutor(RPIRunExecutorOptions{Run: func(context.Context, RPIRunRequest) (RPIRunResult, error) {
		return RPIRunResult{}, nil
	}}); err == nil {
		t.Fatal("expected error when Root is empty")
	}
}

// TestRPIRunJobSpec_AcceptsSupervisorPolicyFields verifies supervisor policy
// fields added by soc-bcrn.3.8 round-trip cleanly through ToJobSpec /
// RPIRunJobSpecFromPayload so daemon submitters can attach gate, landing,
// and kill-switch policy without losing semantics on the wire.
func TestRPIRunJobSpec_AcceptsSupervisorPolicyFields(t *testing.T) {
	spec := NewRPIRunJobSpec("run-policy", "validate supervisor policy fields")
	spec.MaxCycles = 2
	spec.GatePolicy = "required"
	spec.LandingPolicy = "commit"
	spec.LandingBranch = "main"
	spec.BDSyncPolicy = "auto"
	spec.FailurePolicy = "continue"
	spec.KillSwitchPath = ".agents/rpi/KILL"

	job, err := spec.ToJobSpec("job-policy")
	if err != nil {
		t.Fatalf("ToJobSpec: %v", err)
	}
	parsed, err := RPIRunJobSpecFromPayload(job.Payload)
	if err != nil {
		t.Fatalf("RPIRunJobSpecFromPayload: %v", err)
	}
	if parsed.MaxCycles != 2 {
		t.Errorf("MaxCycles = %d, want 2", parsed.MaxCycles)
	}
	if parsed.GatePolicy != "required" {
		t.Errorf("GatePolicy = %q, want required", parsed.GatePolicy)
	}
	if parsed.LandingPolicy != "commit" {
		t.Errorf("LandingPolicy = %q, want commit", parsed.LandingPolicy)
	}
	if parsed.LandingBranch != "main" {
		t.Errorf("LandingBranch = %q, want main", parsed.LandingBranch)
	}
	if parsed.BDSyncPolicy != "auto" {
		t.Errorf("BDSyncPolicy = %q, want auto", parsed.BDSyncPolicy)
	}
	if parsed.FailurePolicy != "continue" {
		t.Errorf("FailurePolicy = %q, want continue", parsed.FailurePolicy)
	}
	if parsed.KillSwitchPath != ".agents/rpi/KILL" {
		t.Errorf("KillSwitchPath = %q, want .agents/rpi/KILL", parsed.KillSwitchPath)
	}
}

// TestRPIRunJobSpec_DefaultsAreEmpty confirms a spec built without explicit
// supervisor policy fields parses with zero values so callers can apply
// supervisor defaults at the cfg layer (back-compat with sub-5a payloads).
func TestRPIRunJobSpec_DefaultsAreEmpty(t *testing.T) {
	spec := NewRPIRunJobSpec("run-defaults", "no supervisor policy")
	job, err := spec.ToJobSpec("job-defaults")
	if err != nil {
		t.Fatalf("ToJobSpec: %v", err)
	}
	// Pre-existing payload without policy fields (older clients).
	delete(job.Payload, "max_cycles")
	delete(job.Payload, "gate_policy")
	delete(job.Payload, "landing_policy")
	delete(job.Payload, "landing_branch")
	delete(job.Payload, "bd_sync_policy")
	delete(job.Payload, "failure_policy")
	delete(job.Payload, "kill_switch_path")

	parsed, err := RPIRunJobSpecFromPayload(job.Payload)
	if err != nil {
		t.Fatalf("RPIRunJobSpecFromPayload: %v", err)
	}
	if parsed.MaxCycles != 0 || parsed.GatePolicy != "" || parsed.LandingPolicy != "" ||
		parsed.LandingBranch != "" || parsed.BDSyncPolicy != "" || parsed.FailurePolicy != "" ||
		parsed.KillSwitchPath != "" {
		t.Fatalf("expected zero supervisor policy fields, got %#v", parsed)
	}
}
