package daemon

import (
	"fmt"
	"path/filepath"
	"time"
)

const ProjectionSchemaVersion = 1

type ProjectionName string

const (
	ProjectionRPIRegistry     ProjectionName = "rpi-registry"
	ProjectionDreamRuns       ProjectionName = "dream-runs"
	ProjectionWikiJobs        ProjectionName = "wiki-jobs"
	ProjectionOpenClaw        ProjectionName = "openclaw-snapshot"
	ProjectionDaemonStatus    ProjectionName = "daemon-status"
	ProjectionDaemonJobStatus ProjectionName = "daemon-job-status"
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
	JobID             string            `json:"job_id"`
	JobType           JobType           `json:"job_type,omitempty"`
	RequestID         string            `json:"request_id"`
	RequestIDs        []string          `json:"request_ids,omitempty"`
	Status            JobStatus         `json:"status"`
	ResultStatus      JobResultStatus   `json:"result_status,omitempty"`
	Failure           *JobFailure       `json:"failure,omitempty"`
	Artifacts         map[string]string `json:"artifacts,omitempty"`
	ProjectionTargets []ProjectionName  `json:"projection_targets,omitempty"`
	CreatedAt         string            `json:"created_at,omitempty"`
	UpdatedAt         string            `json:"updated_at,omitempty"`
	LastEventID       string            `json:"last_event_id,omitempty"`
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

func RebuildProjections(events []LedgerEvent, opts ProjectionRebuildOptions) (ProjectionSet, error) {
	if opts.RebuiltAt.IsZero() {
		opts.RebuiltAt = time.Now().UTC()
	}
	if opts.SourceLedger == "" {
		opts.SourceLedger = filepath.ToSlash(filepath.Join(StoreDirRel, LedgerFileName))
	}
	rebuiltAt := opts.RebuiltAt.UTC().Format(time.RFC3339Nano)
	set := ProjectionSet{
		SchemaVersion: ProjectionSchemaVersion,
		RebuiltAt:     rebuiltAt,
		SourceLedger:  opts.SourceLedger,
		Manifests:     defaultProjectionManifests(opts.SourceLedger, rebuiltAt),
	}

	jobsByID := map[string]*JobProjection{}
	var jobOrder []string
	for _, event := range events {
		normalized, err := NormalizeLedgerEvent(event)
		if err != nil {
			return ProjectionSet{}, fmt.Errorf("event %q: %w", event.EventID, err)
		}
		event = normalized
		set.LastEventID = event.EventID
		if event.EventType == EventProjectionMarkedStale || event.EventType == EventProjectionRebuilt {
			set.applyProjectionLifecycleEvent(event)
			continue
		}

		job := jobsByID[event.JobID]
		if job == nil {
			job = &JobProjection{
				JobID:      event.JobID,
				RequestID:  event.RequestID,
				RequestIDs: []string{event.RequestID},
				Status:     JobStatusQueued,
				Artifacts:  map[string]string{},
			}
			jobsByID[event.JobID] = job
			jobOrder = append(jobOrder, event.JobID)
		}
		appendRequestID(job, event.RequestID)
		if job.RequestID == "" {
			job.RequestID = event.RequestID
		}
		if job.CreatedAt == "" && event.EventType == EventJobAccepted {
			job.CreatedAt = event.OccurredAt
		}
		job.UpdatedAt = event.OccurredAt
		job.LastEventID = event.EventID

		if jobType, ok, err := jobTypeFromPayload(event.Payload); err != nil {
			return ProjectionSet{}, fmt.Errorf("event %q: %w", event.EventID, err)
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
		applyEventToJobProjection(job, event)
	}

	set.Jobs = make([]JobProjection, 0, len(jobOrder))
	for _, jobID := range jobOrder {
		job := *jobsByID[jobID]
		if len(job.Artifacts) == 0 {
			job.Artifacts = nil
		}
		set.Jobs = append(set.Jobs, job)
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
		set.OpenClaw.Resources.Jobs = append(set.OpenClaw.Resources.Jobs, job)
	}
	for name, manifest := range set.Manifests {
		manifest.LastEventID = set.LastEventID
		set.Manifests[name] = manifest
	}
	set.OpenClaw.SchemaVersion = ProjectionSchemaVersion
	set.OpenClaw.SnapshotID = "snap_empty"
	if set.LastEventID != "" {
		set.OpenClaw.SnapshotID = "snap_" + set.LastEventID
	}
	set.OpenClaw.GeneratedAt = rebuiltAt
	set.OpenClaw.Source = ProjectionSource{Ledger: opts.SourceLedger, LastEventID: set.LastEventID}
	set.OpenClaw.Status = set.Manifests[ProjectionOpenClaw].Status
	return set, nil
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
