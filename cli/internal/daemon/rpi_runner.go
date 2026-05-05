package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/gascity"
	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
)

var (
	ErrRPIExecutorRequired = errors.New("daemon rpi runner: executor is required")
	ErrNoRPIJobs           = errors.New("daemon rpi runner: no claimable RPI jobs")
)

type RPIRunnerOptions struct {
	Queue             *Queue
	Executor          RPIPhaseExecutor
	RegistryWriter    cliRPI.RunRegistryWriter
	Actor             string
	HeartbeatInterval time.Duration
	PromptBuilder     RPIPromptBuilder
}

type RPIRunner struct {
	store             *Store
	queue             *Queue
	executor          RPIPhaseExecutor
	registryWriter    cliRPI.RunRegistryWriter
	actor             string
	heartbeatInterval time.Duration
	promptBuilder     RPIPromptBuilder
}

type RPIJobRunResult struct {
	JobID     string            `json:"job_id"`
	RunID     string            `json:"run_id,omitempty"`
	JobType   JobType           `json:"job_type"`
	Status    JobStatus         `json:"status"`
	Artifacts map[string]string `json:"artifacts,omitempty"`
	Failure   *JobFailure       `json:"failure,omitempty"`
}

type RPIPromptBuilder interface {
	BuildRPIPrompt(RPIPhaseExecutionRequest) (string, error)
}

type RPIPromptBuilderFunc func(RPIPhaseExecutionRequest) (string, error)

func (f RPIPromptBuilderFunc) BuildRPIPrompt(req RPIPhaseExecutionRequest) (string, error) {
	return f(req)
}

type RPIPhaseExecutor interface {
	ExecuteRPIPhase(context.Context, RPIPhaseExecutionRequest) (RPIPhaseExecutionResult, error)
}

type RPIPhaseExecutionRequest struct {
	Root                string
	JobID               string
	RunID               string
	Goal                string
	EpicID              string
	ExecutionPacketPath string
	ParentRunJobID      string
	StartPhase          int
	MaxPhase            int
	Phase               int
	PhaseName           string
	Attempt             int
	Backend             RPIBackend
	GasCityCityName     string
	GasCitySessionAlias string
	PhaseTimeout        time.Duration
	Prompt              string
	Progress            RPIPhaseProgressFunc
}

type RPIPhaseExecutionResult struct {
	Status              string            `json:"status,omitempty"`
	Artifacts           map[string]string `json:"artifacts,omitempty"`
	RequestIDs          map[string]string `json:"request_ids,omitempty"`
	GasCityCityName     string            `json:"gascity_city_name,omitempty"`
	GasCitySessionID    string            `json:"gascity_session_id,omitempty"`
	GasCitySessionAlias string            `json:"gascity_session_alias,omitempty"`
	EventCursor         string            `json:"event_cursor,omitempty"`
	EvidencePath        string            `json:"evidence_path,omitempty"`
}

type RPIPhaseProgress struct {
	Phase     int
	Status    string
	Artifacts map[string]string
}

type RPIPhaseProgressFunc func(context.Context, RPIPhaseProgress) error

type RPIPhaseExecutionError struct {
	Code      FailureCode
	Message   string
	Retryable bool
	Cause     error
}

func (e *RPIPhaseExecutionError) Error() string {
	if strings.TrimSpace(e.Message) != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return string(e.Code)
}

func (e *RPIPhaseExecutionError) Unwrap() error {
	return e.Cause
}

func NewRPIRunner(store *Store, opts RPIRunnerOptions) (*RPIRunner, error) {
	if store == nil {
		return nil, errors.New("daemon rpi runner: store is required")
	}
	if opts.Executor == nil {
		return nil, ErrRPIExecutorRequired
	}
	queue := opts.Queue
	if queue == nil {
		queue = NewQueue(store, QueueOptions{})
	}
	actor := strings.TrimSpace(opts.Actor)
	if actor == "" {
		actor = "agentopsd-rpi"
	}
	promptBuilder := opts.PromptBuilder
	if promptBuilder == nil {
		promptBuilder = DefaultRPIPromptBuilder{}
	}
	return &RPIRunner{
		store:             store,
		queue:             queue,
		executor:          opts.Executor,
		registryWriter:    opts.RegistryWriter,
		actor:             actor,
		heartbeatInterval: opts.HeartbeatInterval,
		promptBuilder:     promptBuilder,
	}, nil
}

func (r *RPIRunner) RunNextRPIJob(ctx context.Context) (RPIJobRunResult, error) {
	claim, err := r.claimNextRPIJob()
	if err != nil {
		return RPIJobRunResult{}, err
	}
	return r.runClaimedJob(ctx, claim)
}

