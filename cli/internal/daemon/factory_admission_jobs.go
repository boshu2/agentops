package daemon

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const FactoryAdmissionJobSpecSchemaVersion = 1

type FactoryAdmissionMode string

const (
	FactoryAdmissionModeAdmissionOnly FactoryAdmissionMode = "admission-only"
	FactoryAdmissionModeRPIHandoff    FactoryAdmissionMode = "rpi-handoff"
)

type FactoryLandingPolicy string

const (
	FactoryLandingPolicyOff      FactoryLandingPolicy = "off"
	FactoryLandingPolicyManualPR FactoryLandingPolicy = "manual_pr"
)

type FactoryDigestPolicy string

const (
	FactoryDigestPolicyRequired FactoryDigestPolicy = "required"
)

type FactoryUnknownEvidencePolicy string

const (
	FactoryUnknownEvidenceBlock            FactoryUnknownEvidencePolicy = "block"
	FactoryUnknownEvidenceAllowNonMutating FactoryUnknownEvidencePolicy = "allow_non_mutating"
)

type FactoryTargetType string

const (
	FactoryTargetGoal            FactoryTargetType = "goal"
	FactoryTargetBead            FactoryTargetType = "bead"
	FactoryTargetExecutionPacket FactoryTargetType = "execution_packet"
)

type FactoryCIStatus string

const (
	FactoryCIStatusGreen   FactoryCIStatus = "green"
	FactoryCIStatusRed     FactoryCIStatus = "red"
	FactoryCIStatusUnknown FactoryCIStatus = "unknown"
)

type FactoryHandoffKind string

const (
	FactoryHandoffNone FactoryHandoffKind = "none"
	FactoryHandoffRPI  FactoryHandoffKind = "rpi.run"
)

type FactoryAdmissionJobSpec struct {
	SchemaVersion int                  `json:"schema_version"`
	JobType       JobType              `json:"job_type"`
	RunID         string               `json:"run_id"`
	Mode          FactoryAdmissionMode `json:"mode"`
	WorkOrder     FactoryWorkOrder     `json:"work_order"`
	Handoff       FactoryHandoff       `json:"handoff,omitempty"`
}

type FactoryLocalPilotJobSpec struct {
	SchemaVersion int                  `json:"schema_version"`
	JobType       JobType              `json:"job_type"`
	RunID         string               `json:"run_id"`
	Mode          FactoryAdmissionMode `json:"mode"`
	WorkOrder     FactoryWorkOrder     `json:"work_order"`
	Handoff       FactoryHandoff       `json:"handoff,omitempty"`
	MaxCycles     int                  `json:"max_cycles,omitempty"`
}

type FactoryWorkOrder struct {
	SchemaVersion         int                          `json:"schema_version"`
	WorkOrderID           string                       `json:"work_order_id"`
	GeneratedAt           string                       `json:"generated_at"`
	ExpiresAt             string                       `json:"expires_at"`
	BaseSHA               string                       `json:"base_sha"`
	Target                FactoryTarget                `json:"target"`
	AllowedFiles          []string                     `json:"allowed_files"`
	ValidationCommands    []string                     `json:"validation_commands"`
	LandingPolicy         FactoryLandingPolicy         `json:"landing_policy"`
	DigestPolicy          FactoryDigestPolicy          `json:"digest_policy"`
	OpenPRBlockers        []FactoryOpenPRBlocker       `json:"open_pr_blockers"`
	MainCIBaseline        FactoryMainCIBaseline        `json:"main_ci_baseline"`
	UnknownEvidencePolicy FactoryUnknownEvidencePolicy `json:"unknown_evidence_policy,omitempty"`
}

type FactoryTarget struct {
	Type    FactoryTargetType `json:"type"`
	ID      string            `json:"id"`
	Summary string            `json:"summary"`
}

type FactoryOpenPRBlocker struct {
	PRNumber int      `json:"pr_number"`
	HeadRef  string   `json:"head_ref"`
	Files    []string `json:"files"`
}

type FactoryMainCIBaseline struct {
	Status     FactoryCIStatus `json:"status"`
	RunID      string          `json:"run_id,omitempty"`
	CheckedAt  string          `json:"checked_at"`
	FailedJobs []string        `json:"failed_jobs,omitempty"`
}

type FactoryHandoff struct {
	Kind                FactoryHandoffKind `json:"kind,omitempty"`
	ExecutionPacketPath string             `json:"execution_packet_path,omitempty"`
	EpicID              string             `json:"epic_id,omitempty"`
}

