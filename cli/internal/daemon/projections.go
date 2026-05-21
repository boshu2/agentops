package daemon

import (
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"time"
)

// knownEventTypes is the set of event types the projection reducer knows how
// to fold. Members are registered by file-init() in this package and in
// store.go (schedule.* events). Unknown event types do NOT crash replay —
// see applyEventsToState's forward-compat path (pre-mortem amendment B3).
var knownEventTypes = map[EventType]struct{}{
	EventJobAccepted:                {},
	EventJobClaimed:                 {},
	EventJobHeartbeat:               {},
	EventJobLeaseExpired:            {},
	EventJobCompleted:               {},
	EventJobFailed:                  {},
	EventJobCancelled:               {},
	EventProjectionMarkedStale:      {},
	EventProjectionRebuilt:          {},
	EventFactoryAdmissionDecided:    {},
	EventFactoryJobSubmitted:        {},
	EventFactoryJobClaimed:          {},
	EventFactoryJobStarted:          {},
	EventFactoryRoutingDecided:      {},
	EventFactorySlotAllocated:       {},
	EventFactoryWorktreeAllocated:   {},
	EventFactoryValidationStarted:   {},
	EventFactoryValidationCompleted: {},
	EventFactoryMergeDecision:       {},
	EventFactoryJobTerminal:         {},
	EventFactoryYieldObservation:    {},
	EventScheduleCreated:            {},
	EventScheduleFired:              {},
	EventScheduleSkipped:            {},
	EventScheduleDeleted:            {},
}

func isScheduleEvent(t EventType) bool {
	switch t {
	case EventScheduleCreated, EventScheduleFired, EventScheduleSkipped, EventScheduleDeleted:
		return true
	}
	return false
}

func isFactoryEvent(t EventType) bool {
	switch t {
	case EventFactoryAdmissionDecided,
		EventFactoryJobSubmitted,
		EventFactoryJobClaimed,
		EventFactoryJobStarted,
		EventFactoryRoutingDecided,
		EventFactorySlotAllocated,
		EventFactoryWorktreeAllocated,
		EventFactoryValidationStarted,
		EventFactoryValidationCompleted,
		EventFactoryMergeDecision,
		EventFactoryJobTerminal,
		EventFactoryYieldObservation:
		return true
	default:
		return false
	}
}

const ProjectionSchemaVersion = 1

type ProjectionName string

const (
	ProjectionRPIRegistry     ProjectionName = "rpi-registry"
	ProjectionDreamRuns       ProjectionName = "dream-runs"
	ProjectionWikiJobs        ProjectionName = "wiki-jobs"
	ProjectionOpenClaw        ProjectionName = "openclaw-snapshot"
	ProjectionDaemonStatus    ProjectionName = "daemon-status"
	ProjectionDaemonJobStatus ProjectionName = "daemon-job-status"
	ProjectionPlansManifest   ProjectionName = "plans-manifest"
)

type ProjectionStatus string

const (
	ProjectionStatusCurrent  ProjectionStatus = "current"
	ProjectionStatusStale    ProjectionStatus = "stale"
	ProjectionStatusDegraded ProjectionStatus = "degraded"
)

type ProjectionRebuildOptions struct {
	RebuiltAt    time.Time
	SourceLedger string
	// FromSnapshot, when non-nil, makes RebuildProjections start from the
	// given snapshot's state and apply only events whose EventID is strictly
	// greater than FromSnapshot.LastEventID. Use this to amortize replay cost
	// across daemon restarts (Phase 2-B of TB-Δ3).
	FromSnapshot *ProjectionSet
}

type ProjectionSet struct {
	SchemaVersion   int                                   `json:"schema_version"`
	RebuiltAt       string                                `json:"rebuilt_at"`
	SourceLedger    string                                `json:"source_ledger"`
	LastEventID     string                                `json:"last_event_id,omitempty"`
	Manifests       map[ProjectionName]ProjectionManifest `json:"manifests"`
	Jobs            []JobProjection                       `json:"jobs"`
	RPI             RPIRegistryProjection                 `json:"rpi"`
	Dream           DreamRunsProjection                   `json:"dream"`
	Wiki            WikiJobsProjection                    `json:"wiki"`
	OpenClaw        OpenClawSnapshotProjection            `json:"openclaw"`
	Plans           DaemonPlansProjection                 `json:"plans"`
	Schedules       []RecurringJobTemplate                `json:"schedules,omitempty"`
	Factory         FactoryStatusProjection               `json:"factory"`
	DegradedReasons []string                              `json:"degraded_reasons,omitempty"`
}

type ProjectionManifest struct {
	SchemaVersion   int              `json:"schema_version"`
	Projection      ProjectionName   `json:"projection"`
	SourceLedger    string           `json:"source_ledger"`
	LastEventID     string           `json:"last_event_id,omitempty"`
	Status          ProjectionStatus `json:"status"`
	RebuiltAt       string           `json:"rebuilt_at"`
	OutputPaths     []string         `json:"output_paths,omitempty"`
	DegradedReasons []string         `json:"degraded_reasons,omitempty"`
}

type JobProjection struct {
	JobID             string                 `json:"job_id"`
	JobType           JobType                `json:"job_type,omitempty"`
	RequestID         string                 `json:"request_id"`
	RequestIDs        []string               `json:"request_ids,omitempty"`
	Status            JobStatus              `json:"status"`
	ResultStatus      JobResultStatus        `json:"result_status,omitempty"`
	Failure           *JobFailure            `json:"failure,omitempty"`
	Artifacts         map[string]string      `json:"artifacts,omitempty"`
	ArtifactRefs      map[string]ArtifactRef `json:"artifact_refs,omitempty"`
	ProjectionTargets []ProjectionName       `json:"projection_targets,omitempty"`
	CreatedAt         string                 `json:"created_at,omitempty"`
	UpdatedAt         string                 `json:"updated_at,omitempty"`
	LastEventID       string                 `json:"last_event_id,omitempty"`
}

type RPIRegistryProjection struct {
	Runs []JobProjection `json:"runs"`
}

type DreamRunsProjection struct {
	Runs []JobProjection `json:"runs"`
}

type WikiJobsProjection struct {
	Jobs []JobProjection `json:"jobs"`
}

type ProjectionSource struct {
	Ledger      string `json:"ledger"`
	LastEventID string `json:"last_event_id,omitempty"`
}

type OpenClawSnapshotProjection struct {
	SchemaVersion int               `json:"schema_version"`
	SnapshotID    string            `json:"snapshot_id"`
	GeneratedAt   string            `json:"generated_at"`
	Source        ProjectionSource  `json:"source"`
	Resources     OpenClawResources `json:"resources"`
	Status        ProjectionStatus  `json:"status"`
}

type OpenClawResources struct {
	Runs []JobProjection `json:"runs"`
	Jobs []JobProjection `json:"jobs"`
	Wiki []JobProjection `json:"wiki"`
}

type FactoryStatusProjection struct {
	Admissions              []FactoryAdmissionProjection      `json:"admissions,omitempty"`
	Jobs                    []FactoryJobProjection            `json:"jobs,omitempty"`
	ActiveWorkers           []FactoryWorkerProjection         `json:"active_workers,omitempty"`
	Slots                   []FactorySlotProjection           `json:"slots,omitempty"`
	QueueLanes              []FactoryQueueLaneProjection      `json:"queue_lanes,omitempty"`
	ModelLanes              []FactoryModelLaneProjection      `json:"model_lanes,omitempty"`
	Validations             []FactoryValidationProjection     `json:"validations,omitempty"`
	BlockedValidations      []FactoryValidationProjection     `json:"blocked_validations,omitempty"`
	Worktrees               []FactoryWorktreeProjection       `json:"worktrees,omitempty"`
	RetainedFailedWorktrees []FactoryWorktreeProjection       `json:"retained_failed_worktrees,omitempty"`
	MergeDecisions          []FactoryMergeDecisionProjection  `json:"merge_decisions,omitempty"`
	PendingManualMerges     []FactoryMergeDecisionProjection  `json:"pending_manual_merges,omitempty"`
	TerminalJobs            []FactoryTerminalProjection       `json:"terminal_jobs,omitempty"`
	RecentEvents            []FactoryEventRef                 `json:"recent_events,omitempty"`
	Logs                    []FactoryPointer                  `json:"logs,omitempty"`
	Artifacts               []FactoryPointer                  `json:"artifacts,omitempty"`
	Transcripts             []FactoryPointer                  `json:"transcripts,omitempty"`
	Diffs                   []FactoryPointer                  `json:"diffs,omitempty"`
	LastRoutingDecision     *FactoryRoutingDecisionProjection `json:"last_routing_decision,omitempty"`
}

type FactoryJobProjection struct {
	JobID       string           `json:"job_id"`
	RunID       string           `json:"run_id,omitempty"`
	TaskID      string           `json:"task_id,omitempty"`
	RequestedBy string           `json:"requested_by,omitempty"`
	Objective   string           `json:"objective,omitempty"`
	LaneID      string           `json:"lane_id,omitempty"`
	Provider    string           `json:"provider,omitempty"`
	Runtime     string           `json:"runtime,omitempty"`
	Model       string           `json:"model,omitempty"`
	Authority   RoutingAuthority `json:"authority,omitempty"`
	Status      FactoryJobStatus `json:"status"`
	SubmittedAt string           `json:"submitted_at,omitempty"`
	UpdatedAt   string           `json:"updated_at,omitempty"`
	LastEventID string           `json:"last_event_id,omitempty"`
}

type FactoryAdmissionProjection struct {
	JobID         string                  `json:"job_id"`
	RunID         string                  `json:"run_id,omitempty"`
	WorkOrderID   string                  `json:"work_order_id"`
	Allowed       bool                    `json:"allowed"`
	Reasons       []string                `json:"reasons,omitempty"`
	LandingPolicy FactoryLandingPolicy    `json:"landing_policy,omitempty"`
	DigestPolicy  FactoryDigestPolicy     `json:"digest_policy,omitempty"`
	ChildJobID    string                  `json:"child_job_id,omitempty"`
	Artifacts     map[string]string       `json:"artifacts,omitempty"`
	Evidence      FactoryDecisionEvidence `json:"evidence"`
	DecidedAt     string                  `json:"decided_at,omitempty"`
	LastEventID   string                  `json:"last_event_id,omitempty"`
}

type FactoryWorkerProjection struct {
	WorkerID     string            `json:"worker_id,omitempty"`
	SlotID       string            `json:"slot_id"`
	RunID        string            `json:"run_id,omitempty"`
	JobID        string            `json:"job_id"`
	TaskID       string            `json:"task_id,omitempty"`
	LaneID       string            `json:"lane_id,omitempty"`
	Provider     string            `json:"provider,omitempty"`
	Runtime      string            `json:"runtime,omitempty"`
	Model        string            `json:"model,omitempty"`
	Authority    RoutingAuthority  `json:"authority,omitempty"`
	Status       FactorySlotStatus `json:"status"`
	WorktreePath string            `json:"worktree_path,omitempty"`
	LastEventID  string            `json:"last_event_id,omitempty"`
}

