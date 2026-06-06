package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const c2EventsFileName = "events.jsonl"

// RPIC2Event is one normalized C2/runtime event stored in events.jsonl.
type RPIC2Event struct {
	SchemaVersion int             `json:"schema_version"`
	EventID       string          `json:"event_id"`
	RunID         string          `json:"run_id"`
	CommandID     string          `json:"command_id,omitempty"`
	Phase         int             `json:"phase,omitempty"`
	Backend       string          `json:"backend,omitempty"`
	Source        string          `json:"source,omitempty"`
	WorkerID      string          `json:"worker_id,omitempty"`
	Type          string          `json:"type"`
	Message       string          `json:"message,omitempty"`
	Details       json.RawMessage `json:"details,omitempty"`
	Timestamp     string          `json:"timestamp"`
}

// c2RunInfo is a minimal run descriptor for the mine/compile event scanner.
type c2RunInfo struct {
	RunID     string `json:"run_id"`
	StartedAt string `json:"started_at,omitempty"`
}

// scanRegistryRuns reads the .agents/rpi/runs/ directory and returns basic
// run metadata. This is a simplified reader that extracts only the fields
// needed by mineEvents (RunID and StartedAt).
func scanRegistryRuns(root string) []c2RunInfo {
	runsDir := filepath.Join(root, ".agents", "rpi", "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return nil
	}

	runs := make([]c2RunInfo, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		runID := entry.Name()
		statePath := filepath.Join(runsDir, runID, "phased-state.json")
		data, err := os.ReadFile(statePath)
		if err != nil {
			continue
		}
		var state struct {
			RunID     string `json:"run_id"`
			StartedAt string `json:"started_at"`
		}
		if json.Unmarshal(data, &state) != nil || state.RunID == "" {
			continue
		}
		runs = append(runs, c2RunInfo{
			RunID:     state.RunID,
			StartedAt: state.StartedAt,
		})
	}
	return runs
}

// loadRPIC2Events reads the events.jsonl for a given run.
func loadRPIC2Events(root, runID string) ([]RPIC2Event, error) {
	path := filepath.Join(root, ".agents", "rpi", "runs", runID, c2EventsFileName)
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("open events log: %w", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 128*1024), 2*1024*1024)
	out := make([]RPIC2Event, 0)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var ev RPIC2Event
		if err := json.Unmarshal(line, &ev); err != nil {
			return nil, fmt.Errorf("parse events.jsonl line %d: %w", lineNum, err)
		}
		out = append(out, ev)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan events.jsonl: %w", err)
	}
	return out, nil
}
