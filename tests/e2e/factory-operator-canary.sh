#!/usr/bin/env bash
# tests/e2e/factory-operator-canary.sh — Wave 0 operator canary for soc-kizn.11
#
# Narrow L1/L2 smoke. Prints the five operator-surface anchors a fresh-checkout
# user touches: briefing path, next delivery command, validation command,
# evidence/proof path, closeout path. For each, reports either the resolved
# path/command or an explicit "unavailable" reason.
#
# This canary does NOT claim factory yield, autonomous merge, or L3 proof.
# Full Codex skill/runtime parity cleanup remains in soc-kizn.8.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

UNAVAILABLE=0

emit_path() {
  local label="$1"
  local path="$2"
  if [[ -n "$path" && -e "$path" ]]; then
    printf '%s: %s\n' "$label" "$path"
  else
    UNAVAILABLE=$((UNAVAILABLE + 1))
    if [[ -z "$path" ]]; then
      printf '%s: unavailable (no path resolved)\n' "$label"
    else
      printf '%s: unavailable (missing on disk: %s)\n' "$label" "$path"
    fi
  fi
}

emit_command() {
  local label="$1"
  local cmd="$2"
  local impl_path="${3:-}"
  if [[ -n "$cmd" && ( -z "$impl_path" || -e "$impl_path" ) ]]; then
    printf '%s: %s\n' "$label" "$cmd"
  else
    UNAVAILABLE=$((UNAVAILABLE + 1))
    printf '%s: unavailable (implementation missing: %s)\n' "$label" "$impl_path"
  fi
}

# 1. Briefing path — static onboarding doc a fresh-checkout operator lands on.
emit_path "briefing path" "docs/newcomer-guide.md"

# 2. Next delivery command — canonical RPI operator entry point.
emit_command "next delivery command" '/rpi "<goal>"' "skills/rpi/SKILL.md"

# 3. Validation command — pre-push gate.
emit_command "validation command" "scripts/pre-push-gate.sh --fast" "scripts/pre-push-gate.sh"

# 4. Evidence/proof path — current execution packet (per-run; may be absent on
#    fresh checkout, which is an explicit-unavailable case, not a failure).
if [[ -f .agents/rpi/execution-packet.json ]]; then
  emit_path "evidence/proof path" ".agents/rpi/execution-packet.json"
elif [[ -d .agents/rpi ]]; then
  printf 'evidence/proof path: unavailable (no current execution packet at .agents/rpi/execution-packet.json; directory present)\n'
  UNAVAILABLE=$((UNAVAILABLE + 1))
else
  printf 'evidence/proof path: unavailable (.agents/rpi/ not yet populated on this checkout)\n'
  UNAVAILABLE=$((UNAVAILABLE + 1))
fi

# 5. Closeout path — bd issue closure.
if command -v bd >/dev/null 2>&1; then
  printf 'closeout path: bd close <id>\n'
else
  printf 'closeout path: unavailable (bd CLI not on PATH)\n'
  UNAVAILABLE=$((UNAVAILABLE + 1))
fi

echo
echo "canary status: $UNAVAILABLE surface(s) reported unavailable"
echo "scope: L1/L2 — does not claim factory yield, autonomous merge, or L3 proof"

# Exit 0: bead acceptance is "verifies those paths exist OR records an explicit
# unavailable reason for each unavailable artifact." Both are passes.
exit 0