type FactorySlotProjection struct {
	SlotID                 string            `json:"slot_id"`
	WorkerID               string            `json:"worker_id,omitempty"`
	RunID                  string            `json:"run_id,omitempty"`
	JobID                  string            `json:"job_id"`
	TaskID                 string            `json:"task_id,omitempty"`
	LaneID                 string            `json:"lane_id,omitempty"`
	Provider               string            `json:"provider,omitempty"`
	Runtime                string            `json:"runtime,omitempty"`
	Model                  string            `json:"model,omitempty"`
	Authority              RoutingAuthority  `json:"authority,omitempty"`
	Branch                 string            `json:"branch,omitempty"`
	WorktreeID             string            `json:"worktree_id,omitempty"`
	WorktreePath           string            `json:"worktree_path,omitempty"`
	ResourcePolicy         map[string]any    `json:"resource_policy,omitempty"`
	LeaseEpoch             int               `json:"lease_epoch,omitempty"`
	MaxConcurrencySnapshot int               `json:"max_concurrency_snapshot,omitempty"`
	Status                 FactorySlotStatus `json:"status"`
	AllocatedAt            string            `json:"allocated_at,omitempty"`
	UpdatedAt              string            `json:"updated_at,omitempty"`
	LastEventID            string            `json:"last_event_id,omitempty"`
}

type FactoryQueueLaneProjection struct {
	LaneID         string           `json:"lane_id"`
	Provider       string           `json:"provider,omitempty"`
	Runtime        string           `json:"runtime,omitempty"`
	Model          string           `json:"model,omitempty"`
	Authority      RoutingAuthority `json:"authority,omitempty"`
	QueueDepth     int              `json:"queue_depth"`
	DisabledReason string           `json:"disabled_reason,omitempty"`
	LastEventID    string           `json:"last_event_id,omitempty"`
}

type FactoryModelLaneProjection struct {
	LaneID         string           `json:"lane_id"`
	Provider       string           `json:"provider,omitempty"`
	Runtime        string           `json:"runtime,omitempty"`
	Model          string           `json:"model,omitempty"`
	Authority      RoutingAuthority `json:"authority,omitempty"`
	LastReason     string           `json:"last_reason,omitempty"`
	DisabledReason string           `json:"disabled_reason,omitempty"`
	LastEventID    string           `json:"last_event_id,omitempty"`
}

type FactoryValidationProjection struct {
	ValidationID string                  `json:"validation_id"`
	JobID        string                  `json:"job_id"`
	RunID        string                  `json:"run_id,omitempty"`
	SlotID       string                  `json:"slot_id,omitempty"`
	Level        string                  `json:"level,omitempty"`
	Commands     []string                `json:"commands,omitempty"`
	Status       FactoryValidationStatus `json:"status"`
	Artifacts    map[string]string       `json:"artifacts,omitempty"`
	ArtifactRefs map[string]ArtifactRef  `json:"artifact_refs,omitempty"`
	StartedAt    string                  `json:"started_at,omitempty"`
	CompletedAt  string                  `json:"completed_at,omitempty"`
	DurationMS   int                     `json:"duration_ms,omitempty"`
	LastEventID  string                  `json:"last_event_id,omitempty"`
}

type FactoryWorktreeProjection struct {
	WorktreeID       string            `json:"worktree_id"`
	RunID            string            `json:"run_id,omitempty"`
	JobID            string            `json:"job_id"`
	SlotID           string            `json:"slot_id,omitempty"`
	BaseCommit       string            `json:"base_commit,omitempty"`
	Branch           string            `json:"branch,omitempty"`
	Path             string            `json:"path,omitempty"`
	CreatedAt        string            `json:"created_at,omitempty"`
	DirtyState       string            `json:"dirty_state,omitempty"`
	RetentionPolicy  string            `json:"retention_policy,omitempty"`
	MergeDisposition string            `json:"merge_disposition,omitempty"`
	Status           FactorySlotStatus `json:"status,omitempty"`
	LastEventID      string            `json:"last_event_id,omitempty"`
}

type FactoryMergeDecisionProjection struct {
	JobID         string               `json:"job_id"`
	RunID         string               `json:"run_id,omitempty"`
	SlotID        string               `json:"slot_id,omitempty"`
	Decision      FactoryMergeDecision `json:"decision"`
	Decider       string               `json:"decider,omitempty"`
	Reason        string               `json:"reason,omitempty"`
	Conflicts     []string             `json:"conflicts,omitempty"`
	ManualCommand string               `json:"manual_command,omitempty"`
	DecidedAt     string               `json:"decided_at,omitempty"`
	LastEventID   string               `json:"last_event_id,omitempty"`
}

type FactoryTerminalProjection struct {
	JobID            string                 `json:"job_id"`
	RunID            string                 `json:"run_id,omitempty"`
	SlotID           string                 `json:"slot_id,omitempty"`
	Status           JobStatus              `json:"status"`
	ArtifactRefs     map[string]ArtifactRef `json:"artifact_refs,omitempty"`
	TranscriptRef    string                 `json:"transcript_ref,omitempty"`
	RetainedWorktree bool                   `json:"retained_worktree,omitempty"`
	OccurredAt       string                 `json:"occurred_at,omitempty"`
	LastEventID      string                 `json:"last_event_id,omitempty"`
}

type FactoryRoutingDecisionProjection struct {
	JobID          string           `json:"job_id"`
	RunID          string           `json:"run_id,omitempty"`
	TaskID         string           `json:"task_id,omitempty"`
	LaneID         string           `json:"lane_id,omitempty"`
	Provider       string           `json:"provider,omitempty"`
	Runtime        string           `json:"runtime,omitempty"`
	Model          string           `json:"model,omitempty"`
	Authority      RoutingAuthority `json:"authority,omitempty"`
	Reason         string           `json:"reason,omitempty"`
	DisabledReason string           `json:"disabled_reason,omitempty"`
	DecidedAt      string           `json:"decided_at,omitempty"`
	LastEventID    string           `json:"last_event_id,omitempty"`
}

type FactoryEventRef struct {
	EventID      string    `json:"event_id"`
	EventType    EventType `json:"event_type"`
	JobID        string    `json:"job_id,omitempty"`
	RunID        string    `json:"run_id,omitempty"`
	TaskID       string    `json:"task_id,omitempty"`
	SlotID       string    `json:"slot_id,omitempty"`
	WorktreeID   string    `json:"worktree_id,omitempty"`
	ValidationID string    `json:"validation_id,omitempty"`
	OccurredAt   string    `json:"occurred_at,omitempty"`
}

type FactoryPointer struct {
	JobID   string `json:"job_id,omitempty"`
	RunID   string `json:"run_id,omitempty"`
	SlotID  string `json:"slot_id,omitempty"`
	Kind    string `json:"kind"`
	Name    string `json:"name,omitempty"`
	Path    string `json:"path,omitempty"`
	Ref     string `json:"ref,omitempty"`
	EventID string `json:"event_id,omitempty"`
}

// RebuildProjections rebuilds the projection set by replaying the store's
// ledger and folding events through the package-level [RebuildProjections].
//
// Callers MUST check err before using the returned [ProjectionSet]. On error,
// the returned set is zero-valued: SchemaVersion == 0, RebuiltAt == "",
// Manifests == nil, and Jobs/Schedules/derived buckets are nil. Reading any
// map field (e.g., set.Manifests[name]) on a zero-valued set returns the zero
// value, but mutating those nil maps (e.g., set.Manifests[name] = ...) WILL
// panic. The bug-hunt audit (W-B-22 / soc-58q5.7) confirmed all in-tree
// callers (server.go readState, projections_test.go) check err first; this
// godoc is a guard for future callers.
func (s *Store) RebuildProjections(opts ProjectionRebuildOptions) (ProjectionSet, error) {
	replay, err := s.ReplayLedger()
	if err != nil {
		return ProjectionSet{}, err
	}
	if opts.SourceLedger == "" {
		opts.SourceLedger = filepath.ToSlash(filepath.Join(StoreDirRel, LedgerFileName))
	}
	projections, err := RebuildProjections(replay.Events, opts)
	if err != nil {
		return ProjectionSet{}, err
	}
	if len(replay.Corrupt) > 0 {
		projections.markDegraded(fmt.Sprintf("ledger replay quarantined %d corrupt record(s)", len(replay.Corrupt)))
	}
	return projections, nil
}

// RebuildProjections folds the given ledger events into a fresh
// [ProjectionSet] (or, if opts.FromSnapshot is set, into a delta replay
// seeded from that snapshot).
//
// Callers MUST check err before using the returned set. On error, the
// returned set is zero-valued (SchemaVersion == 0, nil Manifests/Jobs/
// Schedules, empty derived RPI/Dream/Wiki/OpenClaw buckets). Treat the
// SchemaVersion == 0 sentinel as "do not use this set"; downstream code that
// indexes into Manifests or appends to Jobs without checking err first risks
// nil-map panics or silently emitting an empty projection. See the bug-hunt
// L2 test TestRebuildProjections_ErrorReturnsUnusableSet for the contract.
func RebuildProjections(events []LedgerEvent, opts ProjectionRebuildOptions) (ProjectionSet, error) {
	opts = normalizeRebuildOptions(opts)
	rebuiltAt := opts.RebuiltAt.UTC().Format(time.RFC3339Nano)

	var (
		set      ProjectionSet
		jobsByID map[string]*JobProjection
		jobOrder []string
		err      error
	)

	if opts.FromSnapshot != nil {
		set, jobsByID, jobOrder = initStateFromSnapshot(*opts.FromSnapshot, opts.SourceLedger, rebuiltAt)
		delta := filterEventsAfter(events, opts.FromSnapshot.LastEventID)
		_, err = applyEventsToState(delta, &set, jobsByID, &jobOrder)
	} else {
		set = ProjectionSet{
			SchemaVersion: ProjectionSchemaVersion,
			RebuiltAt:     rebuiltAt,
			SourceLedger:  opts.SourceLedger,
			Manifests:     defaultProjectionManifests(opts.SourceLedger, rebuiltAt),
			Plans:         emptyDaemonPlansProjection(rebuiltAt),
		}
		jobsByID = map[string]*JobProjection{}
		_, err = applyEventsToState(events, &set, jobsByID, &jobOrder)
	}
	if err != nil {
		return ProjectionSet{}, err
	}

	collectJobsIntoSet(jobsByID, jobOrder, &set)
	finalizeFactoryProjection(&set)
	finalizeManifests(&set)
	finalizePlansProjection(&set)
	finalizeOpenClawSnapshot(&set, opts.SourceLedger, rebuiltAt)
	set.Schedules = ScheduleStateFromEvents(events)
	return set, nil
}

