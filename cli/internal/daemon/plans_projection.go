package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// DaemonPlansProjectionSchemaVersion is the schema version for the
// daemon-side plans manifest projection.
const DaemonPlansProjectionSchemaVersion = 1

// PlansProjectionEntry is one entry in the daemon-rebuilt plans manifest.
// Source-of-truth fields come from bd via the executor's BdSource.
type PlansProjectionEntry struct {
	BeadsID   string    `json:"beads_id"`
	Title     string    `json:"title,omitempty"`
	Status    string    `json:"status,omitempty"`
	Priority  string    `json:"priority,omitempty"`
	IssueType string    `json:"issue_type,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	Checksum  string    `json:"checksum,omitempty"`
}

// DaemonPlansProjection is the daemon-side wrapper around plans manifest
// state. Mirrors the DaemonRPIRegistryProjection shape (rpi_registry.go:9):
// Entries + LastEventID + a SchemaVersion stamp.
type DaemonPlansProjection struct {
	SchemaVersion int                    `json:"schema_version"`
	ProjectID     string                 `json:"project_id,omitempty"`
	IssuePrefix   string                 `json:"issue_prefix,omitempty"`
	Entries       []PlansProjectionEntry `json:"entries"`
	LastEventID   string                 `json:"last_event_id,omitempty"`
	RebuiltAt     string                 `json:"rebuilt_at,omitempty"`
}

// RebuildDaemonPlansProjection folds the ledger event slice into the latest
// plans projection state. Currently the projection is fully rebuilt by the
// executor on each subscription tick; ledger events are advisory (last-event
// cursor + degraded-flag carry). atom-2 does NOT replay-build entries from
// events because plans.projection is a pull-from-bd projection, not an
// event-sourced one — the source of truth is bd, not the daemon ledger.
func RebuildDaemonPlansProjection(events []LedgerEvent) (DaemonPlansProjection, error) {
	projection := DaemonPlansProjection{
		SchemaVersion: DaemonPlansProjectionSchemaVersion,
		Entries:       []PlansProjectionEntry{},
	}
	for _, event := range events {
		if err := ValidateLedgerEvent(event); err != nil {
			return DaemonPlansProjection{}, err
		}
		projection.LastEventID = event.EventID
	}
	return projection, nil
}

// WriteDaemonPlansProjection writes the plans manifest snapshot atomically:
// the entries are serialised to a JSONL file under root, written to a tmp
// file in the same directory, then renamed to the final path. Concurrent
// writers in the same OutputDir are protected externally via the
// manifest.lock file lock added by atom-3 (G3); the write call here is
// single-writer-safe under that contract.
func WriteDaemonPlansProjection(root string, projection DaemonPlansProjection) (string, error) {
	if root == "" {
		return "", fmt.Errorf("plans projection: root is required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("plans projection mkdir %q: %w", root, err)
	}
	finalPath := filepath.Join(root, "manifest.jsonl")
	tmp, err := os.CreateTemp(root, ".manifest.jsonl.tmp.*")
	if err != nil {
		return "", fmt.Errorf("plans projection tmp: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}
	entries := append([]PlansProjectionEntry{}, projection.Entries...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].BeadsID < entries[j].BeadsID })
	enc := json.NewEncoder(tmp)
	for _, entry := range entries {
		if err := enc.Encode(entry); err != nil {
			cleanup()
			return "", fmt.Errorf("plans projection encode %q: %w", entry.BeadsID, err)
		}
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("plans projection close tmp: %w", err)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("plans projection rename: %w", err)
	}
	return finalPath, nil
}

// ValidateDaemonPlansProjection returns a descriptive error if the projection
// shape is malformed. Used by replay paths and tests.
func ValidateDaemonPlansProjection(projection DaemonPlansProjection) error {
	if projection.SchemaVersion != DaemonPlansProjectionSchemaVersion {
		return fmt.Errorf("plans projection schema_version = %d, want %d", projection.SchemaVersion, DaemonPlansProjectionSchemaVersion)
	}
	seen := map[string]struct{}{}
	for _, entry := range projection.Entries {
		if entry.BeadsID == "" {
			return fmt.Errorf("plans projection entry missing beads_id")
		}
		if _, dup := seen[entry.BeadsID]; dup {
			return fmt.Errorf("plans projection entry %q duplicated", entry.BeadsID)
		}
		seen[entry.BeadsID] = struct{}{}
	}
	return nil
}
