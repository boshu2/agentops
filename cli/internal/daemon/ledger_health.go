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
	out := LedgerHealth{LedgerMaxBytes: s.ledgerMaxBytes}

	if err := s.populateLedgerSize(&out); err != nil {
		return LedgerHealth{}, err
	}
	s.populateSnapshotFacts(&out, now)
	if err := s.populateArchiveFacts(&out); err != nil {
		return LedgerHealth{}, err
	}
	out.WarnReasons = append(out.WarnReasons, ledgerWarnReasons(out, thresholds)...)
	return out, nil
}

func (s *Store) populateLedgerSize(out *LedgerHealth) error {
	info, err := os.Stat(s.LedgerPath())
	switch {
	case err == nil:
		out.LedgerSizeBytes = info.Size()
	case !os.IsNotExist(err):
		return fmt.Errorf("stat ledger: %w", err)
	}
	if out.LedgerMaxBytes > 0 {
		out.LedgerSizeRatio = float64(out.LedgerSizeBytes) / float64(out.LedgerMaxBytes)
	}
	return nil
}

func (s *Store) populateSnapshotFacts(out *LedgerHealth, now time.Time) {
	snapshot, snapshotPath, snapErr := s.LoadLatestProjectionSnapshot()
	if snapErr != nil && !os.IsNotExist(snapErr) {
		out.WarnReasons = append(out.WarnReasons, "snapshot load error: "+snapErr.Error())
		return
	}
	if snapshotPath == "" || snapErr != nil {
		return
	}
	out.HasSnapshot = true
	out.LatestSnapshotPath = snapshotPath
	if snapshot.RebuiltAt == "" {
		return
	}
	if rebuiltAt, err := time.Parse(time.RFC3339Nano, snapshot.RebuiltAt); err == nil {
		out.LatestSnapshotAge = now.Sub(rebuiltAt)
	}
}

func (s *Store) populateArchiveFacts(out *LedgerHealth) error {
	archives, err := s.LedgerArchivePaths()
	if err != nil {
		return fmt.Errorf("list archives: %w", err)
	}
	out.ArchiveCount = len(archives)
	if len(archives) > 0 {
		out.OldestArchiveTime = parseArchiveTimestamp(archives[0])
	}
	return nil
}

func ledgerWarnReasons(h LedgerHealth, thresholds LedgerHealthThresholds) []string {
	var reasons []string
	if h.LedgerMaxBytes > 0 && h.LedgerSizeRatio >= thresholds.LedgerSizeWarnRatio {
		reasons = append(reasons,
			fmt.Sprintf("ledger %d/%d bytes (%.0f%% of cap)",
				h.LedgerSizeBytes, h.LedgerMaxBytes, h.LedgerSizeRatio*100))
	}
	if thresholds.SnapshotMaxAge > 0 && h.HasSnapshot && h.LatestSnapshotAge >= thresholds.SnapshotMaxAge {
		reasons = append(reasons,
			fmt.Sprintf("snapshot age %s (>= %s)", h.LatestSnapshotAge.Round(time.Second), thresholds.SnapshotMaxAge))
	}
	if thresholds.ArchiveCountWarn > 0 && h.ArchiveCount >= thresholds.ArchiveCountWarn {
		reasons = append(reasons,
			fmt.Sprintf("archives=%d (>= %d)", h.ArchiveCount, thresholds.ArchiveCountWarn))
	}
	return reasons
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
