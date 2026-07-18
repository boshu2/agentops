package doctor

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Real debris names captured from live land-queue GC backlog (2026-07-11).
const (
	staleDirExpired = "land-queue-age-h433.19-native.stale-20260711T221704Z"
	retryDirExpired = "land-queue-age-h433.22-native-retry2"
	retryDirEmpty   = "land-queue-age-h433.27-native-retry3"
	staleDirYoung   = "land-queue-age-h433.40-native.stale-20260717T000000Z"
	runtimeDir      = "land-queue-runtime"
)

// newWorkspaceStaleMutateCtx builds a real MutateContext rooted at repo,
// scoped to the workspace stale-queue fixer, with a live actions.jsonl handle.
func newWorkspaceStaleMutateCtx(t *testing.T, repo string) (*MutateContext, *RunArtifact) {
	t.Helper()
	ra, err := NewRunArtifact(repo, "wsstale", time.Now())
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
	return NewMutateContext(ra, caps, t.TempDir(), locks, af, false).WithFixer(workspaceStaleQueueDirsID), ra
}

// workspaceStaleFixtureEnv builds an .agents tree holding the five real-name
// fixture dirs: two expired matched dirs with content, one empty matched dir,
// one young matched dir, and one old non-matching dir. mtimes are controlled
// with os.Chtimes relative to the real clock (the detector uses time.Now).
func workspaceStaleFixtureEnv(t *testing.T) (*DetectEnv, string) {
	t.Helper()
	repo := t.TempDir()
	agents := filepath.Join(repo, ".agents")
	old := time.Now().Add(-30 * 24 * time.Hour)
	young := time.Now().Add(-1 * time.Hour)

	writeWorkspaceFile(t, filepath.Join(agents, staleDirExpired, "verdict.json"), "expired-stale", old)
	writeWorkspaceFile(t, filepath.Join(agents, retryDirExpired, "nested", "log.txt"), "expired-retry", old)
	if err := os.MkdirAll(filepath.Join(agents, retryDirEmpty), 0o755); err != nil {
		t.Fatal(err)
	}
	writeWorkspaceFile(t, filepath.Join(agents, staleDirYoung, "fresh.json"), "young", young)
	writeWorkspaceFile(t, filepath.Join(agents, runtimeDir, "state.json"), "runtime", old)

	return &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir(), Logger: os.Stderr}, repo
}

func TestWorkspaceStaleQueueDirs_DetectFinding(t *testing.T) {
	env, _ := workspaceStaleFixtureEnv(t)

	det := workspaceStaleQueueDirsDetector{}
	findings, err := det.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1 summarizing finding", len(findings))
	}
	f := findings[0]
	if f.ID != "fm-ws-stale-queue-dirs" {
		t.Errorf("finding ID = %q, want fm-ws-stale-queue-dirs", f.ID)
	}
	if f.Subsystem != "workspace" {
		t.Errorf("subsystem = %q, want workspace", f.Subsystem)
	}
	if f.Severity != "P3" {
		t.Errorf("severity = %q, want P3", f.Severity)
	}
	if f.Confidence != 1.0 {
		t.Errorf("confidence = %v, want 1.0", f.Confidence)
	}
	if !f.Remediation.AutoFixable {
		t.Error("finding must be auto-fixable")
	}
	// Expired matched set = explicit-stale + retry-chain + empty matched dir.
	if f.Remediation.EstimatedActions != 3 {
		t.Errorf("EstimatedActions = %d, want 3", f.Remediation.EstimatedActions)
	}
	if want := "ao doctor --fix --only fm-ws-stale-queue-dirs"; f.Remediation.Command != want {
		t.Errorf("remediation command = %q, want %q", f.Remediation.Command, want)
	}
	if f.Evidence.Query == "" {
		t.Error("evidence query one-liner is empty")
	}
}

