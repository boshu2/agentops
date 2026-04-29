package openclaw

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConsumerSnapshotBackwardCompatibilityFixtures(t *testing.T) {
	fixtures := []string{
		"consumer_snapshot_v1.json",
		"consumer_snapshot_v1_legacy_minimal.json",
	}
	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("testdata", fixture))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			snapshot, err := ParseConsumerSnapshot(raw)
			if err != nil {
				t.Fatalf("parse fixture: %v", err)
			}
			if snapshot.SchemaVersion != ConsumerSnapshotSchemaVersion {
				t.Fatalf("schema_version = %d", snapshot.SchemaVersion)
			}
			if snapshot.SnapshotID == "" || snapshot.Source.Ledger == "" {
				t.Fatalf("missing snapshot identity/source: %#v", snapshot)
			}
		})
	}
}

func TestLegacyMinimalSnapshotCanBeRewrittenByCurrentStore(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "consumer_snapshot_v1_legacy_minimal.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	snapshot, err := ParseConsumerSnapshot(raw)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	store := NewSnapshotStore(t.TempDir())
	if err := store.Write(snapshot); err != nil {
		t.Fatalf("write legacy fixture through current store: %v", err)
	}
	roundTrip, err := store.Read(snapshot.SnapshotID)
	if err != nil {
		t.Fatalf("read rewritten fixture: %v", err)
	}
	if len(roundTrip.Resources.Runs) != 1 || roundTrip.Resources.Runs[0].ResourceID != "job-rpi-legacy" {
		t.Fatalf("round-trip resources = %#v", roundTrip.Resources)
	}
}
