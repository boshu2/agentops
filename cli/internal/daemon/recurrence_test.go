package daemon

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// fixedTickAt is a stable wall-clock time used for deterministic submission_id
// computations across tests.
var fixedTickAt = time.Date(2026, 5, 1, 0, 5, 0, 0, time.UTC)

func TestRecurrence_BackpressureDecision_PureFunction(t *testing.T) {
	cases := []struct {
		name        string
		template    RecurringJobTemplate
		queueDepth  int
		hasInFlight bool
		wantFire    bool
		wantReason  string // substring match; "" means wantFire=true
	}{
		{
			name:        "skip_if_running blocks when in-flight",
			template:    RecurringJobTemplate{Backpressure: RecurrenceBackpressure{SkipIfRunning: true}},
			hasInFlight: true,
			wantFire:    false,
			wantReason:  "in-flight",
		},
		{
			name:        "skip_if_running off does not block on in-flight",
			template:    RecurringJobTemplate{Backpressure: RecurrenceBackpressure{SkipIfRunning: false}},
			hasInFlight: true,
			wantFire:    true,
		},
		{
			name:       "max_queue_depth equals depth blocks",
			template:   RecurringJobTemplate{Backpressure: RecurrenceBackpressure{MaxQueueDepth: 5}},
			queueDepth: 5,
			wantFire:   false,
			wantReason: "5",
		},
		{
			name:       "max_queue_depth above depth allows",
			template:   RecurringJobTemplate{Backpressure: RecurrenceBackpressure{MaxQueueDepth: 5}},
			queueDepth: 4,
			wantFire:   true,
		},
		{
			name:       "max_queue_depth zero means no backpressure",
			template:   RecurringJobTemplate{Backpressure: RecurrenceBackpressure{MaxQueueDepth: 0}},
			queueDepth: 9999,
			wantFire:   true,
		},
		{
			name:        "default zero-value Backpressure allows",
			template:    RecurringJobTemplate{},
			queueDepth:  100,
			hasInFlight: true,
			wantFire:    true,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			fire, reason := shouldFire(tc.template, tc.queueDepth, tc.hasInFlight)
			if fire != tc.wantFire {
				t.Fatalf("fire=%v reason=%q; want fire=%v", fire, reason, tc.wantFire)
			}
			if !tc.wantFire {
				if reason == "" {
					t.Fatalf("expected non-empty skip reason")
				}
				if tc.wantReason != "" && !contains(reason, tc.wantReason) {
					t.Fatalf("reason %q does not contain %q", reason, tc.wantReason)
				}
			} else if reason != "" {
				t.Fatalf("expected empty reason when firing; got %q", reason)
			}
		})
	}
}

func TestRecurrence_SubmissionIDIsDeterministic(t *testing.T) {
	t1 := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 5, 1, 12, 5, 0, 0, time.UTC)
	if got1 := submissionID("foo", t1); got1 != submissionID("foo", t1) {
		t.Fatalf("submissionID not deterministic for same inputs")
	}
	if submissionID("foo", t1) == submissionID("bar", t1) {
		t.Fatalf("submissionID collides across schedule names")
	}
	if submissionID("foo", t1) == submissionID("foo", t2) {
		t.Fatalf("submissionID collides across distinct tick times")
	}
	id := submissionID("foo", t1)
	if len(id) != 16 {
		t.Fatalf("submissionID length=%d, want 16 (truncated sha256 hex)", len(id))
	}
}

func TestRecurrence_TickEnqueuesJob(t *testing.T) {
	now := fixedTickAt
	store, queue := newRecurrenceTestRig(t, &now)
	tmpl := RecurringJobTemplate{
		Name:    "wiki-loop",
		Cron:    "*/5 * * * *",
		JobType: JobTypeLLMWikiLoop,
	}
	if err := store.SaveSchedule(tmpl); err != nil {
		t.Fatalf("SaveSchedule: %v", err)
	}
	sup := NewRecurrenceSupervisor(store, queue, NewFakeClock(now))

	if err := sup.tick(context.Background(), now); err != nil {
		t.Fatalf("tick: %v", err)
	}

	jobs := realQueueJobs(t, queue)
	if len(jobs) != 1 {
		t.Fatalf("want 1 real job submitted; got %d", len(jobs))
	}
	subID := submissionID(tmpl.Name, now)
	if got := jobs[0].IdempotencyKey; got != subID {
		t.Fatalf("idempotency key=%q want %q", got, subID)
	}

	if !ledgerHasFiredEvent(t, store, tmpl.Name, subID) {
		t.Fatalf("ledger missing schedule.fired for submission_id=%q", subID)
	}
}

