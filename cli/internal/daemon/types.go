package daemon

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

type JobType string

const (
	JobTypeRPIRun           JobType = "rpi.run"
	JobTypeRPIPhase         JobType = "rpi.phase"
	JobTypeDreamRun         JobType = "dream.run"
	JobTypeDreamStage       JobType = "dream.stage"
	JobTypeWikiBuild        JobType = "wiki.build"
	JobTypeWikiForge        JobType = "wiki.forge"
	JobTypeOpenClawSnapshot JobType = "openclaw.snapshot"
	JobTypePlansProjection  JobType = "plans.projection"
	// JobTypeLLMWikiLoop is the Karpathy-pattern external-knowledge loop job type.
	// Operates on raw/ + wiki/ trees, distinct from internal .agents/ work.
	JobTypeLLMWikiLoop      JobType = "llmwiki.loop"
	JobTypeEvalSuite        JobType = "eval.suite"
	JobTypeEvalSkillDelta   JobType = "eval.skill-delta"
	JobTypeSkillInvoke      JobType = "skill.invoke"
)

// RecurringJobTemplate is a schedule entry that materializes a Job on each cron tick.
type RecurringJobTemplate struct {
	Name         string                 `json:"name"`
	Cron         string                 `json:"cron"`
	JobType      JobType                `json:"job_type"`
	Payload      json.RawMessage        `json:"payload,omitempty"`
	Timeout      time.Duration          `json:"timeout,omitempty"`
	Backpressure RecurrenceBackpressure `json:"backpressure"`
}

// RecurrenceBackpressure controls how the supervisor handles in-flight schedules.
type RecurrenceBackpressure struct {
	SkipIfRunning bool `json:"skip_if_running"`
	MaxQueueDepth int  `json:"max_queue_depth"`
}

// CronParseError preserves the operator's original input for actionable errors.
type CronParseError struct {
	Original string
	Reason   string
}

func (e *CronParseError) Error() string {
	return fmt.Sprintf("invalid cron %q: %s", e.Original, e.Reason)
}

// ParseCron validates a cron expression using the 5-field standard with descriptors
// (e.g., "0 3 * * *", "@daily"). 6-field expressions with seconds are rejected per
// pre-mortem amendment B4 (DoS protection: prevents sub-minute schedules).
func ParseCron(expr string) (cron.Schedule, error) {
	parser := cron.NewParser(
		cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
	)
	sched, err := parser.Parse(expr)
	if err != nil {
		return nil, &CronParseError{Original: expr, Reason: err.Error()}
	}
	return sched, nil
}

type EventType string

const (
	EventJobAccepted           EventType = "job.accepted"
	EventJobClaimed            EventType = "job.claimed"
	EventJobHeartbeat          EventType = "job.heartbeat"
	EventJobLeaseExpired       EventType = "job.lease_expired"
	EventJobCompleted          EventType = "job.completed"
	EventJobFailed             EventType = "job.failed"
	EventJobCancelled          EventType = "job.cancelled"
	EventProjectionMarkedStale EventType = "projection.marked_stale"
	EventProjectionRebuilt     EventType = "projection.rebuilt"

	EventFactoryJobSubmitted        EventType = "factory.job_submitted"
	EventFactoryJobClaimed          EventType = "factory.job_claimed"
	EventFactoryJobStarted          EventType = "factory.job_started"
	EventFactoryRoutingDecided      EventType = "factory.routing_decided"
	EventFactorySlotAllocated       EventType = "factory.slot_allocated"
	EventFactoryWorktreeAllocated   EventType = "factory.worktree_allocated"
	EventFactoryValidationStarted   EventType = "factory.validation_started"
	EventFactoryValidationCompleted EventType = "factory.validation_completed"
	EventFactoryMergeDecision       EventType = "factory.merge_decision"
	EventFactoryJobTerminal         EventType = "factory.job_terminal"
	EventFactoryYieldObservation    EventType = "factory.yield_observation"
)

type JobStatus string

const (
	JobStatusQueued       JobStatus = "queued"
	JobStatusRunning      JobStatus = "running"
	JobStatusRetryWaiting JobStatus = "retry_waiting"
	JobStatusCompleted    JobStatus = "completed"
	JobStatusFailed       JobStatus = "failed"
	JobStatusCancelled    JobStatus = "cancelled"
	JobStatusDegraded     JobStatus = "degraded"
)

type FactoryJobStatus string

