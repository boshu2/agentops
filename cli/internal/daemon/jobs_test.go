package daemon

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestQueueSubmitClaimHeartbeatComplete(t *testing.T) {
	now := projectionTestTime(t, 0)
	queue := newTestQueue(t, &now, QueueOptions{LeaseDuration: 5 * time.Minute, MaxAttempts: 2})

	submitted, err := queue.SubmitJob(SubmitJobInput{
		RequestID:      "req-submit",
		JobID:          "job-rpi",
		JobType:        JobTypeRPIRun,
		IdempotencyKey: "idem-rpi",
		Actor:          "ao",
		Payload:        map[string]any{"goal": "ship daemon"},
	}, QueueMutationOptions{})
	if err != nil {
		t.Fatalf("submit job: %v", err)
	}
	if submitted.Status != JobStatusQueued {
		t.Fatalf("submitted status = %q, want queued", submitted.Status)
	}

	claim, err := queue.ClaimNext("worker-1", QueueMutationOptions{})
	if err != nil {
		t.Fatalf("claim next: %v", err)
	}
	if claim.Job.JobID != "job-rpi" || claim.Job.Status != JobStatusRunning {
		t.Fatalf("claim job = %#v, want running job-rpi", claim.Job)
	}
	if _, err := queue.ClaimJob("job-rpi", "worker-2", QueueMutationOptions{}); !errors.Is(err, ErrJobAlreadyClaimed) {
		t.Fatalf("duplicate claim error = %v, want ErrJobAlreadyClaimed", err)
	}

	now = now.Add(time.Minute)
	heartbeat, err := queue.Heartbeat(HeartbeatInput{
		JobID:      "job-rpi",
		RequestID:  "req-heartbeat",
		ClaimToken: claim.ClaimToken,
		LeaseEpoch: claim.LeaseEpoch,
		Actor:      "worker-1",
	}, QueueMutationOptions{})
	if err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	if heartbeat.Status != JobStatusRunning || heartbeat.LeaseExpiresAt == claim.LeaseExpiresAt {
		t.Fatalf("heartbeat did not keep job running with renewed lease: %#v", heartbeat)
	}

	completed, err := queue.CompleteJob(CompleteJobInput{
		JobID:      "job-rpi",
		RequestID:  "req-complete",
		ClaimToken: claim.ClaimToken,
		LeaseEpoch: claim.LeaseEpoch,
		Actor:      "worker-1",
		Artifacts:  map[string]string{"summary": ".agents/rpi/runs/job-rpi/phase-3-summary.md"},
	}, QueueMutationOptions{})
	if err != nil {
		t.Fatalf("complete job: %v", err)
	}
	if completed.Status != JobStatusCompleted {
		t.Fatalf("completed status = %q, want completed", completed.Status)
	}
	again, err := queue.CompleteJob(CompleteJobInput{
		JobID:      "job-rpi",
		RequestID:  "req-complete-again",
		ClaimToken: claim.ClaimToken,
		LeaseEpoch: claim.LeaseEpoch,
	}, QueueMutationOptions{})
	if err != nil {
		t.Fatalf("idempotent complete: %v", err)
	}
	if again.LastEventID != completed.LastEventID {
		t.Fatalf("terminal idempotency appended another event: before=%s after=%s", completed.LastEventID, again.LastEventID)
	}
}

func TestLeaseExpiryAllowsReclaimWithEpochAndRejectsStaleClaim(t *testing.T) {
	now := projectionTestTime(t, 0)
	queue := newTestQueue(t, &now, QueueOptions{LeaseDuration: time.Minute, MaxAttempts: 3})
	if _, err := queue.SubmitJob(SubmitJobInput{RequestID: "req-submit", JobID: "job-dream", JobType: JobTypeDreamRun}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	first, err := queue.ClaimJob("job-dream", "worker-1", QueueMutationOptions{})
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}

	now = now.Add(2 * time.Minute)
	if _, err := queue.Heartbeat(HeartbeatInput{
		JobID:      "job-dream",
		RequestID:  "req-stale-heartbeat",
		ClaimToken: first.ClaimToken,
		LeaseEpoch: first.LeaseEpoch,
	}, QueueMutationOptions{}); !errors.Is(err, ErrLeaseExpired) {
		t.Fatalf("stale heartbeat error = %v, want ErrLeaseExpired", err)
	}
	second, err := queue.ClaimJob("job-dream", "worker-2", QueueMutationOptions{})
	if err != nil {
		t.Fatalf("second claim after expiry: %v", err)
	}
	if second.LeaseEpoch != first.LeaseEpoch+1 {
		t.Fatalf("second lease epoch = %d, want %d", second.LeaseEpoch, first.LeaseEpoch+1)
	}
	if second.ClaimToken == first.ClaimToken {
		t.Fatalf("claim token was not rotated after lease expiry")
	}
	if _, err := queue.CompleteJob(CompleteJobInput{
		JobID:      "job-dream",
		RequestID:  "req-stale-complete",
		ClaimToken: first.ClaimToken,
		LeaseEpoch: first.LeaseEpoch,
	}, QueueMutationOptions{}); !errors.Is(err, ErrClaimFenceMismatch) {
		t.Fatalf("stale complete error = %v, want ErrClaimFenceMismatch", err)
	}
}

