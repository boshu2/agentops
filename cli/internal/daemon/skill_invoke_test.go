package daemon

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSkillInvokeJobSpecFromPayloadAcceptsStringArgs(t *testing.T) {
	spec, err := SkillInvokeJobSpecFromPayload(map[string]any{
		"skill_name": "compile",
		"args":       "--full --quiet",
	})
	if err != nil {
		t.Fatalf("SkillInvokeJobSpecFromPayload: %v", err)
	}
	if spec.SchemaVersion != SkillInvokeJobSpecSchemaVersion {
		t.Fatalf("SchemaVersion = %d, want %d", spec.SchemaVersion, SkillInvokeJobSpecSchemaVersion)
	}
	if spec.JobType != JobTypeSkillInvoke {
		t.Fatalf("JobType = %q, want %q", spec.JobType, JobTypeSkillInvoke)
	}
	wantArgs := []string{"--full", "--quiet"}
	if !reflect.DeepEqual(spec.Args, wantArgs) {
		t.Fatalf("Args = %#v, want %#v", spec.Args, wantArgs)
	}
}

func TestSkillInvokeJobSpecFromPayloadRejectsUnsafeSkillName(t *testing.T) {
	_, err := SkillInvokeJobSpecFromPayload(map[string]any{
		"skill_name": "../compile",
	})
	if err == nil || !strings.Contains(err.Error(), "skill_name") {
		t.Fatalf("error = %v, want skill_name rejection", err)
	}
}

func TestSkillInvokeExecutorRunsInjectedFunc(t *testing.T) {
	root := t.TempDir()
	var got SkillInvokeRequest
	exec, err := NewSkillInvokeExecutor(SkillInvokeExecutorOptions{
		Root: root,
		Run: func(_ context.Context, req SkillInvokeRequest) (SkillInvokeResult, error) {
			got = req
			return SkillInvokeResult{Artifacts: map[string]string{"log": ".agents/daemon/skill-invoke/job.log"}}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewSkillInvokeExecutor: %v", err)
	}
	job, err := NewSkillInvokeJobSpec("forge", []string{"review", "--dry-run"}).ToJobSpec("job-skill")
	if err != nil {
		t.Fatalf("ToJobSpec: %v", err)
	}
	result, err := exec.RunJob(context.Background(), QueueClaim{Job: QueueJobState{
		JobID:   job.ID,
		JobType: job.Type,
		Payload: job.Payload,
	}})
	if err != nil {
		t.Fatalf("RunJob: %v", err)
	}
	if got.Root != root || got.Spec.SkillName != "forge" {
		t.Fatalf("request = %#v, want root %q skill forge", got, root)
	}
	if !reflect.DeepEqual(got.Spec.Args, []string{"review", "--dry-run"}) {
		t.Fatalf("Args = %#v", got.Spec.Args)
	}
	if result.Artifacts["executor_policy"] != "skill-invoke" || result.Artifacts["skill_name"] != "forge" {
		t.Fatalf("artifacts = %#v", result.Artifacts)
	}
	if result.Artifacts["log"] == "" {
		t.Fatalf("artifacts = %#v, want injected log artifact", result.Artifacts)
	}
}

func TestSkillInvokeExecutorReturnsRunnerErrorWithArtifacts(t *testing.T) {
	wantErr := errors.New("runner failed")
	exec, err := NewSkillInvokeExecutor(SkillInvokeExecutorOptions{
		Root: t.TempDir(),
		Run: func(context.Context, SkillInvokeRequest) (SkillInvokeResult, error) {
			return SkillInvokeResult{Artifacts: map[string]string{"partial": "yes"}}, wantErr
		},
	})
	if err != nil {
		t.Fatalf("NewSkillInvokeExecutor: %v", err)
	}
	job, err := NewSkillInvokeJobSpec("compile", nil).ToJobSpec("job-skill-fail")
	if err != nil {
		t.Fatalf("ToJobSpec: %v", err)
	}
	result, err := exec.RunJob(context.Background(), QueueClaim{Job: QueueJobState{
		JobID:   job.ID,
		JobType: job.Type,
		Payload: job.Payload,
	}})
	if !errors.Is(err, wantErr) {
		t.Fatalf("RunJob error = %v, want %v", err, wantErr)
	}
	if result.Artifacts["partial"] != "yes" {
		t.Fatalf("artifacts = %#v, want partial artifact", result.Artifacts)
	}
}
