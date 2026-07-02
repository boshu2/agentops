package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/embedded"
)

// TestEmbeddedPawlBundleMatchesRepo is the drift gate for the embedded pawl bundle:
// the canonical scripts live at repo-root scripts/ and the verdict schema at schemas/,
// and the embedded copies (synced by `make sync-hooks`) must stay byte-identical so the
// stranger-path verb runs the SAME review logic as the in-checkout dogfood.
func TestEmbeddedPawlBundleMatchesRepo(t *testing.T) {
	cases := []struct {
		embedPath string
		repoParts []string
	}{
		{"pawl/scripts/pawl-review.sh", []string{"scripts", "pawl-review.sh"}},
		{"pawl/scripts/pawl-verdict.sh", []string{"scripts", "pawl-verdict.sh"}},
		{"pawl/scripts/pawl.sh", []string{"scripts", "pawl.sh"}},
		// pawl-review.sh sources this shared codex runner script-relative
		// ($SCRIPT_DIR/lib/codex-exec.sh), so the stranger/embedded bundle MUST carry it or
		// the cold review cannot start (age-gate-the-ungated-egwt.13).
		{"pawl/scripts/lib/codex-exec.sh", []string{"scripts", "lib", "codex-exec.sh"}},
		// pawl-review.sh also sources the per-repo verify-config hook script-relative
		// ($SCRIPT_DIR/lib/verify-config.sh), so the stranger/embedded bundle must carry it
		// or a stranger repo's checked-in .aoverify.yaml is silently ignored (age-rk3r.17).
		{"pawl/scripts/lib/verify-config.sh", []string{"scripts", "lib", "verify-config.sh"}},
		// The membrane-receipts generator + freshness check ride along so `ao verify
		// receipts` renders a repo's proof page from the embedded bundle on the stranger
		// path (age-rk3r.12); they must stay byte-identical to the repo scripts.
		{"pawl/scripts/gen-membrane-receipts.sh", []string{"scripts", "gen-membrane-receipts.sh"}},
		{"pawl/scripts/check-membrane-receipts-freshness.sh", []string{"scripts", "check-membrane-receipts-freshness.sh"}},
		{"pawl/schemas/pawl-verdict.v1.schema.json", []string{"schemas", "pawl-verdict.v1.schema.json"}},
	}
	for _, tc := range cases {
		t.Run(tc.embedPath, func(t *testing.T) {
			embeddedData, err := embedded.PawlFS.ReadFile(tc.embedPath)
			if err != nil {
				t.Fatalf("read embedded %s: %v", tc.embedPath, err)
			}
			repoData, err := os.ReadFile(findRepoFileForTest(t, tc.repoParts...))
			if err != nil {
				t.Fatalf("read repo %v: %v", tc.repoParts, err)
			}
			if string(embeddedData) != string(repoData) {
				t.Fatalf("embedded cli/embedded/%s drifted from repo %s; run 'cd cli && make sync-hooks'",
					tc.embedPath, filepath.Join(tc.repoParts...))
			}
		})
	}
}

