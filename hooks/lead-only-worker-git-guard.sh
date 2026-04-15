#!/usr/bin/env bash
# lead-only-worker-git-guard.sh - PreToolUse hook
#
# Mechanically enforces the swarm "lead-only commit" policy. When the running
# agent is identified as a swarm worker (not the lead), BLOCK any destructive
# git command. The lead is the sole owner of git state mutations.
#
# Forbidden verbs:
#   commit, push, reset, rebase, merge, cherry-pick,
#   branch -D, checkout -B, worktree remove
#
# Non-destructive git commands (status, diff, log, show, blame, fetch, ...)
# are always allowed.
#
# Kill switches:
#   AGENTOPS_HOOKS_DISABLED=1               disable all hooks
#   AGENTOPS_LEAD_ONLY_GUARD_DISABLED=1     disable just this guard
#
# Exit codes: 0 pass, 2 block (PreToolUse contract).

set -euo pipefail

[[ "${AGENTOPS_HOOKS_DISABLED:-}" == "1" ]] && exit 0
[[ "${AGENTOPS_LEAD_ONLY_GUARD_DISABLED:-}" == "1" ]] && exit 0

# ---- worker detection ------------------------------------------------------
is_worker=0
if [[ "${AGENTOPS_SWARM_ROLE:-}" == "worker" ]]; then
    is_worker=1
elif [[ "${AGENTOPS_ROLE:-}" == "worker" ]]; then
    is_worker=1
elif [[ "${CLAUDE_AGENT_NAME:-}" == worker-* ]]; then
    is_worker=1
elif [[ -f ".agents/swarm-role" ]] && grep -q "^worker$" .agents/swarm-role 2>/dev/null; then
    is_worker=1
elif [[ "${PWD}" == *"/.claude/worktrees/"* ]]; then
    is_worker=1
fi

# Lead context: pass through silently.
[[ $is_worker -eq 0 ]] && exit 0

# ---- read tool input -------------------------------------------------------
input=$(cat)
cmd="${CLAUDE_TOOL_INPUT_COMMAND:-}"
if [[ -z "$cmd" ]]; then
    cmd=$(printf '%s' "$input" | jq -r '.tool_input.command // ""' 2>/dev/null || printf '')
fi

# Empty command? Nothing to inspect.
[[ -z "$cmd" ]] && exit 0

# ---- match destructive git verbs -------------------------------------------
# Anchor on a leading `git` token (allow leading whitespace and env prefix
# like `GIT_AUTHOR_NAME=foo git ...`). Use a single regex over the full command
# so chained commands (`do_thing && git commit`) still get caught.
block_reason=""

# Normalize multiple spaces for matching.
norm_cmd=$(printf '%s' "$cmd" | tr -s '[:space:]' ' ')

# Pattern: `git <verb>` with verb being one of the destructive ones.
# Branch -D and checkout -B and worktree remove require a second token.
if [[ "$norm_cmd" =~ (^|[^[:alnum:]_/])git[[:space:]]+(commit|push|reset|rebase|merge|cherry-pick)([[:space:]]|$) ]]; then
    block_reason="destructive git verb: ${BASH_REMATCH[2]}"
elif [[ "$norm_cmd" =~ (^|[^[:alnum:]_/])git[[:space:]]+branch[[:space:]]+-D([[:space:]]|$) ]]; then
    block_reason="destructive git verb: branch -D"
elif [[ "$norm_cmd" =~ (^|[^[:alnum:]_/])git[[:space:]]+checkout[[:space:]]+-B([[:space:]]|$) ]]; then
    block_reason="destructive git verb: checkout -B"
elif [[ "$norm_cmd" =~ (^|[^[:alnum:]_/])git[[:space:]]+worktree[[:space:]]+remove([[:space:]]|$) ]]; then
    block_reason="destructive git verb: worktree remove"
fi

if [[ -n "$block_reason" ]]; then
    msg="BLOCKED: swarm workers must not run destructive git commands (lead-only policy). ${block_reason}. Command: ${cmd}"
    # Emit JSON decision for hook protocol consumers.
    if command -v jq >/dev/null 2>&1; then
        jq -cn --arg m "$msg" '{decision:"block",reason:$m,hookSpecificOutput:{hookEventName:"PreToolUse",additionalContext:$m}}'
    fi
    # Stderr message is what the agent actually sees on exit 2.
    echo "$msg" >&2
    exit 2
fi

exit 0
