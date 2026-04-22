package goals

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// chdir switches cwd and returns a cleanup function.
func chdir(t *testing.T, dir string) func() {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	return func() { _ = os.Chdir(prev) }
}

// writeGoalsMD writes a minimal valid GOALS.md file at path.
func writeGoalsMD(t *testing.T, path, extra string) {
	t.Helper()
	content := `# Fitness Goals

## Mission

Test mission.

## North Stars

- Green CI

## Anti-Stars

- Untested code

## Directives

### 1. Establish baseline

Get gates green.

**Steer:** increase

## Gates

` + extra
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write goals: %v", err)
	}
}

func TestOutputValidateResult_ValidNonJSON(t *testing.T) {
	var buf bytes.Buffer
	result := ValidateResult{Valid: true, GoalCount: 3, Version: 4, Format: "md", Directives: 2}
	if err := OutputValidateResult(&buf, false, result); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	if !strings.Contains(s, "VALID: 3 goals") {
		t.Errorf("output missing summary: %s", s)
	}
	if !strings.Contains(s, "Directives: 2") {
		t.Errorf("output missing directives line: %s", s)
	}
}

func TestOutputValidateResult_InvalidNonJSON(t *testing.T) {
	var buf bytes.Buffer
	result := ValidateResult{Valid: false, Errors: []string{"missing id"}, Warnings: []string{"no mission"}}
	err := OutputValidateResult(&buf, false, result)
	if err == nil {
		t.Fatal("expected error for invalid result")
	}
	s := buf.String()
	if !strings.Contains(s, "INVALID: 1 errors") {
		t.Errorf("missing invalid header: %s", s)
	}
	if !strings.Contains(s, "ERROR: missing id") {
		t.Errorf("missing error detail: %s", s)
	}
	if !strings.Contains(s, "WARN: no mission") {
		t.Errorf("missing warning: %s", s)
	}
}

func TestOutputValidateResult_JSON(t *testing.T) {
	var buf bytes.Buffer
	result := ValidateResult{Valid: true, GoalCount: 2}
	if err := OutputValidateResult(&buf, true, result); err != nil {
		t.Fatal(err)
	}
	var got ValidateResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.GoalCount != 2 || !got.Valid {
		t.Errorf("got %+v", got)
	}
}

func TestDirectivesFromPillars_WithPillars(t *testing.T) {
	gs := []Goal{
		{ID: "a", Pillar: "quality"},
		{ID: "b", Pillar: "quality"},
		{ID: "c", Pillar: "health"},
	}
	dirs := DirectivesFromPillars(gs)
	if len(dirs) != 2 {
		t.Fatalf("expected 2 pillars, got %d", len(dirs))
	}
	if dirs[0].Number != 1 {
		t.Errorf("first number should be 1, got %d", dirs[0].Number)
	}
	if dirs[0].Steer != "increase" {
		t.Errorf("default steer should be 'increase', got %q", dirs[0].Steer)
	}
	if !strings.Contains(dirs[0].Title, "quality") {
		t.Errorf("title should include pillar name, got %q", dirs[0].Title)
	}
}

func TestDirectivesFromPillars_NoPillars(t *testing.T) {
	gs := []Goal{{ID: "a"}, {ID: "b"}}
	dirs := DirectivesFromPillars(gs)
	if len(dirs) != 1 {
		t.Errorf("expected 1 default directive, got %d", len(dirs))
	}
}

func TestFindMissingPath(t *testing.T) {
	tmp := t.TempDir()
	cleanup := chdir(t, tmp)
	defer cleanup()

	_ = os.MkdirAll("scripts", 0o755)
	_ = os.WriteFile("scripts/real.sh", []byte("#!/bin/sh\n"), 0o600)

	// Existing file -> no missing
	if got := FindMissingPath("scripts/real.sh"); got != "" {
		t.Errorf("expected empty for existing, got %q", got)
	}
	// Missing file
	if got := FindMissingPath("scripts/missing.sh"); got != "scripts/missing.sh" {
		t.Errorf("got %q", got)
	}
	// Non-script path with extension detects missing
	if got := FindMissingPath("tests/nope.bats"); got != "tests/nope.bats" {
		t.Errorf("tests path: got %q", got)
	}
	// No filesystem references
	if got := FindMissingPath("echo hello world"); got != "" {
		t.Errorf("pure command should have no missing, got %q", got)
	}
}

