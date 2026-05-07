package daemon

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const RPIJobSpecSchemaVersion = 1

type RPIBackend string

const (
	RPIBackendGasCityAPI     RPIBackend = "gascity-api"
	RPIBackendGasCityCLI     RPIBackend = "gc-cli-fallback"
	RPIBackendForeground     RPIBackend = "foreground"
	RPIBackendDaemonDegraded RPIBackend = "daemon-degraded"
)

type RPIRunJobSpec struct {
	SchemaVersion       int        `json:"schema_version"`
	JobType             JobType    `json:"job_type"`
	RunID               string     `json:"run_id"`
	Goal                string     `json:"goal"`
	EpicID              string     `json:"epic_id,omitempty"`
	ExecutionPacketPath string     `json:"execution_packet_path,omitempty"`
	StartPhase          int        `json:"start_phase"`
	MaxPhase            int        `json:"max_phase"`
	Complexity          string     `json:"complexity,omitempty"`
	TestFirst           bool       `json:"test_first"`
	Backend             RPIBackend `json:"backend"`
	GasCityCityName     string     `json:"gascity_city_name,omitempty"`
	PhaseTimeout        string     `json:"phase_timeout,omitempty"`

	// Supervisor policy fields (soc-bcrn.3.8 / E3.W4 sub-5a-fix). These let
	// daemon-submitted rpi.run jobs preserve gate enforcement and landing
	// semantics that the legacy `ao rpi loop --supervisor` shell wrapper
	// provided. All are optional; zero values mean "let the supervisor pick a
	// safe default" so older payloads stay backward compatible.
	MaxCycles      int    `json:"max_cycles,omitempty"`
	GatePolicy     string `json:"gate_policy,omitempty"`    // off | best-effort | required
	LandingPolicy  string `json:"landing_policy,omitempty"` // off | commit | sync-push
	LandingBranch  string `json:"landing_branch,omitempty"`
	BDSyncPolicy   string `json:"bd_sync_policy,omitempty"` // auto | always | never
	FailurePolicy  string `json:"failure_policy,omitempty"` // stop | continue
	KillSwitchPath string `json:"kill_switch_path,omitempty"`
}

type RPIPhaseJobSpec struct {
	SchemaVersion       int        `json:"schema_version"`
	JobType             JobType    `json:"job_type"`
	RunID               string     `json:"run_id"`
	Goal                string     `json:"goal"`
	EpicID              string     `json:"epic_id,omitempty"`
	ParentRunJobID      string     `json:"parent_run_job_id,omitempty"`
	ExecutionPacketPath string     `json:"execution_packet_path,omitempty"`
	Phase               int        `json:"phase"`
	PhaseName           string     `json:"phase_name"`
	Attempt             int        `json:"attempt,omitempty"`
	Backend             RPIBackend `json:"backend"`
	GasCityCityName     string     `json:"gascity_city_name,omitempty"`
	GasCitySessionAlias string     `json:"gascity_session_alias,omitempty"`
	PhaseTimeout        string     `json:"phase_timeout,omitempty"`
}

func NewRPIRunJobSpec(runID, goal string) RPIRunJobSpec {
	return RPIRunJobSpec{
		SchemaVersion: RPIJobSpecSchemaVersion,
		JobType:       JobTypeRPIRun,
		RunID:         runID,
		Goal:          goal,
		StartPhase:    1,
		MaxPhase:      3,
		TestFirst:     true,
		Backend:       RPIBackendGasCityAPI,
	}
}

func NewRPIPhaseJobSpec(runID, goal string, phase int) RPIPhaseJobSpec {
	return RPIPhaseJobSpec{
		SchemaVersion: RPIJobSpecSchemaVersion,
		JobType:       JobTypeRPIPhase,
		RunID:         runID,
		Goal:          goal,
		Phase:         phase,
		PhaseName:     RPIPhaseName(phase),
		Backend:       RPIBackendGasCityAPI,
	}
}

func (spec RPIRunJobSpec) Validate() error {
	if spec.SchemaVersion != RPIJobSpecSchemaVersion {
		return fmt.Errorf("schema_version mismatch: got %d want %d", spec.SchemaVersion, RPIJobSpecSchemaVersion)
	}
	if spec.JobType != JobTypeRPIRun {
		return fmt.Errorf("job_type = %q, want %q", spec.JobType, JobTypeRPIRun)
	}
	if strings.TrimSpace(spec.RunID) == "" {
		return fmt.Errorf("run_id is required")
	}
	if strings.TrimSpace(spec.Goal) == "" {
		return fmt.Errorf("goal is required")
	}
	if err := validateRPIPhaseNumber("start_phase", spec.StartPhase); err != nil {
		return err
	}
	if err := validateRPIPhaseNumber("max_phase", spec.MaxPhase); err != nil {
		return err
	}
	if spec.MaxPhase < spec.StartPhase {
		return fmt.Errorf("max_phase %d must be >= start_phase %d", spec.MaxPhase, spec.StartPhase)
	}
	if err := validateOptionalDuration("phase_timeout", spec.PhaseTimeout); err != nil {
		return err
	}
	return ValidateRPIBackend(spec.Backend)
}

