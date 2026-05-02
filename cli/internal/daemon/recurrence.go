package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// Pre-mortem amendment A4 contract (write-order + cancel precedence):
//
//  1. Compute submission_id = sha256("<schedule_name>:<tick_unix_seconds>")
//     (deterministic; first 16 hex chars). On crash between AppendLedgerEvent
//     and Queue.Submit, the next tick at the same time will reuse this ID and
//     queue idempotency keeps the second submit a no-op.
//  2. Pre-fire backpressure check first; record schedule.skipped if blocked.
//  3. AppendLedgerEvent("schedule.fired") via Store.RecordScheduleFired.
//  4. THEN Queue.SubmitJob with IdempotencyKey=submission_id.
//
// Cancel-vs-terminal precedence: ledger-append-order wins. If a cancel is
// received after a terminal event has already landed, the cancel is a no-op
// + WARN log (this rule is enforced inside Queue.CancelJob today; the
// recurrence supervisor does not initiate cancels — see soc-5of TB-04a/.5).

// RecurrenceSupervisor ticks RecurringJobTemplate schedules on cron cadence
// and submits jobs into the daemon Queue. One Start() goroutine per supervisor.
type RecurrenceSupervisor struct {
	store *Store
	queue *Queue
	clock Clock

	pollInterval time.Duration

	mu        sync.Mutex
	schedules map[string]*scheduleState
}

type scheduleState struct {
	template RecurringJobTemplate
	sched    cron.Schedule
	nextTick time.Time
}

// NewRecurrenceSupervisor builds a supervisor bound to the given store + queue.
// Production callers pass RealClock{}; tests pass a *FakeClock from clock_test.
func NewRecurrenceSupervisor(store *Store, queue *Queue, clock Clock) *RecurrenceSupervisor {
	if clock == nil {
		clock = RealClock{}
	}
	return &RecurrenceSupervisor{
		store:        store,
		queue:        queue,
		clock:        clock,
		pollInterval: time.Minute,
		schedules:    map[string]*scheduleState{},
	}
}

// WithPollInterval overrides the cadence at which Start() re-reads the
// schedule list and runs tick(). Defaults to 1 minute (matches cron's
// 5-field minimum granularity, amendment B4).
func (s *RecurrenceSupervisor) WithPollInterval(d time.Duration) *RecurrenceSupervisor {
	if d > 0 {
		s.pollInterval = d
	}
	return s
}

// Start runs the supervisor loop until ctx is cancelled. It re-reads the
// schedule list from store on every iteration to pick up adds/deletes.
func (s *RecurrenceSupervisor) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		now := s.clock.Now()
		if err := s.tick(ctx, now); err != nil {
			log.Printf("[recurrence] tick error: %v", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-s.clock.After(s.pollInterval):
		}
	}
}

// tick runs a single supervisor iteration: refresh the schedule cache,
// fire any schedules whose nextTick has elapsed (subject to backpressure
// and crash-resume dedup).
func (s *RecurrenceSupervisor) tick(ctx context.Context, now time.Time) error {
	if err := s.refreshSchedules(now); err != nil {
		return err
	}
	s.mu.Lock()
	due := make([]*scheduleState, 0, len(s.schedules))
	for _, st := range s.schedules {
		if !st.nextTick.After(now) {
			due = append(due, st)
		}
	}
	s.mu.Unlock()

	var firstErr error
	for _, st := range due {
		if err := s.fireOne(ctx, st, st.nextTick); err != nil {
			log.Printf("[recurrence] fire %q: %v", st.template.Name, err)
			if firstErr == nil {
				firstErr = err
			}
		}
		s.mu.Lock()
		st.nextTick = st.sched.Next(now)
		s.mu.Unlock()
	}
	return firstErr
}

