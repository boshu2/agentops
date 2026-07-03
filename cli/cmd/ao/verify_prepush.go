// practices: [design-by-contract, in-toto-provenance]
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// verifyPrePushExitError carries the pre-push gate's process exit code so it
// propagates verbatim through Execute (like pawlReviewExitError): 0 allow, 1
// refuse. Error() is empty because the gate prints its own human message; the
// code IS the decision.
type verifyPrePushExitError struct{ code int }

func (e *verifyPrePushExitError) Error() string { return "" }

// ExitCode returns the process exit code this decision maps to.
func (e *verifyPrePushExitError) ExitCode() int { return e.code }

// verifyPrePushCmd is the RUNTIME entry point the portable pre-push hook
// installed by `ao verify init` (age-rk3r.6) invokes. It reads git's pre-push
// stdin and, for every commit pushed to main/master, requires EITHER a
// commit-bound CONFIRMED cross-family verdict edge in the ledger AS COMMITTED
// at the pushed tip (never the working-tree file — an uncommitted edge never
// reaches the remote and is not proof) OR the provenance-only #trivial waiver —
// the sovereign, local complement to the CI verdict backstop
// (scripts/check-tip-verdict-ci.sh --enforce), enforced with NO repo-local
// scripts: the whole gate is this trusted ao binary (the aoBinaryInside
// boundary the hook rests on).
//
// It is Hidden because it is machine-invoked plumbing, not a human verb. It
// lives under `verify` (its conceptual home) rather than the DisableFlagParsing
// forwarding it inherits from that parent being a problem: the hook only ever
// reaches it AFTER `ao provenance ledger-reader-version` proves the binary is new
// enough (a pre-age-rk3r.6 binary, which would misroute `ao verify pre-push` into
// the review engine, never gets here — the probe fails first and the hook refuses
// with an upgrade message).
var verifyPrePushCmd = &cobra.Command{
	Use:    "pre-push",
	Short:  "Pre-push verdict gate (machine-invoked by the hook `ao verify init` installs)",
	Hidden: true,
	Long: `Read git's pre-push hook stdin (one "local_ref local_sha remote_ref remote_sha"
line per pushed ref) and gate the push:

  (a) the provenance ledger (docs/provenance/ledger.jsonl) AS COMMITTED in the
      pushed tip's tree must be an intact, tamper-evident hash chain (a broken
      chain refuses the push); and
  (b) every commit pushed to main/master must carry proof — EITHER a CONFIRMED
      cross-family verdict edge bound to it in that committed ledger
      (from_type=verdict, to_type=commit, disposition=CONFIRMED, the shape
      'ao verify' binds) OR a REBOUND verdict edge whose lineage + byte-
      equivalence RE-VALIDATE (a committed CONFIRMED-reviewed commit exists that
      is byte-equivalent to this tip — same git patch-id --stable AND byte-exact
      content signature, re-derived here via trusted git; a bare disposition=
      REBOUND is NOT accepted, and the edge's stored patch_id_proof is never
      trusted) OR the provenance-only #trivial waiver (every changed file under
      docs/provenance/).

Both checks read the ledger blob from the PUSHED TIP'S TREE, never the working
tree: the remote only receives committed history, so an appended-but-uncommitted
edge is not proof (a working-tree read here was a refuted fail-open) and
worktree-only dirt cannot break a clean push. A tip whose tree carries no ledger
has, by definition, no verdict edges — its non-waived commits are refused with a
hint that the bound ledger edge must be COMMITTED into the push.

The checked range is base..tip for an update push whose base resolves locally. A
CREATION push (all-zero remote sha) OR an update whose non-zero base is absent
locally (shallow clone / GC'd base) checks the tip's history excluding commits
already on the TARGET remote's trunk ref (git rev-list <tip> --not
refs/remotes/<target-remote>/<branch>, from git's forwarded remote name).
"Already verified" means on THIS target's trunk, NOT on any remote branch: a
commit reachable only from an ungated feature branch (origin/feature-x) or from
ANOTHER remote's main (backup/main) is NEW to this target's trunk and IS checked. There is NO "check only
the tip" path: narrowing to the tip was a fail-open (an unverified history
riding in under one #trivial tip), so it is DELETED — every path either checks
the true new-commit range or refuses. If an absent non-zero base has no trunk
remote-tracking ref at all to derive against, the gate cannot tell which commits
are new and REFUSES (fetch the remote, or push with an explicit base). Every
derived commit is checked regardless of range size — correctness over speed —
with a count line for legibility.

No proof = no verdict = not done: the push is refused with the fix named. The
whole gate is this ao binary — it runs NO script from the repository tree, so a
repo under review cannot subvert its own gate.

This is machine plumbing; run 'ao verify <change-id>' to produce a verdict.

Escape hatch (loud, use only with cause): AGENTOPS_VERIFY_PREPUSH_SKIP=1.`,
	// git may pass <remote-name> <remote-url> positionally; accept and ignore.
	Args: cobra.ArbitraryArgs,
	RunE: runVerifyPrePush,
}

func init() {
	verifyCmd.AddCommand(verifyPrePushCmd)
}

// prePushLine is one parsed pre-push stdin record.
type prePushLine struct {
	localRef  string
	localSHA  string
	remoteRef string
	remoteSHA string
}

var trivialSubjectRE = regexp.MustCompile(`(?i)(^|\s)#trivial\s*$`)
var trivialBodyRE = regexp.MustCompile(`(?im)^\s*#trivial\s*$`)

