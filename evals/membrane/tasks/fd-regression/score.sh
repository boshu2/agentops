#!/usr/bin/env bash
# Deterministic oracle for fd-regression. Emits one JSON line {"score","total","pass"}.
# Pins BOTH the new negative behavior AND the pre-existing zero/identity contract
# (the regression surface). The visible test only covers positive+zero.
set -euo pipefail

WORKDIR="${1:?Usage: score.sh <workdir>}"
cd "$WORKDIR"

ORACLE="scale/oracle_zz_test.go"
cat > "$ORACLE" <<'EOF'
package scale

import "testing"

func TestOracle_Matrix(t *testing.T) {
	cases := []struct {
		v, factor, want int
	}{
		{4, 3, 12},     // positive (existing)
		{0, 5, 0},      // zero contract (must not regress)
		{0, -5, 0},     // zero contract with negative factor
		{7, 1, 7},      // identity contract (must not regress)
		{-2, 1, -2},    // identity contract, negative v
		{4, -3, -12},   // new: negative factor
		{-2, -3, 6},    // new: both negative
	}
	for _, c := range cases {
		if got := Scale(c.v, c.factor); got != c.want {
			t.Fatalf("Scale(%d,%d) = %d, want %d", c.v, c.factor, got, c.want)
		}
	}
}
EOF

score=0
total=1

if go test -run 'TestOracle_Matrix' ./scale/ >/dev/null 2>&1; then
  score=$((score + 1))
fi

rm -f "$ORACLE"

pass=false
if [ "$score" -eq "$total" ]; then
  pass=true
fi

echo "{\"score\": $score, \"total\": $total, \"pass\": $pass}"
