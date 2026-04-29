package rpi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryWriterWritesRunRegistryProjection(t *testing.T) {
	root := t.TempDir()
	state := RunRegistryState{
		SchemaVersion:   RunRegistrySchemaVersion,
		Goal:            "ship daemon",
		EpicID:          "ag-hpb",
		Phase:           2,
		StartPhase:      1,
		Cycle:           1,
		TestFirst:       true,
		Verdicts:        map[string]string{},
		Attempts:        map[string]int{},
		StartedAt:       "2026-04-28T12:00:00Z",
		RunID:           "run-123",
		Backend:         "gascity-api",
		DaemonJobID:     "job-rpi",
		DaemonRequestID: "req-rpi",
	}
	var writer RunRegistryWriter = FileRunRegistryWriter{}
	if err := writer.WriteRunRegistryState(root, state); err != nil {
		t.Fatalf("write registry state: %v", err)
	}
	path := filepath.Join(RPIRunRegistryDir(root, "run-123"), PhasedStateFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read registry state: %v", err)
	}
	var got RunRegistryState
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal registry state: %v", err)
	}
	if got.RunID != state.RunID || got.Goal != state.Goal || got.DaemonJobID != state.DaemonJobID {
		t.Fatalf("registry state = %#v, want run/goal/daemon job preserved", got)
	}
}

func TestRegistryWriterRejectsInvalidProjection(t *testing.T) {
	var writer RunRegistryWriter = FileRunRegistryWriter{}
	err := writer.WriteRunRegistryState(t.TempDir(), RunRegistryState{SchemaVersion: RunRegistrySchemaVersion})
	if err == nil {
		t.Fatal("invalid registry state write succeeded")
	}
}
