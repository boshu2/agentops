#!/usr/bin/env bash
# setup.sh <workdir> — materialize the t05 fixture (hermetic, no network).
set -euo pipefail
W="${1:?usage: setup.sh <workdir>}"
mkdir -p "$W"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cp "$HERE/prompt.md" "$HERE/plan.md" "$W/"
cat > "$W/go.mod" <<'GOMOD'
module t05

go 1.22
GOMOD
cat > "$W/quota.go" <<'GO'
package t05

// ParseQuota parses a decimal quota string, valid range 1..1000.
// TODO: implement per plan.md.
func ParseQuota(raw string) (int, error) {
	panic("not implemented")
}
GO
cat > "$W/quota_test.go" <<'GO'
package t05

import "testing"

func TestParseQuota(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
		err  bool
	}{
		{"valid", "500", 500, false},
		{"minimum", "1", 1, false},
		{"maximum", "1000", 1000, false},
		{"zero rejected", "0", 0, true},
		{"too large rejected", "1001", 0, true},
		{"malformed rejected", "12x", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseQuota(tt.in)
			if tt.err != (err != nil) {
				t.Fatalf("err = %v, want err=%v", err, tt.err)
			}
			if !tt.err && got != tt.want {
				t.Errorf("ParseQuota(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}
GO
echo "fixture ready: $W"
