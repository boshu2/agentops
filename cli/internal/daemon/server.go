package daemon

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/openclaw"
)

type ServerOptions struct {
	Now            func() time.Time
	SourceLedger   string
	QueueOptions   QueueOptions
	MutationPolicy MutationPolicy
}

// MaxJobSubmissionBytes caps how large a /v1/jobs (and /v1/jobs/cancel)
// request body may be. Submissions larger than this return 413 instead of
// being decoded, which prevents oversized job payloads from making it into
// the append-only ledger as events that would later overflow the replay
// reader. Paired with MaxLedgerLineBytes in store.go (replay tolerates
// larger lines than submission so daemon-emitted events have headroom).
const MaxJobSubmissionBytes = 1 * 1024 * 1024

const maxReadOnlyEventsLimit = 1000

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

type CancelJobRequest struct {
	RequestID string `json:"request_id,omitempty"`
	JobID     string `json:"job_id"`
	Reason    string `json:"reason,omitempty"`
}

type CancelJobResponse struct {
	Cancelled        bool             `json:"cancelled"`
	Outcome          CancelJobOutcome `json:"outcome"`
	Job              QueueJobState    `json:"job"`
	ProjectionStatus ProjectionStatus `json:"projection_status"`
	ProjectionLag    ProjectionLag    `json:"projection_lag"`
	DegradedReasons  []string         `json:"degraded_reasons,omitempty"`
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
	policy := server.mutationPolicy
	for _, prefix := range []string{"", "/v1"} {
		registerMutationRoute(mux, prefix+"/jobs", policy, server.handleSubmitJob)
		registerMutationRoute(mux, prefix+"/jobs/cancel", policy, server.handleCancelJob)
	}
	registerMutationRoute(mux, "POST /v1/jobs/{id}/cancel", policy, server.handleCancelJobByPath)
	registerMutationRoute(mux, openclaw.TriggerJobsPath, policy, server.handleOpenClawTriggerJob)
	// Schedule routes (soc-8inr.5).
	registerMutationRoute(mux, "POST /v1/schedules", policy, server.handlePostSchedule)
	registerMutationRoute(mux, "DELETE /v1/schedules/{name}", policy, server.handleDeleteSchedule)
	// GET /v1/schedules is intentionally read-only — no mutation auth.
	registerReadOnlyRoute(mux, "GET /v1/schedules", server.handleListSchedules)
	return mux
}

func (s *ReadOnlyServer) registerReadOnlyRoutes(mux *http.ServeMux) {
	for _, prefix := range []string{"", "/v1"} {
		registerReadOnlyRoute(mux, prefix+"/health", s.handleHealth)
		registerReadOnlyRoute(mux, prefix+"/ready", s.handleReady)
		registerReadOnlyRoute(mux, prefix+"/status", s.handleStatus)
		registerReadOnlyRoute(mux, prefix+"/events", s.handleEvents)
	}
	registerReadOnlyRoute(mux, "/openclaw/v1/health", s.handleOpenClawHealth)
	registerReadOnlyRoute(mux, "/openclaw/v1/snapshot/latest", s.handleOpenClawSnapshotLatest)
	registerReadOnlyRoute(mux, "/openclaw/v1/runs", s.handleOpenClawRuns)
	registerReadOnlyRoute(mux, "/openclaw/v1/jobs", s.handleOpenClawJobs)
	registerReadOnlyRoute(mux, "/openclaw/v1/wiki", s.handleOpenClawWiki)

	// Plans projection (atom-1 registers the read-side routes; atom-2 fills the
	// projection body). Read-side absorption per foundation §6 site 3 (alt) —
	// no entry in DefaultMutationPathCapabilities. Routed via registerReadOnlyRoute
	// to satisfy the bypass-guard at scripts/check-mutation-route-coverage.sh
	// (soc-8inr.5 / amendment A2).
	registerReadOnlyRoute(mux, "/v1/plans/manifest", s.handlePlansManifest)
	registerReadOnlyRoute(mux, "/v1/plans/diff", s.handlePlansDiff)
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
	// Filter the schedule.* sentinel rows out of the status queue projection so
	// `ao daemon jobs list` never shows the placeholder phantom (job_id="schedule",
	// empty job_type) created by ScheduleCreated/Deleted ledger events.
	queue := state.Queue
	if len(queue.Jobs) > 0 {
		filtered := make([]QueueJobState, 0, len(queue.Jobs))
		for _, j := range queue.Jobs {
			if isRealQueueJob(j) {
				filtered = append(filtered, j)
			}
		}
		queue.Jobs = filtered
	}
	writeJSON(w, http.StatusOK, ReadOnlyStatusResponse{
		Ready:         state.Ready,
		ProjectionLag: state.Lag,
		Queue:         queue,
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
	events, ok := filterLedgerEventsAfterCursor(state.Replay.Events, eventCursorFromRequest(r))
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "event cursor not found"})
		return
	}
	events, err = applyReadOnlyEventsLimit(events, r.URL.Query().Get("limit"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, ReadOnlyEventsResponse{
		Events:      events,
		Corrupt:     state.Replay.Corrupt,
		LastEventID: state.Lag.LastEventID,
	})
}