const (
	FactoryJobStatusSubmitted           FactoryJobStatus = "submitted"
	FactoryJobStatusClaimed             FactoryJobStatus = "claimed"
	FactoryJobStatusStarted             FactoryJobStatus = "started"
	FactoryJobStatusRouted              FactoryJobStatus = "routed"
	FactoryJobStatusAllocated           FactoryJobStatus = "allocated"
	FactoryJobStatusValidating          FactoryJobStatus = "validating"
	FactoryJobStatusValidated           FactoryJobStatus = "validated"
	FactoryJobStatusValidationFailed    FactoryJobStatus = "validation_failed"
	FactoryJobStatusAwaitingManualMerge FactoryJobStatus = "awaiting_manual_merge"
	FactoryJobStatusTerminal            FactoryJobStatus = "terminal"
	FactoryJobStatusRetainedFailed      FactoryJobStatus = "retained_failed"
)

type FactorySlotStatus string

const (
	FactorySlotStatusIdle                FactorySlotStatus = "idle"
	FactorySlotStatusAllocated           FactorySlotStatus = "allocated"
	FactorySlotStatusRunning             FactorySlotStatus = "running"
	FactorySlotStatusBlockedValidation   FactorySlotStatus = "blocked_validation"
	FactorySlotStatusAwaitingManualMerge FactorySlotStatus = "awaiting_manual_merge"
	FactorySlotStatusTerminal            FactorySlotStatus = "terminal"
	FactorySlotStatusRetainedFailed      FactorySlotStatus = "retained_failed"
)

type FactoryValidationStatus string

const (
	FactoryValidationStatusRunning   FactoryValidationStatus = "running"
	FactoryValidationStatusPassed    FactoryValidationStatus = "passed"
	FactoryValidationStatusFailed    FactoryValidationStatus = "failed"
	FactoryValidationStatusBlocked   FactoryValidationStatus = "blocked"
	FactoryValidationStatusCancelled FactoryValidationStatus = "cancelled"
)

type FactoryMergeDecision string

const (
	FactoryMergeDecisionNotRequested  FactoryMergeDecision = "not_requested"
	FactoryMergeDecisionManualPending FactoryMergeDecision = "manual_pending"
	FactoryMergeDecisionManualMerged  FactoryMergeDecision = "manual_merged"
	FactoryMergeDecisionRejected      FactoryMergeDecision = "rejected"
	FactoryMergeDecisionAbandoned     FactoryMergeDecision = "abandoned"
)

type JobResultStatus string

const (
	JobResultSucceeded JobResultStatus = "succeeded"
	JobResultFailed    JobResultStatus = "failed"
	JobResultCancelled JobResultStatus = "cancelled"
)

type FailureCode string

const (
	FailureDaemonUnavailable         FailureCode = "daemon_unavailable"
	FailureProviderUnreachable       FailureCode = "provider_unreachable"
	FailureSessionPending            FailureCode = "session_pending"
	FailureSessionLost               FailureCode = "lost"
	FailureEventStreamUnavailable    FailureCode = "event_stream_unavailable"
	FailureTerminalWithoutTranscript FailureCode = "terminal_without_transcript"
	FailureRequestRejected           FailureCode = "request_rejected"
	FailureProjectionDegraded        FailureCode = "projection_degraded"
	FailureRetryExhausted            FailureCode = "retry_exhausted"
)

type LeaseState string

const (
	LeaseNone    LeaseState = "none"
	LeaseFresh   LeaseState = "fresh"
	LeaseExpired LeaseState = "expired"
	LeaseUnknown LeaseState = "unknown"
)

type ProviderStatus string

const (
	ProviderDaemonUnavailable ProviderStatus = "daemon_unavailable"
	ProviderUnreachable       ProviderStatus = "provider_unreachable"
	ProviderSessionPending    ProviderStatus = "session_pending"
	ProviderSessionBound      ProviderStatus = "session_bound"
)

type SnapshotReplayStatus string

const (
	SnapshotReplayComplete SnapshotReplayStatus = "complete"
	SnapshotReplayCorrupt  SnapshotReplayStatus = "corrupt"
)

type SnapshotConsumerResult string

const (
	SnapshotServe              SnapshotConsumerResult = "serve_snapshot"
	SnapshotCompatibilityError SnapshotConsumerResult = "compatibility_error"
	SnapshotProjectionMissing  SnapshotConsumerResult = "projection_missing"
	SnapshotProjectionDegraded SnapshotConsumerResult = "projection_degraded"
)

