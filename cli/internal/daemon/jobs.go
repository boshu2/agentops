package daemon

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

var (
	ErrNoClaimableJobs    = errors.New("daemon queue: no claimable jobs")
	ErrJobAlreadyClaimed  = errors.New("daemon queue: job already claimed")
	ErrClaimFenceMismatch = errors.New("daemon queue: claim token or lease epoch mismatch")
	ErrLeaseExpired       = errors.New("daemon queue: lease expired")
	ErrJobNotFound        = errors.New("daemon queue: job not found")
	ErrFailpoint          = errors.New("daemon queue failpoint")
)

type QueueFailpoint string

const (
	QueueFailpointBeforeAppend         QueueFailpoint = "before_append"
	QueueFailpointAfterAppendBeforeAck QueueFailpoint = "after_append_before_ack"
)

type QueueOptions struct {
	LeaseDuration time.Duration
	MaxAttempts   int
	Actor         string
	Now           func() time.Time
}

type QueueMutationOptions struct {
	Failpoint QueueFailpoint
}

// CancelJobOutcome reports how a cancellation request affected a job.
type CancelJobOutcome string

const (
	// CancelJobOutcomeCancelled means the queue appended a cancellation event.
	CancelJobOutcomeCancelled CancelJobOutcome = "cancelled"
	// CancelJobOutcomeAlreadyTerminalCompleted means the job was already completed.
	CancelJobOutcomeAlreadyTerminalCompleted CancelJobOutcome = "already_terminal_completed"
	// CancelJobOutcomeAlreadyTerminalFailed means the job was already failed.
	CancelJobOutcomeAlreadyTerminalFailed CancelJobOutcome = "already_terminal_failed"
	// CancelJobOutcomeAlreadyTerminalCancelled means the job was already cancelled.
	CancelJobOutcomeAlreadyTerminalCancelled CancelJobOutcome = "already_terminal_cancelled"
)

type Queue struct {
	store *Store
	opts  QueueOptions
	seq   uint64
}

type SubmitJobInput struct {
	RequestID      RequestID
	JobID          string
	JobType        JobType
	IdempotencyKey string
	Actor          string
	Payload        map[string]any
}

type QueueClaim struct {
	Job            QueueJobState `json:"job"`
	ClaimToken     string        `json:"claim_token"`
	LeaseEpoch     int           `json:"lease_epoch"`
	LeaseExpiresAt string        `json:"lease_expires_at"`
}

type QueueJobState struct {
	JobID             string                 `json:"job_id"`
	JobType           JobType                `json:"job_type"`
	RequestID         string                 `json:"request_id"`
	RequestIDs        []string               `json:"request_ids,omitempty"`
	Status            JobStatus              `json:"status"`
	IdempotencyKey    string                 `json:"idempotency_key,omitempty"`
	Attempt           int                    `json:"attempt"`
	MaxAttempts       int                    `json:"max_attempts"`
	ClaimToken        string                 `json:"claim_token,omitempty"`
	LeaseEpoch        int                    `json:"lease_epoch,omitempty"`
	LeaseExpiresAt    string                 `json:"lease_expires_at,omitempty"`
	RetryExhausted    bool                   `json:"retry_exhausted,omitempty"`
	Failure           *JobFailure            `json:"failure,omitempty"`
	Artifacts         map[string]string      `json:"artifacts,omitempty"`
	ArtifactRefs      map[string]ArtifactRef `json:"artifact_refs,omitempty"`
	ProjectionTargets []ProjectionName       `json:"projection_targets,omitempty"`
	Payload           map[string]any         `json:"payload,omitempty"`
	LastEventID       string                 `json:"last_event_id,omitempty"`
	CreatedAt         string                 `json:"created_at,omitempty"`
	UpdatedAt         string                 `json:"updated_at,omitempty"`
}

type QueueSnapshot struct {
	Jobs        []QueueJobState `json:"jobs"`
	LastEventID string          `json:"last_event_id,omitempty"`
}

type HeartbeatInput struct {
	JobID        string
	RequestID    RequestID
	ClaimToken   string
	LeaseEpoch   int
	Actor        string
	Artifacts    map[string]string
	ArtifactRefs map[string]ArtifactRef
}

