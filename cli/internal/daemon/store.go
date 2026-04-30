package daemon

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	StoreDirRel         = ".agents/daemon"
	LedgerFileName      = "ledger.jsonl"
	QuarantineDirName   = "quarantine"
	LedgerSchemaVersion = 1
	// DefaultLedgerMaxBytes is the size threshold at which ledger.jsonl is
	// rotated. Operators may override via Store.WithLedgerMaxBytes.
	DefaultLedgerMaxBytes int64 = 50 * 1024 * 1024
	// ledgerArchivePrefix is the filename prefix for rotated ledger archives.
	// Format: ledger.<RFC3339-no-colons>.jsonl[.gz].
	ledgerArchivePrefix = "ledger."
	ledgerArchiveSuffix = ".jsonl"
)

// Store persists daemon ledger events to ~/<root>/.agents/daemon/ledger.jsonl.
// Concurrent writers are serialized through s.mu; readers (Replay*) operate on
// committed file contents and do not contend for the lock — O_APPEND atomicity
// covers per-line consistency.
type Store struct {
	root           string
	mu             sync.Mutex
	ledgerMaxBytes int64
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
	return &Store{root: root, ledgerMaxBytes: DefaultLedgerMaxBytes}
}

// WithLedgerMaxBytes overrides the rotation threshold and returns the store
// for fluent configuration. Pass <=0 to disable rotation entirely.
func (s *Store) WithLedgerMaxBytes(n int64) *Store {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ledgerMaxBytes = n
	return s
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

// LedgerArchivePaths returns the rotated ledger archives in chronological
// order (oldest first). Each path is one of ledger.<ts>.jsonl[.gz]; the
// active ledger.jsonl is excluded.
func (s *Store) LedgerArchivePaths() ([]string, error) {
	entries, err := os.ReadDir(s.Dir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read store dir: %w", err)
	}
	var archives []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == LedgerFileName {
			continue
		}
		if !strings.HasPrefix(name, ledgerArchivePrefix) {
			continue
		}
		if !strings.HasSuffix(name, ledgerArchiveSuffix) && !strings.HasSuffix(name, ledgerArchiveSuffix+".gz") {
			continue
		}
		archives = append(archives, filepath.Join(s.Dir(), name))
	}
	sort.Strings(archives)
	return archives, nil
}

func (s *Store) AppendLedgerEvent(event LedgerEvent) (LedgerEvent, error) {
	event, err := NormalizeLedgerEvent(event)
	if err != nil {
		return LedgerEvent{}, err
	}

	if err := os.MkdirAll(s.Dir(), 0700); err != nil {
		return LedgerEvent{}, fmt.Errorf("create daemon store dir: %w", err)
	}
	data, err := json.Marshal(event)
	if err != nil {
		return LedgerEvent{}, fmt.Errorf("marshal ledger event: %w", err)
	}
	line := append(data, '\n')

	// Serialize the dedup pre-check, rotation decision, and write together so
	// concurrent writers don't observe a half-rotated state nor write the same
	// EventID twice. Replay outside the lock would race with rotation.
	s.mu.Lock()
	defer s.mu.Unlock()

	replay, err := s.replayLedger(true)
	if err != nil {
		return LedgerEvent{}, err
	}
	for _, existing := range replay.Events {
		if existing.EventID == event.EventID {
			return existing, nil
		}
	}

	if err := s.maybeRotate(int64(len(line))); err != nil {
		return LedgerEvent{}, err
	}

	file, err := os.OpenFile(s.LedgerPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return LedgerEvent{}, fmt.Errorf("open ledger: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(line); err != nil {
		return LedgerEvent{}, fmt.Errorf("append ledger: %w", err)
	}
	if err := file.Sync(); err != nil {
		return LedgerEvent{}, fmt.Errorf("sync ledger: %w", err)
	}
	return event, nil
}

// maybeRotate is called with s.mu held. It checks the current ledger size
// and rotates if appending pendingBytes would push it past the threshold.
func (s *Store) maybeRotate(pendingBytes int64) error {
	if s.ledgerMaxBytes <= 0 {
		return nil
	}
	info, err := os.Stat(s.LedgerPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat ledger: %w", err)
	}
	if info.Size()+pendingBytes <= s.ledgerMaxBytes {
		return nil
	}
	return s.rotateLedger()
}

// rotateLedger atomically renames ledger.jsonl to a timestamped archive and
// gzip-compresses it. A fresh ledger.jsonl is created implicitly on the next
// append. Caller must hold s.mu.
func (s *Store) rotateLedger() error {
	timestamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	archive := filepath.Join(s.Dir(), ledgerArchivePrefix+timestamp+ledgerArchiveSuffix)
	if err := os.Rename(s.LedgerPath(), archive); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("rotate ledger: rename: %w", err)
	}
	if err := gzipFileInPlace(archive); err != nil {
		// Compression failure leaves the .jsonl archive in place; replay still
		// reads it (LedgerArchivePaths accepts both extensions).
		return fmt.Errorf("rotate ledger: gzip: %w", err)
	}
	return nil
}

// gzipFileInPlace compresses src to src+".gz" atomically: writes the gzip
// stream to a sibling tmp file, fsyncs and closes it, then renames it onto the
// final ".gz" path so concurrent readers never observe a partial gzip stream.
// The uncompressed src is removed only after the rename succeeds.
func gzipFileInPlace(src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	dst := src + ".gz"
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	zw := gzip.NewWriter(out)
	if _, err := io.Copy(zw, in); err != nil {
		_ = zw.Close()
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := zw.Close(); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Remove(src)
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

// replayLedger walks rotated archives oldest-first then the current ledger,
// dedup by EventID across the entire chain so rotation is invisible to readers.
func (s *Store) replayLedger(quarantine bool) (ReplayResult, error) {
	archives, err := s.LedgerArchivePaths()
	if err != nil {
		return ReplayResult{}, err
	}

	var result ReplayResult
	seen := map[string]struct{}{}
	lineCounter := 0

	for _, archive := range archives {
		if err := s.replayLedgerFile(archive, &lineCounter, seen, &result, quarantine); err != nil {
			return result, err
		}
	}
	if err := s.replayLedgerFile(s.LedgerPath(), &lineCounter, seen, &result, quarantine); err != nil {
		return result, err
	}
	return result, nil
}

// replayLedgerFile reads a single ledger file (current or archive) and appends
// valid events to result.Events. Deduplication is shared across calls via seen.
func (s *Store) replayLedgerFile(path string, lineCounter *int, seen map[string]struct{}, result *ReplayResult, quarantine bool) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open ledger %s: %w", filepath.Base(path), err)
	}
	defer file.Close()

	var reader io.Reader = file
	if strings.HasSuffix(path, ".gz") {
		zr, err := gzip.NewReader(file)
		if err != nil {
			return fmt.Errorf("open gzip archive %s: %w", filepath.Base(path), err)
		}
		defer zr.Close()
		reader = zr
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		*lineCounter++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event LedgerEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			result.Corrupt = append(result.Corrupt, s.corruptRecord(*lineCounter, line, err, quarantine))
			continue
		}
		if err := ValidateLedgerEvent(event); err != nil {
			result.Corrupt = append(result.Corrupt, s.corruptRecord(*lineCounter, line, err, quarantine))
			continue
		}
		if _, ok := seen[event.EventID]; ok {
			continue
		}
		seen[event.EventID] = struct{}{}
		result.Events = append(result.Events, event)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan ledger %s: %w", filepath.Base(path), err)
	}
	return nil
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
