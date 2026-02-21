#!/bin/bash
# push-gate.sh - PreToolUse hook: block git push/tag when vibe not completed
# Gates on RPI ratchet state. git commit is NOT blocked (local, reversible).
# Cold start (no chain.jsonl and no phased-state.json) = no enforcement.
#
# Evidence check order:
#   1. Run-scoped phased-state.json (verdicts) — avoids false positives from stale chain entries
#   2. Global chain.jsonl — legacy/non-phased runs

# Kill switch
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0

# Workers are exempt (they should never push anyway)
[ "${AGENTOPS_WORKER:-}" = "1" ] && exit 0

# Read all stdin
INPUT=$(cat)

# Extract tool_input.command from JSON
if command -v jq >/dev/null 2>&1; then
    CMD=$(echo "$INPUT" | jq -r '.tool_input.command // ""' 2>/dev/null)
else
    CMD=$(echo "$INPUT" | grep -o '"command"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"command"[[:space:]]*:[[:space:]]*"//;s/"$//')
fi

# No command → pass through
[ -z "$CMD" ] || [ "$CMD" = "null" ] && exit 0

# Hot path: only care about git push/tag (<50ms for non-git commands)
echo "$CMD" | grep -qE 'git\s+(push|tag)' || exit 0

# Find repo root
ROOT=$(git rev-parse --show-toplevel 2>/dev/null)
if [ -z "$ROOT" ]; then
    # Not in a git repo — can't enforce, fail open
    exit 0
fi
ROOT="$(cd "$ROOT" 2>/dev/null && pwd -P 2>/dev/null || printf '%s' "$ROOT")"
# Source hook-helpers from plugin install dir, not repo root (security: prevents malicious repo sourcing)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HELPERS_LIB="$SCRIPT_DIR/../lib/hook-helpers.sh"
CHAIN_LIB="$SCRIPT_DIR/../lib/chain-parser.sh"

if [ -f "$HELPERS_LIB" ]; then
    # shellcheck source=../lib/hook-helpers.sh
    . "$HELPERS_LIB"
else
    # Missing helper library must never hard-fail git push.
    write_failure() { return 0; }
fi

if [ -f "$CHAIN_LIB" ]; then
    # shellcheck source=../lib/chain-parser.sh
    . "$CHAIN_LIB"
else
    # Fallback parser keeps gate functional when helper lib is absent.
    chain_find_entry() {
        local chain_file="$1" step_name="$2"
        grep -E "\"(step|gate)\"[[:space:]]*:[[:space:]]*\"${step_name}\"" "$chain_file" 2>/dev/null | tail -1
    }
    chain_is_done() {
        local entry="$1"
        [ -z "$entry" ] && return 1
        if echo "$entry" | grep -qE '"status"[[:space:]]*:[[:space:]]*"(locked|skipped)"'; then
            return 0
        fi
        if echo "$entry" | grep -qE '"locked"[[:space:]]*:[[:space:]]*true'; then
            return 0
        fi
        return 1
    }
fi

LOG_DIR="$ROOT/.agents/ao"
mkdir -p "$LOG_DIR" 2>/dev/null

PHASED_STATE="$ROOT/.agents/rpi/phased-state.json"

# --- Method 1: Run-scoped phased-state.json ---
# If a phased run is/was active, use its verdicts to determine gate status.
# This avoids false positives from stale entries in the global chain.jsonl.
if [ -f "$PHASED_STATE" ]; then
    # Read phase number and verdicts from run-scoped state
    if command -v jq >/dev/null 2>&1; then
        PHASE_NUM=$(jq -r '.phase // 0' "$PHASED_STATE" 2>/dev/null)
        VIBE_VERDICT=$(jq -r '.verdicts.vibe // ""' "$PHASED_STATE" 2>/dev/null)
        SCHEMA=$(jq -r '.schema_version // 0' "$PHASED_STATE" 2>/dev/null)
    else
        PHASE_NUM=$(grep -o '"phase"[[:space:]]*:[[:space:]]*[0-9]*' "$PHASED_STATE" 2>/dev/null | head -1 | grep -o '[0-9]*$')
        VIBE_VERDICT=$(grep -o '"vibe"[[:space:]]*:[[:space:]]*"[^"]*"' "$PHASED_STATE" 2>/dev/null | sed 's/.*"vibe"[[:space:]]*:[[:space:]]*"//;s/"$//')
        SCHEMA=$(grep -o '"schema_version"[[:space:]]*:[[:space:]]*[0-9]*' "$PHASED_STATE" 2>/dev/null | head -1 | grep -o '[0-9]*$')
    fi

    # Schema v1 phased runs: gate on phase 3 completion (validation phase contains vibe)
    # Phase 3 = validation; if we're past phase 3 or vibe verdict exists, vibe was done
    if [ "${SCHEMA:-0}" -ge 1 ]; then
        VIBE_DONE=false
        if [ -n "$VIBE_VERDICT" ] && [ "$VIBE_VERDICT" != "null" ] && [ "$VIBE_VERDICT" != "" ]; then
            VIBE_DONE=true
        elif [ "${PHASE_NUM:-0}" -ge 3 ]; then
            # Phase 3 was reached — vibe runs within validation phase
            VIBE_DONE=true
        fi

        if [ "$VIBE_DONE" = "false" ]; then
            if [ -n "$CLAUDE_AGENT_NAME" ] && echo "$CLAUDE_AGENT_NAME" | grep -q '^worker-'; then
                MSG="Push blocked: vibe check needed. Report to team lead."
            else
                MSG="BLOCKED: vibe not completed. Run /vibe before pushing.
