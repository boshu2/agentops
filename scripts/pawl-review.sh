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
# LIVE SMOKE (age-rk3r.7): --smoke "<cmd>" (or the reviewed repo's .aoverify.yaml `smoke`
# key, exported by verify-config.sh as PAWL_SMOKE_CMD) runs a REAL runtime check in the
# reviewed repo BEFORE the reviewer, bounded by the review timeout. The membrane reviews the
# DIFF; the smoke reviews the RUNNING code — closing the diff-only blind spot the age-55qz.11
# escape rode (passing-but-mocked tests + a cold-pawl CONFIRMED on the diff). A red or
# timed-out smoke REFUTES fail-first (exit 3) WITHOUT spending a reviewer round; a green smoke
# attaches a LIVE RUNTIME EVIDENCE section to the reviewer packet + the bound verdict evidence.
#
# LAND DISCIPLINE — HIJACK GUARD (age-sylz): this review runs for minutes; NEVER share a
# landing worktree between lanes, or a concurrent `git reset` can move HEAD mid-review and
# bind the verdict onto the wrong commit. pawl-review snapshots HEAD at review start and
# exports it as PAWL_REVIEW_START_HEAD; pawl-verdict.sh refuses to emit/bind the edge if the
# live worktree HEAD no longer matches. Land each bead from its OWN isolated worktree.
#
# Usage: pawl-review.sh <bead-id> [--scope head|staged] [--converge] [--author-family <fam>] [--context "<extra>"] [--smoke "<cmd>"]
# Exit:  0 CONFIRMED(+written for head) · 3 REFUTED (incl. live-smoke red/stall) · 4 --converge advisory-only (no lineage) · 2 usage/precondition · 1 hard error.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# ONE fail-closed hardened `codex exec` runner — the SINGLE source of truth for the
# STALL / ECHO / MISSING-codex defenses (age-gate-the-ungated-egwt.13). The cold codex
# invocation below routes through codex_exec_guarded so this membrane cannot drift a
# subset of those defenses. Sourced from the absolutized SCRIPT_DIR so the stranger
# (embedded) path resolves it script-relative from the extracted bundle's scripts/lib/.
# shellcheck source=scripts/lib/codex-exec.sh
. "$SCRIPT_DIR/lib/codex-exec.sh"
# age-rk3r.17: source the per-repo verify config ONCE — resolves the checked-in
# .aoverify.yaml (at the REVIEWED repo's root, via cwd) into exported PAWL_* using the
# .5 Go bridge `ao verify --export-env`, so a committed, reviewable policy is honored
# without hand-exporting env. Zero config emits nothing => byte-identical. Fail-safe:
# guarded + a silent no-op when no ao is available, so it can never break a review.
# Sourced script-relative so the embedded stranger bundle resolves its sibling copy.
# shellcheck source=scripts/lib/verify-config.sh
[ -f "$SCRIPT_DIR/lib/verify-config.sh" ] && . "$SCRIPT_DIR/lib/verify-config.sh"
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
# age-rk3r.7 (round 3): per-SIDE byte cap for the live-smoke evidence head/tail. No existing
# per-section byte constant to reuse — MAX_INLINE_BYTES (64KB) is the WHOLE-packet cap, far too
# large for a single evidence section — so this introduces one deliberately UNDER it: 8KB head
# + 8KB tail = 16KB max, a quarter of the packet cap, so the smoke section can never dominate
# the packet even when a smoke emits one enormous no-newline blob (a minified bundle / giant
# JSON is line-cheap but byte-huge, so the line cap alone would copy it wholesale). Overridable.
SMOKE_EVIDENCE_MAX_BYTES="${PAWL_SMOKE_EVIDENCE_MAX_BYTES:-8192}"   # 8KB/side (head+tail = 16KB), < MAX_INLINE_BYTES

bead=""; scope="head"; extra=""; author_family="claude"; converge=0; smoke_cmd=""
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

# age-rk3r.7: LIVE-SMOKE helpers. The membrane reviews the DIFF; a live smoke reviews the
# RUNNING code — the diff-only blind spot the age-55qz.11 escape rode (mocked-but-passing
# tests + a CONFIRMED on the diff). Split out as pure formatters + a thin runner so the bats
# suite can source and exercise them, mirroring build_review_body / pawl_tier_note above.

# smoke_output_headtail <file> [head_n] [tail_n] [head_bytes] [tail_bytes]: bounded head/tail
# of a smoke's captured output, so a chatty OR single-giant-line runtime cannot bloat the
# packet / verdict evidence. Bounded by BOTH lines (readability) AND bytes (refuter catch,
# age-rk3r.7 round 3): a multi-MB no-newline blob — a minified bundle, a giant JSON dump — is
# line-cheap but byte-huge, so a line-only cap copied it wholesale into the packet + evidence.
# Pure. Emits the whole file verbatim ONLY when it is within BOTH caps; otherwise the
# byte-capped first head_n lines, one explicit elision marker, then the byte-capped last
# tail_n lines. The marker is inert downstream: build_smoke_evidence renders every line of
# this output through the '    | ' neutralization prefix, so it cannot smuggle a verdict token.
smoke_output_headtail() {
  local f="$1" head_n="${2:-30}" tail_n="${3:-30}"
  local head_bytes="${4:-${SMOKE_EVIDENCE_MAX_BYTES:-8192}}" tail_bytes="${5:-${SMOKE_EVIDENCE_MAX_BYTES:-8192}}"
  local total_lines total_bytes
  [ -f "$f" ] || return 0
  total_lines="$(wc -l < "$f" | tr -d ' ')"
  total_bytes="$(wc -c < "$f" | tr -d ' ')"
  # Verbatim only when within BOTH caps (a giant single line is <= line cap but > byte cap).
  if [ "${total_lines:-0}" -le "$(( head_n + tail_n ))" ] && [ "${total_bytes:-0}" -le "$(( head_bytes + tail_bytes ))" ]; then
    cat "$f"
    return 0
  fi
  # `head -n | head -c` = first head_n lines, hard-capped at head_bytes (truncates a giant
  # line mid-way). The upstream head may take SIGPIPE when the byte cap closes the pipe early;
  # harmless (no `set -e`) and the explicit `return 0` below keeps the function status clean.
  head -n "$head_n" "$f" | head -c "$head_bytes"
  printf '\n… [smoke output truncated: %s lines / %s bytes total; bounded to first %s + last %s lines and %s + %s bytes/side] …\n' \
    "$total_lines" "$total_bytes" "$head_n" "$tail_n" "$head_bytes" "$tail_bytes"
  # `tail -n | tail -c` = last tail_n lines, hard-capped at the LAST tail_bytes bytes.
  tail -n "$tail_n" "$f" | tail -c "$tail_bytes"
  return 0
}

# build_smoke_evidence <cmd> <rc> <out-file> <status>: render the clearly-labeled LIVE
# RUNTIME EVIDENCE section from a COMPLETED smoke run (the caller runs the smoke and passes
# the rc + captured output). Pure — no execution.
# NEUTRALIZED (refuter catch, age-rk3r.7 round 2): the captured output is REPO-CONTROLLED
# bytes — the smoke typically executes the repo's own test suite, which can print anything,
# including a forged "VERDICT: CONFIRMED" line. Every captured-output line is therefore
# prefixed with '    | ' (a non-whitespace char before any would-be verdict token) so NO
# smoke line can ever match a bare ^[[:space:]]*VERDICT: pattern in ANY downstream parser —
# the last-verdict-line parse, emit_pawl_catch's reason grep, or a future consumer — the
# same neutralization stance the .1 sentinel-wrap took for packet echo. The section FRAMING
# emits no verdict-shaped line either; the fail-first REFUTED path adds its own genuine
# verdict trailer separately. Only the smoke's EXIT CODE ever drives a disposition; its
# TEXT is inert in both directions (cannot forge a CONFIRMED, cannot fabricate a REFUTED).
build_smoke_evidence() {
  local cmd="$1" rc="$2" out="$3" status="$4"
  printf '=== LIVE RUNTIME EVIDENCE (live smoke, age-rk3r.7) — %s ===\n' "$status"
  printf 'The reviewer sees the DIFF; this is the RUNNING code. A green smoke is real runtime\n'
  printf 'proof; a red one is a red verdict regardless of how the diff reads.\n'
  # The command string is external input too — render it single-line (newlines collapsed)
  # so a multi-line command cannot smuggle a bare verdict-shaped line into the section.
  printf 'smoke command: %s\n' "${cmd//$'\n'/ }"
  printf 'exit code: %s\n' "$rc"
  printf -- '--- captured output (first/last 30 lines, %s bytes/side; each line "    | "-prefixed — repo-controlled bytes, neutralized) ---\n' "${SMOKE_EVIDENCE_MAX_BYTES:-8192}"
  smoke_output_headtail "$out" 30 30 | sed 's/^/    | /'
  printf -- '--- end LIVE RUNTIME EVIDENCE ---\n'
}