// filterEventsAfter returns only events whose EventID is strictly greater than
// lastEventID. EventID semantics: monotonically increasing string IDs assigned
// at append time. Strict greater-than is correct because lastEventID is the
// final event already folded into the snapshot.
func filterEventsAfter(events []LedgerEvent, lastEventID string) []LedgerEvent {
	if lastEventID == "" {
		return events
	}
	out := make([]LedgerEvent, 0, len(events))
	for _, event := range events {
		if event.EventID > lastEventID {
			out = append(out, event)
		}
	}
	return out
}

// initStateFromSnapshot seeds the rebuild loop with state recovered from a
// previous snapshot. Derived buckets (RPI/Dream/Wiki/OpenClaw.Resources) are
// reset to empty — collectJobsIntoSet rebuilds them from jobsByID after the
// delta events apply. Manifests are re-derived from defaultProjectionManifests
// then refreshed by finalizeManifests, so snapshot Manifests are intentionally
// discarded (they would carry stale RebuiltAt + LastEventID otherwise).
func initStateFromSnapshot(snapshot ProjectionSet, sourceLedger, rebuiltAt string) (ProjectionSet, map[string]*JobProjection, []string) {
	set := ProjectionSet{
		SchemaVersion: ProjectionSchemaVersion,
		RebuiltAt:     rebuiltAt,
		SourceLedger:  sourceLedger,
		LastEventID:   snapshot.LastEventID,
		Manifests:     defaultProjectionManifests(sourceLedger, rebuiltAt),
		Plans:         cloneDaemonPlansProjection(snapshot.Plans),
		Factory:       cloneFactoryProjection(snapshot.Factory),
	}
	jobsByID := make(map[string]*JobProjection, len(snapshot.Jobs))
	jobOrder := make([]string, 0, len(snapshot.Jobs))
	for i := range snapshot.Jobs {
		job := snapshot.Jobs[i]
		artifacts := make(map[string]string, len(job.Artifacts))
		for k, v := range job.Artifacts {
			artifacts[k] = v
		}
		job.Artifacts = artifacts
		// Symmetric deep-copy of ArtifactRefs (W-B-25 / soc-58q5.9):
		// without this, the rebuilt jobsByID entry shares the underlying
		// ArtifactRefs map with the source snapshot's job, so concurrent
		// writers on either side race. ArtifactRef is a flat value struct
		// (string/int64 fields only), so a shallow value copy suffices.
		if job.ArtifactRefs != nil {
			refs := make(map[string]ArtifactRef, len(job.ArtifactRefs))
			for k, v := range job.ArtifactRefs {
				refs[k] = v
			}
			job.ArtifactRefs = refs
		}
		jobsByID[job.JobID] = &job
		jobOrder = append(jobOrder, job.JobID)
	}
	return set, jobsByID, jobOrder
}

func normalizeRebuildOptions(opts ProjectionRebuildOptions) ProjectionRebuildOptions {
	if opts.RebuiltAt.IsZero() {
		opts.RebuiltAt = time.Now().UTC()
	}
	if opts.SourceLedger == "" {
		opts.SourceLedger = filepath.ToSlash(filepath.Join(StoreDirRel, LedgerFileName))
	}
	return opts
}

// applyEventsToState folds events into the supplied set/jobsByID/jobOrder,
// returning the count of events successfully applied. It is shared between
// full replay (empty initial state) and delta replay (state seeded from a
// snapshot via initStateFromSnapshot).
//
// Forward-compat (pre-mortem amendment B3): event types not in
// knownEventTypes are skipped-and-logged, NOT errored. This lets older
// daemon binaries replay newer ledgers without crashing — additive event
// vocabulary is an explicit non-breaking change vector.
func applyEventsToState(events []LedgerEvent, set *ProjectionSet, jobsByID map[string]*JobProjection, jobOrder *[]string) (int, error) {
	applied := 0
	for _, event := range events {
		normalized, err := NormalizeLedgerEvent(event)
		if err != nil {
			// NormalizeLedgerEvent rejects unknown event types via
			// ValidateLedgerEvent → ValidateEventType. To preserve
			// forward-compat (B3), unknown event types must be skipped,
			// not errored. Try a relaxed normalize path for that case.
			if relaxed, ok := relaxedNormalizeForUnknownEvent(event); ok {
				log.Printf("[projection] unknown event_type=%s skipped (event_id=%s)", event.EventType, event.EventID)
				set.LastEventID = relaxed.EventID
				continue
			}
			return applied, fmt.Errorf("event %q: %w", event.EventID, err)
		}
		event = normalized
		set.LastEventID = event.EventID

		if _, known := knownEventTypes[event.EventType]; !known {
			// Defense in depth: even if the validator above accepted the
			// event (e.g., another package registered the type), the
			// projection reducer skip-and-logs anything it doesn't know
			// how to fold. State is left unchanged.
			log.Printf("[projection] unknown event_type=%s skipped (event_id=%s)", event.EventType, event.EventID)
			continue
		}
		if event.EventType == EventProjectionMarkedStale || event.EventType == EventProjectionRebuilt {
			set.applyProjectionLifecycleEvent(event)
			applied++
			continue
		}
		if isFactoryEvent(event.EventType) {
			if err := set.applyFactoryLifecycleEvent(event); err != nil {
				return applied, err
			}
			applied++
			continue
		}
		if isScheduleEvent(event.EventType) {
			// Schedule events do not flow through the job-projection
			// state machine. Per learning 2026-04-30-applyqueue-helper-
			// invariants, do NOT extend applyEventMetadataToJob or
			// applyPayloadToJob with schedule logic. The schedule list
			// is rebuilt en bloc by ScheduleStateFromEvents (store.go),
			// invoked from RebuildProjections after applyEventsToState.
			applied++
			continue
		}
		job, isNew := ensureJobProjection(jobsByID, event)
		if isNew {
			*jobOrder = append(*jobOrder, event.JobID)
		}
		applyEventMetadataToJob(job, event)
		if err := applyPayloadToJob(job, event); err != nil {
			return applied, err
		}
		applyEventToJobProjection(job, event)
		applied++
	}
	return applied, nil
}

// relaxedNormalizeForUnknownEvent re-runs the normalize+validate dance with
// the event-type check skipped, returning the trimmed event if the only
// validation failure was an unknown event_type. This preserves forward-compat
// (pre-mortem B3) without weakening the standard validation path that
// ValidateLedgerEvent enforces for known events.
func relaxedNormalizeForUnknownEvent(event LedgerEvent) (LedgerEvent, bool) {
	if _, known := knownEventTypes[event.EventType]; known {
		return LedgerEvent{}, false
	}
	if event.SchemaVersion == 0 {
		event.SchemaVersion = LedgerSchemaVersion
	}
	if event.SchemaVersion != LedgerSchemaVersion {
		return LedgerEvent{}, false
	}
	if event.EventID == "" || event.RequestID == "" || event.JobID == "" || event.Actor == "" {
		// Even unknown events must have basic identity so downstream
		// observers can index them. Without these, treat as malformed
		// rather than forward-compat-skippable.
		return LedgerEvent{}, false
	}
	return event, true
}

func ensureJobProjection(jobsByID map[string]*JobProjection, event LedgerEvent) (*JobProjection, bool) {
	if job, ok := jobsByID[event.JobID]; ok {
		return job, false
	}
	job := &JobProjection{
		JobID:        event.JobID,
		RequestID:    event.RequestID,
		RequestIDs:   []string{event.RequestID},
		Status:       JobStatusQueued,
		Artifacts:    map[string]string{},
		ArtifactRefs: map[string]ArtifactRef{},
	}
	jobsByID[event.JobID] = job
	return job, true
}

// applyEventMetadataToJob updates job's request-id list, timestamps, and
// last-event marker. It is event-type-agnostic — only EventJobAccepted gates
// CreatedAt; everything else runs for every event. Event-type-conditional
// logic belongs in applyPayloadToJob or applyEventToJobProjection.
func applyEventMetadataToJob(job *JobProjection, event LedgerEvent) {
	appendRequestID(job, event.RequestID)
	if job.RequestID == "" {
		job.RequestID = event.RequestID
	}
	if job.CreatedAt == "" && event.EventType == EventJobAccepted {
		job.CreatedAt = event.OccurredAt
	}
	job.UpdatedAt = event.OccurredAt
	job.LastEventID = event.EventID
}

// applyPayloadToJob applies payload-derived fields (job type, projection
// targets, artifacts) to job. It is event-type-conditional — events whose
// payload omits those keys leave job unchanged. New payload-derived fields
// belong here, or in a new helper dispatched from processLedgerEvents.
// Metadata that must run for every event belongs in applyEventMetadataToJob.
func applyPayloadToJob(job *JobProjection, event LedgerEvent) error {
	if jobType, ok, err := jobTypeFromPayload(event.Payload); err != nil {
		return fmt.Errorf("event %q: %w", event.EventID, err)
	} else if ok {
		job.JobType = jobType
	}
	if targets := projectionTargetsFromPayload(event.Payload); len(targets) > 0 {
		job.ProjectionTargets = targets
	} else if len(job.ProjectionTargets) == 0 && job.JobType != "" {
		job.ProjectionTargets = defaultProjectionTargetsForJobType(job.JobType)
	}
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
	return nil
}

func collectJobsIntoSet(jobsByID map[string]*JobProjection, jobOrder []string, set *ProjectionSet) {
	set.Jobs = make([]JobProjection, 0, len(jobOrder))
	for _, jobID := range jobOrder {
		job := *jobsByID[jobID]
		if len(job.Artifacts) == 0 {
			job.Artifacts = nil
		}
		if len(job.ArtifactRefs) == 0 {
			job.ArtifactRefs = nil
		}
		set.Jobs = append(set.Jobs, job)
		classifyJobIntoBuckets(job, set)
		set.OpenClaw.Resources.Jobs = append(set.OpenClaw.Resources.Jobs, job)
	}
}

func classifyJobIntoBuckets(job JobProjection, set *ProjectionSet) {
	switch job.JobType {
	case JobTypeRPIRun, JobTypeRPIPhase:
		set.RPI.Runs = append(set.RPI.Runs, job)
		set.OpenClaw.Resources.Runs = append(set.OpenClaw.Resources.Runs, job)
	case JobTypeDreamRun, JobTypeDreamStage:
		set.Dream.Runs = append(set.Dream.Runs, job)
		set.OpenClaw.Resources.Runs = append(set.OpenClaw.Resources.Runs, job)
	case JobTypeWikiBuild, JobTypeWikiForge:
		set.Wiki.Jobs = append(set.Wiki.Jobs, job)
		set.OpenClaw.Resources.Wiki = append(set.OpenClaw.Resources.Wiki, job)
	}
}

