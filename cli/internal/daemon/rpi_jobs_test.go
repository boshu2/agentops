package daemon

import "testing"

func TestRPIJobSpecValidatesRunAndPhase(t *testing.T) {
	run := NewRPIRunJobSpec("run-123", "ship daemon")
	run.EpicID = "ag-hpb"
	run.ExecutionPacketPath = ".agents/rpi/execution-packet.json"
	run.PhaseTimeout = "5m0s"
	if err := run.Validate(); err != nil {
		t.Fatalf("run spec validate: %v", err)
	}

	phase := NewRPIPhaseJobSpec("run-123", "ship daemon", 2)
	phase.ParentRunJobID = "job-run"
	phase.GasCityCityName = "agentops"
	phase.GasCitySessionAlias = "rpi-run-123-p2"
	phase.PhaseTimeout = "5m0s"
	if err := phase.Validate(); err != nil {
		t.Fatalf("phase spec validate: %v", err)
	}
}

func TestRPIJobSpecRejectsSchemaEnumAndPhase(t *testing.T) {
	run := NewRPIRunJobSpec("run-123", "ship daemon")
	run.SchemaVersion = 99
	if err := run.Validate(); err == nil {
		t.Fatal("run spec accepted wrong schema version")
	}

	run = NewRPIRunJobSpec("run-123", "ship daemon")
	run.Backend = "ollama"
	if err := run.Validate(); err == nil {
		t.Fatal("run spec accepted invalid backend")
	}

	run = NewRPIRunJobSpec("run-123", "ship daemon")
	run.PhaseTimeout = "0s"
	if err := run.Validate(); err == nil {
		t.Fatal("run spec accepted non-positive phase timeout")
	}

	phase := NewRPIPhaseJobSpec("run-123", "ship daemon", 4)
	if err := phase.Validate(); err == nil {
		t.Fatal("phase spec accepted invalid phase number")
	}

	phase = NewRPIPhaseJobSpec("run-123", "ship daemon", 2)
	phase.PhaseName = "build"
	if err := phase.Validate(); err == nil {
		t.Fatal("phase spec accepted mismatched phase name")
	}
}

func TestRPIJobSpecRoundTripThroughDaemonJobSpec(t *testing.T) {
	run := NewRPIRunJobSpec("run-123", "ship daemon")
	run.PhaseTimeout = "7m0s"
	job, err := run.ToJobSpec("job-run")
	if err != nil {
		t.Fatalf("run ToJobSpec: %v", err)
	}
	if job.Type != JobTypeRPIRun || job.Payload["job_type"] != string(JobTypeRPIRun) {
		t.Fatalf("run job spec = %#v, want rpi.run payload", job)
	}
	if err := ValidateRPIJobSpec(job); err != nil {
		t.Fatalf("validate run job spec: %v", err)
	}
	parsedRun, err := RPIRunJobSpecFromPayload(job.Payload)
	if err != nil {
		t.Fatalf("parse run payload: %v", err)
	}
	if parsedRun.RunID != run.RunID || parsedRun.StartPhase != 1 || parsedRun.MaxPhase != 3 || parsedRun.PhaseTimeout != "7m0s" {
		t.Fatalf("parsed run = %#v, want original run bounds", parsedRun)
	}

	phase := NewRPIPhaseJobSpec("run-123", "ship daemon", 3)
	phaseJob, err := phase.ToJobSpec("job-phase")
	if err != nil {
		t.Fatalf("phase ToJobSpec: %v", err)
	}
	if phaseJob.Type != JobTypeRPIPhase || phaseJob.Payload["phase_name"] != "validation" {
		t.Fatalf("phase job spec = %#v, want rpi.phase validation payload", phaseJob)
	}
	if err := ValidateRPIJobSpec(phaseJob); err != nil {
		t.Fatalf("validate phase job spec: %v", err)
	}
	parsedPhase, err := RPIPhaseJobSpecFromPayload(phaseJob.Payload)
	if err != nil {
		t.Fatalf("parse phase payload: %v", err)
	}
	if parsedPhase.Phase != 3 || parsedPhase.PhaseName != "validation" {
		t.Fatalf("parsed phase = %#v, want validation phase", parsedPhase)
	}
}
