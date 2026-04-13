package overnight

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// seedAgents builds a fake .agents/ tree under cwd with a couple of
// learning and finding files and returns the live .agents path.
func seedAgents(t *testing.T, cwd string) string {
	t.Helper()
	live := filepath.Join(cwd, ".agents")
	mustMkdir(t, filepath.Join(live, "learnings"))
	mustMkdir(t, filepath.Join(live, "findings"))
	mustMkdir(t, filepath.Join(live, "rpi"))

	mustWrite(t, filepath.Join(live, "learnings", "2026-04-09-alpha.md"),
		"---\ntitle: alpha\nmaturity: seed\n---\nbody alpha\n")
	mustWrite(t, filepath.Join(live, "learnings", "2026-04-09-beta.md"),
		"---\ntitle: beta\nholdout_scenario_id: xyz\n---\nbody beta\n")
	mustWrite(t, filepath.Join(live, "findings", "finding-1.md"),
		"---\ntitle: finding-1\n---\nbody\n")
	mustWrite(t, filepath.Join(live, "rpi", "next-work.jsonl"),
		"{\"id\":\"one\"}\n{\"id\":\"two\"}\n")
	return live
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir parent %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// snapshotTree hashes every regular file under root (recursively) and
// returns a stable map of relative-path → sha256 hex. Used to assert
// byte-identical rollback.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = hex.EncodeToString(h.Sum(nil))
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return out
}

func TestCheckpoint_NewCommitRollback_HappyPath(t *testing.T) {
	cwd := t.TempDir()
	live := seedAgents(t, cwd)

	cp, err := NewCheckpoint(cwd, "run1-iter-1", 1<<20)
	if err != nil {
		t.Fatalf("NewCheckpoint: %v", err)
	}
	if cp.SizeBytes <= 0 {
		t.Fatalf("expected positive SizeBytes, got %d", cp.SizeBytes)
	}

	// Mutate inside staging: rewrite alpha and add gamma.
	stagedLearnings := filepath.Join(cp.StagingDir, ".agents", "learnings")
	mustWrite(t, filepath.Join(stagedLearnings, "2026-04-09-alpha.md"),
		"---\ntitle: alpha\nmaturity: promoted\n---\nbody alpha v2\n")
	mustWrite(t, filepath.Join(stagedLearnings, "2026-04-09-gamma.md"),
		"---\ntitle: gamma\n---\nbody gamma\n")

	if err := cp.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Live tree should now reflect the staged mutations.
	got := mustRead(t, filepath.Join(live, "learnings", "2026-04-09-alpha.md"))
	if !strings.Contains(got, "maturity: promoted") {
		t.Fatalf("expected alpha mutation to land in live, got: %s", got)
	}
	if _, err := os.Stat(filepath.Join(live, "learnings", "2026-04-09-gamma.md")); err != nil {
		t.Fatalf("expected gamma to exist in live after commit: %v", err)
	}

	// Prev dir should exist for this iteration, containing the displaced alpha.
	prevAlpha := filepath.Join(cp.PrevDir, "learnings", "2026-04-09-alpha.md")
	if _, err := os.Stat(prevAlpha); err != nil {
		t.Fatalf("expected prev snapshot of alpha at %s: %v", prevAlpha, err)
	}
	prevBody := mustRead(t, prevAlpha)
	if !strings.Contains(prevBody, "maturity: seed") {
		t.Fatalf("expected prev alpha to be the original seed copy, got: %s", prevBody)
	}

	// Marker should be DONE.
	state, err := readMarkerState(cp.MarkerPath)
	if err != nil {
		t.Fatalf("readMarkerState: %v", err)
	}
	if state != markerStateDone {
		t.Fatalf("expected marker state DONE, got %q", state)
	}

	// Staging directory is cleaned up after commit.
	if _, err := os.Stat(cp.StagingDir); !os.IsNotExist(err) {
		t.Fatalf("expected staging dir to be removed after commit, stat err: %v", err)
	}
}

func TestCheckpoint_MaxBytesEnforced(t *testing.T) {
	cwd := t.TempDir()
	live := seedAgents(t, cwd)
	pre := snapshotTree(t, live)

	cp, err := NewCheckpoint(cwd, "iter-small", 32)
	if err == nil {
		t.Fatalf("expected NewCheckpoint to fail with tiny budget, got cp=%+v", cp)
	}
	if !strings.Contains(err.Error(), "maxBytes") && !strings.Contains(err.Error(), "budget") {
		t.Fatalf("expected maxBytes/budget error, got: %v", err)
	}

	// Live tree is untouched.
	post := snapshotTree(t, live)
	if !equalSnapshots(pre, post) {
		t.Fatalf("live .agents mutated after failed NewCheckpoint\npre:  %v\npost: %v", pre, post)
	}

	// No staging tree should survive a failed NewCheckpoint.
	staging := filepath.Join(live, "overnight", "staging", "iter-small")
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatalf("expected staging dir cleanup after failed NewCheckpoint, stat err: %v", err)
	}
}

