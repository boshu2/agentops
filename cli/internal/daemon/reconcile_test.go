package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/gascity"
	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
)

func TestRPIReconciler_ReconstructsActiveLostAndCompletedFromGasCity(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	now := time.Date(2026, 4, 28, 20, 0, 0, 0, time.UTC)
	queue := NewQueue(store, QueueOptions{
		LeaseDuration: time.Second,
		Now:           func() time.Time { return now },
	})
	submitClaimedRPIPhase(t, queue, "job-active", "run-active", 1, "sess-active")
	submitClaimedRPIPhase(t, queue, "job-lost", "run-lost", 1, "sess-lost")
	submitClaimedRPIPhase(t, queue, "job-done", "run-done", 1, "sess-done")
	now = now.Add(2 * time.Second)

	fake := &fakeGasCityRPIClient{
		ready: gascity.ReadinessResponse{Ready: true, Status: "ready"},
		sessions: map[string]gascity.Session{
			"sess-active": {ID: "sess-active", State: "running", Running: true},
			"sess-done":   {ID: "sess-done", State: "closed", Status: "completed"},
		},
		getErrs: map[string]error{
			"sess-lost": fakeGasCityNotFound("sess-lost"),
		},
		transcript: gascity.TranscriptResponse{
			ID:     "tx-done",
			Format: "conversation",
			Turns:  []gascity.TranscriptEntry{{Role: "assistant", Text: "done"}},
		},
	}
	reconciler, err := NewRPIReconciler(store, RPIReconcilerOptions{
		Queue:         queue,
		GasCityClient: fake,
		Actor:         "test-reconciler",
	})
	if err != nil {
		t.Fatalf("NewRPIReconciler: %v", err)
	}

	report, err := reconciler.ReconcileRPIJobs(context.Background())
	if err != nil {
		t.Fatalf("ReconcileRPIJobs: %v", err)
	}
	if report.Active != 1 || report.Lost != 1 || report.Completed != 1 {
		t.Fatalf("report counts = active:%d lost:%d completed:%d failed:%d jobs=%#v", report.Active, report.Lost, report.Completed, report.Failed, report.Jobs)
	}
	byJob := reconcileJobsByID(report.Jobs)
	if byJob["job-active"].Status != RPIReconcileActive {
		t.Fatalf("active job = %#v", byJob["job-active"])
	}
	if byJob["job-lost"].Status != RPIReconcileLost || !byJob["job-lost"].RepairedLedger {
		t.Fatalf("lost job = %#v", byJob["job-lost"])
	}
	if byJob["job-done"].Status != RPIReconcileCompleted || !byJob["job-done"].RepairedLedger {
		t.Fatalf("completed job = %#v", byJob["job-done"])
	}
	if _, err := os.Stat(byJob["job-done"].EvidencePath); err != nil {
		t.Fatalf("completed evidence not written: %v", err)
	}

	snapshot, err := queue.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	statuses := map[string]JobStatus{}
	failures := map[string]FailureCode{}
	for _, job := range snapshot.Jobs {
		statuses[job.JobID] = job.Status
		if job.Failure != nil {
			failures[job.JobID] = job.Failure.Code
		}
	}
	if statuses["job-done"] != JobStatusCompleted {
		t.Fatalf("job-done status = %q", statuses["job-done"])
	}
	if statuses["job-lost"] != JobStatusFailed || failures["job-lost"] != FailureSessionLost {
		t.Fatalf("job-lost status=%q failure=%q", statuses["job-lost"], failures["job-lost"])
	}
	if statuses["job-active"] != JobStatusRetryWaiting {
		t.Fatalf("job-active local status = %q, want retry_waiting while provider is active", statuses["job-active"])
	}
}

