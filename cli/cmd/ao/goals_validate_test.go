// practices: [dora-metrics, lean-startup]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoalsValidate_ValidFile(t *testing.T) {
	dir := t.TempDir()

	md := `# Goals

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
| build-ok | ` + "`echo build`" + ` | 5 | Build passes |
`
	goalsPath := filepath.Join(dir, "GOALS.md")
	if err := os.WriteFile(goalsPath, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	oldFile := goalsFile
	oldJSON := goalsJSON
	defer func() {
		goalsFile = oldFile
		goalsJSON = oldJSON
	}()
	goalsFile = goalsPath
	goalsJSON = false

	// Redirect stdout to avoid test noise
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	err := goalsValidateCmd.RunE(goalsValidateCmd, nil)

	_ = w.Close()
	os.Stdout = oldStdout
	_ = r.Close()

	if err != nil {
		t.Fatalf("validate returned error for valid file: %v", err)
	}
}

func TestGoalsValidate_InvalidFile_MissingFields(t *testing.T) {
	dir := t.TempDir()

	// Goal with empty check and bad weight
	md := `# Goals

Mission.

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| bad-goal | ` + "``" + ` | 0 | |
`
	goalsPath := filepath.Join(dir, "GOALS.md")
	if err := os.WriteFile(goalsPath, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	oldFile := goalsFile
	oldJSON := goalsJSON
	defer func() {
		goalsFile = oldFile
		goalsJSON = oldJSON
	}()
	goalsFile = goalsPath
	goalsJSON = false

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	err := goalsValidateCmd.RunE(goalsValidateCmd, nil)

	_ = w.Close()
	os.Stdout = oldStdout
	_ = r.Close()

	if err == nil {
		t.Fatal("expected error for invalid goals file")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("error = %q, want 'validation failed'", err.Error())
	}
}

func TestGoalsValidate_JSONOutput_Valid(t *testing.T) {
	dir := t.TempDir()

	md := `# Goals

Mission statement.

## Directives

### 1. First

Body.

**Steer:** increase

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
	oldJSON := goalsJSON
	defer func() {
		goalsFile = oldFile
		goalsJSON = oldJSON
	}()
	goalsFile = goalsPath
	goalsJSON = true

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	err := goalsValidateCmd.RunE(goalsValidateCmd, nil)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)

	var result validateResult
	if err := json.Unmarshal(buf[:n], &result); err != nil {
		t.Fatalf("failed to decode JSON: %v (raw: %s)", err, string(buf[:n]))
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

func TestGoalsValidate_WarningsForEmptyMission(t *testing.T) {
	dir := t.TempDir()

	md := `# Goals

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
	oldJSON := goalsJSON
	defer func() {
		goalsFile = oldFile
		goalsJSON = oldJSON
	}()
	goalsFile = goalsPath
	goalsJSON = true

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	err := goalsValidateCmd.RunE(goalsValidateCmd, nil)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)

	var result validateResult
	if err := json.Unmarshal(buf[:n], &result); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if len(result.Warnings) == 0 {
		t.Error("expected warnings for empty mission and no directives")
	}

	hasEmptyMission := false
	hasNoDirectives := false
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

func TestGoalsValidate_WarningsForMissingSteer(t *testing.T) {
	dir := t.TempDir()

	md := `# Goals

Mission.

## Directives

### 1. No steer directive

Body text without steer line.

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
	oldJSON := goalsJSON
	defer func() {
		goalsFile = oldFile
		goalsJSON = oldJSON
	}()
	goalsFile = goalsPath
	goalsJSON = true

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	err := goalsValidateCmd.RunE(goalsValidateCmd, nil)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)

	var result validateResult
	if err := json.Unmarshal(buf[:n], &result); err != nil {
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

func TestGoalsValidate_WiringCheckMissingScript(t *testing.T) {
	dir := t.TempDir()

	md := `# Goals

Mission.

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| missing-script-gate | ` + "`scripts/nonexistent.sh`" + ` | 5 | Missing script |
`
	goalsPath := filepath.Join(dir, "GOALS.md")
	if err := os.WriteFile(goalsPath, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Chdir(dir)

	oldFile := goalsFile
	oldJSON := goalsJSON
	defer func() {
		goalsFile = oldFile
		goalsJSON = oldJSON
	}()
	goalsFile = goalsPath
	goalsJSON = true

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	err := goalsValidateCmd.RunE(goalsValidateCmd, nil)

	_ = w.Close()
	os.Stdout = oldStdout

	// Should fail validation due to missing script
	if err == nil {
		// Read the JSON output to check for errors
		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		var result validateResult
		if jsonErr := json.Unmarshal(buf[:n], &result); jsonErr == nil {
			if len(result.Errors) > 0 {
				// Error is in the result, which is expected
				return
			}
		}
		t.Fatal("expected validation to fail or report error for missing script")
	}
}

func TestGoalsValidate_MissingGoalsFile(t *testing.T) {
	dir := t.TempDir()

	oldFile := goalsFile
	oldJSON := goalsJSON
	defer func() {
		goalsFile = oldFile
		goalsJSON = oldJSON
	}()
	goalsFile = filepath.Join(dir, "GOALS.md") // does not exist
	goalsJSON = true

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	err := goalsValidateCmd.RunE(goalsValidateCmd, nil)

	_ = w.Close()
	os.Stdout = oldStdout

	// outputValidateResult returns the error encoded in the result
	// Whether it errors or outputs JSON with errors is acceptable
	if err != nil {
		return // Error is expected
	}

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	var result validateResult
	if jsonErr := json.Unmarshal(buf[:n], &result); jsonErr == nil {
		if !result.Valid {
			return // Expected invalid result
		}
	}
	t.Fatal("expected error or invalid result for missing goals file")
}

func TestGoalsValidate_CmdAttributes(t *testing.T) {
	if goalsValidateCmd.Use != "validate" {
		t.Errorf("Use = %q, want validate", goalsValidateCmd.Use)
	}
	if goalsValidateCmd.GroupID != "measurement" {
		t.Errorf("GroupID = %q, want measurement", goalsValidateCmd.GroupID)
	}
	found := false
	for _, a := range goalsValidateCmd.Aliases {
		if a == "v" {
			found = true
		}
	}
	if !found {
		t.Error("expected alias 'v' for validate command")
	}
}

func TestValidateResult_Struct(t *testing.T) {
	result := validateResult{
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

	var decoded validateResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if decoded.Valid != true {
		t.Error("Valid should be true")
	}
	if decoded.GoalCount != 5 {
		t.Errorf("GoalCount = %d, want 5", decoded.GoalCount)
	}
	if decoded.Directives != 2 {
		t.Errorf("Directives = %d, want 2", decoded.Directives)
	}
	if len(decoded.Warnings) != 1 {
		t.Errorf("Warnings count = %d, want 1", len(decoded.Warnings))
	}
	if len(decoded.Errors) != 0 {
		t.Errorf("Errors count = %d, want 0", len(decoded.Errors))
	}
}