type CompleteJobInput struct {
	JobID        string
	RequestID    RequestID
	ClaimToken   string
	LeaseEpoch   int
	Actor        string
	Artifacts    map[string]string
	ArtifactRefs map[string]ArtifactRef
}

type FailJobInput struct {
	JobID        string
	RequestID    RequestID
	ClaimToken   string
	LeaseEpoch   int
	Actor        string
	Failure      JobFailure
	Artifacts    map[string]string
	ArtifactRefs map[string]ArtifactRef
}

// CancelJobInput identifies a job cancellation request.
type CancelJobInput struct {
	JobID     string
	RequestID RequestID
	Actor     string
	Reason    string
}

// CancelJobResult returns the job state and cancellation outcome.
type CancelJobResult struct {
	Job     QueueJobState    `json:"job"`
	Outcome CancelJobOutcome `json:"outcome"`
}

// NewQueue constructs a Queue bound to store with the given options.
//
// HTTP handlers must call NewQueue per request rather than reusing a shared
// instance: the returned Queue carries an in-memory sequence counter
// initialized from the durable store, and opts may be request-scoped (Now,
// failpoints, actor). The store itself is shared and concurrency-safe; the
// Queue wrapper is the per-request boundary. See cli/internal/daemon/server.go
// handlers for the canonical pattern.
func NewQueue(store *Store, opts QueueOptions) *Queue {
	if opts.LeaseDuration <= 0 {
		opts.LeaseDuration = time.Minute
	}
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = 3
	}
	if opts.Actor == "" {
		opts.Actor = "agentopsd"
	}
	if opts.Now == nil {
		opts.Now = func() time.Time { return time.Now().UTC() }
	}
	return &Queue{store: store, opts: opts, seq: initialQueueSequence(store)}
}

func (q *Queue) SubmitJob(input SubmitJobInput, opts QueueMutationOptions) (QueueJobState, error) {
	if err := ValidateJobType(input.JobType); err != nil {
		return QueueJobState{}, err
	}
	if strings.TrimSpace(input.JobID) == "" {
		input.JobID = q.nextID("job", string(input.JobType))
	}
	if input.JobID == scheduleSentinelJobID {
		return QueueJobState{}, fmt.Errorf("job_id %q is reserved", scheduleSentinelJobID)
	}
	if input.RequestID == "" {
		input.RequestID = RequestID(q.nextID("req", input.JobID))
	}
	// Best-effort idempotency pre-check (fast path).
	//
	// This snapshot scan runs OUTSIDE Store.AppendLedgerEvent's mutex, so it
	// can be stale under concurrent submission. The authoritative dedup is in
	// Store.AppendLedgerEvent which scans for an existing JobAccepted event
	// with the same IdempotencyKey under s.mu and returns the existing event
	// when found — see TestSubmitJob_DedupRelyOnStoreLock. The pre-check
	// avoids a redundant ledger replay in the common single-submitter case.
	snapshot, err := q.Snapshot()
	if err != nil {
		return QueueJobState{}, err
	}
	for _, job := range snapshot.Jobs {
		if input.IdempotencyKey != "" && job.IdempotencyKey == input.IdempotencyKey {
			return job, nil
		}
		if job.JobID == input.JobID {
			return job, nil
		}
	}

	payload := map[string]any{
		"job_payload":        clonePayload(input.Payload),
		"max_attempts":       q.opts.MaxAttempts,
		"projection_targets": projectionTargetStrings(defaultProjectionTargetsForJobType(input.JobType)),
	}
	if input.IdempotencyKey != "" {
		payload["idempotency_key"] = input.IdempotencyKey
	}
	event, err := NewLedgerEvent(LedgerEventInput{
		EventID:    q.nextEventID(EventJobAccepted, input.JobID),
		RequestID:  input.RequestID,
		JobID:      input.JobID,
		EventType:  EventJobAccepted,
		OccurredAt: q.now(),
		Actor:      q.actor(input.Actor),
		JobType:    input.JobType,
		Payload:    payload,
	})
	if err != nil {
		return QueueJobState{}, err
	}
	committed, err := q.appendQueueEvent(event, opts)
	if err != nil {
		return QueueJobState{}, err
	}
	// committed.JobID may differ from input.JobID when Store.AppendLedgerEvent
	// dedup'd by IdempotencyKey (concurrent same-key submission). Use the
	// durable event's JobID for the post-write snapshot lookup so the caller
	// receives the existing job, not a "not found" for the never-written one.
	snapshot, err = q.Snapshot()
	if err != nil {
		return QueueJobState{}, err
	}
	return snapshot.jobByID(committed.JobID)
}

