#!/usr/bin/env bash
set -euo pipefail

[[ $# -eq 1 ]] || { echo "usage: validate-summary.sh <summary.md>" >&2; exit 2; }
SUMMARY="$1"
[[ -s "$SUMMARY" ]] || { echo "refactor-summary: missing or empty $SUMMARY" >&2; exit 1; }

NAME="$(basename "$SUMMARY")"
[[ "$NAME" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}-refactor-[A-Za-z0-9._-]+\.md$ ]] || {
  echo "refactor-summary: invalid filename $NAME" >&2
  exit 1
}

grep -Eq '^# Refactor: .+' "$SUMMARY"
grep -Eq '^\*\*Date:\*\* [0-9]{4}-[0-9]{2}-[0-9]{2}$' "$SUMMARY"
grep -Eq '^\*\*Mode:\*\* (target|sweep|extract)$' "$SUMMARY"
grep -Eq '^\*\*Files changed:\*\* [0-9]+$' "$SUMMARY"

for heading in '## Targets' '## Metrics' '## Transformations Applied' '## Tests' '## Learnings'; do
  grep -Fqx -- "$heading" "$SUMMARY" || {
    echo "refactor-summary: missing $heading" >&2
    exit 1
  }
done

grep -Fqx '| Metric | Before | After | Delta |' "$SUMMARY"
grep -Eq '^\| [^|]+ \| -?[0-9]+([.][0-9]+)? \| -?[0-9]+([.][0-9]+)? \| [-+]?[0-9]+([.][0-9]+)? \|$' "$SUMMARY"
grep -Eq '^[0-9]+\. .+ -- [0-9a-f]{7,40}$' "$SUMMARY"
[[ "$(grep -Ec '^- Baseline: [0-9]+ passing, [0-9]+ failing, [0-9]+ skipped$' "$SUMMARY")" -eq 1 ]]
[[ "$(grep -Ec '^- Final: [0-9]+ passing, [0-9]+ failing, [0-9]+ skipped$' "$SUMMARY")" -eq 1 ]]
grep -Eq '^- New tests added: [0-9]+$' "$SUMMARY"

echo "OK: $SUMMARY"
