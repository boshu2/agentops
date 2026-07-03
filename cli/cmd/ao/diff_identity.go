// practices: [design-by-contract]
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os/exec"
	"regexp"
	"strings"
)

// diff_identity.go — the GO PORT of scripts/lib/diff-identity.sh, for the
// hostile-repo-safe path (the portable pre-push gate, verify_prepush.go). The
// gate runs NO repo-tree script (the age-rk3r.6 trust boundary: a repo under
// review must not supply the code that gates it), so the shell diff-identity
// library — which lives IN the repo tree — cannot be shelled out to here. This
// file re-derives the SAME signature in Go using a caller-supplied TRUSTED git
// binary (verify_prepush.go's trustedGit), so the REBOUND lineage+proof
// re-validation (age-rk3r.18) can prove a tip is byte-equivalent to a reviewed
// commit without trusting either the repo's scripts or the (uncommitted,
// .agents/-gitignored) verdict file's self-declared patch_id_proof.
//
// PARITY IS LOAD-BEARING AND TESTED: commitContentSigGit must produce the SAME
// normalization the shell commit_content_sig does — a Go signature that drifts
// from the shell would let a rebase the shell REFUSES be authorized by the Go
// gate (or vice-versa), splitting the exact parity class age-rk3r.9 closed. The
// cross-check test (TestDiffIdentity_GoMatchesShell) asserts byte-identical
// output against scripts/lib/diff-identity.sh on sample commits (a text change
// and a mode/binary change).
//
// The two functions mirror the shell exports:
//   commitPatchIDGit    <-> commit_patch_id     (the rebase-stable key)
//   commitContentSigGit <-> commit_content_sig  (sha256 of the byte-exact signature)
// The intermediate commit_content_lines is inlined as commitContentLinesGit
// (unexported) because only the digest is compared here.

// diffIndexTextRE recognizes an `index <pre>..<post>[ <mode>]` line of a TEXT
// hunk — the ONLY case whose blob ids are normalized. It mirrors the shell awk
// pattern /^index [0-9a-f]+\.\.[0-9a-f]+( [0-7]+)?$/ EXACTLY (anchored, lowercase
// hex, optional octal mode). A line that does not match this shape is kept
// VERBATIM (fail-safe, never silently dropped) — the same as the shell's else.
var diffIndexTextRE = regexp.MustCompile(`^index [0-9a-f]+\.\.[0-9a-f]+( [0-7]+)?$`)

// diffIndexModeRE captures the trailing octal mode of an `index …` line so the
// text-hunk normalization keeps the mode while replacing the blob ids
// (mirrors the shell's `if (idx_pending ~ /^index …[0-7]+$/) { … mode … }`).
var diffIndexModeRE = regexp.MustCompile(`^index [0-9a-f]+\.\.[0-9a-f]+ ([0-7]+)$`)

// commitDiffRaw renders a commit's diff with the SAME helper-disabling flags the
// reviewer + the shell library use (--no-ext-diff/--no-textconv/-c
// core.fsmonitor=/--no-color/--format=), via the caller-supplied TRUSTED git
// binary. An untrusted repo's diff drivers therefore cannot execute here, and
// the git binary is never a repo-planted one (trustedGit resolves an absolute
// git off a sanitized PATH). Returns "" and false on any git failure (unknown
// sha, empty diff, git error) — the caller MUST treat that as "cannot prove
// identity" and fail-closed, never as a match.
func commitDiffRaw(gitBin, repo, sha string) (string, bool) {
	if gitBin == "" || sha == "" {
		return "", false
	}
	out, err := exec.Command(gitBin, "-c", "core.fsmonitor=", "-C", repo, "show", sha,
		"--no-ext-diff", "--no-textconv", "--no-color", "--format=").Output() // #nosec G204 -- gitBin is trusted-PATH-resolved (trustedGit); repo is the resolved git toplevel; sha is a ledger/push-supplied object id.
	if err != nil {
		return "", false
	}
	return string(out), true
}

