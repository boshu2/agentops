package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/gascity"
	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
)

type RPIReconcileStatus string

const (
	RPIReconcileActive              RPIReconcileStatus = "active"
	RPIReconcileCompleted           RPIReconcileStatus = "completed"
	RPIReconcileLost                RPIReconcileStatus = "lost"
	RPIReconcileFailed              RPIReconcileStatus = "failed"
	RPIReconcileProviderUnreachable RPIReconcileStatus = "provider_unreachable"
	RPIReconcilePending             RPIReconcileStatus = "pending"
)

type RPIReconcilerOptions struct {
	Queue          *Queue
	GasCityClient  RPIReconcileGasCityClient
	RegistryWriter cliRPI.RunRegistryWriter
	Actor          string
}

type RPIReconciler struct {
	store          *Store
	queue          *Queue
	gasCityClient  RPIReconcileGasCityClient
	registryWriter cliRPI.RunRegistryWriter
	actor          string
}

type RPIReconcileGasCityClient interface {
	CityReadiness(context.Context, string) (gascity.ReadinessResponse, error)
	GetSession(context.Context, string, string, gascity.SessionGetOptions) (gascity.Session, gascity.ResponseMeta, error)
	SessionTranscript(context.Context, string, string, gascity.TranscriptOptions) (gascity.TranscriptResponse, gascity.ResponseMeta, error)
}

type RPIReconcileReport struct {
	Jobs      []RPIReconcileJob `json:"jobs"`
	Active    int               `json:"active"`
	Completed int               `json:"completed"`
	Lost      int               `json:"lost"`
	Failed    int               `json:"failed"`
}

type RPIReconcileJob struct {
	JobID          string             `json:"job_id"`
	RunID          string             `json:"run_id,omitempty"`
	Phase          int                `json:"phase,omitempty"`
	JobStatus      JobStatus          `json:"job_status"`
	Status         RPIReconcileStatus `json:"status"`
	ProviderStatus ProviderStatus     `json:"provider_status,omitempty"`
	CityName       string             `json:"city_name,omitempty"`
	SessionID      string             `json:"session_id,omitempty"`
	SessionAlias   string             `json:"session_alias,omitempty"`
	EventCursor    string             `json:"event_cursor,omitempty"`
	EvidencePath   string             `json:"evidence_path,omitempty"`
	FailureCode    FailureCode        `json:"failure_code,omitempty"`
	RepairedLedger bool               `json:"repaired_ledger,omitempty"`
	Message        string             `json:"message,omitempty"`
}

type rpiReconcileRefs struct {
	RunID        string
	Phase        int
	PhaseName    string
	Goal         string
	CityName     string
	SessionID    string
	SessionAlias string
	EventCursor  string
	Evidence     *cliRPI.GasCityPhaseEvidence
	EvidencePath string
}

func NewRPIReconciler(store *Store, opts RPIReconcilerOptions) (*RPIReconciler, error) {
	if store == nil {
		return nil, errors.New("daemon rpi reconciler: store is required")
	}
	queue := opts.Queue
	if queue == nil {
		queue = NewQueue(store, QueueOptions{})
	}
	actor := strings.TrimSpace(opts.Actor)
	if actor == "" {
		actor = "agentopsd-rpi-reconcile"
	}
	return &RPIReconciler{
		store:          store,
		queue:          queue,
		gasCityClient:  opts.GasCityClient,
		registryWriter: opts.RegistryWriter,
		actor:          actor,
	}, nil
}

func (r *RPIReconciler) ReconcileRPIJobs(ctx context.Context) (RPIReconcileReport, error) {
	snapshot, err := r.queue.Snapshot()
	if err != nil {
		return RPIReconcileReport{}, err
	}
	report := RPIReconcileReport{}
	for _, job := range snapshot.Jobs {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		if !isRPIJobType(job.JobType) {
			continue
		}
		reconciled, err := r.reconcileJob(ctx, job)
		if err != nil {
			return report, err
		}
		report.addJob(reconciled)
	}
	if err := r.writeRPIRegistryProjection(); err != nil {
		return report, err
	}
	return report, nil
}