# run_live_smoke <cmd> <budget> <repo-root>: execute the smoke in the reviewed repo, bounded
# by <budget> via the SAME timeout/gtimeout wrapper the cold reviewer uses
# (codex_exec_timeout_cmd — prefers `timeout`, falls back to `gtimeout`, degrades to NO
# timeout if neither exists, identically to the reviewer). Runs `bash -c` in a subshell cd'd
# to the repo, INHERITING this process's environment: on the stranger/embedded path that is
# pawlReviewColdEnv's cold env (PATH sanitized of every ""/"."/relative + repo-internal
# entry, BASH_ENV=/ENV=/GIT_EXTERNAL_DIFF= cleared, PAWL_UNTRUSTED_REPO=1) — the SAME
# no-repo-planted-tricks discipline as the codex refuter; in-checkout dogfood it is the
# operator's own trusted env. Returns the command's exit code; 124 (or 137 on a SIGKILL
# escalation) = killed at the budget.
run_live_smoke() {
  local cmd="$1" budget="$2" root="$3"
  local -a to=()
  # shellcheck disable=SC2046,SC2206  # intentional word-split of the wrapper argv.
  read -r -a to <<<"$(codex_exec_timeout_cmd "$budget")" || true
  if [ "${#to[@]}" -gt 0 ]; then
    ( cd "$root" && "${to[@]}" bash -c "$cmd" )
  else
    ( cd "$root" && bash -c "$cmd" )
  fi
}

# age-rk3r.10: VERIFY-ERGONOMICS helpers (heartbeat + NO-VERDICT triage). Pure/idempotent
# and defined ABOVE the source-guard so the bats suite can source + exercise them directly,
# mirroring build_review_body / pawl_tier_note / the smoke helpers. They emit ONLY to stderr
# and change NO verdict semantics or exit codes — a healthy long run and a stall become legible
# without touching what the parse trusts.

# _HB_PID — the running heartbeat subshell's PID ("" = none). Declared here so the EXIT trap
# (set far below, before _HB_PID would otherwise exist under `set -u`) can always reference it.
_HB_PID=""

# start_heartbeat <budget-seconds> <reviewer-family> — spawn a background heartbeat that prints
# ONE plain stderr line every PAWL_HEARTBEAT_INTERVAL seconds (default 30; <=0 disables it) so an
# operator can tell a HEALTHY long reviewer run from a stall (a flat-0-bytes mid-run is NORMAL for
# a cold reviewer — this makes "alive" legible instead of tribal). Sets the global _HB_PID.
#
# LOAD-BEARING ISOLATION (given .7 hardened the verdict-parse isolation): the heartbeat writes
# ONLY to fd 2 (stderr). The reviewer's output is captured to a FILE (CODEX_EXEC_OUT_FILE=$raw_file)
# that the verdict parser reads — so a heartbeat line can NEVER interleave into the bytes the parse
# consumes; the two sinks are separate by construction. The subshell's fd 1 is sent to /dev/null so
# it never holds a caller's stdout/command-substitution pipe open. It also CLEARS inherited traps
# first, so being killed can never run the parent's EXIT cleanup (which rm's $raw_file/$prompt_file)
# — a delete-the-evidence-mid-review hazard this must not introduce. NO TTY/cursor codes: every line
# is a plain newline-terminated printf, clean in a non-tty pipe.
start_heartbeat() {
  local budget="${1:-0}" family="${2:-reviewer}" interval="${PAWL_HEARTBEAT_INTERVAL:-30}"
  _HB_PID=""
  [[ "$interval" =~ ^[0-9]+$ ]] || interval=30
  [ "$interval" -gt 0 ] || return 0
  (
    # (1) DROP the inherited EXIT cleanup: being killed must NEVER run the parent's rm of
    #     $raw_file/$prompt_file (a delete-the-evidence-mid-review hazard).
    # (2) On SIGTERM/SIGINT, tear down THIS process AND its current `sleep` child, then exit —
    #     so stop_heartbeat's kill leaves NO orphaned `sleep` still holding an inherited
    #     captured-output pipe (that orphan is what hung `run`-captured callers, since a caller
    #     waits for pipe EOF and the stray sleep held fd 2 open for the whole interval).
    trap - EXIT
    trap 'kill "${_hb_sleep:-}" 2>/dev/null; exit 0' TERM INT
    local hb_start hb_now hb_elapsed hb_remaining _hb_sleep
    hb_start="$(date +%s)"
    while :; do
      # Backgrounded so the TERM trap can kill it on demand; its stdio -> /dev/null so even a
      # stray one can never hold the caller's captured-output pipe (belt over the trap).
      sleep "$interval" >/dev/null 2>&1 &
      _hb_sleep=$!
      wait "$_hb_sleep" 2>/dev/null || true
      hb_now="$(date +%s)"; hb_elapsed=$(( hb_now - hb_start ))
      hb_remaining=$(( budget - hb_elapsed )); [ "$hb_remaining" -lt 0 ] && hb_remaining=0
      printf 'pawl-review: … reviewer (%s) still working — %ss elapsed, ~%ss of the %ss budget remaining (no output yet is NORMAL for a cold review; this heartbeat = alive, not stalled)\n' \
        "$family" "$hb_elapsed" "$hb_remaining" "$budget" >&2
    done
  ) >/dev/null &   # fd1 -> /dev/null so it never holds a caller's stdout/pipe open; fd2 (the heartbeat sink) stays the script's stderr
  _HB_PID="$!"
}

# stop_heartbeat [pid] — kill + reap the heartbeat subshell (default: $_HB_PID) so it can NEVER
# leak past the reviewer call. Safe on an empty or already-dead pid; clears _HB_PID.
stop_heartbeat() {
  local pid="${1:-${_HB_PID:-}}"
  [ -n "$pid" ] || return 0
  kill "$pid" 2>/dev/null || true
  wait "$pid" 2>/dev/null || true
  _HB_PID=""
}

# codex_rc_class <rc> — map a codex_exec_guarded exit code (lib/codex-exec.sh contract, .1) to a
# NO-VERDICT triage class label. Consumes the taxonomy; does NOT redefine it.
codex_rc_class() {
  case "${1:-}" in
    124) printf 'STALL' ;;      # CODEX_EXEC_STALL_TIMEOUT — killed at budget / empty after retry
    125) printf 'ECHO' ;;       # CODEX_EXEC_ECHO — prompt/packet reflected, no real review
    2)   printf 'MISSING' ;;    # CODEX_EXEC_MISSING — reviewer bin not on PATH
    *)   printf 'NO-VERDICT' ;; # ran but no parseable verdict / other
  esac
}

# ---------------------------------------------------------------------------
# REVIEWER FAILOVER CHAIN (age-rk3r.2)
# ---------------------------------------------------------------------------
# With the .1 adapters the cold-review SPOF became a ROUTING problem: when codex is down /
# overloaded / same-family-rejected, the cold path should try the NEXT configured family rather
# than stalling the whole factory — but a fallback-family verdict must SAY so (honest
# degradation). These helpers + the chain loop (far below) implement that. LOAD-BEARING:
#   1. The DEFAULT chain is codex-ONLY. A fallback family joins ONLY by EXPLICIT operator opt-in
#      (PAWL_REVIEWER_CHAIN, or a config key that exports it) — NEVER by default. With no chain
#      set, behavior is byte-identical to the single-codex path (no failover logic is reachable).
#   2. Failover triggers ONLY on OUTAGE-class exits (MISSING / STALL / ECHO / 529-class) — NEVER
#      on REFUTED. A refutation is a RESULT, not an outage: a REFUTED from reviewer 1 is FINAL;
#      reviewer 2 is never asked to overturn it into a CONFIRMED.
#   3. reviewer_family is READ from the resolved refuter (refuters[].family); the
#      honest-degradation flag lands via the .16 `degraded` field on the verdict JSON.
#   4. STRICT tier (a later slice, age-rk3r.13) will REFUSE to degrade. The seam is clean: every
#      failover decision routes through `_has_next` in the loop, which a strict mode can force
#      off. Not implemented here.

# _reviewer_spec <name> — resolve a reviewer NAME to its adapter spec, echoed as TAB-separated
#   reviewer<TAB>bin<TAB>family<TAB>family_re
# The SINGLE source of the reviewer->family/bin mapping (resolve_reviewer parses it into globals;
# the chain pre-pass reads the family_re field for same-family filtering). Byte-compatible with
# the historical inline case arm: codex bin = CODEX_EXEC_BIN (its lib override, NOT REVIEWER_BIN);
# the non-codex adapters share REVIEWER_BIN; local-mlx is eval-only (empty bin, off-roster family,
# empty re). Returns 2 (echoes nothing) for an UNKNOWN name — fail-closed: no roster family means
# the cross-family guarantee cannot be checked.
_reviewer_spec() {
  local r; r="$(printf '%s' "${1:-codex}" | tr '[:upper:]' '[:lower:]')"
  case "$r" in
    ""|codex|codex-exec|gpt|openai)
      printf 'codex\t%s\tcodex\tgpt|codex|openai\n' "${CODEX_EXEC_BIN:-codex}" ;;
    agy|gemini|antigravity|google)
      printf 'agy\t%s\tgemini\tgemini|google|agy|antigravity\n' "${REVIEWER_BIN:-agy}" ;;
    claude|anthropic|fable|opus|sonnet|haiku)
      printf '%s\t%s\tclaude\tclaude|anthropic|fable|opus|sonnet|haiku\n' "$r" "${REVIEWER_BIN:-claude-cold-adapter-unavailable}" ;;
    local-mlx|localmlx|local_mlx|mlx)
      printf 'local-mlx\t\tlocal-mlx\t\n' ;;
    *) return 2 ;;
  esac
}

