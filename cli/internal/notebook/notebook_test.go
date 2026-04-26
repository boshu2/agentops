package notebook

import (
	"testing"
	"unicode/utf8"
)

func TestTruncate_RuneSafe(t *testing.T) {
	got := Truncate("aébbbb", 5)
	if got != "aé..." {
		t.Fatalf("Truncate unicode boundary = %q, want %q", got, "aé...")
	}
	if !utf8.ValidString(got) {
		t.Fatalf("Truncate returned invalid UTF-8: %q", got)
	}
}
