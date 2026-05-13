package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
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
	claim := QueueLease{Job: QueueJobState{
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
	claim := QueueLease{Job: QueueJobState{JobID: "job-phase", JobType: JobTypeRPIPhase}}
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
	result, runErr := exec.RunJob(context.Background(), QueueLease{Job: QueueJobState{
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
	result, runErr := exec.RunJob(context.Background(), QueueLease{Job: QueueJobState{
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

// TestRPIRunEmitsAgentUpdates exercises soc-y0ct.2: when a Store is wired into
// the executor, RunJob emits agent-update phase_start before the runner and
// phase_complete + phase_handoff after a successful run. Without a Store, no
// events are appended (back-compat with sub-5a callers that injected a runner
// only).
func TestRPIRunEmitsAgentUpdates(t *testing.T) {
	for _, tc := range []struct {
		name        string
		runErr      error
		wantStatus  string
		wantHandoff bool
	}{
		{name: "success", runErr: nil, wantStatus: "success", wantHandoff: true},
		{name: "failure", runErr: errors.New("runner exploded"), wantStatus: "failure", wantHandoff: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			store := NewStore(root)
			// Advancing clock so duration_ms is populated on phase_complete (the
			// constructor drops zero-valued duration_ms via omitempty).
			startTime := time.Date(2026, 5, 7, 21, 0, 0, 0, time.UTC)
			tickCount := 0
			tickingClock := func() time.Time {
				t := startTime.Add(time.Duration(tickCount) * 10 * time.Millisecond)
				tickCount++
				return t
			}
			exec, err := NewRPIRunExecutor(RPIRunExecutorOptions{
				Root:  root,
				Store: store,
				Actor: "test-actor",
				Clock: tickingClock,
				Run: func(_ context.Context, _ RPIRunRequest) (RPIRunResult, error) {
					return RPIRunResult{Artifacts: map[string]string{"runner_marker": "ok"}}, tc.runErr
				},
			})
			if err != nil {
				t.Fatalf("new executor: %v", err)
			}
			spec := NewRPIRunJobSpec("run-emit", "exercise emission")
			spec.ExecutionPacketPath = ".agents/rpi/runs/run-emit/execution-packet.json"
			jobSpec, err := spec.ToJobSpec("job-emit")
			if err != nil {
				t.Fatalf("job spec: %v", err)
			}
			claim := QueueLease{Job: QueueJobState{
				JobID:     jobSpec.ID,
				RequestID: "req-emit",
				JobType:   jobSpec.Type,
				Payload:   jobSpec.Payload,
			}}
			if _, err := exec.RunJob(context.Background(), claim); !errors.Is(err, tc.runErr) {
				t.Fatalf("RunJob err = %v, want %v", err, tc.runErr)
			}

			events, err := store.ReadLedger()
			if err != nil {
				t.Fatalf("read ledger: %v", err)
			}
			var got []EventType
			for _, ev := range events {
				got = append(got, ev.EventType)
			}
			want := []EventType{EventAgentUpdatePhaseStart, EventAgentUpdatePhaseComplete}
			if tc.wantHandoff {
				want = append(want, EventAgentUpdatePhaseHandoff)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("emitted event types = %v, want %v", got, want)
			}

			// Verify boilerplate (JobID, Actor, RequestID) and payload (run_id, phase_name)
			// on the phase_start event so we know the wrapper carries claim metadata.
			start := events[0]
			if start.JobID != "job-emit" || string(start.RequestID) != "req-emit" || start.Actor != "test-actor" {
				t.Errorf("phase_start boilerplate = %+v", start)
			}
			if start.Payload["run_id"] != "run-emit" || start.Payload["phase_name"] != "rpi.run" {
				t.Errorf("phase_start payload = %v", start.Payload)
			}

			// Verify phase_complete reports the right status and carries duration_ms.
			complete := events[1]
			if complete.Payload["status"] != tc.wantStatus {
				t.Errorf("phase_complete status = %v, want %s", complete.Payload["status"], tc.wantStatus)
			}
			if _, ok := complete.Payload["duration_ms"]; !ok {
				t.Errorf("phase_complete missing duration_ms; payload = %v", complete.Payload)
			}

			if tc.wantHandoff {
				handoff := events[2]
				if handoff.Payload["from_phase"] != "rpi.run" || handoff.Payload["to_phase"] != "operator" {
					t.Errorf("phase_handoff payload = %v", handoff.Payload)
				}
				if handoff.Payload["packet_path"] != spec.ExecutionPacketPath {
					t.Errorf("phase_handoff packet_path = %v", handoff.Payload["packet_path"])
				}
			}
		})
	}
}

// TestRPIRunEmitsCriterionVerdicts exercises soc-awx8: when wave checkpoints
// exist under .agents/crank/wave-*-checkpoint.json, RunJob emits one
// agent_update.criterion_verdict event per verdict row, ordered AFTER
// phase_complete and BEFORE phase_handoff. Multiple checkpoints contribute in
// lexicographic order (wave-1, wave-2, …).
func TestRPIRunEmitsCriterionVerdicts(t *testing.T) {
	root := t.TempDir()
	checkpointDir := filepath.Join(root, ".agents", "crank")
	if err := os.MkdirAll(checkpointDir, 0o755); err != nil {
		t.Fatalf("mkdir checkpoint: %v", err)
	}
	wave1 := []byte(`{
	  "wave": 1,
	  "criterion_verdicts": [
	    {"id": "ac-foo.1", "status": "PASS", "evidence_path": "cli/foo.go", "notes": "ok"},
	    {"id": "ac-foo.2", "status": "FAIL", "notes": "exit 1"}
	  ]
	}`)
	wave2 := []byte(`{
	  "wave": 2,
	  "criterion_verdicts": [
	    {"id": "ac-bar.1", "status": "SKIP", "notes": "deferred"}
	  ]
	}`)
	if err := os.WriteFile(filepath.Join(checkpointDir, "wave-1-checkpoint.json"), wave1, 0o644); err != nil {
		t.Fatalf("write wave-1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(checkpointDir, "wave-2-checkpoint.json"), wave2, 0o644); err != nil {
		t.Fatalf("write wave-2: %v", err)
	}

	store := NewStore(root)
	startTime := time.Date(2026, 5, 7, 22, 0, 0, 0, time.UTC)
	tickCount := 0
	tickingClock := func() time.Time {
		t := startTime.Add(time.Duration(tickCount) * 10 * time.Millisecond)
		tickCount++
		return t
	}
	exec, err := NewRPIRunExecutor(RPIRunExecutorOptions{
		Root:  root,
		Store: store,
		Actor: "test-actor",
		Clock: tickingClock,
		Run: func(_ context.Context, _ RPIRunRequest) (RPIRunResult, error) {
			return RPIRunResult{}, nil
		},
	})
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}
	spec := NewRPIRunJobSpec("run-verdicts", "exercise verdict emission")
	jobSpec, err := spec.ToJobSpec("job-verdicts")
	if err != nil {
		t.Fatalf("job spec: %v", err)
	}
	claim := QueueLease{Job: QueueJobState{
		JobID:     jobSpec.ID,
		RequestID: "req-verdicts",
		JobType:   jobSpec.Type,
		Payload:   jobSpec.Payload,
	}}
	if _, err := exec.RunJob(context.Background(), claim); err != nil {
		t.Fatalf("RunJob: %v", err)
	}

	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	var got []EventType
	for _, ev := range events {
		got = append(got, ev.EventType)
	}
	want := []EventType{
		EventAgentUpdatePhaseStart,
		EventAgentUpdatePhaseComplete,
		EventAgentUpdateCriterionVerdict,
		EventAgentUpdateCriterionVerdict,
		EventAgentUpdateCriterionVerdict,
		EventAgentUpdatePhaseHandoff,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("event sequence = %v, want %v", got, want)
	}

	// Verify each verdict event carries the right per-row payload + the run_id
	// from the spec.
	verdicts := []struct {
		id, status, evidence string
	}{
		{"ac-foo.1", "PASS", "cli/foo.go"},
		{"ac-foo.2", "FAIL", ""},
		{"ac-bar.1", "SKIP", ""},
	}
	for i, want := range verdicts {
		ev := events[2+i]
		if ev.Payload["criterion_id"] != want.id {
			t.Errorf("event[%d].criterion_id = %v, want %s", 2+i, ev.Payload["criterion_id"], want.id)
		}
		if ev.Payload["status"] != want.status {
			t.Errorf("event[%d].status = %v, want %s", 2+i, ev.Payload["status"], want.status)
		}
		if ev.Payload["run_id"] != "run-verdicts" {
			t.Errorf("event[%d].run_id = %v, want run-verdicts", 2+i, ev.Payload["run_id"])
		}
		if want.evidence != "" && ev.Payload["evidence_path"] != want.evidence {
			t.Errorf("event[%d].evidence_path = %v, want %s", 2+i, ev.Payload["evidence_path"], want.evidence)
		}
	}
}

// TestRPIRunNoEmissionWithoutStore confirms back-compat: callers that don't
// pass a Store get the runner-only behavior with zero events appended.
func TestRPIRunNoEmissionWithoutStore(t *testing.T) {
	root := t.TempDir()
	// Set up a side-channel store JUST to assert no events landed; executor
	// is not configured with it.
	witness := NewStore(root + "-witness")
	exec, err := NewRPIRunExecutor(RPIRunExecutorOptions{
		Root: root,
		Run:  func(_ context.Context, _ RPIRunRequest) (RPIRunResult, error) { return RPIRunResult{}, nil },
	})
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}
	spec := NewRPIRunJobSpec("run-silent", "no store wired")
	jobSpec, err := spec.ToJobSpec("job-silent")
	if err != nil {
		t.Fatalf("job spec: %v", err)
	}
	claim := QueueLease{Job: QueueJobState{JobID: jobSpec.ID, JobType: jobSpec.Type, Payload: jobSpec.Payload}}
	if _, err := exec.RunJob(context.Background(), claim); err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	events, err := witness.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected zero events on the witness store, got %d", len(events))
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
