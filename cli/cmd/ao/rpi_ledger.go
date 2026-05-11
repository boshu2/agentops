// practices: [agile-manifesto, dora-metrics]
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/rpi"
)

const (
	rpiLedgerSchemaVersion = rpi.LedgerSchemaVersion
	rpiLedgerRelativePath  = rpi.LedgerRelativePath
	rpiRunCacheRelativeDir = rpi.RunCacheRelativeDir
)

// RPILedgerRecord is a single append-only event in the RPI ledger.
type RPILedgerRecord = rpi.LedgerRecord

// RPILedgerAppendInput contains fields needed for appending an event.
type RPILedgerAppendInput = rpi.LedgerAppendInput

// RPIRunCache is a materialized cache of the latest state for one run.
type RPIRunCache = rpi.RunCache

// rpiLedgerEvent is the internal event shape used by rpi orchestration code.
type rpiLedgerEvent struct {
	RunID   string
	Phase   string
	Action  string
	Details any
}

// rpiLedgerRecord is the internal alias used by rpi orchestration code.
type rpiLedgerRecord = rpi.LedgerRecord

// rpiLedgerVerifyResult is the machine-readable verify output contract.
type rpiLedgerVerifyResult = rpi.LedgerVerifyResult

// RPILedgerPath returns the absolute ledger file path for a repo root.
func RPILedgerPath(rootDir string) string {
	return filepath.Join(rootDir, rpiLedgerRelativePath)
}

// AppendRPILedgerRecord appends one event with lock + fsync durability.
func AppendRPILedgerRecord(rootDir string, input RPILedgerAppendInput) (RPILedgerRecord, error) {
	if err := rpi.ValidateAppendInput(input); err != nil {
		return RPILedgerRecord{}, err
	}

	ledgerPath := RPILedgerPath(rootDir)
	ledgerDir := filepath.Dir(ledgerPath)
	if err := os.MkdirAll(ledgerDir, 0750); err != nil {
		return RPILedgerRecord{}, fmt.Errorf("create ledger dir: %w", err)
	}

	lockFile, err := acquireLedgerLock(ledgerPath)
	if err != nil {
		return RPILedgerRecord{}, err
	}
	defer releaseLedgerLock(lockFile)

	ledgerFile, err := os.OpenFile(ledgerPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return RPILedgerRecord{}, fmt.Errorf("open ledger: %w", err)
	}
	defer func() { _ = ledgerFile.Close() }()

	prevHash, err := readLastLedgerHash(ledgerFile)
	if err != nil {
		return RPILedgerRecord{}, err
	}

	record, err := rpi.BuildLedgerRecordFromInput(input, prevHash)
	if err != nil {
		return RPILedgerRecord{}, err
	}

	if err := writeLedgerRecord(ledgerFile, record, ledgerDir); err != nil {
		return RPILedgerRecord{}, err
	}

	return record, nil
}

// acquireLedgerLock opens and exclusively locks the ledger lock file.
func acquireLedgerLock(ledgerPath string) (*os.File, error) {
	lockFile, err := os.OpenFile(ledgerPath+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open ledger lock: %w", err)
	}
	if err := flockLock(lockFile); err != nil {
		_ = lockFile.Close()
		return nil, fmt.Errorf("lock ledger: %w", err)
	}
	return lockFile, nil
}

// releaseLedgerLock releases and closes the ledger lock file.
func releaseLedgerLock(lockFile *os.File) {
	_ = flockUnlock(lockFile)
	_ = lockFile.Close()
}

// writeLedgerRecord marshals and appends the record to the ledger file with fsync durability.
func writeLedgerRecord(ledgerFile *os.File, record RPILedgerRecord, ledgerDir string) error {
	line, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal ledger record: %w", err)
	}

	if _, err := ledgerFile.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("seek ledger end: %w", err)
	}
	if _, err := ledgerFile.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("append ledger record: %w", err)
	}
	if err := ledgerFile.Sync(); err != nil {
		return fmt.Errorf("fsync ledger: %w", err)
	}
	return syncDirectory(ledgerDir)
}

// LoadRPILedgerRecords loads all ledger events in append order.
func LoadRPILedgerRecords(rootDir string) ([]RPILedgerRecord, error) {
	return loadRPILedgerRecordsFromPath(RPILedgerPath(rootDir))
}

// VerifyRPILedger verifies the on-disk ledger chain end-to-end.
func VerifyRPILedger(rootDir string) error {
	records, err := LoadRPILedgerRecords(rootDir)
	if err != nil {
		return err
	}
	return rpi.VerifyLedgerChain(records)
}

// appendRPILedgerEvent appends a single run event to the on-disk ledger.
func appendRPILedgerEvent(rootDir string, event rpiLedgerEvent) (rpiLedgerRecord, error) {
	return AppendRPILedgerRecord(rootDir, RPILedgerAppendInput{
		RunID:   event.RunID,
		Phase:   event.Phase,
		Action:  event.Action,
		Details: event.Details,
	})
}