func (s *ReadOnlyServer) handlePlansManifest(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	// atom-1 stub: the Plans field on ProjectionSet is added in atom-2. Until
	// then, return an empty manifest envelope so callers can wire against the
	// shape without depending on the executor.
	writeJSON(w, http.StatusOK, map[string]any{
		"schema_version": ProjectionSchemaVersion,
		"entries":        []any{},
	})
}

func (s *ReadOnlyServer) handlePlansDiff(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	state, err := s.readState()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// atom-1 stub: filter the ledger by the same `since` cursor convention used
	// by /v1/events. atom-2 will scope to plans-relevant events once the
	// executor publishes them.
	events := filterLedgerEventsAfter(state.Replay.Events, r.URL.Query().Get("since"))
	writeJSON(w, http.StatusOK, map[string]any{
		"events":        events,
		"last_event_id": state.Lag.LastEventID,
	})
}

func filterLedgerEventsAfter(events []LedgerEvent, after string) []LedgerEvent {
	filtered, _ := filterLedgerEventsAfterCursor(events, after)
	return filtered
}

func filterLedgerEventsAfterCursor(events []LedgerEvent, after string) ([]LedgerEvent, bool) {
	after = strings.TrimSpace(after)
	if after == "" {
		return events, true
	}
	for i, event := range events {
		if event.EventID == after {
			return events[i+1:], true
		}
	}
	return nil, false
}

func eventCursorFromRequest(r *http.Request) string {
	query := r.URL.Query()
	if since := strings.TrimSpace(query.Get("since")); since != "" {
		return since
	}
	return strings.TrimSpace(query.Get("after_id"))
}

func applyReadOnlyEventsLimit(events []LedgerEvent, rawLimit string) ([]LedgerEvent, error) {
	rawLimit = strings.TrimSpace(rawLimit)
	if rawLimit == "" {
		return events, nil
	}
	limit, err := strconv.Atoi(rawLimit)
	if err != nil || limit < 0 {
		return nil, errors.New("limit must be a non-negative integer")
	}
	if limit > maxReadOnlyEventsLimit {
		limit = maxReadOnlyEventsLimit
	}
	if limit < len(events) {
		return events[:limit], nil
	}
	return events, nil
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
	decision, _ := MutationDecisionFromContext(r.Context())
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
	r.Body = http.MaxBytesReader(w, r.Body, MaxJobSubmissionBytes)
	var req openclaw.TriggerJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
				"error":     "request body exceeds MaxJobSubmissionBytes",
				"max_bytes": MaxJobSubmissionBytes,
			})
			return
		}
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
		Actor:          mutationActor("openclaw-trigger", decision),
		Payload:        req.Payload,
	}, QueueMutationOptions{Failpoint: queueFailpointFromRequest(r)})
	if err != nil {
		writeJSON(w, queueSubmitErrorStatus(err), map[string]string{"error": err.Error()})
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
	r.Body = http.MaxBytesReader(w, r.Body, MaxJobSubmissionBytes)
	decision, _ := MutationDecisionFromContext(r.Context())
	var req SubmitJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
				"error":     "request body exceeds MaxJobSubmissionBytes",
				"max_bytes": MaxJobSubmissionBytes,
			})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	queue := NewQueue(s.store, s.queueOptions())
	job, err := queue.SubmitJob(SubmitJobInput{
		RequestID:      RequestID(req.RequestID),
		JobID:          req.JobID,
		JobType:        req.JobType,
		IdempotencyKey: req.IdempotencyKey,
		Actor:          mutationActor("ao-http", decision),
		Payload:        req.Payload,
	}, QueueMutationOptions{Failpoint: queueFailpointFromRequest(r)})
	if err != nil {
		writeJSON(w, queueSubmitErrorStatus(err), map[string]string{"error": err.Error()})
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

func (s *ReadOnlyServer) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxJobSubmissionBytes)
	var req CancelJobRequest
	if !decodeCancelJobRequest(w, r, &req, false) {
		return
	}
	s.cancelJob(w, r, req)
}

