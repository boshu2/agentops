package daemon

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Schedule event types are additive event-type vocabulary (LedgerSchemaVersion
// stays at 1). Older daemon binaries replaying ledgers with these events must
// skip-and-log instead of erroring — see projections.go's reducer for the
// forward-compat contract (pre-mortem amendment B3).
const (
	EventScheduleCreated EventType = "schedule.created"
	EventScheduleFired   EventType = "schedule.fired"
	EventScheduleSkipped EventType = "schedule.skipped"
	EventScheduleDeleted EventType = "schedule.deleted"
)

// scheduleActor is the actor recorded on schedule-* ledger events when no
// per-call actor is supplied. The supervisor (soc-8inr.4) overwrites this on
// fired/skipped events; created/deleted come from the schedule storage layer
// itself.
const scheduleActor = "agentopsd-scheduler"

// scheduleSentinelJobID is used on schedule.created and schedule.deleted
// events that have no associated job execution. ledger validation requires a
// non-empty job_id; using a stable sentinel keeps the contract intact while
// signalling "this event is schedule-scoped, not job-scoped".
const scheduleSentinelJobID = "schedule"

func init() {
	// Register the new schedule.* event types and the llmwiki.loop job type
	// (added by Worker 1's commit ab563061 but not yet wired into the
	// validation sets). Done via init() so types.go stays untouched.
	eventTypeSet[string(EventScheduleCreated)] = struct{}{}
	eventTypeSet[string(EventScheduleFired)] = struct{}{}
	eventTypeSet[string(EventScheduleSkipped)] = struct{}{}
	eventTypeSet[string(EventScheduleDeleted)] = struct{}{}
	jobTypeSet[string(JobTypeLLMWikiLoop)] = struct{}{}
}

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

// SaveSchedule persists a recurring job template by appending a
// schedule.created event to the ledger. Returns an error if a schedule with
// the same Name already exists (no upsert; callers must DeleteSchedule first
// to replace).
//
// The full RecurringJobTemplate is serialized into the event payload under
// the "template" key so replay can reconstruct the schedule list.
func (s *Store) SaveSchedule(t RecurringJobTemplate) error {
	name := strings.TrimSpace(t.Name)
	if name == "" {
		return fmt.Errorf("schedule name is required")
	}
	if strings.TrimSpace(t.Cron) == "" {
		return fmt.Errorf("schedule %q: cron is required", name)
	}
	if strings.TrimSpace(string(t.JobType)) == "" {
		return fmt.Errorf("schedule %q: job_type is required", name)
	}

	existing, err := s.ListSchedules()
	if err != nil {
		return fmt.Errorf("save schedule %q: %w", name, err)
	}
	for _, s := range existing {
		if s.Name == name {
			return fmt.Errorf("schedule %q already exists", name)
		}
	}

	event, err := scheduleCreatedEvent(t)
	if err != nil {
		return fmt.Errorf("save schedule %q: %w", name, err)
	}
	if _, err := s.AppendLedgerEvent(event); err != nil {
		return fmt.Errorf("save schedule %q: %w", name, err)
	}
	return nil
}

// ListSchedules returns the current set of recurring schedules derived from
// ledger replay. A schedule is present iff its most recent schedule.created
// has not been followed by a matching schedule.deleted. Order matches the
// order schedules were created (stable, from the ledger).
func (s *Store) ListSchedules() ([]RecurringJobTemplate, error) {
	replay, err := s.ReplayLedgerReadOnly()
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}
	return ScheduleStateFromEvents(replay.Events), nil
}

// DeleteSchedule removes a schedule by appending a schedule.deleted event.
// Idempotent: deleting a non-existent schedule is a no-op (logs a warning
// but returns nil) so callers don't have to guard against races.
func (s *Store) DeleteSchedule(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("schedule name is required")
	}
	existing, err := s.ListSchedules()
	if err != nil {
		return fmt.Errorf("delete schedule %q: %w", name, err)
	}
	found := false
	for _, sched := range existing {
		if sched.Name == name {
			found = true
			break
		}
	}
	if !found {
		log.Printf("[store] DeleteSchedule: schedule %q not found, treating as no-op", name)
		return nil
	}

	event, err := scheduleDeletedEvent(name, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("delete schedule %q: %w", name, err)
	}
	if _, err := s.AppendLedgerEvent(event); err != nil {
		return fmt.Errorf("delete schedule %q: %w", name, err)
	}
	return nil
}