func (spec RPIPhaseJobSpec) Validate() error {
	if spec.SchemaVersion != RPIJobSpecSchemaVersion {
		return fmt.Errorf("schema_version mismatch: got %d want %d", spec.SchemaVersion, RPIJobSpecSchemaVersion)
	}
	if spec.JobType != JobTypeRPIPhase {
		return fmt.Errorf("job_type = %q, want %q", spec.JobType, JobTypeRPIPhase)
	}
	if strings.TrimSpace(spec.RunID) == "" {
		return fmt.Errorf("run_id is required")
	}
	if strings.TrimSpace(spec.Goal) == "" {
		return fmt.Errorf("goal is required")
	}
	if err := validateRPIPhaseNumber("phase", spec.Phase); err != nil {
		return err
	}
	if want := RPIPhaseName(spec.Phase); spec.PhaseName != want {
		return fmt.Errorf("phase_name = %q, want %q", spec.PhaseName, want)
	}
	if spec.Attempt < 0 {
		return fmt.Errorf("attempt must be >= 0")
	}
	if err := validateOptionalDuration("phase_timeout", spec.PhaseTimeout); err != nil {
		return err
	}
	return ValidateRPIBackend(spec.Backend)
}

func (spec RPIRunJobSpec) ToJobSpec(jobID string) (JobSpec, error) {
	if err := spec.Validate(); err != nil {
		return JobSpec{}, err
	}
	payload, err := structToMap(spec)
	if err != nil {
		return JobSpec{}, err
	}
	return JobSpec{ID: jobID, Type: JobTypeRPIRun, Payload: payload}, nil
}

func (spec RPIPhaseJobSpec) ToJobSpec(jobID string) (JobSpec, error) {
	if err := spec.Validate(); err != nil {
		return JobSpec{}, err
	}
	payload, err := structToMap(spec)
	if err != nil {
		return JobSpec{}, err
	}
	return JobSpec{ID: jobID, Type: JobTypeRPIPhase, Payload: payload}, nil
}

func ValidateRPIJobSpec(job JobSpec) error {
	switch job.Type {
	case JobTypeRPIRun:
		_, err := RPIRunJobSpecFromPayload(job.Payload)
		return err
	case JobTypeRPIPhase:
		_, err := RPIPhaseJobSpecFromPayload(job.Payload)
		return err
	default:
		return fmt.Errorf("unsupported RPI job type %q", job.Type)
	}
}

func RPIRunJobSpecFromPayload(payload map[string]any) (RPIRunJobSpec, error) {
	var spec RPIRunJobSpec
	if err := mapToStruct(payload, &spec); err != nil {
		return spec, err
	}
	if err := spec.Validate(); err != nil {
		return spec, err
	}
	return spec, nil
}

func RPIPhaseJobSpecFromPayload(payload map[string]any) (RPIPhaseJobSpec, error) {
	var spec RPIPhaseJobSpec
	if err := mapToStruct(payload, &spec); err != nil {
		return spec, err
	}
	if err := spec.Validate(); err != nil {
		return spec, err
	}
	return spec, nil
}

func ValidateRPIBackend(backend RPIBackend) error {
	switch backend {
	case RPIBackendGasCityAPI, RPIBackendGasCityCLI, RPIBackendForeground, RPIBackendDaemonDegraded:
		return nil
	default:
		return fmt.Errorf("invalid RPI backend %q", backend)
	}
}

func RPIPhaseName(phase int) string {
	switch phase {
	case 1:
		return "discovery"
	case 2:
		return "implementation"
	case 3:
		return "validation"
	default:
		return ""
	}
}

func validateRPIPhaseNumber(field string, value int) error {
	if value < 1 || value > 3 {
		return fmt.Errorf("%s must be between 1 and 3", field)
	}
	return nil
}

func parseRPIPhaseTimeout(value string) time.Duration {
	duration, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || duration <= 0 {
		return 0
	}
	return duration
}

func structToMap(value any) (map[string]any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func mapToStruct(payload map[string]any, out any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}
