package checks

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/gates"
	"github.com/boshu2/agentops/cli/internal/ports"
	"github.com/boshu2/agentops/cli/internal/testsupport"
)

// TestMain scrubs git's hook-injected repository-discovery env
// (GIT_DIR, GIT_WORK_TREE, ...) before any test in this package runs. Tests
// here shell out to git through production code; with those vars leaked in
// every fixture git operation is redirected to the REAL repo, which is how
// core.bare=true corrupted the shared .git/config
// (age-cmdao-core-bare-pollution-ek8v, recurred 2026-07-18).
func TestMain(m *testing.M) {
	testsupport.ScrubGitDiscoveryEnv()
	os.Exit(m.Run())
}

// initRepoWithOriginMain builds a temp git repo with a refs/remotes/origin/main
// ref at the base commit and a HEAD commit that adds addFile, returning the
// repo root. `git diff --name-only origin/main...HEAD` in it yields [addFile].
func initRepoWithOriginMain(t *testing.T, addFile string) string {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) string {
		c := exec.Command("git", args...)
		c.Dir = root
		c.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(rel, body string) {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q")
	write("README.md", "base\n")
	run("add", "-A")
	run("commit", "-q", "-m", "base")
	base := run("rev-parse", "HEAD")
	run("update-ref", "refs/remotes/origin/main", base)
	write(addFile, "x\n")
	run("add", "-A")
	run("commit", "-q", "-m", "add "+addFile)
	return root
}

// TestChangedFilesFor_PollutedGitDirFallbackResolvesCorrectRepo is the
// acceptance for the SECURITY-MED fix on the changedFilesFor fallback path
// (the `git diff origin/main...HEAD` run when the orchestrator did not route):
// a leaked GIT_DIR must not route the fallback change set at the wrong repo.
func TestChangedFilesFor_PollutedGitDirFallbackResolvesCorrectRepo(t *testing.T) {
	correct := initRepoWithOriginMain(t, "correct.sh")
	polluted := initRepoWithOriginMain(t, "wrong.sh")

	// Simulate git's hook-injected discovery env pointing at a DIFFERENT repo.
	t.Setenv("GIT_DIR", filepath.Join(polluted, ".git"))

	got := changedFilesFor(context.Background(), gates.RunContext{RepoRoot: correct})
	want := []string{"correct.sh"}
	if !equalSetChecks(got, want) {
		t.Fatalf("polluted GIT_DIR routed fallback change set to %v, want %v (from cmd.Dir repo)", got, want)
	}
}

// TestRunShellcheckChanged_MissingShellcheckFailsClosed pins acceptance (b): a
// missing shellcheck is a BLOCKING FAIL, not a SKIP. A SKIP clears a blocking
// check (orchestrator.isBlockingFail), so an absent shellcheck would silently
// pass shell files. The observable proof is Report.ExitCode()==1 — the same
// fail-closed treatment ScriptRunner gives UNKNOWN.
func TestRunShellcheckChanged_MissingShellcheckFailsClosed(t *testing.T) {
	// A PATH with no shellcheck on it (shellcheck IS installed on this host, so
	// we shrink PATH rather than uninstall anything).
	t.Setenv("PATH", t.TempDir())

	verdict, err := runShellcheckChanged(context.Background(), gates.RunContext{RepoRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("runShellcheckChanged: %v", err)
	}
	if verdict.Status != ports.GateStatusFail {
		t.Fatalf("status = %s, want FAIL (missing shellcheck must fail closed)", verdict.Status)
	}

	// Pin the observable exit code: a blocking check with this verdict exits 1.
	rep := &gates.Report{Results: []gates.CheckResult{{
		Check:   gates.Check{ID: "shell.shellcheck-changed", Blocking: true},
		Verdict: verdict,
	}}}
	if got := rep.ExitCode(); got != 1 {
		t.Fatalf("Report.ExitCode() = %d, want 1 for a blocking missing-shellcheck FAIL", got)
	}
}

// equalSetChecks reports whether a and b hold the same elements (order-independent).
func equalSetChecks(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]int{}
	for _, x := range a {
		seen[x]++
	}
	for _, x := range b {
		seen[x]--
	}
	for _, n := range seen {
		if n != 0 {
			return false
		}
	}
	return true
}

func TestRunChangelogSync_ReadFailureFailsClosed(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte("current\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	verdict, err := runChangelogSync(context.Background(), gates.RunContext{RepoRoot: root})
	if err != nil {
		t.Fatalf("runChangelogSync: %v", err)
	}
	if verdict.Status != ports.GateStatusFail {
		t.Fatalf("status = %s, want FAIL for missing applicable evidence", verdict.Status)
	}
	if !strings.Contains(verdict.Reason, "docs/CHANGELOG.md") {
		t.Fatalf("reason = %q, want missing evidence path", verdict.Reason)
	}
}
