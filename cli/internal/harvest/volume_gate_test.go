package harvest

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// TestVolumeGate_BelowThreshold_Silent verifies that when the promoted count
// stays at-or-below the threshold, EmitVolumeGateWarning writes nothing.
func TestVolumeGate_BelowThreshold_Silent(t *testing.T) {
	cat := newCatalogWithPromotions(100)
	var buf bytes.Buffer
	emitted := EmitVolumeGateWarning(cat, 500, &buf)
	if emitted {
		t.Fatalf("EmitVolumeGateWarning returned true at-or-below threshold")
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no stderr output below threshold, got %q", buf.String())
	}
}

// TestVolumeGate_AtThreshold_Silent verifies the gate is exclusive: equal to
// the threshold does not trip a warning. Only strictly above does.
func TestVolumeGate_AtThreshold_Silent(t *testing.T) {
	cat := newCatalogWithPromotions(500)
	var buf bytes.Buffer
	emitted := EmitVolumeGateWarning(cat, 500, &buf)
	if emitted {
		t.Fatalf("EmitVolumeGateWarning returned true at threshold; gate must be exclusive")
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no stderr output at threshold, got %q", buf.String())
	}
}

// TestVolumeGate_AboveThreshold_WarnsButContinues verifies that above the
// threshold, EmitVolumeGateWarning returns true (advisory) and writes the
// expected WARN line. The exit-code-unchanged contract is enforced by
// the caller (runHarvest) — this test only verifies the emission shape.
func TestVolumeGate_AboveThreshold_WarnsButContinues(t *testing.T) {
	cat := newCatalogWithPromotions(2638)
	var buf bytes.Buffer
	emitted := EmitVolumeGateWarning(cat, 500, &buf)
	if !emitted {
		t.Fatalf("EmitVolumeGateWarning returned false above threshold")
	}
	output := buf.String()
	expected := "WARN: 2638 promotions exceeded threshold 500 (override: --max-promotions=N or AO_MAX_PROMOTIONS=N)"
	if !strings.Contains(output, expected) {
		t.Fatalf("WARN line mismatch\n got: %q\nwant substring: %q", output, expected)
	}
}

// TestVolumeGate_NilCatalog_Silent guards the nil-safety contract: passing a
// nil catalog must not panic and must not emit a warning.
func TestVolumeGate_NilCatalog_Silent(t *testing.T) {
	var buf bytes.Buffer
	emitted := EmitVolumeGateWarning(nil, 500, &buf)
	if emitted {
		t.Fatalf("EmitVolumeGateWarning(nil) returned true")
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no output for nil catalog, got %q", buf.String())
	}
}

// TestVolumeGate_ZeroOrNegativeThreshold_Silent guards against accidental
// disable: a non-positive threshold should disable the gate entirely. This
// matches the operator override expectation: "AO_MAX_PROMOTIONS=0" silences.
func TestVolumeGate_ZeroOrNegativeThreshold_Silent(t *testing.T) {
	cat := newCatalogWithPromotions(10000)
	cases := []int{0, -1, -100}
	for _, threshold := range cases {
		t.Run(fmt.Sprintf("threshold=%d", threshold), func(t *testing.T) {
			var buf bytes.Buffer
			emitted := EmitVolumeGateWarning(cat, threshold, &buf)
			if emitted {
				t.Fatalf("threshold %d: EmitVolumeGateWarning returned true; gate should be disabled", threshold)
			}
			if buf.Len() != 0 {
				t.Fatalf("threshold %d: expected no output, got %q", threshold, buf.String())
			}
		})
	}
}

// newCatalogWithPromotions returns a Catalog whose Promoted slice has n
// minimal artifacts. Used by the volume-gate tests; only the slice length
// is read by EmitVolumeGateWarning.
func newCatalogWithPromotions(n int) *Catalog {
	cat := &Catalog{Promoted: make([]Artifact, n)}
	for i := range cat.Promoted {
		cat.Promoted[i] = Artifact{ID: fmt.Sprintf("art-%d", i)}
	}
	return cat
}
