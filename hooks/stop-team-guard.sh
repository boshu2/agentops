#!/bin/bash
# Stop hook: warn about teams that may have active members
# Only blocks stop if team members are actually running in tmux panes

# Kill switch
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0

TEAMS_DIR="$HOME/.claude/teams"

# --- Stale team cleanup (SessionStart --cleanup mode) ---
cleanup_stale_teams() {
    [ ! -d "$TEAMS_DIR" ] && exit 0

    LOG_DIR="$(git rev-parse --show-toplevel 2>/dev/null || echo .)/.agents/ao"
    mkdir -p "$LOG_DIR"
    LOG_FILE="$LOG_DIR/team-lifecycle.log"
    NOW=$(date +%s)
    STALE_THRESHOLD=7200  # 2 hours in seconds

    find "$TEAMS_DIR" -maxdepth 2 -name "config.json" 2>/dev/null | while IFS= read -r cfg; do
        dir=$(dirname "$cfg")
        name=$(basename "$dir")

        # Check mtime — skip if younger than 2h
        if stat -f %m "$cfg" >/dev/null 2>&1; then
            mtime=$(stat -f %m "$cfg")  # macOS
        else
            mtime=$(stat -c %Y "$cfg" 2>/dev/null || echo "$NOW")  # Linux
        fi
        age=$(( NOW - mtime ))
        [ "$age" -lt "$STALE_THRESHOLD" ] && continue

        # Check for live tmux panes (reuse existing logic)
        pane_ids=$(grep -o '"tmuxPaneId"[[:space:]]*:[[:space:]]*"[^"]*"' "$cfg" 2>/dev/null \
            | sed 's/.*"tmuxPaneId"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/' \
            | grep -v '^$' \
            | grep -v '^in-process$')

        has_live_pane=false
        if [ -n "$pane_ids" ]; then
            while IFS= read -r pane_id; do
                # tmuxPaneId format: "session:window.pane" (e.g., "council-20260217:0.1")
                # %%.*  strips ".pane" suffix → "session:window", which tmux has-session accepts
                if tmux has-session -t "${pane_id%%.*}" 2>/dev/null; then
                    has_live_pane=true
                    break
                fi
            done <<< "$pane_ids"
        fi

        # Skip if any pane is still alive
        [ "$has_live_pane" = "true" ] && continue

        # Remove stale team directory
        rm -rf "$dir"
        echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) CLEANUP: removed stale team '$name' (age=${age}s)" >> "$LOG_FILE"
    done

    exit 0
}

# Handle --cleanup flag (SessionStart mode)
[ "${1:-}" = "--cleanup" ] && cleanup_stale_teams

# --- Read stdin and extract last assistant message ---
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/hook-helpers.sh
. "$SCRIPT_DIR/../lib/hook-helpers.sh"
read_hook_input
mkdir -p "$ROOT/.agents/ao" && echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) STOP: last_msg=${LAST_ASSISTANT_MSG:0:200}" >> "$ROOT/.agents/ao/stop-events.log"

# --- Original Stop guard logic below ---

# If teams directory doesn't exist, nothing to guard
[ ! -d "$TEAMS_DIR" ] && exit 0

# Find team config files
configs=$(find "$TEAMS_DIR" -maxdepth 2 -name "config.json" 2>/dev/null)

# If no configs found, safe to stop
[ -z "$configs" ] && exit 0

# Check each team for actually-running tmux panes
active_teams=""
while IFS= read -r cfg; do
    dir=$(dirname "$cfg")
    name=$(basename "$dir")

    # Extract tmux pane IDs from members (skip "in-process" and empty)
    pane_ids=$(grep -o '"tmuxPaneId"[[:space:]]*:[[:space:]]*"[^"]*"' "$cfg" 2>/dev/null \
        | sed 's/.*"tmuxPaneId"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/' \
        | grep -v '^$' \
        | grep -v '^in-process$')

    # If no tmux panes, this team is stale (in-process agents die with session)
    [ -z "$pane_ids" ] && continue

    # Check if any pane is actually alive in tmux
    has_live_pane=false
    while IFS= read -r pane_id; do
        # tmuxPaneId format: "session:window.pane" — %%.*  yields "session:window"
        if tmux has-session -t "${pane_id%%.*}" 2>/dev/null; then
            has_live_pane=true
            break
        fi
    done <<< "$pane_ids"

    if [ "$has_live_pane" = "true" ]; then
        if [ -z "$active_teams" ]; then
            active_teams="$name"
        else
            active_teams="$active_teams, $name"
        fi
    fi
done <<< "$configs"

# Only block if there are teams with actually-running members
if [ -n "$active_teams" ]; then
    write_failure "stop_team_guard" "stop" 2 "active teams with running members: $active_teams"
    block_msg="Active teams with running members: ${active_teams}. Send shutdown_request to teammates or run TeamDelete before stopping."
    [ -n "${LAST_ASSISTANT_MSG:-}" ] && block_msg="${block_msg} Last agent message: ${LAST_ASSISTANT_MSG:0:100}"
    echo "$block_msg" >&2
    exit 2
fi

# Stale team configs exist but no running members — safe to stop
exit 0
