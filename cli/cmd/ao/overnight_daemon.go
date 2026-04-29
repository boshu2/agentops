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
	summary.Mode = "dream.daemon-submit"
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
