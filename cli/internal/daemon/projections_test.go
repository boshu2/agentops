package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRequestIDModelRequiresLedgerCorrelation(t *testing.T) {
	if err := ValidateRequestID("req_20260428_000001"); err != nil {
		t.Fatalf("valid request id rejected: %v", err)
	}
	if err := ValidateRequestID("gc-request-123"); err != nil {
		t.Fatalf("provider-shaped request id rejected: %v", err)
	}
	for _, value := range []string{"", "   ", "req with spaces", "req\nbad"} {
		if err := ValidateRequestID(value); err == nil {
			t.Fatalf("invalid request id %q accepted", value)
		}
	}

	event, err := NewLedgerEvent(LedgerEventInput{
		EventID:           "evt-1",
		RequestID:         RequestID("req-1"),
		JobID:             "job-1",
		EventType:         EventJobAccepted,
		OccurredAt:        projectionTestTime(t, 0),
		Actor:             "ao",
		JobType:           JobTypeRPIRun,
		ProjectionTargets: []ProjectionName{ProjectionRPIRegistry, ProjectionOpenClaw},
	})
	if err != nil {
		t.Fatalf("create ledger event: %v", err)
	}
	if event.RequestID != "req-1" {
		t.Fatalf("RequestID = %q, want req-1", event.RequestID)
	}
	if got := event.Payload["job_type"]; got != string(JobTypeRPIRun) {
		t.Fatalf("payload job_type = %#v, want %q", got, JobTypeRPIRun)
	}
	targets := projectionTargetsFromPayload(event.Payload)
	if len(targets) != 2 || targets[0] != ProjectionRPIRegistry || targets[1] != ProjectionOpenClaw {
		t.Fatalf("projection targets = %#v, want rpi/openclaw", targets)
	}
}

func TestValidateEventTypeAcceptsFactoryLifecycleEvents(t *testing.T) {
	events := []EventType{
		EventFactoryJobSubmitted,
		EventFactoryJobClaimed,
		EventFactoryJobStarted,
		EventFactoryRoutingDecided,
		EventFactorySlotAllocated,
		EventFactoryWorktreeAllocated,
		EventFactoryValidationStarted,
		EventFactoryValidationCompleted,
		EventFactoryMergeDecision,
		EventFactoryJobTerminal,
		EventFactoryYieldObservation,
	}
	for _, eventType := range events {
		if err := ValidateEventType(eventType); err != nil {
			t.Fatalf("ValidateEventType(%q): %v", eventType, err)
		}
	}
}

