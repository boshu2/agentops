package openclaw

const TriggerJobsPath = "/openclaw/v1/triggers/jobs"

type HealthResponse struct {
	Status          string         `json:"status"`
	Ready           bool           `json:"ready"`
	SnapshotID      string         `json:"snapshot_id"`
	GeneratedAt     string         `json:"generated_at"`
	Source          SnapshotSource `json:"source"`
	SnapshotStatus  SnapshotStatus `json:"snapshot_status"`
	ResourceCounts  ResourceCounts `json:"resource_counts"`
	DegradedReasons []string       `json:"degraded_reasons,omitempty"`
}

type ResourceCounts struct {
	Runs int `json:"runs"`
	Jobs int `json:"jobs"`
	Wiki int `json:"wiki"`
}

type RunsResponse struct {
	Runs []ResourceSummary `json:"runs"`
}

type JobsResponse struct {
	Jobs []ResourceSummary `json:"jobs"`
}

type WikiResponse struct {
	Wiki []ResourceSummary `json:"wiki"`
}

type TriggerJobRequest struct {
	RequestID      string         `json:"request_id,omitempty"`
	JobID          string         `json:"job_id,omitempty"`
	JobType        string         `json:"job_type"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	Payload        map[string]any `json:"payload,omitempty"`
}

type TriggerJobResponse struct {
	Accepted       bool           `json:"accepted"`
	RequestID      string         `json:"request_id"`
	JobID          string         `json:"job_id"`
	JobType        string         `json:"job_type"`
	Status         string         `json:"status"`
	LastEventID    string         `json:"last_event_id,omitempty"`
	SnapshotStatus SnapshotStatus `json:"snapshot_status"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
}
