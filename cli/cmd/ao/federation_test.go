package main

import (
	"errors"
	"io/fs"
	"testing"
)

// probe builds a federatedProbe returning a fixed (found, err).
func probe(found bool, err error) federatedProbe {
	return func() (bool, error) { return found, err }
}

// TestResolveFederated_HardErrorSurfacesOverLaterCleanMiss is invariant (a):
// a hard error on the first probe surfaces even when a later probe clean-misses.
// The failed read must never be flattened into "not found".
func TestResolveFederated_HardErrorSurfacesOverLaterCleanMiss(t *testing.T) {
	hard := errors.New("boom: primary source unreadable")

	found, err := resolveFederated(
		probe(false, hard), // probe 1: hard failure
		probe(false, nil),  // probe 2: clean miss
	)

	if found {
		t.Fatal("found should be false when no probe located the artifact")
	}
	if !errors.Is(err, hard) {
		t.Fatalf("first hard error must surface, got %v", err)
	}
}

// TestResolveFederated_NotFoundOnlyWhenAllCleanMiss is invariant (b):
// (false, nil) is returned ONLY when every probe was a clean miss.
func TestResolveFederated_NotFoundOnlyWhenAllCleanMiss(t *testing.T) {
	found, err := resolveFederated(
		probe(false, nil),
		probe(false, nil),
		probe(false, nil),
	)
	if found {
		t.Fatal("found should be false")
	}
	if err != nil {
		t.Fatalf("all-clean-miss must return nil error (NotFound), got %v", err)
	}
}

// TestResolveFederated_CorruptSourceIsHardError is invariant (c): a corrupt /
// unreadable source (here a permission error) is a hard error, not absence —
// even when it is the LAST probe and every earlier probe clean-missed.
func TestResolveFederated_CorruptSourceIsHardError(t *testing.T) {
	found, err := resolveFederated(
		probe(false, nil),              // clean miss
		probe(false, nil),              // clean miss
		probe(false, fs.ErrPermission), // corrupt/unreadable source
	)
	if found {
		t.Fatal("found should be false")
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("corrupt/unreadable source must surface as a hard error, got %v", err)
	}
}

// TestResolveFederated_FirstHardErrorPreserved verifies that when multiple
// probes hard-fail, the FIRST error is the one preserved (a later hard error
// never overwrites it), mirroring storeref.Resolve's recordErr discipline.
func TestResolveFederated_FirstHardErrorPreserved(t *testing.T) {
	first := errors.New("first hard failure")
	second := errors.New("second hard failure")

	_, err := resolveFederated(
		probe(false, first),
		probe(false, second),
	)
	if !errors.Is(err, first) {
		t.Fatalf("first hard error must be preserved, got %v", err)
	}
	if errors.Is(err, second) {
		t.Fatal("later hard error must not overwrite the first")
	}
}

// TestResolveFederated_FoundShortCircuits verifies a hit short-circuits and
// wins over an earlier recorded hard error: a located artifact is authoritative,
// and probes after the hit are not consulted.
func TestResolveFederated_FoundShortCircuits(t *testing.T) {
	hard := errors.New("earlier source unreadable")
	laterCalled := false

	found, err := resolveFederated(
		probe(false, hard), // hard failure recorded
		probe(true, nil),   // hit — short-circuits, wins over the recorded error
		func() (bool, error) {
			laterCalled = true
			return true, nil
		},
	)
	if !found {
		t.Fatal("found should be true once a probe hits")
	}
	if err != nil {
		t.Fatalf("a clean hit must return nil error even after an earlier hard failure, got %v", err)
	}
	if laterCalled {
		t.Fatal("probes after the hit must not be consulted")
	}
}

// TestResolveFederated_FoundPropagatesEmitError verifies that when the hitting
// probe returns an emit error (e.g. the artifact was found but rendering it
// failed), that error is propagated rather than swallowed.
func TestResolveFederated_FoundPropagatesEmitError(t *testing.T) {
	emitErr := errors.New("render failed")

	found, err := resolveFederated(
		probe(true, emitErr),
	)
	if !found {
		t.Fatal("found should be true")
	}
	if !errors.Is(err, emitErr) {
		t.Fatalf("emit error on a hit must propagate, got %v", err)
	}
}
