// Tests for small pure helpers in beads.go that had 0% coverage.
// beadMinInt powers the citation-resolution clipping (matches[:min(3, len)])
// and beadTruncate appears in the parse-error message (so a regression
// would silently truncate at the wrong boundary). Both are utility-level,
// but the call sites are user-visible.

package main

import "testing"

func TestBeadMinInt_FirstSmaller(t *testing.T) {
	if got := beadMinInt(2, 5); got != 2 {
		t.Fatalf("beadMinInt(2,5) = %d, want 2", got)
	}
}

func TestBeadMinInt_SecondSmaller(t *testing.T) {
	if got := beadMinInt(7, 3); got != 3 {
		t.Fatalf("beadMinInt(7,3) = %d, want 3", got)
	}
}

func TestBeadMinInt_Equal(t *testing.T) {
	if got := beadMinInt(4, 4); got != 4 {
		t.Fatalf("beadMinInt(4,4) = %d, want 4 (equal-case must not flip)", got)
	}
}

func TestBeadMinInt_NegativeValues(t *testing.T) {
	if got := beadMinInt(-1, -3); got != -3 {
		t.Fatalf("beadMinInt(-1,-3) = %d, want -3", got)
	}
	if got := beadMinInt(-5, 2); got != -5 {
		t.Fatalf("beadMinInt(-5,2) = %d, want -5", got)
	}
}

func TestBeadMinInt_ZeroAndPositive(t *testing.T) {
	if got := beadMinInt(0, 3); got != 0 {
		t.Fatalf("beadMinInt(0,3) = %d, want 0", got)
	}
}

func TestBeadTruncate_BelowLimit(t *testing.T) {
	if got := beadTruncate("short", 80); got != "short" {
		t.Fatalf("beadTruncate(short, 80) = %q, want %q", got, "short")
	}
}

func TestBeadTruncate_ExactlyAtLimit(t *testing.T) {
	in := "12345"
	if got := beadTruncate(in, 5); got != in {
		t.Fatalf("beadTruncate(len==n) = %q, want %q (must NOT add ellipsis)", got, in)
	}
}

func TestBeadTruncate_AboveLimit(t *testing.T) {
	in := "abcdefghij"
	got := beadTruncate(in, 5)
	want := "abcde..."
	if got != want {
		t.Fatalf("beadTruncate(over) = %q, want %q", got, want)
	}
}

func TestBeadTruncate_EmptyInput(t *testing.T) {
	if got := beadTruncate("", 5); got != "" {
		t.Fatalf("beadTruncate(\"\", 5) = %q, want empty", got)
	}
}

func TestBeadTruncate_ZeroLimitOnNonEmpty(t *testing.T) {
	// When n=0 and input is non-empty, truncation slices to "" and appends
	// the ellipsis sentinel — this is the actual current behavior the
	// parse-error path relies on, so pin it.
	got := beadTruncate("xyz", 0)
	want := "..."
	if got != want {
		t.Fatalf("beadTruncate(xyz, 0) = %q, want %q", got, want)
	}
}