func TestQueueRetryCapFailsExpiredJob(t *testing.T) {
	now := projectionTestTime(t, 0)
	queue := newTestQueue(t, &now, QueueOptions{LeaseDuration: time.Minute, MaxAttempts: 1})
	if _, err := queue.SubmitJob(SubmitJobInput{RequestID: "req-submit", JobID: "job-wiki", JobType: JobTypeWikiForge}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	if _, err := queue.ClaimJob("job-wiki", "worker-1", QueueMutationOptions{}); err != nil {
		t.Fatalf("claim job: %v", err)
	}

	now = now.Add(2 * time.Minute)
	if _, err := queue.ClaimNext("worker-2", QueueMutationOptions{}); !errors.Is(err, ErrNoClaimableJobs) {
		t.Fatalf("claim after retry cap error = %v, want ErrNoClaimableJobs", err)
	}
	snapshot, err := queue.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	job, err := snapshot.jobByID("job-wiki")
	if err != nil {
		t.Fatalf("job lookup: %v", err)
	}
	if job.Status != JobStatusFailed {
		t.Fatalf("job status = %q, want failed", job.Status)
	}
	if job.Failure == nil || job.Failure.Code != FailureRetryExhausted {
		t.Fatalf("failure = %#v, want retry_exhausted", job.Failure)
	}
}

func TestAckFailpointAfterAppendBeforeAckIsRecoverableByIdempotency(t *testing.T) {
	now := projectionTestTime(t, 0)
	queue := newTestQueue(t, &now, QueueOptions{})
	input := SubmitJobInput{
		RequestID:      "req-submit",
		JobID:          "job-rpi",
		JobType:        JobTypeRPIPhase,
		IdempotencyKey: "idem-rpi-phase",
	}
	if _, err := queue.SubmitJob(input, QueueMutationOptions{Failpoint: QueueFailpointAfterAppendBeforeAck}); !errors.Is(err, ErrFailpoint) {
		t.Fatalf("submit failpoint error = %v, want ErrFailpoint", err)
	}
	recovered, err := queue.SubmitJob(input, QueueMutationOptions{})
	if err != nil {
		t.Fatalf("retry submit after lost ack: %v", err)
	}
	if recovered.JobID != "job-rpi" || recovered.Status != JobStatusQueued {
		t.Fatalf("recovered job = %#v, want queued job-rpi", recovered)
	}
	events, err := queue.store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("ledger has %d events, want one accepted event after idempotent retry", len(events))
	}
}

func TestFailpointBeforeAppendDoesNotAcceptQueueMutation(t *testing.T) {
	now := projectionTestTime(t, 0)
	queue := newTestQueue(t, &now, QueueOptions{})
	_, err := queue.SubmitJob(SubmitJobInput{
		RequestID: "req-submit",
		JobID:     "job-rpi",
		JobType:   JobTypeRPIPhase,
	}, QueueMutationOptions{Failpoint: QueueFailpointBeforeAppend})
	if !errors.Is(err, ErrFailpoint) {
		t.Fatalf("submit failpoint error = %v, want ErrFailpoint", err)
	}
	events, err := queue.store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("ledger has %d events after before-append failpoint, want 0", len(events))
	}
}

