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
	"strings"
	"time"
)

const (
	rpiC2CommandSchemaVersion = 1
	rpiC2CommandsFileName     = "commands.jsonl"
)

// RPIC2Command is one control-plane command entry stored in commands.jsonl.
type RPIC2Command struct {
	SchemaVersion int             `json:"schema_version"`
	CommandID     string          `json:"command_id"`
	RunID         string          `json:"run_id"`
	Phase         int             `json:"phase,omitempty"`
	Kind          string          `json:"kind"`
	Targets       []string        `json:"targets"`
	Message       string          `json:"message,omitempty"`
	Deadline      string          `json:"deadline,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
	Timestamp     string          `json:"timestamp"`
}

type rpiC2CommandInput struct {
	RunID     string
	CommandID string
	Phase     int
	Kind      string
	Targets   []string
	Message   string
	Deadline  time.Time
	Metadata  any
	Timestamp time.Time
}

func rpiC2CommandsPath(root, runID string) string {
	runDir := rpiRunRegistryDir(root, runID)
	if runDir == "" {
		return ""
	}
	return filepath.Join(runDir, rpiC2CommandsFileName)
}

func appendRPIC2Command(root string, input rpiC2CommandInput) (RPIC2Command, error) {
	runID := strings.TrimSpace(input.RunID)
	if runID == "" {
		return RPIC2Command{}, fmt.Errorf("run_id is required")
	}
	kind := strings.TrimSpace(input.Kind)
	if kind == "" {
		return RPIC2Command{}, fmt.Errorf("kind is required")
	}
	targets := normalizeCommandTargets(input.Targets)
	if len(targets) == 0 {
		return RPIC2Command{}, fmt.Errorf("targets are required")
	}

	path := rpiC2CommandsPath(root, runID)
	if path == "" {
		return RPIC2Command{}, fmt.Errorf("run path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return RPIC2Command{}, fmt.Errorf("create run directory: %w", err)
	}

	metadata, err := marshalRPIC2Details(input.Metadata)
	if err != nil {
		return RPIC2Command{}, err
	}
	ts := input.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	commandID := strings.TrimSpace(input.CommandID)
	if commandID == "" {
		commandID = "cmd-" + strings.TrimPrefix(newRPIC2EventID(), "evt-")
	}

	record := RPIC2Command{
		SchemaVersion: rpiC2CommandSchemaVersion,
		CommandID:     commandID,
		RunID:         runID,
		Phase:         input.Phase,
		Kind:          kind,
		Targets:       targets,
		Message:       strings.TrimSpace(input.Message),
		Metadata:      metadata,
		Timestamp:     ts.Format(time.RFC3339Nano),
	}
	if !input.Deadline.IsZero() {
		record.Deadline = input.Deadline.UTC().Format(time.RFC3339Nano)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return RPIC2Command{}, fmt.Errorf("open command log: %w", err)
	}
	if err := json.NewEncoder(file).Encode(record); err != nil {
		_ = file.Close()
		return RPIC2Command{}, fmt.Errorf("append command: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return RPIC2Command{}, fmt.Errorf("sync command log: %w", err)
	}
	if err := file.Close(); err != nil {
		return RPIC2Command{}, fmt.Errorf("close command log: %w", err)
	}

	return record, nil
}

func loadRPIC2Commands(root, runID string) ([]RPIC2Command, error) {
	path := rpiC2CommandsPath(root, runID)
	if path == "" {
		return nil, nil
	}
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("open command log: %w", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 128*1024), 2*1024*1024)
	out := make([]RPIC2Command, 0)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var record RPIC2Command
		if err := json.Unmarshal(line, &record); err != nil {
			return nil, fmt.Errorf("parse commands.jsonl line %d: %w", lineNum, err)
		}
		out = append(out, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan commands.jsonl: %w", err)
	}
	return out, nil
}

func normalizeCommandTargets(targets []string) []string {
	seen := make(map[string]struct{}, len(targets))
	out := make([]string, 0, len(targets))
	for _, raw := range targets {
		t := strings.TrimSpace(raw)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}
