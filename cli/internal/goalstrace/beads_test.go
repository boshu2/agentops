package goalstrace

import (
	"path/filepath"
	"testing"
)

// TestClaimedScenarios_HeuristicDoesNotMatchEnglishAutoWords verifies that
// plain English compound words with an "auto-" prefix (auto-merge, auto-update,
// auto-rebase, auto-close) are NOT matched by the heuristic scanner and
// therefore produce no broken_bead_scenario_claim finding of any severity.
func TestClaimedScenarios_HeuristicDoesNotMatchEnglishAutoWords(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{name: "auto-merge", text: "This bead enables auto-merge for the repo."},
		{name: "auto-update", text: "Triggered by the auto-update workflow."},
		{name: "auto-rebase", text: "git auto-rebase is enabled."},
		{name: "auto-close", text: "PR auto-close on merge."},
		{name: "multiple English auto- words", text: "auto-merge and auto-update both mentioned here."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := beadRecord{
				ID:          "soc-test.1",
				Title:       "test bead",
				Description: tc.text,
				Status:      "open",
			}
			explicit, heuristic := claimedScenarios(b)
			if len(explicit) != 0 {
				t.Errorf("explicit claims = %v, want none", explicit)
			}
			if len(heuristic) != 0 {
				t.Errorf("heuristic claims = %v, want none for English auto- word %q", heuristic, tc.name)
			}
		})
	}
}

// TestClaimedScenarios_HeuristicMatchesRealAutoScenarioIDs verifies that
// genuine auto-generated scenario IDs (multi-segment slugs) ARE matched by the
// heuristic scanner when they appear in free text outside a Scenarios: line.
func TestClaimedScenarios_HeuristicMatchesRealAutoScenarioIDs(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{
			name: "auto-nightly-evolution-dry-run",
			text: "This bead tracks work for auto-nightly-evolution-dry-run scenario.",
			want: "auto-nightly-evolution-dry-run",
		},
		{
			name: "auto-agentops-core-cli",
			text: "Implements auto-agentops-core-cli acceptance vector.",
			want: "auto-agentops-core-cli",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := beadRecord{
				ID:          "soc-test.2",
				Title:       "test bead",
				Description: tc.text,
				Status:      "open",
			}
			explicit, heuristic := claimedScenarios(b)
			if len(explicit) != 0 {
				t.Errorf("explicit claims = %v, want none", explicit)
			}
			found := false
			for _, id := range heuristic {
				if id == tc.want {
					found = true
				}
			}
			if !found {
				t.Errorf("heuristic = %v, want %q to be matched", heuristic, tc.want)
			}
		})
	}
}

// TestClaimedScenarios_ExplicitScenariosLineIsHighConfidence verifies that a
// genuine "Scenarios: s-YYYY-MM-DD-NNN" line is extracted as an explicit
// (high-confidence) claim, not a heuristic one.
func TestClaimedScenarios_ExplicitScenariosLineIsHighConfidence(t *testing.T) {
	b := beadRecord{
		ID:          "soc-test.3",
		Title:       "explicit claim bead",
		Description: "Work toward the trace chain.\nScenarios: s-2026-05-17-001",
		Status:      "open",
	}
	explicit, heuristic := claimedScenarios(b)
	if len(explicit) != 1 || explicit[0] != "s-2026-05-17-001" {
		t.Errorf("explicit = %v, want [s-2026-05-17-001]", explicit)
	}
	// The same ID must not also appear in the heuristic list.
	for _, id := range heuristic {
		if id == "s-2026-05-17-001" {
			t.Errorf("s-2026-05-17-001 duplicated in heuristic list")
		}
	}
}

// TestClaimedScenarios_ExplicitScenariosLineRejectsProse is the soc-bhu6w
// regression: a bead whose "Scenarios:" line reflows into a free-form
// "<slug>: <prose>" bullet (the canonical bead-embedded ## Scenarios form) must
// NOT have its comma/semicolon-split prose tokens (frontmatter field names,
// "and section structure", "graduation path (...)", etc.) mis-claimed as
// explicit scenario IDs. Only tokens matching the real scenario-ID grammar are
// claims; everything else is prose and must be dropped.
func TestClaimedScenarios_ExplicitScenariosLineRejectsProse(t *testing.T) {
	// Mirrors the real soc-4mc2 description that produced the 12 false
	// broken_bead_scenario_claim errors: a Scenarios: line followed by a
	// "<slug>: <prose>" bullet whose prose contains a comma-separated field
	// list and English phrases.
	b := beadRecord{
		ID:    "soc-bhu6w.test",
		Title: "prose scenarios bead",
		Description: "Codify the lesson format.\n" +
			"Scenarios: - lesson-format-spec: docs/contracts/lesson-format.md exists " +
			"and defines frontmatter (id, date, severity, trigger, verifiable, rule, " +
			"falsified_by, practice, related), file naming, graduation path " +
			"(unassigned -> proposed -> accepted -> encoded), and section structure",
		Status: "open",
	}
	explicit, heuristic := claimedScenarios(b)
	if len(explicit) != 0 {
		t.Errorf("explicit = %v, want none — every token is prose, not a scenario ID", explicit)
	}
	// The prose tokens (date, rule, trigger, ...) must not leak into the
	// heuristic list either: none of them match the scenario-ID grammar.
	for _, id := range heuristic {
		for _, prose := range []string{"date", "rule", "trigger", "severity", "practice", "verifiable", "and section structure"} {
			if id == prose {
				t.Errorf("prose token %q leaked into heuristic claims %v", prose, heuristic)
			}
		}
	}
}

