package goals

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/boshu2/agentops/cli/internal/clicontract"

	goalsapp "github.com/boshu2/agentops/cli/internal/goals"
)

// newTestModule builds the goals module with a fixed output mode, constructing
// the command tree directly instead of mutating any package-global state.
func newTestModule(outputMode string) Module {
	return NewModule(clicontract.HostOptions{
		OutputMode: func() string { return outputMode },
		Verbose:    func() bool { return false },
		ProjectRoot: func() string {
			if wd, err := os.Getwd(); err == nil {
				return wd
			}
			return "."
		},
	})
}

// runGoals executes the goals command tree with args, capturing stdout+stderr
// in the returned string.
func runGoals(t *testing.T, outputMode string, args ...string) (string, error) {
	t.Helper()
	cmd := newTestModule(outputMode).Command()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func subcommand(t *testing.T, name string) *cobra.Command {
	t.Helper()
	for _, c := range newTestModule("table").Command().Commands() {
		if c.Name() == name {
			return c
		}
	}
	t.Fatalf("subcommand %q not found", name)
	return nil
}

func TestModule_CommandExists(t *testing.T) {
	command := newTestModule("table").Command()
	if command.Use != "goals" {
		t.Errorf("Use = %q, want goals", command.Use)
	}
	if command.GroupID != "workflow" {
		t.Errorf("GroupID = %q, want workflow", command.GroupID)
	}
}

func TestModule_HasExpectedSubcommands(t *testing.T) {
	subNames := map[string]bool{}
	for _, sub := range newTestModule("table").Command().Commands() {
		subNames[sub.Name()] = true
	}
	for _, name := range []string{"measure", "validate", "drift", "history", "export", "meta", "render", "scenarios"} {
		if !subNames[name] {
			t.Errorf("missing expected subcommand %q", name)
		}
	}
}

func TestModule_HasGroups(t *testing.T) {
	ids := map[string]bool{}
	for _, g := range newTestModule("table").Command().Groups() {
		ids[g.ID] = true
	}
	for _, want := range []string{"measurement", "analysis"} {
		if !ids[want] {
			t.Errorf("missing group %q", want)
		}
	}
}

func TestModule_PersistentFlags(t *testing.T) {
	flags := newTestModule("table").Command().PersistentFlags()
	for _, name := range []string{"file", "timeout"} {
		if flags.Lookup(name) == nil {
			t.Errorf("missing persistent flag %q", name)
		}
	}
	// goals reads the global -o/--output flag, not a local --json bool.
	if flags.Lookup("json") != nil {
		t.Error("goals must not register a local --json flag")
	}
}

func TestModule_DefaultTimeoutCoversRepoRaceGate(t *testing.T) {
	flag := newTestModule("table").Command().PersistentFlags().Lookup("timeout")
	if flag == nil {
		t.Fatal("missing persistent flag \"timeout\"")
	}
	if flag.DefValue != "240" {
		t.Fatalf("timeout default = %q, want 240", flag.DefValue)
	}
	if defaultGoalsTimeoutSeconds != 240 {
		t.Fatalf("defaultGoalsTimeoutSeconds = %d, want 240", defaultGoalsTimeoutSeconds)
	}
}

func TestModule_MeasureTotalTimeoutFlag(t *testing.T) {
	flag := subcommand(t, "measure").Flags().Lookup("total-timeout")
	if flag == nil {
		t.Fatal("missing measure flag \"total-timeout\"")
	}
	if flag.DefValue != "0" {
		t.Fatalf("total-timeout default = %q, want 0", flag.DefValue)
	}
}

func TestModule_JSONOutputSeam(t *testing.T) {
	if !newTestModule("json").jsonOutput() {
		t.Error("jsonOutput() = false with OutputMode json, want true")
	}
	if newTestModule("table").jsonOutput() {
		t.Error("jsonOutput() = true with OutputMode table, want false")
	}
}

func TestModule_ValidateCmdAttributes(t *testing.T) {
	validate := subcommand(t, "validate")
	if validate.Use != "validate" {
		t.Errorf("Use = %q, want validate", validate.Use)
	}
	if validate.GroupID != "measurement" {
		t.Errorf("GroupID = %q, want measurement", validate.GroupID)
	}
	found := false
	for _, a := range validate.Aliases {
		if a == "v" {
			found = true
		}
	}
	if !found {
		t.Error("expected alias 'v' for validate command")
	}
}

func writeGoals(t *testing.T, md string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "GOALS.md")
	if err := os.WriteFile(path, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestModule_Validate_ValidFile(t *testing.T) {
	path := writeGoals(t, `# Goals

Ship reliable software.

## North Stars

- All checks pass

## Directives

### 1. Establish baseline

Set up quality gates.

**Steer:** increase

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| build-ok | `+"`echo build`"+` | 5 | Build passes |
`)
	if _, err := runGoals(t, "table", "validate", "--file", path); err != nil {
		t.Fatalf("validate returned error for valid file: %v", err)
	}
}

func TestModule_Validate_InvalidFile_MissingFields(t *testing.T) {
	path := writeGoals(t, `# Goals

Mission.

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| bad-goal | `+"``"+` | 0 | |
`)
	_, err := runGoals(t, "table", "validate", "--file", path)
	if err == nil {
		t.Fatal("expected error for invalid goals file")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("error = %q, want 'validation failed'", err.Error())
	}
}

func TestModule_Validate_JSONOutput_Valid(t *testing.T) {
	path := writeGoals(t, `# Goals

Mission statement.

## Directives

### 1. First

Body.

**Steer:** increase

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| gate-one | `+"`exit 0`"+` | 5 | Gate one |
`)
	out, err := runGoals(t, "json", "validate", "--file", path)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	var result goalsapp.ValidateResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to decode JSON: %v (raw: %s)", err, out)
	}
	if !result.Valid {
		t.Error("expected Valid=true")
	}
	if result.GoalCount != 1 {
		t.Errorf("GoalCount = %d, want 1", result.GoalCount)
	}
	if result.Version != 4 {
		t.Errorf("Version = %d, want 4", result.Version)
	}
	if result.Format != "md" {
		t.Errorf("Format = %q, want md", result.Format)
	}
	if result.Directives != 1 {
		t.Errorf("Directives = %d, want 1", result.Directives)
	}
}

func TestModule_Validate_WarningsForEmptyMission(t *testing.T) {
	path := writeGoals(t, `# Goals

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| gate-one | `+"`exit 0`"+` | 5 | Gate one |
`)
	out, err := runGoals(t, "json", "validate", "--file", path)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	var result goalsapp.ValidateResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	hasEmptyMission, hasNoDirectives := false, false
	for _, w := range result.Warnings {
		if strings.Contains(w, "empty mission") {
			hasEmptyMission = true
		}
		if strings.Contains(w, "no directives") {
			hasNoDirectives = true
		}
	}
	if !hasEmptyMission {
		t.Error("expected 'empty mission' warning")
	}
	if !hasNoDirectives {
		t.Error("expected 'no directives' warning")
	}
}

func TestModule_Validate_WarningsForMissingSteer(t *testing.T) {
	path := writeGoals(t, `# Goals

Mission.

## Directives

### 1. No steer directive

Body text without steer line.

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| gate-one | `+"`exit 0`"+` | 5 | Gate one |
`)
	out, err := runGoals(t, "json", "validate", "--file", path)
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	var result goalsapp.ValidateResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	hasMissingSteer := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "missing steer") {
			hasMissingSteer = true
		}
	}
	if !hasMissingSteer {
		t.Error("expected 'missing steer' warning for directive without steer")
	}
}