// runVerifyPrePush is the gate. It fails CLOSED everywhere: any inability to
// prove a commit's verdict refuses the push.
func runVerifyPrePush(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	out := cmd.ErrOrStderr()

	// git invokes the pre-push hook as `pre-push <remote-name> <remote-url>`; the
	// installed hook forwards those, so args[0] is the TARGET remote name. It scopes
	// "already verified" to THAT remote's trunk — a commit sitting only on another
	// remote's main (e.g. a backup remote) was never gated for this target and must
	// still be checked. Empty (manual run / push-to-URL) falls back to the branch
	// across all remotes.
	targetRemote := ""
	if len(args) > 0 {
		targetRemote = args[0]
	}

	// Loud, audited escape hatch mirroring the repo's own AGENTOPS_GATE_DISABLED.
	if truthyValue(os.Getenv("AGENTOPS_VERIFY_PREPUSH_SKIP")) {
		fmt.Fprintln(out, "⚠ ao verify pre-push: BYPASSED (AGENTOPS_VERIFY_PREPUSH_SKIP=1) — the verdict ratchet is disabled for this push")
		return nil
	}

	// The floor is enforced by the hook's `ao provenance ledger-reader-version`
	// probe BEFORE this runs, and the running binary is by definition capable of
	// reading its own ledger version — so there is nothing to re-check here.
	_ = provenancegraph.LedgerReaderVersion

	// A push must have supplied stdin. A terminal / manual run has no push lines
	// to gate — skip cleanly (fail-open is correct: there is no push to prove).
	in := cmd.InOrStdin()
	if f, ok := in.(*os.File); ok {
		if info, statErr := f.Stat(); statErr == nil && (info.Mode()&os.ModeCharDevice) != 0 {
			fmt.Fprintln(out, "ao verify pre-push: no pre-push stdin (interactive) — skipped (not a git push hook invocation)")
			return nil
		}
	}
	data, readErr := io.ReadAll(in)
	if readErr != nil {
		fmt.Fprintf(out, "PUSH REFUSED: could not read pre-push stdin — fail-closed: %v\n", readErr)
		return &verifyPrePushExitError{code: 1}
	}
	lines := parsePrePushLines(string(data))
	if len(lines) == 0 {
		fmt.Fprintln(out, "ao verify pre-push: no pre-push stdin — skipped (not a git push hook invocation)")
		return nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(out, "PUSH REFUSED: cannot resolve working directory — fail-closed: %v\n", err)
		return &verifyPrePushExitError{code: 1}
	}
	repo, err := gitToplevel(cwd)
	if err != nil {
		fmt.Fprintf(out, "PUSH REFUSED: not inside a git repository — fail-closed: %v\n", err)
		return &verifyPrePushExitError{code: 1}
	}
	// Resolve git ONCE up front for the clear failure message; every helper
	// below re-resolves through the same trustedGit (identical PATH → identical
	// result), so a repo-planted git is never executed anywhere in the gate.
	if _, gitErr := trustedGit(repo); gitErr != nil {
		fmt.Fprintf(out, "PUSH REFUSED: %v — fail-closed (the gate only runs a git resolved from an absolute PATH entry outside the repo)\n", gitErr)
		return &verifyPrePushExitError{code: 1}
	}

	// Gate each main/master line against the ledger AS COMMITTED at ITS pushed
	// tip. The working-tree ledger is deliberately never consulted (refuted
	// fail-open): the remote only receives committed trees, so an appended-but-
	// uncommitted edge is not proof and worktree dirt cannot break a clean push.
	for _, ln := range lines {
		if !isMainRef(ln.remoteRef) {
			continue
		}
		if isZeroSHA(ln.localSHA) {
			continue // branch delete — nothing to verify
		}
		if gateErr := checkMainPush(out, repo, targetRemote, ln); gateErr != nil {
			return &verifyPrePushExitError{code: 1}
		}
	}
	return nil
}

// checkMainPush gates ONE pushed main/master ref line: chain-verify the ledger
// as committed at the pushed tip, then require per-commit proof over the pushed
// range. A non-nil return means the refusal was already printed and the push
// must exit 1. Fail-closed on every inability to prove.
func checkMainPush(out io.Writer, repo, targetRemote string, ln prePushLine) error {
	if !gitCommitExists(repo, ln.localSHA) {
		fmt.Fprintf(out, "PUSH REFUSED: pushed tip %s does not resolve to a local commit — fail-closed\n", short12(ln.localSHA))
		return fmt.Errorf("unresolvable pushed tip %s", short12(ln.localSHA))
	}

	// Resolve the ledger blob from the TIP'S TREE. Absent is a legitimate state
	// (a repo before its first verdict): the chain is vacuously intact and no
	// verdict edges exist, so every verdict-requiring commit below is refused
	// with the explicit commit-the-ledger hint. Any other read failure cannot
	// prove anything — refuse.
	ledgerBytes, ledgerPresent, err := ledgerBlobAt(repo, ln.localSHA)
	if err != nil {
		fmt.Fprintf(out, "PUSH REFUSED: cannot read %s from the pushed tip %s — fail-closed: %v\n",
			provenancegraph.LedgerRelativePath, short12(ln.localSHA), err)
		return err
	}
	// edges = the verdict-edge set AS COMMITTED at the tip; empty when the tree
	// carries no ledger. Read ONCE per push line (a creation push can span a
	// whole history — a per-commit re-read would be O(commits × ledger)).
	var edges []provenancegraph.Edge
	if ledgerPresent {
		tmpPath, cleanup, mErr := materializeLedger(ledgerBytes)
		if mErr != nil {
			fmt.Fprintf(out, "PUSH REFUSED: %v — fail-closed\n", mErr)
			return mErr
		}
		defer cleanup()
		// (a) chain verify on the committed bytes — tamper trumps everything,
		// including #trivial waivers.
		if cErr := verifyLedgerChain(tmpPath, ln.localSHA, out); cErr != nil {
			return cErr
		}
		var readErr error
		edges, readErr = provenancegraph.NewStore(tmpPath).Read()
		if readErr != nil {
			// The chain verify above already parsed the file, so this is nearly
			// unreachable — but an unreadable ledger proves nothing: fail-closed.
			fmt.Fprintf(out, "PUSH REFUSED: cannot read the committed ledger at %s — fail-closed: %v\n",
				short12(ln.localSHA), readErr)
			return readErr
		}
	}

	// (b) per-commit verdict-or-waiver over the pushed range, scoped to the
	// GATED TRUNK being pushed to (from the push line's remote ref).
	commits, rangeErr := commitRange(repo, targetRemote, ln.remoteSHA, ln.localSHA, refBranch(ln.remoteRef))
	if rangeErr != nil {
		if errors.Is(rangeErr, errIndeterminateBase) {
			fmt.Fprintf(out, "PUSH REFUSED: base %s is on the remote but absent from this clone and there is no "+
				"remote-tracking ref for the trunk (%s) to compute the new commits from — cannot determine which "+
				"commits are new; fetch the remote (git fetch) or push with an explicit base (fail-closed).\n",
				short12(ln.remoteSHA), ln.remoteRef)
			return rangeErr
		}
		fmt.Fprintf(out, "PUSH REFUSED: cannot resolve pushed commit range %s..%s — fail-closed: %v\n",
			short12(ln.remoteSHA), short12(ln.localSHA), rangeErr)
		return rangeErr
	}
	// The range was derived against the trunk's remote-tracking ref(s) (`--not
	// refs/remotes/*/<branch>`) whenever the base was not a locally-resolvable
	// non-zero commit — a creation push OR an unknown-base update. That range can
	// span a whole history; every commit is still checked (correctness over
	// speed), and this count line keeps a large pass/refusal legible.
	if isZeroSHA(ln.remoteSHA) || ln.remoteSHA == "" || !gitCommitExists(repo, ln.remoteSHA) {
		fmt.Fprintf(out, "ao verify pre-push: %s — checking all %d commit(s) not already on the trunk's remote-tracking ref\n",
			ln.remoteRef, len(commits))
	}
	var violations []string
	for _, sha := range commits {
		if confirmedVerdictEdgeIn(edges, sha) {
			continue
		}
		// A committed REBOUND edge authorizes ONLY after Go-side lineage + proof
		// RE-VALIDATION (age-rk3r.18): a bare disposition==REBOUND accept would be a
		// forge-a-rebound fail-open. reboundVerdictAuthorizes proves — from the
		// committed ledger + trusted git alone — that a CONFIRMED-reviewed commit
		// byte-equivalent to sha exists (patch-id + content-signature). It never
		// trusts the (uncommitted) verdict file's self-declared patch_id_proof.
		if reboundVerdictAuthorizes(repo, edges, sha) {
			continue
		}
		waived, werr := trivialWaiver(repo, sha)
		if werr == nil && waived {
			// Path-waivable — but a provenance-only #trivial commit may not tamper
			// with the proof ledger. Enforce APPEND-ONLY; a delete/rewrite is a
			// hard, specifically-named refusal (fail-closed).
			if apErr := ensureLedgerAppendOnly(out, repo, sha); apErr != nil {
				return apErr
			}
			continue
		}
		violations = append(violations, fmt.Sprintf("  %s  %s", short12(sha), gitSubject(repo, sha)))
	}
	if len(violations) == 0 {
		return nil
	}
	fmt.Fprintf(out, "PUSH REFUSED: %d commit(s) pushed to %s carry no proof — no verdict = not done:\n",
		len(violations), ln.remoteRef)
	for _, v := range violations {
		fmt.Fprintln(out, v)
	}
	fmt.Fprintln(out, "")
	if !ledgerPresent {
		fmt.Fprintf(out, "NOTE: %s is ABSENT from the pushed tip's tree (%s) — a verdict counts only once\n",
			provenancegraph.LedgerRelativePath, short12(ln.localSHA))
		fmt.Fprintln(out, "its ledger edge is COMMITTED into the push; an appended-but-uncommitted edge never reaches the remote.")
	}
	fmt.Fprintln(out, "Each pushed commit needs EITHER a CONFIRMED cross-family verdict bound to it in")
	fmt.Fprintln(out, "docs/provenance/ledger.jsonl (as committed in the pushed history), OR to be a provenance-only #trivial commit.")
	fmt.Fprintln(out, "  Fix:  ao verify <change-id>   # run the cross-family review, then COMMIT the bound verdict edge")
	fmt.Fprintln(out, "  (override with AGENTOPS_VERIFY_PREPUSH_SKIP=1 only with cause — it disables the ratchet)")
	return fmt.Errorf("%d unverified commit(s)", len(violations))
}

