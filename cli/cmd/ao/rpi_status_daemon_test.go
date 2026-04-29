package main

import (
	"context"
	"testing"
	"time"

	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
)

func TestRPIStatusFromDaemonQueue(t *testing.T) {
	now := time.Now().UTC()
	runSpec := daemonpkg.NewRPIRunJobSpec("run-daemon", "daemon goal")
	runSpec.StartPhase = 1
	runPayload, err := runSpec.ToJobSpec("job-run")
	if err != nil {
		t.Fatal(err)
	}
	phaseSpec := daemonpkg.NewRPIPhaseJobSpec("run-failed", "failed goal", 2)
	phasePayload, err := phaseSpec.ToJobSpec("job-phase")
	if err != nil {
		t.Fatal(err)
	}
	status := daemonpkg.ReadOnlyStatusResponse{
		Queue: daemonpkg.QueueSnapshot{
			Jobs: []daemonpkg.QueueJobState{
				{
					JobID:     "job-run",
					JobType:   daemonpkg.JobTypeRPIRun,
					Status:    daemonpkg.JobStatusRunning,
					Payload:   runPayload.Payload,
					Artifacts: map[string]string{"active_phase": "1"},
					CreatedAt: now.Add(-time.Minute).Format(time.RFC3339Nano),
				},
				{
					JobID:   "job-phase",
					JobType: daemonpkg.JobTypeRPIPhase,
					Status:  daemonpkg.JobStatusFailed,
					Payload: phasePayload.Payload,
					Failure: &daemonpkg.JobFailure{
						Code:    daemonpkg.FailureSessionLost,
						Message: "session disappeared",
					},
					CreatedAt: now.Add(-2 * time.Minute).Format(time.RFC3339Nano),
				},
			},
		},
	}

	out := buildRPIStatusOutputFromDaemon(status)
	if out.Count != 2 || len(out.Active) != 1 || len(out.Historical) != 1 {
		t.Fatalf("daemon output = %#v", out)
	}
	if out.Active[0].RunID != "run-daemon" ||
		out.Active[0].PhaseName != "discovery" ||
		out.Active[0].Status != string(daemonpkg.JobStatusRunning) {
		t.Fatalf("active daemon run = %#v", out.Active[0])
	}
	if out.Historical[0].RunID != "run-failed" ||
		out.Historical[0].Status != string(daemonpkg.JobStatusFailed) ||
		out.Historical[0].Reason != "session disappeared" {
		t.Fatalf("historical daemon run = %#v", out.Historical[0])
	}
}

func TestRPIStatusDaemonFallback(t *testing.T) {
	tmpDir := t.TempDir()
	writeRegistryRun(t, tmpDir, registryRunSpec{
		runID:  "local-run",
		phase:  1,
		schema: 1,
		goal:   "local fallback",
		hbAge:  time.Minute,
	})

	out, err := buildRPIStatusOutputForMode(context.Background(), tmpDir, rpiStatusDaemonModeOptions{
		Enabled:  true,
		URL:      "://bad-daemon-url",
		Fallback: true,
	})
	if err != nil {
		t.Fatalf("fallback status: %v", err)
	}
	if out.Count != 1 || out.Runs[0].RunID != "local-run" {
		t.Fatalf("fallback output = %#v", out)
	}

	if _, err := buildRPIStatusOutputForMode(context.Background(), tmpDir, rpiStatusDaemonModeOptions{
		Enabled:  true,
		URL:      "://bad-daemon-url",
		Fallback: false,
	}); err == nil {
		t.Fatal("daemon status without fallback error = nil")
	}
}
