#!/usr/bin/env bash
# PostToolUse hook: auto-refresh codex artifacts after edits to skills/<name>/.
# practices: [gitops, continuous-integration]
#
# Filed as soc-7qq9 in the 2026-05-07 CI-push-gate-toil retrospective. The
# pre-push gate enforces codex parity (skills/* must have matching
# skills-codex/<name>/.agents-generated.json hashes), but the refresh is
# manual via scripts/refresh-codex-artifacts.sh. Discovering the drift only
# at push time costs one round-trip per skill-edit PR.
#
# This hook moves discovery to edit time: after every Edit/Write under
# skills/<name>/, run the refresh script in --scope head mode (auto-recompute
# hashes for the touched skill). Operator never sees the parity warning.
#
# Disable with AGENTOPS_HOOKS_DISABLED=1 or AGENTOPS_CODEX_AUTOREFRESH_DISABLED=1.
# Best-effort and silent on success; failures log to stderr but exit 0
# (informational hook — never blocks the editor).

set -uo pipefail

[[ "${AGENTOPS_HOOKS_DISABLED:-}" == "1" ]] && exit 0
[[ "${AGENTOPS_CODEX_AUTOREFRESH_DISABLED:-}" == "1" ]] && exit 0

# Read the JSON input from stdin (Claude Code hook protocol).
INPUT="$(cat)"

# Extract the edited file_path. If jq is unavailable or the input is not JSON,
# fail silent — hook is best-effort.
if ! command -v jq >/dev/null 2>&1; then
    exit 0
fi

FILE_PATH="$(jq -r '.tool_input.file_path // empty' <<<"$INPUT" 2>/dev/null || true)"

# Only fire on edits under skills/<name>/. Skip non-skill edits, skip
# skills-codex/ edits (those are the targets, not sources), skip
# .agents-generated.json edits (would re-trigger ourselves).
case "$FILE_PATH" in
    */skills/*/SKILL.md|*/skills/*/references/*.md|*/skills/*/scripts/*|*/skills/*/schemas/*) ;;
    *) exit 0 ;;
esac

# Resolve repo root from the file path (assumes file lives inside the repo).
REPO_ROOT="$(cd "$(dirname "$FILE_PATH")" 2>/dev/null && git rev-parse --show-toplevel 2>/dev/null || true)"
[[ -z "$REPO_ROOT" ]] && exit 0

REFRESH="$REPO_ROOT/scripts/refresh-codex-artifacts.sh"
[[ -x "$REFRESH" ]] || exit 0

# Run refresh in scope=head (only changed skills since the last commit).
# Output is suppressed unless the script returns non-zero, in which case we
# log a one-line warning to stderr but never block the editor.
if ! out=$("$REFRESH" --scope head 2>&1); then
    printf 'codex auto-refresh: WARN — refresh-codex-artifacts exited non-zero (run manually for details)\n' >&2
    [[ "${AGENTOPS_CODEX_AUTOREFRESH_VERBOSE:-}" == "1" ]] && printf '%s\n' "$out" >&2 || true
fi

exit 0
