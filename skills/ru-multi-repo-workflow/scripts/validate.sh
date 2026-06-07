#!/usr/bin/env bash
# validate.sh — structural check for the ru-multi-repo-workflow skill.
#
# The CI skill lint job runs in a clean GitHub runner. It must not require Bo's
# local `ru` installation or authenticated GitHub state. Set
# AGENTOPS_VALIDATE_LIVE_TOOLS=1 for the operator health check.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
SKILL_MD="$SKILL_DIR/SKILL.md"

fail=0
err() { printf 'FAIL: %s\n' "$1" >&2; fail=1; }
ok() { printf 'ok:   %s\n' "$1"; }

[ -f "$SKILL_MD" ] || { err "SKILL.md missing"; exit 1; }
head -n1 "$SKILL_MD" | grep -qx -- '---' || err "frontmatter must open with ---"
grep -q '^name: ru-multi-repo-workflow$' "$SKILL_MD" || err "name must be ru-multi-repo-workflow"
grep -q '^description:' "$SKILL_MD" || err "description missing"
grep -q '^skill_api_version:' "$SKILL_MD" || err "skill_api_version missing"
grep -q 'ru sync' "$SKILL_MD" || err "ru sync workflow missing"
grep -q 'ru review' "$SKILL_MD" || err "ru review workflow missing"
ok "skill artifact structure present"

if [ "${AGENTOPS_VALIDATE_LIVE_TOOLS:-0}" = "1" ]; then
  command -v ru >/dev/null 2>&1 || err "ru is not installed or not in PATH"
  command -v gh >/dev/null 2>&1 || err "gh is not installed or not in PATH"
  command -v jq >/dev/null 2>&1 || err "jq is not installed or not in PATH"
  command -v git >/dev/null 2>&1 || err "git is not installed or not in PATH"
  if command -v gh >/dev/null 2>&1; then
    gh auth status >/dev/null 2>&1 || err "gh is not authenticated"
    gh api rate_limit >/dev/null 2>&1 || err "GitHub API connectivity failed"
  fi
  if command -v ru >/dev/null 2>&1; then
    ru doctor >/dev/null 2>&1 || ok "ru doctor reported issues; run ru doctor for details"
  fi
else
  ok "live ru/gh smoke skipped (set AGENTOPS_VALIDATE_LIVE_TOOLS=1 to enable)"
fi

if [ "$fail" -eq 0 ]; then
  printf '\nPASS: ru-multi-repo-workflow skill artifact is valid.\n'
  exit 0
fi

printf '\nFAILED: ru-multi-repo-workflow skill artifact validation failed.\n' >&2
exit 1