func TestQueue_ClaimNextMatchingSkipsUnsupported(t *testing.T) {
	now := projectionTestTime(t, 0)
	queue := newTestQueue(t, &now, QueueOptions{LeaseDuration: time.Minute})

	if _, err := queue.SubmitJob(SubmitJobInput{RequestID: "req-dream", JobID: "job-dream", JobType: JobTypeDreamRun}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit dream job: %v", err)
	}
	if _, err := queue.SubmitJob(SubmitJobInput{RequestID: "req-openclaw", JobID: "job-openclaw", JobType: JobTypeOpenClawSnapshot}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit openclaw job: %v", err)
	}

	claim, err := queue.ClaimNextMatching("openclaw-worker", func(job QueueJobState) bool {
		return job.JobType == JobTypeOpenClawSnapshot
	}, QueueMutationOptions{})
	if err != nil {
		t.Fatalf("claim matching job: %v", err)
	}
	if claim.Job.JobID != "job-openclaw" || claim.Job.Status != JobStatusRunning {
		t.Fatalf("claim = %#v, want running job-openclaw", claim.Job)
	}

	snapshot, err := queue.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	dream, err := snapshot.jobByID("job-dream")
	if err != nil {
		t.Fatalf("dream lookup: %v", err)
	}
	if dream.Status != JobStatusQueued {
		t.Fatalf("dream status = %q, want queued", dream.Status)
	}

	_, err = queue.ClaimNextMatching("wiki-worker", func(job QueueJobState) bool {
		return job.JobType == JobTypeWikiForge
	}, QueueMutationOptions{})
	if !errors.Is(err, ErrNoClaimableJobs) {
		t.Fatalf("claim unsupported matcher error = %v, want ErrNoClaimableJobs", err)
	}
}

func TestQueue_CancelJob(t *testing.T) {
	t.Run("queued job appends cancellation event and repeated cancel is idempotent", func(t *testing.T) {
		assertCancelQueuedJob(t)
	})
	t.Run("running job cancels and rejects stale heartbeat", func(t *testing.T) {
		assertCancelRunningJob(t)
	})
	t.Run("completed job returns terminal outcome without appending", func(t *testing.T) {
		assertCancelCompletedJob(t)
	})
	t.Run("failed job returns terminal outcome without appending", func(t *testing.T) {
		assertCancelFailedJob(t)
	})
}

func assertCancelQueuedJob(t *testing.T) {
	t.Helper()
	queue := newCancelJobTestQueue(t)
	submitCancelTestJob(t, queue, "req-submit-queued", "job-queued", JobTypeDreamRun)
	cancelled := cancelTestJob(t, queue, CancelJobInput{
		JobID:     "job-queued",
		RequestID: "req-cancel-queued",
		Actor:     "operator",
		Reason:    "superseded",
	})
	assertCancelResult(t, cancelled, CancelJobOutcomeCancelled, JobStatusCancelled)
	events := readTestQueueEvents(t, queue)
	assertCancelEventPayload(t, events[len(events)-1], "superseded")
	assertRepeatCancelDoesNotAppend(t, queue, cancelled, len(events))
}

func assertCancelRunningJob(t *testing.T) {
	t.Helper()
	queue := newCancelJobTestQueue(t)
	submitCancelTestJob(t, queue, "req-submit-running", "job-running", JobTypeWikiForge)
	claim := claimCancelTestJob(t, queue, "job-running", "worker-1")
	runningCancelled := cancelTestJob(t, queue, CancelJobInput{
		JobID:     "job-running",
		RequestID: "req-cancel-running",
		Actor:     "operator",
	})
	assertCancelResult(t, runningCancelled, CancelJobOutcomeCancelled, JobStatusCancelled)
	events := readTestQueueEvents(t, queue)
	if got, _ := stringPayload(events[len(events)-1].Payload, "reason"); got == "" {
		t.Fatalf("cancel event reason is empty")
	}
	if _, err := queue.Heartbeat(HeartbeatInput{
		JobID:      "job-running",
		RequestID:  "req-stale-heartbeat",
		ClaimToken: claim.ClaimToken,
		LeaseEpoch: claim.LeaseEpoch,
		Actor:      "worker-1",
	}, QueueMutationOptions{}); !errors.Is(err, ErrNoClaimableJobs) {
		t.Fatalf("heartbeat after cancel error = %v, want ErrNoClaimableJobs", err)
	}
}