func (q *Queue) ClaimNext(actor string, opts QueueMutationOptions) (QueueClaim, error) {
	return q.ClaimNextMatching(actor, nil, opts)
}

// ClaimNextMatching claims the next claimable job accepted by match.
func (q *Queue) ClaimNextMatching(actor string, match func(QueueJobState) bool, opts QueueMutationOptions) (QueueClaim, error) {
	snapshot, err := q.Snapshot()
	if err != nil {
		return QueueClaim{}, err
	}
	for _, job := range snapshot.Jobs {
		if !q.isClaimable(job) {
			continue
		}
		if match != nil && !match(job) {
			continue
		}
		return q.claimJobState(job, actor, opts)
	}
	return QueueClaim{}, ErrNoClaimableJobs
}

func (q *Queue) ClaimJob(jobID, actor string, opts QueueMutationOptions) (QueueClaim, error) {
	snapshot, err := q.Snapshot()
	if err != nil {
		return QueueClaim{}, err
	}
	job, err := snapshot.jobByID(jobID)
	if err != nil {
		return QueueClaim{}, err
	}
	if isTerminalStatus(job.Status) {
		return QueueClaim{}, ErrNoClaimableJobs
	}
	// Advisory lock-free lease check; authoritative serialization happens in claimJobState via the ledger append.
	if job.Status == JobStatusRunning && !q.jobLeaseExpired(job) {
		return QueueClaim{}, ErrJobAlreadyClaimed
	}
	if !q.isClaimable(job) {
		return QueueClaim{}, ErrNoClaimableJobs
	}
	return q.claimJobState(job, actor, opts)
}

func (q *Queue) Heartbeat(input HeartbeatInput, opts QueueMutationOptions) (QueueJobState, error) {
	job, err := q.assertLiveClaim(input.JobID, input.ClaimToken, input.LeaseEpoch)
	if err != nil {
		return QueueJobState{}, err
	}
	if err := validateArtifactRefs(input.ArtifactRefs); err != nil {
		return QueueJobState{}, err
	}
	expiresAt := q.now().Add(q.opts.LeaseDuration).UTC()
	event, err := NewLedgerEvent(LedgerEventInput{
		EventID:    q.nextEventID(EventJobHeartbeat, input.JobID),
		RequestID:  q.requestID(input.RequestID, job.RequestID),
		JobID:      input.JobID,
		EventType:  EventJobHeartbeat,
		OccurredAt: q.now(),
		Actor:      q.actor(input.Actor),
		Payload: jobMutationPayload(map[string]any{
			"claim_token":      input.ClaimToken,
			"lease_epoch":      input.LeaseEpoch,
			"lease_expires_at": expiresAt.Format(time.RFC3339Nano),
		}, input.Artifacts, input.ArtifactRefs),
	})
	if err != nil {
		return QueueJobState{}, err
	}
	if _, err := q.appendQueueEvent(event, opts); err != nil {
		return QueueJobState{}, err
	}
	return q.currentJob(input.JobID)
}

