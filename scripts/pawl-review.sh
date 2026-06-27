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
# A maximal-adversarial refuter over LLM prose has an infinite false-alarm tail. So a
# clean adversarial run RECORDS LINEAGE (.agents/pawl-review/<bead>.adversarial.json: the
# reviewed diff-hash), and --converge switches to the CALIBRATED real-safety bar ("any
# REMAINING real fail-open/data-loss/wrong-object defect? parse-tail accepted") to converge
# a heuristic-tail change. --converge is LINEAGE-GATED (council C, age-cwo.8): it writes a
# verdict ONLY if a prior ADVERSARIAL run covered the IDENTICAL diff; no/changed lineage ->
# ADVISORY-ONLY (exit 4), so the adversarial pass can never be skipped (no gate-weakening).
#
# Usage: pawl-review.sh <bead-id> [--scope head|staged] [--converge] [--author-family <fam>] [--context "<extra>"]
# Exit:  0 CONFIRMED(+written for head) · 3 REFUTED · 4 --converge advisory-only (no lineage) · 2 usage/precondition · 1 hard error.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PAWL="$SCRIPT_DIR/pawl-verdict.sh"
# The standing-pawl service script (overridable for tests). Always the real script next
# to this one — NOT the repo-under-review's (they differ for alt worktrees). (ml8.7)
PAWL_SH="${PAWL_SERVICE_SCRIPT:-$SCRIPT_DIR/pawl.sh}"
# The repo UNDER REVIEW (overridable for tests / alt worktrees); PAWL itself is always
# the real script next to this one.
REPO_ROOT="${AGENTOPS_REPO_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}"
VERDICT_DIR="${AGENTOPS_PAWL_VERDICT_DIR:-$REPO_ROOT/.agents/pawl-verdicts}"
EVIDENCE_DIR="$REPO_ROOT/.agents/pawl-evidence"

# emit_pawl_catch <mode>: record this REFUTE as a structured membrane catch (epic
# age-zpj5, S2) so DetectCatches/recall can group recurring classes — domain from
# the changed-files' top dir, reason from the REFUTED verdict text, paths from the
# reviewed files. Fail-safe + NON-BLOCKING: a missing `ao` or any error never blocks
# the REFUTED exit (the catch is observability, not a gate). Reads the globals
# (bead/head/evidence/review_files) at call time.
emit_pawl_catch() {
  local mode="${1:-fresh-context}" ao_bin reason domain paths files
  local -a catch_args=()
  ao_bin="$(command -v ao 2>/dev/null)" || return 0
  [[ -n "${bead:-}" && -n "${head:-}" ]] || return 0
  reason="$(grep -iE '^[[:space:]]*VERDICT:[[:space:]]*REFUTED' "${evidence:-/dev/null}" 2>/dev/null | tail -1 \
            | sed -E 's/^[[:space:]]*VERDICT:[[:space:]]*REFUTED[[:space:]:—-]*//I' | cut -c1-200)"
  [[ -n "$reason" ]] || reason="pawl-review REFUTED for $bead (see evidence)"
  # Changed files for affected_paths — computed DIRECTLY from git by scope, NOT from
  # $review_files (which pawl-review only populates for LARGE diffs > MAX_INLINE_BYTES),
  # so a NORMAL small-diff catch is still path-recallable.
  case "${scope:-head}" in
    staged) files="$(git -C "${REPO_ROOT:-.}" diff --cached --name-only --no-color 2>/dev/null | sed '/^$/d')" ;;
    *)      files="$(git -C "${REPO_ROOT:-.}" show HEAD --name-only --format= --no-color 2>/dev/null | sed '/^$/d')" ;;
  esac
  domain="$(printf '%s\n' "$files" | head -1 | cut -d/ -f1)"
  [[ -n "$domain" ]] || domain="pawl-review"
  paths="$(printf '%s\n' "$files" | head -20 | tr '\n' ',' | sed 's/,$//')" # portable join (BSD paste -sd is unreliable)
  [[ -n "$paths" ]] && catch_args=(--paths "$paths")
  # Run from $REPO_ROOT (the REVIEWED repo): `ao membrane catch` roots its ledger via
  # repoRootOrCwd() from its cwd, so without this a pawl-review invoked from a different
  # cwd / another repo (AGENTOPS_REPO_ROOT=repoA, cwd in repoB) would write the catch to
  # the WRONG .agents/yield and recall in the reviewed repo would never see it. Subshell
  # cd so the rest of the script's cwd is untouched.
  ( cd "${REPO_ROOT:-.}" && "$ao_bin" membrane catch --bead "$bead" --domain "$domain" \
      --reason "$reason" --head "$head" --mode "$mode" "${catch_args[@]}" ) >/dev/null 2>&1 || true
}
PR="${AGENTOPS_PAWL_PR:-0}"          # 0 = push-to-main landing (matches the pre-push gate)
TIMEOUT="${PAWL_REVIEW_TIMEOUT:-300}"
# age-mwhj: above this packet-byte cap, inlining the full diff reliably times the review out
# (a 41KB packet timed an opus pane out; a 62KB cold codex was killed) and then fails CLOSED —
# safe but unusable on exactly the large refactors/generated churn where review matters most.
MAX_INLINE_BYTES="${PAWL_MAX_INLINE_BYTES:-24576}"   # ~24KB

