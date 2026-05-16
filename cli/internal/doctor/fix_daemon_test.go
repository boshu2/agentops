package doctor

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// daemonTestEnv builds an isolated repo + home + .agents/daemon tree and
// returns a DetectEnv plus the repo root. The daemon store base and its
// projections/ subdir are created; callers shape the fixture from there.
func daemonTestEnv(t *testing.T) (*DetectEnv, string) {
	t.Helper()
	repo := t.TempDir()
	home := t.TempDir()
	for _, sub := range []string{
		filepath.Join(repo, ".agents", "daemon", "projections"),
		filepath.Join(repo, ".agents", "handoffs", "sha256"),
	} {
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatalf("mkdir %q: %v", sub, err)
		}
	}
	// Tests assume no live daemon unless they pin otherwise.
	orig := daemonProcessRunning
	daemonProcessRunning = func() bool { return false }
	t.Cleanup(func() { daemonProcessRunning = orig })
	return &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home, Logger: os.Stderr}, repo
}

// newDaemonMutateCtx builds a real MutateContext rooted at repo, scoped to a
// fixer id, with a live actions.jsonl handle. It returns the context and the
// run artifact so tests can inspect backups and undo.
func newDaemonMutateCtx(t *testing.T, repo, fixerID string) (*MutateContext, *RunArtifact) {
	t.Helper()
	ra, err := NewRunArtifact(repo, "dmntest", time.Now())
	if err != nil {
		t.Fatalf("NewRunArtifact: %v", err)
	}
	af, err := ra.OpenActionsFile()
	if err != nil {
		t.Fatalf("OpenActionsFile: %v", err)
	}
	t.Cleanup(func() { _ = af.Close() })
	caps := NewCapabilities("2.0.0")
	locks := NewLockManager(filepath.Join(repo, ".doctor", "locks"))
	return NewMutateContext(ra, caps, t.TempDir(), locks, af, false).WithFixer(fixerID), ra
}

// validLedgerLine returns a well-formed, schema-valid ledger event line.
func validLedgerLine(eventID string) string {
	return `{"schema_version":1,"event_id":"` + eventID + `","request_id":"req-0001",` +
		`"job_id":"job-0001","event_type":"job.submitted","occurred_at":"2026-01-01T00:00:00Z",` +
		`"actor":"agentopsd"}`
}

