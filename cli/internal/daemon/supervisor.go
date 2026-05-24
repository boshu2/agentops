package daemon

import (
	"context"
	"errors"
	"fmt"
	"log"
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
	heartbeatTimeout  time.Duration
	executionTimeout  time.Duration
	claimMu           sync.Mutex
	// heartbeatFn performs one heartbeat ledger write. It defaults to
	// queue.Heartbeat; tests override it to inject a slow/blocking store write
	// and prove the per-tick write is bounded.
	heartbeatFn func(HeartbeatInput, QueueMutationOptions) (QueueJobState, error)
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
	s := &Supervisor{
		queue:             opts.Queue,
		executors:         executors,
		actor:             actor,
		pollInterval:      pollInterval,
		heartbeatInterval: heartbeatInterval,
		executionTimeout:  opts.ExecutionTimeout,
	}
	s.heartbeatFn = s.queue.Heartbeat
	return s, nil
}

// maxHeartbeatTimeout caps the per-tick heartbeat-write deadline so a long
// heartbeat interval does not imply an equally long tolerance for a stuck store
// write. The deadline is otherwise heartbeatInterval/2.
const maxHeartbeatTimeout = 10 * time.Second

// resolveHeartbeatTimeout returns the bound for a single heartbeat ledger write.
// An explicit s.heartbeatTimeout wins (tests set it); otherwise it derives from
// the current heartbeat interval (half the interval, capped) so the write cannot
// stall the loop past the point where the next beat is due.
func (s *Supervisor) resolveHeartbeatTimeout() time.Duration {
	if s.heartbeatTimeout > 0 {
		return s.heartbeatTimeout
	}
	timeout := s.heartbeatInterval / 2
	if timeout <= 0 {
		timeout = s.heartbeatInterval
	}
	if timeout > maxHeartbeatTimeout {
		timeout = maxHeartbeatTimeout
	}
	return timeout
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
	type execution struct {
		result JobExecutionResult
		err    error
	}
	// Buffered (cap 1) so the worker's send never blocks even after an early
	// return leaves no receiver — the worker can always complete and exit.
	done := make(chan execution, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		result, err := safeRunJob(execCtx, executor, claim)
		done <- execution{result: result, err: err}
	}()
	// Single deferred teardown so the worker goroutine has a guaranteed
	// lifecycle on EVERY exit path: cancel() first to signal the worker to
	// stop, then wg.Wait() to join it so the goroutine is reaped rather than
	// leaked. Ordering matters — cancel must precede the wait or the join
	// would block, so they live in one closure instead of two LIFO defers. A
	// context-respecting executor returns promptly once cancelled, so the join
	// does not stall the supervisor.
	defer func() {
		cancel()
		wg.Wait()
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
			if err := s.beat(ctx, claim); err != nil {
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

// beat records one heartbeat ledger write, bounded by a per-call timeout so a
// stuck store write (disk full, lock contention) cannot stall the heartbeat
// loop. The write runs on its own goroutine; beat returns when the write
// completes, when the per-call deadline elapses, or when the parent context is
// cancelled — whichever happens first.
//
// On timeout the beat is logged and skipped (nil error) rather than blocked or
// failed: a single missed beat is recoverable (the next tick retries; the lease
// only lapses after LeaseDuration, which is many beats), whereas blocking the
// select would stall the whole supervisor loop. A real write error is still
// returned so the caller fails the job as before. The leaked goroutine from a
// timed-out write drains into the buffered channel and exits once the store call
// eventually returns; it is not joined because the whole point is not to wait on
// it.
func (s *Supervisor) beat(ctx context.Context, claim QueueLease) error {
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := s.resolveHeartbeatTimeout()
	beatCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Buffered (cap 1) so the write goroutine's send never blocks even after a
	// timeout leaves no receiver — it can always finish and exit.
	done := make(chan error, 1)
	go func() {
		_, err := s.heartbeatFn(HeartbeatInput{
			JobID:      claim.Job.JobID,
			RequestID:  RequestID(claim.Job.RequestID),
			ClaimToken: claim.ClaimToken,
			LeaseEpoch: claim.LeaseEpoch,
			Actor:      s.actor,
		}, QueueMutationOptions{})
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-beatCtx.Done():
		if ctx.Err() != nil {
			// Parent cancellation/deadline: let the loop's own ctx.Done /
			// execCtx.Done cases handle teardown on the next iteration.
			return nil
		}
		// Per-call heartbeat deadline elapsed while the parent is still live:
		// skip this beat rather than block the loop.
		log.Printf("[supervisor] heartbeat write for job %q timed out after %s; skipping beat", claim.Job.JobID, timeout)
		return nil
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
