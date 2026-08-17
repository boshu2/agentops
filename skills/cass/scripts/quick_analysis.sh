#!/bin/bash
#
# Quick Analysis — One-command project overview using cass
#
# Usage:
#     ./quick_analysis.sh /data/projects/PROJECT_NAME
#
# Output:
#     - Index health
#     - Session counts by agent
#     - Activity by date
#     - Top 5 ritual opener candidates
#
# Requires: cass, jq

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROMPT_MINER="$SCRIPT_DIR/prompt_miner.py"

TIMEOUT_BIN=""
for candidate in timeout gtimeout; do
  if command -v "$candidate" >/dev/null 2>&1; then
    TIMEOUT_BIN="$(command -v "$candidate")"
    break
  fi
done
[[ -n "$TIMEOUT_BIN" ]] || {
  echo "Error: timeout or gtimeout is required; refusing an uncapped cass command" >&2
  exit 2
}
COMMAND_TIMEOUT="${CASS_QUICK_TIMEOUT:-30}"
INDEX_TIMEOUT="${CASS_INDEX_TIMEOUT:-600}"
[[ "$COMMAND_TIMEOUT" =~ ^[0-9]+$ && "$COMMAND_TIMEOUT" -ge 1 && "$COMMAND_TIMEOUT" -le 60 ]] \
  || { echo "Error: CASS_QUICK_TIMEOUT must be an integer in [1,60]" >&2; exit 2; }
[[ "$INDEX_TIMEOUT" =~ ^[0-9]+$ && "$INDEX_TIMEOUT" -ge 1 && "$INDEX_TIMEOUT" -le 600 ]] \
  || { echo "Error: CASS_INDEX_TIMEOUT must be an integer in [1,600]" >&2; exit 2; }

run_cass_index() {
  "$TIMEOUT_BIN" "$INDEX_TIMEOUT" cass index --json
}

run_cass() {
  "$TIMEOUT_BIN" "$COMMAND_TIMEOUT" cass "$@"
}

WORKSPACE="${1:-}"

if [ -z "$WORKSPACE" ]; then
    echo "Usage: $0 /data/projects/PROJECT_NAME"
    echo ""
    echo "Examples:"
    echo "  $0 /data/projects/beads_rust"
    echo "  $0 /data/projects/rich_rust"
    exit 1
fi

# Expand path
WORKSPACE=$(realpath "$WORKSPACE" 2>/dev/null || echo "$WORKSPACE")

if ! command -v cass >/dev/null 2>&1; then
    echo "Error: cass is not installed or not in PATH"
    exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
    echo "Error: jq is not installed or not in PATH"
    exit 1
fi

echo "=============================================="
echo "CASS QUICK ANALYSIS: $(basename "$WORKSPACE")"
echo "=============================================="
echo ""

# 1. Health check
echo "--- Index Health ---"
run_cass status --robot-format json | head -c 1048577 | jq '{
    conversations: .database.conversations,
    messages: .database.messages,
    index_fresh: .index.fresh,
    rebuilding: (.index.rebuilding // .rebuild.active // false),
    recommended: .recommended_action
}' || echo "Error: Could not get cass status"
echo ""

# 2. Refresh index (quick, incremental)
echo "--- Refreshing Index ---"
run_cass_index | head -c 1048577 | jq '.indexed // "Index refreshed"' -r || echo "Index refresh attempted"
echo ""

# 3. Agent breakdown
echo "--- Sessions by Agent ---"
run_cass search "*" --workspace "$WORKSPACE" --aggregate agent --limit 1 --json \
    | head -c 1048577 | jq '.aggregations.agent.buckets[] | "\(.key): \(.count) sessions"' -r \
    || echo "No sessions found for this workspace"
echo ""

# 4. Date breakdown (last 7 days of activity)
echo "--- Recent Activity (by date) ---"
run_cass search "*" --workspace "$WORKSPACE" --aggregate date --limit 1 --json \
    | head -c 1048577 | jq '.aggregations.date.buckets | sort_by(.key) | reverse | .[0:7] | .[] | "\(.key): \(.count) hits"' -r \
    || echo "No date information available"
echo ""

# 5. Ritual opener candidates (prompts at lines 1-3 that appear multiple times)
echo "--- Ritual Opener Candidates ---"
echo "(Prompts appearing at session start, sorted by frequency)"
echo ""

# Search for common ritual opener patterns
for pattern in "First read ALL" "AGENTS.md" "comprehensive deep dive" "ultrathink" "think super hard"; do
    count=$(run_cass search "$pattern" --workspace "$WORKSPACE" --json --limit 100 \
        | head -c 1048577 | jq '.total_matches // 0' || echo "0")
    if [ "$count" -gt 2 ]; then
        printf "  %3dx: \"%s\"\n" "$count" "$pattern"
    fi
done
echo ""

# 6. Quick tips
echo "--- Next Steps ---"
echo "1. Find ritual opener:    use cass search with --limit 5 and a ${COMMAND_TIMEOUT}s wrapper"
echo "2. View a session:        resolve one returned source through cass view with an explicit context bound"
echo "3. Find user prompts:     use cass search with an explicit --limit and bounded output"
echo "4. Mine prompts:          python \"$(basename "$PROMPT_MINER")\" with an explicit input root and finite limits"
echo ""
echo "=============================================="
