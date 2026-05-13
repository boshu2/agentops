package daemon

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSupervisor_CompletesFakeJob(t *testing.T) {
	now := projectionTestTime(t, 0)
	queue := newTestQueue(t, &now, QueueOptions{LeaseDuration: time.Minute})
	if _, err := queue.SubmitJob(SubmitJobInput{RequestID: "req-submit", JobID: "job-openclaw", JobType: JobTypeOpenClawSnapshot}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	supervisor := newTestSupervisor(t, queue, testOpenClawSnapshotExecutor{})

	result, err := supervisor.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run once: %v", err)
	}
	if !result.Claimed {
		t.Fatalf("run once did not claim a job")
	}
	if result.Job.Status != JobStatusCompleted {
		t.Fatalf("job status = %q, want completed", result.Job.Status)
	}
	if result.Job.Artifacts["executor_policy"] != "fake" || result.Job.Artifacts["snapshot_status"] != "validated" {
		t.Fatalf("artifacts = %#v, want fake snapshot proof", result.Job.Artifacts)
	}
}

func TestSupervisor_FailsFakeJob(t *testing.T) {
	now := projectionTestTime(t, 0)
	queue := newTestQueue(t, &now, QueueOptions{LeaseDuration: time.Minute})
	if _, err := queue.SubmitJob(SubmitJobInput{RequestID: "req-submit", JobID: "job-openclaw", JobType: JobTypeOpenClawSnapshot}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	supervisor := newTestSupervisor(t, queue, testOpenClawSnapshotExecutor{Err: errors.New("snapshot failed")})

	result, err := supervisor.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run once: %v", err)
	}
	if result.Job.Status != JobStatusFailed {
		t.Fatalf("job status = %q, want failed", result.Job.Status)
	}
	if result.Job.Failure == nil || result.Job.Failure.Message != "snapshot failed" {
		t.Fatalf("failure = %#v, want snapshot failed", result.Job.Failure)
	}
}

func TestSupervisor_HeartbeatsLongJob(t *testing.T) {
	now := projectionTestTime(t, 0)
	queue := newTestQueue(t, &now, QueueOptions{LeaseDuration: time.Minute})
	if _, err := queue.SubmitJob(SubmitJobInput{RequestID: "req-submit", JobID: "job-openclaw", JobType: JobTypeOpenClawSnapshot}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	executor := &blockingOpenClawExecutor{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	supervisor := newTestSupervisor(t, queue, executor)
	supervisor.heartbeatInterval = 5 * time.Millisecond

	resultCh := make(chan SupervisorRunOnceResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := supervisor.RunOnce(context.Background())
		resultCh <- result
		errCh <- err
	}()
	select {
	case <-executor.started:
	case <-time.After(time.Second):
		t.Fatal("executor did not start")
	}
	waitForQueueEvent(t, queue, EventJobHeartbeat)
	close(executor.release)
	if err := <-errCh; err != nil {
		t.Fatalf("run once: %v", err)
	}
	result := <-resultCh
	if result.Job.Status != JobStatusCompleted {
		t.Fatalf("job status = %q, want completed", result.Job.Status)
	}
}

func TestSupervisor_RunLoopStopsOnCancel(t *testing.T) {
	now := projectionTestTime(t, 0)
	queue := newTestQueue(t, &now, QueueOptions{LeaseDuration: time.Minute})
	supervisor := newTestSupervisor(t, queue, testOpenClawSnapshotExecutor{})
	supervisor.pollInterval = 5 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- supervisor.RunLoop(ctx)
	}()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run loop returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("run loop did not stop after context cancellation")
	}
}

func TestSupervisor_RunLoopCancelDoesNotFailRunningJob(t *testing.T) {
	now := projectionTestTime(t, 0)
	queue := newTestQueue(t, &now, QueueOptions{LeaseDuration: time.Minute})
	if _, err := queue.SubmitJob(SubmitJobInput{RequestID: "req-submit", JobID: "job-openclaw", JobType: JobTypeOpenClawSnapshot}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	executor := &blockingOpenClawExecutor{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	supervisor := newTestSupervisor(t, queue, executor)
	supervisor.pollInterval = 5 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- supervisor.RunLoop(ctx)
	}()
	select {
	case <-executor.started:
	case <-time.After(time.Second):
		t.Fatal("executor did not start")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run loop returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("run loop did not stop after context cancellation")
	}
	snapshot, err := queue.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snapshot.Jobs) != 1 || snapshot.Jobs[0].Status != JobStatusRunning {
		t.Fatalf("jobs = %#v, want still running lease without terminal failure", snapshot.Jobs)
	}
}

