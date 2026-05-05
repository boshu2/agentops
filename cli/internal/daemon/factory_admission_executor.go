package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type FactoryAdmissionEvidenceProviderFactory func(FactoryWorkOrder) FactoryAdmissionEvidenceProvider

type FactoryAdmissionExecutorOptions struct {
	Store                   *Store
	Root                    string
	Clock                   func() time.Time
	EvidenceProviderFactory FactoryAdmissionEvidenceProviderFactory
	EnableRPIHandoff        bool
	Actor                   string
}

type FactoryAdmissionExecutor struct {
	store                   *Store
	root                    string
	now                     func() time.Time
	evidenceProviderFactory FactoryAdmissionEvidenceProviderFactory
	enableRPIHandoff        bool
	actor                   string
}

type LocalFactoryAdmissionEvidenceProvider struct {
	Root      string
	WorkOrder FactoryWorkOrder
}

func NewFactoryAdmissionExecutor(opts FactoryAdmissionExecutorOptions) (*FactoryAdmissionExecutor, error) {
	if opts.Store == nil {
		return nil, errors.New("factory admission executor: store is required")
	}
	root := strings.TrimSpace(opts.Root)
	if root == "" {
		root = opts.Store.root
	}
	if root == "" {
		return nil, errors.New("factory admission executor: root is required")
	}
	now := opts.Clock
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	factory := opts.EvidenceProviderFactory
	if factory == nil {
		factory = func(work FactoryWorkOrder) FactoryAdmissionEvidenceProvider {
			return LocalFactoryAdmissionEvidenceProvider{Root: root, WorkOrder: work}
		}
	}
	actor := strings.TrimSpace(opts.Actor)
	if actor == "" {
		actor = "agentopsd-factory"
	}
	return &FactoryAdmissionExecutor{
		store:                   opts.Store,
		root:                    root,
		now:                     now,
		evidenceProviderFactory: factory,
		enableRPIHandoff:        opts.EnableRPIHandoff,
		actor:                   actor,
	}, nil
}

func (e *FactoryAdmissionExecutor) JobTypes() []JobType {
	return []JobType{JobTypeFactoryAdmission, JobTypeFactoryLocalPilot}
}

func (e *FactoryAdmissionExecutor) RunJob(ctx context.Context, claim QueueClaim) (JobExecutionResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	switch claim.Job.JobType {
	case JobTypeFactoryAdmission:
		spec, err := FactoryAdmissionJobSpecFromPayload(claim.Job.Payload)
		if err != nil {
			return JobExecutionResult{}, err
		}
		return e.runAdmission(ctx, claim, spec.RunID, spec.Mode, spec.WorkOrder, spec.Handoff)
	case JobTypeFactoryLocalPilot:
		spec, err := FactoryLocalPilotJobSpecFromPayload(claim.Job.Payload)
		if err != nil {
			return JobExecutionResult{}, err
		}
		return e.runAdmission(ctx, claim, spec.RunID, spec.Mode, spec.WorkOrder, spec.Handoff)
	default:
		return JobExecutionResult{}, fmt.Errorf("factory admission executor does not support job type %s", claim.Job.JobType)
	}
}

