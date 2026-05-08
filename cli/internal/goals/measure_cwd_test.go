package goals

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRunMeasure_RelativeScripts_FromExternalCwd verifies the soc-crzz fix:
// when `ao goals measure --file <abs>/GOALS.md` is invoked from a directory
// outside the repo (e.g. /tmp), goal Check strings that contain relative
// paths must still resolve against the repo containing GOALS.md, not the
// caller's cwd.
//
// The fix lives in withGoalFileCwd (commands.go), which chdir's to the repo
// root for the duration of RunMeasure and restores the prior cwd on exit.
func TestRunMeasure_RelativeScripts_FromExternalCwd(t *testing.T) {
	// Build a fake repo with a goal that runs a relative-path script.
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o700); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	// .git directory alone is not enough — paths.repoRoot uses
	// `git rev-parse --show-toplevel`, so we need a real git init.
	if err := runGit(repo, "init", "-q"); err != nil {
		t.Skipf("git not available: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "scripts"), 0o755); err != nil {
		t.Fatalf("mkdir scripts: %v", err)
	}
	scriptBody := "#!/usr/bin/env bash\necho hello\nexit 0\n"
	if err := os.WriteFile(filepath.Join(repo, "scripts", "ok.sh"), []byte(scriptBody), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	// GOALS.md with one directive whose check uses a relative path.
	goalsMD := `# Fitness Goals

## Mission

Test mission.

## North Stars

- Green CI

## Anti-Stars

- Untested code

## Directives

### 1. Relative-path goal

Run a relative-path script.

**Steer:** increase

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| relative-path-resolves | ` + "`bash scripts/ok.sh`" + ` | 1 | Relative path resolves from repo root |
`
	goalsPath := filepath.Join(repo, "GOALS.md")
	if err := os.WriteFile(goalsPath, []byte(goalsMD), 0o600); err != nil {
		t.Fatalf("write GOALS.md: %v", err)
	}

	// Move cwd OUTSIDE the fake repo to reproduce the bug.
	outside := t.TempDir()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(outside); err != nil {
		t.Fatalf("chdir outside: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })

	// Run the measure command via the same entry point the CLI uses.
	var stdout, stderr bytes.Buffer
	opts := MeasureOptions{
		GoalsFile:    goalsPath,
		Stdout:       &stdout,
		Stderr:       &stderr,
		SnapDir:      filepath.Join(outside, ".agents", "ao", "goals", "baselines"),
		JSON:         true,
		Timeout:      30 * time.Second,
		TotalTimeout: 60 * time.Second,
	}
	if err := RunMeasure(opts); err != nil {
		t.Fatalf("RunMeasure failed: %v\nstdout=%s\nstderr=%s", err, stdout.String(), stderr.String())
	}

	out := stdout.String()
	if strings.Contains(out, "No such file or directory") {
		t.Fatalf("relative-path script did not resolve from external cwd; output:\n%s", out)
	}
	// Must contain the script's stdout, proving it executed against the repo root.
	if !strings.Contains(out, "hello") {
		t.Fatalf("expected goal check to emit %q in measurement output; got:\n%s", "hello", out)
	}

	// Cwd must be restored to `outside` after RunMeasure returns.
	cur, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd post-run: %v", err)
	}
	curResolved, _ := filepath.EvalSymlinks(cur)
	outsideResolved, _ := filepath.EvalSymlinks(outside)
	if curResolved != outsideResolved {
		t.Fatalf("cwd not restored after RunMeasure: got %q, want %q", curResolved, outsideResolved)
	}
}

// TestWithGoalFileCwd_NoRepoFallback verifies that when GoalsFile is not
// inside a git repo, withGoalFileCwd is a no-op (preserves caller cwd).
func TestWithGoalFileCwd_NoRepoFallback(t *testing.T) {
	notARepo := t.TempDir()
	goalsPath := filepath.Join(notARepo, "GOALS.md")
	if err := os.WriteFile(goalsPath, []byte("# stub\n"), 0o600); err != nil {
		t.Fatalf("write goals: %v", err)
	}

	prev, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(prev) })

	starting := t.TempDir()
	if err := os.Chdir(starting); err != nil {
		t.Fatalf("chdir starting: %v", err)
	}

	restore := withGoalFileCwd(goalsPath)
	defer restore()

	cur, _ := os.Getwd()
	curResolved, _ := filepath.EvalSymlinks(cur)
	startingResolved, _ := filepath.EvalSymlinks(starting)
	if curResolved != startingResolved {
		t.Fatalf("cwd should be unchanged when goals file is not in a git repo; got %q, want %q", curResolved, startingResolved)
	}
}

// TestWithGoalFileCwd_EmptyPath verifies that an empty path is a no-op.
func TestWithGoalFileCwd_EmptyPath(t *testing.T) {
	prev, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(prev) })

	restore := withGoalFileCwd("")
	defer restore()

	cur, _ := os.Getwd()
	curResolved, _ := filepath.EvalSymlinks(cur)
	prevResolved, _ := filepath.EvalSymlinks(prev)
	if curResolved != prevResolved {
		t.Fatalf("empty path must be no-op; cwd changed from %q to %q", prevResolved, curResolved)
	}
}

// runGit invokes `git -C dir <args...>`. Returns nil on success.
func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return &gitErr{output: string(out), err: err}
	}
	return nil
}

type gitErr struct {
	output string
	err    error
}

func (g *gitErr) Error() string { return g.err.Error() + ": " + g.output }
