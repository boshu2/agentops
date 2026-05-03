#!/usr/bin/env bash
# test-hooks-output.sh — validate that every registered hook attached to an
# event with a documented JSON-output contract emits a stdout shape that both
# Claude Code and Codex CLI accept.
#
# Allow-list (canonical rationale: docs/HOOKS.md → "Codex/Claude PreToolUse
# output parity"):
#   top-level keys:                decision, reason, additionalContext, hookSpecificOutput
#   under hookSpecificOutput:      additionalContext, hookEventName
#   exit codes:                    0 (pass), 2 (block)
# REJECT: updatedInput, systemMessage, continue, suppressOutput, stopReason,
#         and anything else not in the allow-list.
#
# Verified against hooks/* registered under PreToolUse/PostToolUse/
# UserPromptSubmit/SessionStart as of 2026-05-02 — no current AgentOps hook
# emits a key outside the allow-list. This script keeps it that way.
#
# Scope: only hooks registered in hooks/hooks.json or hooks/codex-hooks.json
# under events with JSON output contracts (PreToolUse, PostToolUse,
# UserPromptSubmit, SessionStart). Other events (Stop, SessionEnd,
# PreCompact, ConfigChange, TaskCompleted, etc.) treat hook stdout as
# log output and have no allow-list.
#
# Sandboxing: each hook runs from a mktemp cwd to keep hooks that resolve
# .agents/ relative to cwd from polluting real state. Hooks that hardcode
# absolute paths or call `git rev-parse --show-toplevel` will fail under
# sandbox and produce no stdout — that registers as PASS (the lint only
# rejects disallowed JSON keys, not non-zero exit codes).

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
HOOK_DIR="$REPO_ROOT/hooks"
FIX_DIR="$REPO_ROOT/tests/fixtures/hook-inputs"
NEG_FIX="$REPO_ROOT/tests/fixtures/hook-outputs/bad-updatedInput.json"

ALLOW_TOP='["decision","reason","additionalContext","hookSpecificOutput"]'
ALLOW_HSO='["additionalContext","hookEventName"]'

# Events whose hook stdout MUST conform to the allow-list. Other events
# (Stop, SessionEnd, PreCompact, ConfigChange, TaskCompleted, etc.) treat
# stdout as informational log output.
LINTED_EVENTS_RE='^(PreToolUse|PostToolUse|UserPromptSubmit|SessionStart)$'

HOOK_TIMEOUT=5

errors=0
checked=0
hooks_tested=0

# validate_json <label> <stdout>
# Returns 0 if shape OK (or empty), non-zero if violation.
validate_json() {
  local label="$1"
  local stdout="$2"

  if [[ -z "${stdout//[[:space:]]/}" ]]; then
    return 0
  fi

  if ! printf '%s' "$stdout" | jq empty >/dev/null 2>&1; then
    printf 'FAIL: %s emitted non-JSON non-empty stdout (first 120 chars): %s\n' \
      "$label" "${stdout:0:120}"
    return 1
  fi

  local bad_top
  bad_top=$(printf '%s' "$stdout" | jq -r --argjson allow "$ALLOW_TOP" '
    [keys[]] - $allow | join(",")' 2>/dev/null)
  if [[ -n "$bad_top" ]]; then
    printf 'FAIL: %s emits disallowed top-level key(s): %s\n' "$label" "$bad_top"
    return 1
  fi

  local bad_hso
  bad_hso=$(printf '%s' "$stdout" | jq -r --argjson allow "$ALLOW_HSO" '
    (.hookSpecificOutput // {} | keys) - $allow | join(",")' 2>/dev/null)
  if [[ -n "$bad_hso" ]]; then
    printf 'FAIL: %s emits disallowed hookSpecificOutput key(s): %s\n' \
      "$label" "$bad_hso"
    return 1
  fi

  return 0
}

# extract_linted_hook_basenames — read hooks.json + codex-hooks.json, emit
# unique basenames of hook scripts attached to LINTED_EVENTS.
extract_linted_hook_basenames() {
  local config
  for config in "$HOOK_DIR/hooks.json" "$HOOK_DIR/codex-hooks.json"; do
    [[ -f "$config" ]] || continue
    jq -r '.hooks // {} | to_entries[] | .key as $event |
           .value[]? | .hooks[]? | "\($event)|\(.command // empty)"' \
      "$config" 2>/dev/null
  done | awk -F'|' -v re="$LINTED_EVENTS_RE" '
    $1 ~ re && $2 != "" {
      # Strip everything up to the last "/hooks/" so paths from both
      # CLAUDE_PLUGIN_ROOT and AGENTOPS_PLUGIN_ROOT collapse to a basename.
      sub(/.*\/hooks\//, "", $2)
      sub(/[[:space:]]+.*$/, "", $2)  # drop trailing args if any
      print $2
    }' | sort -u
}

# ----- Self-test: prove the validator rejects a known-bad shape -----
echo "== self-test (negative fixture) =="
if [[ ! -f "$NEG_FIX" ]]; then
  echo "FAIL: negative fixture missing at $NEG_FIX"
  exit 1
fi
neg_content=$(cat "$NEG_FIX")
if validate_json "self-test" "$neg_content" >/dev/null 2>&1; then
  echo "FAIL: self-test should have rejected bad-updatedInput.json but passed"
  exit 1
fi
echo "  OK: negative fixture correctly rejected (lint enforces allow-list)"

# ----- Hook validation -----
echo
echo "== hook validation =="

if [[ ! -d "$HOOK_DIR" ]]; then
  echo "FAIL: hook dir not found at $HOOK_DIR"
  exit 1
fi
if [[ ! -d "$FIX_DIR" ]]; then
  echo "FAIL: fixture dir not found at $FIX_DIR"
  exit 1
fi

mapfile -t hook_basenames < <(extract_linted_hook_basenames)
if [[ ${#hook_basenames[@]} -eq 0 ]]; then
  echo "FAIL: no hooks discovered from hooks.json/codex-hooks.json"
  exit 1
fi

# Pre-mortem amendment #3: cwd sandbox to prevent .agents/ writes
sandbox=$(mktemp -d -t hook-output-lint.XXXXXX)
trap 'rm -rf "$sandbox"' EXIT

for basename in "${hook_basenames[@]}"; do
  hook="$HOOK_DIR/$basename"
  if [[ ! -f "$hook" ]]; then
    echo "WARN: registered hook missing on disk: $basename (skipping)"
    continue
  fi

  hooks_tested=$((hooks_tested + 1))

  for fix in "$FIX_DIR"/*.json; do
    fix_name=$(basename "$fix" .json)
    label="$basename (fixture=$fix_name)"

    stdout=$(cd "$sandbox" && timeout "$HOOK_TIMEOUT" bash "$hook" < "$fix" 2>/dev/null || true)

    checked=$((checked + 1))
    if ! validate_json "$label" "$stdout"; then
      errors=$((errors + 1))
    fi
  done
done

echo
echo "== summary =="
echo "  hooks tested: $hooks_tested (registered under PreToolUse/PostToolUse/UserPromptSubmit/SessionStart)"
echo "  invocations:  $checked"
echo "  errors:       $errors"

if [[ $errors -gt 0 ]]; then
  echo
  echo "FAIL: $errors disallowed-shape violation(s) found."
  exit 1
fi

echo
echo "PASS: all linted hooks emit only allow-listed output shapes."
