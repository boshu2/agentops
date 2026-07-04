package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveBeadsDirLinkedWorktreeUsesGitCommonDir(t *testing.T) {
	t.Setenv("BEADS_DIR", "")
	root, lane := makeGitRepoWithLinkedWorktree(t)
	if err := os.Mkdir(filepath.Join(root, "_beads"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := resolveBeadsDir(lane, nil)
	want := filepath.Join(root, "_beads")
	if got.Path != want {
		t.Fatalf("resolveBeadsDir(linked worktree).Path = %q, want %q", got.Path, want)
	}
	if got.Source != beadsDirSourceGitCommon {
		t.Fatalf("resolveBeadsDir(linked worktree).Source = %q, want %q", got.Source, beadsDirSourceGitCommon)
	}
}

func TestResolveBeadsDirExplicitEnvWins(t *testing.T) {
	dir := t.TempDir()
	got := resolveBeadsDir(dir, []string{"BEADS_DIR=custom-ledger"})
	want := filepath.Join(dir, "custom-ledger")
	if got.Path != want || got.Source != beadsDirSourceEnv {
		t.Fatalf("resolveBeadsDir(env) = %+v, want path=%q source=%q", got, want, beadsDirSourceEnv)
	}
}

func TestBeadsTrackerCommandContextInDirSetsCanonicalBeadsDir(t *testing.T) {
	t.Setenv("BEADS_DIR", "")
	root, lane := makeGitRepoWithLinkedWorktree(t)
	canonical := filepath.Join(root, "_beads")
	if err := os.Mkdir(canonical, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := beadsTrackerCommandContextInDir(context.Background(), lane, "ready", "--json")
	found := ""
	for _, entry := range cmd.Env {
		if strings.HasPrefix(entry, "BEADS_DIR=") {
			found = strings.TrimPrefix(entry, "BEADS_DIR=")
		}
	}
	if found != canonical {
		t.Fatalf("BEADS_DIR in command env = %q, want %q", found, canonical)
	}
	if cmd.Dir != lane {
		t.Fatalf("cmd.Dir = %q, want linked worktree %q", cmd.Dir, lane)
	}
}

func TestRunBeadsDirJSON(t *testing.T) {
	t.Setenv("BEADS_DIR", "")
	root, lane := makeGitRepoWithLinkedWorktree(t)
	canonical := filepath.Join(root, "_beads")
	if err := os.Mkdir(canonical, 0o755); err != nil {
		t.Fatal(err)
	}
	origJSON := beadsDirJSON
	t.Cleanup(func() { beadsDirJSON = origJSON })
	beadsDirJSON = true

	t.Chdir(lane)

	var out strings.Builder
	cmd := beadsDirCmd
	cmd.SetOut(&out)
	t.Cleanup(func() { cmd.SetOut(nil) })
	if err := runBeadsDir(cmd, nil); err != nil {
		t.Fatal(err)
	}
	var payload map[string]string
	if err := json.Unmarshal([]byte(out.String()), &payload); err != nil {
		t.Fatalf("json output: %v: %s", err, out.String())
	}
	if payload["beads_dir"] != canonical || payload["source"] != beadsDirSourceGitCommon {
		t.Fatalf("payload = %+v, want canonical dir %q from git-common-dir", payload, canonical)
	}
}

// setBeadsDirRequire flips the --require cobra global for one test and
// restores it on cleanup (shared rootCmd state; see .claude/rules/go.md).
func setBeadsDirRequire(t *testing.T, v bool) {
	t.Helper()
	orig := beadsDirRequire
	t.Cleanup(func() { beadsDirRequire = orig })
	beadsDirRequire = v
}

func TestRunBeadsDirRequireFailsClosedWhenNoLedger(t *testing.T) {
	// A resolvable path that exists but holds no ledger artifact must be
	// refused: printing it would let a br WRITE silently target the wrong
	// tracker (age-gstf).
	dir := t.TempDir()
	ledgerless := filepath.Join(dir, "_beads")
	if err := os.Mkdir(ledgerless, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BEADS_DIR", ledgerless)
	setBeadsDirRequire(t, true)

	var out strings.Builder
	cmd := beadsDirCmd
	cmd.SetOut(&out)
	t.Cleanup(func() { cmd.SetOut(nil) })
	err := runBeadsDir(cmd, nil)
	if err == nil {
		t.Fatalf("runBeadsDir(--require) = nil error for ledgerless dir; want fail-closed error")
	}
	if !strings.Contains(err.Error(), "no ledger artifact") {
		t.Fatalf("error = %q, want the no-ledger-artifact reason", err)
	}
	if out.String() != "" {
		t.Fatalf("stdout = %q, want empty (a printed path defeats the fail-closed contract)", out.String())
	}
}

func TestRunBeadsDirRequireFailsClosedWhenPathMissing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BEADS_DIR", filepath.Join(dir, "does-not-exist"))
	setBeadsDirRequire(t, true)

	var out strings.Builder
	cmd := beadsDirCmd
	cmd.SetOut(&out)
	t.Cleanup(func() { cmd.SetOut(nil) })
	err := runBeadsDir(cmd, nil)
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("runBeadsDir(--require) err = %v, want does-not-exist refusal", err)
	}
	if out.String() != "" {
		t.Fatalf("stdout = %q, want empty on refusal", out.String())
	}
}

func TestRunBeadsDirRequirePassesWithLedgerArtifact(t *testing.T) {
	dir := t.TempDir()
	ledger := filepath.Join(dir, "_beads")
	if err := os.Mkdir(ledger, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ledger, "issues.jsonl"), []byte("{\"id\":\"x-1\",\"status\":\"open\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BEADS_DIR", ledger)
	setBeadsDirRequire(t, true)

	var out strings.Builder
	cmd := beadsDirCmd
	cmd.SetOut(&out)
	t.Cleanup(func() { cmd.SetOut(nil) })
	if err := runBeadsDir(cmd, nil); err != nil {
		t.Fatalf("runBeadsDir(--require) = %v for a dir with issues.jsonl; want success", err)
	}
	if got := strings.TrimSpace(out.String()); got != ledger {
		t.Fatalf("stdout = %q, want resolved ledger path %q", got, ledger)
	}
}

func makeGitRepoWithLinkedWorktree(t *testing.T) (root, lane string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	parent := t.TempDir()
	root = filepath.Join(parent, "agentops")
	lane = filepath.Join(parent, "agentops-lane")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "init")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "-c", "user.email=test@example.com", "-c", "user.name=Test", "commit", "-m", "seed")
	runGit(t, root, "worktree", "add", lane, "HEAD")
	if realRoot, err := filepath.EvalSymlinks(root); err == nil {
		root = realRoot
	}
	if realLane, err := filepath.EvalSymlinks(lane); err == nil {
		lane = realLane
	}
	return root, lane
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(out))
	}
}
