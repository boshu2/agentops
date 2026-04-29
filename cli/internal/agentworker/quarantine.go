package agentworker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const QuarantineSchemaVersion = 1

// QuarantineRecord captures worker output that failed strict validation.
type QuarantineRecord struct {
	SchemaVersion int           `json:"schema_version"`
	Kind          string        `json:"kind"`
	Reason        string        `json:"reason"`
	Error         string        `json:"error"`
	JobID         string        `json:"job_id,omitempty"`
	AttemptID     string        `json:"attempt_id,omitempty"`
	RequestID     string        `json:"request_id,omitempty"`
	Session       SessionRef    `json:"session"`
	Terminal      TerminalState `json:"terminal"`
	Attempts      int           `json:"attempts"`
	RawOutput     string        `json:"raw_output"`
	CreatedAt     time.Time     `json:"created_at"`
}

// QuarantineWriter writes durable quarantine files.
type QuarantineWriter struct {
	Dir string
	Now func() time.Time
}

// Write persists a quarantine record atomically and returns its path.
func (w QuarantineWriter) Write(record QuarantineRecord) (string, error) {
	dir := strings.TrimSpace(w.Dir)
	if dir == "" {
		return "", fmt.Errorf("quarantine dir is required")
	}
	if record.SchemaVersion == 0 {
		record.SchemaVersion = QuarantineSchemaVersion
	}
	if record.CreatedAt.IsZero() {
		now := w.Now
		if now == nil {
			now = time.Now
		}
		record.CreatedAt = now().UTC()
	}
	if err := validateQuarantineRecord(record); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create quarantine dir: %w", err)
	}
	name := quarantineFileName(record)
	path := filepath.Join(dir, name)
	tmp := path + ".tmp"
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode quarantine record: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return "", fmt.Errorf("write quarantine temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("commit quarantine file: %w", err)
	}
	return path, nil
}

func validateQuarantineRecord(record QuarantineRecord) error {
	if record.SchemaVersion != QuarantineSchemaVersion {
		return fmt.Errorf("quarantine schema_version must be %d", QuarantineSchemaVersion)
	}
	if strings.TrimSpace(record.Kind) == "" {
		return fmt.Errorf("quarantine kind is required")
	}
	if strings.TrimSpace(record.Reason) == "" {
		return fmt.Errorf("quarantine reason is required")
	}
	if strings.TrimSpace(record.Error) == "" {
		return fmt.Errorf("quarantine error is required")
	}
	if strings.TrimSpace(record.RawOutput) == "" {
		return fmt.Errorf("quarantine raw_output is required")
	}
	if record.Attempts <= 0 {
		return fmt.Errorf("quarantine attempts must be positive")
	}
	if err := record.Session.Validate(); err != nil {
		return fmt.Errorf("quarantine session: %w", err)
	}
	return nil
}

func quarantineFileName(record QuarantineRecord) string {
	stamp := record.CreatedAt.UTC().Format("20060102T150405Z")
	identity := firstNonEmpty(record.Session.SessionID, record.JobID, record.RequestID, "worker-output")
	return stamp + "-" + sanitizeQuarantineFragment(identity) + ".json"
}

func sanitizeQuarantineFragment(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "worker-output"
	}
	return out
}
