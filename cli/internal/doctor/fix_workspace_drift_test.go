package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// namingDriftEnv builds an isolated repo with an `.agents/` workspace root and
// returns a DetectEnv plus the repo root.
func namingDriftEnv(t *testing.T) (*DetectEnv, string) {
	t.Helper()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".agents"), 0o755); err != nil {
		t.Fatalf("mkdir .agents: %v", err)
	}
	return &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir(), Logger: os.Stderr}, repo
}

// newNamingDriftCtx builds a real MutateContext rooted at repo, scoped to the
// naming-drift fixer, with a live actions.jsonl handle. It uses the
// production capabilities verbatim: `.agents` is a canonical write scope, so
// no test-side scope extension is needed (and none is applied — this test
// proves the production envelope admits the workspace fixers).
func newNamingDriftCtx(t *testing.T, repo string, dryRun bool) (*MutateContext, *RunArtifact) {
	t.Helper()
	ra, err := NewRunArtifact(repo, "wsdrift", time.Now())
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
	return NewMutateContext(ra, caps, t.TempDir(), locks, af, dryRun).WithFixer(fmWorkspaceNamingDriftID), ra
}

// writeDriftFile writes content to path, creating parent directories.
func writeDriftFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// readDriftFile reads path and fails the test on error.
func readDriftFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestWorkspaceNamingDrift_DetectOnePerAlias(t *testing.T) {
	env, repo := namingDriftEnv(t)
	agents := filepath.Join(repo, ".agents")
	// Two alias dirs: one with files (incl. nested), one empty.
	writeDriftFile(t, filepath.Join(agents, "post-mortem", "a.md"), "alpha")
	writeDriftFile(t, filepath.Join(agents, "post-mortem", "sub", "nested.md"), "nested")
	if err := os.MkdirAll(filepath.Join(agents, "test"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Canonical dirs and unrelated dirs must not be flagged.
	writeDriftFile(t, filepath.Join(agents, "retro", "r.md"), "retro")
	writeDriftFile(t, filepath.Join(agents, "learnings", "l.md"), "learn")
	// An alias name occupied by a symlink is not a directory-naming drift.
	if err := os.Symlink(filepath.Join(agents, "retro"), filepath.Join(agents, "proof")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	findings, err := workspaceNamingDriftDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("findings = %d, want 2 (one per alias dir): %+v", len(findings), findings)
	}
	// Deterministic alias-sorted order; every finding carries the detector ID.
	for _, f := range findings {
		if f.ID != fmWorkspaceNamingDriftID {
			t.Errorf("finding ID = %q, want %q", f.ID, fmWorkspaceNamingDriftID)
		}
		if f.Subsystem != "workspace" || f.Severity != "P3" {
			t.Errorf("finding subsystem/severity = %q/%q, want workspace/P3", f.Subsystem, f.Severity)
		}
		if !f.Remediation.AutoFixable {
			t.Error("finding not marked auto-fixable")
		}
	}
	if findings[0].Evidence.File != filepath.Join(".agents", "post-mortem") {
		t.Errorf("finding 0 evidence file = %q, want .agents/post-mortem", findings[0].Evidence.File)
	}
	if want := "2 transitive regular file(s) under .agents/post-mortem"; !strings.Contains(findings[0].Evidence.Query, want) {
		t.Errorf("finding 0 evidence query = %q, want it to contain %q", findings[0].Evidence.Query, want)
	}
	if findings[0].Remediation.EstimatedActions != 3 {
		t.Errorf("finding 0 estimated actions = %d, want 3 (2 files + quarantine)", findings[0].Remediation.EstimatedActions)
	}
	if findings[1].Evidence.File != filepath.Join(".agents", "test") {
		t.Errorf("finding 1 evidence file = %q, want .agents/test", findings[1].Evidence.File)
	}
	if want := "0 transitive regular file(s)"; !strings.Contains(findings[1].Evidence.Query, want) {
		t.Errorf("finding 1 evidence query = %q, want it to contain %q", findings[1].Evidence.Query, want)
	}
}

func TestWorkspaceNamingDrift_DetectNoWorkspace(t *testing.T) {
	repo := t.TempDir() // no .agents at all
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir(), Logger: os.Stderr}
	findings, err := workspaceNamingDriftDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("missing .agents produced %d findings", len(findings))
	}
}

