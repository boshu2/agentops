#!/usr/bin/env bash
set -euo pipefail

WORKDIR="${1:?Usage: score.sh <workdir>}"
cd "$WORKDIR"

score=0
total=4

# Check 1: go test ./... passes
if go test ./... >/dev/null 2>&1; then
  score=$((score + 1))
fi

# Check 2: store_test.go has a test for Delete
if grep -qP 'func Test\w*Delete\w*\(t \*testing\.T\)' internal/store/store_test.go; then
  score=$((score + 1))
fi

# Check 3: store_test.go has a test for Get with missing key (ErrKeyNotFound)
if grep -qP 'func Test\w*(GetNotFound|Get\w*Missing|Get\w*NotFound)\w*\(t \*testing\.T\)' internal/store/store_test.go; then
  score=$((score + 1))
elif grep -l 'ErrKeyNotFound' internal/store/store_test.go >/dev/null 2>&1 && \
     grep -qP 'func Test\w*Get\w*\(t \*testing\.T\)' internal/store/store_test.go; then
  # Fallback: any Get test that references ErrKeyNotFound
  score=$((score + 1))
fi

# Check 4: go test -cover shows >= 80% for store package
cover_out=$(go test -cover ./internal/store/ 2>&1)
if echo "$cover_out" | grep -qP 'coverage:\s+(\d+\.?\d*)%'; then
  pct=$(echo "$cover_out" | grep -oP 'coverage:\s+\K\d+\.?\d*')
  if awk "BEGIN { exit ($pct >= 80.0 ? 0 : 1) }"; then
    score=$((score + 1))
  fi
fi

pass=false
if [ "$score" -eq "$total" ]; then
  pass=true
fi

echo "{\"score\": $score, \"total\": $total, \"pass\": $pass}"
