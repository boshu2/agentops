package doctor

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeOversizeBytes creates a regular file of exactly n zero bytes with the
// given mtime, creating parent directories as needed. Sizes in these tests are
// exact on purpose: the detector's threshold comparison is `> threshold`, so
// boundary fixtures sit at threshold and threshold+1.
func writeOversizeBytes(t *testing.T, path string, n int, mtime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, make([]byte, n), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("chtimes %s: %v", path, err)
	}
}

// TestWorkspaceOversize_DetectDirsAndLooseFiles is the main fixture pass at a
// 1 MiB override threshold: two oversize dirs and one oversize loose file are
// flagged with exact byte sizes; boundary-exact dirs/files, small files, and
// symlinks are not. Finding order is deterministic (sorted by name).
func TestWorkspaceOversize_DetectDirsAndLooseFiles(t *testing.T) {
	t.Setenv(workspaceSizeEnvVar, "1") // threshold = 1 MiB = 1048576 bytes
	repo := t.TempDir()
	outside := t.TempDir()
	agents := filepath.Join(repo, ".agents")
	mt := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)

	// Over threshold by exactly one byte: flagged.
	writeOversizeBytes(t, filepath.Join(agents, "aa-bulky", "blob.bin"), mib+1, mt)
	// Exactly AT threshold: NOT flagged (comparison is strictly greater).
	writeOversizeBytes(t, filepath.Join(agents, "trim", "data.bin"), mib, mt)
	// Over threshold via two nested files (1100000 bytes total): flagged.
	writeOversizeBytes(t, filepath.Join(agents, "zz-heap", "a.bin"), 900000, mt)
	writeOversizeBytes(t, filepath.Join(agents, "zz-heap", "nested", "b.bin"), 200000, mt.Add(time.Hour))
	// Loose file over the fixed 2 MiB threshold by one byte: flagged.
	writeOversizeBytes(t, filepath.Join(agents, "wiki-index.jsonl"), 2*mib+1, mt)
	// Loose file exactly AT 2 MiB: NOT flagged.
	writeOversizeBytes(t, filepath.Join(agents, "boundary.jsonl"), 2*mib, mt)
	// Small loose file: NOT flagged.
	writeOversizeBytes(t, filepath.Join(agents, "small.json"), 10, mt)
	// Loose symlink to a huge outside file: NOT flagged (symlink-safe).
	decoy := filepath.Join(outside, "decoy.bin")
	writeOversizeBytes(t, decoy, 3*mib, mt)
	if err := os.Symlink(decoy, filepath.Join(agents, "link-big")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir(), Logger: os.Stderr}
	findings, err := workspaceOversizeDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 3 {
		t.Fatalf("findings = %d, want 3", len(findings))
	}

	wantTitles := []string{
		".agents/aa-bulky is 1048577 bytes (1.0 MiB) — over the 1 MiB threshold",
		"loose file .agents/wiki-index.jsonl is 2097153 bytes (2.0 MiB) — over the fixed 2 MiB loose-file threshold",
		".agents/zz-heap is 1100000 bytes (1.0 MiB) — over the 1 MiB threshold",
	}
	wantFiles := []string{".agents/aa-bulky", ".agents/wiki-index.jsonl", ".agents/zz-heap"}
	for i, f := range findings {
		if f.ID != "fm-ws-oversize" {
			t.Errorf("findings[%d].ID = %q, want fm-ws-oversize", i, f.ID)
		}
		if f.Subsystem != "workspace" {
			t.Errorf("findings[%d].Subsystem = %q, want workspace", i, f.Subsystem)
		}
		if f.Severity != "P3" {
			t.Errorf("findings[%d].Severity = %q, want P3", i, f.Severity)
		}
		if f.Confidence != 1.0 {
			t.Errorf("findings[%d].Confidence = %v, want 1.0", i, f.Confidence)
		}
		if f.Title != wantTitles[i] {
			t.Errorf("findings[%d].Title = %q, want %q", i, f.Title, wantTitles[i])
		}
		if f.Evidence.File != wantFiles[i] {
			t.Errorf("findings[%d].Evidence.File = %q, want %q", i, f.Evidence.File, wantFiles[i])
		}
		if f.Remediation.AutoFixable {
			t.Errorf("findings[%d] is AutoFixable, want report-only", i)
		}
		if want := "ao doctor explain fm-ws-oversize"; f.Remediation.ExplainCommand != want {
			t.Errorf("findings[%d].ExplainCommand = %q, want %q", i, f.Remediation.ExplainCommand, want)
		}
		if f.Remediation.Command == "" {
			t.Errorf("findings[%d].Remediation.Command is empty", i)
		}
	}

	// Dir evidence carries file count and newest mtime.
	if want := "du -sk .agents/aa-bulky  # 1 file(s), newest mtime 2026-07-01T10:00:00Z"; findings[0].Evidence.Query != want {
		t.Errorf("aa-bulky Evidence.Query = %q, want %q", findings[0].Evidence.Query, want)
	}
	if want := "du -sk .agents/zz-heap  # 2 file(s), newest mtime 2026-07-01T11:00:00Z"; findings[2].Evidence.Query != want {
		t.Errorf("zz-heap Evidence.Query = %q, want %q", findings[2].Evidence.Query, want)
	}
	if want := "ls -l .agents/wiki-index.jsonl"; findings[1].Evidence.Query != want {
		t.Errorf("loose-file Evidence.Query = %q, want %q", findings[1].Evidence.Query, want)
	}
}

