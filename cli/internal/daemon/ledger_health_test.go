package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLedgerHealthReportsZeroStateForFreshStore(t *testing.T) {
	store := NewStore(t.TempDir())
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	got, err := store.LedgerHealth(now, LedgerHealthThresholds{})
	if err != nil {
		t.Fatalf("LedgerHealth: %v", err)
	}
	if got.LedgerSizeBytes != 0 {
		t.Fatalf("LedgerSizeBytes = %d, want 0", got.LedgerSizeBytes)
	}
	if got.LedgerMaxBytes != DefaultLedgerMaxBytes {
		t.Fatalf("LedgerMaxBytes = %d, want %d", got.LedgerMaxBytes, DefaultLedgerMaxBytes)
	}
	if got.HasSnapshot {
		t.Fatalf("HasSnapshot = true, want false on fresh store")
	}
	if got.ArchiveCount != 0 {
		t.Fatalf("ArchiveCount = %d, want 0", got.ArchiveCount)
	}
	if len(got.WarnReasons) != 0 {
		t.Fatalf("WarnReasons = %#v, want none on fresh store", got.WarnReasons)
	}
}

func TestLedgerHealthReportsLedgerSizeAndRatio(t *testing.T) {
	store := NewStore(t.TempDir()).WithLedgerMaxBytes(1024)
	if _, err := store.AppendLedgerEvent(mustNewProjectionTestEvent(t, "evt-001", "req-1", "job-1", EventJobAccepted, JobTypeRPIRun, 0, nil)); err != nil {
		t.Fatalf("append: %v", err)
	}
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	got, err := store.LedgerHealth(now, LedgerHealthThresholds{LedgerSizeWarnRatio: 0.001, ArchiveCountWarn: 100, SnapshotMaxAge: time.Hour})
	if err != nil {
		t.Fatalf("LedgerHealth: %v", err)
	}
	if got.LedgerSizeBytes <= 0 {
		t.Fatalf("LedgerSizeBytes = %d, want > 0", got.LedgerSizeBytes)
	}
	if got.LedgerMaxBytes != 1024 {
		t.Fatalf("LedgerMaxBytes = %d, want 1024", got.LedgerMaxBytes)
	}
	if got.LedgerSizeRatio == 0 {
		t.Fatalf("LedgerSizeRatio = 0, want > 0")
	}
	// Ratio threshold 0.001 should fire WARN with any nonzero ledger size.
	foundSizeReason := false
	for _, r := range got.WarnReasons {
		if strings.Contains(r, "of cap") {
			foundSizeReason = true
			break
		}
	}
	if !foundSizeReason {
		t.Fatalf("WarnReasons = %#v, want size warn", got.WarnReasons)
	}
}

