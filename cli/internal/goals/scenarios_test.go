package goals

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestFilterDirectives(t *testing.T) {
	dirs := []ParsedDirective{
		{Number: 1, Title: "One", StableID: "d-one"},
		{Number: 2, Title: "Two", StableID: "d-two"},
		{Number: 3, Title: "Three", StableID: "d-three"},
	}
	cases := []struct {
		name string
		num  int
		id   string
		want []int
	}{
		{"no filter", 0, "", []int{1, 2, 3}},
		{"by number", 2, "", []int{2}},
		{"by stable id", 0, "d-three", []int{3}},
		{"both agree", 1, "d-one", []int{1}},
		{"both disagree", 1, "d-two", nil},
		{"missing number", 9, "", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FilterDirectives(dirs, tc.num, tc.id)
			var nums []int
			for _, d := range got {
				nums = append(nums, d.Number)
			}
			if !equalInts(nums, tc.want) {
				t.Errorf("FilterDirectives(%d,%q) = %v, want %v", tc.num, tc.id, nums, tc.want)
			}
		})
	}
}

func TestBuildScenariosReport(t *testing.T) {
	resolve := func(id string) (*ScenarioMeta, error) {
		switch id {
		case "s-ok":
			return &ScenarioMeta{ID: id, Status: "active", SatisfactionThreshold: 0.8, DirectiveID: "d-one", Path: "spec/scenarios/s-ok.json"}, nil
		case "s-conflict":
			return &ScenarioMeta{ID: id, Status: "draft", SatisfactionThreshold: 0.7, DirectiveID: "d-other", Path: "p"}, nil
		case "s-broken":
			return nil, errors.New("invalid JSON")
		default:
			return nil, nil // not found
		}
	}
	directives := []ParsedDirective{
		{Number: 1, Title: "One", StableID: "d-one", Scenarios: []string{"s-ok", "s-missing", "s-conflict", "s-broken"}},
		{Number: 2, Title: "Two", StableID: "d-two"},
	}
	rep := BuildScenariosReport(directives, resolve)
	if len(rep.Directives) != 2 {
		t.Fatalf("directive count = %d, want 2", len(rep.Directives))
	}

	links := rep.Directives[0].Scenarios
	if len(links) != 4 {
		t.Fatalf("directive 1 links = %d, want 4", len(links))
	}
	wantHealth := map[string]string{
		"s-ok":       LinkHealthOK,
		"s-missing":  LinkHealthMissing,
		"s-conflict": LinkHealthConflict,
		"s-broken":   LinkHealthError,
	}
	for _, l := range links {
		if l.LinkHealth != wantHealth[l.ScenarioID] {
			t.Errorf("%s link health = %q, want %q", l.ScenarioID, l.LinkHealth, wantHealth[l.ScenarioID])
		}
	}
	if links[0].Status != "active" || links[0].SatisfactionThreshold != 0.8 {
		t.Errorf("s-ok link = %+v, want status=active threshold=0.8", links[0])
	}
	if !strings.Contains(links[2].Message, "d-other") {
		t.Errorf("conflict message = %q, want it to name the mismatching directive_id", links[2].Message)
	}
	if rep.Directives[1].Scenarios == nil || len(rep.Directives[1].Scenarios) != 0 {
		t.Errorf("directive 2 scenarios = %v, want empty non-nil slice", rep.Directives[1].Scenarios)
	}
}

func writeScenarioJSON(t *testing.T, dir, id, directiveID, status string, threshold float64) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{
		"id": id, "version": 1, "date": "2026-05-17", "goal": "g",
		"narrative": "n", "expected_outcome": "e",
		"satisfaction_threshold": threshold, "status": status,
	}
	if directiveID != "" {
		body["directive_id"] = directiveID
	}
	data, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunScenarios_EndToEnd(t *testing.T) {
	tmp := t.TempDir()
	goalsPath := filepath.Join(tmp, "GOALS.md")
	specDir := filepath.Join(tmp, "spec", "scenarios")
	src := "# Goals\n\nm\n\n## Directives\n\n" +
		"### 1. Linked directive\n\nbody\n\n**Directive ID:** d-linked\n" +
		"**Steer:** increase (x)\n" +
		"**Scenarios:** s-2026-05-17-001, s-2026-05-17-009\n\n" +
		"### 2. Bare directive\n\nbody\n\n**Steer:** decrease (y)\n"
	if err := os.WriteFile(goalsPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	writeScenarioJSON(t, specDir, "s-2026-05-17-001", "d-linked", "active", 0.8)
	// s-2026-05-17-009 deliberately absent — exercises the missing-link path.

	var out bytes.Buffer
	err := RunScenarios(ScenariosOptions{
		GoalsFile: goalsPath, Stdout: &out, Stderr: &out,
		SpecDirs: []string{specDir},
	})
	if err != nil {
		t.Fatalf("RunScenarios: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"Directive 1 [d-linked]", "s-2026-05-17-001", "ok",
		"s-2026-05-17-009", "missing", "(no linked scenarios)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q\n--- output ---\n%s", want, text)
		}
	}
}

func TestRunScenarios_JSON(t *testing.T) {
	tmp := t.TempDir()
	goalsPath := filepath.Join(tmp, "GOALS.md")
	specDir := filepath.Join(tmp, "spec", "scenarios")
	src := "# Goals\n\nm\n\n## Directives\n\n### 1. Linked\n\nb\n\n" +
		"**Directive ID:** d-linked\n**Scenarios:** s-x\n"
	if err := os.WriteFile(goalsPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	writeScenarioJSON(t, specDir, "s-x", "d-linked", "active", 0.8)

	var out bytes.Buffer
	err := RunScenarios(ScenariosOptions{
		GoalsFile: goalsPath, JSON: true, Stdout: &out, SpecDirs: []string{specDir},
	})
	if err != nil {
		t.Fatalf("RunScenarios: %v", err)
	}
	var rep ScenariosReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("JSON output not parseable: %v\n%s", err, out.String())
	}
	if len(rep.Directives) != 1 || rep.Directives[0].DirectiveID != "d-linked" {
		t.Fatalf("report = %+v, want one d-linked directive", rep)
	}
	if rep.Directives[0].Scenarios[0].LinkHealth != LinkHealthOK {
		t.Errorf("link health = %q, want ok", rep.Directives[0].Scenarios[0].LinkHealth)
	}
}

func TestRunScenarios_FilterNoMatch(t *testing.T) {
	tmp := t.TempDir()
	goalsPath := filepath.Join(tmp, "GOALS.md")
	if err := os.WriteFile(goalsPath, []byte("# Goals\n\nm\n\n## Directives\n\n### 1. Only\n\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := RunScenarios(ScenariosOptions{GoalsFile: goalsPath, DirectiveNum: 99, Stdout: &bytes.Buffer{}})
	if err == nil || !strings.Contains(err.Error(), "no directive matches") {
		t.Errorf("expected a no-match error, got %v", err)
	}
}