type FactoryAdmissionDecision struct {
	SchemaVersion int                     `json:"schema_version"`
	WorkOrderID   string                  `json:"work_order_id"`
	RunID         string                  `json:"run_id"`
	EvaluatedAt   string                  `json:"evaluated_at"`
	Allowed       bool                    `json:"allowed"`
	Reasons       []string                `json:"reasons"`
	LandingPolicy FactoryLandingPolicy    `json:"landing_policy"`
	DigestPolicy  FactoryDigestPolicy     `json:"digest_policy"`
	ChildJobID    string                  `json:"child_job_id,omitempty"`
	ArtifactRefs  map[string]string       `json:"artifact_refs,omitempty"`
	Evidence      FactoryDecisionEvidence `json:"evidence"`
}

type FactoryDecisionEvidence struct {
	BaseSHA            string          `json:"base_sha"`
	OpenPRBlockerCount int             `json:"open_pr_blocker_count"`
	MainCIStatus       FactoryCIStatus `json:"main_ci_status"`
	Stale              bool            `json:"stale,omitempty"`
}

func NewFactoryAdmissionJobSpec(runID string, workOrder FactoryWorkOrder) FactoryAdmissionJobSpec {
	return FactoryAdmissionJobSpec{
		SchemaVersion: FactoryAdmissionJobSpecSchemaVersion,
		JobType:       JobTypeFactoryAdmission,
		RunID:         runID,
		Mode:          FactoryAdmissionModeAdmissionOnly,
		WorkOrder:     workOrder,
		Handoff:       FactoryHandoff{Kind: FactoryHandoffNone},
	}
}

func NewFactoryLocalPilotJobSpec(runID string, workOrder FactoryWorkOrder) FactoryLocalPilotJobSpec {
	return FactoryLocalPilotJobSpec{
		SchemaVersion: FactoryAdmissionJobSpecSchemaVersion,
		JobType:       JobTypeFactoryLocalPilot,
		RunID:         runID,
		Mode:          FactoryAdmissionModeAdmissionOnly,
		WorkOrder:     workOrder,
		Handoff:       FactoryHandoff{Kind: FactoryHandoffNone},
	}
}

func (spec FactoryAdmissionJobSpec) Validate() error {
	if err := validateFactoryAdmissionSpecHeader(spec.SchemaVersion, spec.JobType, JobTypeFactoryAdmission, spec.RunID, spec.Mode); err != nil {
		return err
	}
	if err := spec.WorkOrder.Validate(); err != nil {
		return fmt.Errorf("work_order: %w", err)
	}
	return spec.Handoff.Validate()
}

func (spec FactoryLocalPilotJobSpec) Validate() error {
	if err := validateFactoryAdmissionSpecHeader(spec.SchemaVersion, spec.JobType, JobTypeFactoryLocalPilot, spec.RunID, spec.Mode); err != nil {
		return err
	}
	if spec.MaxCycles < 0 {
		return fmt.Errorf("max_cycles must be >= 0")
	}
	if err := spec.WorkOrder.Validate(); err != nil {
		return fmt.Errorf("work_order: %w", err)
	}
	return spec.Handoff.Validate()
}

func validateFactoryAdmissionSpecHeader(schemaVersion int, got, want JobType, runID string, mode FactoryAdmissionMode) error {
	if schemaVersion != FactoryAdmissionJobSpecSchemaVersion {
		return fmt.Errorf("schema_version mismatch: got %d want %d", schemaVersion, FactoryAdmissionJobSpecSchemaVersion)
	}
	if got != want {
		return fmt.Errorf("job_type = %q, want %q", got, want)
	}
	if strings.TrimSpace(runID) == "" {
		return fmt.Errorf("run_id is required")
	}
	return ValidateFactoryAdmissionMode(mode)
}

