#!/usr/bin/env bash
# auto-resume-stale-claims.sh — periodic stale-claim recovery driver.
#
# Slice 4a of soc-vuu6.27 (fungible-swarm death recovery). Composes the
# read-only listing from slice 2 (`ao beads stale-claims`) with the atomic
# transfer from slice 3 (`ao beads resume`) into a single one-shot driver
# suitable for cron, systemd timers, or future agentopsd job-spec wrapping
# (slice 4b).
#
# Behavior:
#   1. Call `ao beads stale-claims --threshold=<N>h --json`.
#   2. For each stale bead in the output, call `ao beads resume <id>
#      --agent <recovery-agent>`.
#   3. Print a per-bead OK/FAIL line. Exit code summarizes the run.
#
# Flags:
#   --threshold <hours>    staleness threshold passed to slice 2 (default 4)
#   --agent <id>           new claimant id (defaults to BEADS_ACTOR env)
#   --dry-run              print intended transfers but don't act
#   --max <n>              cap transfers per run (default 25)
#   --quiet                only print FAIL lines + summary
#
# Exit codes:
#   0 — at least one transfer succeeded (or zero candidates found)
#   1 — at least one transfer failed
#   2 — usage / environment error

set -euo pipefail

THRESHOLD=4
AGENT=""
DRY_RUN=0
MAX_TRANSFERS=25
QUIET=0

usage() {
  sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --threshold) shift; THRESHOLD="${1:-4}" ;;
    --agent)     shift; AGENT="${1:-}" ;;
    --dry-run)   DRY_RUN=1 ;;
    --max)       shift; MAX_TRANSFERS="${1:-25}" ;;
    --quiet)     QUIET=1 ;;
    -h|--help)   usage 0 ;;
    *)           echo "auto-resume-stale-claims: unknown arg: $1" >&2; usage 2 ;;
  esac
  shift || true
done

if ! command -v ao >/dev/null 2>&1; then
  echo "auto-resume-stale-claims: ao CLI not on PATH" >&2
  exit 2
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "auto-resume-stale-claims: jq required" >&2
  exit 2
fi

log_unless_quiet() {
  [ "$QUIET" -eq 1 ] || echo "$@"
}

# Step 1: enumerate stale claims as JSON.
STALE_JSON="$(ao beads stale-claims --threshold="${THRESHOLD}" --json 2>/dev/null || echo '[]')"
COUNT="$(printf '%s' "$STALE_JSON" | jq 'length' 2>/dev/null || echo 0)"

if [ "$COUNT" -eq 0 ]; then
  log_unless_quiet "auto-resume-stale-claims: no stale claims (threshold ${THRESHOLD}h)"
  exit 0
fi

log_unless_quiet "auto-resume-stale-claims: $COUNT stale claim(s) at threshold ${THRESHOLD}h"

# Step 2: transfer each, up to MAX_TRANSFERS.
SUCCEED=0
FAIL=0
SKIP=0
i=0

# Collect bead ids into a newline-separated list so we can loop without subshell pitfalls.
BEAD_IDS="$(printf '%s' "$STALE_JSON" | jq -r '.[].bead_id')"

while IFS= read -r bead_id; do
  [ -z "$bead_id" ] && continue
  i=$((i + 1))
  if [ "$i" -gt "$MAX_TRANSFERS" ]; then
    SKIP=$((SKIP + 1))
    continue
  fi
  if [ "$DRY_RUN" -eq 1 ]; then
    log_unless_quiet "  DRY-RUN $bead_id"
    SUCCEED=$((SUCCEED + 1))
    continue
  fi
  AGENT_ARG=()
  if [ -n "$AGENT" ]; then
    AGENT_ARG=(--agent "$AGENT")
  fi
  if ao beads resume "$bead_id" "${AGENT_ARG[@]}" >/dev/null 2>&1; then
    log_unless_quiet "  OK   $bead_id"
    SUCCEED=$((SUCCEED + 1))
  else
    echo "  FAIL $bead_id" >&2
    FAIL=$((FAIL + 1))
  fi
done <<EOF
$BEAD_IDS
EOF

echo "auto-resume-stale-claims: ${SUCCEED} succeeded, ${FAIL} failed, ${SKIP} over-cap"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
exit 0
