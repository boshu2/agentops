package evalsubstrate

import (
	"strings"
	"testing"
)

func TestRefusal_Format_MatchesSpec(t *testing.T) {
	r := Refusal{
		GateNumber: 4,
		GateName:   "model_spec_drift",
		Why:        "Run's ModelSpec content_hash differs from baseline run's; comparison would conflate model and endpoint changes.",
		Evidence:   "sampling_defaults.temperature: 0.0 -> 0.2  (server: mlx_lm.server 0.31.3 -> 0.31.5)",
		Fix:        "Either revert MLX server config (see ~/.finance/data/mlx_server.log for restart), OR re-run with --cross-spec to record a deliberate cross-endpoint comparison.",
	}
	got := r.Format()

	wantLines := []string{
		"GATE FAILED: 4 model_spec_drift",
		"  Why:      Run's ModelSpec content_hash differs from baseline run's; comparison would conflate model and endpoint changes.",
		"  Evidence: sampling_defaults.temperature: 0.0 -> 0.2  (server: mlx_lm.server 0.31.3 -> 0.31.5)",
		"  Fix:      Either revert MLX server config (see ~/.finance/data/mlx_server.log for restart), OR re-run with --cross-spec to record a deliberate cross-endpoint comparison.",
	}
	gotLines := strings.Split(got, "\n")
	if len(gotLines) != len(wantLines) {
		t.Fatalf("line count mismatch: got %d, want %d\n%s", len(gotLines), len(wantLines), got)
	}
	for i := range wantLines {
		if gotLines[i] != wantLines[i] {
			t.Errorf("line %d:\n  got:  %q\n  want: %q", i, gotLines[i], wantLines[i])
		}
	}
}

func TestRefusal_StartsWithGateFAILED(t *testing.T) {
	r := Refusal{GateNumber: 1, GateName: "no_held_constant", Why: "x", Evidence: "y", Fix: "z"}
	if !strings.HasPrefix(r.Format(), "GATE FAILED: 1 no_held_constant\n") {
		t.Fatalf("bad prefix: %q", r.Format())
	}
}

func TestRefusal_FieldOrder_WhyEvidenceFix(t *testing.T) {
	r := Refusal{GateNumber: 7, GateName: "gt_superseded", Why: "w", Evidence: "e", Fix: "f"}
	got := r.Format()
	whyIdx := strings.Index(got, "Why:")
	evIdx := strings.Index(got, "Evidence:")
	fixIdx := strings.Index(got, "Fix:")
	if !(whyIdx < evIdx && evIdx < fixIdx) {
		t.Fatalf("field order should be Why<Evidence<Fix; got: %s", got)
	}
}

func TestRefusal_AsError(t *testing.T) {
	var err error = Refusal{GateNumber: 6, GateName: "underpowered", Why: "x", Evidence: "y", Fix: "z"}
	if !strings.Contains(err.Error(), "GATE FAILED: 6 underpowered") {
		t.Fatalf("error format wrong: %s", err.Error())
	}
}

func TestRefusals_FormatJoinsWithBlankLine(t *testing.T) {
	rs := Refusals{
		{GateNumber: 1, GateName: "a", Why: "1", Evidence: "1", Fix: "1"},
		{GateNumber: 2, GateName: "b", Why: "2", Evidence: "2", Fix: "2"},
	}
	got := rs.Format()
	if !strings.Contains(got, "GATE FAILED: 1 a") || !strings.Contains(got, "GATE FAILED: 2 b") {
		t.Fatalf("missing one of the refusals: %s", got)
	}
	if !strings.Contains(got, "\n\nGATE FAILED: 2") {
		t.Fatalf("refusals not joined with blank line: %q", got)
	}
}

func TestRefusals_EmptyMeansPassed(t *testing.T) {
	if !(Refusals{}).Empty() {
		t.Fatal("zero refusals should be Empty()")
	}
	rs := Refusals{{GateNumber: 1}}
	if rs.Empty() {
		t.Fatal("non-empty refusals reported empty")
	}
}
