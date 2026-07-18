package doctor

// Workspace oversize report: fm-ws-oversize (report-only, NO fixer).
//
// The `.agents/` workspace accretes weight two ways: top-level directories
// whose transitive content balloons past useful size (session archives, land
// queues, wiki mirrors), and large regular files dumped directly at the top
// level (e.g. a 4.6M wiki-index.jsonl). Neither has a safe localized
// auto-fix — "what to archive" is a judgment call over live artifacts — so
// this detector only reports: one finding per offender, sorted by name, each
// carrying exact byte sizes and pointing at manual archive rotation. No fixer
// is registered for this ID.
//
// The detector is PURE (stat + readdir via workspaceDirInventory) but NOT a
// quick path: sizing a directory is a full transitive tree walk.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

// workspaceOversizeID is the detector ID for workspace oversize reporting.
// There is deliberately no fixer under this ID.
const workspaceOversizeID = "fm-ws-oversize"

// Workspace size-threshold configuration.
const (
	// workspaceSizeEnvVar overrides the top-level directory size threshold,
	// in whole MiB.
	workspaceSizeEnvVar = "AO_DOCTOR_WS_SIZE_MB"
	// workspaceSizeDefaultMiB is the default directory size threshold in MiB.
	workspaceSizeDefaultMiB = 25
	// mib is one mebibyte in bytes.
	mib = 1024 * 1024
	// workspaceLooseFileThresholdBytes is the FIXED threshold (2 MiB) for a
	// regular file sitting directly under `.agents/` (not inside a subdir).
	// Deliberately not env-configurable: a multi-megabyte loose file at the
	// workspace top level is anomalous regardless of how much bulk the
	// operator tolerates inside subdirectories.
	workspaceLooseFileThresholdBytes int64 = 2 * mib
)

// workspaceSizeThreshold returns the top-level directory size threshold in
// bytes. Defaults to 25 MiB; AO_DOCTOR_WS_SIZE_MB overrides with a positive
// integer MiB count, and any invalid value falls back to the default
// (mirroring workspaceGCTTL).
func workspaceSizeThreshold() int64 {
	if raw := os.Getenv(workspaceSizeEnvVar); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return int64(n) * mib
		}
	}
	return workspaceSizeDefaultMiB * mib
}

func init() {
	RegisterDetector(workspaceOversizeDetector{})
	// NO RegisterFixer: fm-ws-oversize is report-only by design.
}

// workspaceOversizeDetector flags oversize top-level `.agents/` directories
// and oversize loose files directly under `.agents/`.
type workspaceOversizeDetector struct{}

func (workspaceOversizeDetector) ID() string           { return workspaceOversizeID }
func (workspaceOversizeDetector) Subsystem() string    { return subsystemWorkspace }
func (workspaceOversizeDetector) Severity() string     { return "P4" }
func (workspaceOversizeDetector) EstimatedCostMS() int { return 250 }
func (workspaceOversizeDetector) OnlineRequired() bool { return false }
func (workspaceOversizeDetector) QuickPath() bool      { return false }
func (workspaceOversizeDetector) Describe() string {
	return "oversize .agents dirs and top-level loose files needing archive rotation"
}

func (d workspaceOversizeDetector) Detect(env *DetectEnv) ([]Finding, error) {
	base := workspaceAgentsDir(env)
	if _, err := os.Stat(base); err != nil {
		return nil, nil
	}
	inv, err := workspaceDirInventory(base)
	if err != nil {
		return nil, fmt.Errorf("doctor: inventory %s: %w", base, err)
	}
	threshold := workspaceSizeThreshold()

	var findings []Finding
	for _, dir := range inv {
		if dir.ByteSize <= threshold {
			continue
		}
		rel := filepath.Join(".agents", dir.Name)
		findings = append(findings, Finding{
			ID:        d.ID(),
			Severity:  d.Severity(),
			Subsystem: d.Subsystem(),
			Title: fmt.Sprintf("%s is %d bytes (%.1f MiB) — over the %d MiB threshold",
				rel, dir.ByteSize, float64(dir.ByteSize)/float64(mib), threshold/mib),
			Confidence: 1.0,
			Evidence: Evidence{
				File: rel,
				Query: fmt.Sprintf("du -sk %s  # %d file(s), newest mtime %s",
					rel, dir.FileCount, dir.NewestMTime.UTC().Format(time.RFC3339)),
			},
			Remediation: workspaceOversizeRemediation(d.ID(), rel),
		})
	}

	// Loose regular files directly under .agents/ (symlink-safe: ReadDir
	// reports lstat-style types, so a symlink is never IsRegular here).
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, fmt.Errorf("doctor: read %s: %w", base, err)
	}
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		fi, infoErr := e.Info()
		if infoErr != nil {
			continue
		}
		if fi.Size() <= workspaceLooseFileThresholdBytes {
			continue
		}
		rel := filepath.Join(".agents", e.Name())
		findings = append(findings, Finding{
			ID:        d.ID(),
			Severity:  d.Severity(),
			Subsystem: d.Subsystem(),
			Title: fmt.Sprintf("loose file %s is %d bytes (%.1f MiB) — over the fixed %d MiB loose-file threshold",
				rel, fi.Size(), float64(fi.Size())/float64(mib), workspaceLooseFileThresholdBytes/mib),
			Confidence: 1.0,
			Evidence: Evidence{
				File:  rel,
				Query: "ls -l " + rel,
			},
			Remediation: workspaceOversizeRemediation(d.ID(), rel),
		})
	}

	// Deterministic order: offenders (dirs and loose files interleaved)
	// sorted by their .agents-relative path.
	sort.Slice(findings, func(i, j int) bool {
		return findings[i].Evidence.File < findings[j].Evidence.File
	})
	return findings, nil
}

// workspaceOversizeRemediation is the shared report-only remediation: the
// doctor never decides what workspace bulk is disposable, so the instruction
// is a manual archive-rotation review (cf. the detect-only knowledge and
// cli_config findings, which likewise hand a human the exact next command).
func workspaceOversizeRemediation(id, rel string) Remediation {
	return Remediation{
		Command: "Review " + rel + " yourself. Archive or rotate what is no longer live " +
			"(e.g. tar czf " + rel + ".tar.gz " + rel + " && move the archive out of .agents), " +
			"then re-run: ao doctor",
		ExplainCommand: "ao doctor explain " + id,
		AutoFixable:    false,
	}
}