bead=""; scope="head"; extra=""; author_family="claude"; converge=0
need_val() { [[ -n "${2:-}" ]] || { echo "pawl-review: $1 needs a value" >&2; exit 2; }; }

# age-mwhj: assemble the review body. At/below the byte cap, the full inline diff (unchanged).
# Above it, a READ-FILES-NOT-INLINE body — a size note + git --stat + the changed-file ABSOLUTE
# paths — and the reviewer reads the files directly (read-only) instead of choking on a huge
# inline blob. Pure (no git of its own): the caller passes the stat + newline-separated file list,
# so this is unit-testable. Echoes the body.
build_review_body() {
  local diff="$1" max="$2" stat="$3" files="$4" root="$5" bytes f
  bytes="$(printf '%s' "$diff" | wc -c | tr -d ' ')"
  if [ "$bytes" -le "$max" ]; then printf '%s' "$diff"; return 0; fi
  printf 'NOTE: this change is LARGE (%s bytes > %s inline cap) — the ADDED lines are NOT inlined.\n' "$bytes" "$max"
  printf 'READ THE CHANGED FILES DIRECTLY (read-only) at the absolute paths below for the full ADDED content.\n'
  printf 'The diff STRUCTURE (file headers, @@ hunks, and ALL DELETED/removed lines — which you CANNOT\n'
  printf 'recover by reading the current files) is shown inline below, with long added blocks elided.\n\n'
  printf '=== git --stat ===\n%s\n\n' "$stat"
  printf '=== diff: deletions + structure (added content elided — read the files for it) ===\n'
  # Drop ONLY added-content lines (^+ but not the ^+++ file header); keep deletions (^-), hunk
  # headers (@@), file headers, and context — so removed code is never lost (reading the CURRENT
  # file cannot show what was deleted; that was the cross-family review's read-files fidelity catch).
  printf '%s\n' "$diff" | grep -vE '^\+([^+]|$)' || true
  printf '\n=== changed files (read each for the added content) ===\n'
  while IFS= read -r f; do [ -n "$f" ] && printf '  %s/%s\n' "$root" "$f"; done <<< "$files"
  return 0   # the trailing `while read` / grep exits non-zero at EOF — the function itself succeeded
}

# age-bb5l: a human phrase for the tier a routed verdict actually ACHIEVED, so the review/push
# surfaces stop hardcoding "opus+codex duel" (a lie on a single-family or non-opus+codex route).
# multi-model = the real cross-family gate; fresh-context = a SINGLE family (weaker) and carries
# the "add a 2nd family" nudge so a fresh-context land is a conscious choice, not silent.
pawl_tier_note() {
  case "$1" in
    multi-model)   printf 'multi-model cross-family' ;;
    fresh-context) printf 'fresh-context — a SINGLE family, WEAKER than the cross-family gate; add codex or agy and re-run for a multi-model verdict' ;;
    *)             printf '%s' "${1:-unknown-tier}" ;;
  esac
}

