// practices: [agile-manifesto, dora-metrics]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppendRPILedgerRecord(t *testing.T) {
	root := t.TempDir()

	first, err := AppendRPILedgerRecord(root, RPILedgerAppendInput{
		RunID:   "run-append",
		Phase:   "research",
		Action:  "start",
		Details: map[string]any{"step": 1},
	})
	if err != nil {
		t.Fatalf("append first record: %v", err)
	}
	second, err := AppendRPILedgerRecord(root, RPILedgerAppendInput{
		RunID:   "run-append",
		Phase:   "plan",
		Action:  "advance",
		Details: map[string]any{"step": 2},
	})
	if err != nil {
		t.Fatalf("append second record: %v", err)
	}

	records, err := LoadRPILedgerRecords(root)
	if err != nil {
		t.Fatalf("load records: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].PrevHash != "" {
		t.Fatalf("expected first prev_hash empty, got %q", records[0].PrevHash)
	}
	if records[1].PrevHash != records[0].Hash {
		t.Fatalf("expected second prev_hash to match first hash")
	}
	if first.Hash != records[0].Hash || second.Hash != records[1].Hash {
		t.Fatalf("returned records should match file contents")
	}
	if !strings.HasSuffix(records[0].TS, "Z") {
		t.Fatalf("expected UTC timestamp with Z suffix, got %q", records[0].TS)
	}
	if _, err := time.Parse(time.RFC3339Nano, records[0].TS); err != nil {
		t.Fatalf("timestamp must be RFC3339Nano: %v", err)
	}
}

func TestVerifyRPILedgerChain_Success(t *testing.T) {
	root := t.TempDir()
	for i := range 3 {
		_, err := AppendRPILedgerRecord(root, RPILedgerAppendInput{
			RunID:   "run-ok",
			Phase:   "phase",
			Action:  "action",
			Details: map[string]any{"index": i},
		})
		if err != nil {
			t.Fatalf("append record %d: %v", i, err)
		}
	}

	if err := VerifyRPILedger(root); err != nil {
		t.Fatalf("verify should pass, got: %v", err)
	}
}

func TestVerifyRPILedgerChain_TamperFailure(t *testing.T) {
	root := t.TempDir()
	for i := range 2 {
		_, err := AppendRPILedgerRecord(root, RPILedgerAppendInput{
			RunID:   "run-tamper",
			Phase:   "phase",
			Action:  "action",
			Details: map[string]any{"index": i},
		})
		if err != nil {
			t.Fatalf("append record %d: %v", i, err)
		}
	}

	ledgerPath := RPILedgerPath(root)
	data, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 ledger lines, got %d", len(lines))
	}

	var tampered RPILedgerRecord
	if err := json.Unmarshal([]byte(lines[0]), &tampered); err != nil {
		t.Fatalf("decode first line: %v", err)
	}
	tampered.Action = "tampered-action"
	tamperedLine, err := json.Marshal(tampered)
	if err != nil {
		t.Fatalf("re-marshal tampered line: %v", err)
	}
	lines[0] = string(tamperedLine)

	if err := os.WriteFile(ledgerPath, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatalf("write tampered ledger: %v", err)
	}

	err = VerifyRPILedger(root)
	if err == nil {
		t.Fatalf("expected verification failure for tampered ledger")
	}
	if !strings.Contains(err.Error(), "payload_hash mismatch") {
		t.Fatalf("expected payload_hash mismatch error, got: %v", err)
	}
}

