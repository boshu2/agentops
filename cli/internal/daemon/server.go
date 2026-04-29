package daemon

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"time"

	"github.com/boshu2/agentops/cli/internal/openclaw"
)

type ServerOptions struct {
	Now            func() time.Time
	SourceLedger   string
	QueueOptions   QueueOptions
	MutationPolicy MutationPolicy
}

type ReadOnlyServer struct {
	store *Store
	opts  ServerOptions
}

type ReadOnlyHealthResponse struct {
	Status   string `json:"status"`
	Daemon   string `json:"daemon"`
	ReadOnly bool   `json:"read_only"`
	Now      string `json:"now"`
}

type ReadOnlyReadyResponse struct {
	Ready              bool                 `json:"ready"`
	LedgerReplayStatus SnapshotReplayStatus `json:"ledger_replay_status"`
	ProjectionStatus   ProjectionStatus     `json:"projection_status"`
	ProjectionLag      ProjectionLag        `json:"projection_lag"`
	DegradedReasons    []string             `json:"degraded_reasons,omitempty"`
}

type ReadOnlyStatusResponse struct {
	Ready         bool          `json:"ready"`
	ProjectionLag ProjectionLag `json:"projection_lag"`
	Queue         QueueSnapshot `json:"queue"`
	Projections   ProjectionSet `json:"projections"`
}

type ReadOnlyEventsResponse struct {
	Events      []LedgerEvent   `json:"events"`
	Corrupt     []CorruptRecord `json:"corrupt,omitempty"`
	LastEventID string          `json:"last_event_id,omitempty"`
}

type ProjectionLag struct {
	LastEventID        string `json:"last_event_id,omitempty"`
	EventCount         int    `json:"event_count"`
	CorruptRecordCount int    `json:"corrupt_record_count"`
	Degraded           bool   `json:"degraded"`
}

