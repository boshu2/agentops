#!/bin/bash
# research-loop-detector.sh - PostToolUse hook: detect research spirals
# Tracks consecutive read-only tool calls (Read, Grep, Glob, WebSearch, WebFetch).
# Injects escalating warnings when the agent enters a read-only loop without editing.
# Non-blocking (always exit 0). Warnings via additionalContext injection.

# Kill switches
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0
[ "${AGENTOPS_RESEARCH_LOOP_DISABLED:-}" = "1" ] && exit 0

# Configurable thresholds
WARN_THRESHOLD="${AGENTOPS_RESEARCH_WARN_THRESHOLD:-8}"
STRONG_THRESHOLD="${AGENTOPS_RESEARCH_STRONG_THRESHOLD:-12}"
STOP_THRESHOLD="${AGENTOPS_RESEARCH_STOP_THRESHOLD:-15}"

# Read stdin
INPUT=$(cat)

# Extract tool name — prefer env var (fast), fallback to jq
TOOL_NAME="${CLAUDE_TOOL_NAME:-}"
if [ -z "$TOOL_NAME" ]; then
    TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // ""' 2>/dev/null) || exit 0
fi
[ -z "$TOOL_NAME" ] && exit 0

# Quick exit for neutral tools — no git rev-parse needed
case "$TOOL_NAME" in
    Read|Grep|Glob|WebSearch|WebFetch|Edit|Write|NotebookEdit|Bash) ;;
    *) exit 0 ;;
esac

# State file (only computed for tools that affect counter)
ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
STATE_DIR="$ROOT/.agents/ao"
COUNTER_FILE="$STATE_DIR/.read-streak"

mkdir -p "$STATE_DIR" 2>/dev/null

# Classify tool type
case "$TOOL_NAME" in
    Read|Grep|Glob|WebSearch|WebFetch)
        # Read-only tool — increment counter
        COUNT=0
        [ -f "$COUNTER_FILE" ] && COUNT=$(cat "$COUNTER_FILE" 2>/dev/null || echo 0)
        COUNT=$((COUNT + 1))
        echo "$COUNT" > "$COUNTER_FILE" 2>/dev/null

        # Check thresholds
        NUDGE=""
        if [ "$COUNT" -ge "$STOP_THRESHOLD" ]; then
            NUDGE="STOP RESEARCHING. You have made $COUNT consecutive read-only tool calls without editing any files. You are in a research spiral. Produce output NOW with what you have."
        elif [ "$COUNT" -ge "$STRONG_THRESHOLD" ]; then
            NUDGE="WARNING: $COUNT consecutive read-only tool calls without editing any files. Your next action MUST be Edit, Write, or Bash execution. Stop researching."
        elif [ "$COUNT" -ge "$WARN_THRESHOLD" ]; then
            NUDGE="You have made $COUNT consecutive read-only tool calls without editing any files. Consider acting on what you know."
        fi

        if [ -n "$NUDGE" ]; then
            if command -v jq >/dev/null 2>&1; then
                jq -n --arg ctx "$NUDGE" '{"hookSpecificOutput":{"hookEventName":"PostToolUse","additionalContext":$ctx}}'
            else
                safe_msg=${NUDGE//\\/\\\\}
                safe_msg=${safe_msg//\"/\\\"}
                echo "{\"hookSpecificOutput\":{\"hookEventName\":\"PostToolUse\",\"additionalContext\":\"$safe_msg\"}}"
            fi
        fi
        ;;
    Edit|Write|NotebookEdit)
        # Write tool — reset counter
        rm -f "$COUNTER_FILE" 2>/dev/null
        ;;
    Bash)
        # Bash — reset counter (execution counts as action)
        # But don't reset for grep/cat/head/find (read-only bash commands)
        COMMAND="${CLAUDE_TOOL_INPUT_COMMAND:-}"
        if [ -z "$COMMAND" ]; then
            COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // ""' 2>/dev/null) || true
        fi
        COMMAND_LOWER=$(echo "$COMMAND" | tr '[:upper:]' '[:lower:]')
        FIRST_WORD="${COMMAND_LOWER%% *}"
        case "$FIRST_WORD" in
            grep|rg|cat|head|tail|find|ls|wc|file|stat)
                # Read-only bash — treat like read tool
                COUNT=0
                [ -f "$COUNTER_FILE" ] && COUNT=$(cat "$COUNTER_FILE" 2>/dev/null || echo 0)
                COUNT=$((COUNT + 1))
                echo "$COUNT" > "$COUNTER_FILE" 2>/dev/null
                ;;
            *)
                # Execution bash — reset counter
                rm -f "$COUNTER_FILE" 2>/dev/null
                ;;
        esac
        ;;
    # Neutral tools (Agent, Skill, Task*, SendMessage, AskUserQuestion) — ignore
esac

exit 0
