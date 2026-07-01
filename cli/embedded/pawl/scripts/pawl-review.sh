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
# ONE fail-closed hardened `codex exec` runner — the SINGLE source of truth for the
# STALL / ECHO / MISSING-codex defenses (age-gate-the-ungated-egwt.13). The cold codex
# invocation below routes through codex_exec_guarded so this membrane cannot drift a
# subset of those defenses. Sourced from the absolutized SCRIPT_DIR so the stranger
# (embedded) path resolves it script-relative from the extracted bundle's scripts/lib/.
# shellcheck source=scripts/lib/codex-exec.sh
. "$SCRIPT_DIR/lib/codex-exec.sh"
PAWL="$SCRIPT_DIR/pawl-verdict.sh"
# The standing-pawl service script (overridable for tests). Always the real script next
# to this one — NOT the repo-under-review's (they differ for alt worktrees). (ml8.7)
PAWL_SH="${PAWL_SERVICE_SCRIPT:-$SCRIPT_DIR/pawl.sh}"
# The repo UNDER REVIEW (overridable for tests / alt worktrees); PAWL itself is always
# the real script next to this one.
REPO_ROOT="${AGENTOPS_REPO_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}"
VERDICT_DIR="${AGENTOPS_PAWL_VERDICT_DIR:-$REPO_ROOT/.agents/pawl-verdicts}"
EVIDENCE_DIR="$REPO_ROOT/.agents/pawl-evidence"

# resolve_ao: print the ao binary path, preferring the REPO's build over a possibly
# STALE installed `ao`. A globally-installed ao can lag the repo and lack a newly-added
# subcommand/flag (e.g. `membrane recall --include-catches`, `membrane catch`), which
# would silently no-op the membrane catch/recall (epic age-zpj5; codex S3 round-3).
# Order: $AO_BIN, repo build ($REPO_ROOT/cli/bin/ao then $REPO_ROOT/cli/ao), then PATH.
# Prints nothing + returns 1 when none is executable (callers treat that as skip).
#
# SECURITY (age-a9iv.4): when PAWL_UNTRUSTED_REPO=1 (the stranger/embedded path), REPO_ROOT
# is the repo UNDER REVIEW — untrusted — so its $REPO_ROOT/cli/* must NEVER be executed
# (an attacker could plant cli/bin/ao and get arbitrary code-exec before the read-only
# review, via recall_prior_catches/emit_pawl_catch). In that mode use only the explicitly-
# trusted $AO_BIN (the invoking ao binary) then PATH. In-checkout (flag unset) is unchanged:
# REPO_ROOT is the real AgentOps checkout, so preferring its build is correct + intended.
resolve_ao() {
  local c
  if [[ "${PAWL_UNTRUSTED_REPO:-0}" == "1" ]]; then
    # Untrusted repo: ONLY the absolute pinned $AO_BIN. NEVER `command -v ao` — with a
    # `.`/relative PATH entry and cwd inside the repo it would resolve a planted ./ao
    # (RCE). No trusted binary => skip (callers treat empty as "no ao", best-effort).
    if [[ -n "${AO_BIN:-}" && -x "${AO_BIN:-}" ]]; then printf '%s\n' "$AO_BIN"; return 0; fi
    return 1
  fi
  for c in "${AO_BIN:-}" "${REPO_ROOT:-.}/cli/bin/ao" "${REPO_ROOT:-.}/cli/ao"; do
    if [[ -n "$c" && -x "$c" ]]; then printf '%s\n' "$c"; return 0; fi
  done
  command -v ao 2>/dev/null
}

# emit_pawl_catch <mode>: record this REFUTE as a structured membrane catch (epic
# age-zpj5, S2) so DetectCatches/recall can group recurring classes — domain from
# the changed-files' top dir, reason from the REFUTED verdict text, paths from the
# reviewed files. Fail-safe + NON-BLOCKING: a missing `ao` or any error never blocks
# the REFUTED exit (the catch is observability, not a gate). Reads the globals
# (bead/head/evidence/review_files) at call time.
emit_pawl_catch() {
  local mode="${1:-fresh-context}" ao_bin reason domain paths files
  local -a catch_args=()
  ao_bin="$(resolve_ao)"; [[ -n "$ao_bin" ]] || return 0
  [[ -n "${bead:-}" && -n "${head:-}" ]] || return 0
  reason="$(grep -iE '^[[:space:]]*VERDICT:[[:space:]]*REFUTED' "${evidence:-/dev/null}" 2>/dev/null | tail -1 \
            | sed -E 's/^[[:space:]]*VERDICT:[[:space:]]*REFUTED[[:space:]:—-]*//I' | cut -c1-200)"
  [[ -n "$reason" ]] || reason="pawl-review REFUTED for $bead (see evidence)"
  # Changed files for affected_paths — computed DIRECTLY from git by scope, NOT from
  # $review_files (which pawl-review only populates for LARGE diffs > MAX_INLINE_BYTES),
  # so a NORMAL small-diff catch is still path-recallable.
  case "${scope:-head}" in
    staged) files="$(git -c core.fsmonitor= -C "${REPO_ROOT:-.}" diff --cached --no-ext-diff --no-textconv --name-only --no-color 2>/dev/null | sed '/^$/d')" ;;
    *)      files="$(git -c core.fsmonitor= -C "${REPO_ROOT:-.}" show HEAD --no-ext-diff --no-textconv --name-only --format= --no-color 2>/dev/null | sed '/^$/d')" ;;
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