// TestExtractPawlBundle proves the cold-path extraction preserves the scripts/ + schemas/
// SIBLING layout pawl-verdict.sh depends on (EDGE 2: it reads its schema script-relative
// as $SCRIPT_DIR/../schemas/...), and that the scripts come out executable.
func TestExtractPawlBundle(t *testing.T) {
	dir, cleanup, err := extractPawlBundle()
	if err != nil {
		t.Fatalf("extractPawlBundle: %v", err)
	}
	defer cleanup()

	wantExec := []string{
		filepath.Join("scripts", "pawl-review.sh"),
		filepath.Join("scripts", "pawl-verdict.sh"),
		filepath.Join("scripts", "pawl.sh"),
		// The shared codex runner must extract into the sibling scripts/lib/ that
		// pawl-review.sh sources; the nested-dir walk + exec-normalize must handle it
		// (age-gate-the-ungated-egwt.13).
		filepath.Join("scripts", "lib", "codex-exec.sh"),
		// The per-repo verify-config hook extracts into the same sibling scripts/lib/ so the
		// embedded pawl-review sources it and honors a stranger repo's .aoverify.yaml (age-rk3r.17).
		filepath.Join("scripts", "lib", "verify-config.sh"),
		// The membrane-receipts generator + freshness check must extract executable so
		// `ao verify receipts` renders a repo's proof page from the bundle on the
		// stranger path (age-rk3r.12).
		filepath.Join("scripts", "gen-membrane-receipts.sh"),
		filepath.Join("scripts", "check-membrane-receipts-freshness.sh"),
	}
	for _, rel := range wantExec {
		info, statErr := os.Stat(filepath.Join(dir, rel))
		if statErr != nil {
			t.Fatalf("extracted %s missing: %v", rel, statErr)
		}
		if info.Mode().Perm()&0o100 == 0 {
			t.Fatalf("extracted %s is not executable (mode %v)", rel, info.Mode().Perm())
		}
	}

	// EDGE 2: the schema must be a sibling of scripts/ so pawl-verdict.sh's
	// $REPO_ROOT/schemas/pawl-verdict.v1.schema.json (REPO_ROOT=$SCRIPT_DIR/..) resolves.
	schema := filepath.Join(dir, "schemas", "pawl-verdict.v1.schema.json")
	if _, statErr := os.Stat(schema); statErr != nil {
		t.Fatalf("extracted schema sibling missing (EDGE 2 — fail-closed false-REFUTE risk): %v", statErr)
	}
}

// TestPawlReviewColdEnv locks the stranger-path env seam contract: re-root onto the
// user's repo (AGENTOPS_REPO_ROOT) and disable the standing-service probe so a one-shot
// cold run never tries to stand up a warm pane.
func TestPawlReviewColdEnv(t *testing.T) {
	env := pawlReviewColdEnv("/home/stranger/their-repo")
	// Exact seams: re-root onto the user's repo, no warm pane, and — critically — mark the
	// repo under review UNTRUSTED so the script never executes $REPO_ROOT/cli/* (RCE guard).
	want := map[string]bool{
		"AGENTOPS_REPO_ROOT=/home/stranger/their-repo": false,
		"PAWL_NO_SERVICE=1":                            false,
		"PAWL_UNTRUSTED_REPO=1":                        false,
	}
	var sawAOBin bool
	for _, e := range env {
		if _, ok := want[e]; ok {
			want[e] = true
		}
		if strings.HasPrefix(e, "AO_BIN=") && len(e) > len("AO_BIN=") {
			sawAOBin = true
		}
	}
	for k, seen := range want {
		if !seen {
			t.Fatalf("pawlReviewColdEnv missing %q (got %v)", k, env)
		}
	}
	// AO_BIN must pin the trusted invoking binary so the membrane catch/recall never
	// resolve ao from the untrusted repo (os.Executable() resolves in the test binary).
	if !sawAOBin {
		t.Fatalf("pawlReviewColdEnv missing a non-empty AO_BIN (trusted-binary pin); got %v", env)
	}
}

