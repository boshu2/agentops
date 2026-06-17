#!/usr/bin/env bash
# Stage the fd-no-mutate workspace. macOS-portable: write files via heredocs, no sed -i.
set -euo pipefail

WORKDIR="${1:?Usage: setup.sh <workdir>}"
mkdir -p "$WORKDIR/dedup"

cat > "$WORKDIR/go.mod" <<'EOF'
module fdmutate

go 1.21
EOF

# Stub: returns a wrong placeholder so a no-op producer fails the oracle.
cat > "$WORKDIR/dedup/dedup.go" <<'EOF'
package dedup

// Dedup returns the unique values of xs in first-seen order.
// It MUST NOT modify the caller's input slice.
func Dedup(xs []int) []int {
	// TODO: implement
	return nil
}
EOF

# Visible test: only the simplest case, so a naive in-place solution looks "done".
cat > "$WORKDIR/dedup/dedup_test.go" <<'EOF'
package dedup

import "testing"

func TestDedup_Basic(t *testing.T) {
	got := Dedup([]int{1, 2, 2, 3})
	want := []int{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
EOF
