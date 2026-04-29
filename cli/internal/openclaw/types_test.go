package openclaw

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseConsumerSnapshotFixtureV1(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "consumer_snapshot_v1.json"))
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
	if snapshot.SnapshotID != "snap_evt_20260428_000003" {
		t.Fatalf("snapshot_id = %q", snapshot.SnapshotID)
	}
	if len(snapshot.Resources.Runs) != 2 {
		t.Fatalf("runs = %d, want 2", len(snapshot.Resources.Runs))
	}
	if len(snapshot.Resources.Jobs) != 3 {
		t.Fatalf("jobs = %d, want 3", len(snapshot.Resources.Jobs))
	}
	if len(snapshot.Resources.Wiki) != 1 {
		t.Fatalf("wiki = %d, want 1", len(snapshot.Resources.Wiki))
	}
	wiki := snapshot.Resources.Wiki[0]
	if wiki.ResourceKind != ResourceKindWiki || wiki.JobID != "job-wiki-1" {
		t.Fatalf("wiki resource = %#v", wiki)
	}
	if len(wiki.Provenance) != 2 {
		t.Fatalf("wiki provenance = %d, want 2", len(wiki.Provenance))
	}
}

func TestParseConsumerSnapshotRejectsUnsupportedVersion(t *testing.T) {
	raw := []byte(`{
		"schema_version": 2,
		"snapshot_id": "snap_future",
		"generated_at": "2026-04-28T21:00:00Z",
		"source": {"ledger": ".agents/daemon/ledger.jsonl"},
		"status": "current",
		"resources": {"runs": [], "jobs": [], "wiki": []}
	}`)
	_, err := ParseConsumerSnapshot(raw)
	if !errors.Is(err, ErrUnsupportedSnapshotVersion) {
		t.Fatalf("want unsupported version, got %v", err)
	}
}

func TestParseConsumerSnapshotToleratesAdditiveFields(t *testing.T) {
	raw := []byte(`{
		"schema_version": 1,
		"snapshot_id": "snap_additive",
		"generated_at": "2026-04-28T21:00:00Z",
		"source": {"ledger": ".agents/daemon/ledger.jsonl", "last_event_id": "evt-1", "extra": true},
		"status": "current",
		"resources": {"runs": [], "jobs": [], "wiki": []},
		"future_field": {"client_hint": "ignore me"}
	}`)
	if _, err := ParseConsumerSnapshot(raw); err != nil {
		t.Fatalf("parse additive fields: %v", err)
	}
}

func TestParseConsumerSnapshotValidatesResourceKinds(t *testing.T) {
	raw := []byte(`{
		"schema_version": 1,
		"snapshot_id": "snap_bad_kind",
		"generated_at": "2026-04-28T21:00:00Z",
		"source": {"ledger": ".agents/daemon/ledger.jsonl"},
		"status": "current",
		"resources": {
			"runs": [{"resource_id": "bad", "resource_kind": "wiki", "status": "queued"}],
			"jobs": [],
			"wiki": []
		}
	}`)
	_, err := ParseConsumerSnapshot(raw)
	if err == nil || !strings.Contains(err.Error(), "resource_kind") {
		t.Fatalf("want resource_kind error, got %v", err)
	}
}
