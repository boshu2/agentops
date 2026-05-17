// practices: [bdd-gherkin, llm-eval-harness]
package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/goalstrace"
	"github.com/spf13/cobra"
)

// goalsTraceFixtureRoot resolves the goals-trace fixture tree from cmd/ao.
func goalsTraceFixtureRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "tests", "fixtures", "goals-trace"))
	if err != nil {
		t.Fatalf("resolving fixture root: %v", err)
	}
	return root
}

// goalsTraceFixtureBeads mirrors the goalstrace fixture bead set so the
// command tests build a deterministic graph without a real bd binary.
func goalsTraceFixtureBeads() []goalstrace.BeadInput {
	return []goalstrace.BeadInput{
		{
			ID:          "soc-58nt.2.6",
			Title:       "F2.0 scenario-results producer",
			Description: "Build the scenario-results artifact.\nScenarios: s-2026-05-17-001",
			Status:      "closed",
		},
	}
}

// buildFixtureGraph walks the goals-trace fixture with a deterministic static
// bead querier so command tests never touch the real bd tracker.
func buildFixtureGraph(t *testing.T) goalstrace.Graph {
	t.Helper()
	q := goalstrace.NewStaticBeadQuerierFromInputs(true, goalsTraceFixtureBeads())
	g, err := goalstrace.Walk(goalstrace.Options{
		ProjectRoot: goalsTraceFixtureRoot(t),
		Beads:       q,
	})
	if err != nil {
		t.Fatalf("Walk error: %v", err)
	}
	return g
}

// newTraceTestCmd returns a cobra command whose output is captured in buf.
func newTraceTestCmd(buf *bytes.Buffer) *cobra.Command {
	c := &cobra.Command{}
	c.SetOut(buf)
	c.SetErr(buf)
	return c
}

func TestGoalsTrace_RegisteredUnderGoals(t *testing.T) {
	found := false
	for _, c := range goalsCmd.Commands() {
		if c.Name() == "trace" {
			found = true
		}
	}
	if !found {
		t.Fatal("ao goals trace not registered under goals command")
	}
	if goalsTraceCmd.GroupID != "analysis" {
		t.Errorf("GroupID = %q, want analysis", goalsTraceCmd.GroupID)
	}
	if !strings.Contains(goalsTraceCmd.Long, "scenarios --lint") {
		t.Error("help text must cross-reference ao goals scenarios --lint")
	}
}

func TestGoalsTrace_RequiresAModeFlag(t *testing.T) {
	resetCommandState(t)
	traceFrom, traceOrphans, traceStrict = "", false, false
	buf := &bytes.Buffer{}
	err := runGoalsTrace(newTraceTestCmd(buf), nil)
	if err == nil {
		t.Fatal("expected error when neither --from nor --orphans is set")
	}
	if !strings.Contains(err.Error(), "--from") || !strings.Contains(err.Error(), "--orphans") {
		t.Errorf("error must name both modes, got: %v", err)
	}
}

func TestGoalsTrace_FromAndOrphansAreMutuallyExclusive(t *testing.T) {
	resetCommandState(t)
	traceFrom, traceOrphans = "d-fitness-gate-bdd", true
	defer func() { traceFrom, traceOrphans = "", false }()
	buf := &bytes.Buffer{}
	err := runGoalsTrace(newTraceTestCmd(buf), nil)
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("expected mutually-exclusive error, got: %v", err)
	}
}

func TestGoalsTrace_FromRendersTree(t *testing.T) {
	resetCommandState(t)
	output = "table"
	g := buildFixtureGraph(t)
	traceFrom = "d-fitness-gate-bdd"
	defer func() { traceFrom = "" }()
	buf := &bytes.Buffer{}
	if err := runTraceFrom(newTraceTestCmd(buf), g); err != nil {
		t.Fatalf("runTraceFrom error: %v", err)
	}
	out := buf.String()
	wantSubstrings := []string{
		"directive d-fitness-gate-bdd",
		"[directive_has_scenario:high]",
		"scenario s-2026-05-17-001",
		"[scenario_result:high]",
		"[directive_has_learning:high]",
		"low-confidence edge(s)",
	}
	for _, w := range wantSubstrings {
		if !strings.Contains(out, w) {
			t.Errorf("tree output missing %q\nfull output:\n%s", w, out)
		}
	}
	// The tree must use box-drawing branch glyphs.
	if !strings.Contains(out, "└──") && !strings.Contains(out, "├──") {
		t.Errorf("tree output has no branch glyphs:\n%s", out)
	}
}