func assertCancelCompletedJob(t *testing.T) {
	t.Helper()
	queue := newCancelJobTestQueue(t)
	submitCancelTestJob(t, queue, "req-submit-complete", "job-complete", JobTypeRPIPhase)
	completed := completeCancelTestJob(t, queue, "job-complete")
	terminalCancel := cancelTestJob(t, queue, CancelJobInput{
		JobID:     "job-complete",
		RequestID: "req-cancel-complete",
		Reason:    "late",
	})
	if terminalCancel.Outcome != CancelJobOutcomeAlreadyTerminalCompleted {
		t.Fatalf("cancel completed outcome = %q, want already_terminal_completed", terminalCancel.Outcome)
	}
	assertCancelDidNotChangeLastEvent(t, terminalCancel.Job, completed)
}

func assertCancelFailedJob(t *testing.T) {
	t.Helper()
	queue := newCancelJobTestQueue(t)
	submitCancelTestJob(t, queue, "req-submit-fail", "job-fail", JobTypeRPIPhase)
	failed := failCancelTestJob(t, queue, "job-fail")
	failedCancel := cancelTestJob(t, queue, CancelJobInput{
		JobID:     "job-fail",
		RequestID: "req-cancel-fail",
		Reason:    "late",
	})
	if failedCancel.Outcome != CancelJobOutcomeAlreadyTerminalFailed {
		t.Fatalf("cancel failed outcome = %q, want already_terminal_failed", failedCancel.Outcome)
	}
	assertCancelDidNotChangeLastEvent(t, failedCancel.Job, failed)
}

func newCancelJobTestQueue(t *testing.T) *Queue {
	t.Helper()
	now := projectionTestTime(t, 0)
	return newTestQueue(t, &now, QueueOptions{LeaseDuration: time.Minute})
}

func submitCancelTestJob(t *testing.T, queue *Queue, requestID, jobID string, jobType JobType) {
	t.Helper()
	if _, err := queue.SubmitJob(SubmitJobInput{RequestID: RequestID(requestID), JobID: jobID, JobType: jobType}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit %s: %v", jobID, err)
	}
}

func cancelTestJob(t *testing.T, queue *Queue, input CancelJobInput) CancelJobResult {
	t.Helper()
	result, err := queue.CancelJob(input, QueueMutationOptions{})
	if err != nil {
		t.Fatalf("cancel %s: %v", input.JobID, err)
	}
	return result
}

func assertCancelResult(t *testing.T, result CancelJobResult, wantOutcome CancelJobOutcome, wantStatus JobStatus) {
	t.Helper()
	if result.Outcome != wantOutcome || result.Job.Status != wantStatus {
		t.Fatalf("cancel result = %#v, want outcome %s and status %s", result, wantOutcome, wantStatus)
	}
}

func assertCancelEventPayload(t *testing.T, event LedgerEvent, wantReason string) {
	t.Helper()
	if event.EventType != EventJobCancelled {
		t.Fatalf("last event type = %q, want job.cancelled", event.EventType)
	}
	if got, _ := stringPayload(event.Payload, "result_status"); got != string(JobResultCancelled) {
		t.Fatalf("result_status = %q, want cancelled", got)
	}
	if got, _ := stringPayload(event.Payload, "reason"); got != wantReason {
		t.Fatalf("reason = %q, want %s", got, wantReason)
	}
}

func assertRepeatCancelDoesNotAppend(t *testing.T, queue *Queue, cancelled CancelJobResult, beforeRepeat int) {
	t.Helper()
	repeated := cancelTestJob(t, queue, CancelJobInput{
		JobID:     cancelled.Job.JobID,
		RequestID: "req-cancel-queued-again",
		Actor:     "operator",
		Reason:    "repeat",
	})
	if repeated.Outcome != CancelJobOutcomeAlreadyTerminalCancelled {
		t.Fatalf("repeat outcome = %q, want already_terminal_cancelled", repeated.Outcome)
	}
	assertCancelDidNotChangeLastEvent(t, repeated.Job, cancelled.Job)
	if after := len(readTestQueueEvents(t, queue)); after != beforeRepeat {
		t.Fatalf("repeat cancel appended events: before=%d after=%d", beforeRepeat, after)
	}
}

func claimCancelTestJob(t *testing.T, queue *Queue, jobID, actor string) QueueClaim {
	t.Helper()
	claim, err := queue.ClaimJob(jobID, actor, QueueMutationOptions{})
	if err != nil {
		t.Fatalf("claim %s: %v", jobID, err)
	}
	return claim
}

