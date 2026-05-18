// practices: [agile-manifesto, dora-metrics]
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
)

func maybeSubmitRPIPhasedDaemon(ctx context.Context, opts phasedEngineOptions, args []string) (bool, error) {
	if !opts.DaemonSubmit {
		return false, nil
	}
	result, err := submitRPIPhasedDaemon(ctx, opts, args)
	if err != nil {
		if opts.DaemonFallback {
			fmt.Printf("agentopsd unavailable, falling back to foreground RPI: %v\n", err)
			return false, nil
		}
		return true, err
	}
	fmt.Printf("RPI daemon submit accepted: job=%s request=%s status=%s\n", result.JobID, result.RequestID, result.Status)
	return true, nil
}

func submitRPIPhasedDaemon(ctx context.Context, opts phasedEngineOptions, args []string) (daemonpkg.SubmitJobResponse, error) {
	cwd := strings.TrimSpace(opts.WorkingDir)
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return daemonpkg.SubmitJobResponse{}, fmt.Errorf("get working directory: %w", err)
		}
	}
	baseURL, err := resolveDaemonURL(cwd, opts.DaemonURL)
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

	goal, startPhase, err := resolveGoalAndStartPhase(opts, args, cwd)
	if err != nil {
		return daemonpkg.SubmitJobResponse{}, err
	}
	runID := strings.TrimSpace(opts.RunID)
	if runID == "" {
		runID = generateRunID()
	}
	spec := daemonpkg.NewRPIRunJobSpec(runID, goal)
	spec.StartPhase = startPhase
	spec.MaxPhase = 3
	spec.TestFirst = opts.TestFirst
	spec.Complexity = string(classifyComplexity(goal))
	spec.Backend = daemonpkg.RPIBackendGasCityAPI
	if opts.PhaseTimeout > 0 {
		spec.PhaseTimeout = opts.PhaseTimeout.String()
	}
	job, err := spec.ToJobSpec("job-rpi-" + runID)
	if err != nil {
		return daemonpkg.SubmitJobResponse{}, err
	}
	req := daemonpkg.SubmitJobRequest{
		RequestID:      "req-rpi-" + runID,
		JobID:          job.ID,
		JobType:        job.Type,
		IdempotencyKey: "rpi.run:" + runID,
		Payload:        job.Payload,
	}
	return postDaemonSubmitJob(ctx, baseURL, opts.DaemonToken, req)
}

func postDaemonSubmitJob(ctx context.Context, baseURL, token string, request daemonpkg.SubmitJobRequest) (daemonpkg.SubmitJobResponse, error) {
	var response daemonpkg.SubmitJobResponse
	if err := ensureDaemonSubmitRetryIdempotencyKey(&request); err != nil {
		return response, err
	}
	data, err := json.Marshal(request)
	if err != nil {
		return response, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/v1/jobs", bytes.NewReader(data))
	if err != nil {
		return response, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if token != "" {
		httpReq.Header.Set(daemonpkg.DefaultMutationTokenHeader, token)
	}
	resp, err := (&http.Client{}).Do(httpReq)
	if err != nil {
		return response, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return response, fmt.Errorf("daemon submit returned HTTP %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return response, err
	}
	return response, nil
}

// ensureDaemonSubmitRetryIdempotencyKey gives CLI-originated daemon submissions
// a stable retry key when the caller did not provide a semantic one.
func ensureDaemonSubmitRetryIdempotencyKey(request *daemonpkg.SubmitJobRequest) error {
	if request == nil || strings.TrimSpace(request.IdempotencyKey) != "" {
		return nil
	}
	material := struct {
		JobType   daemonpkg.JobType `json:"job_type"`
		JobID     string            `json:"job_id,omitempty"`
		RequestID string            `json:"request_id,omitempty"`
		Payload   map[string]any    `json:"payload,omitempty"`
	}{
		JobType:   request.JobType,
		JobID:     strings.TrimSpace(request.JobID),
		RequestID: strings.TrimSpace(request.RequestID),
		Payload:   request.Payload,
	}
	data, err := json.Marshal(material)
	if err != nil {
		return fmt.Errorf("marshal daemon submit idempotency material: %w", err)
	}
	sum := sha256.Sum256(data)
	request.IdempotencyKey = fmt.Sprintf("cli-submit:%s:%x", request.JobType, sum[:12])
	return nil
}
