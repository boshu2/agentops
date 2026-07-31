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

// TestLearningCoherence_RoutesBothLearningRoots pins the routing globs of the
// learning.coherence gate: a learning changed under the CANONICAL
// .agents/ao/learnings/ (where doctor's split fixer migrates files) must select
// the gate exactly like the legacy .agents/learnings/ root — otherwise a
// migrated learning silently skips the frontmatter check.
func TestLearningCoherence_RoutesBothLearningRoots(t *testing.T) {
	var check gates.Check
	found := false
	for _, c := range gates.Default.All() {
		if c.ID == "learning.coherence" {
			check, found = c, true
			break
		}
	}
	if !found {
		t.Fatal("learning.coherence is not registered in the default gate registry")
	}
	for _, f := range []string{".agents/ao/learnings/x.md", ".agents/learnings/x.md"} {
		if !gates.PathMatchesAny(check.Match, f) {
			t.Errorf("learning.coherence Match %v must route changed file %q", check.Match, f)
		}
	}
}

// TestRunLearningCoherence_ChecksCanonicalAoRoot: the Run func must inspect a
// changed learning under the canonical .agents/ao/learnings/ root, not only the
// legacy .agents/learnings/ one. A frontmatter-less file under the canonical
// root is a FAIL; adding frontmatter turns it PASS.
func TestRunLearningCoherence_ChecksCanonicalAoRoot(t *testing.T) {
	root := t.TempDir()
	rel := filepath.Join(".agents", "ao", "learnings", "bad.md")
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("no frontmatter here\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rc := gates.RunContext{RepoRoot: root, ChangedFiles: []string{".agents/ao/learnings/bad.md"}}
	verdict, err := runLearningCoherence(context.Background(), rc)
	if err != nil {
		t.Fatalf("runLearningCoherence: %v", err)
	}
	if verdict.Status != ports.GateStatusFail {
		t.Fatalf("status = %s, want FAIL for a frontmatter-less canonical-root learning", verdict.Status)
	}
	if !strings.Contains(verdict.Reason, ".agents/ao/learnings/bad.md") {
		t.Fatalf("reason = %q, want the canonical-root file listed", verdict.Reason)
	}

	if err := os.WriteFile(full, []byte("---\ntitle: x\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	verdict, err = runLearningCoherence(context.Background(), rc)
	if err != nil {
		t.Fatalf("runLearningCoherence: %v", err)
	}
	if verdict.Status != ports.GateStatusPass {
		t.Fatalf("status = %s, want PASS once frontmatter is present (reason %q)", verdict.Status, verdict.Reason)
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

// shellcheckTriggeringScript is a shell body shellcheck flags at the gate's
// own -S warning threshold (SC2164: an unguarded `cd`). SC2086-style quoting
// findings are only "info" and would not trip the gate.
const shellcheckTriggeringScript = "#!/usr/bin/env bash\ncd /tmp\n"

// initRepoWithHeadCommit builds a temp git repo whose single HEAD commit adds
// every path in files (rel -> body), returning the repo root. `--scope head`
// in it yields exactly those paths — the shape of a user's first commit after
// installing AgentOps into their own project.
func initRepoWithHeadCommit(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = root
		c.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	for rel, body := range files {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o755); err != nil { // #nosec G306 -- fixture shell scripts.
			t.Fatal(err)
		}
	}
	run("add", "-A")
	run("commit", "-q", "-m", "first commit")
	return root
}

// runRealShellGate runs the REAL registered shell.shellcheck-changed check
// through the real orchestrator against a fixture repo, exactly as `ao gate
// check` does, and returns the report's exit code.
func runRealShellGate(t *testing.T, root string) int {
	t.Helper()
	registered, ok := gates.Default.Get("shell.shellcheck-changed")
	if !ok {
		t.Fatal("shell.shellcheck-changed is not registered")
	}
	reg := gates.NewRegistry()
	if err := reg.Add(registered); err != nil {
		t.Fatal(err)
	}
	report, err := gates.NewOrchestrator(reg, nil, gates.NewGitChangedFiles(root), root).
		Run(context.Background(), gates.RunOptions{Mode: gates.Fast, Scope: gates.ScopeHead})
	if err != nil {
		t.Fatalf("orchestrator Run: %v", err)
	}
	return report.ExitCode()
}

// TestRunShellcheckChanged_SkipsInstalledSkillCopies is the L2 acceptance for
// the observed fresh-install failure: after `ao init` + a first commit, `ao
// gate check` FAILED shellcheck on AgentOps' OWN installed skill scripts
// (.agents/skills/cass/scripts/multi_machine_search.sh matched `**/*.sh`) — a
// gate the user had no way to repair, on files they did not write. Installed
// skill copies must not be gated; the user's own shell files still must be.
func TestRunShellcheckChanged_SkipsInstalledSkillCopies(t *testing.T) {
	if _, err := exec.LookPath("shellcheck"); err != nil {
		t.Skip("shellcheck not installed")
	}

	installedOnly := initRepoWithHeadCommit(t, map[string]string{
		"README.md": "hello\n",
		".agents/skills/cass/scripts/multi_machine_search.sh": shellcheckTriggeringScript,
		".claude/skills/plan/scripts/run.sh":                  shellcheckTriggeringScript,
	})
	if got := runRealShellGate(t, installedOnly); got != 0 {
		t.Fatalf("exit code = %d, want 0 (installed skill copies must not fail a user's gate)", got)
	}

	// Negative witness: the same bad script under a first-party path still FAILs,
	// so the exclusion narrows scope rather than defanging the gate.
	firstParty := initRepoWithHeadCommit(t, map[string]string{
		"README.md":           "hello\n",
		"scripts/deploy.sh":   shellcheckTriggeringScript,
		".agents/skills/x.sh": shellcheckTriggeringScript,
	})
	if got := runRealShellGate(t, firstParty); got != 1 {
		t.Fatalf("exit code = %d, want 1 (a first-party shell file must still fail)", got)
	}
}

// TestRegisteredNativeChecks_RepairHintsAreAudienceSafe is the registry-wide
// invariant behind the second half of the same defect: the failing gate's
// repair text said "inspect native gate shell.shellcheck-changed in
// cli/internal/gates" — a path that does not exist on a machine that installed
// the CLI.
//
// Scope is the NATIVE checks, and deliberately so. A script-backed check runs
// only inside the agentops repository (ScriptRunner returns a first-class
// not-applicable SKIP elsewhere, gates.NotApplicableReason), so its
// `bash scripts/...` rerun hint addresses a reader who has the checkout by
// construction. The native checks carry no such guard: they execute in a user's
// own repository, so their repair text must be actionable from there.
func TestRegisteredNativeChecks_RepairHintsAreAudienceSafe(t *testing.T) {
	sourceOnly := []string{"cli/internal/", "cli/cmd/"}
	checked := 0
	for _, check := range gates.Default.All() {
		if check.Run == nil {
			continue
		}
		checked++
		hint := check.EffectiveRepairHint()
		if hint == "" {
			t.Errorf("native check %q has an empty repair hint", check.ID)
			continue
		}
		for _, needle := range sourceOnly {
			if strings.Contains(hint, needle) {
				t.Errorf("native check %q repair hint names source-checkout path %q: %s", check.ID, needle, hint)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no native checks registered; the invariant proved nothing")
	}
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
