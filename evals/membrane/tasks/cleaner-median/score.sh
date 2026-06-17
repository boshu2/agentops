#!/usr/bin/env bash
# Deterministic oracle for cleaner-median. Emits one JSON line {"score","total","pass"}.
set -euo pipefail

WORKDIR="${1:?Usage: score.sh <workdir>}"
cd "$WORKDIR"

ORACLE="stats/oracle_zz_test.go"
cat > "$ORACLE" <<'EOF'
package stats

import "testing"

func TestOracle_Empty(t *testing.T) {
	if got := Median([]int{}); got != 0 {
		t.Fatalf("Median(empty) = %v, want 0", got)
	}
}

func TestOracle_Odd(t *testing.T) {
	if got := Median([]int{5, 1, 3}); got != 3 {
		t.Fatalf("Median(5,1,3) = %v, want 3", got)
	}
}

func TestOracle_Even(t *testing.T) {
	if got := Median([]int{4, 1, 3, 2}); got != 2.5 {
		t.Fatalf("Median(4,1,3,2) = %v, want 2.5", got)
	}
}

func TestOracle_NoMutation(t *testing.T) {
	in := []int{9, 1, 5, 3}
	cp := make([]int, len(in))
	copy(cp, in)
	_ = Median(in)
	for i := range cp {
		if in[i] != cp[i] {
			t.Fatalf("input mutated: %v, want %v", in, cp)
		}
	}
}
EOF

score=0
total=4

if go test -run 'TestOracle_Empty' ./stats/ >/dev/null 2>&1; then
  score=$((score + 1))
fi
if go test -run 'TestOracle_Odd' ./stats/ >/dev/null 2>&1; then
  score=$((score + 1))
fi
if go test -run 'TestOracle_Even' ./stats/ >/dev/null 2>&1; then
  score=$((score + 1))
fi
if go test -run 'TestOracle_NoMutation' ./stats/ >/dev/null 2>&1; then
  score=$((score + 1))
fi

rm -f "$ORACLE"

pass=false
if [ "$score" -eq "$total" ]; then
  pass=true
fi

echo "{\"score\": $score, \"total\": $total, \"pass\": $pass}"
