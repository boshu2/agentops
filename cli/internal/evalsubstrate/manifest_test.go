package evalsubstrate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewRunWriter_CreatesPendingManifest(t *testing.T) {
	root := t.TempDir()
	w, err := NewRunWriter(root, "run-1", Manifest{TaskRef: "finance-categorize-txn"})
	if err != nil {
		t.Fatal(err)
	}
	got := w.Manifest()
	if got.Status != StatusPending {
		t.Fatalf("status = %q, want pending", got.Status)
	}
	if got.StartedAtUnixMs == 0 {
		t.Fatal("started_at_unix_ms not stamped")
	}
	if _, err := os.Stat(w.Path()); err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
}

func TestNewRunWriter_RefusesDuplicateRunDir(t *testing.T) {
	root := t.TempDir()
	if _, err := NewRunWriter(root, "run-x", Manifest{}); err != nil {
		t.Fatal(err)
	}
	_, err := NewRunWriter(root, "run-x", Manifest{})
	if err == nil {
		t.Fatal("expected duplicate-run-dir refusal")
	}
}

func TestRunWriter_PendingToRunningToComplete(t *testing.T) {
	root := t.TempDir()
	w, err := NewRunWriter(root, "run-2", Manifest{TaskRef: "t"})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Transition(StatusRunning, nil); err != nil {
		t.Fatalf("pending->running: %v", err)
	}
	if err := w.Transition(StatusComplete, func(m *Manifest) {
		m.Verdict = &Verdict{Kind: VerdictImproved}
	}); err != nil {
		t.Fatalf("running->complete: %v", err)
	}
	loaded, err := LoadManifest(w.Path())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status != StatusComplete {
		t.Fatalf("status = %q", loaded.Status)
	}
	if loaded.FinishedAtUnixMs == 0 {
		t.Fatal("finished_at_unix_ms not stamped")
	}
	if loaded.Verdict == nil || loaded.Verdict.Kind != VerdictImproved {
		t.Fatalf("verdict = %v", loaded.Verdict)
	}
}

func TestRunWriter_IllegalTransitionRejected(t *testing.T) {
	root := t.TempDir()
	w, err := NewRunWriter(root, "run-3", Manifest{})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Transition(StatusComplete, nil); err == nil {
		t.Fatal("expected illegal transition error")
	}
}

func TestRunWriter_RetractedIsTerminal(t *testing.T) {
	root := t.TempDir()
	w, err := NewRunWriter(root, "run-4", Manifest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []RunStatus{StatusRunning, StatusComplete, StatusRetracted} {
		if err := w.Transition(s, nil); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	for _, s := range []RunStatus{StatusComplete, StatusRunning, StatusFailed} {
		if err := w.Transition(s, nil); err == nil {
			t.Fatalf("retracted should be terminal, but transition to %s allowed", s)
		}
	}
	if w.Manifest().RetractedAtUnixMs == nil {
		t.Fatal("retracted_at_unix_ms not stamped")
	}
}

func TestValidateForComplete_FlagsMissingFields(t *testing.T) {
	m := &Manifest{Status: StatusComplete, SchemaVersion: 1, ID: "run-0"}
	missing := ValidateForComplete(m)
	if len(missing) == 0 {
		t.Fatal("expected missing fields on near-empty manifest")
	}
	wantAny := []string{"task_ref", "harness_ref", "harness_content_hash",
		"model_spec_ref", "model_spec_hash", "ground_truth_ref", "ground_truth_hash",
		"sample_split", "n_samples", "seeds(>=3)", "inspect_command", "inspect_version", "rig_id"}
	missingSet := strings.Join(missing, ",")
	for _, w := range wantAny {
		if !strings.Contains(missingSet, w) {
			t.Errorf("expected %q in missing list, got %v", w, missing)
		}
	}
}

func TestValidateForComplete_FullyPopulatedPasses(t *testing.T) {
	m := &Manifest{
		SchemaVersion:       1,
		ID:                  "run-0",
		Status:              StatusComplete,
		StartedAtUnixMs:     1,
		TaskRef:             "t",
		HarnessRef:          "h",
		HarnessContentHash:  "sha256:aa",
		ModelSpecRef:        "ms",
		ModelSpecHash:       "sha256:bb",
		GroundTruthRef:      "gt",
		GroundTruthHash:     "sha256:cc",
		SampleSplit:         "dev",
		NSamples:            50,
		Seeds:               []int{1, 2, 3},
		InspectCommand:      "inspect ...",
		InspectVersion:      "0.3.216",
		RigID:               "bo-mac",
		ValidityGatesPassed: []string{},
	}
	if missing := ValidateForComplete(m); len(missing) > 0 {
		t.Fatalf("unexpected missing: %v", missing)
	}
}

func TestRunWriter_PersistsValidJSON(t *testing.T) {
	root := t.TempDir()
	w, err := NewRunWriter(root, "run-json", Manifest{TaskRef: "t"})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(w.Path())
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("manifest is not valid JSON: %v", err)
	}
	if got["task_ref"] != "t" {
		t.Fatalf("task_ref not persisted: %v", got["task_ref"])
	}
	if got["status"] != "pending" {
		t.Fatalf("status not persisted: %v", got["status"])
	}
}

func TestGenerateRunID_HasExpectedShape(t *testing.T) {
	id := GenerateRunID("bo-mac-m5")
	if !strings.HasPrefix(id, "run-") {
		t.Fatalf("missing run- prefix: %s", id)
	}
	parts := strings.Split(id, "-")
	if len(parts) < 5 {
		t.Fatalf("unexpected shape: %s", id)
	}
}

func TestRunWriter_TmpFileSweepableAfterCrashSimulation(t *testing.T) {
	root := t.TempDir()
	runDir := filepath.Join(root, "runs", "run-crashed")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tmpPath := filepath.Join(runDir, "manifest.json.tmp")
	if err := os.WriteFile(tmpPath, []byte(`{"status":"pending"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-10 * time.Minute)
	_ = os.Chtimes(tmpPath, old, old)

	removed, err := SweepTempFiles(root, 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 {
		t.Fatalf("expected 1 file removed, got %v", removed)
	}
}