func TestGoalsTrace_FromUnknownRootErrors(t *testing.T) {
	resetCommandState(t)
	output = "table"
	g := buildFixtureGraph(t)
	traceFrom = "d-does-not-exist"
	defer func() { traceFrom = "" }()
	buf := &bytes.Buffer{}
	err := runTraceFrom(newTraceTestCmd(buf), g)
	if err == nil || !strings.Contains(err.Error(), "d-does-not-exist") {
		t.Fatalf("expected unknown-root error, got: %v", err)
	}
}

func TestGoalsTrace_FromJSONIsLineDelimitedAndDeterministic(t *testing.T) {
	resetCommandState(t)
	output = "json"
	g := buildFixtureGraph(t)
	traceFrom = "d-fitness-gate-bdd"
	defer func() { traceFrom = "" }()

	render := func() string {
		buf := &bytes.Buffer{}
		if err := runTraceFrom(newTraceTestCmd(buf), g); err != nil {
			t.Fatalf("runTraceFrom json error: %v", err)
		}
		return buf.String()
	}
	first := render()
	second := render()
	if first != second {
		t.Errorf("JSON output not deterministic:\nfirst:\n%s\nsecond:\n%s", first, second)
	}

	lines := strings.Split(strings.TrimSpace(first), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least one edge line + summary, got %d lines", len(lines))
	}
	// Every line must be a standalone JSON object.
	for i, ln := range lines {
		var obj map[string]any
		if err := json.Unmarshal([]byte(ln), &obj); err != nil {
			t.Errorf("line %d is not valid JSON: %q (%v)", i, ln, err)
		}
	}
	// The final line is the summary record.
	var summary map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &summary); err != nil {
		t.Fatalf("summary line invalid: %v", err)
	}
	if summary["summary"] != true {
		t.Errorf("last line is not the summary record: %v", summary)
	}
	if _, ok := summary["error_count"]; !ok {
		t.Error("summary missing error_count field")
	}
}

func TestGoalsTrace_OrphansClassifiesErrorsAndWarnings(t *testing.T) {
	resetCommandState(t)
	output = "table"
	g := buildFixtureGraph(t)
	findings := collectOrphans(g)
	sortOrphans(findings)

	byCode := map[string]orphanFinding{}
	for _, f := range findings {
		byCode[f.Code] = f
	}
	// Broken artifact->bead reference is an ERROR (explicit link broken).
	bad, ok := byCode[goalstrace.DefectBrokenArtifactBeadRef]
	if !ok {
		t.Fatalf("expected a broken_artifact_bead_ref finding; got codes %v", byCode)
	}
	if bad.Severity != string(goalstrace.SeverityError) {
		t.Errorf("broken_artifact_bead_ref severity = %q, want error", bad.Severity)
	}
	// directive_no_scenarios is a WARNING (missing optional yield).
	warn, ok := byCode[goalstrace.DefectDirectiveNoScenario]
	if !ok {
		t.Fatalf("expected a directive_no_scenarios finding")
	}
	if warn.Severity != string(goalstrace.SeverityWarning) {
		t.Errorf("directive_no_scenarios severity = %q, want warning", warn.Severity)
	}
	// no_learning_yield is a derived WARNING for the learning-less directive.
	nl, ok := byCode[goalstrace.DefectNoLearningYield]
	if !ok {
		t.Fatalf("expected a no_learning_yield finding")
	}
	if nl.DirectiveID != "d-trace-chain-render" {
		t.Errorf("no_learning_yield directive = %q, want d-trace-chain-render", nl.DirectiveID)
	}
}

