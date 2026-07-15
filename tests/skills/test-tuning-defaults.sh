#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PREMORTEM="$ROOT/skills/premortem/SKILL.md"
POSTMORTEM="$ROOT/skills/postmortem/SKILL.md"
DUELING="$ROOT/skills/dueling-idea-genies/SKILL.md"
PLAN="$ROOT/skills/plan/SKILL.md"

grep -Fq 'optional plan-challenge strategy' "$PREMORTEM"
grep -Fq 'does not authorize readiness' "$PREMORTEM"
grep -Fq 'dependencies: []' "$PREMORTEM"
grep -Fq 'dependencies: []' "$POSTMORTEM"
grep -Fq 'advisory evidence for Plan' "$DUELING"
grep -Fq 'Emit no readiness' "$DUELING"
grep -Fq 'one active behavior' "$PLAN"

for path in \
  "$ROOT/skills/discovery" \
  "$ROOT/skills/goal-design" \
  "$ROOT/skills/behavior-first-planning"; do
  [[ ! -e "$path" ]]
done

echo 'optional strategy defaults: PASS'