func finalizeManifests(set *ProjectionSet) {
	for name, manifest := range set.Manifests {
		manifest.LastEventID = set.LastEventID
		set.Manifests[name] = manifest
	}
}

func finalizePlansProjection(set *ProjectionSet) {
	if set.Plans.SchemaVersion == 0 {
		set.Plans.SchemaVersion = DaemonPlansProjectionSchemaVersion
	}
	if set.Plans.Entries == nil {
		set.Plans.Entries = []PlansProjectionEntry{}
	}
	set.Plans.LastEventID = set.LastEventID
	if set.Plans.RebuiltAt == "" {
		set.Plans.RebuiltAt = set.RebuiltAt
	}
}

func finalizeOpenClawSnapshot(set *ProjectionSet, sourceLedger, rebuiltAt string) {
	set.OpenClaw.SchemaVersion = ProjectionSchemaVersion
	set.OpenClaw.SnapshotID = "snap_empty"
	if set.LastEventID != "" {
		set.OpenClaw.SnapshotID = "snap_" + set.LastEventID
	}
	set.OpenClaw.GeneratedAt = rebuiltAt
	set.OpenClaw.Source = ProjectionSource{Ledger: sourceLedger, LastEventID: set.LastEventID}
	set.OpenClaw.Status = set.Manifests[ProjectionOpenClaw].Status
}

func applyEventToJobProjection(job *JobProjection, event LedgerEvent) {
	if isTerminalStatus(job.Status) {
		return
	}
	switch event.EventType {
	case EventJobAccepted:
		job.Status = JobStatusQueued
	case EventJobClaimed, EventJobHeartbeat:
		job.Status = JobStatusRunning
	case EventJobLeaseExpired:
		job.Status = JobStatusRetryWaiting
	case EventJobCompleted:
		job.Status = JobStatusCompleted
		job.ResultStatus = JobResultSucceeded
	case EventJobFailed:
		job.Status = JobStatusFailed
		job.ResultStatus = JobResultFailed
		failure := failureFromPayload(event.Payload)
		job.Failure = &failure
	case EventJobCancelled:
		job.Status = JobStatusCancelled
		job.ResultStatus = JobResultCancelled
	}
}

func (set *ProjectionSet) applyProjectionLifecycleEvent(event LedgerEvent) {
	status := ProjectionStatusCurrent
	if event.EventType == EventProjectionMarkedStale {
		status = ProjectionStatusStale
	}
	targets := projectionTargetsFromPayload(event.Payload)
	if len(targets) == 0 {
		targets = allProjectionNames()
	}
	for _, target := range targets {
		manifest, ok := set.Manifests[target]
		if !ok {
			continue
		}
		manifest.Status = status
		manifest.LastEventID = event.EventID
		set.Manifests[target] = manifest
	}
}

const (
	factoryUnroutedLaneID  = "unrouted"
	maxFactoryRecentEvents = 25
)

func (set *ProjectionSet) applyFactoryLifecycleEvent(event LedgerEvent) error {
	factory := &set.Factory
	factory.recordFactoryEvent(event)
	factory.recordFactoryPointers(event)
	return factory.applyFactoryEvent(event)
}

func (factory *FactoryStatusProjection) applyFactoryEvent(event LedgerEvent) error {
	switch event.EventType {
	case EventFactoryAdmissionDecided:
		return factory.applyFactoryAdmissionDecided(event)
	case EventFactoryJobSubmitted:
		factory.applyFactoryJobSubmitted(event)
	case EventFactoryJobClaimed:
		factory.applyFactoryJobClaimed(event)
	case EventFactoryJobStarted:
		factory.applyFactoryJobStarted(event)
	case EventFactoryRoutingDecided:
		return factory.applyFactoryRoutingDecided(event)
	case EventFactorySlotAllocated:
		return factory.applyFactorySlotAllocated(event)
	case EventFactoryWorktreeAllocated:
		factory.applyFactoryWorktreeAllocated(event)
	case EventFactoryValidationStarted:
		return factory.applyFactoryValidationStarted(event)
	case EventFactoryValidationCompleted:
		return factory.applyFactoryValidationCompleted(event)
	case EventFactoryMergeDecision:
		return factory.applyFactoryMergeDecision(event)
	case EventFactoryJobTerminal:
		return factory.applyFactoryJobTerminal(event)
	}
	return nil
}

func (factory *FactoryStatusProjection) applyFactoryAdmissionDecided(event LedgerEvent) error {
	admission, err := factoryAdmissionFromPayload(event)
	if err != nil {
		return fmt.Errorf("event %q: %w", event.EventID, err)
	}
	factory.upsertFactoryAdmission(admission)

	job := factory.upsertFactoryJob(event.JobID)
	factory.applyFactoryJobMetadata(job, event)
	if admission.Allowed {
		job.Status = FactoryJobStatusAdmitted
	} else {
		job.Status = FactoryJobStatusAdmissionBlocked
	}
	return nil
}

func (factory *FactoryStatusProjection) applyFactoryJobSubmitted(event LedgerEvent) {
	job := factory.upsertFactoryJob(event.JobID)
	factory.applyFactoryJobMetadata(job, event)
	if job.SubmittedAt == "" {
		job.SubmittedAt = event.OccurredAt
	}
	job.Status = FactoryJobStatusSubmitted
}

func (factory *FactoryStatusProjection) applyFactoryJobClaimed(event LedgerEvent) {
	job := factory.upsertFactoryJob(event.JobID)
	factory.applyFactoryJobMetadata(job, event)
	job.Status = FactoryJobStatusClaimed
	if slot := factory.slotForJobOrPayload(event); slot != nil {
		if workerID, ok := stringPayload(event.Payload, "worker_id"); ok {
			slot.WorkerID = workerID
		}
		slot.UpdatedAt = event.OccurredAt
		slot.LastEventID = event.EventID
	}
}

func (factory *FactoryStatusProjection) applyFactoryJobStarted(event LedgerEvent) {
	job := factory.upsertFactoryJob(event.JobID)
	factory.applyFactoryJobMetadata(job, event)
	job.Status = FactoryJobStatusStarted
	if slot := factory.slotForJobOrPayload(event); slot != nil {
		slot.Status = FactorySlotStatusRunning
		if workerID, ok := stringPayload(event.Payload, "worker_id"); ok {
			slot.WorkerID = workerID
		}
		slot.UpdatedAt = event.OccurredAt
		slot.LastEventID = event.EventID
	}
}

func (factory *FactoryStatusProjection) applyFactoryRoutingDecided(event LedgerEvent) error {
	job := factory.upsertFactoryJob(event.JobID)
	factory.applyFactoryJobMetadata(job, event)

	decision, err := factory.routingDecisionFromEvent(event, job)
	if err != nil {
		return fmt.Errorf("event %q: %w", event.EventID, err)
	}
	job.LaneID = decision.LaneID
	job.Provider = decision.Provider
	job.Runtime = decision.Runtime
	job.Model = decision.Model
	job.Authority = decision.Authority
	job.Status = FactoryJobStatusRouted

	factory.upsertModelLane(decision)
	decisionCopy := decision
	factory.LastRoutingDecision = &decisionCopy
	return nil
}

func (factory *FactoryStatusProjection) applyFactorySlotAllocated(event LedgerEvent) error {
	job := factory.upsertFactoryJob(event.JobID)
	factory.applyFactoryJobMetadata(job, event)

	slotID, _ := stringPayload(event.Payload, "slot_id")
	if slotID == "" {
		return nil
	}
	slot := factory.upsertFactorySlot(slotID)
	slot.JobID = event.JobID
	if err := factory.applyFactoryRouteToSlot(slot, event, job); err != nil {
		return fmt.Errorf("event %q: %w", event.EventID, err)
	}
	if workerID, ok := stringPayload(event.Payload, "worker_id"); ok {
		slot.WorkerID = workerID
	}
	if resourcePolicy, ok := mapPayload(event.Payload, "resource_policy"); ok {
		slot.ResourcePolicy = resourcePolicy
	}
	if leaseEpoch, ok := intPayload(event.Payload, "lease_epoch"); ok {
		slot.LeaseEpoch = leaseEpoch
	}
	if maxConcurrency, ok := intPayload(event.Payload, "max_concurrency_snapshot"); ok {
		slot.MaxConcurrencySnapshot = maxConcurrency
	}
	if slot.AllocatedAt == "" {
		slot.AllocatedAt = event.OccurredAt
	}
	slot.Status = FactorySlotStatusAllocated
	slot.UpdatedAt = event.OccurredAt
	slot.LastEventID = event.EventID
	job.Status = FactoryJobStatusAllocated
	return nil
}

func (factory *FactoryStatusProjection) applyFactoryWorktreeAllocated(event LedgerEvent) {
	job := factory.upsertFactoryJob(event.JobID)
	factory.applyFactoryJobMetadata(job, event)

	slotID, _ := stringPayload(event.Payload, "slot_id")
	if slotID == "" {
		if slot := factory.factorySlotByJob(event.JobID); slot != nil {
			slotID = slot.SlotID
		}
	}
	worktreeID, _ := stringPayload(event.Payload, "worktree_id")
	if worktreeID != "" {
		worktree := factory.upsertFactoryWorktree(worktreeID)
		worktree.JobID = event.JobID
		worktree.SlotID = slotID
		factory.applyFactoryWorktreePayload(worktree, event)
	}

	slot := factory.factorySlotByID(slotID)
	if slot == nil {
		slot = factory.factorySlotByJob(event.JobID)
	}
	if slot != nil {
		if worktreeID != "" {
			slot.WorktreeID = worktreeID
		}
		if path, ok := stringPayload(event.Payload, "path"); ok {
			slot.WorktreePath = path
		}
		if branch, ok := stringPayload(event.Payload, "branch"); ok {
			slot.Branch = branch
		}
		slot.UpdatedAt = event.OccurredAt
		slot.LastEventID = event.EventID
	}
	if job.Status == "" || job.Status == FactoryJobStatusSubmitted || job.Status == FactoryJobStatusRouted {
		job.Status = FactoryJobStatusAllocated
	}
}

func (factory *FactoryStatusProjection) applyFactoryValidationStarted(event LedgerEvent) error {
	status := FactoryValidationStatusRunning
	if payloadStatus, ok, err := factoryValidationStatusFromPayload(event.Payload); err != nil {
		return fmt.Errorf("event %q: %w", event.EventID, err)
	} else if ok {
		status = payloadStatus
	}
	validation := factory.upsertFactoryValidation(validationIDFromEvent(event))
	validation.JobID = event.JobID
	factory.applyFactoryValidationPayload(validation, event)
	validation.Status = status
	if validation.StartedAt == "" {
		validation.StartedAt = event.OccurredAt
	}
	validation.LastEventID = event.EventID

	job := factory.upsertFactoryJob(event.JobID)
	factory.applyFactoryJobMetadata(job, event)
	job.Status = FactoryJobStatusValidating
	if slot := factory.slotForValidation(event, validation); slot != nil {
		slot.Status = FactorySlotStatusBlockedValidation
		slot.UpdatedAt = event.OccurredAt
		slot.LastEventID = event.EventID
		validation.SlotID = slot.SlotID
	}
	return nil
}