func TestCheckpoint_RollbackRestoresState(t *testing.T) {
	cwd := t.TempDir()
	live := seedAgents(t, cwd)

	// Snapshot live BEFORE checkpoint. Rollback must leave live
	// byte-identical to this snapshot.
	pre := snapshotTree(t, live)

	cp, err := NewCheckpoint(cwd, "iter-rb", 1<<20)
	if err != nil {
		t.Fatalf("NewCheckpoint: %v", err)
	}

	// Heavily mutate staging.
	stagedLearnings := filepath.Join(cp.StagingDir, ".agents", "learnings")
	mustWrite(t, filepath.Join(stagedLearnings, "2026-04-09-alpha.md"),
		"---\ntitle: alpha\nmaturity: stripped\n---\ndifferent body\n")
	_ = os.Remove(filepath.Join(stagedLearnings, "2026-04-09-beta.md"))
	mustWrite(t, filepath.Join(stagedLearnings, "2026-04-09-new.md"),
		"---\ntitle: new\n---\nshould not land\n")

	if err := cp.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	post := snapshotTree(t, live)
	if !equalSnapshots(pre, post) {
		t.Fatalf("rollback did not restore byte-identical state\npre:  %v\npost: %v",
			snapshotKeys(pre), snapshotKeys(post))
	}

	if _, err := os.Stat(cp.StagingDir); !os.IsNotExist(err) {
		t.Fatalf("expected staging dir removed after rollback, got stat err: %v", err)
	}
	if _, err := os.Stat(cp.MarkerPath); !os.IsNotExist(err) {
		t.Fatalf("expected marker removed after rollback, got stat err: %v", err)
	}
}

func TestCheckpoint_VerifyMetadataRoundTrip_DropsDetectsNewField(t *testing.T) {
	cwd := t.TempDir()
	live := seedAgents(t, cwd)

	cp, err := NewCheckpoint(cwd, "iter-pm005", 1<<20)
	if err != nil {
		t.Fatalf("NewCheckpoint: %v", err)
	}

	// Ensure the staged beta contains holdout_scenario_id (it does via seed).
	stagedBeta := filepath.Join(cp.StagingDir, ".agents", "learnings", "2026-04-09-beta.md")
	body := mustRead(t, stagedBeta)
	if !strings.Contains(body, "holdout_scenario_id") {
		t.Fatalf("expected seeded beta to contain holdout_scenario_id, got: %s", body)
	}

	// Post-V1 fix semantic: LIVE is the pristine baseline and STAGING is
	// the reducer's output. A key present in LIVE but missing from STAGING
	// = the reducer stripped it. Simulate a reducer that overwrote the
	// staged copy and dropped holdout_scenario_id; the live copy (which
	// still has the key) is the reference point.
	_ = live // live already contains the seeded key; keep the baseline untouched
	mustWrite(t, stagedBeta, "---\ntitle: beta\n---\nbody beta\n")

	report := VerifyMetadataRoundTrip(cp)
	if report.Pass {
		t.Fatalf("expected Pass=false, got Pass=true report=%+v", report)
	}
	if len(report.StrippedFields) == 0 {
		t.Fatalf("expected at least one stripped field, got none")
	}
	found := false
	for _, sf := range report.StrippedFields {
		if sf.Key == "holdout_scenario_id" &&
			strings.HasSuffix(sf.File, "2026-04-09-beta.md") &&
			strings.HasPrefix(sf.File, "learnings/") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected StrippedFields to include holdout_scenario_id on learnings/2026-04-09-beta.md, got: %+v",
			report.StrippedFields)
	}
}

func TestCheckpoint_RefusesMissingAgentsDir(t *testing.T) {
	cwd := t.TempDir()
	// Deliberately do NOT seed .agents/.
	cp, err := NewCheckpoint(cwd, "iter-missing", 1<<20)
	if err == nil {
		t.Fatalf("expected error for missing .agents, got cp=%+v", cp)
	}
	if !strings.Contains(err.Error(), ".agents") {
		t.Fatalf("expected error mentioning .agents, got: %v", err)
	}
}

