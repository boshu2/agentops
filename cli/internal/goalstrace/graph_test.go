package goalstrace

import "testing"

func TestGraph_Summarize_CountsDefectsAndLowConfidence(t *testing.T) {
	g := Graph{
		Edges: []Edge{
			{Type: EdgeDirectiveHasScenario, Confidence: ConfidenceHigh},
			{
				Type:       EdgeDirectiveHasScenario,
				Confidence: ConfidenceLow,
				Defects: []Defect{
					{Code: DefectBrokenScenarioRef, Severity: SeverityError},
				},
			},
			{
				Type:       EdgeScenarioClaimedByBead,
				Confidence: ConfidenceLow,
				Defects: []Defect{
					{Code: DefectScenarioNoBeadClaim, Severity: SeverityWarning},
					{Code: DefectScenarioNoResult, Severity: SeverityWarning},
				},
			},
		},
	}
	s := g.Summarize()
	if !s.Summary {
		t.Error("Summary flag must be true")
	}
	if s.ErrorCount != 1 {
		t.Errorf("ErrorCount = %d, want 1", s.ErrorCount)
	}
	if s.WarningCount != 2 {
		t.Errorf("WarningCount = %d, want 2", s.WarningCount)
	}
	if s.LowConfidenceEdges != 2 {
		t.Errorf("LowConfidenceEdges = %d, want 2", s.LowConfidenceEdges)
	}
}

func TestGraph_Sort_IsDeterministic(t *testing.T) {
	g := Graph{
		Nodes: []Node{
			{ID: "z", Type: NodeScenario},
			{ID: "a", Type: NodeScenario},
			{ID: "m", Type: NodeDirective},
		},
		Edges: []Edge{
			{Type: EdgeScenarioResult, FromID: "b", ToID: "y"},
			{Type: EdgeDirectiveHasScenario, FromID: "d", ToID: "s"},
			{Type: EdgeDirectiveHasScenario, FromID: "a", ToID: "s"},
		},
	}
	g.Sort()
	// Nodes: directive type sorts before scenario type.
	if g.Nodes[0].Type != NodeDirective {
		t.Errorf("first node type = %q, want directive", g.Nodes[0].Type)
	}
	if g.Nodes[1].ID != "a" || g.Nodes[2].ID != "z" {
		t.Errorf("scenario nodes not ID-sorted: %q, %q", g.Nodes[1].ID, g.Nodes[2].ID)
	}
	// Edges: directive_has_scenario sorts before scenario_result.
	if g.Edges[0].Type != EdgeDirectiveHasScenario {
		t.Errorf("first edge type = %q, want directive_has_scenario", g.Edges[0].Type)
	}
	if g.Edges[0].FromID != "a" || g.Edges[1].FromID != "d" {
		t.Errorf("directive_has_scenario edges not from-sorted: %q, %q",
			g.Edges[0].FromID, g.Edges[1].FromID)
	}
}

func TestClaimedScenarios_ExplicitVsHeuristic(t *testing.T) {
	cases := []struct {
		name      string
		bead      beadRecord
		explicit  []string
		heuristic []string
	}{
		{
			name: "explicit Scenarios line",
			bead: beadRecord{
				Description: "Do work.\nScenarios: s-2026-05-17-001, s-2026-05-17-002",
			},
			explicit:  []string{"s-2026-05-17-001", "s-2026-05-17-002"},
			heuristic: nil,
		},
		{
			name: "free-text mention only is heuristic",
			bead: beadRecord{
				Description: "This relates to s-2026-05-17-009 somehow.",
			},
			explicit:  nil,
			heuristic: []string{"s-2026-05-17-009"},
		},
		{
			name: "explicit claim suppresses its heuristic duplicate",
			bead: beadRecord{
				Description: "Scenarios: s-2026-05-17-001",
				Notes:       "again s-2026-05-17-001 mentioned",
			},
			explicit:  []string{"s-2026-05-17-001"},
			heuristic: nil,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotExp, gotHeu := claimedScenarios(c.bead)
			assertIDs(t, "explicit", gotExp, c.explicit)
			assertIDs(t, "heuristic", gotHeu, c.heuristic)
		})
	}
}

// assertIDs compares two string slices for exact equality.
func assertIDs(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: len = %d, want %d (got=%v)", label, len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s[%d] = %q, want %q", label, i, got[i], want[i])
		}
	}
}

func TestParseBeadList_AcceptsArrayAndEnvelope(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  int
	}{
		{"bare array", `[{"id":"a-1"},{"id":"a-2"}]`, 2},
		{"issues envelope", `{"issues":[{"id":"a-1"}]}`, 1},
		{"beads envelope", `{"beads":[{"id":"a-1"},{"id":"a-2"}]}`, 2},
		{"empty input", ``, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseBeadList([]byte(c.input))
			if err != nil {
				t.Fatalf("parseBeadList error: %v", err)
			}
			if len(got) != c.want {
				t.Errorf("got %d beads, want %d", len(got), c.want)
			}
		})
	}
}
