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

func newTestQueue(t *testing.T, now *time.Time, opts QueueOptions) *Queue {
	t.Helper()
	if opts.Now == nil {
		opts.Now = func() time.Time { return *now }
	}
	return NewQueue(NewStore(t.TempDir()), opts)
}