// refreshSchedules reconciles the in-memory schedule cache with the store's
// current schedule list. New schedules get a parsed cron.Schedule and an
// initial nextTick computed from now.
func (s *RecurrenceSupervisor) refreshSchedules(now time.Time) error {
	templates, err := s.store.ListSchedules()
	if err != nil {
		return fmt.Errorf("list schedules: %w", err)
	}
	current := map[string]struct{}{}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range templates {
		current[t.Name] = struct{}{}
		existing, ok := s.schedules[t.Name]
		if ok && existing.template.Cron == t.Cron {
			existing.template = t
			continue
		}
		sched, parseErr := ParseCron(t.Cron)
		if parseErr != nil {
			log.Printf("[recurrence] schedule %q has invalid cron %q: %v",
				t.Name, t.Cron, parseErr)
			delete(s.schedules, t.Name)
			continue
		}
		// Use Next(now-1ns) so a tick that lands exactly on a cron boundary
		// (common in tests with FakeClock and during deterministic schedule
		// installs) fires immediately rather than waiting another full period.
		s.schedules[t.Name] = &scheduleState{
			template: t,
			sched:    sched,
			nextTick: sched.Next(now.Add(-time.Nanosecond)),
		}
	}
	for name := range s.schedules {
		if _, kept := current[name]; !kept {
			delete(s.schedules, name)
		}
	}
	return nil
}

// fireOne implements amendment A4: dedup → backpressure → ledger → submit.
func (s *RecurrenceSupervisor) fireOne(ctx context.Context, st *scheduleState, tickAt time.Time) error {
	_ = ctx
	subID := submissionID(st.template.Name, tickAt)

	// Crash-resume dedup: if a prior schedule.fired event with this
	// submission_id is already in the ledger, the previous attempt at least
	// reached step (3) of A4. Queue.SubmitJob's idempotency key makes step
	// (4) safe to retry, but we still want to avoid double-recording the
	// ledger event itself.
	alreadyFired, err := s.alreadyFired(st.template.Name, subID)
	if err != nil {
		return fmt.Errorf("dedup check: %w", err)
	}

	depth, hasInFlight, err := s.queueState(st.template.Name)
	if err != nil {
		return fmt.Errorf("queue state: %w", err)
	}
	fire, reason := shouldFire(st.template, depth, hasInFlight)
	if !fire {
		if alreadyFired {
			// We previously chose to fire and recorded it; current backpressure
			// would block, but we still need to honour the ledger commitment by
			// retrying Queue.SubmitJob (idempotent on subID). The skipped event
			// already in the ledger would mean a different prior outcome, so
			// only suppress logging here.
			return s.submitJob(st.template, subID, tickAt)
		}
		if recErr := s.store.RecordScheduleSkipped(st.template.Name, reason, tickAt); recErr != nil {
			return fmt.Errorf("record skipped: %w", recErr)
		}
		return nil
	}

	// A4 step 3: AppendLedgerEvent("schedule.fired") FIRST.
	if !alreadyFired {
		if recErr := s.store.RecordScheduleFired(st.template.Name, subID, tickAt); recErr != nil {
			return fmt.Errorf("record fired: %w", recErr)
		}
	}
	// A4 step 4: THEN Queue.SubmitJob (idempotency key = submission_id).
	return s.submitJob(st.template, subID, tickAt)
}

// submitJob enqueues the materialized job. The IdempotencyKey is the
// deterministic submission_id so a crash between RecordScheduleFired and
// SubmitJob retries safely on the next tick.
func (s *RecurrenceSupervisor) submitJob(t RecurringJobTemplate, subID string, tickAt time.Time) error {
	payload, err := decodePayload(t.Payload)
	if err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	payload["schedule_name"] = t.Name
	payload["submission_id"] = subID
	payload["tick_at"] = tickAt.UTC().Format(time.RFC3339Nano)
	if t.Timeout > 0 {
		payload["timeout"] = t.Timeout.String()
	}
	// Auto-fill spec-required fields from schedule context so starter payloads
	// don't have to duplicate the dispatch target. The template's top-level
	// JobType is authoritative for routing; copy it into the payload only when
	// absent so explicit operator overrides still win.
	if v, ok := payload["job_type"]; !ok || strings.TrimSpace(fmt.Sprint(v)) == "" {
		payload["job_type"] = string(t.JobType)
	}
	// Dream specs additionally require a dream_run_id and mode. Synthesize
	// defaults from schedule context so each tick gets a stable, unique run id.
	if t.JobType == JobTypeDreamRun || t.JobType == JobTypeDreamStage {
		if v, ok := payload["dream_run_id"]; !ok || strings.TrimSpace(fmt.Sprint(v)) == "" {
			payload["dream_run_id"] = fmt.Sprintf("schedule-%s-%s", t.Name, subID)
		}
		if v, ok := payload["mode"]; !ok || strings.TrimSpace(fmt.Sprint(v)) == "" {
			payload["mode"] = string(DreamModeDaemon)
		}
	}
	_, err = s.queue.SubmitJob(SubmitJobInput{
		JobType:        t.JobType,
		IdempotencyKey: subID,
		Actor:          scheduleActor,
		Payload:        payload,
	}, QueueMutationOptions{})
	if err != nil {
		return fmt.Errorf("submit job: %w", err)
	}
	return nil
}

