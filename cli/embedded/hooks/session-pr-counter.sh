#!/bin/bash
# shellcheck shell=bash
set -uo pipefail
# session-pr-counter.sh — PreToolUse hook: mechanical session-scope enforcement (soc-1aou)
#
# Strict-mode choice: -uo pipefail (catches typos and mid-pipe failures); we
# intentionally omit `-e` because advisory hooks must fail open on command
# failures (e.g., `gh` unreachable, parse errors) per the shell-standards
# "advisory = fail open" rule.
#
# Sister rule to coherent-arc (soc-waxr, PR #361). Counts PRs the current user
# opened in the last 24 hours; if that count is >=4 (so the about-to-be-created
# PR would make 5+), injects a post-mortem reminder as additionalContext.
#
# Non-blocking by design: the reminder shows up in the agent's context, the
# agent decides whether to proceed. Mechanical means "always-on visible signal",
# not "hard block". Hard-block via $AGENTOPS_SESSION_PR_BLOCK=1 (opt-in).
#
# Kill switches:
#   AGENTOPS_HOOKS_DISABLED=1          — all hooks off
#   AGENTOPS_SESSION_PR_COUNTER_DISABLED=1 — this hook off
#
# Triggers: Bash with `gh pr create` substring. No-op for everything else.

[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0
[ "${AGENTOPS_SESSION_PR_COUNTER_DISABLED:-}" = "1" ] && exit 0

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -f "$SCRIPT_DIR/../lib/hook-helpers.sh" ]; then
    # shellcheck source=../lib/hook-helpers.sh
    . "$SCRIPT_DIR/../lib/hook-helpers.sh"
elif [ -f "$SCRIPT_DIR/hook-helpers.sh" ]; then
    # shellcheck source=../lib/hook-helpers.sh
    . "$SCRIPT_DIR/hook-helpers.sh"
fi

INPUT=$(cat)
if declare -F try_managed_hook_backend >/dev/null 2>&1; then
    try_managed_hook_backend "session-pr-counter" "$INPUT" && exit 0
fi

# Resolve tool name + command (env var first, jq fallback).
TOOL_NAME="${CLAUDE_TOOL_NAME:-}"
COMMAND="${CLAUDE_TOOL_INPUT_COMMAND:-}"
if [ -z "$TOOL_NAME" ] || [ -z "$COMMAND" ]; then
    if command -v jq >/dev/null 2>&1; then
        IFS=$'\t' read -r _jq_tool _jq_cmd < <(echo "$INPUT" | jq -r '[.tool_name // "", .tool_input.command // ""] | @tsv' 2>/dev/null) || exit 0
        [ -z "$TOOL_NAME" ] && TOOL_NAME="$_jq_tool"
        [ -z "$COMMAND" ] && COMMAND="$_jq_cmd"
    fi
fi

# Only fire on Bash + `gh pr create`.
[ "$TOOL_NAME" = "Bash" ] || exit 0
echo "$COMMAND" | grep -q 'gh pr create' || exit 0

# Bail if `gh` itself isn't available — can't count.
command -v gh >/dev/null 2>&1 || exit 0

# Window for "current session" — last N hours. Default 24h matches the
# operational definition of an autonomous session (cron-loop spans, etc.).
WINDOW_HOURS="${AGENTOPS_SESSION_PR_WINDOW_HOURS:-24}"
THRESHOLD="${AGENTOPS_SESSION_PR_THRESHOLD:-5}"

# Count my PRs (any state) opened in the window.
# `gh pr list --search` honors the current repo; state:all includes merged.
SINCE_ISO="$(date -u -d "${WINDOW_HOURS} hours ago" +%FT%TZ 2>/dev/null || date -u +%FT%TZ)"
PR_COUNT="$(gh pr list \
    --search "author:@me created:>=${SINCE_ISO}" \
    --state all \
    --limit 50 \
    --json number 2>/dev/null | jq -r 'length' 2>/dev/null)"

# If we can't get a count, fail open (no notice).
case "$PR_COUNT" in
    ''|*[!0-9]*) exit 0 ;;
esac

# The PR about to be created is PR #(PR_COUNT + 1). At PR_COUNT >= THRESHOLD-1
# (e.g., 4), the next one tips into post-mortem territory.
NEXT_PR_NUMBER=$((PR_COUNT + 1))
if [ "$NEXT_PR_NUMBER" -lt "$THRESHOLD" ]; then
    exit 0
fi

# Hard-block mode for operators who want the gate to refuse.
if [ "${AGENTOPS_SESSION_PR_BLOCK:-0}" = "1" ]; then
    cat >&2 <<EOF
BLOCKED: session-pr-counter detected ${PR_COUNT} PRs created in the last ${WINDOW_HOURS}h.
This would be PR #${NEXT_PR_NUMBER} in the session window — past the post-mortem
threshold (${THRESHOLD}). Per the session-scope rule (soc-waxr, see CLAUDE.md
"Workflow"), stop and run a 1-2 sentence post-mortem before continuing:

  - Which PRs were planned vs reactive?
  - How many self-corrections so far?
  - Is the marginal PR discovery or churn?

To proceed anyway: set AGENTOPS_SESSION_PR_BLOCK=0 (default), or set
AGENTOPS_SESSION_PR_COUNTER_DISABLED=1 to skip the check entirely.
EOF
    exit 2
fi

# Informational mode (default): inject as additionalContext.
REMINDER="SESSION-SCOPE NOTICE (soc-waxr): ${PR_COUNT} PR(s) opened in the last ${WINDOW_HOURS}h; this would be #${NEXT_PR_NUMBER} (threshold: ${THRESHOLD}).

Per the session-scope rule (CLAUDE.md \"Workflow\"), >=5 PRs triggers a mandatory post-mortem before continuing. Run a 1-2 sentence post-mortem first:
- Which PRs were planned vs reactive?
- How many self-corrections so far?
- Is the marginal PR discovery or churn?

Then proceed if the marginal PR survives the post-mortem. Set AGENTOPS_SESSION_PR_BLOCK=1 for a hard block, or AGENTOPS_SESSION_PR_COUNTER_DISABLED=1 to silence."

if declare -F emit_hook_context >/dev/null 2>&1; then
    emit_hook_context "PreToolUse" "$REMINDER"
elif command -v jq >/dev/null 2>&1; then
    jq -n --arg ctx "$REMINDER" '{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":$ctx}}'
else
    safe_msg=${REMINDER//\\/\\\\}
    safe_msg=${safe_msg//\"/\\\"}
    safe_msg=$(echo "$safe_msg" | tr '\n' ' ')
    echo "{\"hookSpecificOutput\":{\"hookEventName\":\"PreToolUse\",\"additionalContext\":\"$safe_msg\"}}"
fi

exit 0
