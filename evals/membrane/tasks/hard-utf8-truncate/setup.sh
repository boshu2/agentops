#!/usr/bin/env bash
# Trap: the obvious `return s[:maxBytes]` looks like "truncate to N bytes" and
# passes the visible ASCII test, but it SPLITS a multi-byte rune mid-sequence,
# producing invalid UTF-8. The real requirement (don't split runes) is stated in
# the prompt; a rushed producer ships the byte-slice.
set -euo pipefail
WORKDIR="${1:?Usage: setup.sh <workdir>}"
mkdir -p "$WORKDIR/truncate"
cat > "$WORKDIR/go.mod" <<'GOMOD'
module hardutf8truncate

go 1.21
GOMOD
cat > "$WORKDIR/truncate/truncate.go" <<'GO'
package truncate

// TruncateBytes returns the longest prefix of s that is <= maxBytes bytes and
// still valid UTF-8 (never splitting a multi-byte rune).
func TruncateBytes(s string, maxBytes int) string {
	// TODO: implement
	return s
}
GO
cat > "$WORKDIR/truncate/truncate_test.go" <<'GO'
package truncate

import "testing"

// Visible test: ASCII only — a naive byte-slice passes this.
func TestTruncateBytes_ASCII(t *testing.T) {
	if got := TruncateBytes("hello", 3); got != "hel" {
		t.Fatalf("ascii truncate = %q, want %q", got, "hel")
	}
	if got := TruncateBytes("hi", 10); got != "hi" {
		t.Fatalf("under-limit changed: %q", got)
	}
}
GO
