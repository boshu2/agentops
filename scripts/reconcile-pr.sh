#!/usr/bin/env bash
#
# reconcile-pr.sh — drive ONE PR to a confirmed merge, then close its bead.
#
# This is the committed, tested form of the ephemeral /tmp reconcile engine that
# drove the bead-crank session (ag-9gac). The behavior it enforces used to be
# advisory prose in skills/crank/SKILL.md + skills/evolve/SKILL.md:
#   - never merge while substantive checks are pending or failing
#   - tolerate the two known soft signals (claude-review usage-limit soft-fail,
#     the correctness-ubuntu tar-cache-restore exit-2 flake — rerun ONCE)
#   - close the bead ONLY after gh confirms the PR state is MERGED
#   - (optional) close the epic ONLY when every child is closed
#
# Usage:
#   scripts/reconcile-pr.sh <pr-number> <bead-id> [--epic <epic-id>] \
#     [--poll-max N] [--poll-sleep S] [--dry-run]
#
# Exit codes (documented contract — tests assert these exactly):
#   0  PR merged AND bead closed (and, in --epic mode, epic reconciled)
#   1  gate failure (--epic mode: epic has an open/in_progress child; epic NOT closed)
#   2  blocked — substantive check(s) failing OR still PENDING after POLL_MAX
#      (green CI is strictly necessary; pending-forever never reaches merge);
#      did NOT merge, did NOT close
#   3  merge attempted but PR state is not MERGED; bead NOT closed
#   4  usage / missing-dependency / bad-input error
#   5  pawl-gate HOLD — green CI but the pawl verdict does NOT authorize:
#      absent / REFUTED / ESCALATE / HOLD / diversity-floor-unmet / empty-or-STALE
#      head_sha (head unresolvable, or a new commit was pushed after the review) /
#      missing reviewer evidence / schema-invalid. Did NOT merge, did NOT close.
#      Fail-closed: green CI is necessary but NOT sufficient at the
#      mutate-shared-trunk door (docs/contracts/pawls.md). A tripped circuit
#      breaker surfaces for a human.
#
# Dependencies: gh, bd, jq (stubbed via PATH in the hermetic bats suite).
# The pawl verdict (fresh-context default; multi-model opt-in) is read via
# scripts/pawl-verdict.sh (sibling).

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

PR=""
BEAD=""
EPIC=""
POLL_MAX=30
POLL_SLEEP=20
DRY_RUN=false
# Where the pawl verdict lives (see scripts/pawl-verdict.sh).
# Overridable for hermetic tests; defaults to the repo's untracked runtime dir.
VERDICT_DIR="$SCRIPT_DIR/../.agents/pawl-verdicts"

# Checks we never treat as blocking:
#   claude-review — soft-fails on usage-limit; advisory, not a merge gate.
SOFT_FAIL_CHECK="claude-review"
# The lone known flake: a tar-cache-restore exit-2 on this check. Rerun ONCE.
FLAKE_CHECK="correctness (ubuntu-latest)"

usage() {
  cat >&2 <<'USAGE'
Usage: scripts/reconcile-pr.sh <pr-number> <bead-id> [options]

Drives one PR to a confirmed merge, then closes its bead. Polls checks until
terminal, reruns once on the known correctness-ubuntu flake, treats only
substantive non-claude-review failures as blocking, merges --squash --admin,
confirms state==MERGED, then `bd update <bead> --status closed`.

Options:
  --epic <epic-id>   After closing the bead, reconcile <epic-id>: close it only
                     if check-epic-children-closed.sh reports no open children.
  --poll-max N       Max poll iterations waiting for pending checks (default 30).
  --poll-sleep S     Seconds between polls (default 20).
  --verdict-dir D    Directory holding the pawl verdict for this bead
                     (default .agents/pawl-verdicts). Green CI is necessary
                     but NOT sufficient — a CONFIRMED pawl verdict tied
                     to this bead+PR must exist or the merge is refused (HOLD).
  --dry-run          Print actions; do not merge or mutate beads.
  -h, --help         Show this help.

Exit codes: 0 merged+closed · 1 gate-fail · 2 blocked · 3 merge-fail ·
            4 usage · 5 pawl-gate HOLD (green CI, no CONFIRMED pawl verdict).
USAGE
}

