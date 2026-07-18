package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeWorkspaceFile creates a file with the given content and mtime,
// creating parent directories as needed.
func writeWorkspaceFile(t *testing.T, path, content string, mtime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("chtimes %s: %v", path, err)
	}
}

func TestWorkspaceAgentsDir(t *testing.T) {
	env := &DetectEnv{RepoRoot: "/repo/root", CWD: "/somewhere/else"}
	got := workspaceAgentsDir(env)
	want := filepath.Join("/repo/root", ".agents")
	if got != want {
		t.Errorf("workspaceAgentsDir = %q, want %q", got, want)
	}
}

func TestWorkspaceDirInventory(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()

	t1 := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 7, 5, 12, 30, 0, 0, time.UTC)
	t3 := time.Date(2026, 7, 3, 8, 0, 0, 0, time.UTC)

	// alpha: two regular files (one nested), plus a symlink to a decoy file
	// that must contribute nothing.
	writeWorkspaceFile(t, filepath.Join(base, "alpha", "a.txt"), "aaaaa", t1) // 5 bytes
	writeWorkspaceFile(t, filepath.Join(base, "alpha", "sub", "b.txt"), "bbb", t2)
	// Decoy: much larger and much newer than anything in the fixture. If any
	// symlink were followed, FileCount/ByteSize/NewestMTime would all drift.
	decoy := filepath.Join(outside, "decoy.bin")
	writeWorkspaceFile(t, decoy, "0123456789012345678901234567890123456789", time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))
	if err := os.Symlink(decoy, filepath.Join(base, "alpha", "link-file")); err != nil {
		t.Fatalf("symlink file: %v", err)
	}

	// beta: empty directory.
	if err := os.MkdirAll(filepath.Join(base, "beta"), 0o755); err != nil {
		t.Fatal(err)
	}

	// gamma: one file.
	writeWorkspaceFile(t, filepath.Join(base, "gamma", "g.bin"), "ggggggg", t3) // 7 bytes

	// Top-level symlink to a directory: must not be inventoried.
	if err := os.Symlink(filepath.Join(base, "alpha"), filepath.Join(base, "link-dir")); err != nil {
		t.Fatalf("symlink dir: %v", err)
	}
	// Top-level regular file: not a directory, skipped.
	writeWorkspaceFile(t, filepath.Join(base, "plain.txt"), "x", t1)

	got, err := workspaceDirInventory(base)
	if err != nil {
		t.Fatalf("workspaceDirInventory: %v", err)
	}
	want := []workspaceDirInfo{
		// alpha's symlink entry is counted as an OtherEntry, never followed.
		{Name: "alpha", FileCount: 2, ByteSize: 8, NewestMTime: t2, OtherEntries: 1},
		{Name: "beta", FileCount: 0, ByteSize: 0, NewestMTime: time.Time{}},
		{Name: "gamma", FileCount: 1, ByteSize: 7, NewestMTime: t3},
	}
	if len(got) != len(want) {
		t.Fatalf("inventory length = %d, want %d (%+v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Name != want[i].Name {
			t.Errorf("entry %d Name = %q, want %q", i, got[i].Name, want[i].Name)
		}
		if got[i].FileCount != want[i].FileCount {
			t.Errorf("%s FileCount = %d, want %d", want[i].Name, got[i].FileCount, want[i].FileCount)
		}
		if got[i].ByteSize != want[i].ByteSize {
			t.Errorf("%s ByteSize = %d, want %d", want[i].Name, got[i].ByteSize, want[i].ByteSize)
		}
		if !got[i].NewestMTime.Equal(want[i].NewestMTime) {
			t.Errorf("%s NewestMTime = %v, want %v", want[i].Name, got[i].NewestMTime, want[i].NewestMTime)
		}
		if got[i].OtherEntries != want[i].OtherEntries {
			t.Errorf("%s OtherEntries = %d, want %d", want[i].Name, got[i].OtherEntries, want[i].OtherEntries)
		}
		if got[i].WalkErrs != 0 {
			t.Errorf("%s WalkErrs = %d, want 0", want[i].Name, got[i].WalkErrs)
		}
	}
}