func TestRPIReconciler_CompletesFromRPIEvidenceFile(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	now := time.Date(2026, 4, 28, 20, 0, 0, 0, time.UTC)
	queue := NewQueue(store, QueueOptions{
		LeaseDuration: time.Second,
		Now:           func() time.Time { return now },
	})
	submitClaimedRPIPhase(t, queue, "job-evidence", "run-evidence", 2, "sess-evidence")
	now = now.Add(2 * time.Second)
	evidencePath, err := cliRPI.WriteGasCityPhaseEvidence(root, cliRPI.GasCityPhaseEvidence{
		RunID:                "run-evidence",
		Phase:                2,
		PhaseName:            "implementation",
		CityName:             "agentops",
		SessionID:            "sess-evidence",
		SessionAlias:         "rpi-run-evidence-p2",
		Status:               gascity.TerminalStatusCompleted,
		TranscriptID:         "tx-evidence",
		TranscriptFormat:     "conversation",
		TranscriptCapturedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("WriteGasCityPhaseEvidence: %v", err)
	}
	reconciler, err := NewRPIReconciler(store, RPIReconcilerOptions{Queue: queue})
	if err != nil {
		t.Fatalf("NewRPIReconciler: %v", err)
	}

	report, err := reconciler.ReconcileRPIJobs(context.Background())
	if err != nil {
		t.Fatalf("ReconcileRPIJobs: %v", err)
	}
	if report.Completed != 1 || len(report.Jobs) != 1 {
		t.Fatalf("report = %#v", report)
	}
	if report.Jobs[0].EvidencePath != evidencePath || !report.Jobs[0].RepairedLedger {
		t.Fatalf("job report = %#v", report.Jobs[0])
	}
	statePath := filepath.Join(root, ".agents", "rpi", "runs", "run-evidence", cliRPI.PhasedStateFile)
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read registry state: %v", err)
	}
	var state cliRPI.RunRegistryState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("decode registry state: %v", err)
	}
	if state.TerminalStatus != "completed" || state.Phase != 2 {
		t.Fatalf("registry state = %#v", state)
	}
}

func submitClaimedRPIPhase(t *testing.T, queue *Queue, jobID, runID string, phase int, sessionID string) {
	t.Helper()
	spec := NewRPIPhaseJobSpec(runID, "Reconcile "+runID, phase)
	spec.GasCityCityName = "agentops"
	spec.GasCitySessionAlias = "rpi-" + runID + "-p" + string(rune('0'+phase))
	jobSpec, err := spec.ToJobSpec(jobID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := queue.SubmitJob(SubmitJobInput{
		RequestID: RequestID("req-" + jobID),
		JobID:     jobID,
		JobType:   jobSpec.Type,
		Payload:   jobSpec.Payload,
	}, QueueMutationOptions{}); err != nil {
		t.Fatalf("SubmitJob %s: %v", jobID, err)
	}
	claim, err := queue.ClaimJob(jobID, "test-runner", QueueMutationOptions{})
	if err != nil {
		t.Fatalf("ClaimJob %s: %v", jobID, err)
	}
	if _, err := queue.Heartbeat(HeartbeatInput{
		JobID:      jobID,
		RequestID:  RequestID(claim.Job.RequestID),
		ClaimToken: claim.ClaimToken,
		LeaseEpoch: claim.LeaseEpoch,
		Actor:      "test-runner",
		Artifacts: map[string]string{
			"active_phase": string(rune('0' + phase)),
			phaseArtifactKey(phase, "gascity_city_name"):     "agentops",
			phaseArtifactKey(phase, "gascity_session_id"):    sessionID,
			phaseArtifactKey(phase, "gascity_session_alias"): spec.GasCitySessionAlias,
		},
	}, QueueMutationOptions{}); err != nil {
		t.Fatalf("Heartbeat %s: %v", jobID, err)
	}
}

func reconcileJobsByID(jobs []RPIReconcileJob) map[string]RPIReconcileJob {
	out := map[string]RPIReconcileJob{}
	for _, job := range jobs {
		out[job.JobID] = job
	}
	return out
}