// RecordScheduleFired appends a schedule.fired event linking a recurrence
// tick to the submission_id of the job it enqueued. Used by the recurrence
// supervisor (soc-8inr.4).
func (s *Store) RecordScheduleFired(name, submissionID string, tickAt time.Time) error {
	name = strings.TrimSpace(name)
	submissionID = strings.TrimSpace(submissionID)
	if name == "" {
		return fmt.Errorf("schedule name is required")
	}
	if submissionID == "" {
		return fmt.Errorf("submission_id is required")
	}
	event, err := scheduleFiredEvent(name, submissionID, tickAt)
	if err != nil {
		return fmt.Errorf("record schedule fired %q: %w", name, err)
	}
	if _, err := s.AppendLedgerEvent(event); err != nil {
		return fmt.Errorf("record schedule fired %q: %w", name, err)
	}
	return nil
}

// RecordScheduleSkipped appends a schedule.skipped event when the supervisor
// elects not to enqueue a tick (e.g., backpressure: SkipIfRunning,
// MaxQueueDepth exceeded). reason is free-form so future backpressure
// strategies can record their own labels without a schema change.
func (s *Store) RecordScheduleSkipped(name, reason string, tickAt time.Time) error {
	name = strings.TrimSpace(name)
	reason = strings.TrimSpace(reason)
	if name == "" {
		return fmt.Errorf("schedule name is required")
	}
	if reason == "" {
		return fmt.Errorf("reason is required")
	}
	event, err := scheduleSkippedEvent(name, reason, tickAt)
	if err != nil {
		return fmt.Errorf("record schedule skipped %q: %w", name, err)
	}
	if _, err := s.AppendLedgerEvent(event); err != nil {
		return fmt.Errorf("record schedule skipped %q: %w", name, err)
	}
	return nil
}

// scheduleCreatedEvent builds a schedule.created LedgerEvent from a template.
// The template is serialized via json.Marshal/Unmarshal-into-map so the
// resulting payload is plain JSON (string-keyed maps, no Go types) and
// survives a round-trip through the ledger.
func scheduleCreatedEvent(t RecurringJobTemplate) (LedgerEvent, error) {
	templateMap, err := templateToMap(t)
	if err != nil {
		return LedgerEvent{}, fmt.Errorf("marshal template: %w", err)
	}
	return LedgerEvent{
		SchemaVersion: LedgerSchemaVersion,
		EventID:       fmt.Sprintf("evt-schedule-created-%s-%d", t.Name, time.Now().UTC().UnixNano()),
		RequestID:     fmt.Sprintf("schedule-%s", t.Name),
		JobID:         scheduleSentinelJobID,
		EventType:     EventScheduleCreated,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339Nano),
		Actor:         scheduleActor,
		Payload: map[string]any{
			"name":     t.Name,
			"template": templateMap,
		},
	}, nil
}

func scheduleDeletedEvent(name string, deletedAt time.Time) (LedgerEvent, error) {
	deletedAtStr := deletedAt.UTC().Format(time.RFC3339Nano)
	return LedgerEvent{
		SchemaVersion: LedgerSchemaVersion,
		EventID:       fmt.Sprintf("evt-schedule-deleted-%s-%d", name, deletedAt.UTC().UnixNano()),
		RequestID:     fmt.Sprintf("schedule-%s", name),
		JobID:         scheduleSentinelJobID,
		EventType:     EventScheduleDeleted,
		OccurredAt:    deletedAtStr,
		Actor:         scheduleActor,
		Payload: map[string]any{
			"name":       name,
			"deleted_at": deletedAtStr,
		},
	}, nil
}

