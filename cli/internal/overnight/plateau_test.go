package overnight

import (
	"strings"
	"testing"
)

// mustPlateau constructs a PlateauState for tests, failing the test if
// NewPlateauState returns an error.
func mustPlateau(t *testing.T, k int, epsilon float64) *PlateauState {
	t.Helper()
	p, err := NewPlateauState(k, epsilon)
	if err != nil {
		t.Fatalf("NewPlateauState(%d, %g) unexpected error: %v", k, epsilon, err)
	}
	return p
}

func TestPlateauState_NewErrorsOnKLessThan2(t *testing.T) {
	p, err := NewPlateauState(1, 0.01)
	if err == nil {
		t.Fatal("expected error on k=1, got none")
	}
	if p != nil {
		t.Fatalf("expected nil PlateauState on error, got %v", p)
	}
}

func TestPlateauState_ObserveK2_FiresOnConsecutivePair(t *testing.T) {
	p := mustPlateau(t, 2, 0.01)

	if fired := p.Observe(0.005); fired {
		t.Fatal("first sub-epsilon observation must not fire")
	}
	if fired := p.Observe(0.005); !fired {
		t.Fatal("second consecutive sub-epsilon observation must fire")
	}
}

func TestPlateauState_ObserveK2_NoisySampleAbsorbed(t *testing.T) {
	p := mustPlateau(t, 2, 0.01)

	if fired := p.Observe(0.005); fired {
		t.Fatal("observation 1 should not fire")
	}
	if fired := p.Observe(0.1); fired {
		t.Fatal("observation 2 (noise) must not fire")
	}
	if fired := p.Observe(0.005); fired {
		t.Fatal("observation 3 should not fire (window was reset by noise)")
	}
	if fired := p.Observe(0.005); !fired {
		t.Fatal("observation 4 should fire (two fresh sub-epsilon samples)")
	}
}

func TestPlateauState_ObserveK3_FiresOnThreeInARow(t *testing.T) {
	p := mustPlateau(t, 3, 0.01)

	if fired := p.Observe(0.001); fired {
		t.Fatal("observation 1 should not fire")
	}
	if fired := p.Observe(0.002); fired {
		t.Fatal("observation 2 should not fire")
	}
	if fired := p.Observe(0.0005); !fired {
		t.Fatal("observation 3 should fire for k=3")
	}
}

func TestPlateauState_ObserveAfterHaltIsIdempotent(t *testing.T) {
	p := mustPlateau(t, 2, 0.01)
	_ = p.Observe(0.005)
	if fired := p.Observe(0.005); !fired {
		t.Fatal("expected second call to fire")
	}
	for i := 0; i < 3; i++ {
		if fired := p.Observe(0.005); fired {
			t.Errorf("post-halt observation %d returned true, want false", i)
		}
	}
}

func TestPlateauState_ReasonBeforeHaltIsEmpty(t *testing.T) {
	p := mustPlateau(t, 2, 0.01)
	if got := p.Reason(); got != "" {
		t.Errorf("Reason before halt = %q, want empty", got)
	}
	_ = p.Observe(0.005)
	if got := p.Reason(); got != "" {
		t.Errorf("Reason after single sub-epsilon = %q, want empty", got)
	}
}

func TestPlateauState_ReasonAfterHaltDescribesConfig(t *testing.T) {
	p := mustPlateau(t, 2, 0.01)
	_ = p.Observe(0.005)
	_ = p.Observe(0.005)

	reason := p.Reason()
	if reason == "" {
		t.Fatal("expected non-empty reason after halt")
	}
	if !strings.Contains(reason, "2") {
		t.Errorf("Reason %q missing K value", reason)
	}
	if !strings.Contains(reason, "0.01") {
		t.Errorf("Reason %q missing epsilon value", reason)
	}
}

func TestPlateauState_AbsoluteValueUsed(t *testing.T) {
	p := mustPlateau(t, 2, 0.01)

	// Both deltas negative but |delta| < epsilon -> should still fire.
	if fired := p.Observe(-0.005); fired {
		t.Fatal("first negative sub-epsilon should not fire")
	}
	if fired := p.Observe(-0.002); !fired {
		t.Fatal("second negative sub-epsilon should fire")
	}
}

func TestPlateauState_Reset(t *testing.T) {
	p := mustPlateau(t, 2, 0.01)
	_ = p.Observe(0.005)
	_ = p.Observe(0.005)
	if p.Reason() == "" {
		t.Fatal("sanity: expected halted state before Reset")
	}

	p.Reset()

	if got := p.Reason(); got != "" {
		t.Errorf("Reason after Reset = %q, want empty", got)
	}
	// After reset, a single sub-epsilon must not fire.
	if fired := p.Observe(0.005); fired {
		t.Error("post-reset first observation fired unexpectedly")
	}
	if fired := p.Observe(0.005); !fired {
		t.Error("post-reset second observation should fire normally")
	}
}
