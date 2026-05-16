package doctor

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// knowledgeTestEnv builds an isolated repo + home + knowledge-store tree and
// returns a DetectEnv plus the repo root. The store base .agents/ao is created;
// callers add or remove subdirs to shape the fixture.
func knowledgeTestEnv(t *testing.T) (*DetectEnv, string) {
	t.Helper()
	repo := t.TempDir()
	home := t.TempDir()
	base := filepath.Join(repo, ".agents", "ao")
	for _, sub := range requiredSubdirs {
		if err := os.MkdirAll(filepath.Join(base, sub), 0o755); err != nil {
			t.Fatalf("mkdir store sub %q: %v", sub, err)
		}
	}
	return &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: home, Logger: os.Stderr}, repo
}

// newKnowledgeMutateCtx builds a real MutateContext rooted at repo, scoped to a
// fixer id, with a live actions.jsonl handle. It returns the context and the
// run artifact so tests can inspect backups and undo.
func newKnowledgeMutateCtx(t *testing.T, repo, fixerID string) (*MutateContext, *RunArtifact) {
	t.Helper()
	ra, err := NewRunArtifact(repo, "knowtest", time.Now())
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

// validIndexLine is a well-formed search-index JSONL entry for the given path.
func validIndexLine(path string) string {
	return `{"path":"` + path + `","content":"x","modified_at":"2026-01-01T00:00:00Z"}`
}

// ---------------------------------------------------------------------------
// fm-knowledge-missing-substructure
// ---------------------------------------------------------------------------

func TestKnowledgeMissingSubstructure_DetectAndFix(t *testing.T) {
	env, repo := knowledgeTestEnv(t)
	base := knowledgeBaseDir(env)
	// Remove two of the three required subdirs.
	if err := os.RemoveAll(filepath.Join(base, "index")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(base, "provenance")); err != nil {
		t.Fatal(err)
	}
	// Keep a witness file in the surviving sessions/ dir; the fixer must not touch it.
	witness := filepath.Join(base, "sessions", "keep.txt")
	if err := os.WriteFile(witness, []byte("untouched"), 0o644); err != nil {
		t.Fatal(err)
	}

	det := missingSubstructureDetector{}
	findings, err := det.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if findings[0].ID != "fm-knowledge-missing-substructure" {
		t.Fatalf("finding ID = %q", findings[0].ID)
	}
	if findings[0].Remediation.EstimatedActions != 2 {
		t.Fatalf("estimated actions = %d, want 2", findings[0].Remediation.EstimatedActions)
	}

	ctx, _ := newKnowledgeMutateCtx(t, repo, det.ID())
	res, err := missingSubstructureFixer{}.Fix(ctx, env, findings)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if !res.Fixed {
		t.Fatal("Fix did not report Fixed")
	}
	if res.ActionsTaken != 2 {
		t.Fatalf("ActionsTaken = %d, want 2", res.ActionsTaken)
	}
	for _, sub := range requiredSubdirs {
		st, err := os.Stat(filepath.Join(base, sub))
		if err != nil || !st.IsDir() {
			t.Fatalf("subdir %q not re-created", sub)
		}
	}
	// Witness file untouched.
	got, err := os.ReadFile(witness)
	if err != nil || string(got) != "untouched" {
		t.Fatalf("witness file changed: %q err=%v", got, err)
	}
	// Re-detect: clean.
	again, _ := det.Detect(env)
	if len(again) != 0 {
		t.Fatalf("post-fix detect found %d findings, want 0", len(again))
	}
}

func TestKnowledgeMissingSubstructure_RefusesNonDirSlot(t *testing.T) {
	env, repo := knowledgeTestEnv(t)
	base := knowledgeBaseDir(env)
	if err := os.RemoveAll(filepath.Join(base, "index")); err != nil {
		t.Fatal(err)
	}
	// A regular file occupies the index/ slot.
	if err := os.WriteFile(filepath.Join(base, "index"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, _ := newKnowledgeMutateCtx(t, repo, "fm-knowledge-missing-substructure")
	res, err := missingSubstructureFixer{}.Fix(ctx, env, nil)
	if err == nil {
		t.Fatal("expected refusal on non-directory slot")
	}
	if res.Fixed {
		t.Fatal("Fix reported Fixed despite refusal")
	}
}

func TestKnowledgeMissingSubstructure_NoFindingWhenHealthy(t *testing.T) {
	env, _ := knowledgeTestEnv(t)
	findings, err := missingSubstructureDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("healthy store produced %d findings", len(findings))
	}
}

// ---------------------------------------------------------------------------
// fm-knowledge-corrupt-index-lines
// ---------------------------------------------------------------------------

func TestKnowledgeCorruptIndexLines_DetectFixUndo(t *testing.T) {
	env, repo := knowledgeTestEnv(t)
	idx := searchIndexPath(env)
	good1 := validIndexLine(".agents/learnings/a.md")
	good2 := validIndexLine(".agents/learnings/b.md")
	badJSON := `{this is not json`
	badSchema := `{"content":"orphan","modified_at":"2026-01-01T00:00:00Z"}`
	// Interleave good lines around the bad ones to prove order preservation.
	original := good1 + "\n" + badJSON + "\n" + badSchema + "\n" + good2 + "\n"
	if err := os.WriteFile(idx, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	det := corruptIndexLinesDetector{}
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

	ctx, ra := newKnowledgeMutateCtx(t, repo, det.ID())
	res, err := corruptIndexLinesFixer{}.Fix(ctx, env, findings)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if !res.Fixed || res.ActionsTaken != 1 {
		t.Fatalf("Fix result: fixed=%t actions=%d", res.Fixed, res.ActionsTaken)
	}
	// Surviving file: exactly the two good lines, in order, with trailing newline.
	got, err := os.ReadFile(idx)
	if err != nil {
		t.Fatal(err)
	}
	want := good1 + "\n" + good2 + "\n"
	if string(got) != want {
		t.Fatalf("post-fix index = %q, want %q", got, want)
	}
	// Backup is byte-identical to the corrupt original.
	backup := filepath.Join(ra.BackupsDir(), ".agents", "ao", "index", "search-index.jsonl")
	bgot, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	if string(bgot) != original {
		t.Fatalf("backup = %q, want original %q", bgot, original)
	}
	// actions.jsonl has exactly one WriteFile line.
	recs, err := readActions(ra.ActionsPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Op != "WriteFile" || !recs[0].OK {
		t.Fatalf("actions = %+v", recs)
	}
	// Re-detect: clean.
	again, _ := det.Detect(env)
	if len(again) != 0 {
		t.Fatalf("post-fix detect found %d findings", len(again))
	}
	// Undo restores byte-identical original (corrupt lines back).
	ur, err := Undo(repo, ra.RunID, true, false)
	if err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if ur.Restored != 1 {
		t.Fatalf("Undo restored = %d, want 1", ur.Restored)
	}
	restored, _ := os.ReadFile(idx)
	if string(restored) != original {
		t.Fatalf("after undo = %q, want original %q", restored, original)
	}
}

func TestKnowledgeCorruptIndexLines_RefusesAllCorrupt(t *testing.T) {
	env, repo := knowledgeTestEnv(t)
	idx := searchIndexPath(env)
	if err := os.WriteFile(idx, []byte("{bad\n{also bad\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, _ := newKnowledgeMutateCtx(t, repo, "fm-knowledge-corrupt-index-lines")
	res, err := corruptIndexLinesFixer{}.Fix(ctx, env, nil)
	if err == nil {
		t.Fatal("expected refusal when every line is corrupt")
	}
	if res.Fixed {
		t.Fatal("Fix reported Fixed despite all-corrupt refusal")
	}
}

// ---------------------------------------------------------------------------
// fm-knowledge-torn-append-line
// ---------------------------------------------------------------------------

func TestKnowledgeTornAppendLine_DetectFixUndo(t *testing.T) {
	env, repo := knowledgeTestEnv(t)
	idx := searchIndexPath(env)
	good1 := validIndexLine(".agents/learnings/a.md")
	good2 := validIndexLine(".agents/learnings/b.md")
	torn := `{"path":".agents/learnings/x.md","content":"truncat`
	// Good lines newline-terminated; the torn tail has NO terminating newline.
	original := good1 + "\n" + good2 + "\n" + torn
	if err := os.WriteFile(idx, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	det := tornAppendLineDetector{}
	findings, err := det.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 || findings[0].ID != "fm-knowledge-torn-append-line" {
		t.Fatalf("findings = %v", findings)
	}

	ctx, ra := newKnowledgeMutateCtx(t, repo, det.ID())
	res, err := tornAppendLineFixer{}.Fix(ctx, env, findings)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if !res.Fixed || res.ActionsTaken != 1 {
		t.Fatalf("Fix result: fixed=%t actions=%d", res.Fixed, res.ActionsTaken)
	}
	want := good1 + "\n" + good2 + "\n"
	got, _ := os.ReadFile(idx)
	if string(got) != want {
		t.Fatalf("post-fix index = %q, want %q", got, want)
	}
	// Backup byte-identical to torn original.
	backup := filepath.Join(ra.BackupsDir(), ".agents", "ao", "index", "search-index.jsonl")
	bgot, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	if string(bgot) != original {
		t.Fatalf("backup = %q, want %q", bgot, original)
	}
	recs, _ := readActions(ra.ActionsPath())
	if len(recs) != 1 || recs[0].Op != "WriteFile" {
		t.Fatalf("actions = %+v", recs)
	}
	// Undo restores the torn original byte-identically.
	if _, err := Undo(repo, ra.RunID, true, false); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	restored, _ := os.ReadFile(idx)
	if string(restored) != original {
		t.Fatalf("after undo = %q, want %q", restored, original)
	}
}

func TestKnowledgeTornAppendLine_RefusesNonTrailingCorruption(t *testing.T) {
	env, repo := knowledgeTestEnv(t)
	idx := searchIndexPath(env)
	good := validIndexLine(".agents/learnings/a.md")
	torn := `{"path":".agents/learnings/x.md","content":"trunc`
	// A mid-file corrupt line plus a torn tail: torn fixer must refuse and
	// defer to corrupt-index-lines.
	original := good + "\n{mid file garbage\n" + good + "\n" + torn
	if err := os.WriteFile(idx, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, _ := newKnowledgeMutateCtx(t, repo, "fm-knowledge-torn-append-line")
	res, err := tornAppendLineFixer{}.Fix(ctx, env, nil)
	if err == nil {
		t.Fatal("expected refusal on non-trailing corruption")
	}
	if res.Fixed {
		t.Fatal("Fix reported Fixed despite refusal")
	}
	// File untouched by the refusal.
	got, _ := os.ReadFile(idx)
	if string(got) != original {
		t.Fatalf("refusal mutated the file: %q", got)
	}
}

func TestKnowledgeTornAppendLine_NoFindingOnHealthyIndex(t *testing.T) {
	env, _ := knowledgeTestEnv(t)
	idx := searchIndexPath(env)
	healthy := validIndexLine(".agents/learnings/a.md") + "\n"
	if err := os.WriteFile(idx, []byte(healthy), 0o600); err != nil {
		t.Fatal(err)
	}
	findings, err := tornAppendLineDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("healthy index produced %d torn findings", len(findings))
	}
}

// ---------------------------------------------------------------------------
// fm-knowledge-orphaned-flywheel-learnings
// ---------------------------------------------------------------------------

func TestKnowledgeOrphanedFlywheelLearnings_DetectFixUndo(t *testing.T) {
	env, repo := knowledgeTestEnv(t)
	primary := filepath.Join(knowledgeBaseDir(env), "learnings")
	fallback := filepath.Join(env.CWD, ".agents", "learnings")
	if err := os.MkdirAll(primary, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(fallback, 0o755); err != nil {
		t.Fatal(err)
	}
	primaryContent := map[string]string{
		"2026-01-01-promoted-pattern.md": "primary one",
		"p2.md":                          "primary two",
	}
	fallbackContent := map[string]string{
		"f1.md": "fallback one",
		"f2.md": "fallback two",
	}
	for n, c := range primaryContent {
		if err := os.WriteFile(filepath.Join(primary, n), []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for n, c := range fallbackContent {
		if err := os.WriteFile(filepath.Join(fallback, n), []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	det := orphanedFlywheelLearningsDetector{}
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

	ctx, ra := newKnowledgeMutateCtx(t, repo, det.ID())
	res, err := orphanedFlywheelLearningsFixer{}.Fix(ctx, env, findings)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if !res.Fixed || res.ActionsTaken != 2 {
		t.Fatalf("Fix result: fixed=%t actions=%d", res.Fixed, res.ActionsTaken)
	}
	// Fallback dir holds zero learning files; primary holds all four.
	if got := listLearningFiles(fallback); len(got) != 0 {
		t.Fatalf("fallback still has %v", got)
	}
	if got := listLearningFiles(primary); len(got) != 4 {
		t.Fatalf("primary has %v, want 4 files", got)
	}
	// Moved content byte-identical.
	for n, c := range fallbackContent {
		got, err := os.ReadFile(filepath.Join(primary, n))
		if err != nil || string(got) != c {
			t.Fatalf("moved file %q content = %q err=%v, want %q", n, got, err, c)
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
	// Undo: both learnings trees restored.
	if _, err := Undo(repo, ra.RunID, true, false); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	for n, c := range fallbackContent {
		got, err := os.ReadFile(filepath.Join(fallback, n))
		if err != nil || string(got) != c {
			t.Fatalf("after undo fallback %q = %q err=%v, want %q", n, got, err, c)
		}
	}
	if got := listLearningFiles(primary); len(got) != 2 {
		t.Fatalf("after undo primary has %v, want 2", got)
	}
}

func TestKnowledgeOrphanedFlywheelLearnings_RefusesOnCollision(t *testing.T) {
	env, repo := knowledgeTestEnv(t)
	primary := filepath.Join(knowledgeBaseDir(env), "learnings")
	fallback := filepath.Join(env.CWD, ".agents", "learnings")
	if err := os.MkdirAll(primary, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(fallback, 0o755); err != nil {
		t.Fatal(err)
	}
	// Same basename in both dirs with distinct content.
	if err := os.WriteFile(filepath.Join(primary, "dup.md"), []byte("primary"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fallback, "dup.md"), []byte("fallback"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, _ := newKnowledgeMutateCtx(t, repo, "fm-knowledge-orphaned-flywheel-learnings")
	res, err := orphanedFlywheelLearningsFixer{}.Fix(ctx, env, nil)
	if err == nil {
		t.Fatal("expected refusal on basename collision")
	}
	if res.Fixed {
		t.Fatal("Fix reported Fixed despite collision refusal")
	}
	// Neither file moved.
	if got, _ := os.ReadFile(filepath.Join(fallback, "dup.md")); string(got) != "fallback" {
		t.Fatalf("fallback dup.md changed: %q", got)
	}
}

// ---------------------------------------------------------------------------
// fm-knowledge-stale-index-drift (detect-only)
// ---------------------------------------------------------------------------

func TestKnowledgeStaleIndexDrift_DetectThreeClasses(t *testing.T) {
	env, _ := knowledgeTestEnv(t)
	learnings := filepath.Join(env.CWD, ".agents", "learnings")
	if err := os.MkdirAll(learnings, 0o755); err != nil {
		t.Fatal(err)
	}
	// alive.md is fresh; drifted.md will be touched newer; uncaptured.md is
	// never indexed; dead.md is referenced by the index but absent on disk.
	alive := filepath.Join(learnings, "alive.md")
	drifted := filepath.Join(learnings, "drifted.md")
	uncaptured := filepath.Join(learnings, "uncaptured.md")
	for _, p := range []string{alive, drifted, uncaptured} {
		if err := os.WriteFile(p, []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx := searchIndexPath(env)
	indexLines := `{"path":".agents/learnings/alive.md","content":"x","modified_at":"2030-01-01T00:00:00Z"}` + "\n" +
		`{"path":".agents/learnings/drifted.md","content":"x","modified_at":"2000-01-01T00:00:00Z"}` + "\n" +
		`{"path":".agents/learnings/dead.md","content":"x","modified_at":"2030-01-01T00:00:00Z"}` + "\n"
	if err := os.WriteFile(idx, []byte(indexLines), 0o600); err != nil {
		t.Fatal(err)
	}
	// Force drifted.md mtime newer than its index modified_at (2000).
	future := time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(drifted, future, future); err != nil {
		t.Fatal(err)
	}

	det := staleIndexDriftDetector{}
	findings, err := det.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if findings[0].Remediation.AutoFixable {
		t.Fatal("stale-index-drift must be detect-only (auto_fixable=false)")
	}
	if findings[0].Remediation.Command != "ao store rebuild" {
		t.Fatalf("remediation command = %q, want `ao store rebuild`", findings[0].Remediation.Command)
	}
}

func TestKnowledgeStaleIndexDrift_FixerRefuses(t *testing.T) {
	env, repo := knowledgeTestEnv(t)
	fx := staleIndexDriftFixer{}
	if fx.AutoFixable() {
		t.Fatal("staleIndexDriftFixer.AutoFixable() must be false")
	}
	ctx, ra := newKnowledgeMutateCtx(t, repo, fx.ID())
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
// fm-knowledge-false-freshness (detect-only)
// ---------------------------------------------------------------------------

func TestKnowledgeFalseFreshness_DetectSkew(t *testing.T) {
	env, _ := knowledgeTestEnv(t)
	sessionsDir := filepath.Join(knowledgeBaseDir(env), "sessions")
	// A real session JSONL with a date backdated 60 days; file mtime forced old.
	sessionPath := filepath.Join(sessionsDir, "old-session.jsonl")
	if err := os.WriteFile(sessionPath, []byte(`{"date":"2025-01-01T00:00:00Z"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(sessionPath, old, old); err != nil {
		t.Fatal(err)
	}
	// A stray scratch file with a recent mtime — the false freshness signal.
	scratch := filepath.Join(sessionsDir, "scratch.txt")
	if err := os.WriteFile(scratch, []byte("noise"), 0o644); err != nil {
		t.Fatal(err)
	}
	recent := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(scratch, recent, recent); err != nil {
		t.Fatal(err)
	}
	// Pin the clock so fs_age < 14 days holds deterministically.
	orig := nowProvider
	nowProvider = func() time.Time { return time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { nowProvider = orig })

	det := falseFreshnessDetector{}
	findings, err := det.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if findings[0].Remediation.AutoFixable {
		t.Fatal("false-freshness must be detect-only (auto_fixable=false)")
	}
	if findings[0].Severity != "P3" {
		t.Fatalf("severity = %q, want P3", findings[0].Severity)
	}
}

func TestKnowledgeFalseFreshness_NoFindingWhenAligned(t *testing.T) {
	env, _ := knowledgeTestEnv(t)
	sessionsDir := filepath.Join(knowledgeBaseDir(env), "sessions")
	sessionPath := filepath.Join(sessionsDir, "fresh-session.jsonl")
	if err := os.WriteFile(sessionPath, []byte(`{"date":"2026-05-15T12:00:00Z"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	aligned := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(sessionPath, aligned, aligned); err != nil {
		t.Fatal(err)
	}
	orig := nowProvider
	nowProvider = func() time.Time { return time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { nowProvider = orig })

	findings, err := falseFreshnessDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("aligned store produced %d false-freshness findings", len(findings))
	}
}

func TestKnowledgeFalseFreshness_FixerRefuses(t *testing.T) {
	env, repo := knowledgeTestEnv(t)
	fx := falseFreshnessFixer{}
	if fx.AutoFixable() {
		t.Fatal("falseFreshnessFixer.AutoFixable() must be false")
	}
	ctx, ra := newKnowledgeMutateCtx(t, repo, fx.ID())
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
// Registration
// ---------------------------------------------------------------------------

func TestKnowledgeDetectorsAndFixersRegistered(t *testing.T) {
	wantDetectors := []string{
		"fm-knowledge-corrupt-index-lines",
		"fm-knowledge-false-freshness",
		"fm-knowledge-missing-substructure",
		"fm-knowledge-orphaned-flywheel-learnings",
		"fm-knowledge-stale-index-drift",
		"fm-knowledge-torn-append-line",
	}
	for _, id := range wantDetectors {
		found := false
		for _, d := range Detectors() {
			if d.ID() == id {
				found = true
				if d.Subsystem() != "knowledge" {
					t.Fatalf("detector %q subsystem = %q, want knowledge", id, d.Subsystem())
				}
			}
		}
		if !found {
			t.Fatalf("detector %q not registered", id)
		}
	}
	autoFixable := map[string]bool{
		"fm-knowledge-corrupt-index-lines":         true,
		"fm-knowledge-missing-substructure":        true,
		"fm-knowledge-orphaned-flywheel-learnings": true,
		"fm-knowledge-torn-append-line":            true,
		"fm-knowledge-stale-index-drift":           false,
		"fm-knowledge-false-freshness":             false,
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
