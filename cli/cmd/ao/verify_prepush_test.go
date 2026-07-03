// practices: [design-by-contract, in-toto-provenance]
package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// --- test helpers -----------------------------------------------------------

// cleanGitEnv returns the process env with git's discovery variables stripped so
// a `git -C <tempRepo>` op in a test can never accidentally target the real repo
// via a leaked GIT_DIR (go.md test-isolation rule).
func cleanGitEnv() []string {
	var env []string
	for _, e := range os.Environ() {
		switch {
		case strings.HasPrefix(e, "GIT_DIR="),
			strings.HasPrefix(e, "GIT_WORK_TREE="),
			strings.HasPrefix(e, "GIT_INDEX_FILE="),
			strings.HasPrefix(e, "GIT_COMMON_DIR="),
			strings.HasPrefix(e, "GIT_PREFIX="),
			strings.HasPrefix(e, "GIT_OBJECT_DIRECTORY="),
			strings.HasPrefix(e, "GIT_NAMESPACE="):
			continue
		}
		env = append(env, e)
	}
	return env
}

// runGitT runs `git -C repo args...` in an isolated env, failing the test on error.
func runGitT(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	cmd.Env = cleanGitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// gitInitRepoT creates a real, isolated git repo with an initial commit and
// returns its (symlink-resolved) path. main is the default branch.
func gitInitRepoT(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	runGitT(t, dir, "init", "-q", "--initial-branch=main")
	runGitT(t, dir, "config", "user.name", "test")
	runGitT(t, dir, "config", "user.email", "test@example.com")
	runGitT(t, dir, "config", "commit.gpgsign", "false")
	return dir
}

// commitFileT writes file=content in repo and commits it with msg, returning the
// new HEAD sha.
func commitFileT(t *testing.T, repo, file, content, msg string) string {
	t.Helper()
	full := filepath.Join(repo, file)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", file, err)
	}
	runGitT(t, repo, "add", file)
	runGitT(t, repo, "commit", "-q", "-m", msg)
	return runGitT(t, repo, "rev-parse", "HEAD")
}

// bindVerdictT appends a CONFIRMED verdict edge bound to codeSHA to the repo's
// ledger and commits it as a #trivial provenance-only commit (mirroring the real
// bind flow). Returns the bind commit's sha.
func bindVerdictT(t *testing.T, repo, bead, codeSHA string) string {
	t.Helper()
	ledger := filepath.Join(repo, provenancegraph.LedgerRelativePath)
	store := provenancegraph.NewStore(ledger)
	_, err := store.Append(provenancegraph.Edge{
		FromID:      bead + "@" + codeSHA[:7],
		FromType:    "verdict",
		ToID:        codeSHA,
		ToType:      "commit",
		Relation:    "wasDerivedFrom",
		EvidenceRef: "pawl-verdict " + bead + " disposition=CONFIRMED",
		TrustTier:   "inferred",
		TS:          "2026-07-02T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("append verdict edge: %v", err)
	}
	runGitT(t, repo, "add", provenancegraph.LedgerRelativePath)
	runGitT(t, repo, "commit", "-q", "-m", "chore(provenance): bind verdict for "+bead+" #trivial")
	return runGitT(t, repo, "rev-parse", "HEAD")
}

// runGateT invokes the pre-push gate with cwd=repo and the given stdin, returning
// the exit code (0 allow, 1 refuse) and combined output.
func runGateT(t *testing.T, repo, stdin string) (int, string) {
	t.Helper()
	t.Chdir(repo)
	var buf bytes.Buffer
	c := &cobra.Command{}
	c.SetIn(strings.NewReader(stdin))
	c.SetOut(&buf)
	c.SetErr(&buf)
	err := runVerifyPrePush(c, nil)
	if err == nil {
		return 0, buf.String()
	}
	var exitErr *verifyPrePushExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), buf.String()
	}
	t.Fatalf("unexpected non-exit error: %v", err)
	return -1, buf.String()
}

// runGateRemoteT is runGateT but forwards a git remote NAME as the gate's argv[0]
// (git invokes the pre-push hook as `pre-push <remote-name> <remote-url>`), so the
// gate scopes "already verified" to THAT target remote's trunk.
func runGateRemoteT(t *testing.T, repo, remote, stdin string) (int, string) {
	t.Helper()
	t.Chdir(repo)
	var buf bytes.Buffer
	c := &cobra.Command{}
	c.SetIn(strings.NewReader(stdin))
	c.SetOut(&buf)
	c.SetErr(&buf)
	err := runVerifyPrePush(c, []string{remote, "file://" + repo})
	if err == nil {
		return 0, buf.String()
	}
	var exitErr *verifyPrePushExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), buf.String()
	}
	t.Fatalf("unexpected non-exit error: %v", err)
	return -1, buf.String()
}

const zeroSHA = "0000000000000000000000000000000000000000"

// pushLine formats a git pre-push stdin record.
func pushLine(remoteRef, localSHA, remoteSHA string) string {
	return "refs/heads/x " + localSHA + " " + remoteRef + " " + remoteSHA + "\n"
}

// --- tests ------------------------------------------------------------------

// The cross-family refuter's exact repro (age-rk3r.6 amend): an appended-but-
// UNCOMMITTED ledger edge must NOT satisfy the gate. Proof is what the PUSHED
// TREE carries — the remote never sees the working tree, so a working-tree
// ledger read here is a fail-open on the core contract (no verdict = not done).
func TestPrePush_RefusesUncommittedLedgerEdge(t *testing.T) {
	repo := gitInitRepoT(t)
	base := commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	code := commitFileT(t, repo, "app.go", "package main\n", "feat: change (age-r1)")

	// Append a CONFIRMED, commit-bound edge WITHOUT git-adding or committing it:
	// the ledger exists only in the working tree, not in any pushed tree.
	ledger := filepath.Join(repo, provenancegraph.LedgerRelativePath)
	if _, err := provenancegraph.NewStore(ledger).Append(provenancegraph.Edge{
		FromID: "age-r1@" + code[:7], FromType: "verdict",
		ToID: code, ToType: "commit", Relation: "wasDerivedFrom",
		EvidenceRef: "pawl-verdict age-r1 disposition=CONFIRMED", TrustTier: "inferred",
		TS: "2026-07-02T00:00:00Z",
	}); err != nil {
		t.Fatalf("append uncommitted edge: %v", err)
	}

	rc, out := runGateT(t, repo, pushLine("refs/heads/main", code, base))
	if rc != 1 {
		t.Fatalf("an uncommitted ledger edge must NOT authorize the push: got exit %d\n%s", rc, out)
	}
	if !strings.Contains(out, "ao verify") {
		t.Fatalf("refusal must name `ao verify` as the fix:\n%s", out)
	}
}

// Positive twin of the repro: the SAME edge, COMMITTED into the pushed tip's
// tree, authorizes the push.
func TestPrePush_AllowsCommittedLedgerEdgeAtTip(t *testing.T) {
	repo := gitInitRepoT(t)
	base := commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	code := commitFileT(t, repo, "app.go", "package main\n", "feat: change (age-r2)")
	tip := bindVerdictT(t, repo, "age-r2", code) // commits the ledger (#trivial bind)

	rc, out := runGateT(t, repo, pushLine("refs/heads/main", tip, base))
	if rc != 0 {
		t.Fatalf("a committed edge in the pushed tip must authorize: got %d\n%s", rc, out)
	}
}

// Locks the tip-tree semantics from the other direction: a clean COMMITTED
// ledger must authorize even when the WORKING-TREE copy is tampered — the gate
// verifies the bytes the push actually transports, not local dirt.
func TestPrePush_IgnoresWorktreeOnlyLedgerTamper(t *testing.T) {
	repo := gitInitRepoT(t)
	base := commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	code := commitFileT(t, repo, "app.go", "package main\n", "feat: change (age-r3)")
	tip := bindVerdictT(t, repo, "age-r3", code)

	// Tamper ONLY the working tree; the committed tip stays intact.
	ledger := filepath.Join(repo, provenancegraph.LedgerRelativePath)
	data, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	dirty := strings.Replace(string(data), "disposition=CONFIRMED", "disposition=TAMPERED", 1)
	if dirty == string(data) {
		t.Fatalf("tamper precondition not met")
	}
	if err := os.WriteFile(ledger, []byte(dirty), 0o644); err != nil {
		t.Fatalf("write ledger: %v", err)
	}

	rc, out := runGateT(t, repo, pushLine("refs/heads/main", tip, base))
	if rc != 0 {
		t.Fatalf("worktree-only tamper must not affect the gate (it reads the pushed tree): got %d\n%s", rc, out)
	}
}