func (r *RPIReconciler) writeRPIRegistryProjection() error {
	projection, err := r.store.RebuildRPIRegistryProjection()
	if err != nil {
		return err
	}
	return WriteRPIRegistryProjection(r.store.root, projection, r.registryWriter)
}

func (r *RPIReconciler) reconcileJob(ctx context.Context, job QueueJobState) (RPIReconcileJob, error) {
	refs, err := r.reconcileRefs(job)
	if err != nil {
		return RPIReconcileJob{}, err
	}
	out := RPIReconcileJob{
		JobID:          job.JobID,
		RunID:          refs.RunID,
		Phase:          refs.Phase,
		JobStatus:      job.Status,
		CityName:       refs.CityName,
		SessionID:      refs.SessionID,
		SessionAlias:   refs.SessionAlias,
		EventCursor:    refs.EventCursor,
		EvidencePath:   refs.EvidencePath,
		ProviderStatus: ProjectProviderStatus(ProviderProjectionInput{DaemonReady: true, GasCityReady: r.gasCityClient != nil, WorkerSessionKnown: refs.SessionID != "" || refs.SessionAlias != ""}),
	}
	if isTerminalStatus(job.Status) {
		out.Status = reconcileStatusFromTerminalJob(job)
		if job.Failure != nil {
			out.FailureCode = job.Failure.Code
			out.Message = job.Failure.Message
		}
		return out, nil
	}
	if terminal, ok, err := r.reconcileFromEvidence(job, refs, &out); ok || err != nil {
		return terminal, err
	}
	if r.gasCityClient == nil {
		out.Status = RPIReconcilePending
		out.Message = "no GasCity client configured for restart reconciliation"
		return out, nil
	}
	if refs.CityName == "" {
		out.Status = RPIReconcilePending
		out.Message = "missing GasCity city name"
		return out, nil
	}
	ready, err := r.gasCityClient.CityReadiness(ctx, refs.CityName)
	if err != nil || !ready.Ready {
		out.Status = RPIReconcileProviderUnreachable
		out.ProviderStatus = ProviderUnreachable
		if err != nil {
			out.Message = err.Error()
		} else {
			out.Message = firstNonEmpty(ready.Status, "not ready")
		}
		return out, nil
	}
	out.ProviderStatus = ProjectProviderStatus(ProviderProjectionInput{DaemonReady: true, GasCityReady: true, WorkerSessionKnown: refs.SessionID != "" || refs.SessionAlias != ""})
	if refs.SessionID == "" {
		out.Status = RPIReconcilePending
		out.Message = "missing GasCity session ID"
		return out, nil
	}
	return r.reconcileFromGasCity(ctx, job, refs, out)
}

func (r *RPIReconciler) reconcileFromEvidence(job QueueJobState, refs rpiReconcileRefs, out *RPIReconcileJob) (RPIReconcileJob, bool, error) {
	if refs.Evidence == nil {
		return RPIReconcileJob{}, false, nil
	}
	status := strings.ToLower(strings.TrimSpace(refs.Evidence.Status))
	switch status {
	case gascity.TerminalStatusCompleted:
		repaired, err := r.completeJobFromReconcile(job, refs, map[string]string{
			phaseArtifactKey(refs.Phase, "gascity_evidence"): refs.EvidencePath,
		})
		out.Status = RPIReconcileCompleted
		out.RepairedLedger = repaired
		return *out, true, err
	case gascity.TerminalStatusLost:
		repaired, err := r.failJobFromReconcile(job, refs, JobFailure{
			Code:      FailureSessionLost,
			Message:   "GasCity evidence records lost session",
			Retryable: false,
		})
		out.Status = RPIReconcileLost
		out.FailureCode = FailureSessionLost
		out.RepairedLedger = repaired
		return *out, true, err
	case gascity.TerminalStatusFailed, gascity.TerminalStatusCancelled, gascity.TerminalStatusTerminalWithoutTranscript:
		code := failureCodeForTerminalStatus(status)
		repaired, err := r.failJobFromReconcile(job, refs, JobFailure{
			Code:      code,
			Message:   "GasCity evidence records terminal " + status,
			Retryable: false,
		})
		out.Status = RPIReconcileFailed
		out.FailureCode = code
		out.RepairedLedger = repaired
		return *out, true, err
	default:
		return RPIReconcileJob{}, false, nil
	}
}

