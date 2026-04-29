package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
)

func TestRPIRegistryProjectionRebuildsFromLedger(t *testing.T) {
	runSpec := NewRPIRunJobSpec("run-123", "ship daemon")
	runSpec.EpicID = "ag-hpb"
	runJob, err := runSpec.ToJobSpec("job-rpi-run")
	if err != nil {
		t.Fatalf("run ToJobSpec: %v", err)
	}
	phaseSpec := NewRPIPhaseJobSpec("run-123", "ship daemon", 2)
	phaseSpec.EpicID = "ag-hpb"
	phaseSpec.Attempt = 1
	phaseJob, err := phaseSpec.ToJobSpec("job-rpi-phase")
	if err != nil {
		t.Fatalf("phase ToJobSpec: %v", err)
	}
	events := []LedgerEvent{
		mustAcceptedRPIRegistryEvent(t, "evt-run-accepted", "req-run", runJob),
		mustAcceptedRPIRegistryEvent(t, "evt-phase-accepted", "req-phase", phaseJob),
		mustNewProjectionTestEvent(t, "evt-phase-claimed", "req-phase-claim", "job-rpi-phase", EventJobClaimed, "", 1, nil),
		mustNewProjectionTestEvent(t, "evt-phase-completed", "req-phase-complete", "job-rpi-phase", EventJobCompleted, "", 2, nil),
	}

	projection, err := RebuildRPIRegistryProjection(events)
	if err != nil {
		t.Fatalf("rebuild RPI registry projection: %v", err)
	}
	if err := ValidateRPIRegistryProjection(projection); err != nil {
		t.Fatalf("validate RPI registry projection: %v", err)
	}
	if len(projection.States) != 1 {
		t.Fatalf("projection states = %d, want 1", len(projection.States))
	}
	state := projection.States[0]
	if state.RunID != "run-123" || state.Goal != "ship daemon" || state.Phase != 2 {
		t.Fatalf("state = %#v, want run-123 phase 2", state)
	}
	if state.TerminalStatus != "completed" {
		t.Fatalf("terminal status = %q, want completed", state.TerminalStatus)
	}
	if state.DaemonJobID != "job-rpi-phase" || state.DaemonRequestID != "req-phase-complete" {
		t.Fatalf("daemon refs = job %q req %q, want latest phase refs", state.DaemonJobID, state.DaemonRequestID)
	}
}

func TestRPIRegistryProjectionWritesPhasedState(t *testing.T) {
	root := t.TempDir()
	runSpec := NewRPIRunJobSpec("run-123", "ship daemon")
	runJob, err := runSpec.ToJobSpec("job-rpi-run")
	if err != nil {
		t.Fatalf("run ToJobSpec: %v", err)
	}
	projection, err := RebuildRPIRegistryProjection([]LedgerEvent{
		mustAcceptedRPIRegistryEvent(t, "evt-run-accepted", "req-run", runJob),
	})
	if err != nil {
		t.Fatalf("rebuild RPI registry projection: %v", err)
	}
	if err := WriteRPIRegistryProjection(root, projection, nil); err != nil {
		t.Fatalf("write RPI registry projection: %v", err)
	}
	path := filepath.Join(cliRPI.RPIRunRegistryDir(root, "run-123"), cliRPI.PhasedStateFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read projected phased state: %v", err)
	}
	var state cliRPI.RunRegistryState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("unmarshal projected phased state: %v", err)
	}
	if state.RunID != "run-123" || state.DaemonJobID != "job-rpi-run" {
		t.Fatalf("projected state = %#v, want daemon-backed run state", state)
	}
}

func mustAcceptedRPIRegistryEvent(t *testing.T, eventID, requestID string, job JobSpec) LedgerEvent {
	t.Helper()
	event, err := NewLedgerEvent(LedgerEventInput{
		EventID:    eventID,
		RequestID:  RequestID(requestID),
		JobID:      job.ID,
		EventType:  EventJobAccepted,
		OccurredAt: projectionTestTime(t, 0),
		Actor:      "test",
		JobType:    job.Type,
		Payload: map[string]any{
			"job_payload": job.Payload,
		},
	})
	if err != nil {
		t.Fatalf("accepted event %s: %v", eventID, err)
	}
	return event
}
