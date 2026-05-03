package overnight

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// withExecShim installs a replacement ExecCommand for the duration of
// the test and restores the original on cleanup. Boundary tests use
// this to intercept forbidden subprocess invocations without mocking
// individual call sites.
func withExecShim(t *testing.T, shim func(string, ...string) *exec.Cmd) {
	t.Helper()
	orig := ExecCommand
	ExecCommand = shim
	t.Cleanup(func() { ExecCommand = orig })
}

// newBoundaryOpts builds a minimal RunLoopOptions pointed at a temp
// repo root with a pre-created .agents/overnight output directory.
// Used by every boundary test so the four tests share a single
// fixture shape.
func newBoundaryOpts(t *testing.T, repoRoot string) RunLoopOptions {
	t.Helper()
	outDir := filepath.Join(repoRoot, ".agents", "overnight", "boundary-run")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir outDir: %v", err)
	}
	// Seed a minimal .agents/ corpus so any future Wave 4 wiring that
	// walks learnings/findings/patterns has something to iterate.
	for _, sub := range []string{"learnings", "findings", "patterns"} {
		if err := os.MkdirAll(filepath.Join(repoRoot, ".agents", sub), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", sub, err)
		}
	}
	seed := filepath.Join(repoRoot, ".agents", "learnings", "seed.md")
	if err := os.WriteFile(seed, []byte("# seed learning\n\nboundary fixture\n"), 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	return RunLoopOptions{
		Cwd:            repoRoot,
		OutputDir:      outDir,
		RunID:          "test-boundary-run",
		RunTimeout:     30 * time.Second,
		MaxIterations:  1,
		PlateauEpsilon: 0.01,
		PlateauWindowK: 2,
		WarnOnly:       true,
		LogWriter:      &bytes.Buffer{},
	}
}

// runGit runs a git command via the real exec.Command (NOT the package
// shim) so test-harness git operations are never flagged by boundary
// shims. Fails the test on any error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(out))
	}
}

// TestRunLoop_NeverMutatesGitTrackedFiles verifies that a full Dream
// iteration leaves git-tracked files untouched. We use a temp git repo
// with one committed file, run the loop against a fake .agents/ under
// it, and assert git status is clean afterward (ignoring untracked
// .agents/ output).
//
// NOTE: While the Wave 1 RunLoop skeleton short-circuits without
// running stages, this test trivially passes. Once Wave 4 wires the
// stage drivers in, the test becomes a real mechanical enforcement of
// boundary #1: "Dream never mutates source code in any git-tracked
// directory."
func TestRunLoop_NeverMutatesGitTrackedFiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available; skipping mechanical boundary test")
	}
	repo := t.TempDir()

	runGit(t, repo, "init", "--quiet", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "test")

	readme := filepath.Join(repo, "README.md")
	readmeContents := []byte("# tracked file\n\nmust not be mutated by Dream\n")
	if err := os.WriteFile(readme, readmeContents, 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "--quiet", "-m", "seed")

	// Capture HEAD tree before RunLoop.
	beforeCmd := exec.Command("git", "rev-parse", "HEAD")
	beforeCmd.Dir = repo
	beforeOut, err := beforeCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v\n%s", err, string(beforeOut))
	}
	headBefore := strings.TrimSpace(string(beforeOut))

	opts := newBoundaryOpts(t, repo)
	if _, err := RunLoop(context.Background(), opts); err != nil {
		t.Fatalf("RunLoop: %v", err)
	}

	// HEAD must be unchanged (no commits made).
	afterCmd := exec.Command("git", "rev-parse", "HEAD")
	afterCmd.Dir = repo
	afterOut, err := afterCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse HEAD after: %v\n%s", err, string(afterOut))
	}
	headAfter := strings.TrimSpace(string(afterOut))
	if headBefore != headAfter {
		t.Fatalf("HEAD mutated by RunLoop: before=%s after=%s", headBefore, headAfter)
	}

	// README contents must be byte-identical.
	readmeAfter, err := os.ReadFile(readme)
	if err != nil {
		t.Fatalf("read README after: %v", err)
	}
	if !bytes.Equal(readmeContents, readmeAfter) {
		t.Fatalf("tracked file mutated by RunLoop:\nwant %q\n got %q", readmeContents, readmeAfter)
	}

	// Parse `git status --porcelain` and assert no tracked file is
	// modified, deleted, renamed, or staged. Untracked entries are
	// allowed — .agents/ is expected to be uncommitted scratch.
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = repo
	statusOut, err := statusCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git status: %v\n%s", err, string(statusOut))
	}
	for _, line := range strings.Split(strings.TrimSpace(string(statusOut)), "\n") {
		if line == "" {
			continue
		}
		// Porcelain format: XY <path>. "??" means untracked (allowed).
		if strings.HasPrefix(line, "??") {
			continue
		}
		t.Fatalf("RunLoop dirtied tracked file: %q", line)
	}
}

