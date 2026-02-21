#!/bin/bash
# pre-mortem-gate.sh - PreToolUse hook: block /crank when epic has 3+ issues and no pre-mortem
# Evidence: 6/6 consecutive positive pre-mortem ROI across epics.
#
# Pre-mortem evidence is checked in order:
#   1. Run-scoped phased-state.json (verdicts.pre_mortem) — avoids false positives from stale globals
#   2. Run-scoped orchestration log  — inline verdict entries
#   3. Council artifacts scoped to this epic or today — session-local fallback
#   4. Ratchet chain (chain.jsonl)   — legacy/non-phased runs

# Kill switches
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0
[ "${AGENTOPS_SKIP_PRE_MORTEM_GATE:-}" = "1" ] && exit 0
AO_TIMEOUT_BIN="timeout"
command -v "$AO_TIMEOUT_BIN" >/dev/null 2>&1 || AO_TIMEOUT_BIN="gtimeout"

run_ao_quick() {
    if command -v "$AO_TIMEOUT_BIN" >/dev/null 2>&1; then
        "$AO_TIMEOUT_BIN" "${AGENTOPS_PRE_MORTEM_GATE_TIMEOUT:-2}" ao "$@" 2>/dev/null
        return $?
    fi
    ao "$@" 2>/dev/null
}

# Workers are exempt
[ "${AGENTOPS_WORKER:-}" = "1" ] && exit 0

# Read all stdin
INPUT=$(cat)

# Extract tool name and args
if command -v jq >/dev/null 2>&1; then
    TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // ""' 2>/dev/null)
    SKILL_NAME=$(echo "$INPUT" | jq -r '.tool_input.skill // ""' 2>/dev/null)
    SKILL_ARGS=$(echo "$INPUT" | jq -r '.tool_input.args // ""' 2>/dev/null)
else
    # Fallback without jq
    TOOL_NAME=$(echo "$INPUT" | grep -o '"tool_name"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"tool_name"[[:space:]]*:[[:space:]]*"//;s/"$//')
    SKILL_NAME=$(echo "$INPUT" | grep -o '"skill"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"skill"[[:space:]]*:[[:space:]]*"//;s/"$//')
    SKILL_ARGS=$(echo "$INPUT" | grep -o '"args"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"args"[[:space:]]*:[[:space:]]*"//;s/"$//')
fi

# Only gate Skill tool calls for crank
[ "$TOOL_NAME" = "Skill" ] || exit 0
echo "$SKILL_NAME" | grep -qiE '^(crank|agentops:crank)$' || exit 0

# Check for --skip-pre-mortem bypass in args
echo "$SKILL_ARGS" | grep -q "\-\-skip-pre-mortem" && exit 0

# Extract epic-id from args (first arg that looks like a bead ID)
EPIC_ID=$(echo "$SKILL_ARGS" | grep -oE '[a-z]{2}-[a-z0-9]+' | head -1)
[ -z "$EPIC_ID" ] && exit 0  # No epic ID found, can't check — fail open

# Count children
if ! command -v bd &>/dev/null; then
    exit 0  # No bd CLI, can't count — fail open
fi
CHILD_COUNT=$(bd children "$EPIC_ID" 2>/dev/null | wc -l | tr -d ' ')
[ "$CHILD_COUNT" -lt 3 ] && exit 0  # Less than 3 issues, no gate needed

# --- Method 1: Run-scoped phased-state.json (canonical, avoids stale globals) ---
ROOT=$(git rev-parse --show-toplevel 2>/dev/null || echo ".")
ROOT="$(cd "$ROOT" 2>/dev/null && pwd -P 2>/dev/null || printf '%s' "$ROOT")"
PHASED_STATE="$ROOT/.agents/rpi/phased-state.json"

