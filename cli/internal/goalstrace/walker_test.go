package goalstrace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// fixtureRoot returns the absolute path of the goals-trace fixture tree.
// The test runs with CWD = cli/internal/goalstrace, so the repo root is three
// levels up.
func fixtureRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "tests", "fixtures", "goals-trace"))
	if err != nil {
		t.Fatalf("resolving fixture root: %v", err)
	}
	return root
}

// fixtureBeads returns the bead records the fixture walker should see: one
// bead with an explicit Scenarios: claim, one with only a free-text mention.
func fixtureBeads() []beadRecord {
	return []beadRecord{
		{
			ID:          "soc-58nt.2.6",
			Title:       "F2.0 scenario-results producer",
			Description: "Build the scenario-results artifact.\nScenarios: s-2026-05-17-001",
			Status:      "closed",
		},
		{
			ID:          "soc-58nt.4.1",
			Title:       "F4.1 trace walker",
			Description: "Walk the chain. Mentions s-2026-05-17-002 in passing prose.",
			Status:      "open",
		},
	}
}

// findEdge returns the first edge matching type/from/to, or false.
func findEdge(g Graph, et EdgeType, from, to string) (Edge, bool) {
	for _, e := range g.Edges {
		if e.Type == et && e.FromID == from && e.ToID == to {
			return e, true
		}
	}
	return Edge{}, false
}

// hasNode reports whether the graph contains a node with the given ID and type.
func hasNode(g Graph, id string, nt NodeType) bool {
	for _, n := range g.Nodes {
		if n.ID == id && n.Type == nt {
			return true
		}
	}
	return false
}

func TestWalk_ConnectedGraphForFixtureDirective(t *testing.T) {
	g, err := Walk(Options{
		ProjectRoot: fixtureRoot(t),
		Beads:       NewStaticBeadQuerier(true, fixtureBeads()),
	})
	if err != nil {
		t.Fatalf("Walk returned error: %v", err)
	}

	// The directive node must be present.
	if !hasNode(g, "d-fitness-gate-bdd", NodeDirective) {
		t.Fatalf("missing directive node d-fitness-gate-bdd; nodes=%+v", g.Nodes)
	}
	// directive -> scenario.
	if !hasNode(g, "s-2026-05-17-001", NodeScenario) {
		t.Fatalf("missing scenario node s-2026-05-17-001")
	}
	// scenario -> result artifact.
	if !hasNode(g, "result:s-2026-05-17-001", NodeArtifact) {
		t.Fatalf("missing scenario_result artifact node")
	}
	// scenario -> bead.
	if !hasNode(g, "soc-58nt.2.6", NodeBead) {
		t.Fatalf("missing bead node soc-58nt.2.6")
	}
	// directive -> learning.
	learningRel := filepath.ToSlash(filepath.Join("docs", "learnings", "2026-05-17-trace-chain.md"))
	if !hasNode(g, learningRel, NodeLearning) {
		t.Fatalf("missing learning node %s; nodes=%+v", learningRel, g.Nodes)
	}

	// The chain is connected: every edge type that should exist for the
	// fixture directive is present.
	chain := []struct {
		et       EdgeType
		from, to string
	}{
		{EdgeDirectiveHasScenario, "d-fitness-gate-bdd", "s-2026-05-17-001"},
		{EdgeScenarioResult, "s-2026-05-17-001", "result:s-2026-05-17-001"},
		{EdgeScenarioClaimedByBead, "s-2026-05-17-001", "soc-58nt.2.6"},
		{EdgeDirectiveHasLearning, "d-fitness-gate-bdd", learningRel},
	}
	for _, c := range chain {
		if _, ok := findEdge(g, c.et, c.from, c.to); !ok {
			t.Errorf("missing chain edge %s %s->%s", c.et, c.from, c.to)
		}
	}
}

