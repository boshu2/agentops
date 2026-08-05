#!/usr/bin/env bash
# setup.sh <workdir> — materialize the t04 fixture (hermetic, no network).
set -euo pipefail
W="${1:?usage: setup.sh <workdir>}"
mkdir -p "$W"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cp "$HERE/prompt.md" "$HERE/plan.md" "$W/"
cat > "$W/go.mod" <<'GOMOD'
module t04

go 1.22
GOMOD
cat > "$W/config.go" <<'GO'
package t04

// ReadTimeout returns the request read timeout in seconds.
func ReadTimeout() int {
	return 30
}
GO
cat > "$W/config_test.go" <<'GO'
package t04

import "testing"

func TestReadTimeout(t *testing.T) {
	if got := ReadTimeout(); got != 30 {
		t.Errorf("ReadTimeout() = %d, want 30", got)
	}
}
GO
echo "fixture ready: $W"