// verifyRPILedger verifies on-disk ledger integrity and reports the first
// broken index without failing the call for chain mismatches.
func verifyRPILedger(rootDir string) (rpiLedgerVerifyResult, error) {
	records, err := LoadRPILedgerRecords(rootDir)
	if err != nil {
		return rpiLedgerVerifyResult{}, err
	}
	result := rpi.VerifyLedgerDetailed(records)
	return result, nil
}

// materializeRPIRunCache refreshes the mutable run cache for one run.
func materializeRPIRunCache(rootDir, runID string) error {
	return MaterializeRPIRunCache(rootDir, runID)
}

// VerifyRPILedgerChain verifies hashes and prev-hash links for all records.
func VerifyRPILedgerChain(records []RPILedgerRecord) error {
	return rpi.VerifyLedgerChain(records)
}

// MaterializeRPIRunCache writes .agents/rpi/runs/<run_id>.json for one run.
func MaterializeRPIRunCache(rootDir, runID string) error {
	if err := rpi.ValidateRunID(runID); err != nil {
		return err
	}

	records, err := LoadRPILedgerRecords(rootDir)
	if err != nil {
		return err
	}
	if err := rpi.VerifyLedgerChain(records); err != nil {
		return err
	}

	latest, count := rpi.FilterRunRecords(records, runID)
	if count == 0 {
		return os.ErrNotExist
	}

	return writeRunCache(rootDir, runID, latest, count)
}

func writeRunCache(rootDir, runID string, latest RPILedgerRecord, count int) error {
	cache := rpi.BuildRunCache(runID, latest, count)
	cacheBytes, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal run cache: %w", err)
	}
	cacheBytes = append(cacheBytes, '\n')

	cachePath := filepath.Join(rootDir, rpiRunCacheRelativeDir, runID+".json")
	cacheDir := filepath.Dir(cachePath)
	if err := os.MkdirAll(cacheDir, 0750); err != nil {
		return fmt.Errorf("create run cache dir: %w", err)
	}
	return writeFileAtomic(cachePath, cacheBytes, 0600)
}

func readLastLedgerHash(file *os.File) (string, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("seek ledger start: %w", err)
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	lastHash := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		record, err := rpi.ParseLedgerLine(line)
		if err != nil {
			return "", fmt.Errorf("decode existing ledger record: %w", err)
		}
		lastHash = record.Hash
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan ledger: %w", err)
	}
	return lastHash, nil
}

func loadRPILedgerRecordsFromPath(path string) ([]RPILedgerRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open ledger: %w", err)
	}
	defer func() { _ = file.Close() }()

	var records []RPILedgerRecord
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		record, err := rpi.ParseLedgerLine(line)
		if err != nil {
			return nil, fmt.Errorf("decode ledger line %d: %w", lineNum, err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan ledger: %w", err)
	}
	return records, nil
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tempFile, err := os.CreateTemp(dir, ".tmp-*.json")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	cleanup := true
	defer func() {
		_ = tempFile.Close()
		if cleanup {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := tempFile.Write(data); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tempFile.Chmod(mode); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("fsync temp file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	if err := syncDirectory(dir); err != nil {
		return err
	}

	cleanup = false
	return nil
}

// Thin wrappers for internal functions used by tests and other cmd/ao code.

func validateAppendInput(input RPILedgerAppendInput) error {
	return rpi.ValidateAppendInput(input)
}

func validateRunID(runID string) error {
	return rpi.ValidateRunID(runID)
}

func validateLedgerRecord(record RPILedgerRecord) error {
	return rpi.ValidateLedgerRecord(record)
}

func validateLedgerRequiredFields(record RPILedgerRecord) error {
	// Delegate to ValidateLedgerRecord which includes required fields check.
	// Tests call this directly, so we need a thin wrapper that matches the old behavior.
	fields := []struct {
		value string
		name  string
	}{
		{record.EventID, "event_id"},
		{record.RunID, "run_id"},
		{record.Phase, "phase"},
		{record.Action, "action"},
		{record.TS, "ts"},
		{record.PayloadHash, "payload_hash"},
		{record.Hash, "hash"},
	}
	for _, f := range fields {
		if strings.TrimSpace(f.value) == "" {
			return fmt.Errorf("%s is required", f.name)
		}
	}
	return nil
}

func validateLedgerTimestamp(ts string) error {
	return rpi.ValidateLedgerTimestamp(ts)
}

func computeLedgerHashes(record RPILedgerRecord) (string, string, error) {
	return rpi.ComputeLedgerHashes(record)
}

func normalizeDetails(details any) (json.RawMessage, error) {
	return rpi.NormalizeDetails(details)
}

func roundTripJSON(data []byte) (json.RawMessage, error) {
	// Round-trip through NormalizeDetails with []byte input.
	return rpi.NormalizeDetails(data)
}

func filterRunRecords(records []RPILedgerRecord, runID string) (RPILedgerRecord, int) {
	return rpi.FilterRunRecords(records, runID)
}

func newRPILedgerEventID() string {
	return rpi.NewLedgerEventID()
}

func hashHex(data []byte) string {
	return rpi.HashHex(data)
}

func syncDirectory(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open directory for fsync: %w", err)
	}
	defer func() { _ = f.Close() }()
	if err := f.Sync(); err != nil {
		return nil // silently ignore directory sync errors
	}
	return nil
}
