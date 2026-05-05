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

# Check 2: Divide(10,2) works correctly — write a small Go test
cat > /tmp/go_eval_divide_test.go << 'GOEOF'
package calc

import (
	"math"
	"testing"
)

func TestDivideEvalCorrect(t *testing.T) {
	got, err := Divide(10, 2)
	if err != nil {
		t.Fatalf("Divide(10,2) unexpected error: %v", err)
	}
	if math.Abs(got-5.0) > 1e-9 {
		t.Errorf("Divide(10,2) = %v, want 5", got)
	}
}
GOEOF
cp /tmp/go_eval_divide_test.go "$WORKDIR/internal/calc/eval_divide_test.go"
if go test ./internal/calc/ -run TestDivideEvalCorrect >/dev/null 2>&1; then
  score=$((score + 1))
fi
rm -f "$WORKDIR/internal/calc/eval_divide_test.go" /tmp/go_eval_divide_test.go

# Check 3: Divide(x,0) returns an error
cat > /tmp/go_eval_divzero_test.go << 'GOEOF'
package calc

import "testing"

func TestDivideEvalByZero(t *testing.T) {
	_, err := Divide(5, 0)
	if err == nil {
		t.Fatal("Divide(5,0) expected error, got nil")
	}
}
GOEOF
cp /tmp/go_eval_divzero_test.go "$WORKDIR/internal/calc/eval_divzero_test.go"
if go test ./internal/calc/ -run TestDivideEvalByZero >/dev/null 2>&1; then
  score=$((score + 1))
fi
rm -f "$WORKDIR/internal/calc/eval_divzero_test.go" /tmp/go_eval_divzero_test.go

# Check 4: Test cases exist for Divide in calc_test.go
if grep -qP 'func Test\w*Divide\w*\(t \*testing\.T\)' internal/calc/calc_test.go || \
   grep -qP '["\x27]divide' internal/calc/calc_test.go; then
  score=$((score + 1))
fi

pass=false
if [ "$score" -eq "$total" ]; then
  pass=true
fi

echo "{\"score\": $score, \"total\": $total, \"pass\": $pass}"