type JobSpec struct {
	ID        string         `json:"id"`
	Type      JobType        `json:"type"`
	Payload   map[string]any `json:"payload,omitempty"`
	CreatedAt string         `json:"created_at,omitempty"`
}

type JobResult struct {
	Status      JobResultStatus   `json:"status"`
	Artifacts   map[string]string `json:"artifacts,omitempty"`
	CompletedAt string            `json:"completed_at,omitempty"`
}

type JobFailure struct {
	Code      FailureCode `json:"code"`
	Message   string      `json:"message,omitempty"`
	Retryable bool        `json:"retryable,omitempty"`
}

type JobStatusProjectionInput struct {
	TerminalEvent   EventType
	Lease           LeaseState
	ProjectionStale bool
}

type ProviderProjectionInput struct {
	DaemonReady        bool
	GasCityReady       bool
	WorkerSessionKnown bool
}

type SnapshotProjectionInput struct {
	ReplayStatus     SnapshotReplayStatus
	FileExists       bool
	VersionSupported bool
}

func ValidateJobType(value JobType) error {
	return validateStringEnum("job type", string(value), jobTypeSet)
}

func ValidateEventType(value EventType) error {
	return validateStringEnum("event type", string(value), eventTypeSet)
}

func ValidateJobStatus(value JobStatus) error {
	return validateStringEnum("job status", string(value), jobStatusSet)
}

func ValidateFactorySlotStatus(value FactorySlotStatus) error {
	return validateStringEnum("factory slot status", string(value), factorySlotStatusSet)
}

func ValidateFactoryValidationStatus(value FactoryValidationStatus) error {
	return validateStringEnum("factory validation status", string(value), factoryValidationStatusSet)
}

func ValidateFactoryMergeDecision(value FactoryMergeDecision) error {
	return validateStringEnum("factory merge decision", string(value), factoryMergeDecisionSet)
}

func ValidateJobResultStatus(value JobResultStatus) error {
	return validateStringEnum("job result", string(value), jobResultStatusSet)
}

func ValidateFailureCode(value FailureCode) error {
	return validateStringEnum("failure code", string(value), failureCodeSet)
}

func ValidateLeaseState(value LeaseState) error {
	return validateStringEnum("lease state", string(value), leaseStateSet)
}

func ProjectJobStatus(input JobStatusProjectionInput) JobStatus {
	switch input.TerminalEvent {
	case EventJobCompleted:
		return JobStatusCompleted
	case EventJobFailed:
		return JobStatusFailed
	case EventJobCancelled:
		return JobStatusCancelled
	}
	if input.ProjectionStale || input.Lease == LeaseUnknown {
		return JobStatusDegraded
	}
	switch input.Lease {
	case LeaseFresh:
		return JobStatusRunning
	case LeaseExpired:
		return JobStatusRetryWaiting
	default:
		return JobStatusQueued
	}
}

func CanTransitionJobStatus(from, to JobStatus) bool {
	if from == to {
		return true
	}
	if isTerminalStatus(from) {
		return false
	}
	for _, allowed := range allowedJobStatusTransitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}

func ProjectProviderStatus(input ProviderProjectionInput) ProviderStatus {
	if !input.DaemonReady {
		return ProviderDaemonUnavailable
	}
	if !input.GasCityReady {
		return ProviderUnreachable
	}
	if !input.WorkerSessionKnown {
		return ProviderSessionPending
	}
	return ProviderSessionBound
}

func ProjectSnapshotResult(input SnapshotProjectionInput) SnapshotConsumerResult {
	if input.ReplayStatus == SnapshotReplayCorrupt {
		return SnapshotProjectionDegraded
	}
	if !input.FileExists {
		return SnapshotProjectionMissing
	}
	if !input.VersionSupported {
		return SnapshotCompatibilityError
	}
	return SnapshotServe
}

func validateStringEnum(name, value string, allowed map[string]struct{}) error {
	if _, ok := allowed[value]; ok {
		return nil
	}
	return fmt.Errorf("invalid %s %q", name, value)
}

func isTerminalStatus(status JobStatus) bool {
	switch status {
	case JobStatusCompleted, JobStatusFailed, JobStatusCancelled:
		return true
	default:
		return false
	}
}

