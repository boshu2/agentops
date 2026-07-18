package doctor

// Workspace stale-queue GC: fm-ws-stale-queue-dirs (auto-fixable).
//
// Land-queue lanes leave debris directories at the top level of `.agents/`:
// explicitly staled dirs (`*.stale-<timestamp>`) and retry-chain dirs
// (`*-retry<N>`). Once their newest content is older than the workspace GC
// TTL (workspaceGCTTL: 14d default, AO_DOCTOR_WS_TTL_DAYS override) they are
// dead weight. The detector flags them in ONE summarizing finding; the fixer
// quarantines each expired dir by renaming it to
// <RunDir>/quarantine/workspace/<dirName> (never a delete).
//
// The detector is PURE (stat + readdir via workspaceDirInventory). The fixer
// re-scans at fix time — it never trusts stale findings — and each move flows
// through the mutation chokepoint discipline via workspaceQuarantineRename
// (see that helper for why Mutate itself cannot take a directory path).

import (
	"fmt"
	"os"
	"time"
)

// workspaceStaleQueueDirsID is the shared detector/fixer ID for workspace
// stale-queue GC.
const workspaceStaleQueueDirsID = "fm-ws-stale-queue-dirs"

func init() {
	RegisterDetector(workspaceStaleQueueDirsDetector{})
	RegisterFixer(workspaceStaleQueueDirsFixer{})
}

// workspaceExpiredStaleDirs returns the top-level directories under base whose
// NAME classifies as stale/retry debris (isWorkspaceStaleDirName) AND whose
// newest content mtime is older than the GC TTL relative to now. An empty
// matched directory (no regular files, zero NewestMTime) counts as expired:
// there is nothing fresh in it by definition. Non-matching or young
// directories are simply not selected. Pure: stat + readdir only.
func workspaceExpiredStaleDirs(base string, now time.Time) ([]workspaceDirInfo, error) {
	inv, err := workspaceDirInventory(base)
	if err != nil {
		return nil, err
	}
	cutoff := now.Add(-workspaceGCTTL())
	var out []workspaceDirInfo
	for _, d := range inv {
		if !isWorkspaceStaleDirName(d.Name) {
			continue
		}
		if d.NewestMTime.IsZero() || d.NewestMTime.Before(cutoff) {
			out = append(out, d)
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Detector
// ---------------------------------------------------------------------------

// workspaceStaleQueueDirsDetector flags expired stale/retry land-queue debris
// directories at the top level of `.agents/`.
type workspaceStaleQueueDirsDetector struct{}

func (workspaceStaleQueueDirsDetector) ID() string           { return workspaceStaleQueueDirsID }
func (workspaceStaleQueueDirsDetector) Subsystem() string    { return subsystemWorkspace }
func (workspaceStaleQueueDirsDetector) Severity() string     { return "P3" }
func (workspaceStaleQueueDirsDetector) EstimatedCostMS() int { return 10 }
func (workspaceStaleQueueDirsDetector) OnlineRequired() bool { return false }
func (workspaceStaleQueueDirsDetector) QuickPath() bool      { return true }
func (workspaceStaleQueueDirsDetector) Describe() string {
	return "expired stale/retry land-queue dirs under .agents awaiting quarantine"
}

func (d workspaceStaleQueueDirsDetector) Detect(env *DetectEnv) ([]Finding, error) {
	base := workspaceAgentsDir(env)
	if _, err := os.Stat(base); err != nil {
		return nil, nil
	}
	expired, err := workspaceExpiredStaleDirs(base, time.Now())
	if err != nil {
		return nil, fmt.Errorf("doctor: inventory %s: %w", base, err)
	}
	if len(expired) == 0 {
		return nil, nil
	}
	ttlDays := int(workspaceGCTTL() / day)
	return []Finding{{
		ID:         d.ID(),
		Severity:   d.Severity(),
		Subsystem:  d.Subsystem(),
		Title:      fmt.Sprintf("%d stale/retry workspace dir(s) past the %dd GC TTL", len(expired), ttlDays),
		Confidence: 1.0,
		Evidence: Evidence{
			File:  ".agents",
			Query: fmt.Sprintf(`find .agents -mindepth 1 -maxdepth 1 -type d \( -name '*.stale-*' -o -name '*-retry[0-9]*' \) -mtime +%d`, ttlDays),
		},
		Remediation: Remediation{
			Command:          "ao doctor --fix --only " + d.ID(),
			ExplainCommand:   "ao doctor explain " + d.ID(),
			AutoFixable:      true,
			EstimatedActions: len(expired),
		},
	}}, nil
}

// ---------------------------------------------------------------------------
// Fixer
// ---------------------------------------------------------------------------

// workspaceStaleQueueDirsFixer quarantines each expired stale/retry directory
// via a chokepoint-disciplined Rename into <RunDir>/quarantine/workspace/. It
// re-scans at fix time so a stale finding can never select a directory that
// has since gone young, been renamed, or disappeared.
type workspaceStaleQueueDirsFixer struct{}

func (workspaceStaleQueueDirsFixer) ID() string { return workspaceStaleQueueDirsID }
func (workspaceStaleQueueDirsFixer) Preconditions() []string {
	return []string{
		".agents exists and is a directory",
		"directory name matches the stale/retry debris pattern",
		"newest content mtime is older than the workspace GC TTL (an empty dir counts as expired)",
	}
}
func (workspaceStaleQueueDirsFixer) WritesTo() []string { return []string{".agents"} }
func (workspaceStaleQueueDirsFixer) Ops() []string      { return []string{"Rename"} }
func (workspaceStaleQueueDirsFixer) Reversible() bool   { return true }
func (workspaceStaleQueueDirsFixer) Idempotent() bool   { return true }
func (workspaceStaleQueueDirsFixer) AutoFixable() bool  { return true }

func (f workspaceStaleQueueDirsFixer) Fix(ctx *MutateContext, env *DetectEnv, _ []Finding) (FixResult, error) {
	res := FixResult{FixerID: f.ID(), FindingIDs: []string{f.ID()}}
	base := workspaceAgentsDir(env)
	if info, err := os.Stat(base); err != nil || !info.IsDir() {
		// Nothing to GC; the finding (if any) is stale.
		res.Fixed = true
		return res, nil
	}
	expired, err := workspaceExpiredStaleDirs(base, time.Now())
	if err != nil {
		res.Err = fmt.Errorf("doctor: %s: re-scan %s: %w", f.ID(), base, err)
		return res, res.Err
	}
	for _, d := range expired {
		dest := workspaceQuarantineDest(ctx, d.Name)
		if err := workspaceQuarantineRename(ctx, base, d.Name, dest); err != nil {
			res.Err = fmt.Errorf("doctor: %s: quarantine %s: %w", f.ID(), d.Name, err)
			return res, res.Err
		}
		res.ActionsTaken++
	}
	res.Fixed = true
	return res, nil
}

// workspaceQuarantineRename moves the workspace DIRECTORY <base>/<name> to
// dest through the shared workspace directory-rename chokepoint adapter
// (workspaceDirRename), with the bare-path-element structural guard applied
// by workspaceQuarantineDirByName.
func workspaceQuarantineRename(ctx *MutateContext, base, name, dest string) error {
	return workspaceQuarantineDirByName(ctx, base, name, dest, nil)
}