# Source-guard: tests source this file to exercise build_review_body / pawl_tier_note; the
# codex-running flow below only executes when the script is run directly.
[ "${BASH_SOURCE[0]:-$0}" = "${0}" ] || return 0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --scope)         need_val "$1" "${2:-}"; scope="$2"; shift 2 ;;
    --author-family) need_val "$1" "${2:-}"; author_family="$2"; shift 2 ;;
    --context)       need_val "$1" "${2:-}"; extra="$2"; shift 2 ;;
    --converge)      converge=1; shift ;;
    -h|--help)       sed -n '2,30p' "$0"; exit 0 ;;
    -*)              echo "pawl-review: unknown flag $1" >&2; exit 2 ;;
    *)               bead="$1"; shift ;;
  esac
done
[[ -n "$bead" ]] || { echo "pawl-review: need <bead-id>" >&2; exit 2; }
# --converge (the calibrated real-safety bar) certifies a COMMIT, so it requires
# --scope head; and it is lineage-gated below (age-cwo.8 / council C).
[[ "$converge" -eq 1 && "$scope" != "head" ]] && { echo "pawl-review: --converge requires --scope head (it certifies a commit)" >&2; exit 2; }
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

# age-mwhj: choose inline vs read-files-not-inline by packet size. Above the cap, the reviewer
# (cold codex --sandbox read-only OR the warm panes) reads the changed files directly.
diff_bytes="$(printf '%s' "$diff" | wc -c | tr -d ' ')"
review_stat=""; review_files=""
if [[ "$diff_bytes" -gt "$MAX_INLINE_BYTES" ]]; then
  case "$scope" in
    head)   review_stat="$(git -C "$REPO_ROOT" show HEAD --stat --format= --no-color 2>/dev/null)"; review_files="$(git -C "$REPO_ROOT" show HEAD --name-only --format= --no-color 2>/dev/null | sed '/^$/d')" ;;
    staged) review_stat="$(git -C "$REPO_ROOT" diff --cached --stat --no-color 2>/dev/null)"; review_files="$(git -C "$REPO_ROOT" diff --cached --name-only --no-color 2>/dev/null | sed '/^$/d')" ;;
  esac
fi
review_body="$(build_review_body "$diff" "$MAX_INLINE_BYTES" "$review_stat" "$review_files" "$REPO_ROOT")"
if [[ "$diff_bytes" -gt "$MAX_INLINE_BYTES" ]]; then
  read_instr="This change is LARGE and is NOT inlined. READ the changed files listed below directly (read-only); they are the change under review. Do not modify anything."
  echo "pawl-review: large diff (${diff_bytes}B > ${MAX_INLINE_BYTES}B cap) — packet uses read-files-not-inline (age-mwhj)" >&2
else
  read_instr="Do NOT use tools. Do NOT read files. Review ONLY the change below and reply with a verdict."
fi

# Lineage key: the content hash of the reviewed diff. The adversarial run records it;
# --converge requires a prior adversarial run on the IDENTICAL diff (no fuzzy "material
# change" — an exact-hash match, which is unambiguous and safe). (age-cwo.8)
PAWL_REVIEW_DIR="$REPO_ROOT/.agents/pawl-review"
lineage_file="$PAWL_REVIEW_DIR/${bead}.adversarial.json"
if command -v shasum >/dev/null 2>&1; then diff_hash="$(printf '%s' "$diff" | shasum -a 256 | cut -d' ' -f1)"
else diff_hash="$(printf '%s' "$diff" | sha256sum | cut -d' ' -f1)"; fi