# recall_prior_catches: print the MEMBRANE MEMORY block (prior catches in this change's
# domain + touched files) for injection into the reviewer prompt — the consumption side
# of membrane memory (epic age-zpj5, S3). Fail-safe: prints NOTHING (and never errors)
# when ao is absent, no domain resolves, or there are no recorded catches. Output is
# `ao membrane recall` text (controlled, not diff content).
recall_prior_catches() {
  local ao_bin domain files recalled
  ao_bin="$(resolve_ao)"; [[ -n "$ao_bin" ]] || return 0
  case "${scope:-head}" in
    staged) files="$(git -c core.fsmonitor= -C "${REPO_ROOT:-.}" diff --cached --no-ext-diff --no-textconv --name-only --no-color 2>/dev/null | sed '/^$/d')" ;;
    *)      files="$(git -c core.fsmonitor= -C "${REPO_ROOT:-.}" show HEAD --no-ext-diff --no-textconv --name-only --format= --no-color 2>/dev/null | sed '/^$/d')" ;;
  esac
  domain="$(printf '%s\n' "$files" | head -1 | cut -d/ -f1)"
  [[ -n "$domain" ]] || return 0
  # Recall by DOMAIN only (no --paths): surface EVERY catch in this bounded context so
  # the injection never misses one (a path filter would need every touched file, and a
  # cap on that list silently drops catches for files past the cap — codex S3 round-2).
  # The path-overlap narrowing stays available on `ao membrane recall --paths` for
  # explicit queries (and is unit-tested), but the auto-injection wants never-miss.
  recalled="$( ( cd "${REPO_ROOT:-.}" && "$ao_bin" membrane recall --domain "$domain" --include-catches ) 2>/dev/null )" || return 0
  # Only inject when there are ACTUAL catches (skip the "No past catches" line).
  if printf '%s' "$recalled" | grep -q 'past catch class'; then
    printf '\n=== MEMBRANE MEMORY — PRIOR CATCHES IN THIS AREA (domain %s; verify the change does NOT reintroduce these) ===\n%s\n' "$domain" "$recalled"
  fi
}
# scale_review_timeout (age-wedge-all-in-dyr0.11, measured 2026-07-01): read-files mode
# COMPLETES on huge diffs but needs wall-clock proportional to size — 117KB verdicted
# inside 420s; a 209KB deletion died at 420s and verdicted clean at ~540s. Pure: prints
# the effective timeout given diff bytes, the inline cap, the current timeout, and
# whether the operator pinned PAWL_REVIEW_TIMEOUT explicitly (explicit pin always wins).
# Ceiling 900s: beyond that a review is a smell, not a budget problem.
scale_review_timeout() {
  local diff_bytes="$1" cap="$2" current="$3" pinned="$4"
  if [[ -n "$pinned" || "$diff_bytes" -le "$cap" ]]; then
    printf '%s\n' "$current"; return 0
  fi
  local overage_kb=$(( (diff_bytes - cap) / 1024 ))
  local scaled=$(( 300 + overage_kb * 2 ))
  [[ "$scaled" -gt 900 ]] && scaled=900
  [[ "$scaled" -gt "$current" ]] && current="$scaled"
  printf '%s\n' "$current"
}

