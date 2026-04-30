package daemon

import (
	"context"
	"errors"
	"fmt"

	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
)

// RPIJobExecutor is a JobExecutor (Supervisor-compatible) for rpi.run and
// rpi.phase jobs. The Supervisor handles claim/heartbeat/terminal-write; this
// executor only runs the user-visible RPI work for an already-claimed job by
// delegating to an RPIPhaseExecutor (today: GasCityRPIPhaseExecutor).
//
// soc-5of.8 (TB-Δ8) — production caller for daemon-submitted RPI jobs under
// the gascity executor policy. Symmetric in shape to WikiForgeExecutor +
// DreamExecutor.
type RPIJobExecutor struct {
	runner *RPIRunner
}

// RPIJobExecutorOptions configures an RPIJobExecutor.
type RPIJobExecutorOptions struct {
	Store         *Store
	Executor      RPIPhaseExecutor
	PromptBuilder RPIPromptBuilder
	// RegistryWriter is optional; used to keep the rpi-registry projection
	// fresh after each phase. Pass nil to disable writes.
	RegistryWriter cliRPI.RunRegistryWriter
}

// NewRPIJobExecutor constructs an executor for rpi.run / rpi.phase jobs. The
// underlying RPIRunner is reused so behavior matches the operator-driven
// `ao rpi run` path; only the top-level claim/terminal-record loop differs
// (Supervisor owns that).
func NewRPIJobExecutor(opts RPIJobExecutorOptions) (*RPIJobExecutor, error) {
	if opts.Store == nil {
		return nil, errors.New("daemon rpi executor: store is required")
	}
	if opts.Executor == nil {
		return nil, ErrRPIExecutorRequired
	}
	runner, err := NewRPIRunner(opts.Store, RPIRunnerOptions{
		Executor:       opts.Executor,
		PromptBuilder:  opts.PromptBuilder,
		RegistryWriter: opts.RegistryWriter,
	})
	if err != nil {
		return nil, err
	}
	return &RPIJobExecutor{runner: runner}, nil
}

// JobTypes reports the daemon job types this executor handles.
func (e *RPIJobExecutor) JobTypes() []JobType {
	return []JobType{JobTypeRPIRun, JobTypeRPIPhase}
}

// RunJob executes a claimed RPI job. The supervisor wraps this call with
// claim/heartbeat/terminal-record bookkeeping; we only contribute the
// per-job execution.
func (e *RPIJobExecutor) RunJob(ctx context.Context, claim QueueClaim) (JobExecutionResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if !isRPIJobType(claim.Job.JobType) {
		return JobExecutionResult{}, fmt.Errorf("rpi executor does not support job type %s", claim.Job.JobType)
	}
	artifacts, _, err := e.runner.ExecuteClaim(ctx, claim)
	return JobExecutionResult{Artifacts: artifacts}, err
}