func (s *ReadOnlyServer) handleCancelJobByPath(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxJobSubmissionBytes)
	pathJobID := strings.TrimSpace(r.PathValue("id"))
	if pathJobID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "job_id is required"})
		return
	}
	var req CancelJobRequest
	if !decodeCancelJobRequest(w, r, &req, true) {
		return
	}
	if strings.TrimSpace(req.JobID) != "" && strings.TrimSpace(req.JobID) != pathJobID {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "job_id does not match path"})
		return
	}
	req.JobID = pathJobID
	s.cancelJob(w, r, req)
}

func decodeCancelJobRequest(w http.ResponseWriter, r *http.Request, req *CancelJobRequest, allowEmpty bool) bool {
	err := json.NewDecoder(r.Body).Decode(req)
	if err == nil {
		return true
	}
	if allowEmpty && errors.Is(err, io.EOF) {
		return true
	}
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
			"error":     "request body exceeds MaxJobSubmissionBytes",
			"max_bytes": MaxJobSubmissionBytes,
		})
		return false
	}
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
	return false
}

func (s *ReadOnlyServer) cancelJob(w http.ResponseWriter, r *http.Request, req CancelJobRequest) {
	if strings.TrimSpace(req.JobID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "job_id is required"})
		return
	}
	decision, _ := MutationDecisionFromContext(r.Context())
	queue := NewQueue(s.store, s.queueOptions())
	cancelled, err := queue.CancelJob(CancelJobInput{
		RequestID: RequestID(req.RequestID),
		JobID:     req.JobID,
		Actor:     mutationActor("ao-http", decision),
		Reason:    req.Reason,
	}, QueueMutationOptions{Failpoint: queueFailpointFromRequest(r)})
	if err != nil {
		writeJSON(w, queueCancelErrorStatus(err), map[string]string{"error": err.Error()})
		return
	}
	projectionStatus := ProjectionStatusCurrent
	var degradedReasons []string
	state, err := s.readState()
	lag := ProjectionLag{}
	if err != nil {
		projectionStatus = ProjectionStatusDegraded
		degradedReasons = []string{err.Error()}
	} else {
		projectionStatus = state.Projections.Manifests[ProjectionDaemonJobStatus].Status
		degradedReasons = state.Projections.DegradedReasons
		lag = state.Lag
	}
	writeJSON(w, http.StatusAccepted, CancelJobResponse{
		Cancelled:        cancelled.Job.Status == JobStatusCancelled,
		Outcome:          cancelled.Outcome,
		Job:              cancelled.Job,
		ProjectionStatus: projectionStatus,
		ProjectionLag:    lag,
		DegradedReasons:  degradedReasons,
	})
}

func queueSubmitErrorStatus(err error) int {
	if errors.Is(err, ErrFailpoint) {
		return http.StatusServiceUnavailable
	}
	if isQueueValidationError(err) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func queueCancelErrorStatus(err error) int {
	if errors.Is(err, ErrFailpoint) {
		return http.StatusServiceUnavailable
	}
	if errors.Is(err, ErrJobNotFound) {
		return http.StatusNotFound
	}
	if isQueueValidationError(err) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func isQueueValidationError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.HasPrefix(msg, "invalid job type ") ||
		strings.HasPrefix(msg, "request_id ") ||
		msg == "request_id is required"
}

// ListSchedulesResponse is the body of GET /v1/schedules.
type ListSchedulesResponse struct {
	Schedules []RecurringJobTemplate `json:"schedules"`
}

// CreateScheduleResponse is the body of a successful POST /v1/schedules.
type CreateScheduleResponse struct {
	Name string `json:"name"`
}

// DeleteScheduleResponse is the body of a successful DELETE /v1/schedules/{name}.
type DeleteScheduleResponse struct {
	Name    string `json:"name"`
	Deleted bool   `json:"deleted"`
}

// handlePostSchedule serves POST /v1/schedules. Body is a JSON
// RecurringJobTemplate. Auth is enforced by registerMutationRoute (admin
// capability per DefaultMutationPathCapabilities).
//
// Returns:
//   - 201 Created on success
//   - 400 Bad Request on malformed JSON or missing required fields
//   - 409 Conflict when a schedule with the same name already exists
//   - 500 Internal Server Error on store failure
func (s *ReadOnlyServer) handlePostSchedule(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var template RecurringJobTemplate
	if err := dec.Decode(&template); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json: " + err.Error()})
		return
	}
	if strings.TrimSpace(template.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "schedule name is required"})
		return
	}
	if strings.ContainsAny(template.Name, "/\\") || strings.Contains(template.Name, "..") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "schedule name must not contain path separators or '..'"})
		return
	}
	if strings.TrimSpace(template.Cron) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cron is required"})
		return
	}
	if _, err := ParseCron(template.Cron); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if strings.TrimSpace(string(template.JobType)) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "job_type is required"})
		return
	}
	if err := ValidateRecurringJobTemplatePayload(template); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload: " + err.Error()})
		return
	}
	if err := s.store.SaveSchedule(template); err != nil {
		// Duplicate-name path is a 409; everything else is 500.
		if strings.Contains(err.Error(), "already exists") {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, CreateScheduleResponse{Name: template.Name})
}