func TestSplitCommaSeparated(t *testing.T) {
	cases := map[string][]string{
		"a, b, c":    {"a", "b", "c"},
		"a,,b":       {"a", "b"},
		"  ":         nil,
		"":           nil,
		"one":        {"one"},
		"  a  ,  b ": {"a", "b"},
	}
	for in, want := range cases {
		got := SplitCommaSeparated(in)
		if len(got) != len(want) {
			t.Errorf("%q: got %v, want %v", in, got, want)
			continue
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("%q[%d]: got %q, want %q", in, i, got[i], want[i])
			}
		}
	}
}

func TestValidSteers(t *testing.T) {
	for _, s := range []string{"increase", "decrease", "hold", "explore"} {
		if !ValidSteers[s] {
			t.Errorf("%q should be valid", s)
		}
	}
	if ValidSteers["nonsense"] {
		t.Error("nonsense should not be valid")
	}
}

func TestDetectGates_GoDir(t *testing.T) {
	tmp := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmp, "cli"), 0o755)
	_ = os.WriteFile(filepath.Join(tmp, "cli", "go.mod"), []byte("module x\n"), 0o600)

	goals := DetectGates(tmp)
	ids := map[string]bool{}
	for _, g := range goals {
		ids[g.ID] = true
	}
	if !ids["go-build"] || !ids["go-test"] {
		t.Errorf("expected go gates, got %v", ids)
	}
}

func TestDetectGates_RootGoMod(t *testing.T) {
	tmp := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module x\n"), 0o600)

	goals := DetectGates(tmp)
	found := false
	for _, g := range goals {
		if g.Check == "go build ./..." {
			found = true
		}
	}
	if !found {
		t.Errorf("root go.mod should yield 'go build ./...', got %+v", goals)
	}
}

func TestDetectGates_Multi(t *testing.T) {
	tmp := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmp, "package.json"), []byte("{}"), 0o600)
	_ = os.WriteFile(filepath.Join(tmp, "Cargo.toml"), []byte(""), 0o600)
	_ = os.WriteFile(filepath.Join(tmp, "pyproject.toml"), []byte(""), 0o600)
	_ = os.WriteFile(filepath.Join(tmp, "Makefile"), []byte(""), 0o600)

	gates := DetectGates(tmp)
	want := []string{"npm-test", "cargo-test", "python-test", "make-build"}
	found := map[string]bool{}
	for _, g := range gates {
		found[g.ID] = true
	}
	for _, w := range want {
		if !found[w] {
			t.Errorf("missing gate %q (got %v)", w, found)
		}
	}
}

