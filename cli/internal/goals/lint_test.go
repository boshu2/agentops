package goals

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLintScenarios_ErrorClasses(t *testing.T) {
	resolve := func(id string) (*ScenarioMeta, error) {
		switch id {
		case "s-conflict":
			return &ScenarioMeta{ID: id, Status: "active", DirectiveID: "d-other", Path: "spec/scenarios/s-conflict.json"}, nil
		case "s-broken":
			return nil, errors.New("invalid JSON")
		default:
			return nil, nil // missing
		}
	}
	directives := []ParsedDirective{
		{Number: 1, Title: "One", StableID: "d-one", Scenarios: []string{"s-missing", "s-conflict", "s-broken"}},
	}
	rep := LintScenarios(directives, resolve, nil, "spec/scenarios")

	codeSeverity := map[string]string{}
	for _, f := range rep.Findings {
		codeSeverity[f.Code] = f.Severity
	}
	for _, code := range []string{CodeMissingScenario, CodeDirectiveIDConflict, CodeMalformedScenario} {
		if codeSeverity[code] != SeverityError {
			t.Errorf("%s reported with severity %q, want error", code, codeSeverity[code])
		}
	}
	if rep.Errors != 3 {
		t.Errorf("Errors = %d, want 3", rep.Errors)
	}
}

func TestLintScenarios_WarningClasses(t *testing.T) {
	resolve := func(id string) (*ScenarioMeta, error) {
		if id == "s-active" {
			return &ScenarioMeta{ID: id, Status: "active", DirectiveID: "d-one", Path: "spec/scenarios/s-active.json"}, nil
		}
		return nil, nil
	}
	directives := []ParsedDirective{
		{Number: 1, Title: "One", StableID: "d-one", Scenarios: []string{"s-active"}},
		{Number: 2, Title: "Two", StableID: "d-two"}, // zero linked scenarios
	}
	files := []ScenarioMeta{
		{ID: "s-active", Path: "spec/scenarios/s-active.json"},
		{ID: "s-orphan", Path: "spec/scenarios/s-orphan.json"},                                               // promoted + unlinked → orphan
		{ID: "s-holdout", Path: filepath.Join(".agents", "holdout", "s-holdout.json")},                       // ad hoc holdout → not flagged
		{ID: "s-claims", DirectiveID: "d-ghost", Path: filepath.Join(".agents", "holdout", "s-claims.json")}, // one-sided → orphan
	}
	rep := LintScenarios(directives, resolve, files, "spec/scenarios")

	var zero, orphans int
	for _, f := range rep.Findings {
		switch f.Code {
		case CodeZeroScenarioDirective:
			zero++
		case CodeOrphanScenario:
			orphans++
		}
	}
	if zero != 1 {
		t.Errorf("zero-scenario-directive count = %d, want 1", zero)
	}
	if orphans != 2 {
		t.Errorf("orphan-scenario count = %d, want 2 (s-orphan + s-claims; the ad hoc holdout scenario must not be flagged)", orphans)
	}
	if rep.Errors != 0 {
		t.Errorf("Errors = %d, want 0 (warning classes only)", rep.Errors)
	}
}

func TestRunLint_ExitCodes(t *testing.T) {
	tmp := t.TempDir()
	goalsPath := filepath.Join(tmp, "GOALS.md")
	specDir := filepath.Join(tmp, "spec", "scenarios")
	src := "# Goals\n\nm\n\n## Directives\n\n" +
		"### 1. One\n\nb\n\n**Directive ID:** d-one\n**Scenarios:** s-2026-05-17-001\n\n" +
		"### 2. Two\n\nb\n\n**Directive ID:** d-two\n"
	if err := os.WriteFile(goalsPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Directive 1 links a missing scenario → an error → non-zero exit.
	if err := RunLint(LintOptions{GoalsFile: goalsPath, SpecDirs: []string{specDir}, Stdout: &bytes.Buffer{}}); err == nil {
		t.Error("RunLint default must exit non-zero when an error exists")
	}

	// Provide the scenario: only the directive-2 warning remains.
	writeScenarioJSON(t, specDir, "s-2026-05-17-001", "d-one", "active", 0.8)
	if err := RunLint(LintOptions{GoalsFile: goalsPath, SpecDirs: []string{specDir}, Stdout: &bytes.Buffer{}}); err != nil {
		t.Errorf("RunLint default must exit zero when only warnings remain, got %v", err)
	}
	if err := RunLint(LintOptions{GoalsFile: goalsPath, SpecDirs: []string{specDir}, Strict: true, Stdout: &bytes.Buffer{}}); err == nil {
		t.Error("RunLint --strict must exit non-zero when a warning exists")
	}
}

func TestRunLint_JSON(t *testing.T) {
	tmp := t.TempDir()
	goalsPath := filepath.Join(tmp, "GOALS.md")
	src := "# Goals\n\nm\n\n## Directives\n\n### 1. One\n\nb\n\n" +
		"**Directive ID:** d-one\n**Scenarios:** s-missing\n"
	if err := os.WriteFile(goalsPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	_ = RunLint(LintOptions{
		GoalsFile: goalsPath, SpecDirs: []string{filepath.Join(tmp, "spec")},
		JSON: true, Stdout: &out,
	})
	var rep LintReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("lint JSON not parseable: %v\n%s", err, out.String())
	}
	found := false
	for _, f := range rep.Findings {
		if f.Code == CodeMissingScenario && f.ScenarioID == "s-missing" && f.Severity == SeverityError {
			found = true
		}
	}
	if !found {
		t.Errorf("missing-scenario error finding absent from JSON: %+v", rep)
	}
}
