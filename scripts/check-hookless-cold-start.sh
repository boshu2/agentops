#!/usr/bin/env bash
# check-hookless-cold-start.sh — scenario-2 regression gate for soc-qh2g1.
#
# AgentOps 3.0 is hookless: no SessionStart/Stop hook ships, so no cold-start or
# continuity doc/skill may present a `hooks/<name>.sh` path as a CURRENTLY ACTIVE
# surface. A hook path is allowed only on a line that hedges it as opt-in /
# author-it-yourself / historical / removed (the hooks-authoring escape hatch).
#
# Scope is a FIXED list of cold-start / continuity surfaces. It deliberately does
# NOT scan all of docs/+skills/ (release notes, the hooks-authoring skill, and
# scope guards legitimately name hook paths) and deliberately excludes the repo
# CLAUDE.md (its workflow-discipline `session-pr-counter.sh` reference is tracked
# separately, out of cold-start scope).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

FILES=(
  "AGENTS.md"
  "docs/architecture/primitive-chains.md"
  "skills/session-bootstrap/SKILL.md"
  "skills-codex/session-bootstrap/SKILL.md"
  "docs/newcomer-guide.md"
)

# A hook path is "hedged" (allowed) when its line also carries one of these.
# These hedge ARTIFACT EXISTENCE (opt-in / author-it-yourself / historical /
# removed). "When supported" is deliberately NOT here: it hedges harness
# capability, not whether AgentOps ships the hook (it does not).
HEDGE='opt-in|optional|author|if you author|hooks-authoring|historical|removed|no such hook|ships no|example|if installed|when hooks are installed|when installed'
HOOK_PATH='hooks/[A-Za-z0-9_.-]+\.sh'

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

violations=0
scanned=0
for rel in "${FILES[@]}"; do
  file="$ROOT/$rel"
  [[ -f "$file" ]] || continue
  scanned=$((scanned + 1))
  while IFS= read -r match; do
    [[ -n "$match" ]] || continue
    lineno="${match%%:*}"
    content="${match#*:}"
    if ! grep -qiE "$HEDGE" <<<"$content"; then
      printf 'FAIL: %s:%s presents a hook path as an active surface without an opt-in/historical hedge\n' \
        "$rel" "$lineno" >&2
      printf '      %s\n' "$content" >&2
      violations=$((violations + 1))
    fi
  done < <(grep -nE "$HOOK_PATH" "$file" || true)
done

[[ "$scanned" -gt 0 ]] || fail "no cold-start surfaces found under $ROOT"

if [[ "$violations" -gt 0 ]]; then
  fail "$violations stale hook promise(s) in cold-start/continuity surfaces. Reframe as explicit \`ao\` commands or hedge as opt-in (author via the hooks-authoring skill)."
fi

printf 'PASS: no stale hook-context promises in %d cold-start/continuity surface(s)\n' "$scanned"