// TestWorkspaceOversize_DefaultThresholdBoundary exercises the 25 MiB default
// boundary with exact sizes: threshold+1 is flagged, exactly-threshold is not.
func TestWorkspaceOversize_DefaultThresholdBoundary(t *testing.T) {
	t.Setenv(workspaceSizeEnvVar, "") // empty = unset: 25 MiB default
	repo := t.TempDir()
	agents := filepath.Join(repo, ".agents")
	mt := time.Date(2026, 7, 2, 9, 0, 0, 0, time.UTC)

	writeOversizeBytes(t, filepath.Join(agents, "over", "blob.bin"), 25*mib+1, mt)
	writeOversizeBytes(t, filepath.Join(agents, "under", "blob.bin"), 25*mib, mt)

	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir(), Logger: os.Stderr}
	findings, err := workspaceOversizeDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1 (only the over-threshold dir)", len(findings))
	}
	if want := ".agents/over is 26214401 bytes (25.0 MiB) — over the 25 MiB threshold"; findings[0].Title != want {
		t.Errorf("Title = %q, want %q", findings[0].Title, want)
	}
}

// TestWorkspaceOversize_ThresholdEnvOverride is the env-override table for
// workspaceSizeThreshold: valid values apply, invalid values (zero, negative,
// garbage) and unset/empty fall back to the 25 MiB default.
func TestWorkspaceOversize_ThresholdEnvOverride(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int64
	}{
		{"unset/empty", "", 25 * mib},
		{"valid one MiB", "1", mib},
		{"valid large", "100", 100 * mib},
		{"cap boundary accepted", "1048576", 1048576 * mib},
		{"over cap falls back", "1048577", 25 * mib},
		// The overflow class from the hardening finding: MiB counts above
		// ~2^43 wrap the int64 byte product negative, flagging everything.
		{"int64-overflow value falls back", "9000000000000", 25 * mib},
		{"zero invalid", "0", 25 * mib},
		{"negative invalid", "-5", 25 * mib},
		{"garbage invalid", "abc", 25 * mib},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(workspaceSizeEnvVar, tt.raw)
			if got := workspaceSizeThreshold(); got != tt.want {
				t.Errorf("workspaceSizeThreshold() with %q = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}

func TestWorkspaceOversize_DetectNoAgentsDir(t *testing.T) {
	repo := t.TempDir()
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir(), Logger: os.Stderr}
	findings, err := workspaceOversizeDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("missing .agents produced %d findings, want 0", len(findings))
	}
}

// TestWorkspaceOversize_EngineFixPassZeroMutations proves the report-only
// contract end-to-end: a full detect+fix engine pass scoped to this ID has no
// registered fixer (FixerByID nil), takes zero actions, writes zero action
// records, and leaves every offender byte-for-byte in place.
func TestWorkspaceOversize_EngineFixPassZeroMutations(t *testing.T) {
	t.Setenv(workspaceSizeEnvVar, "1")
	repo := t.TempDir()
	agents := filepath.Join(repo, ".agents")
	mt := time.Date(2026, 7, 3, 8, 0, 0, 0, time.UTC)
	dirBlob := filepath.Join(agents, "bulky", "blob.bin")
	loose := filepath.Join(agents, "wiki-index.jsonl")
	writeOversizeBytes(t, dirBlob, mib+1, mt)
	writeOversizeBytes(t, loose, 2*mib+1, mt)

	if fx := FixerByID(workspaceOversizeID); fx != nil {
		t.Fatalf("FixerByID(%q) = %T, want nil (report-only detector)", workspaceOversizeID, fx)
	}

	rep, err := Fix(Options{
		RepoRoot:    repo,
		CWD:         repo,
		HomeDir:     t.TempDir(),
		ToolVersion: "test",
		Only:        []string{workspaceOversizeID},
	})
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if got := rep.Summary.TotalFindings; got != 2 {
		t.Fatalf("TotalFindings = %d, want 2", got)
	}
	if rep.ActionsTaken != 0 {
		t.Fatalf("ActionsTaken = %d, want 0 (no fixer registered)", rep.ActionsTaken)
	}
	// Unfixable findings remain: the fix pass reports partial, never healthy.
	if rep.ExitCode != ExitFixPartial {
		t.Errorf("ExitCode = %d, want %d (ExitFixPartial)", rep.ExitCode, ExitFixPartial)
	}

	// Zero action records in the run's actions.jsonl.
	recs, err := readActions(filepath.Join(repo, rep.ActionsPath))
	if err != nil {
		t.Fatalf("readActions: %v", err)
	}
	if len(recs) != 0 {
		t.Fatalf("action records = %d, want 0", len(recs))
	}

	// Offenders untouched, exact sizes preserved.
	for path, want := range map[string]int64{dirBlob: mib + 1, loose: 2*mib + 1} {
		st, err := os.Stat(path)
		if err != nil {
			t.Fatalf("offender %s missing after fix pass: %v", path, err)
		}
		if st.Size() != want {
			t.Errorf("offender %s size = %d, want %d (mutated!)", path, st.Size(), want)
		}
	}
}

func TestWorkspaceOversize_Registration(t *testing.T) {
	var det Detector
	for _, d := range Detectors() {
		if d.ID() == workspaceOversizeID {
			det = d
		}
	}
	if det == nil {
		t.Fatalf("detector %q not registered", workspaceOversizeID)
	}
	if det.Subsystem() != "workspace" {
		t.Errorf("detector subsystem = %q, want workspace", det.Subsystem())
	}
	if det.Severity() != "P3" {
		t.Errorf("detector severity = %q, want P3", det.Severity())
	}
	if det.QuickPath() {
		t.Error("detector QuickPath() = true, want false (full tree walk)")
	}
	if det.OnlineRequired() {
		t.Error("detector OnlineRequired() = true, want false")
	}
	if det.EstimatedCostMS() != 250 {
		t.Errorf("detector EstimatedCostMS() = %d, want 250", det.EstimatedCostMS())
	}
	if det.Describe() == "" {
		t.Error("detector Describe() is empty")
	}
	if fx := FixerByID(workspaceOversizeID); fx != nil {
		t.Errorf("FixerByID(%q) = %T, want nil: fm-ws-oversize is report-only", workspaceOversizeID, fx)
	}
}
