package goals_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/goals"
)

// --- TestGoalsValidate_ValidMD ---

func TestGoalsValidate_ValidMD(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	md := `# Goals

Ship reliable software.

## North Stars

- All checks pass on every commit

## Anti Stars

- Shipping without tests

## Directives

### 1. Establish baseline

Set up initial quality gates.

**Steer:** increase

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| build-ok | ` + "`echo build`" + ` | 5 | Build passes |
| test-ok | ` + "`echo test`" + ` | 5 | Tests pass |
`

	goalsPath := filepath.Join(dir, "GOALS.md")
	if err := os.WriteFile(goalsPath, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	gf, err := goals.LoadGoals(goalsPath)
	if err != nil {
		t.Fatalf("LoadGoals returned error: %v", err)
	}

	if gf.Version != 4 {
		t.Errorf("Version = %d, want 4", gf.Version)
	}
	if gf.Format != "md" {
		t.Errorf("Format = %q, want md", gf.Format)
	}

	errs := goals.ValidateGoals(gf)
	if len(errs) != 0 {
		t.Errorf("expected 0 validation errors, got %d: %v", len(errs), errs)
	}

	if len(gf.Goals) != 2 {
		t.Fatalf("expected 2 goals, got %d", len(gf.Goals))
	}
	if gf.Goals[0].ID != "build-ok" {
		t.Errorf("Goal[0].ID = %q, want build-ok", gf.Goals[0].ID)
	}
	if gf.Goals[1].ID != "test-ok" {
		t.Errorf("Goal[1].ID = %q, want test-ok", gf.Goals[1].ID)
	}
}

// --- TestGoalsValidate_InvalidFormat ---

func TestGoalsValidate_InvalidFormat(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	t.Run("empty file", func(t *testing.T) {
		t.Parallel()
		p := filepath.Join(dir, "empty.md")
		if err := os.WriteFile(p, []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := goals.LoadGoals(p)
		if err == nil {
			t.Error("expected error for empty goals file")
		}
	})

	t.Run("yaml_bad_version", func(t *testing.T) {
		t.Parallel()
		p := filepath.Join(dir, "bad_version.yaml")
		content := "version: 99\ngoals:\n  - id: foo\n    description: d\n    check: echo ok\n    weight: 5\n"
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := goals.LoadGoals(p)
		if err == nil {
			t.Error("expected error for unsupported YAML version")
		}
	})

	t.Run("malformed_yaml", func(t *testing.T) {
		t.Parallel()
		p := filepath.Join(dir, "malformed.yaml")
		content := "version: [\nbroken yaml\n"
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := goals.LoadGoals(p)
		if err == nil {
			t.Error("expected error for malformed YAML")
		}
	})

	t.Run("goals_with_missing_fields", func(t *testing.T) {
		t.Parallel()
		gf := &goals.GoalFile{
			Version: 2,
			Goals: []goals.Goal{
				{}, // all fields missing
			},
		}
		errs := goals.ValidateGoals(gf)
		if len(errs) == 0 {
			t.Error("expected validation errors for empty goal, got none")
		}
		// Should have errors for: id, description, check, weight
		fields := map[string]bool{}
		for _, e := range errs {
			fields[e.Field] = true
		}
		for _, required := range []string{"id", "description", "check", "weight"} {
			if !fields[required] {
				t.Errorf("expected validation error for field %q", required)
			}
		}
	})
}

// --- TestGoalsValidate_DirectiveCounting ---

func TestGoalsValidate_DirectiveCounting(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		md             string
		wantDirectives int
		wantGoals      int
	}{
		{
			name: "zero directives",
			md: `# Goals

Mission statement.

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| g1 | ` + "`echo ok`" + ` | 5 | Goal one |
`,
			wantDirectives: 0,
			wantGoals:      1,
		},
		{
			name: "one directive",
			md: `# Goals

Mission.

## Directives

### 1. First directive

Body text.

**Steer:** increase

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| g1 | ` + "`echo ok`" + ` | 5 | Goal one |
`,
			wantDirectives: 1,
			wantGoals:      1,
		},
		{
			name: "three directives",
			md: `# Goals

Mission.

## Directives

### 1. First

Body one.

**Steer:** increase

### 2. Second

Body two.

**Steer:** decrease

### 3. Third

Body three.

**Steer:** maintain

## Gates

| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| g1 | ` + "`echo ok`" + ` | 5 | Goal one |
`,
			wantDirectives: 3,
			wantGoals:      1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			p := filepath.Join(dir, "GOALS.md")
			if err := os.WriteFile(p, []byte(tt.md), 0o644); err != nil {
				t.Fatal(err)
			}

			gf, err := goals.LoadGoals(p)
			if err != nil {
				t.Fatalf("LoadGoals: %v", err)
			}

			if len(gf.Directives) != tt.wantDirectives {
				t.Errorf("directives = %d, want %d", len(gf.Directives), tt.wantDirectives)
			}
			if len(gf.Goals) != tt.wantGoals {
				t.Errorf("goals = %d, want %d", len(gf.Goals), tt.wantGoals)
			}
		})
	}
}

