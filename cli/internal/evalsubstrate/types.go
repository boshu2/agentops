// Package evalsubstrate implements the Day-2 eval substrate per
// ~/.agents/evals/SCHEMA.md rc3: §3 primitives, §4 atomic-write contract,
// §6 manifest-checkable gates (1/6/7/8/9), §7 content-addressing,
// and (rc3 addition) verdict-driven corpus mutation.
//
// Distinct from cli/internal/eval (legacy deterministic-suite RunRecord).
package evalsubstrate

import (
	"bytes"
	"encoding/json"
	"fmt"
)

const SchemaVersion = 1

type RunStatus string

const (
	StatusPending   RunStatus = "pending"
	StatusRunning   RunStatus = "running"
	StatusComplete  RunStatus = "complete"
	StatusFailed    RunStatus = "failed"
	StatusAborted   RunStatus = "aborted"
	StatusRetracted RunStatus = "retracted"
)

func (s RunStatus) Terminal() bool {
	switch s {
	case StatusComplete, StatusFailed, StatusAborted, StatusRetracted:
		return true
	}
	return false
}

type Task struct {
	SchemaVersion int               `yaml:"schema_version" json:"schema_version"`
	ID            string            `yaml:"id" json:"id"`
	Domain        string            `yaml:"domain" json:"domain"`
	Description   string            `yaml:"description,omitempty" json:"description,omitempty"`
	HarnessRef    string            `yaml:"harness_ref,omitempty" json:"harness_ref,omitempty"`
	Stats         TaskStat          `yaml:"stats" json:"stats"`
	Samples       map[string]string `yaml:"samples,omitempty" json:"samples,omitempty"`
}

type TaskStat struct {
	Metric       string       `yaml:"metric" json:"metric"`
	Paired       bool         `yaml:"paired" json:"paired"`
	MinNSamples  int          `yaml:"min_n_samples" json:"min_n_samples"`
	DecisionRule DecisionRule `yaml:"decision_rule" json:"decision_rule"`
}

type DecisionRule struct {
	Kind       string  `yaml:"kind" json:"kind"`
	Confidence float64 `yaml:"confidence,omitempty" json:"confidence,omitempty"`
	MinDelta   float64 `yaml:"min_delta,omitempty" json:"min_delta,omitempty"`
}

type Suite struct {
	SchemaVersion int          `yaml:"schema_version" json:"schema_version"`
	ID            string       `yaml:"id" json:"id"`
	Kind          string       `yaml:"kind" json:"kind"`
	VariedAxis    VariedAxis   `yaml:"varied_axis" json:"varied_axis"`
	HeldConstant  HeldConstant `yaml:"held_constant" json:"held_constant"`
	SampleSplit   string       `yaml:"sample_split" json:"sample_split"`
	NSamples      int          `yaml:"n_samples" json:"n_samples"`
	Stats         SuiteStat    `yaml:"stats" json:"stats"`
	BaselineRun   *string      `yaml:"baseline_run,omitempty" json:"baseline_run,omitempty"`
}

type VariedAxis struct {
	Kind   string   `yaml:"kind" json:"kind"`
	Values []string `yaml:"values" json:"values"`
}

type HeldConstant struct {
	Task               string                 `yaml:"task,omitempty" json:"task,omitempty"`
	Harness            string                 `yaml:"harness,omitempty" json:"harness,omitempty"`
	Judge              *string                `yaml:"judge,omitempty" json:"judge,omitempty"`
	GroundTruthVersion string                 `yaml:"ground_truth_version,omitempty" json:"ground_truth_version,omitempty"`
	Decoding           map[string]interface{} `yaml:"decoding,omitempty" json:"decoding,omitempty"`
}

func (h HeldConstant) IsEmpty() bool {
	return h.Task == "" && h.Harness == "" && h.Judge == nil &&
		h.GroundTruthVersion == "" && len(h.Decoding) == 0
}

type SuiteStat struct {
	DecisionRule          DecisionRule `yaml:"decision_rule" json:"decision_rule"`
	Power                 *Power       `yaml:"power,omitempty" json:"power,omitempty"`
	MultiComparisonMethod string       `yaml:"multi_comparison_method,omitempty" json:"multi_comparison_method,omitempty"`
	ComparisonFamily      string       `yaml:"comparison_family,omitempty" json:"comparison_family,omitempty"`
	ReferenceArm          string       `yaml:"reference_arm,omitempty" json:"reference_arm,omitempty"`
	Paired                bool         `yaml:"paired,omitempty" json:"paired,omitempty"`
}

