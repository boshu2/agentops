package probeheadroom

import (
	"path/filepath"
	"testing"
)

// fixtureRoot resolves the committed gate fixtures. The package lives at
// cli/internal/probeheadroom, so the repo root is three directories up.
func fixtureRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "tests", "fixtures", "skill-probes"))
	if err != nil {
		t.Fatalf("resolve fixture root: %v", err)
	}
	return root
}

// loadFixtureGroup reads one committed fixture directory as a single probe
// group. It deliberately round-trips the REAL persisted bytes through the
// production parser rather than hand-building Scorecard values, so a fixture
// shape the harness could never emit cannot give a false green.
func loadFixtureGroup(t *testing.T, name string) []Scorecard {
	t.Helper()
	dir := filepath.Join(fixtureRoot(t), name)
	groups, probes, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("load fixture dir %s: %v", dir, err)
	}
	if len(probes) != 1 {
		t.Fatalf("fixture dir %s: got %d probe groups %v, want exactly 1", dir, len(probes), probes)
	}
	cards := groups[probes[0]]
	if len(cards) != 2 {
		t.Fatalf("fixture dir %s: got %d scorecards, want the committed pair (2)", dir, len(cards))
	}
	return cards
}

// TestClassify_SeparatesSaturatedFromHeadroom is the RED fixture flip: both
// committed pairs carry the SAME probe verdict (INERT), so anything that reads
// only the verdict cannot tell them apart. The headroom rule must.
func TestClassify_SeparatesSaturatedFromHeadroom(t *testing.T) {
	saturated := loadFixtureGroup(t, "saturated")
	headroom := loadFixtureGroup(t, "headroom")

	for _, card := range append(append([]Scorecard{}, saturated...), headroom...) {
		if card.Verdict != "INERT" {
			t.Fatalf("fixture %s: verdict %q, want INERT (the pairs must be indistinguishable by verdict)", card.Path, card.Verdict)
		}
	}

	gotSaturated, err := Classify(saturated)
	if err != nil {
		t.Fatalf("classify saturated fixture pair: %v", err)
	}
	if gotSaturated.Class != Saturated {
		t.Errorf("saturated fixture pair: got %q, want %q", gotSaturated.Class, Saturated)
	}
	if gotSaturated.Class.HasHeadroom() {
		t.Error("saturated fixture pair: HasHeadroom() = true, want false (the row is void and must be flagged)")
	}
	if gotSaturated.Class.ExitCode() != 3 {
		t.Errorf("saturated fixture pair: exit code %d, want 3", gotSaturated.Class.ExitCode())
	}
	if len(gotSaturated.AcedEfforts) != 2 {
		t.Errorf("saturated fixture pair: aced efforts %v, want 2 distinct levels", gotSaturated.AcedEfforts)
	}

	gotHeadroom, err := Classify(headroom)
	if err != nil {
		t.Fatalf("classify headroom fixture pair: %v", err)
	}
	if gotHeadroom.Class != Separated {
		t.Errorf("headroom fixture pair: got %q, want %q", gotHeadroom.Class, Separated)
	}
	if !gotHeadroom.Class.HasHeadroom() {
		t.Error("headroom fixture pair: HasHeadroom() = false, want true (an INERT with headroom is a real null, not a void row)")
	}
	if gotHeadroom.Class.ExitCode() != 0 {
		t.Errorf("headroom fixture pair: exit code %d, want 0", gotHeadroom.Class.ExitCode())
	}
}

func rate(v float64) *float64 { return &v }

func card(probe, effort string, control, treatment Arm) Scorecard {
	return Scorecard{
		Schema:    "agentops-skill-probe.v1",
		Probe:     probe,
		Skill:     "validate",
		Producer:  Producer{Model: "gpt-5.6-luna", Effort: effort},
		Control:   control,
		Treatment: treatment,
		Verdict:   "INERT",
	}
}

