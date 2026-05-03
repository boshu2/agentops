package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
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

func TestRecurrence_MaterializesPartialRPIRunPayload(t *testing.T) {
	tmpl := RecurringJobTemplate{
		Name:    "fast-fire",
		Cron:    "* * * * *",
		JobType: JobTypeRPIRun,
		Payload: json.RawMessage(`{"epic_id":"soc-test"}`),
	}
	payload, err := materializeRecurringJobPayload(tmpl, "sub-123", fixedTickAt)
	if err != nil {
		t.Fatalf("materialize payload: %v", err)
	}
	spec, err := RPIRunJobSpecFromPayload(payload)
	if err != nil {
		t.Fatalf("RPIRunJobSpecFromPayload: %v", err)
	}
	if spec.RunID == "" || spec.Goal == "" || spec.EpicID != "soc-test" {
		t.Fatalf("materialized RPI spec = %#v, want defaults plus epic_id", spec)
	}
}

func TestRecurrence_TickLogsFireAndSkipTelemetry(t *testing.T) {
	var logs bytes.Buffer
	prevOutput := log.Writer()
	prevFlags := log.Flags()
	log.SetOutput(&logs)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevOutput)
		log.SetFlags(prevFlags)
	})

	now := fixedTickAt
	store, queue := newRecurrenceTestRig(t, &now)
	fireTmpl := RecurringJobTemplate{Name: "fire-log", Cron: "*/5 * * * *", JobType: JobTypeLLMWikiLoop}
	if err := store.SaveSchedule(fireTmpl); err != nil {
		t.Fatalf("SaveSchedule fire-log: %v", err)
	}
	sup := NewRecurrenceSupervisor(store, queue, NewFakeClock(now))
	if err := sup.tick(context.Background(), now); err != nil {
		t.Fatalf("tick fire: %v", err)
	}
	subID := submissionID(fireTmpl.Name, now)
	if got := logs.String(); !contains(got, "[recurrence] fired fire-log submission_id="+subID) {
		t.Fatalf("fire telemetry log = %q, want fired line with submission_id", got)
	}

	logs.Reset()
	skipTmpl := RecurringJobTemplate{
		Name:    "skip-log",
		Cron:    "*/5 * * * *",
		JobType: JobTypeLLMWikiLoop,
		Backpressure: RecurrenceBackpressure{
			SkipIfRunning: true,
		},
	}
	if err := store.SaveSchedule(skipTmpl); err != nil {
		t.Fatalf("SaveSchedule skip-log: %v", err)
	}
	preSeedRunningJob(t, queue, skipTmpl.Name, "preseeded-skip-log", JobTypeLLMWikiLoop, now)
	sup = NewRecurrenceSupervisor(store, queue, NewFakeClock(now))
	if err := sup.tick(context.Background(), now); err != nil {
		t.Fatalf("tick skip: %v", err)
	}
	if got := logs.String(); !contains(got, "[recurrence] skipped skip-log reason=skip_if_running:in-flight") {
		t.Fatalf("skip telemetry log = %q, want skipped line with reason", got)
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

func TestRecurrence_BackpressureSoakHundredEveryMinuteSchedules(t *testing.T) {
	now := fixedTickAt
	store, queue := newRecurrenceTestRig(t, &now)
	for i := 0; i < 100; i++ {
		tmpl := RecurringJobTemplate{
			Name:    fmt.Sprintf("soak-%03d", i),
			Cron:    "* * * * *",
			JobType: JobTypeLLMWikiLoop,
			Backpressure: RecurrenceBackpressure{
				MaxQueueDepth: 1,
			},
		}
		if err := store.SaveSchedule(tmpl); err != nil {
			t.Fatalf("SaveSchedule %s: %v", tmpl.Name, err)
		}
	}
	sup := NewRecurrenceSupervisor(store, queue, NewFakeClock(now))
	if err := sup.tick(context.Background(), now); err != nil {
		t.Fatalf("first tick: %v", err)
	}
	if got := len(realQueueJobs(t, queue)); got != 100 {
		t.Fatalf("after first tick queue depth = %d, want 100", got)
	}

	now = now.Add(time.Minute)
	if err := sup.tick(context.Background(), now); err != nil {
		t.Fatalf("second tick: %v", err)
	}
	if got := len(realQueueJobs(t, queue)); got != 100 {
		t.Fatalf("backpressure allowed queue growth: depth = %d, want 100", got)
	}
	if skips := countSkippedEvents(t, store); skips != 100 {
		t.Fatalf("skipped event count = %d, want 100", skips)
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

// TestFireOne_QueueSnapshotConsistent guards against the race where fireOne
// reads the queue twice and the queue drains between reads, producing a skip
// reason that doesn't match the state we acted on. After W-B-07 (soc-58q5.5),
// fireOne must take a single queue snapshot at entry and reuse the same
// (depth, hasInFlight) for both shouldFire and the recorded skip reason.
//
// The Queue type is a concrete struct, so this test asserts the invariant
// behaviorally: it preseeds the queue with two in-flight jobs (depth=2,
// running=true) for a schedule whose template only has SkipIfRunning set.
// Then it asserts the recorded skip reason matches the snapshot taken at
// the start of fireOne (skip_if_running:in-flight). A second snapshot call
// inside fireOne could in principle observe a different reason if anything
// drained between calls, so this also serves as a regression sentinel for
// "fireOne should not re-snapshot mid-decision".
func TestFireOne_QueueSnapshotConsistent(t *testing.T) {
	now := fixedTickAt
	store, queue := newRecurrenceTestRig(t, &now)
	tmpl := RecurringJobTemplate{
		Name:    "snap-consistent",
		Cron:    "*/5 * * * *",
		JobType: JobTypeLLMWikiLoop,
		Backpressure: RecurrenceBackpressure{
			SkipIfRunning: true,
			MaxQueueDepth: 5,
		},
	}
	if err := store.SaveSchedule(tmpl); err != nil {
		t.Fatalf("SaveSchedule: %v", err)
	}
	preSeedRunningJob(t, queue, tmpl.Name, "preseed-a", JobTypeLLMWikiLoop, now)
	preSeedRunningJob(t, queue, tmpl.Name, "preseed-b", JobTypeLLMWikiLoop, now)

	sup := NewRecurrenceSupervisor(store, queue, NewFakeClock(now))

	if err := sup.tick(context.Background(), now); err != nil {
		t.Fatalf("tick: %v", err)
	}

	// Verify the skip reason recorded matches what shouldFire would return for
	// the snapshot taken at fireOne entry. Because SkipIfRunning is checked
	// first in shouldFire, the reason MUST be skip_if_running:in-flight, not
	// max_queue_depth:5 or empty. A drain between two snapshot reads could
	// flip this to max_queue_depth or even allow a fire — which is the bug
	// W-B-07 prevents.
	if got := lastSkipReason(t, store, tmpl.Name); got != "skip_if_running:in-flight" {
		t.Fatalf("skip reason = %q, want %q (snapshot must be consistent across fireOne)", got, "skip_if_running:in-flight")
	}

	// And no new accepted job for this schedule (skip should not have submitted).
	count := 0
	for _, j := range realQueueJobs(t, queue) {
		if jobBelongsToSchedule(j, tmpl.Name) && j.JobID != "preseed-a" && j.JobID != "preseed-b" {
			count++
		}
	}
	if count != 0 {
		t.Fatalf("unexpected new jobs for schedule %q: %d", tmpl.Name, count)
	}
}

// TestFireOne_DerivedQueueStateIsPure pins derivedQueueState as a pure
// function so the single-snapshot invariant in fireOne can rely on reusing
// the same QueueSnapshot for every backpressure-relevant decision. If a
// future change makes derivedQueueState read external state, this test
// becomes the failure surface.
func TestFireOne_DerivedQueueStateIsPure(t *testing.T) {
	snap := QueueSnapshot{
		Jobs: []QueueJobState{
			{JobID: "a", JobType: "llm-wiki-loop", Status: JobStatusRunning, Payload: map[string]any{"schedule_name": "s"}},
			{JobID: "b", JobType: "llm-wiki-loop", Status: JobStatusQueued, Payload: map[string]any{"schedule_name": "s"}},
			{JobID: "c", JobType: "llm-wiki-loop", Status: JobStatusCompleted, Payload: map[string]any{"schedule_name": "s"}},
			{JobID: "d", JobType: "llm-wiki-loop", Status: JobStatusQueued, Payload: map[string]any{"schedule_name": "other"}},
		},
	}
	depth1, inFlight1 := derivedQueueState(snap, "s")
	depth2, inFlight2 := derivedQueueState(snap, "s")
	if depth1 != depth2 || inFlight1 != inFlight2 {
		t.Fatalf("derivedQueueState not pure: (%d,%v) vs (%d,%v)", depth1, inFlight1, depth2, inFlight2)
	}
	if depth1 != 2 {
		t.Fatalf("depth = %d, want 2 (a running + b queued for schedule s; c terminal; d wrong schedule)", depth1)
	}
	if !inFlight1 {
		t.Fatalf("hasInFlight = false, want true (job a is running)")
	}
}

// TestRefreshSchedules_RecomputesNextTickOnCronChange asserts that when a
// schedule template's Cron field is mutated mid-run, refreshSchedules re-parses
// the new cron expression and recomputes nextTick from s.clock.Now() so the
// new cadence takes effect rather than the stale one. (soc-58q5.6 / W-B-10)
//
// The contract from the pre-mortem (Gap-5): recomputation MUST use
// s.clock.Now(), not time.Now(); RecurrenceSupervisor exposes s.clock for
// exactly this reason and direct time.Now() calls would break test-time clock
// injection silently.
func TestRefreshSchedules_RecomputesNextTickOnCronChange(t *testing.T) {
	t0 := fixedTickAt
	store, queue := newRecurrenceTestRig(t, &t0)
	clock := NewFakeClock(t0)
	sup := NewRecurrenceSupervisor(store, queue, clock)

	// Cron A fires every 5 minutes; Cron B fires every hour. Their Next() from
	// t0 differ enough that the test cannot pass by accident.
	cronA := "*/5 * * * *"
	cronB := "0 * * * *"
	tmpl := RecurringJobTemplate{
		Name:    "cadence-shift",
		Cron:    cronA,
		JobType: JobTypeLLMWikiLoop,
	}
	if err := store.SaveSchedule(tmpl); err != nil {
		t.Fatalf("SaveSchedule cronA: %v", err)
	}
	if err := sup.refreshSchedules(t0); err != nil {
		t.Fatalf("refreshSchedules cronA: %v", err)
	}

	sup.mu.Lock()
	stA, ok := sup.schedules[tmpl.Name]
	if !ok {
		sup.mu.Unlock()
		t.Fatalf("schedule %q missing after first refresh", tmpl.Name)
	}
	nextTickA := stA.nextTick
	schedA := stA.sched
	sup.mu.Unlock()

	// Sanity: cron A's first tick from t0-1ns matches what we cached.
	wantA := schedA.Next(t0.Add(-time.Nanosecond))
	if !nextTickA.Equal(wantA) {
		t.Fatalf("nextTick for cronA = %v, want %v", nextTickA, wantA)
	}

	// Operator updates the template's Cron mid-run. Advance the clock too so we
	// can prove the recompute used s.clock.Now() (not the original t0 nor the
	// `now` parameter passed to refreshSchedules).
	clock.Advance(30 * time.Second)
	updated := tmpl
	updated.Cron = cronB
	if err := store.DeleteSchedule(tmpl.Name); err != nil {
		t.Fatalf("DeleteSchedule: %v", err)
	}
	if err := store.SaveSchedule(updated); err != nil {
		t.Fatalf("SaveSchedule cronB: %v", err)
	}

	// Pass the original t0 as `now` to refreshSchedules deliberately. The fix
	// must use s.clock.Now() (which is t0 + 30s) for the recompute, NOT the
	// passed-in `now`. If the implementation regresses to time.Now() or to
	// the stale `now` parameter, this assertion catches it.
	if err := sup.refreshSchedules(t0); err != nil {
		t.Fatalf("refreshSchedules cronB: %v", err)
	}

	sup.mu.Lock()
	stB, ok := sup.schedules[tmpl.Name]
	if !ok {
		sup.mu.Unlock()
		t.Fatalf("schedule %q missing after cron change refresh", tmpl.Name)
	}
	nextTickB := stB.nextTick
	schedB := stB.sched
	gotTemplateCron := stB.template.Cron
	sup.mu.Unlock()

	if gotTemplateCron != cronB {
		t.Fatalf("template.Cron = %q after change, want %q", gotTemplateCron, cronB)
	}
	if nextTickB.Equal(nextTickA) {
		t.Fatalf("nextTick unchanged after Cron change: %v (cron field updated but cadence stale)", nextTickB)
	}
	wantB := schedB.Next(clock.Now())
	if !nextTickB.Equal(wantB) {
		t.Fatalf("nextTick after cron change = %v, want sched_B.Next(clock.Now()) = %v", nextTickB, wantB)
	}
	// Also verify the recompute used the live clock (t0+30s), not stale t0.
	if !nextTickB.Equal(schedB.Next(t0)) && nextTickB.Equal(schedB.Next(t0.Add(-time.Nanosecond))) {
		t.Fatalf("nextTick used the stale `now` parameter (t0) rather than s.clock.Now() (t0+30s)")
	}
}

// TestRefreshSchedules_KeepsPreviousCadenceOnInvalidCronUpdate asserts that
// when a Cron mutation introduces an invalid expression, the supervisor keeps
// the previously-parsed schedule + nextTick rather than dropping the entry,
// so an operator typo cannot break a running schedule mid-flight.
func TestRefreshSchedules_KeepsPreviousCadenceOnInvalidCronUpdate(t *testing.T) {
	t0 := fixedTickAt
	store, queue := newRecurrenceTestRig(t, &t0)
	clock := NewFakeClock(t0)
	sup := NewRecurrenceSupervisor(store, queue, clock)

	tmpl := RecurringJobTemplate{
		Name:    "typo-protect",
		Cron:    "*/5 * * * *",
		JobType: JobTypeLLMWikiLoop,
	}
	if err := store.SaveSchedule(tmpl); err != nil {
		t.Fatalf("SaveSchedule: %v", err)
	}
	if err := sup.refreshSchedules(t0); err != nil {
		t.Fatalf("refreshSchedules initial: %v", err)
	}
	sup.mu.Lock()
	priorState := sup.schedules[tmpl.Name]
	if priorState == nil {
		sup.mu.Unlock()
		t.Fatalf("schedule missing after initial refresh")
	}
	priorSched := priorState.sched
	priorNext := priorState.nextTick
	sup.mu.Unlock()

	// Operator typo: change to an unparseable cron.
	bad := tmpl
	bad.Cron = "not a cron"
	if err := store.DeleteSchedule(tmpl.Name); err != nil {
		t.Fatalf("DeleteSchedule: %v", err)
	}
	if err := store.SaveSchedule(bad); err != nil {
		t.Fatalf("SaveSchedule bad: %v", err)
	}
	if err := sup.refreshSchedules(t0); err != nil {
		t.Fatalf("refreshSchedules after bad: %v", err)
	}

	sup.mu.Lock()
	got := sup.schedules[tmpl.Name]
	sup.mu.Unlock()
	if got == nil {
		t.Fatalf("running schedule was dropped after invalid cron update; want previous cadence kept")
	}
	if got.sched != priorSched {
		t.Fatalf("sched replaced after invalid cron update; want previous parsed schedule retained")
	}
	if !got.nextTick.Equal(priorNext) {
		t.Fatalf("nextTick changed after invalid cron update: got %v want %v", got.nextTick, priorNext)
	}
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

func countSkippedEvents(t *testing.T, store *Store) int {
	t.Helper()
	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("ReadLedger: %v", err)
	}
	n := 0
	for _, ev := range events {
		if ev.EventType == EventScheduleSkipped {
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