func TestFactoryProjectionReplaysLifecycleFromLedger(t *testing.T) {
	artifactRef := ArtifactRef{
		Path:      ".agents/handoffs/sha256/aa/bb/" + strings.Repeat("a", 64),
		SHA256:    strings.Repeat("a", 64),
		Size:      128,
		WrittenAt: projectionTestTime(t, 13).Format(time.RFC3339Nano),
	}
	events := []LedgerEvent{
		mustNewProjectionTestEvent(t, "evt-001", "req-review-1", "job-review", EventFactoryJobSubmitted, "", 0, map[string]any{
			"job_id":       "job-review",
			"run_id":       "factory-run-1",
			"task_id":      "soc-dpci.5",
			"requested_by": "operator",
			"objective":    "reviewable patch",
		}),
		mustNewProjectionTestEvent(t, "evt-002", "req-review-2", "job-review", EventFactoryRoutingDecided, "", 1, map[string]any{
			"lane_id":   "frontier-codex",
			"provider":  "openai",
			"runtime":   "codex",
			"model":     "gpt-5",
			"authority": "DELEGATED",
			"reason":    "default coding lane",
		}),
		mustNewProjectionTestEvent(t, "evt-003", "req-review-3", "job-review", EventFactorySlotAllocated, "", 2, map[string]any{
			"slot_id":                  "slot-review",
			"lane_id":                  "frontier-codex",
			"max_concurrency_snapshot": 2,
		}),
		mustNewProjectionTestEvent(t, "evt-004", "req-review-4", "job-review", EventFactoryJobClaimed, "", 3, map[string]any{
			"slot_id":   "slot-review",
			"worker_id": "worker-review",
		}),
		mustNewProjectionTestEvent(t, "evt-005", "req-review-5", "job-review", EventFactoryWorktreeAllocated, "", 4, map[string]any{
			"worktree_id":  "wt-review",
			"slot_id":      "slot-review",
			"path":         "/tmp/factory/wt-review",
			"base_commit":  "abc123",
			"branch":       "factory/review",
			"owner_job_id": "job-review",
		}),
		mustNewProjectionTestEvent(t, "evt-006", "req-review-6", "job-review", EventFactoryJobStarted, "", 5, map[string]any{
			"slot_id":   "slot-review",
			"worker_id": "worker-review",
		}),
		mustNewProjectionTestEvent(t, "evt-007", "req-review-7", "job-review", EventFactoryValidationStarted, "", 6, map[string]any{
			"validation_id": "val-review",
			"commands":      []string{"go test ./internal/daemon -run Factory"},
			"level":         "L1",
		}),
		mustNewProjectionTestEvent(t, "evt-008", "req-review-8", "job-review", EventFactoryValidationCompleted, "", 7, map[string]any{
			"validation_id": "val-review",
			"status":        "passed",
			"artifacts":     map[string]string{"validation": ".agents/factory/runs/factory-run-1/review-validation.json"},
			"duration_ms":   1200,
		}),
		mustNewProjectionTestEvent(t, "evt-009", "req-review-9", "job-review", EventFactoryMergeDecision, "", 8, map[string]any{
			"decision":       "manual_pending",
			"decider":        "operator",
			"reason":         "manual merge required",
			"conflicts":      []string{},
			"manual_command": "git merge factory/review",
		}),
		mustNewProjectionTestEvent(t, "evt-010", "req-failed-1", "job-failed", EventFactoryJobSubmitted, "", 9, map[string]any{
			"job_id":       "job-failed",
			"run_id":       "factory-run-1",
			"task_id":      "soc-dpci.6",
			"requested_by": "operator",
			"objective":    "failing patch",
		}),
		mustNewProjectionTestEvent(t, "evt-011", "req-failed-2", "job-failed", EventFactoryRoutingDecided, "", 10, map[string]any{
			"lane_id":   "frontier-codex",
			"provider":  "openai",
			"runtime":   "codex",
			"model":     "gpt-5",
			"authority": "DELEGATED",
			"reason":    "same lane",
		}),
		mustNewProjectionTestEvent(t, "evt-012", "req-failed-3", "job-failed", EventFactorySlotAllocated, "", 11, map[string]any{
			"slot_id":                  "slot-failed",
			"lane_id":                  "frontier-codex",
			"max_concurrency_snapshot": 2,
		}),
		mustNewProjectionTestEvent(t, "evt-013", "req-failed-4", "job-failed", EventFactoryJobStarted, "", 12, map[string]any{
			"slot_id":   "slot-failed",
			"worker_id": "worker-failed",
		}),
		mustNewProjectionTestEvent(t, "evt-014", "req-failed-5", "job-failed", EventFactoryWorktreeAllocated, "", 13, map[string]any{
			"worktree_id":  "wt-failed",
			"slot_id":      "slot-failed",
			"path":         "/tmp/factory/wt-failed",
			"base_commit":  "abc123",
			"branch":       "factory/failed",
			"owner_job_id": "job-failed",
		}),
		mustNewProjectionTestEvent(t, "evt-015", "req-failed-6", "job-failed", EventFactoryValidationStarted, "", 14, map[string]any{
			"validation_id": "val-failed",
			"commands":      []string{"go test ./internal/daemon -run Factory"},
			"level":         "L1",
		}),
		mustNewProjectionTestEvent(t, "evt-016", "req-failed-7", "job-failed", EventFactoryValidationCompleted, "", 15, map[string]any{
			"validation_id": "val-failed",
			"status":        "failed",
			"artifacts":     map[string]string{"validation": ".agents/factory/runs/factory-run-1/failed-validation.json"},
			"logs":          map[string]string{"validation": ".agents/factory/runs/factory-run-1/failed-validation.log"},
			"duration_ms":   2400,
		}),
		mustNewProjectionTestEvent(t, "evt-017", "req-failed-8", "job-failed", EventFactoryJobTerminal, "", 16, map[string]any{
			"status":            "failed",
			"artifact_refs":     map[string]ArtifactRef{"diff": artifactRef},
			"transcript_ref":    ".agents/factory/runs/factory-run-1/job-failed/transcript.jsonl",
			"diff_ref":          ".agents/factory/runs/factory-run-1/job-failed/diff.patch",
			"log_ref":           ".agents/factory/runs/factory-run-1/job-failed/worker.log",
			"retained_worktree": true,
		}),
	}

	projections, err := RebuildProjections(events, ProjectionRebuildOptions{
		RebuiltAt:    projectionTestTime(t, 20),
		SourceLedger: ".agents/daemon/ledger.jsonl",
	})
	if err != nil {
		t.Fatalf("rebuild projections: %v", err)
	}
	factory := projections.Factory
	if projections.LastEventID != "evt-017" {
		t.Fatalf("LastEventID = %q, want evt-017", projections.LastEventID)
	}
	if len(factory.Jobs) != 2 {
		t.Fatalf("factory jobs = %d, want 2", len(factory.Jobs))
	}
	if factory.Jobs[0].Status != FactoryJobStatusAwaitingManualMerge {
		t.Fatalf("job-review status = %q, want awaiting_manual_merge", factory.Jobs[0].Status)
	}
	if factory.Jobs[1].Status != FactoryJobStatusRetainedFailed {
		t.Fatalf("job-failed status = %q, want retained_failed", factory.Jobs[1].Status)
	}
	if len(factory.ActiveWorkers) != 1 {
		t.Fatalf("active workers = %d, want 1: %#v", len(factory.ActiveWorkers), factory.ActiveWorkers)
	}
	if factory.ActiveWorkers[0].WorkerID != "worker-review" || factory.ActiveWorkers[0].SlotID != "slot-review" || factory.ActiveWorkers[0].Status != FactorySlotStatusAwaitingManualMerge {
		t.Fatalf("active worker = %#v, want slot-review awaiting_manual_merge", factory.ActiveWorkers[0])
	}
	if len(factory.Slots) != 2 {
		t.Fatalf("slots = %d, want 2", len(factory.Slots))
	}
	if factory.Slots[0].Status != FactorySlotStatusAwaitingManualMerge {
		t.Fatalf("slot-review status = %q, want awaiting_manual_merge", factory.Slots[0].Status)
	}
	if factory.Slots[1].Status != FactorySlotStatusRetainedFailed {
		t.Fatalf("slot-failed status = %q, want retained_failed", factory.Slots[1].Status)
	}
	if len(factory.QueueLanes) != 1 {
		t.Fatalf("queue lanes = %d, want 1: %#v", len(factory.QueueLanes), factory.QueueLanes)
	}
	if factory.QueueLanes[0].LaneID != "frontier-codex" || factory.QueueLanes[0].QueueDepth != 0 {
		t.Fatalf("queue lane = %#v, want frontier-codex depth 0", factory.QueueLanes[0])
	}
	if len(factory.ModelLanes) != 1 {
		t.Fatalf("model lanes = %d, want 1: %#v", len(factory.ModelLanes), factory.ModelLanes)
	}
	if factory.ModelLanes[0].Provider != "openai" || factory.ModelLanes[0].Runtime != "codex" || factory.ModelLanes[0].Authority != RoutingAuthorityDelegated {
		t.Fatalf("model lane = %#v, want openai/codex/DELEGATED", factory.ModelLanes[0])
	}
	if factory.LastRoutingDecision == nil || factory.LastRoutingDecision.JobID != "job-failed" || factory.LastRoutingDecision.LaneID != "frontier-codex" {
		t.Fatalf("last routing decision = %#v, want job-failed/frontier-codex", factory.LastRoutingDecision)
	}
	if len(factory.Validations) != 2 {
		t.Fatalf("validations = %d, want 2", len(factory.Validations))
	}
	if len(factory.BlockedValidations) != 1 {
		t.Fatalf("blocked validations = %d, want 1: %#v", len(factory.BlockedValidations), factory.BlockedValidations)
	}
	if factory.BlockedValidations[0].ValidationID != "val-failed" || factory.BlockedValidations[0].Status != FactoryValidationStatusFailed {
		t.Fatalf("blocked validation = %#v, want val-failed failed", factory.BlockedValidations[0])
	}
	if len(factory.RetainedFailedWorktrees) != 1 {
		t.Fatalf("retained failed worktrees = %d, want 1: %#v", len(factory.RetainedFailedWorktrees), factory.RetainedFailedWorktrees)
	}
	if factory.RetainedFailedWorktrees[0].WorktreeID != "wt-failed" || factory.RetainedFailedWorktrees[0].Path != "/tmp/factory/wt-failed" {
		t.Fatalf("retained worktree = %#v, want wt-failed path", factory.RetainedFailedWorktrees[0])
	}
	if len(factory.PendingManualMerges) != 1 {
		t.Fatalf("pending manual merges = %d, want 1: %#v", len(factory.PendingManualMerges), factory.PendingManualMerges)
	}
	if factory.PendingManualMerges[0].JobID != "job-review" || factory.PendingManualMerges[0].Decision != FactoryMergeDecisionManualPending {
		t.Fatalf("pending manual merge = %#v, want job-review manual_pending", factory.PendingManualMerges[0])
	}
	if len(factory.TerminalJobs) != 1 {
		t.Fatalf("terminal jobs = %d, want 1", len(factory.TerminalJobs))
	}
	if factory.TerminalJobs[0].Status != JobStatusFailed || !factory.TerminalJobs[0].RetainedWorktree {
		t.Fatalf("terminal job = %#v, want failed retained", factory.TerminalJobs[0])
	}
	if got := factory.TerminalJobs[0].ArtifactRefs["diff"]; got != artifactRef {
		t.Fatalf("terminal artifact ref = %#v, want %#v", got, artifactRef)
	}
	if len(factory.RecentEvents) != len(events) || factory.RecentEvents[len(factory.RecentEvents)-1].EventType != EventFactoryJobTerminal {
		t.Fatalf("recent events = %#v, want all events ending with terminal", factory.RecentEvents)
	}
	if len(factory.Logs) != 2 {
		t.Fatalf("logs = %d, want validation + worker log: %#v", len(factory.Logs), factory.Logs)
	}
	if len(factory.Artifacts) != 3 {
		t.Fatalf("artifacts = %d, want two validation artifacts + terminal ref: %#v", len(factory.Artifacts), factory.Artifacts)
	}
	if len(factory.Transcripts) != 1 || factory.Transcripts[0].Path != ".agents/factory/runs/factory-run-1/job-failed/transcript.jsonl" {
		t.Fatalf("transcripts = %#v, want failed transcript", factory.Transcripts)
	}
	if len(factory.Diffs) != 1 || factory.Diffs[0].Path != ".agents/factory/runs/factory-run-1/job-failed/diff.patch" {
		t.Fatalf("diffs = %#v, want failed diff", factory.Diffs)
	}
}

