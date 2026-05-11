// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"context"
	"fmt"
	"time"

	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
)

type overnightDaemonModeOptions struct {
	Enabled  bool
	URL      string
	Token    string
	Fallback bool
	Wait     bool
	Timeout  time.Duration
}

func maybeSubmitOvernightDaemon(
	ctx context.Context,
	cwd string,
	summary *overnightSummary,
	startedAt time.Time,
	opts overnightDaemonModeOptions,
) (bool, error) {
	if !opts.Enabled {
		return false, nil
	}
	result, err := submitOvernightDaemon(ctx, cwd, *summary, opts)
	if err != nil {
		if opts.Fallback {
			summary.Degraded = append(summary.Degraded,
				fmt.Sprintf("daemon-submit: %v; falling back to one-shot", err))
			return false, nil
		}
		return true, err
	}
	applyOvernightDaemonSubmitResult(summary, result)
	if opts.Wait {
		waitErr := waitForOvernightDaemonRun(ctx, cwd, summary, startedAt, result, opts)
		if waitErr != nil {
			return true, waitErr
		}
		return true, nil
	}
	if err := finalizeOvernightSummary(summary, startedAt); err != nil {
		return true, err
	}
	return true, outputOvernightSummary(*summary)
}

func submitOvernightDaemon(
	ctx context.Context,
	cwd string,
	summary overnightSummary,
	opts overnightDaemonModeOptions,
) (daemonpkg.SubmitJobResponse, error) {
	baseURL, err := resolveDaemonURL(cwd, opts.URL)
	if err != nil {
		return daemonpkg.SubmitJobResponse{}, err
	}
	ready, err := fetchDaemonReady(ctx, baseURL)
	if err != nil {
		return daemonpkg.SubmitJobResponse{}, fmt.Errorf("daemon ready check failed: %w", err)
	}
	if !ready.Ready {
		return daemonpkg.SubmitJobResponse{}, fmt.Errorf("daemon is not ready: replay=%s projection=%s", ready.LedgerReplayStatus, ready.ProjectionStatus)
	}
	spec := daemonpkg.NewDreamRunJobSpec(summary.RunID, summary.OutputDir)
	spec.Goal = summary.Goal
	spec.MaxIterations = overnightMaxIterations
	spec.ExecutionTimeout = summary.Runtime.EffectiveTimeout
	job, err := spec.ToJobSpec("job-dream-" + summary.RunID)
	if err != nil {
		return daemonpkg.SubmitJobResponse{}, err
	}
	req := daemonpkg.SubmitJobRequest{
		RequestID:      "req-dream-" + summary.RunID,
		JobID:          job.ID,
		JobType:        job.Type,
		IdempotencyKey: "dream.run:" + summary.RunID,
		Payload:        job.Payload,
	}
	return postDaemonSubmitJob(ctx, baseURL, opts.Token, req)
}

func applyOvernightDaemonSubmitResult(summary *overnightSummary, result daemonpkg.SubmitJobResponse) {
	summary.Mode = "dream.daemon-queue"
	summary.Status = "queued"
	summary.Steps = []overnightStepSummary{{
		Name:    "daemon-submit",
		Status:  "done",
		Command: "agentopsd /v1/jobs",
		Note: fmt.Sprintf("job=%s request=%s status=%s projection=%s",
			result.JobID, result.RequestID, result.Status, result.ProjectionStatus),
	}}
	if summary.Artifacts == nil {
		summary.Artifacts = map[string]string{}
	}
	summary.Artifacts["daemon_summary_json"] = summary.Artifacts["summary_json"]
}

func waitForOvernightDaemonRun(
	ctx context.Context,
	cwd string,
	summary *overnightSummary,
	startedAt time.Time,
	result daemonpkg.SubmitJobResponse,
	opts overnightDaemonModeOptions,
) error {
	baseURL, err := resolveDaemonURL(cwd, opts.URL)
	if err != nil {
		return err
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	job, waitErr := waitForDaemonJobStatus(waitCtx, baseURL, result.JobID, timeout)
	applyOvernightDaemonWaitResult(summary, result, job, waitErr)
	if err := finalizeOvernightSummary(summary, startedAt); err != nil {
		return err
	}
	if err := outputOvernightSummary(*summary); err != nil {
		return err
	}
	if waitErr != nil {
		return waitErr
	}
	switch job.Status {
	case daemonpkg.JobStatusCompleted:
		return nil
	case daemonpkg.JobStatusFailed, daemonpkg.JobStatusCancelled:
		return fmt.Errorf("daemon dream job %s ended with status %s", job.JobID, job.Status)
	default:
		return fmt.Errorf("daemon dream job %s ended with non-terminal status %s", job.JobID, job.Status)
	}
}

func applyOvernightDaemonWaitResult(
	summary *overnightSummary,
	result daemonpkg.SubmitJobResponse,
	job daemonpkg.QueueJobState,
	waitErr error,
) {
	summary.Mode = "dream.daemon-run"
	status := string(job.Status)
	if status == "" {
		status = "unknown"
	}
	if waitErr != nil {
		status = "timeout"
	}
	summary.Status = status
	waitStatus := "done"
	waitNote := fmt.Sprintf("job=%s status=%s", result.JobID, status)
	if waitErr != nil {
		waitStatus = "failed"
		waitNote = waitErr.Error()
	} else if job.Status != daemonpkg.JobStatusCompleted {
		waitStatus = "failed"
		if job.Failure != nil {
			waitNote = fmt.Sprintf("job=%s status=%s failure=%s %s", job.JobID, job.Status, job.Failure.Code, job.Failure.Message)
		}
	}
	summary.Steps = append(summary.Steps, overnightStepSummary{
		Name:    "daemon-wait",
		Status:  waitStatus,
		Command: "ao daemon jobs wait " + result.JobID,
		Note:    waitNote,
	})
	if summary.Artifacts == nil {
		summary.Artifacts = map[string]string{}
	}
	for key, value := range job.Artifacts {
		summary.Artifacts[key] = value
	}
}
