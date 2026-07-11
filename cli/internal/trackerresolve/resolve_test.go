package trackerresolve

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveSelectedTrackerDoesNotFallback(t *testing.T) {
	look := func(name string) (string, error) {
		if name == BD {
			return "/fake/bd", nil
		}
		return "", errors.New("missing")
	}
	got, err := ResolveWithLookPath(t.TempDir(), []string{"AGENTOPS_TRACKER=br"}, look)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tracker != BR || got.Binary != BR {
		t.Fatalf("selected BR silently fell back: %+v", got)
	}
}

func TestResolveBeadsDirLinkedWorktreeUsesGitCommonDir(t *testing.T) {
	root := initTrackerRepo(t)
	lane := filepath.Join(t.TempDir(), "lane")
	runGit(t, root, "worktree", "add", "-b", "test-lane", lane)
	ledger := filepath.Join(root, "_beads")
	if err := os.Mkdir(ledger, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveWithLookPath(lane, []string{"HOME=" + t.TempDir()}, fakeLookPath(true, true))
	if err != nil {
		t.Fatal(err)
	}
	if got.Tracker != BR || got.LedgerDir != ledger || got.RepoRoot != root {
		t.Fatalf("linked-worktree resolution = %+v, want br ledger %q rooted at %q", got, ledger, root)
	}
	if got.LedgerSource != LedgerSourceGitCommon || got.GitCommonDir != filepath.Join(root, ".git") {
		t.Fatalf("linked-worktree source = %+v", got)
	}
	if got.WorkDir != lane || envLast(got.ChildEnv, "BEADS_DIR") != ledger {
		t.Fatalf("br child context = workdir %q env %q", got.WorkDir, envLast(got.ChildEnv, "BEADS_DIR"))
	}
}

func TestResolveTrackerBDStripsBeadsDirFromChildEnvironment(t *testing.T) {
	root := initTrackerRepo(t)
	ledger := filepath.Join(root, ".beads")
	if err := os.Mkdir(ledger, 0o755); err != nil {
		t.Fatal(err)
	}
	env := []string{"HOME=" + t.TempDir(), "AGENTOPS_TRACKER=bd", "BEADS_DIR=/wrong", "KEEP=present"}
	got, err := ResolveWithLookPath(root, env, fakeLookPath(true, true))
	if err != nil {
		t.Fatal(err)
	}
	if got.Tracker != BD || got.LedgerDir != ledger || got.WorkDir != root {
		t.Fatalf("bd resolution = %+v", got)
	}
	if _, ok := envValue(got.ChildEnv, "BEADS_DIR"); ok {
		t.Fatalf("bd child environment leaked BEADS_DIR: %v", got.ChildEnv)
	}
	if envLast(got.ChildEnv, "KEEP") != "present" {
		t.Fatalf("bd child environment dropped unrelated values: %v", got.ChildEnv)
	}
}

func TestResolveTrackerIgnoresForeignBeadsDirDuringBackendSelection(t *testing.T) {
	root := initTrackerRepo(t)
	bdLedger := filepath.Join(root, ".beads")
	if err := os.Mkdir(bdLedger, 0o755); err != nil {
		t.Fatal(err)
	}
	foreignBRLedger := filepath.Join(t.TempDir(), "_beads")
	if err := os.Mkdir(foreignBRLedger, 0o755); err != nil {
		t.Fatal(err)
	}

	env := []string{"HOME=" + t.TempDir(), "BEADS_DIR=" + foreignBRLedger}
	got, err := ResolveWithLookPath(root, env, fakeLookPath(true, true))
	if err != nil {
		t.Fatal(err)
	}
	if got.Tracker != BD || got.LedgerDir != bdLedger || got.Source != SourceLedger {
		t.Fatalf("foreign BEADS_DIR hijacked backend selection: got %+v, want bd ledger %q", got, bdLedger)
	}
	if _, ok := envValue(got.ChildEnv, "BEADS_DIR"); ok {
		t.Fatalf("selected bd child environment leaked foreign BEADS_DIR: %v", got.ChildEnv)
	}
}

func TestResolveBeadsDirExplicitRelativeOverride(t *testing.T) {
	cwd := t.TempDir()
	got := ResolveLedger(cwd, []string{"BEADS_DIR=private/ledger"}, BR)
	want := filepath.Join(cwd, "private", "ledger")
	if got.Path != want || got.Source != LedgerSourceEnv {
		t.Fatalf("ResolveLedger() = %+v, want path=%q source=%q", got, want, LedgerSourceEnv)
	}
}

func initTrackerRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "fixture")
	if real, err := filepath.EvalSymlinks(root); err == nil {
		root = real
	}
	return root
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func fakeLookPath(br, bd bool) LookPath {
	return func(name string) (string, error) {
		if name == BR && br || name == BD && bd {
			return "/fake/bin/" + name, nil
		}
		return "", exec.ErrNotFound
	}
}

func envLast(env []string, key string) string {
	value, _ := envValue(env, key)
	return value
}
