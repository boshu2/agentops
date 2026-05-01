package evalsubstrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper: minimal Suite/Task that pass all gates.
func okSuite() *Suite {
	return &Suite{
		ID:   "suite-1",
		Kind: "paired-comparison",
		VariedAxis: VariedAxis{
			Kind:   "model",
			Values: []string{"ms:a", "ms:b"},
		},
		HeldConstant: HeldConstant{
			Task:               "t",
			Harness:            "h",
			GroundTruthVersion: "v1",
		},
		SampleSplit: "dev",
		NSamples:    100,
		Stats: SuiteStat{
			DecisionRule: DecisionRule{Kind: "ci_excludes_zero", Confidence: 0.95},
		},
	}
}

func okTask() *Task {
	return &Task{
		ID: "t",
		Stats: TaskStat{
			Metric:      "accuracy",
			Paired:      true,
			MinNSamples: 50,
			DecisionRule: DecisionRule{Kind: "ci_excludes_zero", Confidence: 0.95},
		},
	}
}

func TestGate1_NoHeldConstant_Refuses(t *testing.T) {
	s := okSuite()
	s.HeldConstant = HeldConstant{}
	rs := RunGates(GateInputs{Suite: s, Task: okTask()})
	if !contains(rs, 1) {
		t.Fatalf("gate 1 should fire, got %v", refusalNumbers(rs))
	}
}

func TestGate1_CalibrationKindBypasses(t *testing.T) {
	s := okSuite()
	s.HeldConstant = HeldConstant{}
	s.Kind = "calibration"
	rs := RunGates(GateInputs{Suite: s, Task: okTask()})
	if contains(rs, 1) {
		t.Fatalf("calibration should bypass gate 1, got %v", refusalNumbers(rs))
	}
}

func TestGate6_BelowMinNSamples_Refuses(t *testing.T) {
	s := okSuite()
	s.NSamples = 10 // below min_n_samples=50
	rs := RunGates(GateInputs{Suite: s, Task: okTask()})
	if !contains(rs, 6) {
		t.Fatalf("gate 6 should fire, got %v", refusalNumbers(rs))
	}
	formatted := rs.Format()
	if !strings.Contains(formatted, "n=10") || !strings.Contains(formatted, "n_required=50") {
		t.Fatalf("evidence missing n + n_required: %s", formatted)
	}
}

func TestGate6_AtOrAboveMinNSamples_Passes(t *testing.T) {
	s := okSuite()
	s.NSamples = 50 // exactly at floor
	rs := RunGates(GateInputs{Suite: s, Task: okTask()})
	if contains(rs, 6) {
		t.Fatalf("gate 6 should pass at n_required, got %v", refusalNumbers(rs))
	}
}

func TestGate7_SupersededGroundTruth_Refuses(t *testing.T) {
	rs := RunGates(GateInputs{
		Suite: okSuite(),
		Task:  okTask(),
		GroundTruth: []GroundTruthRow{
			{ID: "gt-1", Confidence: "validated", Split: "dev"},
			{ID: "gt-1-v2", Confidence: "validated", Split: "dev", Supersedes: "gt-1"},
		},
		GTRequested: "gt-1",
	})
	if !contains(rs, 7) {
		t.Fatalf("gate 7 should fire on superseded gt, got %v", refusalNumbers(rs))
	}
	if !strings.Contains(rs.Format(), "superseded_by=gt-1-v2") {
		t.Fatalf("expected superseded_by evidence: %s", rs.Format())
	}
}

func TestGate7_WeakLabels_RefusesWithoutFlag(t *testing.T) {
	rs := RunGates(GateInputs{
		Suite: okSuite(),
		Task:  okTask(),
		GroundTruth: []GroundTruthRow{
			{ID: "gt-1", Confidence: "weak", Split: "dev"},
		},
		GTRequested: "gt-1",
	})
	if !contains(rs, 7) {
		t.Fatalf("gate 7 should fire on weak gt, got %v", refusalNumbers(rs))
	}
}

func TestGate7_WeakLabels_AllowedWithFlag(t *testing.T) {
	rs := RunGates(GateInputs{
		Suite: okSuite(),
		Task:  okTask(),
		GroundTruth: []GroundTruthRow{
			{ID: "gt-1", Confidence: "weak", Split: "dev"},
		},
		GTRequested: "gt-1",
		AllowWeak:   true,
	})
	if contains(rs, 7) {
		t.Fatalf("--allow-weak-labels should bypass gate 7, got %v", refusalNumbers(rs))
	}
}

func TestGate8_HarnessDrift_Refuses(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "harness")
	writeMiniHarness(t, dir, map[string][]byte{
		"SKILL.md": []byte("v1\n"),
	})
	_, lock, err := SnapshotHarness(dir, "h-id", "x")
	if err != nil {
		t.Fatal(err)
	}
	// Mutate file: drift.
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rs := RunGates(GateInputs{
		Suite:       okSuite(),
		Task:        okTask(),
		Harness:     &Harness{ID: "h-id", ContentHash: lock.ContentHash},
		HarnessLock: lock,
		HarnessDir:  dir,
	})
	if !contains(rs, 8) {
		t.Fatalf("gate 8 should fire on drift, got %v", refusalNumbers(rs))
	}
}