// useFakeSelfAo points pawlSelfBinary at a benign no-op ao so the cold-path AO_BIN does
// not resolve to the test binary itself (which the script would re-invoke with
// `membrane recall …`, re-running the whole suite). Returns nothing; restores on cleanup.
func useFakeSelfAo(t *testing.T) {
	t.Helper()
	fake := filepath.Join(t.TempDir(), "ao")
	if err := os.WriteFile(fake, []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	prev := pawlSelfBinary
	pawlSelfBinary = func() (string, error) { return fake, nil }
	t.Cleanup(func() { pawlSelfBinary = prev })
}

// TestTrustedPATH drops every entry the untrusted repo could control: "", ".", any
// relative path, AND any absolute dir inside excludeRoot (e.g. $PWD/bin while reviewing) —
// keeping the remaining absolute dirs in order.
func TestTrustedPATH(t *testing.T) {
	sep := string(os.PathListSeparator)
	repo := t.TempDir()
	inside := filepath.Join(repo, "bin") // an absolute entry that lives in the repo
	t.Setenv("PATH", strings.Join([]string{"/usr/bin", ".", "", "rel/dir", inside, "/opt/x", "../up"}, sep))

	if got, want := trustedPATH(repo), strings.Join([]string{"/usr/bin", "/opt/x"}, sep); got != want {
		t.Fatalf("trustedPATH(repo) = %q, want %q (repo-internal %q must be dropped)", got, want, inside)
	}
	// excludeRoot "" still strips relative/"." entries but keeps the absolute repo dir.
	if got, want := trustedPATH(""), strings.Join([]string{"/usr/bin", inside, "/opt/x"}, sep); got != want {
		t.Fatalf("trustedPATH(\"\") = %q, want %q", got, want)
	}
}

// TestGitToplevel_NoExec proves repo-root discovery is pure Go: it must NOT execute any
// `git` binary (a repo-planted git earlier on PATH would otherwise run before the repo's
// trust is even established). It walks up for `.git` and returns the symlink-resolved root.
func TestGitToplevel_NoExec(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(repo, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	// Hostile git FIRST on PATH: if gitToplevel execs `git`, this drops a sentinel.
	bin := t.TempDir()
	sentinel := filepath.Join(bin, "GIT_RAN")
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte("#!/usr/bin/env bash\ntouch "+sentinel+"\necho /wrong/root\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	got, err := gitToplevel(sub)
	if err != nil {
		t.Fatalf("gitToplevel: %v", err)
	}
	if want := realpathOrSelf(repo); got != want {
		t.Fatalf("gitToplevel(sub) = %q, want repo root %q", got, want)
	}
	if _, err := os.Stat(sentinel); err == nil {
		t.Fatal("SECURITY: gitToplevel executed a PATH-resolved git during root discovery")
	}
}

// TestRunPawlReview_ColdRepoDoesNotRunRepoBinary is the regression test for the cross-family
// refuter's finding on age-a9iv.4: on the stranger path REPO_ROOT is the untrusted repo under
// review, so resolve_ao MUST NOT execute $REPO_ROOT/cli/bin/ao. A repo that plants a hostile
// cli/bin/ao (write a sentinel) must never get it run by `ao pawl review` (which would be
// arbitrary code-exec before the read-only review, via recall_prior_catches).
func TestRunPawlReview_ColdRepoDoesNotRunRepoBinary(t *testing.T) {
	for _, tool := range []string{"bash", "git", "jq"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("hostile cold-repo test needs %s on PATH", tool)
		}
	}
	useFakeSelfAo(t)

	repo := t.TempDir()
	gitRun := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@e.com",
			"GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@e.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	gitRun("init", "--quiet")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("init\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun("add", "README.md")
	gitRun("commit", "--quiet", "-m", "init")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("init\nchange\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun("add", "README.md")
	gitRun("commit", "--quiet", "-m", "feat: change (age-rev-test)")

	// Plant HOSTILE binaries in the repo under review covering EVERY bare-binary vector:
	// $REPO_ROOT/cli/bin/ao + cli/ao (resolve_ao's repo-build search), a bare ./ao, AND a
	// bare ./codex (the refuter binary). All would resolve from the repo iff the scripts
	// ran with cwd inside it and `.` is on PATH. If any executes it drops the shared
	// sentinel; with cwd pinned to the trusted cache + the AO_BIN guards, none must run.
	sentinel := filepath.Join(repo, "PWNED")
	hostile := "#!/usr/bin/env bash\ntouch " + sentinel + "\nexit 0\n"
	if err := os.MkdirAll(filepath.Join(repo, "cli", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{filepath.Join("cli", "bin", "ao"), filepath.Join("cli", "ao"), "ao", "codex"} {
		if err := os.WriteFile(filepath.Join(repo, rel), []byte(hostile), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// BASH_ENV injection: non-interactive bash sources this BEFORE the script. The cold env
	// neutralizes BASH_ENV/ENV, so this repo-controlled file must never be sourced.
	evil := filepath.Join(repo, "evil.sh")
	if err := os.WriteFile(evil, []byte("touch "+sentinel+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BASH_ENV", evil)

	bin := t.TempDir()
	stub := "#!/usr/bin/env bash\ncat >/dev/null\necho codex\necho 'Reviewed; no defects.'\necho 'VERDICT: CONFIRMED'\n"
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	// "." FIRST in PATH + cwd = the repo: the bare-binary RCE precondition. The cold path
	// SANITIZES PATH (drops "."/relative entries) before running, so the real bin/codex is
	// used and nothing from the repo runs. A regression that dropped the sanitize would let
	// ./codex (and ./ao) resolve from the repo here and trip the sentinel.
	t.Setenv("PATH", "."+string(os.PathListSeparator)+bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	prevDir := testProjectDir
	prevSU, prevSE := pawlReviewCmd.SilenceUsage, pawlReviewCmd.SilenceErrors
	testProjectDir = repo
	t.Cleanup(func() {
		testProjectDir = prevDir
		pawlReviewCmd.SilenceUsage = prevSU
		pawlReviewCmd.SilenceErrors = prevSE
	})

	// We don't care about the verdict here, only that the planted binary never executed.
	_ = runPawlReview(pawlReviewCmd, []string{"age-rev-test", "--scope", "head"})

	if _, err := os.Stat(sentinel); err == nil {
		t.Fatal("SECURITY: planted $REPO_ROOT/cli/bin/ao was EXECUTED on the stranger path (arbitrary code-exec from the untrusted repo under review)")
	}
}

// TestRunPawlReview_ColdRepoWritesVerdict is the S2 acceptance integration test: in a
// throwaway non-AgentOps git repo (NO docs/contracts/agents-write-surfaces.md, NO
// skills/) with codex stubbed on PATH, `ao pawl review --scope head` must NOT die with
// "locating AgentOps repo root", must run the EMBEDDED scripts, and on a CONFIRMED
// refuter must write a commit-bound verdict JSON into the USER's repo that passes the
// schema-validated check (exit 0). This exercises the whole cold path incl. EDGE 2.
func TestRunPawlReview_ColdRepoWritesVerdict(t *testing.T) {
	for _, tool := range []string{"bash", "git", "jq"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("cold-repo e2e needs %s on PATH", tool)
		}
	}
	useFakeSelfAo(t)

	repo := t.TempDir()
	gitRun := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@e.com",
			"GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@e.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	gitRun("init", "--quiet")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("init\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun("add", "README.md")
	gitRun("commit", "--quiet", "-m", "init")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("init\nchange\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun("add", "README.md")
	gitRun("commit", "--quiet", "-m", "feat(x): a change (age-rev-test)")

	// Cold-repo sanity: this dir must NOT resolve as an AgentOps checkout.
	if !strings.Contains(repo, string(os.PathSeparator)) { // defensive
		t.Fatal("unexpected temp repo path")
	}

	// Fake codex refuter on PATH: marker line + CONFIRMED verdict, clean exit.
	bin := t.TempDir()
	stub := "#!/usr/bin/env bash\ncat >/dev/null\necho codex\necho 'Reviewed; no defects.'\necho 'VERDICT: CONFIRMED'\n"
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	// Point resolution at the cold repo; restore shared globals (cmd/ao isolation contract).
	prevDir := testProjectDir
	prevSU, prevSE := pawlReviewCmd.SilenceUsage, pawlReviewCmd.SilenceErrors
	testProjectDir = repo
	t.Cleanup(func() {
		testProjectDir = prevDir
		pawlReviewCmd.SilenceUsage = prevSU
		pawlReviewCmd.SilenceErrors = prevSE
	})

	if err := runPawlReview(pawlReviewCmd, []string{"age-rev-test", "--scope", "head"}); err != nil {
		t.Fatalf("cold-repo pawl review should exit 0 (CONFIRMED + verdict written + verified), got %v", err)
	}

	// The verdict must land in the USER's repo (re-rooted), not anywhere in the source tree.
	toplevel, err := gitToplevel(repo)
	if err != nil {
		t.Fatalf("gitToplevel: %v", err)
	}
	verdict := filepath.Join(toplevel, ".agents", "pawl-verdicts", "age-rev-test.json")
	data, err := os.ReadFile(verdict)
	if err != nil {
		t.Fatalf("verdict JSON not written to the user's repo at %s: %v", verdict, err)
	}
	if !strings.Contains(string(data), `"disposition": "CONFIRMED"`) {
		t.Fatalf("verdict missing CONFIRMED disposition:\n%s", data)
	}
	head, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), strings.TrimSpace(string(head))) {
		t.Fatalf("verdict not bound to HEAD sha %s:\n%s", strings.TrimSpace(string(head)), data)
	}
}

// TestRunPawlReview_ForgedMarkersUsesEmbedded is the regression test for the second
// cross-family refuter finding on age-a9iv.4: a repo under review can FORGE the AgentOps
// marker files (docs/contracts/agents-write-surfaces.md + skills/ + scripts/pawl-review.sh)
// to make resolveAgentsRepoRoot() succeed. With an INSTALLED ao (binary outside the repo)
// the live path must NOT be taken — otherwise `ao pawl review` would execute the repo's
// PLANTED scripts/pawl-review.sh (RCE). The trust gate (aoBinaryInside) must route to the
// embedded bundle instead, never running the hostile script.
func TestRunPawlReview_ForgedMarkersUsesEmbedded(t *testing.T) {
	for _, tool := range []string{"bash", "git", "jq"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("forged-markers test needs %s on PATH", tool)
		}
	}
	useFakeSelfAo(t) // installed-ao simulation: trusted binary lives OUTSIDE the repo.

	repo := t.TempDir()
	gitRun := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@e.com",
			"GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@e.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	gitRun("init", "--quiet")

	// FORGE the AgentOps markers so resolveAgentsRepoRoot() accepts this untrusted repo.
	if err := os.MkdirAll(filepath.Join(repo, "docs", "contracts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "docs", "contracts", "agents-write-surfaces.md"), []byte("forged"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	// HOSTILE planted pawl-review.sh: drops a sentinel if ever executed.
	sentinel := filepath.Join(repo, "PWNED")
	if err := os.MkdirAll(filepath.Join(repo, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	hostile := "#!/usr/bin/env bash\ntouch " + sentinel + "\nexit 0\n"
	if err := os.WriteFile(filepath.Join(repo, "scripts", "pawl-review.sh"), []byte(hostile), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("init\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun("add", "-A")
	gitRun("commit", "--quiet", "-m", "forged markers + hostile pawl-review.sh")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("init\nchange\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun("add", "README.md")
	gitRun("commit", "--quiet", "-m", "feat: change (age-rev-test)")

	bin := t.TempDir()
	stub := "#!/usr/bin/env bash\ncat >/dev/null\necho codex\necho 'Reviewed; no defects.'\necho 'VERDICT: CONFIRMED'\n"
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	prevDir := testProjectDir
	prevSU, prevSE := pawlReviewCmd.SilenceUsage, pawlReviewCmd.SilenceErrors
	testProjectDir = repo
	t.Cleanup(func() {
		testProjectDir = prevDir
		pawlReviewCmd.SilenceUsage = prevSU
		pawlReviewCmd.SilenceErrors = prevSE
	})

	_ = runPawlReview(pawlReviewCmd, []string{"age-rev-test", "--scope", "head"})

	if _, err := os.Stat(sentinel); err == nil {
		t.Fatal("SECURITY: forged markers caused the repo's PLANTED scripts/pawl-review.sh to be EXECUTED (RCE via trust-boundary forgery)")
	}
}
