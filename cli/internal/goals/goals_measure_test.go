package goals_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/goals"
)

func TestGoalsMeasure_BasicRun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	md := `# Goals

Mission.

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| pass-gate | ` + "`exit 0`" + ` | 5 | Always passes |
| fail-gate | ` + "`exit 1`" + ` | 3 | Always fails |
`
	goalsPath := filepath.Join(dir, "GOALS.md")
	if err := os.WriteFile(goalsPath, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := goals.RunMeasure(goals.MeasureOptions{
		GoalsFile: goalsPath,
		JSON:      true,
		Timeout:   10 * time.Second,
		Stdout:    &buf,
		SnapDir:   filepath.Join(dir, "baselines"),
	})
	if err != nil {
		t.Fatalf("measure returned error: %v", err)
	}

	var snap goals.Snapshot
	if err := json.Unmarshal(buf.Bytes(), &snap); err != nil {
		t.Fatalf("failed to decode JSON: %v (raw: %s)", err, buf.String())
	}

	if snap.Summary.Total != 2 {
		t.Errorf("Total = %d, want 2", snap.Summary.Total)
	}
	if snap.Summary.Passing != 1 {
		t.Errorf("Passing = %d, want 1", snap.Summary.Passing)
	}
	if snap.Summary.Failing != 1 {
		t.Errorf("Failing = %d, want 1", snap.Summary.Failing)
	}
}

func TestGoalsMeasure_SingleGoalFilter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	md := `# Goals

Mission.

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| target-goal | ` + "`exit 0`" + ` | 5 | Target |
| other-goal | ` + "`exit 0`" + ` | 5 | Other |
`
	goalsPath := filepath.Join(dir, "GOALS.md")
	if err := os.WriteFile(goalsPath, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := goals.RunMeasure(goals.MeasureOptions{
		GoalsFile: goalsPath,
		JSON:      true,
		Timeout:   10 * time.Second,
		GoalID:    "target-goal",
		Stdout:    &buf,
		SnapDir:   filepath.Join(dir, "baselines"),
	})
	if err != nil {
		t.Fatalf("measure returned error: %v", err)
	}

	var snap goals.Snapshot
	if err := json.Unmarshal(buf.Bytes(), &snap); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if snap.Summary.Total != 1 {
		t.Errorf("Total = %d, want 1 (filtered to single goal)", snap.Summary.Total)
	}
	if len(snap.Goals) != 1 || snap.Goals[0].GoalID != "target-goal" {
		t.Errorf("expected only target-goal in results")
	}
}

func TestGoalsMeasure_GoalNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	md := `# Goals

Mission.

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| only-goal | ` + "`exit 0`" + ` | 5 | Only |
`
	goalsPath := filepath.Join(dir, "GOALS.md")
	if err := os.WriteFile(goalsPath, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	err := goals.RunMeasure(goals.MeasureOptions{
		GoalsFile: goalsPath,
		GoalID:    "nonexistent-goal",
		Timeout:   10 * time.Second,
		SnapDir:   filepath.Join(dir, "baselines"),
	})
	if err == nil {
		t.Fatal("expected error for nonexistent goal ID")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err.Error())
	}
}