func (factory *FactoryStatusProjection) applyFactoryValidationCompleted(event LedgerEvent) error {
	status := FactoryValidationStatusPassed
	if payloadStatus, ok, err := factoryValidationStatusFromPayload(event.Payload); err != nil {
		return fmt.Errorf("event %q: %w", event.EventID, err)
	} else if ok {
		status = payloadStatus
	}
	validation := factory.upsertFactoryValidation(validationIDFromEvent(event))
	validation.JobID = event.JobID
	factory.applyFactoryValidationPayload(validation, event)
	validation.Status = status
	validation.CompletedAt = event.OccurredAt
	if durationMS, ok := intPayload(event.Payload, "duration_ms"); ok {
		validation.DurationMS = durationMS
	}
	for key, value := range artifactsFromPayload(event.Payload) {
		if validation.Artifacts == nil {
			validation.Artifacts = map[string]string{}
		}
		validation.Artifacts[key] = value
	}
	for key, ref := range artifactRefsFromPayload(event.Payload) {
		if validation.ArtifactRefs == nil {
			validation.ArtifactRefs = map[string]ArtifactRef{}
		}
		validation.ArtifactRefs[key] = ref
	}
	validation.LastEventID = event.EventID

	job := factory.upsertFactoryJob(event.JobID)
	factory.applyFactoryJobMetadata(job, event)
	if status == FactoryValidationStatusPassed {
		job.Status = FactoryJobStatusValidated
	} else {
		job.Status = FactoryJobStatusValidationFailed
	}
	if slot := factory.slotForValidation(event, validation); slot != nil {
		if status == FactoryValidationStatusPassed || status == FactoryValidationStatusCancelled {
			slot.Status = FactorySlotStatusAllocated
		} else {
			slot.Status = FactorySlotStatusBlockedValidation
		}
		slot.UpdatedAt = event.OccurredAt
		slot.LastEventID = event.EventID
		validation.SlotID = slot.SlotID
	}
	return nil
}

func (factory *FactoryStatusProjection) applyFactoryMergeDecision(event LedgerEvent) error {
	decisionValue := FactoryMergeDecisionNotRequested
	if payloadDecision, ok, err := factoryMergeDecisionFromPayload(event.Payload); err != nil {
		return fmt.Errorf("event %q: %w", event.EventID, err)
	} else if ok {
		decisionValue = payloadDecision
	}

	merge := factory.upsertFactoryMergeDecision(event.JobID)
	merge.JobID = event.JobID
	merge.RunID = factory.runIDForEvent(event)
	merge.SlotID = factory.slotIDForEvent(event)
	merge.Decision = decisionValue
	if decider, ok := stringPayload(event.Payload, "decider"); ok {
		merge.Decider = decider
	}
	if reason, ok := stringPayload(event.Payload, "reason"); ok {
		merge.Reason = reason
	}
	merge.Conflicts = stringSlicePayload(event.Payload, "conflicts")
	if manualCommand, ok := stringPayload(event.Payload, "manual_command"); ok {
		merge.ManualCommand = manualCommand
	}
	merge.DecidedAt = event.OccurredAt
	merge.LastEventID = event.EventID

	job := factory.upsertFactoryJob(event.JobID)
	factory.applyFactoryJobMetadata(job, event)
	if decisionValue == FactoryMergeDecisionManualPending {
		job.Status = FactoryJobStatusAwaitingManualMerge
	}
	if slot := factory.slotForJobOrPayload(event); slot != nil {
		if decisionValue == FactoryMergeDecisionManualPending {
			slot.Status = FactorySlotStatusAwaitingManualMerge
		}
		slot.UpdatedAt = event.OccurredAt
		slot.LastEventID = event.EventID
	}
	if worktree := factory.worktreeForJobOrSlot(event); worktree != nil {
		worktree.MergeDisposition = string(decisionValue)
		worktree.LastEventID = event.EventID
	}
	return nil
}

func (factory *FactoryStatusProjection) applyFactoryJobTerminal(event LedgerEvent) error {
	status, ok, err := terminalJobStatusFromPayload(event.Payload)
	if err != nil {
		return fmt.Errorf("event %q: %w", event.EventID, err)
	}
	retainedWorktree, _ := boolPayload(event.Payload, "retained_worktree")

	terminal := factory.upsertFactoryTerminal(event.JobID)
	terminal.JobID = event.JobID
	terminal.RunID = factory.runIDForEvent(event)
	terminal.SlotID = factory.slotIDForEvent(event)
	if ok {
		terminal.Status = status
	}
	terminal.ArtifactRefs = cloneArtifactRefs(artifactRefsFromPayload(event.Payload))
	if transcriptRef, ok := stringPayload(event.Payload, "transcript_ref"); ok {
		terminal.TranscriptRef = transcriptRef
	}
	terminal.RetainedWorktree = retainedWorktree
	terminal.OccurredAt = event.OccurredAt
	terminal.LastEventID = event.EventID

	job := factory.upsertFactoryJob(event.JobID)
	factory.applyFactoryJobMetadata(job, event)
	if retainedWorktree && status == JobStatusFailed {
		job.Status = FactoryJobStatusRetainedFailed
	} else {
		job.Status = FactoryJobStatusTerminal
	}
	if slot := factory.slotForJobOrPayload(event); slot != nil {
		if retainedWorktree && status == JobStatusFailed {
			slot.Status = FactorySlotStatusRetainedFailed
		} else {
			slot.Status = FactorySlotStatusTerminal
		}
		slot.UpdatedAt = event.OccurredAt
		slot.LastEventID = event.EventID
	}
	if worktree := factory.worktreeForJobOrSlot(event); worktree != nil {
		if retainedWorktree && status == JobStatusFailed {
			worktree.Status = FactorySlotStatusRetainedFailed
			if worktree.RetentionPolicy == "" {
				worktree.RetentionPolicy = "retain_on_failure"
			}
		} else {
			worktree.Status = FactorySlotStatusTerminal
		}
		worktree.LastEventID = event.EventID
	}
	return nil
}

func (factory *FactoryStatusProjection) applyFactoryJobMetadata(job *FactoryJobProjection, event LedgerEvent) {
	if job.JobID == "" {
		job.JobID = event.JobID
	}
	if runID, ok := stringPayload(event.Payload, "run_id"); ok {
		job.RunID = runID
	}
	if taskID, ok := stringPayload(event.Payload, "task_id"); ok {
		job.TaskID = taskID
	}
	if requestedBy, ok := stringPayload(event.Payload, "requested_by"); ok {
		job.RequestedBy = requestedBy
	}
	if objective, ok := stringPayload(event.Payload, "objective"); ok {
		job.Objective = objective
	}
	job.UpdatedAt = event.OccurredAt
	job.LastEventID = event.EventID
}

func (factory *FactoryStatusProjection) routingDecisionFromEvent(event LedgerEvent, job *FactoryJobProjection) (FactoryRoutingDecisionProjection, error) {
	authority, ok, err := routingAuthorityFromPayload(event.Payload)
	if err != nil {
		return FactoryRoutingDecisionProjection{}, err
	}
	if !ok && job != nil {
		authority = job.Authority
	}
	decision := FactoryRoutingDecisionProjection{
		JobID:       event.JobID,
		RunID:       factory.runIDForEvent(event),
		TaskID:      factory.taskIDForEvent(event),
		Authority:   authority,
		DecidedAt:   event.OccurredAt,
		LastEventID: event.EventID,
	}
	if laneID, ok := stringPayload(event.Payload, "lane_id"); ok {
		decision.LaneID = laneID
	} else if job != nil {
		decision.LaneID = job.LaneID
	}
	if provider, ok := stringPayload(event.Payload, "provider"); ok {
		decision.Provider = provider
	} else if job != nil {
		decision.Provider = job.Provider
	}
	if runtime, ok := stringPayload(event.Payload, "runtime"); ok {
		decision.Runtime = runtime
	} else if job != nil {
		decision.Runtime = job.Runtime
	}
	if model, ok := stringPayload(event.Payload, "model"); ok {
		decision.Model = model
	} else if job != nil {
		decision.Model = job.Model
	}
	if reason, ok := stringPayload(event.Payload, "reason"); ok {
		decision.Reason = reason
	}
	if disabledReason, ok := stringPayload(event.Payload, "disabled_reason"); ok {
		decision.DisabledReason = disabledReason
	}
	return decision, nil
}

func (factory *FactoryStatusProjection) applyFactoryRouteToSlot(slot *FactorySlotProjection, event LedgerEvent, job *FactoryJobProjection) error {
	slot.RunID = factory.runIDForEvent(event)
	slot.TaskID = factory.taskIDForEvent(event)
	if laneID, ok := stringPayload(event.Payload, "lane_id"); ok {
		slot.LaneID = laneID
	} else if job != nil {
		slot.LaneID = job.LaneID
	}
	if provider, ok := stringPayload(event.Payload, "provider"); ok {
		slot.Provider = provider
	} else if job != nil {
		slot.Provider = job.Provider
	}
	if runtime, ok := stringPayload(event.Payload, "runtime"); ok {
		slot.Runtime = runtime
	} else if job != nil {
		slot.Runtime = job.Runtime
	}
	if model, ok := stringPayload(event.Payload, "model"); ok {
		slot.Model = model
	} else if job != nil {
		slot.Model = job.Model
	}
	if authority, ok, err := routingAuthorityFromPayload(event.Payload); err != nil {
		return err
	} else if ok {
		slot.Authority = authority
	} else if job != nil {
		slot.Authority = job.Authority
	}
	return nil
}

