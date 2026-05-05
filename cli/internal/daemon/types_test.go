package daemon

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestEnumValidation(t *testing.T) {
	validators := []struct {
		name string
		err  error
	}{
		{"job type", ValidateJobType(JobTypeRPIRun)},
		{"factory admission job type", ValidateJobType(JobTypeFactoryAdmission)},
		{"factory local pilot job type", ValidateJobType(JobTypeFactoryLocalPilot)},
		{"event type", ValidateEventType(EventJobCompleted)},
		{"job status", ValidateJobStatus(JobStatusRetryWaiting)},
		{"job result", ValidateJobResultStatus(JobResultSucceeded)},
		{"failure code", ValidateFailureCode(FailureProviderUnreachable)},
		{"lease state", ValidateLeaseState(LeaseFresh)},
	}
	for _, tc := range validators {
		if tc.err != nil {
			t.Fatalf("%s validation failed: %v", tc.name, tc.err)
		}
	}

	invalid := []struct {
		name string
		err  error
	}{
		{"job type", ValidateJobType("bad.job")},
		{"event type", ValidateEventType("job.done")},
		{"job status", ValidateJobStatus("done")},
		{"job result", ValidateJobResultStatus("ok")},
		{"failure code", ValidateFailureCode("provider_down")},
		{"lease state", ValidateLeaseState("stale")},
	}
	for _, tc := range invalid {
		if tc.err == nil {
			t.Fatalf("%s validation accepted invalid value", tc.name)
		}
	}
}

