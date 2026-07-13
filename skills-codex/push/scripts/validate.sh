#!/usr/bin/env bash
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
SKILL="$SKILL_DIR/SKILL.md"
FEATURE="$SKILL_DIR/references/push.feature"

required=(
  'Repository policy wins.'
  'Deterministic checks only.'
  'Proof is input, not permission.'
  'No lifecycle mutation.'
  'direct push | PR | user-owned CI'
  'Push never requires another LLM landing verdict'
  'Do not create or manage a'
  'Tracker closeout and lifecycle'
)

for marker in "${required[@]}"; do
  grep -Fq -- "$marker" "$SKILL" || {
    echo "FAIL: push contract missing marker: $marker" >&2
    exit 1
  }
done

if grep -Eqi 'pawl-review|pawl-land|ao[[:space:]]+land|CONFIRMED[^.]*push|commit-bound[^.]*verdict|no verdict means no push' "$SKILL"; then
  echo "FAIL: push contract still requires an LLM landing authority" >&2
  exit 1
fi

grep -Fq 'repository-selected delivery' "$FEATURE" || {
  echo "FAIL: push feature omits repository-selected delivery" >&2
  exit 1
}
grep -Fq 'deterministic checks' "$FEATURE" || {
  echo "FAIL: push feature omits deterministic checks" >&2
  exit 1
}
grep -Fq 'does not require another LLM verdict' "$FEATURE" || {
  echo "FAIL: push feature omits no-second-LLM scenario" >&2
  exit 1
}

echo "push adapter contract: PASS"