func TestLedgerHealthReportsSnapshotAgeAndArchiveCount(t *testing.T) {
	store := NewStore(t.TempDir())
	rebuiltAt := time.Date(2026, 4, 30, 6, 0, 0, 0, time.UTC)
	now := rebuiltAt.Add(48 * time.Hour) // age = 48h
	set := ProjectionSet{
		SchemaVersion: ProjectionSchemaVersion,
		RebuiltAt:     rebuiltAt.Format(time.RFC3339Nano),
		LastEventID:   "evt-snap",
		Manifests:     map[ProjectionName]ProjectionManifest{},
	}
	if _, err := store.WriteProjectionSnapshot(set); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
	// Plant 3 archive files with valid timestamps.
	if err := os.MkdirAll(store.Dir(), 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, ts := range []string{"20260101T000000.000000000Z", "20260201T000000.000000000Z", "20260301T000000.000000000Z"} {
		path := filepath.Join(store.Dir(), "ledger."+ts+".jsonl")
		if err := os.WriteFile(path, []byte("{}\n"), 0600); err != nil {
			t.Fatalf("plant archive: %v", err)
		}
	}
	got, err := store.LedgerHealth(now, LedgerHealthThresholds{
		LedgerSizeWarnRatio: 0.99,
		SnapshotMaxAge:      24 * time.Hour,
		ArchiveCountWarn:    2,
	})
	if err != nil {
		t.Fatalf("LedgerHealth: %v", err)
	}
	if !got.HasSnapshot {
		t.Fatal("HasSnapshot = false, want true")
	}
	if got.LatestSnapshotAge < 47*time.Hour || got.LatestSnapshotAge > 49*time.Hour {
		t.Fatalf("LatestSnapshotAge = %s, want ~48h", got.LatestSnapshotAge)
	}
	if got.ArchiveCount != 3 {
		t.Fatalf("ArchiveCount = %d, want 3", got.ArchiveCount)
	}
	expectedOldest := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if !got.OldestArchiveTime.Equal(expectedOldest) {
		t.Fatalf("OldestArchiveTime = %v, want %v", got.OldestArchiveTime, expectedOldest)
	}
	foundSnapAge := false
	foundArchiveCount := false
	for _, r := range got.WarnReasons {
		if strings.Contains(r, "snapshot age") {
			foundSnapAge = true
		}
		if strings.Contains(r, "archives=3") {
			foundArchiveCount = true
		}
	}
	if !foundSnapAge {
		t.Fatalf("WarnReasons = %#v, want snapshot-age reason", got.WarnReasons)
	}
	if !foundArchiveCount {
		t.Fatalf("WarnReasons = %#v, want archive-count reason", got.WarnReasons)
	}
}

func TestLedgerHealth_ClampsNegativeAge(t *testing.T) {
	// Regression for S-24 (soc-58q5.22): when snapshot.RebuiltAt parses as a
	// future time (clock skew between writer and reader), now.Sub(rebuiltAt)
	// is negative. The exposed JSON field LatestSnapshotAge must clamp to 0
	// rather than leak a negative duration to operators / dashboards.
	store := NewStore(t.TempDir())
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	rebuiltAt := now.Add(time.Hour) // future relative to caller's clock
	set := ProjectionSet{
		SchemaVersion: ProjectionSchemaVersion,
		RebuiltAt:     rebuiltAt.Format(time.RFC3339Nano),
		LastEventID:   "evt-future",
		Manifests:     map[ProjectionName]ProjectionManifest{},
	}
	if _, err := store.WriteProjectionSnapshot(set); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
	got, err := store.LedgerHealth(now, LedgerHealthThresholds{
		LedgerSizeWarnRatio: 0.99,
		SnapshotMaxAge:      24 * time.Hour,
		ArchiveCountWarn:    100,
	})
	if err != nil {
		t.Fatalf("LedgerHealth: %v", err)
	}
	if !got.HasSnapshot {
		t.Fatal("HasSnapshot = false, want true (snapshot was written)")
	}
	if got.LatestSnapshotAge != 0 {
		t.Fatalf("LatestSnapshotAge = %s, want 0 (clock skew should clamp negative duration)", got.LatestSnapshotAge)
	}
	// Snapshot-age WARN must not fire when clamped to zero (0 < 24h).
	for _, r := range got.WarnReasons {
		if strings.Contains(r, "snapshot age") {
			t.Fatalf("WarnReasons = %#v, must not include snapshot-age warn for clamped duration", got.WarnReasons)
		}
	}
}

func TestLedgerHealthIgnoresDisabledThresholds(t *testing.T) {
	store := NewStore(t.TempDir()).WithLedgerMaxBytes(0) // rotation disabled
	if _, err := store.AppendLedgerEvent(mustNewProjectionTestEvent(t, "evt-001", "req-1", "job-1", EventJobAccepted, JobTypeRPIRun, 0, nil)); err != nil {
		t.Fatalf("append: %v", err)
	}
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	got, err := store.LedgerHealth(now, LedgerHealthThresholds{
		LedgerSizeWarnRatio: 0.5,
		SnapshotMaxAge:      0, // disabled
		ArchiveCountWarn:    0, // disabled
	})
	if err != nil {
		t.Fatalf("LedgerHealth: %v", err)
	}
	if got.LedgerMaxBytes != 0 {
		t.Fatalf("LedgerMaxBytes = %d, want 0 (rotation disabled)", got.LedgerMaxBytes)
	}
	if got.LedgerSizeRatio != 0 {
		t.Fatalf("LedgerSizeRatio = %v, want 0 with rotation disabled", got.LedgerSizeRatio)
	}
	if len(got.WarnReasons) != 0 {
		t.Fatalf("WarnReasons = %#v, want none with all thresholds disabled or rotation off", got.WarnReasons)
	}
}
