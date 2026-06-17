#!/usr/bin/env bash
# Stage the cleaner-median workspace. macOS-portable: heredocs only, no sed -i.
set -euo pipefail

WORKDIR="${1:?Usage: setup.sh <workdir>}"
mkdir -p "$WORKDIR/stats"

cat > "$WORKDIR/go.mod" <<'EOF'
module cleanermedian

go 1.21
EOF

cat > "$WORKDIR/stats/stats.go" <<'EOF'
package stats

// Median returns the median of xs.
// Empty input returns 0. For even-length input, average the two middle values.
func Median(xs []int) float64 {
	// TODO: implement
	return 0
}
EOF

cat > "$WORKDIR/stats/stats_test.go" <<'EOF'
package stats

import "testing"

func TestMedian_Odd(t *testing.T) {
	if got := Median([]int{3, 1, 2}); got != 2 {
		t.Fatalf("Median(3,1,2) = %v, want 2", got)
	}
}
EOF
