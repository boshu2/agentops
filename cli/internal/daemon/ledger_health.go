package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LedgerHealth summarizes durability state for the ao doctor surface (TB-Δ3
// Phase 2-C). Read-only — populated by Store.LedgerHealth from on-disk facts.
type LedgerHealth struct {
	LedgerSizeBytes      int64         `json:"ledger_size_bytes"`
	LedgerMaxBytes       int64         `json:"ledger_max_bytes"`
	LedgerSizeRatio      float64       `json:"ledger_size_ratio"`
	LatestSnapshotPath   string        `json:"latest_snapshot_path,omitempty"`
	LatestSnapshotAge    time.Duration `json:"latest_snapshot_age_ns"`
	HasSnapshot          bool          `json:"has_snapshot"`
	ArchiveCount         int           `json:"archive_count"`
	OldestArchiveTime    time.Time     `json:"oldest_archive_time,omitempty"`
	WarnReasons          []string      `json:"warn_reasons,omitempty"`
}

// LedgerHealthThresholds controls the WARN bands. Zero-value means "use
// LedgerHealthDefaultThresholds()". Operators may override per-call.
type LedgerHealthThresholds struct {
	// LedgerSizeWarnRatio fires WARN when ledger.jsonl size / max >= ratio.
	LedgerSizeWarnRatio float64
	// SnapshotMaxAge fires WARN when latest snapshot age >= this duration.
	// Zero disables the snapshot-age check.
	SnapshotMaxAge time.Duration
	// ArchiveCountWarn fires WARN when archive count >= this value. Zero
	// disables the check.
	ArchiveCountWarn int
}

func LedgerHealthDefaultThresholds() LedgerHealthThresholds {
	return LedgerHealthThresholds{
		LedgerSizeWarnRatio: 0.80,
		SnapshotMaxAge:      24 * time.Hour,
		ArchiveCountWarn:    20,
	}
}

// LedgerHealth gathers ledger-durability facts for the doctor surface. now
// must be supplied (caller's clock) so tests can be deterministic. thresholds
// uses LedgerHealthDefaultThresholds when its LedgerSizeWarnRatio is zero.
func (s *Store) LedgerHealth(now time.Time, thresholds LedgerHealthThresholds) (LedgerHealth, error) {
	if thresholds.LedgerSizeWarnRatio == 0 {
		thresholds = LedgerHealthDefaultThresholds()
	}
	out := LedgerHealth{
		LedgerMaxBytes: s.ledgerMaxBytes,
	}

	if info, err := os.Stat(s.LedgerPath()); err == nil {
		out.LedgerSizeBytes = info.Size()
	} else if !os.IsNotExist(err) {
		return LedgerHealth{}, fmt.Errorf("stat ledger: %w", err)
	}
	if out.LedgerMaxBytes > 0 {
		out.LedgerSizeRatio = float64(out.LedgerSizeBytes) / float64(out.LedgerMaxBytes)
	}

	snapshot, snapshotPath, snapErr := s.LoadLatestProjectionSnapshot()
	if snapErr != nil && !os.IsNotExist(snapErr) {
		// Surface as a warn reason; don't fail the whole check.
		out.WarnReasons = append(out.WarnReasons, "snapshot load error: "+snapErr.Error())
	}
	if snapshotPath != "" && snapErr == nil {
		out.HasSnapshot = true
		out.LatestSnapshotPath = snapshotPath
		if snapshot.RebuiltAt != "" {
			if rebuiltAt, err := time.Parse(time.RFC3339Nano, snapshot.RebuiltAt); err == nil {
				out.LatestSnapshotAge = now.Sub(rebuiltAt)
			}
		}
	}

	archives, err := s.LedgerArchivePaths()
	if err != nil {
		return LedgerHealth{}, fmt.Errorf("list archives: %w", err)
	}
	out.ArchiveCount = len(archives)
	if len(archives) > 0 {
		out.OldestArchiveTime = parseArchiveTimestamp(archives[0])
	}

	if out.LedgerMaxBytes > 0 && out.LedgerSizeRatio >= thresholds.LedgerSizeWarnRatio {
		out.WarnReasons = append(out.WarnReasons,
			fmt.Sprintf("ledger %d/%d bytes (%.0f%% of cap)",
				out.LedgerSizeBytes, out.LedgerMaxBytes, out.LedgerSizeRatio*100))
	}
	if thresholds.SnapshotMaxAge > 0 && out.HasSnapshot && out.LatestSnapshotAge >= thresholds.SnapshotMaxAge {
		out.WarnReasons = append(out.WarnReasons,
			fmt.Sprintf("snapshot age %s (>= %s)", out.LatestSnapshotAge.Round(time.Second), thresholds.SnapshotMaxAge))
	}
	if thresholds.ArchiveCountWarn > 0 && out.ArchiveCount >= thresholds.ArchiveCountWarn {
		out.WarnReasons = append(out.WarnReasons,
			fmt.Sprintf("archives=%d (>= %d)", out.ArchiveCount, thresholds.ArchiveCountWarn))
	}
	return out, nil
}

// parseArchiveTimestamp extracts the UTC timestamp encoded in
// ledger.<UTC-ts>.jsonl[.gz]. Returns zero time on parse failure (caller
// treats as "unknown" rather than erroring).
func parseArchiveTimestamp(path string) time.Time {
	name := filepath.Base(path)
	name = strings.TrimPrefix(name, ledgerArchivePrefix)
	name = strings.TrimSuffix(name, ".gz")
	name = strings.TrimSuffix(name, ledgerArchiveSuffix)
	for _, layout := range []string{"20060102T150405.000000000Z", "20060102T150405Z"} {
		if t, err := time.Parse(layout, name); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