if [ -f "$PHASED_STATE" ]; then
    if command -v jq >/dev/null 2>&1; then
        PM_VERDICT=$(jq -r '.verdicts.pre_mortem // ""' "$PHASED_STATE" 2>/dev/null)
    else
        PM_VERDICT=$(grep -o '"pre_mortem"[[:space:]]*:[[:space:]]*"[^"]*"' "$PHASED_STATE" 2>/dev/null | sed 's/.*"pre_mortem"[[:space:]]*:[[:space:]]*"//;s/"$//')
    fi
    if [ -n "$PM_VERDICT" ] && [ "$PM_VERDICT" != "null" ]; then
        exit 0
    fi

    # Also check run_id to scope orchestration log search
    if command -v jq >/dev/null 2>&1; then
        RUN_ID=$(jq -r '.run_id // ""' "$PHASED_STATE" 2>/dev/null)
    else
        RUN_ID=$(grep -o '"run_id"[[:space:]]*:[[:space:]]*"[^"]*"' "$PHASED_STATE" 2>/dev/null | head -1 | sed 's/.*"run_id"[[:space:]]*:[[:space:]]*"//;s/"$//')
    fi
fi

# --- Method 2: Run-scoped orchestration log ---
LOG_FILE="$ROOT/.agents/rpi/phased-orchestration.log"
if [ -f "$LOG_FILE" ] && [ -n "${RUN_ID:-}" ]; then
    if grep -qE "\[${RUN_ID}\].*pre-mortem verdict:" "$LOG_FILE" 2>/dev/null; then
        exit 0
    fi
elif [ -f "$LOG_FILE" ]; then
    # No run_id: scan log for any recent pre-mortem verdict
    TODAY=$(date +%Y-%m-%d)
    if grep -qE "\[$TODAY" "$LOG_FILE" 2>/dev/null && grep -qE "pre-mortem verdict:" "$LOG_FILE" 2>/dev/null; then
        exit 0
    fi
fi

# --- Method 3: Council artifacts scoped to epic or today (session-local fallback) ---
# Method 3a: Epic-scoped council artifact
if ls "$ROOT/.agents/council/"*-pre-mortem-"$EPIC_ID"* >/dev/null 2>&1 || \
   ls "$ROOT/.agents/council/"*-pre-mortem-"${EPIC_ID%%.*}"* >/dev/null 2>&1; then
    exit 0
fi
# Method 3b: Any pre-mortem from today (same session)
TODAY=$(date +%Y-%m-%d)
if ls "$ROOT/.agents/council/$TODAY"-*pre-mortem* >/dev/null 2>&1; then
    exit 0
fi

# --- Method 4: Ratchet chain (legacy/non-phased runs) ---
if command -v ao &>/dev/null; then
    if run_ao_quick ratchet status -o json | grep -q '"pre-mortem"'; then
        exit 0
    fi
fi

# No evidence found — block
# Source hook-helpers from plugin install dir, not repo root (security: prevents malicious repo sourcing)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/hook-helpers.sh
. "$SCRIPT_DIR/../lib/hook-helpers.sh"

LOG_DIR="$ROOT/.agents/ao"
mkdir -p "$LOG_DIR" 2>/dev/null
echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) GATE_BLOCK: pre-mortem-gate blocked crank for $EPIC_ID ($CHILD_COUNT children)" >> "$LOG_DIR/hook-errors.log" 2>/dev/null
write_failure "pre_mortem_gate" "bd children $EPIC_ID" 2 "Epic $EPIC_ID has $CHILD_COUNT issues, no pre-mortem evidence found"

cat >&2 <<EOMSG
BLOCKED: Epic $EPIC_ID has $CHILD_COUNT issues. Pre-mortem is mandatory for 3+ issue epics.
(6/6 consecutive positive ROI — this gate prevents implementation waste.)

Options:
  1. /pre-mortem                         -- run pre-mortem validation
  2. /crank $EPIC_ID --skip-pre-mortem   -- bypass with justification
EOMSG
exit 2
