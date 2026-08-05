#!/usr/bin/env bash
# setup.sh <workdir> — materialize the t02 fixture (hermetic, no network).
set -euo pipefail
W="${1:?usage: setup.sh <workdir>}"
mkdir -p "$W"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cp "$HERE/prompt.md" "$HERE/plan.md" "$W/"
cat > "$W/go.mod" <<'GOMOD'
module t02

go 1.22
GOMOD
cat > "$W/port.go" <<'GO'
package t02

// ValidatePort parses and validates a decimal TCP port string.
// TODO: implement per plan.md.
func ValidatePort(raw string) (int, error) {
	panic("not implemented")
}
GO
cat > "$W/port_test.go" <<'GO'
package t02

import "testing"

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
		err  bool
	}{
		{"valid", "8080", 8080, false},
		{"minimum", "1", 1, false},
		{"maximum", "65535", 65535, false},
		{"zero rejected", "0", 0, true},
		{"too large rejected", "65536", 0, true},
		{"non-numeric rejected", "http", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidatePort(tt.in)
			if tt.err != (err != nil) {
				t.Fatalf("err = %v, want err=%v", err, tt.err)
			}
			if !tt.err && got != tt.want {
				t.Errorf("ValidatePort(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}
GO
echo "fixture ready: $W"