// handleListSchedules serves GET /v1/schedules. Read-only — no mutation auth
// (registered via registerReadOnlyRoute).
func (s *ReadOnlyServer) handleListSchedules(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	schedules, err := s.store.ListSchedules()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if schedules == nil {
		schedules = []RecurringJobTemplate{}
	}
	writeJSON(w, http.StatusOK, ListSchedulesResponse{Schedules: schedules})
}

// handleDeleteSchedule serves DELETE /v1/schedules/{name}. Auth is enforced
// by registerMutationRoute. DeleteSchedule is idempotent at the store layer.
func (s *ReadOnlyServer) handleDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodDelete) {
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "schedule name is required"})
		return
	}
	// Reject names that look like path traversal or contain path separators —
	// schedule names are flat keys, not paths. Without this check, a DELETE on
	// "../../etc/passwd" gets URL-decoded into the {name} pattern and the
	// store dutifully reports "deleted" for a name no operator ever created.
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "schedule name must not contain path separators or '..'"})
		return
	}
	if err := s.store.DeleteSchedule(name); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, DeleteScheduleResponse{Name: name, Deleted: true})
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
	rebuildOpts := ProjectionRebuildOptions{
		RebuiltAt:    s.now(),
		SourceLedger: sourceLedger,
	}
	// Skip-and-rebuild on stale/corrupt snapshot: surface the reason via
	// degraded_reasons rather than blocking the read.
	snapshot, snapshotPath, snapshotErr := s.store.LoadLatestProjectionSnapshot()
	if snapshotErr == nil && snapshotPath != "" {
		rebuildOpts.FromSnapshot = &snapshot
	}
	projections, err := RebuildProjections(replay.Events, rebuildOpts)
	if err != nil {
		return readOnlyState{}, err
	}
	if snapshotErr != nil {
		projections.markDegraded("ignored stale projection snapshot: " + snapshotErr.Error())
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
			ArtifactRefs:      openClawArtifactRefs(job.ArtifactRefs),
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

func openClawArtifactRefs(in map[string]ArtifactRef) map[string]openclaw.ArtifactRef {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]openclaw.ArtifactRef, len(in))
	for key, ref := range in {
		out[key] = openclaw.ArtifactRef{
			Path:      ref.Path,
			SHA256:    ref.SHA256,
			Size:      ref.Size,
			WrittenAt: ref.WrittenAt,
		}
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
		policy.AllowedPaths = []string{
			"/jobs", "/v1/jobs",
			"/jobs/cancel", "/v1/jobs/cancel", "/v1/jobs/*/cancel",
			openclaw.TriggerJobsPath,
			"/v1/schedules",   // POST + GET (GET bypasses auth via registerReadOnlyRoute)
			"/v1/schedules/*", // DELETE /v1/schedules/{name}
		}
	}
	if len(policy.AllowedMethods) == 0 {
		policy.AllowedMethods = []string{http.MethodPost, http.MethodDelete}
	}
	if len(policy.PathCapabilities) == 0 {
		policy.PathCapabilities = DefaultMutationPathCapabilities()
	}
	policy.RequireLocalRemote = true
	return policy
}

func mutationActor(base string, decision MutationDecision) string {
	if strings.TrimSpace(decision.TokenName) == "" {
		return base
	}
	return base + ":" + sanitizeIDPart(decision.TokenName)
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