die() { echo "ERROR: $*" >&2; exit 4; }

# --- arg parse ---------------------------------------------------------------
positional=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --epic)       EPIC="${2:-}"; shift 2 || die "--epic needs a value" ;;
    --poll-max)   POLL_MAX="${2:-}"; shift 2 || die "--poll-max needs a value" ;;
    --poll-sleep) POLL_SLEEP="${2:-}"; shift 2 || die "--poll-sleep needs a value" ;;
    --verdict-dir) VERDICT_DIR="${2:-}"; shift 2 || die "--verdict-dir needs a value" ;;
    --dry-run)    DRY_RUN=true; shift ;;
    -h|--help)    usage; exit 0 ;;
    --*)          usage; die "unknown flag: $1" ;;
    *)            positional+=("$1"); shift ;;
  esac
done

[[ ${#positional[@]} -eq 2 ]] || { usage; die "need exactly <pr-number> <bead-id>"; }
PR="${positional[0]}"
BEAD="${positional[1]}"

[[ "$PR" =~ ^[0-9]+$ ]] || die "pr-number must be numeric, got: $PR"
[[ -n "$BEAD" ]] || die "bead-id must be non-empty"

command -v gh >/dev/null 2>&1 || die "gh CLI not on PATH"
command -v bd >/dev/null 2>&1 || die "bd CLI not on PATH"
command -v jq >/dev/null 2>&1 || die "jq not on PATH"

# --- helpers -----------------------------------------------------------------

# Emit the PR's checks as JSON array of {name,state} where state is one of
# the `gh pr checks --json state` values (PENDING/SUCCESS/FAILURE/...).
pr_checks_json() {
  gh pr checks "$PR" --json name,state 2>/dev/null
}

# Count non-skipping checks whose state is "pending"-like.
count_pending() {
  pr_checks_json | jq '[.[] | select((.state|ascii_downcase) as $s | $s=="pending" or $s=="queued" or $s=="in_progress" or $s=="waiting")] | length'
}

# Print the names of substantive failing checks (state failure-like),
# EXCLUDING the soft-fail claude-review check. One name per line.
failing_checks() {
  pr_checks_json | jq -r --arg soft "$SOFT_FAIL_CHECK" '
    .[]
    | select((.state|ascii_downcase) as $s | $s=="failure" or $s=="error" or $s=="cancelled" or $s=="timed_out")
    | select(.name != $soft)
    | .name'
}

# Does the failing set consist of EXACTLY the lone flake check?
only_flake_failing() {
  local fails
  fails="$(failing_checks)"
  [[ "$fails" == "$FLAKE_CHECK" ]]
}

rerun_flake() {
  # Best-effort: rerun failed jobs for the latest run, then let the poll loop
  # pick the new states up. We don't hard-fail if rerun itself errors.
  echo "FLAKE: lone '$FLAKE_CHECK' failure — gh run rerun --failed (once)" >&2
  if $DRY_RUN; then return 0; fi
  local run_id head_ref
  head_ref="$(gh pr view "$PR" --json headRefName -q .headRefName 2>/dev/null || true)"
  run_id="$(gh run list --branch "$head_ref" --limit 1 --json databaseId -q '.[0].databaseId' 2>/dev/null || true)"
  if [[ -n "$run_id" ]]; then
    gh run rerun "$run_id" --failed >/dev/null 2>&1 || true
  else
    gh run rerun --failed >/dev/null 2>&1 || true
  fi
}

# --- poll loop ---------------------------------------------------------------
flake_rerun_used=false
i=0
pend=0
while (( i < POLL_MAX )); do
  i=$((i+1))
  pend="$(count_pending)"
  pend="${pend:-0}"
  if [[ "$pend" -eq 0 ]]; then
    break
  fi
  echo "poll $i/$POLL_MAX: $pend pending check(s)..." >&2
  if $DRY_RUN; then
    # In dry-run we don't sleep forever; one observation is enough.
    break
  fi
  sleep "$POLL_SLEEP"
done

# --- evaluate terminal state -------------------------------------------------
fails="$(failing_checks)"

if [[ -n "$fails" ]]; then
  if only_flake_failing && [[ "$flake_rerun_used" == false ]]; then
    flake_rerun_used=true
    rerun_flake
    # Re-poll after the rerun.
    i=0
    while (( i < POLL_MAX )); do
      i=$((i+1))
      pend="$(count_pending)"; pend="${pend:-0}"
      [[ "$pend" -eq 0 ]] && break
      echo "re-poll $i/$POLL_MAX: $pend pending after rerun..." >&2
      $DRY_RUN && break
      sleep "$POLL_SLEEP"
    done
    fails="$(failing_checks)"
  fi
fi

if [[ -n "$fails" ]]; then
  echo "BLOCKED fails=[$(echo "$fails" | paste -sd, -)]" >&2
  exit 2
fi

# --- terminal-green requirement (FAIL-CLOSED on still-pending) ---------------
# Green CI must be strictly NECESSARY: a check that never concluded (still
# PENDING/QUEUED/IN_PROGRESS after POLL_MAX) must NOT be treated as "not
# failing" and slipped through to merge. Re-observe one final time; if anything
# is still pending, BLOCK (exit 2) — pending-forever never reaches merge.
pend="$(count_pending)"; pend="${pend:-0}"
if [[ "$pend" -ne 0 ]]; then
  echo "BLOCKED: $pend check(s) still PENDING after $POLL_MAX polls — not concluded green; did NOT merge (green CI is strictly necessary)" >&2
  exit 2
fi

# --- pawl gate (FAIL-CLOSED) -------------------------------------------------
# Green CI is necessary but NOT sufficient at the mutate-shared-trunk door
# (docs/contracts/pawls.md). Before merging, a CONFIRMED, EVIDENCE-BOUND,
# COMMIT-CURRENT pawl verdict (fresh-context default; multi-model opt-in) tied
# to THIS bead+PR MUST exist, with head_sha == the PR's CURRENT head. Absent /
# REFUTED / ESCALATE / STALE-head / no-evidence / schema-invalid => HOLD
# (exit 5): do NOT merge, do NOT close. Non-convergence (a tripped circuit
# breaker) surfaces for a human.
#
# Fetch the PR's CURRENT head sha so a verdict cannot be reused across a re-push:
# if a new commit landed after the panel reviewed, the verdict is STALE and the
# gate fail-closes. FAIL-CLOSED on the lookup itself: if `gh pr view headRefOid`
# fails or returns empty we CANNOT prove the verdict is commit-current, so we
# HOLD (exit 5) rather than call the gate with an empty head (which would skip
# the staleness comparison and let a stale verdict authorize a merge).
cur_head="$(gh pr view "$PR" --json headRefOid -q .headRefOid 2>/dev/null || true)"
if [[ -z "$cur_head" ]]; then
  echo "PAWL-HOLD: could not resolve current PR head (gh pr view headRefOid failed/empty) for PR=$PR — cannot prove the verdict is commit-current; did NOT merge, did NOT close. Fail-closed." >&2
  exit 5
fi
pawl_status=0
"$SCRIPT_DIR/pawl-verdict.sh" check "$BEAD" "$PR" --dir "$VERDICT_DIR" --head "$cur_head" || pawl_status=$?

# --- yield-ledger: gate-verdict (fail-open observability, NEVER a gate) -------
# Project the pawl-verdict into a bead-keyed yield event so the dynamo's Q/E
# gauges are computable from data (ag-grcz3). This emit fires for EVERY
# disposition (CONFIRMED | REFUTED | ESCALATE | HOLD) RIGHT AFTER the gate
# returns its verdict — BEFORE the exit-5-on-non-confirmed below — so Q's
# denominator (every attempted bead) and E (ESCALATE/HOLD count) are
# computable, not just CONFIRMED merges. Best-effort: every step is guarded so
# a missing `ao`/jq, an empty verdict (the no-verdict HOLD), or a malformed
# verdict cannot block the gate. difficulty + author_family are emit-time
# inputs the script does not know; the orchestrator supplies them via
# AO_YIELD_DIFFICULTY / AO_YIELD_AUTHOR_FAMILY.
emit_yield_gate_verdict() {
  command -v ao >/dev/null 2>&1 || return 0
  command -v jq >/dev/null 2>&1 || return 0
  local vfile="$VERDICT_DIR/$BEAD.json"
  [[ -s "$vfile" ]] || return 0
  local run_id="${AO_YIELD_RUN_ID:-reconcile-$BEAD}"
  local difficulty="${AO_YIELD_DIFFICULTY:-1}"
  local author_family="${AO_YIELD_AUTHOR_FAMILY:-unknown}"
  local body
  body="$(jq -c \
    --arg head "$cur_head" --argjson diff "$difficulty" --arg af "$author_family" '
    {
      difficulty: $diff,
      pawl_verdict_ref: {bead_id: .bead_id, head_sha: (.head_sha // $head)},
      disposition: .disposition,
      head_sha: (.head_sha // $head),
      attempt: (.attempt // 1),
      mode: (.mode // "fresh-context"),
      author_context_id: .author_context_id,
      refuter_families: ([.refuters[]?.family] | unique),
      author_family: $af,
      cross_family: (([.refuters[]?.family] | unique | length) >= 2),
      author_ne_reviewer: ([.refuters[]?.context_id] | index(.author_context_id) | not),
      evidence_present: ([.refuters[]?.evidence // empty] | length > 0)
    }' "$vfile" 2>/dev/null)" || return 0
  [[ -n "$body" ]] || return 0
  ao yield emit gate-verdict --bead "$BEAD" --run "$run_id" --json "$body" >/dev/null 2>&1 || true
}
# Run SYNCHRONOUSLY (not backgrounded): on the exit-5 path this emit must finish
# recording the REFUTED/ESCALATE/HOLD disposition before the script exits — that
# is the whole point of B3 (Q's denominator + E count). It is pre-merge, so it
# cannot delay a merge; the gate already gates the merge.
emit_yield_gate_verdict || true

if [[ "$pawl_status" -ne 0 ]]; then
  # yield-ledger: emit USAGE for the rejected/HOLD attempt too, so its spend is
  # recorded for L/R (ag-qzinh's read-time join classifies a never-accepted bead's
  # usage as loss). Symmetric to the accepted-path usage emit; without this,
  # rejected spend is invisible. Synchronous + guarded + pre-exit; this path never
  # merges, so a fast local append cannot delay anything. (codex review attempt-2 BLOCKING.)
  if command -v ao >/dev/null 2>&1; then
    ao yield emit usage --bead "$BEAD" --run "${AO_YIELD_RUN_ID:-reconcile-$BEAD}" \
      --json "{\"tokens_in\":${AO_YIELD_TOKENS_IN:-0},\"tokens_out\":${AO_YIELD_TOKENS_OUT:-0},\"cost_usd\":${AO_YIELD_COST_USD:-0},\"wall_clock_s\":${AO_YIELD_WALL_CLOCK_S:-0},\"model\":\"${AO_YIELD_MODEL:-unknown}\",\"phase\":\"${AO_YIELD_PHASE:-review}\"}" \
      >/dev/null 2>&1 || true
  fi
  echo "PAWL-HOLD: green CI but no CONFIRMED pawl verdict (fresh-context default; multi-model opt-in) for bead=$BEAD PR=$PR (pawl-verdict.sh check exit=$pawl_status) — did NOT merge, did NOT close. Fail-closed; surface for human on non-convergence." >&2
  exit 5
fi

# --- merge -------------------------------------------------------------------
if $DRY_RUN; then
  echo "DRY-RUN: would gh pr merge $PR --squash --admin, then bd update $BEAD --status closed" >&2
  exit 0
fi

gh pr merge "$PR" --squash --admin >/dev/null 2>&1 || true
sleep 3
state="$(gh pr view "$PR" --json state -q .state 2>/dev/null)"

if [[ "$state" != "MERGED" ]]; then
  echo "merge-FAILED: PR $PR state=$state (expected MERGED); bead NOT closed" >&2
  exit 3
fi

echo "MERGED: PR $PR" >&2

# --- yield-ledger: accept (fail-open observability, NEVER a gate) -------------
# Emit the terminal-accept event keyed by bead so the dynamo's A gauge is
# computable from data (ag-grcz3). The authorizing pawl is referenced by
# bead_id+head_sha; merge_sha falls back to the reviewed head_sha when the
# squash sha is not resolvable. Guarded — never blocks the close path.
emit_yield_accept() {
  command -v ao >/dev/null 2>&1 || return 0
  local run_id="${AO_YIELD_RUN_ID:-reconcile-$BEAD}"
  local merge_sha
  merge_sha="$(gh pr view "$PR" --json mergeCommit -q .mergeCommit.oid 2>/dev/null || true)"
  [[ -n "$merge_sha" ]] || merge_sha="$cur_head"
  ao yield emit accept --bead "$BEAD" --run "$run_id" \
    --json "{\"merge_sha\":\"$merge_sha\",\"merged_by\":\"reconcile-pr\",\"gate_verdict_ref\":{\"bead_id\":\"$BEAD\",\"head_sha\":\"$cur_head\"}}" \
    >/dev/null 2>&1 || true
}
# Post-decision (the merge is already confirmed MERGED above), so backgrounded:
# a slow/hung emit can never delay the close path.
emit_yield_accept & disown 2>/dev/null || true

# --- close bead (use `bd update --status closed`: bd close has a dolt --------
#     blocker-query glitch on this server) -----------------------------------
bd update "$BEAD" --status closed >/dev/null 2>&1 \
  || { echo "WARN: bead $BEAD close command failed" >&2; }
echo "CLOSED bead=$BEAD" >&2

# --- yield-ledger: usage (fail-open observability, NEVER a gate) --------------
# Emit a per-bead usage event so the dynamo's R / A-R / L gauges have an
# automated source (ag-grcz3). The orchestrator supplies per-bead spend via the
# AO_YIELD_* env (model + tokens/cost/wall-clock); absent metrics default to 0.
# phase is ALWAYS present (defaults to implement). Guarded — never blocks close.
emit_yield_usage() {
  command -v ao >/dev/null 2>&1 || return 0
  local run_id="${AO_YIELD_RUN_ID:-reconcile-$BEAD}"
  local model="${AO_YIELD_MODEL:-unknown}"
  local phase="${AO_YIELD_PHASE:-implement}"
  local tokens_in="${AO_YIELD_TOKENS_IN:-0}"
  local tokens_out="${AO_YIELD_TOKENS_OUT:-0}"
  local cost_usd="${AO_YIELD_COST_USD:-0}"
  local wall_clock_s="${AO_YIELD_WALL_CLOCK_S:-0}"
  local body
  if command -v jq >/dev/null 2>&1; then
    body="$(jq -nc \
      --argjson ti "$tokens_in" --argjson to "$tokens_out" \
      --argjson cost "$cost_usd" --argjson wall "$wall_clock_s" \
      --arg model "$model" --arg phase "$phase" '
      {tokens_in: $ti, tokens_out: $to, cost_usd: $cost, wall_clock_s: $wall, model: $model, phase: $phase}' \
      2>/dev/null)" || return 0
  else
    body="{\"tokens_in\":$tokens_in,\"tokens_out\":$tokens_out,\"cost_usd\":$cost_usd,\"wall_clock_s\":$wall_clock_s,\"model\":\"$model\",\"phase\":\"$phase\"}"
  fi
  [[ -n "$body" ]] || return 0
  ao yield emit usage --bead "$BEAD" --run "$run_id" --json "$body" >/dev/null 2>&1 || true
}
emit_yield_usage & disown 2>/dev/null || true

# --- optional epic reconcile -------------------------------------------------
if [[ -n "$EPIC" ]]; then
  if "$SCRIPT_DIR/check-epic-children-closed.sh" "$EPIC"; then
    bd update "$EPIC" --status closed >/dev/null 2>&1 \
      || { echo "WARN: epic $EPIC close command failed" >&2; }
    echo "CLOSED epic=$EPIC" >&2
  else
    echo "EPIC-GATE: $EPIC has open child(ren) — epic NOT closed" >&2
    exit 1
  fi
fi

exit 0