func (work FactoryWorkOrder) Validate() error {
	if work.SchemaVersion != FactoryAdmissionJobSpecSchemaVersion {
		return fmt.Errorf("schema_version mismatch: got %d want %d", work.SchemaVersion, FactoryAdmissionJobSpecSchemaVersion)
	}
	if strings.TrimSpace(work.WorkOrderID) == "" {
		return fmt.Errorf("work_order_id is required")
	}
	generatedAt, err := parseFactoryAdmissionTime("generated_at", work.GeneratedAt)
	if err != nil {
		return err
	}
	expiresAt, err := parseFactoryAdmissionTime("expires_at", work.ExpiresAt)
	if err != nil {
		return err
	}
	if !expiresAt.After(generatedAt) {
		return fmt.Errorf("expires_at must be after generated_at")
	}
	if strings.TrimSpace(work.BaseSHA) == "" {
		return fmt.Errorf("base_sha is required")
	}
	if err := work.Target.Validate(); err != nil {
		return fmt.Errorf("target: %w", err)
	}
	if err := validateRelativePathList("allowed_files", work.AllowedFiles); err != nil {
		return err
	}
	if err := validateNonEmptyStringList("validation_commands", work.ValidationCommands); err != nil {
		return err
	}
	if err := ValidateFactoryLandingPolicy(work.LandingPolicy); err != nil {
		return err
	}
	if err := ValidateFactoryDigestPolicy(work.DigestPolicy); err != nil {
		return err
	}
	if work.UnknownEvidencePolicy != "" {
		if err := ValidateFactoryUnknownEvidencePolicy(work.UnknownEvidencePolicy); err != nil {
			return err
		}
	}
	if work.OpenPRBlockers == nil {
		return fmt.Errorf("open_pr_blockers is required")
	}
	for i, blocker := range work.OpenPRBlockers {
		if err := blocker.Validate(); err != nil {
			return fmt.Errorf("open_pr_blockers[%d]: %w", i, err)
		}
	}
	return work.MainCIBaseline.Validate()
}

