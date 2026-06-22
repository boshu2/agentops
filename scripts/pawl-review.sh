#!/usr/bin/env bash
# pawl-review.sh — RUN the cross-family membrane review and, on CONFIRMED, write the
# pawl verdict. This is the missing executable half of the pawl: `pawl-verdict.sh`
# write/check is the verdict BOOKKEEPING, and pre-land-refuters is the DOCS — but
# actually running the fresh-context adversarial review (the thing that produces the
# verdict) was a manual codex-exec + prompt + parse + write dance repeated on every
# land. This makes it one command.
#
#   Producer (author)  = whatever made the change (--author-family, default claude).
#   Membrane (refuter) = `codex exec` — fresh-context, read-only, verdict-only. A
#                        DIFFERENT model family from a Claude author (cross-family by
#                        construction). LAW 0: NEVER `claude -p`; the refuter is codex.
#
# Flow: diff -> adversarial refuter prompt -> codex exec -> parse VERDICT -> evidence
#   - CONFIRMED (scope head): write + verify the commit-bound verdict (exit 0).
#   - CONFIRMED (scope staged): REVIEW-ONLY — print the verdict, write NOTHING (there is
#       no commit to bind), exit 0. Commit, then re-run with --scope head to certify.
#   - REFUTED:   print the defects + save them as the evidence file (for the author to
#                act on), write NO VERDICT, exit 3 (author must fix + re-run).
#
# Usage: pawl-review.sh <bead-id> [--scope head|staged] [--author-family <fam>] [--context "<extra>"]
# Exit:  0 CONFIRMED(+written for head) · 3 REFUTED · 2 usage/precondition · 1 hard error.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PAWL="$SCRIPT_DIR/pawl-verdict.sh"
# The repo UNDER REVIEW (overridable for tests / alt worktrees); PAWL itself is always
# the real script next to this one.
REPO_ROOT="${AGENTOPS_REPO_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}"
VERDICT_DIR="${AGENTOPS_PAWL_VERDICT_DIR:-$REPO_ROOT/.agents/pawl-verdicts}"
EVIDENCE_DIR="$REPO_ROOT/.agents/pawl-evidence"
PR="${AGENTOPS_PAWL_PR:-0}"          # 0 = push-to-main landing (matches the pre-push gate)
TIMEOUT="${PAWL_REVIEW_TIMEOUT:-300}"

bead=""; scope="head"; extra=""; author_family="claude"
need_val() { [[ -n "${2:-}" ]] || { echo "pawl-review: $1 needs a value" >&2; exit 2; }; }
while [[ $# -gt 0 ]]; do
  case "$1" in
    --scope)         need_val "$1" "${2:-}"; scope="$2"; shift 2 ;;
    --author-family) need_val "$1" "${2:-}"; author_family="$2"; shift 2 ;;
    --context)       need_val "$1" "${2:-}"; extra="$2"; shift 2 ;;
    -h|--help)       sed -n '2,24p' "$0"; exit 0 ;;
    -*)              echo "pawl-review: unknown flag $1" >&2; exit 2 ;;
    *)               bead="$1"; shift ;;
  esac
done
[[ -n "$bead" ]] || { echo "pawl-review: need <bead-id>" >&2; exit 2; }
[[ -x "$PAWL" ]] || { echo "pawl-review: $PAWL not executable" >&2; exit 1; }
command -v codex >/dev/null 2>&1 || { echo "pawl-review: codex (the cross-family refuter) not on PATH" >&2; exit 1; }