PR="${AGENTOPS_PAWL_PR:-0}"          # 0 = push-to-main landing (matches the pre-push gate)
TIMEOUT="${PAWL_REVIEW_TIMEOUT:-300}"
# Inline cap. ABOVE this the packet switches to read-files mode, which orders the reviewer to
# open the changed files itself + elides the added lines — and that mode is what makes a cold
# `codex exec` WANDER (grep the filesystem) or ECHO the prompt instead of reviewing (age-a9iv:
# observed on 60KB+ diffs). Inline mode ("review ONLY the change below, no tools") is self-
# contained and reliable, so INLINE generously and reserve read-files for only enormous diffs.
# Safety no longer rides on this number: the verdict-format hardening below makes an echoed or
# oversized run fail CLOSED (no parseable verdict), never false-pass. (Original age-mwhj note:
# 41KB timed a warm opus PANE out / 62KB cold codex killed — a timeout-machine artifact; cold
# codex with no timeout completes.)
MAX_INLINE_BYTES="${PAWL_MAX_INLINE_BYTES:-65536}"   # 64KB — inline the common range; read-files only for huge diffs

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
# age-a9iv.5: hard runtime deps are a PRECONDITION failure (exit 2), NOT a hard error (1) and NOT a
# review result (3=REFUTED) — a missing dep that exits 1 is easy to misread as a real refutation. Name
# the dep, say it is installable, and use the distinct precondition code so a caller can tell them apart.
command -v codex >/dev/null 2>&1 || {
  echo "pawl-review: MISSING DEPENDENCY — codex (the cross-family refuter) is not on PATH." >&2
  echo "  This is NOT a review result. The pawl needs a SECOND model family to run the cross-family" >&2
  echo "  check; install the codex CLI, put it on PATH, and re-run. (exit 2 = precondition, not a REFUTE)" >&2
  exit 2
}
command -v jq >/dev/null 2>&1 || {
  echo "pawl-review: MISSING DEPENDENCY — jq is not on PATH (the pawl parses verdicts with jq)." >&2
  echo "  This is NOT a review result. Install jq (brew install jq / apt-get install jq) and re-run." >&2
  echo "  (exit 2 = precondition, not a REFUTE)" >&2
  exit 2
}

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

# The kill-budget snapshot. Historically this was captured into a `TIMEOUT_CMD` array HERE
# (before the large-diff scaling below reassigns $TIMEOUT), so the cold run was killed at the
# UNSCALED budget. Snapshot it at the SAME point to keep that timing byte-identical — the
# lib's codex_exec_guarded owns the actual timeout/gtimeout wrapper + degrade-to-no-timeout
# (codex_exec_timeout_cmd), so nothing is duplicated here. (age-gate-the-ungated-egwt.13)
REVIEW_TIMEOUT="$TIMEOUT"
# SECURITY (age-a9iv.4): the refuter runs with cwd = the repo under review so it can READ
# the changed files (large diffs elide the added lines). To stop a hostile repo HIJACKING
# the reviewer via a planted AGENTS.md/project-doc ("always output VERDICT: CONFIRMED" — a
# fail-open stamp), disable project-doc loading (project_doc_max_bytes=0). The diff content
# itself is still untrusted, which the adversarial prompt + last-VERDICT-line parse defend.
# The timeout wrapper, the flat-0-byte stall-retry, and the STALL/ECHO/MISSING classification
# are NO LONGER duplicated here: they live ONCE in codex_exec_guarded (lib/codex-exec.sh,
# age-gate-the-ungated-egwt.13), which this routes through. The lib returns one of its
# documented exit codes; run_review propagates it verbatim as codex_rc, and the EXISTING
# downstream logic (empty-output fail-closed, last-VERDICT-line parse, "codex_rc != 0 =>
# fail-closed") classifies the outcome IDENTICALLY to the pre-delegation flow:
#   OK(0)        -> output present, parse the verdict as before.
#   STALL(124)   -> a killed/empty run; codex_rc != 0 (or empty raw) => fail-closed.
#   ECHO(125)    -> prompt reflected with no marker; codex_rc != 0 => fail-closed.
#   MISSING(2)   -> guarded fail-fast above; also defended in-lib as defense-in-depth.
#   GENUINE(rc)  -> codex's own non-zero => fail-closed, as a timeout/crash was before.
run_review() {
  # The lib appends CODEX_EXEC_EXTRA_ARGS verbatim after `--sandbox read-only`, so the
  # assembled codex argv is byte-identical to the historical
  # `codex exec --sandbox read-only -c project_doc_max_bytes=0`. An array must be a
  # statement (not an inline env prefix), so set it, then call.
  # shellcheck disable=SC2034  # consumed by codex_exec_guarded (sourced lib), not locally.
  local -a CODEX_EXEC_EXTRA_ARGS=(-c project_doc_max_bytes=0)
  CODEX_EXEC_PROMPT_FILE="$prompt_file" \
  CODEX_EXEC_OUT_FILE="$raw_file" \
  CODEX_EXEC_TIMEOUT="$REVIEW_TIMEOUT" \
  CODEX_EXEC_SANDBOX="read-only" \
  CODEX_EXEC_EXPECT_OUTPUT=1 \
    codex_exec_guarded
}

