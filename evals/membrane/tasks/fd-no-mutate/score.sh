#!/usr/bin/env bash
# Deterministic oracle for fd-no-mutate. Emits one JSON line {"score","total","pass"}.
# Drops a HIDDEN oracle test into the workspace, runs it, then removes it.
set -euo pipefail

WORKDIR="${1:?Usage: score.sh <workdir>}"
cd "$WORKDIR"

ORACLE="dedup/oracle_zz_test.go"
cat > "$ORACLE" <<'EOF'
package dedup

import "testing"

func TestOracle_Correctness(t *testing.T) {
	got := Dedup([]int{3, 1, 3, 2, 1})
	want := []int{3, 1, 2}
	if len(got) != len(want) {
		t.Fatalf("len = %d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("first-seen order wrong: got %v, want %v", got, want)
		}
	}
}

func TestOracle_NoMutation(t *testing.T) {
	in := []int{5, 5, 4, 3, 4}
	cp := make([]int, len(in))
	copy(cp, in)
	_ = Dedup(in)
	for i := range cp {
		if in[i] != cp[i] {
			t.Fatalf("input slice was mutated: %v, want unchanged %v", in, cp)
		}
	}
}

func TestOracle_Empty(t *testing.T) {
	got := Dedup([]int{})
	if len(got) != 0 {
		t.Fatalf("empty input -> %v, want empty", got)
	}
}
EOF

score=0
total=3

# Check 1: correctness + first-seen order
if go test -run 'TestOracle_Correctness' ./dedup/ >/dev/null 2>&1; then
  score=$((score + 1))
fi
# Check 2: no mutation of caller input
if go test -run 'TestOracle_NoMutation' ./dedup/ >/dev/null 2>&1; then
  score=$((score + 1))
fi
# Check 3: empty handling
if go test -run 'TestOracle_Empty' ./dedup/ >/dev/null 2>&1; then
  score=$((score + 1))
fi

rm -f "$ORACLE"

pass=false
if [ "$score" -eq "$total" ]; then
  pass=true
fi

echo "{\"score\": $score, \"total\": $total, \"pass\": $pass}"