func (r *RPIRunner) RunRPIJob(ctx context.Context, jobID string) (RPIJobRunResult, error) {
	snapshot, err := r.queue.Snapshot()
	if err != nil {
		return RPIJobRunResult{}, err
	}
	job, err := snapshot.jobByID(jobID)
	if err != nil {
		return RPIJobRunResult{}, err
	}
	if !isRPIJobType(job.JobType) {
		return RPIJobRunResult{}, fmt.Errorf("%w: job %s type %s", ErrNoRPIJobs, job.JobID, job.JobType)
	}
	claim, err := r.queue.ClaimJob(jobID, r.actor, QueueMutationOptions{})
	if err != nil {
		return RPIJobRunResult{}, err
	}
	return r.runClaimedJob(ctx, claim)
}

func (r *RPIRunner) claimNextRPIJob() (QueueClaim, error) {
	snapshot, err := r.queue.Snapshot()
	if err != nil {
		return QueueClaim{}, err
	}
	for _, job := range snapshot.Jobs {
		if !isRPIJobType(job.JobType) {
			continue
		}
		claim, err := r.queue.ClaimJob(job.JobID, r.actor, QueueMutationOptions{})
		if errors.Is(err, ErrNoClaimableJobs) || errors.Is(err, ErrJobAlreadyClaimed) {
			continue
		}
		if err != nil {
			return QueueClaim{}, err
		}
		return claim, nil
	}
	return QueueClaim{}, ErrNoRPIJobs
}

func (r *RPIRunner) runClaimedJob(ctx context.Context, claim QueueClaim) (RPIJobRunResult, error) {
	_ = r.writeRPIRegistryProjection()
	stopHeartbeat := r.startHeartbeat(ctx, claim)
	defer stopHeartbeat()

	artifacts, runID, execErr := r.ExecuteClaim(ctx, claim)
	if execErr != nil {
		failure := r.failureForError(execErr)
		job, failErr := r.queue.FailJob(FailJobInput{
			JobID:      claim.Job.JobID,
			RequestID:  RequestID(claim.Job.RequestID),
			ClaimToken: claim.ClaimToken,
			LeaseEpoch: claim.LeaseEpoch,
			Actor:      r.actor,
			Failure:    failure,
		}, QueueMutationOptions{})
		_ = r.writeRPIRegistryProjection()
		if failErr != nil {
			return RPIJobRunResult{}, failErr
		}
		return RPIJobRunResult{
			JobID:   job.JobID,
			RunID:   runID,
			JobType: job.JobType,
			Status:  job.Status,
			Failure: job.Failure,
		}, execErr
	}

	job, err := r.queue.CompleteJob(CompleteJobInput{
		JobID:      claim.Job.JobID,
		RequestID:  RequestID(claim.Job.RequestID),
		ClaimToken: claim.ClaimToken,
		LeaseEpoch: claim.LeaseEpoch,
		Actor:      r.actor,
		Artifacts:  artifacts,
	}, QueueMutationOptions{})
	_ = r.writeRPIRegistryProjection()
	if err != nil {
		return RPIJobRunResult{}, err
	}
	return RPIJobRunResult{
		JobID:     job.JobID,
		RunID:     runID,
		JobType:   job.JobType,
		Status:    job.Status,
		Artifacts: job.Artifacts,
	}, nil
}

// ExecuteClaim runs the user-visible RPI work for a job that has already
// been claimed by some caller (the supervisor's claim path is one such
// caller; the operator-driven `ao rpi run` is another). It does not handle
// claim, heartbeat, or terminal write — those are the caller's responsibility.
func (r *RPIRunner) ExecuteClaim(ctx context.Context, claim QueueClaim) (map[string]string, string, error) {
	switch claim.Job.JobType {
	case JobTypeRPIRun:
		spec, err := RPIRunJobSpecFromPayload(claim.Job.Payload)
		if err != nil {
			return nil, "", err
		}
		return r.executeRun(ctx, claim, spec)
	case JobTypeRPIPhase:
		spec, err := RPIPhaseJobSpecFromPayload(claim.Job.Payload)
		if err != nil {
			return nil, "", err
		}
		result, err := r.executePhase(ctx, claim, requestFromPhaseSpec(r.root(), claim, spec))
		return result.Artifacts, spec.RunID, err
	default:
		return nil, "", fmt.Errorf("%w: job %s type %s", ErrNoRPIJobs, claim.Job.JobID, claim.Job.JobType)
	}
}