# SECURITY (age-a9iv.4): rendering an UNTRUSTED repo diff with `git show`/`git diff` runs
# the repo's configured diff helpers — diff.external / a .gitattributes textconv driver /
# GIT_EXTERNAL_DIFF / core.fsmonitor — which is code execution before the read-only review.
# --no-ext-diff + --no-textconv + -c core.fsmonitor= disable them (GIT_EXTERNAL_DIFF also
# cleared in the cold env). --stat/--name-only below render no content, so they are safe.
case "$scope" in
  head)   diff="$(git -c core.fsmonitor= -C "$REPO_ROOT" show HEAD --no-ext-diff --no-textconv --no-color 2>/dev/null)" ;;
  staged) diff="$(git -c core.fsmonitor= -C "$REPO_ROOT" diff --cached --no-ext-diff --no-textconv --no-color 2>/dev/null)" ;;
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
    head)   review_stat="$(git -c core.fsmonitor= -C "$REPO_ROOT" show HEAD --no-ext-diff --no-textconv --stat --format= --no-color 2>/dev/null)"; review_files="$(git -c core.fsmonitor= -C "$REPO_ROOT" show HEAD --no-ext-diff --no-textconv --name-only --format= --no-color 2>/dev/null | sed '/^$/d')" ;;
    staged) review_stat="$(git -c core.fsmonitor= -C "$REPO_ROOT" diff --cached --no-ext-diff --no-textconv --stat --no-color 2>/dev/null)"; review_files="$(git -c core.fsmonitor= -C "$REPO_ROOT" diff --cached --no-ext-diff --no-textconv --name-only --no-color 2>/dev/null | sed '/^$/d')" ;;
  esac
fi
review_body="$(build_review_body "$diff" "$MAX_INLINE_BYTES" "$review_stat" "$review_files" "$REPO_ROOT")"
if [[ "$diff_bytes" -gt "$MAX_INLINE_BYTES" ]]; then
  read_instr="This change is LARGE and is NOT inlined. READ the changed files listed below directly (read-only); they are the change under review. Do not modify anything."
  echo "pawl-review: large diff (${diff_bytes}B > ${MAX_INLINE_BYTES}B cap) — packet uses read-files-not-inline (age-mwhj)" >&2
  _scaled_timeout="$(scale_review_timeout "$diff_bytes" "$MAX_INLINE_BYTES" "$TIMEOUT" "${PAWL_REVIEW_TIMEOUT:-}")"
  if [[ "$_scaled_timeout" != "$TIMEOUT" ]]; then
    TIMEOUT="$_scaled_timeout"
    echo "pawl-review: review timeout scaled to ${TIMEOUT}s for ${diff_bytes}B read-files packet (set PAWL_REVIEW_TIMEOUT to pin)" >&2
  fi
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

# Posture (age-cwo.8/council C; RECALIBRATED 2026-06-28, age-pawl-good-bar): the merge gate binds
# the CALIBRATED "good" bar by default — a thorough real-defect search with a 3-filter BLOCK
# threshold (INTRODUCED-by-this-diff × REAL/verifiable × BLOCKING). This REPLACES the old default
# maximal-adversarial "REFUTE anything plausible-but-wrong" posture, which had a documented infinite
# false-alarm tail: on a multi-round landing it refuted ~9× and ~2/3 were cosmetic / pre-existing /
# theoretical / hallucinated nitpicks, so a CONFIRM was never reachable. The anti-Goodhart property
# council C built is preserved: this calibrated bar is the MANDATORY default — there is no
# author-selectable knob to route around it. --converge stays the even-narrower real-safety-only
# convergence-tail bar. "Good" = "would a thoughtful senior engineer BLOCK this merge?", never "can
# I find any imperfection?". (full definition: docs/contracts/pawls.md "What good means")
if [[ "$converge" -eq 1 ]]; then
  posture="This is a CALIBRATED real-safety CONVERGENCE pass: the change ALREADY went through a maximal-adversarial review and its real defects were fixed. Answer ONLY the BOUNDED question: is there any REMAINING REAL SAFETY defect — a concrete path that (a) writes/certifies something it should NOT (a fail-open), (b) loses or corrupts data, or (c) targets the wrong object? Novel theoretical or parse edge-cases, cosmetic wording, and 'a producer could output something weirder' are the ACCEPTED TAIL and are NOT grounds to refute. REFUTE ONLY for a concrete real-safety defect."