# The codex refuter is model family 'gpt' (codex|openai|gpt). A same-family AUTHOR
# would make this a SAME-family review (shared blind spots), not the cross-family
# check this command exists to provide — refuse it (use a different-family author, or
# a different reviewer for codex-authored work). Defends a same-family false-CONFIRMED.
# Case-INSENSITIVE + substring so Codex / GPT / openai-gpt cannot bypass the guard.
af_lc="$(printf '%s' "$author_family" | tr '[:upper:]' '[:lower:]')"
case "$af_lc" in
  *gpt*|*codex*|*openai*)
    echo "pawl-review: --author-family '$author_family' is the SAME model family (gpt/codex/openai) as the codex refuter — that is a same-family review, not the cross-family pawl this command provides. Review codex-authored work with a different-family reviewer." >&2
    exit 2 ;;
esac

# Pick a timeout wrapper; DEFAULT macOS ships no `timeout` (it is coreutils' gtimeout
# or absent), so degrade to running codex with no timeout rather than failing closed
# and being unusable on bo-mac. (defends defect: timeout not portable)
TIMEOUT_CMD=()
if command -v timeout >/dev/null 2>&1; then TIMEOUT_CMD=(timeout "$TIMEOUT")
elif command -v gtimeout >/dev/null 2>&1; then TIMEOUT_CMD=(gtimeout "$TIMEOUT")
else echo "pawl-review: no timeout/gtimeout found — running codex without a timeout (brew install coreutils for one)" >&2
fi
run_review() {
  if [[ "${#TIMEOUT_CMD[@]}" -gt 0 ]]; then
    "${TIMEOUT_CMD[@]}" codex exec --sandbox read-only < "$prompt_file" > "$raw_file" 2>&1
  else
    codex exec --sandbox read-only < "$prompt_file" > "$raw_file" 2>&1
  fi
}

case "$scope" in
  head)   diff="$(git -C "$REPO_ROOT" show HEAD --no-color 2>/dev/null)" ;;
  staged) diff="$(git -C "$REPO_ROOT" diff --cached --no-color 2>/dev/null)" ;;
  *) echo "pawl-review: --scope must be head|staged" >&2; exit 2 ;;
esac
head="$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null)"
[[ -n "$diff" ]] || { echo "pawl-review: empty diff for scope=$scope — nothing to review" >&2; exit 2; }
[[ -n "$head" && "${#head}" -ge 7 ]] || { echo "pawl-review: cannot resolve HEAD sha" >&2; exit 1; }

mkdir -p "$EVIDENCE_DIR"
ctx="codex-fresh-${bead}-$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null)"
evidence="$EVIDENCE_DIR/${bead}-pawl-review.txt"
prompt_file="$(mktemp "${TMPDIR:-/tmp}/pawl-review-prompt.XXXXXX")"
raw_file="$(mktemp "${TMPDIR:-/tmp}/pawl-review-raw.XXXXXX")"
trap 'rm -f "$prompt_file" "$raw_file"' EXIT

# The refuter prompt: adversarial, evidence-only, verdict-shaped. Diff is appended from
# a FILE (never shell-interpolated) so $(...) / backticks in the diff are not evaluated.
{
  cat <<PROMPT
You are a fresh-context, cross-family adversarial reviewer — the REFUTER in a two-model pawl. Do NOT use
tools. Do NOT read files. Review ONLY the diff below and reply with a verdict. Your default posture is
skepticism: actively try to REFUTE that this change is correct and safe to land on main. Look for real
defects — logic bugs, fail-open holes, data loss, races, missing edge cases, broken contracts, tests that
would pass even if the code were wrong, over-claims in docs. A plausible-but-wrong change must be REFUTED.
${extra:+
EXTRA CONTEXT FROM THE AUTHOR:
$extra
}
Reply with EXACTLY this shape and nothing else:
VERDICT: CONFIRMED
   -- or --
VERDICT: REFUTED
DEFECTS:
 - <one concrete defect per line: the symptom and why it matters>

=== DIFF UNDER REVIEW (bead $bead, scope $scope, head ${head:0:12}) ===
PROMPT
  printf '%s\n' "$diff"
} > "$prompt_file"