func TestAutoDetectTemplate(t *testing.T) {
	cases := []struct {
		name    string
		marker  string
		content string
		want    string
	}{
		{"go", "go.mod", "module x\n", "go-cli"},
		{"cli/go", "cli/go.mod", "module x\n", "go-cli"},
		{"rust", "Cargo.toml", "", "rust-cli"},
		{"python", "pyproject.toml", "", "python-lib"},
		{"python setup", "setup.py", "", "python-lib"},
		{"web", "package.json", "{}", "web-app"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			p := filepath.Join(tmp, tc.marker)
			_ = os.MkdirAll(filepath.Dir(p), 0o755)
			_ = os.WriteFile(p, []byte(tc.content), 0o600)
			if got := AutoDetectTemplate(tmp); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}

	// No markers -> empty
	if got := AutoDetectTemplate(t.TempDir()); got != "" {
		t.Errorf("expected empty for no markers, got %q", got)
	}
}

func TestLoadTemplate_ValidYAML(t *testing.T) {
	body := `name: test
description: testing
gates:
  - id: g1
    description: gate one
    check: echo ok
    weight: 3
    type: health
`
	fsys := fstest.MapFS{"templates/test.yaml": &fstest.MapFile{Data: []byte(body)}}
	tmpl, err := LoadTemplate(fsys, "test")
	if err != nil {
		t.Fatal(err)
	}
	if tmpl.Name != "test" {
		t.Errorf("name = %q", tmpl.Name)
	}
	if len(tmpl.Gates) != 1 {
		t.Fatalf("expected 1 gate, got %d", len(tmpl.Gates))
	}
	if tmpl.Gates[0].ID != "g1" {
		t.Errorf("gate id = %q", tmpl.Gates[0].ID)
	}
}

func TestLoadTemplate_Missing(t *testing.T) {
	fsys := fstest.MapFS{}
	if _, err := LoadTemplate(fsys, "nope"); err == nil {
		t.Fatal("expected error for missing template")
	}
}

func TestLoadTemplate_InvalidYAML(t *testing.T) {
	// YAML that parses as a list, not as a GoalTemplate struct -> yaml decoder errors
	fsys := fstest.MapFS{"templates/bad.yaml": &fstest.MapFile{Data: []byte("- just a list item\n- another\n")}}
	_, err := LoadTemplate(fsys, "bad")
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestTemplateGatesToGoals(t *testing.T) {
	tmpl := &GoalTemplate{Gates: []GoalTemplateGate{
		{ID: "a", Description: "A", Check: "echo a", Weight: 1, Type: "health"},
		{ID: "b", Description: "B", Check: "echo b", Weight: 2, Type: "quality"},
	}}
	goals := TemplateGatesToGoals(tmpl)
	if len(goals) != 2 {
		t.Fatalf("got %d", len(goals))
	}
	if goals[0].ID != "a" || goals[1].Type != GoalType("quality") {
		t.Errorf("goals = %+v", goals)
	}
}

func TestBuildInteractiveGoalFile_AllDefaults(t *testing.T) {
	in := strings.NewReader("\n\n\n\n\n")
	gf, err := BuildInteractiveGoalFile(in)
	if err != nil {
		t.Fatal(err)
	}
	if gf.Version != 4 || gf.Format != "md" {
		t.Errorf("version/format: %d %q", gf.Version, gf.Format)
	}
	if gf.Mission == "" {
		t.Error("mission should have default")
	}
	if len(gf.NorthStars) == 0 || len(gf.AntiStars) == 0 || len(gf.Directives) == 0 {
		t.Errorf("defaults not populated: %+v", gf)
	}
	if gf.Directives[0].Steer != "increase" {
		t.Errorf("default steer = %q", gf.Directives[0].Steer)
	}
}

func TestBuildInteractiveGoalFile_CustomValues(t *testing.T) {
	in := strings.NewReader("Custom mission\nNS1, NS2\nAS1\nFirst title\nFirst desc\n")
	gf, err := BuildInteractiveGoalFile(in)
	if err != nil {
		t.Fatal(err)
	}
	if gf.Mission != "Custom mission" {
		t.Errorf("mission = %q", gf.Mission)
	}
	if len(gf.NorthStars) != 2 || gf.NorthStars[0] != "NS1" {
		t.Errorf("north stars = %v", gf.NorthStars)
	}
	if gf.Directives[0].Title != "First title" {
		t.Errorf("title = %q", gf.Directives[0].Title)
	}
}

func TestBuildDefaultGoalFile(t *testing.T) {
	gf := BuildDefaultGoalFile()
	if gf.Version != 4 || gf.Format != "md" {
		t.Errorf("version/format")
	}
	if len(gf.Directives) == 0 {
		t.Error("no directives")
	}
	if gf.Directives[0].Steer != "increase" {
		t.Errorf("steer = %q", gf.Directives[0].Steer)
	}
}

func TestRunSteerAdd_RejectsInvalidSteer(t *testing.T) {
	opts := SteerAddOptions{Title: "x", Description: "y", Steer: "bogus"}
	err := RunSteerAdd(opts)
	if err == nil {
		t.Fatal("expected error for invalid steer")
	}
	if !strings.Contains(err.Error(), "invalid steer") {
		t.Errorf("err = %v", err)
	}
}

func TestRunSteerAdd_AddsDirective(t *testing.T) {
	tmp := t.TempDir()
	cleanup := chdir(t, tmp)
	defer cleanup()
	writeGoalsMD(t, "GOALS.md", "")

	var buf bytes.Buffer
	opts := SteerAddOptions{
		Title: "New directive", Description: "Desc", Steer: "increase",
		GoalsFile: "GOALS.md", Stdout: &buf,
	}
	if err := RunSteerAdd(opts); err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(buf.String(), "Added directive #2") {
		t.Errorf("output = %q", buf.String())
	}
	// File should now have the directive
	data, _ := os.ReadFile("GOALS.md")
	if !strings.Contains(string(data), "New directive") {
		t.Errorf("file missing new directive")
	}
}

func TestRunSteerAdd_DryRun(t *testing.T) {
	tmp := t.TempDir()
	cleanup := chdir(t, tmp)
	defer cleanup()
	writeGoalsMD(t, "GOALS.md", "")
	before, _ := os.ReadFile("GOALS.md")

	var buf bytes.Buffer
	opts := SteerAddOptions{
		Title: "Dry", Description: "Dry", Steer: "hold",
		GoalsFile: "GOALS.md", DryRun: true, Stdout: &buf,
	}
	if err := RunSteerAdd(opts); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Would add") {
		t.Errorf("output = %q", buf.String())
	}
	after, _ := os.ReadFile("GOALS.md")
	if string(before) != string(after) {
		t.Errorf("file should be unchanged on dry-run")
	}
}

func TestRunSteerRemove_NotFound(t *testing.T) {
	tmp := t.TempDir()
	cleanup := chdir(t, tmp)
	defer cleanup()
	writeGoalsMD(t, "GOALS.md", "")

	opts := SteerRemoveOptions{Number: 99, GoalsFile: "GOALS.md"}
	err := RunSteerRemove(opts)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %v", err)
	}
}

