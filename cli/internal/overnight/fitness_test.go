package overnight

import (
	"errors"
	"reflect"
	"testing"
)

func TestFitnessSnapshot_Delta_AllImproved(t *testing.T) {
	prev := &FitnessSnapshot{Metrics: map[string]float64{
		"a": 0.40,
		"b": 0.50,
		"c": 0.60,
	}}
	curr := FitnessSnapshot{Metrics: map[string]float64{
		"a": 0.50,
		"b": 0.55,
		"c": 0.70,
	}}

	composite, regressions, regressed := curr.Delta(prev, nil, 0.05)

	if regressed {
		t.Fatalf("expected no regression, got %+v", regressions)
	}
	if len(regressions) != 0 {
		t.Fatalf("expected empty regression list, got %+v", regressions)
	}
	want := (0.10 + 0.05 + 0.10) / 3.0
	if !floatsNear(composite, want, 1e-9) {
		t.Fatalf("composite = %v, want %v", composite, want)
	}
}

func TestFitnessSnapshot_Delta_MetricRegressionBeyondFloor(t *testing.T) {
	prev := &FitnessSnapshot{Metrics: map[string]float64{
		"a": 0.70,
		"b": 0.50,
	}}
	curr := FitnessSnapshot{Metrics: map[string]float64{
		"a": 0.60, // drop 0.10 > floor 0.05
		"b": 0.52,
	}}

	composite, regressions, regressed := curr.Delta(prev, nil, 0.05)

	if !regressed {
		t.Fatal("expected regressed=true")
	}
	if len(regressions) != 1 {
		t.Fatalf("expected 1 regression, got %d: %+v", len(regressions), regressions)
	}
	got := regressions[0]
	if got.Name != "a" {
		t.Errorf("Name = %q, want %q", got.Name, "a")
	}
	if !floatsNear(got.Drop, 0.10, 1e-9) {
		t.Errorf("Drop = %v, want %v", got.Drop, 0.10)
	}
	if !floatsNear(got.FloorBreached, 0.05, 1e-9) {
		t.Errorf("FloorBreached = %v, want %v", got.FloorBreached, 0.05)
	}
	if !floatsNear(got.Previous, 0.70, 1e-9) {
		t.Errorf("Previous = %v", got.Previous)
	}
	if !floatsNear(got.Current, 0.60, 1e-9) {
		t.Errorf("Current = %v", got.Current)
	}
	want := (-0.10 + 0.02) / 2.0
	if !floatsNear(composite, want, 1e-9) {
		t.Errorf("composite = %v, want %v", composite, want)
	}
}

func TestFitnessSnapshot_Delta_RegressionUnderFloor(t *testing.T) {
	prev := &FitnessSnapshot{Metrics: map[string]float64{
		"a": 0.50,
	}}
	curr := FitnessSnapshot{Metrics: map[string]float64{
		"a": 0.47, // drop 0.03 < floor 0.05
	}}

	composite, regressions, regressed := curr.Delta(prev, nil, 0.05)

	if regressed {
		t.Fatalf("expected regressed=false, got %+v", regressions)
	}
	if len(regressions) != 0 {
		t.Fatalf("expected no regressions, got %+v", regressions)
	}
	if !floatsNear(composite, -0.03, 1e-9) {
		t.Errorf("composite = %v, want %v", composite, -0.03)
	}
}

func TestFitnessSnapshot_Delta_MixedImprovementAndRegression(t *testing.T) {
	prev := &FitnessSnapshot{Metrics: map[string]float64{
		"up":     0.30,
		"down":   0.80,
		"noise":  0.50,
		"stable": 0.60,
	}}
	curr := FitnessSnapshot{Metrics: map[string]float64{
		"up":     0.50, // +0.20 improvement
		"down":   0.60, // -0.20 regression, floor 0.05 breached
		"noise":  0.48, // -0.02 absorbed into composite
		"stable": 0.60, // 0 delta
	}}

	composite, regressions, regressed := curr.Delta(prev, nil, 0.05)

	if !regressed {
		t.Fatal("expected regressed=true")
	}
	if len(regressions) != 1 || regressions[0].Name != "down" {
		t.Fatalf("expected only 'down' regressed, got %+v", regressions)
	}
	want := (0.20 + -0.20 + -0.02 + 0.0) / 4.0
	if !floatsNear(composite, want, 1e-9) {
		t.Errorf("composite = %v, want %v", composite, want)
	}
}

func TestFitnessSnapshot_Delta_NilPrev(t *testing.T) {
	curr := FitnessSnapshot{Metrics: map[string]float64{"a": 0.9, "b": 0.1}}

	composite, regressions, regressed := curr.Delta(nil, nil, 0.05)

	if composite != 0 {
		t.Errorf("composite = %v, want 0", composite)
	}
	if regressions != nil {
		t.Errorf("regressions = %+v, want nil", regressions)
	}
	if regressed {
		t.Errorf("regressed = true, want false")
	}
}