# Run the refuter, CAPTURING the exit status: a timeout/crash must NOT be trusted as a
# clean review (a partial output containing 'VERDICT: CONFIRMED' from a killed run could
# otherwise write a passing verdict — fail-open). Retry once on a flat 0-byte stall.
echo "pawl-review: running cross-family (codex) review of $scope diff for $bead (head ${head:0:12})…" >&2
codex_rc=0
run_review || codex_rc=$?
if [[ ! -s "$raw_file" ]]; then
  echo "pawl-review: codex produced no output (stall) — retrying once…" >&2
  codex_rc=0
  run_review || codex_rc=$?
fi

# Codex prints a 'codex' marker line before its answer; drop the marker itself and keep
# what follows (evidence starts at the reviewer's content, not the marker). Fall back to
# the full output if there is no marker.
verdict_block="$(awk '/^codex$/{c=1; next} c' "$raw_file" 2>/dev/null)"
[[ -n "$verdict_block" ]] || verdict_block="$(cat "$raw_file")"
printf '%s\n' "$verdict_block" > "$evidence"
[[ -s "$evidence" ]] || { echo "pawl-review: no reviewer output captured — fail-closed" >&2; exit 1; }

# Decide on the FINAL verdict-shaped line only: the reviewer's real answer comes LAST,
# after any preamble or echoed prompt template — an echoed "VERDICT: CONFIRMED" from the
# instructions must not be mistaken for the verdict. (defends a quoted/multi-verdict
# false-CONFIRMED)
final_verdict="$(grep -iE '^[[:space:]]*VERDICT:[[:space:]]*(CONFIRMED|REFUTED)' "$evidence" | tail -1)"
if grep -qiE 'REFUTED' <<<"$final_verdict"; then
  echo "=== PAWL REVIEW: REFUTED — defects below (fix, recommit, re-run; NO verdict written) ===" >&2
  sed -n '/^[[:space:]]*VERDICT:[[:space:]]*REFUTED/,$p' "$evidence" >&2
  exit 3
fi
if ! grep -qiE 'CONFIRMED' <<<"$final_verdict"; then
  echo "pawl-review: reviewer's FINAL line is not a clear VERDICT: CONFIRMED|REFUTED — fail-closed. Raw output in $evidence" >&2
  exit 1
fi
# A CONFIRMED is only trustworthy from a CLEANLY-EXITED reviewer run (defends defect #3).
if [[ "$codex_rc" -ne 0 ]]; then
  echo "pawl-review: reviewer exited non-zero ($codex_rc, e.g. timeout 124) — refusing to trust a CONFIRMED from an incomplete run — fail-closed" >&2
  exit 1
fi

# scope=staged is REVIEW-ONLY: the reviewed change is not committed, so there is no
# object to commit-bind a verdict to. Print the result; do NOT write (defends defect #1).
if [[ "$scope" == "staged" ]]; then
  echo "pawl-review: CONFIRMED (review-only: scope=staged has no commit to bind). Commit, then re-run with --scope head to certify." >&2
  exit 0
fi

# scope=head, CONFIRMED, clean run — write the commit-bound verdict + verify it passes
# the same check the pre-push gate runs. context_id differs from --author-context
# (fresh-context floor). Absolute evidence path so check resolves it regardless of which
# repo root the checker uses.
"$PAWL" write "$bead" "$PR" \
  --disposition CONFIRMED --head "$head" \
  --author-context "author-${author_family}-${bead}" --author-family "$author_family" \
  --refuter "codex:CONFIRMED:${ctx}:${evidence}" \
  --dir "$VERDICT_DIR" >/dev/null || { echo "pawl-review: verdict write failed" >&2; exit 1; }

if "$PAWL" check "$bead" "$PR" --dir "$VERDICT_DIR" --head "$head" >&2; then
  echo "pawl-review: CONFIRMED + verdict written + verified for $bead @ ${head:0:12} — ready to push." >&2
  exit 0
fi
echo "pawl-review: verdict written but the check did not pass (see above) — fail-closed" >&2
exit 1