// alreadyFired returns true if a schedule.fired event for (name, submissionID)
// is already present in the ledger. Used to suppress double-recording on
// crash-resume.
func (s *RecurrenceSupervisor) alreadyFired(name, submissionID string) (bool, error) {
	replay, err := s.store.ReplayLedgerReadOnly()
	if err != nil {
		return false, err
	}
	for _, ev := range replay.Events {
		if ev.EventType != EventScheduleFired {
			continue
		}
		evName, _ := ev.Payload["name"].(string)
		evSub, _ := ev.Payload["submission_id"].(string)
		if evName == name && evSub == submissionID {
			return true, nil
		}
	}
	return false, nil
}

// queueState returns (queueDepth, hasInFlight) for the schedule. queueDepth is
// the count of non-terminal jobs that share schedule_name in their payload (or
// idempotency key prefix); hasInFlight is whether any of them is running.
func (s *RecurrenceSupervisor) queueState(scheduleName string) (int, bool, error) {
	if s.queue == nil {
		return 0, false, nil
	}
	snap, err := s.queue.Snapshot()
	if err != nil {
		if errors.Is(err, ErrJobNotFound) {
			return 0, false, nil
		}
		return 0, false, err
	}
	depth := 0
	hasInFlight := false
	for _, job := range snap.Jobs {
		if isTerminalStatus(job.Status) {
			continue
		}
		if !isRealQueueJob(job) {
			continue
		}
		if !jobBelongsToSchedule(job, scheduleName) {
			continue
		}
		depth++
		if job.Status == JobStatusRunning {
			hasInFlight = true
		}
	}
	return depth, hasInFlight, nil
}

// isRealQueueJob filters out phantom snapshot entries created by schedule.*
// events that share the JobID="schedule" sentinel. Real jobs always have a
// non-empty JobType (set by SubmitJob when materializing job.accepted).
func isRealQueueJob(job QueueJobState) bool {
	if job.JobID == "" || job.JobID == scheduleSentinelJobID {
		return false
	}
	return job.JobType != ""
}

func jobBelongsToSchedule(job QueueJobState, scheduleName string) bool {
	if job.Payload == nil {
		return false
	}
	if name, ok := job.Payload["schedule_name"].(string); ok && name == scheduleName {
		return true
	}
	return false
}

// shouldFire is a PURE FUNCTION (per amendment B / test pyramid): given a
// template and current queue state, decide whether to fire this tick.
// Returns (fire, skipReason). When fire=true, skipReason is "".
func shouldFire(t RecurringJobTemplate, queueDepth int, hasInFlight bool) (bool, string) {
	if t.Backpressure.SkipIfRunning && hasInFlight {
		return false, "skip_if_running:in-flight"
	}
	if t.Backpressure.MaxQueueDepth > 0 && queueDepth >= t.Backpressure.MaxQueueDepth {
		return false, fmt.Sprintf("max_queue_depth:%d", t.Backpressure.MaxQueueDepth)
	}
	return true, ""
}

// submissionID is deterministic per (schedule_name, tick_unix_seconds). This
// is the dedup key on Queue.SubmitJob so a crash between AppendLedgerEvent and
// SubmitJob retries idempotently (amendment A4).
func submissionID(scheduleName string, tickAt time.Time) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s:%d", scheduleName, tickAt.Unix())
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func decodePayload(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "" {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		return map[string]any{}, nil
	}
	return out, nil
}
