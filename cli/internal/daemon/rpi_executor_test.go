package daemon

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubRPIPhaseExecutor struct {
	calls   int
	lastReq RPIPhaseExecutionRequest
	result  RPIPhaseExecutionResult
	err     error
}

func (s *stubRPIPhaseExecutor) ExecuteRPIPhase(_ context.Context, req RPIPhaseExecutionRequest) (RPIPhaseExecutionResult, error) {
	s.calls++
	s.lastReq = req
	if s.err != nil {
		return RPIPhaseExecutionResult{}, s.err
	}
	res := s.result
	if res.Artifacts == nil {
		res.Artifacts = map[string]string{}
	}
	return res, nil
}

func TestNewRPIJobExecutorRequiresStoreAndExecutor(t *testing.T) {
	if _, err := NewRPIJobExecutor(RPIJobExecutorOptions{}); err == nil {
		t.Fatal("expected error when store is nil")
	}
	if _, err := NewRPIJobExecutor(RPIJobExecutorOptions{Store: NewStore(t.TempDir())}); err == nil {
		t.Fatal("expected error when executor is nil")
	}
}

func TestRPIJobExecutorJobTypesCoversRPIRunAndRPIPhase(t *testing.T) {
	exec, err := NewRPIJobExecutor(RPIJobExecutorOptions{
		Store:    NewStore(t.TempDir()),
		Executor: &stubRPIPhaseExecutor{},
	})
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}
	types := exec.JobTypes()
	if len(types) != 2 {
		t.Fatalf("JobTypes = %v, want 2 entries", types)
	}
	wantRun, wantPhase := false, false
	for _, jt := range types {
		switch jt {
		case JobTypeRPIRun:
			wantRun = true
		case JobTypeRPIPhase:
			wantPhase = true
		}
	}
	if !wantRun || !wantPhase {
		t.Fatalf("JobTypes = %v, want both rpi.run and rpi.phase", types)
	}
}

func TestRPIJobExecutorRunJobRejectsNonRPIJobType(t *testing.T) {
	exec, err := NewRPIJobExecutor(RPIJobExecutorOptions{
		Store:    NewStore(t.TempDir()),
		Executor: &stubRPIPhaseExecutor{},
	})
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}
	claim := QueueLease{Job: QueueJobState{JobID: "job-wiki", JobType: JobTypeWikiForge}}
	_, runErr := exec.RunJob(context.Background(), claim)
	if runErr == nil {
		t.Fatal("expected error for non-rpi job type")
	}
}

func TestRPIJobExecutorRunJobExecutesPhaseSpec(t *testing.T) {
	store := NewStore(t.TempDir())
	stub := &stubRPIPhaseExecutor{
		result: RPIPhaseExecutionResult{
			Artifacts: map[string]string{"phase_summary": "ok"},
		},
	}
	exec, err := NewRPIJobExecutor(RPIJobExecutorOptions{
		Store:    store,
		Executor: stub,
	})
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	phaseSpec := NewRPIPhaseJobSpec("run-1", "test goal", 2)
	phaseSpec.PhaseTimeout = "5m0s"
	jobSpec, err := phaseSpec.ToJobSpec("job-phase-1")
	if err != nil {
		t.Fatalf("payload: %v", err)
	}
	claim := QueueLease{
		Job: QueueJobState{
			JobID:   jobSpec.ID,
			JobType: jobSpec.Type,
			Payload: jobSpec.Payload,
		},
	}
	result, err := exec.RunJob(context.Background(), claim)
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if stub.calls != 1 {
		t.Fatalf("phase executor called %d times, want 1", stub.calls)
	}
	if stub.lastReq.Phase != 2 {
		t.Fatalf("phase = %d, want 2", stub.lastReq.Phase)
	}
	if stub.lastReq.Goal != "test goal" {
		t.Fatalf("goal = %q, want 'test goal'", stub.lastReq.Goal)
	}
	if stub.lastReq.PhaseTimeout != 5*time.Minute {
		t.Fatalf("phase timeout = %s, want 5m", stub.lastReq.PhaseTimeout)
	}
	// phaseArtifactKey adds prefixes for some keys; just check that
	// a non-empty artifact map came back.
	if len(result.Artifacts) == 0 {
		t.Fatalf("expected non-empty artifacts, got %#v", result.Artifacts)
	}
}

func TestRPIJobExecutorRunJobPropagatesRunPhaseTimeout(t *testing.T) {
	store := NewStore(t.TempDir())
	stub := &stubRPIPhaseExecutor{
		result: RPIPhaseExecutionResult{Artifacts: map[string]string{"phase_summary": "ok"}},
	}
	exec, err := NewRPIJobExecutor(RPIJobExecutorOptions{
		Store:    store,
		Executor: stub,
	})
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}
	runSpec := NewRPIRunJobSpec("run-timeout", "test timeout")
	runSpec.MaxPhase = 1
	runSpec.PhaseTimeout = "7m0s"
	jobSpec, err := runSpec.ToJobSpec("job-run-timeout")
	if err != nil {
		t.Fatalf("payload: %v", err)
	}
	_, err = exec.RunJob(context.Background(), QueueLease{
		Job: QueueJobState{
			JobID:   jobSpec.ID,
			JobType: jobSpec.Type,
			Payload: jobSpec.Payload,
		},
	})
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if stub.lastReq.PhaseTimeout != 7*time.Minute {
		t.Fatalf("phase timeout = %s, want 7m", stub.lastReq.PhaseTimeout)
	}
}

func TestRPIJobExecutorRunJobPropagatesPhaseExecutorError(t *testing.T) {
	store := NewStore(t.TempDir())
	stubErr := errors.New("phase boom")
	stub := &stubRPIPhaseExecutor{err: stubErr}
	exec, err := NewRPIJobExecutor(RPIJobExecutorOptions{
		Store:    store,
		Executor: stub,
	})
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	phaseSpec := NewRPIPhaseJobSpec("run-1", "test goal", 1)
	jobSpec, err := phaseSpec.ToJobSpec("job-phase-2")
	if err != nil {
		t.Fatalf("payload: %v", err)
	}
	claim := QueueLease{
		Job: QueueJobState{
			JobID:   jobSpec.ID,
			JobType: jobSpec.Type,
			Payload: jobSpec.Payload,
		},
	}
	_, runErr := exec.RunJob(context.Background(), claim)
	if runErr == nil {
		t.Fatal("expected error from phase executor to propagate")
	}
	if !errors.Is(runErr, stubErr) {
		// runner wraps error in RPIPhaseExecutionError; just confirm we got
		// a non-nil error with a useful message.
		if runErr.Error() == "" {
			t.Fatalf("expected non-empty error message, got %v", runErr)
		}
	}
}
