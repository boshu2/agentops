package truncate

import "unicode/utf8"

// TruncateBytes returns the longest prefix of s that is <= maxBytes bytes and
// still valid UTF-8 (never splitting a multi-byte rune).
//
// FROZEN CONTROL SOLUTION (a genuine TRUE-DONE for the calibration corpus).
// This is correct: it backs up off any continuation byte to the nearest rune
// boundary, so "aé" truncated to 2 bytes yields "a" and a 4-byte rune truncated
// to 3 bytes yields "". It exists (alongside cleaner-median) so the false-refute
// rate has a denominator > 1 — a second correct solution the membrane must ACK.
func TruncateBytes(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	if maxBytes <= 0 {
		return ""
	}
	b := maxBytes
	for b > 0 && !utf8.RuneStart(s[b]) {
		b--
	}
	return s[:b]
}
