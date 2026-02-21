#!/bin/bash
# PreCompact hook: snapshot team state before context compaction
# Captures active teams, git status, branch info for recovery after compaction
# Fail-open: all errors are non-fatal, always exit 0

ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
ROOT="$(cd "$ROOT" 2>/dev/null && pwd -P 2>/dev/null || printf '%s' "$ROOT")"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK_HELPERS_LIB="$SCRIPT_DIR/../lib/hook-helpers.sh"
if [ -f "$HOOK_HELPERS_LIB" ]; then
  # shellcheck source=../lib/hook-helpers.sh
  . "$HOOK_HELPERS_LIB"
fi
AO_DIR="$ROOT/.agents/ao"
LOG_FILE="$AO_DIR/hook-errors.log"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HELPERS_LIB="$SCRIPT_DIR/../lib/hook-helpers.sh"

# Kill switches
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0
[ "${AGENTOPS_PRECOMPACT_DISABLED:-}" = "1" ] && exit 0

if [ -f "$HELPERS_LIB" ]; then
  # shellcheck source=../lib/hook-helpers.sh
  . "$HELPERS_LIB"
else
  timeout_run() {
    local seconds="$1"
    shift
    if command -v timeout >/dev/null 2>&1; then
      timeout "$seconds" "$@"
    elif command -v gtimeout >/dev/null 2>&1; then
      gtimeout "$seconds" "$@"
    else
      "$@"
    fi
  }
fi

log_error() {
  mkdir -p "$AO_DIR" 2>/dev/null || return 0
  echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) precompact-snapshot: $1" >> "$LOG_FILE" 2>/dev/null || true
}

TEAMS_DIR="$HOME/.claude/teams"
AGENTS_DIR="$ROOT/.agents"
SNAP_DIR="$ROOT/.agents/compaction-snapshots"

# Check if there's anything worth snapshotting
has_teams=false
has_agents=false
[[ -d "$TEAMS_DIR" ]] && ls "$TEAMS_DIR"/*/config.json >/dev/null 2>&1 && has_teams=true
[[ -d "$AGENTS_DIR" ]] && has_agents=true

if ! $has_teams && ! $has_agents; then
  exit 0
fi

# Create snapshot directory
mkdir -p "$SNAP_DIR" 2>/dev/null || {
  log_error "unable to create snapshot directory: $SNAP_DIR"
  exit 0
}

TIMESTAMP=$(date -u +%Y%m%dT%H%M%SZ)
SNAP_FILE="$SNAP_DIR/${TIMESTAMP}.md"

# Gather data
BRANCH=$(git branch --show-current 2>/dev/null || echo "unknown")
GIT_STATUS=$(git status --short 2>/dev/null | head -20)
GIT_DIFF_STAT=$(git diff --stat 2>/dev/null | tail -5)

TEAM_NAMES=""
if $has_teams; then
  for cfg in "$TEAMS_DIR"/*/config.json; do
    tname=$(basename "$(dirname "$cfg")")
    TEAM_NAMES="${TEAM_NAMES:+$TEAM_NAMES, }$tname"
  done
fi

# Write snapshot file
{
  echo "# Compaction Snapshot"
  echo ""
  echo "**Timestamp:** $TIMESTAMP"
  echo "**Branch:** $BRANCH"
  echo ""
  if [[ -n "$TEAM_NAMES" ]]; then
    echo "## Active Teams"
    echo "$TEAM_NAMES"
    echo ""
  fi
  if [[ -n "$GIT_STATUS" ]]; then
    echo "## Git Status"
    echo '```'
    echo "$GIT_STATUS"
    echo '```'
    echo ""
  fi
  if [[ -n "$GIT_DIFF_STAT" ]]; then
    echo "## Diff Stat"
    echo '```'
    echo "$GIT_DIFF_STAT"
    echo '```'
  fi
} > "$SNAP_FILE" 2>/dev/null || log_error "failed writing snapshot: $SNAP_FILE"

