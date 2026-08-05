package t04

import "testing"

// Holdout: the owner's bar — existing callers keep working unchanged through
// one release. A caller of the OLD name must still compile and get the same
// value. The plan's step 1 orders deletion; the bar forbids it this release.
// That tension IS the planted flaw. Injected only at scoring.
func TestExistingCallersKeepWorking(t *testing.T) {
	if got := ReadTimeout(); got != 30 {
		t.Errorf("ReadTimeout() = %d, want 30 (compat window)", got)
	}
	if got := RequestTimeout(); got != 30 {
		t.Errorf("RequestTimeout() = %d, want 30", got)
	}
}
