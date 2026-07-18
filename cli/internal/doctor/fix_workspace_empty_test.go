package doctor

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// workspaceEmptyTestEnv builds an isolated repo whose .agents/ tree holds the
// canonical fixture for the empty-dirs failure mode:
//
//	stub/                          empty            -> flagged
//	nested-empty/a/b/              only empty subs  -> flagged
//	deep/                          one deep file    -> NOT flagged
//	ao/                            empty store root -> NOT flagged (knowledge's)
//	handoff/                       empty canonical  -> NOT flagged
//	land-queue-x.stale-.../        empty stale name -> NOT flagged (GC's)
//
// It returns the DetectEnv and the repo root.
func workspaceEmptyTestEnv(t *testing.T) (*DetectEnv, string) {
	t.Helper()
	repo := t.TempDir()
	agents := filepath.Join(repo, ".agents")
	for _, dir := range []string{
		filepath.Join(agents, "stub"),
		filepath.Join(agents, "nested-empty", "a", "b"),
		filepath.Join(agents, "ao"),
		filepath.Join(agents, "handoff"),
		filepath.Join(agents, "land-queue-x.stale-20260711T221032Z"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	deepFile := filepath.Join(agents, "deep", "sub", "keep.md")
	if err := os.MkdirAll(filepath.Dir(deepFile), 0o755); err != nil {
		t.Fatalf("mkdir deep: %v", err)
	}
	if err := os.WriteFile(deepFile, []byte("content"), 0o644); err != nil {
		t.Fatalf("write deep file: %v", err)
	}
	return &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir(), Logger: os.Stderr}, repo
}

// newWorkspaceEmptyMutateCtx builds a real MutateContext rooted at repo with a
// live actions.jsonl handle, scoped to the fm-ws-empty-dirs fixer.
func newWorkspaceEmptyMutateCtx(t *testing.T, repo string) (*MutateContext, *RunArtifact) {
	t.Helper()
	ra, err := NewRunArtifact(repo, "wsemptytest", time.Now())
	if err != nil {
		t.Fatalf("NewRunArtifact: %v", err)
	}
	af, err := ra.OpenActionsFile()
	if err != nil {
		t.Fatalf("OpenActionsFile: %v", err)
	}
	t.Cleanup(func() { _ = af.Close() })
	caps := NewCapabilities("test")
	locks := NewLockManager(filepath.Join(repo, ".doctor", "locks"))
	return NewMutateContext(ra, caps, t.TempDir(), locks, af, false).WithFixer(fmWorkspaceEmptyDirsID), ra
}

func TestWorkspaceEmptyDirsRegistration(t *testing.T) {
	var foundDet bool
	for _, d := range Detectors() {
		if d.ID() == fmWorkspaceEmptyDirsID {
			foundDet = true
			if d.Subsystem() != subsystemWorkspace {
				t.Errorf("detector subsystem = %q, want %q", d.Subsystem(), subsystemWorkspace)
			}
			if d.Severity() != "P4" {
				t.Errorf("detector severity = %q, want P4", d.Severity())
			}
			if !d.QuickPath() {
				t.Error("detector QuickPath = false, want true")
			}
		}
	}
	if !foundDet {
		t.Fatalf("detector %s not registered", fmWorkspaceEmptyDirsID)
	}
	f := FixerByID(fmWorkspaceEmptyDirsID)
	if f == nil {
		t.Fatalf("fixer %s not registered", fmWorkspaceEmptyDirsID)
	}
	if got := f.Ops(); len(got) != 1 || got[0] != "Rename" {
		t.Errorf("fixer Ops = %v, want [Rename]", got)
	}
	if !f.Reversible() || !f.Idempotent() || !f.AutoFixable() {
		t.Errorf("fixer flags = reversible %v idempotent %v autofixable %v, want all true",
			f.Reversible(), f.Idempotent(), f.AutoFixable())
	}
}

func TestWorkspaceEmptyDirs_DetectAndFix(t *testing.T) {
	env, repo := workspaceEmptyTestEnv(t)
	agents := workspaceAgentsDir(env)

	det := workspaceEmptyDirsDetector{}
	findings, err := det.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1 (%+v)", len(findings), findings)
	}
	fd := findings[0]
	if fd.ID != fmWorkspaceEmptyDirsID {
		t.Fatalf("finding ID = %q, want %q", fd.ID, fmWorkspaceEmptyDirsID)
	}
	if fd.Severity != "P4" {
		t.Errorf("finding severity = %q, want P4", fd.Severity)
	}
	if !fd.Remediation.AutoFixable {
		t.Error("finding not marked AutoFixable")
	}
	// Exactly two qualifying dirs: stub and nested-empty.
	if fd.Remediation.EstimatedActions != 2 {
		t.Fatalf("EstimatedActions = %d, want 2 (title: %s)", fd.Remediation.EstimatedActions, fd.Title)
	}

	ctx, ra := newWorkspaceEmptyMutateCtx(t, repo)
	res, err := workspaceEmptyDirsFixer{}.Fix(ctx, env, findings)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if !res.Fixed {
		t.Fatal("Fix did not report Fixed")
	}
	if res.ActionsTaken != 2 {
		t.Fatalf("ActionsTaken = %d, want 2", res.ActionsTaken)
	}

	// The qualifying set is quarantined...
	for _, name := range []string{"stub", "nested-empty"} {
		if _, err := os.Stat(filepath.Join(agents, name)); !os.IsNotExist(err) {
			t.Errorf("%s still present under .agents (err=%v)", name, err)
		}
		q := filepath.Join(ra.RunDir, "quarantine", subsystemWorkspace, name)
		st, err := os.Stat(q)
		if err != nil || !st.IsDir() {
			t.Errorf("%s not quarantined at %s (err=%v)", name, q, err)
		}
	}
	// ...and every excluded dir survives untouched.
	for _, name := range []string{"deep", "ao", "handoff", "land-queue-x.stale-20260711T221032Z"} {
		st, err := os.Stat(filepath.Join(agents, name))
		if err != nil || !st.IsDir() {
			t.Errorf("excluded dir %s missing after fix (err=%v)", name, err)
		}
	}
	// The deep file's content is intact.
	got, err := os.ReadFile(filepath.Join(agents, "deep", "sub", "keep.md"))
	if err != nil || string(got) != "content" {
		t.Errorf("deep file changed: %q err=%v", got, err)
	}

	// Re-detect: clean.
	again, err := det.Detect(env)
	if err != nil {
		t.Fatalf("post-fix Detect: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("post-fix detect found %d findings, want 0 (%+v)", len(again), again)
	}
}

func TestWorkspaceEmptyDirs_FixIdempotent(t *testing.T) {
	env, repo := workspaceEmptyTestEnv(t)
	ctx, _ := newWorkspaceEmptyMutateCtx(t, repo)
	fixer := workspaceEmptyDirsFixer{}

	first, err := fixer.Fix(ctx, env, nil)
	if err != nil {
		t.Fatalf("first Fix: %v", err)
	}
	if first.ActionsTaken != 2 {
		t.Fatalf("first ActionsTaken = %d, want 2", first.ActionsTaken)
	}

	second, err := fixer.Fix(ctx, env, nil)
	if err != nil {
		t.Fatalf("second Fix: %v", err)
	}
	if !second.Fixed {
		t.Fatal("second Fix did not report Fixed")
	}
	if second.ActionsTaken != 0 {
		t.Fatalf("second ActionsTaken = %d, want 0", second.ActionsTaken)
	}
}

func TestWorkspaceEmptyDirs_SkipsDirThatGainedFiles(t *testing.T) {
	env, repo := workspaceEmptyTestEnv(t)
	agents := workspaceAgentsDir(env)

	findings, err := workspaceEmptyDirsDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 || findings[0].Remediation.EstimatedActions != 2 {
		t.Fatalf("unexpected detect result: %+v", findings)
	}

	// Between detect and fix, stub gains a file.
	if err := os.WriteFile(filepath.Join(agents, "stub", "late.md"), []byte("arrived"), 0o644); err != nil {
		t.Fatalf("write late file: %v", err)
	}

	ctx, _ := newWorkspaceEmptyMutateCtx(t, repo)
	res, err := workspaceEmptyDirsFixer{}.Fix(ctx, env, findings)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if !res.Fixed {
		t.Fatal("Fix did not report Fixed")
	}
	// Only nested-empty is still qualifying.
	if res.ActionsTaken != 1 {
		t.Fatalf("ActionsTaken = %d, want 1", res.ActionsTaken)
	}
	if st, err := os.Stat(filepath.Join(agents, "stub")); err != nil || !st.IsDir() {
		t.Errorf("stub (which gained files) was quarantined (err=%v)", err)
	}
	if _, err := os.Stat(filepath.Join(agents, "nested-empty")); !os.IsNotExist(err) {
		t.Errorf("nested-empty not quarantined (err=%v)", err)
	}
}

func TestWorkspaceEmptyDirs_DetectMissingBaseClean(t *testing.T) {
	repo := t.TempDir() // no .agents at all
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir(), Logger: os.Stderr}
	findings, err := workspaceEmptyDirsDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect on missing base: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("findings = %d, want 0", len(findings))
	}
}

func TestWorkspaceEmptyDirClaimed(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"ao", true},
		// Every canonical alias target is claimed.
		{"postmortem", true},
		{"pre-mortem-checks", true},
		{"handoff", true},
		{"retro", true},
		{"proofs", true},
		{"tests", true},
		// Stale/retry names belong to the GC failure mode.
		{"land-queue-age-h433.19.stale-20260711T221032Z", true},
		{"land-queue-age-h433.22-native-retry2", true},
		// Unclaimed names.
		{"stub", false},
		{"scratch", false},
		// Alias KEYS (drifted spellings) are not claimed — an empty drifted
		// dir is quarantinable debris.
		{"handoffs", false},
		{"post-mortems", false},
	}
	for _, tt := range tests {
		if got := workspaceEmptyDirClaimed(tt.name); got != tt.want {
			t.Errorf("workspaceEmptyDirClaimed(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