mkdir -p "$EVIDENCE_DIR"
ctx="codex-fresh-${bead}-$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null)"
evidence="$EVIDENCE_DIR/${bead}-pawl-review.txt"
prompt_file="$(mktemp "${TMPDIR:-/tmp}/pawl-review-prompt.XXXXXX")"
raw_file="$(mktemp "${TMPDIR:-/tmp}/pawl-review-raw.XXXXXX")"
trap 'rm -f "$prompt_file" "$raw_file"' EXIT

# Posture: ADVERSARIAL (default, max catch) vs the CALIBRATED real-safety bar (--converge,
# the bounded land-question for converging a heuristic-tail change AFTER its real defects
# were already found by an adversarial pass — lineage-gated below). (age-cwo.8 / council C)
if [[ "$converge" -eq 1 ]]; then
  posture="This is a CALIBRATED real-safety CONVERGENCE pass: the change ALREADY went through a maximal-adversarial review and its real defects were fixed. Answer ONLY the BOUNDED question: is there any REMAINING REAL SAFETY defect — a concrete path that (a) writes/certifies something it should NOT (a fail-open), (b) loses or corrupts data, or (c) targets the wrong object? Novel theoretical or parse edge-cases, cosmetic wording, and 'a producer could output something weirder' are the ACCEPTED TAIL and are NOT grounds to refute. REFUTE ONLY for a concrete real-safety defect."
else
  posture="Your default posture is skepticism: actively try to REFUTE that this change is correct and safe to land on main. Look for real defects — logic bugs, fail-open holes, data loss, races, missing edge cases, broken contracts, tests that would pass even if the code were wrong, over-claims in docs. A plausible-but-wrong change must be REFUTED."
fi

# The refuter prompt: posture + diff. Diff is appended from a FILE (never shell-
# interpolated) so $(...) / backticks in the diff are not evaluated.
{
  cat <<PROMPT
You are a fresh-context, cross-family reviewer — the REFUTER in a two-model pawl. $read_instr
$posture
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

=== CHANGE UNDER REVIEW (bead $bead, scope $scope, head ${head:0:12}) ===
PROMPT
  printf '%s\n' "$review_body"
} > "$prompt_file"

# Lazy auto-start (the "membrane is never silently cold again" fix): when routing is
# eligible (head scope, not --converge, not opted out) but the standing service is DOWN,
# stand it up ONCE here so this and every later review in the session run WARM through the
# tri-model duel instead of paying a cold codex-exec each time (the gap that left ~13 reviews
# cold in a single session). One-time ~couple-min warmup; fail-safe — if `up` fails the
# health check below stays false and we fall through to the cold path. Opt out:
# PAWL_NO_SERVICE=1 (disable the whole service path) or PAWL_NO_AUTOUP=1 (route-if-up only).
if [[ "$converge" -eq 0 && "$scope" == "head" && "${PAWL_NO_SERVICE:-0}" != "1" && "${PAWL_NO_AUTOUP:-0}" != "1" ]] \
   && ! bash "$PAWL_SH" health >/dev/null 2>&1; then
  echo "pawl-review: standing pawl-service not up — starting it once (warm cross-family pawl-service)…" >&2
  bash "$PAWL_SH" up >&2 || echo "pawl-review: pawl up failed — falling through to cold codex-exec" >&2
fi