# resolve_reviewer <name> — set the reviewer globals (reviewer / reviewer_bin / reviewer_family /
# reviewer_family_re) from _reviewer_spec. Returns 2 (globals untouched) on an unknown name.
resolve_reviewer() {
  local _spec; _spec="$(_reviewer_spec "$1")" || return 2
  IFS=$'\t' read -r reviewer reviewer_bin reviewer_family reviewer_family_re <<<"$_spec"
  return 0
}

# _is_outage_class <reviewer_rc> <evidence_file> — 0 (true) when a FAILED reviewer attempt is
# OUTAGE-class (the reviewer could not produce a trustworthy verdict because it was unavailable
# or overloaded) so the chain should FAIL OVER; 1 (false) for a NON-outage failure (a genuine
# auth error, a garbled clean-exit no-verdict) which must NOT fail over (invariant 2 lists exactly
# MISSING/STALL/529-class). At this LAUNCHED-exit classifier the outage set is STALL(124) PLUS a
# 529-class content probe ONLY — rc=2 (a launched reviewer's own auth/config failure; a truly-
# absent binary is the pre-launch precondition's job) and rc=125 (ECHO, a fail-closed malfunction)
# are NOT outages here (age-rk3r.2 cross-family refuter). The 529-class content probe (an API
# overload / rate-limit surfaced by the reviewer —
# codex's 529 window), trusted ONLY on an actually-failed run (rc != 0) so a clean no-verdict whose
# text merely mentions "rate limit" is never mistaken for an outage. NEVER called on a REFUTED (the
# REFUTED branch exits first), so a refutation that discusses 529s can never trigger a failover.
# Sets _OUTAGE_LABEL to the SPECIFIC reason (STALL / ECHO / MISSING / 529-class) for the failover
# trail — the ONE place the 529-class signature lives, so the label and the decision never drift.
_OUTAGE_LABEL=""
_is_outage_class() {
  local rc="${1:-}" ev="${2:-}"
  _OUTAGE_LABEL=""
  case "$rc" in
    124) _OUTAGE_LABEL="STALL";   return 0 ;;   # CODEX_EXEC_STALL_TIMEOUT — killed at budget / API slow
    # rc=2 and rc=125 are DELIBERATELY NOT outages at this LAUNCHED-exit classifier:
    #   - rc=2 (CODEX_EXEC_MISSING) as a truly-absent binary is caught by the pre-launch
    #     bin precondition (command -v), which advances the chain BEFORE any review runs.
    #     Once a reviewer has LAUNCHED, exit 2 is its OWN genuine failure (auth/config) —
    #     NOT an outage, so it must fail CLOSED, never fail over to a second family.
    #   - rc=125 (CODEX_EXEC_ECHO) is a reviewer MALFUNCTION (it echoed the packet instead
    #     of reviewing) — the anti-fabrication defense treats it fail-closed, not an outage.
    # Invariant 2 lists the outage set as STALL/MISSING/529-class; MISSING is the precondition's
    # job, so the only LAUNCHED-exit outages here are STALL(124) + the 529-class probe below.
  esac
  if [[ "$rc" -ne 0 && -n "$ev" && -f "$ev" ]] \
     && grep -qiE '(^|[^0-9])(429|529)([^0-9]|$)|overloaded|rate[ _-]?limit|too many requests|service unavailable|temporarily unavailable' "$ev" 2>/dev/null; then
    _OUTAGE_LABEL="529-class"; return 0
  fi
  return 1
}

# triage_block <class> [evidence-path] — print a one-paragraph triage block to stderr for a
# NO-VERDICT outcome (STALL / ECHO / NO-VERDICT / MISSING). A NO-VERDICT is NOT a REFUTED: no
# trustworthy review was produced, so name (1) what the exit code means, (2) the ONE next command,
# and (3) where the raw evidence is — so an operator (or a stranger with no second model) never
# bounces on a bare error. Pure formatter (no exec); reads $bead/$scope at call time. Writes only
# to stderr; exit codes + verdict semantics are unchanged (this is purely informational).
triage_block() {
  local class="${1:-NO-VERDICT}" ev="${2:-}"
  [ -n "$ev" ] || ev="n/a (no reviewer output captured)"
  local self="ao pawl review ${bead:-<change-id>} --scope ${scope:-head}"
  {
    printf '\n=== PAWL REVIEW: NO VERDICT (%s) — triage (this is NOT a REFUTED; no trustworthy review was produced) ===\n' "$class"
    case "$class" in
      STALL)
        printf 'What it means: the reviewer was killed at the timeout budget or produced no output after a retry (a stall), so exit is 1 (no-verdict, fail-closed) — NOT exit 3 (REFUTED).\n'
        printf 'Next: re-run ONCE, foreground: timeout 450 %s\n' "$self"
        printf 'If it stalls again: run `ao doctor` (checks the reviewer CLI is reachable), or switch reviewer family for this run: REVIEWER=agy %s (automatic failover lands in a later slice).\n' "$self" ;;
      ECHO|NO-VERDICT)
        printf 'What it means: the reviewer returned but its FINAL line was not a clear "VERDICT: CONFIRMED|REFUTED" (an echoed prompt / garbled run), so exit is 1 (no-verdict, fail-closed) — NOT exit 3 (REFUTED).\n'
        printf 'Next: re-run ONCE: %s\n' "$self"
        printf 'If it recurs: run `ao doctor` (reviewer reachability), or switch reviewer family: REVIEWER=agy %s.\n' "$self" ;;
      MISSING)
        printf 'What it means: no reviewer CLI is on PATH — the cross-family pawl needs a SECOND model family, so exit is 2 (precondition, fail-closed) — NOT exit 3 (REFUTED).\n'
        printf 'Next: run `ao doctor` (checks reviewer reachability + install hints), then install a reviewer CLI (codex, or agy) and put it on PATH.\n'
        printf 'No second model? Run the reviewer-OPTIONAL lane: %s --smoke "<your build/test cmd>" — a live runtime check needs no reviewer, and a red runtime still REFUTES fail-first.\n' "$self" ;;
    esac
    printf 'Raw evidence: %s\n' "$ev"
    printf '=== end triage ===\n'
  } >&2
}

# age-6idm: AUTO-RUN-THE-REPRO helpers. A REVIEWER REFUTED that NAMES an executable repro is
# a candidate FALSE refute — the 2026-07-02 build-tag class had 3/3 REFUTEDs each naming a
# repro that actually PASSED (hallucinated on cobra legacy-tag expectations + an install.sh
# --tier claim). These extract the named repro + gate it at the ARGV level so it can be run
# ONCE to check the refute. Split out as pure functions so the bats suite can source + exercise
# them, mirroring build_review_body / pawl_tier_note above.

# _repro_trim <s>: strip leading + trailing whitespace (no echo of a newline).
_repro_trim() {
  local s="$1"
  s="${s#"${s%%[![:space:]]*}"}"   # ltrim
  s="${s%"${s##*[![:space:]]}"}"   # rtrim
  printf '%s' "$s"
}

# _repro_looks_like_cmd <s>: 0 when the trimmed string's FIRST whitespace token is a repro
# tool we will consider (`go`, `bats`, or `*/bats`). This is only a candidate FILTER; the
# full security allowlist is repro_argv_allowed.
_repro_looks_like_cmd() {
  local c first
  c="$(_repro_trim "$1")"
  first="${c%%[[:space:]]*}"
  case "$first" in
    go|bats|*/bats) return 0 ;;
    *)              return 1 ;;
  esac
}

# extract_repro_command: read reviewer/evidence text on STDIN and print the FIRST plausible
# repro COMMAND STRING it names — a line inside a ``` fenced block, or a `backtick-quoted`
# span — whose first token is go/bats. Parses CONSERVATIVELY: an unquoted inline command is
# NOT parsed, and no candidate found prints NOTHING (=> no execution). Finding a candidate is
# separate from allowing it: repro_argv_allowed gates the tokenized argv before anything runs.
extract_repro_command() {
  local line in_fence=0 span
  while IFS= read -r line || [[ -n "$line" ]]; do
    case "$line" in
      '```'*) in_fence=$(( in_fence ^ 1 )); continue ;;   # fence open/close toggle
    esac
    if [[ "$in_fence" -eq 1 ]]; then
      if _repro_looks_like_cmd "$line"; then printf '%s\n' "$(_repro_trim "$line")"; return 0; fi
    else
      # inline `backtick` spans on this line, in order
      while IFS= read -r span; do
        [[ -n "$span" ]] || continue
        if _repro_looks_like_cmd "$span"; then printf '%s\n' "$(_repro_trim "$span")"; return 0; fi
      done < <(printf '%s\n' "$line" | grep -oE '`[^`]+`' | sed 's/^`//; s/`$//')
    fi
  done
  return 0
}

