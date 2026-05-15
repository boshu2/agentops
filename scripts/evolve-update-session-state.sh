#!/usr/bin/env bash
set -euo pipefail

# Derive .agents/evolve/session-state.json from cycle-history.jsonl tail.
#
# session-state.json is *resume-only* state — derived signals computed from
# the canonical cycle ledger. Running this after every cycle-history write
# keeps state coherent without depending on Claude remembering to update it.
#
# Computed fields:
#   cycle                    last cycle number in history
#   last_result              last entry's result
#   last_mode                last entry's mode
#   last_selected_source     last entry's selected_source
#   idle_streak              trailing run of result in {idle, unchanged}
#   mode_repeat_streak       trailing run of identical mode values
#   updated_at               this script's run timestamp (ISO 8601 UTC)
#
# Preserves any other keys already present in session-state.json (skill-owned
# fields like program_path, validation_commands, generator_empty_streak).

usage() {
  cat <<'EOF'
Usage: scripts/evolve-update-session-state.sh [options]

Derive session-state.json fields from cycle-history.jsonl tail.

Options:
  --history <path>        History file (default: .agents/evolve/cycle-history.jsonl)
  --state <path>          State file   (default: .agents/evolve/session-state.json)
  --print                 Print the merged state to stdout in addition to writing
  -h, --help              Show help

Exit codes:
  0  state written
  1  invalid args or unwritable destination
  2  history file missing or empty (state untouched)
EOF
}

HISTORY=".agents/evolve/cycle-history.jsonl"
STATE=".agents/evolve/session-state.json"
PRINT=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --history) HISTORY="${2:-}"; shift 2 ;;
    --state)   STATE="${2:-}";   shift 2 ;;
    --print)   PRINT=true;       shift   ;;
    -h|--help) usage; exit 0 ;;
    *) echo "ERROR: unknown option: $1" >&2; usage >&2; exit 1 ;;
  esac
done

command -v jq >/dev/null 2>&1 || { echo "ERROR: jq required" >&2; exit 1; }

if [[ ! -s "$HISTORY" ]]; then
  echo "WARN: history missing or empty: $HISTORY (state untouched)" >&2
  exit 2
fi

mkdir -p "$(dirname "$STATE")"

# Read the last well-formed JSON line.
LAST="$(awk 'NF { line = $0 } END { print line }' "$HISTORY")"
printf '%s' "$LAST" | jq -e . >/dev/null 2>&1 || {
  echo "ERROR: last history line is not valid JSON" >&2
  exit 1
}

CYCLE="$(printf '%s' "$LAST" | jq -r '.cycle // empty')"
LAST_RESULT="$(printf '%s' "$LAST" | jq -r '.result // ""')"
LAST_MODE="$(printf '%s' "$LAST" | jq -r '.mode // ""')"
LAST_SOURCE="$(printf '%s' "$LAST" | jq -r '.selected_source // ""')"
UPDATED_AT="$(date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -Iseconds)"

# Trailing run of idle/unchanged results.
IDLE_STREAK="$(
  awk -F'"result"[[:space:]]*:[[:space:]]*"' '
    NF > 1 {
      v = $2; sub(/".*/, "", v)
      if (v == "idle" || v == "unchanged") streak++
      else streak = 0
    }
    END { print streak + 0 }
  ' "$HISTORY"
)"

# Trailing run of identical mode values.
MODE_REPEAT_STREAK="$(
  awk -F'"mode"[[:space:]]*:[[:space:]]*"' -v last="$LAST_MODE" '
    NF > 1 {
      v = $2; sub(/".*/, "", v)
      lines[NR] = v
      total = NR
    }
    END {
      if (last == "") { print 0; exit }
      streak = 0
      for (i = total; i >= 1; i--) {
        if (lines[i] == last) streak++
        else break
      }
      print streak
    }
  ' "$HISTORY"
)"

# Load existing state (or empty object) and merge derived fields on top.
EXISTING="{}"
if [[ -s "$STATE" ]]; then
  if jq -e . "$STATE" >/dev/null 2>&1; then
    EXISTING="$(cat "$STATE")"
  else
    echo "WARN: existing state is not valid JSON, rebuilding from scratch" >&2
  fi
fi

MERGED="$(
  jq -n \
    --argjson existing "$EXISTING" \
    --argjson cycle "${CYCLE:-0}" \
    --arg last_result "$LAST_RESULT" \
    --arg last_mode "$LAST_MODE" \
    --arg last_source "$LAST_SOURCE" \
    --argjson idle_streak "${IDLE_STREAK:-0}" \
    --argjson mode_repeat_streak "${MODE_REPEAT_STREAK:-0}" \
    --arg updated_at "$UPDATED_AT" \
    '$existing + {
       cycle: $cycle,
       last_result: $last_result,
       last_mode: $last_mode,
       last_selected_source: $last_source,
       idle_streak: $idle_streak,
       mode_repeat_streak: $mode_repeat_streak,
       updated_at: $updated_at
     }'
)"

TMP="$(mktemp "${STATE}.XXXXXX")"
printf '%s\n' "$MERGED" >"$TMP"
mv "$TMP" "$STATE"

if [[ "$PRINT" == true ]]; then
  printf '%s\n' "$MERGED"
fi