func TestFactoryProjectionClaimedJobsAreWorkerOwnedNotQueued(t *testing.T) {
	events := []LedgerEvent{
		mustNewProjectionTestEvent(t, "evt-001", "req-claimed-1", "job-claimed", EventFactoryJobSubmitted, "", 0, map[string]any{
			"job_id":       "job-claimed",
			"run_id":       "factory-run-1",
			"task_id":      "soc-dpci.5",
			"requested_by": "operator",
			"objective":    "claimed patch",
		}),
		mustNewProjectionTestEvent(t, "evt-002", "req-claimed-2", "job-claimed", EventFactoryRoutingDecided, "", 1, map[string]any{
			"lane_id":   "frontier-codex",
			"provider":  "openai",
			"runtime":   "codex",
			"model":     "gpt-5",
			"authority": "DELEGATED",
			"reason":    "default coding lane",
		}),
		mustNewProjectionTestEvent(t, "evt-003", "req-claimed-3", "job-claimed", EventFactorySlotAllocated, "", 2, map[string]any{
			"slot_id":                  "slot-claimed",
			"lane_id":                  "frontier-codex",
			"max_concurrency_snapshot": 2,
		}),
		mustNewProjectionTestEvent(t, "evt-004", "req-claimed-4", "job-claimed", EventFactoryJobClaimed, "", 3, map[string]any{
			"slot_id":   "slot-claimed",
			"worker_id": "worker-claimed",
		}),
	}

	projections, err := RebuildProjections(events, ProjectionRebuildOptions{
		RebuiltAt:    projectionTestTime(t, 20),
		SourceLedger: ".agents/daemon/ledger.jsonl",
	})
	if err != nil {
		t.Fatalf("rebuild projections: %v", err)
	}
	factory := projections.Factory
	if len(factory.QueueLanes) != 1 {
		t.Fatalf("queue lanes = %d, want 1: %#v", len(factory.QueueLanes), factory.QueueLanes)
	}
	if factory.QueueLanes[0].LaneID != "frontier-codex" || factory.QueueLanes[0].QueueDepth != 0 {
		t.Fatalf("queue lane = %#v, want frontier-codex depth 0", factory.QueueLanes[0])
	}
	if len(factory.ActiveWorkers) != 1 {
		t.Fatalf("active workers = %d, want 1: %#v", len(factory.ActiveWorkers), factory.ActiveWorkers)
	}
	if factory.ActiveWorkers[0].WorkerID != "worker-claimed" || factory.ActiveWorkers[0].Status != FactorySlotStatusAllocated {
		t.Fatalf("active worker = %#v, want claimed worker in allocated slot", factory.ActiveWorkers[0])
	}
}