// TestRunLoop_NeverInvokesRpi verifies that no code path in the
// overnight package invokes `ao rpi`, `rpi`, or any rpi subcommand via
// subprocess. We install a ExecCommand shim that t.Fatalf's on any
// forbidden invocation. Enforces boundary #2: "Dream never invokes
// /rpi or any code-mutating flow via subprocess."
func TestRunLoop_NeverInvokesRpi(t *testing.T) {
	repo := t.TempDir()

	withExecShim(t, func(name string, args ...string) *exec.Cmd {
		combined := strings.ToLower(strings.Join(append([]string{name}, args...), " "))
		// Forbid any subprocess whose name or args mention rpi.
		// "rpi" as a bare token catches `ao rpi ...`, `./rpi`, etc.
		for _, tok := range strings.Fields(combined) {
			if tok == "rpi" || strings.HasSuffix(tok, "/rpi") {
				t.Fatalf("forbidden: Dream invoked rpi via subprocess: %s %s", name, strings.Join(args, " "))
			}
		}
		// Allow all other subprocesses to pass through to the real
		// exec.Command so any test-harness git use still works.
		return exec.Command(name, args...)
	})

	opts := newBoundaryOpts(t, repo)
	if _, err := RunLoop(context.Background(), opts); err != nil {
		t.Fatalf("RunLoop: %v", err)
	}
}

// TestRunLoop_NoSymlinksCreated verifies that a full Dream iteration
// does not create any symlinks under .agents/. Enforces boundary #3:
// "Dream never creates symlinks anywhere in .agents/." Repo-wide
// plugin-load-test already rejects symlinks at commit time; this is
// the runtime counterpart.
func TestRunLoop_NoSymlinksCreated(t *testing.T) {
	repo := t.TempDir()
	opts := newBoundaryOpts(t, repo)

	if _, err := RunLoop(context.Background(), opts); err != nil {
		t.Fatalf("RunLoop: %v", err)
	}

	agentsDir := filepath.Join(repo, ".agents")
	walkErr := filepath.Walk(agentsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("forbidden: Dream created symlink at %s", path)
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk .agents: %v", walkErr)
	}
}

// TestRunLoop_NoGitOpsInvoked verifies that no code path in the
// overnight package invokes mutating git operations via subprocess.
// Enforces boundary #4: "Dream never calls git commit / git push /
// git reset / git checkout."
//
// Read-only git operations (status, rev-parse, log, diff) are
// deliberately allowed so a future inject-refresh fallback or
// read-only provenance walk is not blocked. If the overnight package
// ever needs to shell out to git for any reason, add an explicit
// allowlist entry here AND document why in the commit message.
func TestRunLoop_NoGitOpsInvoked(t *testing.T) {
	repo := t.TempDir()

	forbiddenSubcommands := map[string]struct{}{
		"commit":      {},
		"push":        {},
		"reset":       {},
		"checkout":    {},
		"rebase":      {},
		"merge":       {},
		"cherry-pick": {},
		"am":          {},
		"apply":       {},
		"restore":     {},
		"switch":      {},
		"stash":       {},
		"clean":       {},
		"tag":         {},
		"branch":      {},
	}

	withExecShim(t, func(name string, args ...string) *exec.Cmd {
		base := filepath.Base(name)
		if base == "git" && len(args) > 0 {
			sub := args[0]
			if _, bad := forbiddenSubcommands[sub]; bad {
				t.Fatalf("forbidden: Dream invoked mutating git op: git %s", strings.Join(args, " "))
			}
		}
		return exec.Command(name, args...)
	})

	opts := newBoundaryOpts(t, repo)
	if _, err := RunLoop(context.Background(), opts); err != nil {
		t.Fatalf("RunLoop: %v", err)
	}
}
