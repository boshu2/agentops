// practices: [design-by-contract]
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// diff_identity_test.go — the PARITY guard (age-rk3r.18). The Go diff-identity
// port (commitPatchIDGit / commitContentSigGit, diff_identity.go) is the
// hostile-repo-safe re-derivation the portable pre-push gate uses to prove a tip
// is byte-equivalent to a reviewed commit. It MUST produce the SAME normalization
// the shell library scripts/lib/diff-identity.sh does — a drift would let a
// rebase the shell REFUSES be authorized by the Go gate (or vice-versa),
// splitting the exact parity class age-rk3r.9 closed. This cross-checks the Go
// signature BYTE-FOR-BYTE against the shell `commit_content_sig` + patch-id on
// sample commits (a text change AND a mode/binary change).

// findDiffIdentityShell walks up from the test source to the shell diff-identity
// library, so the cross-check runs against the real production shell.
func findDiffIdentityShell(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)
	for range 6 {
		cand := filepath.Join(dir, "scripts", "lib", "diff-identity.sh")
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
		dir = filepath.Dir(dir)
	}
	t.Fatalf("could not locate scripts/lib/diff-identity.sh above %s", filepath.Dir(thisFile))
	return ""
}

// shellSig runs the shell library's commit_content_sig + commit_patch_id for
// commit sha in repo, returning (contentSig, patchID). It sources the real
// production shell so any drift in the Go port is caught. Skips the test if
// bash/git are unavailable.
func shellSig(t *testing.T, shellLib, repo, sha string) (contentSig, patchID string) {
	t.Helper()
	script := "set -euo pipefail\n" +
		"source \"$1\"\n" +
		"commit_content_sig \"$2\" \"$3\"\n" +
		"echo\n" + // separator newline between the two outputs
		"commit_patch_id \"$2\" \"$3\"\n"
	cmd := exec.Command("bash", "-c", script, "bash", shellLib, sha, repo)
	cmd.Env = cleanGitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shell diff-identity failed for %s: %v\n%s", sha[:7], err, out)
	}
	// Two lines: content_sig, then patch_id (the echo adds the separating newline
	// after content_sig, which printf '%s' emitted with no trailing newline).
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("shell diff-identity output malformed for %s:\n%s", sha[:7], out)
	}
	return strings.TrimSpace(lines[0]), strings.TrimSpace(lines[len(lines)-1])
}

// trustedGitForTest resolves git the same way the gate does, so the Go port runs
// through the identical binary. On failure it falls back to a bare PATH git so
// the parity test still runs (the trust-boundary is exercised by the gate tests,
// not this pure-parity check).
func trustedGitForTest(t *testing.T, repo string) string {
	t.Helper()
	if g, err := trustedGit(repo); err == nil {
		return g
	}
	g, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not on PATH")
	}
	return g
}

func TestDiffIdentity_GoMatchesShell_TextChange(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash unavailable")
	}
	shellLib := findDiffIdentityShell(t)
	repo := gitInitRepoT(t)
	commitFileT(t, repo, "f.txt", "base\n", "chore: base")
	sha := commitFileT(t, repo, "f.txt", "base\nthe-change\n", "feat: add a line")

	git := trustedGitForTest(t, repo)
	goSig := commitContentSigGit(git, repo, sha)
	goPID := commitPatchIDGit(git, repo, sha)
	if goSig == "" || goPID == "" {
		t.Fatalf("Go port produced empty sig/patch-id (sig=%q pid=%q)", goSig, goPID)
	}
	shSig, shPID := shellSig(t, shellLib, repo, sha)
	if goSig != shSig {
		t.Fatalf("content-signature DRIFT (text change): Go=%q shell=%q", goSig, shSig)
	}
	if goPID != shPID {
		t.Fatalf("patch-id DRIFT (text change): Go=%q shell=%q", goPID, shPID)
	}
}

func TestDiffIdentity_GoMatchesShell_ModeChange(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash unavailable")
	}
	shellLib := findDiffIdentityShell(t)
	repo := gitInitRepoT(t)
	commitFileT(t, repo, "s.sh", "#!/bin/sh\necho hi\n", "chore: base script")
	// Flip the file's mode to executable (100644 -> 100755): a mode-only change,
	// the category patch-id ignores but the byte-exact content signature keeps.
	runGitT(t, repo, "update-index", "--chmod=+x", "s.sh")
	runGitT(t, repo, "commit", "-q", "-m", "chore: make executable")
	sha := runGitT(t, repo, "rev-parse", "HEAD")

	git := trustedGitForTest(t, repo)
	goSig := commitContentSigGit(git, repo, sha)
	if goSig == "" {
		t.Fatalf("Go port produced empty sig for a mode change")
	}
	shSig, _ := shellSig(t, shellLib, repo, sha)
	if goSig != shSig {
		t.Fatalf("content-signature DRIFT (mode change): Go=%q shell=%q", goSig, shSig)
	}
	// patch-id is whitespace/mode-insensitive; the byte-exact signature is the key
	// that discriminates a mode change, so parity on the SIGNATURE is the load-
	// bearing assertion here.
}

