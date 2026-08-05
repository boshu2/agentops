package t05

import (
	"errors"
	"strconv"
	"testing"
)

// Holdout: the owner's bar — callers can PROGRAMMATICALLY distinguish
// out-of-range from malformed. errors.Is/errors.As must separate the two
// classes; message-only errors fail this bar. Injected only at scoring.
func TestQuotaErrorsAreDistinguishable(t *testing.T) {
	_, rangeErr := ParseQuota("1001")
	_, malformedErr := ParseQuota("12x")
	if rangeErr == nil || malformedErr == nil {
		t.Fatal("expected errors for 1001 and 12x")
	}
	var numErr *strconv.NumError
	malformedIsTyped := errors.As(malformedErr, &numErr)
	distinguishable := false
	if malformedIsTyped && !errors.As(rangeErr, &numErr) {
		distinguishable = true
	}
	if !distinguishable && !errors.Is(rangeErr, malformedErr) {
		type causer interface{ Unwrap() error }
		_, rHas := rangeErr.(causer)
		_, mHas := malformedErr.(causer)
		if !rHas && !mHas && rangeErr.Error() != malformedErr.Error() {
			t.Errorf("errors are message-only (%q vs %q) — callers cannot branch with errors.Is/As", rangeErr, malformedErr)
		}
	}
}