func (factory *FactoryStatusProjection) applyFactoryWorktreePayload(worktree *FactoryWorktreeProjection, event LedgerEvent) {
	worktree.RunID = factory.runIDForEvent(event)
	if ownerJobID, ok := stringPayload(event.Payload, "owner_job_id"); ok {
		worktree.JobID = ownerJobID
	}
	if ownerSlotID, ok := stringPayload(event.Payload, "owner_slot_id"); ok {
		worktree.SlotID = ownerSlotID
	} else if slotID, ok := stringPayload(event.Payload, "slot_id"); ok {
		worktree.SlotID = slotID
	}
	if baseCommit, ok := stringPayload(event.Payload, "base_commit"); ok {
		worktree.BaseCommit = baseCommit
	}
	if branch, ok := stringPayload(event.Payload, "branch"); ok {
		worktree.Branch = branch
	}
	if path, ok := stringPayload(event.Payload, "path"); ok {
		worktree.Path = path
	}
	if dirtyState, ok := stringPayload(event.Payload, "dirty_state"); ok {
		worktree.DirtyState = dirtyState
	}
	if retentionPolicy, ok := stringPayload(event.Payload, "retention_policy"); ok {
		worktree.RetentionPolicy = retentionPolicy
	}
	if mergeDisposition, ok := stringPayload(event.Payload, "merge_disposition"); ok {
		worktree.MergeDisposition = mergeDisposition
	}
	if worktree.CreatedAt == "" {
		worktree.CreatedAt = event.OccurredAt
	}
	if worktree.Status == "" {
		worktree.Status = FactorySlotStatusAllocated
	}
	worktree.LastEventID = event.EventID
}

func (factory *FactoryStatusProjection) applyFactoryValidationPayload(validation *FactoryValidationProjection, event LedgerEvent) {
	validation.RunID = factory.runIDForEvent(event)
	validation.SlotID = factory.slotIDForEvent(event)
	if level, ok := stringPayload(event.Payload, "level"); ok {
		validation.Level = level
	}
	if commands := stringSlicePayload(event.Payload, "commands"); len(commands) > 0 {
		validation.Commands = commands
	}
}

func (factory *FactoryStatusProjection) upsertModelLane(decision FactoryRoutingDecisionProjection) {
	if decision.LaneID == "" {
		return
	}
	for i := range factory.ModelLanes {
		if factory.ModelLanes[i].LaneID == decision.LaneID {
			factory.ModelLanes[i].Provider = decision.Provider
			factory.ModelLanes[i].Runtime = decision.Runtime
			factory.ModelLanes[i].Model = decision.Model
			factory.ModelLanes[i].Authority = decision.Authority
			factory.ModelLanes[i].LastReason = decision.Reason
			factory.ModelLanes[i].DisabledReason = decision.DisabledReason
			factory.ModelLanes[i].LastEventID = decision.LastEventID
			return
		}
	}
	factory.ModelLanes = append(factory.ModelLanes, FactoryModelLaneProjection{
		LaneID:         decision.LaneID,
		Provider:       decision.Provider,
		Runtime:        decision.Runtime,
		Model:          decision.Model,
		Authority:      decision.Authority,
		LastReason:     decision.Reason,
		DisabledReason: decision.DisabledReason,
		LastEventID:    decision.LastEventID,
	})
}

func finalizeFactoryProjection(set *ProjectionSet) {
	factory := &set.Factory
	factory.QueueLanes = factory.deriveQueueLanes()
	factory.ActiveWorkers = factory.deriveActiveWorkers()
	factory.BlockedValidations = factory.deriveBlockedValidations()
	factory.RetainedFailedWorktrees = factory.deriveRetainedFailedWorktrees()
	factory.PendingManualMerges = factory.derivePendingManualMerges()
}

func (factory *FactoryStatusProjection) deriveQueueLanes() []FactoryQueueLaneProjection {
	counts := map[string]int{}
	lastEventByLane := map[string]string{}
	for _, job := range factory.Jobs {
		if !isQueuedFactoryJobStatus(job.Status) {
			continue
		}
		laneID := job.LaneID
		if laneID == "" {
			laneID = factoryUnroutedLaneID
		}
		counts[laneID]++
		lastEventByLane[laneID] = job.LastEventID
	}

	var lanes []FactoryQueueLaneProjection
	seen := map[string]struct{}{}
	if count := counts[factoryUnroutedLaneID]; count > 0 {
		lanes = append(lanes, FactoryQueueLaneProjection{
			LaneID:      factoryUnroutedLaneID,
			QueueDepth:  count,
			LastEventID: lastEventByLane[factoryUnroutedLaneID],
		})
		seen[factoryUnroutedLaneID] = struct{}{}
	}
	for _, modelLane := range factory.ModelLanes {
		lanes = append(lanes, FactoryQueueLaneProjection{
			LaneID:         modelLane.LaneID,
			Provider:       modelLane.Provider,
			Runtime:        modelLane.Runtime,
			Model:          modelLane.Model,
			Authority:      modelLane.Authority,
			QueueDepth:     counts[modelLane.LaneID],
			DisabledReason: modelLane.DisabledReason,
			LastEventID:    modelLane.LastEventID,
		})
		seen[modelLane.LaneID] = struct{}{}
	}
	for _, job := range factory.Jobs {
		if !isQueuedFactoryJobStatus(job.Status) {
			continue
		}
		laneID := job.LaneID
		if laneID == "" {
			laneID = factoryUnroutedLaneID
		}
		if _, ok := seen[laneID]; ok {
			continue
		}
		lanes = append(lanes, FactoryQueueLaneProjection{
			LaneID:      laneID,
			Provider:    job.Provider,
			Runtime:     job.Runtime,
			Model:       job.Model,
			Authority:   job.Authority,
			QueueDepth:  counts[laneID],
			LastEventID: lastEventByLane[laneID],
		})
		seen[laneID] = struct{}{}
	}
	return lanes
}

func (factory *FactoryStatusProjection) deriveActiveWorkers() []FactoryWorkerProjection {
	workers := make([]FactoryWorkerProjection, 0, len(factory.Slots))
	for _, slot := range factory.Slots {
		if !isActiveFactorySlotStatus(slot.Status) {
			continue
		}
		workerID := slot.WorkerID
		if workerID == "" {
			workerID = slot.SlotID
		}
		workers = append(workers, FactoryWorkerProjection{
			WorkerID:     workerID,
			SlotID:       slot.SlotID,
			RunID:        slot.RunID,
			JobID:        slot.JobID,
			TaskID:       slot.TaskID,
			LaneID:       slot.LaneID,
			Provider:     slot.Provider,
			Runtime:      slot.Runtime,
			Model:        slot.Model,
			Authority:    slot.Authority,
			Status:       slot.Status,
			WorktreePath: slot.WorktreePath,
			LastEventID:  slot.LastEventID,
		})
	}
	return workers
}

func (factory *FactoryStatusProjection) deriveBlockedValidations() []FactoryValidationProjection {
	var validations []FactoryValidationProjection
	for _, validation := range factory.Validations {
		switch validation.Status {
		case FactoryValidationStatusRunning, FactoryValidationStatusFailed, FactoryValidationStatusBlocked:
			validations = append(validations, validation)
		}
	}
	return validations
}

func (factory *FactoryStatusProjection) deriveRetainedFailedWorktrees() []FactoryWorktreeProjection {
	var worktrees []FactoryWorktreeProjection
	for _, worktree := range factory.Worktrees {
		if worktree.Status == FactorySlotStatusRetainedFailed {
			worktrees = append(worktrees, worktree)
		}
	}
	return worktrees
}

func (factory *FactoryStatusProjection) derivePendingManualMerges() []FactoryMergeDecisionProjection {
	var decisions []FactoryMergeDecisionProjection
	for _, decision := range factory.MergeDecisions {
		if decision.Decision == FactoryMergeDecisionManualPending {
			decisions = append(decisions, decision)
		}
	}
	return decisions
}

func isQueuedFactoryJobStatus(status FactoryJobStatus) bool {
	return status == FactoryJobStatusSubmitted || status == FactoryJobStatusRouted
}

func isActiveFactorySlotStatus(status FactorySlotStatus) bool {
	switch status {
	case FactorySlotStatusAllocated, FactorySlotStatusRunning, FactorySlotStatusBlockedValidation, FactorySlotStatusAwaitingManualMerge:
		return true
	default:
		return false
	}
}

func (factory *FactoryStatusProjection) recordFactoryEvent(event LedgerEvent) {
	validationID := ""
	if event.EventType == EventFactoryValidationStarted || event.EventType == EventFactoryValidationCompleted {
		validationID = validationIDFromEvent(event)
	}
	ref := FactoryEventRef{
		EventID:      event.EventID,
		EventType:    event.EventType,
		JobID:        event.JobID,
		RunID:        factory.runIDForEvent(event),
		TaskID:       factory.taskIDForEvent(event),
		SlotID:       factory.slotIDForEvent(event),
		WorktreeID:   factory.worktreeIDForEvent(event),
		ValidationID: validationID,
		OccurredAt:   event.OccurredAt,
	}
	factory.RecentEvents = append(factory.RecentEvents, ref)
	if len(factory.RecentEvents) > maxFactoryRecentEvents {
		factory.RecentEvents = factory.RecentEvents[len(factory.RecentEvents)-maxFactoryRecentEvents:]
	}
}

func (factory *FactoryStatusProjection) recordFactoryPointers(event LedgerEvent) {
	base := FactoryPointer{
		JobID:   event.JobID,
		RunID:   factory.runIDForEvent(event),
		SlotID:  factory.slotIDForEvent(event),
		EventID: event.EventID,
	}
	for key, value := range artifactsFromPayload(event.Payload) {
		pointer := base
		pointer.Kind = "artifact"
		pointer.Name = key
		pointer.Path = value
		factory.Artifacts = append(factory.Artifacts, pointer)
	}
	for key, ref := range artifactRefsFromPayload(event.Payload) {
		pointer := base
		pointer.Kind = "artifact_ref"
		pointer.Name = key
		pointer.Path = ref.Path
		pointer.Ref = ref.SHA256
		factory.Artifacts = append(factory.Artifacts, pointer)
	}
	for key, value := range stringMapPayload(event.Payload, "logs") {
		pointer := base
		pointer.Kind = "log"
		pointer.Name = key
		pointer.Path = value
		factory.Logs = append(factory.Logs, pointer)
	}
	for key, value := range stringMapPayload(event.Payload, "log_refs") {
		pointer := base
		pointer.Kind = "log"
		pointer.Name = key
		pointer.Path = value
		factory.Logs = append(factory.Logs, pointer)
	}
	if value, ok := stringPayload(event.Payload, "log_ref"); ok {
		pointer := base
		pointer.Kind = "log"
		pointer.Path = value
		factory.Logs = append(factory.Logs, pointer)
	}
	for key, value := range stringMapPayload(event.Payload, "transcript_refs") {
		pointer := base
		pointer.Kind = "transcript"
		pointer.Name = key
		pointer.Path = value
		factory.Transcripts = append(factory.Transcripts, pointer)
	}
	if value, ok := stringPayload(event.Payload, "transcript_ref"); ok {
		pointer := base
		pointer.Kind = "transcript"
		pointer.Path = value
		factory.Transcripts = append(factory.Transcripts, pointer)
	}
	for key, value := range stringMapPayload(event.Payload, "diff_refs") {
		pointer := base
		pointer.Kind = "diff"
		pointer.Name = key
		pointer.Path = value
		factory.Diffs = append(factory.Diffs, pointer)
	}
	if value, ok := stringPayload(event.Payload, "diff_ref"); ok {
		pointer := base
		pointer.Kind = "diff"
		pointer.Path = value
		factory.Diffs = append(factory.Diffs, pointer)
	}
}

