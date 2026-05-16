package doctor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// sha256Hex returns the "sha256:"-prefixed hex digest of b.
func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}

// RunID derives the deterministic run id from a target SHA and a timestamp.
// It is sha256(target_sha + ISO8601_utc_seconds)[..6] — stable to the second
// so two runs in the same second naturally collide and the caller bumps the
// seconds counter. The returned id is the short 6-hex-char suffix.
func RunID(targetSHA string, ts time.Time) string {
	iso := ts.UTC().Format("2006-01-02T15:04:05Z")
	h := sha256.Sum256([]byte(targetSHA + iso))
	return hex.EncodeToString(h[:])[:6]
}

// runDirName returns the on-disk run directory name: <ISO8601>__<runid>, where
// the ISO8601 timestamp uses dashes for the time component (filesystem-safe).
func runDirName(ts time.Time, runID string) string {
	iso := ts.UTC().Format("2006-01-02T15-04-05Z")
	return iso + "__" + runID
}

// RunArtifact owns the .doctor/runs/<run-dir>/ directory and all writers into
// it. Run-artifact writes are diagnostic metadata, not user-state mutations,
// so they do not flow through Mutate — but they are still confined to .doctor/.
type RunArtifact struct {
	RepoRoot  string
	DoctorDir string // <repo>/.doctor
	RunID     string // 6-char short id
	RunDir    string // absolute path to .doctor/runs/<ISO>__<runid>
	StartedAt time.Time
}

// NewRunArtifact creates a fresh run directory under <repoRoot>/.doctor/runs/,
// including backups/ and quarantine/ subdirs, refreshes the .doctor/latest
// symlink, and ensures .doctor/ is gitignored. If the chosen run dir already
// exists (same-second collision), the timestamp is advanced one second.
func NewRunArtifact(repoRoot, targetSHA string, now time.Time) (*RunArtifact, error) {
	doctorDir := filepath.Join(repoRoot, ".doctor")
	runsDir := filepath.Join(doctorDir, "runs")
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		return nil, fmt.Errorf("doctor: create runs dir: %w", err)
	}

	ts := now.UTC().Truncate(time.Second)
	var runID, runDir string
	for i := 0; i < 64; i++ {
		runID = RunID(targetSHA, ts)
		runDir = filepath.Join(runsDir, runDirName(ts, runID))
		if _, err := os.Stat(runDir); os.IsNotExist(err) {
			break
		}
		ts = ts.Add(time.Second)
	}
	for _, sub := range []string{"", "backups", "quarantine", "reports"} {
		if err := os.MkdirAll(filepath.Join(runDir, sub), 0o755); err != nil {
			return nil, fmt.Errorf("doctor: create run subdir %q: %w", sub, err)
		}
	}

	ra := &RunArtifact{
		RepoRoot:  repoRoot,
		DoctorDir: doctorDir,
		RunID:     runID,
		RunDir:    runDir,
		StartedAt: ts,
	}
	if err := ra.updateLatestSymlink(); err != nil {
		return nil, err
	}
	if err := ensureGitignore(repoRoot); err != nil {
		return nil, err
	}
	return ra, nil
}

// updateLatestSymlink atomically repoints .doctor/latest at this run dir.
func (ra *RunArtifact) updateLatestSymlink() error {
	latest := filepath.Join(ra.DoctorDir, "latest")
	target := filepath.Join("runs", filepath.Base(ra.RunDir))
	tmp := filepath.Join(ra.DoctorDir, fmt.Sprintf(".latest.tmp.%d", time.Now().UnixNano()))
	if err := os.Symlink(target, tmp); err != nil {
		return fmt.Errorf("doctor: create latest symlink: %w", err)
	}
	if err := os.Rename(tmp, latest); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("doctor: swap latest symlink: %w", err)
	}
	return nil
}

// gitRootOrSelf returns the nearest ancestor of dir that contains a .git
// entry, or dir itself if none is found. dir is resolved to an absolute path
// first — a relative path like "." has filepath.Dir(".") == ".", which would
// otherwise stop the walk immediately at the cwd.
func gitRootOrSelf(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	for d := abs; ; {
		if _, statErr := os.Stat(filepath.Join(d, ".git")); statErr == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return abs
		}
		d = parent
	}
}

// ensureGitignore appends ".doctor/" to the git repo's root .gitignore if
// absent. It targets the repository root (nearest ancestor with a .git entry)
// rather than the doctor's cwd, so running `ao doctor` from a subdirectory
// never scatters stray .gitignore files. ".doctor/" matches at any depth, so a
// single root entry covers every run location.
func ensureGitignore(repoRoot string) error {
	gi := filepath.Join(gitRootOrSelf(repoRoot), ".gitignore")
	data, err := os.ReadFile(gi)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("doctor: read .gitignore: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == ".doctor/" || strings.TrimSpace(line) == ".doctor" {
			return nil
		}
	}
	content := string(data)
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += ".doctor/\n"
	tmp := gi + fmt.Sprintf(".doctor.tmp.%d", time.Now().UnixNano())
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return fmt.Errorf("doctor: write .gitignore tmp: %w", err)
	}
	if err := os.Rename(tmp, gi); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("doctor: swap .gitignore: %w", err)
	}
	return nil
}