func TestWalk_EdgeConfidenceClassification(t *testing.T) {
	g, err := Walk(Options{
		ProjectRoot: fixtureRoot(t),
		Beads:       NewStaticBeadQuerier(true, fixtureBeads()),
	})
	if err != nil {
		t.Fatalf("Walk error: %v", err)
	}

	cases := []struct {
		name     string
		et       EdgeType
		from, to string
		want     Confidence
	}{
		{
			name: "scenario with matching directive_id is high",
			et:   EdgeDirectiveHasScenario, from: "d-fitness-gate-bdd", to: "s-2026-05-17-001",
			want: ConfidenceHigh,
		},
		{
			name: "scenario without directive backref is low",
			et:   EdgeDirectiveHasScenario, from: "d-fitness-gate-bdd", to: "s-2026-05-17-002",
			want: ConfidenceLow,
		},
		{
			name: "explicit Scenarios bead claim is high",
			et:   EdgeScenarioClaimedByBead, from: "s-2026-05-17-001", to: "soc-58nt.2.6",
			want: ConfidenceHigh,
		},
		{
			name: "free-text bead mention is low",
			et:   EdgeScenarioClaimedByBead, from: "s-2026-05-17-002", to: "soc-58nt.4.1",
			want: ConfidenceLow,
		},
		{
			name: "scenario result edge is high",
			et:   EdgeScenarioResult, from: "s-2026-05-17-001", to: "result:s-2026-05-17-001",
			want: ConfidenceHigh,
		},
		{
			name: "frontmatter directive_id learning edge is high",
			et:   EdgeDirectiveHasLearning, from: "d-fitness-gate-bdd",
			to:   filepath.ToSlash(filepath.Join("docs", "learnings", "2026-05-17-trace-chain.md")),
			want: ConfidenceHigh,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, ok := findEdge(g, c.et, c.from, c.to)
			if !ok {
				t.Fatalf("edge %s %s->%s not found", c.et, c.from, c.to)
			}
			if e.Confidence != c.want {
				t.Errorf("confidence = %q, want %q", e.Confidence, c.want)
			}
		})
	}
}

func TestWalk_DirectiveWithNoScenariosWarning(t *testing.T) {
	g, err := Walk(Options{
		ProjectRoot: fixtureRoot(t),
		Beads:       NewStaticBeadQuerier(true, fixtureBeads()),
	})
	if err != nil {
		t.Fatalf("Walk error: %v", err)
	}
	e, ok := findEdge(g, EdgeDirectiveHasScenario, "d-trace-chain-render", "")
	if !ok {
		t.Fatalf("expected a directive_no_scenarios edge for d-trace-chain-render")
	}
	if len(e.Defects) != 1 {
		t.Fatalf("want exactly 1 defect, got %d", len(e.Defects))
	}
	if e.Defects[0].Code != DefectDirectiveNoScenario {
		t.Errorf("defect code = %q, want %q", e.Defects[0].Code, DefectDirectiveNoScenario)
	}
	if e.Defects[0].Severity != SeverityWarning {
		t.Errorf("severity = %q, want %q", e.Defects[0].Severity, SeverityWarning)
	}
}

func TestWalk_MissingLearningsDirDoesNotCrash(t *testing.T) {
	// The no-learnings fixture has a valid GOALS.md + scenarios but no
	// docs/learnings/ directory: graceful degradation, not a crash.
	root := filepath.Join(fixtureRoot(t), "no-learnings")
	g, err := Walk(Options{
		ProjectRoot: root,
		Beads:       NewStaticBeadQuerier(false, nil),
	})
	if err != nil {
		t.Fatalf("Walk must not error on missing learnings dir: %v", err)
	}
	// The directive->scenario edge must still be built.
	if _, ok := findEdge(g, EdgeDirectiveHasScenario, "d-fitness-gate-bdd", "s-2026-05-17-001"); !ok {
		t.Errorf("directive->scenario edge missing despite absent learnings dir")
	}
	// A diagnostic for the absent learnings dir must be recorded.
	foundLearningDiag := false
	foundBeadDiag := false
	for _, d := range g.Diagnostics {
		if contains(d, "docs/learnings/") {
			foundLearningDiag = true
		}
		if contains(d, "bd not available") {
			foundBeadDiag = true
		}
	}
	if !foundLearningDiag {
		t.Errorf("expected a docs/learnings diagnostic, got %v", g.Diagnostics)
	}
	if !foundBeadDiag {
		t.Errorf("expected a bd-unavailable diagnostic, got %v", g.Diagnostics)
	}
}