func factoryAdmissionFromPayload(event LedgerEvent) (FactoryAdmissionProjection, error) {
	admission := FactoryAdmissionProjection{
		JobID:       event.JobID,
		Reasons:     stringSlicePayload(event.Payload, "reasons"),
		Artifacts:   artifactsFromPayload(event.Payload),
		DecidedAt:   event.OccurredAt,
		LastEventID: event.EventID,
	}
	if runID, ok := stringPayload(event.Payload, "run_id"); ok {
		admission.RunID = runID
	}
	if workOrderID, ok := stringPayload(event.Payload, "work_order_id"); ok {
		admission.WorkOrderID = workOrderID
	}
	if allowed, ok := boolPayload(event.Payload, "allowed"); ok {
		admission.Allowed = allowed
	}
	if landing, ok := stringPayload(event.Payload, "landing_policy"); ok {
		admission.LandingPolicy = FactoryLandingPolicy(landing)
	}
	if digest, ok := stringPayload(event.Payload, "digest_policy"); ok {
		admission.DigestPolicy = FactoryDigestPolicy(digest)
	}
	if childJobID, ok := stringPayload(event.Payload, "child_job_id"); ok {
		admission.ChildJobID = childJobID
	}
	if evidence, ok, err := factoryDecisionEvidenceFromPayload(event.Payload); err != nil {
		return FactoryAdmissionProjection{}, err
	} else if ok {
		admission.Evidence = evidence
	}
	if admission.WorkOrderID == "" {
		return FactoryAdmissionProjection{}, fmt.Errorf("work_order_id is required")
	}
	if admission.LandingPolicy != "" {
		if err := ValidateFactoryLandingPolicy(admission.LandingPolicy); err != nil {
			return FactoryAdmissionProjection{}, err
		}
	}
	if admission.DigestPolicy != "" {
		if err := ValidateFactoryDigestPolicy(admission.DigestPolicy); err != nil {
			return FactoryAdmissionProjection{}, err
		}
	}
	return admission, nil
}

func factoryDecisionEvidenceFromPayload(payload map[string]any) (FactoryDecisionEvidence, bool, error) {
	raw, ok := payload["evidence"]
	if !ok {
		return FactoryDecisionEvidence{}, false, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return FactoryDecisionEvidence{}, true, err
	}
	var evidence FactoryDecisionEvidence
	if err := json.Unmarshal(data, &evidence); err != nil {
		return FactoryDecisionEvidence{}, true, err
	}
	if err := evidence.Validate(); err != nil {
		return FactoryDecisionEvidence{}, true, err
	}
	return evidence, true, nil
}

func (factory *FactoryStatusProjection) upsertFactoryAdmission(admission FactoryAdmissionProjection) {
	admission.Reasons = append([]string{}, admission.Reasons...)
	admission.Artifacts = cloneStringMap(admission.Artifacts)
	for i := range factory.Admissions {
		existing := &factory.Admissions[i]
		if existing.JobID == admission.JobID && existing.WorkOrderID == admission.WorkOrderID {
			factory.Admissions[i] = admission
			return
		}
	}
	factory.Admissions = append(factory.Admissions, admission)
}

func (factory *FactoryStatusProjection) upsertFactoryJob(jobID string) *FactoryJobProjection {
	for i := range factory.Jobs {
		if factory.Jobs[i].JobID == jobID {
			return &factory.Jobs[i]
		}
	}
	factory.Jobs = append(factory.Jobs, FactoryJobProjection{JobID: jobID, Status: FactoryJobStatusSubmitted})
	return &factory.Jobs[len(factory.Jobs)-1]
}

func (factory *FactoryStatusProjection) factoryJobByID(jobID string) *FactoryJobProjection {
	for i := range factory.Jobs {
		if factory.Jobs[i].JobID == jobID {
			return &factory.Jobs[i]
		}
	}
	return nil
}

func (factory *FactoryStatusProjection) upsertFactorySlot(slotID string) *FactorySlotProjection {
	for i := range factory.Slots {
		if factory.Slots[i].SlotID == slotID {
			return &factory.Slots[i]
		}
	}
	factory.Slots = append(factory.Slots, FactorySlotProjection{SlotID: slotID, Status: FactorySlotStatusAllocated})
	return &factory.Slots[len(factory.Slots)-1]
}

func (factory *FactoryStatusProjection) factorySlotByID(slotID string) *FactorySlotProjection {
	if slotID == "" {
		return nil
	}
	for i := range factory.Slots {
		if factory.Slots[i].SlotID == slotID {
			return &factory.Slots[i]
		}
	}
	return nil
}

func (factory *FactoryStatusProjection) factorySlotByJob(jobID string) *FactorySlotProjection {
	for i := len(factory.Slots) - 1; i >= 0; i-- {
		if factory.Slots[i].JobID == jobID {
			return &factory.Slots[i]
		}
	}
	return nil
}

func (factory *FactoryStatusProjection) upsertFactoryValidation(validationID string) *FactoryValidationProjection {
	for i := range factory.Validations {
		if factory.Validations[i].ValidationID == validationID {
			return &factory.Validations[i]
		}
	}
	factory.Validations = append(factory.Validations, FactoryValidationProjection{ValidationID: validationID})
	return &factory.Validations[len(factory.Validations)-1]
}

func (factory *FactoryStatusProjection) upsertFactoryWorktree(worktreeID string) *FactoryWorktreeProjection {
	for i := range factory.Worktrees {
		if factory.Worktrees[i].WorktreeID == worktreeID {
			return &factory.Worktrees[i]
		}
	}
	factory.Worktrees = append(factory.Worktrees, FactoryWorktreeProjection{WorktreeID: worktreeID})
	return &factory.Worktrees[len(factory.Worktrees)-1]
}

func (factory *FactoryStatusProjection) upsertFactoryMergeDecision(jobID string) *FactoryMergeDecisionProjection {
	for i := range factory.MergeDecisions {
		if factory.MergeDecisions[i].JobID == jobID {
			return &factory.MergeDecisions[i]
		}
	}
	factory.MergeDecisions = append(factory.MergeDecisions, FactoryMergeDecisionProjection{JobID: jobID})
	return &factory.MergeDecisions[len(factory.MergeDecisions)-1]
}

func (factory *FactoryStatusProjection) upsertFactoryTerminal(jobID string) *FactoryTerminalProjection {
	for i := range factory.TerminalJobs {
		if factory.TerminalJobs[i].JobID == jobID {
			return &factory.TerminalJobs[i]
		}
	}
	factory.TerminalJobs = append(factory.TerminalJobs, FactoryTerminalProjection{JobID: jobID})
	return &factory.TerminalJobs[len(factory.TerminalJobs)-1]
}

func (factory *FactoryStatusProjection) slotForValidation(event LedgerEvent, validation *FactoryValidationProjection) *FactorySlotProjection {
	if slot := factory.factorySlotByID(validation.SlotID); slot != nil {
		return slot
	}
	return factory.slotForJobOrPayload(event)
}

func (factory *FactoryStatusProjection) slotForJobOrPayload(event LedgerEvent) *FactorySlotProjection {
	if slot := factory.factorySlotByID(factory.slotIDForEvent(event)); slot != nil {
		return slot
	}
	return factory.factorySlotByJob(event.JobID)
}

func (factory *FactoryStatusProjection) worktreeForJobOrSlot(event LedgerEvent) *FactoryWorktreeProjection {
	worktreeID := factory.worktreeIDForEvent(event)
	if worktreeID != "" {
		for i := range factory.Worktrees {
			if factory.Worktrees[i].WorktreeID == worktreeID {
				return &factory.Worktrees[i]
			}
		}
	}
	slotID := factory.slotIDForEvent(event)
	for i := len(factory.Worktrees) - 1; i >= 0; i-- {
		if slotID != "" && factory.Worktrees[i].SlotID == slotID {
			return &factory.Worktrees[i]
		}
		if factory.Worktrees[i].JobID == event.JobID {
			return &factory.Worktrees[i]
		}
	}
	return nil
}

func (factory *FactoryStatusProjection) runIDForEvent(event LedgerEvent) string {
	if runID, ok := stringPayload(event.Payload, "run_id"); ok {
		return runID
	}
	if job := factory.factoryJobByID(event.JobID); job != nil {
		return job.RunID
	}
	if slot := factory.factorySlotByJob(event.JobID); slot != nil {
		return slot.RunID
	}
	return ""
}

func (factory *FactoryStatusProjection) taskIDForEvent(event LedgerEvent) string {
	if taskID, ok := stringPayload(event.Payload, "task_id"); ok {
		return taskID
	}
	if job := factory.factoryJobByID(event.JobID); job != nil {
		return job.TaskID
	}
	if slot := factory.factorySlotByJob(event.JobID); slot != nil {
		return slot.TaskID
	}
	return ""
}

func (factory *FactoryStatusProjection) slotIDForEvent(event LedgerEvent) string {
	if slotID, ok := stringPayload(event.Payload, "slot_id"); ok {
		return slotID
	}
	if ownerSlotID, ok := stringPayload(event.Payload, "owner_slot_id"); ok {
		return ownerSlotID
	}
	if slot := factory.factorySlotByJob(event.JobID); slot != nil {
		return slot.SlotID
	}
	return ""
}

func (factory *FactoryStatusProjection) worktreeIDForEvent(event LedgerEvent) string {
	if worktreeID, ok := stringPayload(event.Payload, "worktree_id"); ok {
		return worktreeID
	}
	if slot := factory.factorySlotByJob(event.JobID); slot != nil {
		return slot.WorktreeID
	}
	return ""
}

func validationIDFromEvent(event LedgerEvent) string {
	if validationID, ok := stringPayload(event.Payload, "validation_id"); ok {
		return validationID
	}
	return event.JobID
}

func routingAuthorityFromPayload(payload map[string]any) (RoutingAuthority, bool, error) {
	value, ok := stringPayload(payload, "authority")
	if !ok {
		return "", false, nil
	}
	authority := RoutingAuthority(value)
	if err := ValidateRoutingAuthority(authority); err != nil {
		return "", true, err
	}
	return authority, true, nil
}

func factoryValidationStatusFromPayload(payload map[string]any) (FactoryValidationStatus, bool, error) {
	value, ok := stringPayload(payload, "status")
	if !ok {
		return "", false, nil
	}
	status := FactoryValidationStatus(value)
	if err := ValidateFactoryValidationStatus(status); err != nil {
		return "", true, err
	}
	return status, true, nil
}

