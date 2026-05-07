package daemon

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// RPIRunRequest is the per-job input handed to the runner function injected
// into RPIRunExecutor. It carries the parsed RPIRunJobSpec, the original
// QueueClaim (so the runner can correlate IDs / heartbeat metadata), and the
// daemon root cwd.
//
// soc-bcrn.3.6 (E3.W4 sub-5a): introduced as part of the in-process executor
// swap that retires RPICLIExecutor's shell-out path. The cmd/ao package
// supplies the runner so the daemon stays free of cmd/ao imports.
type RPIRunRequest struct {
	Spec  RPIRunJobSpec
	Claim QueueClaim
	Root  string
}

// RPIRunResult is the per-job output from the injected runner. Artifacts are
// merged on top of the executor-default artifacts before being handed back to
// the supervisor.
type RPIRunResult struct {
	Artifacts map[string]string
}

// RPIRunFunc executes a single rpi.run claim in-process. The cmd/ao package
// (agentopsd.go) supplies the implementation; this lets the daemon package
// stay free of cmd/ao imports. Mirrors the function-pointer pattern used by
// DreamExecutor.
type RPIRunFunc func(ctx context.Context, req RPIRunRequest) (RPIRunResult, error)

// RPIRunExecutorOptions configures NewRPIRunExecutor.
type RPIRunExecutorOptions struct {
	Run  RPIRunFunc
	Root string
}

// RPIRunExecutor is a JobExecutor for rpi.run that calls the injected runner
// in-process. Replaces RPICLIExecutor's shell-out path on the daemon
// CLI-fallback wire-up. Mirrors DreamExecutor's function-pointer pattern.
//
// soc-bcrn.3.6 (E3.W4 sub-5a): see RPIRunRequest for context.
type RPIRunExecutor struct {
	run  RPIRunFunc
	root string
}

// NewRPIRunExecutor constructs an RPIRunExecutor. Both the runner function and
// the root cwd are required.
func NewRPIRunExecutor(opts RPIRunExecutorOptions) (*RPIRunExecutor, error) {
	if opts.Run == nil {
		return nil, errors.New("rpi run executor: Run is required")
	}
	if strings.TrimSpace(opts.Root) == "" {
		return nil, errors.New("rpi run executor: Root is required")
	}
	return &RPIRunExecutor{run: opts.Run, root: opts.Root}, nil
}

// JobTypes reports the queue job types this executor handles.
func (e *RPIRunExecutor) JobTypes() []JobType { return []JobType{JobTypeRPIRun} }

// RunJob parses the spec, validates it (only full cycles supported), and
// dispatches to the injected runner. Runner-emitted artifacts are merged on
// top of the executor-default artifacts so per-run output paths surface to the
// supervisor's terminal record.
//
// RunJob requires a non-nil ctx; callers passing nil receive a Background ctx.
func (e *RPIRunExecutor) RunJob(ctx context.Context, claim QueueClaim) (JobExecutionResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if claim.Job.JobType != JobTypeRPIRun {
		return JobExecutionResult{}, fmt.Errorf("rpi run executor does not support job type %s", claim.Job.JobType)
	}
	spec, err := RPIRunJobSpecFromPayload(claim.Job.Payload)
	if err != nil {
		return JobExecutionResult{}, err
	}
	artifacts := rpiRunArtifactsFor(claim, spec)
	if err := validateRPIRunFullRun(spec); err != nil {
		return JobExecutionResult{Artifacts: artifacts}, err
	}
	res, err := e.run(ctx, RPIRunRequest{Spec: spec, Claim: claim, Root: e.root})
	if err != nil {
		// Merge any runner-emitted artifacts even on failure so operators can
		// inspect partial outputs (e.g., the run log path the runner already
		// opened) alongside the failure record.
		for k, v := range res.Artifacts {
			artifacts[k] = v
		}
		return JobExecutionResult{Artifacts: artifacts}, err
	}
	for k, v := range res.Artifacts {
		artifacts[k] = v
	}
	return JobExecutionResult{Artifacts: artifacts}, nil
}

// rpiRunArtifactsFor mirrors RPICLIExecutor.artifactsFor but tags
// executor_policy="in-process" and backend="in-process" so downstream readers
// can distinguish the new path from the legacy CLI-fallback shell-out during
// the 5a/5b/5c migration.
func rpiRunArtifactsFor(claim QueueClaim, spec RPIRunJobSpec) map[string]string {
	runID := sanitizeIDPart(spec.RunID)
	if runID == "" {
		runID = "unknown-run"
	}
	jobID := sanitizeIDPart(claim.Job.JobID)
	if jobID == "" {
		jobID = "unknown-job"
	}
	logPath := filepath.ToSlash(filepath.Join(".agents", "daemon", "rpi", "runs", runID, jobID, "rpi-run.log"))
	return map[string]string{
		"executor_policy":     "in-process",
		"backend":             "in-process",
		"requested_backend":   string(spec.Backend),
		"run_id":              spec.RunID,
		"goal":                spec.Goal,
		"rpi_run_log":         logPath,
		"landing_policy":      "off",
		"rpi_wrapper_command": "ao rpi loop --supervisor",
	}
}

// validateRPIRunFullRun mirrors validateRPICLIFullRun: only full cycles are
// supported on the CLI-fallback wire-up (start_phase=1, max_phase=3). Partial
// phase runs flow through the gascity path's RPIJobExecutor instead.
func validateRPIRunFullRun(spec RPIRunJobSpec) error {
	if spec.StartPhase != 1 || spec.MaxPhase != 3 {
		return fmt.Errorf("rpi run executor only supports full rpi.run cycles: start_phase=%d max_phase=%d", spec.StartPhase, spec.MaxPhase)
	}
	return nil
}

var _ JobExecutor = (*RPIRunExecutor)(nil)
