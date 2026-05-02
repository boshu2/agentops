#!/usr/bin/env bash
# PostToolUse hook: append one JSON line per Edit/Write to .agents/ao/edit-audit.log
# capturing {ts, tool, file, sha256, git_status, head_sha}. Diagnostic-only —
# always exits 0, never blocks.
#
# Used to investigate the silent-revert phenomenon (f-2026-05-02-005): if a
# subsequent Edit shows a different sha256 from the prior post-Edit hash with
# no intervening tool call, the revert occurred between calls and the audit
# log captures the mismatch.
#
# Disable with AGENTOPS_HOOKS_DISABLED=1 or AGENTOPS_EDIT_AUDIT_DISABLED=1.

set -uo pipefail

[[ "${AGENTOPS_HOOKS_DISABLED:-}" == "1" ]] && exit 0
[[ "${AGENTOPS_EDIT_AUDIT_DISABLED:-}" == "1" ]] && exit 0

INPUT="$(cat)"
TOOL_NAME=""
FILE_PATH=""
if command -v jq >/dev/null 2>&1; then
    TOOL_NAME="$(jq -r '.tool_name // ""' <<<"$INPUT" 2>/dev/null || true)"
    FILE_PATH="$(jq -r '.tool_input.file_path // ""' <<<"$INPUT" 2>/dev/null || true)"
fi

case "$TOOL_NAME" in
    Edit|Write|MultiEdit) ;;
    *) exit 0 ;;
esac

[[ -n "$FILE_PATH" ]] || exit 0
[[ -f "$FILE_PATH" ]] || exit 0

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
LOG_DIR="$REPO_ROOT/.agents/ao"
LOG_FILE="$LOG_DIR/edit-audit.log"
mkdir -p "$LOG_DIR" 2>/dev/null || exit 0

if command -v shasum >/dev/null 2>&1; then
    SHA256="$(shasum -a 256 "$FILE_PATH" 2>/dev/null | awk '{print $1}')"
elif command -v sha256sum >/dev/null 2>&1; then
    SHA256="$(sha256sum "$FILE_PATH" 2>/dev/null | awk '{print $1}')"
else
    SHA256=""
fi

REL_PATH="$FILE_PATH"
if [[ "$FILE_PATH" == "$REPO_ROOT"/* ]]; then
    REL_PATH="${FILE_PATH#$REPO_ROOT/}"
fi

GIT_STATUS=""
HEAD_SHA=""
if git -C "$REPO_ROOT" rev-parse --git-dir >/dev/null 2>&1; then
    GIT_STATUS="$(git -C "$REPO_ROOT" status --porcelain -- "$REL_PATH" 2>/dev/null | head -1 | tr -d '\n')"
    HEAD_SHA="$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null)"
fi

TS="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

# Build JSON without jq dependency (jq output already escaped above; we control the rest)
escape() {
    printf '%s' "$1" | python3 -c 'import sys,json; print(json.dumps(sys.stdin.read()), end="")' 2>/dev/null \
        || printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g' | awk '{printf "\"%s\"", $0}'
}

{
    printf '{"ts":%s,"tool":%s,"file":%s,"sha256":%s,"git_status":%s,"head_sha":%s}\n' \
        "$(escape "$TS")" \
        "$(escape "$TOOL_NAME")" \
        "$(escape "$REL_PATH")" \
        "$(escape "$SHA256")" \
        "$(escape "$GIT_STATUS")" \
        "$(escape "$HEAD_SHA")"
} >> "$LOG_FILE" 2>/dev/null || true

exit 0