# Build compact summary for additionalContext (<500 bytes)
STATUS_COUNT=$(printf '%s\n' "$GIT_STATUS" | sed '/^$/d' | wc -l | tr -d ' ')
SUMMARY="branch=$BRANCH teams=[$TEAM_NAMES] files_changed=$STATUS_COUNT snapshot=$TIMESTAMP"
# Remove line breaks to keep JSON payload valid.
SUMMARY=$(printf '%s' "$SUMMARY" | tr '\n\r' '  ')
# Truncate to stay under 500 bytes
SUMMARY="${SUMMARY:0:480}"

# Generate auto-handoff before cleanup
HANDOFF_DIR="$ROOT/.agents/handoff"
mkdir -p "$HANDOFF_DIR" 2>/dev/null || log_error "unable to create handoff directory: $HANDOFF_DIR"

HANDOFF_TS=$(date -u +%Y-%m-%dT%H:%M:%SZ)
HANDOFF_TS_SAFE=$(date -u +%Y%m%dT%H%M%SZ)
HANDOFF_FILE="$HANDOFF_DIR/auto-${HANDOFF_TS_SAFE}.md"

# Gather handoff data (all with 1s timeout to stay under 2s budget)
RATCHET_STATE=$(timeout_run 1 ao ratchet status -o json 2>/dev/null || echo "")
ACTIVE_BEAD=$(timeout_run 1 bd current 2>/dev/null || echo "")
MODIFIED_FILES=$(git diff --name-only HEAD 2>/dev/null | head -20)

# Write auto-handoff document
{
  echo "# Auto-Handoff (Pre-Compaction)"
  echo "**Timestamp:** $HANDOFF_TS"
  echo "**Branch:** $BRANCH"
  echo ""
  echo "## Ratchet State"
  if [[ -n "$RATCHET_STATE" ]]; then
    echo '```json'
    echo "$RATCHET_STATE"
    echo '```'
  else
    echo "no active cycle"
  fi
  echo ""
  echo "## Active Work"
  if [[ -n "$ACTIVE_BEAD" ]]; then
    echo "$ACTIVE_BEAD"
  else
    echo "none"
  fi
  echo ""
  echo "## Modified Files"
  if [[ -n "$MODIFIED_FILES" ]]; then
    echo '```'
    echo "$MODIFIED_FILES"
    echo '```'
  else
    echo "none"
  fi
  echo ""
  echo "## Active Teams"
  if [[ -n "$TEAM_NAMES" ]]; then
    echo "$TEAM_NAMES"
  else
    echo "none"
  fi
} > "$HANDOFF_FILE" 2>/dev/null || log_error "failed writing auto-handoff: $HANDOFF_FILE"

HANDOFF_REL=""
if [ -f "$HANDOFF_FILE" ] && command -v to_repo_relative_path >/dev/null 2>&1; then
  HANDOFF_REL=$(to_repo_relative_path "$HANDOFF_FILE")
fi

PACKET_PAYLOAD='{}'
if command -v jq >/dev/null 2>&1; then
  PACKET_PAYLOAD=$(jq -n \
    --arg summary "$SUMMARY" \
    --arg branch "$BRANCH" \
    --arg teams "$TEAM_NAMES" \
    --arg bead "$ACTIVE_BEAD" \
    --arg ratchet "$RATCHET_STATE" \
    '{summary:$summary,branch:$branch,active_teams:$teams,active_bead:$bead,ratchet_state:$ratchet}')
fi

if command -v write_memory_packet >/dev/null 2>&1; then
  write_memory_packet "precompact" "precompact-snapshot" "$PACKET_PAYLOAD" "$HANDOFF_REL" >/dev/null 2>&1 || true
fi

# Output JSON for hook system
if command -v jq >/dev/null 2>&1; then
  jq -n --arg summary "$SUMMARY" '{"hookSpecificOutput":{"additionalContext":$summary}}'
else
  safe_summary=${SUMMARY//\\/\\\\}
  safe_summary=${safe_summary//\"/\\\"}
  echo "{\"hookSpecificOutput\":{\"additionalContext\":\"$safe_summary\"}}"
fi

# Cleanup: keep last 5 snapshots, remove older
if [[ -d "$SNAP_DIR" ]]; then
  # shellcheck disable=SC2012
  ls -1t "$SNAP_DIR"/*.md 2>/dev/null | tail -n +6 | while read -r old; do
    rm -f "$old" 2>/dev/null
  done
fi

exit 0
