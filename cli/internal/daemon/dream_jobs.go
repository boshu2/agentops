package daemon

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const DreamJobSpecSchemaVersion = 1

type DreamMode string

const (
	DreamModeDaemon  DreamMode = "daemon"
	DreamModeOneShot DreamMode = "one-shot"
)

type DreamStage string

const (
	DreamStageIngest  DreamStage = "ingest"
	DreamStageReduce  DreamStage = "reduce"
	DreamStageMeasure DreamStage = "measure"
	DreamStageCommit  DreamStage = "commit"
	DreamStageReport  DreamStage = "report"
)

type DreamRunJobSpec struct {
	SchemaVersion    int       `json:"schema_version"`
	JobType          JobType   `json:"job_type"`
	DreamRunID       string    `json:"dream_run_id"`
	Goal             string    `json:"goal,omitempty"`
	Mode             DreamMode `json:"mode"`
	OutputDir        string    `json:"output_dir"`
	MaxIterations    int       `json:"max_iterations,omitempty"`
	ExecutionTimeout string    `json:"execution_timeout,omitempty"`
}

type DreamStageJobSpec struct {
	SchemaVersion int        `json:"schema_version"`
	JobType       JobType    `json:"job_type"`
	DreamRunID    string     `json:"dream_run_id"`
	IterationID   string     `json:"iteration_id,omitempty"`
	Iteration     int        `json:"iteration,omitempty"`
	Stage         DreamStage `json:"stage"`
	Mode          DreamMode  `json:"mode"`
	OutputDir     string     `json:"output_dir"`
	CheckpointDir string     `json:"checkpoint_dir,omitempty"`
	ParentJobID   string     `json:"parent_job_id,omitempty"`
}