// pathIn returns the absolute path of name inside the run directory.
func (ra *RunArtifact) pathIn(name string) string {
	return filepath.Join(ra.RunDir, name)
}

// writeFileAtomic writes data to a file inside the run dir via temp+rename.
func (ra *RunArtifact) writeFileAtomic(name string, data []byte, mode os.FileMode) error {
	target := ra.pathIn(name)
	tmp, err := os.CreateTemp(ra.RunDir, ".artifact.tmp.*")
	if err != nil {
		return fmt.Errorf("doctor: create temp for %s: %w", name, err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("doctor: write %s: %w", name, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("doctor: sync %s: %w", name, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("doctor: close %s: %w", name, err)
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return fmt.Errorf("doctor: chmod %s: %w", name, err)
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		return fmt.Errorf("doctor: rename %s: %w", name, err)
	}
	return nil
}

// WriteReportJSON writes report.json (and a stdout.json replay copy).
func (ra *RunArtifact) WriteReportJSON(report any) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("doctor: marshal report.json: %w", err)
	}
	data = append(data, '\n')
	if err := ra.writeFileAtomic("report.json", data, 0o644); err != nil {
		return err
	}
	return ra.writeFileAtomic("stdout.json", data, 0o644)
}

// WriteReportMD writes the human-readable report.md narrative.
func (ra *RunArtifact) WriteReportMD(md string) error {
	return ra.writeFileAtomic("report.md", []byte(md), 0o644)
}

// WriteStderrLog writes the captured stderr for the run.
func (ra *RunArtifact) WriteStderrLog(text string) error {
	return ra.writeFileAtomic("stderr.log", []byte(text), 0o644)
}

// WriteUndoScript writes an idempotent undo.sh shell wrapper for the run.
func (ra *RunArtifact) WriteUndoScript() error {
	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

# undo.sh — restore from .doctor/runs/%s/backups/
# Generated by %s doctor %s.

cd "$(dirname "$0")/../../.."   # cd to repo root
%s doctor undo %s --strict "$@"
`, filepath.Base(ra.RunDir), ToolName, DoctorVersion, ToolName, filepath.Base(ra.RunDir))
	return ra.writeFileAtomic("undo.sh", []byte(script), 0o755)
}

// OpenActionsFile opens (creating) the run's actions.jsonl for append.
func (ra *RunArtifact) OpenActionsFile() (*os.File, error) {
	f, err := os.OpenFile(ra.pathIn("actions.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("doctor: open actions.jsonl: %w", err)
	}
	return f, nil
}

// ActionsPath returns the absolute path to the run's actions.jsonl.
func (ra *RunArtifact) ActionsPath() string { return ra.pathIn("actions.jsonl") }

// BackupsDir returns the absolute path to the run's backups/ directory.
func (ra *RunArtifact) BackupsDir() string { return ra.pathIn("backups") }

// QuarantineDir returns the absolute path to the run's quarantine/ directory.
func (ra *RunArtifact) QuarantineDir() string { return ra.pathIn("quarantine") }

// scorecardHistoryLine is one line in .doctor/scorecard_history.jsonl.
type scorecardHistoryLine struct {
	RunID         string         `json:"run_id"`
	StartedAt     string         `json:"started_at"`
	ToolVersion   string         `json:"tool_version"`
	DoctorVersion string         `json:"doctor_version"`
	OK            bool           `json:"ok"`
	TotalFindings int            `json:"total_findings"`
	BySeverity    map[string]int `json:"by_severity"`
	ActionsTaken  int            `json:"actions_taken"`
	DurationMS    int64          `json:"duration_ms"`
}

// AppendScorecardHistory appends one trend-analysis line to
// .doctor/scorecard_history.jsonl.
func (ra *RunArtifact) AppendScorecardHistory(toolVersion string, ok bool, total int, bySeverity map[string]int, actions int, dur time.Duration) error {
	line := scorecardHistoryLine{
		RunID:         ra.RunID,
		StartedAt:     ra.StartedAt.Format(time.RFC3339),
		ToolVersion:   toolVersion,
		DoctorVersion: DoctorVersion,
		OK:            ok,
		TotalFindings: total,
		BySeverity:    bySeverity,
		ActionsTaken:  actions,
		DurationMS:    dur.Milliseconds(),
	}
	data, err := json.Marshal(line)
	if err != nil {
		return fmt.Errorf("doctor: marshal scorecard history: %w", err)
	}
	data = append(data, '\n')
	path := filepath.Join(ra.DoctorDir, "scorecard_history.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("doctor: open scorecard history: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("doctor: write scorecard history: %w", err)
	}
	return f.Sync()
}
