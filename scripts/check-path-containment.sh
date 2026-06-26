#!/usr/bin/env bash
# check-path-containment.sh — warn-only ratchet (age-0dq9.6 / audit M-2).
#
# Surfaces filepath.Join calls in non-test Go whose arguments come from a
# HIGH-CONFIDENCE external-input source, so a reviewer can confirm each routes
# through the canonical path containment (filepath.EvalSymlinks + filepath.Rel,
# the pattern cli/internal/{worktree,resolver} already use). It does NOT try to
# decide containment itself (see FAIL-CLOSED below) — it flags every such Join
# for review, contained or not.
#
# SPELLING LIMITATION (accepted): it matches the canonical literal `filepath.Join(`
# call. Aliased imports (`fp "path/filepath"` then `fp.Join(...)`), `path.Join`,
# and Join wrapped inside a helper are out of scope — an accepted gap for a
# warn-only ratchet, not a guarantee. Catching arbitrary spellings would require
# a Go AST/type pass, which is a deliberate non-goal here.
#
# SCOPE / HONEST LIMITATION: this is a heuristic FORWARD ratchet scoped to a
# narrow set of explicit taint markers (CLI args, web request fields, and
# variables explicitly named *untrusted/external/remote/requested/userSupplied/
# userInput*Path). It is NOT a sound taint analysis — it cannot follow arbitrary
# data flows from corpus/bead/remote *content* into a Join. Its job is to make a
# NEW instance of the named anti-pattern visible in review without
# false-positiving the ~6k existing internal-only Join sites (the current tree
# flags zero, by construction — verified by the bats fixture's clean case).
#
# FAIL-CLOSED by design: it does NOT try to auto-clear a flagged Join when some
# containment call happens to be nearby. A proximity check ("EvalSymlinks within
# N lines ⇒ safe") would FAIL OPEN — an unrelated nearby filepath.Rel/EvalSymlinks
# could silently suppress a genuinely unsafe Join. So every external-input Join
# is surfaced and a human confirms it routes through the canonical containment.
#
# Exit code: ALWAYS 0 (warn-only, per the warn-then-fail-ratchet convention;
# see scripts/check-paths-resolver-coverage.sh). Flipping to blocking is a
# separate decision after a baseline period.
#
# ACCEPTANCE (age-0dq9.6): flags an injected unsafe Join; clean tree flags zero;
# covered by tests/scripts/check-path-containment.bats.
set -uo pipefail

REPO_ROOT="${AGENTOPS_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
cd "$REPO_ROOT" 2>/dev/null || exit 0

# Directory to scan (overridable for tests).
SCAN_ROOT="${PATH_CONTAINMENT_SCAN_ROOT:-cli}"

# High-confidence external-input markers. Deliberately narrow so the current
# tree flags zero; widen only with evidence. Covers CLI input (os.Args, flag.Arg,
# cobra positional `args[...]`), web request fields, and variables explicitly
# named *untrusted/external/remote/requested/userSupplied/userInput*Path.
#
# NOT a target (and intentionally not matched): an external value used as the
# Join BASE directory with only literal children, e.g.
# `filepath.Join(os.Getenv("HOME"), ".agentops", "config.yaml")` — a fixed base +
# constant segments cannot traverse. The containment risk is external input used
# as an APPENDED segment (which can carry `../`); this heuristic does not parse
# argument position, so it leans on the named-marker set above rather than
# flagging every `os.Getenv`/`cwd` base. It is a ratchet signal, not a proof.
TAINT_RE='os\.Args|flag\.Arg|args\[|\.FormValue\(|\.Query\(|r\.URL|req\.URL|request\.[A-Za-z]|(untrusted|external|remote|requested|userSupplied|userInput)[A-Za-z]*[Pp]ath'

# Safety cap on how many lines a single Join call may span before we stop
# reading (prevents runaway on a malformed/never-closing paren).
MAX_SPAN=60

# join_call_region FILE START_LINE → emits the full filepath.Join(...) call,
# from START_LINE until its parentheses balance (so a multi-line Join whose
# external-input arg is on a later line is NOT missed — closing the fail-open
# hole of a fixed line window).
join_call_region() {
  awk -v s="$2" -v cap="$MAX_SPAN" '
    NR < s { next }
    NR > s + cap { exit }
    {
      print
      o = gsub(/\(/, "(")
      c = gsub(/\)/, ")")
      depth += o - c
      if (depth <= 0) { exit }
    }
  ' "$1"
}

flagged=0

# Non-test Go files containing a filepath.Join call.
while IFS= read -r f; do
  [ -n "$f" ] || continue
  # Each line where a filepath.Join call begins.
  while IFS=: read -r ln _; do
    [ -n "$ln" ] || continue
    # Read the WHOLE Join call (balanced parens). If any of its arguments
    # reference an external-input marker, flag it for manual containment review.
    # No proximity "looks contained" suppression — that would fail open.
    if join_call_region "$f" "$ln" | grep -Eq "$TAINT_RE"; then
      echo "WARN path-containment: ${f}:${ln} — filepath.Join on an external-input segment; confirm it routes through EvalSymlinks+filepath.Rel containment"
      flagged=$(( flagged + 1 ))
    fi
  done < <(grep -nE 'filepath\.Join\(' "$f" 2>/dev/null || true)
done < <(grep -rlE 'filepath\.Join\(' --include='*.go' "$SCAN_ROOT" 2>/dev/null | grep -vE '_test\.go$' || true)

echo "path-containment ratchet: ${flagged} external-input Join site(s) to review for containment (warn-only)."
exit 0