type SubmitJobRequest struct {
	RequestID      string         `json:"request_id,omitempty"`
	JobID          string         `json:"job_id,omitempty"`
	JobType        JobType        `json:"job_type"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	Payload        map[string]any `json:"payload,omitempty"`
}

type SubmitJobResponse struct {
	Accepted         bool             `json:"accepted"`
	RequestID        string           `json:"request_id"`
	JobID            string           `json:"job_id"`
	Status           JobStatus        `json:"status"`
	LastEventID      string           `json:"last_event_id,omitempty"`
	ProjectionStatus ProjectionStatus `json:"projection_status"`
	ProjectionLag    ProjectionLag    `json:"projection_lag"`
	DegradedReasons  []string         `json:"degraded_reasons,omitempty"`
	IdempotencyKey   string           `json:"idempotency_key,omitempty"`
}

type readOnlyState struct {
	Replay      ReplayResult
	Queue       QueueSnapshot
	Projections ProjectionSet
	Lag         ProjectionLag
	Ready       bool
}

func NewReadOnlyRouter(store *Store, opts ServerOptions) http.Handler {
	server := &ReadOnlyServer{store: store, opts: opts}
	mux := http.NewServeMux()
	server.registerReadOnlyRoutes(mux)
	return mux
}

func NewDaemonRouter(store *Store, opts ServerOptions) http.Handler {
	server := &ReadOnlyServer{store: store, opts: opts}
	mux := http.NewServeMux()
	server.registerReadOnlyRoutes(mux)
	for _, prefix := range []string{"", "/v1"} {
		mux.HandleFunc(prefix+"/jobs", server.handleSubmitJob)
	}
	mux.HandleFunc(openclaw.TriggerJobsPath, server.handleOpenClawTriggerJob)
	return mux
}

func (s *ReadOnlyServer) registerReadOnlyRoutes(mux *http.ServeMux) {
	for _, prefix := range []string{"", "/v1"} {
		mux.HandleFunc(prefix+"/health", s.handleHealth)
		mux.HandleFunc(prefix+"/ready", s.handleReady)
		mux.HandleFunc(prefix+"/status", s.handleStatus)
		mux.HandleFunc(prefix+"/events", s.handleEvents)
	}
	mux.HandleFunc("/openclaw/v1/health", s.handleOpenClawHealth)
	mux.HandleFunc("/openclaw/v1/snapshot/latest", s.handleOpenClawSnapshotLatest)
	mux.HandleFunc("/openclaw/v1/runs", s.handleOpenClawRuns)
	mux.HandleFunc("/openclaw/v1/jobs", s.handleOpenClawJobs)
	mux.HandleFunc("/openclaw/v1/wiki", s.handleOpenClawWiki)
}

func (s *ReadOnlyServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, ReadOnlyHealthResponse{
		Status:   "ok",
		Daemon:   "agentopsd",
		ReadOnly: true,
		Now:      s.now().Format(time.RFC3339Nano),
	})
}

func (s *ReadOnlyServer) handleReady(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	state, err := s.readState()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	status := SnapshotReplayComplete
	if len(state.Replay.Corrupt) > 0 {
		status = SnapshotReplayCorrupt
	}
	projectionStatus := ProjectionStatusCurrent
	if state.Lag.Degraded {
		projectionStatus = ProjectionStatusDegraded
	}
	writeJSON(w, http.StatusOK, ReadOnlyReadyResponse{
		Ready:              state.Ready,
		LedgerReplayStatus: status,
		ProjectionStatus:   projectionStatus,
		ProjectionLag:      state.Lag,
		DegradedReasons:    state.Projections.DegradedReasons,
	})
}

func (s *ReadOnlyServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	state, err := s.readState()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, ReadOnlyStatusResponse{
		Ready:         state.Ready,
		ProjectionLag: state.Lag,
		Queue:         state.Queue,
		Projections:   state.Projections,
	})
}

func (s *ReadOnlyServer) handleEvents(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	state, err := s.readState()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, ReadOnlyEventsResponse{
		Events:      state.Replay.Events,
		Corrupt:     state.Replay.Corrupt,
		LastEventID: state.Lag.LastEventID,
	})
}

func (s *ReadOnlyServer) handleOpenClawHealth(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	snapshot, state, err := s.readOpenClawSnapshot()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, openclaw.HealthResponse{
		Status:         "ok",
		Ready:          state.Ready,
		SnapshotID:     snapshot.SnapshotID,
		GeneratedAt:    snapshot.GeneratedAt,
		Source:         snapshot.Source,
		SnapshotStatus: snapshot.Status,
		ResourceCounts: openclaw.ResourceCounts{
			Runs: len(snapshot.Resources.Runs),
			Jobs: len(snapshot.Resources.Jobs),
			Wiki: len(snapshot.Resources.Wiki),
		},
		DegradedReasons: state.Projections.DegradedReasons,
	})
}

func (s *ReadOnlyServer) handleOpenClawSnapshotLatest(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	snapshot, _, err := s.readOpenClawSnapshot()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *ReadOnlyServer) handleOpenClawRuns(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	snapshot, _, err := s.readOpenClawSnapshot()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, openclaw.RunsResponse{Runs: snapshot.Resources.Runs})
}

func (s *ReadOnlyServer) handleOpenClawJobs(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	snapshot, _, err := s.readOpenClawSnapshot()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, openclaw.JobsResponse{Jobs: snapshot.Resources.Jobs})
}

func (s *ReadOnlyServer) handleOpenClawWiki(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	snapshot, _, err := s.readOpenClawSnapshot()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, openclaw.WikiResponse{Wiki: snapshot.Resources.Wiki})
}

func (s *ReadOnlyServer) handleOpenClawTriggerJob(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	policy := s.mutationPolicy()
	if err := AuthorizeMutation(r, policy); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}
	state, err := s.readState()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !state.Ready {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error":            "daemon is not ready",
			"projection_lag":   state.Lag,
			"degraded_reasons": state.Projections.DegradedReasons,
		})
		return
	}
	var req openclaw.TriggerJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	jobType := JobType(req.JobType)
	if !openClawTriggerAllowedJobType(jobType) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "job_type is not allowlisted for OpenClaw trigger"})
		return
	}
	queue := NewQueue(s.store, s.queueOptions())
	job, err := queue.SubmitJob(SubmitJobInput{
		RequestID:      RequestID(req.RequestID),
		JobID:          req.JobID,
		JobType:        jobType,
		IdempotencyKey: req.IdempotencyKey,
		Actor:          "openclaw-trigger",
		Payload:        req.Payload,
	}, QueueMutationOptions{Failpoint: queueFailpointFromRequest(r)})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrFailpoint) {
			status = http.StatusServiceUnavailable
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	state, err = s.readState()
	snapshotStatus := openclaw.SnapshotStatusDegraded
	if err == nil {
		snapshotStatus = mapOpenClawSnapshotStatus(state)
	}
	writeJSON(w, http.StatusAccepted, openclaw.TriggerJobResponse{
		Accepted:       true,
		RequestID:      job.RequestID,
		JobID:          job.JobID,
		JobType:        string(job.JobType),
		Status:         string(job.Status),
		LastEventID:    job.LastEventID,
		SnapshotStatus: snapshotStatus,
		IdempotencyKey: job.IdempotencyKey,
	})
}

func (s *ReadOnlyServer) handleSubmitJob(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	policy := s.mutationPolicy()
	if err := AuthorizeMutation(r, policy); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}
	var req SubmitJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	queue := NewQueue(s.store, s.queueOptions())
	job, err := queue.SubmitJob(SubmitJobInput{
		RequestID:      RequestID(req.RequestID),
		JobID:          req.JobID,
		JobType:        req.JobType,
		IdempotencyKey: req.IdempotencyKey,
		Actor:          "ao-http",
		Payload:        req.Payload,
	}, QueueMutationOptions{Failpoint: queueFailpointFromRequest(r)})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrFailpoint) {
			status = http.StatusServiceUnavailable
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}

	projectionStatus := ProjectionStatusCurrent
	var degradedReasons []string
	state, err := s.readState()
	if err != nil {
		projectionStatus = ProjectionStatusDegraded
		degradedReasons = []string{err.Error()}
	} else {
		projectionStatus = state.Projections.Manifests[ProjectionDaemonJobStatus].Status
		degradedReasons = state.Projections.DegradedReasons
	}
	if r.Header.Get("X-AgentOps-Failpoint") == "projection_rebuild" {
		projectionStatus = ProjectionStatusDegraded
		degradedReasons = append(degradedReasons, "projection rebuild failpoint after durable ledger append")
	}
	lag := ProjectionLag{}
	if err == nil {
		lag = state.Lag
	}
	writeJSON(w, http.StatusAccepted, SubmitJobResponse{
		Accepted:         true,
		RequestID:        job.RequestID,
		JobID:            job.JobID,
		Status:           job.Status,
		LastEventID:      job.LastEventID,
		ProjectionStatus: projectionStatus,
		ProjectionLag:    lag,
		DegradedReasons:  degradedReasons,
		IdempotencyKey:   job.IdempotencyKey,
	})
}

func (s *ReadOnlyServer) readState() (readOnlyState, error) {
	replay, err := s.store.ReplayLedgerReadOnly()
	if err != nil {
		return readOnlyState{}, err
	}
	sourceLedger := s.opts.SourceLedger
	if sourceLedger == "" {
		sourceLedger = filepath.ToSlash(filepath.Join(StoreDirRel, LedgerFileName))
	}
	projections, err := RebuildProjections(replay.Events, ProjectionRebuildOptions{
		RebuiltAt:    s.now(),
		SourceLedger: sourceLedger,
	})
	if err != nil {
		return readOnlyState{}, err
	}
	lag := ProjectionLag{
		LastEventID:        projections.LastEventID,
		EventCount:         len(replay.Events),
		CorruptRecordCount: len(replay.Corrupt),
		Degraded:           len(replay.Corrupt) > 0,
	}
	if lag.Degraded {
		projections.markDegraded("read-only ledger replay observed corrupt records")
	}
	queueOpts := s.opts.QueueOptions
	queueOpts.Now = s.now
	queue := NewQueue(s.store, queueOpts)
	queueSnapshot, err := queue.snapshotFromEvents(replay.Events)
	if err != nil {
		return readOnlyState{}, err
	}
	return readOnlyState{
		Replay:      replay,
		Queue:       queueSnapshot,
		Projections: projections,
		Lag:         lag,
		Ready:       !lag.Degraded,
	}, nil
}

func (s *ReadOnlyServer) readOpenClawSnapshot() (openclaw.ConsumerSnapshot, readOnlyState, error) {
	state, err := s.readState()
	if err != nil {
		return openclaw.ConsumerSnapshot{}, readOnlyState{}, err
	}
	generatedAt, err := time.Parse(time.RFC3339Nano, state.Projections.RebuiltAt)
	if err != nil {
		generatedAt = s.now()
	}
	snapshot, err := openclaw.BuildConsumerSnapshot(openclaw.ProjectionInput{
		GeneratedAt: generatedAt,
		Source: openclaw.SnapshotSource{
			Ledger:      state.Projections.SourceLedger,
			LastEventID: state.Projections.LastEventID,
		},
		Status: mapOpenClawSnapshotStatus(state),
		Runs:   openClawResourcesFromJobs(state.Projections.OpenClaw.Resources.Runs, openclaw.ResourceKindRun),
		Jobs:   openClawResourcesFromJobs(state.Projections.OpenClaw.Resources.Jobs, openclaw.ResourceKindJob),
		Wiki:   openClawResourcesFromJobs(state.Projections.OpenClaw.Resources.Wiki, openclaw.ResourceKindWiki),
	})
	if err != nil {
		return openclaw.ConsumerSnapshot{}, readOnlyState{}, err
	}
	return snapshot, state, nil
}

func mapOpenClawSnapshotStatus(state readOnlyState) openclaw.SnapshotStatus {
	if state.Lag.Degraded {
		return openclaw.SnapshotStatusDegraded
	}
	switch state.Projections.Manifests[ProjectionOpenClaw].Status {
	case ProjectionStatusStale:
		return openclaw.SnapshotStatusStale
	case ProjectionStatusDegraded:
		return openclaw.SnapshotStatusDegraded
	default:
		return openclaw.SnapshotStatusCurrent
	}
}

func openClawResourcesFromJobs(jobs []JobProjection, kind openclaw.ResourceKind) []openclaw.ResourceSummary {
	out := make([]openclaw.ResourceSummary, 0, len(jobs))
	for _, job := range jobs {
		resourceID := job.JobID
		if kind == openclaw.ResourceKindWiki {
			resourceID = "wiki-" + job.JobID
		}
		resource := openclaw.ResourceSummary{
			ResourceID:        resourceID,
			ResourceKind:      kind,
			JobID:             job.JobID,
			JobType:           string(job.JobType),
			RunID:             resourceRunID(job),
			RequestID:         job.RequestID,
			RequestIDs:        append([]string{}, job.RequestIDs...),
			Status:            string(job.Status),
			ResultStatus:      string(job.ResultStatus),
			Failure:           openClawFailure(job.Failure),
			Artifacts:         cloneStringMap(job.Artifacts),
			ProjectionTargets: projectionTargetStrings(job.ProjectionTargets),
			CreatedAt:         job.CreatedAt,
			UpdatedAt:         job.UpdatedAt,
			LastEventID:       job.LastEventID,
		}
		out = append(out, openclaw.WithProvenance(resource))
	}
	if out == nil {
		return []openclaw.ResourceSummary{}
	}
	return out
}

func resourceRunID(job JobProjection) string {
	switch job.JobType {
	case JobTypeRPIRun, JobTypeRPIPhase, JobTypeDreamRun, JobTypeDreamStage:
		return job.JobID
	default:
		return ""
	}
}

func openClawFailure(failure *JobFailure) *openclaw.FailureSummary {
	if failure == nil {
		return nil
	}
	return &openclaw.FailureSummary{
		Code:      string(failure.Code),
		Message:   failure.Message,
		Retryable: failure.Retryable,
	}
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func (s *ReadOnlyServer) now() time.Time {
	if s.opts.Now != nil {
		return s.opts.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *ReadOnlyServer) queueOptions() QueueOptions {
	opts := s.opts.QueueOptions
	opts.Now = s.now
	return opts
}

func (s *ReadOnlyServer) mutationPolicy() MutationPolicy {
	policy := s.opts.MutationPolicy
	if policy.TokenHeader == "" {
		policy.TokenHeader = DefaultMutationTokenHeader
	}
	if len(policy.AllowedPaths) == 0 {
		policy.AllowedPaths = []string{"/jobs", "/v1/jobs", openclaw.TriggerJobsPath}
	}
	if len(policy.AllowedMethods) == 0 {
		policy.AllowedMethods = []string{http.MethodPost}
	}
	policy.RequireLocalRemote = true
	return policy
}

func openClawTriggerAllowedJobType(jobType JobType) bool {
	switch jobType {
	case JobTypeOpenClawSnapshot, JobTypeRPIRun, JobTypeDreamRun, JobTypeWikiForge:
		return true
	default:
		return false
	}
}

func queueFailpointFromRequest(r *http.Request) QueueFailpoint {
	switch r.Header.Get("X-AgentOps-Failpoint") {
	case string(QueueFailpointBeforeAppend):
		return QueueFailpointBeforeAppend
	case string(QueueFailpointAfterAppendBeforeAck):
		return QueueFailpointAfterAppendBeforeAck
	default:
		return ""
	}
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	w.Header().Set("Allow", method)
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	return false
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
