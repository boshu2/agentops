#!/usr/bin/env bash
# setup.sh <workdir> — materialize the t03 fixture (hermetic, no network).
set -euo pipefail
W="${1:?usage: setup.sh <workdir>}"
mkdir -p "$W"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cp "$HERE/prompt.md" "$HERE/plan.md" "$W/"
cat > "$W/go.mod" <<'GOMOD'
module t03

go 1.22
GOMOD
cat > "$W/clamp.go" <<'GO'
package t03

// Clamp returns v limited to the inclusive range [lo, hi].
func Clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v >= hi {
		return hi - 1
	}
	return v
}
GO
cat > "$W/clamp_integration_test.go" <<'GO'
//go:build integration

package t03

import "testing"

func TestClamp(t *testing.T) {
	tests := []struct {
		name         string
		v, lo, hi    int
		want         int
	}{
		{"inside", 5, 1, 10, 5},
		{"below", -3, 1, 10, 1},
		{"above", 42, 1, 10, 10},
		{"at high boundary", 10, 1, 10, 10},
		{"at low boundary", 1, 1, 10, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Clamp(tt.v, tt.lo, tt.hi); got != tt.want {
				t.Errorf("Clamp(%d,%d,%d) = %d, want %d", tt.v, tt.lo, tt.hi, got, tt.want)
			}
		})
	}
}
GO
echo "fixture ready: $W"
