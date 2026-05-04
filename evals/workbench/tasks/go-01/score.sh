#!/usr/bin/env bash
set -euo pipefail

WORKDIR="${1:?Usage: score.sh <workdir>}"
cd "$WORKDIR"

score=0
total=3

# Check 1: go test ./... passes
if go test ./... >/dev/null 2>&1; then
  score=$((score + 1))
fi

# Check 2: go vet ./... clean
if go vet ./... >/dev/null 2>&1; then
  score=$((score + 1))
fi

# Check 3: Multiply function exists and returns a * b (no off-by-one)
# Look for the correct implementation line
if grep -qP '^\s*return a \* b\s*$' internal/calc/calc.go; then
  score=$((score + 1))
fi

pass=false
if [ "$score" -eq "$total" ]; then
  pass=true
fi

echo "{\"score\": $score, \"total\": $total, \"pass\": $pass}"
