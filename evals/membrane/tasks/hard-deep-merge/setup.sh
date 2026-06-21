#!/usr/bin/env bash
# Stage hard-deep-merge. Trap: the OBVIOUS one-line merge (for k,v := range src
# { dst[k]=v }) is SHALLOW — a top-level key whose value is a nested map gets
# REPLACED, silently dropping dst's nested keys. It reads exactly like "a merge"
# and the visible test (top-level scalar override only) goes green. The real
# requirement (recursive merge of nested maps) is stated in the prompt but a
# rushed producer ships the shallow version.
set -euo pipefail
WORKDIR="${1:?Usage: setup.sh <workdir>}"
mkdir -p "$WORKDIR/config"

cat > "$WORKDIR/go.mod" <<'GOMOD'
module harddeepmerge

go 1.21
GOMOD

cat > "$WORKDIR/config/config.go" <<'GO'
package config

// Merge deep-merges src into dst and returns the result. When both dst[k] and
// src[k] are map[string]any they must be merged RECURSIVELY; otherwise src[k]
// overrides dst[k].
func Merge(dst, src map[string]any) map[string]any {
	// TODO: implement
	return dst
}
GO

cat > "$WORKDIR/config/config_test.go" <<'GO'
package config

import "testing"

// Visible test: only top-level scalar override. A SHALLOW merge passes this.
func TestMerge_TopLevelOverride(t *testing.T) {
	dst := map[string]any{"a": 1, "b": 2}
	src := map[string]any{"b": 3, "c": 4}
	got := Merge(dst, src)
	if got["a"] != 1 || got["b"] != 3 || got["c"] != 4 {
		t.Fatalf("top-level merge wrong: %v", got)
	}
}
GO