func (q *Queue) CompleteJob(input CompleteJobInput, opts QueueMutationOptions) (QueueJobState, error) {
	job, err := q.terminalClaimJob(input.JobID, input.ClaimToken, input.LeaseEpoch)
	if err != nil {
		return QueueJobState{}, err
	}
	if err := validateArtifactRefs(input.ArtifactRefs); err != nil {
		return QueueJobState{}, err
	}
	if isTerminalStatus(job.Status) {
		return job, nil
	}
	event, err := NewLedgerEvent(LedgerEventInput{
		EventID:    q.nextEventID(EventJobCompleted, input.JobID),
		RequestID:  q.requestID(input.RequestID, job.RequestID),
		JobID:      input.JobID,
		EventType:  EventJobCompleted,
		OccurredAt: q.now(),
		Actor:      q.actor(input.Actor),
		Payload: jobMutationPayload(map[string]any{
			"claim_token":   input.ClaimToken,
			"lease_epoch":   input.LeaseEpoch,
			"result_status": string(JobResultSucceeded),
		}, input.Artifacts, input.ArtifactRefs),
	})
	if err != nil {
		return QueueJobState{}, err
	}
	if _, err := q.appendQueueEvent(event, opts); err != nil {
		return QueueJobState{}, err
	}
	return q.currentJob(input.JobID)
}

func (q *Queue) FailJob(input FailJobInput, opts QueueMutationOptions) (QueueJobState, error) {
	job, err := q.terminalClaimJob(input.JobID, input.ClaimToken, input.LeaseEpoch)
	if err != nil {
		return QueueJobState{}, err
	}
	if err := validateArtifactRefs(input.ArtifactRefs); err != nil {
		return QueueJobState{}, err
	}
	if isTerminalStatus(job.Status) {
		return job, nil
	}
	if input.Failure.Code == "" {
		input.Failure.Code = FailureRequestRejected
	}
	if err := ValidateFailureCode(input.Failure.Code); err != nil {
		return QueueJobState{}, err
	}
	event, err := NewLedgerEvent(LedgerEventInput{
		EventID:    q.nextEventID(EventJobFailed, input.JobID),
		RequestID:  q.requestID(input.RequestID, job.RequestID),
		JobID:      input.JobID,
		EventType:  EventJobFailed,
		OccurredAt: q.now(),
		Actor:      q.actor(input.Actor),
		Payload: jobMutationPayload(map[string]any{
			"claim_token": input.ClaimToken,
			"lease_epoch": input.LeaseEpoch,
			"failure": map[string]any{
				"code":      string(input.Failure.Code),
				"message":   input.Failure.Message,
				"retryable": input.Failure.Retryable,
			},
		}, input.Artifacts, input.ArtifactRefs),
	})
	if err != nil {
		return QueueJobState{}, err
	}
	if _, err := q.appendQueueEvent(event, opts); err != nil {
		return QueueJobState{}, err
	}
	return q.currentJob(input.JobID)
}

// CancelJob appends a terminal cancellation event for a non-terminal job.
func (q *Queue) CancelJob(input CancelJobInput, opts QueueMutationOptions) (CancelJobResult, error) {
	job, err := q.currentJob(input.JobID)
	if err != nil {
		return CancelJobResult{}, err
	}
	if isTerminalStatus(job.Status) {
		return CancelJobResult{Job: job, Outcome: cancelJobTerminalOutcome(job.Status)}, nil
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		reason = "cancelled"
	}
	event, err := NewLedgerEvent(LedgerEventInput{
		EventID:    q.nextEventID(EventJobCancelled, input.JobID),
		RequestID:  q.requestID(input.RequestID, job.RequestID),
		JobID:      input.JobID,
		EventType:  EventJobCancelled,
		OccurredAt: q.now(),
		Actor:      q.actor(input.Actor),
		Payload: map[string]any{
			"result_status": string(JobResultCancelled),
			"reason":        reason,
		},
	})
	if err != nil {
		return CancelJobResult{}, err
	}
	if _, err := q.appendQueueEvent(event, opts); err != nil {
		return CancelJobResult{}, err
	}
	updated, err := q.currentJob(input.JobID)
	if err != nil {
		return CancelJobResult{}, err
	}
	return CancelJobResult{Job: updated, Outcome: CancelJobOutcomeCancelled}, nil
}

func (q *Queue) Snapshot() (QueueSnapshot, error) {
	events, err := q.store.ReadLedger()
	if err != nil {
		return QueueSnapshot{}, err
	}
	return q.snapshotFromEvents(events)
}