func TestWorkspaceDirInventory_PermissionErrorSkipped(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; permission denial is not enforceable")
	}
	base := t.TempDir()
	t1 := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	writeWorkspaceFile(t, filepath.Join(base, "alpha", "a.txt"), "aa", t1) // 2 bytes
	writeWorkspaceFile(t, filepath.Join(base, "alpha", "locked", "hidden.txt"), "hhhh", t1.Add(time.Hour))
	locked := filepath.Join(base, "alpha", "locked")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	got, err := workspaceDirInventory(base)
	if err != nil {
		t.Fatalf("workspaceDirInventory: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("inventory length = %d, want 1 (%+v)", len(got), got)
	}
	// The unreadable subtree is skipped: only a.txt is counted.
	if got[0].FileCount != 1 {
		t.Errorf("FileCount = %d, want 1", got[0].FileCount)
	}
	if got[0].ByteSize != 2 {
		t.Errorf("ByteSize = %d, want 2", got[0].ByteSize)
	}
	if !got[0].NewestMTime.Equal(t1) {
		t.Errorf("NewestMTime = %v, want %v", got[0].NewestMTime, t1)
	}
	// The skip is TALLIED: unreadable content means the inventory is a lower
	// bound, and consumers must be able to tell "empty" from "could not read".
	if got[0].WalkErrs == 0 {
		t.Error("WalkErrs = 0, want > 0 for an unreadable subtree")
	}
}

func TestWorkspaceDirInventory_MissingBase(t *testing.T) {
	_, err := workspaceDirInventory(filepath.Join(t.TempDir(), "does-not-exist"))
	if !os.IsNotExist(err) {
		t.Errorf("err = %v, want os.IsNotExist", err)
	}
}

func TestWorkspaceCanonicalAliases(t *testing.T) {
	tests := []struct {
		alias, canonical string
	}{
		{"post-mortem", "postmortem"},
		{"post-mortems", "postmortem"},
		{"pre-mortem", "pre-mortem-checks"},
		{"pre-mortems", "pre-mortem-checks"},
		{"premortem-checks", "pre-mortem-checks"},
		{"handoffs", "handoff"},
		{"mto-handoff", "handoff"},
		{"retros", "retro"},
		{"proof", "proofs"},
		{"test", "tests"},
	}
	if len(workspaceCanonicalAliases) != len(tests) {
		t.Errorf("registry has %d entries, want %d", len(workspaceCanonicalAliases), len(tests))
	}
	for _, tt := range tests {
		got, ok := workspaceCanonicalAliases[tt.alias]
		if !ok {
			t.Errorf("alias %q missing from registry", tt.alias)
			continue
		}
		if got != tt.canonical {
			t.Errorf("alias %q -> %q, want %q", tt.alias, got, tt.canonical)
		}
	}
}

func TestWorkspaceStaleNameMatching(t *testing.T) {
	tests := []struct {
		name          string
		explicitStale bool
		retryChain    bool
	}{
		// Real names that MUST match.
		{"land-queue-age-h433.19-native.stale-20260711T221704Z", true, false},
		{"land-queue-age-h433.19.stale-20260711T221032Z", true, false},
		{"land-queue-age-h433.22-native-retry2", false, true},
		{"land-queue-age-h433.27-native-retry2", false, true},
		// A staled retry-chain dir is both.
		{"land-queue-age-h433.22-native-retry2.stale-20260711T221704Z", true, true},
		// Names that must NOT match.
		{"land-queue", false, false},
		{"land-queue-runtime", false, false},
		{"archive", false, false},
		{"knowledge", false, false},
		{"retro", false, false},
	}
	for _, tt := range tests {
		explicitStale, retryChain := workspaceStaleNameKind(tt.name)
		if explicitStale != tt.explicitStale {
			t.Errorf("workspaceStaleNameKind(%q) explicitStale = %v, want %v", tt.name, explicitStale, tt.explicitStale)
		}
		if retryChain != tt.retryChain {
			t.Errorf("workspaceStaleNameKind(%q) retryChain = %v, want %v", tt.name, retryChain, tt.retryChain)
		}
		wantStale := tt.explicitStale || tt.retryChain
		if got := isWorkspaceStaleDirName(tt.name); got != wantStale {
			t.Errorf("isWorkspaceStaleDirName(%q) = %v, want %v", tt.name, got, wantStale)
		}
	}
}