// ledgerBlobAt returns the bytes of docs/provenance/ledger.jsonl AS COMMITTED in
// the tree of commit tip, with present=false when that tree carries no entry at
// the path (a legitimate pre-first-verdict state, not an error). The working
// tree is deliberately never consulted. Failures other than absence are errors
// the caller treats fail-closed. A committed SYMLINK at the path resolves to its
// target-string blob, which cannot parse as a valid chained ledger — so it fails
// the chain verify rather than smuggling content past the gate.
func ledgerBlobAt(repo, tip string) (content []byte, present bool, err error) {
	spec := tip + ":" + provenancegraph.LedgerRelativePath
	gitBin, err := trustedGit(repo)
	if err != nil {
		return nil, false, err // cannot trust any git — caller fail-closed
	}
	// Existence probe first: cat-file -e exits non-zero for a missing path in
	// the tree — legitimate absence, not a failure (the tip commit itself was
	// already proven to resolve by the caller).
	if probeErr := exec.Command(gitBin, "-C", repo, "cat-file", "-e", spec).Run(); probeErr != nil { // #nosec G204 -- gitBin is trusted-PATH-resolved; repo is the resolved git toplevel; tip is a push-supplied object id.
		return nil, false, nil
	}
	outStr, err := gitStdout(repo, "cat-file", "blob", spec)
	if err != nil {
		return nil, false, fmt.Errorf("read committed ledger blob %s: %w", spec, err)
	}
	return []byte(outStr), true, nil
}

// gitFirstParent returns the first-parent sha of commit sha, or "" when sha is a
// root commit (no parent — a legitimately empty base). A git error is returned
// so the caller can fail-closed rather than mistake a failure for "no parent".
func gitFirstParent(repo, sha string) (string, error) {
	out, err := gitStdout(repo, "rev-list", "--parents", "-n", "1", sha)
	if err != nil {
		return "", err
	}
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) < 2 { // fields[0] is sha itself; no parent => root commit
		return "", nil
	}
	return fields[1], nil
}