// The round-4 refuter's repro: a hostile repo plants an executable `git` INSIDE
// the repo and gets its directory onto PATH. The live vector is an absolute
// repo-internal PATH entry (e.g. a direnv-style $PWD/bin — Go's exec already
// refuses "."/relative PATH results via ErrDot, but absolute repo-internal
// entries resolve normally). The hook runs with cwd = the repo worktree, so a
// bare exec.Command("git") would execute the planted binary and let it forge
// rev-parse/cat-file/rev-list results into a silent PASS — violating the gate's
// own "NO repo-tree code is trusted" boundary. The gate must resolve git on a
// sanitized PATH and NEVER execute the planted one.
func TestPrePush_PlantedRepoGitNeverExecuted(t *testing.T) {
	repo := gitInitRepoT(t)
	base := commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	tip := commitFileT(t, repo, "app.go", "package main\n", "feat: unverified (age-p1)")

	// Plant <repo>/bin/git: touches a sentinel when executed and forges every
	// answer the gate asks for — rev-parse ok (exit 0), "no ledger" for
	// cat-file (exit 1), and an EMPTY rev-list (exit 0, no stdout) so the
	// checked range collapses to nothing and the push would silently pass.
	sentinel := filepath.Join(t.TempDir(), "PWNED")
	binDir := filepath.Join(repo, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir planted bin: %v", err)
	}
	planted := "#!/bin/sh\necho x >> " + sentinel + "\ncase \"$*\" in *cat-file*) exit 1 ;; esac\nexit 0\n"
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(planted), 0o755); err != nil { // #nosec G306 -- test fixture must be executable.
		t.Fatalf("write planted git: %v", err)
	}
	// Hostile PATH: the repo-internal absolute dir first, "." for completeness.
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+"."+string(os.PathListSeparator)+os.Getenv("PATH"))

	rc, out := runGateT(t, repo, pushLine("refs/heads/main", tip, base))
	if _, err := os.Stat(sentinel); err == nil {
		t.Fatalf("planted repo-internal git WAS EXECUTED (sentinel exists) — the gate trusted repo-tree code:\n%s", out)
	}
	// The verdict must match the real git's: the tip is unverified → refusal.
	if rc != 1 {
		t.Fatalf("gate verdict must match the real git's (refuse the unverified commit): got exit %d\n%s", rc, out)
	}
	if !strings.Contains(out, "ao verify") {
		t.Fatalf("refusal must name `ao verify` as the fix:\n%s", out)
	}
}

func TestPrePush_RefusesCommitWithNoVerdict(t *testing.T) {
	repo := gitInitRepoT(t)
	base := commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	tip := commitFileT(t, repo, "app.go", "package main\n", "feat: a change (age-x1)")

	code, out := runGateT(t, repo, pushLine("refs/heads/main", tip, base))
	if code != 1 {
		t.Fatalf("expected refuse (exit 1), got %d\n%s", code, out)
	}
	if !strings.Contains(out, "PUSH REFUSED") {
		t.Fatalf("output should announce refusal:\n%s", out)
	}
	if !strings.Contains(out, "ao verify") {
		t.Fatalf("refusal must name `ao verify` as the fix:\n%s", out)
	}
	if !strings.Contains(out, tip[:12]) {
		t.Fatalf("refusal should name the offending commit %s:\n%s", tip[:12], out)
	}
}

func TestPrePush_AllowsCommitWithConfirmedVerdict(t *testing.T) {
	repo := gitInitRepoT(t)
	base := commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	code := commitFileT(t, repo, "app.go", "package main\n", "feat: a change (age-x2)")
	bind := bindVerdictT(t, repo, "age-x2", code)

	// Push the range base..bind = [code, bind]: code carries a verdict edge,
	// bind is a #trivial provenance-only commit.
	rc, out := runGateT(t, repo, pushLine("refs/heads/main", bind, base))
	if rc != 0 {
		t.Fatalf("expected allow (exit 0), got %d\n%s", rc, out)
	}
}

func TestPrePush_AllowsTrivialProvenanceOnlyCommit(t *testing.T) {
	repo := gitInitRepoT(t)
	base := commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	// A provenance-only commit tagged #trivial — no verdict required.
	tip := commitFileT(t, repo, "docs/provenance/notes.txt", "n\n", "chore(provenance): note #trivial")

	rc, out := runGateT(t, repo, pushLine("refs/heads/main", tip, base))
	if rc != 0 {
		t.Fatalf("expected allow for #trivial provenance-only commit, got %d\n%s", rc, out)
	}
}

func TestPrePush_RefusesTrivialTagOnNonProvenancePath(t *testing.T) {
	repo := gitInitRepoT(t)
	base := commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	// #trivial tag but the diff touches code — the waiver must be REFUSED.
	tip := commitFileT(t, repo, "app.go", "package main\n", "feat: sneaky code #trivial")

	rc, out := runGateT(t, repo, pushLine("refs/heads/main", tip, base))
	if rc != 1 {
		t.Fatalf("expected refuse for #trivial tag on a code path, got %d\n%s", rc, out)
	}
	if !strings.Contains(out, "ao verify") {
		t.Fatalf("refusal must name `ao verify`:\n%s", out)
	}
}