func TestGate8_HarnessClean_Passes(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "harness")
	writeMiniHarness(t, dir, map[string][]byte{
		"SKILL.md": []byte("v1\n"),
	})
	h, lock, err := SnapshotHarness(dir, "h-id", "x")
	if err != nil {
		t.Fatal(err)
	}
	rs := RunGates(GateInputs{
		Suite:       okSuite(),
		Task:        okTask(),
		Harness:     h,
		HarnessLock: lock,
		HarnessDir:  dir,
	})
	if contains(rs, 8) {
		t.Fatalf("gate 8 should not fire on clean harness, got %v", refusalNumbers(rs))
	}
}

func TestGate9_3Arms_NoMethod_Refuses(t *testing.T) {
	s := okSuite()
	s.VariedAxis.Values = []string{"ms:a", "ms:b", "ms:c"}
	rs := RunGates(GateInputs{Suite: s, Task: okTask()})
	if !contains(rs, 9) {
		t.Fatalf("gate 9 should fire on 3 arms with no method, got %v", refusalNumbers(rs))
	}
	if !strings.Contains(rs.Format(), "multi_comparison_method") {
		t.Fatalf("evidence missing method label: %s", rs.Format())
	}
}

func TestGate9_BonferroniNeedsFamily(t *testing.T) {
	s := okSuite()
	s.VariedAxis.Values = []string{"a", "b", "c", "d"}
	s.Stats.MultiComparisonMethod = "bonferroni"
	rs := RunGates(GateInputs{Suite: s, Task: okTask()})
	if !contains(rs, 9) {
		t.Fatalf("gate 9 should require family for bonferroni, got %v", refusalNumbers(rs))
	}
}

func TestGate9_VsReferenceNeedsReferenceArm(t *testing.T) {
	s := okSuite()
	s.VariedAxis.Values = []string{"a", "b", "c"}
	s.Stats.MultiComparisonMethod = "bonferroni"
	s.Stats.ComparisonFamily = "vs_reference"
	// no reference_arm set
	rs := RunGates(GateInputs{Suite: s, Task: okTask()})
	if !contains(rs, 9) {
		t.Fatalf("gate 9 should require reference_arm for vs_reference, got %v", refusalNumbers(rs))
	}
}

func TestGate9_FullySpecified_Passes(t *testing.T) {
	s := okSuite()
	s.VariedAxis.Values = []string{"a", "b", "c"}
	s.Stats.MultiComparisonMethod = "bonferroni"
	s.Stats.ComparisonFamily = "vs_reference"
	s.Stats.ReferenceArm = "a"
	rs := RunGates(GateInputs{Suite: s, Task: okTask()})
	if contains(rs, 9) {
		t.Fatalf("gate 9 should pass when fully specified, got %v: %s", refusalNumbers(rs), rs.Format())
	}
}

func TestGate9_2Arms_Bypassed(t *testing.T) {
	rs := RunGates(GateInputs{Suite: okSuite(), Task: okTask()})
	if contains(rs, 9) {
		t.Fatalf("gate 9 should bypass for 2 arms, got %v", refusalNumbers(rs))
	}
}

func TestFamilySizeK_VsReference(t *testing.T) {
	if k := FamilySizeK("vs_reference", 4); k != 3 {
		t.Fatalf("vs_reference k=%d, want 3", k)
	}
}

func TestFamilySizeK_AllPairs(t *testing.T) {
	if k := FamilySizeK("all_pairs", 4); k != 6 {
		t.Fatalf("all_pairs k=%d, want 6", k)
	}
}

func TestFamilySizeK_HypothesisSet(t *testing.T) {
	// Caller fills in via len(declared_set) — function returns -1 sentinel.
	if k := FamilySizeK("hypothesis_set", 4); k != -1 {
		t.Fatalf("hypothesis_set k=%d, want -1 sentinel", k)
	}
}

// TestGates_AllPass_OnHappyPath: a fully-correct setup produces zero refusals.
func TestGates_AllPass_OnHappyPath(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "harness")
	writeMiniHarness(t, dir, map[string][]byte{
		"SKILL.md": []byte("v1\n"),
	})
	h, lock, err := SnapshotHarness(dir, "h-id", "x")
	if err != nil {
		t.Fatal(err)
	}
	rs := RunGates(GateInputs{
		Suite:       okSuite(),
		Task:        okTask(),
		Harness:     h,
		HarnessLock: lock,
		HarnessDir:  dir,
		GroundTruth: []GroundTruthRow{
			{ID: "gt-1", Confidence: "validated", Split: "dev"},
		},
		GTRequested: "gt-1",
	})
	if !rs.Empty() {
		t.Fatalf("happy path should pass all gates, got: %s", rs.Format())
	}
}

func contains(rs Refusals, n int) bool {
	for _, r := range rs {
		if r.GateNumber == n {
			return true
		}
	}
	return false
}

func refusalNumbers(rs Refusals) []int {
	out := make([]int, len(rs))
	for i, r := range rs {
		out[i] = r.GateNumber
	}
	return out
}