// ensureLedgerAppendOnly enforces that a #trivial-waived commit's change to the
// PROOF LEDGER is APPEND-ONLY relative to its parent: the ledger is the substrate
// that proves "no verdict = not done", so a waived (unreviewed) provenance-only
// commit may only add new rows, never delete/truncate/re-chain existing ones.
//
// The parent tree's ledger content must be an exact byte-PREFIX of sha's tree
// ledger content (JSONL append-only — existing rows immutable, new verdict rows
// only append). It prints a specific refusal and returns an error on violation,
// nil when append-only. Reads both blobs via trusted-git ledgerBlobAt. Cases:
//   - parent has NO ledger (first-ever verdict / no parent): empty prefix →
//     any valid new ledger is an append → OK.
//   - ledger present at parent, ABSENT at sha: DELETION → refuse.
//   - present at both, but base not a byte-prefix of tip: truncation / mid-file
//     edit / re-chain → refuse.
//   - base is a byte-prefix of tip: a pure append → OK (the tip-ledger
//     chain-verify already ran, so it is append-only AND chain-valid).
//
// Fail-closed: any git/read error refuses.
func ensureLedgerAppendOnly(out io.Writer, repo, sha string) error {
	tipBytes, tipPresent, err := ledgerBlobAt(repo, sha)
	if err != nil {
		fmt.Fprintf(out, "PUSH REFUSED: cannot read the proof ledger at %s — fail-closed: %v\n", short12(sha), err)
		return fmt.Errorf("read tip ledger at %s: %w", short12(sha), err)
	}
	parent, err := gitFirstParent(repo, sha)
	if err != nil {
		fmt.Fprintf(out, "PUSH REFUSED: cannot resolve the parent of %s to check the proof ledger — fail-closed: %v\n", short12(sha), err)
		return fmt.Errorf("resolve parent of %s: %w", short12(sha), err)
	}
	var baseBytes []byte
	basePresent := false
	if parent != "" {
		baseBytes, basePresent, err = ledgerBlobAt(repo, parent)
		if err != nil {
			fmt.Fprintf(out, "PUSH REFUSED: cannot read the proof ledger at the parent of %s — fail-closed: %v\n", short12(sha), err)
			return fmt.Errorf("read base ledger at %s: %w", short12(parent), err)
		}
	}
	if !basePresent {
		return nil // nothing to preserve — any tip ledger is an append over empty
	}
	if !tipPresent {
		fmt.Fprintf(out, "PUSH REFUSED: #trivial commit %s DELETES the proof ledger %s — a #trivial provenance-only commit may not delete the proof ledger (fail-closed).\n",
			short12(sha), provenancegraph.LedgerRelativePath)
		return fmt.Errorf("#trivial commit %s deletes the proof ledger", short12(sha))
	}
	if !bytes.HasPrefix(tipBytes, baseBytes) {
		fmt.Fprintf(out, "PUSH REFUSED: #trivial commit %s REWRITES the proof ledger %s — the #trivial waiver permits only appends to the proof ledger; base rows must be preserved verbatim (fail-closed).\n",
			short12(sha), provenancegraph.LedgerRelativePath)
		return fmt.Errorf("#trivial commit %s rewrites the proof ledger (not an append)", short12(sha))
	}
	return nil
}

// materializeLedger writes the committed ledger bytes to a private temp file so
// the provenancegraph readers (the SAME code 'ao provenance verify' runs) verify
// the tip's content verbatim. The caller must invoke cleanup.
func materializeLedger(content []byte) (string, func(), error) {
	f, err := os.CreateTemp("", "ao-prepush-ledger-*.jsonl")
	if err != nil {
		return "", func() {}, fmt.Errorf("materialize pushed ledger: %w", err)
	}
	path := f.Name()
	cleanup := func() { _ = os.Remove(path) }
	if _, wErr := f.Write(content); wErr != nil {
		_ = f.Close()
		cleanup()
		return "", func() {}, fmt.Errorf("materialize pushed ledger: %w", wErr)
	}
	if cErr := f.Close(); cErr != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("materialize pushed ledger: %w", cErr)
	}
	return path, cleanup, nil
}

// PARSING-DISCIPLINE SWEEP (age-rk3r.6): every verdict-edge / ledger / git-output
// recognition in the gate is exact-match and fail-closed; a near-miss is
// rejected, not accepted. Concretely:
//   - confirmedVerdictEdgeIn: exact relation (wasDerivedFrom) + shaBindsCommit
//     (hex-validated) + parseDisposition == "CONFIRMED" (exact token, never a
//     substring) — shared with the `ao done` recognizer.
//   - the #trivial waiver: git diff-tree --name-only -z (raw NUL-separated paths)
//   - the shared provenanceOnlyChangedFiles, so a leading-space / quoted /
//     surprising-byte path is matched exactly, never trimmed into the allowlist.
//   - splitSHAList / commitRange feed shas that any downstream match (shaBindsCommit,
//     git plumbing) re-validates, so junk is fail-closed (refused), not waived.
//   - isMainRef / isZeroSHA / refBranch / trunkRemoteRefs compare exact ref/sha
//     forms; the pre-push stdin is git-generated (not repo-controlled), and a line
//     with fewer than 4 fields is a blank/terminator and is skipped.
//
// parsePrePushLines parses git's pre-push stdin: each non-blank line is
// "local_ref local_sha remote_ref remote_sha". A line with <4 fields is a blank
// terminator (dropped); the first 4 fields are the ref/sha tuple (git never
// emits extras, so taking the first 4 never drops a real ref — the fail-closed
// direction, unlike requiring exactly 4 which would skip a line on any junk).
func parsePrePushLines(data string) []prePushLine {
	var lines []prePushLine
	for _, raw := range strings.Split(data, "\n") {
		f := strings.Fields(raw)
		if len(f) < 4 {
			continue
		}
		lines = append(lines, prePushLine{localRef: f[0], localSHA: f[1], remoteRef: f[2], remoteSHA: f[3]})
	}
	return lines
}

// isMainRef reports whether remoteRef is the trunk (main or master).
func isMainRef(remoteRef string) bool {
	return remoteRef == "refs/heads/main" || remoteRef == "refs/heads/master"
}

// isZeroSHA reports whether a sha is the all-zeros object id git uses for a
// deleted ref (any length: 40 for SHA-1, 64 for SHA-256).
func isZeroSHA(sha string) bool {
	return sha != "" && strings.Trim(sha, "0") == ""
}

