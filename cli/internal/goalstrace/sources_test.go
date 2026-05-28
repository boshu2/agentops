package goalstrace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestResolveScenario_PromotedPathIsHigherPriority verifies that a scenario
// file in spec/scenarios/ (promoted) is preferred over .agents/holdout/ when
// both exist, and marks promoted=true.
func TestResolveScenario_PromotedPathIsHigherPriority(t *testing.T) {
	root := t.TempDir()
	spec := filepath.Join(root, "spec", "scenarios")
	holdout := filepath.Join(root, ".agents", "holdout")
	if err := os.MkdirAll(spec, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(holdout, 0o755); err != nil {
		t.Fatal(err)
	}
	sf := scenarioFile{ID: "s-2026-05-17-001", Goal: "promoted goal"}
	writeJSON(t, filepath.Join(spec, "s-2026-05-17-001.json"), sf)
	sfH := scenarioFile{ID: "s-2026-05-17-001", Goal: "holdout goal"}
	writeJSON(t, filepath.Join(holdout, "s-2026-05-17-001.json"), sfH)

	res := resolveScenario(root, "s-2026-05-17-001")
	if !res.Found {
		t.Fatal("resolveScenario should find the scenario")
	}
	if !res.File.promoted {
		t.Error("promoted flag must be true when resolved from spec/scenarios/")
	}
	if res.File.Goal != "promoted goal" {
		t.Errorf("Goal = %q, want promoted goal", res.File.Goal)
	}
}

// TestResolveScenario_HoldoutFallback verifies that a scenario only in
// .agents/holdout/ is found with promoted=false.
func TestResolveScenario_HoldoutFallback(t *testing.T) {
	root := t.TempDir()
	holdout := filepath.Join(root, ".agents", "holdout")
	if err := os.MkdirAll(holdout, 0o755); err != nil {
		t.Fatal(err)
	}
	sf := scenarioFile{ID: "s-2026-05-17-042", Goal: "holdout only"}
	writeJSON(t, filepath.Join(holdout, "s-2026-05-17-042.json"), sf)

	res := resolveScenario(root, "s-2026-05-17-042")
	if !res.Found {
		t.Fatal("resolveScenario should find the holdout scenario")
	}
	if res.File.promoted {
		t.Error("promoted flag must be false when resolved from .agents/holdout/")
	}
	if res.File.Goal != "holdout only" {
		t.Errorf("Goal = %q, want holdout only", res.File.Goal)
	}
}

// TestResolveScenario_AbsentYieldsNotFound verifies that a missing scenario ID
// returns Found=false and File=nil without panicking.
func TestResolveScenario_AbsentYieldsNotFound(t *testing.T) {
	res := resolveScenario(t.TempDir(), "s-2026-05-17-999")
	if res.Found {
		t.Error("absent scenario must not be found")
	}
	if res.File != nil {
		t.Error("File must be nil when not found")
	}
}

// TestLoadAllScenarios_ReturnsOnlyJSONFilesIDSorted verifies that loadAllScenarios
// reads spec/scenarios/ files in deterministic ID order and skips non-JSON.
func TestLoadAllScenarios_ReturnsOnlyJSONFilesIDSorted(t *testing.T) {
	root := t.TempDir()
	spec := filepath.Join(root, "spec", "scenarios")
	if err := os.MkdirAll(spec, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write in reverse order to verify sort.
	writeJSON(t, filepath.Join(spec, "s-2026-05-17-003.json"), scenarioFile{ID: "s-2026-05-17-003"})
	writeJSON(t, filepath.Join(spec, "s-2026-05-17-001.json"), scenarioFile{ID: "s-2026-05-17-001"})
	writeJSON(t, filepath.Join(spec, "s-2026-05-17-002.json"), scenarioFile{ID: "s-2026-05-17-002"})
	// A non-JSON file must be skipped.
	if err := os.WriteFile(filepath.Join(spec, "README.md"), []byte("# readme"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := loadAllScenarios(root)
	if len(got) != 3 {
		t.Fatalf("loadAllScenarios returned %d scenarios, want 3", len(got))
	}
	want := []string{"s-2026-05-17-001", "s-2026-05-17-002", "s-2026-05-17-003"}
	for i, w := range want {
		if got[i].ID != w {
			t.Errorf("got[%d].ID = %q, want %q", i, got[i].ID, w)
		}
		if !got[i].promoted {
			t.Errorf("got[%d] from spec/scenarios must have promoted=true", i)
		}
	}
}

// TestLoadAllScenarios_MissingDirReturnsEmpty verifies graceful degradation
// when spec/scenarios/ does not exist.
func TestLoadAllScenarios_MissingDirReturnsEmpty(t *testing.T) {
	got := loadAllScenarios(t.TempDir())
	if len(got) != 0 {
		t.Errorf("loadAllScenarios on missing dir returned %d entries, want 0", len(got))
	}
}

// TestParseLearning_ExtractsFrontmatterFields verifies that parseLearning
// correctly extracts directive_id, scenario_id, and source from YAML frontmatter.
func TestParseLearning_ExtractsFrontmatterFields(t *testing.T) {
	content := `---
directive_id: d-fitness-gate-bdd
scenario_id: s-2026-05-17-001
source: .agents/rpi/runs/2026-05-17-soc-58nt.2.6/verdict.md
---
The body of the learning.
`
	lf := parseLearning("/docs/learnings/2026-05-17-test.md", content)

	if lf.fm.directiveID != "d-fitness-gate-bdd" {
		t.Errorf("directiveID = %q, want d-fitness-gate-bdd", lf.fm.directiveID)
	}
	if lf.fm.scenarioID != "s-2026-05-17-001" {
		t.Errorf("scenarioID = %q, want s-2026-05-17-001", lf.fm.scenarioID)
	}
	if lf.fm.source != ".agents/rpi/runs/2026-05-17-soc-58nt.2.6/verdict.md" {
		t.Errorf("source = %q, want artifact path", lf.fm.source)
	}
	if lf.body != "The body of the learning.\n" {
		t.Errorf("body = %q, want the body line", lf.body)
	}
}

// TestParseLearning_NoFrontmatterLeavesFieldsEmpty verifies that a learning
// without a YAML block produces empty frontmatter fields.
func TestParseLearning_NoFrontmatterLeavesFieldsEmpty(t *testing.T) {
	lf := parseLearning("/docs/learnings/2026-05-17-plain.md", "# plain\nno frontmatter here")
	if lf.fm.directiveID != "" || lf.fm.scenarioID != "" || lf.fm.source != "" {
		t.Errorf("expected empty frontmatter fields, got %+v", lf.fm)
	}
}

// TestParseLearning_UnterminatedFrontmatterLeavesFieldsEmpty verifies that an
// unterminated frontmatter block (no closing ---) is not parsed.
func TestParseLearning_UnterminatedFrontmatterLeavesFieldsEmpty(t *testing.T) {
	content := "---\ndirective_id: d-foo\nno closing fence"
	lf := parseLearning("/docs/learnings/2026-05-17-bad.md", content)
	if lf.fm.directiveID != "" {
		t.Errorf("unterminated frontmatter must not extract directive_id, got %q", lf.fm.directiveID)
	}
}

// TestParseLearning_QuotedValuesAreTrimmed verifies that quoted field values
// have their surrounding quotes removed.
func TestParseLearning_QuotedValuesAreTrimmed(t *testing.T) {
	content := "---\ndirective_id: \"d-trace-chain-render\"\n---\nbody"
	lf := parseLearning("/docs/learnings/2026-05-17-quoted.md", content)
	if lf.fm.directiveID != "d-trace-chain-render" {
		t.Errorf("directiveID = %q, want d-trace-chain-render (without quotes)", lf.fm.directiveID)
	}
}

// writeJSON is a test helper that marshals v to JSON and writes it to path.
func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %T: %v", v, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
