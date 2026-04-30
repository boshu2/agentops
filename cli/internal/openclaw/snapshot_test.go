package openclaw

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSnapshotStoreWritesLatestAndVersion(t *testing.T) {
	store := NewSnapshotStore(t.TempDir())
	snapshot := mustBuildSnapshot(t)

	if err := store.Write(snapshot); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
	versionPath, err := store.VersionPath(snapshot.SnapshotID)
	if err != nil {
		t.Fatalf("version path: %v", err)
	}
	if _, err := os.Stat(versionPath); err != nil {
		t.Fatalf("version snapshot missing: %v", err)
	}
	if _, err := os.Stat(store.LatestPath()); err != nil {
		t.Fatalf("latest snapshot missing: %v", err)
	}

	latest, err := store.ReadLatest()
	if err != nil {
		t.Fatalf("read latest: %v", err)
	}
	if latest.SnapshotID != snapshot.SnapshotID {
		t.Fatalf("latest snapshot_id = %q, want %q", latest.SnapshotID, snapshot.SnapshotID)
	}
	versioned, err := store.Read(snapshot.SnapshotID)
	if err != nil {
		t.Fatalf("read version: %v", err)
	}
	if versioned.Source.LastEventID != "evt-store-1" {
		t.Fatalf("version source event = %q", versioned.Source.LastEventID)
	}
}

func TestSnapshotProjectionRebuildsFromInput(t *testing.T) {
	store := NewSnapshotStore(t.TempDir())
	generatedAt := time.Date(2026, 4, 28, 21, 0, 0, 0, time.UTC)
	snapshot, err := store.WriteRebuilt(ProjectionInput{
		GeneratedAt: generatedAt,
		Source: SnapshotSource{
			Ledger:      ".agents/daemon/ledger.jsonl",
			LastEventID: "evt-rebuild-1",
		},
		Status: SnapshotStatusDegraded,
		Runs: []ResourceSummary{{
			RunID:  "rpi-run-1",
			JobID:  "job-rpi-1",
			Status: "completed",
		}},
		Jobs: []ResourceSummary{{
			JobID:  "job-rpi-1",
			Status: "completed",
		}},
		Wiki: []ResourceSummary{{
			JobID:  "job-wiki-1",
			Status: "queued",
		}},
	})
	if err != nil {
		t.Fatalf("write rebuilt snapshot: %v", err)
	}
	if snapshot.SnapshotID != "snap_evt-rebuild-1" {
		t.Fatalf("snapshot_id = %q", snapshot.SnapshotID)
	}
	if snapshot.GeneratedAt != generatedAt.Format(time.RFC3339Nano) {
		t.Fatalf("generated_at = %q", snapshot.GeneratedAt)
	}
	if snapshot.Status != SnapshotStatusDegraded {
		t.Fatalf("status = %q", snapshot.Status)
	}
	if snapshot.Resources.Runs[0].ResourceKind != ResourceKindRun || snapshot.Resources.Runs[0].ResourceID != "rpi-run-1" {
		t.Fatalf("run resource = %#v", snapshot.Resources.Runs[0])
	}
	if snapshot.Resources.Jobs[0].ResourceKind != ResourceKindJob || snapshot.Resources.Jobs[0].ResourceID != "job-rpi-1" {
		t.Fatalf("job resource = %#v", snapshot.Resources.Jobs[0])
	}
	if snapshot.Resources.Wiki[0].ResourceKind != ResourceKindWiki || snapshot.Resources.Wiki[0].ResourceID != "job-wiki-1" {
		t.Fatalf("wiki resource = %#v", snapshot.Resources.Wiki[0])
	}
}

func TestSnapshotReadRejectsUnsupportedVersion(t *testing.T) {
	store := NewSnapshotStore(t.TempDir())
	if err := os.MkdirAll(store.Dir(), 0700); err != nil {
		t.Fatalf("mkdir snapshot dir: %v", err)
	}
	raw := []byte(`{"schema_version":2,"snapshot_id":"snap_future","generated_at":"2026-04-28T21:00:00Z","source":{"ledger":".agents/daemon/ledger.jsonl"},"status":"current","resources":{"runs":[],"jobs":[],"wiki":[]}}`)
	if err := os.WriteFile(store.LatestPath(), raw, 0600); err != nil {
		t.Fatalf("write future latest: %v", err)
	}
	_, err := store.ReadLatest()
	if !errors.Is(err, ErrUnsupportedSnapshotVersion) {
		t.Fatalf("want unsupported version, got %v", err)
	}
}

func TestSnapshotStoreRejectsUnsafeSnapshotID(t *testing.T) {
	store := NewSnapshotStore(t.TempDir())
	snapshot := mustBuildSnapshot(t)
	snapshot.SnapshotID = "../escape"
	if err := store.Write(snapshot); err == nil {
		t.Fatal("unsafe snapshot_id was accepted")
	}
	if _, err := store.VersionPath("../escape"); err == nil {
		t.Fatal("unsafe version path was accepted")
	}
}

func TestSnapshot_PermsPreservedAt0o600(t *testing.T) {
	store := NewSnapshotStore(t.TempDir())
	snapshot := mustBuildSnapshot(t)
	if err := store.Write(snapshot); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
	versionPath, err := store.VersionPath(snapshot.SnapshotID)
	if err != nil {
		t.Fatalf("version path: %v", err)
	}
	for _, path := range []string{versionPath, store.LatestPath()} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("perm %s = %o, want 0o600", path, got)
		}
	}
}

func TestSnapshotStoreReadMissingLatest(t *testing.T) {
	store := NewSnapshotStore(t.TempDir())
	_, err := store.ReadLatest()
	if err == nil {
		t.Fatal("missing latest read succeeded")
	}
	if !os.IsNotExist(errors.Unwrap(err)) {
		t.Fatalf("want not-exist wrapper, got %v", err)
	}
}

func mustBuildSnapshot(t *testing.T) ConsumerSnapshot {
	t.Helper()
	snapshot, err := BuildConsumerSnapshot(ProjectionInput{
		GeneratedAt: time.Date(2026, 4, 28, 21, 0, 0, 0, time.UTC),
		Source: SnapshotSource{
			Ledger:      filepath.ToSlash(filepath.Join(".agents", "daemon", "ledger.jsonl")),
			LastEventID: "evt-store-1",
		},
		Runs: []ResourceSummary{{
			ResourceID:   "run-rpi-1",
			ResourceKind: ResourceKindRun,
			RunID:        "rpi-run-1",
			JobID:        "job-rpi-1",
			Status:       "completed",
		}},
		Jobs: []ResourceSummary{{
			ResourceID:   "job-rpi-1",
			ResourceKind: ResourceKindJob,
			JobID:        "job-rpi-1",
			Status:       "completed",
		}},
		Wiki: []ResourceSummary{{
			ResourceID:   "wiki-job-wiki-1",
			ResourceKind: ResourceKindWiki,
			JobID:        "job-wiki-1",
			Status:       "queued",
		}},
	})
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	return snapshot
}