func TestModule_Validate_WiringCheckMissingScript(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "GOALS.md")
	if err := os.WriteFile(path, []byte(`# Goals

Mission.

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| missing-script-gate | `+"`scripts/nonexistent.sh`"+` | 5 | Missing script |
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	out, err := runGoals(t, "json", "validate", "--file", path)
	if err == nil {
		var result goalsapp.ValidateResult
		if jsonErr := json.Unmarshal([]byte(out), &result); jsonErr == nil && len(result.Errors) > 0 {
			return
		}
		t.Fatal("expected validation to fail or report error for missing script")
	}
}

func TestModule_Validate_MissingGoalsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GOALS.md") // does not exist
	out, err := runGoals(t, "json", "validate", "--file", path)
	if err != nil {
		return // Error is expected.
	}
	var result goalsapp.ValidateResult
	if jsonErr := json.Unmarshal([]byte(out), &result); jsonErr == nil && !result.Valid {
		return // Expected invalid result.
	}
	t.Fatal("expected error or invalid result for missing goals file")
}

func TestValidateResult_Struct(t *testing.T) {
	result := goalsapp.ValidateResult{
		Valid:      true,
		GoalCount:  5,
		Version:    4,
		Format:     "md",
		Directives: 2,
		Errors:     nil,
		Warnings:   []string{"warn1"},
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var decoded goalsapp.ValidateResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if decoded.Valid != true || decoded.GoalCount != 5 || decoded.Directives != 2 {
		t.Errorf("decoded = %+v, want Valid/GoalCount=5/Directives=2", decoded)
	}
	if len(decoded.Warnings) != 1 || len(decoded.Errors) != 0 {
		t.Errorf("warnings=%d errors=%d, want 1/0", len(decoded.Warnings), len(decoded.Errors))
	}
}

func TestModule_Integration_MeasureNoGoalsFile(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := runGoals(t, "table", "measure"); err == nil {
		t.Fatal("expected error when no goals file exists, got nil")
	}
}

func TestModule_Integration_MeasureDirectivesJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "GOALS.md"), []byte(`# Fitness Goals

## Mission

Measure project fitness.

## North Stars

- Passing checks

## Anti-Stars

- Hidden regressions

## Directives

### 1. Establish baseline

Keep the deterministic floor green.

**Steer:** increase

## Gates
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	out, err := runGoals(t, "table", "measure", "--directives")
	if err != nil {
		t.Fatalf("goals measure --directives failed: %v", err)
	}
	if !strings.Contains(out, "Establish baseline") {
		t.Errorf("expected directive title in JSON output, got: %s", out)
	}
}
