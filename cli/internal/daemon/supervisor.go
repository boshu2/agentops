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
	Artifacts    map[string]string
	ArtifactRefs map[string]ArtifactRef
}

// JobExecutor runs claimed daemon jobs for one or more job types.
type JobExecutor interface {
	JobTypes() []JobType
	RunJob(context.Context, QueueLease) (JobExecutionResult, error)
}

// SupervisorOptions configures a daemon queue supervisor.
type SupervisorOptions struct {
	Queue             *Queue
	Executors         []JobExecutor
	Actor             string
	PollInterval      time.Duration
	HeartbeatInterval time.Duration
	ExecutionTimeout  time.Duration
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
	executionTimeout  time.Duration
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
		executionTimeout:  opts.ExecutionTimeout,
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
	artifacts := result.Artifacts
	artifactRefs := result.ArtifactRefs
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
			Artifacts:    artifacts,
			ArtifactRefs: artifactRefs,
		}, QueueMutationOptions{})
		if err != nil {
			return SupervisorRunOnceResult{}, err
		}
		return SupervisorRunOnceResult{Claimed: true, Job: failed}, nil
	}
	completed, err := s.queue.CompleteJob(CompleteJobInput{
		JobID:        claim.Job.JobID,
		RequestID:    RequestID(claim.Job.RequestID),
		ClaimToken:   claim.ClaimToken,
		LeaseEpoch:   claim.LeaseEpoch,
		Actor:        s.actor,
		Artifacts:    artifacts,
		ArtifactRefs: artifactRefs,
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

func (s *Supervisor) runExecutorWithHeartbeat(ctx context.Context, executor JobExecutor, claim QueueLease) (JobExecutionResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	execCtx, cancel := context.WithCancel(ctx)
	if s.executionTimeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, s.executionTimeout)
	}
	defer cancel()
	type execution struct {
		result JobExecutionResult
		err    error
	}
	done := make(chan execution, 1)
	go func() {
		result, err := safeRunJob(execCtx, executor, claim)
		done <- execution{result: result, err: err}
	}()

	ticker := time.NewTicker(s.heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case exec := <-done:
			if s.executionTimeout > 0 && errors.Is(execCtx.Err(), context.DeadlineExceeded) {
				return exec.result, fmt.Errorf("executor timed out after %s", s.executionTimeout)
			}
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
		case <-execCtx.Done():
			if s.executionTimeout > 0 && errors.Is(execCtx.Err(), context.DeadlineExceeded) {
				return JobExecutionResult{}, fmt.Errorf("executor timed out after %s", s.executionTimeout)
			}
			return JobExecutionResult{}, execCtx.Err()
		}
	}
}

// safeRunJob invokes the executor's RunJob with panic recovery. A panic inside
// RunJob is recovered and converted into a job error so the supervisor loop (and
// the daemon process) survives a misbehaving executor instead of crashing. The
// non-panic path returns the executor's result and error unchanged.
func safeRunJob(ctx context.Context, executor JobExecutor, claim QueueLease) (result JobExecutionResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			result = JobExecutionResult{}
			err = fmt.Errorf("executor panicked: %v", r)
		}
	}()
	return executor.RunJob(ctx, claim)
}