func TestWalk_MissingGoalsFileYieldsEmptyGraph(t *testing.T) {
	g, err := Walk(Options{
		ProjectRoot: t.TempDir(),
		Beads:       NewStaticBeadQuerier(false, nil),
	})
	if err != nil {
		t.Fatalf("Walk must not error on missing GOALS.md: %v", err)
	}
	if len(g.Nodes) != 0 {
		t.Errorf("nodes = %d, want 0", len(g.Nodes))
	}
	if len(g.Edges) != 0 {
		t.Errorf("edges = %d, want 0", len(g.Edges))
	}
	if len(g.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %d, want 1", len(g.Diagnostics))
	}
	if !contains(g.Diagnostics[0], "GOALS.md not readable") {
		t.Errorf("diagnostic = %q, want GOALS.md-not-readable message", g.Diagnostics[0])
	}
}

func TestWalk_DeterministicOrdering(t *testing.T) {
	opts := Options{
		ProjectRoot: fixtureRoot(t),
		Beads:       NewStaticBeadQuerier(true, fixtureBeads()),
	}
	first, err := Walk(opts)
	if err != nil {
		t.Fatalf("first Walk error: %v", err)
	}
	second, err := Walk(opts)
	if err != nil {
		t.Fatalf("second Walk error: %v", err)
	}
	fb, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal first: %v", err)
	}
	sb, err := json.Marshal(second)
	if err != nil {
		t.Fatalf("marshal second: %v", err)
	}
	if string(fb) != string(sb) {
		t.Errorf("graph JSON not deterministic:\nfirst=%s\nsecond=%s", fb, sb)
	}
}

func TestWalk_AllSixEdgeTypesDefined(t *testing.T) {
	got := EdgeTypes()
	want := []EdgeType{
		EdgeDirectiveHasScenario,
		EdgeScenarioResult,
		EdgeScenarioClaimedByBead,
		EdgeBeadProducedArtifact,
		EdgeArtifactCitedByLearning,
		EdgeDirectiveHasLearning,
	}
	if len(got) != 6 {
		t.Fatalf("EdgeTypes() returned %d types, want 6", len(got))
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("EdgeTypes()[%d] = %q, want %q", i, got[i], w)
		}
	}
}

// TestWalk_AllSixEdgeTypesOccurInConnectedWalk verifies that a full walk over
// the fixture tree (with beads and artifacts) emits at least one edge of each
// of the six ADR-0005 edge types in a single connected run, not merely that the
// constants are declared.
func TestWalk_AllSixEdgeTypesOccurInConnectedWalk(t *testing.T) {
	g, err := Walk(Options{
		ProjectRoot: fixtureRoot(t),
		Beads:       NewStaticBeadQuerier(true, fixtureBeads()),
	})
	if err != nil {
		t.Fatalf("Walk error: %v", err)
	}
	seen := map[EdgeType]bool{}
	for _, e := range g.Edges {
		seen[e.Type] = true
	}
	for _, et := range EdgeTypes() {
		if !seen[et] {
			t.Errorf("edge type %q never appeared in the connected walk; edges=%+v", et, g.Edges)
		}
	}
}