func (r *RPIReconciler) reconcileFromGasCity(ctx context.Context, job QueueJobState, refs rpiReconcileRefs, out RPIReconcileJob) (RPIReconcileJob, error) {
	session, getMeta, err := r.gasCityClient.GetSession(ctx, refs.CityName, refs.SessionID, gascity.SessionGetOptions{Peek: true})
	if err != nil {
		if gasCityStatusCode(err) == http.StatusNotFound {
			repaired, failErr := r.failJobFromReconcile(job, refs, JobFailure{
				Code:      FailureSessionLost,
				Message:   fmt.Sprintf("GasCity session %s missing after daemon restart", refs.SessionID),
				Retryable: false,
			})
			out.Status = RPIReconcileLost
			out.FailureCode = FailureSessionLost
			out.RepairedLedger = repaired
			return out, failErr
		}
		out.Status = RPIReconcileProviderUnreachable
		out.ProviderStatus = ProviderUnreachable
		out.Message = err.Error()
		return out, nil
	}
	classification := gascity.ClassifyTerminalState(gascity.TerminalStateInput{
		SessionState:  session.State,
		SessionStatus: session.Status,
	})
	if !classification.Terminal {
		out.Status = RPIReconcileActive
		return out, nil
	}
	if classification.Status != gascity.TerminalStatusCompleted || classification.Degraded {
		code := failureCodeForTerminalStatus(classification.Status)
		message := fmt.Sprintf("GasCity session %s terminal %s after daemon restart", refs.SessionID, classification.Status)
		if classification.Reason != "" {
			message += ": " + classification.Reason
		}
		repaired, err := r.failJobFromReconcile(job, refs, JobFailure{
			Code:      code,
			Message:   message,
			Retryable: false,
		})
		out.Status = RPIReconcileFailed
		out.FailureCode = code
		out.RepairedLedger = repaired
		return out, err
	}
	transcript, transcriptMeta, err := r.gasCityClient.SessionTranscript(ctx, refs.CityName, refs.SessionID, gascity.TranscriptOptions{Format: "conversation"})
	if err != nil {
		repaired, failErr := r.failJobFromReconcile(job, refs, JobFailure{
			Code:      FailureTerminalWithoutTranscript,
			Message:   fmt.Sprintf("GasCity session %s terminal without transcript: %v", refs.SessionID, err),
			Retryable: false,
		})
		out.Status = RPIReconcileFailed
		out.FailureCode = FailureTerminalWithoutTranscript
		out.RepairedLedger = repaired
		return out, failErr
	}
	evidencePath, err := writeReconciledEvidence(r.store.root, refs, transcript, map[string]string{
		"get_session": getMeta.RequestID,
		"transcript":  transcriptMeta.RequestID,
	})
	if err != nil {
		return out, err
	}
	refs.EvidencePath = evidencePath
	out.EvidencePath = evidencePath
	repaired, err := r.completeJobFromReconcile(job, refs, map[string]string{
		phaseArtifactKey(refs.Phase, "gascity_evidence"): evidencePath,
	})
	out.Status = RPIReconcileCompleted
	out.RepairedLedger = repaired
	return out, err
}

func (r *RPIReconciler) completeJobFromReconcile(job QueueJobState, refs rpiReconcileRefs, artifacts map[string]string) (bool, error) {
	if isTerminalStatus(job.Status) {
		return false, nil
	}
	mergeArtifacts(artifacts, refs.artifacts())
	claim, err := r.queue.ClaimJob(job.JobID, r.actor, QueueMutationOptions{})
	if err != nil {
		if errors.Is(err, ErrJobAlreadyClaimed) || errors.Is(err, ErrNoClaimableJobs) {
			return false, nil
		}
		return false, err
	}
	_, err = r.queue.CompleteJob(CompleteJobInput{
		JobID:      claim.Job.JobID,
		RequestID:  RequestID(claim.Job.RequestID),
		ClaimToken: claim.ClaimToken,
		LeaseEpoch: claim.LeaseEpoch,
		Actor:      r.actor,
		Artifacts:  artifacts,
	}, QueueMutationOptions{})
	return err == nil, err
}