func TestFitnessSnapshot_Delta_MissingMetricInCurr(t *testing.T) {
	prev := &FitnessSnapshot{Metrics: map[string]float64{
		"A": 0.5,
		"B": 0.2,
	}}
	curr := FitnessSnapshot{Metrics: map[string]float64{
		"B": 0.2,
	}}

	composite, regressions, regressed := curr.Delta(prev, nil, 0.1)

	if !regressed {
		t.Fatal("expected regressed=true due to missing 'A'")
	}
	if len(regressions) != 1 || regressions[0].Name != "A" {
		t.Fatalf("expected regression for 'A', got %+v", regressions)
	}
	if !floatsNear(regressions[0].Drop, 0.5, 1e-9) {
		t.Errorf("Drop = %v, want %v", regressions[0].Drop, 0.5)
	}
	if !floatsNear(regressions[0].Previous, 0.5, 1e-9) {
		t.Errorf("Previous = %v, want %v", regressions[0].Previous, 0.5)
	}
	if regressions[0].Current != 0 {
		t.Errorf("Current = %v, want 0", regressions[0].Current)
	}
	// composite: (-0.5 + 0) / 2
	if !floatsNear(composite, -0.25, 1e-9) {
		t.Errorf("composite = %v, want -0.25", composite)
	}
}

func TestFitnessSnapshot_Delta_PerMetricFloorOverride(t *testing.T) {
	prev := &FitnessSnapshot{Metrics: map[string]float64{
		"tight": 0.50,
		"loose": 0.50,
	}}
	curr := FitnessSnapshot{Metrics: map[string]float64{
		"tight": 0.48, // drop 0.02 — below default 0.05 but above override 0.01
		"loose": 0.48, // drop 0.02 — absorbed under default floor
	}}

	floors := map[string]float64{"tight": 0.01}
	_, regressions, regressed := curr.Delta(prev, floors, 0.05)

	if !regressed {
		t.Fatal("expected regressed=true because 'tight' breaches override")
	}
	if len(regressions) != 1 || regressions[0].Name != "tight" {
		t.Fatalf("expected only 'tight' regressed, got %+v", regressions)
	}
	if !floatsNear(regressions[0].FloorBreached, 0.01, 1e-9) {
		t.Errorf("FloorBreached = %v, want 0.01", regressions[0].FloorBreached)
	}
}

func TestFitnessSnapshot_Delta_Deterministic(t *testing.T) {
	prev := &FitnessSnapshot{Metrics: map[string]float64{
		"z": 0.90,
		"a": 0.10,
		"m": 0.50,
		"b": 0.40,
	}}
	curr := FitnessSnapshot{Metrics: map[string]float64{
		"z": 0.70, // regression
		"a": 0.05, // drop 0.05 — not > floor 0.05, absorbed
		"m": 0.30, // regression
		"b": 0.38, // absorbed
	}}

	c1, r1, _ := curr.Delta(prev, nil, 0.05)
	c2, r2, _ := curr.Delta(prev, nil, 0.05)

	if c1 != c2 {
		t.Errorf("composite not deterministic: %v vs %v", c1, c2)
	}
	if !reflect.DeepEqual(r1, r2) {
		t.Errorf("regression list not deterministic:\n%+v\n%+v", r1, r2)
	}
	// Sorted order: a, b, m, z -> regressions are 'm' then 'z'
	if len(r1) != 2 {
		t.Fatalf("expected 2 regressions, got %d: %+v", len(r1), r1)
	}
	if r1[0].Name != "m" || r1[1].Name != "z" {
		t.Errorf("regression order = [%s, %s], want [m, z]", r1[0].Name, r1[1].Name)
	}
}

func TestEvaluateMeasureHalt_StrictRegression(t *testing.T) {
	prev := &FitnessSnapshot{Metrics: map[string]float64{"precision": 0.90}}
	curr := FitnessSnapshot{Metrics: map[string]float64{"precision": 0.70}}

	halt := EvaluateMeasureHalt(MeasureHaltInput{
		Current:         curr,
		Previous:        prev,
		RegressionFloor: 0.05,
		WarnOnly:        false,
	})

	if !halt.ShouldHalt {
		t.Fatal("ShouldHalt=false, want true")
	}
	if halt.Kind != MeasureHaltRegression {
		t.Fatalf("Kind=%q, want %q", halt.Kind, MeasureHaltRegression)
	}
	if !halt.Regressed || len(halt.Regressions) != 1 || halt.Regressions[0].Name != "precision" {
		t.Fatalf("regression output = %#v", halt)
	}
	if !floatsNear(halt.FitnessDelta, -0.20, 1e-9) {
		t.Fatalf("FitnessDelta=%v, want -0.20", halt.FitnessDelta)
	}
}

func TestEvaluateMeasureHalt_MeasureErrorCap(t *testing.T) {
	halt := EvaluateMeasureHalt(MeasureHaltInput{
		MeasureError:               errors.New("synthetic measure failure"),
		ConsecutiveMeasureFailures: 2,
		MaxConsecutiveFailures:     2,
		WarnOnly:                   true,
	})

	if !halt.ShouldHalt {
		t.Fatal("ShouldHalt=false, want true")
	}
	if halt.Kind != MeasureHaltMeasureError {
		t.Fatalf("Kind=%q, want %q", halt.Kind, MeasureHaltMeasureError)
	}
	if halt.Reason == "" {
		t.Fatal("expected measure-error halt reason")
	}
}

// floatsNear compares two float64 values within tolerance. Replaces
// the banned != / == direct float comparisons.
func floatsNear(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}