// commitPatchIDGit prints the STABLE git patch-id of a commit's diff — the
// REBASE-STABLE key, the Go twin of the shell commit_patch_id. It pipes the
// trusted-git diff through `git patch-id --stable` (same trusted binary) and
// returns the first field of the first line. Empty on any failure; the caller
// fail-closes on empty (never treats it as a match).
//
// CAVEAT (verbatim from the shell): patch-id ALONE is NOT sufficient for the
// REBOUND safety claim — it is WHITESPACE-INSENSITIVE and ignores file mode /
// trailing newline, so a whitespace-only change, a chmod, or a dropped final
// newline can share a patch-id. Identity ALSO requires commitContentSigGit.
func commitPatchIDGit(gitBin, repo, sha string) string {
	diff, ok := commitDiffRaw(gitBin, repo, sha)
	if !ok {
		return ""
	}
	pid := exec.Command(gitBin, "patch-id", "--stable") // #nosec G204 -- gitBin is the trusted-PATH-resolved git; no repo-supplied args.
	pid.Stdin = strings.NewReader(diff)
	out, err := pid.Output()
	if err != nil {
		return ""
	}
	// First field of the first line is the patch-id (git prints "<patch-id> <commit-id>").
	line := out
	if i := indexByte(out, '\n'); i >= 0 {
		line = out[:i]
	}
	fields := strings.Fields(string(line))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// indexByte returns the index of the first b in s, or -1 (a tiny helper so the
// patch-id first-line split does not pull in bytes just for IndexByte).
func indexByte(s []byte, b byte) int {
	for i := range s {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// commitContentLinesGit is the Go port of the shell commit_content_lines awk
// program: a commit diff's BYTE-EXACT change identity, normalized ONLY where a
// legitimate rebase of the SAME change provably shifts. It is a DENYLIST
// (keep-everything-except-provably-volatile), NOT an allowlist — the same stance
// age-rk3r.9 landed to close the whole escape class (whitespace → mode → binary
// → `\ No newline`) at once.
//
// Byte-exact EXCEPT the two parts that legitimately shift across a rebase:
//  1. TEXT-hunk blob ids in an `index <pre>..<post>[ <mode>]` line → replaced
//     with "index BLOB..BLOB[ <mode>]" (blob ids constant, trailing MODE kept). A
//     BINARY hunk's index line (followed by "Binary files … differ") is kept
//     VERBATIM: the blob id IS the content identity for a binary file.
//  2. `@@ -a,b +c,d @@ [ctx]` → the WHOLE line normalized to "@@ POS @@"
//     (positions AND the trailing function-context, which copies a shifting
//     nearby source line).
//
// EVERYTHING ELSE is kept VERBATIM and SIGNIFICANT: all +/- content
// (whitespace-exact), the diff/---/+++ headers, old/new/new-file/deleted-file
// mode lines, "Binary files … differ", AND git's `\ No newline at end of file`
// marker. The `index` line is BUFFERED until the next significant line reveals
// whether its file-hunk is binary or text — identical to the awk's flush_index.
//
// Returns "" and false when the diff is empty or git failed — the caller
// fail-closes on empty (cannot prove content identity).
func commitContentLinesGit(gitBin, repo, sha string) (string, bool) {
	raw, ok := commitDiffRaw(gitBin, repo, sha)
	if !ok {
		return "", false
	}
	// Split on '\n' preserving the exact line content. The shell awk sees git's
	// output line-by-line (records split on '\n'); a trailing '\n' produces a
	// final empty record that awk drops (no line to print), which strings.Split
	// also yields and we skip via the same "no trailing empty emitted" behavior:
	// we join with '\n' and never append for a bare-empty final element.
	lines := strings.Split(raw, "\n")
	// Drop a single trailing empty element from a terminating newline so the
	// reconstructed output has no spurious final blank line (matches awk, which
	// only prints lines it actually processed).
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}

	var out []string
	idxPending := ""
	isBinary := false

	// flushIndex mirrors the awk flush_index(): emit the buffered `index` line,
	// normalizing blob ids ONLY for a TEXT hunk; keep VERBATIM for binary or an
	// unexpected shape (fail-safe, never silently dropped).
	flushIndex := func() {
		if idxPending == "" {
			return
		}
		switch {
		case isBinary:
			out = append(out, idxPending)
		case diffIndexTextRE.MatchString(idxPending):
			mode := ""
			if m := diffIndexModeRE.FindStringSubmatch(idxPending); m != nil {
				mode = " " + m[1]
			}
			out = append(out, "index BLOB..BLOB"+mode)
		default:
			out = append(out, idxPending) // unexpected index shape: keep VERBATIM
		}
		idxPending = ""
	}

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			flushIndex()
			isBinary = false
			out = append(out, line)
		case strings.HasPrefix(line, "index "):
			flushIndex()
			idxPending = line // buffer; classify on the next line
		case strings.HasPrefix(line, "Binary files "):
			isBinary = true
			flushIndex() // binary marker -> the buffered index kept verbatim
			out = append(out, line)
		case strings.HasPrefix(line, "@@ "):
			isBinary = false
			flushIndex()
			out = append(out, "@@ POS @@") // normalize the WHOLE @@ line (positions + ctx)
		default:
			isBinary = false
			flushIndex()
			out = append(out, line) // EVERY other line verbatim (incl. the \ No newline marker)
		}
	}
	flushIndex() // END { flush_index() }

	if len(out) == 0 {
		return "", false
	}
	return strings.Join(out, "\n"), true
}

// commitContentSigGit is the Go port of the shell commit_content_sig: the
// sha256 of commitContentLinesGit, a fixed-width digest of the byte-exact
// content signature. The shell hashes `printf '%s'` of the lines (NO trailing
// newline), so this hashes the joined string with NO trailing newline — the
// parity the cross-check test asserts. Empty on any failure (fail-closed, same
// as the shell).
func commitContentSigGit(gitBin, repo, sha string) string {
	lines, ok := commitContentLinesGit(gitBin, repo, sha)
	if !ok || lines == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(lines))
	return hex.EncodeToString(sum[:])
}
