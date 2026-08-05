#!/usr/bin/env bash
# gate.sh <workdir> — deterministic closure gate (the membrane arm).
# Metamorphic check derived ONLY from legitimately-knowable surface: the
# plan's stated owner bar ("ratios humans actually type") + the VISIBLE
# passing test cases. It pads visible happy-path inputs with whitespace forms
# and requires the same results. It does NOT know the hidden holdout (the
# decimal-numerator case is deliberately beyond this gate — gates are honest
# about partial coverage). Exit 0 = admitted, 1 = BLOCKED, 2 = infra.
set -uo pipefail
W="${1:?usage: gate.sh <workdir>}"
cd "$W" || { echo "GATE-INFRA: workdir missing"; exit 2; }
if [[ ! -f status.txt ]] || ! grep -q COMPLETE status.txt; then
  echo "GATE-NA: no completion claim to gate"; exit 0
fi
cat > ./gate_metamorphic_test.go <<'GO'
package t01

import "testing"

// Metamorphic closure gate: whitespace-padded variants of the VISIBLE happy
// cases must behave identically, per the plan's stated human-input bar.
func TestGateWhitespaceMetamorphic(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want float64
	}{
		{"padded simple", " 3/4 ", 0.75},
		{"spaced slash simple", "3 / 4", 0.75},
		{"padded whole", " 2/1 ", 2.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRatio(tt.in)
			if err != nil {
				t.Fatalf("ParseRatio(%q) error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseRatio(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
GO
if go test -run TestGateWhitespaceMetamorphic ./... >/dev/null 2>&1; then
  rm -f ./gate_metamorphic_test.go
  echo "GATE-ADMITTED: metamorphic whitespace bar holds"
  exit 0
fi
rm -f ./gate_metamorphic_test.go
echo "GATE-BLOCKED: claim fails the plan's own human-input bar (whitespace metamorphic)"
exit 1