func (r *RPIRunner) executeRun(ctx context.Context, claim QueueClaim, spec RPIRunJobSpec) (map[string]string, string, error) {
	artifacts := map[string]string{}
	for phase := spec.StartPhase; phase <= spec.MaxPhase; phase++ {
		phaseSpec := NewRPIPhaseJobSpec(spec.RunID, spec.Goal, phase)
		phaseSpec.EpicID = spec.EpicID
		phaseSpec.ParentRunJobID = claim.Job.JobID
		phaseSpec.ExecutionPacketPath = spec.ExecutionPacketPath
		phaseSpec.Attempt = claim.Job.Attempt
		phaseSpec.Backend = spec.Backend
		phaseSpec.GasCityCityName = spec.GasCityCityName
		phaseSpec.PhaseTimeout = spec.PhaseTimeout
		req := requestFromPhaseSpec(r.root(), claim, phaseSpec)
		req.StartPhase = spec.StartPhase
		req.MaxPhase = spec.MaxPhase
		result, err := r.executePhase(ctx, claim, req)
		mergeArtifacts(artifacts, result.Artifacts)
		if err != nil {
			return artifacts, spec.RunID, err
		}
	}
	return artifacts, spec.RunID, nil
}

func (r *RPIRunner) executePhase(ctx context.Context, claim QueueClaim, req RPIPhaseExecutionRequest) (RPIPhaseExecutionResult, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		prompt, err := r.promptBuilder.BuildRPIPrompt(req)
		if err != nil {
			return RPIPhaseExecutionResult{}, err
		}
		req.Prompt = prompt
	}
	req.Progress = r.recordPhaseProgress(claim)
	result, err := r.executor.ExecuteRPIPhase(ctx, req)
	if result.Artifacts == nil {
		result.Artifacts = map[string]string{}
	}
	if result.EvidencePath != "" {
		result.Artifacts[phaseArtifactKey(req.Phase, "gascity_evidence")] = result.EvidencePath
	}
	if result.GasCitySessionID != "" {
		result.Artifacts[phaseArtifactKey(req.Phase, "gascity_session_id")] = result.GasCitySessionID
	}
	if result.EventCursor != "" {
		result.Artifacts[phaseArtifactKey(req.Phase, "gascity_event_cursor")] = result.EventCursor
	}
	return result, err
}

func (r *RPIRunner) recordPhaseProgress(claim QueueClaim) RPIPhaseProgressFunc {
	return func(ctx context.Context, progress RPIPhaseProgress) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		artifacts := map[string]string{}
		if progress.Phase > 0 {
			artifacts["active_phase"] = fmt.Sprintf("%d", progress.Phase)
		}
		if strings.TrimSpace(progress.Status) != "" && progress.Phase > 0 {
			artifacts[phaseArtifactKey(progress.Phase, "gascity_status")] = progress.Status
		}
		mergeArtifacts(artifacts, progress.Artifacts)
		_, err := r.queue.Heartbeat(HeartbeatInput{
			JobID:      claim.Job.JobID,
			RequestID:  RequestID(claim.Job.RequestID),
			ClaimToken: claim.ClaimToken,
			LeaseEpoch: claim.LeaseEpoch,
			Actor:      r.actor,
			Artifacts:  artifacts,
		}, QueueMutationOptions{})
		return err
	}
}

