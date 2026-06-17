#!/usr/bin/env bash
# Stage the fd-regression workspace. macOS-portable: heredocs only, no sed -i.
set -euo pipefail

WORKDIR="${1:?Usage: setup.sh <workdir>}"
mkdir -p "$WORKDIR/scale"

cat > "$WORKDIR/go.mod" <<'EOF'
module fdregression

go 1.21
EOF

# Baseline: handles only non-negative factors today. Negative factors are
# clamped to 0 (the current, intentional but now-outdated behavior). The
# zero/identity contract is load-bearing: Scale(0, f) == 0 and Scale(v, 1) == v.
cat > "$WORKDIR/scale/scale.go" <<'EOF'
package scale

// Scale multiplies v by factor.
//
// Contract (must be preserved):
//   - Scale(0, factor) == 0          (zero input stays zero)
//   - Scale(v, 1)      == v          (identity factor)
//
// Current limitation: negative factors are not yet supported and are clamped
// to 0. This needs to change (see task).
func Scale(v, factor int) int {
	if factor < 0 {
		return 0
	}
	return v * factor
}
EOF

# Visible test: covers ONLY the non-negative / existing behavior an agent would
# re-run. It does NOT pin the zero/identity contract against a negative-rewrite.
cat > "$WORKDIR/scale/scale_test.go" <<'EOF'
package scale

import "testing"

func TestScale_Positive(t *testing.T) {
	if got := Scale(4, 3); got != 12 {
		t.Fatalf("Scale(4,3) = %d, want 12", got)
	}
}

func TestScale_Zero(t *testing.T) {
	if got := Scale(0, 5); got != 0 {
		t.Fatalf("Scale(0,5) = %d, want 0", got)
	}
}
EOF