func TestRecurrence_BackpressureSkipsWhenInFlight(t *testing.T) {
	now := fixedTickAt
	store, queue := newRecurrenceTestRig(t, &now)
	tmpl := RecurringJobTemplate{
		Name:    "wiki-loop",
		Cron:    "*/5 * * * *",
		JobType: JobTypeLLMWikiLoop,
		Backpressure: RecurrenceBackpressure{
			SkipIfRunning: true,
		},
	}
	if err := store.SaveSchedule(tmpl); err != nil {
		t.Fatalf("SaveSchedule: %v", err)
	}

	// Pre-seed an in-flight job tagged with the schedule name.
	preSeedRunningJob(t, queue, tmpl.Name, "preseeded-job", JobTypeLLMWikiLoop, now)

	sup := NewRecurrenceSupervisor(store, queue, NewFakeClock(now))
	if err := sup.tick(context.Background(), now); err != nil {
		t.Fatalf("tick: %v", err)
	}

	// No additional accepted job for this schedule.
	scheduleJobs := 0
	for _, j := range realQueueJobs(t, queue) {
		if jobBelongsToSchedule(j, tmpl.Name) {
			scheduleJobs++
		}
	}
	if scheduleJobs != 1 {
		t.Fatalf("want exactly 1 schedule-tagged job (the pre-seeded one); got %d", scheduleJobs)
	}
	reason := lastSkipReason(t, store, tmpl.Name)
	if !contains(reason, "in-flight") {
		t.Fatalf("expected skipped reason mentioning in-flight; got %q", reason)
	}
}

func TestRecurrence_BackpressureSkipsAtMaxQueueDepth(t *testing.T) {
	now := fixedTickAt
	store, queue := newRecurrenceTestRig(t, &now)
	tmpl := RecurringJobTemplate{
		Name:    "wiki-loop",
		Cron:    "*/5 * * * *",
		JobType: JobTypeLLMWikiLoop,
		Backpressure: RecurrenceBackpressure{
			MaxQueueDepth: 2,
		},
	}
	if err := store.SaveSchedule(tmpl); err != nil {
		t.Fatalf("SaveSchedule: %v", err)
	}
	preSeedQueuedJob(t, queue, tmpl.Name, "preseed-1", JobTypeLLMWikiLoop)
	preSeedQueuedJob(t, queue, tmpl.Name, "preseed-2", JobTypeLLMWikiLoop)

	sup := NewRecurrenceSupervisor(store, queue, NewFakeClock(now))
	if err := sup.tick(context.Background(), now); err != nil {
		t.Fatalf("tick: %v", err)
	}

	scheduleJobs := 0
	for _, j := range realQueueJobs(t, queue) {
		if jobBelongsToSchedule(j, tmpl.Name) {
			scheduleJobs++
		}
	}
	if scheduleJobs != 2 {
		t.Fatalf("want exactly 2 schedule-tagged jobs (no new submission); got %d", scheduleJobs)
	}
	reason := lastSkipReason(t, store, tmpl.Name)
	if !contains(reason, "2") {
		t.Fatalf("expected skipped reason to include max depth value 2; got %q", reason)
	}
}

