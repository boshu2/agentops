// practices: [agile-manifesto, dora-metrics]
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

// RPIC2Event is one normalized C2/runtime event stored in a run's events.jsonl.
//
// The type and its reader (loadRPIC2Events) live outside the rpi command
// surface so mine.go can read historical run-event logs after the rpi engine
// is removed. The writer (appendRPIC2Event) stays with the rpi engine and is
// deleted with it. (ag-362v / ag-llni)
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

// loadRPIC2Events reads the normalized C2 event log for a run from
// <root>/.agents/rpi/runs/<runID>/events.jsonl. A missing file returns
// (nil, nil). The path is computed inline so this reader carries no dependency
// on the rpi command surface (mirrors internal/rpi.RPIRunRegistryDir).
func loadRPIC2Events(root, runID string) ([]RPIC2Event, error) {
	if root == "" || runID == "" {
		return nil, nil
	}
	path := filepath.Join(root, ".agents", "rpi", "runs", runID, "events.jsonl")
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