type Power struct {
	MinimumDetectableEffect float64 `yaml:"minimum_detectable_effect" json:"minimum_detectable_effect"`
	Alpha                   float64 `yaml:"alpha" json:"alpha"`
}

type Harness struct {
	SchemaVersion int                    `yaml:"schema_version" json:"schema_version"`
	ID            string                 `yaml:"id" json:"id"`
	ContentHash   string                 `yaml:"content_hash" json:"content_hash"`
	Source        HarnessSource          `yaml:"source" json:"source"`
	Files         []HarnessFile          `yaml:"files,omitempty" json:"files,omitempty"`
	Imports       []HarnessFile          `yaml:"imports,omitempty" json:"imports,omitempty"`
	Config        map[string]interface{} `yaml:"config,omitempty" json:"config,omitempty"`
	LockFile      string                 `yaml:"lock_file,omitempty" json:"lock_file,omitempty"`
	CapturedAt    string                 `yaml:"captured_at,omitempty" json:"captured_at,omitempty"`
	CapturedBy    string                 `yaml:"captured_by,omitempty" json:"captured_by,omitempty"`
}

type HarnessSource struct {
	Kind      string `yaml:"kind" json:"kind"`
	Path      string `yaml:"path" json:"path"`
	GitRemote string `yaml:"git_remote,omitempty" json:"git_remote,omitempty"`
	GitSha    string `yaml:"git_sha,omitempty" json:"git_sha,omitempty"`
}

type HarnessFile struct {
	Path   string `yaml:"path,omitempty" json:"path,omitempty"`
	Target string `yaml:"target,omitempty" json:"target,omitempty"`
	SHA256 string `yaml:"sha256" json:"sha256"`
	Role   string `yaml:"role,omitempty" json:"role,omitempty"`
}

type ModelSpec struct {
	SchemaVersion    int                    `yaml:"schema_version" json:"schema_version"`
	ID               string                 `yaml:"id" json:"id"`
	ContentHash      string                 `yaml:"content_hash" json:"content_hash"`
	Provider         string                 `yaml:"provider" json:"provider"`
	BaseURL          string                 `yaml:"base_url,omitempty" json:"base_url,omitempty"`
	ModelName        string                 `yaml:"model_name" json:"model_name"`
	Quantization     string                 `yaml:"quantization,omitempty" json:"quantization,omitempty"`
	ContextLimit     int                    `yaml:"context_limit,omitempty" json:"context_limit,omitempty"`
	ToolCallSupport  bool                   `yaml:"tool_call_support" json:"tool_call_support"`
	Server           map[string]interface{} `yaml:"server,omitempty" json:"server,omitempty"`
	SamplingDefaults map[string]interface{} `yaml:"sampling_defaults" json:"sampling_defaults"`
	RigID            string                 `yaml:"rig_id,omitempty" json:"rig_id,omitempty"`
	RigSpecs         map[string]interface{} `yaml:"rig_specs,omitempty" json:"rig_specs,omitempty"`
	CapturedAt       string                 `yaml:"captured_at,omitempty" json:"captured_at,omitempty"`
}

