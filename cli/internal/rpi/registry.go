package rpi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const RunRegistrySchemaVersion = 1

type RunRegistryState struct {
	SchemaVersion   int               `json:"schema_version"`
	Goal            string            `json:"goal"`
	EpicID          string            `json:"epic_id,omitempty"`
	TrackerMode     string            `json:"tracker_mode,omitempty"`
	TrackerReason   string            `json:"tracker_reason,omitempty"`
	Phase           int               `json:"phase"`
	StartPhase      int               `json:"start_phase"`
	Cycle           int               `json:"cycle"`
	FastPath        bool              `json:"fast_path,omitempty"`
	TestFirst       bool              `json:"test_first"`
	Complexity      string            `json:"complexity,omitempty"`
	Verdicts        map[string]string `json:"verdicts"`
	Attempts        map[string]int    `json:"attempts"`
	StartedAt       string            `json:"started_at"`
	RunID           string            `json:"run_id"`
	Backend         string            `json:"backend,omitempty"`
	TerminalStatus  string            `json:"terminal_status,omitempty"`
	TerminalReason  string            `json:"terminal_reason,omitempty"`
	TerminatedAt    string            `json:"terminated_at,omitempty"`
	DaemonJobID     string            `json:"daemon_job_id,omitempty"`
	DaemonRequestID string            `json:"daemon_request_id,omitempty"`
	LastEventID     string            `json:"last_event_id,omitempty"`
}

type RunRegistryWriter interface {
	WriteRunRegistryState(root string, state RunRegistryState) error
}

type FileRunRegistryWriter struct{}

func (FileRunRegistryWriter) WriteRunRegistryState(root string, state RunRegistryState) error {
	if err := state.Validate(); err != nil {
		return err
	}
	runDir := RPIRunRegistryDir(root, state.RunID)
	if runDir == "" {
		return fmt.Errorf("run_id is required")
	}
	if err := os.MkdirAll(runDir, 0750); err != nil {
		return fmt.Errorf("create run registry dir: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal run registry state: %w", err)
	}
	data = append(data, '\n')
	if err := WritePhasedStateAtomic(filepath.Join(runDir, PhasedStateFile), data); err != nil {
		return fmt.Errorf("write run registry state: %w", err)
	}
	return nil
}

func (state RunRegistryState) Validate() error {
	if state.SchemaVersion != RunRegistrySchemaVersion {
		return fmt.Errorf("schema_version mismatch: got %d want %d", state.SchemaVersion, RunRegistrySchemaVersion)
	}
	if strings.TrimSpace(state.RunID) == "" {
		return fmt.Errorf("run_id is required")
	}
	if strings.TrimSpace(state.Goal) == "" {
		return fmt.Errorf("goal is required")
	}
	if state.Phase < 1 || state.Phase > 3 {
		return fmt.Errorf("phase must be between 1 and 3")
	}
	if state.StartPhase < 1 || state.StartPhase > 3 {
		return fmt.Errorf("start_phase must be between 1 and 3")
	}
	if state.Cycle <= 0 {
		return fmt.Errorf("cycle must be positive")
	}
	if state.Verdicts == nil {
		return fmt.Errorf("verdicts map is required")
	}
	if state.Attempts == nil {
		return fmt.Errorf("attempts map is required")
	}
	return nil
}