func (r *RPIReconciler) failJobFromReconcile(job QueueJobState, refs rpiReconcileRefs, failure JobFailure) (bool, error) {
	if isTerminalStatus(job.Status) {
		return false, nil
	}
	claim, err := r.queue.ClaimJob(job.JobID, r.actor, QueueMutationOptions{})
	if err != nil {
		if errors.Is(err, ErrJobAlreadyClaimed) || errors.Is(err, ErrNoClaimableJobs) {
			return false, nil
		}
		return false, err
	}
	_, err = r.queue.FailJob(FailJobInput{
		JobID:      claim.Job.JobID,
		RequestID:  RequestID(claim.Job.RequestID),
		ClaimToken: claim.ClaimToken,
		LeaseEpoch: claim.LeaseEpoch,
		Actor:      r.actor,
		Failure:    failure,
	}, QueueMutationOptions{})
	return err == nil, err
}

func (r *RPIReconciler) reconcileRefs(job QueueJobState) (rpiReconcileRefs, error) {
	refs := rpiReconcileRefs{}
	switch job.JobType {
	case JobTypeRPIRun:
		spec, err := RPIRunJobSpecFromPayload(job.Payload)
		if err != nil {
			return refs, err
		}
		refs.RunID = spec.RunID
		refs.Phase = firstPositive(artifactInt(job.Artifacts, "active_phase"), spec.StartPhase)
		refs.PhaseName = RPIPhaseName(refs.Phase)
		refs.Goal = spec.Goal
		refs.CityName = spec.GasCityCityName
	case JobTypeRPIPhase:
		spec, err := RPIPhaseJobSpecFromPayload(job.Payload)
		if err != nil {
			return refs, err
		}
		refs.RunID = spec.RunID
		refs.Phase = firstPositive(artifactInt(job.Artifacts, "active_phase"), spec.Phase)
		refs.PhaseName = spec.PhaseName
		refs.Goal = spec.Goal
		refs.CityName = spec.GasCityCityName
		refs.SessionAlias = spec.GasCitySessionAlias
	default:
		return refs, fmt.Errorf("%w: job %s type %s", ErrNoRPIJobs, job.JobID, job.JobType)
	}
	refs.fillFromArtifacts(job.Artifacts)
	refs.fillFromEvidence(r.store.root)
	return refs, nil
}

func (refs *rpiReconcileRefs) fillFromArtifacts(artifacts map[string]string) {
	if refs.Phase <= 0 {
		return
	}
	prefix := fmt.Sprintf("phase_%d_gascity_", refs.Phase)
	refs.CityName = firstNonEmpty(artifacts[prefix+"city_name"], refs.CityName)
	refs.SessionID = firstNonEmpty(artifacts[prefix+"session_id"], refs.SessionID)
	refs.SessionAlias = firstNonEmpty(artifacts[prefix+"session_alias"], refs.SessionAlias)
	refs.EventCursor = firstNonEmpty(artifacts[prefix+"event_cursor"], refs.EventCursor)
	refs.EvidencePath = firstNonEmpty(artifacts[prefix+"evidence"], refs.EvidencePath)
}