// Parity with TestDoneProvenanceOnly_LeadingSpacePathNotWaived: the pre-push
// #trivial waiver must apply the SAME allowlist discipline as the done path — a
// path literally named " docs/provenance/ledger.jsonl" (LEADING SPACE) is NOT
// under docs/provenance/ and must NOT be waived. Trimming it into the allowlist
// was a fail-open that let an unverified push through.
func TestPrePush_LeadingSpacePathNotWaived(t *testing.T) {
	repo := gitInitRepoT(t)
	base := commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	// A #trivial commit whose only changed file has a LEADING SPACE in its path.
	if err := os.MkdirAll(filepath.Join(repo, " docs", "provenance"), 0o755); err != nil {
		t.Fatalf("mkdir leading-space dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, " docs", "provenance", "ledger.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write leading-space file: %v", err)
	}
	runGitT(t, repo, "add", "--", " docs")
	runGitT(t, repo, "commit", "-q", "-m", "chore(prov): sneaky #trivial")
	tip := runGitT(t, repo, "rev-parse", "HEAD")

	rc, out := runGateT(t, repo, pushLine("refs/heads/main", tip, base))
	if rc != 1 {
		t.Fatalf("a #trivial commit touching a leading-space path must REFUSE (not waive): got %d\n%s", rc, out)
	}
	if !strings.Contains(out, "ao verify") {
		t.Fatalf("refusal must name `ao verify`:\n%s", out)
	}
}

// appendLedgerEdgeT appends one valid, chained verdict edge to the repo's ledger
// via the PRODUCTION writer (Store.Append) — fixture fidelity — without committing.
func appendLedgerEdgeT(t *testing.T, repo, bead, toSHA string) {
	t.Helper()
	ledger := filepath.Join(repo, provenancegraph.LedgerRelativePath)
	if _, err := provenancegraph.NewStore(ledger).Append(provenancegraph.Edge{
		FromID: bead + "@" + toSHA[:7], FromType: "verdict",
		ToID: toSHA, ToType: "commit", Relation: "wasDerivedFrom",
		EvidenceRef: "pawl-verdict " + bead + " disposition=CONFIRMED", TrustTier: "inferred",
		TS: "2026-07-02T00:00:00Z",
	}); err != nil {
		t.Fatalf("append ledger edge: %v", err)
	}
}

// The #trivial waiver must protect the PROOF LEDGER itself: a provenance-only
// commit may only APPEND to docs/provenance/ledger.jsonl (existing rows are
// immutable). Deleting it is refused.
func TestPrePush_TrivialCannotDeleteLedger(t *testing.T) {
	repo := gitInitRepoT(t)
	init := commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	appendLedgerEdgeT(t, repo, "age-seed", init)
	runGitT(t, repo, "add", provenancegraph.LedgerRelativePath)
	runGitT(t, repo, "commit", "-q", "-m", "chore(prov): seed ledger #trivial")
	base := runGitT(t, repo, "rev-parse", "HEAD")

	// #trivial commit that DELETES the proof ledger.
	runGitT(t, repo, "rm", "-q", provenancegraph.LedgerRelativePath)
	runGitT(t, repo, "commit", "-q", "-m", "chore(prov): erase ledger #trivial")
	tip := runGitT(t, repo, "rev-parse", "HEAD")

	rc, out := runGateT(t, repo, pushLine("refs/heads/main", tip, base))
	if rc != 1 {
		t.Fatalf("a #trivial commit deleting the proof ledger must REFUSE: got %d\n%s", rc, out)
	}
	if !strings.Contains(out, "may not delete the proof ledger") {
		t.Fatalf("refusal must name the delete-ledger reason:\n%s", out)
	}
}

// A non-append REWRITE (existing base rows not preserved verbatim) is refused —
// even when the rewritten ledger is itself chain-valid (so the chain-verify
// alone would pass).
func TestPrePush_TrivialCannotRewriteLedger(t *testing.T) {
	repo := gitInitRepoT(t)
	init := commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	appendLedgerEdgeT(t, repo, "age-a", init)
	appendLedgerEdgeT(t, repo, "age-b", init)
	runGitT(t, repo, "add", provenancegraph.LedgerRelativePath)
	runGitT(t, repo, "commit", "-q", "-m", "chore(prov): seed 2 edges #trivial")
	base := runGitT(t, repo, "rev-parse", "HEAD")

	// Rewrite: replace the ledger with a DIFFERENT valid chain (genesis of a new
	// edge) — chain-valid, but base's rows are no longer a byte-prefix.
	if err := os.Remove(filepath.Join(repo, provenancegraph.LedgerRelativePath)); err != nil {
		t.Fatalf("remove ledger: %v", err)
	}
	appendLedgerEdgeT(t, repo, "age-c", init)
	runGitT(t, repo, "add", provenancegraph.LedgerRelativePath)
	runGitT(t, repo, "commit", "-q", "-m", "chore(prov): rewrite ledger #trivial")
	tip := runGitT(t, repo, "rev-parse", "HEAD")

	rc, out := runGateT(t, repo, pushLine("refs/heads/main", tip, base))
	if rc != 1 {
		t.Fatalf("a #trivial commit rewriting the proof ledger must REFUSE: got %d\n%s", rc, out)
	}
	if !strings.Contains(out, "only appends") {
		t.Fatalf("refusal must name the append-only reason:\n%s", out)
	}
}

// A pure APPEND (base rows preserved verbatim, chain-valid) still waives.
func TestPrePush_TrivialLedgerAppendStillWaives(t *testing.T) {
	repo := gitInitRepoT(t)
	init := commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	appendLedgerEdgeT(t, repo, "age-a", init)
	runGitT(t, repo, "add", provenancegraph.LedgerRelativePath)
	runGitT(t, repo, "commit", "-q", "-m", "chore(prov): seed #trivial")
	base := runGitT(t, repo, "rev-parse", "HEAD")

	appendLedgerEdgeT(t, repo, "age-b", init) // Store.Append preserves e1, appends e2
	runGitT(t, repo, "add", provenancegraph.LedgerRelativePath)
	runGitT(t, repo, "commit", "-q", "-m", "chore(prov): append verdict #trivial")
	tip := runGitT(t, repo, "rev-parse", "HEAD")

	rc, out := runGateT(t, repo, pushLine("refs/heads/main", tip, base))
	if rc != 0 {
		t.Fatalf("a pure append to the proof ledger must still waive: got %d\n%s", rc, out)
	}
}

// The first-ever ledger (base has none) is an append over empty — waives.
func TestPrePush_TrivialFirstLedgerWaives(t *testing.T) {
	repo := gitInitRepoT(t)
	init := commitFileT(t, repo, "README.md", "hi\n", "chore: init") // base, no ledger

	appendLedgerEdgeT(t, repo, "age-a", init) // create the ledger genesis
	runGitT(t, repo, "add", provenancegraph.LedgerRelativePath)
	runGitT(t, repo, "commit", "-q", "-m", "chore(prov): first verdict #trivial")
	tip := runGitT(t, repo, "rev-parse", "HEAD")

	rc, out := runGateT(t, repo, pushLine("refs/heads/main", tip, init))
	if rc != 0 {
		t.Fatalf("the first-ever ledger (append over empty) must waive: got %d\n%s", rc, out)
	}
}

func TestPrePush_RefusesTrivialMidSubjectMention(t *testing.T) {
	repo := gitInitRepoT(t)
	base := commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	// #trivial mentioned mid-subject (not a trailing tag) must NOT waive — even
	// on a provenance-only path (this was a real fail-open class).
	tip := commitFileT(t, repo, "docs/provenance/x.txt", "x\n", "fix: stop #trivial from bypassing the gate")

	rc, _ := runGateT(t, repo, pushLine("refs/heads/main", tip, base))
	if rc != 1 {
		t.Fatalf("a mid-subject #trivial mention must not waive; expected refuse, got %d", rc)
	}
}

func TestPrePush_RefusesVerdictBoundToWrongCommit(t *testing.T) {
	repo := gitInitRepoT(t)
	base := commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	code := commitFileT(t, repo, "app.go", "v1\n", "feat: change (age-x3)")
	// Bind a verdict to `code`, then add a NEW code commit with no verdict.
	bindVerdictT(t, repo, "age-x3", code)
	tip := commitFileT(t, repo, "app.go", "v2\n", "feat: second change (age-x4)")

	rc, out := runGateT(t, repo, pushLine("refs/heads/main", tip, base))
	if rc != 1 {
		t.Fatalf("a verdict bound to a DIFFERENT commit must not authorize; got %d\n%s", rc, out)
	}
	if !strings.Contains(out, tip[:12]) {
		t.Fatalf("should name the unverified commit %s:\n%s", tip[:12], out)
	}
}

func TestPrePush_RefusesTamperedLedger(t *testing.T) {
	repo := gitInitRepoT(t)
	base := commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	code := commitFileT(t, repo, "app.go", "v1\n", "feat: change (age-x5)")
	bindVerdictT(t, repo, "age-x5", code)

	// Tamper: alter a payload field in place without recomputing the hash.
	ledger := filepath.Join(repo, provenancegraph.LedgerRelativePath)
	data, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	tampered := strings.Replace(string(data), "disposition=CONFIRMED", "disposition=TAMPERED", 1)
	if tampered == string(data) {
		t.Fatalf("tamper precondition not met")
	}
	if err := os.WriteFile(ledger, []byte(tampered), 0o644); err != nil {
		t.Fatalf("write ledger: %v", err)
	}
	runGitT(t, repo, "add", provenancegraph.LedgerRelativePath)
	runGitT(t, repo, "commit", "-q", "-m", "chore: tamper #trivial")
	tip := runGitT(t, repo, "rev-parse", "HEAD")

	rc, out := runGateT(t, repo, pushLine("refs/heads/main", tip, base))
	if rc != 1 {
		t.Fatalf("a tampered ledger must refuse the push, got %d\n%s", rc, out)
	}
	if !strings.Contains(out, "BROKEN") && !strings.Contains(out, "TAMPERED") {
		t.Fatalf("refusal should cite the broken chain:\n%s", out)
	}
}

func TestPrePush_SkipsNonTrunkRef(t *testing.T) {
	repo := gitInitRepoT(t)
	base := commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	tip := commitFileT(t, repo, "app.go", "x\n", "feat: unverified (age-x6)")

	// Pushing to a feature branch (not main/master) is not gated.
	rc, out := runGateT(t, repo, pushLine("refs/heads/feature", tip, base))
	if rc != 0 {
		t.Fatalf("non-trunk push must not be gated, got %d\n%s", rc, out)
	}
}

func TestPrePush_SkipsBranchDelete(t *testing.T) {
	repo := gitInitRepoT(t)
	commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	// local_sha all-zeros => a branch delete, nothing to verify.
	rc, out := runGateT(t, repo, pushLine("refs/heads/main", zeroSHA, zeroSHA))
	if rc != 0 {
		t.Fatalf("branch delete must not be gated, got %d\n%s", rc, out)
	}
}

// The round-3 refuter's exact repro: a CREATION push (remote_sha all-zeros —
// the first push of main, or a re-created branch) must check the WHOLE new
// history, not just the tip. Checking only the tip let an unverified code
// commit ride in under a single provenance-only #trivial tip commit.
func TestPrePush_NewMainChecksWholeHistory(t *testing.T) {
	repo := gitInitRepoT(t)
	code := commitFileT(t, repo, "app.go", "package main\n", "feat: change (age-n1)")
	tip := commitFileT(t, repo, "docs/provenance/notes.txt", "n\n", "chore(provenance): note #trivial")

	rc, out := runGateT(t, repo, pushLine("refs/heads/main", tip, zeroSHA))
	if rc != 1 {
		t.Fatalf("a creation push must check the whole new history, not just the #trivial tip: got exit %d\n%s", rc, out)
	}
	if !strings.Contains(out, code[:12]) {
		t.Fatalf("refusal should name the unverified code commit %s:\n%s", code[:12], out)
	}
	if !strings.Contains(out, "ao verify") {
		t.Fatalf("refusal must name `ao verify` as the fix:\n%s", out)
	}
}

// Positive twin: the same creation-push shape, but the code commit carries a
// committed CONFIRMED verdict edge in the tip's ledger — the whole history is
// proven, so the push proceeds.
func TestPrePush_NewMainAllowsVerifiedHistory(t *testing.T) {
	repo := gitInitRepoT(t)
	code := commitFileT(t, repo, "app.go", "package main\n", "feat: change (age-n2)")
	tip := bindVerdictT(t, repo, "age-n2", code) // #trivial bind carrying the ledger

	rc, out := runGateT(t, repo, pushLine("refs/heads/main", tip, zeroSHA))
	if rc != 0 {
		t.Fatalf("a creation push whose history is fully proven must proceed: got %d\n%s", rc, out)
	}
}

// A creation push excludes commits already reachable from a remote-tracking
// ref: only history genuinely NEW to the remotes needs proof (re-creating main
// from already-pushed commits is not a fresh landing of those commits) — and
// the exclusion must not swallow new unverified work past that boundary.
func TestPrePush_NewMainExcludesRemoteTrackedHistory(t *testing.T) {
	repo := gitInitRepoT(t)
	old := commitFileT(t, repo, "README.md", "hi\n", "chore: unverified old history")
	// Record `old` as already on a remote (the client-side remote-tracking state
	// a pre-push hook can consult).
	runGitT(t, repo, "update-ref", "refs/remotes/origin/main", old)
	// New, proven work on top: code commit + committed bind.
	code := commitFileT(t, repo, "app.go", "package main\n", "feat: change (age-n3)")
	tip := bindVerdictT(t, repo, "age-n3", code)

	rc, out := runGateT(t, repo, pushLine("refs/heads/main", tip, zeroSHA))
	if rc != 0 {
		t.Fatalf("remote-tracked history must be excluded from a creation push's checked range: got %d\n%s", rc, out)
	}
	// New unverified work past the remote-tracked boundary still refuses, and
	// the refusal names only the NEW commit, not the already-tracked history.
	unverified := commitFileT(t, repo, "other.go", "package main\n", "feat: unverified (age-n4)")
	rc, out = runGateT(t, repo, pushLine("refs/heads/main", unverified, zeroSHA))
	if rc != 1 {
		t.Fatalf("new unverified work past the remote-tracked boundary must refuse: got %d\n%s", rc, out)
	}
	if !strings.Contains(out, unverified[:12]) {
		t.Fatalf("refusal should name the new unverified commit:\n%s", out)
	}
	if strings.Contains(out, old[:12]) {
		t.Fatalf("already-remote-tracked commit %s must NOT be re-checked:\n%s", old[:12], out)
	}
}

// The DELETED tip-only fallback (round-7): a non-zero base absent from this
// clone (shallow / GC'd) previously narrowed the checked range to the tip — a
// fail-open, since unverified code behind a provenance-only #trivial tip rode
// in. With NO remote-tracking ref to derive "new" against, the gate cannot tell
// which commits are new, so it must REFUSE (fail-closed), never tip-only.
func TestPrePush_UnknownBaseRefusesFailClosedWithoutRemotes(t *testing.T) {
	repo := gitInitRepoT(t)
	code := commitFileT(t, repo, "app.go", "package main\n", "feat: unverified (age-n5)")
	tip := commitFileT(t, repo, "docs/provenance/notes.txt", "n\n", "chore(provenance): note #trivial")

	fakeBase := strings.Repeat("deadbeef", 5) // 40 hex chars, resolves to nothing
	rc, out := runGateT(t, repo, pushLine("refs/heads/main", tip, fakeBase))
	if rc != 1 {
		t.Fatalf("unknown non-zero base with no remote-tracking ref must REFUSE (fail-closed), not narrow to the tip: got %d\n%s", rc, out)
	}
	if !strings.Contains(out, "cannot determine which commits are new") {
		t.Fatalf("refusal must name the indeterminate base + the fix:\n%s", out)
	}
	// It must NOT have silently narrowed to the #trivial tip and passed; the
	// unverified code commit is precisely what tip-only let slip.
	_ = code
}

// When a remote-tracking ref DOES exist, an unknown-base update derives the true
// new range with `--not --remotes` (the same machinery the creation case uses):
// genuinely-landed history is excluded, unverified new work is still checked.
func TestPrePush_UnknownBaseChecksDerivedRangeWithRemotes(t *testing.T) {
	repo := gitInitRepoT(t)
	old := commitFileT(t, repo, "README.md", "hi\n", "chore: unverified landed history")
	runGitT(t, repo, "update-ref", "refs/remotes/origin/main", old) // already on a remote
	code := commitFileT(t, repo, "app.go", "package main\n", "feat: unverified (age-n6)")
	tip := commitFileT(t, repo, "docs/provenance/notes.txt", "n\n", "chore(provenance): note #trivial")

	fakeBase := strings.Repeat("deadbeef", 5) // non-zero, unresolvable locally
	rc, out := runGateT(t, repo, pushLine("refs/heads/main", tip, fakeBase))
	if rc != 1 {
		t.Fatalf("new unverified work past the remote-tracked boundary must refuse: got %d\n%s", rc, out)
	}
	if !strings.Contains(out, code[:12]) {
		t.Fatalf("refusal should name the new unverified commit:\n%s", out)
	}
	if strings.Contains(out, old[:12]) {
		t.Fatalf("already-remote-tracked history must NOT be re-checked:\n%s", old[:12])
	}

	// Binding a committed verdict to the new code commit authorizes the push.
	bindVerdictT(t, repo, "age-n6", code)
	tip2 := runGitT(t, repo, "rev-parse", "HEAD")
	rc, out = runGateT(t, repo, pushLine("refs/heads/main", tip2, fakeBase))
	if rc != 0 {
		t.Fatalf("a fully-verified derived range must proceed: got %d\n%s", rc, out)
	}
}

// Round-10 correctness: a commit already on an UNGATED feature branch's
// remote-tracking ref is NOT "already verified" — only the GATED TRUNK
// (refs/remotes/*/<pushed-branch>) counts. Excluding via `--not --remotes`
// (all branches) let unverified code pushed to origin/feature-x ride onto main
// as verified. The exclusion must be scoped to the trunk ref.
func TestPrePush_ChecksCommitOnlyOnUngatedFeatureBranch(t *testing.T) {
	repo := gitInitRepoT(t)
	base := commitFileT(t, repo, "README.md", "hi\n", "chore: base")
	runGitT(t, repo, "update-ref", "refs/remotes/origin/main", base) // the gated trunk, at base
	// An unverified code commit that ALSO lives on an ungated feature branch.
	code := commitFileT(t, repo, "app.go", "package main\n", "feat: unverified (age-f1)")
	runGitT(t, repo, "update-ref", "refs/remotes/origin/feature-x", code)
	tip := commitFileT(t, repo, "docs/provenance/notes.txt", "n\n", "chore(provenance): note #trivial")

	// Unknown base → derived range. `code` is on origin/feature-x but NOT on
	// origin/main → it is NEW to the trunk and must be CHECKED, not excluded.
	fakeBase := strings.Repeat("deadbeef", 5)
	rc, out := runGateT(t, repo, pushLine("refs/heads/main", tip, fakeBase))
	if rc != 1 {
		t.Fatalf("a commit only on an ungated feature branch must be CHECKED (trunk-scoped exclusion): got %d\n%s", rc, out)
	}
	if !strings.Contains(out, code[:12]) {
		t.Fatalf("refusal should name the feature-branch-only unverified commit %s:\n%s", code[:12], out)
	}
	// A commit genuinely on the trunk (base) is still excluded.
	if strings.Contains(out, base[:12]) {
		t.Fatalf("a commit on the gated trunk (origin/main) must NOT be re-checked:\n%s", out)
	}
}

// A commit already on ANOTHER remote's main (a backup remote) is NOT gated for the
// TARGET remote — excluding every refs/remotes/*/main let an unverified commit that
// happened to sit on backup/main ride onto origin/main with no verdict. The
// exclusion must scope to the TARGET remote git forwards to the hook (age-rk3r.6
// cross-family refuter).
func TestPrePush_NewMainDoesNotExcludeOtherRemoteMain(t *testing.T) {
	repo := gitInitRepoT(t)
	base := commitFileT(t, repo, "README.md", "hi\n", "chore: base")
	runGitT(t, repo, "update-ref", "refs/remotes/origin/main", base) // target trunk, at base
	// An unverified code commit that lives on ANOTHER remote's main (backup).
	code := commitFileT(t, repo, "app.go", "package main\n", "feat: unverified (age-o1)")
	tip := commitFileT(t, repo, "docs/provenance/notes.txt", "n\n", "chore(provenance): note #trivial")
	runGitT(t, repo, "update-ref", "refs/remotes/backup/main", code)

	// Push to origin (target). Unknown base → derived range scoped to origin/main.
	// `code` is on backup/main but NOT origin/main → NEW to the target trunk → must
	// be CHECKED, not excluded. Old code globbed refs/remotes/*/main and excluded it.
	fakeBase := strings.Repeat("deadbeef", 5)
	rc, out := runGateRemoteT(t, repo, "origin", pushLine("refs/heads/main", tip, fakeBase))
	if rc != 1 {
		t.Fatalf("a commit only on ANOTHER remote's main must be CHECKED for the target trunk: got %d\n%s", rc, out)
	}
	if !strings.Contains(out, code[:12]) {
		t.Fatalf("refusal should name the backup-only unverified commit %s:\n%s", code[:12], out)
	}
	// The genuinely-gated trunk commit (base, on origin/main) is still excluded.
	if strings.Contains(out, base[:12]) {
		t.Fatalf("a commit on the target's gated trunk (origin/main) must NOT be re-checked:\n%s", out)
	}
}

// The mirror-remote fallback (any-remote trunk when the target has none) is for a
// CREATION push only. An UPDATE push whose non-zero base is absent locally with no
// target trunk-tracking ref must NOT fall back to globbing — that would exclude an
// unverified commit sitting only on another remote's main from an ESTABLISHED trunk
// push. It stays strict and refuses indeterminate (age-rk3r.6 cross-family refuter).
func TestPrePush_UnknownBaseNoTargetTrackingDoesNotExcludeOtherRemoteMain(t *testing.T) {
	repo := gitInitRepoT(t)
	_ = commitFileT(t, repo, "README.md", "hi\n", "chore: base")
	code := commitFileT(t, repo, "app.go", "package main\n", "feat: unverified (age-u2)")
	tip := commitFileT(t, repo, "docs/provenance/notes.txt", "n\n", "chore(provenance): note #trivial")
	// The unverified commit lives ONLY on another remote's main; the TARGET remote
	// (origin) has NO local trunk-tracking ref.
	runGitT(t, repo, "update-ref", "refs/remotes/backup/main", code)

	// Update push (NON-ZERO base) whose base is absent locally, target origin.
	fakeBase := strings.Repeat("deadbeef", 5)
	rc, out := runGateRemoteT(t, repo, "origin", pushLine("refs/heads/main", tip, fakeBase))
	if rc != 1 {
		t.Fatalf("an absent non-zero base with no target trunk must refuse (no mirror fallback for updates): got %d\n%s", rc, out)
	}
	// It refuses indeterminate — never silently excludes backup/main and allows.
	if strings.Contains(out, code[:12]) && !strings.Contains(out, "cannot determine") {
		t.Logf("note: refusal names the unverified commit — acceptable; strict-refuse output:\n%s", out)
	}
}

// Parsing-discipline sweep: the shared provenance allowlist parser consumes
// NUL-separated (-z) raw paths and matches the docs/provenance/ directory
// EXACTLY — a leading-space path, a prefix-substring sibling dir, or any mixed
// non-provenance path must NOT waive.
func TestProvenanceOnlyChangedFiles_ExactAndFailClosed(t *testing.T) {
	nul := func(paths ...string) string { return strings.Join(paths, "\x00") + "\x00" }
	cases := []struct {
		name string
		out  string
		want bool
	}{
		{"single-provenance", nul("docs/provenance/ledger.jsonl"), true},
		{"multi-provenance", nul("docs/provenance/a.jsonl", "docs/provenance/b.jsonl"), true},
		{"leading-space", nul(" docs/provenance/ledger.jsonl"), false},
		{"prefix-substring-sibling-dir", nul("docs/provenance-evil/x"), false},
		{"bare-provenance-file", nul("docs/provenance"), false},
		{"mixed-one-bad", nul("docs/provenance/a.jsonl", "cli/cmd/ao/x.go"), false},
		{"empty", "", false},
		{"only-nul", "\x00", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := provenanceOnlyChangedFiles(tc.out); got != tc.want {
				t.Fatalf("provenanceOnlyChangedFiles(%q)=%v, want %v", tc.out, got, tc.want)
			}
		})
	}
}