func TestJobStatusTruthTable(t *testing.T) {
	tests := []struct {
		name  string
		input JobStatusProjectionInput
		want  JobStatus
	}{
		{"completed terminal wins", JobStatusProjectionInput{TerminalEvent: EventJobCompleted, Lease: LeaseFresh, ProjectionStale: true}, JobStatusCompleted},
		{"failed terminal wins", JobStatusProjectionInput{TerminalEvent: EventJobFailed, Lease: LeaseFresh}, JobStatusFailed},
		{"cancelled terminal wins", JobStatusProjectionInput{TerminalEvent: EventJobCancelled, Lease: LeaseFresh}, JobStatusCancelled},
		{"fresh lease running", JobStatusProjectionInput{Lease: LeaseFresh}, JobStatusRunning},
		{"expired lease retry waiting", JobStatusProjectionInput{Lease: LeaseExpired}, JobStatusRetryWaiting},
		{"accepted no lease queued", JobStatusProjectionInput{Lease: LeaseNone}, JobStatusQueued},
		{"stale projection degraded", JobStatusProjectionInput{Lease: LeaseFresh, ProjectionStale: true}, JobStatusDegraded},
		{"unknown lease degraded", JobStatusProjectionInput{Lease: LeaseUnknown}, JobStatusDegraded},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ProjectJobStatus(tc.input); got != tc.want {
				t.Fatalf("ProjectJobStatus(%#v) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestJobStatusTransitionMatrix(t *testing.T) {
	allowed := [][2]JobStatus{
		{JobStatusQueued, JobStatusRunning},
		{JobStatusQueued, JobStatusCancelled},
		{JobStatusRunning, JobStatusCompleted},
		{JobStatusRunning, JobStatusRetryWaiting},
		{JobStatusRetryWaiting, JobStatusQueued},
		{JobStatusDegraded, JobStatusRunning},
	}
	for _, pair := range allowed {
		if !CanTransitionJobStatus(pair[0], pair[1]) {
			t.Fatalf("transition %s -> %s should be allowed", pair[0], pair[1])
		}
	}

	rejected := [][2]JobStatus{
		{JobStatusCompleted, JobStatusRunning},
		{JobStatusFailed, JobStatusQueued},
		{JobStatusCancelled, JobStatusRunning},
		{JobStatusQueued, JobStatusCompleted},
	}
	for _, pair := range rejected {
		if CanTransitionJobStatus(pair[0], pair[1]) {
			t.Fatalf("transition %s -> %s should be rejected", pair[0], pair[1])
		}
	}
}

func TestProviderStatusTruthTable(t *testing.T) {
	tests := []struct {
		name  string
		input ProviderProjectionInput
		want  ProviderStatus
	}{
		{"daemon unavailable wins", ProviderProjectionInput{}, ProviderDaemonUnavailable},
		{"provider unreachable", ProviderProjectionInput{DaemonReady: true}, ProviderUnreachable},
		{"session pending", ProviderProjectionInput{DaemonReady: true, GasCityReady: true}, ProviderSessionPending},
		{"session bound", ProviderProjectionInput{DaemonReady: true, GasCityReady: true, WorkerSessionKnown: true}, ProviderSessionBound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ProjectProviderStatus(tc.input); got != tc.want {
				t.Fatalf("ProjectProviderStatus(%#v) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestSnapshotTruthTable(t *testing.T) {
	tests := []struct {
		name  string
		input SnapshotProjectionInput
		want  SnapshotConsumerResult
	}{
		{"serve", SnapshotProjectionInput{ReplayStatus: SnapshotReplayComplete, FileExists: true, VersionSupported: true}, SnapshotServe},
		{"compatibility error", SnapshotProjectionInput{ReplayStatus: SnapshotReplayComplete, FileExists: true}, SnapshotCompatibilityError},
		{"missing projection", SnapshotProjectionInput{ReplayStatus: SnapshotReplayComplete, VersionSupported: true}, SnapshotProjectionMissing},
		{"corrupt replay degraded", SnapshotProjectionInput{ReplayStatus: SnapshotReplayCorrupt, FileExists: true, VersionSupported: true}, SnapshotProjectionDegraded},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ProjectSnapshotResult(tc.input); got != tc.want {
				t.Fatalf("ProjectSnapshotResult(%#v) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestRecurringJobTemplate_RoundTripJSON(t *testing.T) {
	orig := RecurringJobTemplate{
		Name:    "nightly-wiki",
		Cron:    "0 3 * * *",
		JobType: JobTypeLLMWikiLoop,
		Payload: json.RawMessage(`{"vault":"~/wiki"}`),
		Timeout: 4 * time.Hour,
		Backpressure: RecurrenceBackpressure{
			SkipIfRunning: true,
			MaxQueueDepth: 5,
		},
	}
	blob, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got RecurringJobTemplate
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Name != orig.Name || got.Cron != orig.Cron || got.JobType != orig.JobType {
		t.Fatalf("mismatch: got=%+v want=%+v", got, orig)
	}
	if got.Timeout != orig.Timeout {
		t.Fatalf("timeout mismatch: got=%v want=%v", got.Timeout, orig.Timeout)
	}
	if got.Backpressure != orig.Backpressure {
		t.Fatalf("backpressure mismatch: got=%+v want=%+v", got.Backpressure, orig.Backpressure)
	}
}

func TestParseCron_Valid5Field(t *testing.T) {
	cases := []string{"0 3 * * *", "*/5 * * * *", "@daily", "@hourly", "0 0 1 * *"}
	for _, c := range cases {
		if _, err := ParseCron(c); err != nil {
			t.Errorf("expected %q to parse; got error: %v", c, err)
		}
	}
}

func TestParseCron_Rejects6FieldWithSeconds(t *testing.T) {
	// 6-field with seconds is the DoS vector per amendment B4 — must be rejected.
	_, err := ParseCron("* * * * * *")
	if err == nil {
		t.Fatal("expected 6-field cron to be rejected; got nil")
	}
	var cpe *CronParseError
	if !errors.As(err, &cpe) {
		t.Fatalf("expected CronParseError; got %T: %v", err, err)
	}
	if cpe.Original != "* * * * * *" {
		t.Errorf("expected original to be preserved; got %q", cpe.Original)
	}
}

func TestParseCron_RejectsInvalid(t *testing.T) {
	_, err := ParseCron("not a cron")
	if err == nil {
		t.Fatal("expected invalid cron to error")
	}
	var cpe *CronParseError
	if !errors.As(err, &cpe) {
		t.Fatalf("expected CronParseError; got %T", err)
	}
	if cpe.Original != "not a cron" {
		t.Errorf("expected original preserved; got %q", cpe.Original)
	}
}

func TestRecurrenceBackpressure_DefaultsAreSane(t *testing.T) {
	// Zero-value: SkipIfRunning=false, MaxQueueDepth=0 means "no backpressure"
	var bp RecurrenceBackpressure
	if bp.SkipIfRunning {
		t.Error("expected SkipIfRunning=false on zero value")
	}
	if bp.MaxQueueDepth != 0 {
		t.Errorf("expected MaxQueueDepth=0 on zero value; got %d", bp.MaxQueueDepth)
	}
}