var jobTypeSet = map[string]struct{}{
	string(JobTypeRPIRun):           {},
	string(JobTypeRPIPhase):         {},
	string(JobTypeDreamRun):         {},
	string(JobTypeDreamStage):       {},
	string(JobTypeWikiBuild):        {},
	string(JobTypeWikiForge):        {},
	string(JobTypeOpenClawSnapshot): {},
	string(JobTypePlansProjection):  {},
	string(JobTypeLLMWikiLoop):      {},
	string(JobTypeEvalSuite):        {},
	string(JobTypeEvalSkillDelta):   {},
	string(JobTypeSkillInvoke):      {},
}

var eventTypeSet = map[string]struct{}{
	string(EventJobAccepted):                {},
	string(EventJobClaimed):                 {},
	string(EventJobHeartbeat):               {},
	string(EventJobLeaseExpired):            {},
	string(EventJobCompleted):               {},
	string(EventJobFailed):                  {},
	string(EventJobCancelled):               {},
	string(EventProjectionMarkedStale):      {},
	string(EventProjectionRebuilt):          {},
	string(EventFactoryJobSubmitted):        {},
	string(EventFactoryJobClaimed):          {},
	string(EventFactoryJobStarted):          {},
	string(EventFactoryRoutingDecided):      {},
	string(EventFactorySlotAllocated):       {},
	string(EventFactoryWorktreeAllocated):   {},
	string(EventFactoryValidationStarted):   {},
	string(EventFactoryValidationCompleted): {},
	string(EventFactoryMergeDecision):       {},
	string(EventFactoryJobTerminal):         {},
	string(EventFactoryYieldObservation):    {},
}

var jobStatusSet = map[string]struct{}{
	string(JobStatusQueued):       {},
	string(JobStatusRunning):      {},
	string(JobStatusRetryWaiting): {},
	string(JobStatusCompleted):    {},
	string(JobStatusFailed):       {},
	string(JobStatusCancelled):    {},
	string(JobStatusDegraded):     {},
}

var factorySlotStatusSet = map[string]struct{}{
	string(FactorySlotStatusIdle):                {},
	string(FactorySlotStatusAllocated):           {},
	string(FactorySlotStatusRunning):             {},
	string(FactorySlotStatusBlockedValidation):   {},
	string(FactorySlotStatusAwaitingManualMerge): {},
	string(FactorySlotStatusTerminal):            {},
	string(FactorySlotStatusRetainedFailed):      {},
}

var factoryValidationStatusSet = map[string]struct{}{
	string(FactoryValidationStatusRunning):   {},
	string(FactoryValidationStatusPassed):    {},
	string(FactoryValidationStatusFailed):    {},
	string(FactoryValidationStatusBlocked):   {},
	string(FactoryValidationStatusCancelled): {},
}

var factoryMergeDecisionSet = map[string]struct{}{
	string(FactoryMergeDecisionNotRequested):  {},
	string(FactoryMergeDecisionManualPending): {},
	string(FactoryMergeDecisionManualMerged):  {},
	string(FactoryMergeDecisionRejected):      {},
	string(FactoryMergeDecisionAbandoned):     {},
}

var jobResultStatusSet = map[string]struct{}{
	string(JobResultSucceeded): {},
	string(JobResultFailed):    {},
	string(JobResultCancelled): {},
}

var failureCodeSet = map[string]struct{}{
	string(FailureDaemonUnavailable):         {},
	string(FailureProviderUnreachable):       {},
	string(FailureSessionPending):            {},
	string(FailureSessionLost):               {},
	string(FailureEventStreamUnavailable):    {},
	string(FailureTerminalWithoutTranscript): {},
	string(FailureRequestRejected):           {},
	string(FailureProjectionDegraded):        {},
	string(FailureRetryExhausted):            {},
}

var leaseStateSet = map[string]struct{}{
	string(LeaseNone):    {},
	string(LeaseFresh):   {},
	string(LeaseExpired): {},
	string(LeaseUnknown): {},
}

var allowedJobStatusTransitions = map[JobStatus][]JobStatus{
	JobStatusQueued:       {JobStatusRunning, JobStatusCancelled, JobStatusFailed, JobStatusDegraded},
	JobStatusRunning:      {JobStatusCompleted, JobStatusFailed, JobStatusCancelled, JobStatusRetryWaiting, JobStatusDegraded},
	JobStatusRetryWaiting: {JobStatusQueued, JobStatusRunning, JobStatusFailed, JobStatusCancelled, JobStatusDegraded},
	JobStatusDegraded:     {JobStatusQueued, JobStatusRunning, JobStatusFailed, JobStatusCancelled},
}