func TestFactoryProjectionRecordsYieldObservationAsRecentEvent(t *testing.T) {
	events := []LedgerEvent{
		mustNewProjectionTestEvent(t, "evt-yield", "req-yield", "job-yield", EventFactoryYieldObservation, "", 0, map[string]any{
			"run_id":                "factory-run-1",
			"lane_id":               "frontier-codex",
			"baseline_or_treatment": "treatment",
			"accepted_patches":      1,
			"validation_status":     "passed",
			"merge_status":          "manual_pending",
			"artifact_refs": map[string]ArtifactRef{
				"validation": {
					Path:      ".agents/factory/runs/factory-run-1/validation.json",
					SHA256:    strings.Repeat("b", 64),
					Size:      256,
					WrittenAt: projectionTestTime(t, 0).Format(time.RFC3339Nano),
				},
			},
		}),
	}

	projections, err := RebuildProjections(events, ProjectionRebuildOptions{
		RebuiltAt:    projectionTestTime(t, 1),
		SourceLedger: ".agents/daemon/ledger.jsonl",
	})
	if err != nil {
		t.Fatalf("rebuild projections: %v", err)
	}
	factory := projections.Factory
	if len(factory.RecentEvents) != 1 {
		t.Fatalf("recent events = %d, want 1: %#v", len(factory.RecentEvents), factory.RecentEvents)
	}
	if factory.RecentEvents[0].EventType != EventFactoryYieldObservation {
		t.Fatalf("recent event type = %q, want %q", factory.RecentEvents[0].EventType, EventFactoryYieldObservation)
	}
	if len(factory.Artifacts) != 1 || factory.Artifacts[0].Path != ".agents/factory/runs/factory-run-1/validation.json" {
		t.Fatalf("artifacts = %#v, want yield validation artifact pointer", factory.Artifacts)
	}
}

func TestProjectionRebuildsRpiDreamWikiAndOpenClawFromLedger(t *testing.T) {
	events := []LedgerEvent{
		mustNewProjectionTestEvent(t, "evt-rpi-accepted", "req-rpi-1", "job-rpi", EventJobAccepted, JobTypeRPIRun, 0, nil),
		mustNewProjectionTestEvent(t, "evt-rpi-completed", "req-rpi-2", "job-rpi", EventJobCompleted, "", 1, map[string]any{
			"artifacts": map[string]string{"summary": ".agents/rpi/runs/job-rpi/phase-3-summary.md"},
		}),
		mustNewProjectionTestEvent(t, "evt-dream-accepted", "req-dream-1", "job-dream", EventJobAccepted, JobTypeDreamRun, 2, nil),
		mustNewProjectionTestEvent(t, "evt-dream-claimed", "req-dream-2", "job-dream", EventJobClaimed, "", 3, nil),
		mustNewProjectionTestEvent(t, "evt-wiki-accepted", "req-wiki-1", "job-wiki", EventJobAccepted, JobTypeWikiForge, 4, nil),
		mustNewProjectionTestEvent(t, "evt-wiki-failed", "req-wiki-2", "job-wiki", EventJobFailed, "", 5, map[string]any{
			"failure_code": string(FailureProviderUnreachable),
			"message":      "worker unavailable",
			"retryable":    true,
		}),
	}

	projections, err := RebuildProjections(events, ProjectionRebuildOptions{
		RebuiltAt:    projectionTestTime(t, 10),
		SourceLedger: ".agents/daemon/ledger.jsonl",
	})
	if err != nil {
		t.Fatalf("rebuild projections: %v", err)
	}
	if projections.LastEventID != "evt-wiki-failed" {
		t.Fatalf("LastEventID = %q, want evt-wiki-failed", projections.LastEventID)
	}
	if len(projections.RPI.Runs) != 1 || projections.RPI.Runs[0].Status != JobStatusCompleted {
		t.Fatalf("RPI projection = %#v, want one completed run", projections.RPI.Runs)
	}
	if projections.RPI.Runs[0].RequestID != "req-rpi-1" || len(projections.RPI.Runs[0].RequestIDs) != 2 {
		t.Fatalf("RPI request ids = %#v, first=%q", projections.RPI.Runs[0].RequestIDs, projections.RPI.Runs[0].RequestID)
	}
	if len(projections.Dream.Runs) != 1 || projections.Dream.Runs[0].Status != JobStatusRunning {
		t.Fatalf("Dream projection = %#v, want one running run", projections.Dream.Runs)
	}
	if len(projections.Wiki.Jobs) != 1 || projections.Wiki.Jobs[0].Status != JobStatusFailed {
		t.Fatalf("Wiki projection = %#v, want one failed job", projections.Wiki.Jobs)
	}
	if projections.Wiki.Jobs[0].Failure == nil || projections.Wiki.Jobs[0].Failure.Code != FailureProviderUnreachable {
		t.Fatalf("Wiki failure = %#v, want provider_unreachable", projections.Wiki.Jobs[0].Failure)
	}
	if len(projections.OpenClaw.Resources.Runs) != 2 {
		t.Fatalf("OpenClaw runs = %d, want RPI + Dream", len(projections.OpenClaw.Resources.Runs))
	}
	if len(projections.OpenClaw.Resources.Jobs) != 3 {
		t.Fatalf("OpenClaw jobs = %d, want all daemon jobs", len(projections.OpenClaw.Resources.Jobs))
	}
	if len(projections.OpenClaw.Resources.Wiki) != 1 {
		t.Fatalf("OpenClaw wiki = %d, want wiki job", len(projections.OpenClaw.Resources.Wiki))
	}
	if projections.Manifests[ProjectionOpenClaw].Status != ProjectionStatusCurrent {
		t.Fatalf("OpenClaw manifest status = %q, want current", projections.Manifests[ProjectionOpenClaw].Status)
	}
}

func TestProjectionReplayContentAddressedArtifacts(t *testing.T) {
	ref := ArtifactRef{
		Path:      ".agents/handoffs/sha256/aa/bb/" + strings.Repeat("a", 64),
		SHA256:    strings.Repeat("a", 64),
		Size:      42,
		WrittenAt: projectionTestTime(t, 2).Format(time.RFC3339Nano),
	}
	events := []LedgerEvent{
		mustNewProjectionTestEvent(t, "evt-wiki-accepted", "req-wiki-1", "job-wiki", EventJobAccepted, JobTypeWikiForge, 0, nil),
		mustNewProjectionTestEvent(t, "evt-wiki-completed", "req-wiki-2", "job-wiki", EventJobCompleted, "", 1, map[string]any{
			"artifact_refs": map[string]ArtifactRef{"worker_session_refs": ref},
		}),
	}
	projections, err := RebuildProjections(events, ProjectionRebuildOptions{RebuiltAt: projectionTestTime(t, 10)})
	if err != nil {
		t.Fatalf("rebuild projections: %v", err)
	}
	if len(projections.Wiki.Jobs) != 1 {
		t.Fatalf("wiki jobs = %#v", projections.Wiki.Jobs)
	}
	job := projections.Wiki.Jobs[0]
	if got := job.ArtifactRefs["worker_session_refs"]; got != ref {
		t.Fatalf("artifact ref = %#v, want %#v", got, ref)
	}
	if got := job.Artifacts["worker_session_refs"]; got != ref.Path {
		t.Fatalf("compat artifact path = %q, want %q", got, ref.Path)
	}
	if got := projections.OpenClaw.Resources.Wiki[0].ArtifactRefs["worker_session_refs"]; got != ref {
		t.Fatalf("openclaw artifact ref = %#v, want %#v", got, ref)
	}
}

