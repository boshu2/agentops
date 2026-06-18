#!/usr/bin/env bash
# pane-liveness-audit.sh — ground-truth audit of every pane in a session
#
# Usage: pane-liveness-audit.sh <session> [repo_path]
#
# For each pane, prints:
#   index | pane_current_command | pane_pid | last_commit_by_pane | recent_activity_flag
#
# Flags "zsh" / "bash" as DEAD-CLI (agent exited, prompt would land at shell).
# Per OC-026 (Verify Agent CLI Is Actually Running) + OBSERVABILITY.md "Liveness Signals That Can Lie".

set -u
SESSION="${1:?usage: pane-liveness-audit.sh <session> [repo_path]}"
REPO="${2:-$PWD}"

# Discover window index once (OC-028).
WIN=$(tmux list-windows -t "$SESSION" -F '#{window_index}' 2>/dev/null | head -1)
if [ -z "$WIN" ]; then
  echo "ERROR: tmux session '$SESSION' not found or tmux server not running" >&2
  exit 2
fi

printf '%-4s %-18s %-8s %-14s %s\n' "IDX" "CURRENT_COMMAND" "PID" "LAST_COMMIT" "STATUS"
printf '%s\n' "─────────────────────────────────────────────────────────────────────────"

tmux list-panes -t "$SESSION:$WIN" -F '#{pane_index} #{pane_current_command} #{pane_pid}' \
  | while read -r IDX CMD PID; do
  LAST_COMMIT=$(git -C "$REPO" log --since="1 hour ago" --format='%h' --author-date-order 2>/dev/null | head -1)
  LAST_COMMIT=${LAST_COMMIT:-"-"}

  case "$CMD" in
    zsh|bash|sh)       STATUS="⚠ DEAD-CLI (bare shell — re-launch via OC-027)";;
    claude)            STATUS="cc live";;
    bun|node)          STATUS="codex live";;
    gemini|python|python3) STATUS="gmi live";;
    cargo|rustc|go)      STATUS="build-in-flight (do not interrupt)";;
    *)                 STATUS="? (unknown command — investigate)";;
  esac

  printf '%-4s %-18s %-8s %-14s %s\n' "$IDX" "$CMD" "$PID" "$LAST_COMMIT" "$STATUS"
done

echo
echo "Legend: DEAD-CLI panes need OC-026 audit + OC-027 two-step relaunch before any send."
echo "Cross-reference the productivity signal with: git log --since='15 minutes ago' --oneline"