func TestWorkspaceQuarantineDest(t *testing.T) {
	ctx := &MutateContext{RunDir: filepath.Join("/repo", ".doctor", "runs", "r1")}
	got := workspaceQuarantineDest(ctx, "land-queue-age-h433.22-native-retry2")
	want := filepath.Join("/repo", ".doctor", "runs", "r1", "quarantine", "workspace", "land-queue-age-h433.22-native-retry2")
	if got != want {
		t.Errorf("workspaceQuarantineDest = %q, want %q", got, want)
	}
}

func TestWorkspaceRealDir(t *testing.T) {
	base := t.TempDir()
	realDir := filepath.Join(base, "real")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	regFile := filepath.Join(base, "file.txt")
	writeWorkspaceFile(t, regFile, "x", time.Now())
	linkToDir := filepath.Join(base, "link-dir")
	if err := os.Symlink(realDir, linkToDir); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"real directory", realDir, true},
		{"regular file", regFile, false},
		{"symlink to a directory", linkToDir, false},
		{"absent", filepath.Join(base, "nope"), false},
	}
	for _, tt := range tests {
		if got := workspaceRealDir(tt.path); got != tt.want {
			t.Errorf("workspaceRealDir(%s) = %v, want %v", tt.name, got, tt.want)
		}
	}

	// Fixer-side counterpart: absent is a clean no-op, symlink/file refuse.
	if exists, err := workspaceRequireRealAgentsDir(realDir); err != nil || !exists {
		t.Errorf("workspaceRequireRealAgentsDir(real dir) = %v, %v, want true, nil", exists, err)
	}
	if exists, err := workspaceRequireRealAgentsDir(filepath.Join(base, "nope")); err != nil || exists {
		t.Errorf("workspaceRequireRealAgentsDir(absent) = %v, %v, want false, nil", exists, err)
	}
	for _, path := range []string{regFile, linkToDir} {
		exists, err := workspaceRequireRealAgentsDir(path)
		if err == nil || exists {
			t.Errorf("workspaceRequireRealAgentsDir(%s) = %v, %v, want refused_unsafe error", path, exists, err)
			continue
		}
		if !strings.Contains(err.Error(), "refused_unsafe") {
			t.Errorf("workspaceRequireRealAgentsDir(%s) error = %v, want it to mention refused_unsafe", path, err)
		}
	}
}