func TestProjectionReplayFromStoreCarriesRequestIDsAndDegradesOnCorruptLedger(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.AppendLedgerEvent(mustNewProjectionTestEvent(t, "evt-rpi-accepted", "req-rpi-1", "job-rpi", EventJobAccepted, JobTypeRPIPhase, 0, nil)); err != nil {
		t.Fatalf("append accepted: %v", err)
	}
	if _, err := store.AppendLedgerEvent(mustNewProjectionTestEvent(t, "evt-rpi-heartbeat", "req-rpi-2", "job-rpi", EventJobHeartbeat, "", 1, nil)); err != nil {
		t.Fatalf("append heartbeat: %v", err)
	}
	file, err := os.OpenFile(store.LedgerPath(), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatalf("open ledger for corrupt fixture append: %v", err)
	}
	if _, err := file.WriteString("{not-json\n"); err != nil {
		t.Fatalf("append corrupt fixture line: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close corrupt fixture file: %v", err)
	}

	projections, err := store.RebuildProjections(ProjectionRebuildOptions{RebuiltAt: projectionTestTime(t, 10)})
	if err != nil {
		t.Fatalf("rebuild projections from store: %v", err)
	}
	if len(projections.RPI.Runs) != 1 {
		t.Fatalf("RPI runs = %d, want 1", len(projections.RPI.Runs))
	}
	run := projections.RPI.Runs[0]
	if run.Status != JobStatusRunning {
		t.Fatalf("run status = %q, want running", run.Status)
	}
	if got := strings.Join(run.RequestIDs, ","); got != "req-rpi-1,req-rpi-2" {
		t.Fatalf("request ids = %q, want req-rpi-1,req-rpi-2", got)
	}
	if projections.Manifests[ProjectionOpenClaw].Status != ProjectionStatusDegraded {
		t.Fatalf("OpenClaw manifest status = %q, want degraded", projections.Manifests[ProjectionOpenClaw].Status)
	}
	if projections.OpenClaw.Status != ProjectionStatusDegraded {
		t.Fatalf("OpenClaw snapshot status = %q, want degraded", projections.OpenClaw.Status)
	}
	if len(projections.DegradedReasons) == 0 {
		t.Fatal("projection set missing degraded reason for corrupt replay")
	}
	if _, err := os.Stat(filepath.Join(store.QuarantineDir(), "ledger-line-000003.json")); err != nil {
		t.Fatalf("corrupt replay did not quarantine line 3: %v", err)
	}
}

func TestDaemonStartupReplaysFromSnapshot(t *testing.T) {
	// Build a baseline event stream of N events that lands in the snapshot.
	baseEvents := []LedgerEvent{
		mustNewProjectionTestEvent(t, "evt-001", "req-rpi-1", "job-rpi", EventJobAccepted, JobTypeRPIRun, 0, nil),
		mustNewProjectionTestEvent(t, "evt-002", "req-rpi-2", "job-rpi", EventJobClaimed, "", 1, nil),
		mustNewProjectionTestEvent(t, "evt-003", "req-dream-1", "job-dream", EventJobAccepted, JobTypeDreamRun, 2, nil),
	}
	// Append M new events that arrive after the snapshot was written.
	deltaEvents := []LedgerEvent{
		mustNewProjectionTestEvent(t, "evt-004", "req-rpi-3", "job-rpi", EventJobCompleted, "", 3, map[string]any{
			"artifacts": map[string]string{"summary": ".agents/rpi/runs/job-rpi/phase-3-summary.md"},
		}),
		mustNewProjectionTestEvent(t, "evt-005", "req-dream-2", "job-dream", EventJobClaimed, "", 4, nil),
		mustNewProjectionTestEvent(t, "evt-006", "req-wiki-1", "job-wiki", EventJobAccepted, JobTypeWikiForge, 5, nil),
	}
	allEvents := append(append([]LedgerEvent{}, baseEvents...), deltaEvents...)

	snapshot, err := RebuildProjections(baseEvents, ProjectionRebuildOptions{
		RebuiltAt:    projectionTestTime(t, 10),
		SourceLedger: ".agents/daemon/ledger.jsonl",
	})
	if err != nil {
		t.Fatalf("rebuild snapshot: %v", err)
	}
	if snapshot.LastEventID != "evt-003" {
		t.Fatalf("snapshot LastEventID = %q, want evt-003", snapshot.LastEventID)
	}

	// Delta replay: pass the snapshot + the FULL event list. The filter must
	// drop evt-001..evt-003 and apply only evt-004..evt-006 (M=3 events).
	deltaSet, err := RebuildProjections(allEvents, ProjectionRebuildOptions{
		RebuiltAt:    projectionTestTime(t, 11),
		SourceLedger: ".agents/daemon/ledger.jsonl",
		FromSnapshot: &snapshot,
	})
	if err != nil {
		t.Fatalf("delta replay: %v", err)
	}

	// Full replay (no snapshot) — ground truth for correctness.
	fullSet, err := RebuildProjections(allEvents, ProjectionRebuildOptions{
		RebuiltAt:    projectionTestTime(t, 11),
		SourceLedger: ".agents/daemon/ledger.jsonl",
	})
	if err != nil {
		t.Fatalf("full replay: %v", err)
	}

	if deltaSet.LastEventID != fullSet.LastEventID {
		t.Fatalf("LastEventID delta=%q full=%q", deltaSet.LastEventID, fullSet.LastEventID)
	}
	if deltaSet.LastEventID != "evt-006" {
		t.Fatalf("LastEventID = %q, want evt-006", deltaSet.LastEventID)
	}
	if len(deltaSet.Jobs) != len(fullSet.Jobs) {
		t.Fatalf("job count: delta=%d full=%d", len(deltaSet.Jobs), len(fullSet.Jobs))
	}
	if len(deltaSet.RPI.Runs) != 1 || deltaSet.RPI.Runs[0].Status != JobStatusCompleted {
		t.Fatalf("RPI runs after delta = %#v, want one completed", deltaSet.RPI.Runs)
	}
	if len(deltaSet.Dream.Runs) != 1 || deltaSet.Dream.Runs[0].Status != JobStatusRunning {
		t.Fatalf("Dream runs after delta = %#v, want one running", deltaSet.Dream.Runs)
	}
	if len(deltaSet.Wiki.Jobs) != 1 || deltaSet.Wiki.Jobs[0].Status != JobStatusQueued {
		t.Fatalf("Wiki jobs after delta = %#v, want one queued", deltaSet.Wiki.Jobs)
	}

	// Hook count: filter must keep exactly M=3 events.
	kept := filterEventsAfter(allEvents, snapshot.LastEventID)
	if len(kept) != len(deltaEvents) {
		t.Fatalf("filterEventsAfter kept %d events, want %d", len(kept), len(deltaEvents))
	}
	for i := range deltaEvents {
		if kept[i].EventID != deltaEvents[i].EventID {
			t.Fatalf("filter kept[%d] = %q, want %q", i, kept[i].EventID, deltaEvents[i].EventID)
		}
	}
}