// TestClaimedScenarios_ExplicitScenariosLineKeepsRealIDAmidProse verifies the
// true-positive half of the soc-bhu6w fix: a genuine scenario ID on a
// "Scenarios:" line is still extracted as an explicit claim even when the line
// also carries prose tokens. A dangling-but-real-format ID must survive to be
// flagged downstream; prose around it must not.
func TestClaimedScenarios_ExplicitScenariosLineKeepsRealIDAmidProse(t *testing.T) {
	b := beadRecord{
		ID:          "soc-bhu6w.test2",
		Title:       "mixed scenarios bead",
		Description: "Scenarios: s-2026-05-17-001, file naming, auto-nightly-evolution-dry-run, and section structure",
		Status:      "open",
	}
	explicit, _ := claimedScenarios(b)
	want := map[string]bool{"s-2026-05-17-001": true, "auto-nightly-evolution-dry-run": true}
	if len(explicit) != len(want) {
		t.Fatalf("explicit = %v, want exactly the two real IDs %v", explicit, want)
	}
	for _, id := range explicit {
		if !want[id] {
			t.Errorf("explicit contains non-ID token %q", id)
		}
	}
}

// TestBeadClaimEdge_ExplicitMissingScenarioIsError verifies that a
// broken_bead_scenario_claim from an explicit "Scenarios:" line (high
// confidence) is classified as error severity — this is the true-defect case
// that must never be silenced.
func TestBeadClaimEdge_ExplicitMissingScenarioIsError(t *testing.T) {
	root := t.TempDir() // empty root: no scenario files, so nothing resolves
	e := beadClaimEdge(root, "soc-test.1", "s-2026-05-17-999", ConfidenceHigh)
	if e.Confidence != ConfidenceHigh {
		t.Errorf("confidence = %q, want high", e.Confidence)
	}
	if len(e.Defects) != 1 {
		t.Fatalf("defects = %d, want 1", len(e.Defects))
	}
	if e.Defects[0].Code != DefectBrokenBeadScenarioClaim {
		t.Errorf("defect code = %q, want %q", e.Defects[0].Code, DefectBrokenBeadScenarioClaim)
	}
	if e.Defects[0].Severity != SeverityError {
		t.Errorf("severity = %q, want error — explicit broken links must be errors", e.Defects[0].Severity)
	}
}

// TestBeadClaimEdge_HeuristicMissingScenarioIsWarningNotError verifies that a
// broken_bead_scenario_claim from a heuristic (low-confidence) free-text match
// is classified as warning severity, never error. This is the core fix for
// soc-58nt.4.9: English "auto-merge" etc. must not produce ERROR findings.
func TestBeadClaimEdge_HeuristicMissingScenarioIsWarningNotError(t *testing.T) {
	root := t.TempDir() // empty root: nothing resolves
	cases := []struct {
		name       string
		scenarioID string
	}{
		{"human-format unresolvable", "s-2026-05-17-999"},
		{"auto multi-segment unresolvable", "auto-nightly-evolution-dry-run"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := beadClaimEdge(root, "soc-test.2", tc.scenarioID, ConfidenceLow)
			if e.Confidence != ConfidenceLow {
				t.Errorf("confidence = %q, want low", e.Confidence)
			}
			if len(e.Defects) != 1 {
				t.Fatalf("defects = %d, want 1", len(e.Defects))
			}
			if e.Defects[0].Code != DefectBrokenBeadScenarioClaim {
				t.Errorf("defect code = %q, want %q", e.Defects[0].Code, DefectBrokenBeadScenarioClaim)
			}
			if e.Defects[0].Severity != SeverityWarning {
				t.Errorf("severity = %q, want warning — heuristic claims must NEVER be errors", e.Defects[0].Severity)
			}
		})
	}
}

// TestBeadClaimEdge_ResolvableScenarioHasNoDefect verifies that a bead claiming
// a scenario that actually resolves produces an edge with no defects, regardless
// of confidence level.
func TestBeadClaimEdge_ResolvableScenarioHasNoDefect(t *testing.T) {
	// Use the standard fixture tree which has s-2026-05-17-001.
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "tests", "fixtures", "goals-trace"))
	if err != nil {
		t.Fatalf("resolving fixture root: %v", err)
	}
	for _, conf := range []Confidence{ConfidenceHigh, ConfidenceLow} {
		t.Run(string(conf), func(t *testing.T) {
			e := beadClaimEdge(root, "soc-test.3", "s-2026-05-17-001", conf)
			if len(e.Defects) != 0 {
				t.Errorf("defects = %v, want none for a resolvable scenario", e.Defects)
			}
		})
	}
}

// TestWalk_EnglishAutoWordInBeadDescriptionProducesNoError verifies the
// end-to-end fix: a bead description containing "auto-merge" (a plain English
// word, not a scenario ID) produces zero broken_bead_scenario_claim findings of
// error severity in a full Walk.
func TestWalk_EnglishAutoWordInBeadDescriptionProducesNoError(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "tests", "fixtures", "goals-trace"))
	if err != nil {
		t.Fatalf("resolving fixture root: %v", err)
	}
	beads := []beadRecord{
		{
			ID:          "soc-test.4",
			Title:       "auto-merge bead",
			Description: "This bead implements auto-merge and auto-update workflows.",
			Status:      "open",
		},
	}
	g, err := Walk(Options{
		ProjectRoot: root,
		Beads:       NewStaticBeadQuerier(true, beads),
	})
	if err != nil {
		t.Fatalf("Walk error: %v", err)
	}
	for _, e := range g.Edges {
		for _, d := range e.Defects {
			if d.Code == DefectBrokenBeadScenarioClaim && d.Severity == SeverityError {
				t.Errorf("unexpected error-severity broken_bead_scenario_claim: edge %+v defect %+v", e, d)
			}
		}
	}
}