func TestRecurrence_RestartResumesSchedules(t *testing.T) {
	now := fixedTickAt
	store, queue := newRecurrenceTestRig(t, &now)

	tmplA := RecurringJobTemplate{Name: "alpha", Cron: "*/5 * * * *", JobType: JobTypeLLMWikiLoop}
	tmplB := RecurringJobTemplate{Name: "beta", Cron: "*/5 * * * *", JobType: JobTypeLLMWikiLoop}
	if err := store.SaveSchedule(tmplA); err != nil {
		t.Fatalf("SaveSchedule alpha: %v", err)
	}
	if err := store.SaveSchedule(tmplB); err != nil {
		t.Fatalf("SaveSchedule beta: %v", err)
	}

	// "Restart": construct a fresh supervisor against the same store/queue.
	sup := NewRecurrenceSupervisor(store, queue, NewFakeClock(now))

	listed, err := store.ListSchedules()
	if err != nil {
		t.Fatalf("ListSchedules: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("want 2 schedules in store; got %d", len(listed))
	}

	if err := sup.tick(context.Background(), now); err != nil {
		t.Fatalf("tick: %v", err)
	}

	if !ledgerHasFiredEvent(t, store, "alpha", submissionID("alpha", now)) {
		t.Fatalf("alpha did not fire after restart")
	}
	if !ledgerHasFiredEvent(t, store, "beta", submissionID("beta", now)) {
		t.Fatalf("beta did not fire after restart")
	}
}

func TestRecurrence_CrashBetweenLedgerAndSubmit_DoesNotDoubleFire(t *testing.T) {
	now := fixedTickAt
	store, queue := newRecurrenceTestRig(t, &now)
	tmpl := RecurringJobTemplate{
		Name:    "wiki-loop",
		Cron:    "*/5 * * * *",
		JobType: JobTypeLLMWikiLoop,
	}
	if err := store.SaveSchedule(tmpl); err != nil {
		t.Fatalf("SaveSchedule: %v", err)
	}

	subID := submissionID(tmpl.Name, now)
	// Simulate the crash window: the prior process recorded schedule.fired but
	// crashed before Queue.SubmitJob landed.
	if err := store.RecordScheduleFired(tmpl.Name, subID, now); err != nil {
		t.Fatalf("RecordScheduleFired: %v", err)
	}

	// Restart: new supervisor, same now.
	sup := NewRecurrenceSupervisor(store, queue, NewFakeClock(now))
	if err := sup.tick(context.Background(), now); err != nil {
		t.Fatalf("tick: %v", err)
	}

	// Job should now be submitted by the resume path (idempotency-keyed).
	jobs := realQueueJobs(t, queue)
	if len(jobs) != 1 {
		t.Fatalf("want exactly 1 real job submitted post-resume; got %d", len(jobs))
	}

	// Ledger must NOT carry two schedule.fired events for the same submission_id.
	count := countFiredEvents(t, store, tmpl.Name, subID)
	if count != 1 {
		t.Fatalf("schedule.fired event count for subID=%s is %d; want exactly 1 (no double-fire)", subID, count)
	}

	// Run the same tick again — Queue.SubmitJob's idempotency must keep the
	// total at 1 job and the ledger.fired count at 1.
	if err := sup.tick(context.Background(), now); err != nil {
		t.Fatalf("second tick: %v", err)
	}
	jobs2 := realQueueJobs(t, queue)
	if len(jobs2) != 1 {
		t.Fatalf("idempotency violated: want 1 real job after second tick; got %d", len(jobs2))
	}
	if countFiredEvents(t, store, tmpl.Name, subID) != 1 {
		t.Fatalf("schedule.fired duplicated after second tick")
	}
}

// TestRecurrence_AutoFillsSpecRequiredFields exercises the submitJob auto-fill
// pass: starter schedule payloads (which omit job_type / dream_run_id / mode)
// must still produce spec-valid jobs in the queue.
func TestRecurrence_AutoFillsSpecRequiredFields(t *testing.T) {
	now := fixedTickAt
	store, queue := newRecurrenceTestRig(t, &now)
	tmpl := RecurringJobTemplate{
		Name:    "nightly-dream",
		Cron:    "*/5 * * * *",
		JobType: JobTypeDreamRun,
		Payload: mustEncodePayload(t, map[string]any{
			"schema_version": 1,
			"output_dir":     ".agents/overnight",
		}),
	}
	if err := store.SaveSchedule(tmpl); err != nil {
		t.Fatalf("SaveSchedule: %v", err)
	}
	sup := NewRecurrenceSupervisor(store, queue, NewFakeClock(now))
	if err := sup.tick(context.Background(), now); err != nil {
		t.Fatalf("tick: %v", err)
	}
	jobs := realQueueJobs(t, queue)
	if len(jobs) != 1 {
		t.Fatalf("want 1 real job; got %d", len(jobs))
	}
	got, ok := jobs[0].Payload["job_type"].(string)
	if !ok || got != string(JobTypeDreamRun) {
		t.Fatalf("payload job_type = %v, want %q", jobs[0].Payload["job_type"], JobTypeDreamRun)
	}
	if drID, _ := jobs[0].Payload["dream_run_id"].(string); drID == "" {
		t.Fatalf("payload dream_run_id is empty; auto-fill missed")
	}
	if mode, _ := jobs[0].Payload["mode"].(string); mode != string(DreamModeDaemon) {
		t.Fatalf("payload mode = %v, want %q", jobs[0].Payload["mode"], DreamModeDaemon)
	}
}

func mustEncodePayload(t *testing.T, m map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return b
}

// --- test helpers ---

func newRecurrenceTestRig(t *testing.T, now *time.Time) (*Store, *Queue) {
	t.Helper()
	store := NewStore(t.TempDir())
	queue := NewQueue(store, QueueOptions{
		LeaseDuration: 5 * time.Minute,
		MaxAttempts:   3,
		Now:           func() time.Time { return *now },
	})
	return store, queue
}

func preSeedQueuedJob(t *testing.T, queue *Queue, scheduleName, jobID string, jobType JobType) {
	t.Helper()
	_, err := queue.SubmitJob(SubmitJobInput{
		JobID:   jobID,
		JobType: jobType,
		Actor:   "test",
		Payload: map[string]any{"schedule_name": scheduleName},
	}, QueueMutationOptions{})
	if err != nil {
		t.Fatalf("preSeedQueuedJob: %v", err)
	}
}

func preSeedRunningJob(t *testing.T, queue *Queue, scheduleName, jobID string, jobType JobType, now time.Time) {
	t.Helper()
	preSeedQueuedJob(t, queue, scheduleName, jobID, jobType)
	if _, err := queue.ClaimJob(jobID, "test-worker", QueueMutationOptions{}); err != nil {
		t.Fatalf("ClaimJob: %v", err)
	}
}

func realQueueJobs(t *testing.T, queue *Queue) []QueueJobState {
	t.Helper()
	snap, err := queue.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	out := make([]QueueJobState, 0, len(snap.Jobs))
	for _, job := range snap.Jobs {
		if isRealQueueJob(job) {
			out = append(out, job)
		}
	}
	return out
}

func ledgerHasFiredEvent(t *testing.T, store *Store, name, subID string) bool {
	t.Helper()
	return countFiredEvents(t, store, name, subID) >= 1
}

func countFiredEvents(t *testing.T, store *Store, name, subID string) int {
	t.Helper()
	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("ReadLedger: %v", err)
	}
	n := 0
	for _, ev := range events {
		if ev.EventType != EventScheduleFired {
			continue
		}
		evName, _ := ev.Payload["name"].(string)
		evSub, _ := ev.Payload["submission_id"].(string)
		if evName == name && evSub == subID {
			n++
		}
	}
	return n
}

func lastSkipReason(t *testing.T, store *Store, name string) string {
	t.Helper()
	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("ReadLedger: %v", err)
	}
	reason := ""
	for _, ev := range events {
		if ev.EventType != EventScheduleSkipped {
			continue
		}
		evName, _ := ev.Payload["name"].(string)
		if evName != name {
			continue
		}
		if r, ok := ev.Payload["reason"].(string); ok {
			reason = r
		}
	}
	return reason
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && stringIndex(haystack, needle) >= 0)
}

func stringIndex(haystack, needle string) int {
	// Avoid pulling in strings just for this test helper.
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// silence unused warning for json (keeps imports stable in case a future test
// uses json.RawMessage payloads).
var _ = json.RawMessage(nil)