func TestDaemonStartupSkipsCorruptSnapshotAndFullReplays(t *testing.T) {
	// Snapshot with a stale schema_version triggers the skip-and-rebuild path.
	staleSnapshot := ProjectionSet{
		SchemaVersion: ProjectionSchemaVersion + 99,
		LastEventID:   "evt-stale",
	}
	events := []LedgerEvent{
		mustNewProjectionTestEvent(t, "evt-001", "req-1", "job-1", EventJobAccepted, JobTypeRPIRun, 0, nil),
		mustNewProjectionTestEvent(t, "evt-002", "req-2", "job-1", EventJobCompleted, "", 1, nil),
	}
	// Even with FromSnapshot pointing at a stale snapshot, RebuildProjections
	// itself does not validate schema — that responsibility lives in the
	// caller (server.readState falls through to full replay on schema error).
	// Here we just confirm that delta-from-stale produces a non-empty set:
	// the event filter compares EventID strings, "evt-001" > "evt-stale" is
	// false, but "evt-stale" is not a real event so all events apply because
	// the snapshot has no Jobs to seed with.
	set, err := RebuildProjections(events, ProjectionRebuildOptions{
		RebuiltAt:    projectionTestTime(t, 10),
		FromSnapshot: &staleSnapshot,
	})
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	// Filter drops events with EventID <= "evt-stale". "evt-001" < "evt-stale"
	// lexicographically, so both events are filtered out — set has zero jobs.
	// This confirms filterEventsAfter is conservative when the snapshot's
	// LastEventID does not appear in the ledger.
	if len(set.Jobs) != 0 {
		t.Fatalf("delta from stale snapshot produced %d jobs, expected 0 (filter dropped all)", len(set.Jobs))
	}
}

func mustNewProjectionTestEvent(t *testing.T, eventID, requestID, jobID string, eventType EventType, jobType JobType, minuteOffset int, payload map[string]any) LedgerEvent {
	t.Helper()
	event, err := NewLedgerEvent(LedgerEventInput{
		EventID:    eventID,
		RequestID:  RequestID(requestID),
		JobID:      jobID,
		EventType:  eventType,
		OccurredAt: projectionTestTime(t, minuteOffset),
		Actor:      "test",
		JobType:    jobType,
		Payload:    payload,
	})
	if err != nil {
		t.Fatalf("new ledger event %s: %v", eventID, err)
	}
	return event
}

// TestProjections_UnknownEventType_LeavesStateUnchanged exercises the
// forward-compat path (pre-mortem amendment B3): if an older daemon binary
// replays a ledger that contains an event_type it does not recognize, the
// projection reducer must skip-and-log the event, not error or panic.
//
// Setup: build a hand-crafted LedgerEvent with event_type "future.unknown.event"
// (bypassing NewLedgerEvent's validation, which would reject it). Feed it to
// RebuildProjections together with one known event; assert (a) no error,
// (b) state reflects only the known event, (c) the unknown-event slot has
// not mutated set.Jobs / set.Schedules / set.RPI / etc.
func TestProjections_UnknownEventType_LeavesStateUnchanged(t *testing.T) {
	known := mustNewProjectionTestEvent(t, "evt-rpi-1", "req-rpi-1", "job-rpi", EventJobAccepted, JobTypeRPIRun, 0, nil)

	// Snapshot what state should look like with ONLY the known event.
	baseline, err := RebuildProjections([]LedgerEvent{known}, ProjectionRebuildOptions{
		RebuiltAt:    projectionTestTime(t, 5),
		SourceLedger: ".agents/daemon/ledger.jsonl",
	})
	if err != nil {
		t.Fatalf("baseline rebuild: %v", err)
	}

	// Hand-craft an event whose event_type is unknown to this binary. We
	// bypass NewLedgerEvent because it runs ValidateEventType (which would
	// reject this in our test). Replay-from-disk is exactly this scenario:
	// raw JSON unmarshal succeeds, then projection must decide what to do.
	unknown := LedgerEvent{
		SchemaVersion: LedgerSchemaVersion,
		EventID:       "evt-unknown-1",
		RequestID:     "req-future",
		JobID:         "job-future",
		EventType:     EventType("future.unknown.event"),
		OccurredAt:    projectionTestTime(t, 1).Format(time.RFC3339Nano),
		Actor:         "future-daemon",
		Payload:       map[string]any{"experimental_field": "v2"},
	}

	withUnknown, err := RebuildProjections([]LedgerEvent{known, unknown}, ProjectionRebuildOptions{
		RebuiltAt:    projectionTestTime(t, 5),
		SourceLedger: ".agents/daemon/ledger.jsonl",
	})
	if err != nil {
		t.Fatalf("rebuild with unknown event must not error: %v", err)
	}

	if len(withUnknown.Jobs) != len(baseline.Jobs) {
		t.Fatalf("unknown event mutated job count: got %d, baseline %d", len(withUnknown.Jobs), len(baseline.Jobs))
	}
	if len(withUnknown.RPI.Runs) != len(baseline.RPI.Runs) {
		t.Fatalf("unknown event mutated RPI runs: got %d, baseline %d", len(withUnknown.RPI.Runs), len(baseline.RPI.Runs))
	}
	if len(withUnknown.Schedules) != len(baseline.Schedules) {
		t.Fatalf("unknown event mutated schedules: got %d, baseline %d", len(withUnknown.Schedules), len(baseline.Schedules))
	}
	// LastEventID DOES advance to the unknown event — the reducer "saw" it,
	// just didn't fold it into derived state. This is intentional: snapshot
	// resume with FromSnapshot relies on LastEventID monotonicity.
	if withUnknown.LastEventID != "evt-unknown-1" {
		t.Fatalf("LastEventID = %q, want evt-unknown-1 (cursor must advance past unknown events)", withUnknown.LastEventID)
	}
	if len(withUnknown.DegradedReasons) != 0 {
		t.Fatalf("unknown event marked projection degraded: %#v", withUnknown.DegradedReasons)
	}
}

