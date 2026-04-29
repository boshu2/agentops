package daemon

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// JobExecutionResult is the terminal output from a daemon job executor.
type JobExecutionResult struct {
	Artifacts map[string]string
}

// JobExecutor runs claimed daemon jobs for one or more job types.
type JobExecutor interface {
	JobTypes() []JobType
	RunJob(context.Context, QueueClaim) (JobExecutionResult, error)
}

// SupervisorOptions configures a daemon queue supervisor.
type SupervisorOptions struct {
	Queue             *Queue
	Executors         []JobExecutor
	Actor             string
	PollInterval      time.Duration
	HeartbeatInterval time.Duration
}

// SupervisorRunOnceResult reports one supervisor claim attempt.
type SupervisorRunOnceResult struct {
	Claimed bool
	Job     QueueJobState
}

// Supervisor claims queue jobs, runs executors, and records terminal state.
type Supervisor struct {
	queue             *Queue
	executors         map[JobType]JobExecutor
	actor             string
	pollInterval      time.Duration
	heartbeatInterval time.Duration
	claimMu           sync.Mutex
}

// NewSupervisor builds a queue supervisor from explicit executors.
func NewSupervisor(opts SupervisorOptions) (*Supervisor, error) {
	if opts.Queue == nil {
		return nil, errors.New("daemon supervisor: queue is required")
	}
	executors := map[JobType]JobExecutor{}
	for _, executor := range opts.Executors {
		if executor == nil {
			continue
		}
		for _, jobType := range executor.JobTypes() {
			if err := ValidateJobType(jobType); err != nil {
				return nil, err
			}
			executors[jobType] = executor
		}
	}
	if len(executors) == 0 {
		return nil, errors.New("daemon supervisor: at least one executor is required")
	}
	actor := opts.Actor
	if actor == "" {
		actor = "agentopsd-worker"
	}
	pollInterval := opts.PollInterval
	if pollInterval <= 0 {
		pollInterval = time.Second
	}
	heartbeatInterval := opts.HeartbeatInterval
	if heartbeatInterval <= 0 {
		heartbeatInterval = 15 * time.Second
	}
	return &Supervisor{
		queue:             opts.Queue,
		executors:         executors,
		actor:             actor,
		pollInterval:      pollInterval,
		heartbeatInterval: heartbeatInterval,
	}, nil
}

// RunOnce attempts to claim and execute one supported job.
func (s *Supervisor) RunOnce(ctx context.Context) (SupervisorRunOnceResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return SupervisorRunOnceResult{}, nil
	}
	s.claimMu.Lock()
	claim, err := s.queue.ClaimNextMatching(s.actor, s.supportsJob, QueueMutationOptions{})
	s.claimMu.Unlock()
	if err != nil {
		if errors.Is(err, ErrNoClaimableJobs) {
			return SupervisorRunOnceResult{}, nil
		}
		return SupervisorRunOnceResult{}, err
	}
	executor := s.executors[claim.Job.JobType]
	if executor == nil {
		return SupervisorRunOnceResult{}, fmt.Errorf("daemon supervisor: no executor for job type %s", claim.Job.JobType)
	}
	result, execErr := s.runExecutorWithHeartbeat(ctx, executor, claim)
	if execErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil && errors.Is(execErr, ctxErr) {
			return SupervisorRunOnceResult{Claimed: true, Job: claim.Job}, nil
		}
		failed, err := s.queue.FailJob(FailJobInput{
			JobID:      claim.Job.JobID,
			RequestID:  RequestID(claim.Job.RequestID),
			ClaimToken: claim.ClaimToken,
			LeaseEpoch: claim.LeaseEpoch,
			Actor:      s.actor,
			Failure: JobFailure{
				Code:    FailureRequestRejected,
				Message: execErr.Error(),
			},
		}, QueueMutationOptions{})
		if err != nil {
			return SupervisorRunOnceResult{}, err
		}
		return SupervisorRunOnceResult{Claimed: true, Job: failed}, nil
	}
	completed, err := s.queue.CompleteJob(CompleteJobInput{
		JobID:      claim.Job.JobID,
		RequestID:  RequestID(claim.Job.RequestID),
		ClaimToken: claim.ClaimToken,
		LeaseEpoch: claim.LeaseEpoch,
		Actor:      s.actor,
		Artifacts:  result.Artifacts,
	}, QueueMutationOptions{})
	if err != nil {
		return SupervisorRunOnceResult{}, err
	}
	return SupervisorRunOnceResult{Claimed: true, Job: completed}, nil
}

// RunLoop claims and executes supported jobs until the context is cancelled.
func (s *Supervisor) RunLoop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()
	for {
		result, err := s.RunOnce(ctx)
		if err != nil {
			return err
		}
		if result.Claimed {
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (s *Supervisor) supportsJob(job QueueJobState) bool {
	_, ok := s.executors[job.JobType]
	return ok
}

func (s *Supervisor) runExecutorWithHeartbeat(ctx context.Context, executor JobExecutor, claim QueueClaim) (JobExecutionResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	type execution struct {
		result JobExecutionResult
		err    error
	}
	done := make(chan execution, 1)
	go func() {
		result, err := executor.RunJob(ctx, claim)
		done <- execution{result: result, err: err}
	}()

	ticker := time.NewTicker(s.heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case exec := <-done:
			return exec.result, exec.err
		case <-ticker.C:
			if _, err := s.queue.Heartbeat(HeartbeatInput{
				JobID:      claim.Job.JobID,
				RequestID:  RequestID(claim.Job.RequestID),
				ClaimToken: claim.ClaimToken,
				LeaseEpoch: claim.LeaseEpoch,
				Actor:      s.actor,
			}, QueueMutationOptions{}); err != nil {
				return JobExecutionResult{}, err
			}
		case <-ctx.Done():
			return JobExecutionResult{}, ctx.Err()
		}
	}
}

// FakeOpenClawSnapshotExecutor is a CI-safe fake executor for openclaw.snapshot jobs.
type FakeOpenClawSnapshotExecutor struct {
	Delay     time.Duration
	Err       error
	Artifacts map[string]string
}

// JobTypes returns the existing OpenClaw snapshot job type.
func (e FakeOpenClawSnapshotExecutor) JobTypes() []JobType {
	return []JobType{JobTypeOpenClawSnapshot}
}

// RunJob returns deterministic fake OpenClaw snapshot artifacts.
func (e FakeOpenClawSnapshotExecutor) RunJob(ctx context.Context, claim QueueClaim) (JobExecutionResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if claim.Job.JobType != JobTypeOpenClawSnapshot {
		return JobExecutionResult{}, fmt.Errorf("fake executor does not support job type %s", claim.Job.JobType)
	}
	if e.Delay > 0 {
		timer := time.NewTimer(e.Delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return JobExecutionResult{}, ctx.Err()
		case <-timer.C:
		}
	}
	artifacts := map[string]string{
		"executor_policy": "fake",
		"snapshot_status": "validated",
	}
	for key, value := range e.Artifacts {
		artifacts[key] = value
	}
	return JobExecutionResult{Artifacts: artifacts}, e.Err
}