func TestPrePush_SkipsWhenNoStdin(t *testing.T) {
	repo := gitInitRepoT(t)
	commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	rc, out := runGateT(t, repo, "")
	if rc != 0 {
		t.Fatalf("no stdin => not a push hook invocation => skip (exit 0), got %d\n%s", rc, out)
	}
	if !strings.Contains(out, "no pre-push stdin") {
		t.Fatalf("should announce the skip:\n%s", out)
	}
}

func TestPrePush_EscapeHatchBypasses(t *testing.T) {
	repo := gitInitRepoT(t)
	base := commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	tip := commitFileT(t, repo, "app.go", "x\n", "feat: unverified (age-x8)")

	t.Setenv("AGENTOPS_VERIFY_PREPUSH_SKIP", "1")
	rc, out := runGateT(t, repo, pushLine("refs/heads/main", tip, base))
	if rc != 0 {
		t.Fatalf("escape hatch must allow the push, got %d\n%s", rc, out)
	}
	if !strings.Contains(out, "BYPASSED") {
		t.Fatalf("bypass must be announced loudly:\n%s", out)
	}
}

func TestPrePush_TrivialWaiverUnit(t *testing.T) {
	repo := gitInitRepoT(t)
	commitFileT(t, repo, "README.md", "hi\n", "chore: init")

	cases := []struct {
		name   string
		file   string
		msg    string
		waived bool
	}{
		{"provenance-only-trailing-tag", "docs/provenance/a.txt", "chore(prov): a #trivial", true},
		{"provenance-only-no-marker", "docs/provenance/b.txt", "chore(prov): b", false},
		{"code-path-with-tag", "src.go", "feat: c #trivial", false},
		{"mid-subject-mention", "docs/provenance/d.txt", "fix: #trivial bypass guard", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sha := commitFileT(t, repo, tc.file, "x\n", tc.msg)
			got, err := trivialWaiver(repo, sha)
			if err != nil {
				t.Fatalf("trivialWaiver error: %v", err)
			}
			if got != tc.waived {
				t.Fatalf("trivialWaiver(%q)=%v, want %v", tc.msg, got, tc.waived)
			}
		})
	}
}