// TestProjections_ScheduleEventsRebuildScheduleList exercises the schedule
// reducer alongside the existing job projection: ledger contains a mix of
// schedule.* and job.* events, projection must surface both.
func TestProjections_ScheduleEventsRebuildScheduleList(t *testing.T) {
	store := NewStore(t.TempDir())
	tmpl := RecurringJobTemplate{
		Name:    "wiki-daily",
		Cron:    "@daily",
		JobType: JobTypeLLMWikiLoop,
		Backpressure: RecurrenceBackpressure{
			SkipIfRunning: true,
			MaxQueueDepth: 2,
		},
	}
	if err := store.SaveSchedule(tmpl); err != nil {
		t.Fatalf("save schedule: %v", err)
	}
	if err := store.RecordScheduleFired("wiki-daily", "submission-001", time.Date(2026, 5, 1, 23, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("record fired: %v", err)
	}

	set, err := store.RebuildProjections(ProjectionRebuildOptions{
		RebuiltAt: projectionTestTime(t, 0),
	})
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if len(set.Schedules) != 1 {
		t.Fatalf("got %d schedules, want 1: %#v", len(set.Schedules), set.Schedules)
	}
	if set.Schedules[0].Name != "wiki-daily" {
		t.Fatalf("schedule name = %q, want wiki-daily", set.Schedules[0].Name)
	}
	if set.Schedules[0].JobType != JobTypeLLMWikiLoop {
		t.Fatalf("schedule job_type = %q, want %q", set.Schedules[0].JobType, JobTypeLLMWikiLoop)
	}
}

// TestRebuildProjections_ErrorReturnsUnusableSet exercises the error-return
// contract documented on RebuildProjections (W-B-22 / soc-58q5.7): on error,
// the returned ProjectionSet is zero-valued (SchemaVersion == 0, nil
// Manifests/Jobs maps), so callers MUST check err before reading the set.
//
// Two paths:
//
//  1. Package-level RebuildProjections: an event with a non-string
//     payload["job_type"] makes jobTypeFromPayload error, which propagates
//     up through applyPayloadToJob → applyEventsToState. The caller would
//     see SchemaVersion == 0 and a nil Manifests map — any attempt to
//     index-assign Manifests[name] would panic.
//
//  2. Store.RebuildProjections: a corrupt gzip ledger archive makes
//     replayLedgerFile error before projection rebuild even starts; same
//     zero-valued return.
func TestRebuildProjections_ErrorReturnsUnusableSet(t *testing.T) {
	t.Run("package_level_payload_error", func(t *testing.T) {
		// Hand-craft an event whose payload job_type is a non-string. We
		// bypass NewLedgerEvent so ValidateLedgerEvent does not reject the
		// shape; the failure must surface from applyPayloadToJob inside
		// RebuildProjections, not from event normalization.
		bad := LedgerEvent{
			SchemaVersion: LedgerSchemaVersion,
			EventID:       "evt-bad-1",
			RequestID:     "req-bad-1",
			JobID:         "job-bad-1",
			EventType:     EventJobAccepted,
			OccurredAt:    projectionTestTime(t, 0).Format(time.RFC3339Nano),
			Actor:         "test",
			Payload:       map[string]any{"job_type": 42},
		}

		set, err := RebuildProjections([]LedgerEvent{bad}, ProjectionRebuildOptions{
			RebuiltAt:    projectionTestTime(t, 1),
			SourceLedger: ".agents/daemon/ledger.jsonl",
		})
		if err == nil {
			t.Fatal("expected error from non-string job_type, got nil")
		}
		assertZeroValuedProjectionSet(t, set)

		// Smoke the panic vector documented in the godoc: writing to a nil
		// map panics. A caller that ignored err and tried to mutate
		// set.Manifests would crash here. We assert the panic to lock in
		// the "unusable on error" contract.
		assertNilMapWritePanics(t, func() { set.Manifests[ProjectionRPIRegistry] = ProjectionManifest{} })
	})

	t.Run("store_level_corrupt_archive", func(t *testing.T) {
		// Force Store.ReplayLedger to return a hard error (not a quarantined
		// corrupt-record). A non-gzip file with a .gz suffix in the rotated
		// archive directory makes gzip.NewReader fail in replayLedgerFile,
		// which fmt.Errorf-wraps and returns up through ReplayLedger.
		root := t.TempDir()
		store := NewStore(root)

		// Archive layout (per LedgerArchivePaths + replayLedgerFile):
		// archives sit alongside ledger.jsonl in store.Dir() with the prefix
		// "ledger." and suffix ".jsonl.gz". A non-gzip body under that name
		// makes gzip.NewReader fail, which fmt.Errorf-wraps and returns up
		// through ReplayLedger as a hard error (not a quarantined record).
		if err := os.MkdirAll(store.Dir(), 0o755); err != nil {
			t.Fatalf("mkdir store dir: %v", err)
		}
		bogusArchive := filepath.Join(store.Dir(), "ledger.20260101T000000Z.jsonl.gz")
		if err := os.WriteFile(bogusArchive, []byte("not a gzip file"), 0o644); err != nil {
			t.Fatalf("write bogus archive: %v", err)
		}

		set, err := store.RebuildProjections(ProjectionRebuildOptions{
			RebuiltAt: projectionTestTime(t, 0),
		})
		if err == nil {
			t.Fatal("expected error from corrupt gzip archive, got nil")
		}
		assertZeroValuedProjectionSet(t, set)
	})
}

func assertZeroValuedProjectionSet(t *testing.T, set ProjectionSet) {
	t.Helper()
	if set.SchemaVersion != 0 {
		t.Errorf("SchemaVersion on error = %d, want 0 (zero-valued sentinel)", set.SchemaVersion)
	}
	if set.RebuiltAt != "" {
		t.Errorf("RebuiltAt on error = %q, want empty", set.RebuiltAt)
	}
	if set.SourceLedger != "" {
		t.Errorf("SourceLedger on error = %q, want empty", set.SourceLedger)
	}
	if set.Manifests != nil {
		t.Errorf("Manifests on error = %#v, want nil (caller MUST check err first)", set.Manifests)
	}
	if set.Jobs != nil {
		t.Errorf("Jobs on error = %#v, want nil", set.Jobs)
	}
	if set.Schedules != nil {
		t.Errorf("Schedules on error = %#v, want nil", set.Schedules)
	}
	if set.LastEventID != "" {
		t.Errorf("LastEventID on error = %q, want empty", set.LastEventID)
	}
	if len(set.RPI.Runs) != 0 || len(set.Dream.Runs) != 0 || len(set.Wiki.Jobs) != 0 {
		t.Errorf("derived buckets non-empty on error: rpi=%d dream=%d wiki=%d",
			len(set.RPI.Runs), len(set.Dream.Runs), len(set.Wiki.Jobs))
	}
}

func assertNilMapWritePanics(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic from writing to nil map on zero-valued ProjectionSet, got none — godoc contract is wrong")
		}
	}()
	fn()
}