func completeCancelTestJob(t *testing.T, queue *Queue, jobID string) QueueJobState {
	t.Helper()
	claim := claimCancelTestJob(t, queue, jobID, "worker-2")
	completed, err := queue.CompleteJob(CompleteJobInput{
		JobID:      jobID,
		RequestID:  "req-complete",
		ClaimToken: claim.ClaimToken,
		LeaseEpoch: claim.LeaseEpoch,
		Actor:      "worker-2",
	}, QueueMutationOptions{})
	if err != nil {
		t.Fatalf("complete %s: %v", jobID, err)
	}
	return completed
}

func failCancelTestJob(t *testing.T, queue *Queue, jobID string) QueueJobState {
	t.Helper()
	claim := claimCancelTestJob(t, queue, jobID, "worker-3")
	failed, err := queue.FailJob(FailJobInput{
		JobID:      jobID,
		RequestID:  "req-fail",
		ClaimToken: claim.ClaimToken,
		LeaseEpoch: claim.LeaseEpoch,
		Actor:      "worker-3",
		Failure:    JobFailure{Code: FailureRequestRejected, Message: "rejected"},
	}, QueueMutationOptions{})
	if err != nil {
		t.Fatalf("fail %s: %v", jobID, err)
	}
	return failed
}

func assertCancelDidNotChangeLastEvent(t *testing.T, after, before QueueJobState) {
	t.Helper()
	if after.LastEventID != before.LastEventID {
		t.Fatalf("cancel changed last event: before=%s after=%s", before.LastEventID, after.LastEventID)
	}
}

func TestQueue_DuplicateTerminalEventsDoNotMutateFinalState(t *testing.T) {
	t.Run("completed before cancelled", func(t *testing.T) {
		now := projectionTestTime(t, 0)
		queue := newTestQueue(t, &now, QueueOptions{LeaseDuration: time.Minute})
		if _, err := queue.SubmitJob(SubmitJobInput{RequestID: "req-submit", JobID: "job-complete", JobType: JobTypeRPIPhase}, QueueMutationOptions{}); err != nil {
			t.Fatalf("submit job: %v", err)
		}
		claim, err := queue.ClaimJob("job-complete", "worker-1", QueueMutationOptions{})
		if err != nil {
			t.Fatalf("claim job: %v", err)
		}
		completed, err := queue.CompleteJob(CompleteJobInput{
			JobID:      "job-complete",
			RequestID:  "req-complete",
			ClaimToken: claim.ClaimToken,
			LeaseEpoch: claim.LeaseEpoch,
			Actor:      "worker-1",
		}, QueueMutationOptions{})
		if err != nil {
			t.Fatalf("complete job: %v", err)
		}

		appendRawQueueEvent(t, queue.store, LedgerEventInput{
			EventID:    "evt_cancel_after_complete",
			RequestID:  "req-cancel-after-complete",
			JobID:      "job-complete",
			EventType:  EventJobCancelled,
			OccurredAt: now.Add(time.Minute),
			Actor:      "operator",
			Payload: map[string]any{
				"result_status": string(JobResultCancelled),
				"reason":        "late",
			},
		})

		snapshot, err := queue.Snapshot()
		if err != nil {
			t.Fatalf("snapshot: %v", err)
		}
		job, err := snapshot.jobByID("job-complete")
		if err != nil {
			t.Fatalf("job lookup: %v", err)
		}
		if job.Status != JobStatusCompleted {
			t.Fatalf("job status = %q, want completed", job.Status)
		}
		if job.LastEventID != completed.LastEventID {
			t.Fatalf("last event = %s, want first terminal %s", job.LastEventID, completed.LastEventID)
		}
	})

	t.Run("cancelled before completed", func(t *testing.T) {
		now := projectionTestTime(t, 0)
		queue := newTestQueue(t, &now, QueueOptions{LeaseDuration: time.Minute})
		if _, err := queue.SubmitJob(SubmitJobInput{RequestID: "req-submit", JobID: "job-cancel", JobType: JobTypeRPIPhase}, QueueMutationOptions{}); err != nil {
			t.Fatalf("submit job: %v", err)
		}
		cancelled, err := queue.CancelJob(CancelJobInput{
			JobID:     "job-cancel",
			RequestID: "req-cancel",
			Actor:     "operator",
			Reason:    "stop",
		}, QueueMutationOptions{})
		if err != nil {
			t.Fatalf("cancel job: %v", err)
		}

		appendRawQueueEvent(t, queue.store, LedgerEventInput{
			EventID:    "evt_complete_after_cancel",
			RequestID:  "req-complete-after-cancel",
			JobID:      "job-cancel",
			EventType:  EventJobCompleted,
			OccurredAt: now.Add(time.Minute),
			Actor:      "worker-1",
			Payload: map[string]any{
				"result_status": string(JobResultSucceeded),
			},
		})

		snapshot, err := queue.Snapshot()
		if err != nil {
			t.Fatalf("snapshot: %v", err)
		}
		job, err := snapshot.jobByID("job-cancel")
		if err != nil {
			t.Fatalf("job lookup: %v", err)
		}
		if job.Status != JobStatusCancelled {
			t.Fatalf("job status = %q, want cancelled", job.Status)
		}
		if job.LastEventID != cancelled.Job.LastEventID {
			t.Fatalf("last event = %s, want first terminal %s", job.LastEventID, cancelled.Job.LastEventID)
		}
	})
}

