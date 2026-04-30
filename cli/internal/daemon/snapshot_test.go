package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestProjectionSnapshotRoundTrip(t *testing.T) {
	store := NewStore(t.TempDir())

	original := ProjectionSet{
		SchemaVersion: ProjectionSchemaVersion,
		RebuiltAt:     "2026-04-30T12:00:00Z",
		SourceLedger:  ".agents/daemon/ledger.jsonl",
		LastEventID:   "evt-42",
		Manifests: map[ProjectionName]ProjectionManifest{
			ProjectionDaemonStatus: {
				SchemaVersion: ProjectionSchemaVersion,
				Projection:    ProjectionDaemonStatus,
				SourceLedger:  ".agents/daemon/ledger.jsonl",
				Status:        ProjectionStatusCurrent,
				RebuiltAt:     "2026-04-30T12:00:00Z",
			},
		},
		Jobs: []JobProjection{
			{
				JobID:     "job-1",
				JobType:   JobTypeOpenClawSnapshot,
				RequestID: "req-1",
				Status:    JobStatusCompleted,
				Artifacts: map[string]string{
					"executor_policy": "fake",
					"snapshot_status": "validated",
				},
			},
		},
	}

	path, err := store.WriteProjectionSnapshot(original)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !strings.HasPrefix(filepath.Base(path), ProjectionSnapshotPrefix) {
		t.Fatalf("written path %q missing prefix %q", filepath.Base(path), ProjectionSnapshotPrefix)
	}
	if !strings.HasSuffix(path, ProjectionSnapshotSuffix) {
		t.Fatalf("written path %q missing suffix %q", path, ProjectionSnapshotSuffix)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat written snapshot: %v", err)
	}

	loaded, loadedPath, err := store.LoadLatestProjectionSnapshot()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loadedPath != path {
		t.Fatalf("loaded path = %q, want %q", loadedPath, path)
	}
	if !reflect.DeepEqual(loaded, original) {
		oj, _ := json.MarshalIndent(original, "", "  ")
		lj, _ := json.MarshalIndent(loaded, "", "  ")
		t.Fatalf("round-trip mismatch\noriginal:\n%s\nloaded:\n%s", oj, lj)
	}
}

func TestLoadLatestProjectionSnapshotPicksMostRecent(t *testing.T) {
	store := NewStore(t.TempDir())

	for i, eventID := range []string{"evt-1", "evt-2", "evt-3"} {
		set := ProjectionSet{
			SchemaVersion: ProjectionSchemaVersion,
			RebuiltAt:     "2026-04-30T12:00:00Z",
			SourceLedger:  ".agents/daemon/ledger.jsonl",
			LastEventID:   eventID,
			Manifests:     map[ProjectionName]ProjectionManifest{},
		}
		if _, err := store.WriteProjectionSnapshot(set); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
		// Sleep a millisecond so the filename timestamps are strictly increasing
		// and the chronological sort behaves deterministically. Snapshot writes
		// embed RFC3339 nanos so the resolution is well below a millisecond.
		time.Sleep(time.Millisecond)
	}

	loaded, _, err := store.LoadLatestProjectionSnapshot()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.LastEventID != "evt-3" {
		t.Fatalf("loaded LastEventID = %q, want %q", loaded.LastEventID, "evt-3")
	}

	all, err := store.ListProjectionSnapshots()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("list returned %d snapshots, want 3", len(all))
	}
}

func TestLoadLatestProjectionSnapshotMissing(t *testing.T) {
	store := NewStore(t.TempDir())

	loaded, path, err := store.LoadLatestProjectionSnapshot()
	if err != nil {
		t.Fatalf("expected nil err for missing dir, got %v", err)
	}
	if path != "" {
		t.Fatalf("expected empty path for missing snapshot, got %q", path)
	}
	if loaded.SchemaVersion != 0 {
		t.Fatalf("expected zero ProjectionSet, got SchemaVersion=%d", loaded.SchemaVersion)
	}

	// And: an existing-but-empty dir should also be a clean no-op (not error).
	if err := os.MkdirAll(store.ProjectionSnapshotDir(), 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	loaded, path, err = store.LoadLatestProjectionSnapshot()
	if err != nil {
		t.Fatalf("expected nil err for empty dir, got %v", err)
	}
	if path != "" || loaded.SchemaVersion != 0 {
		t.Fatalf("expected zero output for empty dir, got path=%q schema=%d", path, loaded.SchemaVersion)
	}
}

// TestWriteProjectionSnapshotRejectsZeroSchema guards against silently writing
// an "empty" snapshot that would later fail the load-time schema check.
func TestWriteProjectionSnapshotRejectsZeroSchema(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.WriteProjectionSnapshot(ProjectionSet{}); err == nil {
		t.Fatal("expected error writing zero-schema snapshot")
	}
}

// TestLoadLatestProjectionSnapshotRejectsSchemaVersionMismatch guards future
// schema bumps: an old-version snapshot must not be silently consumed.
func TestLoadLatestProjectionSnapshotRejectsSchemaVersionMismatch(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := os.MkdirAll(store.ProjectionSnapshotDir(), 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Hand-write a schema_version=999 snapshot (out of band).
	bogus := map[string]any{
		"schema_version": 999,
		"rebuilt_at":     "2026-04-30T12:00:00Z",
		"source_ledger":  ".agents/daemon/ledger.jsonl",
	}
	data, _ := json.MarshalIndent(bogus, "", "  ")
	path := filepath.Join(store.ProjectionSnapshotDir(), ProjectionSnapshotPrefix+"99999999T999999.999999999Z"+ProjectionSnapshotSuffix)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write bogus: %v", err)
	}

	_, _, err := store.LoadLatestProjectionSnapshot()
	if err == nil {
		t.Fatal("expected error for schema_version mismatch")
	}
	if !strings.Contains(err.Error(), "schema_version") {
		t.Fatalf("error %q missing 'schema_version'", err)
	}
}

// TestProjectionSnapshotIgnoresTmpFiles ensures a crashed write's leftover
// .tmp file does not get loaded as a snapshot.
func TestProjectionSnapshotIgnoresTmpFiles(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := os.MkdirAll(store.ProjectionSnapshotDir(), 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	tmpPath := filepath.Join(store.ProjectionSnapshotDir(), ProjectionSnapshotPrefix+"20260430T120000.000000000Z"+ProjectionSnapshotSuffix+".tmp")
	if err := os.WriteFile(tmpPath, []byte("partial-not-json"), 0o600); err != nil {
		t.Fatalf("write tmp: %v", err)
	}

	paths, err := store.ListProjectionSnapshots()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, p := range paths {
		if strings.HasSuffix(p, ".tmp") {
			t.Fatalf("list returned .tmp file: %s", p)
		}
	}

	_, path, err := store.LoadLatestProjectionSnapshot()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if path != "" {
		t.Fatalf("load returned a path for .tmp-only dir: %q", path)
	}
}
