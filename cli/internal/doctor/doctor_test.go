package doctor

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRunID_Determinism verifies the run-id is deterministic to the second.
func TestRunID_Determinism(t *testing.T) {
	ts := time.Date(2026, 5, 6, 14, 23, 7, 0, time.UTC)
	a := RunID("deadbeef", ts)
	b := RunID("deadbeef", ts)
	if a != b {
		t.Fatalf("RunID not deterministic: %q != %q", a, b)
	}
	if len(a) != 6 {
		t.Fatalf("RunID length = %d, want 6", len(a))
	}
	// Same second, same SHA collide; one second later differs.
	c := RunID("deadbeef", ts.Add(time.Second))
	if c == a {
		t.Fatalf("RunID did not change with timestamp: %q", c)
	}
	// Different SHA differs.
	d := RunID("cafebabe", ts)
	if d == a {
		t.Fatalf("RunID did not change with target SHA: %q", d)
	}
}

// TestMutate_RoundTrip verifies write -> backup -> actions.jsonl -> undo.
func TestMutate_RoundTrip(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	scopeDir := filepath.Join(repo, ".agents", "ao")
	if err := os.MkdirAll(scopeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(scopeDir, "thing.txt")
	original := []byte("original content\n")
	if err := os.WriteFile(target, original, 0o644); err != nil {
		t.Fatal(err)
	}

	ra, err := NewRunArtifact(repo, "abc123", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	caps := NewCapabilities("2.0.0")
	locks := NewLockManager(filepath.Join(repo, ".doctor", "locks"))
	af, err := ra.OpenActionsFile()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = af.Close() }()
	mctx := NewMutateContext(ra, caps, home, locks, af, false).WithFixer("fm-test")

	newContent := []byte("rewritten content\n")
	res, err := Mutate(mctx, target, WriteFile{Content: newContent, Mode: 0o644})
	if err != nil {
		t.Fatalf("Mutate failed: %v", err)
	}
	if !res.OK {
		t.Fatal("Mutate result not OK")
	}

	// File rewritten.
	got, _ := os.ReadFile(target)
	if string(got) != string(newContent) {
		t.Fatalf("file content = %q, want %q", got, newContent)
	}
	// Backup exists and is byte-identical to original.
	backup := filepath.Join(ra.BackupsDir(), ".agents", "ao", "thing.txt")
	bgot, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	if string(bgot) != string(original) {
		t.Fatalf("backup content = %q, want %q", bgot, original)
	}
	// actions.jsonl has exactly one line.
	recs, err := readActions(ra.ActionsPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("actions.jsonl lines = %d, want 1", len(recs))
	}
	if recs[0].Op != "WriteFile" || recs[0].FixerID != "fm-test" || !recs[0].OK {
		t.Fatalf("unexpected action record: %+v", recs[0])
	}

	// Undo restores byte-identical original.
	_ = af.Close()
	ur, err := Undo(repo, ra.RunID, true, false)
	if err != nil {
		t.Fatalf("Undo failed: %v", err)
	}
	if ur.Restored != 1 {
		t.Fatalf("Undo restored = %d, want 1", ur.Restored)
	}
	restored, _ := os.ReadFile(target)
	if string(restored) != string(original) {
		t.Fatalf("after undo content = %q, want %q", restored, original)
	}
}

// TestLock_Contention verifies a second Acquire on the same path returns ErrLockHeld.
func TestLock_Contention(t *testing.T) {
	dir := t.TempDir()
	lm := NewLockManager(filepath.Join(dir, "locks"))
	target := filepath.Join(dir, "file.txt")

	g1, err := lm.Acquire(target)
	if err != nil {
		t.Fatalf("first Acquire failed: %v", err)
	}
	_, err = lm.Acquire(target)
	if err != ErrLockHeld {
		t.Fatalf("second Acquire error = %v, want ErrLockHeld", err)
	}
	if err := g1.Release(); err != nil {
		t.Fatalf("Release failed: %v", err)
	}
	// After release, re-acquire succeeds.
	g2, err := lm.Acquire(target)
	if err != nil {
		t.Fatalf("re-Acquire after release failed: %v", err)
	}
	_ = g2.Release()
}

// TestEnsureInScope_RejectsOutOfScope verifies path-scope enforcement.
func TestEnsureInScope_RejectsOutOfScope(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	caps := NewCapabilities("2.0.0")

	inScope := filepath.Join(repo, ".agents", "ao", "index", "search-index.jsonl")
	if err := EnsureInScope(caps, repo, home, inScope); err != nil {
		t.Fatalf("in-scope path rejected: %v", err)
	}
	outOfScope := filepath.Join(repo, ".git", "config")
	if err := EnsureInScope(caps, repo, home, outOfScope); err == nil {
		t.Fatal("out-of-scope .git path was accepted")
	}
	traversal := filepath.Join(repo, ".doctor", "..", "..", "etc", "passwd")
	if err := EnsureInScope(caps, repo, home, traversal); err == nil {
		t.Fatal("path traversal escaped write scopes")
	}
}

// TestMutate_DryRunTouchesNothing verifies dry-run does not write.
func TestMutate_DryRunTouchesNothing(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	scopeDir := filepath.Join(repo, ".agents", "ao")
	if err := os.MkdirAll(scopeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(scopeDir, "dry.txt")
	original := []byte("untouched\n")
	if err := os.WriteFile(target, original, 0o644); err != nil {
		t.Fatal(err)
	}

	ra, err := NewRunArtifact(repo, "sha", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	caps := NewCapabilities("2.0.0")
	locks := NewLockManager(filepath.Join(repo, ".doctor", "locks"))
	af, err := ra.OpenActionsFile()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = af.Close() }()
	mctx := NewMutateContext(ra, caps, home, locks, af, true) // dryRun = true

	if _, err := Mutate(mctx, target, WriteFile{Content: []byte("CHANGED"), Mode: 0o644}); err != nil {
		t.Fatalf("dry-run Mutate failed: %v", err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != string(original) {
		t.Fatalf("dry-run modified the file: %q", got)
	}
	backup := filepath.Join(ra.BackupsDir(), ".agents", "ao", "dry.txt")
	if _, err := os.Stat(backup); err == nil {
		t.Fatal("dry-run wrote a backup")
	}
	recs, _ := readActions(ra.ActionsPath())
	if len(recs) != 0 {
		t.Fatalf("dry-run wrote %d action records, want 0", len(recs))
	}
}

// TestCapabilities_JSONValidates verifies the capabilities document is sane.
func TestCapabilities_JSONValidates(t *testing.T) {
	caps := NewCapabilities("2.5.0")
	if caps.SchemaVersion != SchemaVersion || caps.Tool != "ao" {
		t.Fatalf("bad capabilities header: %+v", caps)
	}
	if caps.ToolVersion != "2.5.0" {
		t.Fatalf("tool version = %q, want 2.5.0", caps.ToolVersion)
	}
	if caps.Detectors == nil || caps.Fixers == nil {
		t.Fatal("detectors/fixers must be non-nil (empty arrays)")
	}
	// Every registered detector must have a unique, non-empty id.
	seen := make(map[string]bool)
	for _, d := range caps.Detectors {
		if d.ID == "" {
			t.Fatal("capabilities detector with empty id")
		}
		if seen[d.ID] {
			t.Fatalf("duplicate detector id in capabilities: %q", d.ID)
		}
		seen[d.ID] = true
	}
	if len(caps.WriteScopes) == 0 {
		t.Fatal("write scopes must be populated from safety envelope")
	}
	if caps.ExitCodes["5"] != "concurrency_lost" {
		t.Fatalf("exit code 5 = %q, want concurrency_lost", caps.ExitCodes["5"])
	}
}

// TestNewRunArtifact_LayoutAndGitignore verifies run dir layout + .gitignore.
func TestNewRunArtifact_LayoutAndGitignore(t *testing.T) {
	repo := t.TempDir()
	ra, err := NewRunArtifact(repo, "sha", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"backups", "quarantine"} {
		if info, err := os.Stat(filepath.Join(ra.RunDir, sub)); err != nil || !info.IsDir() {
			t.Fatalf("run subdir %q missing", sub)
		}
	}
	// latest symlink resolves to this run.
	resolved, err := resolveRunDir(repo, "latest")
	if err != nil {
		t.Fatalf("resolve latest failed: %v", err)
	}
	if resolved != ra.RunDir {
		t.Fatalf("latest = %q, want %q", resolved, ra.RunDir)
	}
	// .gitignore contains .doctor/.
	gi, err := os.ReadFile(filepath.Join(repo, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if want := ".doctor/"; !contains(string(gi), want) {
		t.Fatalf(".gitignore missing %q, got %q", want, gi)
	}
}

// TestDiagnose_NoDetectorsSelectedHealthy verifies that when the detector
// selection is empty (here forced via an --only filter that matches nothing),
// diagnose reports a healthy workspace and still writes its run artifacts.
func TestDiagnose_NoDetectorsSelectedHealthy(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	rep, err := Diagnose(Options{
		RepoRoot: repo, CWD: repo, HomeDir: home, ToolVersion: "2.0.0",
		Only: []string{"__no_such_detector__"},
	})
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}
	if rep.ExitCode != ExitHealthy {
		t.Fatalf("empty-registry diagnose exit = %d, want %d", rep.ExitCode, ExitHealthy)
	}
	if len(rep.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(rep.Findings))
	}
	// report.json was written.
	if _, err := os.Stat(filepath.Join(repo, ".doctor", "runs", filepath.Base(rep.RunDir)+"")); err != nil {
		// run dir path is repo-relative in the report; verify via latest.
		if _, lerr := resolveRunDir(repo, "latest"); lerr != nil {
			t.Fatalf("run dir not created: %v", lerr)
		}
	}
}

// TestFix_NoDetectorsSelectedNoFindings verifies fix with an empty detector
// selection (forced via an --only filter that matches nothing) exits 0.
func TestFix_NoDetectorsSelectedNoFindings(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	rep, err := Fix(Options{
		RepoRoot: repo, CWD: repo, HomeDir: home, ToolVersion: "2.0.0",
		Only: []string{"__no_such_detector__"},
	})
	if err != nil {
		t.Fatalf("Fix failed: %v", err)
	}
	if rep.ExitCode != ExitHealthy {
		t.Fatalf("empty-registry fix exit = %d, want %d", rep.ExitCode, ExitHealthy)
	}
}

// TestFix_DryRunExitsZeroWithFindings verifies that `--fix --dry-run` exits 0
// per the doctor CLI contract even when corrupt state would yield findings,
// and that the dry-run touches nothing on disk.
func TestFix_DryRunExitsZeroWithFindings(t *testing.T) {
	env, repo := daemonTestEnv(t)
	ledger := daemonLedgerPath(env)
	corrupt := validLedgerLine("evt-0001") + "\n{bad json\n"
	if err := os.WriteFile(ledger, []byte(corrupt), 0o600); err != nil {
		t.Fatal(err)
	}

	rep, err := Fix(Options{
		RepoRoot: repo, CWD: repo, HomeDir: t.TempDir(), ToolVersion: "2.0.0",
		Only:   []string{"fm-daemon-corrupt-ledger-line"},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("Fix dry-run failed: %v", err)
	}
	if len(rep.Findings) == 0 {
		t.Fatal("expected the corrupt ledger to produce a finding")
	}
	if rep.ExitCode != ExitHealthy {
		t.Errorf("dry-run exit_code = %d, want %d (ExitHealthy)", rep.ExitCode, ExitHealthy)
	}
	if !rep.OK {
		t.Error("dry-run ok = false, want true")
	}
	got, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != corrupt {
		t.Errorf("dry-run modified the ledger:\n got %q\nwant %q", got, corrupt)
	}
}

// TestGC_RefusesWithoutGates verifies gc never deletes silently.
func TestGC_RefusesWithoutGates(t *testing.T) {
	repo := t.TempDir()
	if _, err := GC(repo, time.Time{}, true); err == nil {
		t.Fatal("gc accepted a zero cutoff")
	}
	if _, err := GC(repo, time.Now(), false); err == nil {
		t.Fatal("gc accepted without --yes")
	}
}

// TestTopoSort_DependencyGraph verifies dependency ordering.
func TestTopoSort_DependencyGraph(t *testing.T) {
	graph := []byte(`{"nodes":["a","b","c"],"edges":[{"from":"a","to":"b"},{"from":"b","to":"c"}]}`)
	order, err := topoSortGraph(graph)
	if err != nil {
		t.Fatal(err)
	}
	pos := map[string]int{}
	for i, n := range order {
		pos[n] = i
	}
	if pos["a"] >= pos["b"] || pos["b"] >= pos["c"] {
		t.Fatalf("topo order violates dependencies: %v", order)
	}
}

// TestRobotDocs_ContainsContract verifies the handbook covers the contract.
func TestRobotDocs_ContainsContract(t *testing.T) {
	docs := RobotDocs()
	for _, want := range []string{
		"ao doctor", "Exit codes", "NEVER do", "gc --before", "capabilities --json",
	} {
		if !contains(docs, want) {
			t.Fatalf("robot-docs missing %q", want)
		}
	}
	if docs[len(docs)-1] != '\n' {
		t.Fatal("robot-docs must end with exactly one newline")
	}
}

// TestDescribeOp covers each op variant.
func TestDescribeOp(t *testing.T) {
	cases := []struct {
		op   Op
		want string
	}{
		{WriteFile{Content: []byte("xy"), Mode: 0o644}, "WriteFile (2 bytes, mode 644)"},
		{AppendFile{Content: []byte("z")}, "AppendFile (1 bytes)"},
		{Rename{To: "/q/x"}, "Rename -> /q/x"},
		{Chmod{Mode: 0o600}, "Chmod 600"},
		{SymlinkAtomic{Target: "/t"}, "SymlinkAtomic -> /t"},
	}
	for _, c := range cases {
		if got := DescribeOp(c.op); got != c.want {
			t.Fatalf("DescribeOp(%T) = %q, want %q", c.op, got, c.want)
		}
	}
}

// TestEnsureOpAllowed_DBOpsRejected verifies DB ops are declared-but-unsupported.
func TestEnsureOpAllowed_DBOpsRejected(t *testing.T) {
	caps := NewCapabilities("2.0.0")
	if err := EnsureOpAllowed(caps, DbExec{SQL: "SELECT 1"}); err != ErrDBOpsUnused {
		t.Fatalf("DbExec error = %v, want ErrDBOpsUnused", err)
	}
	if err := EnsureOpAllowed(caps, DbMigrate{From: 1, To: 2}); err != ErrDBOpsUnused {
		t.Fatalf("DbMigrate error = %v, want ErrDBOpsUnused", err)
	}
	if err := EnsureOpAllowed(caps, WriteFile{}); err != nil {
		t.Fatalf("WriteFile rejected: %v", err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
