#!/usr/bin/env bash
# citation-tracker.sh - PostToolUse(Read) hook: track passive citations of knowledge artifacts
# When a Read targets .agents/learnings/, .agents/patterns/, or .agents/research/,
# appends a citation event to .agents/ao/citations.jsonl.
# Kill switch: AGENTOPS_HOOKS_DISABLED=1
# practices: [wiki-knowledge-surface, distributed-tracing]

set -euo pipefail

# Kill switch
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0

# Read all of stdin (hook pipes JSON)
INPUT=$(cat)

# Extract file_path from tool_input
if command -v jq >/dev/null 2>&1; then
    FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty' 2>/dev/null)
else
    FILE_PATH=$(echo "$INPUT" | grep -o '"file_path"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"file_path"[[:space:]]*:[[:space:]]*"//;s/"$//')
fi

# Hot-path exit: no file_path or not under .agents/
[ -z "$FILE_PATH" ] && exit 0
case "$FILE_PATH" in
    */.agents/learnings/*|*/.agents/patterns/*|*/.agents/research/*) ;;
    .agents/learnings/*|.agents/patterns/*|.agents/research/*) ;;
    *) exit 0 ;;
esac

# Normalize: extract the .agents/... relative portion for the citation record
ARTIFACT_PATH="${FILE_PATH##*/}"  # fallback
case "$FILE_PATH" in
    */.agents/*) ARTIFACT_PATH=".agents/${FILE_PATH#*/.agents/}" ;;
    .agents/*)   ARTIFACT_PATH="$FILE_PATH" ;;
esac

# Session-level dedup: one citation per path per session
SESSION_ID="${CLAUDE_SESSION_ID:-unknown}"
DEDUP_FILE="/tmp/citation-tracker-${SESSION_ID}.seen"

if [ -f "$DEDUP_FILE" ] && grep -qFx "$ARTIFACT_PATH" "$DEDUP_FILE" 2>/dev/null; then
    exit 0
fi

# Record dedup
echo "$ARTIFACT_PATH" >> "$DEDUP_FILE"

# Resolve repo root
REPO_ROOT=$(git rev-parse --show-toplevel 2>/dev/null || echo ".")

# Source the canonical state-path resolver (soc-irg1.1) so AO_HOME /
# CLAUDE_PLUGIN_DATA precedence is honored. Fail-open: if the resolver is not
# present, fall back to the legacy ${REPO_ROOT}/.agents layout.
if [ -z "${AO_AGENTS_DIR:-}" ] && [ -f "$REPO_ROOT/lib/ao-paths.sh" ]; then
    eval "$(bash "$REPO_ROOT/lib/ao-paths.sh" 2>/dev/null)" 2>/dev/null || true
fi
AO_DIR="${AO_AGENTS_DIR:-$REPO_ROOT/.agents}/ao"
CITATIONS_FILE="$AO_DIR/citations.jsonl"

# Ensure output directory exists
mkdir -p "$AO_DIR" 2>/dev/null

# Append citation event
TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ)
if command -v jq >/dev/null 2>&1; then
    jq -cn \
        --arg path "$ARTIFACT_PATH" \
        --arg sid "$SESSION_ID" \
        --arg ts "$TIMESTAMP" \
        --arg ct "passive_read" \
        '{artifact_path: $path, session_id: $sid, cited_at: $ts, citation_type: $ct}' \
        >> "$CITATIONS_FILE"
else
    printf '{"artifact_path":"%s","session_id":"%s","cited_at":"%s","citation_type":"passive_read"}\n' \
        "$ARTIFACT_PATH" "$SESSION_ID" "$TIMESTAMP" \
        >> "$CITATIONS_FILE"
fi

exit 0
