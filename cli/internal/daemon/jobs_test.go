package daemon

import (
	"errors"
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
	now := projectionTestTime(t, 0)
	queue := newTestQueue(t, &now, QueueOptions{LeaseDuration: time.Minute})

	if _, err := queue.SubmitJob(SubmitJobInput{RequestID: "req-submit-queued", JobID: "job-queued", JobType: JobTypeDreamRun}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit queued job: %v", err)
	}
	cancelled, err := queue.CancelJob(CancelJobInput{
		JobID:     "job-queued",
		RequestID: "req-cancel-queued",
		Actor:     "operator",
		Reason:    "superseded",
	}, QueueMutationOptions{})
	if err != nil {
		t.Fatalf("cancel queued job: %v", err)
	}
	if cancelled.Outcome != CancelJobOutcomeCancelled || cancelled.Job.Status != JobStatusCancelled {
		t.Fatalf("cancel queued result = %#v, want cancelled outcome and status", cancelled)
	}
	events := readTestQueueEvents(t, queue)
	cancelEvent := events[len(events)-1]
	if cancelEvent.EventType != EventJobCancelled {
		t.Fatalf("last event type = %q, want job.cancelled", cancelEvent.EventType)
	}
	if got, _ := stringPayload(cancelEvent.Payload, "result_status"); got != string(JobResultCancelled) {
		t.Fatalf("result_status = %q, want cancelled", got)
	}
	if got, _ := stringPayload(cancelEvent.Payload, "reason"); got != "superseded" {
		t.Fatalf("reason = %q, want superseded", got)
	}

	beforeRepeat := len(events)
	repeated, err := queue.CancelJob(CancelJobInput{
		JobID:     "job-queued",
		RequestID: "req-cancel-queued-again",
		Actor:     "operator",
		Reason:    "repeat",
	}, QueueMutationOptions{})
	if err != nil {
		t.Fatalf("cancel already cancelled job: %v", err)
	}
	if repeated.Outcome != CancelJobOutcomeAlreadyTerminalCancelled {
		t.Fatalf("repeat outcome = %q, want already_terminal_cancelled", repeated.Outcome)
	}
	if repeated.Job.LastEventID != cancelled.Job.LastEventID {
		t.Fatalf("repeat cancel changed last event: before=%s after=%s", cancelled.Job.LastEventID, repeated.Job.LastEventID)
	}
	if after := len(readTestQueueEvents(t, queue)); after != beforeRepeat {
		t.Fatalf("repeat cancel appended events: before=%d after=%d", beforeRepeat, after)
	}

	if _, err := queue.SubmitJob(SubmitJobInput{RequestID: "req-submit-running", JobID: "job-running", JobType: JobTypeWikiForge}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit running job: %v", err)
	}
	claim, err := queue.ClaimJob("job-running", "worker-1", QueueMutationOptions{})
	if err != nil {
		t.Fatalf("claim running job: %v", err)
	}
	runningCancelled, err := queue.CancelJob(CancelJobInput{
		JobID:     "job-running",
		RequestID: "req-cancel-running",
		Actor:     "operator",
	}, QueueMutationOptions{})
	if err != nil {
		t.Fatalf("cancel running job: %v", err)
	}
	if runningCancelled.Outcome != CancelJobOutcomeCancelled || runningCancelled.Job.Status != JobStatusCancelled {
		t.Fatalf("cancel running result = %#v, want cancelled outcome and status", runningCancelled)
	}
	events = readTestQueueEvents(t, queue)
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

	if _, err := queue.SubmitJob(SubmitJobInput{RequestID: "req-submit-complete", JobID: "job-complete", JobType: JobTypeRPIPhase}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit complete job: %v", err)
	}
	completeClaim, err := queue.ClaimJob("job-complete", "worker-2", QueueMutationOptions{})
	if err != nil {
		t.Fatalf("claim complete job: %v", err)
	}
	completed, err := queue.CompleteJob(CompleteJobInput{
		JobID:      "job-complete",
		RequestID:  "req-complete",
		ClaimToken: completeClaim.ClaimToken,
		LeaseEpoch: completeClaim.LeaseEpoch,
		Actor:      "worker-2",
	}, QueueMutationOptions{})
	if err != nil {
		t.Fatalf("complete job: %v", err)
	}
	terminalCancel, err := queue.CancelJob(CancelJobInput{
		JobID:     "job-complete",
		RequestID: "req-cancel-complete",
		Reason:    "late",
	}, QueueMutationOptions{})
	if err != nil {
		t.Fatalf("cancel completed job: %v", err)
	}
	if terminalCancel.Outcome != CancelJobOutcomeAlreadyTerminalCompleted {
		t.Fatalf("cancel completed outcome = %q, want already_terminal_completed", terminalCancel.Outcome)
	}
	if terminalCancel.Job.LastEventID != completed.LastEventID {
		t.Fatalf("cancel completed changed last event: before=%s after=%s", completed.LastEventID, terminalCancel.Job.LastEventID)
	}

	if _, err := queue.SubmitJob(SubmitJobInput{RequestID: "req-submit-fail", JobID: "job-fail", JobType: JobTypeRPIPhase}, QueueMutationOptions{}); err != nil {
		t.Fatalf("submit fail job: %v", err)
	}
	failClaim, err := queue.ClaimJob("job-fail", "worker-3", QueueMutationOptions{})
	if err != nil {
		t.Fatalf("claim fail job: %v", err)
	}
	failed, err := queue.FailJob(FailJobInput{
		JobID:      "job-fail",
		RequestID:  "req-fail",
		ClaimToken: failClaim.ClaimToken,
		LeaseEpoch: failClaim.LeaseEpoch,
		Actor:      "worker-3",
		Failure:    JobFailure{Code: FailureRequestRejected, Message: "rejected"},
	}, QueueMutationOptions{})
	if err != nil {
		t.Fatalf("fail job: %v", err)
	}
	failedCancel, err := queue.CancelJob(CancelJobInput{
		JobID:     "job-fail",
		RequestID: "req-cancel-fail",
		Reason:    "late",
	}, QueueMutationOptions{})
	if err != nil {
		t.Fatalf("cancel failed job: %v", err)
	}
	if failedCancel.Outcome != CancelJobOutcomeAlreadyTerminalFailed {
		t.Fatalf("cancel failed outcome = %q, want already_terminal_failed", failedCancel.Outcome)
	}
	if failedCancel.Job.LastEventID != failed.LastEventID {
		t.Fatalf("cancel failed changed last event: before=%s after=%s", failed.LastEventID, failedCancel.Job.LastEventID)
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