type GroundTruthRow struct {
	ID            string `json:"id"`
	Value         string `json:"value"`
	Source        string `json:"source"`
	SourceVersion string `json:"source_version,omitempty"`
	RubricVersion int    `json:"rubric_version,omitempty"`
	Evidence      string `json:"evidence,omitempty"`
	Validator     string `json:"validator,omitempty"`
	Confidence    string `json:"confidence"`
	Split         string `json:"split"`
	Supersedes    string `json:"supersedes,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
}

type Manifest struct {
	SchemaVersion int       `json:"schema_version"`
	ID            string    `json:"id"`
	StartedAt     string    `json:"started_at"`
	FinishedAt    string    `json:"finished_at,omitempty"`
	Kind          string    `json:"kind"`
	Status        RunStatus `json:"status"`
	QuickSession  bool      `json:"quick_session"`

	StartedAtUnixMs   int64  `json:"started_at_unix_ms"`
	FinishedAtUnixMs  int64  `json:"finished_at_unix_ms,omitempty"`
	RetractedAtUnixMs *int64 `json:"retracted_at_unix_ms"`
	RetractionReason  string `json:"retraction_reason,omitempty"`

	SuiteRef           string  `json:"suite_ref"`
	TaskRef            string  `json:"task_ref"`
	HarnessRef         string  `json:"harness_ref"`
	HarnessContentHash string  `json:"harness_content_hash"`
	RoleRef            *string `json:"role_ref"`
	JudgeRef           *string `json:"judge_ref"`
	ModelSpecRef       string  `json:"model_spec_ref"`
	ModelSpecHash      string  `json:"model_spec_hash"`
	GroundTruthRef     string  `json:"ground_truth_ref"`
	GroundTruthHash    string  `json:"ground_truth_hash"`

	SampleSplit string `json:"sample_split"`
	NSamples    int    `json:"n_samples"`
	Seeds       []int  `json:"seeds"`

	InspectLogPath      string                 `json:"inspect_log_path,omitempty"`
	InspectCommand      string                 `json:"inspect_command"`
	InspectVersion      string                 `json:"inspect_version"`
	Metrics             map[string]interface{} `json:"metrics,omitempty"`
	DiffFrom            string                 `json:"diff_from,omitempty"`
	Verdict             *Verdict               `json:"verdict,omitempty"`
	ValidityGatesPassed []string               `json:"validity_gates_passed"`

	RigID      string `json:"rig_id"`
	CapturedBy string `json:"captured_by"`
	Notes      string `json:"notes,omitempty"`

	PairedSampleIDsHash   string `json:"paired_sample_ids_hash,omitempty"`
	MultiComparisonMethod string `json:"multi_comparison_method,omitempty"`
	ComparisonFamily      string `json:"comparison_family,omitempty"`
	ReferenceArm          string `json:"reference_arm,omitempty"`
	FamilySizeK           int    `json:"family_size_k,omitempty"`
	BootstrapInputsHash   string `json:"bootstrap_inputs_hash,omitempty"`
}

// VerdictKind enumerates the §6.5 5-verdict outcome set plus rc3 underpowered.
type VerdictKind string

const (
	VerdictImproved                 VerdictKind = "improved"
	VerdictRegressed                VerdictKind = "regressed"
	VerdictNoChange                 VerdictKind = "no_change"
	VerdictUnderpowered             VerdictKind = "underpowered"
	VerdictInconclusiveHighVariance VerdictKind = "inconclusive_high_variance"
	VerdictInconclusiveDegenerate   VerdictKind = "inconclusive_degenerate"
)

// Verdict is the rc3 enriched run-verdict record used by the verdict-compiler
// hook (hooks/eval-verdict-compiler.sh) to drive corpus mutation.
//
// The JSON form is either:
//
//	"verdict": null
//	"verdict": "improved"             (legacy rc2 string form — back-compat)
//	"verdict": {kind: "improved", utility: 0.85, ...}   (rc3 struct form)
//
// UnmarshalJSON accepts all three. Marshaling always emits the struct form.
type Verdict struct {
	Kind                VerdictKind `json:"kind"`
	DeltaPoint          float64     `json:"delta_point,omitempty"`
	CILow               float64     `json:"ci_low,omitempty"`
	CIHigh              float64     `json:"ci_high,omitempty"`
	Utility             float64     `json:"utility,omitempty"`
	ApplicableArtifacts []string    `json:"applicable_artifacts,omitempty"`
	Notes               string      `json:"notes,omitempty"`
}

// UnmarshalJSON accepts both the legacy string form and the rc3 struct form.
// `null` produces a zero-valued Verdict (caller can detect via *Verdict==nil
// since the field is a pointer; this method is only invoked when the field
// is present and non-null).
func (v *Verdict) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return fmt.Errorf("verdict: legacy string form: %w", err)
		}
		v.Kind = VerdictKind(s)
		return nil
	}
	type rawVerdict Verdict
	var raw rawVerdict
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return fmt.Errorf("verdict: struct form: %w", err)
	}
	*v = Verdict(raw)
	return nil
}
