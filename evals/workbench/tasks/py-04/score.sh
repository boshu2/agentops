#!/usr/bin/env bash
set -euo pipefail

WORKDIR="${1:?Usage: score.sh <workdir>}"
cd "$WORKDIR"

if [ -d ".venv" ]; then
  source .venv/bin/activate
else
  echo "ERROR: no .venv found in $WORKDIR" >&2
  echo '{"score": 0, "total": 3, "pass": false}'
  exit 0
fi

score=0
total=3

# Check 1: pytest passes
if python -m pytest tests/ -q >/dev/null 2>&1; then
  score=$((score + 1))
fi

# Check 2: no f-string SQL in db.py (grep for f"...{...}..." patterns in execute calls)
if ! grep -Pn 'execute\(f["\x27]' app/db.py >/dev/null 2>&1; then
  score=$((score + 1))
fi

# Check 3: all queries use parameterized ? placeholders
# Count execute() calls and verify each one with a WHERE/VALUES/SET uses ?
failed=0
while IFS= read -r line; do
  # Skip CREATE TABLE and COUNT(*) — they don't need params
  if echo "$line" | grep -qE '(CREATE TABLE|SELECT COUNT)'; then
    continue
  fi
  # Lines with WHERE, VALUES, SET, LIMIT, OFFSET should use ? not f-strings
  if echo "$line" | grep -qE '(WHERE|VALUES|LIMIT|OFFSET)'; then
    if ! echo "$line" | grep -q '?'; then
      failed=$((failed + 1))
    fi
  fi
done < <(grep 'execute(' app/db.py)

if [ "$failed" -eq 0 ]; then
  score=$((score + 1))
fi

pass=false
if [ "$score" -eq "$total" ]; then
  pass=true
fi

echo "{\"score\": $score, \"total\": $total, \"pass\": $pass}"
