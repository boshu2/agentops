package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestFactoryAdmissionExecutorEnqueuesRPIHandoffWhenAllowed(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	queue := NewQueue(store, QueueOptions{LeaseDuration: time.Minute, Now: fixedFactoryAdmissionClock})

	spec := NewFactoryLocalPilotJobSpec("factory-run-1", validFactoryWorkOrder())
	spec.Mode = FactoryAdmissionModeRPIHandoff
	spec.Handoff = FactoryHandoff{
		Kind:                FactoryHandoffRPI,
		ExecutionPacketPath: ".agents/rpi/execution-packet.json",
		EpicID:              "soc-ff7b.7",
	}
	jobSpec, err := spec.ToJobSpec("job-factory-pilot")
	if err != nil {
		t.Fatalf("factory job spec: %v", err)
	}
	if _, err := queue.SubmitJob(SubmitJobInput{
		RequestID: "req-factory-pilot",
		JobID:     jobSpec.ID,
		JobType:   jobSpec.Type,
		Payload:   jobSpec.Payload,
	}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit factory job: %v", err)
	}

	executor := newTestFactoryAdmissionExecutor(t, store, root, true)
	supervisor, err := NewSupervisor(SupervisorOptions{
		Queue:     queue,
		Executors: []JobExecutor{executor},
		Actor:     "test-worker",
	})
	if err != nil {
		t.Fatalf("supervisor: %v", err)
	}
	result, err := supervisor.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run once: %v", err)
	}
	if !result.Claimed || result.Job.Status != JobStatusCompleted {
		t.Fatalf("result = %#v, want completed admission job", result)
	}
	childJobID := result.Job.Artifacts["child_job_id"]
	if childJobID == "" {
		t.Fatalf("artifacts = %#v, want child_job_id", result.Job.Artifacts)
	}
	snapshot, err := queue.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	child, err := snapshot.jobByID(childJobID)
	if err != nil {
		t.Fatalf("child job %q not found in queue: %v", childJobID, err)
	}
	if child.JobType != JobTypeRPIRun || child.Status != JobStatusQueued {
		t.Fatalf("child job = %#v, want queued rpi.run", child)
	}
	assertFactoryAdmissionArtifactExists(t, root, result.Job.Artifacts["admission"])

	projections, err := store.RebuildProjections(ProjectionRebuildOptions{RebuiltAt: fixedFactoryAdmissionClock()})
	if err != nil {
		t.Fatalf("rebuild projections: %v", err)
	}
	if len(projections.Factory.Admissions) != 1 {
		t.Fatalf("admissions = %#v, want one admission", projections.Factory.Admissions)
	}
	admission := projections.Factory.Admissions[0]
	if !admission.Allowed || admission.ChildJobID != childJobID {
		t.Fatalf("admission projection = %#v, want allowed child handoff", admission)
	}
	if len(projections.Factory.Jobs) != 1 || projections.Factory.Jobs[0].Status != FactoryJobStatusAdmitted {
		t.Fatalf("factory jobs = %#v, want admitted parent", projections.Factory.Jobs)
	}
}

func TestFactoryAdmissionExecutorBlocksRPIHandoffWhenDisabled(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	queue := NewQueue(store, QueueOptions{LeaseDuration: time.Minute, Now: fixedFactoryAdmissionClock})

	spec := NewFactoryLocalPilotJobSpec("factory-run-1", validFactoryWorkOrder())
	spec.Mode = FactoryAdmissionModeRPIHandoff
	spec.Handoff = FactoryHandoff{
		Kind:                FactoryHandoffRPI,
		ExecutionPacketPath: ".agents/rpi/execution-packet.json",
	}
	jobSpec, err := spec.ToJobSpec("job-factory-pilot")
	if err != nil {
		t.Fatalf("factory job spec: %v", err)
	}
	if _, err := queue.SubmitJob(SubmitJobInput{
		RequestID: "req-factory-pilot",
		JobID:     jobSpec.ID,
		JobType:   jobSpec.Type,
		Payload:   jobSpec.Payload,
	}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit factory job: %v", err)
	}

	executor := newTestFactoryAdmissionExecutor(t, store, root, false)
	supervisor, err := NewSupervisor(SupervisorOptions{
		Queue:     queue,
		Executors: []JobExecutor{executor},
		Actor:     "test-worker",
	})
	if err != nil {
		t.Fatalf("supervisor: %v", err)
	}
	result, err := supervisor.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run once: %v", err)
	}
	if !result.Claimed || result.Job.Status != JobStatusCompleted {
		t.Fatalf("result = %#v, want completed blocked admission job", result)
	}
	if got := result.Job.Artifacts["allowed"]; got != "false" {
		t.Fatalf("allowed artifact = %q, want false", got)
	}
	if childJobID := result.Job.Artifacts["child_job_id"]; childJobID != "" {
		t.Fatalf("child_job_id = %q, want no child", childJobID)
	}
	var decision FactoryAdmissionDecision
	readFactoryAdmissionArtifact(t, root, result.Job.Artifacts["admission"], &decision)
	if decision.Allowed {
		t.Fatalf("decision = %#v, want blocked", decision)
	}
	if !slices.Contains(decision.Reasons, FactoryAdmissionReasonRPIHandoffUnavailable) {
		t.Fatalf("reasons = %v, want %q", decision.Reasons, FactoryAdmissionReasonRPIHandoffUnavailable)
	}
}

func newTestFactoryAdmissionExecutor(t *testing.T, store *Store, root string, handoff bool) *FactoryAdmissionExecutor {
	t.Helper()
	executor, err := NewFactoryAdmissionExecutor(FactoryAdmissionExecutorOptions{
		Store:            store,
		Root:             root,
		Clock:            fixedFactoryAdmissionClock,
		EnableRPIHandoff: handoff,
		EvidenceProviderFactory: func(FactoryWorkOrder) FactoryAdmissionEvidenceProvider {
			return validFactoryAdmissionEvidence()
		},
	})
	if err != nil {
		t.Fatalf("new factory admission executor: %v", err)
	}
	return executor
}

func assertFactoryAdmissionArtifactExists(t *testing.T, root, relPath string) {
	t.Helper()
	if relPath == "" {
		t.Fatal("artifact path is empty")
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relPath))); err != nil {
		t.Fatalf("artifact %s: %v", relPath, err)
	}
}

func readFactoryAdmissionArtifact(t *testing.T, root, relPath string, out any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relPath)))
	if err != nil {
		t.Fatalf("read artifact %s: %v", relPath, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode artifact %s: %v", relPath, err)
	}
}
