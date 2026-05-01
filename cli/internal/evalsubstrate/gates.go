package evalsubstrate

import (
	"fmt"
	"path/filepath"
	"strings"
)

// GateInputs aggregates everything needed to run the Day-2 manifest-checkable
// gates. Day 4 will extend with Judge calibration + holdout-burn-ledger inputs.
type GateInputs struct {
	Suite             *Suite
	Task              *Task
	Harness           *Harness         // declared content_hash from harness.yaml
	HarnessLock       *HarnessLock     // computed lock file
	HarnessDir        string           // source dir for re-verification (gate #8)
	GroundTruth       []GroundTruthRow // current GT rows
	GTRequested       string           // ground_truth_id requested by the run
	AllowWeak         bool             // --allow-weak-labels passthrough
	NRequiredOverride int              // Day-3 power-derived n_required (overrides Task.stats.min_n_samples)
}

// RunGates executes Day-2 gates 1, 6, 7, 8, 9 in §6 order.
// Returns the populated Refusals; callers proceed iff Empty().
//
// Day-2 scope: gates that need only manifest-time inputs (Suite + Task +
// Harness + GT). Gates 2/3/4/5 (Judge calibration, holdout burn, ModelSpec
// drift, self-grading) require Day-4 surfaces.
func RunGates(in GateInputs) Refusals {
	var rs Refusals
	if r := gate1NoHeldConstant(in); r != nil {
		rs = append(rs, *r)
	}
	if r := gate6Underpowered(in); r != nil {
		rs = append(rs, *r)
	}
	if r := gate7GroundTruth(in); r != nil {
		rs = append(rs, *r)
	}
	if r := gate8HarnessDrift(in); r != nil {
		rs = append(rs, *r)
	}
	if r := gate9MultiComparison(in); r != nil {
		rs = append(rs, *r)
	}
	return rs
}

// gate1NoHeldConstant: §6 #1 — Suite has no held_constant declaration.
func gate1NoHeldConstant(in GateInputs) *Refusal {
	if in.Suite == nil || !in.Suite.HeldConstant.IsEmpty() {
		return nil
	}
	if in.Suite.Kind == "calibration" {
		return nil // calibration suites are exempt
	}
	return &Refusal{
		GateNumber: 1,
		GateName:   "no_held_constant",
		Why:        "Suite.held_constant is empty; without pinning what's held constant, comparisons across runs conflate intended and unintended variation.",
		Evidence:   fmt.Sprintf("suite_id=%s held_constant={}", in.Suite.ID),
		Fix:        "Add a held_constant block to the Suite (task, harness, judge, ground_truth_version, decoding) or change kind to 'calibration'.",
	}
}

// gate6Underpowered: §6 #6 — n_samples < n_required.
//
// Day-3 (this commit): when caller provides a power-derived n_required via
// GateInputs.NRequiredOverride > 0, that wins. Otherwise fall back to
// Task.stats.min_n_samples (Day-2 behavior). Day-3 makes the CLI compute
// the override via `ao eval suite n-required`.
func gate6Underpowered(in GateInputs) *Refusal {
	if in.Suite == nil || in.Task == nil {
		return nil
	}
	required := in.Task.Stats.MinNSamples
	if in.NRequiredOverride > 0 {
		required = in.NRequiredOverride
	}
	if required <= 0 {
		return nil // no enforceable floor available
	}
	if in.Suite.NSamples >= required {
		return nil
	}
	mde := 0.0
	power := 0.0
	alpha := 0.0
	if in.Suite.Stats.Power != nil {
		mde = in.Suite.Stats.Power.MinimumDetectableEffect
		alpha = in.Suite.Stats.Power.Alpha
		power = 0.80 // standard convention; Day-3 §6.5 makes this explicit
	}
	return &Refusal{
		GateNumber: 6,
		GateName:   "underpowered",
		Why:        fmt.Sprintf("n_samples=%d < n_required=%d (Day-2 fallback to Task.stats.min_n_samples; Day-3 graduates to power-derived n_required).", in.Suite.NSamples, required),
		Evidence:   fmt.Sprintf("n=%d, n_required=%d (min_n_samples), MDE=%.3f, power=%.2f, alpha=%.3f", in.Suite.NSamples, required, mde, power, alpha),
		Fix:        fmt.Sprintf("Increase Suite.n_samples to >=%d, or relax Task.stats.min_n_samples after re-deriving sample size from §6.5 power calc.", required),
	}
}

