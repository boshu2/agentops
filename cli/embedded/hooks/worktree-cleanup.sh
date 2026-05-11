#!/bin/bash
# WorktreeRemove hook: archive artifacts and sync state before worktree deletion
# Preserves learnings and beads state from isolated worktree
# practices: [gitops, event-sourcing-cqrs]

# Kill switch
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
# shellcheck source=../lib/hook-helpers.sh
. "$SCRIPT_DIR/../lib/hook-helpers.sh"

read_hook_input

# Extract worktree path from input
WORKTREE_PATH=""
if [ -n "$INPUT" ]; then
    if command -v jq >/dev/null 2>&1; then
        WORKTREE_PATH=$(echo "$INPUT" | jq -r '.worktree_path // .path // ""' 2>/dev/null) || true
    fi
    if [ -z "$WORKTREE_PATH" ]; then
        WORKTREE_PATH=$(echo "$INPUT" | grep -o '"worktree_path"[[:space:]]*:[[:space:]]*"[^"]*"' 2>/dev/null \
            | sed 's/.*"worktree_path"[[:space:]]*:[[:space:]]*"//;s/"$//' 2>/dev/null) || true
    fi
fi

# Skip if no path or path doesn't exist
[ -z "$WORKTREE_PATH" ] && exit 0
[ ! -d "$WORKTREE_PATH" ] && exit 0

# Archive a beads snapshot when bd is available. Current bd releases auto-sync
# issue writes, so an explicit 'bd sync' command is no longer available.
if command -v bd >/dev/null 2>&1; then
    TIMESTAMP=$(date -u +%Y-%m-%dT%H%M%SZ)
    ARCHIVE_DIR="$ROOT/.agents/archived-worktrees/$TIMESTAMP"
    mkdir -p "$ARCHIVE_DIR"
    (cd "$WORKTREE_PATH" && timeout_run 5 bd export -o "$ARCHIVE_DIR/beads.jsonl" >/dev/null 2>&1) || true
fi

# Archive .agents/ from worktree to parent repo
if [ -d "$WORKTREE_PATH/.agents" ]; then
    TIMESTAMP=${TIMESTAMP:-$(date -u +%Y-%m-%dT%H%M%SZ)}
    ARCHIVE_DIR="${ARCHIVE_DIR:-$ROOT/.agents/archived-worktrees/$TIMESTAMP}"
    mkdir -p "$ARCHIVE_DIR"
    cp -r "$WORKTREE_PATH/.agents/." "$ARCHIVE_DIR/" 2>/dev/null || true
fi

# Log lifecycle event
METADATA_DIR="$ROOT/.agents/ao"
mkdir -p "$METADATA_DIR"
echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) REMOVE: worktree=$WORKTREE_PATH" \
    >> "$METADATA_DIR/worktree-lifecycle.log" 2>/dev/null

exit 0