// Scenario (a) + (d): clean merge into a pre-existing canonical dir, alias dir
// quarantined, second detect clean, and undo restores the original tree.
func TestWorkspaceNamingDrift_CleanMergeQuarantineUndo(t *testing.T) {
	env, repo := namingDriftEnv(t)
	agents := filepath.Join(repo, ".agents")
	alias := filepath.Join(agents, "post-mortems")
	canonical := filepath.Join(agents, "postmortem")
	writeDriftFile(t, filepath.Join(alias, "a.md"), "alpha body")
	writeDriftFile(t, filepath.Join(alias, "sub", "nested.md"), "nested body")
	// Canonical pre-exists with an unrelated file that must remain.
	writeDriftFile(t, filepath.Join(canonical, "unrelated.md"), "keep me")

	det := workspaceNamingDriftDetector{}
	findings, err := det.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 || findings[0].ID != fmWorkspaceNamingDriftID {
		t.Fatalf("findings = %+v, want exactly one %s", findings, fmWorkspaceNamingDriftID)
	}

	ctx, ra := newNamingDriftCtx(t, repo, false)
	res, err := workspaceNamingDriftFixer{}.Fix(ctx, env, findings)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if !res.Fixed {
		t.Errorf("Fix not reported Fixed; skipped = %v", res.Skipped)
	}
	if res.ActionsTaken != 3 {
		t.Errorf("ActionsTaken = %d, want 3 (a.md, sub, quarantine)", res.ActionsTaken)
	}
	if len(res.Skipped) != 0 {
		t.Errorf("Skipped = %v, want empty", res.Skipped)
	}
	// Exact contents after the merge.
	if got := readDriftFile(t, filepath.Join(canonical, "unrelated.md")); got != "keep me" {
		t.Errorf("unrelated.md = %q, want %q", got, "keep me")
	}
	if got := readDriftFile(t, filepath.Join(canonical, "a.md")); got != "alpha body" {
		t.Errorf("moved a.md = %q, want %q", got, "alpha body")
	}
	if got := readDriftFile(t, filepath.Join(canonical, "sub", "nested.md")); got != "nested body" {
		t.Errorf("moved sub/nested.md = %q, want %q", got, "nested body")
	}
	// Alias dir gone from the workspace, present (empty) in quarantine.
	if _, err := os.Lstat(alias); !os.IsNotExist(err) {
		t.Errorf("alias dir still present after clean merge (err=%v)", err)
	}
	qdir := filepath.Join(ctx.RunDir, "quarantine", "workspace", "post-mortems")
	qinfo, err := os.Stat(qdir)
	if err != nil || !qinfo.IsDir() {
		t.Fatalf("quarantined alias dir missing: %v", err)
	}
	if qents, _ := os.ReadDir(qdir); len(qents) != 0 {
		t.Errorf("quarantined alias dir not empty: %d entries", len(qents))
	}
	// Journal: three OK Rename records.
	recs, err := readActions(ra.ActionsPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 3 {
		t.Fatalf("actions = %d, want 3: %+v", len(recs), recs)
	}
	for _, r := range recs {
		if r.Op != "Rename" || r.RenameTo == "" || !r.OK {
			t.Errorf("bad rename record: %+v", r)
		}
	}
	// Scenario (d): second detect finds nothing after a clean merge.
	again, err := det.Detect(env)
	if err != nil {
		t.Fatalf("re-Detect: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("post-fix detect found %d findings, want 0", len(again))
	}
	// Undo restores the exact original tree.
	ur, err := Undo(repo, ra.RunID, true, false)
	if err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if ur.Restored != 3 {
		t.Errorf("Undo restored = %d, want 3", ur.Restored)
	}
	if got := readDriftFile(t, filepath.Join(alias, "a.md")); got != "alpha body" {
		t.Errorf("after undo a.md = %q, want %q", got, "alpha body")
	}
	if got := readDriftFile(t, filepath.Join(alias, "sub", "nested.md")); got != "nested body" {
		t.Errorf("after undo sub/nested.md = %q, want %q", got, "nested body")
	}
	if ents, _ := os.ReadDir(canonical); len(ents) != 1 {
		t.Errorf("after undo canonical has %d entries, want only unrelated.md", len(ents))
	}
}

// Scenario (b): a same-named file in both dirs is skipped, both copies stay
// byte-intact, and the alias dir is left in place.
func TestWorkspaceNamingDrift_CollisionSkipsBothIntact(t *testing.T) {
	env, repo := namingDriftEnv(t)
	agents := filepath.Join(repo, ".agents")
	alias := filepath.Join(agents, "proof")
	canonical := filepath.Join(agents, "proofs")
	writeDriftFile(t, filepath.Join(alias, "dup.md"), "alias body")
	writeDriftFile(t, filepath.Join(alias, "ok.md"), "ok body")
	writeDriftFile(t, filepath.Join(canonical, "dup.md"), "canonical body")

	ctx, _ := newNamingDriftCtx(t, repo, false)
	res, err := workspaceNamingDriftFixer{}.Fix(ctx, env, nil)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if res.Fixed {
		t.Error("Fix reported Fixed despite a skipped collision")
	}
	if res.ActionsTaken != 1 {
		t.Errorf("ActionsTaken = %d, want 1 (only ok.md)", res.ActionsTaken)
	}
	if len(res.Skipped) == 0 {
		t.Fatal("Skipped is empty, want the collision reported")
	}
	wantPath := filepath.Join(".agents", "proof", "dup.md")
	if !strings.Contains(res.Skipped[0], wantPath) {
		t.Errorf("Skipped[0] = %q, want it to name %q", res.Skipped[0], wantPath)
	}
	// BOTH files byte-intact after the fix.
	if got := readDriftFile(t, filepath.Join(alias, "dup.md")); got != "alias body" {
		t.Errorf("alias dup.md = %q, want %q", got, "alias body")
	}
	if got := readDriftFile(t, filepath.Join(canonical, "dup.md")); got != "canonical body" {
		t.Errorf("canonical dup.md = %q, want %q", got, "canonical body")
	}
	// The clean entry moved; the alias dir was NOT quarantined.
	if got := readDriftFile(t, filepath.Join(canonical, "ok.md")); got != "ok body" {
		t.Errorf("moved ok.md = %q, want %q", got, "ok body")
	}
	if info, err := os.Stat(alias); err != nil || !info.IsDir() {
		t.Errorf("alias dir with a skipped entry must remain (err=%v)", err)
	}
	// The retained alias dir is itself noted in Skipped.
	last := res.Skipped[len(res.Skipped)-1]
	if !strings.Contains(last, filepath.Join(".agents", "proof")) || !strings.Contains(last, "left in place") {
		t.Errorf("Skipped tail = %q, want a left-in-place note for .agents/proof", last)
	}
}

// Scenario (c): the canonical dir does not exist; Rename creates it and every
// file arrives byte-intact.
func TestWorkspaceNamingDrift_CanonicalAbsentCreated(t *testing.T) {
	env, repo := namingDriftEnv(t)
	agents := filepath.Join(repo, ".agents")
	alias := filepath.Join(agents, "retros")
	canonical := filepath.Join(agents, "retro")
	writeDriftFile(t, filepath.Join(alias, "r1.md"), "retro one")
	writeDriftFile(t, filepath.Join(alias, "r2.md"), "retro two")

	ctx, _ := newNamingDriftCtx(t, repo, false)
	res, err := workspaceNamingDriftFixer{}.Fix(ctx, env, nil)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if !res.Fixed || len(res.Skipped) != 0 {
		t.Errorf("Fix result: fixed=%t skipped=%v, want clean", res.Fixed, res.Skipped)
	}
	if res.ActionsTaken != 3 {
		t.Errorf("ActionsTaken = %d, want 3 (r1, r2, quarantine)", res.ActionsTaken)
	}
	if got := readDriftFile(t, filepath.Join(canonical, "r1.md")); got != "retro one" {
		t.Errorf("r1.md = %q, want %q", got, "retro one")
	}
	if got := readDriftFile(t, filepath.Join(canonical, "r2.md")); got != "retro two" {
		t.Errorf("r2.md = %q, want %q", got, "retro two")
	}
	if _, err := os.Lstat(alias); !os.IsNotExist(err) {
		t.Errorf("alias dir still present (err=%v)", err)
	}
	// Idempotency: a second fix run is a no-op.
	res2, err := workspaceNamingDriftFixer{}.Fix(ctx, env, nil)
	if err != nil {
		t.Fatalf("second Fix: %v", err)
	}
	if !res2.Fixed || res2.ActionsTaken != 0 || len(res2.Skipped) != 0 {
		t.Errorf("second Fix: fixed=%t actions=%d skipped=%v, want clean no-op", res2.Fixed, res2.ActionsTaken, res2.Skipped)
	}
}

// A symlink entry inside the alias dir is never moved: moving it could change
// what it resolves to. It is reported in Skipped and the alias dir remains.
func TestWorkspaceNamingDrift_SymlinkEntrySkipped(t *testing.T) {
	env, repo := namingDriftEnv(t)
	agents := filepath.Join(repo, ".agents")
	alias := filepath.Join(agents, "handoffs")
	canonical := filepath.Join(agents, "handoff")
	writeDriftFile(t, filepath.Join(alias, "real.md"), "real body")
	linkTarget := filepath.Join(repo, "outside.txt")
	writeDriftFile(t, linkTarget, "outside")
	if err := os.Symlink(linkTarget, filepath.Join(alias, "link.md")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	ctx, _ := newNamingDriftCtx(t, repo, false)
	res, err := workspaceNamingDriftFixer{}.Fix(ctx, env, nil)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if res.Fixed {
		t.Error("Fix reported Fixed despite a skipped symlink")
	}
	if res.ActionsTaken != 1 {
		t.Errorf("ActionsTaken = %d, want 1 (only real.md)", res.ActionsTaken)
	}
	found := false
	for _, s := range res.Skipped {
		if strings.Contains(s, filepath.Join(".agents", "handoffs", "link.md")) {
			found = true
		}
	}
	if !found {
		t.Errorf("Skipped = %v, want the symlink entry named", res.Skipped)
	}
	if got := readDriftFile(t, filepath.Join(canonical, "real.md")); got != "real body" {
		t.Errorf("moved real.md = %q, want %q", got, "real body")
	}
	// Symlink left in place and still resolving.
	if got := readDriftFile(t, filepath.Join(alias, "link.md")); got != "outside" {
		t.Errorf("symlink no longer resolves to original content: %q", got)
	}
	if info, err := os.Lstat(filepath.Join(alias, "link.md")); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("link.md is no longer a symlink (err=%v)", err)
	}
}

func TestWorkspaceNamingDrift_DryRunTouchesNothing(t *testing.T) {
	env, repo := namingDriftEnv(t)
	agents := filepath.Join(repo, ".agents")
	writeDriftFile(t, filepath.Join(agents, "retros", "r1.md"), "retro one")
	writeDriftFile(t, filepath.Join(agents, "retros", "sub", "n.md"), "nested")

	ctx, ra := newNamingDriftCtx(t, repo, true)
	res, err := workspaceNamingDriftFixer{}.Fix(ctx, env, nil)
	if err != nil {
		t.Fatalf("dry-run Fix: %v", err)
	}
	if len(res.Skipped) != 0 {
		t.Errorf("dry-run Skipped = %v, want empty", res.Skipped)
	}
	// Nothing on disk changed.
	if got := readDriftFile(t, filepath.Join(agents, "retros", "r1.md")); got != "retro one" {
		t.Errorf("dry-run moved r1.md: %q", got)
	}
	if got := readDriftFile(t, filepath.Join(agents, "retros", "sub", "n.md")); got != "nested" {
		t.Errorf("dry-run moved sub/n.md: %q", got)
	}
	if _, err := os.Stat(filepath.Join(agents, "retro")); !os.IsNotExist(err) {
		t.Errorf("dry-run created the canonical dir (err=%v)", err)
	}
	recs, err := readActions(ra.ActionsPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 0 {
		t.Errorf("dry-run wrote %d action records, want 0", len(recs))
	}
}

func TestWorkspaceNamingDrift_Registered(t *testing.T) {
	var det Detector
	for _, d := range Detectors() {
		if d.ID() == fmWorkspaceNamingDriftID {
			det = d
		}
	}
	if det == nil {
		t.Fatalf("detector %s not registered", fmWorkspaceNamingDriftID)
	}
	if det.Subsystem() != "workspace" || det.Severity() != "P3" || !det.QuickPath() || det.OnlineRequired() {
		t.Errorf("detector contract: subsystem=%q severity=%q quick=%t online=%t",
			det.Subsystem(), det.Severity(), det.QuickPath(), det.OnlineRequired())
	}
	fx := FixerByID(fmWorkspaceNamingDriftID)
	if fx == nil {
		t.Fatalf("fixer %s not registered", fmWorkspaceNamingDriftID)
	}
	if !fx.AutoFixable() || !fx.Reversible() || !fx.Idempotent() {
		t.Errorf("fixer contract: auto=%t reversible=%t idempotent=%t",
			fx.AutoFixable(), fx.Reversible(), fx.Idempotent())
	}
	if ops := fx.Ops(); len(ops) != 1 || ops[0] != "Rename" {
		t.Errorf("fixer ops = %v, want [Rename]", ops)
	}
}