// --- TestGoalsDrift_DetectsChange ---

func TestGoalsDrift_DetectsChange(t *testing.T) {
	t.Parallel()
	baseline := &goals.Snapshot{
		Goals: []goals.Measurement{
			{GoalID: "stable-goal", Result: "pass", Weight: 5},
			{GoalID: "will-regress", Result: "pass", Weight: 8},
			{GoalID: "will-improve", Result: "fail", Weight: 3},
		},
	}
	current := &goals.Snapshot{
		Goals: []goals.Measurement{
			{GoalID: "stable-goal", Result: "pass", Weight: 5},
			{GoalID: "will-regress", Result: "fail", Weight: 8},
			{GoalID: "will-improve", Result: "pass", Weight: 3},
		},
	}

	drifts := goals.ComputeDrift(baseline, current)
	if len(drifts) != 3 {
		t.Fatalf("expected 3 drift results, got %d", len(drifts))
	}

	// Results should be sorted: regressions first, then improvements, then unchanged.
	if drifts[0].Delta != "regressed" {
		t.Errorf("drifts[0].Delta = %q, want regressed", drifts[0].Delta)
	}
	if drifts[0].GoalID != "will-regress" {
		t.Errorf("drifts[0].GoalID = %q, want will-regress", drifts[0].GoalID)
	}

	if drifts[1].Delta != "improved" {
		t.Errorf("drifts[1].Delta = %q, want improved", drifts[1].Delta)
	}
	if drifts[1].GoalID != "will-improve" {
		t.Errorf("drifts[1].GoalID = %q, want will-improve", drifts[1].GoalID)
	}

	if drifts[2].Delta != "unchanged" {
		t.Errorf("drifts[2].Delta = %q, want unchanged", drifts[2].Delta)
	}
	if drifts[2].GoalID != "stable-goal" {
		t.Errorf("drifts[2].GoalID = %q, want stable-goal", drifts[2].GoalID)
	}
}

func TestGoalsDrift_NewGoalShowsAsNew(t *testing.T) {
	t.Parallel()
	baseline := &goals.Snapshot{
		Goals: []goals.Measurement{},
	}
	current := &goals.Snapshot{
		Goals: []goals.Measurement{
			{GoalID: "brand-new", Result: "pass", Weight: 5},
		},
	}

	drifts := goals.ComputeDrift(baseline, current)
	if len(drifts) != 1 {
		t.Fatalf("expected 1 drift result, got %d", len(drifts))
	}
	if drifts[0].Before != "new" {
		t.Errorf("Before = %q, want new", drifts[0].Before)
	}
	if drifts[0].Delta != "unchanged" {
		t.Errorf("Delta = %q, want unchanged (new goals are unchanged)", drifts[0].Delta)
	}
}

func TestGoalsDrift_ValueDeltaComputed(t *testing.T) {
	t.Parallel()
	baseVal := 2.5
	curVal := 4.0
	baseline := &goals.Snapshot{
		Goals: []goals.Measurement{
			{GoalID: "metric-g", Result: "pass", Weight: 1, Value: &baseVal},
		},
	}
	current := &goals.Snapshot{
		Goals: []goals.Measurement{
			{GoalID: "metric-g", Result: "pass", Weight: 1, Value: &curVal},
		},
	}

	drifts := goals.ComputeDrift(baseline, current)
	if len(drifts) != 1 {
		t.Fatalf("expected 1 drift result, got %d", len(drifts))
	}
	if drifts[0].ValueDelta == nil {
		t.Fatal("expected ValueDelta to be computed")
	}
	want := 1.5 // 4.0 - 2.5
	if *drifts[0].ValueDelta != want {
		t.Errorf("ValueDelta = %f, want %f", *drifts[0].ValueDelta, want)
	}
}

// --- TestGoalsMeasure_WeightedScoring (subsystem-level) ---

func TestGoalsMeasure_WeightedScoring_Subsystem(t *testing.T) {
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

// --- TestGoalsMeasure_SkippedGoals (subsystem-level) ---

func TestGoalsMeasure_SkippedGoals_Subsystem(t *testing.T) {
	t.Parallel()
	gf := &goals.GoalFile{
		Version: 2,
		Goals: []goals.Goal{
			{ID: "pass-1", Check: "exit 0", Weight: 5, Type: goals.GoalTypeHealth},
			{ID: "timeout-1", Check: "sleep 5", Weight: 10, Type: goals.GoalTypeHealth},
		},
	}
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