func (q *Queue) claimJobState(job QueueJobState, actor string, opts QueueMutationOptions) (QueueClaim, error) {
	if (job.Status == JobStatusRetryWaiting || job.RetryExhausted) && job.Attempt >= job.MaxAttempts {
		if err := q.appendRetryExhausted(job, actor); err != nil {
			return QueueClaim{}, err
		}
		return QueueClaim{}, ErrNoClaimableJobs
	}
	// Advisory lock-free lease check; the appendLeaseExpired/appendRetryExhausted calls below serialize via the ledger.
	if job.Status == JobStatusRunning && q.jobLeaseExpired(job) {
		if job.Attempt >= job.MaxAttempts {
			if err := q.appendRetryExhausted(job, actor); err != nil {
				return QueueClaim{}, err
			}
			return QueueClaim{}, ErrNoClaimableJobs
		}
		if err := q.appendLeaseExpired(job, actor); err != nil {
			return QueueClaim{}, err
		}
		job.Status = JobStatusRetryWaiting
	}

	epoch := job.LeaseEpoch + 1
	attempt := job.Attempt + 1
	expiresAt := q.now().Add(q.opts.LeaseDuration).UTC()
	claimToken := q.nextID("claim", fmt.Sprintf("%s-%d", job.JobID, epoch))
	event, err := NewLedgerEvent(LedgerEventInput{
		EventID:    q.nextEventID(EventJobClaimed, job.JobID),
		RequestID:  RequestID(job.RequestID),
		JobID:      job.JobID,
		EventType:  EventJobClaimed,
		OccurredAt: q.now(),
		Actor:      q.actor(actor),
		Payload: map[string]any{
			"claim_token":      claimToken,
			"lease_epoch":      epoch,
			"lease_expires_at": expiresAt.Format(time.RFC3339Nano),
			"attempt":          attempt,
		},
	})
	if err != nil {
		return QueueClaim{}, err
	}
	if _, err := q.appendQueueEvent(event, opts); err != nil {
		return QueueClaim{}, err
	}
	updated, err := q.currentJob(job.JobID)
	if err != nil {
		return QueueClaim{}, err
	}
	return QueueClaim{
		Job:            updated,
		ClaimToken:     claimToken,
		LeaseEpoch:     epoch,
		LeaseExpiresAt: expiresAt.Format(time.RFC3339Nano),
	}, nil
}

func (q *Queue) appendLeaseExpired(job QueueJobState, actor string) error {
	event, err := NewLedgerEvent(LedgerEventInput{
		EventID:    q.nextEventID(EventJobLeaseExpired, job.JobID),
		RequestID:  RequestID(job.RequestID),
		JobID:      job.JobID,
		EventType:  EventJobLeaseExpired,
		OccurredAt: q.now(),
		Actor:      q.actor(actor),
		Payload: map[string]any{
			"claim_token": job.ClaimToken,
			"lease_epoch": job.LeaseEpoch,
		},
	})
	if err != nil {
		return err
	}
	_, err = q.appendQueueEvent(event, QueueMutationOptions{})
	return err
}

func (q *Queue) appendRetryExhausted(job QueueJobState, actor string) error {
	event, err := NewLedgerEvent(LedgerEventInput{
		EventID:    q.nextEventID(EventJobFailed, job.JobID),
		RequestID:  RequestID(job.RequestID),
		JobID:      job.JobID,
		EventType:  EventJobFailed,
		OccurredAt: q.now(),
		Actor:      q.actor(actor),
		Payload: map[string]any{
			"failure": map[string]any{
				"code":      string(FailureRetryExhausted),
				"message":   "lease expired after retry cap",
				"retryable": false,
			},
		},
	})
	if err != nil {
		return err
	}
	_, err = q.appendQueueEvent(event, QueueMutationOptions{})
	return err
}

func (q *Queue) assertLiveClaim(jobID, claimToken string, leaseEpoch int) (QueueJobState, error) {
	job, err := q.currentJob(jobID)
	if err != nil {
		return QueueJobState{}, err
	}
	if isTerminalStatus(job.Status) {
		return QueueJobState{}, ErrNoClaimableJobs
	}
	if job.ClaimToken != claimToken || job.LeaseEpoch != leaseEpoch {
		return QueueJobState{}, ErrClaimFenceMismatch
	}
	// Advisory lock-free lease check; ledger append at the call site provides the authoritative ordering.
	if q.jobLeaseExpired(job) {
		return QueueJobState{}, ErrLeaseExpired
	}
	return job, nil
}

