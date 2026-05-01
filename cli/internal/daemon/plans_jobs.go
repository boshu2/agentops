package daemon

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PlansProjectionJobSpecSchemaVersion is the schema version for
// PlansProjectionJobSpec round-trips. Bumped when the spec shape changes.
const PlansProjectionJobSpecSchemaVersion = 1

// PlansProjectionRefreshTrigger names how the daemon decided to (re)run the
// projection. Used for diagnostics and ledger correlation.
type PlansProjectionRefreshTrigger string

const (
	PlansProjectionTriggerManual       PlansProjectionRefreshTrigger = "manual"
	PlansProjectionTriggerInterval     PlansProjectionRefreshTrigger = "interval"
	PlansProjectionTriggerSubscription PlansProjectionRefreshTrigger = "subscription"
)

var plansProjectionTriggerSet = map[string]struct{}{
	string(PlansProjectionTriggerManual):       {},
	string(PlansProjectionTriggerInterval):     {},
	string(PlansProjectionTriggerSubscription): {},
}

// PlansProjectionJobSpec is the payload contract for plans.projection jobs.
// Mirrors the rpi/dream spec round-trip pattern.
type PlansProjectionJobSpec struct {
	SchemaVersion  int                           `json:"schema_version"`
	JobType        JobType                       `json:"job_type"`
	ProjectID      string                        `json:"project_id"`
	IssuePrefix    string                        `json:"issue_prefix"`
	RefreshTrigger PlansProjectionRefreshTrigger `json:"refresh_trigger"`
	OutputDir      string                        `json:"output_dir"`
}

// NewPlansProjectionJobSpec builds a spec with default schema/job_type values.
func NewPlansProjectionJobSpec(projectID, issuePrefix, outputDir string) PlansProjectionJobSpec {
	return PlansProjectionJobSpec{
		SchemaVersion:  PlansProjectionJobSpecSchemaVersion,
		JobType:        JobTypePlansProjection,
		ProjectID:      projectID,
		IssuePrefix:    issuePrefix,
		RefreshTrigger: PlansProjectionTriggerManual,
		OutputDir:      outputDir,
	}
}

// Validate enforces the spec contract for submission and replay.
func (spec PlansProjectionJobSpec) Validate() error {
	if spec.SchemaVersion != PlansProjectionJobSpecSchemaVersion {
		return fmt.Errorf("schema_version mismatch: got %d want %d", spec.SchemaVersion, PlansProjectionJobSpecSchemaVersion)
	}
	if spec.JobType != JobTypePlansProjection {
		return fmt.Errorf("job_type = %q, want %q", spec.JobType, JobTypePlansProjection)
	}
	if strings.TrimSpace(spec.ProjectID) == "" {
		return fmt.Errorf("project_id is required")
	}
	if strings.TrimSpace(spec.OutputDir) == "" {
		return fmt.Errorf("output_dir is required")
	}
	if spec.RefreshTrigger == "" {
		return fmt.Errorf("refresh_trigger is required")
	}
	return validateStringEnum("refresh trigger", string(spec.RefreshTrigger), plansProjectionTriggerSet)
}

// IdempotencyKey returns the singleton-per-project key used by the queue to
// collapse duplicate submissions per foundation §1 idempotency rule.
func (spec PlansProjectionJobSpec) IdempotencyKey() string {
	return fmt.Sprintf("plans.projection:%s:%d", spec.ProjectID, spec.SchemaVersion)
}

// PlansProjectionJobSpecFromPayload parses a JobSpec.Payload back into a typed
// spec. Returns an error if required fields are missing or invalid.
func PlansProjectionJobSpecFromPayload(payload map[string]any) (PlansProjectionJobSpec, error) {
	var spec PlansProjectionJobSpec
	raw, err := json.Marshal(payload)
	if err != nil {
		return spec, fmt.Errorf("plans.projection payload marshal: %w", err)
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		return spec, fmt.Errorf("plans.projection payload unmarshal: %w", err)
	}
	if err := spec.Validate(); err != nil {
		return spec, err
	}
	return spec, nil
}
