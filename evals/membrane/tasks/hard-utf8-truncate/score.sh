#!/usr/bin/env bash
# Oracle: truncating a multi-byte string at a boundary that lands mid-rune must
# NOT split the rune — the result must be valid UTF-8 and the longest such prefix.
set -euo pipefail
WORKDIR="${1:?Usage: score.sh <workdir>}"
cd "$WORKDIR"
ORACLE="truncate/oracle_zz_test.go"
cat > "$ORACLE" <<'GO'
package truncate

import (
	"testing"
	"unicode/utf8"
)

func TestOracle_NoSplitRune(t *testing.T) {
	// "é" is 2 bytes (0xC3 0xA9). "aé" = 3 bytes. Truncate to 2 bytes: the only
	// valid prefix is "a" (1 byte) — taking 2 bytes splits é.
	got := TruncateBytes("aé", 2)
	if !utf8.ValidString(got) {
		t.Fatalf("result is not valid UTF-8: %q (split a rune)", got)
	}
	if got != "a" {
		t.Fatalf("TruncateBytes(\"aé\", 2) = %q, want \"a\"", got)
	}
	// A 4-byte rune (😀 = 0xF0 0x9F 0x98 0x80) truncated to 3 bytes => "" (can't fit).
	if got := TruncateBytes("😀", 3); got != "" || !utf8.ValidString(got) {
		t.Fatalf("TruncateBytes(emoji,3) = %q, want \"\"", got)
	}
	// Under-limit unchanged.
	if got := TruncateBytes("aé", 10); got != "aé" {
		t.Fatalf("under-limit changed: %q", got)
	}
}
GO
score=0; total=1
if go test -run 'TestOracle_NoSplitRune' ./truncate/ >/dev/null 2>&1; then score=1; fi
rm -f "$ORACLE"
pass=false; [ "$score" -eq "$total" ] && pass=true
echo "{\"score\": $score, \"total\": $total, \"pass\": $pass}"