Options:
  1. /vibe              -- full council validation
  2. /vibe --quick      -- fast inline check
  3. ao ratchet skip vibe --reason \"<why>\""
            fi
            echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) GATE_BLOCK: push-gate blocked (vibe, phased): $CMD" >> "$LOG_DIR/hook-errors.log" 2>/dev/null
            write_failure "push_gate_vibe" "git push" 2 "vibe not completed before push (phased run phase=${PHASE_NUM}): $CMD"
            echo "$MSG" >&2
            exit 2
        fi

        # Vibe done — also check post-mortem if it was recorded in verdicts
        # Post-mortem runs within phase 3 (validation); if phase 3 completed, post-mortem ran.
        # We only block if post-mortem is explicitly tracked and missing.
        PM_DONE=true
        if [ "${PHASE_NUM:-0}" -lt 3 ]; then
            PM_DONE=false
        fi
        # If post-mortem verdict is explicitly set to empty/missing after phase 3 completed, still pass
        # (post-mortem may not write to verdicts; phase completion is sufficient evidence)

        if [ "$PM_DONE" = "false" ]; then
            if [ -n "$CLAUDE_AGENT_NAME" ] && echo "$CLAUDE_AGENT_NAME" | grep -q '^worker-'; then
                PM_MSG="Push blocked: post-mortem needed. Report to team lead."
            else
                PM_MSG="BLOCKED: post-mortem not completed. Run /post-mortem to capture learnings before pushing.
Options:
  1. /post-mortem          -- full council wrap-up
  2. ao ratchet skip post-mortem --reason '<why>'"
            fi
            echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) GATE_BLOCK: push-gate blocked (post-mortem, phased): $CMD" >> "$LOG_DIR/hook-errors.log" 2>/dev/null
            write_failure "push_gate_postmortem" "git push" 2 "post-mortem not completed before push (phased run phase=${PHASE_NUM}): $CMD"
            echo "$PM_MSG" >&2
            exit 2
        fi

        # Phased run gates passed
        exit 0
    fi
    # Fall through to chain-based check for schema v0 / legacy state files
fi

# --- Method 2: Global chain.jsonl (legacy/non-phased runs) ---
# Cold start: no chain = no enforcement
[ ! -f "$ROOT/.agents/ao/chain.jsonl" ] && exit 0

# Parse chain directly for speed (avoid spawning ao process)
VIBE_LINE=$(chain_find_entry "$ROOT/.agents/ao/chain.jsonl" "vibe")

VIBE_DONE=false
if [ -n "$VIBE_LINE" ] && chain_is_done "$VIBE_LINE"; then
    VIBE_DONE=true
fi

if [ "$VIBE_DONE" = "false" ]; then
    # Vibe not completed — block push
    if [ -n "$CLAUDE_AGENT_NAME" ] && echo "$CLAUDE_AGENT_NAME" | grep -q '^worker-'; then
        MSG="Push blocked: vibe check needed. Report to team lead."
    else
        MSG="BLOCKED: vibe not completed. Run /vibe before pushing.
Options:
  1. /vibe              -- full council validation
  2. /vibe --quick      -- fast inline check
  3. ao ratchet skip vibe --reason \"<why>\""
    fi

    echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) GATE_BLOCK: push-gate blocked (vibe): $CMD" >> "$LOG_DIR/hook-errors.log" 2>/dev/null
    write_failure "push_gate_vibe" "git push" 2 "vibe not completed before push: $CMD"
    echo "$MSG" >&2
    exit 2
fi

# --- Post-mortem gate (chain-based) ---
# If vibe exists, check that post-mortem is also done before allowing push
PM_LINE=$(chain_find_entry "$ROOT/.agents/ao/chain.jsonl" "post-mortem")

if [ -z "$PM_LINE" ]; then
    :
elif chain_is_done "$PM_LINE"; then
    exit 0
fi

# Post-mortem not completed — block push
if [ -n "$CLAUDE_AGENT_NAME" ] && echo "$CLAUDE_AGENT_NAME" | grep -q '^worker-'; then
    PM_MSG="Push blocked: post-mortem needed. Report to team lead."
else
    PM_MSG="BLOCKED: post-mortem not completed. Run /post-mortem to capture learnings before pushing.
Options:
  1. /post-mortem          -- full council wrap-up
  2. ao ratchet skip post-mortem --reason '<why>'"
fi

echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) GATE_BLOCK: push-gate blocked (post-mortem): $CMD" >> "$LOG_DIR/hook-errors.log" 2>/dev/null
write_failure "push_gate_postmortem" "git push" 2 "post-mortem not completed before push: $CMD"
echo "$PM_MSG" >&2
exit 2