func (target FactoryTarget) Validate() error {
	if err := ValidateFactoryTargetType(target.Type); err != nil {
		return err
	}
	if strings.TrimSpace(target.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(target.Summary) == "" {
		return fmt.Errorf("summary is required")
	}
	return nil
}

func (blocker FactoryOpenPRBlocker) Validate() error {
	if blocker.PRNumber <= 0 {
		return fmt.Errorf("pr_number must be > 0")
	}
	if strings.TrimSpace(blocker.HeadRef) == "" {
		return fmt.Errorf("head_ref is required")
	}
	return validateRelativePathList("files", blocker.Files)
}

func (baseline FactoryMainCIBaseline) Validate() error {
	if err := ValidateFactoryCIStatus(baseline.Status); err != nil {
		return err
	}
	if _, err := parseFactoryAdmissionTime("checked_at", baseline.CheckedAt); err != nil {
		return err
	}
	return validateNonEmptyStringListAllowEmpty("failed_jobs", baseline.FailedJobs)
}

func (handoff FactoryHandoff) Validate() error {
	kind := handoff.Kind
	if kind == "" {
		kind = FactoryHandoffNone
	}
	if err := ValidateFactoryHandoffKind(kind); err != nil {
		return err
	}
	if kind == FactoryHandoffRPI && strings.TrimSpace(handoff.ExecutionPacketPath) == "" {
		return fmt.Errorf("execution_packet_path is required for rpi.run handoff")
	}
	return nil
}

func (decision FactoryAdmissionDecision) Validate() error {
	if decision.SchemaVersion != FactoryAdmissionJobSpecSchemaVersion {
		return fmt.Errorf("schema_version mismatch: got %d want %d", decision.SchemaVersion, FactoryAdmissionJobSpecSchemaVersion)
	}
	if strings.TrimSpace(decision.WorkOrderID) == "" {
		return fmt.Errorf("work_order_id is required")
	}
	if strings.TrimSpace(decision.RunID) == "" {
		return fmt.Errorf("run_id is required")
	}
	if _, err := parseFactoryAdmissionTime("evaluated_at", decision.EvaluatedAt); err != nil {
		return err
	}
	if !decision.Allowed && len(decision.Reasons) == 0 {
		return fmt.Errorf("reasons are required when allowed is false")
	}
	if err := validateNonEmptyStringListAllowEmpty("reasons", decision.Reasons); err != nil {
		return err
	}
	if err := ValidateFactoryLandingPolicy(decision.LandingPolicy); err != nil {
		return err
	}
	if err := ValidateFactoryDigestPolicy(decision.DigestPolicy); err != nil {
		return err
	}
	return decision.Evidence.Validate()
}

func (evidence FactoryDecisionEvidence) Validate() error {
	if strings.TrimSpace(evidence.BaseSHA) == "" {
		return fmt.Errorf("base_sha is required")
	}
	if evidence.OpenPRBlockerCount < 0 {
		return fmt.Errorf("open_pr_blocker_count must be >= 0")
	}
	return ValidateFactoryCIStatus(evidence.MainCIStatus)
}

func (spec FactoryAdmissionJobSpec) ToJobSpec(jobID string) (JobSpec, error) {
	if err := spec.Validate(); err != nil {
		return JobSpec{}, err
	}
	payload, err := structToMap(spec)
	if err != nil {
		return JobSpec{}, err
	}
	return JobSpec{ID: jobID, Type: JobTypeFactoryAdmission, Payload: payload}, nil
}

func (spec FactoryLocalPilotJobSpec) ToJobSpec(jobID string) (JobSpec, error) {
	if err := spec.Validate(); err != nil {
		return JobSpec{}, err
	}
	payload, err := structToMap(spec)
	if err != nil {
		return JobSpec{}, err
	}
	return JobSpec{ID: jobID, Type: JobTypeFactoryLocalPilot, Payload: payload}, nil
}

func FactoryAdmissionJobSpecFromPayload(payload map[string]any) (FactoryAdmissionJobSpec, error) {
	var spec FactoryAdmissionJobSpec
	if err := mapToStruct(payload, &spec); err != nil {
		return spec, err
	}
	if err := spec.Validate(); err != nil {
		return spec, err
	}
	return spec, nil
}

func FactoryLocalPilotJobSpecFromPayload(payload map[string]any) (FactoryLocalPilotJobSpec, error) {
	var spec FactoryLocalPilotJobSpec
	if err := mapToStruct(payload, &spec); err != nil {
		return spec, err
	}
	if err := spec.Validate(); err != nil {
		return spec, err
	}
	return spec, nil
}

func ValidateFactoryAdmissionMode(mode FactoryAdmissionMode) error {
	switch mode {
	case FactoryAdmissionModeAdmissionOnly, FactoryAdmissionModeRPIHandoff:
		return nil
	default:
		return fmt.Errorf("invalid factory admission mode %q", mode)
	}
}

func ValidateFactoryLandingPolicy(policy FactoryLandingPolicy) error {
	switch policy {
	case FactoryLandingPolicyOff, FactoryLandingPolicyManualPR:
		return nil
	default:
		return fmt.Errorf("invalid factory landing policy %q", policy)
	}
}

func ValidateFactoryDigestPolicy(policy FactoryDigestPolicy) error {
	switch policy {
	case FactoryDigestPolicyRequired:
		return nil
	default:
		return fmt.Errorf("invalid factory digest policy %q", policy)
	}
}

func ValidateFactoryUnknownEvidencePolicy(policy FactoryUnknownEvidencePolicy) error {
	switch policy {
	case FactoryUnknownEvidenceBlock, FactoryUnknownEvidenceAllowNonMutating:
		return nil
	default:
		return fmt.Errorf("invalid factory unknown evidence policy %q", policy)
	}
}

func ValidateFactoryTargetType(targetType FactoryTargetType) error {
	switch targetType {
	case FactoryTargetGoal, FactoryTargetBead, FactoryTargetExecutionPacket:
		return nil
	default:
		return fmt.Errorf("invalid factory target type %q", targetType)
	}
}

func ValidateFactoryCIStatus(status FactoryCIStatus) error {
	switch status {
	case FactoryCIStatusGreen, FactoryCIStatusRed, FactoryCIStatusUnknown:
		return nil
	default:
		return fmt.Errorf("invalid factory CI status %q", status)
	}
}

func ValidateFactoryHandoffKind(kind FactoryHandoffKind) error {
	switch kind {
	case FactoryHandoffNone, FactoryHandoffRPI:
		return nil
	default:
		return fmt.Errorf("invalid factory handoff kind %q", kind)
	}
}

func parseFactoryAdmissionTime(field, value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("%s is required", field)
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s is invalid: %w", field, err)
	}
	return parsed, nil
}

func validateRelativePathList(field string, values []string) error {
	if len(values) == 0 {
		return fmt.Errorf("%s are required", field)
	}
	for i, value := range values {
		if err := validateRelativePath(value); err != nil {
			return fmt.Errorf("%s[%d]: %w", field, i, err)
		}
	}
	return nil
}

func validateRelativePath(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("path is required")
	}
	if filepath.IsAbs(trimmed) {
		return fmt.Errorf("path %q must be relative", value)
	}
	if containsParentPathSegment(trimmed) {
		return fmt.Errorf("path %q must not contain parent-directory segments", value)
	}
	clean := filepath.Clean(trimmed)
	if clean == "." {
		return fmt.Errorf("path %q must not escape the repository", value)
	}
	return nil
}

func containsParentPathSegment(value string) bool {
	normalized := strings.ReplaceAll(value, "\\", "/")
	for _, segment := range strings.Split(normalized, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

func validateNonEmptyStringList(field string, values []string) error {
	if len(values) == 0 {
		return fmt.Errorf("%s are required", field)
	}
	return validateNonEmptyStringListAllowEmpty(field, values)
}

func validateNonEmptyStringListAllowEmpty(field string, values []string) error {
	for i, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s[%d] is required", field, i)
		}
	}
	return nil
}