func TestQueue_RestartReclaimsExpiredRunningJob(t *testing.T) {
	now := projectionTestTime(t, 0)
	store := NewStore(t.TempDir())
	queueOpts := QueueOptions{
		LeaseDuration: time.Minute,
		MaxAttempts:   3,
		Now:           func() time.Time { return now },
	}
	queue := NewQueue(store, queueOpts)
	if _, err := queue.SubmitJob(SubmitJobInput{RequestID: "req-submit", JobID: "job-dream", JobType: JobTypeDreamRun}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit job: %v", err)
	}
	first, err := queue.ClaimJob("job-dream", "worker-1", QueueMutationOptions{})
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}

	now = now.Add(2 * time.Minute)
	restarted := NewQueue(store, queueOpts)
	second, err := restarted.ClaimNext("worker-2", QueueMutationOptions{})
	if err != nil {
		t.Fatalf("claim after restart: %v", err)
	}
	if second.Job.JobID != "job-dream" || second.Job.Status != JobStatusRunning {
		t.Fatalf("second claim job = %#v, want running job-dream", second.Job)
	}
	if second.LeaseEpoch != first.LeaseEpoch+1 {
		t.Fatalf("second lease epoch = %d, want %d", second.LeaseEpoch, first.LeaseEpoch+1)
	}
	if second.ClaimToken == first.ClaimToken {
		t.Fatalf("claim token was not rotated after restart reclaim")
	}

	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	var claimIDs []string
	for _, event := range events {
		if event.JobID != "job-dream" {
			continue
		}
		if event.EventType == EventJobClaimed {
			claimIDs = append(claimIDs, event.EventID)
		}
	}
	if len(claimIDs) != 2 {
		t.Fatalf("claim event count = %d (%v), want 2", len(claimIDs), claimIDs)
	}
	if claimIDs[0] == claimIDs[1] {
		t.Fatalf("restart generated duplicate claim event ID %q", claimIDs[0])
	}
}

// TestSubmitJob_DedupRelyOnStoreLock is the contract test for the
// IdempotencyKey dedup pre-check in SubmitJob. It spawns 50 goroutines that
// concurrently call SubmitJob on the same Queue with the same
// IdempotencyKey, then asserts that exactly one unique JobID is returned
// across all callers.
//
// The pre-check at the top of SubmitJob runs OUTSIDE Store.AppendLedgerEvent's
// mutex (see the comment in jobs.go SubmitJob), and AppendLedgerEvent dedups
// by EventID rather than by IdempotencyKey. This test pins the dedup contract
// the API surface promises: same IdempotencyKey -> same JobID for all
// callers, even under concurrent submission. If the queue ever produces
// multiple JobIDs for a single IdempotencyKey, this test fails and the audit
// finding (W-B-05) is reproduced.
func TestSubmitJob_DedupRelyOnStoreLock(t *testing.T) {
	now := projectionTestTime(t, 0)
	queue := newTestQueue(t, &now, QueueOptions{LeaseDuration: time.Minute, MaxAttempts: 3})

	const submitters = 50
	const idemKey = "idem-concurrent-submit"

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		jobIDs  = make(map[string]int, 1)
		errs    []error
		startCh = make(chan struct{})
	)

	wg.Add(submitters)
	for i := 0; i < submitters; i++ {
		go func() {
			defer wg.Done()
			<-startCh
			job, err := queue.SubmitJob(SubmitJobInput{
				JobType:        JobTypeRPIRun,
				IdempotencyKey: idemKey,
				Actor:          "ao",
				Payload:        map[string]any{"goal": "ship daemon"},
			}, QueueMutationOptions{})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			jobIDs[job.JobID]++
		}()
	}
	close(startCh)
	wg.Wait()

	if len(errs) > 0 {
		t.Fatalf("concurrent SubmitJob returned %d errors; first: %v", len(errs), errs[0])
	}
	if len(jobIDs) != 1 {
		t.Fatalf("concurrent same-IdempotencyKey submit produced %d distinct JobIDs (%v), want 1", len(jobIDs), jobIDs)
	}
	var observed int
	for _, count := range jobIDs {
		observed = count
	}
	if observed != submitters {
		t.Fatalf("observed %d successful submits across one JobID, want %d", observed, submitters)
	}

	// Ledger contract: exactly one JobAccepted event for the deduped key.
	events, err := queue.store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	var accepted int
	for _, event := range events {
		if event.EventType == EventJobAccepted {
			accepted++
		}
	}
	if accepted != 1 {
		t.Fatalf("ledger has %d JobAccepted events for one IdempotencyKey, want 1", accepted)
	}
}

