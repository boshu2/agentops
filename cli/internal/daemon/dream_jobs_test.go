package daemon

import (
	"encoding/json"
	"testing"
)

func TestDreamStageManifestJSONFixtureValidates(t *testing.T) {
	fixture := []byte(`{
	  "schema_version": 1,
	  "dream_run_id": "dream-20260428",
	  "mode": "daemon",
	  "output_dir": ".agents/overnight/dream-20260428",
	  "stages": [
	    {"stage": "ingest", "job_id": "job-ingest", "required": true},
	    {"stage": "reduce", "job_id": "job-reduce", "required": true},
	    {"stage": "measure", "job_id": "job-measure", "required": true},
	    {"stage": "commit", "job_id": "job-commit", "required": true},
	    {"stage": "report", "job_id": "job-report", "required": true}
	  ],
	  "metadata": {"source": "fixture"}
	}`)
	var manifest DreamStageManifest
	if err := json.Unmarshal(fixture, &manifest); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	if err := manifest.Validate(); err != nil {
		t.Fatalf("manifest validate: %v", err)
	}
	if manifest.Stages[0].Stage != DreamStageIngest || manifest.Stages[len(manifest.Stages)-1].Stage != DreamStageReport {
		t.Fatalf("manifest stages = %#v, want ingest -> report", manifest.Stages)
	}
}

func TestDreamJobSpecsValidateAndRoundTrip(t *testing.T) {
	run := NewDreamRunJobSpec("dream-20260428", ".agents/overnight/dream-20260428")
	run.Goal = "compound knowledge"
	run.MaxIterations = 3
	if err := run.Validate(); err != nil {
		t.Fatalf("run spec validate: %v", err)
	}
	runJob, err := run.ToJobSpec("job-dream-run")
	if err != nil {
		t.Fatalf("run ToJobSpec: %v", err)
	}
	if runJob.Type != JobTypeDreamRun || runJob.Payload["job_type"] != string(JobTypeDreamRun) {
		t.Fatalf("run job spec = %#v, want dream.run payload", runJob)
	}
	if err := ValidateDreamJobSpec(runJob); err != nil {
		t.Fatalf("validate run job: %v", err)
	}

	stage := NewDreamStageJobSpec("dream-20260428", ".agents/overnight/dream-20260428", DreamStageReduce)
	stage.IterationID = "dream-20260428-iter-1"
	stage.Iteration = 1
	stage.CheckpointDir = ".agents/overnight/dream-20260428/checkpoints/iter-1"
	stageJob, err := stage.ToJobSpec("job-dream-reduce")
	if err != nil {
		t.Fatalf("stage ToJobSpec: %v", err)
	}
	if stageJob.Type != JobTypeDreamStage || stageJob.Payload["stage"] != string(DreamStageReduce) {
		t.Fatalf("stage job spec = %#v, want dream.stage reduce payload", stageJob)
	}
	parsed, err := DreamStageJobSpecFromPayload(stageJob.Payload)
	if err != nil {
		t.Fatalf("parse stage payload: %v", err)
	}
	if parsed.Stage != DreamStageReduce || parsed.Iteration != 1 {
		t.Fatalf("parsed stage = %#v, want reduce iteration 1", parsed)
	}
}

func TestDreamStageManifestRejectsInvalidStageModeAndOrder(t *testing.T) {
	manifest := DefaultDreamStageManifest("dream-20260428", ".agents/overnight/dream-20260428")
	manifest.Mode = "autonomous"
	if err := manifest.Validate(); err == nil {
		t.Fatal("manifest accepted invalid mode")
	}

	manifest = DefaultDreamStageManifest("dream-20260428", ".agents/overnight/dream-20260428")
	manifest.Stages[1].Stage = "summarize"
	if err := manifest.Validate(); err == nil {
		t.Fatal("manifest accepted invalid stage")
	}

	manifest = DreamStageManifest{
		SchemaVersion: DreamJobSpecSchemaVersion,
		DreamRunID:    "dream-20260428",
		Mode:          DreamModeDaemon,
		OutputDir:     ".agents/overnight/dream-20260428",
		Stages: []DreamStageEntry{
			{Stage: DreamStageMeasure, Required: true},
			{Stage: DreamStageReduce, Required: true},
		},
	}
	if err := manifest.Validate(); err == nil {
		t.Fatal("manifest accepted out-of-order stages")
	}
}
