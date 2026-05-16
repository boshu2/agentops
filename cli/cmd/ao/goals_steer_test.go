// practices: [dora-metrics, lean-startup]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/goals"
)

// --- Helper ---

func writeTestGoalsMD(t *testing.T, dir string) string {
	t.Helper()
	md := `# Goals

Mission.

## Directives

### 1. Ship fast

Deploy continuously.

**Steer:** increase

### 2. Stay secure

No vulnerabilities.

**Steer:** hold

### 3. Reduce debt

Pay down tech debt.

**Steer:** decrease

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| gate-one | ` + "`exit 0`" + ` | 5 | Gate one |
`
	goalsPath := filepath.Join(dir, "GOALS.md")
	if err := os.WriteFile(goalsPath, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	return goalsPath
}

// --- steer command ---

func TestGoalsSteer_HasSubcommands(t *testing.T) {
	subNames := map[string]bool{}
	for _, sub := range goalsSteerCmd.Commands() {
		subNames[sub.Name()] = true
	}

	for _, want := range []string{"add", "remove", "prioritize"} {
		if !subNames[want] {
			t.Errorf("missing steer subcommand %q", want)
		}
	}
}

func TestGoalsSteer_CmdAttributes(t *testing.T) {
	if goalsSteerCmd.Use != "steer" {
		t.Errorf("Use = %q, want steer", goalsSteerCmd.Use)
	}
	if goalsSteerCmd.GroupID != "management" {
		t.Errorf("GroupID = %q, want management", goalsSteerCmd.GroupID)
	}
}

// --- steer add ---

func TestSteerAdd_InvalidSteer(t *testing.T) {
	dir := t.TempDir()
	goalsPath := writeTestGoalsMD(t, dir)

	oldFile := goalsFile
	oldSteer := steerAddSteer
	oldDesc := steerAddDescription
	defer func() {
		goalsFile = oldFile
		steerAddSteer = oldSteer
		steerAddDescription = oldDesc
	}()
	goalsFile = goalsPath
	steerAddSteer = "bogus"
	steerAddDescription = "desc"

	err := goalsSteerAddCmd.RunE(goalsSteerAddCmd, []string{"New directive"})
	if err == nil {
		t.Fatal("expected error for invalid steer value")
	}
	if !strings.Contains(err.Error(), "invalid steer value") {
		t.Errorf("error = %q, want 'invalid steer value'", err.Error())
	}
}

func TestSteerAdd_ValidSteers(t *testing.T) {
	for steer := range validSteers {
		t.Run(steer, func(t *testing.T) {
			dir := t.TempDir()
			goalsPath := writeTestGoalsMD(t, dir)

			oldFile := goalsFile
			oldSteer := steerAddSteer
			oldDesc := steerAddDescription
			oldDryRun := dryRun
			defer func() {
				goalsFile = oldFile
				steerAddSteer = oldSteer
				steerAddDescription = oldDesc
				dryRun = oldDryRun
			}()
			goalsFile = goalsPath
			steerAddSteer = steer
			steerAddDescription = "Test description"
			dryRun = true

			err := goalsSteerAddCmd.RunE(goalsSteerAddCmd, []string{"Test directive"})
			if err != nil {
				t.Fatalf("steer add with steer=%q returned error: %v", steer, err)
			}
		})
	}
}

func TestSteerAdd_AppendsDirective(t *testing.T) {
	dir := t.TempDir()
	goalsPath := writeTestGoalsMD(t, dir)

	oldFile := goalsFile
	oldSteer := steerAddSteer
	oldDesc := steerAddDescription
	oldDryRun := dryRun
	oldJSON := output
	defer func() {
		goalsFile = oldFile
		steerAddSteer = oldSteer
		steerAddDescription = oldDesc
		dryRun = oldDryRun
		output = oldJSON
	}()
	goalsFile = goalsPath
	steerAddSteer = "explore"
	steerAddDescription = "Try new things"
	dryRun = false
	output = "table"

	err := goalsSteerAddCmd.RunE(goalsSteerAddCmd, []string{"Experiment more"})
	if err != nil {
		t.Fatalf("steer add returned error: %v", err)
	}

	gf, err := goals.LoadGoals(goalsPath)
	if err != nil {
		t.Fatalf("LoadGoals: %v", err)
	}

	if len(gf.Directives) != 4 {
		t.Fatalf("directives count = %d, want 4", len(gf.Directives))
	}

	last := gf.Directives[3]
	if last.Number != 4 {
		t.Errorf("last directive number = %d, want 4", last.Number)
	}
	if last.Title != "Experiment more" {
		t.Errorf("last directive title = %q, want 'Experiment more'", last.Title)
	}
	if last.Steer != "explore" {
		t.Errorf("last directive steer = %q, want 'explore'", last.Steer)
	}
}

func TestSteerAdd_CobraArgsValidation(t *testing.T) {
	if err := goalsSteerAddCmd.Args(goalsSteerAddCmd, []string{}); err == nil {
		t.Error("expected error for 0 args")
	}
	if err := goalsSteerAddCmd.Args(goalsSteerAddCmd, []string{"title"}); err != nil {
		t.Errorf("unexpected error for 1 arg: %v", err)
	}
	if err := goalsSteerAddCmd.Args(goalsSteerAddCmd, []string{"a", "b"}); err == nil {
		t.Error("expected error for 2 args")
	}
}

func TestSteerAdd_RequiresMarkdownFormat(t *testing.T) {
	dir := t.TempDir()

	yaml := `version: 2
mission: Test
goals:
  - id: g1
    description: Goal
    check: "exit 0"
    weight: 5
`
	goalsPath := filepath.Join(dir, "GOALS.yaml")
	if err := os.WriteFile(goalsPath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	oldFile := goalsFile
	oldSteer := steerAddSteer
	oldDesc := steerAddDescription
	defer func() {
		goalsFile = oldFile
		steerAddSteer = oldSteer
		steerAddDescription = oldDesc
	}()
	goalsFile = goalsPath
	steerAddSteer = "increase"
	steerAddDescription = "desc"

	err := goalsSteerAddCmd.RunE(goalsSteerAddCmd, []string{"New directive"})
	if err == nil {
		t.Fatal("expected error for YAML format")
	}
	if !strings.Contains(err.Error(), "GOALS.md format") {
		t.Errorf("error = %q, want mention of GOALS.md format", err.Error())
	}
}

// --- steer remove ---

func TestSteerRemove_RemovesAndRenumbers(t *testing.T) {
	dir := t.TempDir()
	goalsPath := writeTestGoalsMD(t, dir)

	oldFile := goalsFile
	oldDryRun := dryRun
	oldJSON := output
	defer func() {
		goalsFile = oldFile
		dryRun = oldDryRun
		output = oldJSON
	}()
	goalsFile = goalsPath
	dryRun = false
	output = "table"

	// Remove directive #2 ("Stay secure")
	err := goalsSteerRemoveCmd.RunE(goalsSteerRemoveCmd, []string{"2"})
	if err != nil {
		t.Fatalf("steer remove returned error: %v", err)
	}

	gf, err := goals.LoadGoals(goalsPath)
	if err != nil {
		t.Fatalf("LoadGoals: %v", err)
	}

	if len(gf.Directives) != 2 {
		t.Fatalf("directives count = %d, want 2", len(gf.Directives))
	}

	// Should be renumbered: Ship fast = 1, Reduce debt = 2
	if gf.Directives[0].Number != 1 || gf.Directives[0].Title != "Ship fast" {
		t.Errorf("directive 0: number=%d title=%q, want 1/Ship fast", gf.Directives[0].Number, gf.Directives[0].Title)
	}
	if gf.Directives[1].Number != 2 || gf.Directives[1].Title != "Reduce debt" {
		t.Errorf("directive 1: number=%d title=%q, want 2/Reduce debt", gf.Directives[1].Number, gf.Directives[1].Title)
	}
}

func TestSteerRemove_NotFound(t *testing.T) {
	dir := t.TempDir()
	goalsPath := writeTestGoalsMD(t, dir)

	oldFile := goalsFile
	defer func() { goalsFile = oldFile }()
	goalsFile = goalsPath

	err := goalsSteerRemoveCmd.RunE(goalsSteerRemoveCmd, []string{"99"})
	if err == nil {
		t.Fatal("expected error for nonexistent directive")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err.Error())
	}
}

func TestSteerRemove_InvalidNumber(t *testing.T) {
	dir := t.TempDir()
	goalsPath := writeTestGoalsMD(t, dir)

	oldFile := goalsFile
	defer func() { goalsFile = oldFile }()
	goalsFile = goalsPath

	err := goalsSteerRemoveCmd.RunE(goalsSteerRemoveCmd, []string{"abc"})
	if err == nil {
		t.Fatal("expected error for non-integer argument")
	}
	if !strings.Contains(err.Error(), "integer") {
		t.Errorf("error = %q, want 'integer'", err.Error())
	}
}

func TestSteerRemove_DryRun(t *testing.T) {
	dir := t.TempDir()
	goalsPath := writeTestGoalsMD(t, dir)

	oldFile := goalsFile
	oldDryRun := dryRun
	oldJSON := output
	defer func() {
		goalsFile = oldFile
		dryRun = oldDryRun
		output = oldJSON
	}()
	goalsFile = goalsPath
	dryRun = true
	output = "table"

	err := goalsSteerRemoveCmd.RunE(goalsSteerRemoveCmd, []string{"1"})
	if err != nil {
		t.Fatalf("steer remove dry-run returned error: %v", err)
	}

	// File should be unchanged
	gf, err := goals.LoadGoals(goalsPath)
	if err != nil {
		t.Fatalf("LoadGoals: %v", err)
	}
	if len(gf.Directives) != 3 {
		t.Errorf("directives count = %d, want 3 (dry-run should not modify)", len(gf.Directives))
	}
}

func TestSteerRemove_CobraArgsValidation(t *testing.T) {
	if err := goalsSteerRemoveCmd.Args(goalsSteerRemoveCmd, []string{}); err == nil {
		t.Error("expected error for 0 args")
	}
	if err := goalsSteerRemoveCmd.Args(goalsSteerRemoveCmd, []string{"1"}); err != nil {
		t.Errorf("unexpected error for 1 arg: %v", err)
	}
}

// --- steer prioritize ---

func TestSteerPrioritize_MovesToNewPosition(t *testing.T) {
	dir := t.TempDir()
	goalsPath := writeTestGoalsMD(t, dir)

	oldFile := goalsFile
	oldDryRun := dryRun
	oldJSON := output
	defer func() {
		goalsFile = oldFile
		dryRun = oldDryRun
		output = oldJSON
	}()
	goalsFile = goalsPath
	dryRun = false
	output = "table"

	// Move directive #3 ("Reduce debt") to position 1
	err := goalsSteerPrioritizeCmd.RunE(goalsSteerPrioritizeCmd, []string{"3", "1"})
	if err != nil {
		t.Fatalf("steer prioritize returned error: %v", err)
	}

	gf, err := goals.LoadGoals(goalsPath)
	if err != nil {
		t.Fatalf("LoadGoals: %v", err)
	}

	if len(gf.Directives) != 3 {
		t.Fatalf("directives count = %d, want 3", len(gf.Directives))
	}

	// Order should be: Reduce debt (1), Ship fast (2), Stay secure (3)
	if gf.Directives[0].Title != "Reduce debt" {
		t.Errorf("directive 0 = %q, want 'Reduce debt'", gf.Directives[0].Title)
	}
	if gf.Directives[1].Title != "Ship fast" {
		t.Errorf("directive 1 = %q, want 'Ship fast'", gf.Directives[1].Title)
	}
	if gf.Directives[2].Title != "Stay secure" {
		t.Errorf("directive 2 = %q, want 'Stay secure'", gf.Directives[2].Title)
	}

	// Verify renumbering
	for i, d := range gf.Directives {
		if d.Number != i+1 {
			t.Errorf("directive %d has number %d, want %d", i, d.Number, i+1)
		}
	}
}

func TestSteerPrioritize_NotFound(t *testing.T) {
	dir := t.TempDir()
	goalsPath := writeTestGoalsMD(t, dir)

	oldFile := goalsFile
	defer func() { goalsFile = oldFile }()
	goalsFile = goalsPath

	err := goalsSteerPrioritizeCmd.RunE(goalsSteerPrioritizeCmd, []string{"99", "1"})
	if err == nil {
		t.Fatal("expected error for nonexistent directive")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err.Error())
	}
}

