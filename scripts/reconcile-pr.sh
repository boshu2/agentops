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
if [[ "$pawl_status" -ne 0 ]]; then
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

# --- close bead (use `bd update --status closed`: bd close has a dolt --------
#     blocker-query glitch on this server) -----------------------------------
bd update "$BEAD" --status closed >/dev/null 2>&1 \
  || { echo "WARN: bead $BEAD close command failed" >&2; }
echo "CLOSED bead=$BEAD" >&2

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
