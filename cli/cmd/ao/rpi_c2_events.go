// practices: [agile-manifesto, dora-metrics]
package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	rpiC2EventSchemaVersion = 1
	rpiC2EventsFileName     = "events.jsonl"
)

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

type rpiC2EventInput struct {
	RunID     string
	CommandID string
	Phase     int
	Backend   string
	Source    string
	WorkerID  string
	Type      string
	Message   string
	Details   any
	Timestamp time.Time
}

func rpiC2EventsPath(root, runID string) string {
	runDir := rpiRunRegistryDir(root, runID)
	if runDir == "" {
		return ""
	}
	return filepath.Join(runDir, rpiC2EventsFileName)
}

func appendRPIC2Event(root string, input rpiC2EventInput) (RPIC2Event, error) {
	runID := strings.TrimSpace(input.RunID)
	if runID == "" {
		return RPIC2Event{}, fmt.Errorf("run_id is required")
	}
	typ := strings.TrimSpace(input.Type)
	if typ == "" {
		return RPIC2Event{}, fmt.Errorf("type is required")
	}

	details, err := marshalRPIC2Details(input.Details)
	if err != nil {
		return RPIC2Event{}, err
	}

	ts := input.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	ev := RPIC2Event{
		SchemaVersion: rpiC2EventSchemaVersion,
		EventID:       newRPIC2EventID(),
		RunID:         runID,
		CommandID:     strings.TrimSpace(input.CommandID),
		Phase:         input.Phase,
		Backend:       strings.TrimSpace(input.Backend),
		Source:        strings.TrimSpace(input.Source),
		WorkerID:      strings.TrimSpace(input.WorkerID),
		Type:          typ,
		Message:       strings.TrimSpace(input.Message),
		Details:       details,
		Timestamp:     ts.Format(time.RFC3339Nano),
	}

	if err := appendRPIC2EventRecord(root, ev); err != nil {
		return RPIC2Event{}, err
	}

	for _, mirrorRoot := range mirrorRootsForEvent(root, runID) {
		if filepath.Clean(mirrorRoot) == filepath.Clean(root) {
			continue
		}
		if err := appendRPIC2EventRecord(mirrorRoot, ev); err != nil {
			VerbosePrintf("Warning: mirror event write skipped for %s: %v\n", mirrorRoot, err)
		}
	}
	return ev, nil
}

func appendRPIC2EventRecord(root string, ev RPIC2Event) error {
	path := rpiC2EventsPath(root, ev.RunID)
	if path == "" {
		return fmt.Errorf("run path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create run directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open events log: %w", err)
	}
	if err := json.NewEncoder(file).Encode(ev); err != nil {
		_ = file.Close()
		return fmt.Errorf("append event: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync event log: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close event log: %w", err)
	}
	return nil
}

func mirrorRootsForEvent(root, runID string) []string {
	roots := artifactRootsForRun(root, runID)
	if len(roots) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(roots))
	out := make([]string, 0, len(roots))
	for _, r := range roots {
		clean := filepath.Clean(strings.TrimSpace(r))
		if clean == "." || clean == "" {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	return out
}

func loadRPIC2Events(root, runID string) ([]RPIC2Event, error) {
	path := rpiC2EventsPath(root, runID)
	if path == "" {
		return nil, nil
	}
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

func appendRPIC2WorkerLogEvents(root, runID string, phaseNum int, backend, workerID, logPath string) error {
	file, err := os.Open(logPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("open worker log %s: %w", logPath, err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 128*1024), 2*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		input := rpiC2EventInput{
			RunID:    runID,
			Phase:    phaseNum,
			Backend:  backend,
			Source:   "tmux_worker_log",
			WorkerID: workerID,
			Type:     "worker.log",
			Message:  line,
			Details: map[string]any{
				"line": line,
			},
		}

		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err == nil {
			if typ, ok := payload["type"].(string); ok && strings.TrimSpace(typ) != "" {
				input.Type = "worker." + strings.TrimSpace(typ)
			}
			if msg, ok := payload["message"].(string); ok && strings.TrimSpace(msg) != "" {
				input.Message = msg
			}
			input.Details = payload
		}

		if _, err := appendRPIC2Event(root, input); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan worker log %s: %w", logPath, err)
	}
	return nil
}

func mapStreamEventToRPIC2(runID string, phaseNum int, ev StreamEvent) rpiC2EventInput {
	evType := strings.TrimSpace(ev.Type)
	if evType == "" {
		evType = "unknown"
	}

	details := make(map[string]any)
	if s := strings.TrimSpace(ev.Subtype); s != "" {
		details["subtype"] = s
	}
	if s := strings.TrimSpace(ev.SessionID); s != "" {
		details["session_id"] = s
	}
	if s := strings.TrimSpace(ev.ToolName); s != "" {
		details["tool_name"] = s
	}
	if ev.ToolInput != nil {
		details["tool_input"] = json.RawMessage(ev.ToolInput)
	}
	if ev.CostUSD > 0 {
		details["cost_usd"] = ev.CostUSD
	}
	if ev.DurationMS > 0 {
		details["duration_ms"] = ev.DurationMS
	}
	if ev.DurationAPIMS > 0 {
		details["duration_api_ms"] = ev.DurationAPIMS
	}
	if ev.NumTurns > 0 {
		details["num_turns"] = ev.NumTurns
	}
	if ev.IsError {
		details["is_error"] = true
	}

	input := rpiC2EventInput{
		RunID:   runID,
		Phase:   phaseNum,
		Backend: "stream",
		Source:  "claude_stream",
		Type:    "stream." + evType,
		Message: strings.TrimSpace(ev.Message),
	}
	if len(details) > 0 {
		input.Details = details
	}
	return input
}

func marshalRPIC2Details(details any) (json.RawMessage, error) {
	if details == nil {
		return nil, nil
	}
	data, err := json.Marshal(details)
	if err != nil {
		return nil, fmt.Errorf("marshal details: %w", err)
	}
	if bytes.Equal(data, []byte("null")) {
		return nil, nil
	}
	return json.RawMessage(data), nil
}

func newRPIC2EventID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("evt-%d", time.Now().UnixNano())
	}
	return "evt-" + hex.EncodeToString(buf)
}