func TestMaterializeRPIRunCache(t *testing.T) {
	root := t.TempDir()

	events := []RPILedgerAppendInput{
		{
			RunID:   "run-a",
			Phase:   "research",
			Action:  "start",
			Details: map[string]any{"order": 1},
		},
		{
			RunID:   "run-b",
			Phase:   "research",
			Action:  "start",
			Details: map[string]any{"order": 1},
		},
		{
			RunID:   "run-a",
			Phase:   "plan",
			Action:  "finish",
			Details: map[string]any{"order": 2},
		},
	}
	for i, event := range events {
		if _, err := AppendRPILedgerRecord(root, event); err != nil {
			t.Fatalf("append event %d: %v", i, err)
		}
	}

	if err := MaterializeRPIRunCache(root, "run-a"); err != nil {
		t.Fatalf("materialize cache: %v", err)
	}

	cachePath := filepath.Join(root, ".agents/rpi/runs/run-a.json")
	cacheBytes, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read cache: %v", err)
	}

	var cache RPIRunCache
	if err := json.Unmarshal(cacheBytes, &cache); err != nil {
		t.Fatalf("decode cache: %v", err)
	}
	if cache.RunID != "run-a" {
		t.Fatalf("expected run_id run-a, got %q", cache.RunID)
	}
	if cache.EventCount != 2 {
		t.Fatalf("expected 2 run-a events, got %d", cache.EventCount)
	}
	if cache.Latest.Action != "finish" {
		t.Fatalf("expected latest action finish, got %q", cache.Latest.Action)
	}
	if cache.Latest.Phase != "plan" {
		t.Fatalf("expected latest phase plan, got %q", cache.Latest.Phase)
	}
}

// ---------------------------------------------------------------------------
// verifyRPILedger (50%) — exercise all branches
// ---------------------------------------------------------------------------

func TestVerifyRPILedger_EmptyLedger(t *testing.T) {
	root := t.TempDir()
	// No ledger file → LoadRPILedgerRecords returns nil, nil
	result, err := verifyRPILedger(root)
	if err != nil {
		t.Fatalf("verifyRPILedger() error = %v", err)
	}
	if !result.Pass {
		t.Error("expected pass=true for empty ledger")
	}
	if result.RecordCount != 0 {
		t.Errorf("expected 0 records, got %d", result.RecordCount)
	}
}