# ml8.7 + tri-model: route the DEFAULT adversarial pawl through the standing pawl-service (the
# warm opus+codex+agy DUEL) instead of a cold per-pawl `codex exec`, when a healthy service is
# up. The routed verdict is STRONGER (multi-model agreement vs codex-only fresh-context) and
# warm (no per-bead subprocess spin-up — the anti-pattern this deprecates). Fail SAFE: any
# routing error falls through to the codex-exec path below (never fail-open). --converge
# (lineage-gated, bounded, codex-only) AND --scope staged (REVIEW-ONLY, no commit to bind —
# routing would wrongly write a HEAD-bound verdict for an uncommitted diff) stay on the cold
# path. Opt out: PAWL_NO_SERVICE=1.
if [[ "$converge" -eq 0 && "$scope" == "head" && "${PAWL_NO_SERVICE:-0}" != "1" ]] \
   && bash "$PAWL_SH" health >/dev/null 2>&1; then
  route_pkt="$(mktemp "${TMPDIR:-/tmp}/pawl-route-pkt.XXXXXX")"
  # The routing packet is the review content WITHOUT pawl-review's own VERDICT instruction —
  # pawl.sh route appends its own nonce-tagged verdict format ("PAWL <nonce> CONFIRMED|REFUTED").
  { printf '%s\n' "$read_instr"
    printf '%s\n' "$posture"
    [[ -n "$extra" ]] && printf '\nEXTRA CONTEXT FROM THE AUTHOR:\n%s\n' "$extra"
    printf '\n=== CHANGE UNDER REVIEW (bead %s, scope %s, head %s) ===\n' "$bead" "$scope" "${head:0:12}"
    printf '%s\n' "$review_body"
  } > "$route_pkt"
  echo "pawl-review: routing through the standing pawl-service (warm cross-family panel, ml8.7)…" >&2
  route_rc=0
  # Pass the REAL PR ($PR, from AGENTOPS_PAWL_PR) — NOT a hardcoded 0 — so the routed
  # verdict binds to the right PR (push-to-main is PR 0; a PR review is its number).
  bash "$PAWL_SH" route "$bead" "$route_pkt" "$PR" >&2 || route_rc=$?
  rm -f "$route_pkt"
  # Trust the route ONLY if it actually wrote a verdict bound to THIS head (fail-safe: a
  # routing error must not be read as a clean pass, and an absent/stale verdict falls back).
  # The routed path deliberately does NOT write --converge lineage: --converge is a cold,
  # codex-only bounded re-review that folds in the COLD adversarial run's preserved defects;
  # a routed duel is a different mode, so leaving no lineage makes --converge correctly
  # require a genuine cold adversarial run first (closes the auditability gap codex flagged).
  # Trust the route's CONFIRMED ONLY if the written verdict PASSES the REAL gate — the same
  # `pawl-verdict.sh check` the push gate runs (schema + PR + head-binding + cross-family
  # evidence/diversity). A shallow head+disposition jq is NOT enough: a malformed verdict
  # could slip through (codex caught exactly this). A REFUTED route is a real HOLD; anything
  # else (no gate-valid verdict) falls back to the cold codex-exec — never fail-open.
  if [[ "$route_rc" -eq 0 ]] && "$PAWL" check "$bead" "$PR" --dir "$VERDICT_DIR" --head "$head" >&2; then
    routed_mode="$(jq -r '.mode // "multi-model"' "$VERDICT_DIR/${bead}.json" 2>/dev/null)"
    echo "pawl-review: CONFIRMED (routed: $(pawl_tier_note "$routed_mode")) + VERIFIED by pawl-verdict.sh check for $bead @ ${head:0:12} — ready to push." >&2
    exit 0
  fi
  routed_disp="$(jq -r 'select(.head_sha=="'"$head"'") | .disposition // empty' \
                  "$VERDICT_DIR/${bead}.json" 2>/dev/null | tail -1)"
  if [[ "$route_rc" -eq 1 && "$routed_disp" == "REFUTED" ]]; then
    echo "=== PAWL ROUTE: REFUTED — the cross-family panel did not all CONFIRM (verdict recorded). Fix, recommit, re-run. ===" >&2
    emit_pawl_catch multi-model
    exit 3
  fi
  echo "pawl-review: pawl-route did not produce a head-bound verdict (rc=$route_rc, disp=${routed_disp:-none}) — falling back to cold codex-exec…" >&2
fi

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

