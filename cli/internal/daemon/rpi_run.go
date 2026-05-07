package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
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
//
// Store/Clock/Actor are optional. When Store is non-nil, RunJob emits agent-update
// ledger events on phase boundaries (phase_start before the runner; phase_complete
// + phase_handoff after) per soc-y0ct.2. When Store is nil, no events are emitted —
// preserves back-compat with sub-wave 5a tests that injected a fake runner only.
type RPIRunExecutorOptions struct {
	Run   RPIRunFunc
	Root  string
	Store *Store
	Clock func() time.Time
	Actor string
}

// RPIRunExecutor is a JobExecutor for rpi.run that calls the injected runner
// in-process. Replaces RPICLIExecutor's shell-out path on the daemon
// CLI-fallback wire-up. Mirrors DreamExecutor's function-pointer pattern.
//
// soc-bcrn.3.6 (E3.W4 sub-5a): see RPIRunRequest for context.
// soc-y0ct.2: emits agent-update events on phase boundaries when store != nil.
type RPIRunExecutor struct {
	run   RPIRunFunc
	root  string
	store *Store
	now   func() time.Time
	actor string
}

// NewRPIRunExecutor constructs an RPIRunExecutor. Run and Root are required;
// Store/Clock/Actor are optional and gate agent-update emission.
func NewRPIRunExecutor(opts RPIRunExecutorOptions) (*RPIRunExecutor, error) {
	if opts.Run == nil {
		return nil, errors.New("rpi run executor: Run is required")
	}
	if strings.TrimSpace(opts.Root) == "" {
		return nil, errors.New("rpi run executor: Root is required")
	}
	now := opts.Clock
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	actor := strings.TrimSpace(opts.Actor)
	if actor == "" {
		actor = "agentopsd-rpi-run"
	}
	return &RPIRunExecutor{run: opts.Run, root: opts.Root, store: opts.Store, now: now, actor: actor}, nil
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

	queue := e.queueOrNil()
	e.emitAgentUpdate(claim, queue, NewAgentUpdatePhaseStartEvent(AgentUpdatePhaseStart{
		PhaseName: "rpi.run",
		RunID:     spec.RunID,
		Timestamp: e.now().UTC().Format(time.RFC3339Nano),
	}))

	startedAt := e.now()
	res, err := e.run(ctx, RPIRunRequest{Spec: spec, Claim: claim, Root: e.root})
	durationMs := e.now().Sub(startedAt).Milliseconds()

	// Merge runner-emitted artifacts (success or failure) so operators can inspect
	// partial outputs (e.g., the run log path the runner already opened).
	for k, v := range res.Artifacts {
		artifacts[k] = v
	}

	completeStatus := "success"
	if err != nil {
		completeStatus = "failure"
	}
	e.emitAgentUpdate(claim, queue, NewAgentUpdatePhaseCompleteEvent(AgentUpdatePhaseComplete{
		PhaseName:  "rpi.run",
		RunID:      spec.RunID,
		Timestamp:  e.now().UTC().Format(time.RFC3339Nano),
		Status:     completeStatus,
		DurationMs: durationMs,
		Artifacts:  res.Artifacts,
	}))

	// soc-awx8: emit one criterion_verdict event per verdict found in any wave
	// checkpoint under .agents/crank/wave-*-checkpoint.json. Best-effort —
	// missing/unreadable checkpoints emit zero events and do NOT fail the job.
	// Order: AFTER phase_complete, BEFORE phase_handoff, so consumers see the
	// per-criterion results as part of the closing-out sequence.
	if err == nil {
		for _, v := range readWaveCheckpointVerdicts(e.root) {
			e.emitAgentUpdate(claim, queue, NewAgentUpdateCriterionVerdictEvent(AgentUpdateCriterionVerdict{
				CriterionID:  v.ID,
				Status:       v.Status,
				EvidencePath: v.EvidencePath,
				Notes:        v.Notes,
				RunID:        spec.RunID,
				Timestamp:    e.now().UTC().Format(time.RFC3339Nano),
			}))
		}
	}

	if err == nil {
		e.emitAgentUpdate(claim, queue, NewAgentUpdatePhaseHandoffEvent(AgentUpdatePhaseHandoff{
			FromPhase:  "rpi.run",
			ToPhase:    "operator",
			RunID:      spec.RunID,
			Timestamp:  e.now().UTC().Format(time.RFC3339Nano),
			PacketPath: spec.ExecutionPacketPath,
		}))
	}

	return JobExecutionResult{Artifacts: artifacts}, err
}

// queueOrNil returns a Queue scoped to this executor's actor/clock when a Store
// is configured, otherwise nil. Used to mint deterministic event IDs without
// forcing every caller to pass a Queue through.
func (e *RPIRunExecutor) queueOrNil() *Queue {
	if e.store == nil {
		return nil
	}
	return NewQueue(e.store, QueueOptions{Actor: e.actor, Now: e.now})
}

// emitAgentUpdate stamps the boilerplate ledger fields onto a payload-only
// LedgerEventInput from the agent_update.go constructors and appends to the
// store. No-op when the executor has no Store configured (back-compat with
// callers that only inject a runner). Errors are logged via the queue contract
// but do not fail the job — the runner result is the source of truth for the
// supervisor's terminal-record.
func (e *RPIRunExecutor) emitAgentUpdate(claim QueueClaim, queue *Queue, base LedgerEventInput) {
	if e.store == nil || queue == nil {
		return
	}
	requestID := RequestID(claim.Job.RequestID)
	if strings.TrimSpace(string(requestID)) == "" {
		requestID = RequestID("req-" + claim.Job.JobID)
	}
	base.EventID = queue.nextEventID(base.EventType, claim.Job.JobID)
	base.RequestID = requestID
	base.JobID = claim.Job.JobID
	base.JobType = claim.Job.JobType
	base.Actor = e.actor
	base.OccurredAt = e.now()
	event, err := NewLedgerEvent(base)
	if err != nil {
		return
	}
	_, _ = e.store.AppendLedgerEvent(event)
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

// waveCheckpointVerdict mirrors the verdict-row shape /crank writes into
// `.agents/crank/wave-*-checkpoint.json` per skills/crank/SKILL.md Step 5.7.
// Verdict source-of-truth is the per-criterion-rubric reference doc; this is
// the read-side projection used by the daemon's agent-update emitter.
type waveCheckpointVerdict struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	EvidencePath string `json:"evidence_path,omitempty"`
	Notes        string `json:"notes,omitempty"`
}

// readWaveCheckpointVerdicts scans `<root>/.agents/crank/wave-*-checkpoint.json`
// and returns the flattened list of `criterion_verdicts` rows. Best-effort:
// missing dir, unreadable files, and malformed JSON yield an empty slice (no
// error surfaced) so the daemon's emission loop degrades to "zero verdict
// events" rather than failing the job. Filenames are scanned in lexicographic
// order so wave-1, wave-2, ... emit in the same order operators would read
// them off disk.
func readWaveCheckpointVerdicts(root string) []waveCheckpointVerdict {
	matches, err := filepath.Glob(filepath.Join(root, ".agents", "crank", "wave-*-checkpoint.json"))
	if err != nil || len(matches) == 0 {
		return nil
	}
	var verdicts []waveCheckpointVerdict
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var checkpoint struct {
			CriterionVerdicts []waveCheckpointVerdict `json:"criterion_verdicts"`
		}
		if err := json.Unmarshal(data, &checkpoint); err != nil {
			continue
		}
		verdicts = append(verdicts, checkpoint.CriterionVerdicts...)
	}
	return verdicts
}