// gzipBytes returns the gzip-compressed form of s (test helper).
func gzipBytes(t *testing.T, s string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write([]byte(s)); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

// ---------------------------------------------------------------------------
// fm-daemon-corrupt-ledger-line
// ---------------------------------------------------------------------------

func TestDaemonCorruptLedgerLine_DetectFixUndo(t *testing.T) {
	env, repo := daemonTestEnv(t)
	ledger := daemonLedgerPath(env)
	good1 := validLedgerLine("evt-0001")
	good2 := validLedgerLine("evt-0002")
	badJSON := `{"event_id":"evt-bad","schema_version":1,`
	badSchema := `{"schema_version":99,"event_id":"evt-x","request_id":"req-0001","job_id":"job-0001","event_type":"job.submitted","occurred_at":"2026-01-01T00:00:00Z","actor":"agentopsd"}`
	original := good1 + "\n" + badJSON + "\n" + badSchema + "\n" + good2 + "\n"
	if err := os.WriteFile(ledger, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	det := corruptLedgerLineDetector{}
	findings, err := det.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if got := findings[0].Evidence.Lines; len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("corrupt line numbers = %v, want [2 3]", got)
	}

	ctx, ra := newDaemonMutateCtx(t, repo, det.ID())
	res, err := corruptLedgerLineFixer{}.Fix(ctx, env, findings)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	// 1 quarantine WriteFile + 1 ledger rewrite = 2 actions.
	if !res.Fixed || res.ActionsTaken != 2 {
		t.Fatalf("Fix result: fixed=%t actions=%d, want fixed=true actions=2", res.Fixed, res.ActionsTaken)
	}
	got, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	want := good1 + "\n" + good2 + "\n"
	if string(got) != want {
		t.Fatalf("post-fix ledger = %q, want %q", got, want)
	}
	// Backup is byte-identical to the corrupt original.
	backup := filepath.Join(ra.BackupsDir(), ".agents", "daemon", "ledger.jsonl")
	bgot, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	if string(bgot) != original {
		t.Fatalf("backup = %q, want original %q", bgot, original)
	}
	recs, err := readActions(ra.ActionsPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 {
		t.Fatalf("actions = %d, want 2", len(recs))
	}
	for _, r := range recs {
		if r.Op != "WriteFile" || !r.OK {
			t.Fatalf("action = %+v, want WriteFile OK", r)
		}
	}
	// Quarantine file holds the two corrupt raw lines.
	q := filepath.Join(daemonStoreDir(env), "quarantine", "ledger-corrupt-"+ctx.RunID+".jsonl")
	qgot, err := os.ReadFile(q)
	if err != nil {
		t.Fatalf("quarantine file missing: %v", err)
	}
	// The corrupt raw lines are embedded as JSON-escaped `raw` string fields,
	// so match the post-escape forms (evt-bad from line 2, schema_version 99
	// from line 3).
	if !bytes.Contains(qgot, []byte("evt-bad")) || !bytes.Contains(qgot, []byte("evt-x")) {
		t.Fatalf("quarantine file missing corrupt lines: %q", qgot)
	}
	if !bytes.Contains(qgot, []byte(`\"schema_version\":99`)) {
		t.Fatalf("quarantine file missing escaped schema_version 99 line: %q", qgot)
	}
	// Re-detect: clean.
	again, _ := det.Detect(env)
	if len(again) != 0 {
		t.Fatalf("post-fix detect found %d findings, want 0", len(again))
	}
	// Undo restores the corrupt original byte-identically.
	ur, err := Undo(repo, ra.RunID, true, false)
	if err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if ur.Restored < 1 {
		t.Fatalf("Undo restored = %d, want >= 1", ur.Restored)
	}
	restored, _ := os.ReadFile(ledger)
	if string(restored) != original {
		t.Fatalf("after undo = %q, want original %q", restored, original)
	}
}

func TestDaemonCorruptLedgerLine_RefusesOnLiveDaemon(t *testing.T) {
	env, repo := daemonTestEnv(t)
	ledger := daemonLedgerPath(env)
	if err := os.WriteFile(ledger, []byte("{bad\n"+validLedgerLine("evt-0001")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	daemonProcessRunning = func() bool { return true }
	ctx, ra := newDaemonMutateCtx(t, repo, "fm-daemon-corrupt-ledger-line")
	res, err := corruptLedgerLineFixer{}.Fix(ctx, env, nil)
	if err == nil {
		t.Fatal("expected refusal when daemon is live")
	}
	if res.Fixed {
		t.Fatal("Fix reported Fixed despite live-daemon refusal")
	}
	recs, _ := readActions(ra.ActionsPath())
	if len(recs) != 0 {
		t.Fatalf("live-daemon refusal wrote %d action records, want 0", len(recs))
	}
}

// ---------------------------------------------------------------------------
// fm-daemon-truncated-trailing-line
// ---------------------------------------------------------------------------

func TestDaemonTruncatedTrailingLine_DetectFixUndo(t *testing.T) {
	env, repo := daemonTestEnv(t)
	ledger := daemonLedgerPath(env)
	good1 := validLedgerLine("evt-0001")
	good2 := validLedgerLine("evt-0002")
	torn := `{"event_id":"evt-0003","schema_version":1,"kind":"job.subm`
	original := good1 + "\n" + good2 + "\n" + torn // no terminating newline
	if err := os.WriteFile(ledger, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	det := truncatedTrailingLineDetector{}
	findings, err := det.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 || findings[0].ID != "fm-daemon-truncated-trailing-line" {
		t.Fatalf("findings = %v", findings)
	}

	ctx, ra := newDaemonMutateCtx(t, repo, det.ID())
	res, err := truncatedTrailingLineFixer{}.Fix(ctx, env, findings)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if !res.Fixed || res.ActionsTaken != 2 {
		t.Fatalf("Fix result: fixed=%t actions=%d, want fixed=true actions=2", res.Fixed, res.ActionsTaken)
	}
	want := good1 + "\n" + good2 + "\n"
	got, _ := os.ReadFile(ledger)
	if string(got) != want {
		t.Fatalf("post-fix ledger = %q, want %q", got, want)
	}
	if got[len(got)-1] != '\n' {
		t.Fatal("post-fix ledger does not end with newline")
	}
	// Quarantine fragment holds the exact torn bytes.
	q := filepath.Join(daemonStoreDir(env), "quarantine", "trailing-fragment-"+ctx.RunID+".bin")
	qgot, err := os.ReadFile(q)
	if err != nil {
		t.Fatalf("fragment file missing: %v", err)
	}
	if string(qgot) != torn {
		t.Fatalf("quarantine fragment = %q, want %q", qgot, torn)
	}
	// Backup byte-identical to the torn original.
	backup := filepath.Join(ra.BackupsDir(), ".agents", "daemon", "ledger.jsonl")
	bgot, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	if string(bgot) != original {
		t.Fatalf("backup = %q, want %q", bgot, original)
	}
	recs, _ := readActions(ra.ActionsPath())
	if len(recs) != 2 {
		t.Fatalf("actions = %d, want 2", len(recs))
	}
	// Re-detect: clean.
	again, _ := det.Detect(env)
	if len(again) != 0 {
		t.Fatalf("post-fix detect found %d findings", len(again))
	}
	// Undo restores the torn original byte-identically.
	if _, err := Undo(repo, ra.RunID, true, false); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	restored, _ := os.ReadFile(ledger)
	if string(restored) != original {
		t.Fatalf("after undo = %q, want %q", restored, original)
	}
}

func TestDaemonTruncatedTrailingLine_NoFindingWhenTerminated(t *testing.T) {
	env, _ := daemonTestEnv(t)
	ledger := daemonLedgerPath(env)
	if err := os.WriteFile(ledger, []byte(validLedgerLine("evt-0001")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	findings, err := truncatedTrailingLineDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("newline-terminated ledger produced %d findings", len(findings))
	}
}

// ---------------------------------------------------------------------------
// fm-daemon-snapshot-schema-mismatch
// ---------------------------------------------------------------------------

func TestDaemonSnapshotSchemaMismatch_DetectFixUndo(t *testing.T) {
	env, repo := daemonTestEnv(t)
	projections := daemonProjectionsDir(env)
	stale1 := filepath.Join(projections, "snapshot-20260101T000000.000000000Z.json")
	stale2 := filepath.Join(projections, "snapshot-20260101T000100.000000000Z.json")
	current := filepath.Join(projections, "snapshot-20260101T000200.000000000Z.json")
	staleBody1 := `{"schema_version":99,"jobs":[]}` + "\n"
	staleBody2 := `{"schema_version":2,"jobs":[]}` + "\n"
	currentBody := `{"schema_version":1,"jobs":[]}` + "\n"
	for path, body := range map[string]string{stale1: staleBody1, stale2: staleBody2, current: currentBody} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	det := snapshotSchemaMismatchDetector{}
	findings, err := det.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if findings[0].Remediation.EstimatedActions != 2 {
		t.Fatalf("estimated actions = %d, want 2", findings[0].Remediation.EstimatedActions)
	}

	ctx, ra := newDaemonMutateCtx(t, repo, det.ID())
	res, err := snapshotSchemaMismatchFixer{}.Fix(ctx, env, findings)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if !res.Fixed || res.ActionsTaken != 2 {
		t.Fatalf("Fix result: fixed=%t actions=%d, want fixed=true actions=2", res.Fixed, res.ActionsTaken)
	}
	// projections/ now holds only the current-schema snapshot.
	if got := snapshotFiles(projections); len(got) != 1 || filepath.Base(got[0]) != filepath.Base(current) {
		t.Fatalf("projections after fix = %v, want only the current snapshot", got)
	}
	// Both stale snapshots present under retired/, byte-identical.
	for path, body := range map[string]string{stale1: staleBody1, stale2: staleBody2} {
		retired := filepath.Join(projections, "retired", filepath.Base(path))
		got, rerr := os.ReadFile(retired)
		if rerr != nil || string(got) != body {
			t.Fatalf("retired snapshot %q = %q err=%v, want %q", filepath.Base(path), got, rerr, body)
		}
	}
	recs, _ := readActions(ra.ActionsPath())
	if len(recs) != 2 {
		t.Fatalf("actions = %d, want 2", len(recs))
	}
	for _, r := range recs {
		if r.Op != "Rename" || r.RenameTo == "" {
			t.Fatalf("bad rename record: %+v", r)
		}
	}
	// Re-detect: clean.
	again, _ := det.Detect(env)
	if len(again) != 0 {
		t.Fatalf("post-fix detect found %d findings", len(again))
	}
	// Undo: both stale snapshots back in projections/.
	if _, err := Undo(repo, ra.RunID, true, false); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	for path, body := range map[string]string{stale1: staleBody1, stale2: staleBody2} {
		got, rerr := os.ReadFile(path)
		if rerr != nil || string(got) != body {
			t.Fatalf("after undo %q = %q err=%v, want %q", filepath.Base(path), got, rerr, body)
		}
	}
}

func TestDaemonSnapshotSchemaMismatch_NoFindingWhenAllCurrent(t *testing.T) {
	env, _ := daemonTestEnv(t)
	projections := daemonProjectionsDir(env)
	if err := os.WriteFile(filepath.Join(projections, "snapshot-20260101T000000.000000000Z.json"),
		[]byte(`{"schema_version":1,"jobs":[]}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	findings, err := snapshotSchemaMismatchDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("all-current snapshots produced %d findings", len(findings))
	}
}

// ---------------------------------------------------------------------------
// fm-daemon-unreachable (detect-only)
// ---------------------------------------------------------------------------

func TestDaemonUnreachable_DetectWhenDown(t *testing.T) {
	env, _ := daemonTestEnv(t)
	// daemonProcessRunning is pinned false by daemonTestEnv → daemon is DOWN.
	det := daemonUnreachableDetector{}
	findings, err := det.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if findings[0].Remediation.AutoFixable {
		t.Fatal("daemon-unreachable must be detect-only (auto_fixable=false)")
	}
	if findings[0].Remediation.Command != "ao daemon start" {
		t.Fatalf("remediation command = %q, want `ao daemon start`", findings[0].Remediation.Command)
	}
}

func TestDaemonUnreachable_NoFindingWhenRunning(t *testing.T) {
	env, _ := daemonTestEnv(t)
	daemonProcessRunning = func() bool { return true }
	findings, err := daemonUnreachableDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("running daemon produced %d unreachable findings", len(findings))
	}
}

func TestDaemonUnreachable_FixerRefuses(t *testing.T) {
	env, repo := daemonTestEnv(t)
	fx := daemonUnreachableFixer{}
	if fx.AutoFixable() {
		t.Fatal("daemonUnreachableFixer.AutoFixable() must be false")
	}
	ctx, ra := newDaemonMutateCtx(t, repo, fx.ID())
	res, err := fx.Fix(ctx, env, nil)
	if err == nil {
		t.Fatal("detect-only fixer must refuse with an error")
	}
	if res.Fixed {
		t.Fatal("detect-only fixer reported Fixed")
	}
	recs, _ := readActions(ra.ActionsPath())
	if len(recs) != 0 {
		t.Fatalf("detect-only fixer wrote %d action records, want 0", len(recs))
	}
}

// ---------------------------------------------------------------------------
// fm-daemon-orphan-tmp-files
// ---------------------------------------------------------------------------

func TestDaemonOrphanTmpFiles_DetectFixUndo(t *testing.T) {
	env, repo := daemonTestEnv(t)
	store := daemonStoreDir(env)
	// Pin a deterministic clock; orphans back-dated past the grace window.
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	orig := daemonNow
	daemonNow = func() time.Time { return now }
	t.Cleanup(func() { daemonNow = orig })
	old := now.Add(-time.Hour)

	orphans := map[string]string{
		filepath.Join(store, "projections", "snapshot-20260101T000000.000000000Z.json.tmp"): "snap-temp",
		filepath.Join(store, "ledger.20260101T000000.000000000Z.jsonl.gz.tmp"):              "gz-temp",
		filepath.Join(daemonHandoffsDir(env), ".abc123def456.tmp"):                          "artifact-temp",
	}
	for path, body := range orphans {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}
	// A fresh control temp inside the grace window — must not be touched.
	fresh := filepath.Join(store, "snapshot-fresh.json.tmp")
	if err := os.WriteFile(fresh, []byte("fresh"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(fresh, now, now); err != nil {
		t.Fatal(err)
	}

	det := orphanTmpFilesDetector{}
	findings, err := det.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if findings[0].Remediation.EstimatedActions != 3 {
		t.Fatalf("estimated actions = %d, want 3 (fresh temp excluded)", findings[0].Remediation.EstimatedActions)
	}

	ctx, ra := newDaemonMutateCtx(t, repo, det.ID())
	res, err := orphanTmpFilesFixer{}.Fix(ctx, env, findings)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if !res.Fixed || res.ActionsTaken != 3 {
		t.Fatalf("Fix result: fixed=%t actions=%d, want fixed=true actions=3", res.Fixed, res.ActionsTaken)
	}
	// The three orphans are gone from their original locations.
	for path := range orphans {
		if _, statErr := os.Stat(path); statErr == nil {
			t.Fatalf("orphan %q still present at original path", path)
		}
	}
	// And present, byte-identical, under quarantine/orphan-tmp/<run>/.
	for path, body := range orphans {
		rel, _ := filepath.Rel(repo, path)
		q := filepath.Join(store, "quarantine", "orphan-tmp", ctx.RunID, rel)
		got, qerr := os.ReadFile(q)
		if qerr != nil || string(got) != body {
			t.Fatalf("quarantined %q = %q err=%v, want %q", filepath.Base(path), got, qerr, body)
		}
	}
	// Fresh control temp untouched.
	if got, _ := os.ReadFile(fresh); string(got) != "fresh" {
		t.Fatalf("fresh control temp changed: %q", got)
	}
	recs, _ := readActions(ra.ActionsPath())
	if len(recs) != 3 {
		t.Fatalf("actions = %d, want 3", len(recs))
	}
	// Re-detect: clean.
	again, _ := det.Detect(env)
	if len(again) != 0 {
		t.Fatalf("post-fix detect found %d findings", len(again))
	}
	// Undo restores the three orphans to their original paths.
	if _, err := Undo(repo, ra.RunID, true, false); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	for path, body := range orphans {
		got, rerr := os.ReadFile(path)
		if rerr != nil || string(got) != body {
			t.Fatalf("after undo %q = %q err=%v, want %q", filepath.Base(path), got, rerr, body)
		}
	}
}

func TestDaemonOrphanTmpFiles_NoFindingWhenAllFresh(t *testing.T) {
	env, _ := daemonTestEnv(t)
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	orig := daemonNow
	daemonNow = func() time.Time { return now }
	t.Cleanup(func() { daemonNow = orig })
	fresh := filepath.Join(daemonStoreDir(env), "snapshot-fresh.json.tmp")
	if err := os.WriteFile(fresh, []byte("fresh"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(fresh, now, now); err != nil {
		t.Fatal(err)
	}
	findings, err := orphanTmpFilesDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("all-fresh temps produced %d findings", len(findings))
	}
}

// ---------------------------------------------------------------------------
// fm-daemon-corrupt-gzip-archive
// ---------------------------------------------------------------------------

func TestDaemonCorruptGzipArchive_DetectFixUndo(t *testing.T) {
	env, repo := daemonTestEnv(t)
	store := daemonStoreDir(env)
	// Sound control archive.
	soundBody := validLedgerLine("evt-sound") + "\n"
	sound := filepath.Join(store, "ledger.20260101T000000.000000000Z.jsonl.gz")
	if err := os.WriteFile(sound, gzipBytes(t, soundBody), 0o600); err != nil {
		t.Fatal(err)
	}
	// Truncated archive: a valid gzip stream cut to its first 20 bytes.
	truncatedFull := gzipBytes(t, validLedgerLine("evt-trunc-1")+"\n"+validLedgerLine("evt-trunc-2")+"\n")
	truncated := filepath.Join(store, "ledger.20260101T000100.000000000Z.jsonl.gz")
	cut := truncatedFull
	if len(cut) > 20 {
		cut = cut[:20]
	}
	truncatedOriginal := append([]byte(nil), cut...)
	if err := os.WriteFile(truncated, truncatedOriginal, 0o600); err != nil {
		t.Fatal(err)
	}
	// Invalid-header archive: literal non-gzip bytes.
	invalidOriginal := []byte("not a gzip file at all\n")
	invalid := filepath.Join(store, "ledger.20260101T000200.000000000Z.jsonl.gz")
	if err := os.WriteFile(invalid, invalidOriginal, 0o600); err != nil {
		t.Fatal(err)
	}

	det := corruptGzipArchiveDetector{}
	findings, err := det.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}

	ctx, ra := newDaemonMutateCtx(t, repo, det.ID())
	res, err := corruptGzipArchiveFixer{}.Fix(ctx, env, findings)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if !res.Fixed {
		t.Fatal("Fix did not report Fixed")
	}
	// Every remaining .gz archive must be a valid gzip stream.
	for _, f := range gzipArchiveFiles(env) {
		if kind := gzipArchiveCheck(f); kind != "" {
			t.Fatalf("archive %q still corrupt (%s) after fix", filepath.Base(f), kind)
		}
	}
	// The two corrupt archives are present, byte-identical, under quarantine.
	for path, body := range map[string][]byte{truncated: truncatedOriginal, invalid: invalidOriginal} {
		q := filepath.Join(store, "quarantine", "bad-archives", ctx.RunID, filepath.Base(path))
		got, qerr := os.ReadFile(q)
		if qerr != nil || !bytes.Equal(got, body) {
			t.Fatalf("quarantined %q mismatch err=%v", filepath.Base(path), qerr)
		}
	}
	// The sound control archive is untouched.
	if got, _ := os.ReadFile(sound); !bytes.Equal(got, gzipBytes(t, soundBody)) {
		t.Fatal("sound control archive was modified")
	}
	recs, _ := readActions(ra.ActionsPath())
	if len(recs) < 2 {
		t.Fatalf("actions = %d, want >= 2 (two retire renames)", len(recs))
	}
	if !bytes.Contains([]byte(recs[0].Path), []byte("ledger.")) {
		t.Fatalf("first action path = %q, want a ledger archive", recs[0].Path)
	}
	// Re-detect: clean.
	again, _ := det.Detect(env)
	if len(again) != 0 {
		t.Fatalf("post-fix detect found %d findings", len(again))
	}
	// Undo: the two corrupt archives are restored byte-identically.
	if _, err := Undo(repo, ra.RunID, true, false); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	for path, body := range map[string][]byte{truncated: truncatedOriginal, invalid: invalidOriginal} {
		got, rerr := os.ReadFile(path)
		if rerr != nil || !bytes.Equal(got, body) {
			t.Fatalf("after undo %q mismatch err=%v", filepath.Base(path), rerr)
		}
	}
}

func TestDaemonCorruptGzipArchive_NoFindingWhenAllSound(t *testing.T) {
	env, _ := daemonTestEnv(t)
	sound := filepath.Join(daemonStoreDir(env), "ledger.20260101T000000.000000000Z.jsonl.gz")
	if err := os.WriteFile(sound, gzipBytes(t, validLedgerLine("evt-sound")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	findings, err := corruptGzipArchiveDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("all-sound archives produced %d findings", len(findings))
	}
}

// ---------------------------------------------------------------------------
// fm-daemon-archive-unbounded-growth
// ---------------------------------------------------------------------------

func TestDaemonArchiveUnboundedGrowth_DetectFixUndo(t *testing.T) {
	env, repo := daemonTestEnv(t)
	store := daemonStoreDir(env)
	projections := daemonProjectionsDir(env)
	// daemonArchiveRetention=30, daemonSnapshotRetention=10. Build 32 archives
	// and 12 snapshots so 2 of each are excess.
	const archCount, snapCount = 32, 12
	var archives, snapshots []string
	for i := 0; i < archCount; i++ {
		ts := time.Date(2026, 1, 1, 0, i, 0, 0, time.UTC).Format("20060102T150405.000000000Z")
		p := filepath.Join(store, "ledger."+ts+".jsonl.gz")
		if err := os.WriteFile(p, gzipBytes(t, validLedgerLine("evt-"+ts)+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		archives = append(archives, p)
	}
	for i := 0; i < snapCount; i++ {
		ts := time.Date(2026, 1, 1, 0, i, 0, 0, time.UTC).Format("20060102T150405.000000000Z")
		p := filepath.Join(projections, "snapshot-"+ts+".json")
		if err := os.WriteFile(p, []byte(`{"schema_version":1,"jobs":[]}`+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		snapshots = append(snapshots, p)
	}
	// Active ledger.jsonl — must never be touched.
	ledger := daemonLedgerPath(env)
	if err := os.WriteFile(ledger, []byte(validLedgerLine("evt-live")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	det := archiveUnboundedGrowthDetector{}
	findings, err := det.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if findings[0].Remediation.EstimatedActions != 4 {
		t.Fatalf("estimated actions = %d, want 4 (2 archives + 2 snapshots)", findings[0].Remediation.EstimatedActions)
	}

	ctx, ra := newDaemonMutateCtx(t, repo, det.ID())
	res, err := archiveUnboundedGrowthFixer{}.Fix(ctx, env, findings)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if !res.Fixed || res.ActionsTaken != 4 {
		t.Fatalf("Fix result: fixed=%t actions=%d, want fixed=true actions=4", res.Fixed, res.ActionsTaken)
	}
	// Exactly 30 archives and 10 snapshots remain — the newest of each.
	if got := allArchiveFiles(env); len(got) != daemonArchiveRetention {
		t.Fatalf("archives remaining = %d, want %d", len(got), daemonArchiveRetention)
	}
	if got := snapshotFiles(projections); len(got) != daemonSnapshotRetention {
		t.Fatalf("snapshots remaining = %d, want %d", len(got), daemonSnapshotRetention)
	}
	// The two oldest archives are retired, byte-identical.
	for _, src := range archives[:2] {
		q := filepath.Join(store, "quarantine", "retired-archives", ctx.RunID, filepath.Base(src))
		if _, statErr := os.Stat(q); statErr != nil {
			t.Fatalf("oldest archive %q not retired", filepath.Base(src))
		}
		if _, statErr := os.Stat(src); statErr == nil {
			t.Fatalf("retired archive %q still at original path", filepath.Base(src))
		}
	}
	for _, src := range snapshots[:2] {
		q := filepath.Join(store, "quarantine", "retired-snapshots", ctx.RunID, filepath.Base(src))
		if _, statErr := os.Stat(q); statErr != nil {
			t.Fatalf("oldest snapshot %q not retired", filepath.Base(src))
		}
	}
	// The active ledger.jsonl is untouched.
	if got, _ := os.ReadFile(ledger); string(got) != validLedgerLine("evt-live")+"\n" {
		t.Fatalf("active ledger.jsonl was modified: %q", got)
	}
	recs, _ := readActions(ra.ActionsPath())
	if len(recs) != 4 {
		t.Fatalf("actions = %d, want 4", len(recs))
	}
	// Re-detect: clean.
	again, _ := det.Detect(env)
	if len(again) != 0 {
		t.Fatalf("post-fix detect found %d findings", len(again))
	}
	// Undo restores all 32 archives and 12 snapshots.
	if _, err := Undo(repo, ra.RunID, true, false); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if got := allArchiveFiles(env); len(got) != archCount {
		t.Fatalf("after undo archives = %d, want %d", len(got), archCount)
	}
	if got := snapshotFiles(projections); len(got) != snapCount {
		t.Fatalf("after undo snapshots = %d, want %d", len(got), snapCount)
	}
}

func TestDaemonArchiveUnboundedGrowth_NoFindingWithinCap(t *testing.T) {
	env, _ := daemonTestEnv(t)
	store := daemonStoreDir(env)
	for i := 0; i < 5; i++ {
		ts := time.Date(2026, 1, 1, 0, i, 0, 0, time.UTC).Format("20060102T150405.000000000Z")
		if err := os.WriteFile(filepath.Join(store, "ledger."+ts+".jsonl.gz"),
			gzipBytes(t, validLedgerLine("evt-"+ts)+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	findings, err := archiveUnboundedGrowthDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("within-cap store produced %d findings", len(findings))
	}
}

func TestDaemonArchiveUnboundedGrowth_RefusesOnLiveDaemon(t *testing.T) {
	env, repo := daemonTestEnv(t)
	daemonProcessRunning = func() bool { return true }
	ctx, ra := newDaemonMutateCtx(t, repo, "fm-daemon-archive-unbounded-growth")
	res, err := archiveUnboundedGrowthFixer{}.Fix(ctx, env, nil)
	if err == nil {
		t.Fatal("expected refusal when daemon is live")
	}
	if res.Fixed {
		t.Fatal("Fix reported Fixed despite live-daemon refusal")
	}
	recs, _ := readActions(ra.ActionsPath())
	if len(recs) != 0 {
		t.Fatalf("live-daemon refusal wrote %d action records, want 0", len(recs))
	}
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func TestDaemonDetectorsAndFixersRegistered(t *testing.T) {
	wantDetectors := []string{
		"fm-daemon-archive-unbounded-growth",
		"fm-daemon-corrupt-gzip-archive",
		"fm-daemon-corrupt-ledger-line",
		"fm-daemon-orphan-tmp-files",
		"fm-daemon-snapshot-schema-mismatch",
		"fm-daemon-truncated-trailing-line",
		"fm-daemon-unreachable",
	}
	for _, id := range wantDetectors {
		found := false
		for _, d := range Detectors() {
			if d.ID() == id {
				found = true
				if d.Subsystem() != "daemon" {
					t.Fatalf("detector %q subsystem = %q, want daemon", id, d.Subsystem())
				}
			}
		}
		if !found {
			t.Fatalf("detector %q not registered", id)
		}
	}
	autoFixable := map[string]bool{
		"fm-daemon-corrupt-ledger-line":      true,
		"fm-daemon-truncated-trailing-line":  true,
		"fm-daemon-snapshot-schema-mismatch": true,
		"fm-daemon-orphan-tmp-files":         true,
		"fm-daemon-corrupt-gzip-archive":     true,
		"fm-daemon-archive-unbounded-growth": true,
		"fm-daemon-unreachable":              false,
	}
	for id, want := range autoFixable {
		fx := FixerByID(id)
		if fx == nil {
			t.Fatalf("fixer %q not registered", id)
		}
		if fx.AutoFixable() != want {
			t.Fatalf("fixer %q AutoFixable() = %t, want %t", id, fx.AutoFixable(), want)
		}
	}
}
