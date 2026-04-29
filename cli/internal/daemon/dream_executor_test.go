package daemon

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDreamExecutorCompletesJobWithArtifacts(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cwd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cwd, ".agents"), 0o755); err != nil {
		t.Fatalf("mkdir .agents: %v", err)
	}
	now := projectionTestTime(t, 0)
	queue := newTestQueue(t, &now, QueueOptions{LeaseDuration: time.Minute})
	spec := NewDreamRunJobSpec("dream-daemon-test", filepath.Join(cwd, ".agents", "overnight", "dream-daemon-test"))
	spec.MaxIterations = 1
	spec.ExecutionTimeout = "30s"
	jobSpec, err := spec.ToJobSpec("job-dream-daemon-test")
	if err != nil {
		t.Fatalf("ToJobSpec: %v", err)
	}
	if _, err := queue.SubmitJob(SubmitJobInput{
		RequestID: "req-dream-daemon-test",
		JobID:     jobSpec.ID,
		JobType:   jobSpec.Type,
		Payload:   jobSpec.Payload,
	}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	executor, err := NewDreamExecutor(DreamExecutorOptions{
		Cwd: cwd,
		RunLoop: func(ctx context.Context, opts DreamRunLoopOptions) (DreamRunLoopResult, error) {
			_, _ = io.WriteString(opts.LogWriter, "dream loop completed\n")
			return DreamRunLoopResult{IterationCount: 1}, nil
		},
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewDreamExecutor: %v", err)
	}
	supervisor := newTestSupervisor(t, queue, executor)

	result, err := supervisor.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if result.Job.Status != JobStatusCompleted {
		t.Fatalf("job status = %q, want completed", result.Job.Status)
	}
	for _, key := range []string{"summary_json", "summary_markdown", "overnight_log"} {
		path := result.Job.Artifacts[key]
		if path == "" {
			t.Fatalf("missing artifact %q in %#v", key, result.Job.Artifacts)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("artifact %s stat: %v", key, err)
		}
	}
	if result.Job.Artifacts["failure_report"] != "" {
		t.Fatalf("unexpected failure_report artifact: %#v", result.Job.Artifacts)
	}
}

func TestDreamExecutorExecutionTimeoutFailsJob(t *testing.T) {
	cwd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cwd, ".agents"), 0o755); err != nil {
		t.Fatalf("mkdir .agents: %v", err)
	}
	now := projectionTestTime(t, 0)
	queue := newTestQueue(t, &now, QueueOptions{LeaseDuration: time.Minute})
	spec := NewDreamRunJobSpec("dream-timeout", filepath.Join(cwd, ".agents", "overnight", "dream-timeout"))
	spec.ExecutionTimeout = "1ms"
	jobSpec, err := spec.ToJobSpec("job-dream-timeout")
	if err != nil {
		t.Fatalf("ToJobSpec: %v", err)
	}
	if _, err := queue.SubmitJob(SubmitJobInput{
		RequestID: "req-dream-timeout",
		JobID:     jobSpec.ID,
		JobType:   jobSpec.Type,
		Payload:   jobSpec.Payload,
	}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	executor, err := NewDreamExecutor(DreamExecutorOptions{
		Cwd: cwd,
		RunLoop: func(ctx context.Context, opts DreamRunLoopOptions) (DreamRunLoopResult, error) {
			_, _ = io.WriteString(opts.LogWriter, "blocking until daemon execution timeout\n")
			<-ctx.Done()
			return DreamRunLoopResult{}, ctx.Err()
		},
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewDreamExecutor: %v", err)
	}
	supervisor := newTestSupervisor(t, queue, executor)

	result, err := supervisor.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if result.Job.Status != JobStatusFailed {
		t.Fatalf("job status = %q, want failed", result.Job.Status)
	}
	if result.Job.Failure == nil || result.Job.Failure.Code != FailureRequestRejected {
		t.Fatalf("failure = %#v, want request_rejected", result.Job.Failure)
	}
	for _, key := range []string{"summary_json", "summary_markdown", "overnight_log", "failure_report"} {
		path := result.Job.Artifacts[key]
		if path == "" {
			t.Fatalf("missing artifact %q in %#v", key, result.Job.Artifacts)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("artifact %s stat: %v", key, err)
		}
	}
}