// gate7GroundTruth: §6 #7 — ground-truth version superseded OR weak-confidence
// floor not met.
func gate7GroundTruth(in GateInputs) *Refusal {
	if in.GTRequested == "" || len(in.GroundTruth) == 0 {
		return nil
	}
	// Build supersession map: id -> superseded_by (i.e., reverse of supersedes)
	supersededBy := map[string]string{}
	for _, row := range in.GroundTruth {
		if row.Supersedes != "" {
			supersededBy[row.Supersedes] = row.ID
		}
	}
	if newer, ok := supersededBy[in.GTRequested]; ok {
		return &Refusal{
			GateNumber: 7,
			GateName:   "ground_truth_superseded",
			Why:        "Requested ground-truth row has been superseded; longitudinal claims would be invalid against re-labeled data.",
			Evidence:   fmt.Sprintf("ground_truth_ref=%s superseded_by=%s", in.GTRequested, newer),
			Fix:        fmt.Sprintf("Re-run with ground_truth_ref=%s (the head of the supersession chain).", newer),
		}
	}
	// Weak-confidence floor: any matching row with confidence=weak unless --allow-weak-labels
	if !in.AllowWeak {
		for _, row := range in.GroundTruth {
			if row.ID == in.GTRequested && row.Confidence == "weak" {
				return &Refusal{
					GateNumber: 7,
					GateName:   "ground_truth_weak_confidence",
					Why:        "Requested ground-truth row carries confidence=weak; runs against weak labels need explicit acknowledgement.",
					Evidence:   fmt.Sprintf("ground_truth_ref=%s confidence=weak", row.ID),
					Fix:        "Re-run with --allow-weak-labels (records weak_label_fraction in manifest), or upgrade the ground-truth row via human review.",
				}
			}
		}
	}
	return nil
}

// gate8HarnessDrift: §6 #8 — harness content_hash differs from harness.lock.json.
// Source of truth: re-walk srcDir + recompute aggregate hash (lock-vs-now).
func gate8HarnessDrift(in GateInputs) *Refusal {
	if in.Harness == nil || in.HarnessLock == nil || in.HarnessDir == "" {
		return nil
	}
	ok, computed, err := VerifyHarnessLock(in.HarnessDir, in.HarnessLock)
	if err != nil {
		return &Refusal{
			GateNumber: 8,
			GateName:   "harness_drift_unverifiable",
			Why:        "Harness directory could not be re-walked to verify lock; cannot prove content_hash integrity.",
			Evidence:   fmt.Sprintf("harness_dir=%s err=%v", filepath.Clean(in.HarnessDir), err),
			Fix:        "Re-snapshot the harness with `ao eval harness snapshot <dir>` and retry.",
		}
	}
	if !ok {
		return &Refusal{
			GateNumber: 8,
			GateName:   "harness_drift",
			Why:        "Harness content_hash differs from harness.lock.json; the recorded harness no longer matches what's on disk.",
			Evidence:   fmt.Sprintf("lock=%s computed=%s", short(in.HarnessLock.ContentHash), short(computed)),
			Fix:        "Re-snapshot the harness (`ao eval harness snapshot <dir>`) and bump the harness id, OR revert the source files to match the lock.",
		}
	}
	return nil
}

// gate9MultiComparison: §6 #9 — when |varied_axis.values| > 2, the Suite MUST
// declare multi_comparison_method AND comparison_family. When family=vs_reference,
// reference_arm is also required.
func gate9MultiComparison(in GateInputs) *Refusal {
	if in.Suite == nil {
		return nil
	}
	n := len(in.Suite.VariedAxis.Values)
	if n <= 2 {
		return nil
	}
	method := in.Suite.Stats.MultiComparisonMethod
	family := in.Suite.Stats.ComparisonFamily
	ref := in.Suite.Stats.ReferenceArm

	missing := []string{}
	if method == "" {
		missing = append(missing, "multi_comparison_method")
	}
	if (method == "bonferroni" || method == "benjamini_hochberg") && family == "" {
		missing = append(missing, "comparison_family")
	}
	if family == "vs_reference" && ref == "" {
		missing = append(missing, "reference_arm")
	}
	if len(missing) == 0 {
		return nil
	}
	return &Refusal{
		GateNumber: 9,
		GateName:   "multi_comparison_unspecified",
		Why:        fmt.Sprintf("|varied_axis.values|=%d > 2 but multi-comparison plan is incomplete; family-wise FPR rises sharply (4-arm at alpha=0.05 ~= 18%%).", n),
		Evidence:   fmt.Sprintf("missing=%s method=%q family=%q reference_arm=%q", strings.Join(missing, ","), method, family, ref),
		Fix:        "Add Suite.stats.multi_comparison_method in {bonferroni,benjamini_hochberg,pre_registered_alpha} AND comparison_family in {vs_reference,all_pairs,hypothesis_set}; when family=vs_reference, also set reference_arm. All must be pre-registered before run start.",
	}
}

// short returns the last 12 chars of a sha256:HEX string for evidence
// display. We don't truncate hashes for matching, only for the human-facing
// Evidence line.
func short(h string) string {
	if len(h) < 12 {
		return h
	}
	return "..." + h[len(h)-12:]
}

// FamilySizeK computes k from comparison_family + |varied_axis.values|.
// Stamped into Manifest.FamilySizeK at run start per §4.
func FamilySizeK(family string, nValues int) int {
	switch family {
	case "vs_reference":
		if nValues > 0 {
			return nValues - 1
		}
	case "all_pairs":
		if nValues >= 2 {
			return nValues * (nValues - 1) / 2
		}
	case "hypothesis_set":
		// declared comparisons; caller substitutes len(hypothesis_set)
		return -1
	}
	return 0
}