func (e *FactoryAdmissionExecutor) runAdmission(ctx context.Context, claim QueueClaim, runID string, mode FactoryAdmissionMode, work FactoryWorkOrder, handoff FactoryHandoff) (JobExecutionResult, error) {
	queue := NewQueue(e.store, QueueOptions{Actor: e.actor, Now: e.now})
	evaluator := FactoryAdmissionEvaluator{
		Clock:    e.now,
		Evidence: e.evidenceProviderFactory(work),
	}
	decision, err := evaluator.evaluate(ctx, runID, work)
	if err != nil {
		return JobExecutionResult{}, err
	}

	if factoryAdmissionWantsRPIHandoff(mode, handoff) && decision.Allowed {
		if !e.enableRPIHandoff {
			decision = factoryBlockedAdmissionDecision(decision, FactoryAdmissionReasonRPIHandoffUnavailable)
		} else {
			childJobID, err := e.submitRPIHandoff(ctx, queue, claim, runID, work, handoff)
			if err != nil {
				return JobExecutionResult{}, err
			}
			decision.ChildJobID = childJobID
		}
	}

	artifacts, err := e.writeAdmissionArtifacts(runID, work, decision)
	if err != nil {
		return JobExecutionResult{}, err
	}
	decision.ArtifactRefs = cloneStringMap(artifacts)
	if err := writeFactoryAdmissionJSON(e.root, artifacts["admission"], decision); err != nil {
		return JobExecutionResult{}, err
	}
	if err := decision.Validate(); err != nil {
		return JobExecutionResult{}, fmt.Errorf("admission decision: %w", err)
	}
	if err := e.appendAdmissionEvent(queue, claim, decision, artifacts); err != nil {
		return JobExecutionResult{}, err
	}

	resultArtifacts := cloneStringMap(artifacts)
	resultArtifacts["allowed"] = fmt.Sprintf("%t", decision.Allowed)
	if decision.ChildJobID != "" {
		resultArtifacts["child_job_id"] = decision.ChildJobID
	}
	if len(decision.Reasons) > 0 {
		resultArtifacts["reasons"] = strings.Join(decision.Reasons, ",")
	}
	return JobExecutionResult{Artifacts: resultArtifacts}, nil
}

func (e *FactoryAdmissionExecutor) submitRPIHandoff(ctx context.Context, queue *Queue, claim QueueClaim, runID string, work FactoryWorkOrder, handoff FactoryHandoff) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	childJobID := sanitizeIDPart(claim.Job.JobID + "-rpi")
	if childJobID == "" {
		childJobID = sanitizeIDPart(runID + "-rpi")
	}
	spec := NewRPIRunJobSpec(runID, work.Target.Summary)
	spec.ExecutionPacketPath = handoff.ExecutionPacketPath
	spec.EpicID = handoff.EpicID
	jobSpec, err := spec.ToJobSpec(childJobID)
	if err != nil {
		return "", err
	}
	requestID := claim.Job.RequestID + "-rpi"
	if strings.TrimSpace(requestID) == "-rpi" {
		requestID = "req-" + childJobID
	}
	submitted, err := queue.SubmitJob(SubmitJobInput{
		RequestID:      RequestID(requestID),
		JobID:          jobSpec.ID,
		JobType:        jobSpec.Type,
		IdempotencyKey: fmt.Sprintf("factory-admission:%s:rpi", work.WorkOrderID),
		Actor:          e.actor,
		Payload:        jobSpec.Payload,
	}, QueueMutationOptions{})
	if err != nil {
		return "", err
	}
	return submitted.JobID, nil
}

func (e *FactoryAdmissionExecutor) appendAdmissionEvent(queue *Queue, claim QueueClaim, decision FactoryAdmissionDecision, artifacts map[string]string) error {
	payload := map[string]any{
		"run_id":         decision.RunID,
		"work_order_id":  decision.WorkOrderID,
		"allowed":        decision.Allowed,
		"reasons":        append([]string{}, decision.Reasons...),
		"landing_policy": string(decision.LandingPolicy),
		"digest_policy":  string(decision.DigestPolicy),
		"artifact_refs":  cloneStringMap(artifacts),
		"artifacts":      cloneStringMap(artifacts),
		"evidence":       decision.Evidence,
		"objective":      decision.WorkOrderID,
		"requested_by":   e.actor,
	}
	if decision.ChildJobID != "" {
		payload["child_job_id"] = decision.ChildJobID
	}
	requestID := RequestID(claim.Job.RequestID)
	if strings.TrimSpace(string(requestID)) == "" {
		requestID = RequestID("req-" + claim.Job.JobID)
	}
	event, err := NewLedgerEvent(LedgerEventInput{
		EventID:    queue.nextEventID(EventFactoryAdmissionDecided, claim.Job.JobID),
		RequestID:  requestID,
		JobID:      claim.Job.JobID,
		EventType:  EventFactoryAdmissionDecided,
		OccurredAt: e.now(),
		Actor:      e.actor,
		JobType:    claim.Job.JobType,
		Payload:    payload,
	})
	if err != nil {
		return err
	}
	_, err = e.store.AppendLedgerEvent(event)
	return err
}