// TestSubmitJob_RejectsReservedJobID pins the contract that user-supplied
// JobIDs cannot collide with the schedule sentinel. The schedule.fired and
// schedule.skipped events use scheduleSentinelJobID ("schedule") as their
// JobID; allowing a user-submitted JobID with the same value would conflate
// schedule-scoped events with a real job's lifecycle. Auto-generated JobIDs
// use a "job_" prefix so they cannot collide; only the exact sentinel value
// is rejected. The "schedule_" prefix on user-supplied IDs is allowed.
func TestSubmitJob_RejectsReservedJobID(t *testing.T) {
	now := projectionTestTime(t, 0)
	queue := newTestQueue(t, &now, QueueOptions{LeaseDuration: time.Minute, MaxAttempts: 3})

	_, err := queue.SubmitJob(SubmitJobInput{
		JobID:   scheduleSentinelJobID,
		JobType: JobTypeRPIRun,
		Actor:   "ao",
		Payload: map[string]any{"goal": "should be rejected"},
	}, QueueMutationOptions{})
	if err == nil {
		t.Fatalf("SubmitJob with reserved JobID %q returned nil error, want rejection", scheduleSentinelJobID)
	}
	msg := err.Error()
	if !strings.Contains(msg, "reserved") {
		t.Fatalf("error message %q does not contain %q", msg, "reserved")
	}
	if !strings.Contains(msg, scheduleSentinelJobID) {
		t.Fatalf("error message %q does not contain sentinel value %q", msg, scheduleSentinelJobID)
	}

	// Verify no event was appended for the rejected submission.
	events := readTestQueueEvents(t, queue)
	if len(events) != 0 {
		t.Fatalf("rejected SubmitJob appended %d events, want 0: %+v", len(events), events)
	}

	// Sibling check: a user-supplied JobID with the "schedule_" prefix is
	// allowed — only the exact sentinel string is reserved.
	allowed, err := queue.SubmitJob(SubmitJobInput{
		JobID:   "schedule_user_supplied",
		JobType: JobTypeRPIRun,
		Actor:   "ao",
		Payload: map[string]any{"goal": "should pass"},
	}, QueueMutationOptions{})
	if err != nil {
		t.Fatalf("SubmitJob with schedule_-prefixed JobID returned error: %v", err)
	}
	if allowed.JobID != "schedule_user_supplied" {
		t.Fatalf("allowed.JobID = %q, want %q", allowed.JobID, "schedule_user_supplied")
	}
}

func newTestQueue(t *testing.T, now *time.Time, opts QueueOptions) *Queue {
	t.Helper()
	if opts.Now == nil {
		opts.Now = func() time.Time { return *now }
	}
	return NewQueue(NewStore(t.TempDir()), opts)
}

func readTestQueueEvents(t *testing.T, queue *Queue) []LedgerEvent {
	t.Helper()
	events, err := queue.store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	return events
}

func appendRawQueueEvent(t *testing.T, store *Store, input LedgerEventInput) {
	t.Helper()
	event, err := NewLedgerEvent(input)
	if err != nil {
		t.Fatalf("new ledger event: %v", err)
	}
	if _, err := store.AppendLedgerEvent(event); err != nil {
		t.Fatalf("append ledger event: %v", err)
	}
}