# repro_argv_allowed <argv...>: SECURITY GATE (age-6idm; the plan-duel judge's non-negotiable
# contract). Enforced at the ARGV level — the caller tokenizes the reviewer text itself with
# `read -r -a` and NOTHING is ever handed to `sh -c`/eval. Returns 0 ONLY when argv[0] is
# exactly `go` with argv[1] in {test,build,vet}, OR argv[0] is `bats` / ends in `/bats`; AND
# no argument is a shell metacharacter, a dangerous go flag (-exec*/-toolexec*/-ldflags*/
# -gcflags*/-o), an absolute path, or a `..` traversal component (Go's `./...` wildcard, whose
# component is three dots, is allowed). Any violation => nonzero (the caller does NOT execute).
repro_argv_allowed() {
  [[ $# -ge 1 ]] || return 1
  local a0="$1" a1="${2:-}" arg
  case "$a0" in
    go) case "$a1" in test|build|vet) ;; *) return 1 ;; esac ;;
    bats|*/bats) ;;
    *) return 1 ;;
  esac
  for arg in "$@"; do
    # shell metacharacters (rejected even though no shell is ever invoked — defense in depth)
    case "$arg" in
      *';'*|*'|'*|*'&'*|*'$'*|*'`'*|*'<'*|*'>'*|*'('*|*')'*|*'{'*|*'}'*|*'*'*|*'?'*|*'['*|*']'*|*'!'*|*'~'*|*'\'*|*"'"*|*'"'*) return 1 ;;
    esac
    # embedded newline / carriage-return / tab (line-smuggling)
    case "$arg" in
      *$'\n'*|*$'\r'*|*$'\t'*) return 1 ;;
    esac
    # dangerous go build/test flags that can execute or write outside the test
    case "$arg" in
      -exec*|-toolexec*|-ldflags*|-gcflags*|-o) return 1 ;;
    esac
    # absolute paths (we run ONLY from the repo root) + `..` traversal components. The four
    # patterns reject `..`, `../x`, `x/..`, `x/../y` WITHOUT matching Go's three-dot `./...`.
    case "$arg" in
      /*|..|../*|*/..|*/../*) return 1 ;;
    esac
  done
  return 0
}

# build_tag_sibling_context <diff> <root> <budget> (age-kg5l): when the diff touches any
# build-tagged (//go:build) .go file, emit a clearly-delimited CONTEXT-NOT-DIFF section of the
# tag-SIBLING files a cross-family refuter reviewing ONLY the diff cannot see — the cause of the
# 2026-07-02 false-REFUTED class (the tag-guarded expectations live in sibling files). Siblings
# = same-directory basename-stem siblings (foo.go -> dir/foo_*.go) + the legacy/flywheel base
# variants (strip a trailing _legacy*/_flywheel* to a base stem, then dir/base.go + dir/base_*.go),
# EXCLUDING the touched files themselves. Pure (reads files under <root>, no git) so bats can
# exercise it. Emits NOTHING when no touched file is build-tagged => the caller's packet stays
# byte-identical to today. The rendered section is truncated to <budget> BYTES (siblings only —
# never the diff), so appending it keeps the total packet under the inline ceiling.
build_tag_sibling_context() {
  local diff="$1" root="$2" budget="$3"
  [[ "${budget:-0}" -gt 0 ]] || return 0
  # touched files (b/ side of the +++ headers; drop /dev/null deletions)
  local touched
  touched="$(printf '%s\n' "$diff" | sed -n 's|^+++ b/||p' | sed '/^\/dev\/null$/d' | sort -u)"
  [[ -n "$touched" ]] || return 0
  # A //go:build line ANYWHERE in the diff makes the touched .go files tag-relevant even before
  # the file exists on disk (a file being newly tagged); the per-file head check catches the rest.
  local diff_has_tag=0
  printf '%s' "$diff" | grep -q '//go:build' && diff_has_tag=1
  local f tagfiles=""
  while IFS= read -r f; do
    [[ -n "$f" ]] || continue
    case "$f" in *.go) ;; *) continue ;; esac
    if [[ "$diff_has_tag" -eq 1 ]] || { [[ -f "$root/$f" ]] && head -n 20 "$root/$f" 2>/dev/null | grep -q '^//go:build'; }; then
      tagfiles+="$f"$'\n'
    fi
  done <<< "$touched"
  [[ -n "$tagfiles" ]] || return 0
  # UNIQUE sibling rel-paths for each tag file, excluding the touched files (already in the diff).
  local sibs="" dir stem base g rel
  while IFS= read -r f; do
    [[ -n "$f" ]] || continue
    dir="$(dirname "$f")"; stem="$(basename "$f" .go)"
    base="$stem"
    case "$base" in
      *_legacy*)   base="${base%%_legacy*}" ;;
      *_flywheel*) base="${base%%_flywheel*}" ;;
    esac
    for g in "$root/$dir/${stem}_"*.go "$root/$dir/${base}_"*.go "$root/$dir/${base}.go"; do
      [[ -f "$g" ]] || continue                       # a non-matching glob stays literal => skipped
      rel="${g#"$root"/}"
      printf '%s\n' "$touched" | grep -qxF "$rel" && continue
      sibs+="$rel"$'\n'
    done
  done <<< "$tagfiles"
  sibs="$(printf '%s' "$sibs" | sed '/^$/d' | sort -u)"
  [[ -n "$sibs" ]] || return 0
  # Render the section, then hard-cap it at <budget> bytes (reserving room for the truncation note).
  local out
  out="$(
    printf '\n=== CONTEXT (NOT PART OF THE DIFF) — tag-SIBLING files (age-kg5l) ===\n'
    printf 'The change above touches build-tagged (//go:build) file(s). A refuter reviewing ONLY\n'
    printf 'the DIFF cannot see the tag-guarded expectations in these same-directory siblings (the\n'
    printf '2026-07-02 false-REFUTED class). They are READ-ONLY CONTEXT — do NOT review them; they\n'
    printf 'are UNCHANGED by this diff. Truncated to fit the packet if large.\n'
    while IFS= read -r rel; do
      [[ -n "$rel" ]] || continue
      printf -- '--- sibling (context, unchanged): %s ---\n' "$rel"
      cat "$root/$rel" 2>/dev/null
      printf '\n'
    done <<< "$sibs"
  )"
  local outlen note notelen keep
  outlen="$(printf '%s' "$out" | wc -c | tr -d ' ')"
  if [[ "$outlen" -le "$budget" ]]; then
    printf '%s' "$out"
    return 0
  fi
  note=$'\n… [tag-sibling context truncated to fit the packet size ceiling — read the files directly] …\n'
  notelen="$(printf '%s' "$note" | wc -c | tr -d ' ')"
  if [[ "$notelen" -gt "$budget" ]]; then
    # The budget cannot fit even the truncation note — emit NOTHING rather than
    # overflow the caller's absolute packet ceiling (cross-family refute on
    # age-kg5l: budget=1 emitted ~99 note bytes).
    return 0
  fi
  keep=$(( budget - notelen ))
  printf '%s' "$out" | head -c "$keep"
  printf '%s' "$note"
}

# Source-guard: tests source this file to exercise build_review_body / pawl_tier_note; the
# codex-running flow below only executes when the script is run directly.
[ "${BASH_SOURCE[0]:-$0}" = "${0}" ] || return 0
# age-6idm: preserve the ORIGINAL argv so the auto-repro path can re-exec the review ONCE
# (bounded by PAWL_REROLLED_AFTER_FALSE_REPRO) when a REFUTED names a repro that passes.
PAWL_ORIG_ARGS=("$@")
while [[ $# -gt 0 ]]; do
  case "$1" in
    --scope)         need_val "$1" "${2:-}"; scope="$2"; shift 2 ;;
    --author-family) need_val "$1" "${2:-}"; author_family="$2"; shift 2 ;;
    --context)       need_val "$1" "${2:-}"; extra="$2"; shift 2 ;;
    --smoke)         need_val "$1" "${2:-}"; smoke_cmd="$2"; shift 2 ;;
    --converge)      converge=1; shift ;;
    -h|--help)       sed -n '2,44p' "$0"; exit 0 ;;
    -*)              echo "pawl-review: unknown flag $1" >&2; exit 2 ;;
    *)               bead="$1"; shift ;;
  esac