func TestDiffIdentity_GoMatchesShell_BinaryChange(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash unavailable")
	}
	shellLib := findDiffIdentityShell(t)
	repo := gitInitRepoT(t)
	// A binary blob: git renders "Binary files … differ" with a verbatim index
	// line (the blob id IS the content identity) — the binary-hunk branch of the
	// denylist. Write bytes that force git's binary detection (a NUL byte).
	binPath := filepath.Join(repo, "b.bin")
	if err := os.WriteFile(binPath, []byte{0x00, 0x01, 0x02, 0x03}, 0o644); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	runGitT(t, repo, "add", "b.bin")
	runGitT(t, repo, "commit", "-q", "-m", "chore: add binary")
	sha := runGitT(t, repo, "rev-parse", "HEAD")

	git := trustedGitForTest(t, repo)
	goSig := commitContentSigGit(git, repo, sha)
	if goSig == "" {
		t.Fatalf("Go port produced empty sig for a binary change")
	}
	shSig, _ := shellSig(t, shellLib, repo, sha)
	if goSig != shSig {
		t.Fatalf("content-signature DRIFT (binary change): Go=%q shell=%q", goSig, shSig)
	}
}

// TestDiffIdentity_GoMatchesShell_TrickyDiffs cross-checks the Go port against
// the shell on the boundary cases where the awk-vs-Go line handling could drift:
// a trailing BLANK line (the added line's content is "+", never empty — only the
// diff's terminating newline yields an empty record), a multi-hunk diff (multiple
// "@@ POS @@" normalizations), and a file whose final line has no newline (the
// `\ No newline at end of file` marker kept verbatim).
func TestDiffIdentity_GoMatchesShell_TrickyDiffs(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash unavailable")
	}
	shellLib := findDiffIdentityShell(t)
	cases := []struct {
		name, base, change string
	}{
		{"trailing-blank-line", "a\nb\n", "a\nb\n\n"},
		{"multi-hunk", "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\n", "X\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nY\n"},
		{"no-final-newline", "data\n", "data\nX"},
		{"leading-whitespace-content", "def f():\n    pass\n", "def f():\n    pass\n    y = 1\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := gitInitRepoT(t)
			commitFileT(t, repo, "f.txt", tc.base, "chore: base")
			sha := commitFileT(t, repo, "f.txt", tc.change, "feat: change")
			git := trustedGitForTest(t, repo)
			goSig := commitContentSigGit(git, repo, sha)
			goPID := commitPatchIDGit(git, repo, sha)
			if goSig == "" || goPID == "" {
				t.Fatalf("empty Go sig/pid (sig=%q pid=%q)", goSig, goPID)
			}
			shSig, shPID := shellSig(t, shellLib, repo, sha)
			if goSig != shSig {
				t.Fatalf("content-signature DRIFT (%s): Go=%q shell=%q", tc.name, goSig, shSig)
			}
			if goPID != shPID {
				t.Fatalf("patch-id DRIFT (%s): Go=%q shell=%q", tc.name, goPID, shPID)
			}
		})
	}
}

// TestDiffIdentity_EquivalentRebaseSameKeys locks the core equivalence property
// the REBOUND gate rests on: a genuine no-op rebase (same change, new sha/date/
// message) produces the SAME patch-id AND the SAME content signature, while a
// one-line-different commit produces a DIFFERENT patch-id — proving the keys
// discriminate a real change from a rebase.
func TestDiffIdentity_EquivalentRebaseSameKeys(t *testing.T) {
	repo := gitInitRepoT(t)
	commitFileT(t, repo, "f.txt", "base\n", "chore: base")
	reviewed := commitFileT(t, repo, "f.txt", "base\nX\n", "feat: add X")
	git := trustedGitForTest(t, repo)
	rPID, rSig := commitPatchIDGit(git, repo, reviewed), commitContentSigGit(git, repo, reviewed)

	// Rebase: reset one back, re-apply the SAME diff with a new message.
	runGitT(t, repo, "reset", "-q", "--hard", "HEAD~1")
	rebased := commitFileT(t, repo, "f.txt", "base\nX\n", "feat: add X (reworded)")
	if rebased == reviewed {
		t.Fatal("rebased sha must differ from reviewed sha")
	}
	if got := commitPatchIDGit(git, repo, rebased); got != rPID {
		t.Fatalf("equivalent rebase patch-id must match: reviewed=%q rebased=%q", rPID, got)
	}
	if got := commitContentSigGit(git, repo, rebased); got != rSig {
		t.Fatalf("equivalent rebase content-sig must match: reviewed=%q rebased=%q", rSig, got)
	}

	// A DIFFERENT change must produce a different patch-id.
	runGitT(t, repo, "reset", "-q", "--hard", "HEAD~1")
	different := commitFileT(t, repo, "f.txt", "base\nY\n", "feat: add Y")
	if got := commitPatchIDGit(git, repo, different); got == rPID {
		t.Fatalf("a different change must NOT share the reviewed patch-id (got %q)", got)
	}
}