func TestCheckpoint_HandlesMissingOptionalSubpath(t *testing.T) {
	cwd := t.TempDir()
	live := filepath.Join(cwd, ".agents")
	mustMkdir(t, filepath.Join(live, "learnings"))
	mustWrite(t, filepath.Join(live, "learnings", "2026-04-09-only.md"),
		"---\ntitle: only\n---\nbody\n")
	// No findings/ patterns/ knowledge/ rpi/ — all optional.

	cp, err := NewCheckpoint(cwd, "iter-optional", 1<<20)
	if err != nil {
		t.Fatalf("NewCheckpoint should tolerate missing optional subpaths: %v", err)
	}
	stagedOnly := filepath.Join(cp.StagingDir, ".agents", "learnings", "2026-04-09-only.md")
	if _, err := os.Stat(stagedOnly); err != nil {
		t.Fatalf("expected learnings to be staged, got: %v", err)
	}
	// Missing subpaths should not appear under staging.
	stagedFindings := filepath.Join(cp.StagingDir, ".agents", "findings")
	if _, err := os.Stat(stagedFindings); !os.IsNotExist(err) {
		t.Fatalf("expected missing findings to stay missing in staging, got stat err: %v", err)
	}

	if err := cp.Commit(); err != nil {
		t.Fatalf("Commit with only learnings: %v", err)
	}
	// Live learnings intact.
	if _, err := os.Stat(filepath.Join(live, "learnings", "2026-04-09-only.md")); err != nil {
		t.Fatalf("expected live learnings file post-commit: %v", err)
	}
}

func TestCopyFile_UsesCloneStrategy(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	mustWrite(t, src, "byte-copy input")

	previous := checkpointCloneFile
	t.Cleanup(func() { checkpointCloneFile = previous })

	called := false
	checkpointCloneFile = func(gotSrc, gotDst string, mode os.FileMode) (int64, bool, error) {
		called = true
		if gotSrc != src {
			t.Fatalf("clone src = %q, want %q", gotSrc, src)
		}
		if gotDst != dst {
			t.Fatalf("clone dst = %q, want %q", gotDst, dst)
		}
		if mode.Perm() != 0o600 {
			t.Fatalf("clone mode = %v, want 0600", mode.Perm())
		}
		if err := os.WriteFile(gotDst, []byte("clone fast path"), mode.Perm()); err != nil {
			return 0, true, err
		}
		return int64(len("clone fast path")), true, nil
	}

	n, err := copyFile(src, dst, 0o600)
	if err != nil {
		t.Fatalf("copyFile: %v", err)
	}
	if !called {
		t.Fatal("expected clone strategy to be called")
	}
	if n != int64(len("clone fast path")) {
		t.Fatalf("copyFile bytes = %d, want %d", n, len("clone fast path"))
	}
	if got := mustRead(t, dst); got != "clone fast path" {
		t.Fatalf("dst content = %q", got)
	}
}

func TestCopyFile_FallsBackWhenCloneUnavailable(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	mustWrite(t, src, "portable fallback")

	previous := checkpointCloneFile
	t.Cleanup(func() { checkpointCloneFile = previous })

	calls := 0
	checkpointCloneFile = func(string, string, os.FileMode) (int64, bool, error) {
		calls++
		return 0, false, nil
	}

	n, err := copyFile(src, dst, 0o640)
	if err != nil {
		t.Fatalf("copyFile: %v", err)
	}
	if calls != 1 {
		t.Fatalf("clone strategy calls = %d, want 1", calls)
	}
	if n != int64(len("portable fallback")) {
		t.Fatalf("copyFile bytes = %d, want %d", n, len("portable fallback"))
	}
	if got := mustRead(t, dst); got != "portable fallback" {
		t.Fatalf("dst content = %q", got)
	}
}

func equalSnapshots(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func snapshotKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestNewCheckpoint_RejectsPathTraversal(t *testing.T) {
	cwd := t.TempDir()
	// Seed .agents/ so the rest of NewCheckpoint would succeed.
	mustMkdir(t, filepath.Join(cwd, ".agents", "learnings"))
	bad := []string{"../etc/passwd", "a/b", "a\\b", "..", "iter with space", "\x00bad"}
	for _, id := range bad {
		_, err := NewCheckpoint(cwd, id, 1<<20)
		if err == nil {
			t.Errorf("NewCheckpoint(%q) should have failed, got nil error", id)
		}
	}
	// Positive case: valid ID still works.
	_, err := NewCheckpoint(cwd, "iter-1", 1<<20)
	if err != nil {
		t.Errorf("NewCheckpoint with valid id failed: %v", err)
	}
}
