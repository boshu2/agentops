package evalsubstrate

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestManifest_VerdictRoundTrip — rc3 struct form round-trips byte-stable.
// All seven fields preserved; floats exact; array order preserved; omitempty honored.
func TestManifest_VerdictRoundTrip(t *testing.T) {
	want := Manifest{
		ID:     "run-1",
		Status: StatusComplete,
		Verdict: &Verdict{
			Kind:                VerdictImproved,
			DeltaPoint:          0.034444444444444444,
			CILow:               0.020000000000000004,
			CIHigh:              0.05333333333333334,
			Utility:             0.85,
			ApplicableArtifacts: []string{".agents/learnings/foo.md", ".agents/learnings/bar.md"},
			Notes:               "n=10",
		},
	}
	bs, err := json.Marshal(&want)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Manifest
	if err := json.Unmarshal(bs, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Verdict == nil {
		t.Fatal("verdict was lost in round-trip")
	}
	if got.Verdict.Kind != want.Verdict.Kind {
		t.Errorf("kind: got %q, want %q", got.Verdict.Kind, want.Verdict.Kind)
	}
	if got.Verdict.DeltaPoint != want.Verdict.DeltaPoint {
		t.Errorf("delta_point: got %.18f, want %.18f", got.Verdict.DeltaPoint, want.Verdict.DeltaPoint)
	}
	if got.Verdict.CILow != want.Verdict.CILow {
		t.Errorf("ci_low: got %.18f, want %.18f", got.Verdict.CILow, want.Verdict.CILow)
	}
	if got.Verdict.CIHigh != want.Verdict.CIHigh {
		t.Errorf("ci_high: got %.18f, want %.18f", got.Verdict.CIHigh, want.Verdict.CIHigh)
	}
	if got.Verdict.Utility != want.Verdict.Utility {
		t.Errorf("utility: got %f, want %f", got.Verdict.Utility, want.Verdict.Utility)
	}
	if len(got.Verdict.ApplicableArtifacts) != len(want.Verdict.ApplicableArtifacts) {
		t.Fatalf("applicable_artifacts length: got %d, want %d",
			len(got.Verdict.ApplicableArtifacts), len(want.Verdict.ApplicableArtifacts))
	}
	for i, a := range want.Verdict.ApplicableArtifacts {
		if got.Verdict.ApplicableArtifacts[i] != a {
			t.Errorf("applicable_artifacts[%d]: got %q, want %q",
				i, got.Verdict.ApplicableArtifacts[i], a)
		}
	}
	if got.Verdict.Notes != want.Verdict.Notes {
		t.Errorf("notes: got %q, want %q", got.Verdict.Notes, want.Verdict.Notes)
	}
}

// TestManifest_VerdictLegacyStringCompat — pre-mortem C1 fix.
// rc2 manifests stored verdict as a bare string ("improved"). New *Verdict struct
// MUST accept that legacy form via custom UnmarshalJSON; otherwise existing
// runs would fail to load.
func TestManifest_VerdictLegacyStringCompat(t *testing.T) {
	raw := `{"id":"run-legacy","status":"complete","verdict":"improved","quick_session":false}`
	var m Manifest
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("unmarshal legacy string verdict: %v", err)
	}
	if m.Verdict == nil {
		t.Fatal("legacy string verdict produced nil")
	}
	if m.Verdict.Kind != VerdictImproved {
		t.Errorf("legacy verdict.kind: got %q, want %q", m.Verdict.Kind, VerdictImproved)
	}
	// Other fields zero-valued.
	if m.Verdict.DeltaPoint != 0 || m.Verdict.CILow != 0 || m.Verdict.CIHigh != 0 ||
		m.Verdict.Utility != 0 || m.Verdict.Notes != "" ||
		len(m.Verdict.ApplicableArtifacts) != 0 {
		t.Errorf("legacy verdict should leave non-Kind fields zero, got: %+v", m.Verdict)
	}
}

// TestManifest_VerdictNullHandling — explicit `null` -> nil pointer.
func TestManifest_VerdictNullHandling(t *testing.T) {
	raw := `{"id":"run-null","status":"running","verdict":null,"quick_session":false}`
	var m Manifest
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("unmarshal null verdict: %v", err)
	}
	if m.Verdict != nil {
		t.Errorf("null verdict should produce nil pointer, got %+v", m.Verdict)
	}
}

// TestManifest_VerdictMissingHandling — verdict field omitted entirely.
func TestManifest_VerdictMissingHandling(t *testing.T) {
	raw := `{"id":"run-missing","status":"running","quick_session":false}`
	var m Manifest
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("unmarshal missing verdict: %v", err)
	}
	if m.Verdict != nil {
		t.Errorf("missing verdict should produce nil pointer, got %+v", m.Verdict)
	}
}

// TestVerdict_OmitemptyOnMarshal — zero-valued sub-fields don't pollute JSON.
func TestVerdict_OmitemptyOnMarshal(t *testing.T) {
	v := &Verdict{Kind: VerdictUnderpowered}
	bs, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	got := string(bs)
	if !strings.Contains(got, `"kind":"underpowered"`) {
		t.Errorf("missing kind: %s", got)
	}
	for _, key := range []string{"delta_point", "ci_low", "ci_high", "utility", "applicable_artifacts", "notes"} {
		if strings.Contains(got, key) {
			t.Errorf("omitempty failed: zero-valued %q in output: %s", key, got)
		}
	}
}

// TestVerdict_KindEnumExhaustive — all 6 verdict kinds round-trip cleanly.
func TestVerdict_KindEnumExhaustive(t *testing.T) {
	kinds := []VerdictKind{
		VerdictImproved, VerdictRegressed, VerdictNoChange,
		VerdictUnderpowered, VerdictInconclusiveHighVariance, VerdictInconclusiveDegenerate,
	}
	for _, k := range kinds {
		v := &Verdict{Kind: k}
		bs, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("%s marshal: %v", k, err)
		}
		var back Verdict
		if err := json.Unmarshal(bs, &back); err != nil {
			t.Fatalf("%s unmarshal: %v", k, err)
		}
		if back.Kind != k {
			t.Errorf("%s round-trip lost: got %q", k, back.Kind)
		}
	}
}