// TestWorkspaceSymlinkedAgentsRoot is the F-mode fixture for a `.agents` root
// that is a SYMLINK to a directory outside the repository: every workspace
// detector that consumes the root must report nothing, every workspace fixer
// must refuse with refused_unsafe, and the external tree must remain
// byte-for-byte untouched (the lexical scope check alone would have passed).
func TestWorkspaceSymlinkedAgentsRoot(t *testing.T) {
	repo := t.TempDir()
	external := t.TempDir()
	old := time.Now().Add(-30 * 24 * time.Hour)

	// The external tree carries one would-be finding for each failure mode.
	writeWorkspaceFile(t, filepath.Join(external, "post-mortems", "a.md"), "drift bait", old)
	if err := os.MkdirAll(filepath.Join(external, "stub"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeWorkspaceFile(t, filepath.Join(external, "land-queue-x.stale-20260601T000000Z", "v.json"), "stale bait", old)
	writeWorkspaceFile(t, filepath.Join(external, "learnings", "l.md"), "legacy", old)
	writeWorkspaceFile(t, filepath.Join(external, "ao", "learnings", "c.md"), "canonical", old)

	if err := os.Symlink(external, filepath.Join(repo, ".agents")); err != nil {
		t.Fatalf("symlink .agents: %v", err)
	}
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir(), Logger: os.Stderr}

	// Every root-consuming detector: zero findings.
	detectors := map[string]Detector{
		"drift":     workspaceNamingDriftDetector{},
		"empty":     workspaceEmptyDirsDetector{},
		"stale":     workspaceStaleQueueDirsDetector{},
		"oversize":  workspaceOversizeDetector{},
		"dualstore": workspaceDualStoreDetector{},
	}
	for name, d := range detectors {
		findings, err := d.Detect(env)
		if err != nil {
			t.Errorf("%s Detect on symlinked root: %v", name, err)
		}
		if len(findings) != 0 {
			t.Errorf("%s Detect on symlinked root = %d findings, want 0", name, len(findings))
		}
	}

	// Every workspace fixer: refuse, mutate nothing.
	ctx, ra := newWorkspaceStaleMutateCtx(t, repo)
	fixers := map[string]Fixer{
		"drift": workspaceNamingDriftFixer{},
		"empty": workspaceEmptyDirsFixer{},
		"stale": workspaceStaleQueueDirsFixer{},
	}
	for name, f := range fixers {
		res, err := f.Fix(ctx, env, nil)
		if err == nil || res.Err == nil {
			t.Errorf("%s Fix on symlinked root: err=%v res.Err=%v, want refused_unsafe", name, err, res.Err)
			continue
		}
		if !strings.Contains(err.Error(), "refused_unsafe") {
			t.Errorf("%s Fix error = %v, want it to mention refused_unsafe", name, err)
		}
		if res.ActionsTaken != 0 {
			t.Errorf("%s Fix took %d actions through a symlinked root", name, res.ActionsTaken)
		}
	}
	recs, err := readActions(ra.ActionsPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 0 {
		t.Fatalf("action records = %d, want 0", len(recs))
	}

	// The external tree is untouched.
	for path, content := range map[string]string{
		filepath.Join(external, "post-mortems", "a.md"):                          "drift bait",
		filepath.Join(external, "land-queue-x.stale-20260601T000000Z", "v.json"): "stale bait",
		filepath.Join(external, "learnings", "l.md"):                             "legacy",
		filepath.Join(external, "ao", "learnings", "c.md"):                       "canonical",
	} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != content {
			t.Errorf("external file %s = %q err=%v, want %q untouched", path, got, err, content)
		}
	}
	if st, err := os.Stat(filepath.Join(external, "stub")); err != nil || !st.IsDir() {
		t.Errorf("external stub dir missing after fix attempts (err=%v)", err)
	}
}

// TestWorkspaceDirRename_JournalFailureCompensates forces the actions.jsonl
// append to fail at the WRITE stage (dead handle) AFTER the rename executed:
// the record definitely never persisted, so the helper must rename the
// directory back to its original path (disk state matches the empty journal;
// undo is never blind) and report both facts in the error.
func TestWorkspaceDirRename_JournalFailureCompensates(t *testing.T) {
	repo := t.TempDir()
	dir := filepath.Join(repo, ".agents", "doomed")
	writeWorkspaceFile(t, filepath.Join(dir, "keep.md"), "payload", time.Now())

	ra, err := NewRunArtifact(repo, "wsjournalfail", time.Now())
	if err != nil {
		t.Fatalf("NewRunArtifact: %v", err)
	}
	af, err := ra.OpenActionsFile()
	if err != nil {
		t.Fatalf("OpenActionsFile: %v", err)
	}
	// Close the handle NOW: the rename will succeed, the journal append will fail.
	if err := af.Close(); err != nil {
		t.Fatalf("close actions file: %v", err)
	}
	caps := NewCapabilities("test")
	locks := NewLockManager(filepath.Join(repo, ".doctor", "locks"))
	ctx := NewMutateContext(ra, caps, t.TempDir(), locks, af, false).WithFixer("test-journal-fail")

	dest := workspaceQuarantineDest(ctx, "doomed")
	renameErr := workspaceDirRename(ctx, dir, dest, nil)
	if renameErr == nil {
		t.Fatal("workspaceDirRename succeeded despite a dead actions.jsonl handle")
	}
	if !strings.Contains(renameErr.Error(), "compensated") {
		t.Errorf("error = %v, want it to report the compensating rename-back", renameErr)
	}
	// The directory is back at its original path, content intact; dest gone.
	got, err := os.ReadFile(filepath.Join(dir, "keep.md"))
	if err != nil || string(got) != "payload" {
		t.Fatalf("dir not restored at original path: content=%q err=%v", got, err)
	}
	if _, err := os.Lstat(dest); !os.IsNotExist(err) {
		t.Errorf("quarantine dest still present after compensation (err=%v)", err)
	}
}

// TestWorkspaceDirRename_SyncFailureLeavesRenameInPlace: when the journal
// WRITE succeeds and only the fsync fails, the record has probably been
// persisted — a compensating rename-back would desync disk from the journal
// (undo would replay an unnecessary reverse rename). The helper must leave
// the rename in place and surface the durability uncertainty. The sync stage
// is simulated through the workspaceAppendAction seam because forcing a real
// fsync failure after a successful write is not portable.
func TestWorkspaceDirRename_SyncFailureLeavesRenameInPlace(t *testing.T) {
	repo := t.TempDir()
	dir := filepath.Join(repo, ".agents", "doomed")
	writeWorkspaceFile(t, filepath.Join(dir, "keep.md"), "payload", time.Now())

	ra, err := NewRunArtifact(repo, "wssyncfail", time.Now())
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
	ctx := NewMutateContext(ra, caps, t.TempDir(), locks, af, false).WithFixer("test-sync-fail")

	// Seam override: write landed, fsync failed. Restored via t.Cleanup so no
	// state leaks into shuffled test orders.
	orig := workspaceAppendAction
	t.Cleanup(func() { workspaceAppendAction = orig })
	workspaceAppendAction = func(ctx *MutateContext, rec ActionRecord) (bool, error) {
		if wrote, werr := orig(ctx, rec); werr != nil {
			return wrote, werr // real write failure would be a test-setup bug
		}
		return true, fmt.Errorf("simulated fsync failure")
	}

	dest := workspaceQuarantineDest(ctx, "doomed")
	renameErr := workspaceDirRename(ctx, dir, dest, nil)
	if renameErr == nil {
		t.Fatal("workspaceDirRename succeeded despite a failing journal sync")
	}
	if !strings.Contains(renameErr.Error(), "left in place") {
		t.Errorf("error = %v, want it to report the rename left in place", renameErr)
	}
	if strings.Contains(renameErr.Error(), "compensated") {
		t.Errorf("error = %v, must NOT report a compensating rename-back", renameErr)
	}
	// The rename stays applied: dest holds the content, the source is gone —
	// consistent with the (written) journal record.
	got, err := os.ReadFile(filepath.Join(dest, "keep.md"))
	if err != nil || string(got) != "payload" {
		t.Fatalf("dest content = %q err=%v, want the rename left in place", got, err)
	}
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Errorf("source dir still present after a sync-stage failure (err=%v)", err)
	}
	// The record IS in the journal (the write succeeded), so undo can replay.
	recs, err := readActions(ra.ActionsPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Op != "Rename" || recs[0].RenameTo != dest {
		t.Fatalf("journal records = %+v, want the one written Rename record", recs)
	}
}

// TestWorkspaceFileMoveNoClobber_DestExistsKernelRefusal calls the file-move
// helper directly against an EXISTING destination: os.Link must refuse with
// EEXIST (the kernel-atomic no-clobber, independent of any caller lstat
// pre-check), nothing moves, and nothing is journaled.
func TestWorkspaceFileMoveNoClobber_DestExistsKernelRefusal(t *testing.T) {
	repo := t.TempDir()
	src := filepath.Join(repo, ".agents", "retros", "dup.md")
	dest := filepath.Join(repo, ".agents", "retro", "dup.md")
	writeWorkspaceFile(t, src, "source body", time.Now())
	writeWorkspaceFile(t, dest, "dest body", time.Now())

	ra, err := NewRunArtifact(repo, "wsfilemove", time.Now())
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
	ctx := NewMutateContext(ra, caps, t.TempDir(), locks, af, false).WithFixer("test-file-move")

	collided, err := workspaceFileMoveNoClobber(ctx, src, dest)
	if err != nil {
		t.Fatalf("workspaceFileMoveNoClobber: %v", err)
	}
	if !collided {
		t.Fatal("existing destination not reported as a collision")
	}
	if got, err := os.ReadFile(src); err != nil || string(got) != "source body" {
		t.Errorf("source = %q err=%v, want untouched", got, err)
	}
	if got, err := os.ReadFile(dest); err != nil || string(got) != "dest body" {
		t.Errorf("dest = %q err=%v, want untouched (never overwritten)", got, err)
	}
	recs, err := readActions(ra.ActionsPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 0 {
		t.Fatalf("collided move wrote %d action records, want 0", len(recs))
	}
}

// TestWorkspaceFileMoveNoClobber_JournalWriteFailureCompensates: a WRITE-stage
// journal failure after link+remove executed must move the file back (disk
// matches the empty journal) and report the compensation.
func TestWorkspaceFileMoveNoClobber_JournalWriteFailureCompensates(t *testing.T) {
	repo := t.TempDir()
	src := filepath.Join(repo, ".agents", "retros", "a.md")
	dest := filepath.Join(repo, ".agents", "retro", "a.md")
	writeWorkspaceFile(t, src, "payload", time.Now())

	ra, err := NewRunArtifact(repo, "wsfilejournal", time.Now())
	if err != nil {
		t.Fatalf("NewRunArtifact: %v", err)
	}
	af, err := ra.OpenActionsFile()
	if err != nil {
		t.Fatalf("OpenActionsFile: %v", err)
	}
	// Close the handle NOW: the move will succeed, the journal write will fail.
	if err := af.Close(); err != nil {
		t.Fatalf("close actions file: %v", err)
	}
	caps := NewCapabilities("test")
	locks := NewLockManager(filepath.Join(repo, ".doctor", "locks"))
	ctx := NewMutateContext(ra, caps, t.TempDir(), locks, af, false).WithFixer("test-file-journal-fail")

	collided, moveErr := workspaceFileMoveNoClobber(ctx, src, dest)
	if moveErr == nil {
		t.Fatal("workspaceFileMoveNoClobber succeeded despite a dead actions.jsonl handle")
	}
	if collided {
		t.Error("journal failure misreported as a collision")
	}
	if !strings.Contains(moveErr.Error(), "compensated") {
		t.Errorf("error = %v, want it to report the compensating move-back", moveErr)
	}
	if got, err := os.ReadFile(src); err != nil || string(got) != "payload" {
		t.Fatalf("source not restored: content=%q err=%v", got, err)
	}
	if _, err := os.Lstat(dest); !os.IsNotExist(err) {
		t.Errorf("dest still present after compensation (err=%v)", err)
	}
}

func TestWorkspaceGCTTL(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{"default when unset", "", 14 * 24 * time.Hour},
		{"override 7 days", "7", 7 * 24 * time.Hour},
		{"override 1 day", "1", 24 * time.Hour},
		{"cap boundary accepted", "3650", 3650 * 24 * time.Hour},
		{"over cap falls back", "3651", 14 * 24 * time.Hour},
		// The overflow class from the hardening finding: day counts above
		// ~106751 wrap time.Duration negative, which would expire everything.
		{"duration-overflow value falls back", "10675200", 14 * 24 * time.Hour},
		{"zero falls back", "0", 14 * 24 * time.Hour},
		{"negative falls back", "-3", 14 * 24 * time.Hour},
		{"non-numeric falls back", "abc", 14 * 24 * time.Hour},
		{"float falls back", "2.5", 14 * 24 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(workspaceTTLEnvVar, tt.value)
			if got := workspaceGCTTL(); got != tt.want {
				t.Errorf("workspaceGCTTL() with %q = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