func TestVerifyRPILedger_ValidChain(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 3; i++ {
		_, err := AppendRPILedgerRecord(root, RPILedgerAppendInput{
			RunID:   "run-verify",
			Phase:   "research",
			Action:  "step",
			Details: map[string]any{"index": i},
		})
		if err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	result, err := verifyRPILedger(root)
	if err != nil {
		t.Fatalf("verifyRPILedger() error = %v", err)
	}
	if !result.Pass {
		t.Errorf("expected pass=true, got message: %s", result.Message)
	}
	if result.RecordCount != 3 {
		t.Errorf("expected 3 records, got %d", result.RecordCount)
	}
	if result.FirstBrokenIndex != -1 {
		t.Errorf("expected FirstBrokenIndex=-1, got %d", result.FirstBrokenIndex)
	}
}

func TestVerifyRPILedger_TamperedRecord(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 2; i++ {
		_, err := AppendRPILedgerRecord(root, RPILedgerAppendInput{
			RunID:   "run-tamper",
			Phase:   "plan",
			Action:  "advance",
			Details: map[string]any{"i": i},
		})
		if err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	// Tamper with the first record
	ledgerPath := RPILedgerPath(root)
	data, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var record RPILedgerRecord
	if err := json.Unmarshal([]byte(lines[0]), &record); err != nil {
		t.Fatal(err)
	}
	record.Action = "TAMPERED"
	tampered, _ := json.Marshal(record)
	lines[0] = string(tampered)
	if err := os.WriteFile(ledgerPath, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := verifyRPILedger(root)
	if err != nil {
		t.Fatalf("verifyRPILedger() error = %v", err)
	}
	if result.Pass {
		t.Error("expected pass=false for tampered ledger")
	}
	if result.FirstBrokenIndex != 1 {
		t.Errorf("expected FirstBrokenIndex=1, got %d", result.FirstBrokenIndex)
	}
	if result.Message == "" {
		t.Error("expected non-empty message")
	}
}

func TestVerifyRPILedger_BrokenPrevHash(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 2; i++ {
		_, err := AppendRPILedgerRecord(root, RPILedgerAppendInput{
			RunID:   "run-chain",
			Phase:   "implement",
			Action:  "do",
			Details: map[string]any{"i": i},
		})
		if err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	// Break the prev_hash link
	ledgerPath := RPILedgerPath(root)
	data, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var record RPILedgerRecord
	if err := json.Unmarshal([]byte(lines[1]), &record); err != nil {
		t.Fatal(err)
	}
	record.PrevHash = "0000000000000000000000000000000000000000000000000000000000000000"
	broken, _ := json.Marshal(record)
	lines[1] = string(broken)
	if err := os.WriteFile(ledgerPath, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := verifyRPILedger(root)
	if err != nil {
		t.Fatalf("verifyRPILedger() error = %v", err)
	}
	if result.Pass {
		t.Error("expected pass=false for broken prev_hash")
	}
	if !strings.Contains(result.Message, "prev_hash mismatch") {
		t.Errorf("expected prev_hash mismatch message, got: %s", result.Message)
	}
}

// ---------------------------------------------------------------------------
// validateAppendInput — edge cases
// ---------------------------------------------------------------------------

func TestValidateAppendInput_MissingFields(t *testing.T) {
	tests := []struct {
		name  string
		input RPILedgerAppendInput
	}{
		{"missing run_id", RPILedgerAppendInput{RunID: "", Phase: "plan", Action: "start"}},
		{"missing phase", RPILedgerAppendInput{RunID: "r1", Phase: "", Action: "start"}},
		{"missing action", RPILedgerAppendInput{RunID: "r1", Phase: "plan", Action: ""}},
		{"whitespace run_id", RPILedgerAppendInput{RunID: "  ", Phase: "plan", Action: "start"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateAppendInput(tt.input); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// validateRunID
// ---------------------------------------------------------------------------

func TestValidateRunID(t *testing.T) {
	tests := []struct {
		name    string
		runID   string
		wantErr bool
	}{
		{"valid", "run-abc", false},
		{"empty", "", true},
		{"whitespace", "  ", true},
		{"path separator", "run/bad", true},
		{"dot-dot", "run..bad", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRunID(tt.runID)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRunID(%q) error = %v, wantErr %v", tt.runID, err, tt.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// filterRunRecords
// ---------------------------------------------------------------------------

func TestFilterRunRecords(t *testing.T) {
	records := []RPILedgerRecord{
		{RunID: "run-a", Action: "start"},
		{RunID: "run-b", Action: "start"},
		{RunID: "run-a", Action: "finish"},
	}

	latest, count := filterRunRecords(records, "run-a")
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
	if latest.Action != "finish" {
		t.Errorf("expected latest action 'finish', got %q", latest.Action)
	}

	_, count = filterRunRecords(records, "run-c")
	if count != 0 {
		t.Errorf("expected 0 for nonexistent run, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// normalizeDetails — various input types
// ---------------------------------------------------------------------------

func TestNormalizeDetails_Nil(t *testing.T) {
	result, err := normalizeDetails(nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if string(result) != "{}" {
		t.Errorf("expected '{}', got %q", string(result))
	}
}

func TestNormalizeDetails_EmptyBytes(t *testing.T) {
	result, err := normalizeDetails(json.RawMessage([]byte("  ")))
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if string(result) != "{}" {
		t.Errorf("expected '{}', got %q", string(result))
	}
}

func TestNormalizeDetails_ValidMap(t *testing.T) {
	input := map[string]any{"key": "value"}
	result, err := normalizeDetails(input)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(string(result), "key") {
		t.Errorf("expected 'key' in result, got %q", string(result))
	}
}

// ---------------------------------------------------------------------------
// validateLedgerTimestamp
// ---------------------------------------------------------------------------

func TestValidateLedgerTimestamp(t *testing.T) {
	tests := []struct {
		name    string
		ts      string
		wantErr bool
	}{
		{"valid UTC", "2026-01-15T10:30:00Z", false},
		{"valid UTC nano", "2026-01-15T10:30:00.123456789Z", false},
		{"invalid format", "2026-01-15", true},
		{"non-UTC", "2026-01-15T10:30:00+05:00", true},
		{"empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLedgerTimestamp(tt.ts)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateLedgerTimestamp(%q) error = %v, wantErr %v", tt.ts, err, tt.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// validateLedgerRequiredFields
// ---------------------------------------------------------------------------

func TestValidateLedgerRequiredFields_AllPresent(t *testing.T) {
	record := RPILedgerRecord{
		EventID:     "evt-123",
		RunID:       "run-1",
		Phase:       "plan",
		Action:      "start",
		TS:          "2026-01-15T10:30:00Z",
		PayloadHash: "abc123",
		Hash:        "def456",
	}
	if err := validateLedgerRequiredFields(record); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateLedgerRequiredFields_Missing(t *testing.T) {
	record := RPILedgerRecord{
		EventID: "evt-123",
		// Missing RunID
		Phase:       "plan",
		Action:      "start",
		TS:          "2026-01-15T10:30:00Z",
		PayloadHash: "abc123",
		Hash:        "def456",
	}
	if err := validateLedgerRequiredFields(record); err == nil {
		t.Error("expected error for missing run_id")
	}
}

// ---------------------------------------------------------------------------
// writeRunCache
// ---------------------------------------------------------------------------

func TestWriteRunCache(t *testing.T) {
	root := t.TempDir()
	record := RPILedgerRecord{
		RunID:  "run-cache-test",
		Phase:  "validate",
		Action: "complete",
	}

	err := writeRunCache(root, "run-cache-test", record, 5)
	if err != nil {
		t.Fatalf("writeRunCache() error = %v", err)
	}

	cachePath := filepath.Join(root, ".agents/rpi/runs/run-cache-test.json")
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read cache: %v", err)
	}

	var cache RPIRunCache
	if err := json.Unmarshal(data, &cache); err != nil {
		t.Fatalf("parse cache: %v", err)
	}
	if cache.RunID != "run-cache-test" {
		t.Errorf("run_id = %q, want run-cache-test", cache.RunID)
	}
	if cache.EventCount != 5 {
		t.Errorf("event_count = %d, want 5", cache.EventCount)
	}
}

// ---------------------------------------------------------------------------
// writeFileAtomic
// ---------------------------------------------------------------------------

func TestWriteFileAtomic(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.json")

	data := []byte(`{"hello": "world"}`)
	if err := writeFileAtomic(path, data, 0644); err != nil {
		t.Fatalf("writeFileAtomic() error = %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != string(data) {
		t.Errorf("content = %q, want %q", string(content), string(data))
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0644 {
		t.Errorf("mode = %o, want 0644", info.Mode().Perm())
	}
}

func TestWriteFileAtomic_Overwrite(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "overwrite.json")

	// Write initial content
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	// Overwrite with atomic write
	if err := writeFileAtomic(path, []byte("new"), 0644); err != nil {
		t.Fatalf("writeFileAtomic() error = %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new" {
		t.Errorf("expected 'new', got %q", string(content))
	}
}

// ---------------------------------------------------------------------------
// hashHex
// ---------------------------------------------------------------------------

func TestHashHex(t *testing.T) {
	hash := hashHex([]byte("test"))
	if len(hash) != 64 { // SHA-256 hex = 64 chars
		t.Errorf("expected 64 hex chars, got %d", len(hash))
	}
	// Same input should produce same hash
	hash2 := hashHex([]byte("test"))
	if hash != hash2 {
		t.Error("expected deterministic hash")
	}
	// Different input should produce different hash
	hash3 := hashHex([]byte("other"))
	if hash == hash3 {
		t.Error("expected different hash for different input")
	}
}

// ---------------------------------------------------------------------------
// newRPILedgerEventID
// ---------------------------------------------------------------------------

func TestNewRPILedgerEventID(t *testing.T) {
	id := newRPILedgerEventID()
	if !strings.HasPrefix(id, "evt-") {
		t.Errorf("expected 'evt-' prefix, got %q", id)
	}
	if len(id) < 10 {
		t.Errorf("expected longer event ID, got %q", id)
	}

	// Should be unique
	id2 := newRPILedgerEventID()
	if id == id2 {
		t.Error("expected unique event IDs")
	}
}

// ---------------------------------------------------------------------------
// RPILedgerPath
// ---------------------------------------------------------------------------

func TestRPILedgerPath(t *testing.T) {
	path := RPILedgerPath("/root")
	expected := filepath.Join("/root", ".agents/ledger/rpi-events.jsonl")
	if path != expected {
		t.Errorf("RPILedgerPath() = %q, want %q", path, expected)
	}
}

// ---------------------------------------------------------------------------
// appendRPILedgerEvent (internal alias)
// ---------------------------------------------------------------------------

func TestAppendRPILedgerEvent(t *testing.T) {
	root := t.TempDir()
	record, err := appendRPILedgerEvent(root, rpiLedgerEvent{
		RunID:   "run-alias",
		Phase:   "research",
		Action:  "start",
		Details: map[string]any{"test": true},
	})
	if err != nil {
		t.Fatalf("appendRPILedgerEvent() error = %v", err)
	}
	if record.RunID != "run-alias" {
		t.Errorf("run_id = %q, want 'run-alias'", record.RunID)
	}
}

// ---------------------------------------------------------------------------
// materializeRPIRunCache (internal alias)
// ---------------------------------------------------------------------------

func TestMaterializeRPIRunCache_NotFound(t *testing.T) {
	root := t.TempDir()
	// Append one record for a different run
	_, err := AppendRPILedgerRecord(root, RPILedgerAppendInput{
		RunID:   "run-other",
		Phase:   "research",
		Action:  "start",
		Details: map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = materializeRPIRunCache(root, "run-missing")
	if err == nil {
		t.Error("expected error for nonexistent run")
	}
}

// ---------------------------------------------------------------------------
// computeLedgerHashes — deterministic
// ---------------------------------------------------------------------------

func TestComputeLedgerHashes_Deterministic(t *testing.T) {
	record := RPILedgerRecord{
		SchemaVersion: 1,
		EventID:       "evt-test",
		RunID:         "run-hash",
		TS:            "2026-01-15T10:30:00Z",
		Phase:         "plan",
		Action:        "start",
		Details:       json.RawMessage(`{"key":"value"}`),
		PrevHash:      "",
	}

	ph1, h1, err := computeLedgerHashes(record)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	ph2, h2, err := computeLedgerHashes(record)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if ph1 != ph2 {
		t.Error("expected deterministic payload hash")
	}
	if h1 != h2 {
		t.Error("expected deterministic hash")
	}
}

func TestLoadRecords_EmptyFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "empty.jsonl")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	records, err := loadRPILedgerRecordsFromPath(path)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

func TestLoadRecords_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "bad.jsonl")
	if err := os.WriteFile(path, []byte("not json\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := loadRPILedgerRecordsFromPath(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadRecords_BlankLines(t *testing.T) {
	root := t.TempDir()
	// Append a record
	_, err := AppendRPILedgerRecord(root, RPILedgerAppendInput{
		RunID:   "run-blanks",
		Phase:   "plan",
		Action:  "start",
		Details: map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Inject blank lines
	ledgerPath := RPILedgerPath(root)
	data, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	newData := "\n\n" + string(data) + "\n\n"
	if err := os.WriteFile(ledgerPath, []byte(newData), 0644); err != nil {
		t.Fatal(err)
	}

	records, err := loadRPILedgerRecordsFromPath(ledgerPath)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 record (blank lines skipped), got %d", len(records))
	}
}