// short12 renders at most the first 12 chars of a sha for messages.
func short12(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// errIndeterminateBase signals that a non-zero base absent from this clone has
// NO remote-tracking ref for the GATED TRUNK to derive the new-commit range
// against — the gate cannot tell which commits are new and refuses (fail-closed)
// rather than narrow to the tip (a deleted fail-open). checkMainPush renders the
// operator fix.
var errIndeterminateBase = errors.New("indeterminate base: cannot determine which commits are new")

// refBranch returns the branch name of a refs/heads/<branch> ref.
func refBranch(ref string) string {
	return strings.TrimPrefix(ref, "refs/heads/")
}

// commitRange returns the commits actually introduced by the push, oldest first.
// branch is the trunk being pushed to (from the push line's remote ref). An
// empty range (tip already at base) returns no commits. There is NO "check only
// the tip" path (round-7 convergence): every case either checks the true
// new-commit range or refuses — nothing silently narrows to the tip.
//
// Update push (base resolves locally): base..tip.
//
// CREATION push (base empty/all-zeros — first push of main / re-created branch)
// AND update push whose non-zero base is absent LOCALLY (shallow clone, GC'd
// base): the tip's history excluding commits already reachable from the GATED
// TRUNK's remote-tracking ref(s) — `git rev-list <tip> --not
// refs/remotes/*/<branch>` (round-10 correctness). Mechanism notes:
//   - "Already verified" means ON THE GATED TRUNK, not on any remote branch.
//     Excluding via `--not --remotes` (every remote branch) was a fail-open: an
//     unverified commit pushed to an UNGATED feature branch (origin/feature-x)
//     would ride onto main as already-verified. Only refs/remotes/*/<branch> —
//     the same-named branch on every remote (origin/main for the common single
//     remote) — is the gated trunk; a commit reachable only from a feature
//     branch is NEW to the trunk and IS checked.
//   - The gate's stdin-only contract carries no remote name (git passes it as
//     hook argv, which `ao verify pre-push` ignores), so ALL remotes' <branch>
//     refs are excluded rather than one remote's — a commit on SOME remote's
//     trunk landed through a gate (or predates the ratchet), and a stale/absent
//     tracking state only WIDENS the checked set (the fail-closed direction).
//   - `--not --all` would be wrong: the pushed tip is itself reachable from the
//     local branch being pushed, so --all empties the range (a fail-open).
//   - Size is NOT capped: correctness over speed; the caller emits a count line.
//
// A NON-ZERO base absent locally WITH NO trunk remote-tracking ref at all is
// indeterminate — deriving against nothing would re-check the whole history and
// falsely refuse long-landed commits, and narrowing to the tip is the deleted
// fail-open — so it returns errIndeterminateBase (refuse; the operator fetches
// the remote or pushes with an explicit base). A creation push (zero base) with
// no trunk ref is genuinely all-new history, so it legitimately checks everything.
func commitRange(repo, targetRemote, base, tip, branch string) ([]string, error) {
	if base != "" && !isZeroSHA(base) && gitCommitExists(repo, base) {
		outStr, err := gitStdout(repo, "rev-list", "--reverse", base+".."+tip)
		if err != nil {
			return nil, err
		}
		return splitSHAList(outStr), nil
	}
	trunk, err := trunkRemoteRefs(repo, targetRemote, branch)
	if err != nil {
		return nil, fmt.Errorf("resolve trunk remote-tracking refs for %s: %w", branch, err)
	}
	// New / mirror remote: ONLY a CREATION push (zero base) to a target with no trunk
	// ref means "establishing a mirror/backup remote from already-gated history" — a
	// brand-new remote has no established trunk to protect, so fall back to ANY
	// remote's same-branch trunk to grandfather ancient history (the mirror workflow).
	// An UPDATE push whose non-zero base is absent locally is deliberately NOT given
	// this fallback: globbing there would let an unverified commit sitting only on
	// ANOTHER remote's main be excluded from an ESTABLISHED trunk push (the backup/main
	// fail-open) — that path stays strict and hits errIndeterminateBase below (refuse;
	// fetch the remote or push with an explicit base).
	if len(trunk) == 0 && targetRemote != "" && (base == "" || isZeroSHA(base)) {
		trunk, err = trunkRemoteRefs(repo, "", branch)
		if err != nil {
			return nil, fmt.Errorf("resolve fallback trunk remote-tracking refs for %s: %w", branch, err)
		}
	}
	// Unknown non-zero base with no trunk ref to bound against: indeterminate.
	// (A creation/zero base with no trunk ref is genuinely all-new → check all.)
	if len(trunk) == 0 && base != "" && !isZeroSHA(base) {
		return nil, errIndeterminateBase
	}
	return revListNotTrunk(repo, tip, trunk)
}

// revListNotTrunk returns tip's commits (oldest first) that are not reachable
// from any trunkRef. With no trunkRefs it returns tip's whole history (a genuine
// first push of the trunk — all commits are new).
func revListNotTrunk(repo, tip string, trunkRefs []string) ([]string, error) {
	args := []string{"rev-list", "--reverse", tip}
	if len(trunkRefs) > 0 {
		args = append(args, "--not")
		args = append(args, trunkRefs...)
	}
	outStr, err := gitStdout(repo, args...)
	if err != nil {
		return nil, fmt.Errorf("derive new-commit range for %s: %w", short12(tip), err)
	}
	return splitSHAList(outStr), nil
}

// trunkRemoteRefs returns the remote-tracking refs for the trunk branch being
// pushed. When targetRemote is set (the installed hook always forwards git's
// remote name), it scopes to EXACTLY refs/remotes/<targetRemote>/<branch> — a
// commit already on ANOTHER remote's same-named branch (e.g. a backup remote's
// main) was never gated for THIS target, so excluding it would wave an unverified
// commit onto the target trunk (a refuted fail-open). When targetRemote is empty
// (a manual `ao verify pre-push` run, or a push-to-URL with no matching remote-
// tracking ref), it falls back to the branch across ALL remotes. Either way the
// branch is matched exactly (the whole ref path after the remote component), so a
// feature branch — even "feature/main" — is never mistaken for the trunk, and
// origin/HEAD is excluded.
func trunkRemoteRefs(repo, targetRemote, branch string) ([]string, error) {
	out, err := gitStdout(repo, "for-each-ref", "--format=%(refname)", "refs/remotes/")
	if err != nil {
		return nil, err
	}
	var refs []string
	for _, r := range strings.Split(strings.TrimSpace(out), "\n") {
		r = strings.TrimSpace(r)
		rest := strings.TrimPrefix(r, "refs/remotes/")
		if r == "" || rest == r {
			continue
		}
		slash := strings.IndexByte(rest, '/')
		if slash < 0 || rest[slash+1:] != branch {
			continue
		}
		if targetRemote != "" && rest[:slash] != targetRemote {
			continue // a different remote's trunk — not gated for this target
		}
		refs = append(refs, r)
	}
	return refs, nil
}

// splitSHAList splits rev-list output into its non-empty lines.
func splitSHAList(s string) []string {
	var commits []string
	for _, l := range strings.Split(strings.TrimSpace(s), "\n") {
		if l != "" {
			commits = append(commits, l)
		}
	}
	return commits
}

// hasConfirmedVerdictEdge reports whether the ledger at ledgerPath — in the gate,
// the materialized blob AS COMMITTED at the pushed tip, never the working-tree
// file — carries a CONFIRMED verdict edge bound to commit sha. A read error is
// fail-closed (false). Thin path-based wrapper over confirmedVerdictEdgeIn.
func hasConfirmedVerdictEdge(ledgerPath, sha string) bool {
	edges, err := provenancegraph.NewStore(ledgerPath).Read()
	if err != nil {
		return false
	}
	return confirmedVerdictEdgeIn(edges, sha)
}

// confirmedVerdictEdgeIn reports whether edges contains a CONFIRMED verdict edge
// bound to commit sha — the EXACT shape `ao provenance emit-verdict` writes and
// `ao done` (lookupDoneVerdicts) recognizes: relation=wasDerivedFrom,
// from_type=verdict, to_type=commit, to_id binding the sha, and an EXACT-TOKEN
// disposition == CONFIRMED. Recognition is EXACT and fail-closed (parsing sweep):
//
//   - Relation is required EXACTLY (wasDerivedFrom). A hash-valid row with a
//     different relation (e.g. wasAttributedTo) is NOT a verdict authorization.
//   - shaBindsCommit is the SHARED sha match (done path): both sides hex-valid,
//     >=7 chars, one a prefix of the other — a non-hex or too-short to_id is
//     rejected, not accepted.
//   - parseDisposition extracts the EXACT `disposition=<v>` whitespace-delimited
//     token; the check is v == "CONFIRMED", NEVER strings.Contains — so
//     "disposition=CONFIRMEDLY", "xdisposition=CONFIRMED", or an evidence_ref
//     that merely mentions the word does NOT authorize.
//
// This is COMMIT-scoped (not bead-scoped), matching the CI backstop
// (scripts/check-tip-verdict-ci.sh verdict_event_for): the gate certifies that
// the COMMIT was reviewed + CONFIRMED, whichever bead's work it carries — a
// CONFIRMED verdict on a commit means its diff passed an independent review.
// (The bead-scoping in lookupDoneVerdicts is a close-THIS-bead concern.) An
// empty/nil edge set (no committed ledger at the tip) confirms nothing.
//
// CONFIRMED authorizes directly; a REBOUND authorizes only through
// reboundVerdictAuthorizes (Go-side lineage + proof re-validation, age-rk3r.18) —
// this function is the CONFIRMED half and is deliberately exact about the
// disposition token so a REBOUND edge is NOT accepted here (it must earn its
// authorization by proving byte-equivalence to a reviewed commit).
func confirmedVerdictEdgeIn(edges []provenancegraph.Edge, sha string) bool {
	for _, e := range edges {
		if e.Relation != "wasDerivedFrom" || e.FromType != "verdict" || e.ToType != "commit" {
			continue
		}
		if !shaBindsCommit(sha, e.ToID) {
			continue
		}
		if parseDisposition(e.EvidenceRef) == doneStampConfirmed {
			return true
		}
	}
	return false
}

// doneStampRebound is the exact disposition token a REBOUND verdict edge carries
// in its evidence_ref ("pawl-verdict <bead> disposition=REBOUND", the shape
// emit-verdict writes for a rebind). Matched EXACTLY, never as a substring.
const doneStampRebound = "REBOUND"

// reboundEdgeBoundTo reports whether edges carries a REBOUND verdict edge bound
// to commit sha — the shape `ao provenance emit-verdict` writes for a rebind
// (relation=wasDerivedFrom, from_type=verdict, to_type=commit, an EXACT-TOKEN
// disposition==REBOUND, to_id binding sha). Exact + fail-closed, the same
// discipline as confirmedVerdictEdgeIn.
func reboundEdgeBoundTo(edges []provenancegraph.Edge, sha string) bool {
	for _, e := range edges {
		if e.Relation != "wasDerivedFrom" || e.FromType != "verdict" || e.ToType != "commit" {
			continue
		}
		if !shaBindsCommit(sha, e.ToID) {
			continue
		}
		if parseDisposition(e.EvidenceRef) == doneStampRebound {
			return true
		}
	}
	return false
}

// confirmedVerdictCommitSHAs returns the DISTINCT to_id commit shas of every
// CONFIRMED verdict edge in edges — the candidate REBOUND lineage roots. Each is
// a commit an independent cross-family review CONFIRMED, as committed in the
// pushed ledger. Order is first-seen (deterministic); duplicates collapse.
func confirmedVerdictCommitSHAs(edges []provenancegraph.Edge) []string {
	seen := map[string]bool{}
	var shas []string
	for _, e := range edges {
		if e.Relation != "wasDerivedFrom" || e.FromType != "verdict" || e.ToType != "commit" {
			continue
		}
		if parseDisposition(e.EvidenceRef) != doneStampConfirmed {
			continue
		}
		if e.ToID == "" || seen[e.ToID] {
			continue
		}
		seen[e.ToID] = true
		shas = append(shas, e.ToID)
	}
	return shas
}

// reboundVerdictAuthorizes reports whether a committed REBOUND verdict edge bound
// to commit sha authorizes the push — the Go twin of scripts/pawl-verdict.sh's
// REBOUND lineage gate (do_check REBOUND branch), for the hostile-repo-safe
// portable pre-push path (age-rk3r.18).
//
// WHY THE SHAPE DIFFERS FROM THE SHELL (load-bearing): the shell `check` reads a
// verdict JSON FILE that carries rebound_from_verdict / rebound_from_sha /
// patch_id_proof. Those fields live ONLY in that .agents/-gitignored file — they
// are NEVER projected into the COMMITTED ledger edge (emit-verdict writes only
// evidence_ref "pawl-verdict <bead> disposition=REBOUND" + to_id=tip). The gate's
// age-rk3r.6 trust boundary forbids reading the repo tree's .agents/ (or running
// any repo script), so the self-declared rebound_from_sha/proof are UNAVAILABLE
// and UNTRUSTED here. Instead the gate proves the SAME safety property straight
// from the committed ledger + trusted git:
//
//	A commit sha carrying a committed REBOUND edge is authorized IFF there exists
//	SOME committed CONFIRMED verdict edge bound to a DISTINCT commit R whose diff
//	is BYTE-EQUIVALENT to sha's — proven by RE-DERIVING, via trusted git, BOTH
//	git patch-id --stable AND the byte-exact content signature (the Go port,
//	commitPatchIDGit + commitContentSigGit) for BOTH commits and requiring an
//	EXACT match on BOTH keys.
//
// This is at least as strict as the shell path and closes the forge-a-rebound
// hole by construction: authorization requires a REAL, committed, CONFIRMED-
// reviewed commit that is byte-equivalent to the tip. It never reads or trusts
// the edge's (absent) patch_id_proof; a forged proof cannot help because nothing
// here consults it — the equivalence recomputes the reviewed commit's keys from
// git. Any failure (no REBOUND edge, no equivalent CONFIRMED lineage, unresolvable
// trusted git, empty/error signature) returns false → the caller falls through to
// the normal "no CONFIRMED = refuse" path (fail-closed).
//
// REACHABILITY (honest scoping, age-rk3r.18): this re-derivation requires the
// REVIEWED commit R to be resolvable in the local object store. At LOCAL PUSH time
// that holds — a just-rebased R is still reachable via the reflog / as a dangling
// object (pre-gc) — so the local pre-push gate honors REBOUND. The CI backstop
// (scripts/check-tip-verdict-ci.sh) runs in a CLEAN clone where a rebase-orphaned R
// is absent; it therefore cannot re-derive the equivalence and REFUSES fail-closed
// with a distinct message (never a false authorization). age-rk3r.19 tracks a
// keep-ref design that makes CI-REBOUND work with an orphaned reviewed commit.
//
// R != sha is required: a REBOUND descends from a DISTINCT prior reviewed commit
// (the whole point is re-binding across a rebase to a new sha). A CONFIRMED edge
// bound to sha itself is already handled by confirmedVerdictEdgeIn upstream, so
// skipping the self-sha here changes nothing and keeps the lineage semantics
// honest (a REBOUND that names its own commit as its lineage proves nothing).
func reboundVerdictAuthorizes(repo string, edges []provenancegraph.Edge, sha string) bool {
	if !reboundEdgeBoundTo(edges, sha) {
		return false
	}
	gitBin, err := trustedGit(repo)
	if err != nil {
		return false // cannot trust any git → cannot re-derive → fail-closed
	}
	// Re-derive the tip's keys ONCE.
	tipPID := commitPatchIDGit(gitBin, repo, sha)
	tipSig := commitContentSigGit(gitBin, repo, sha)
	if tipPID == "" || tipSig == "" {
		return false // cannot prove the tip's identity → fail-closed
	}
	for _, r := range confirmedVerdictCommitSHAs(edges) {
		// SECURITY (age-rk3r.18 refuter fix): the lineage to_id is fed to git as a
		// commit for the diff re-derivation, so it MUST be a HEX commit id resolved
		// as an OBJECT — never a revision expression. The direct-CONFIRMED path
		// (confirmedVerdictEdgeIn → shaBindsCommit) already requires this; the
		// REBOUND lineage MUST apply the identical discipline, or a crafted ledger
		// with a fake CONFIRMED edge to_id="HEAD~1" (a ref alias, not a hex id)
		// would let `git show HEAD~1` supply a matching diff and certify an
		// unreviewed tip (fail-open). hexCommitObjectID rejects any non-hex /
		// non-committish / ref-alias to_id, returning the resolved full oid ("" =
		// reject → skip this candidate).
		rOID := hexCommitObjectID(gitBin, repo, r)
		if rOID == "" {
			continue // non-hex / ref-alias / non-committish lineage to_id → not a valid lineage
		}
		if shaBindsCommit(sha, rOID) {
			continue // a CONFIRMED on the tip itself is the confirmedVerdictEdgeIn path, not lineage
		}
		rPID := commitPatchIDGit(gitBin, repo, rOID)
		rSig := commitContentSigGit(gitBin, repo, rOID)
		if rPID == "" || rSig == "" {
			continue // cannot prove this candidate's identity → not a valid lineage
		}
		// AUTHORITATIVE: the reviewed commit's patch-id AND byte-exact content
		// signature must BOTH equal the tip's — the same two-key equivalence the
		// shell requires (patch-id is whitespace/mode/newline-insensitive; the
		// content signature catches every diff-byte difference it misses).
		if rPID == tipPID && rSig == tipSig {
			return true
		}
	}
	return false
}

// trivialWaiver ports scripts/lib/trivial-waiver.sh: waive a commit from the
// verdict requirement ONLY when it carries an explicit #trivial marker AND every
// file it changed is under docs/provenance/. The marker must be a trailing tag at
// the END of the subject line or a standalone body trailer — a #trivial merely
// mentioned mid-prose does NOT waive (that was a real fail-open). #trivial is an
// author ASSERTION, so triviality is proven from the diff, not the message; an
// unreadable diff cannot prove triviality and is fail-closed (not waived).
//
// The path allowlist is the SHARED provenanceOnlyChangedFiles — the SAME
// discipline `ao done` uses (doneCommitProvenanceOnly). Routing both through one
// helper closes the parity gap: an earlier per-line strings.TrimSpace here
// trimmed a leading-space path " docs/provenance/x" INTO the allowlist and
// waived it (a fail-open the done path already guards — TestDoneProvenanceOnly_
// LeadingSpacePathNotWaived / TestPrePush_LeadingSpacePathNotWaived).
func trivialWaiver(repo, sha string) (bool, error) {
	subject := gitSubject(repo, sha)
	body := gitBody(repo, sha)
	if !trivialSubjectRE.MatchString(subject) && !trivialBodyRE.MatchString(body) {
		return false, nil // no marker → not waived (needs a verdict)
	}
	// --no-renames forces a rename to show as delete(old)+add(new) so a rename
	// FROM a non-provenance path INTO docs/provenance/ still exposes the old path.
	// -z emits raw, unquoted, NUL-separated paths so the allowlist check compares
	// exact bytes (parsing sweep) — see provenanceOnlyChangedFiles.
	changed, err := gitStdout(repo, "diff-tree", "--no-commit-id", "--no-renames", "--name-only", "-z", "-r", sha)
	if err != nil {
		return false, err // cannot prove triviality → fail-closed
	}
	return provenanceOnlyChangedFiles(changed), nil
}

// verifyLedgerChain runs the in-place hash-chain verification (the same
// provenancegraph.Store.VerifyFile that backs `ao provenance verify`) over the
// ledger AS COMMITTED at the pushed tip, materialized at ledgerPath. tipSHA
// labels the messages so a refusal names the commit whose tree failed. An empty
// ledger passes (Pass=true); a broken/tampered chain, or an unreadable
// materialization, refuses the push.
func verifyLedgerChain(ledgerPath, tipSHA string, out io.Writer) error {
	res, err := provenancegraph.NewStore(ledgerPath).VerifyFile()
	if err != nil {
		fmt.Fprintf(out, "PUSH REFUSED: cannot verify %s as committed at %s — fail-closed: %v\n",
			provenancegraph.LedgerRelativePath, short12(tipSHA), err)
		return err
	}
	if !res.Pass {
		fmt.Fprintf(out, "PUSH REFUSED: provenance ledger chain BROKEN/TAMPERED (as committed at %s) at ledger line %d: %s\n",
			short12(tipSHA), res.FirstBrokenLine, res.Message)
		return fmt.Errorf("provenance ledger chain broken at line %d", res.FirstBrokenLine)
	}
	return nil
}

// trustedGit resolves the git binary for gate/init operations against repo to an
// ABSOLUTE path on a SANITIZED PATH: empty/"."/relative entries and absolute
// entries INSIDE repo are excluded (trustedLookPath — the same rules as the pawl
// cold-env, pawlReviewColdEnv). The hook runs with cwd = the repo worktree, so a
// bare exec.Command("git") could otherwise resolve a repo-PLANTED git and let it
// forge rev-parse/cat-file/rev-list results into a silent pass — a fail-open on
// the gate's own "NO repo-tree code is trusted" boundary. Go's exec.ErrDot
// already refuses "."/relative PATH results; the live vector this closes is the
// absolute repo-internal PATH entry (e.g. a direnv-style $PWD/bin) plus any
// GODEBUG=execerrdot=0 environment.
//
// INVARIANT (convergence-by-deletion, age-rk3r.6 — the git twin of the hook's
// baked-ao rule): The gate trusts EXACTLY a git resolved from an absolute PATH
// entry OUTSIDE the repo. No PATH resolution of a repo-internal binary, no
// repo-relative, no repo-internal-absolute — an unresolvable trusted git REFUSES
// the push (runVerifyPrePush fail-closes up front; gitCommitExists/gitStdout
// propagate the failure to a refusal), it never falls back. This deletes the
// planted-binary class rather than patching each vector: the same stance the
// hook takes for ao (baked-only) it takes for git (sanitized-lookup-only).
//
// Ambient-env audit for the gate's exec calls: the binary resolution was the
// hole. GIT_DIR/GIT_WORK_TREE are git-handled and, under a pre-push hook, point
// at the very repo being pushed; the plumbing used here (rev-parse, cat-file,
// rev-list, log --format, diff-tree --name-only) invokes no external diff/pager
// helpers, so no other env-supplied executable is reachable.
func trustedGit(repo string) (string, error) {
	return trustedLookPath("git", repo)
}

// gitCommitExists reports whether ref resolves to a commit object in repo.
// An unresolvable trusted git is fail-closed (false — nothing can be proven).
func gitCommitExists(repo, ref string) bool {
	gitBin, err := trustedGit(repo)
	if err != nil {
		return false
	}
	return exec.Command(gitBin, "-C", repo, "rev-parse", "--verify", "--quiet", ref+"^{commit}").Run() == nil // #nosec G204 -- gitBin is trusted-PATH-resolved; repo is the resolved git toplevel; ref is a push-supplied object id.
}

// hexCommitObjectID validates that candidate is a HEX commit id (the exact
// discipline confirmedVerdictEdgeIn/shaBindsCommit applies to a bound to_id) and
// resolves it to its FULL commit object id — returning "" (reject) for anything
// that is not a hex, committish OBJECT. It is the fix for the age-rk3r.18 refuter
// fail-open: the REBOUND lineage to_id is fed to `git show` for the diff re-
// derivation, so a ledger-supplied REVISION EXPRESSION (e.g. "HEAD~1", a branch
// name, a tag, ":/msg") must never be treated as the reviewed commit — that would
// let a crafted CONFIRMED edge point its lineage at an arbitrary revision whose
// diff matches the tip and certify an unreviewed commit.
//
// Discipline, fail-closed at every step:
//   - candidate must be a HEX token of >= minShaPrefixLen chars (isHexToken +
//     length — the SAME predicate shaBindsCommit uses). This alone rejects every
//     revision expression, since "~", "^", ":", "/", "HEAD", branch/tag names all
//     carry non-hex bytes.
//   - it is resolved with `git rev-parse --verify --quiet <hex>^{commit}` via the
//     trusted git, so a non-existent or non-committish object rejects.
//   - DEFENSE-IN-DEPTH against a ref NAMED like a hex prefix: the resolved full
//     oid must BIND the input hex (shaBindsCommit — one a prefix of the other). A
//     branch "deadbeef" whose tip oid does not start with "deadbeef" is thereby
//     rejected even if git resolved the name; only a genuine object-id resolution
//     (oid has the hex as a prefix, or vice-versa) passes.
//
// Returns the resolved full oid on success, "" on any rejection.
func hexCommitObjectID(gitBin, repo, candidate string) string {
	if len(candidate) < minShaPrefixLen || !isHexToken(candidate) {
		return "" // not a hex commit id → reject (never treat as a revision)
	}
	out, err := exec.Command(gitBin, "-C", repo, "rev-parse", "--verify", "--quiet", candidate+"^{commit}").Output() // #nosec G204 -- gitBin is trusted-PATH-resolved; repo is the resolved git toplevel; candidate is hex-validated above (no revision metacharacters).
	if err != nil {
		return "" // non-existent / non-committish object → reject
	}
	oid := strings.TrimSpace(string(out))
	if oid == "" || !shaBindsCommit(oid, candidate) {
		return "" // resolved something that does not bind the hex (a ref alias) → reject
	}
	return oid
}

// gitStdout runs `git -C repo args...` and returns STDOUT only (with trailing
// newline) plus any error. Stdout-only is load-bearing here: stderr warnings must
// never contaminate a parsed SHA list or changed-file list. (Distinct from the
// package's CombinedOutput-based gitOutput in skills_edit.go.) git is resolved
// via trustedGit, never a bare PATH lookup from the repo cwd.
func gitStdout(repo string, args ...string) (string, error) {
	gitBin, err := trustedGit(repo)
	if err != nil {
		return "", err
	}
	full := append([]string{"-C", repo}, args...)
	out, err := exec.Command(gitBin, full...).Output() // #nosec G204 -- gitBin is trusted-PATH-resolved; repo is the resolved git toplevel; args are fixed verbs + push-supplied object ids.
	return string(out), err
}

// gitSubject returns the one-line subject of commit sha ("" on error).
func gitSubject(repo, sha string) string {
	out, _ := gitStdout(repo, "log", "-1", "--format=%s", sha)
	return strings.TrimSpace(out)
}

// gitBody returns the commit body of sha (NOT trimmed — body markers are
// matched multi-line), or "" on error.
func gitBody(repo, sha string) string {
	out, _ := gitStdout(repo, "log", "-1", "--format=%b", sha)
	return out
}

// truthyValue reports whether s is a truthy flag value (1/true/yes/y/on).
func truthyValue(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "y", "on":
		return true
	}
	return false
}