func TestSteerPrioritize_InvalidPosition(t *testing.T) {
	dir := t.TempDir()
	goalsPath := writeTestGoalsMD(t, dir)

	oldFile := goalsFile
	defer func() { goalsFile = oldFile }()
	goalsFile = goalsPath

	tests := []struct {
		name    string
		pos     string
		wantErr string
	}{
		{"zero position", "0", "must be between"},
		{"negative position", "-1", "must be between"},
		{"too high position", "99", "must be between"},
		{"non-integer", "abc", "integer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := goalsSteerPrioritizeCmd.RunE(goalsSteerPrioritizeCmd, []string{"1", tt.pos})
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestSteerPrioritize_InvalidSourceNumber(t *testing.T) {
	dir := t.TempDir()
	goalsPath := writeTestGoalsMD(t, dir)

	oldFile := goalsFile
	defer func() { goalsFile = oldFile }()
	goalsFile = goalsPath

	err := goalsSteerPrioritizeCmd.RunE(goalsSteerPrioritizeCmd, []string{"xyz", "1"})
	if err == nil {
		t.Fatal("expected error for non-integer directive number")
	}
	if !strings.Contains(err.Error(), "integer") {
		t.Errorf("error = %q, want 'integer'", err.Error())
	}
}

func TestSteerPrioritize_EmptyDirectives(t *testing.T) {
	dir := t.TempDir()

	md := `# Goals

Mission.

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| gate-one | ` + "`exit 0`" + ` | 5 | Gate one |
`
	goalsPath := filepath.Join(dir, "GOALS.md")
	if err := os.WriteFile(goalsPath, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	oldFile := goalsFile
	defer func() { goalsFile = oldFile }()
	goalsFile = goalsPath

	err := goalsSteerPrioritizeCmd.RunE(goalsSteerPrioritizeCmd, []string{"1", "1"})
	if err == nil {
		t.Fatal("expected error for empty directives")
	}
	if !strings.Contains(err.Error(), "no directives") {
		t.Errorf("error = %q, want 'no directives'", err.Error())
	}
}

func TestSteerPrioritize_DryRun(t *testing.T) {
	dir := t.TempDir()
	goalsPath := writeTestGoalsMD(t, dir)

	oldFile := goalsFile
	oldDryRun := dryRun
	oldJSON := output
	defer func() {
		goalsFile = oldFile
		dryRun = oldDryRun
		output = oldJSON
	}()
	goalsFile = goalsPath
	dryRun = true
	output = "table"

	err := goalsSteerPrioritizeCmd.RunE(goalsSteerPrioritizeCmd, []string{"3", "1"})
	if err != nil {
		t.Fatalf("steer prioritize dry-run returned error: %v", err)
	}

	// Verify file unchanged
	gf, err := goals.LoadGoals(goalsPath)
	if err != nil {
		t.Fatalf("LoadGoals: %v", err)
	}
	if gf.Directives[0].Title != "Ship fast" {
		t.Errorf("first directive = %q, want 'Ship fast' (dry-run should not modify)", gf.Directives[0].Title)
	}
}

func TestSteerPrioritize_CobraArgsValidation(t *testing.T) {
	if err := goalsSteerPrioritizeCmd.Args(goalsSteerPrioritizeCmd, []string{}); err == nil {
		t.Error("expected error for 0 args")
	}
	if err := goalsSteerPrioritizeCmd.Args(goalsSteerPrioritizeCmd, []string{"1"}); err == nil {
		t.Error("expected error for 1 arg")
	}
	if err := goalsSteerPrioritizeCmd.Args(goalsSteerPrioritizeCmd, []string{"1", "2"}); err != nil {
		t.Errorf("unexpected error for 2 args: %v", err)
	}
}

// --- loadMDGoals / writeMDGoals ---

func TestLoadMDGoals_RejectsYAML(t *testing.T) {
	dir := t.TempDir()

	yaml := `version: 2
mission: Test
goals:
  - id: g1
    description: Goal
    check: "exit 0"
    weight: 5
`
	goalsPath := filepath.Join(dir, "GOALS.yaml")
	if err := os.WriteFile(goalsPath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	oldFile := goalsFile
	defer func() { goalsFile = oldFile }()
	goalsFile = goalsPath

	_, _, err := loadMDGoals()
	if err == nil {
		t.Fatal("expected error for YAML file")
	}
	if !strings.Contains(err.Error(), "GOALS.md format") {
		t.Errorf("error = %q, want 'GOALS.md format'", err.Error())
	}
}

func TestLoadMDGoals_AcceptsMD(t *testing.T) {
	dir := t.TempDir()
	goalsPath := writeTestGoalsMD(t, dir)

	oldFile := goalsFile
	defer func() { goalsFile = oldFile }()
	goalsFile = goalsPath

	gf, path, err := loadMDGoals()
	if err != nil {
		t.Fatalf("loadMDGoals returned error: %v", err)
	}
	if gf == nil {
		t.Fatal("GoalFile is nil")
	}
	if path == "" {
		t.Error("resolved path is empty")
	}
	if gf.Format != "md" {
		t.Errorf("Format = %q, want md", gf.Format)
	}
}

func TestWriteMDGoals_WritesFile(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "GOALS.md")

	gf := &goals.GoalFile{
		Version:    4,
		Format:     "md",
		Mission:    "Test mission",
		Directives: []goals.Directive{{Number: 1, Title: "D1", Steer: "increase"}},
		Goals:      []goals.Goal{{ID: "g1", Check: "exit 0", Weight: 5, Description: "Goal"}},
	}

	err := writeMDGoals(gf, outPath)
	if err != nil {
		t.Fatalf("writeMDGoals returned error: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("could not read written file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "Test mission") {
		t.Error("written file missing mission")
	}
	if !strings.Contains(content, "D1") {
		t.Error("written file missing directive")
	}
	if !strings.Contains(content, "g1") {
		t.Error("written file missing goal")
	}
}

func TestWriteMDGoals_FixesExtension(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "GOALS.yaml")

	gf := &goals.GoalFile{
		Version: 4,
		Format:  "md",
		Mission: "Test",
	}

	err := writeMDGoals(gf, yamlPath)
	if err != nil {
		t.Fatalf("writeMDGoals returned error: %v", err)
	}

	// Should have written to GOALS.md, not GOALS.yaml
	mdPath := filepath.Join(dir, "GOALS.md")
	if _, err := os.Stat(mdPath); os.IsNotExist(err) {
		t.Error("expected GOALS.md to be created when given .yaml path")
	}
}