func TestGoalsMeasure_DirectivesAndGoalConflict(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	md := `# Goals

Mission.

## Directives

### 1. First

Body.

**Steer:** increase

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| g1 | ` + "`exit 0`" + ` | 5 | Goal |
`
	goalsPath := filepath.Join(dir, "GOALS.md")
	if err := os.WriteFile(goalsPath, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	err := goals.RunMeasure(goals.MeasureOptions{
		GoalsFile:  goalsPath,
		GoalID:     "g1",
		Directives: true,
		SnapDir:    filepath.Join(dir, "baselines"),
	})
	if err == nil {
		t.Fatal("expected error when --directives and --goal are both set")
	}
	if !strings.Contains(err.Error(), "cannot be combined") {
		t.Errorf("error = %q, want 'cannot be combined'", err.Error())
	}
}

func TestGoalsMeasure_DirectivesOutput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	md := `# Goals

Mission.

## Directives

### 1. Ship fast

Deploy continuously.

**Steer:** increase

### 2. Stay secure

No vulnerabilities.

**Steer:** hold

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| g1 | ` + "`exit 0`" + ` | 5 | Goal |
`
	goalsPath := filepath.Join(dir, "GOALS.md")
	if err := os.WriteFile(goalsPath, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := goals.RunMeasure(goals.MeasureOptions{
		GoalsFile:  goalsPath,
		Directives: true,
		Stdout:     &buf,
		SnapDir:    filepath.Join(dir, "baselines"),
	})
	if err != nil {
		t.Fatalf("measure --directives returned error: %v", err)
	}

	var dirs []goals.Directive
	if err := json.Unmarshal(buf.Bytes(), &dirs); err != nil {
		t.Fatalf("failed to decode directives JSON: %v", err)
	}

	if len(dirs) != 2 {
		t.Fatalf("expected 2 directives, got %d", len(dirs))
	}
	if dirs[0].Title != "Ship fast" {
		t.Errorf("dirs[0].Title = %q, want 'Ship fast'", dirs[0].Title)
	}
	if dirs[1].Steer != "hold" {
		t.Errorf("dirs[1].Steer = %q, want 'hold'", dirs[1].Steer)
	}
}

func TestGoalsMeasure_DirectivesOnYAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	yaml := `version: 2
mission: Test
goals:
  - id: g1
    description: Goal one
    check: "exit 0"
    weight: 5
`
	goalsPath := filepath.Join(dir, "GOALS.yaml")
	if err := os.WriteFile(goalsPath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	err := goals.RunMeasure(goals.MeasureOptions{
		GoalsFile:  goalsPath,
		Directives: true,
		SnapDir:    filepath.Join(dir, "baselines"),
	})
	if err == nil {
		t.Fatal("expected error for YAML + --directives, got nil")
	}
	if !strings.Contains(err.Error(), "--directives requires GOALS.md format") {
		t.Errorf("error = %q, want '--directives requires GOALS.md format'", err.Error())
	}
}

func TestGoalsMeasure_MissingGoalsFile(t *testing.T) {
	t.Parallel()

	err := goals.RunMeasure(goals.MeasureOptions{
		GoalsFile: "/nonexistent/GOALS.md",
		SnapDir:   t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected error for missing goals file")
	}
}

func TestGoalsMeasure_TableOutput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	md := `# Goals

Mission.

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| pass-gate | ` + "`exit 0`" + ` | 5 | Always passes |
| fail-gate | ` + "`exit 1`" + ` | 3 | Always fails |
`
	goalsPath := filepath.Join(dir, "GOALS.md")
	if err := os.WriteFile(goalsPath, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := goals.RunMeasure(goals.MeasureOptions{
		GoalsFile: goalsPath,
		JSON:      false,
		Timeout:   10 * time.Second,
		Stdout:    &buf,
		SnapDir:   filepath.Join(dir, "baselines"),
	})
	if err != nil {
		t.Fatalf("measure returned error: %v", err)
	}

	output := buf.String()

	// Verify table header
	if !strings.Contains(output, "GOAL") {
		t.Error("table output missing GOAL header")
	}
	if !strings.Contains(output, "RESULT") {
		t.Error("table output missing RESULT header")
	}
	if !strings.Contains(output, "DURATION") {
		t.Error("table output missing DURATION header")
	}
	if !strings.Contains(output, "WEIGHT") {
		t.Error("table output missing WEIGHT header")
	}

	// Verify separator dashes
	if !strings.Contains(output, "----") {
		t.Error("table output missing header separator")
	}

	// Verify goal IDs appear
	if !strings.Contains(output, "pass-gate") {
		t.Error("table output missing pass-gate row")
	}
	if !strings.Contains(output, "fail-gate") {
		t.Error("table output missing fail-gate row")
	}

	// Verify score summary line
	if !strings.Contains(output, "Score:") {
		t.Error("table output missing Score summary")
	}
}

func TestGoalsMeasure_WeightedScoring(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		goals     []goals.Goal
		wantScore float64
		wantPass  int
		wantFail  int
	}{
		{
			name: "all pass equal weight",
			goals: []goals.Goal{
				{ID: "a", Check: "exit 0", Weight: 5, Type: goals.GoalTypeHealth},
				{ID: "b", Check: "exit 0", Weight: 5, Type: goals.GoalTypeHealth},
			},
			wantScore: 100.0,
			wantPass:  2,
			wantFail:  0,
		},
		{
			name: "one pass one fail weighted",
			goals: []goals.Goal{
				{ID: "heavy-pass", Check: "exit 0", Weight: 8, Type: goals.GoalTypeHealth},
				{ID: "light-fail", Check: "exit 1", Weight: 2, Type: goals.GoalTypeHealth},
			},
			wantScore: 80.0,
			wantPass:  1,
			wantFail:  1,
		},
		{
			name: "all fail",
			goals: []goals.Goal{
				{ID: "f1", Check: "exit 1", Weight: 5, Type: goals.GoalTypeHealth},
				{ID: "f2", Check: "exit 1", Weight: 3, Type: goals.GoalTypeHealth},
			},
			wantScore: 0.0,
			wantPass:  0,
			wantFail:  2,
		},
		{
			name:      "empty goals",
			goals:     []goals.Goal{},
			wantScore: 0.0,
			wantPass:  0,
			wantFail:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gf := &goals.GoalFile{Version: 2, Goals: tt.goals}
			snap := goals.Measure(gf, 5*time.Second)

			if snap.Summary.Score != tt.wantScore {
				t.Errorf("Score = %f, want %f", snap.Summary.Score, tt.wantScore)
			}
			if snap.Summary.Passing != tt.wantPass {
				t.Errorf("Passing = %d, want %d", snap.Summary.Passing, tt.wantPass)
			}
			if snap.Summary.Failing != tt.wantFail {
				t.Errorf("Failing = %d, want %d", snap.Summary.Failing, tt.wantFail)
			}
		})
	}
}

func TestGoalsMeasure_SkippedGoals(t *testing.T) {
	t.Parallel()
	gf := &goals.GoalFile{
		Version: 2,
		Goals: []goals.Goal{
			{ID: "pass-1", Check: "exit 0", Weight: 5, Type: goals.GoalTypeHealth},
			{ID: "timeout-1", Check: "sleep 5", Weight: 10, Type: goals.GoalTypeHealth},
		},
	}
	// Timeout remains shorter than the slow goal without racing shell startup.
	snap := goals.Measure(gf, 2*time.Second)

	if snap.Summary.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", snap.Summary.Skipped)
	}
	if snap.Summary.Passing != 1 {
		t.Errorf("Passing = %d, want 1", snap.Summary.Passing)
	}
	if snap.Summary.Score != 100.0 {
		t.Errorf("Score = %f, want 100.0 (skipped excluded from denominator)", snap.Summary.Score)
	}
}
