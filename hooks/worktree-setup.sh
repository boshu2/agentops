#!/bin/bash
# WorktreeCreate hook: initialize isolated worktree for RPI/swarm work
# Sets up beads tracking, injects knowledge, records metadata

# Kill switch
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$SCRIPT_DIR/../lib/hook-helpers.sh"

read_hook_input

# Extract worktree metadata from input
WORKTREE_PATH=""
WORKTREE_NAME=""
if [ -n "$INPUT" ]; then
    if command -v jq >/dev/null 2>&1; then
        WORKTREE_PATH=$(echo "$INPUT" | jq -r '.worktree_path // .path // ""' 2>/dev/null) || true
        WORKTREE_NAME=$(echo "$INPUT" | jq -r '.worktree_name // .name // ""' 2>/dev/null) || true
    fi
    if [ -z "$WORKTREE_PATH" ]; then
        WORKTREE_PATH=$(echo "$INPUT" | grep -o '"worktree_path"[[:space:]]*:[[:space:]]*"[^"]*"' 2>/dev/null \
            | sed 's/.*"worktree_path"[[:space:]]*:[[:space:]]*"//;s/"$//' 2>/dev/null) || true
    fi
fi

# Skip if no worktree path available
[ -z "$WORKTREE_PATH" ] && exit 0

# Initialize beads in worktree (per-command timeout)
if command -v bd >/dev/null 2>&1 && [ -d "$WORKTREE_PATH" ]; then
    (cd "$WORKTREE_PATH" && timeout_run 5 bd init 2>/dev/null) || {
        echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) HOOK_WARN: bd init failed in worktree $WORKTREE_PATH" \
            >> "$ROOT/.agents/ao/hook-errors.log" 2>/dev/null
    }
fi

# Inject knowledge into worktree (per-command timeout)
if command -v ao >/dev/null 2>&1 && [ -d "$WORKTREE_PATH" ]; then
    (cd "$WORKTREE_PATH" && timeout_run 5 ao inject --apply-decay --max-tokens 1000 2>/dev/null) || {
        echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) HOOK_WARN: ao inject failed in worktree $WORKTREE_PATH" \
            >> "$ROOT/.agents/ao/hook-errors.log" 2>/dev/null
    }
fi

# Record worktree metadata
METADATA_DIR="$ROOT/.agents/ao"
mkdir -p "$METADATA_DIR"

TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ)

if command -v jq >/dev/null 2>&1; then
    jq -n \
        --arg created_at "$TIMESTAMP" \
        --arg worktree_path "$WORKTREE_PATH" \
        --arg worktree_name "${WORKTREE_NAME:-unnamed}" \
        --arg parent_repo "$ROOT" \
        --arg session "${CLAUDE_SESSION_ID:-unknown}" \
        '{created_at:$created_at,worktree_path:$worktree_path,worktree_name:$worktree_name,parent_repo:$parent_repo,session_id:$session}' \
        > "$METADATA_DIR/worktree-metadata.json" 2>/dev/null
else
    printf '{"created_at":"%s","worktree_path":"%s","worktree_name":"%s","parent_repo":"%s","session_id":"%s"}\n' \
        "$TIMESTAMP" "$WORKTREE_PATH" "${WORKTREE_NAME:-unnamed}" "$ROOT" "${CLAUDE_SESSION_ID:-unknown}" \
        > "$METADATA_DIR/worktree-metadata.json" 2>/dev/null
fi

# Log lifecycle event
echo "$TIMESTAMP CREATE: worktree=$WORKTREE_PATH name=${WORKTREE_NAME:-unnamed}" \
    >> "$METADATA_DIR/worktree-lifecycle.log" 2>/dev/null

exit 0