func (q *Queue) terminalClaimJob(jobID, claimToken string, leaseEpoch int) (QueueJobState, error) {
	job, err := q.currentJob(jobID)
	if err != nil {
		return QueueJobState{}, err
	}
	if isTerminalStatus(job.Status) {
		return job, nil
	}
	if job.ClaimToken != claimToken || job.LeaseEpoch != leaseEpoch {
		return QueueJobState{}, ErrClaimFenceMismatch
	}
	// Advisory lock-free lease check; ledger append at the call site provides the authoritative ordering.
	if q.jobLeaseExpired(job) {
		return QueueJobState{}, ErrLeaseExpired
	}
	return job, nil
}

// appendQueueEvent appends event to the ledger and returns the durable event
// (which may differ from the input when the store dedups by EventID or by
// IdempotencyKey for JobAccepted events — see Store.AppendLedgerEvent).
// Callers that need to know which event actually committed (e.g. SubmitJob
// resolving the post-dedup JobID) must inspect the returned event.
func (q *Queue) appendQueueEvent(event LedgerEvent, opts QueueMutationOptions) (LedgerEvent, error) {
	if opts.Failpoint == QueueFailpointBeforeAppend {
		return LedgerEvent{}, fmt.Errorf("%w: %s", ErrFailpoint, QueueFailpointBeforeAppend)
	}
	committed, err := q.store.AppendLedgerEvent(event)
	if err != nil {
		return LedgerEvent{}, err
	}
	if opts.Failpoint == QueueFailpointAfterAppendBeforeAck {
		return LedgerEvent{}, fmt.Errorf("%w: %s", ErrFailpoint, QueueFailpointAfterAppendBeforeAck)
	}
	return committed, nil
}

func (q *Queue) snapshotFromEvents(events []LedgerEvent) (QueueSnapshot, error) {
	snapshot := QueueSnapshot{}
	jobsByID := map[string]*QueueJobState{}
	var order []string
	for _, event := range events {
		if err := ValidateLedgerEvent(event); err != nil {
			return QueueSnapshot{}, err
		}
		snapshot.LastEventID = event.EventID
		if event.EventType == EventProjectionMarkedStale || event.EventType == EventProjectionRebuilt {
			continue
		}
		job := jobsByID[event.JobID]
		if job == nil {
			job = &QueueJobState{
				JobID:        event.JobID,
				RequestID:    event.RequestID,
				RequestIDs:   []string{event.RequestID},
				Status:       JobStatusQueued,
				MaxAttempts:  q.opts.MaxAttempts,
				Artifacts:    map[string]string{},
				ArtifactRefs: map[string]ArtifactRef{},
			}
			jobsByID[event.JobID] = job
			order = append(order, event.JobID)
		}
		if q.applyQueueEvent(job, event) {
			appendQueueRequestID(job, event.RequestID)
			job.LastEventID = event.EventID
			job.UpdatedAt = event.OccurredAt
			if job.CreatedAt == "" && event.EventType == EventJobAccepted {
				job.CreatedAt = event.OccurredAt
			}
		}
	}
	for _, jobID := range order {
		job := *jobsByID[jobID]
		if len(job.Artifacts) == 0 {
			job.Artifacts = nil
		}
		if len(job.ArtifactRefs) == 0 {
			job.ArtifactRefs = nil
		}
		// Advisory lock-free lease check used to surface lease-expired status in snapshots; not authoritative.
		if job.Status == JobStatusRunning && q.jobLeaseExpired(job) {
			job.Status = JobStatusRetryWaiting
			if job.Attempt >= job.MaxAttempts {
				job.RetryExhausted = true
			}
		}
		snapshot.Jobs = append(snapshot.Jobs, job)
	}
	return snapshot, nil
}