# Record ADVERSARIAL lineage: a clean adversarial run (any verdict) on THIS exact diff.
# --converge later requires this for the identical diff-hash. Written here — before the
# REFUTED exit — so a REFUTED-on-cosmetic-tail run (the convergence trigger) still records
# that this diff faced maximal refutation. NOT written by --converge itself (it must not
# self-certify lineage) nor by a crashed run. (age-cwo.8 / council C)
if [[ "$converge" -eq 0 && "$codex_rc" -eq 0 && -n "$final_verdict" ]]; then
  mkdir -p "$PAWL_REVIEW_DIR"
  _outcome="$(grep -qi REFUTED <<<"$final_verdict" && echo REFUTED || echo CONFIRMED)"
  printf '{"bead":"%s","diff_hash":"%s","head_sha":"%s","outcome":"%s","ts":"%s"}\n' \
    "$bead" "$diff_hash" "$head" "$_outcome" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$lineage_file"
  # Preserve the adversarial review's FULL output (its defects). --converge folds it into
  # the verdict so the adversarial findings being ACCEPTED-AS-TAIL are recorded, never
  # silently bypassed — even a REFUTED adversarial run's defects are auditable in the
  # converge verdict (the "both bars" the council required).
  cp "$evidence" "$PAWL_REVIEW_DIR/${bead}.adversarial.evidence.txt" 2>/dev/null || true
fi

if grep -qiE 'REFUTED' <<<"$final_verdict"; then
  echo "=== PAWL REVIEW: REFUTED — defects below (fix, recommit, re-run; NO verdict written) ===" >&2
  sed -n '/^[[:space:]]*VERDICT:[[:space:]]*REFUTED/,$p' "$evidence" >&2
  emit_pawl_catch fresh-context
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

# --converge LINEAGE GATE (council C): the calibrated real-safety CONFIRMED may write a
# verdict ONLY if a prior ADVERSARIAL review covered THIS exact diff (identical hash). No
# lineage / a changed diff => ADVISORY ONLY (the calibrated review printed above), NO
# verdict (exit 4) — this prevents skipping the adversarial pass (a Goodhart gate-weaken).
if [[ "$converge" -eq 1 ]]; then
  lineage_hash=""
  [[ -f "$lineage_file" ]] && lineage_hash="$(sed -n 's/.*"diff_hash":"\([a-f0-9]*\)".*/\1/p' "$lineage_file" | head -1)"
  if [[ -z "$lineage_hash" ]]; then
    echo "pawl-review: --converge but NO adversarial lineage for $bead — the calibrated real-safety bar requires a prior adversarial review of this diff. Run 'pawl-review.sh $bead' (adversarial) first. ADVISORY-ONLY: no verdict written." >&2
    exit 4
  fi
  if [[ "$lineage_hash" != "$diff_hash" ]]; then
    echo "pawl-review: --converge but the diff CHANGED since the adversarial review (lineage hash ${lineage_hash:0:12} != current ${diff_hash:0:12}) — re-run the adversarial review on this diff first. ADVISORY-ONLY: no verdict written." >&2
    exit 4
  fi
  # Record BOTH bars in the evidence (council C): this CONFIRMED is the calibrated real-
  # safety pass over a diff with adversarial lineage. FOLD IN the full adversarial findings
  # so a REFUTED adversarial run's defects are recorded as ACCEPTED-AS-TAIL — never silently
  # bypassed (the dogfood-caught flaw: a REFUTED lineage must not hide real adversarial
  # findings behind a calibrated CONFIRMED).
  _adv_outcome="$(sed -n 's/.*"outcome":"\([A-Z]*\)".*/\1/p' "$lineage_file" | head -1)"
  {
    echo ""
    echo "[convergence] Calibrated real-safety CONFIRMED over a diff with ADVERSARIAL lineage"
    echo "(diff_hash ${diff_hash:0:12}, adversarial outcome: ${_adv_outcome}). Both bars recorded below."
    if [[ -s "$PAWL_REVIEW_DIR/${bead}.adversarial.evidence.txt" ]]; then
      echo ""
      echo "=== ADVERSARIAL FINDINGS (ACCEPTED AS TAIL by this convergence — audit them) ==="
      cat "$PAWL_REVIEW_DIR/${bead}.adversarial.evidence.txt"
    fi
  } >> "$evidence"
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