// TestWorkspaceStaleQueueDirs_DetectSelection is the table-driven single-dir
// selection matrix: name match x age decides the flag.
func TestWorkspaceStaleQueueDirs_DetectSelection(t *testing.T) {
	tests := []struct {
		name     string
		dirName  string
		empty    bool
		mtimeAge time.Duration
		want     int // findings
	}{
		{"matched and expired", staleDirExpired, false, 30 * 24 * time.Hour, 1},
		{"matched retry and expired", retryDirExpired, false, 15 * 24 * time.Hour, 1},
		{"matched but empty counts as expired", retryDirEmpty, true, 0, 1},
		{"matched but young", staleDirYoung, false, time.Hour, 0},
		{"non-matching even when old", runtimeDir, false, 90 * 24 * time.Hour, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := t.TempDir()
			dir := filepath.Join(repo, ".agents", tt.dirName)
			if tt.empty {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
			} else {
				writeWorkspaceFile(t, filepath.Join(dir, "f.txt"), "x", time.Now().Add(-tt.mtimeAge))
			}
			env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir(), Logger: os.Stderr}
			findings, err := workspaceStaleQueueDirsDetector{}.Detect(env)
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			if len(findings) != tt.want {
				t.Fatalf("findings = %d, want %d", len(findings), tt.want)
			}
			if tt.want == 1 && findings[0].Remediation.EstimatedActions != 1 {
				t.Errorf("EstimatedActions = %d, want 1", findings[0].Remediation.EstimatedActions)
			}
		})
	}
}

func TestWorkspaceStaleQueueDirs_DetectNoAgentsDir(t *testing.T) {
	repo := t.TempDir()
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir(), Logger: os.Stderr}
	findings, err := workspaceStaleQueueDirsDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("missing .agents produced %d findings, want 0", len(findings))
	}
}

func TestWorkspaceStaleQueueDirs_TTLOverride(t *testing.T) {
	t.Setenv(workspaceTTLEnvVar, "1")
	repo := t.TempDir()
	// 2 days old: expired under the 1-day override, young under the 14d default.
	writeWorkspaceFile(t, filepath.Join(repo, ".agents", retryDirExpired, "f.txt"), "x", time.Now().Add(-48*time.Hour))
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir(), Logger: os.Stderr}
	findings, err := workspaceStaleQueueDirsDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1 under AO_DOCTOR_WS_TTL_DAYS=1", len(findings))
	}
}

