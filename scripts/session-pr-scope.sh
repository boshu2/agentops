#!/usr/bin/env bash
# session-pr-scope.sh — hookless session-PR-scope signal (ag-o5xp).
#
# Supersedes the removed PreToolUse hook `hooks/session-pr-counter.sh` (PR #362,
# soc-1aou), which was deleted in the 3.0 hookless teardown (#511). AgentOps 3.0
# ships no hooks; this is the hookless replacement for the session-scope count.
#
# Counts PRs the current GitHub user opened within a trailing window and emits a
# session-scope verdict (OK | WARN | BLOCK) against SESSION_PR_THRESHOLD. It is the
# canonical session-PR count source:
#   - the /evolve post-mortem checkpoint reads it for $session_pr_count
#     (skills/evolve/references/postmortem-checkpoint.md);
#   - anyone wanting the old pre-creation signal can wrap it in an opt-in hook
#     authored via the hooks-authoring skill (AgentOps ships none by default);
#   - it can run manually or as a warn-only CI step.
#
# Fail-open: advisory tooling must never block on infrastructure failure. If `gh`
# or `jq` is unavailable, or the count cannot be parsed, it exits 0 with
# verdict=unknown.
#
# Usage:
#   scripts/session-pr-scope.sh            # human-readable verdict
#   scripts/session-pr-scope.sh --json     # {count,threshold,window_hours,verdict,over,block_mode}
#   scripts/session-pr-scope.sh --count    # just the integer count (for $session_pr_count)
#
# Env:
#   SESSION_PR_THRESHOLD                (default 5)   — post-mortem checkpoint threshold
#   AGENTOPS_SESSION_PR_WINDOW_HOURS    (default 24)  — "current session" window
#   AGENTOPS_SESSION_PR_BLOCK           (unset)       — when =1, verdict BLOCK + exit 2 at count >= threshold
set -uo pipefail

THRESHOLD="${SESSION_PR_THRESHOLD:-5}"
WINDOW_HOURS="${AGENTOPS_SESSION_PR_WINDOW_HOURS:-24}"
BLOCK_MODE="${AGENTOPS_SESSION_PR_BLOCK:-}"

MODE="human"
case "${1:-}" in
  --json)  MODE="json" ;;
  --count) MODE="count" ;;
  ""|--human) MODE="human" ;;
  -h|--help) sed -n '2,33p' "$0"; exit 0 ;;
  *) printf 'unknown arg: %s\n' "$1" >&2; exit 64 ;;
esac

emit_unknown() {
  case "$MODE" in
    json)  printf '{"count":null,"threshold":%s,"window_hours":%s,"verdict":"unknown","over":false,"block_mode":%s}\n' \
             "$THRESHOLD" "$WINDOW_HOURS" "$([ "$BLOCK_MODE" = 1 ] && echo true || echo false)" ;;
    count) printf '0\n' ;;
    human) printf 'session-pr-scope: verdict=unknown (gh/jq unavailable or unparseable; failing open)\n' ;;
  esac
  exit 0
}

command -v gh >/dev/null 2>&1 || emit_unknown
command -v jq >/dev/null 2>&1 || emit_unknown

SINCE_ISO="$(date -u -d "${WINDOW_HOURS} hours ago" +%FT%TZ 2>/dev/null || date -u +%FT%TZ)"
COUNT="$(gh pr list --search "author:@me created:>=${SINCE_ISO}" --state all --limit 100 --json number 2>/dev/null | jq -r 'length' 2>/dev/null)"

case "$COUNT" in
  ''|*[!0-9]*) emit_unknown ;;
esac

# Verdict: WARN once the count reaches threshold-1 (the next PR tips the session
# over); BLOCK only when over the threshold AND block mode is opted in.
WARN_AT=$((THRESHOLD - 1))
verdict="ok"
over=false
exit_code=0
if [ "$COUNT" -ge "$THRESHOLD" ]; then
  over=true
  if [ "$BLOCK_MODE" = "1" ]; then verdict="block"; exit_code=2; else verdict="warn"; fi
elif [ "$COUNT" -ge "$WARN_AT" ]; then
  verdict="warn"
fi

reminder="Session-scope: ${COUNT} PR(s) in the last ${WINDOW_HOURS}h (threshold ${THRESHOLD}). Run a real post-mortem (/post-mortem --deep, council-gated) before the next PR — reactive-PR spirals are the dominant back-half failure mode (soc-waxr). The /evolve loop enforces this mechanically at checkpoint #6."

case "$MODE" in
  count) printf '%s\n' "$COUNT"; exit 0 ;;
  json)
    printf '{"count":%s,"threshold":%s,"window_hours":%s,"verdict":"%s","over":%s,"block_mode":%s}\n' \
      "$COUNT" "$THRESHOLD" "$WINDOW_HOURS" "$verdict" "$over" \
      "$([ "$BLOCK_MODE" = 1 ] && echo true || echo false)"
    exit "$exit_code" ;;
  human)
    case "$verdict" in
      ok)    printf 'session-pr-scope: OK — %s PR(s) in %sh (threshold %s)\n' "$COUNT" "$WINDOW_HOURS" "$THRESHOLD" ;;
      warn)  printf 'session-pr-scope: WARN — %s\n' "$reminder" ;;
      block) printf 'session-pr-scope: BLOCK — %s\n' "$reminder" >&2 ;;
    esac
    exit "$exit_code" ;;
esac