// applyQueueEvent folds a single ledger event into job's in-memory state and
// returns whether the job was mutated.
//
// Cyclomatic complexity is in the warn band (currently ~22; CI fail threshold
// is 25 per scripts/check-go-complexity.sh). Adding another event-type branch
// pushes this past the threshold — refactor by extracting per-event handlers
// or a dispatch table before adding a new case.
func (q *Queue) applyQueueEvent(job *QueueJobState, event LedgerEvent) bool {
	if isTerminalStatus(job.Status) {
		return false
	}
	applyQueueEventMetadata(job, event)
	applyQueueEventArtifacts(job, event)
	applyQueueEventStatus(job, event)
	return true
}

func applyQueueEventMetadata(job *QueueJobState, event LedgerEvent) {
	if jobType, ok, err := jobTypeFromPayload(event.Payload); err == nil && ok {
		job.JobType = jobType
	}
	if targets := projectionTargetsFromPayload(event.Payload); len(targets) > 0 {
		job.ProjectionTargets = targets
	} else if len(job.ProjectionTargets) == 0 && job.JobType != "" {
		job.ProjectionTargets = defaultProjectionTargetsForJobType(job.JobType)
	}
	if key, ok := stringPayload(event.Payload, "idempotency_key"); ok {
		job.IdempotencyKey = key
	}
	if payload, ok := event.Payload["job_payload"].(map[string]any); ok {
		job.Payload = clonePayload(payload)
	}
	if maxAttempts, ok := intPayload(event.Payload, "max_attempts"); ok {
		job.MaxAttempts = maxAttempts
	}
}

func applyQueueEventArtifacts(job *QueueJobState, event LedgerEvent) {
	for key, value := range artifactsFromPayload(event.Payload) {
		job.Artifacts[key] = value
	}
	for key, ref := range artifactRefsFromPayload(event.Payload) {
		if job.ArtifactRefs == nil {
			job.ArtifactRefs = map[string]ArtifactRef{}
		}
		job.ArtifactRefs[key] = ref
		if ref.Path != "" {
			job.Artifacts[key] = ref.Path
		}
	}
}

func applyQueueEventStatus(job *QueueJobState, event LedgerEvent) {
	switch event.EventType {
	case EventJobAccepted:
		job.Status = JobStatusQueued
	case EventJobClaimed:
		applyJobClaimedEvent(job, event)
	case EventJobHeartbeat:
		applyJobHeartbeatEvent(job, event)
	case EventJobLeaseExpired:
		job.Status = JobStatusRetryWaiting
		job.ClaimToken = ""
	case EventJobCompleted:
		job.Status = JobStatusCompleted
	case EventJobFailed:
		job.Status = JobStatusFailed
		failure := failureFromPayload(event.Payload)
		job.Failure = &failure
	case EventJobCancelled:
		job.Status = JobStatusCancelled
	}
}

func applyJobClaimedEvent(job *QueueJobState, event LedgerEvent) {
	job.Status = JobStatusRunning
	job.RetryExhausted = false
	job.ClaimToken, _ = stringPayload(event.Payload, "claim_token")
	job.LeaseEpoch, _ = intPayload(event.Payload, "lease_epoch")
	job.LeaseExpiresAt, _ = stringPayload(event.Payload, "lease_expires_at")
	if attempt, ok := intPayload(event.Payload, "attempt"); ok {
		job.Attempt = attempt
	}
}

func applyJobHeartbeatEvent(job *QueueJobState, event LedgerEvent) {
	job.Status = JobStatusRunning
	if token, ok := stringPayload(event.Payload, "claim_token"); ok {
		job.ClaimToken = token
	}
	if epoch, ok := intPayload(event.Payload, "lease_epoch"); ok {
		job.LeaseEpoch = epoch
	}
	if expiresAt, ok := stringPayload(event.Payload, "lease_expires_at"); ok {
		job.LeaseExpiresAt = expiresAt
	}
}

func (q *Queue) isClaimable(job QueueJobState) bool {
	if isTerminalStatus(job.Status) {
		return false
	}
	if job.Status == JobStatusRunning {
		// Advisory lock-free lease check; the real claim attempt re-validates via the ledger.
		return q.jobLeaseExpired(job)
	}
	return job.Status == JobStatusQueued || job.Status == JobStatusRetryWaiting
}