func (r *RPIRunner) startHeartbeat(ctx context.Context, claim QueueClaim) func() {
	interval := r.heartbeatInterval
	if interval <= 0 {
		interval = r.queue.opts.LeaseDuration / 2
	}
	if interval <= 0 {
		return func() {}
	}
	heartbeatCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
				_, err := r.queue.Heartbeat(HeartbeatInput{
					JobID:      claim.Job.JobID,
					RequestID:  RequestID(claim.Job.RequestID),
					ClaimToken: claim.ClaimToken,
					LeaseEpoch: claim.LeaseEpoch,
					Actor:      r.actor,
				}, QueueMutationOptions{})
				if err != nil {
					return
				}
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func (r *RPIRunner) writeRPIRegistryProjection() error {
	projection, err := r.store.RebuildRPIRegistryProjection()
	if err != nil {
		return err
	}
	return WriteRPIRegistryProjection(r.root(), projection, r.registryWriter)
}

func (r *RPIRunner) failureForError(err error) JobFailure {
	failure := JobFailure{
		Code:      FailureRequestRejected,
		Message:   err.Error(),
		Retryable: false,
	}
	var execErr *RPIPhaseExecutionError
	if errors.As(err, &execErr) {
		if execErr.Code != "" {
			failure.Code = execErr.Code
		}
		failure.Retryable = execErr.Retryable
		if strings.TrimSpace(execErr.Message) != "" {
			failure.Message = execErr.Message
		}
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		failure.Code = FailureDaemonUnavailable
		failure.Retryable = true
	}
	if providerFailureLooksRetryable(failure.Code) {
		failure.Retryable = true
	}
	return failure
}

func (r *RPIRunner) root() string {
	return r.store.root
}

type DefaultRPIPromptBuilder struct{}

func (DefaultRPIPromptBuilder) BuildRPIPrompt(req RPIPhaseExecutionRequest) (string, error) {
	if strings.TrimSpace(req.Goal) == "" {
		return "", errors.New("rpi phase prompt: goal is required")
	}
	phaseName := strings.TrimSpace(req.PhaseName)
	if phaseName == "" {
		phaseName = RPIPhaseName(req.Phase)
	}
	return fmt.Sprintf(
		"Run AgentOps RPI phase %d (%s) for run %s.\n\nGoal:\n%s\n\nProduce the expected AgentOps RPI artifacts under .agents/rpi for this phase, preserve durable evidence, and finish with a concise phase summary.",
		req.Phase,
		phaseName,
		req.RunID,
		req.Goal,
	), nil
}

type GasCityRPIClient interface {
	CityReadiness(context.Context, string) (gascity.ReadinessResponse, error)
	CreateSession(context.Context, string, gascity.SessionCreateRequest) (gascity.Session, gascity.ResponseMeta, error)
	GetSession(context.Context, string, string, gascity.SessionGetOptions) (gascity.Session, gascity.ResponseMeta, error)
	SubmitSession(context.Context, string, string, gascity.SessionSubmitRequest) (gascity.SessionSubmitResponse, gascity.ResponseMeta, error)
	StreamCityEvents(context.Context, string, gascity.EventStreamOptions) (GasCityRPIEventStream, gascity.ResponseMeta, error)
	SessionTranscript(context.Context, string, string, gascity.TranscriptOptions) (gascity.TranscriptResponse, gascity.ResponseMeta, error)
}

type GasCityRPIEventStream interface {
	NextEvent() (gascity.EventStreamFrame, error)
	Close() error
}

type GasCityClientAdapter struct {
	Client *gascity.Client
}

func (a GasCityClientAdapter) CityReadiness(ctx context.Context, cityName string) (gascity.ReadinessResponse, error) {
	return a.Client.CityReadiness(ctx, cityName)
}

func (a GasCityClientAdapter) CreateSession(ctx context.Context, cityName string, req gascity.SessionCreateRequest) (gascity.Session, gascity.ResponseMeta, error) {
	return a.Client.CreateSession(ctx, cityName, req)
}

func (a GasCityClientAdapter) SubmitSession(ctx context.Context, cityName string, id string, req gascity.SessionSubmitRequest) (gascity.SessionSubmitResponse, gascity.ResponseMeta, error) {
	return a.Client.SubmitSession(ctx, cityName, id, req)
}

func (a GasCityClientAdapter) GetSession(ctx context.Context, cityName string, id string, opts gascity.SessionGetOptions) (gascity.Session, gascity.ResponseMeta, error) {
	return a.Client.GetSession(ctx, cityName, id, opts)
}

func (a GasCityClientAdapter) StreamCityEvents(ctx context.Context, cityName string, opts gascity.EventStreamOptions) (GasCityRPIEventStream, gascity.ResponseMeta, error) {
	return a.Client.StreamCityEvents(ctx, cityName, opts)
}

func (a GasCityClientAdapter) SessionTranscript(ctx context.Context, cityName string, id string, opts gascity.TranscriptOptions) (gascity.TranscriptResponse, gascity.ResponseMeta, error) {
	return a.Client.SessionTranscript(ctx, cityName, id, opts)
}

type GasCityRPIPhaseExecutor struct {
	Client           GasCityRPIClient
	CityName         string
	SessionAgentName string
	PhaseTimeout     time.Duration
}

func (e GasCityRPIPhaseExecutor) ExecuteRPIPhase(ctx context.Context, req RPIPhaseExecutionRequest) (RPIPhaseExecutionResult, error) {
	cityName, err := resolveAndCheckCityReadiness(ctx, e, req)
	if err != nil {
		return RPIPhaseExecutionResult{}, err
	}
	sessionID, sessionAlias, createMeta, err := createGasCitySession(ctx, e, cityName, req)
	if err != nil {
		return RPIPhaseExecutionResult{}, err
	}
	submitMeta, err := submitGasCitySession(ctx, e, cityName, sessionID, req.Prompt)
	if err != nil {
		return RPIPhaseExecutionResult{}, err
	}
	result, err := buildInitialResult(ctx, req, cityName, sessionID, sessionAlias, createMeta, submitMeta)
	if err != nil {
		return result, err
	}

	timeout := e.PhaseTimeout
	if req.PhaseTimeout > 0 {
		timeout = req.PhaseTimeout
	}
	streamCtx := ctx
	cancel := func() {}
	if timeout > 0 {
		streamCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()
	stream, streamMeta, err := e.Client.StreamCityEvents(streamCtx, cityName, gascity.EventStreamOptions{})
	if err != nil {
		return result, &RPIPhaseExecutionError{
			Code:      FailureEventStreamUnavailable,
			Message:   fmt.Sprintf("gascity event stream for %q: %v", sessionID, err),
			Retryable: true,
			Cause:     err,
		}
	}
	addRequestID(result.RequestIDs, "stream", streamMeta.RequestID)
	defer stream.Close()

	result, err = processGasCityEventStreamFrames(ctx, streamCtx, stream, sessionID, sessionAlias, req, e, result)
	if err == nil {
		return result, nil
	}
	reconcileCtx := ctx
	reconcileCancel := func() {}
	if streamCtx.Err() != nil {
		reconcileCtx, reconcileCancel = context.WithTimeout(context.Background(), 10*time.Second)
	}
	defer reconcileCancel()
	done, reconcileErr := reconcileGasCitySessionTerminal(reconcileCtx, req, e, &result, true)
	if reconcileErr == nil && done {
		return result, nil
	}
	if streamCtx.Err() != nil {
		interruptGasCitySessionAfterAbort(result, e, streamCtx.Err())
		return result, err
	}
	if reconcileErr != nil {
		return result, reconcileErr
	}
	return result, err
}

// resolveAndCheckCityReadiness validates the executor client, resolves the
// effective city name, and confirms the city is ready to accept RPI work.
func resolveAndCheckCityReadiness(ctx context.Context, e GasCityRPIPhaseExecutor, req RPIPhaseExecutionRequest) (string, error) {
	if e.Client == nil {
		return "", &RPIPhaseExecutionError{Code: FailureProviderUnreachable, Message: "gascity client is required", Retryable: true}
	}
	cityName := strings.TrimSpace(req.GasCityCityName)
	if cityName == "" {
		cityName = strings.TrimSpace(e.CityName)
	}
	if cityName == "" {
		return "", &RPIPhaseExecutionError{Code: FailureRequestRejected, Message: "gascity city name is required"}
	}
	ready, err := e.Client.CityReadiness(ctx, cityName)
	if err != nil {
		return "", &RPIPhaseExecutionError{
			Code:      FailureProviderUnreachable,
			Message:   fmt.Sprintf("gascity city readiness for %q: %v", cityName, err),
			Retryable: true,
			Cause:     err,
		}
	}
	if !ready.IsReady() {
		status := strings.TrimSpace(ready.EffectiveStatus())
		if status == "" {
			status = "not ready"
		}
		return "", &RPIPhaseExecutionError{
			Code:      FailureProviderUnreachable,
			Message:   fmt.Sprintf("gascity city %q not ready: %s", cityName, status),
			Retryable: true,
		}
	}
	return cityName, nil
}

// createGasCitySession resolves the session alias and creates the GasCity
// session for the requested phase. Returns the session ID, alias, and metadata.
func createGasCitySession(ctx context.Context, e GasCityRPIPhaseExecutor, cityName string, req RPIPhaseExecutionRequest) (string, string, gascity.ResponseMeta, error) {
	sessionAlias := strings.TrimSpace(req.GasCitySessionAlias)
	if sessionAlias == "" {
		sessionAlias = fmt.Sprintf("rpi-%s-p%d", req.RunID, req.Phase)
	}
	sessionAgentName := strings.TrimSpace(e.SessionAgentName)
	if sessionAgentName == "" {
		sessionAgentName = "worker"
	}
	session, createMeta, err := e.Client.CreateSession(ctx, cityName, gascity.SessionCreateRequest{
		Kind:  "agent",
		Name:  sessionAgentName,
		Alias: sessionAlias,
		Async: true,
		Title: fmt.Sprintf("RPI %s phase %d", req.RunID, req.Phase),
	})
	if err != nil {
		return "", "", gascity.ResponseMeta{}, &RPIPhaseExecutionError{
			Code:      FailureProviderUnreachable,
			Message:   fmt.Sprintf("gascity create session %q: %v", sessionAlias, err),
			Retryable: true,
			Cause:     err,
		}
	}
	sessionID := strings.TrimSpace(session.ID)
	if sessionID == "" {
		return "", "", gascity.ResponseMeta{}, &RPIPhaseExecutionError{
			Code:    FailureSessionPending,
			Message: fmt.Sprintf("gascity create session %q returned empty session ID", sessionAlias),
		}
	}
	return sessionID, sessionAlias, createMeta, nil
}

// submitGasCitySession submits the prompt to the previously-created session
// and returns the submit metadata.
func submitGasCitySession(ctx context.Context, e GasCityRPIPhaseExecutor, cityName, sessionID, prompt string) (gascity.ResponseMeta, error) {
	_, submitMeta, err := e.Client.SubmitSession(ctx, cityName, sessionID, gascity.SessionSubmitRequest{
		Message: prompt,
		Intent:  "follow_up",
	})
	if err != nil {
		return gascity.ResponseMeta{}, &RPIPhaseExecutionError{
			Code:      FailureProviderUnreachable,
			Message:   fmt.Sprintf("gascity submit session %q: %v", sessionID, err),
			Retryable: true,
			Cause:     err,
		}
	}
	return submitMeta, nil
}

// buildInitialResult constructs the initial RPIPhaseExecutionResult, seeds
// RequestIDs with create/submit metadata, and emits the first running progress.
func buildInitialResult(ctx context.Context, req RPIPhaseExecutionRequest, cityName, sessionID, sessionAlias string, createMeta, submitMeta gascity.ResponseMeta) (RPIPhaseExecutionResult, error) {
	result := RPIPhaseExecutionResult{
		Status:              gascity.TerminalStatusRunning,
		RequestIDs:          map[string]string{},
		GasCityCityName:     cityName,
		GasCitySessionID:    sessionID,
		GasCitySessionAlias: sessionAlias,
	}
	addRequestID(result.RequestIDs, "create", createMeta.RequestID)
	addRequestID(result.RequestIDs, "submit", submitMeta.RequestID)
	if err := emitRPIPhaseProgress(ctx, req, result, gascity.TerminalStatusRunning); err != nil {
		return result, err
	}
	return result, nil
}

// processGasCityEventStreamFrames consumes frames from the event stream until
// the session reaches a terminal state, returning the final result. The caller
// owns the stream lifecycle (defer Close + defer cancel of streamCtx). ctx is
// the outer request context used for progress callbacks and evidence capture;
// streamCtx is the (possibly timeout-bound) stream context used to classify
// stream-level errors.
func processGasCityEventStreamFrames(ctx, streamCtx context.Context, stream GasCityRPIEventStream, sessionID, sessionAlias string, req RPIPhaseExecutionRequest, e GasCityRPIPhaseExecutor, result RPIPhaseExecutionResult) (RPIPhaseExecutionResult, error) {
	for {
		frame, err := stream.NextEvent()
		if err != nil {
			return result, classifyStreamFrameError(streamCtx, sessionID, err)
		}
		done, err := processStreamFrame(ctx, frame, sessionID, sessionAlias, req, e, &result)
		if err != nil {
			return result, err
		}
		if done {
			return result, nil
		}
	}
}

// classifyStreamFrameError maps a stream-read error onto a RPIPhaseExecutionError
// with the right retry semantics, distinguishing context expiry, EOF, and
// generic stream failures.
func classifyStreamFrameError(streamCtx context.Context, sessionID string, err error) *RPIPhaseExecutionError {
	if streamCtx.Err() != nil {
		return &RPIPhaseExecutionError{
			Code:      FailureEventStreamUnavailable,
			Message:   fmt.Sprintf("gascity event wait for %q: %v", sessionID, streamCtx.Err()),
			Retryable: true,
			Cause:     streamCtx.Err(),
		}
	}
	if errors.Is(err, io.EOF) {
		return &RPIPhaseExecutionError{
			Code:      FailureEventStreamUnavailable,
			Message:   fmt.Sprintf("gascity event stream ended before terminal state for %q", sessionID),
			Retryable: true,
			Cause:     err,
		}
	}
	return &RPIPhaseExecutionError{
		Code:      FailureEventStreamUnavailable,
		Message:   fmt.Sprintf("gascity event stream for %q: %v", sessionID, err),
		Retryable: true,
		Cause:     err,
	}
}

// processStreamFrame applies a single stream frame to the result. Returns
// done=true when a terminal completed state has been reached and evidence
// has been captured. Mutates result in place.
func processStreamFrame(ctx context.Context, frame gascity.EventStreamFrame, sessionID, sessionAlias string, req RPIPhaseExecutionRequest, e GasCityRPIPhaseExecutor, result *RPIPhaseExecutionResult) (bool, error) {
	if cursor := gascity.CursorFromFrame(frame); cursor != "" {
		result.EventCursor = cursor
		if err := emitRPIPhaseProgress(ctx, req, *result, gascity.TerminalStatusRunning); err != nil {
			return false, err
		}
	}
	event := frame.CityEvent
	if event == nil {
		return reconcileGasCitySessionTerminal(ctx, req, e, result, false)
	}
	if !gasCityEventMatchesSession(event, sessionID, sessionAlias) {
		return false, nil
	}
	classification := gascity.ClassifyTerminalState(gascity.TerminalStateInput{
		EventType:    event.Type,
		EventPayload: event.Payload,
	})
	if !classification.Terminal {
		return false, nil
	}
	result.Status = classification.Status
	if err := emitRPIPhaseProgress(ctx, req, *result, classification.Status); err != nil {
		return false, err
	}
	if classification.Status != gascity.TerminalStatusCompleted || classification.Degraded {
		code := failureCodeForTerminalStatus(classification.Status)
		message := fmt.Sprintf("gascity session %q terminal %s", sessionID, classification.Status)
		if classification.Reason != "" {
			message += ": " + classification.Reason
		}
		return false, &RPIPhaseExecutionError{Code: code, Message: message}
	}
	evidencePath, err := captureGasCityRPIEvidence(ctx, e.Client, req, *result)
	if err != nil {
		return false, err
	}
	result.EvidencePath = evidencePath
	result.Artifacts = map[string]string{
		phaseArtifactKey(req.Phase, "gascity_evidence"): evidencePath,
	}
	return true, nil
}

func reconcileGasCitySessionTerminal(ctx context.Context, req RPIPhaseExecutionRequest, e GasCityRPIPhaseExecutor, result *RPIPhaseExecutionResult, strict bool) (bool, error) {
	session, meta, err := e.Client.GetSession(ctx, result.GasCityCityName, result.GasCitySessionID, gascity.SessionGetOptions{Peek: true})
	if err != nil {
		if strict {
			return false, &RPIPhaseExecutionError{
				Code:      FailureProviderUnreachable,
				Message:   fmt.Sprintf("gascity get session %q: %v", result.GasCitySessionID, err),
				Retryable: true,
				Cause:     err,
			}
		}
		return false, nil
	}
	addRequestID(result.RequestIDs, "get_session", meta.RequestID)
	classification := gascity.ClassifyTerminalState(gascity.TerminalStateInput{
		SessionState:  session.State,
		SessionStatus: session.Status,
	})
	if !classification.Terminal {
		return false, nil
	}
	result.Status = classification.Status
	if err := emitRPIPhaseProgress(ctx, req, *result, classification.Status); err != nil {
		return false, err
	}
	if classification.Status != gascity.TerminalStatusCompleted || classification.Degraded {
		code := failureCodeForTerminalStatus(classification.Status)
		message := fmt.Sprintf("gascity session %q terminal %s", result.GasCitySessionID, classification.Status)
		if classification.Reason != "" {
			message += ": " + classification.Reason
		}
		return false, &RPIPhaseExecutionError{Code: code, Message: message}
	}
	evidencePath, err := captureGasCityRPIEvidence(ctx, e.Client, req, *result)
	if err != nil {
		return false, err
	}
	result.EvidencePath = evidencePath
	result.Artifacts = map[string]string{
		phaseArtifactKey(req.Phase, "gascity_evidence"): evidencePath,
	}
	return true, nil
}

func interruptGasCitySessionAfterAbort(result RPIPhaseExecutionResult, e GasCityRPIPhaseExecutor, cause error) {
	if e.Client == nil || result.GasCityCityName == "" || result.GasCitySessionID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	message := fmt.Sprintf("AgentOps daemon RPI phase stopped: %v", cause)
	_, meta, _ := e.Client.SubmitSession(ctx, result.GasCityCityName, result.GasCitySessionID, gascity.SessionSubmitRequest{
		Message: message,
		Intent:  "interrupt_now",
	})
	addRequestID(result.RequestIDs, "interrupt", meta.RequestID)
}

func emitRPIPhaseProgress(ctx context.Context, req RPIPhaseExecutionRequest, result RPIPhaseExecutionResult, status string) error {
	if req.Progress == nil {
		return nil
	}
	artifacts := map[string]string{}
	if result.GasCityCityName != "" {
		artifacts[phaseArtifactKey(req.Phase, "gascity_city_name")] = result.GasCityCityName
	}
	if result.GasCitySessionID != "" {
		artifacts[phaseArtifactKey(req.Phase, "gascity_session_id")] = result.GasCitySessionID
	}
	if result.GasCitySessionAlias != "" {
		artifacts[phaseArtifactKey(req.Phase, "gascity_session_alias")] = result.GasCitySessionAlias
	}
	if result.EventCursor != "" {
		artifacts[phaseArtifactKey(req.Phase, "gascity_event_cursor")] = result.EventCursor
	}
	return req.Progress(ctx, RPIPhaseProgress{
		Phase:     req.Phase,
		Status:    status,
		Artifacts: artifacts,
	})
}

func captureGasCityRPIEvidence(ctx context.Context, client GasCityRPIClient, req RPIPhaseExecutionRequest, result RPIPhaseExecutionResult) (string, error) {
	transcript, meta, err := client.SessionTranscript(ctx, result.GasCityCityName, result.GasCitySessionID, gascity.TranscriptOptions{
		Format: "conversation",
	})
	if err != nil {
		return "", &RPIPhaseExecutionError{
			Code:    FailureTerminalWithoutTranscript,
			Message: fmt.Sprintf("gascity transcript for %q: %v", result.GasCitySessionID, err),
			Cause:   err,
		}
	}
	addRequestID(result.RequestIDs, "transcript", meta.RequestID)
	artifacts := make([]cliRPI.GasCityTranscriptArtifact, 0, len(transcript.Artifacts))
	for _, artifact := range transcript.Artifacts {
		artifacts = append(artifacts, cliRPI.GasCityTranscriptArtifact{
			Path: artifact.Path,
			Kind: artifact.Kind,
		})
	}
	path, err := cliRPI.WriteGasCityPhaseEvidence(req.Root, cliRPI.GasCityPhaseEvidence{
		RunID:                req.RunID,
		Phase:                req.Phase,
		PhaseName:            req.PhaseName,
		CityName:             result.GasCityCityName,
		SessionID:            result.GasCitySessionID,
		SessionAlias:         result.GasCitySessionAlias,
		Status:               result.Status,
		EventCursor:          result.EventCursor,
		RequestIDs:           result.RequestIDs,
		TranscriptID:         firstNonEmpty(transcript.ID, transcript.SessionID, result.GasCitySessionID),
		TranscriptFormat:     transcript.Format,
		TranscriptTurnCount:  len(transcript.Turns),
		TranscriptMsgCount:   len(transcript.Messages),
		TranscriptArtifacts:  artifacts,
		TranscriptCapturedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

func requestFromPhaseSpec(root string, claim QueueClaim, spec RPIPhaseJobSpec) RPIPhaseExecutionRequest {
	return RPIPhaseExecutionRequest{
		Root:                root,
		JobID:               claim.Job.JobID,
		RunID:               spec.RunID,
		Goal:                spec.Goal,
		EpicID:              spec.EpicID,
		ExecutionPacketPath: spec.ExecutionPacketPath,
		ParentRunJobID:      spec.ParentRunJobID,
		Phase:               spec.Phase,
		PhaseName:           spec.PhaseName,
		Attempt:             firstPositive(spec.Attempt, claim.Job.Attempt),
		Backend:             spec.Backend,
		GasCityCityName:     spec.GasCityCityName,
		GasCitySessionAlias: spec.GasCitySessionAlias,
		PhaseTimeout:        parseRPIPhaseTimeout(spec.PhaseTimeout),
	}
}

func isRPIJobType(jobType JobType) bool {
	return jobType == JobTypeRPIRun || jobType == JobTypeRPIPhase
}

func providerFailureLooksRetryable(code FailureCode) bool {
	switch code {
	case FailureProviderUnreachable, FailureEventStreamUnavailable, FailureDaemonUnavailable:
		return true
	default:
		return false
	}
}

func failureCodeForTerminalStatus(status string) FailureCode {
	switch status {
	case gascity.TerminalStatusLost:
		return FailureSessionLost
	case gascity.TerminalStatusProviderUnreachable:
		return FailureProviderUnreachable
	case gascity.TerminalStatusEventStreamUnavailable:
		return FailureEventStreamUnavailable
	case gascity.TerminalStatusTerminalWithoutTranscript:
		return FailureTerminalWithoutTranscript
	default:
		return FailureRequestRejected
	}
}

func gasCityEventMatchesSession(event *gascity.EventStreamEnvelope, sessionID, sessionAlias string) bool {
	if event == nil {
		return false
	}
	for _, candidate := range []string{
		event.Subject,
		payloadString(event.Payload, "session_id"),
		payloadString(event.Payload, "sessionId"),
		payloadString(event.Payload, "alias"),
	} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if candidate == sessionID || candidate == sessionAlias {
			return true
		}
	}
	return false
}

func payloadString(payload map[string]any, key string) string {
	if len(payload) == 0 {
		return ""
	}
	raw, ok := payload[key]
	if !ok {
		return ""
	}
	switch value := raw.(type) {
	case string:
		return value
	default:
		return fmt.Sprint(value)
	}
}

func addRequestID(ids map[string]string, key, value string) {
	if strings.TrimSpace(value) != "" {
		ids[key] = value
	}
}

func mergeArtifacts(dst, src map[string]string) {
	for key, value := range src {
		if strings.TrimSpace(value) != "" {
			dst[key] = value
		}
	}
}

func phaseArtifactKey(phase int, label string) string {
	return fmt.Sprintf("phase_%d_%s", phase, label)
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