func TestRunSteerRemove_RemovesAndRenumbers(t *testing.T) {
	tmp := t.TempDir()
	cleanup := chdir(t, tmp)
	defer cleanup()
	writeGoalsMD(t, "GOALS.md", "")
	// Add a couple extra directives
	var buf bytes.Buffer
	_ = RunSteerAdd(SteerAddOptions{Title: "B", Description: "desc B", Steer: "increase", GoalsFile: "GOALS.md", Stdout: &buf})
	_ = RunSteerAdd(SteerAddOptions{Title: "C", Description: "desc C", Steer: "hold", GoalsFile: "GOALS.md", Stdout: &buf})

	buf.Reset()
	if err := RunSteerRemove(SteerRemoveOptions{Number: 1, GoalsFile: "GOALS.md", Stdout: &buf}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Removed directive #1") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestRunSteerPrioritize_InvalidPosition(t *testing.T) {
	tmp := t.TempDir()
	cleanup := chdir(t, tmp)
	defer cleanup()
	writeGoalsMD(t, "GOALS.md", "")

	err := RunSteerPrioritize(SteerPrioritizeOptions{Number: 1, NewPosition: 99, GoalsFile: "GOALS.md"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "between 1 and") {
		t.Errorf("err = %v", err)
	}
}

func TestRunSteerPrioritize_NumberNotFound(t *testing.T) {
	tmp := t.TempDir()
	cleanup := chdir(t, tmp)
	defer cleanup()
	writeGoalsMD(t, "GOALS.md", "")

	err := RunSteerPrioritize(SteerPrioritizeOptions{Number: 5, NewPosition: 1, GoalsFile: "GOALS.md"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %v", err)
	}
}

func TestRunSteerPrioritize_Moves(t *testing.T) {
	tmp := t.TempDir()
	cleanup := chdir(t, tmp)
	defer cleanup()
	writeGoalsMD(t, "GOALS.md", "")

	var buf bytes.Buffer
	_ = RunSteerAdd(SteerAddOptions{Title: "Second", Description: "d", Steer: "increase", GoalsFile: "GOALS.md", Stdout: &buf})
	_ = RunSteerAdd(SteerAddOptions{Title: "Third", Description: "d", Steer: "hold", GoalsFile: "GOALS.md", Stdout: &buf})

	buf.Reset()
	// Move directive 3 (Third) to position 1
	err := RunSteerPrioritize(SteerPrioritizeOptions{Number: 3, NewPosition: 1, GoalsFile: "GOALS.md", Stdout: &buf})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Moved directive") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestRunValidate_ValidFile(t *testing.T) {
	tmp := t.TempDir()
	cleanup := chdir(t, tmp)
	defer cleanup()
	writeGoalsMD(t, "GOALS.md", `

### build
**Weight:** 5
**Type:** health

`+"```bash\ntrue\n```\n")

	var buf bytes.Buffer
	if err := RunValidate(ValidateOptions{GoalsFile: "GOALS.md", Stdout: &buf}); err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(buf.String(), "VALID") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestRunValidate_MissingFile(t *testing.T) {
	var buf bytes.Buffer
	err := RunValidate(ValidateOptions{GoalsFile: "/nope/nonexistent.md", Stdout: &buf})
	if err == nil {
		t.Fatal("expected error")
	}
	// Output should contain "INVALID"
	if !strings.Contains(buf.String(), "INVALID") {
		t.Errorf("output should contain INVALID, got %q", buf.String())
	}
}

func TestRunPrune_NoStale(t *testing.T) {
	tmp := t.TempDir()
	cleanup := chdir(t, tmp)
	defer cleanup()
	writeGoalsMD(t, "GOALS.md", `

### health
**Weight:** 5

`+"```bash\ntrue\n```\n")

	var buf bytes.Buffer
	err := RunPrune(PruneOptions{GoalsFile: "GOALS.md", DryRun: true, Stdout: &buf})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(buf.String(), "No stale goals") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestRunPrune_DryRunDoesNotModify(t *testing.T) {
	tmp := t.TempDir()
	cleanup := chdir(t, tmp)
	defer cleanup()
	writeGoalsMD(t, "GOALS.md", `

### stale-gate
**Weight:** 5

`+"```bash\nscripts/nonexistent-script.sh\n```\n")

	before, _ := os.ReadFile("GOALS.md")
	var buf bytes.Buffer
	if err := RunPrune(PruneOptions{GoalsFile: "GOALS.md", DryRun: true, Stdout: &buf}); err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(buf.String(), "stale goal") {
		t.Errorf("expected stale detected, got %q", buf.String())
	}
	after, _ := os.ReadFile("GOALS.md")
	if string(before) != string(after) {
		t.Errorf("dry-run modified file")
	}
}

func TestRunMigrate_YAMLv1ToV2(t *testing.T) {
	tmp := t.TempDir()
	cleanup := chdir(t, tmp)
	defer cleanup()

	// v1 YAML
	yaml := `version: 1
goals:
  - id: g1
    description: desc
    check: "true"
    weight: 5
`
	if err := os.WriteFile("GOALS.yaml", []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	// Note: LoadGoals emits a warning on v1 via stderr. Suppress by redirecting stderr isn't needed for test assertion.
	err := RunMigrate(MigrateOptions{ToMD: false, GoalsFile: "GOALS.yaml", Stdout: &buf})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat("GOALS.yaml.v1.bak"); err != nil {
		t.Errorf("expected backup: %v", err)
	}
	if !strings.Contains(buf.String(), "Migrated") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestRunMigrate_AlreadyV2(t *testing.T) {
	tmp := t.TempDir()
	cleanup := chdir(t, tmp)
	defer cleanup()

	yaml := `version: 2
goals:
  - id: g1
    description: desc
    check: "true"
    weight: 5
`
	_ = os.WriteFile("GOALS.yaml", []byte(yaml), 0o600)

	var buf bytes.Buffer
	err := RunMigrate(MigrateOptions{ToMD: false, GoalsFile: "GOALS.yaml", Stdout: &buf})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "no migration needed") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestRunInit_CreatesFile(t *testing.T) {
	tmp := t.TempDir()
	cleanup := chdir(t, tmp)
	defer cleanup()

	var buf bytes.Buffer
	err := RunInit(InitOptions{
		NonInteractive: true,
		GoalsFile:      "GOALS.md",
		Stdout:         &buf,
		Stdin:          strings.NewReader(""),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat("GOALS.md"); err != nil {
		t.Errorf("file not created: %v", err)
	}
	if !strings.Contains(buf.String(), "Created") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestRunInit_FailsIfExists(t *testing.T) {
	tmp := t.TempDir()
	cleanup := chdir(t, tmp)
	defer cleanup()
	_ = os.WriteFile("GOALS.md", []byte("existing"), 0o600)

	var buf bytes.Buffer
	err := RunInit(InitOptions{NonInteractive: true, GoalsFile: "GOALS.md", Stdout: &buf})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("err = %v", err)
	}
}

func TestRunInit_DryRun(t *testing.T) {
	tmp := t.TempDir()
	cleanup := chdir(t, tmp)
	defer cleanup()

	var buf bytes.Buffer
	err := RunInit(InitOptions{
		NonInteractive: true, DryRun: true, GoalsFile: "GOALS.md", Stdout: &buf,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("GOALS.md"); err == nil {
		t.Errorf("dry-run should not create file")
	}
	if !strings.Contains(buf.String(), "Would write") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestRunInit_WithTemplate(t *testing.T) {
	tmp := t.TempDir()
	cleanup := chdir(t, tmp)
	defer cleanup()

	tmplBody := `name: test
gates:
  - id: t1
    description: template gate
    check: "true"
    weight: 3
    type: health
`
	fsys := fstest.MapFS{"templates/test.yaml": &fstest.MapFile{Data: []byte(tmplBody)}}

	var buf bytes.Buffer
	err := RunInit(InitOptions{
		NonInteractive: true,
		Template:       "test",
		GoalsFile:      "GOALS.md",
		Stdout:         &buf,
		TemplatesFS:    readFileFSAdapter{fsys},
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	data, _ := os.ReadFile("GOALS.md")
	if !strings.Contains(string(data), "t1") {
		t.Errorf("template gate not present: %s", string(data))
	}
}

// readFileFSAdapter adapts an fs.FS (MapFS implements fs.ReadFileFS already, but
// this keeps the interface explicit at call sites).
type readFileFSAdapter struct{ fs.FS }

func (a readFileFSAdapter) ReadFile(name string) ([]byte, error) {
	return fs.ReadFile(a.FS, name)
}
