#!/usr/bin/env bash
# cron-tune-cadence.sh — mechanize symmetric cron-cadence tuning for /evolve.
#
# Reads .agents/evolve/session-state.json + current repo state, computes a
# state-hash, updates a heartbeat streak, and prints a tuning recommendation:
#
#   TUNE_DOWN    state hasn't changed for N consecutive fires; cadence too tight
#   TUNE_UP      productive cycle happened with open work; cadence may be too slow
#   STAY         state changed naturally; no tune needed
#
# The agent reads the recommendation in cron-procedure step 6 and decides
# whether to CronDelete + CronCreate (cron tools are agent-only, not shell-
# callable). This script's job is to remove self-reported metrics from the
# decision — the streak counter is updated atomically here.
#
# Derivation: cycle 237 of 2026-05-20 — agent self-reported "9 heartbeats"
# when 7 was accurate. Judge B (council 220-240) flagged self-reported
# metrics in the same note arguing for self-edit. Mechanizing the count
# removes that failure mode.
#
# Symmetric tuning rule (from feedback_self_editing_cron.md):
#   * Heartbeat streak ≥ 3 with identical state-hash → TUNE_DOWN
#   * Productive cycle WITH queued in-flight workload → TUNE_UP
#   * Otherwise → STAY
#
# Bead: soc-adwq
#
# Usage:
#   cron-tune-cadence.sh <cycle-result>
#     cycle-result is one of: productive, heartbeat, blocked-on-failure, teardown
#
# Output (stdout): one of TUNE_DOWN, TUNE_UP, STAY
# Stderr: explanation + new streak value
# Exit code: 0 always (caller decides what to do)

set -euo pipefail

CYCLE_RESULT="${1:-}"

if [[ -z "$CYCLE_RESULT" ]]; then
  echo "usage: $0 <cycle-result>" >&2
  echo "  cycle-result ∈ {productive, heartbeat, blocked-on-failure, teardown}" >&2
  exit 2
fi

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo "$PWD")"
STATE_FILE="$REPO_ROOT/.agents/evolve/session-state.json"
HEARTBEAT_THRESHOLD="${CRON_TUNE_HEARTBEAT_THRESHOLD:-3}"

if [[ ! -f "$STATE_FILE" ]]; then
  echo "cron-tune-cadence: $STATE_FILE missing — cannot decide; default STAY" >&2
  echo "STAY"
  exit 0
fi

# Compute state-hash = sha1(main_sha + sorted batch_pr_states)
# Tolerate missing git context (tests run from non-git dirs).
MAIN_SHA="$(git rev-parse origin/main 2>/dev/null || git rev-parse HEAD 2>/dev/null || echo "no-git")"
BATCH_PRS="$(jq -r '.batch_prs[]? // empty' "$STATE_FILE" 2>/dev/null || true)"

PR_STATES=""
if [[ -n "$BATCH_PRS" ]]; then
  if command -v gh >/dev/null 2>&1; then
    while IFS= read -r pr; do
      [[ -z "$pr" ]] && continue
      STATE_LINE="$(gh pr view "$pr" --json state,mergeStateStatus --jq '"\(.state) \(.mergeStateStatus)"' 2>/dev/null || echo "UNKNOWN UNKNOWN")"
      PR_STATES="${PR_STATES}${pr}:${STATE_LINE}"$'\n'
    done <<< "$BATCH_PRS"
  fi
fi

# Sorted, deterministic
PR_STATES_SORTED="$(printf '%s' "$PR_STATES" | sort)"
STATE_HASH="$(printf '%s\n%s' "$MAIN_SHA" "$PR_STATES_SORTED" | sha1sum | awk '{print $1}')"

# Read prior values
PREV_HASH="$(jq -r '.state_hash // ""' "$STATE_FILE")"
PREV_STREAK="$(jq -r '.heartbeat_streak // 0' "$STATE_FILE")"
OPEN_PR_COUNT="$(printf '%s' "$BATCH_PRS" | wc -l | tr -d ' ')"

# Decide
RECOMMENDATION="STAY"
NEW_STREAK=0
REASON=""

case "$CYCLE_RESULT" in
  productive|teardown)
    NEW_STREAK=0
    if [[ "$CYCLE_RESULT" == "productive" ]] && [[ "$OPEN_PR_COUNT" -gt 0 ]]; then
      RECOMMENDATION="TUNE_UP"
      REASON="productive cycle + ${OPEN_PR_COUNT} open PRs remain → suggest cadence faster"
    else
      REASON="productive/teardown — streak reset, no tune"
    fi
    ;;
  heartbeat|blocked-on-failure)
    if [[ "$STATE_HASH" == "$PREV_HASH" ]]; then
      NEW_STREAK=$((PREV_STREAK + 1))
      if [[ "$NEW_STREAK" -ge "$HEARTBEAT_THRESHOLD" ]]; then
        RECOMMENDATION="TUNE_DOWN"
        REASON="heartbeat_streak=$NEW_STREAK >= $HEARTBEAT_THRESHOLD with identical state-hash → suggest cadence slower"
      else
        REASON="heartbeat (streak=$NEW_STREAK < $HEARTBEAT_THRESHOLD) — no tune yet"
      fi
    else
      NEW_STREAK=1
      REASON="state-hash changed — streak reset to 1, no tune"
    fi
    ;;
  *)
    REASON="unknown cycle-result '$CYCLE_RESULT' — no tune"
    ;;
esac

# Persist new state atomically
TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT
jq --arg hash "$STATE_HASH" --argjson streak "$NEW_STREAK" \
   --arg rec "$RECOMMENDATION" --arg ts "$(date -u +%FT%TZ)" \
   '. + {state_hash: $hash, heartbeat_streak: $streak, last_tune_recommendation: $rec, last_tune_check: $ts}' \
   "$STATE_FILE" > "$TMP"
mv "$TMP" "$STATE_FILE"
trap - EXIT

echo "cron-tune-cadence: $REASON" >&2
echo "cron-tune-cadence: state_hash=${STATE_HASH:0:8} streak=$NEW_STREAK rec=$RECOMMENDATION" >&2
echo "$RECOMMENDATION"