done
# age-rk3r.10: the change-id (bead) is a LABEL ONLY — it keys the verdict/evidence file names and
# is echoed into the reviewer packet; it is NEVER validated against a tracker (recon-confirmed). So
# when it is OMITTED, DERIVE a label rather than bouncing the caller a round trip (a coordinator hit
# "need <bead-id>" and lost a round): the current git branch (sanitized to a filename-safe token),
# else the short HEAD sha, else a timestamp. PRINT a note naming the derived label so the operator
# knows what the verdict/evidence files are keyed on. Passing a change-id works EXACTLY as before.
if [[ -z "$bead" ]]; then
  _derived_src=""
  _derived_branch="$(git -C "$REPO_ROOT" rev-parse --abbrev-ref HEAD 2>/dev/null)"
  if [[ -n "$_derived_branch" && "$_derived_branch" != "HEAD" ]]; then
    # Sanitize: the label becomes a filename under the evidence/review/verdicts dirs, so collapse
    # anything outside [A-Za-z0-9._-] (e.g. a `feat/foo` branch's slash) to '-' and trim leading/trailing '-'.
    bead="$(printf '%s' "$_derived_branch" | tr -c 'A-Za-z0-9._-' '-' | sed 's/^-*//; s/-*$//')"
    [[ -n "$bead" ]] && _derived_src="branch '$_derived_branch'"
  fi
  if [[ -z "$bead" ]]; then
    bead="$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null)"
    [[ -n "$bead" ]] && _derived_src="short HEAD sha"
  fi
  [[ -n "$bead" ]] || { bead="pawl-$(date -u +%Y%m%dT%H%M%SZ)"; _derived_src="timestamp (no branch or HEAD)"; }
  echo "pawl-review: no change-id given — using the derived label '$bead' (from $_derived_src). The change-id is a LABEL only (names the verdict/evidence files), never tracker-validated; pass one explicitly to override." >&2
fi
# age-rk3r.7: resolve the live-smoke command. Precedence: explicit --smoke flag >
# PAWL_SMOKE_CMD (from the operator's env, or the reviewed repo's .aoverify.yaml `smoke`
# key exported by verify-config.sh at entry) > none.
# SECURITY: a CONFIG-sourced smoke is a REPO-PLANTED command, so on the UNTRUSTED
# stranger/embedded path (PAWL_UNTRUSTED_REPO=1) ONLY the operator's explicit --smoke flag
# is honored — the reviewed repo's own `smoke:` config is IGNORED (never auto-run a stranger
# repo's arbitrary command as a side effect of reviewing it). In-checkout dogfood (trusted)
# honors both. This mirrors resolve_ao's untrusted-repo stance for the command SOURCE, on
# top of the cold env the smoke already inherits (see run_live_smoke).
if [[ -z "$smoke_cmd" && "${PAWL_UNTRUSTED_REPO:-0}" != "1" ]]; then
  smoke_cmd="${PAWL_SMOKE_CMD:-}"
fi
# --converge (the calibrated real-safety bar) certifies a COMMIT, so it requires
# --scope head; and it is lineage-gated below (age-cwo.8 / council C).
[[ "$converge" -eq 1 && "$scope" != "head" ]] && { echo "pawl-review: --converge requires --scope head (it certifies a commit)" >&2; exit 2; }
[[ -x "$PAWL" ]] || { echo "pawl-review: $PAWL not executable" >&2; exit 1; }
# age-rk3r.2: resolve the cold REVIEWER CHAIN (default codex-only). The lib (lib/codex-exec.sh)
# owns the per-adapter argv/marker/echo/sandbox contract + run classification; this script is a
# THIN switch that resolves each family (via _reviewer_spec, the SINGLE mapping source), honors it
# for the precondition + same-family guard, routes a non-codex reviewer to the COLD adapter (the
# warm tmux service stays on its codex-family route), and passes REVIEWER to the lib per attempt.
# reviewer_family is the label written into the binding verdict's refuter entry (age-rk3r.1 DEFECT 2:
# a hardcoded "codex" made REVIEWER=agy certify family=codex). Labels are values pawl-verdict.sh's
# normalize_family accepts: codex->"codex" (byte-compat; canonicalizes to gpt), agy->CANONICAL
# "gemini"; local-mlx gets a deliberately OFF-roster label (an opted-in eval run must never pass the
# prod roster check — 2026-06-23 ruling).
#
# THE CHAIN: DEFAULT = codex-only (or a single explicit REVIEWER), preserving today's behavior
# byte-for-byte (invariant 1). PAWL_REVIEWER_CHAIN is the EXPLICIT operator opt-in — a comma list
# tried in order on OUTAGE, e.g. "codex,agy". The .5/.17 config layer may export it from a repo's
# .aoverify.yaml; env is the primary surface. Every member is roster-validated below (unknown =>
# fail-closed) and local-mlx stays prod-refused IN-LIB, so even a config-fed chain can only select a
# real cross-family reviewer, never inject an arbitrary or weak-eval one.
reviewer_chain=()
if [[ -n "${PAWL_REVIEWER_CHAIN:-}" ]]; then
  IFS=',' read -r -a _rc_raw <<<"$PAWL_REVIEWER_CHAIN"
  for _rc in "${_rc_raw[@]}"; do
    _rc="$(printf '%s' "$_rc" | tr -d '[:space:]')"   # tolerate "codex, agy"
    [[ -n "$_rc" ]] && reviewer_chain+=("$_rc")
  done