func TestClassify_RuleEdges(t *testing.T) {
	tests := []struct {
		name  string
		cards []Scorecard
		want  Classification
	}{
		{
			name: "aced at one effort level only is not saturated",
			cards: []Scorecard{
				card("p", "low", Arm{Present: 2, Usable: 2, Rate: rate(1.0)}, Arm{Present: 2, Usable: 2, Rate: rate(1.0)}),
				card("p", "xhigh", Arm{Present: 1, Usable: 2, Rate: rate(0.5)}, Arm{Present: 2, Usable: 2, Rate: rate(1.0)}),
			},
			want: Separated,
		},
		{
			name: "exactly at the ceiling counts as aced",
			cards: []Scorecard{
				card("p", "low", Arm{Present: 3, Usable: 4, Rate: rate(ControlCeiling)}, Arm{Present: 3, Usable: 4, Rate: rate(0.75)}),
				card("p", "xhigh", Arm{Present: 3, Usable: 4, Rate: rate(ControlCeiling)}, Arm{Present: 3, Usable: 4, Rate: rate(0.75)}),
			},
			want: Saturated,
		},
		{
			name: "one usable control rep never counts as an effort level",
			cards: []Scorecard{
				card("p", "low", Arm{Present: 1, Usable: 1, Rate: rate(1.0)}, Arm{Present: 1, Usable: 2, Rate: rate(0.5)}),
				card("p", "xhigh", Arm{Present: 1, Usable: 1, Rate: rate(1.0)}, Arm{Present: 1, Usable: 2, Rate: rate(0.5)}),
			},
			want: Separated,
		},
		{
			name: "two unlabelled cards are one unknown level, not two",
			cards: []Scorecard{
				card("p", "", Arm{Present: 2, Usable: 2, Rate: rate(1.0)}, Arm{Present: 2, Usable: 2, Rate: rate(1.0)}),
				card("p", "", Arm{Present: 2, Usable: 2, Rate: rate(1.0)}, Arm{Present: 2, Usable: 2, Rate: rate(1.0)}),
			},
			// One deduped unknown level with an aced control: since the 2026-08-26
			// single-level rule this is UNMEASURED (was SEPARATED) — the dedup point
			// this case pins is unchanged: one level, never two.
			want: Unmeasured,
		},
		{
			name: "treatment never acts at any level is FLOOR",
			cards: []Scorecard{
				card("p", "low", Arm{Present: 0, Usable: 2, Rate: rate(0.0)}, Arm{Present: 0, Usable: 2, Rate: rate(0.0)}),
				card("p", "xhigh", Arm{Present: 0, Usable: 2, Rate: rate(0.0)}, Arm{Present: 0, Usable: 2, Rate: rate(0.0)}),
			},
			want: Floor,
		},
		{
			name: "no usable treatment reps is UNMEASURED, never INERT",
			cards: []Scorecard{
				card("p", "low", Arm{Present: 2, Usable: 2, Rate: rate(1.0)}, Arm{Present: 0, Usable: 0, Rate: nil}),
				card("p", "xhigh", Arm{Present: 2, Usable: 2, Rate: rate(1.0)}, Arm{Present: 0, Usable: 0, Rate: nil}),
			},
			want: Unmeasured,
		},
		{
			name: "UNMEASURED outranks a saturated-looking control arm",
			cards: []Scorecard{
				card("p", "low", Arm{Present: 2, Usable: 2, Rate: rate(1.0)}, Arm{Present: 0, Usable: 0, Rate: nil}),
			},
			want: Unmeasured,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Classify(tc.cards)
			if err != nil {
				t.Fatalf("Classify: %v", err)
			}
			if got.Class != tc.want {
				t.Errorf("Classify = %q, want %q (detail: %s)", got.Class, tc.want, got.Detail)
			}
		})
	}
}

func TestClassify_RejectsMixedProbesAndEmptyGroups(t *testing.T) {
	if _, err := Classify(nil); err == nil {
		t.Error("Classify(nil) = nil error, want an error (an empty group must not classify as SEPARATED)")
	}

	mixed := []Scorecard{
		card("probe-a", "low", Arm{Present: 2, Usable: 2, Rate: rate(1.0)}, Arm{Present: 2, Usable: 2, Rate: rate(1.0)}),
		card("probe-b", "xhigh", Arm{Present: 2, Usable: 2, Rate: rate(1.0)}, Arm{Present: 2, Usable: 2, Rate: rate(1.0)}),
	}
	if _, err := Classify(mixed); err == nil {
		t.Error("Classify(mixed probes) = nil error, want an error (two scenarios cannot share one headroom verdict)")
	}
}

func TestParseScorecard_RejectsForeignSchema(t *testing.T) {
	if _, err := ParseScorecard([]byte(`{"schema":"verdict.v2","probe":"p"}`), "x.json"); err == nil {
		t.Error("ParseScorecard(verdict.v2) = nil error, want a schema rejection")
	}
	if _, err := ParseScorecard([]byte(`{"schema":"agentops-skill-probe.v3"}`), "x.json"); err == nil {
		t.Error("ParseScorecard(no probe id) = nil error, want an empty-probe rejection")
	}
	got, err := ParseScorecard([]byte(`{"schema":"agentops-skill-probe.v3","probe":"p","skill":"validate"}`), "x.json")
	if err != nil {
		t.Fatalf("ParseScorecard(v3): %v", err)
	}
	if got.Probe != "p" || got.Skill != "validate" || got.Path != "x.json" {
		t.Errorf("ParseScorecard(v3) = %+v, want probe=p skill=validate path=x.json", got)
	}
}

func TestEffortLabel_DefaultsToUnknown(t *testing.T) {
	if got := (Scorecard{}).EffortLabel(); got != "?" {
		t.Errorf("EffortLabel() with no producer = %q, want %q", got, "?")
	}
	if got := (Scorecard{Producer: Producer{Effort: " "}}).EffortLabel(); got != "?" {
		t.Errorf("EffortLabel() with blank producer effort = %q, want %q", got, "?")
	}
	if got := (Scorecard{Producer: Producer{Effort: "xhigh"}}).EffortLabel(); got != "xhigh" {
		t.Errorf("EffortLabel() = %q, want %q", got, "xhigh")
	}
}

// TestClassify_SingleLevelAcedControlIsNotSeparated pins the 2026-08-26 L2
// finding: a group measured at ONE effort level whose control arm aced it
// must not be labeled SEPARATED — the label was an artifact of the missing
// second level, and a verdict row filed over it would be the void row the
// pre-screen exists to keep out. Such a group is UNMEASURED-for-headroom
// until a second level exists.
func TestClassify_SingleLevelAcedControlIsNotSeparated(t *testing.T) {
	cards := []Scorecard{{
		Probe: "council-caller-challenge-t2", Skill: "council",
		Producer:  Producer{Effort: "low"},
		Control:   Arm{Present: 2, Usable: 2, Rate: rate(1.0)},
		Treatment: Arm{Present: 2, Usable: 2, Rate: rate(1.0)},
	}}
	res, err := Classify(cards)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if res.Class == Separated {
		t.Fatalf("single-level aced-control group classified SEPARATED; want a non-passing class (got %v, detail %q)", res.Class, res.Detail)
	}
	if res.Class != Unmeasured {
		t.Fatalf("single-level aced-control group: got %v, want Unmeasured", res.Class)
	}
}