func TestPrePush_HasConfirmedVerdictEdge_MatchesShortToID(t *testing.T) {
	repo := gitInitRepoT(t)
	code := commitFileT(t, repo, "app.go", "x\n", "feat: change (age-x9)")
	ledger := filepath.Join(repo, provenancegraph.LedgerRelativePath)
	store := provenancegraph.NewStore(ledger)
	// A verdict bind may carry a SHORT head — a >=7-char prefix must still match.
	if _, err := store.Append(provenancegraph.Edge{
		FromID: "age-x9@" + code[:7], FromType: "verdict",
		ToID: code[:12], ToType: "commit", Relation: "wasDerivedFrom",
		EvidenceRef: "pawl-verdict age-x9 disposition=CONFIRMED", TrustTier: "inferred",
		TS: "2026-07-02T00:00:00Z",
	}); err != nil {
		t.Fatalf("append: %v", err)
	}
	if !hasConfirmedVerdictEdge(ledger, code) {
		t.Fatalf("short (12-char) to_id prefix should match the full sha")
	}
	// A non-CONFIRMED disposition must NOT count.
	other := commitFileT(t, repo, "app.go", "y\n", "feat: other (age-x10)")
	if _, err := store.Append(provenancegraph.Edge{
		FromID: "age-x10@" + other[:7], FromType: "verdict",
		ToID: other, ToType: "commit", Relation: "wasDerivedFrom",
		EvidenceRef: "pawl-verdict age-x10 disposition=REFUTED", TrustTier: "inferred",
		TS: "2026-07-02T00:00:01Z",
	}); err != nil {
		t.Fatalf("append: %v", err)
	}
	if hasConfirmedVerdictEdge(ledger, other) {
		t.Fatalf("a REFUTED verdict edge must not count as proof")
	}
}

