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
