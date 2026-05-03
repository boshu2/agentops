package daemon

import (
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
	EventJobAccepted:           {},
	EventJobClaimed:            {},
	EventJobHeartbeat:          {},
	EventJobLeaseExpired:       {},
	EventJobCompleted:          {},
	EventJobFailed:             {},
	EventJobCancelled:          {},
	EventProjectionMarkedStale: {},
	EventProjectionRebuilt:     {},
	EventScheduleCreated:       {},
	EventScheduleFired:         {},
	EventScheduleSkipped:       {},
	EventScheduleDeleted:       {},
}

func isScheduleEvent(t EventType) bool {
	switch t {
	case EventScheduleCreated, EventScheduleFired, EventScheduleSkipped, EventScheduleDeleted:
		return true
	}
	return false
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
		}
		jobsByID = map[string]*JobProjection{}
		_, err = applyEventsToState(events, &set, jobsByID, &jobOrder)
	}
	if err != nil {
		return ProjectionSet{}, err
	}

	collectJobsIntoSet(jobsByID, jobOrder, &set)
	finalizeManifests(&set)
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
