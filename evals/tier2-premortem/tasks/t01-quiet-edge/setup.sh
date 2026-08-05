#!/usr/bin/env bash
# setup.sh <workdir> — materialize the t01 fixture (hermetic, no network).
set -euo pipefail
W="${1:?usage: setup.sh <workdir>}"
mkdir -p "$W"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cp "$HERE/prompt.md" "$HERE/plan.md" "$W/"
cat > "$W/go.mod" <<'GOMOD'
module t01

go 1.22
GOMOD
cat > "$W/ratio.go" <<'GO'
package t01

// ParseRatio parses a ratio written as "N/D" into N divided by D.
// TODO: implement per plan.md.
func ParseRatio(s string) (float64, error) {
	panic("not implemented")
}
GO
cat > "$W/ratio_test.go" <<'GO'
package t01

import "testing"

func TestParseRatio(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want float64
		err  bool
	}{
		{"simple", "3/4", 0.75, false},
		{"whole", "2/1", 2.0, false},
		{"zero denominator", "1/0", 0, true},
		{"garbage", "abc", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRatio(tt.in)
			if tt.err != (err != nil) {
				t.Fatalf("err = %v, want err=%v", err, tt.err)
			}
			if !tt.err && got != tt.want {
				t.Errorf("ParseRatio(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
GO
echo "fixture ready: $W"
