package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	StoreDirRel         = ".agents/daemon"
	LedgerFileName      = "ledger.jsonl"
	QuarantineDirName   = "quarantine"
	LedgerSchemaVersion = 1
)

type Store struct {
	root string
}

type LedgerEvent struct {
	SchemaVersion int            `json:"schema_version"`
	EventID       string         `json:"event_id"`
	RequestID     string         `json:"request_id"`
	JobID         string         `json:"job_id"`
	EventType     EventType      `json:"event_type"`
	OccurredAt    string         `json:"occurred_at"`
	Actor         string         `json:"actor"`
	Payload       map[string]any `json:"payload,omitempty"`
}

type ReplayResult struct {
	Events  []LedgerEvent   `json:"events"`
	Corrupt []CorruptRecord `json:"corrupt,omitempty"`
}

type CorruptRecord struct {
	LineNumber     int    `json:"line_number"`
	Error          string `json:"error"`
	QuarantinePath string `json:"quarantine_path,omitempty"`
}

func NewStore(root string) *Store {
	return &Store{root: root}
}

func (s *Store) Dir() string {
	return filepath.Join(s.root, StoreDirRel)
}

func (s *Store) LedgerPath() string {
	return filepath.Join(s.Dir(), LedgerFileName)
}

func (s *Store) QuarantineDir() string {
	return filepath.Join(s.Dir(), QuarantineDirName)
}

func (s *Store) AppendLedgerEvent(event LedgerEvent) (LedgerEvent, error) {
	event, err := NormalizeLedgerEvent(event)
	if err != nil {
		return LedgerEvent{}, err
	}

	replay, err := s.ReplayLedger()
	if err != nil {
		return LedgerEvent{}, err
	}
	for _, existing := range replay.Events {
		if existing.EventID == event.EventID {
			return existing, nil
		}
	}

	if err := os.MkdirAll(s.Dir(), 0700); err != nil {
		return LedgerEvent{}, fmt.Errorf("create daemon store dir: %w", err)
	}
	data, err := json.Marshal(event)
	if err != nil {
		return LedgerEvent{}, fmt.Errorf("marshal ledger event: %w", err)
	}
	file, err := os.OpenFile(s.LedgerPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return LedgerEvent{}, fmt.Errorf("open ledger: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return LedgerEvent{}, fmt.Errorf("append ledger: %w", err)
	}
	if err := file.Sync(); err != nil {
		return LedgerEvent{}, fmt.Errorf("sync ledger: %w", err)
	}
	return event, nil
}

func (s *Store) ReadLedger() ([]LedgerEvent, error) {
	replay, err := s.ReplayLedger()
	if err != nil {
		return nil, err
	}
	return replay.Events, nil
}

func (s *Store) ReplayLedger() (ReplayResult, error) {
	return s.replayLedger(true)
}

func (s *Store) ReplayLedgerReadOnly() (ReplayResult, error) {
	return s.replayLedger(false)
}

func (s *Store) replayLedger(quarantine bool) (ReplayResult, error) {
	file, err := os.Open(s.LedgerPath())
	if err != nil {
		if os.IsNotExist(err) {
			return ReplayResult{}, nil
		}
		return ReplayResult{}, fmt.Errorf("open ledger: %w", err)
	}
	defer file.Close()

	var result ReplayResult
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	seen := map[string]struct{}{}
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event LedgerEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			result.Corrupt = append(result.Corrupt, s.corruptRecord(lineNumber, line, err, quarantine))
			continue
		}
		if err := ValidateLedgerEvent(event); err != nil {
			result.Corrupt = append(result.Corrupt, s.corruptRecord(lineNumber, line, err, quarantine))
			continue
		}
		if _, ok := seen[event.EventID]; ok {
			continue
		}
		seen[event.EventID] = struct{}{}
		result.Events = append(result.Events, event)
	}
	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("scan ledger: %w", err)
	}
	return result, nil
}

func ValidateLedgerEvent(event LedgerEvent) error {
	if event.SchemaVersion != LedgerSchemaVersion {
		return fmt.Errorf("schema_version mismatch: got %d want %d", event.SchemaVersion, LedgerSchemaVersion)
	}
	required := []struct {
		name  string
		value string
	}{
		{"event_id", event.EventID},
		{"request_id", event.RequestID},
		{"job_id", event.JobID},
		{"event_type", string(event.EventType)},
		{"occurred_at", event.OccurredAt},
		{"actor", event.Actor},
	}
	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%s is required", field.name)
		}
	}
	if err := ValidateRequestID(event.RequestID); err != nil {
		return err
	}
	if err := ValidateEventType(event.EventType); err != nil {
		return err
	}
	if _, err := time.Parse(time.RFC3339Nano, event.OccurredAt); err != nil {
		return fmt.Errorf("invalid occurred_at: %w", err)
	}
	return nil
}

func (s *Store) quarantineCorruptLine(lineNumber int, line string, cause error) CorruptRecord {
	return s.corruptRecord(lineNumber, line, cause, true)
}

func (s *Store) corruptRecord(lineNumber int, line string, cause error, quarantine bool) CorruptRecord {
	record := CorruptRecord{
		LineNumber: lineNumber,
		Error:      cause.Error(),
	}
	if !quarantine {
		return record
	}
	if err := os.MkdirAll(s.QuarantineDir(), 0700); err != nil {
		record.Error = fmt.Sprintf("%s; quarantine failed: %v", record.Error, err)
		return record
	}
	record.QuarantinePath = filepath.Join(s.QuarantineDir(), fmt.Sprintf("ledger-line-%06d.json", lineNumber))
	payload := map[string]any{
		"line_number": lineNumber,
		"error":       cause.Error(),
		"raw":         line,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		record.Error = fmt.Sprintf("%s; quarantine marshal failed: %v", record.Error, err)
		return record
	}
	data = append(data, '\n')
	if err := os.WriteFile(record.QuarantinePath, data, 0600); err != nil {
		record.Error = fmt.Sprintf("%s; quarantine write failed: %v", record.Error, err)
		record.QuarantinePath = ""
	}
	return record
}