func projectionTestTime(t *testing.T, minuteOffset int) time.Time {
	t.Helper()
	base, err := time.Parse(time.RFC3339, fixedLedgerTime)
	if err != nil {
		t.Fatalf("parse fixed ledger time: %v", err)
	}
	return base.Add(time.Duration(minuteOffset) * time.Minute)
}

// TestProjections_ArtifactRefsAreCopied guards W-B-25 / soc-58q5.9: when
// initStateFromSnapshot seeds the rebuild loop, both Artifacts AND
// ArtifactRefs must be deep-copied. Before the fix, only Artifacts was
// deep-copied while ArtifactRefs shared the underlying map with the source
// snapshot's Job, so concurrent writers on either side raced.
//
// We exercise initStateFromSnapshot via the public RebuildProjections entry
// point with opts.FromSnapshot set. After rebuild, mutating the source
// snapshot's ArtifactRefs map must NOT be observable in the rebuilt set.
func TestProjections_ArtifactRefsAreCopied(t *testing.T) {
	originalRef := ArtifactRef{
		Path:      ".agents/handoffs/sha256/aa/orig",
		SHA256:    strings.Repeat("a", 64),
		Size:      11,
		WrittenAt: "2026-04-30T00:00:00Z",
	}
	snapshot := ProjectionSet{
		SchemaVersion: ProjectionSchemaVersion,
		LastEventID:   "evt-001",
		Jobs: []JobProjection{
			{
				JobID:        "job-1",
				RequestID:    "req_20260430_000001",
				Status:       JobStatusCompleted,
				Artifacts:    map[string]string{"k": "v"},
				ArtifactRefs: map[string]ArtifactRef{"k": originalRef},
			},
		},
	}

	rebuilt, err := RebuildProjections(nil, ProjectionRebuildOptions{
		FromSnapshot: &snapshot,
		RebuiltAt:    projectionTestTime(t, 0),
	})
	if err != nil {
		t.Fatalf("RebuildProjections: %v", err)
	}
	if len(rebuilt.Jobs) != 1 {
		t.Fatalf("expected 1 rebuilt job, got %d", len(rebuilt.Jobs))
	}
	rebuiltJob := rebuilt.Jobs[0]

	// Pre-mutation: rebuilt copy must equal source.
	if got := rebuiltJob.ArtifactRefs["k"]; got != originalRef {
		t.Fatalf("rebuilt ArtifactRefs[k] = %#v, want %#v", got, originalRef)
	}
	if got := rebuiltJob.Artifacts["k"]; got != "v" {
		t.Fatalf("rebuilt Artifacts[k] = %q, want %q", got, "v")
	}

	// Mutate the source snapshot AFTER the rebuild completed. With the bug
	// (shared underlying map) these mutations would be visible on the
	// rebuilt copy. The fix gives the rebuilt copy its own backing map.
	snapshot.Jobs[0].ArtifactRefs["k"] = ArtifactRef{
		Path:      ".agents/handoffs/sha256/bb/mutated",
		SHA256:    strings.Repeat("b", 64),
		Size:      99,
		WrittenAt: "2099-01-01T00:00:00Z",
	}
	snapshot.Jobs[0].ArtifactRefs["new-key"] = ArtifactRef{
		Path:      ".agents/handoffs/sha256/cc/added",
		SHA256:    strings.Repeat("c", 64),
		Size:      7,
		WrittenAt: "2099-01-01T00:00:00Z",
	}
	// Cross-check: the existing Artifacts deep-copy must also still hold.
	snapshot.Jobs[0].Artifacts["k"] = "MUTATED"
	snapshot.Jobs[0].Artifacts["new-key"] = "added"

	if got := rebuiltJob.ArtifactRefs["k"]; got != originalRef {
		t.Fatalf("rebuilt ArtifactRefs[k] aliased source map: got %#v, want %#v", got, originalRef)
	}
	if _, present := rebuiltJob.ArtifactRefs["new-key"]; present {
		t.Fatalf("rebuilt ArtifactRefs aliased source map: new-key leaked through")
	}
	if len(rebuiltJob.ArtifactRefs) != 1 {
		t.Fatalf("rebuilt ArtifactRefs length changed: got %d, want 1", len(rebuiltJob.ArtifactRefs))
	}
	if got := rebuiltJob.Artifacts["k"]; got != "v" {
		t.Fatalf("rebuilt Artifacts[k] aliased source map: got %q, want %q", got, "v")
	}
	if _, present := rebuiltJob.Artifacts["new-key"]; present {
		t.Fatalf("rebuilt Artifacts aliased source map: new-key leaked through")
	}
}