else
  posture="You are the merge-to-main gate's reviewer. Your job is NOT to find any imperfection — it is to answer ONE question: would a thoughtful senior engineer BLOCK this merge? Search hard for REAL defects (logic bugs, fail-open holes, data loss/corruption, races, wrong-object/SHA, broken contracts, a test that passes even if the code were wrong, a feature that does not do what the commit claims). But a finding REFUTES the change ONLY if it clears ALL THREE filters: (1) INTRODUCED — created or newly made-reachable BY THIS DIFF, not a pre-existing condition and not adjacent hardening the change merely 'could have also done'; (2) REAL — concrete and survives deterministic ground truth: name the check that would expose it (go vet, go test ./<pkg>, bash -n <file>, a parity/regen/audit script) and its expected result, and if a green check you can actually run contradicts the finding, DROP it — and never assert a defect about a file you did not read; (3) BLOCKING — it breaks correctness or safety (writes/certifies what it should not = fail-open; loses or corrupts data; targets the wrong object), makes a CLAIMED contract false (a documented behavior, a commit-message promise, a public interface, or a test's stated guarantee), or ships NON-WORKING (does not build/parse, the relevant test fails, or it provably does not do what the change says — assert this only from a check you actually ran). A finding that fails ANY of the three filters is the ACCEPTED TAIL — cosmetic/wording/style, a coverage gap on otherwise-correct code, a novel theoretical 'a producer could emit something weirder' edge-case with no reachable path in THIS diff, adjacent/out-of-scope hardening the change never claimed, a pre-existing condition, or a design-disagreement with a deliberate documented choice whose worst case stays fail-closed. Surface tail findings as accepted NOTES; do NOT refute on them. CONFIRM when the only remaining findings are accepted tail. Fail-closed is never relaxed: if you cannot run or read the change, or your review errors out, emit NO verdict — absence of a clean review is never a CONFIRMED."
fi

# The refuter prompt: posture + diff. Diff is appended from a FILE (never shell-
# interpolated) so $(...) / backticks in the diff are not evaluated.
prior_catches="$(recall_prior_catches)"   # membrane memory: prior catches in this area (S3), empty if none
{
  cat <<PROMPT
You are a fresh-context, cross-family reviewer — the REFUTER in a two-model pawl. $read_instr
$posture
${extra:+
EXTRA CONTEXT FROM THE AUTHOR:
$extra
}
Reply with nothing but your review. The FINAL line is the verdict: the token "VERDICT:" then one space then exactly one uppercase word, either CONFIRMED (no blocking defect found) or REFUTED (a blocking defect found). If you refute, put a "DEFECTS:" header above that final line with one concrete defect per line (the symptom and why it matters). (This instruction deliberately does not print a ready-made verdict line, so that an echo of this prompt cannot be mistaken for your verdict.)
$prior_catches
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
    # S3: the SAME membrane-memory injection as the cold path — so head-scope reviews
    # through the warm pawl-service ALSO consume prior catches (computed at $prior_catches).
    [[ -n "$prior_catches" ]] && printf '%s\n' "$prior_catches"
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
# age-wjp0: backgrounding-reap hazard. When this script is launched DETACHED (harness
# run_in_background / `nohup &`), its process group can be reaped mid-`codex exec` —
# killing the review before the retry + fail-closed logic below ever runs, leaving ONLY
# the line above as output. That silent flatline looks like a codex stall but is a reap.
# External reaping cannot be prevented from inside the script, so make the failure
# SELF-EXPLAINING: emit the diagnostic BEFORE the vulnerable exec so it survives the kill.
# Fires only when non-interactive (the exact condition where the hazard exists and the
# operator has no live feedback). (memory: pawl-run-foreground-not-background)
if ! { [ -t 1 ] || [ -t 2 ]; }; then
  echo "pawl-review: ⚠ non-interactive run — codex review takes ~3-5 min. If NO verdict follows this line, the codex subprocess was reaped (common when backgrounded). Re-run FOREGROUND: timeout 450 ao pawl review $bead --scope $scope" >&2
fi
codex_rc=0
# The flat-0-byte stall-retry lives in codex_exec_guarded now (CODEX_EXEC_RETRY_ON_EMPTY=1,
# its default): a first run that produces NOTHING is retried once before it is classified as
# a STALL, so it is no longer re-implemented here (single source of truth, age-gate-the-
# ungated-egwt.13). codex_rc carries the lib's outcome code; a STALL/ECHO/timeout returns
# non-zero and the fail-closed logic below (empty-output guard + "codex_rc != 0") holds.
run_review || codex_rc=$?

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