func scheduleFiredEvent(name, submissionID string, tickAt time.Time) (LedgerEvent, error) {
	tickAtStr := tickAt.UTC().Format(time.RFC3339Nano)
	return LedgerEvent{
		SchemaVersion: LedgerSchemaVersion,
		EventID:       fmt.Sprintf("evt-schedule-fired-%s-%d", name, tickAt.UTC().UnixNano()),
		RequestID:     fmt.Sprintf("schedule-%s", name),
		JobID:         submissionID,
		EventType:     EventScheduleFired,
		OccurredAt:    tickAtStr,
		Actor:         scheduleActor,
		Payload: map[string]any{
			"name":          name,
			"submission_id": submissionID,
			"tick_at":       tickAtStr,
		},
	}, nil
}

func scheduleSkippedEvent(name, reason string, tickAt time.Time) (LedgerEvent, error) {
	tickAtStr := tickAt.UTC().Format(time.RFC3339Nano)
	return LedgerEvent{
		SchemaVersion: LedgerSchemaVersion,
		EventID:       fmt.Sprintf("evt-schedule-skipped-%s-%d", name, tickAt.UTC().UnixNano()),
		RequestID:     fmt.Sprintf("schedule-%s", name),
		JobID:         scheduleSentinelJobID,
		EventType:     EventScheduleSkipped,
		OccurredAt:    tickAtStr,
		Actor:         scheduleActor,
		Payload: map[string]any{
			"name":    name,
			"reason":  reason,
			"tick_at": tickAtStr,
		},
	}, nil
}

// templateToMap converts a RecurringJobTemplate to a JSON-shaped map so it
// can ride inside a LedgerEvent.Payload (which is map[string]any). Going via
// json.Marshal/Unmarshal preserves field tags + omitempty semantics.
func templateToMap(t RecurringJobTemplate) (map[string]any, error) {
	data, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// templateFromMap is the inverse of templateToMap. Used by the projection
// reducer + ListSchedules to reconstruct typed templates from ledger
// payloads.
func templateFromMap(raw any) (RecurringJobTemplate, error) {
	if raw == nil {
		return RecurringJobTemplate{}, fmt.Errorf("template missing from payload")
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return RecurringJobTemplate{}, err
	}
	var out RecurringJobTemplate
	if err := json.Unmarshal(data, &out); err != nil {
		return RecurringJobTemplate{}, err
	}
	return out, nil
}

// ScheduleStateFromEvents reduces a ledger event slice to the active
// schedule list. Insertion order is preserved (the slice key is creation
// time). schedule.deleted removes the entry by name. Unknown event types
// are ignored — schedule reduction only cares about its own vocabulary.
//
// Exposed for tests + projection helpers in projections.go.
func ScheduleStateFromEvents(events []LedgerEvent) []RecurringJobTemplate {
	type slot struct {
		template RecurringJobTemplate
		index    int
	}
	byName := map[string]slot{}
	order := 0
	for _, ev := range events {
		switch ev.EventType {
		case EventScheduleCreated:
			tmpl, err := templateFromMap(ev.Payload["template"])
			if err != nil {
				log.Printf("[store] schedule.created event %q has malformed template payload: %v", ev.EventID, err)
				continue
			}
			if tmpl.Name == "" {
				if name, ok := ev.Payload["name"].(string); ok {
					tmpl.Name = name
				}
			}
			if tmpl.Name == "" {
				log.Printf("[store] schedule.created event %q has no name; skipping", ev.EventID)
				continue
			}
			byName[tmpl.Name] = slot{template: tmpl, index: order}
			order++
		case EventScheduleDeleted:
			name, _ := ev.Payload["name"].(string)
			if name == "" {
				continue
			}
			delete(byName, name)
		}
	}
	out := make([]RecurringJobTemplate, 0, len(byName))
	for _, sl := range byName {
		out = append(out, sl.template)
	}
	sort.Slice(out, func(i, j int) bool {
		return byName[out[i].Name].index < byName[out[j].Name].index
	})
	return out
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