func TestGoalsTrace_OrphansExitCodes(t *testing.T) {
	cases := []struct {
		name     string
		findings []orphanFinding
		strict   bool
		wantErr  bool
	}{
		{
			name:     "no findings is clean exit",
			findings: nil,
			wantErr:  false,
		},
		{
			name:     "errors always fail",
			findings: []orphanFinding{{Severity: "error", Code: "broken_scenario_ref"}},
			strict:   false,
			wantErr:  true,
		},
		{
			name:     "warnings do not fail by default",
			findings: []orphanFinding{{Severity: "warning", Code: "directive_no_scenarios"}},
			strict:   false,
			wantErr:  false,
		},
		{
			name:     "warnings fail under --strict",
			findings: []orphanFinding{{Severity: "warning", Code: "directive_no_scenarios"}},
			strict:   true,
			wantErr:  true,
		},
		{
			name: "errors fail under --strict too",
			findings: []orphanFinding{
				{Severity: "error", Code: "broken_scenario_ref"},
				{Severity: "warning", Code: "directive_no_scenarios"},
			},
			strict:  true,
			wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			origStrict := traceStrict
			traceStrict = c.strict
			defer func() { traceStrict = origStrict }()
			err := orphanExit(c.findings)
			if (err != nil) != c.wantErr {
				t.Errorf("orphanExit err = %v, wantErr = %v", err, c.wantErr)
			}
		})
	}
}

func TestGoalsTrace_OrphansJSONCarriesADRFields(t *testing.T) {
	resetCommandState(t)
	output = "json"
	g := buildFixtureGraph(t)
	traceOrphans = true
	traceStrict = false
	defer func() { traceOrphans = false }()
	buf := &bytes.Buffer{}
	err := runTraceOrphans(newTraceTestCmd(buf), g)
	// Fixture has broken_artifact_bead_ref errors, so a non-zero exit is expected.
	if err == nil {
		t.Fatal("expected non-zero exit for a fixture with broken references")
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatal("orphans JSON produced no output")
	}
	var f orphanFinding
	if jerr := json.Unmarshal([]byte(lines[0]), &f); jerr != nil {
		t.Fatalf("orphan finding line is not valid JSON: %v", jerr)
	}
	if f.Severity == "" || f.Code == "" || f.Message == "" {
		t.Errorf("orphan finding missing required ADR-0005 fields: %+v", f)
	}
}

func TestGoalsTrace_OrphansHumanRenderSummarizes(t *testing.T) {
	resetCommandState(t)
	output = "table"
	g := buildFixtureGraph(t)
	findings := collectOrphans(g)
	sortOrphans(findings)
	buf := &bytes.Buffer{}
	renderOrphans(newTraceTestCmd(buf), findings)
	out := buf.String()
	if !strings.Contains(out, "ERROR") {
		t.Errorf("human render missing ERROR marker:\n%s", out)
	}
	if !strings.Contains(out, "error(s)") || !strings.Contains(out, "warning(s)") {
		t.Errorf("human render missing the count summary:\n%s", out)
	}
}

func TestGoalsTrace_RootedSubgraphIsForwardClosure(t *testing.T) {
	g := buildFixtureGraph(t)
	sub, ok := rootedSubgraph(g, "s-2026-05-17-001")
	if !ok {
		t.Fatal("rootedSubgraph could not root at s-2026-05-17-001")
	}
	// The scenario root must reach its result artifact but NOT the directive
	// above it (forward traversal only).
	ids := map[string]bool{}
	for _, n := range sub.Nodes {
		ids[n.ID] = true
	}
	if !ids["s-2026-05-17-001"] {
		t.Error("subgraph missing its own root node")
	}
	if !ids["result:s-2026-05-17-001"] {
		t.Error("subgraph missing the forward-reachable result artifact")
	}
	if ids["d-fitness-gate-bdd"] {
		t.Error("subgraph wrongly included the upstream directive (not forward-reachable)")
	}
}