// Parsing-discipline sweep: the verdict-edge matcher must be EXACT — the correct
// relation AND an exact-token disposition == "CONFIRMED", never a substring. A
// near-miss (wrong relation, an adjacent disposition token, wrong node type,
// non-hex to_id) must be REJECTED.
func TestPrePush_ConfirmedVerdictEdgeIn_ExactMatchOnly(t *testing.T) {
	const sha = "abcdef1234567890abcdef1234567890abcdef12"
	base := func() provenancegraph.Edge {
		return provenancegraph.Edge{
			FromID: "age-x@" + sha[:7], FromType: "verdict",
			ToID: sha, ToType: "commit", Relation: "wasDerivedFrom",
			EvidenceRef: "pawl-verdict age-x disposition=CONFIRMED",
		}
	}
	cases := []struct {
		name string
		mut  func(e *provenancegraph.Edge)
		want bool
	}{
		{"exact-confirmed", func(e *provenancegraph.Edge) {}, true},
		{"exact-confirmed-short-toid", func(e *provenancegraph.Edge) { e.ToID = sha[:12] }, true},
		{"wrong-relation-wasAttributedTo", func(e *provenancegraph.Edge) { e.Relation = "wasAttributedTo" }, false},
		{"disposition-superstring-CONFIRMEDLY", func(e *provenancegraph.Edge) {
			e.EvidenceRef = "pawl-verdict age-x disposition=CONFIRMEDLY"
		}, false},
		{"disposition-refuted", func(e *provenancegraph.Edge) {
			e.EvidenceRef = "pawl-verdict age-x disposition=REFUTED"
		}, false},
		{"disposition-absent", func(e *provenancegraph.Edge) { e.EvidenceRef = "pawl-verdict age-x" }, false},
		{"disposition-embedded-substring", func(e *provenancegraph.Edge) {
			e.EvidenceRef = "note: xdisposition=CONFIRMED (not a real field)"
		}, false},
		{"wrong-from-type", func(e *provenancegraph.Edge) { e.FromType = "artifact" }, false},
		{"wrong-to-type", func(e *provenancegraph.Edge) { e.ToType = "artifact" }, false},
		{"non-hex-toid", func(e *provenancegraph.Edge) { e.ToID = "not-a-sha-xyz" }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := base()
			tc.mut(&e)
			if got := confirmedVerdictEdgeIn([]provenancegraph.Edge{e}, sha); got != tc.want {
				t.Fatalf("confirmedVerdictEdgeIn(%s)=%v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

// ============================================================================
// REBOUND authorization (age-rk3r.18) — the portable pre-push gate honors a
// committed REBOUND verdict edge ONLY after Go-side lineage + proof re-
// validation: a committed CONFIRMED-reviewed commit that is BYTE-EQUIVALENT to
// the tip must exist (same git patch-id --stable AND byte-exact content
// signature, re-derived via trusted git). A bare disposition=REBOUND is NEVER
// accepted; the edge's stored patch_id_proof is never consulted. These tests
// mirror age-rk3r.9's escape corpus (whitespace / mode / binary / no-newline) at
// the Go gate, RED-first where behavior changed.
// ============================================================================

// appendReboundEdgeT appends a REBOUND verdict edge bound to toSHA via the
// PRODUCTION writer (fixture fidelity), WITHOUT committing. This is the exact
// committed shape `ao provenance emit-verdict` writes for a rebind: only
// disposition=REBOUND + to_id ride the ledger edge — the lineage fields live in
// the (uncommitted) verdict file, which the gate deliberately cannot read.
func appendReboundEdgeT(t *testing.T, repo, bead, toSHA string) {
	t.Helper()
	ledger := filepath.Join(repo, provenancegraph.LedgerRelativePath)
	if _, err := provenancegraph.NewStore(ledger).Append(provenancegraph.Edge{
		FromID: bead + "@" + toSHA[:7], FromType: "verdict",
		ToID: toSHA, ToType: "commit", Relation: "wasDerivedFrom",
		EvidenceRef: "pawl-verdict " + bead + " disposition=REBOUND", TrustTier: "inferred",
		TS: "2026-07-02T00:00:00Z",
	}); err != nil {
		t.Fatalf("append REBOUND edge: %v", err)
	}
}

// reboundFixture builds the canonical REBOUND scenario in a fresh repo and
// returns the pieces the tests assert over. The SHARED base-of-`file` commit
// lives BEFORE the branch point so BOTH R and C build on the identical tree —
// the only new code commit on the rebound line is tip C itself (the commit under
// test). It creates:
//   - README -> shared base-of-`file` -> reviewed R (applies `change`) on main,
//     with a CONFIRMED verdict edge bound to R;
//   - a divergent branch `rebound` off the shared base-of-`file` that applies the
//     SAME `change` (byte-identical diff) as tip C — a distinct sha with an
//     identical patch-id + content signature (a true no-op rebase);
//   - a REBOUND verdict edge bound to C, appended after the CONFIRMED edge;
//   - the ledger committed as a #trivial bind on the `rebound` branch, so C's
//     pushed tip tree carries BOTH edges.
//
// Returns (repo, branchBase, reviewedR, tipC, boundLedgerCommit). The push under
// test is branchBase..boundLedgerCommit, so the pushed range is exactly [C, bind]
// — both proof-bearing (C via REBOUND, bind via #trivial). file/change let a test
// vary the diff category.
func reboundFixture(t *testing.T, file, baseContent, changeContent string) (repo, branchBase, reviewedR, tipC, bound string) {
	t.Helper()
	repo = gitInitRepoT(t)
	commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	// The SHARED base-of-file commit — the branch point for both R and C.
	branchBase = commitFileT(t, repo, file, baseContent, "chore: base "+file)
	// The reviewed commit R on main.
	reviewedR = commitFileT(t, repo, file, changeContent, "feat: the change (reviewed)")

	// Bind a CONFIRMED verdict to R (in the working-tree ledger, not yet committed).
	appendLedgerEdgeT(t, repo, "age-rev", reviewedR)

	// Diverge from the SHARED base and apply the IDENTICAL change -> tip C with the
	// same patch-id + content signature but a distinct sha (a no-op rebase).
	runGitT(t, repo, "checkout", "-q", "-b", "rebound", branchBase)
	tipC = commitFileT(t, repo, file, changeContent, "feat: the change (rebound)")

	// Append the REBOUND edge for C AFTER the CONFIRMED edge, then commit the whole
	// ledger as the #trivial bind so C's tip tree carries both edges.
	appendReboundEdgeT(t, repo, "age-reb", tipC)
	runGitT(t, repo, "add", provenancegraph.LedgerRelativePath)
	runGitT(t, repo, "commit", "-q", "-m", "chore(provenance): bind REBOUND + lineage #trivial")
	bound = runGitT(t, repo, "rev-parse", "HEAD")
	return repo, branchBase, reviewedR, tipC, bound
}

// A VALID REBOUND (lineage CONFIRMED at a byte-equivalent reviewed commit, tip
// byte-equivalent) authorizes the push. RED-first: BEFORE age-rk3r.18 the gate
// authorized ONLY CONFIRMED, so a REBOUND tip was REFUSED — this proves the
// wiring changed behavior.
func TestPrePush_REBOUND_ValidLineageAuthorizes(t *testing.T) {
	repo, base, reviewedR, tipC, bound := reboundFixture(t, "app.go", "package main\n", "package main\n\nfunc F() {}\n")
	if tipC == reviewedR {
		t.Fatal("rebound tip must be a distinct sha from the reviewed commit")
	}
	rc, out := runGateT(t, repo, pushLine("refs/heads/main", bound, base))
	if rc != 0 {
		t.Fatalf("a VALID REBOUND (byte-equivalent to a CONFIRMED-reviewed commit) must authorize: got %d\n%s", rc, out)
	}
}

// FORGE (a): the REBOUND's lineage is NOT a CONFIRMED — the only reviewed commit
// carries a REFUTED (or absent) verdict. With no CONFIRMED-reviewed commit
// byte-equivalent to the tip, the REBOUND is REFUSED (fail-closed).
func TestPrePush_REBOUND_LineageNotConfirmed_Refused(t *testing.T) {
	repo := gitInitRepoT(t)
	commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	branchBase := commitFileT(t, repo, "app.go", "package main\n", "chore: base app.go")
	// Reviewed commit R with a NON-CONFIRMED (REFUTED) verdict edge.
	reviewedR := commitFileT(t, repo, "app.go", "package main\n\nfunc F() {}\n", "feat: the change (reviewed)")
	ledger := filepath.Join(repo, provenancegraph.LedgerRelativePath)
	if _, err := provenancegraph.NewStore(ledger).Append(provenancegraph.Edge{
		FromID: "age-rev@" + reviewedR[:7], FromType: "verdict",
		ToID: reviewedR, ToType: "commit", Relation: "wasDerivedFrom",
		EvidenceRef: "pawl-verdict age-rev disposition=REFUTED", TrustTier: "inferred",
		TS: "2026-07-02T00:00:00Z",
	}); err != nil {
		t.Fatalf("append REFUTED edge: %v", err)
	}
	// Rebound tip C (byte-equivalent to R) off the SHARED base, with a REBOUND edge.
	runGitT(t, repo, "checkout", "-q", "-b", "rebound", branchBase)
	tipC := commitFileT(t, repo, "app.go", "package main\n\nfunc F() {}\n", "feat: the change (rebound)")
	appendReboundEdgeT(t, repo, "age-reb", tipC)
	runGitT(t, repo, "add", provenancegraph.LedgerRelativePath)
	runGitT(t, repo, "commit", "-q", "-m", "chore(provenance): bind REBOUND (thin lineage) #trivial")
	bound := runGitT(t, repo, "rev-parse", "HEAD")

	rc, out := runGateT(t, repo, pushLine("refs/heads/main", bound, branchBase))
	if rc != 1 {
		t.Fatalf("a REBOUND whose only lineage is REFUTED (not CONFIRMED) must REFUSE: got %d\n%s", rc, out)
	}
	// The refusal must name the REBOUND TIP itself (proving the REBOUND logic
	// rejected it, not a stray unverified sibling commit).
	if !strings.Contains(out, tipC[:12]) {
		t.Fatalf("refusal must name the unauthorized rebound tip %s:\n%s", tipC[:12], out)
	}
}

// FORGE (b): a lied proof — the tip's diff is NOT byte-equivalent to the reviewed
// commit (a genuinely different change), yet a REBOUND edge is bound to it. Even
// though a CONFIRMED-reviewed commit exists in the ledger, it is NOT equivalent
// to the tip, so the REBOUND is REFUSED. (The stored patch_id_proof is never
// consulted; equivalence is re-derived from git.)
func TestPrePush_REBOUND_TipNotEquivalent_Refused(t *testing.T) {
	repo := gitInitRepoT(t)
	commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	branchBase := commitFileT(t, repo, "app.go", "package main\n", "chore: base app.go")
	// Reviewed commit R, CONFIRMED (func F).
	reviewedR := commitFileT(t, repo, "app.go", "package main\n\nfunc F() {}\n", "feat: change F (reviewed)")
	appendLedgerEdgeT(t, repo, "age-rev", reviewedR)
	// Rebound tip C off the SHARED base applies a DIFFERENT change (func G, not F)
	// — NOT byte-equivalent to the CONFIRMED commit.
	runGitT(t, repo, "checkout", "-q", "-b", "rebound", branchBase)
	tipC := commitFileT(t, repo, "app.go", "package main\n\nfunc G() {}\n", "feat: change G (forged rebound)")
	appendReboundEdgeT(t, repo, "age-reb", tipC)
	runGitT(t, repo, "add", provenancegraph.LedgerRelativePath)
	runGitT(t, repo, "commit", "-q", "-m", "chore(provenance): bind forged REBOUND #trivial")
	bound := runGitT(t, repo, "rev-parse", "HEAD")

	rc, out := runGateT(t, repo, pushLine("refs/heads/main", bound, branchBase))
	if rc != 1 {
		t.Fatalf("a REBOUND whose tip is NOT byte-equivalent to any CONFIRMED commit must REFUSE: got %d\n%s", rc, out)
	}
	if !strings.Contains(out, tipC[:12]) {
		t.Fatalf("refusal must name the unauthorized rebound tip %s:\n%s", tipC[:12], out)
	}
}

// FORGE (c1) WHITESPACE: the tip differs from the reviewed commit ONLY by leading
// whitespace on a content line (same patch-id, different diff bytes). The byte-
// exact content signature catches it → REFUSED. (age-rk3r.9 DEFECT 1 at the Go gate.)
func TestPrePush_REBOUND_WhitespaceOnlyDrift_Refused(t *testing.T) {
	repo := gitInitRepoT(t)
	commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	branchBase := commitFileT(t, repo, "a.py", "def g():\n    return 1\n", "chore: base py")
	// Reviewed: add an UNINDENTED statement.
	reviewedR := commitFileT(t, repo, "a.py", "def g():\n    return 1\nx = 2\n", "feat: add x (reviewed)")
	appendLedgerEdgeT(t, repo, "age-rev", reviewedR)
	// Rebound tip off the SHARED base: add the SAME statement INDENTED — same
	// patch-id, different bytes.
	runGitT(t, repo, "checkout", "-q", "-b", "rebound", branchBase)
	tipC := commitFileT(t, repo, "a.py", "def g():\n    return 1\n    x = 2\n", "feat: add x indented (rebound)")

	// Precondition: the two commits DO share a patch-id (the trap the signature must catch).
	git := trustedGitForTest(t, repo)
	if commitPatchIDGit(git, repo, reviewedR) != commitPatchIDGit(git, repo, tipC) {
		t.Skip("environment did not reproduce the shared-patch-id whitespace trap")
	}
	appendReboundEdgeT(t, repo, "age-reb", tipC)
	runGitT(t, repo, "add", provenancegraph.LedgerRelativePath)
	runGitT(t, repo, "commit", "-q", "-m", "chore(provenance): bind whitespace-drift REBOUND #trivial")
	bound := runGitT(t, repo, "rev-parse", "HEAD")

	rc, out := runGateT(t, repo, pushLine("refs/heads/main", bound, branchBase))
	if rc != 1 {
		t.Fatalf("a REBOUND whose tip differs only by whitespace (same patch-id) must REFUSE: got %d\n%s", rc, out)
	}
	if !strings.Contains(out, tipC[:12]) {
		t.Fatalf("refusal must name the unauthorized rebound tip %s:\n%s", tipC[:12], out)
	}
}

// FORGE (c2) MODE: the tip drops the reviewed commit's chmod (100755 -> 100644) —
// same +/- text, different mode. The byte-exact content signature (mode-aware)
// catches it → REFUSED.
func TestPrePush_REBOUND_ModeDrift_Refused(t *testing.T) {
	repo := gitInitRepoT(t)
	commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	// Shared base: f.dat with `data\n`, mode 100644.
	branchBase := commitFileT(t, repo, "f.dat", "data\n", "chore: base dat")
	// Reviewed R off the shared base: add X AND make executable (100644 -> 100755).
	if err := os.WriteFile(filepath.Join(repo, "f.dat"), []byte("data\nX\n"), 0o644); err != nil {
		t.Fatalf("write f.dat: %v", err)
	}
	runGitT(t, repo, "add", "--chmod=+x", "f.dat")
	runGitT(t, repo, "commit", "-q", "-m", "feat: add X +chmod (reviewed)")
	reviewedR := runGitT(t, repo, "rev-parse", "HEAD")
	appendLedgerEdgeT(t, repo, "age-rev", reviewedR)
	// Rebound tip off the SHARED base: SAME text, NO chmod (stays 100644) — same
	// +/- text, different mode. -f discards the reviewed commit's on-disk exec bit
	// so the checkout to the 100644 base is clean (we deliberately reset to base).
	runGitT(t, repo, "checkout", "-q", "-f", "-b", "rebound", branchBase)
	tipC := commitFileT(t, repo, "f.dat", "data\nX\n", "feat: add X no-chmod (rebound)")
	appendReboundEdgeT(t, repo, "age-reb", tipC)
	runGitT(t, repo, "add", provenancegraph.LedgerRelativePath)
	runGitT(t, repo, "commit", "-q", "-m", "chore(provenance): bind mode-drift REBOUND #trivial")
	bound := runGitT(t, repo, "rev-parse", "HEAD")

	rc, out := runGateT(t, repo, pushLine("refs/heads/main", bound, branchBase))
	if rc != 1 {
		t.Fatalf("a REBOUND whose tip drops the reviewed chmod must REFUSE: got %d\n%s", rc, out)
	}
	if !strings.Contains(out, tipC[:12]) {
		t.Fatalf("refusal must name the unauthorized rebound tip %s:\n%s", tipC[:12], out)
	}
}

// FORGE (c3) NO-NEWLINE: the tip drops the final newline (same patch-id, same
// +/- text, different diff — git's `\ No newline` marker). The denylist content
// signature keeps that marker byte-exact → REFUSED.
func TestPrePush_REBOUND_NoNewlineDrift_Refused(t *testing.T) {
	repo := gitInitRepoT(t)
	commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	branchBase := commitFileT(t, repo, "f.dat", "data\n", "chore: base dat")
	// Reviewed R off the shared base: add a NEWLINE-TERMINATED line.
	reviewedR := commitFileT(t, repo, "f.dat", "data\nX\n", "feat: add X newline (reviewed)")
	appendLedgerEdgeT(t, repo, "age-rev", reviewedR)
	// Rebound tip off the SHARED base: add the SAME line WITHOUT a trailing newline.
	runGitT(t, repo, "checkout", "-q", "-b", "rebound", branchBase)
	if err := os.WriteFile(filepath.Join(repo, "f.dat"), []byte("data\nX"), 0o644); err != nil {
		t.Fatalf("write no-newline f.dat: %v", err)
	}
	runGitT(t, repo, "add", "f.dat")
	runGitT(t, repo, "commit", "-q", "-m", "feat: add X no-final-newline (rebound)")
	tipC := runGitT(t, repo, "rev-parse", "HEAD")

	// Precondition: the patch-ids MATCH (the trap the byte-exact signature must catch).
	git := trustedGitForTest(t, repo)
	if commitPatchIDGit(git, repo, reviewedR) != commitPatchIDGit(git, repo, tipC) {
		t.Skip("environment did not reproduce the shared-patch-id no-newline trap")
	}
	appendReboundEdgeT(t, repo, "age-reb", tipC)
	runGitT(t, repo, "add", provenancegraph.LedgerRelativePath)
	runGitT(t, repo, "commit", "-q", "-m", "chore(provenance): bind no-newline REBOUND #trivial")
	bound := runGitT(t, repo, "rev-parse", "HEAD")

	rc, out := runGateT(t, repo, pushLine("refs/heads/main", bound, branchBase))
	if rc != 1 {
		t.Fatalf("a REBOUND whose tip drops the final newline (same patch-id) must REFUSE: got %d\n%s", rc, out)
	}
	if !strings.Contains(out, tipC[:12]) {
		t.Fatalf("refusal must name the unauthorized rebound tip %s:\n%s", tipC[:12], out)
	}
}

// A bare REBOUND edge with NO CONFIRMED lineage anywhere in the ledger (the
// simplest forge: just write a REBOUND edge on an unreviewed commit) is REFUSED.
func TestPrePush_REBOUND_NoConfirmedLineageAnywhere_Refused(t *testing.T) {
	repo := gitInitRepoT(t)
	base := commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	code := commitFileT(t, repo, "app.go", "package main\n", "feat: unreviewed change")
	// Only a REBOUND edge — no CONFIRMED edge exists in the ledger at all.
	appendReboundEdgeT(t, repo, "age-reb", code)
	runGitT(t, repo, "add", provenancegraph.LedgerRelativePath)
	runGitT(t, repo, "commit", "-q", "-m", "chore(provenance): bind bare REBOUND #trivial")
	bound := runGitT(t, repo, "rev-parse", "HEAD")

	rc, out := runGateT(t, repo, pushLine("refs/heads/main", bound, base))
	if rc != 1 {
		t.Fatalf("a REBOUND with no CONFIRMED lineage anywhere must REFUSE: got %d\n%s", rc, out)
	}
	if !strings.Contains(out, code[:12]) {
		t.Fatalf("refusal must name the unauthorized commit %s:\n%s", code[:12], out)
	}
}

// appendConfirmedEdgeRawToID appends a CONFIRMED verdict edge whose to_id is an
// ARBITRARY raw string (not necessarily a hex commit id) via the production
// writer, WITHOUT committing. It exists to build the refuter's exact attack: a
// crafted CONFIRMED lineage edge whose to_id is a REVISION EXPRESSION ("HEAD~1").
// The writer (Store.Append / ValidateFields) only requires to_id non-empty, so a
// non-hex to_id is persistable — which is precisely why the GATE must enforce the
// hex-commit-id + object-resolution discipline (a permissive writer is not the
// boundary; the gate is).
func appendConfirmedEdgeRawToID(t *testing.T, repo, bead, rawToID string) {
	t.Helper()
	ledger := filepath.Join(repo, provenancegraph.LedgerRelativePath)
	if _, err := provenancegraph.NewStore(ledger).Append(provenancegraph.Edge{
		FromID: bead + "@" + "fakehed", FromType: "verdict",
		ToID: rawToID, ToType: "commit", Relation: "wasDerivedFrom",
		EvidenceRef: "pawl-verdict " + bead + " disposition=CONFIRMED", TrustTier: "inferred",
		TS: "2026-07-02T00:00:00Z",
	}); err != nil {
		t.Fatalf("append raw-to_id CONFIRMED edge: %v", err)
	}
}

// FORGE (refuter fix, age-rk3r.18): a crafted CONFIRMED lineage edge whose to_id
// is a REVISION EXPRESSION ("HEAD~1"), NOT a hex commit id, must NOT authorize a
// REBOUND on an unreviewed commit C. The pre-fix REBOUND path fed the lineage
// to_id straight to `git show`, which happily resolves "HEAD~1" and — because its
// diff patch-id/content-sig matches the tip's — certified C fail-open. The direct
// CONFIRMED path always rejected the non-hex to_id (shaBindsCommit); the REBOUND
// path must apply the IDENTICAL hex-commit-id + object-resolution discipline.
// RED-first: proven to authorize (exit 0) against the pre-fix code; REFUSES after.
func TestPrePush_REBOUND_NonHexLineageRefAlias_Refused(t *testing.T) {
	repo := gitInitRepoT(t)
	base := commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	// The genuinely-reviewed prior work R is on the SHARED base so HEAD~1 (from the
	// pushed tip's history) has the SAME diff as the unreviewed tip C — the attack
	// only works when some reachable revision is byte-equivalent to C.
	branchBase := commitFileT(t, repo, "app.go", "package main\n", "chore: base app.go")
	// R: the change, on main — but DELIBERATELY given NO CONFIRMED verdict (only the
	// crafted ref-alias edge below points "at" it, via HEAD~1).
	commitFileT(t, repo, "app.go", "package main\n\nfunc F() {}\n", "feat: the change (unreviewed R)")
	// Unreviewed tip C off the SHARED base applies the IDENTICAL change (byte-
	// equivalent to R, i.e. to HEAD~1 of C's own history once the bind commit lands).
	runGitT(t, repo, "checkout", "-q", "-b", "rebound", branchBase)
	tipC := commitFileT(t, repo, "app.go", "package main\n\nfunc F() {}\n", "feat: the change (unreviewed C)")
	// CRAFTED: a "CONFIRMED" edge whose lineage to_id is the REVISION EXPRESSION
	// "HEAD~1" (which, from the bind commit, resolves to tipC's parent-line — a
	// byte-equivalent commit) + a REBOUND edge bound to the unreviewed tipC.
	appendConfirmedEdgeRawToID(t, repo, "age-fake", "HEAD~1")
	appendReboundEdgeT(t, repo, "age-reb", tipC)
	runGitT(t, repo, "add", provenancegraph.LedgerRelativePath)
	runGitT(t, repo, "commit", "-q", "-m", "chore(provenance): bind ref-alias forge #trivial")
	bound := runGitT(t, repo, "rev-parse", "HEAD")

	rc, out := runGateT(t, repo, pushLine("refs/heads/main", bound, base))
	if rc != 1 {
		t.Fatalf("a REBOUND whose CONFIRMED lineage to_id is a REVISION EXPRESSION (HEAD~1), not a hex commit id, must REFUSE (fail-closed): got %d\n%s", rc, out)
	}
	if !strings.Contains(out, tipC[:12]) {
		t.Fatalf("refusal must name the unauthorized rebound tip %s:\n%s", tipC[:12], out)
	}
}

// FORGE (refuter fix, age-rk3r.18): a lineage to_id that is a NON-HEX junk string
// (not a revision, just garbage) must also be rejected — the hex predicate is the
// first gate. Guards the isHexToken branch of hexCommitObjectID directly.
func TestPrePush_REBOUND_NonHexJunkLineage_Refused(t *testing.T) {
	repo := gitInitRepoT(t)
	base := commitFileT(t, repo, "README.md", "hi\n", "chore: init")
	branchBase := commitFileT(t, repo, "app.go", "package main\n", "chore: base app.go")
	commitFileT(t, repo, "app.go", "package main\n\nfunc F() {}\n", "feat: the change (unreviewed R)")
	runGitT(t, repo, "checkout", "-q", "-b", "rebound", branchBase)
	tipC := commitFileT(t, repo, "app.go", "package main\n\nfunc F() {}\n", "feat: the change (unreviewed C)")
	// A non-hex junk lineage to_id (":/feat" is a git :/message revision; "zzzz…"
	// is pure junk) — both must be rejected by the hex predicate.
	appendConfirmedEdgeRawToID(t, repo, "age-fake", "zzzzzzzzzzzz")
	appendReboundEdgeT(t, repo, "age-reb", tipC)
	runGitT(t, repo, "add", provenancegraph.LedgerRelativePath)
	runGitT(t, repo, "commit", "-q", "-m", "chore(provenance): bind junk-lineage forge #trivial")
	bound := runGitT(t, repo, "rev-parse", "HEAD")

	rc, out := runGateT(t, repo, pushLine("refs/heads/main", bound, base))
	if rc != 1 {
		t.Fatalf("a REBOUND whose CONFIRMED lineage to_id is non-hex junk must REFUSE: got %d\n%s", rc, out)
	}
	if !strings.Contains(out, tipC[:12]) {
		t.Fatalf("refusal must name the unauthorized rebound tip %s:\n%s", tipC[:12], out)
	}
}

// hexCommitObjectID unit coverage: a hex commit id resolves to its full oid; a
// revision expression, a non-committish, and non-hex junk all reject ("").
func TestHexCommitObjectID(t *testing.T) {
	repo := gitInitRepoT(t)
	commitFileT(t, repo, "a.txt", "1\n", "chore: c1")
	c2 := commitFileT(t, repo, "a.txt", "1\n2\n", "chore: c2")
	git, err := trustedGit(repo)
	if err != nil {
		t.Fatalf("trustedGit: %v", err)
	}
	// A full hex commit id resolves to itself.
	if got := hexCommitObjectID(git, repo, c2); got != c2 {
		t.Fatalf("full hex id: got %q, want %q", got, c2)
	}
	// A >=7-char hex PREFIX resolves to the full oid (binds it).
	if got := hexCommitObjectID(git, repo, c2[:10]); got != c2 {
		t.Fatalf("hex prefix: got %q, want %q", got, c2)
	}
	// Revision expressions must be REJECTED (never treated as a commit).
	for _, rev := range []string{"HEAD", "HEAD~1", "HEAD^", "main", ":/chore", "@"} {
		if got := hexCommitObjectID(git, repo, rev); got != "" {
			t.Fatalf("revision expression %q must reject, got %q", rev, got)
		}
	}
	// Non-hex junk and too-short hex reject.
	for _, junk := range []string{"zzzzzzz", "abc", "", "  ", "deadbee-"} {
		if got := hexCommitObjectID(git, repo, junk); got != "" {
			t.Fatalf("junk %q must reject, got %q", junk, got)
		}
	}
	// A hex string that does not resolve to any object rejects.
	if got := hexCommitObjectID(git, repo, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"); got != "" {
		t.Fatalf("non-existent hex oid must reject, got %q", got)
	}
}

// Unit-level coverage of the authorization predicate: it accepts a REBOUND when a
// byte-equivalent CONFIRMED lineage exists, and rejects when the disposition is
// bare REBOUND with no equivalent CONFIRMED. Exercises reboundEdgeBoundTo +
// confirmedVerdictCommitSHAs wiring directly.
func TestReboundEdgeBoundTo_ExactDispositionToken(t *testing.T) {
	sha := "abc1234def5678"
	rebound := provenancegraph.Edge{
		FromID: "age-r@" + sha[:7], FromType: "verdict", ToID: sha, ToType: "commit",
		Relation: "wasDerivedFrom", EvidenceRef: "pawl-verdict age-r disposition=REBOUND",
	}
	if !reboundEdgeBoundTo([]provenancegraph.Edge{rebound}, sha) {
		t.Fatal("a REBOUND edge bound to the sha must be recognized")
	}
	// A CONFIRMED edge is NOT a REBOUND edge.
	confirmed := rebound
	confirmed.EvidenceRef = "pawl-verdict age-r disposition=CONFIRMED"
	if reboundEdgeBoundTo([]provenancegraph.Edge{confirmed}, sha) {
		t.Fatal("a CONFIRMED edge must NOT be recognized as a REBOUND edge")
	}
	// A substring/near-miss token must NOT match (exact-token discipline).
	near := rebound
	near.EvidenceRef = "pawl-verdict age-r disposition=REBOUNDLY"
	if reboundEdgeBoundTo([]provenancegraph.Edge{near}, sha) {
		t.Fatal("disposition=REBOUNDLY must NOT match the exact REBOUND token")
	}
	// FIRST-token contract (the CI-parity anchor, age-rk3r.18): a double-disposition
	// edge whose FIRST token is REFUTED is NOT a REBOUND (parseDisposition returns
	// the first token). The CI backstop's jq dispvalue must mirror this exactly.
	doubleReb := rebound
	doubleReb.EvidenceRef = "pawl-verdict age-r disposition=REFUTED disposition=REBOUND"
	if reboundEdgeBoundTo([]provenancegraph.Edge{doubleReb}, sha) {
		t.Fatal("disposition=REFUTED disposition=REBOUND must NOT match REBOUND (first token wins)")
	}
	if confirmedVerdictEdgeIn([]provenancegraph.Edge{doubleReb}, sha) {
		t.Fatal("a double-disposition REBOUND edge must NOT be a CONFIRMED either")
	}
	// The symmetric CONFIRMED case: first token REFUTED ⇒ not a CONFIRMED.
	doubleConf := rebound
	doubleConf.EvidenceRef = "pawl-verdict age-r disposition=REFUTED disposition=CONFIRMED"
	if confirmedVerdictEdgeIn([]provenancegraph.Edge{doubleConf}, sha) {
		t.Fatal("disposition=REFUTED disposition=CONFIRMED must NOT match CONFIRMED (first token wins)")
	}
	if got := confirmedVerdictCommitSHAs([]provenancegraph.Edge{doubleConf}); len(got) != 0 {
		t.Fatalf("a double-disposition (first=REFUTED) edge must not be a lineage candidate, got %v", got)
	}
	// confirmedVerdictCommitSHAs collects only CONFIRMED to_ids, deduped.
	shas := confirmedVerdictCommitSHAs([]provenancegraph.Edge{confirmed, confirmed, rebound})
	if len(shas) != 1 || shas[0] != sha {
		t.Fatalf("confirmedVerdictCommitSHAs = %v, want [%s]", shas, sha)
	}
}
