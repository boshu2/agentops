package daemon

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const SkillInvokeJobSpecSchemaVersion = 1

var skillInvokeNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

type SkillInvokeJobSpec struct {
	SchemaVersion int      `json:"schema_version"`
	JobType       JobType  `json:"job_type"`
	SkillName     string   `json:"skill_name"`
	Args          []string `json:"args,omitempty"`
}

type SkillInvokeRequest struct {
	Spec  SkillInvokeJobSpec
	Claim QueueLease
	Root  string
}

type SkillInvokeResult struct {
	Artifacts map[string]string
}

type SkillInvokeFunc func(context.Context, SkillInvokeRequest) (SkillInvokeResult, error)

type SkillInvokeExecutorOptions struct {
	Root string
	Run  SkillInvokeFunc
}

type SkillInvokeExecutor struct {
	root string
	run  SkillInvokeFunc
}

func NewSkillInvokeExecutor(opts SkillInvokeExecutorOptions) (*SkillInvokeExecutor, error) {
	if strings.TrimSpace(opts.Root) == "" {
		return nil, fmt.Errorf("skill invoke executor: Root is required")
	}
	if opts.Run == nil {
		return nil, fmt.Errorf("skill invoke executor: Run is required")
	}
	return &SkillInvokeExecutor{root: opts.Root, run: opts.Run}, nil
}

func (e *SkillInvokeExecutor) JobTypes() []JobType {
	return []JobType{JobTypeSkillInvoke}
}

func (e *SkillInvokeExecutor) RunJob(ctx context.Context, claim QueueLease) (JobExecutionResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if claim.Job.JobType != JobTypeSkillInvoke {
		return JobExecutionResult{}, fmt.Errorf("skill invoke executor does not support job type %s", claim.Job.JobType)
	}
	spec, err := SkillInvokeJobSpecFromPayload(claim.Job.Payload)
	if err != nil {
		return JobExecutionResult{}, err
	}
	artifacts := map[string]string{
		"executor_policy": "skill-invoke",
		"skill_name":      spec.SkillName,
		"args_count":      strconv.Itoa(len(spec.Args)),
	}
	result, runErr := e.run(ctx, SkillInvokeRequest{Spec: spec, Claim: claim, Root: e.root})
	for k, v := range result.Artifacts {
		artifacts[k] = v
	}
	return JobExecutionResult{Artifacts: artifacts}, runErr
}

func NewSkillInvokeJobSpec(skillName string, args []string) SkillInvokeJobSpec {
	return SkillInvokeJobSpec{
		SchemaVersion: SkillInvokeJobSpecSchemaVersion,
		JobType:       JobTypeSkillInvoke,
		SkillName:     skillName,
		Args:          append([]string{}, args...),
	}
}

func (spec SkillInvokeJobSpec) Validate() error {
	if spec.SchemaVersion != SkillInvokeJobSpecSchemaVersion {
		return fmt.Errorf("schema_version mismatch: got %d want %d", spec.SchemaVersion, SkillInvokeJobSpecSchemaVersion)
	}
	if spec.JobType != JobTypeSkillInvoke {
		return fmt.Errorf("job_type = %q, want %q", spec.JobType, JobTypeSkillInvoke)
	}
	if !skillInvokeNamePattern.MatchString(spec.SkillName) {
		return fmt.Errorf("skill_name %q must match ^[a-z][a-z0-9-]*$", spec.SkillName)
	}
	for i, arg := range spec.Args {
		if strings.Contains(arg, "\x00") {
			return fmt.Errorf("args[%d] contains NUL byte", i)
		}
	}
	return nil
}

func (spec SkillInvokeJobSpec) ToJobSpec(jobID string) (JobSpec, error) {
	if err := spec.Validate(); err != nil {
		return JobSpec{}, err
	}
	payload, err := structToMap(spec)
	if err != nil {
		return JobSpec{}, err
	}
	return JobSpec{ID: jobID, Type: JobTypeSkillInvoke, Payload: payload}, nil
}

func SkillInvokeJobSpecFromPayload(payload map[string]any) (SkillInvokeJobSpec, error) {
	raw := map[string]any{}
	for k, v := range payload {
		raw[k] = v
	}
	if _, ok := raw["schema_version"]; !ok {
		raw["schema_version"] = SkillInvokeJobSpecSchemaVersion
	}
	if _, ok := raw["job_type"]; !ok {
		raw["job_type"] = string(JobTypeSkillInvoke)
	}

	args, err := parseSkillInvokeArgs(raw["args"])
	if err != nil {
		return SkillInvokeJobSpec{}, err
	}
	delete(raw, "args")

	var spec SkillInvokeJobSpec
	if err := mapToStruct(raw, &spec); err != nil {
		return spec, err
	}
	spec.Args = args
	if err := spec.Validate(); err != nil {
		return spec, err
	}
	return spec, nil
}

func parseSkillInvokeArgs(value any) ([]string, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case string:
		return strings.Fields(v), nil
	case []string:
		return append([]string{}, v...), nil
	case []any:
		args := make([]string, 0, len(v))
		for i, item := range v {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("args[%d] must be a string", i)
			}
			args = append(args, text)
		}
		return args, nil
	default:
		return nil, fmt.Errorf("args must be a string or list of strings")
	}
}