fi
# No chain configured => the single-reviewer default (REVIEWER if set, else codex). This is the
# invariant-1 lock: with nothing set the chain is exactly [codex] and no failover is observable.
[[ ${#reviewer_chain[@]} -gt 0 ]] || reviewer_chain=("${REVIEWER:-codex}")

# Validate every chain member up front (a typo'd reviewer is a config error, caught before any
# review runs) and compute the USABLE list = the members that are CROSS-family to the author, in
# order. A member that is the SAME family as --author-family is SKIPPED — that is CORRECT ROUTING
# (pick the first cross-family reviewer), NOT degradation: the first USABLE reviewer is the "first
# choice", and a verdict from it is degraded=false. Same case-insensitive family-regex test the
# historical single-reviewer same-family guard used (round-5 pawl catch: an empty re skips the guard).
af_lc="$(printf '%s' "$author_family" | tr '[:upper:]' '[:lower:]')"
usable_reviewers=()
for _rc in "${reviewer_chain[@]}"; do
  _spec="$(_reviewer_spec "$_rc")" || {
    # FAIL-CLOSED (round-5 pawl catch, preserved): an unknown reviewer name has no roster family,
    # so the cross-family guarantee this command exists to provide cannot be checked.
    echo "pawl-review: unknown reviewer '$_rc' — no roster family, so the cross-family guard cannot run." >&2
    echo "  Add the adapter to the reviewer roster (scripts/pawl-review.sh + lib/codex-exec.sh) with an" >&2
    echo "  explicit family; refusing fail-closed. (exit 2 = precondition, not a REFUTE)" >&2
    exit 2
  }
  _fam_re="$(printf '%s' "$_spec" | cut -f4)"
  if [[ -n "$_fam_re" && "$af_lc" =~ ($_fam_re) ]]; then
    continue   # same family as the author — skip (routing); keep looking for a cross-family member
  fi
  usable_reviewers+=("$_rc")
done
# No cross-family member anywhere in the chain => a same-family review is all that is possible,
# which is not the cross-family pawl this command provides. Refuse (byte-compatible with the
# historical single-reviewer same-family guard: exit 2, message contains "SAME model family").
if [[ ${#usable_reviewers[@]} -eq 0 ]]; then
  echo "pawl-review: no cross-family reviewer available — every reviewer in the chain (${reviewer_chain[*]}) is the SAME model family as --author-family '$author_family'. That is a same-family review, not the cross-family pawl this command provides. Configure a different-family reviewer (or author with a different family)." >&2
  exit 2
fi

command -v jq >/dev/null 2>&1 || {
  echo "pawl-review: MISSING DEPENDENCY — jq is not on PATH (the pawl parses verdicts with jq)." >&2
  echo "  This is NOT a review result. Install jq (brew install jq / apt-get install jq) and re-run." >&2
  echo "  (exit 2 = precondition, not a REFUTE)" >&2
  exit 2
}

# The kill-budget snapshot. INITIAL value here; FINALIZED after the large-diff scaling
# below (see the age-iian re-snapshot). age-iian (FOLDED into age-rk3r.1): historically this
# was the ONLY snapshot and it was captured BEFORE scale_review_timeout reassigns $TIMEOUT,
# so a large read-files diff that scaled its budget up (e.g. 300s -> 540s) STILL got killed at
# the UNSCALED 300s — the auto-scaled budget never reached the cold kill. run_review reads
# REVIEW_TIMEOUT at CALL time (far below), so the fix is to RE-SNAPSHOT after scaling runs.
# The lib's codex_exec_guarded owns the actual timeout/gtimeout wrapper + degrade-to-no-timeout
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
  # REVIEWER is passed EXPLICITLY as the resolved/normalized adapter name, so the lib
  # dispatches exactly what this script validated (precondition + same-family guard);
  # the codex-specific vars (SANDBOX/EXTRA_ARGS) are ignored by non-codex adapters.
  REVIEWER="$reviewer" \
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
# age-sylz HIJACK GUARD: snapshot the HEAD this review is ABOUT and export it, so the
# verdict step (pawl-verdict.sh write, cold path here; and the routed pawl.sh path, which
# inherits the env) can refuse to bind if a concurrent lane resets a SHARED landing
# worktree mid-review — a review runs for minutes, and binding onto a hijacked HEAD would
# certify the WRONG commit (real incident 2026-07-02). The `check` stale-guard misses this
# (it passes when verdict.head_sha and --head moved together).
export PAWL_REVIEW_START_HEAD="$head"

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
# age-kg5l: if the diff touches build-tagged (//go:build) files, append a CONTEXT-NOT-DIFF
# section of the tag-SIBLING files AFTER the diff, truncated to keep the total packet under the
# inline ceiling (siblings truncated, never the diff). A refuter reviewing only the diff cannot
# see the tag-guarded sibling expectations — the 2026-07-02 false-REFUTED class. Non-tag diffs
# produce an empty section => the packet stays byte-identical to today.
_sib_body_bytes="$(printf '%s' "$review_body" | wc -c | tr -d ' ')"
_sib_budget=$(( MAX_INLINE_BYTES - _sib_body_bytes ))
if [[ "$_sib_budget" -gt 0 ]]; then
  _sibling_ctx="$(build_tag_sibling_context "$diff" "$REPO_ROOT" "$_sib_budget")"
  [[ -n "$_sibling_ctx" ]] && review_body="${review_body}${_sibling_ctx}"
fi
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

# age-iian (FOLDED into age-rk3r.1): RE-SNAPSHOT the kill budget AFTER scale_review_timeout has
# (possibly) raised $TIMEOUT for a large read-files packet. The earlier REVIEW_TIMEOUT snapshot
# was taken BEFORE this scaling, so without this line the auto-scaled budget never reached the
# cold reviewer kill (a large diff needing 540s was still killed at the unscaled 300s).
# run_review consumes REVIEW_TIMEOUT (CODEX_EXEC_TIMEOUT) when it is CALLED below, so finalizing
# the snapshot here lets the scaled budget through to the exec wrapper.
REVIEW_TIMEOUT="$TIMEOUT"

# Lineage key: the content hash of the reviewed diff. The adversarial run records it;
# --converge requires a prior adversarial run on the IDENTICAL diff (no fuzzy "material
# change" — an exact-hash match, which is unambiguous and safe). (age-cwo.8)
PAWL_REVIEW_DIR="$REPO_ROOT/.agents/pawl-review"
lineage_file="$PAWL_REVIEW_DIR/${bead}.adversarial.json"
if command -v shasum >/dev/null 2>&1; then diff_hash="$(printf '%s' "$diff" | shasum -a 256 | cut -d' ' -f1)"
else diff_hash="$(printf '%s' "$diff" | sha256sum | cut -d' ' -f1)"; fi

mkdir -p "$EVIDENCE_DIR"
# The refuter context id (ctx) names the RESOLVED reviewer and so is computed PER ATTEMPT inside
# the failover loop below (age-rk3r.2) — on a fail-over it must name the WINNING reviewer, not the
# first-choice one. (age-rk3r.1 DEFECT 2: codex stays "codex-fresh-…" byte-compat; agy is honestly
# "agy-fresh-…".)
ctx=""
evidence="$EVIDENCE_DIR/${bead}-pawl-review.txt"
prompt_file="$(mktemp "${TMPDIR:-/tmp}/pawl-review-prompt.XXXXXX")"
raw_file="$(mktemp "${TMPDIR:-/tmp}/pawl-review-raw.XXXXXX")"
trap 'rm -f "$prompt_file" "$raw_file"; [ -n "${_HB_PID:-}" ] && kill "${_HB_PID}" 2>/dev/null; true' EXIT

# age-rk3r.7: LIVE SMOKE — run the configured runtime check BEFORE any reviewer round, so a
# red runtime fails FIRST (cheap; no reviewer spent) and a green runtime attaches real
# evidence. Bounded by the (already diff-size-scaled) $REVIEW_TIMEOUT via the reviewer's own
# timeout wrapper; a hanging smoke is killed and fails closed (STALL). The smoke inherits the
# cold env on the stranger path (see run_live_smoke) and runs with cwd=$REPO_ROOT.
smoke_evidence=""
if [[ -n "$smoke_cmd" ]]; then
  smoke_out="$(mktemp "${TMPDIR:-/tmp}/pawl-review-smoke.XXXXXX")"
  trap 'rm -f "$prompt_file" "$raw_file" "$smoke_out"; [ -n "${_HB_PID:-}" ] && kill "${_HB_PID}" 2>/dev/null; true' EXIT
  echo "pawl-review: LIVE SMOKE — running the configured runtime check in $REPO_ROOT (bounded by ${REVIEW_TIMEOUT}s) BEFORE the reviewer…" >&2
  smoke_rc=0
  run_live_smoke "$smoke_cmd" "$REVIEW_TIMEOUT" "$REPO_ROOT" >"$smoke_out" 2>&1 || smoke_rc=$?

  # A killed run (the timeout wrapper returns 124, or 137 on a SIGKILL escalation) is a
  # STALL: no green runtime in budget — fail-closed, no CONFIRMED possible. Guarded on a
  # timeout wrapper ACTUALLY being applied (codex_exec_timeout_cmd non-empty), so on a host
  # with neither timeout nor gtimeout a genuine smoke exit of 124/137 still reads as a normal
  # non-zero REFUTE below rather than being mislabeled a stall.
  if { [ "$smoke_rc" -eq 124 ] || [ "$smoke_rc" -eq 137 ]; } && [ -n "$(codex_exec_timeout_cmd "$REVIEW_TIMEOUT")" ]; then
    {
      build_smoke_evidence "$smoke_cmd" "$smoke_rc" "$smoke_out" "TIMED-OUT (killed at ${REVIEW_TIMEOUT}s budget)"
      echo "DEFECTS:"
      echo "- live smoke TIMED OUT after ${REVIEW_TIMEOUT}s and was KILLED (rc=$smoke_rc): the runtime did not come up green in budget — fail-closed (STALL)."
      echo "VERDICT: REFUTED"
    } > "$evidence"
    echo "=== PAWL REVIEW: REFUTED — live smoke STALLED (killed at ${REVIEW_TIMEOUT}s budget); NO reviewer round spent, NO verdict written ===" >&2
    sed -n '/^=== LIVE RUNTIME EVIDENCE/,$p' "$evidence" >&2
    emit_pawl_catch live-smoke
    exit 3
  fi

  # A non-zero smoke is a RED RUNTIME — REFUTE FIRST, without spending a reviewer round. A
  # red runtime is a red verdict regardless of how the diff reads (the age-55qz.11 blind spot).
  if [[ "$smoke_rc" -ne 0 ]]; then
    {
      build_smoke_evidence "$smoke_cmd" "$smoke_rc" "$smoke_out" "FAILED (exit $smoke_rc)"
      echo "DEFECTS:"
      echo "- live smoke exited NON-ZERO (exit $smoke_rc): the runtime is RED at head ${head:0:12} regardless of how the diff reads. Fix the runtime, recommit, re-run."
      echo "VERDICT: REFUTED"
    } > "$evidence"
    echo "=== PAWL REVIEW: REFUTED — live smoke FAILED (exit $smoke_rc); NO reviewer round spent, NO verdict written ===" >&2
    sed -n '/^=== LIVE RUNTIME EVIDENCE/,$p' "$evidence" >&2
    emit_pawl_catch live-smoke
    exit 3
  fi

  # PASS: build the LIVE RUNTIME EVIDENCE section attached to the reviewer packet below +
  # persisted into the bound verdict evidence file after the reviewer runs.
  smoke_evidence="$(build_smoke_evidence "$smoke_cmd" "$smoke_rc" "$smoke_out" "PASSED (exit 0)")"
  echo "pawl-review: LIVE SMOKE passed (exit 0) — attaching runtime evidence to the reviewer packet + verdict evidence." >&2
fi

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
$smoke_evidence
=== CHANGE UNDER REVIEW (bead $bead, scope $scope, head ${head:0:12}) ===
PROMPT
  printf '%s\n' "$review_body"
} > "$prompt_file"

# ---- age-rk3r.2: FAILOVER LOOP over the usable (cross-family) reviewers ----
# The first USABLE reviewer is the FIRST CHOICE (a verdict from it is degraded=false). On an
# OUTAGE (MISSING / STALL / ECHO / 529-class) with a fallback remaining, fail over to the next
# family and mark degraded=true (a non-first-choice family produced the verdict). A REFUTED is a
# RESULT — final, NO failover (a REFUTED from reviewer 1 is never re-asked of reviewer 2). A
# non-outage failure (auth error / garbled clean no-verdict) also does NOT fail over. With a
# SINGLE usable reviewer (the default) `_has_next` is always 0, so every branch takes exactly the
# historical single-reviewer exit — byte-identical (invariant 1). The prompt/smoke above are
# reviewer-agnostic and built ONCE; only the reviewer resolution + run + classification loop.
degraded=false
reviewer_attempts=0
failover_trail=""
_n_usable=${#usable_reviewers[@]}
for (( _ui=0; _ui<_n_usable; _ui++ )); do
  resolve_reviewer "${usable_reviewers[_ui]}"   # sets reviewer/reviewer_bin/reviewer_family/reviewer_family_re (roster-validated above)
  reviewer_attempts=$(( reviewer_attempts + 1 ))
  _has_next=0; [[ $(( _ui + 1 )) -lt "$_n_usable" ]] && _has_next=1
  # Defense-in-depth (round-5 pawl catch: an empty family regex once SKIPPED the same-family guard,
  # a self-approval bypass). The usable list already excluded same-family members, but re-assert on
  # the RESOLVED reviewer: a reviewer that is somehow the SAME family as the author is a same-family
  # review — refuse fail-closed rather than self-approve (never reachable in normal routing).
  if [[ -n "$reviewer_family_re" && "$af_lc" =~ ($reviewer_family_re) ]]; then
    echo "pawl-review: internal guard — resolved reviewer '$reviewer' is the SAME model family as --author-family '$author_family' despite cross-family routing; refusing fail-closed. (exit 2)" >&2
    exit 2
  fi
  # The refuter context id names the RESOLVED reviewer (codex stays "codex-fresh-…" byte-compat;
  # agy is honestly "agy-fresh-…") — recomputed per attempt so a fail-over names the WINNER.
  ctx="${reviewer}-fresh-${bead}-$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null)"

  # PRECONDITION (age-a9iv.5 / age-rk3r.1): the resolved reviewer's binary must be on PATH — a
  # PRECONDITION failure (exit 2), NOT a REFUTE. age-rk3r.2: a MISSING bin is an OUTAGE — with a
  # fallback remaining, fail over (degraded); with none, the byte-identical MISSING exit. codex
  # keeps its exact message; a non-codex adapter checks its own bin; local-mlx has no bin precond
  # here (the lib hard-refuses it in prod without PAWL_EVAL_ADAPTERS_OK=1).
  if { [[ "$reviewer" == "codex" ]] || [[ -n "$reviewer_bin" ]]; } && ! command -v "$reviewer_bin" >/dev/null 2>&1; then
    if [[ "$_has_next" -eq 1 ]]; then
      degraded=true; failover_trail="${failover_trail}${failover_trail:+ -> }${reviewer_family}(MISSING)"
      echo "pawl-review: reviewer '$reviewer' MISSING (bin '$reviewer_bin' not on PATH) — OUTAGE; failing over to the next configured reviewer…" >&2
      continue
    fi
    if [[ "$reviewer" == "codex" ]]; then
      echo "pawl-review: MISSING DEPENDENCY — codex (the cross-family refuter) is not on PATH." >&2
      echo "  This is NOT a review result. The pawl needs a SECOND model family to run the cross-family" >&2
      echo "  check; install the codex CLI, put it on PATH, and re-run. (exit 2 = precondition, not a REFUTE)" >&2
    else
      echo "pawl-review: MISSING DEPENDENCY — '$reviewer_bin' (the '$reviewer' cross-family reviewer) is not on PATH." >&2
      echo "  This is NOT a review result. Install the '$reviewer' reviewer CLI, put it on PATH, and re-run." >&2
      echo "  (exit 2 = precondition, not a REFUTE)" >&2
    fi
    triage_block MISSING ""   # DUEL AMENDMENT (age-rk3r.10): name `ao doctor` + the --smoke reviewer-optional lane
    exit 2
  fi

  # Lazy auto-start (the "membrane is never silently cold again" fix): when routing is
  # eligible (head scope, not --converge, not opted out) but the standing service is DOWN,
  # stand it up ONCE here so this and every later review in the session run WARM through the
  # tri-model duel instead of paying a cold codex-exec each time (the gap that left ~13 reviews
  # cold in a single session). One-time ~couple-min warmup; fail-safe — if `up` fails the
  # health check below stays false and we fall through to the cold path. Opt out:
  # PAWL_NO_SERVICE=1 (disable the whole service path) or PAWL_NO_AUTOUP=1 (route-if-up only).
  # age-rk3r.1: only for the codex-family route. An explicit non-codex REVIEWER (e.g. agy) wants
  # the COLD portable adapter below — the warm tmux service stays on its own codex-family route.
  # age-rk3r.2: also gated on the NON-DEGRADED first codex attempt — a warm route writes its OWN
  # verdict with no degraded flag, so it must never run for a fall-over-to-codex (rare) attempt.
  if [[ "$degraded" == false && "$converge" -eq 0 && "$scope" == "head" && "$reviewer" == "codex" && "${PAWL_NO_SERVICE:-0}" != "1" && "${PAWL_NO_AUTOUP:-0}" != "1" ]] \
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
  # path. Opt out: PAWL_NO_SERVICE=1. age-rk3r.1: an explicit non-codex REVIEWER also stays on the
  # cold adapter below (the warm service route is codex-family); "$reviewer" == "codex" gates this.
  # age-rk3r.2: also gated on the non-degraded first attempt (see auto-up above).
  if [[ "$degraded" == false && "$converge" -eq 0 && "$scope" == "head" && "$reviewer" == "codex" && "${PAWL_NO_SERVICE:-0}" != "1" ]] \
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
      # age-rk3r.7: the SAME live-smoke runtime evidence goes into the routed packet too, so a
      # head-scope review through the warm pawl-service also sees the running-code proof.
      [[ -n "$smoke_evidence" ]] && printf '%s\n' "$smoke_evidence"
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
  echo "pawl-review: running cross-family ($reviewer) review of $scope diff for $bead (head ${head:0:12})…" >&2
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
  # age-rk3r.10: a heartbeat around the reviewer call makes a healthy long run legible vs a stall.
  # It writes ONLY to stderr; run_review captures the reviewer to $raw_file, so the heartbeat can
  # never interleave into the bytes the verdict parse reads. Stopped unconditionally after the call
  # (below), and the EXIT trap kills any orphan if the script dies mid-review.
  start_heartbeat "$REVIEW_TIMEOUT" "$reviewer"
  run_review || codex_rc=$?
  stop_heartbeat

  # Codex prints a 'codex' marker line before its answer; drop the marker itself and keep
  # what follows (evidence starts at the reviewer's content, not the marker). Fall back to
  # the full output if there is no marker.
  verdict_block="$(awk '/^codex$/{c=1; next} c' "$raw_file" 2>/dev/null)"
  [[ -n "$verdict_block" ]] || verdict_block="$(cat "$raw_file")"
  printf '%s\n' "$verdict_block" > "$evidence"
  # age-rk3r.2: EMPTY output = a STALL. With a fallback remaining, fail over (degraded); with
  # none, the byte-identical fail-closed STALL exit.
  if [[ ! -s "$evidence" ]]; then
    if [[ "$_has_next" -eq 1 ]]; then
      degraded=true; failover_trail="${failover_trail}${failover_trail:+ -> }${reviewer_family}(STALL)"
      echo "pawl-review: reviewer '$reviewer' produced NO output (STALL) — OUTAGE; failing over to the next configured reviewer…" >&2
      continue
    fi
    echo "pawl-review: no reviewer output captured — fail-closed" >&2; triage_block STALL "$evidence"; exit 1
  fi

  # Decide on the FINAL verdict-shaped line only — parsed from the REVIEWER's OWN bytes
  # ($verdict_block, which only ever holds the reviewer subprocess's output), NEVER from the
  # evidence file (refuter catch, age-rk3r.7 round 2): the live-smoke section appended below
  # embeds REPO-CONTROLLED output (the smoke typically runs the repo's own test suite, which
  # can print anything — including a forged "VERDICT: CONFIRMED" placed AFTER a real REFUTED),
  # so smoke bytes must be STRUCTURALLY incapable of reaching this parse. The reviewer's real
  # answer comes LAST, after any preamble or echoed prompt template — an echoed
  # "VERDICT: CONFIRMED" from the instructions must not be mistaken for the verdict. (defends
  # a quoted/multi-verdict false-CONFIRMED)
  final_verdict="$(printf '%s\n' "$verdict_block" | grep -iE '^[[:space:]]*VERDICT:[[:space:]]*(CONFIRMED|REFUTED)' | tail -1)"

  # age-rk3r.7: persist the live-smoke runtime evidence into the SAME verdict evidence file the
  # CONFIRMED verdict binds — so the proof artifact carries the RUNTIME proof, not just the diff
  # review. Appended AFTER (and independent of) the verdict parse above; defense-in-depth,
  # every captured smoke-output line is "    | "-neutralized by build_smoke_evidence, so no
  # smoke line can match a bare ^VERDICT: pattern in ANY downstream consumer of this file
  # (emit_pawl_catch's reason grep included). Empty when no smoke ran => byte-identical.
  [[ -n "$smoke_evidence" ]] && printf '\n%s\n' "$smoke_evidence" >> "$evidence"

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

  # REFUTED is a RESULT, not an outage: FINAL, NEVER a failover (invariant 2). Even with a
  # fallback configured, a REFUTED from this reviewer stands — reviewer 2 is not asked to
  # overturn it into a CONFIRMED.
  if grep -qiE 'REFUTED' <<<"$final_verdict"; then
    # age-6idm: AUTO-RUN-THE-REPRO. A REVIEWER REFUTED that NAMES an executable repro is a
    # candidate FALSE refute (the 2026-07-02 build-tag class: 3/3 REFUTEDs each named a repro
    # that actually PASSED). Extract the repro from the DEFECTS text, gate it at the ARGV level
    # (repro_argv_allowed — never sh -c/eval), and run it ONCE, timeboxed:
    #   - repro PASSES (exit 0) -> the refute is SUSPECT; re-roll the review ONCE. Bounded by
    #     PAWL_REROLLED_AFTER_FALSE_REPRO (the marker that survives the re-exec) so a second
    #     false-repro can NOT loop — on an already-rerolled run this branch is skipped and the
    #     REFUTED STANDS (surfaced with the run) even if its repro passes.
    #   - repro FAILS -> the REFUTED STANDS; attach the run's exit code + output tail as evidence.
    #   - no candidate / disallowed argv -> no execution; the REFUTED stands exactly as today.
    repro_cmd="$(extract_repro_command < "$evidence")"
    if [[ -n "$repro_cmd" ]]; then
      read -r -a repro_argv <<<"$repro_cmd"   # ARGV-level tokenize; NOTHING goes to a shell
      if [[ "${#repro_argv[@]}" -ge 1 ]] && repro_argv_allowed "${repro_argv[@]}"; then
        echo "pawl-review: REFUTED names a repro — running it ONCE (timeboxed, argv-allowlisted) to check the refute: ${repro_argv[*]}" >&2
        repro_out="$(mktemp "${TMPDIR:-/tmp}/pawl-review-repro.XXXXXX")"
        _rt=(); read -r -a _rt <<<"$(codex_exec_timeout_cmd 300)" || true   # same timeout wrapper as the reviewer
        repro_rc=0
        if [[ "${#_rt[@]}" -gt 0 ]]; then
          ( cd "$REPO_ROOT" && "${_rt[@]}" "${repro_argv[@]}" ) >"$repro_out" 2>&1 || repro_rc=$?
        else
          ( cd "$REPO_ROOT" && "${repro_argv[@]}" ) >"$repro_out" 2>&1 || repro_rc=$?
        fi
        if [[ "$repro_rc" -eq 0 && -z "${PAWL_REROLLED_AFTER_FALSE_REPRO:-}" ]]; then
          echo "pawl-review: the named repro PASSED (exit 0) — the REFUTED is SUSPECT (false repro). Re-rolling the review ONCE (bounded)…" >&2
          # exec skips the EXIT trap, so clean the temp files this run owns before replacing.
          rm -f "$repro_out" "$prompt_file" "$raw_file"
          [[ -n "${smoke_out:-}" ]] && rm -f "$smoke_out"
          PAWL_REROLLED_AFTER_FALSE_REPRO=1 exec bash "$0" "${PAWL_ORIG_ARGS[@]}"
        fi
        # repro FAILED, or we already re-rolled once (bounded) — the REFUTED STANDS. Attach evidence.
        {
          printf '\n=== AUTO-REPRO (age-6idm) — the REFUTED named a repro; it was run ONCE, timeboxed ===\n'
          printf 'repro command: %s\n' "${repro_argv[*]}"
          printf 'exit code: %s\n' "$repro_rc"
          if [[ -n "${PAWL_REROLLED_AFTER_FALSE_REPRO:-}" ]]; then
            printf 'NOTE: this review was already RE-ROLLED once after a false repro (rerolled_after_false_repro);\n'
            printf 'a second REFUTED STANDS even if its repro passes — surfaced here with the run.\n'
          fi
          printf -- '--- repro output tail (each line "    | "-prefixed — run output, neutralized) ---\n'
          tail -n 30 "$repro_out" 2>/dev/null | sed 's/^/    | /'
          printf -- '--- end AUTO-REPRO ---\n'
        } >> "$evidence"
        rm -f "$repro_out"
      else
        echo "pawl-review: REFUTED named a repro '$repro_cmd' NOT in the allowed argv set (go test|build|vet or bats, no dangerous args/paths) — NOT executing; the REFUTED stands." >&2
      fi
    fi
    echo "=== PAWL REVIEW: REFUTED — defects below (fix, recommit, re-run; NO verdict written) ===" >&2
    sed -n '/^[[:space:]]*VERDICT:[[:space:]]*REFUTED/,$p' "$evidence" >&2
    emit_pawl_catch fresh-context
    exit 3
  fi
  # No clear CONFIRMED. age-rk3r.2: an OUTAGE (STALL/ECHO/529-class) with a fallback remaining
  # fails over (degraded); a NON-outage no-verdict (a genuine garbled run) does NOT — it takes
  # the byte-identical fail-closed exit.
  if ! grep -qiE 'CONFIRMED' <<<"$final_verdict"; then
    if [[ "$_has_next" -eq 1 ]] && _is_outage_class "$codex_rc" "$evidence"; then
      degraded=true; failover_trail="${failover_trail}${failover_trail:+ -> }${reviewer_family}(${_OUTAGE_LABEL})"
      echo "pawl-review: reviewer '$reviewer' produced NO clear verdict + OUTAGE (${_OUTAGE_LABEL}) — failing over to the next configured reviewer…" >&2
      continue
    fi
    echo "pawl-review: reviewer's FINAL line is not a clear VERDICT: CONFIRMED|REFUTED — fail-closed. Raw output in $evidence" >&2
    triage_block "$(codex_rc_class "$codex_rc")" "$evidence"
    exit 1
  fi
  # A CONFIRMED is only trustworthy from a CLEANLY-EXITED reviewer run (defends defect #3).
  # age-rk3r.2: an incomplete-run CONFIRMED is an OUTAGE — fail over if a fallback remains.
  if [[ "$codex_rc" -ne 0 ]]; then
    if [[ "$_has_next" -eq 1 ]] && _is_outage_class "$codex_rc" "$evidence"; then
      degraded=true; failover_trail="${failover_trail}${failover_trail:+ -> }${reviewer_family}(${_OUTAGE_LABEL})"
      echo "pawl-review: reviewer '$reviewer' CONFIRMED from an INCOMPLETE run ($codex_rc, ${_OUTAGE_LABEL}) — OUTAGE; failing over to the next configured reviewer…" >&2
      continue
    fi
    echo "pawl-review: reviewer exited non-zero ($codex_rc, e.g. timeout 124) — refusing to trust a CONFIRMED from an incomplete run — fail-closed" >&2
    triage_block "$(codex_rc_class "$codex_rc")" "$evidence"
    exit 1
  fi

  # CONFIRMED + clean run — this reviewer WON. Leave the loop and write the verdict below.
  failover_trail="${failover_trail}${failover_trail:+ -> }${reviewer_family}(CONFIRMED)"
  break
done

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
# The refuter entry certifies the RESOLVED reviewer's family (age-rk3r.1 DEFECT 2 fix —
# a hardcoded "codex" here made REVIEWER=agy certify family=codex in the binding verdict).
#
# age-rk3r.2: when this verdict came from a FALL-OVER (a non-first-choice family, after an
# OUTAGE on an earlier one), stamp the .16 `degraded` flag so downstream (metrics/audits) can
# tell a nominal verdict from a weaker-than-nominal one. reviewer_family is READ from the
# resolved refuter (NOT re-added top-level — invariant 3). The attempt COUNT + family trail are
# recorded in the BOUND evidence (not a new schema field), so the winning reviewer's evidence
# self-documents the fall-over. With no chain / no fall-over, degraded stays false and NO flag /
# note is emitted — byte-identical (invariant 1).
_degraded_args=()
if [[ "$degraded" == true ]]; then
  _degraded_args=(--degraded true)
  {
    echo ""
    echo "=== DEGRADED FALLBACK (age-rk3r.2) ==="
    echo "This verdict was produced by a FALL-OVER reviewer after an OUTAGE on an earlier configured"
    echo "family — a DEGRADED (weaker-than-nominal) review posture, honestly labeled degraded=true."
    echo "Reviewer chain: ${reviewer_chain[*]}"
    echo "Usable (cross-family) chain: ${usable_reviewers[*]}"
    echo "Attempts: ${reviewer_attempts}   Trail: ${failover_trail}   Winning family: ${reviewer_family}"
    echo "=== end DEGRADED FALLBACK ==="
  } >> "$evidence"
  # A degraded fallback that produced THIN evidence is not trustworthy: the .11 evidence-quality
  # floor still applies. `$PAWL check` below runs pawl_evidence_floor on this verdict (advisory
  # now, HOLD once it enforces on/after its flip date) — a degraded verdict is NEVER exempted.
  echo "pawl-review: DEGRADED verdict — fell over to '${reviewer_family}' after ${reviewer_attempts} attempt(s) [${failover_trail}]; stamping degraded=true. The .11 evidence-quality floor still applies below (a thin-evidence degraded fallback is not trustworthy)." >&2
fi
"$PAWL" write "$bead" "$PR" \
  --disposition CONFIRMED --head "$head" \
  --author-context "author-${author_family}-${bead}" --author-family "$author_family" \
  --refuter "${reviewer_family}:CONFIRMED:${ctx}:${evidence}" \
  ${_degraded_args[@]+"${_degraded_args[@]}"} \
  --dir "$VERDICT_DIR" >/dev/null || { echo "pawl-review: verdict write failed" >&2; exit 1; }

_deg_suffix=""; [[ "$degraded" == true ]] && _deg_suffix=" (DEGRADED fallback — see PAWL-FLOOR above)"
if "$PAWL" check "$bead" "$PR" --dir "$VERDICT_DIR" --head "$head" >&2; then
  echo "pawl-review: CONFIRMED + verdict written + verified for $bead @ ${head:0:12}${_deg_suffix} — ready to push." >&2
  exit 0
fi
echo "pawl-review: verdict written but the check did not pass (see above) — fail-closed" >&2
exit 1
