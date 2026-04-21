#!/bin/bash
# git-worker-guard.sh - PreToolUse hook: block git write operations for swarm workers
# Workers should NEVER commit/push/add-all — only the lead does that.
# Non-blocking for leads (exit 0). Blocks workers (exit 2).

# Kill switches
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0
[ "${AGENTOPS_GIT_WORKER_GUARD_DISABLED:-}" = "1" ] && exit 0

# Determine if we're a worker via multiple detection paths:
# 1. AGENTOPS_ROLE env var set by swarm dispatch
# 2. CLAUDE_AGENT_NAME matching "worker-*" pattern (native teams)
# 3. .agents/swarm-role file containing "worker" (filesystem signal)
IS_WORKER=0
if [ "${AGENTOPS_ROLE:-}" = "worker" ]; then
    IS_WORKER=1
elif [[ "${CLAUDE_AGENT_NAME:-}" == worker-* ]]; then
    IS_WORKER=1
elif [ -f ".agents/swarm-role" ] && grep -q "^worker$" ".agents/swarm-role" 2>/dev/null; then
    IS_WORKER=1
fi

[ "$IS_WORKER" -eq 1 ] || exit 0

# Read stdin
INPUT=$(cat)

# Extract command from stdin JSON or env var
COMMAND="${CLAUDE_TOOL_INPUT_COMMAND:-}"
if [ -z "$COMMAND" ]; then
    COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // ""' 2>/dev/null) || exit 0
fi

# Check for git write operations
COMMAND_LOWER=$(echo "$COMMAND" | tr '[:upper:]' '[:lower:]')

# Block git commit, git push (subcommand names are case-insensitive in practice
# via shells/aliases, so match against the lowercased form).
case "$COMMAND_LOWER" in
    *"git commit"*|*"git push"*|*"git add --all"*|*"git add ."*)
        echo "BLOCKED: Workers cannot run git write operations. Only the lead commits/pushes." >&2
        echo "Command: $COMMAND" >&2
        exit 2
        ;;
esac

# Block `git add -A` bulk staging. Match on the original (un-lowercased) command
# so that selective flags like `-a`, `-au`, or filenames containing `-a` (e.g.
# `file-a.txt`) do not false-block. `-A` is the only uppercase bulk-add flag.
case "$COMMAND" in
    *"git add -A"*)
        echo "BLOCKED: Workers cannot run git write operations. Only the lead commits/pushes." >&2
        echo "Command: $COMMAND" >&2
        exit 2
        ;;
esac

# Block workers from spawning sub-workers (git worktree add, agent spawn operations)
case "$COMMAND_LOWER" in
    *"git worktree add"*)
        echo "BLOCKED: Workers cannot create worktrees. Only the lead spawns sub-workers." >&2
        echo "Command: $COMMAND" >&2
        exit 2
        ;;
    *"spawn_agent"*|*"claude agent"*|*"codex agent"*|*"gc session nudge"*)
        echo "BLOCKED: Workers cannot spawn sub-workers. Only the lead dispatches agents." >&2
        echo "Command: $COMMAND" >&2
        exit 2
        ;;
esac

exit 0