func TestSupervisor_KillsHungWorker(t *testing.T) {
	now := projectionTestTime(t, 0)
	queue := newTestQueue(t, &now, QueueOptions{LeaseDuration: time.Minute})
	if _, err := queue.SubmitJob(SubmitJobInput{RequestID: "req-submit", JobID: "job-openclaw", JobType: JobTypeOpenClawSnapshot}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	executor := &blockingOpenClawExecutor{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	supervisor := newTestSupervisor(t, queue, executor)
	supervisor.executionTimeout = 20 * time.Millisecond

	result, err := supervisor.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run once: %v", err)
	}
	if result.Job.Status != JobStatusFailed {
		t.Fatalf("job status = %q, want failed", result.Job.Status)
	}
	if result.Job.Failure == nil || !strings.Contains(result.Job.Failure.Message, "executor timed out") {
		t.Fatalf("failure = %#v, want timeout failure", result.Job.Failure)
	}
}

func TestSupervisor_OOMWorkerDoesNotKillDaemon(t *testing.T) {
	now := projectionTestTime(t, 0)
	queue := newTestQueue(t, &now, QueueOptions{LeaseDuration: time.Minute})
	if _, err := queue.SubmitJob(SubmitJobInput{RequestID: "req-oom", JobID: "job-oom", JobType: JobTypeOpenClawSnapshot}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit oom job: %v", err)
	}
	executor := &oneShotOOMExecutor{}
	supervisor := newTestSupervisor(t, queue, executor)
	first, err := supervisor.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("first run once: %v", err)
	}
	if first.Job.Status != JobStatusFailed {
		t.Fatalf("first status = %q, want failed", first.Job.Status)
	}
	if _, err := queue.SubmitJob(SubmitJobInput{RequestID: "req-after", JobID: "job-after", JobType: JobTypeOpenClawSnapshot}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit second job: %v", err)
	}
	second, err := supervisor.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("second run once: %v", err)
	}
	if second.Job.Status != JobStatusCompleted {
		t.Fatalf("second status = %q, want completed after worker OOM failure", second.Job.Status)
	}
}

func newTestSupervisor(t *testing.T, queue *Queue, executor JobExecutor) *Supervisor {
	t.Helper()
	supervisor, err := NewSupervisor(SupervisorOptions{
		Queue:             queue,
		Executors:         []JobExecutor{executor},
		Actor:             "test-supervisor",
		PollInterval:      10 * time.Millisecond,
		HeartbeatInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("new supervisor: %v", err)
	}
	return supervisor
}

type testOpenClawSnapshotExecutor struct {
	Err       error
	Artifacts map[string]string
}

func (e testOpenClawSnapshotExecutor) JobTypes() []JobType {
	return []JobType{JobTypeOpenClawSnapshot}
}

func (e testOpenClawSnapshotExecutor) RunJob(_ context.Context, claim QueueLease) (JobExecutionResult, error) {
	if claim.Job.JobType != JobTypeOpenClawSnapshot {
		return JobExecutionResult{}, errors.New("test executor received unsupported job type")
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

type oneShotOOMExecutor struct {
	calls int
}

func (e *oneShotOOMExecutor) JobTypes() []JobType {
	return []JobType{JobTypeOpenClawSnapshot}
}

func (e *oneShotOOMExecutor) RunJob(_ context.Context, _ QueueLease) (JobExecutionResult, error) {
	e.calls++
	if e.calls == 1 {
		return JobExecutionResult{}, errors.New("worker killed by cgroup memory.max")
	}
	return JobExecutionResult{Artifacts: map[string]string{"executor_policy": "recovered"}}, nil
}

type blockingOpenClawExecutor struct {
	started chan struct{}
	release chan struct{}
}

func (e *blockingOpenClawExecutor) JobTypes() []JobType {
	return []JobType{JobTypeOpenClawSnapshot}
}

func (e *blockingOpenClawExecutor) RunJob(ctx context.Context, claim QueueLease) (JobExecutionResult, error) {
	close(e.started)
	select {
	case <-ctx.Done():
		return JobExecutionResult{}, ctx.Err()
	case <-e.release:
		return JobExecutionResult{Artifacts: map[string]string{"executor_policy": "test"}}, nil
	}
}

func waitForQueueEvent(t *testing.T, queue *Queue, eventType EventType) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		events, err := queue.store.ReadLedger()
		if err != nil {
			t.Fatalf("read ledger: %v", err)
		}
		for _, event := range events {
			if event.EventType == eventType {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s event", eventType)
}
