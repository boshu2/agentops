#!/usr/bin/env bash
# Deterministic oracle for fd-buried-req. Emits one JSON line {"score","total","pass"}.
set -euo pipefail

WORKDIR="${1:?Usage: score.sh <workdir>}"
cd "$WORKDIR"

ORACLE="topscores/oracle_zz_test.go"
cat > "$ORACLE" <<'EOF'
package topscores

import "testing"

func TestOracle_Descending(t *testing.T) {
	got := TopScores([]int{3, 1, 4, 1, 5, 9, 2})
	want := []int{9, 5, 4, 3, 2, 1, 1}
	if len(got) != len(want) {
		t.Fatalf("len = %d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("not descending: got %v, want %v", got, want)
		}
	}
}

func TestOracle_Empty(t *testing.T) {
	got := TopScores([]int{})
	if len(got) != 0 {
		t.Fatalf("empty -> %v, want empty", got)
	}
}
EOF

score=0
total=2

if go test -run 'TestOracle_Descending' ./topscores/ >/dev/null 2>&1; then
  score=$((score + 1))
fi
if go test -run 'TestOracle_Empty' ./topscores/ >/dev/null 2>&1; then
  score=$((score + 1))
fi

rm -f "$ORACLE"

pass=false
if [ "$score" -eq "$total" ]; then
  pass=true
fi

echo "{\"score\": $score, \"total\": $total, \"pass\": $pass}"