func TestWorkspaceStaleQueueDirs_FixQuarantinesExpiredOnly(t *testing.T) {
	env, repo := workspaceStaleFixtureEnv(t)
	agents := filepath.Join(repo, ".agents")

	det := workspaceStaleQueueDirsDetector{}
	findings, err := det.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	ctx, ra := newWorkspaceStaleMutateCtx(t, repo)
	res, err := workspaceStaleQueueDirsFixer{}.Fix(ctx, env, findings)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if !res.Fixed {
		t.Fatal("Fix did not report Fixed")
	}
	if res.ActionsTaken != 3 {
		t.Fatalf("ActionsTaken = %d, want 3", res.ActionsTaken)
	}
	if len(res.Skipped) != 0 {
		t.Fatalf("Skipped = %v, want none", res.Skipped)
	}

	// Expired matched set gone from .agents/, present under quarantine/workspace/.
	quarantine := filepath.Join(ra.QuarantineDir(), "workspace")
	for _, name := range []string{staleDirExpired, retryDirExpired, retryDirEmpty} {
		if _, err := os.Stat(filepath.Join(agents, name)); !os.IsNotExist(err) {
			t.Errorf("%s still present in .agents (err=%v)", name, err)
		}
		st, err := os.Stat(filepath.Join(quarantine, name))
		if err != nil || !st.IsDir() {
			t.Errorf("%s not quarantined as a directory (err=%v)", name, err)
		}
	}
	// Quarantined content survived byte-for-byte.
	got, err := os.ReadFile(filepath.Join(quarantine, staleDirExpired, "verdict.json"))
	if err != nil || string(got) != "expired-stale" {
		t.Errorf("quarantined verdict.json = %q err=%v, want %q", got, err, "expired-stale")
	}
	got, err = os.ReadFile(filepath.Join(quarantine, retryDirExpired, "nested", "log.txt"))
	if err != nil || string(got) != "expired-retry" {
		t.Errorf("quarantined nested log.txt = %q err=%v, want %q", got, err, "expired-retry")
	}

	// Young matched dir and non-matching dir untouched.
	for name, content := range map[string]string{
		filepath.Join(staleDirYoung, "fresh.json"): "young",
		filepath.Join(runtimeDir, "state.json"):    "runtime",
	} {
		got, err := os.ReadFile(filepath.Join(agents, name))
		if err != nil || string(got) != content {
			t.Errorf("untouched file %s = %q err=%v, want %q", name, got, err, content)
		}
	}

	// actions.jsonl: exactly three OK Rename records into quarantine/workspace/.
	recs, err := readActions(ra.ActionsPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 3 {
		t.Fatalf("action records = %d, want 3", len(recs))
	}
	for _, r := range recs {
		if r.Op != "Rename" || !r.OK || r.RenameTo == "" {
			t.Errorf("bad rename record: %+v", r)
		}
	}

	// Idempotency: second detect is clean, second fix takes zero actions.
	again, err := det.Detect(env)
	if err != nil {
		t.Fatalf("post-fix Detect: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("post-fix detect found %d findings, want 0", len(again))
	}
	res2, err := workspaceStaleQueueDirsFixer{}.Fix(ctx, env, nil)
	if err != nil {
		t.Fatalf("second Fix: %v", err)
	}
	if !res2.Fixed || res2.ActionsTaken != 0 {
		t.Fatalf("second Fix: fixed=%t actions=%d, want fixed with 0 actions", res2.Fixed, res2.ActionsTaken)
	}
}

func TestWorkspaceStaleQueueDirs_UndoRestoresQuarantinedDirs(t *testing.T) {
	env, repo := workspaceStaleFixtureEnv(t)
	agents := filepath.Join(repo, ".agents")

	ctx, ra := newWorkspaceStaleMutateCtx(t, repo)
	res, err := workspaceStaleQueueDirsFixer{}.Fix(ctx, env, nil)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if res.ActionsTaken != 3 {
		t.Fatalf("ActionsTaken = %d, want 3", res.ActionsTaken)
	}

	ur, err := Undo(repo, ra.RunID, true, false)
	if err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if ur.Restored != 3 {
		t.Fatalf("Undo restored = %d, want 3", ur.Restored)
	}
	// Directories and content are back in .agents.
	got, err := os.ReadFile(filepath.Join(agents, staleDirExpired, "verdict.json"))
	if err != nil || string(got) != "expired-stale" {
		t.Fatalf("after undo verdict.json = %q err=%v, want %q", got, err, "expired-stale")
	}
	st, err := os.Stat(filepath.Join(agents, retryDirEmpty))
	if err != nil || !st.IsDir() {
		t.Fatalf("after undo %s not restored (err=%v)", retryDirEmpty, err)
	}
}

func TestWorkspaceStaleQueueDirs_FixNoAgentsDir(t *testing.T) {
	repo := t.TempDir()
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir(), Logger: os.Stderr}
	ctx, ra := newWorkspaceStaleMutateCtx(t, repo)
	res, err := workspaceStaleQueueDirsFixer{}.Fix(ctx, env, nil)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if !res.Fixed || res.ActionsTaken != 0 {
		t.Fatalf("Fix on missing .agents: fixed=%t actions=%d, want fixed with 0 actions", res.Fixed, res.ActionsTaken)
	}
	recs, err := readActions(ra.ActionsPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 0 {
		t.Fatalf("wrote %d action records on missing .agents, want 0", len(recs))
	}
}

func TestWorkspaceStaleQueueDirs_Registration(t *testing.T) {
	var det Detector
	for _, d := range Detectors() {
		if d.ID() == workspaceStaleQueueDirsID {
			det = d
		}
	}
	if det == nil {
		t.Fatalf("detector %q not registered", workspaceStaleQueueDirsID)
	}
	if det.Subsystem() != "workspace" {
		t.Errorf("detector subsystem = %q, want workspace", det.Subsystem())
	}
	if det.Severity() != "P3" {
		t.Errorf("detector severity = %q, want P3", det.Severity())
	}
	if !det.QuickPath() {
		t.Error("detector QuickPath() = false, want true")
	}
	if det.OnlineRequired() {
		t.Error("detector OnlineRequired() = true, want false")
	}

	fx := FixerByID(workspaceStaleQueueDirsID)
	if fx == nil {
		t.Fatalf("fixer %q not registered", workspaceStaleQueueDirsID)
	}
	if !fx.AutoFixable() {
		t.Error("fixer AutoFixable() = false, want true")
	}
	if !fx.Reversible() {
		t.Error("fixer Reversible() = false, want true")
	}
	if !fx.Idempotent() {
		t.Error("fixer Idempotent() = false, want true")
	}
	if ops := fx.Ops(); len(ops) != 1 || ops[0] != "Rename" {
		t.Errorf("fixer Ops() = %v, want [Rename]", ops)
	}
	if wt := fx.WritesTo(); len(wt) != 1 || wt[0] != ".agents" {
		t.Errorf("fixer WritesTo() = %v, want [.agents]", wt)
	}
	if len(fx.Preconditions()) == 0 {
		t.Error("fixer Preconditions() is empty")
	}
}