type DreamStageManifest struct {
	SchemaVersion int               `json:"schema_version"`
	DreamRunID    string            `json:"dream_run_id"`
	Mode          DreamMode         `json:"mode"`
	OutputDir     string            `json:"output_dir"`
	Stages        []DreamStageEntry `json:"stages"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type DreamStageEntry struct {
	Stage       DreamStage `json:"stage"`
	JobID       string     `json:"job_id,omitempty"`
	IterationID string     `json:"iteration_id,omitempty"`
	Required    bool       `json:"required"`
}

func NewDreamRunJobSpec(dreamRunID, outputDir string) DreamRunJobSpec {
	return DreamRunJobSpec{
		SchemaVersion: DreamJobSpecSchemaVersion,
		JobType:       JobTypeDreamRun,
		DreamRunID:    dreamRunID,
		Mode:          DreamModeDaemon,
		OutputDir:     outputDir,
	}
}

func NewDreamStageJobSpec(dreamRunID, outputDir string, stage DreamStage) DreamStageJobSpec {
	return DreamStageJobSpec{
		SchemaVersion: DreamJobSpecSchemaVersion,
		JobType:       JobTypeDreamStage,
		DreamRunID:    dreamRunID,
		Stage:         stage,
		Mode:          DreamModeDaemon,
		OutputDir:     outputDir,
	}
}

func DefaultDreamStageManifest(dreamRunID, outputDir string) DreamStageManifest {
	stages := []DreamStage{DreamStageIngest, DreamStageReduce, DreamStageMeasure, DreamStageCommit, DreamStageReport}
	entries := make([]DreamStageEntry, 0, len(stages))
	for _, stage := range stages {
		entries = append(entries, DreamStageEntry{Stage: stage, Required: true})
	}
	return DreamStageManifest{
		SchemaVersion: DreamJobSpecSchemaVersion,
		DreamRunID:    dreamRunID,
		Mode:          DreamModeDaemon,
		OutputDir:     outputDir,
		Stages:        entries,
	}
}

func (spec DreamRunJobSpec) Validate() error {
	if spec.SchemaVersion != DreamJobSpecSchemaVersion {
		return fmt.Errorf("schema_version mismatch: got %d want %d", spec.SchemaVersion, DreamJobSpecSchemaVersion)
	}
	if spec.JobType != JobTypeDreamRun {
		return fmt.Errorf("job_type = %q, want %q", spec.JobType, JobTypeDreamRun)
	}
	if strings.TrimSpace(spec.DreamRunID) == "" {
		return fmt.Errorf("dream_run_id is required")
	}
	if err := validateOutputDir("output_dir", spec.OutputDir); err != nil {
		return err
	}
	if spec.MaxIterations < 0 {
		return fmt.Errorf("max_iterations must be >= 0")
	}
	if err := validateOptionalDuration("execution_timeout", spec.ExecutionTimeout); err != nil {
		return err
	}
	return ValidateDreamMode(spec.Mode)
}

func (spec DreamStageJobSpec) Validate() error {
	if spec.SchemaVersion != DreamJobSpecSchemaVersion {
		return fmt.Errorf("schema_version mismatch: got %d want %d", spec.SchemaVersion, DreamJobSpecSchemaVersion)
	}
	if spec.JobType != JobTypeDreamStage {
		return fmt.Errorf("job_type = %q, want %q", spec.JobType, JobTypeDreamStage)
	}
	if strings.TrimSpace(spec.DreamRunID) == "" {
		return fmt.Errorf("dream_run_id is required")
	}
	if err := validateOutputDir("output_dir", spec.OutputDir); err != nil {
		return err
	}
	if spec.Iteration < 0 {
		return fmt.Errorf("iteration must be >= 0")
	}
	if err := ValidateDreamStage(spec.Stage); err != nil {
		return err
	}
	return ValidateDreamMode(spec.Mode)
}

func (manifest DreamStageManifest) Validate() error {
	if manifest.SchemaVersion != DreamJobSpecSchemaVersion {
		return fmt.Errorf("schema_version mismatch: got %d want %d", manifest.SchemaVersion, DreamJobSpecSchemaVersion)
	}
	if strings.TrimSpace(manifest.DreamRunID) == "" {
		return fmt.Errorf("dream_run_id is required")
	}
	if err := validateOutputDir("output_dir", manifest.OutputDir); err != nil {
		return err
	}
	if err := ValidateDreamMode(manifest.Mode); err != nil {
		return err
	}
	if len(manifest.Stages) == 0 {
		return fmt.Errorf("stages are required")
	}
	lastOrder := 0
	seen := map[DreamStage]struct{}{}
	for _, entry := range manifest.Stages {
		if err := ValidateDreamStage(entry.Stage); err != nil {
			return err
		}
		order := dreamStageOrder(entry.Stage)
		if order < lastOrder {
			return fmt.Errorf("stage %q appears out of order", entry.Stage)
		}
		lastOrder = order
		if _, ok := seen[entry.Stage]; ok {
			return fmt.Errorf("stage %q appears more than once", entry.Stage)
		}
		seen[entry.Stage] = struct{}{}
	}
	return nil
}

func (spec DreamRunJobSpec) ToJobSpec(jobID string) (JobSpec, error) {
	if err := spec.Validate(); err != nil {
		return JobSpec{}, err
	}
	payload, err := structToMap(spec)
	if err != nil {
		return JobSpec{}, err
	}
	return JobSpec{ID: jobID, Type: JobTypeDreamRun, Payload: payload}, nil
}

func (spec DreamStageJobSpec) ToJobSpec(jobID string) (JobSpec, error) {
	if err := spec.Validate(); err != nil {
		return JobSpec{}, err
	}
	payload, err := structToMap(spec)
	if err != nil {
		return JobSpec{}, err
	}
	return JobSpec{ID: jobID, Type: JobTypeDreamStage, Payload: payload}, nil
}

func ValidateDreamJobSpec(job JobSpec) error {
	switch job.Type {
	case JobTypeDreamRun:
		_, err := DreamRunJobSpecFromPayload(job.Payload)
		return err
	case JobTypeDreamStage:
		_, err := DreamStageJobSpecFromPayload(job.Payload)
		return err
	default:
		return fmt.Errorf("unsupported Dream job type %q", job.Type)
	}
}

func DreamRunJobSpecFromPayload(payload map[string]any) (DreamRunJobSpec, error) {
	var spec DreamRunJobSpec
	if err := mapToStruct(payload, &spec); err != nil {
		return spec, err
	}
	if err := spec.Validate(); err != nil {
		return spec, err
	}
	return spec, nil
}

func DreamStageJobSpecFromPayload(payload map[string]any) (DreamStageJobSpec, error) {
	var spec DreamStageJobSpec
	if err := mapToStruct(payload, &spec); err != nil {
		return spec, err
	}
	if err := spec.Validate(); err != nil {
		return spec, err
	}
	return spec, nil
}

func ValidateDreamMode(mode DreamMode) error {
	switch mode {
	case DreamModeDaemon, DreamModeOneShot:
		return nil
	default:
		return fmt.Errorf("invalid Dream mode %q", mode)
	}
}

// validateOutputDir rejects an operator-supplied output_dir that is empty or
// that escapes its parent via ".." traversal. The job payload is
// operator-controlled, so an unvalidated output_dir lets a caller redirect
// dream summary/log writes outside the intended tree (soc-ly33, SEC-C2).
// filepath.Clean collapses interior ".." segments, so any escape survives as
// a leading "..". Absolute-path containment against the daemon working
// directory is enforced separately in the executor (outputDirContained),
// where the daemon cwd is known; the symlink pre-check in dream_executor.go
// covers the complementary symlink-redirect vector.
func validateOutputDir(field, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", field)
	}
	cleaned := filepath.Clean(value)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s %q escapes its parent via .. traversal", field, value)
	}
	return nil
}

// outputDirContained verifies that an operator-supplied output_dir resolves to
// a path inside the daemon working directory cwd. It is the absolute-path
// counterpart to validateOutputDir's ".." check (soc-ly33, SEC-C2).
func outputDirContained(cwd, outputDir string) error {
	abs := filepath.Clean(outputDir)
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(cwd, abs)
	}
	rel, err := filepath.Rel(filepath.Clean(cwd), abs)
	if err != nil {
		return fmt.Errorf("output_dir %q is not contained within the daemon working directory", outputDir)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("output_dir %q escapes the daemon working directory", outputDir)
	}
	return nil
}

func validateOptionalDuration(field, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return fmt.Errorf("%s is invalid: %w", field, err)
	}
	if duration <= 0 {
		return fmt.Errorf("%s must be > 0", field)
	}
	return nil
}

func ValidateDreamStage(stage DreamStage) error {
	if dreamStageOrder(stage) == 0 {
		return fmt.Errorf("invalid Dream stage %q", stage)
	}
	return nil
}

func dreamStageOrder(stage DreamStage) int {
	switch stage {
	case DreamStageIngest:
		return 1
	case DreamStageReduce:
		return 2
	case DreamStageMeasure:
		return 3
	case DreamStageCommit:
		return 4
	case DreamStageReport:
		return 5
	default:
		return 0
	}
}
