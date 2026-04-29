package daemon

import "testing"

func TestEnumValidation(t *testing.T) {
	validators := []struct {
		name string
		err  error
	}{
		{"job type", ValidateJobType(JobTypeRPIRun)},
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