// jobLeaseExpired reports whether job's lease has elapsed against the
// queue's clock. The read is intentionally lock-free: callers use this as
// an advisory pre-check (e.g. should we attempt a re-claim?), and the
// authoritative serialization happens inside Store.AppendLedgerEvent.
// Callers that need strict ordering must follow up with a transactional
// claim attempt — see assertLiveClaim and claimJobState.
func (q *Queue) jobLeaseExpired(job QueueJobState) bool {
	if job.LeaseExpiresAt == "" {
		return false
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, job.LeaseExpiresAt)
	if err != nil {
		return true
	}
	return !q.now().Before(expiresAt)
}

func (q *Queue) currentJob(jobID string) (QueueJobState, error) {
	snapshot, err := q.Snapshot()
	if err != nil {
		return QueueJobState{}, err
	}
	return snapshot.jobByID(jobID)
}

func (snapshot QueueSnapshot) jobByID(jobID string) (QueueJobState, error) {
	for _, job := range snapshot.Jobs {
		if job.JobID == jobID {
			return job, nil
		}
	}
	return QueueJobState{}, ErrJobNotFound
}

func (q *Queue) nextEventID(eventType EventType, jobID string) string {
	return q.nextID("evt_"+sanitizeIDPart(string(eventType)), jobID)
}

func jobMutationPayload(base map[string]any, artifacts map[string]string, refs map[string]ArtifactRef) map[string]any {
	if len(artifacts) > 0 {
		base["artifacts"] = artifacts
	}
	if len(refs) > 0 {
		base["artifact_refs"] = refs
	}
	return base
}

func (q *Queue) nextID(prefix, seed string) string {
	seq := atomic.AddUint64(&q.seq, 1)
	seed = sanitizeIDPart(seed)
	if seed == "" {
		seed = "daemon"
	}
	return fmt.Sprintf("%s_%s_%06d", prefix, seed, seq)
}

func (q *Queue) now() time.Time {
	return q.opts.Now().UTC()
}

func (q *Queue) actor(actor string) string {
	if strings.TrimSpace(actor) != "" {
		return strings.TrimSpace(actor)
	}
	return q.opts.Actor
}

func (q *Queue) requestID(input RequestID, fallback string) RequestID {
	if strings.TrimSpace(string(input)) != "" {
		return input
	}
	return RequestID(fallback)
}

func initialQueueSequence(store *Store) uint64 {
	if store == nil {
		return 0
	}
	replay, err := store.ReplayLedgerReadOnly()
	if err != nil {
		return 0
	}
	return uint64(len(replay.Events))
}

func cancelJobTerminalOutcome(status JobStatus) CancelJobOutcome {
	switch status {
	case JobStatusCompleted:
		return CancelJobOutcomeAlreadyTerminalCompleted
	case JobStatusFailed:
		return CancelJobOutcomeAlreadyTerminalFailed
	case JobStatusCancelled:
		return CancelJobOutcomeAlreadyTerminalCancelled
	default:
		return CancelJobOutcomeCancelled
	}
}

func appendQueueRequestID(job *QueueJobState, requestID string) {
	for _, existing := range job.RequestIDs {
		if existing == requestID {
			return
		}
	}
	job.RequestIDs = append(job.RequestIDs, requestID)
}

func stringPayload(payload map[string]any, key string) (string, bool) {
	raw, ok := payload[key]
	if !ok {
		return "", false
	}
	value, ok := raw.(string)
	return value, ok && value != ""
}

func intPayload(payload map[string]any, key string) (int, bool) {
	raw, ok := payload[key]
	if !ok {
		return 0, false
	}
	switch value := raw.(type) {
	case int:
		return value, true
	case int64:
		return int(value), true
	case float64:
		return int(value), true
	default:
		return 0, false
	}
}

func sanitizeIDPart(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, ".", "_")
	value = strings.ReplaceAll(value, "/", "_")
	value = strings.ReplaceAll(value, " ", "_")
	value = strings.ReplaceAll(value, ":", "_")
	return value
}