func (e *FactoryAdmissionExecutor) writeAdmissionArtifacts(runID string, work FactoryWorkOrder, decision FactoryAdmissionDecision) (map[string]string, error) {
	dir := factoryAdmissionRunDir(runID)
	artifacts := map[string]string{
		"work_order":       filepath.ToSlash(filepath.Join(dir, "work-order.json")),
		"admission":        filepath.ToSlash(filepath.Join(dir, "admission.json")),
		"blocker_matrix":   filepath.ToSlash(filepath.Join(dir, "blocker-matrix.json")),
		"main_ci_baseline": filepath.ToSlash(filepath.Join(dir, "main-ci-baseline.json")),
	}
	if err := writeFactoryAdmissionJSON(e.root, artifacts["work_order"], work); err != nil {
		return nil, err
	}
	if err := writeFactoryAdmissionJSON(e.root, artifacts["blocker_matrix"], FactoryPRBlockerMatrix{Known: true, Blockers: work.OpenPRBlockers}); err != nil {
		return nil, err
	}
	if err := writeFactoryAdmissionJSON(e.root, artifacts["main_ci_baseline"], work.MainCIBaseline); err != nil {
		return nil, err
	}
	return artifacts, nil
}

func factoryAdmissionRunDir(runID string) string {
	runID = sanitizeIDPart(runID)
	if runID == "" {
		runID = "unknown-run"
	}
	return filepath.ToSlash(filepath.Join(".agents", "daemon", "factory", "runs", runID))
}

func writeFactoryAdmissionJSON(root, relPath string, value any) error {
	if strings.TrimSpace(relPath) == "" {
		return fmt.Errorf("factory admission artifact path is required")
	}
	absPath := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(absPath), 0o700); err != nil {
		return fmt.Errorf("create factory admission artifact dir: %w", err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(absPath, data, 0o600); err != nil {
		return fmt.Errorf("write factory admission artifact %s: %w", relPath, err)
	}
	return nil
}

func factoryAdmissionWantsRPIHandoff(mode FactoryAdmissionMode, handoff FactoryHandoff) bool {
	return mode == FactoryAdmissionModeRPIHandoff && handoff.Kind == FactoryHandoffRPI
}

func factoryBlockedAdmissionDecision(decision FactoryAdmissionDecision, reason string) FactoryAdmissionDecision {
	decision.Allowed = false
	decision.ChildJobID = ""
	decision.Reasons = uniqueNonEmptyFactoryAdmissionReasons(append(decision.Reasons, reason))
	return decision
}

func (p LocalFactoryAdmissionEvidenceProvider) RepoState(ctx context.Context) (FactoryRepoState, error) {
	head, err := runFactoryAdmissionGit(ctx, p.Root, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return FactoryRepoState{}, err
	}
	status, err := runFactoryAdmissionGit(ctx, p.Root, "status", "--porcelain")
	if err != nil {
		return FactoryRepoState{}, err
	}
	trackedAgents, err := runFactoryAdmissionGit(ctx, p.Root, "ls-files", ".agents")
	if err != nil {
		return FactoryRepoState{}, err
	}
	return FactoryRepoState{
		HeadSHA:       strings.TrimSpace(head),
		Dirty:         strings.TrimSpace(status) != "",
		TrackedAgents: splitFactoryAdmissionLines(trackedAgents),
	}, nil
}

func (p LocalFactoryAdmissionEvidenceProvider) OpenPRBlockers(context.Context, []string) (FactoryPRBlockerMatrix, error) {
	return FactoryPRBlockerMatrix{Known: true}, nil
}

func (p LocalFactoryAdmissionEvidenceProvider) MainCIBaseline(context.Context) (FactoryCIBaselineEvidence, error) {
	return FactoryCIBaselineEvidence{Known: true, Baseline: p.WorkOrder.MainCIBaseline}, nil
}

func runFactoryAdmissionGit(ctx context.Context, root string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func splitFactoryAdmissionLines(value string) []string {
	lines := strings.Split(value, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

var _ JobExecutor = (*FactoryAdmissionExecutor)(nil)