func factoryMergeDecisionFromPayload(payload map[string]any) (FactoryMergeDecision, bool, error) {
	value, ok := stringPayload(payload, "decision")
	if !ok {
		return "", false, nil
	}
	decision := FactoryMergeDecision(value)
	if err := ValidateFactoryMergeDecision(decision); err != nil {
		return "", true, err
	}
	return decision, true, nil
}

func terminalJobStatusFromPayload(payload map[string]any) (JobStatus, bool, error) {
	value, ok := stringPayload(payload, "status")
	if !ok {
		return "", false, nil
	}
	status := JobStatus(value)
	if err := ValidateJobStatus(status); err != nil {
		return "", true, err
	}
	if !isTerminalStatus(status) {
		return "", true, fmt.Errorf("factory terminal status %q is not terminal", value)
	}
	return status, true, nil
}

func boolPayload(payload map[string]any, key string) (bool, bool) {
	raw, ok := payload[key]
	if !ok {
		return false, false
	}
	value, ok := raw.(bool)
	return value, ok
}

func stringSlicePayload(payload map[string]any, key string) []string {
	raw, ok := payload[key]
	if !ok {
		return nil
	}
	switch values := raw.(type) {
	case []string:
		return append([]string{}, values...)
	case []any:
		out := make([]string, 0, len(values))
		for _, value := range values {
			if str, ok := value.(string); ok && str != "" {
				out = append(out, str)
			}
		}
		return out
	case string:
		if values != "" {
			return []string{values}
		}
	}
	return nil
}

func stringMapPayload(payload map[string]any, key string) map[string]string {
	raw, ok := payload[key]
	if !ok {
		return nil
	}
	out := map[string]string{}
	switch values := raw.(type) {
	case map[string]string:
		for k, v := range values {
			if v != "" {
				out[k] = v
			}
		}
	case map[string]any:
		for k, v := range values {
			if str, ok := v.(string); ok && str != "" {
				out[k] = str
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func mapPayload(payload map[string]any, key string) (map[string]any, bool) {
	raw, ok := payload[key]
	if !ok {
		return nil, false
	}
	values, ok := raw.(map[string]any)
	if !ok {
		return nil, false
	}
	return cloneAnyMap(values), true
}

func cloneFactoryProjection(in FactoryStatusProjection) FactoryStatusProjection {
	out := FactoryStatusProjection{
		Admissions:              append([]FactoryAdmissionProjection{}, in.Admissions...),
		Jobs:                    append([]FactoryJobProjection{}, in.Jobs...),
		ActiveWorkers:           append([]FactoryWorkerProjection{}, in.ActiveWorkers...),
		QueueLanes:              append([]FactoryQueueLaneProjection{}, in.QueueLanes...),
		ModelLanes:              append([]FactoryModelLaneProjection{}, in.ModelLanes...),
		BlockedValidations:      append([]FactoryValidationProjection{}, in.BlockedValidations...),
		RetainedFailedWorktrees: append([]FactoryWorktreeProjection{}, in.RetainedFailedWorktrees...),
		MergeDecisions:          append([]FactoryMergeDecisionProjection{}, in.MergeDecisions...),
		PendingManualMerges:     append([]FactoryMergeDecisionProjection{}, in.PendingManualMerges...),
		RecentEvents:            append([]FactoryEventRef{}, in.RecentEvents...),
		Logs:                    append([]FactoryPointer{}, in.Logs...),
		Artifacts:               append([]FactoryPointer{}, in.Artifacts...),
		Transcripts:             append([]FactoryPointer{}, in.Transcripts...),
		Diffs:                   append([]FactoryPointer{}, in.Diffs...),
	}
	for i := range out.Admissions {
		out.Admissions[i].Reasons = append([]string{}, in.Admissions[i].Reasons...)
		out.Admissions[i].Artifacts = cloneStringMap(in.Admissions[i].Artifacts)
	}
	out.Slots = make([]FactorySlotProjection, len(in.Slots))
	for i := range in.Slots {
		out.Slots[i] = in.Slots[i]
		out.Slots[i].ResourcePolicy = cloneAnyMap(in.Slots[i].ResourcePolicy)
	}
	out.Validations = make([]FactoryValidationProjection, len(in.Validations))
	for i := range in.Validations {
		out.Validations[i] = in.Validations[i]
		out.Validations[i].Commands = append([]string{}, in.Validations[i].Commands...)
		out.Validations[i].Artifacts = cloneStringMap(in.Validations[i].Artifacts)
		out.Validations[i].ArtifactRefs = cloneArtifactRefs(in.Validations[i].ArtifactRefs)
	}
	out.Worktrees = append([]FactoryWorktreeProjection{}, in.Worktrees...)
	out.TerminalJobs = make([]FactoryTerminalProjection, len(in.TerminalJobs))
	for i := range in.TerminalJobs {
		out.TerminalJobs[i] = in.TerminalJobs[i]
		out.TerminalJobs[i].ArtifactRefs = cloneArtifactRefs(in.TerminalJobs[i].ArtifactRefs)
	}
	if in.LastRoutingDecision != nil {
		decision := *in.LastRoutingDecision
		out.LastRoutingDecision = &decision
	}
	return out
}

func cloneAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func (set *ProjectionSet) markDegraded(reason string) {
	set.DegradedReasons = append(set.DegradedReasons, reason)
	for name, manifest := range set.Manifests {
		manifest.Status = ProjectionStatusDegraded
		manifest.DegradedReasons = append(manifest.DegradedReasons, reason)
		set.Manifests[name] = manifest
	}
	set.OpenClaw.Status = ProjectionStatusDegraded
}

func defaultProjectionManifests(sourceLedger, rebuiltAt string) map[ProjectionName]ProjectionManifest {
	manifests := make(map[ProjectionName]ProjectionManifest)
	for _, name := range allProjectionNames() {
		manifests[name] = ProjectionManifest{
			SchemaVersion: ProjectionSchemaVersion,
			Projection:    name,
			SourceLedger:  sourceLedger,
			Status:        ProjectionStatusCurrent,
			RebuiltAt:     rebuiltAt,
			OutputPaths:   defaultProjectionOutputPaths(name),
		}
	}
	return manifests
}

func allProjectionNames() []ProjectionName {
	return []ProjectionName{
		ProjectionRPIRegistry,
		ProjectionDreamRuns,
		ProjectionWikiJobs,
		ProjectionOpenClaw,
		ProjectionDaemonStatus,
		ProjectionDaemonJobStatus,
		ProjectionPlansManifest,
	}
}

func defaultProjectionOutputPaths(name ProjectionName) []string {
	switch name {
	case ProjectionRPIRegistry:
		return []string{".agents/rpi/runs/*/phased-state.json"}
	case ProjectionDreamRuns:
		return []string{".agents/overnight/*/summary.json", ".agents/overnight/*/summary.md"}
	case ProjectionWikiJobs:
		return []string{".agents/wiki"}
	case ProjectionOpenClaw:
		return []string{".agents/daemon/projections/openclaw/latest.json"}
	case ProjectionDaemonStatus:
		return []string{".agents/daemon/projections/status.json"}
	case ProjectionDaemonJobStatus:
		return []string{".agents/daemon/projections/jobs.json"}
	case ProjectionPlansManifest:
		return []string{".agents/plans/*/manifest.jsonl"}
	default:
		return nil
	}
}

func defaultProjectionTargetsForJobType(jobType JobType) []ProjectionName {
	targets := []ProjectionName{ProjectionDaemonStatus, ProjectionDaemonJobStatus}
	switch jobType {
	case JobTypeRPIRun, JobTypeRPIPhase:
		targets = append([]ProjectionName{ProjectionRPIRegistry, ProjectionOpenClaw}, targets...)
	case JobTypeDreamRun, JobTypeDreamStage:
		targets = append([]ProjectionName{ProjectionDreamRuns, ProjectionOpenClaw}, targets...)
	case JobTypeWikiBuild, JobTypeWikiForge:
		targets = append([]ProjectionName{ProjectionWikiJobs, ProjectionOpenClaw}, targets...)
	case JobTypeOpenClawSnapshot:
		targets = append([]ProjectionName{ProjectionOpenClaw}, targets...)
	case JobTypePlansProjection:
		targets = append([]ProjectionName{ProjectionPlansManifest}, targets...)
	}
	return targets
}

func projectionTargetStrings(targets []ProjectionName) []string {
	out := make([]string, 0, len(targets))
	for _, target := range targets {
		out = append(out, string(target))
	}
	return out
}

func projectionTargetsFromPayload(payload map[string]any) []ProjectionName {
	raw, ok := payload["projection_targets"]
	if !ok {
		return nil
	}
	var targets []ProjectionName
	switch values := raw.(type) {
	case []ProjectionName:
		return append(targets, values...)
	case []string:
		for _, value := range values {
			targets = append(targets, ProjectionName(value))
		}
	case []any:
		for _, value := range values {
			if str, ok := value.(string); ok {
				targets = append(targets, ProjectionName(str))
			}
		}
	case string:
		targets = append(targets, ProjectionName(values))
	}
	return targets
}

func jobTypeFromPayload(payload map[string]any) (JobType, bool, error) {
	raw, ok := payload["job_type"]
	if !ok {
		return "", false, nil
	}
	value, ok := raw.(string)
	if !ok || value == "" {
		return "", false, fmt.Errorf("payload job_type must be a non-empty string")
	}
	jobType := JobType(value)
	if err := ValidateJobType(jobType); err != nil {
		return "", false, err
	}
	return jobType, true, nil
}

func appendRequestID(job *JobProjection, requestID string) {
	for _, existing := range job.RequestIDs {
		if existing == requestID {
			return
		}
	}
	job.RequestIDs = append(job.RequestIDs, requestID)
}

func artifactsFromPayload(payload map[string]any) map[string]string {
	raw, ok := payload["artifacts"]
	if !ok {
		return nil
	}
	out := map[string]string{}
	switch values := raw.(type) {
	case map[string]string:
		for key, value := range values {
			out[key] = value
		}
	case map[string]any:
		for key, value := range values {
			if str, ok := value.(string); ok {
				out[key] = str
			}
		}
	}
	return out
}

func failureFromPayload(payload map[string]any) JobFailure {
	failure := JobFailure{Code: FailureRequestRejected}
	if raw, ok := payload["failure"]; ok {
		if values, ok := raw.(map[string]any); ok {
			if code, ok := values["code"].(string); ok && code != "" {
				failure.Code = FailureCode(code)
			}
			if message, ok := values["message"].(string); ok {
				failure.Message = message
			}
			if retryable, ok := values["retryable"].(bool); ok {
				failure.Retryable = retryable
			}
		}
	}
	if code, ok := payload["failure_code"].(string); ok && code != "" {
		failure.Code = FailureCode(code)
	}
	if message, ok := payload["message"].(string); ok {
		failure.Message = message
	}
	if retryable, ok := payload["retryable"].(bool); ok {
		failure.Retryable = retryable
	}
	return failure
}