// TestWalk_BrokenDirectiveBackrefIsError verifies that a scenario whose
// directive_id back-reference names an unknown directive produces a
// broken_directive_backref error on its reverse edge.
func TestWalk_BrokenDirectiveBackrefIsError(t *testing.T) {
	root := t.TempDir()

	// Write a minimal GOALS.md with one directive.
	goalsContent := `# Goals

## Directives

### 1. Known directive

**Directive ID:** d-known
**Steer:** known
`
	if err := os.WriteFile(filepath.Join(root, "GOALS.md"), []byte(goalsContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write a scenario that back-references a directive that does NOT exist.
	specDir := filepath.Join(root, "spec", "scenarios")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sf := `{"id":"s-2026-05-17-007","directive_id":"d-does-not-exist","goal":"orphan","status":"active"}`
	if err := os.WriteFile(filepath.Join(specDir, "s-2026-05-17-007.json"), []byte(sf), 0o644); err != nil {
		t.Fatal(err)
	}

	g, err := Walk(Options{
		ProjectRoot: root,
		Beads:       NewStaticBeadQuerier(false, nil),
	})
	if err != nil {
		t.Fatalf("Walk error: %v", err)
	}

	// Find the reverse-link edge emitted for the broken back-reference.
	var brokenEdge *Edge
	for i := range g.Edges {
		e := &g.Edges[i]
		if e.Type == EdgeDirectiveHasScenario && e.ToID == "s-2026-05-17-007" {
			brokenEdge = e
			break
		}
	}
	if brokenEdge == nil {
		t.Fatalf("expected a directive_has_scenario edge for s-2026-05-17-007; edges=%+v", g.Edges)
	}
	if len(brokenEdge.Defects) == 0 {
		t.Fatalf("expected at least 1 defect on the broken-backref edge, got none")
	}
	found := false
	for _, d := range brokenEdge.Defects {
		if d.Code == DefectBrokenDirectiveBackref && d.Severity == SeverityError {
			found = true
		}
	}
	if !found {
		t.Errorf("broken_directive_backref error defect not found; defects=%+v", brokenEdge.Defects)
	}
}

// TestBuilderAddNode_DeduplicatesKeepingFirstLabelAndPath verifies the builder
// deduplication contract: adding a node with the same ID a second time keeps
// the first non-empty label and path but does not create a duplicate node.
func TestBuilderAddNode_DeduplicatesKeepingFirstLabelAndPath(t *testing.T) {
	b := newBuilder()
	b.addNode(Node{ID: "d-foo", Type: NodeDirective, Label: "first label", Path: "GOALS.md"})
	b.addNode(Node{ID: "d-foo", Type: NodeDirective, Label: "second label", Path: "other.md"})
	b.addNode(Node{ID: "d-foo", Type: NodeDirective}) // empty label+path variant

	g := b.graph()
	var fooNodes []Node
	for _, n := range g.Nodes {
		if n.ID == "d-foo" {
			fooNodes = append(fooNodes, n)
		}
	}
	if len(fooNodes) != 1 {
		t.Fatalf("builder emitted %d nodes for d-foo, want exactly 1", len(fooNodes))
	}
	if fooNodes[0].Label != "first label" {
		t.Errorf("Label = %q, want first label (first write wins)", fooNodes[0].Label)
	}
	if fooNodes[0].Path != "GOALS.md" {
		t.Errorf("Path = %q, want GOALS.md (first write wins)", fooNodes[0].Path)
	}
}

// TestBuilderAddNode_FillsEmptyLabelFromSubsequentAdd verifies that the builder
// fills an empty label from the second add when the first was empty.
func TestBuilderAddNode_FillsEmptyLabelFromSubsequentAdd(t *testing.T) {
	b := newBuilder()
	b.addNode(Node{ID: "d-bar", Type: NodeDirective}) // no label
	b.addNode(Node{ID: "d-bar", Type: NodeDirective, Label: "filled label", Path: "GOALS.md"})

	g := b.graph()
	for _, n := range g.Nodes {
		if n.ID == "d-bar" {
			if n.Label != "filled label" {
				t.Errorf("Label = %q, want filled label (second add fills empty)", n.Label)
			}
			if n.Path != "GOALS.md" {
				t.Errorf("Path = %q, want GOALS.md", n.Path)
			}
			return
		}
	}
	t.Fatal("d-bar node not found")
}

// contains is a small substring helper to keep assertions readable.
func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0)
}

// indexOf returns the index of needle in haystack, or -1.
func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