func (refs *rpiReconcileRefs) fillFromEvidence(root string) {
	if refs.RunID == "" || refs.Phase <= 0 {
		return
	}
	evidencePath := refs.EvidencePath
	if evidencePath == "" {
		evidencePath = cliRPI.GasCityPhaseEvidencePath(root, refs.RunID, refs.Phase)
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		return
	}
	var evidence cliRPI.GasCityPhaseEvidence
	if err := json.Unmarshal(data, &evidence); err != nil {
		return
	}
	refs.Evidence = &evidence
	refs.EvidencePath = evidencePath
	refs.CityName = firstNonEmpty(evidence.CityName, refs.CityName)
	refs.SessionID = firstNonEmpty(evidence.SessionID, refs.SessionID)
	refs.SessionAlias = firstNonEmpty(evidence.SessionAlias, refs.SessionAlias)
	refs.EventCursor = firstNonEmpty(evidence.EventCursor, refs.EventCursor)
	if refs.PhaseName == "" {
		refs.PhaseName = evidence.PhaseName
	}
}

func (refs rpiReconcileRefs) artifacts() map[string]string {
	artifacts := map[string]string{}
	if refs.Phase <= 0 {
		return artifacts
	}
	if refs.CityName != "" {
		artifacts[phaseArtifactKey(refs.Phase, "gascity_city_name")] = refs.CityName
	}
	if refs.SessionID != "" {
		artifacts[phaseArtifactKey(refs.Phase, "gascity_session_id")] = refs.SessionID
	}
	if refs.SessionAlias != "" {
		artifacts[phaseArtifactKey(refs.Phase, "gascity_session_alias")] = refs.SessionAlias
	}
	if refs.EventCursor != "" {
		artifacts[phaseArtifactKey(refs.Phase, "gascity_event_cursor")] = refs.EventCursor
	}
	if refs.EvidencePath != "" {
		artifacts[phaseArtifactKey(refs.Phase, "gascity_evidence")] = refs.EvidencePath
	}
	return artifacts
}

func writeReconciledEvidence(root string, refs rpiReconcileRefs, transcript gascity.TranscriptResponse, requestIDs map[string]string) (string, error) {
	artifacts := make([]cliRPI.GasCityTranscriptArtifact, 0, len(transcript.Artifacts))
	for _, artifact := range transcript.Artifacts {
		artifacts = append(artifacts, cliRPI.GasCityTranscriptArtifact{
			Path: artifact.Path,
			Kind: artifact.Kind,
		})
	}
	return cliRPI.WriteGasCityPhaseEvidence(root, cliRPI.GasCityPhaseEvidence{
		RunID:                refs.RunID,
		Phase:                refs.Phase,
		PhaseName:            refs.PhaseName,
		CityName:             refs.CityName,
		SessionID:            refs.SessionID,
		SessionAlias:         refs.SessionAlias,
		Status:               gascity.TerminalStatusCompleted,
		EventCursor:          refs.EventCursor,
		RequestIDs:           requestIDs,
		TranscriptID:         firstNonEmpty(transcript.ID, transcript.SessionID, refs.SessionID),
		TranscriptFormat:     transcript.Format,
		TranscriptTurnCount:  len(transcript.Turns),
		TranscriptMsgCount:   len(transcript.Messages),
		TranscriptArtifacts:  artifacts,
		TranscriptCapturedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

func reconcileStatusFromTerminalJob(job QueueJobState) RPIReconcileStatus {
	switch job.Status {
	case JobStatusCompleted:
		return RPIReconcileCompleted
	case JobStatusFailed:
		if job.Failure != nil && job.Failure.Code == FailureSessionLost {
			return RPIReconcileLost
		}
		return RPIReconcileFailed
	default:
		return RPIReconcileFailed
	}
}

func (report *RPIReconcileReport) addJob(job RPIReconcileJob) {
	report.Jobs = append(report.Jobs, job)
	switch job.Status {
	case RPIReconcileActive, RPIReconcilePending, RPIReconcileProviderUnreachable:
		report.Active++
	case RPIReconcileCompleted:
		report.Completed++
	case RPIReconcileLost:
		report.Lost++
	case RPIReconcileFailed:
		report.Failed++
	}
}

func gasCityStatusCode(err error) int {
	var apiErr *gascity.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode
	}
	return 0
}

func artifactInt(artifacts map[string]string, key string) int {
	if artifacts == nil {
		return 0
	}
	value, err := strconv.Atoi(strings.TrimSpace(artifacts[key]))
	if err != nil {
		return 0
	}
	return value
}
