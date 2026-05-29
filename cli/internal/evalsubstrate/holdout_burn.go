package evalsubstrate

// BurnRecord is one consumption of holdout-split observation: a single run that
// graded against a suite's holdout ground-truth version. The canonical store is
// the global Dolt holdout-burn ledger (SCHEMA §6 gate 3); this is the projected
// row gate #3 reasons over. The Dolt read/write adapter is a separate concern —
// gate #3 takes the projected records the same way the other gates take injected
// GroundTruth rows rather than reading a backend directly.
type BurnRecord struct {
	SuiteRef   string `json:"suite_ref"`  // suite the holdout was burned against
	GTVersion  string `json:"gt_version"` // ground-truth version/id whose holdout split was observed
	RunID      string `json:"run_id"`     // run that consumed the budget
	BurnedAtMs int64  `json:"burned_at_ms,omitempty"`
}

// HoldoutBurnLedger is the projection of global burn state that gate #3 checks
// against. Budget is the maximum number of distinct holdout observations
// permitted for a given (suite_ref, gt_version) before the holdout split is
// statistically spent; a non-positive Budget means no enforceable ceiling is
// configured (Day-4 input absent → gate #3 is a no-op).
type HoldoutBurnLedger struct {
	Budget  int          `json:"budget"`
	Records []BurnRecord `json:"records,omitempty"`
}

// Spent returns how many burns the ledger already records for the given
// (suiteRef, gtVersion) pair. The ledger is global, so burns against other
// suites or other ground-truth versions never count toward this pair's quota.
func (l HoldoutBurnLedger) Spent(suiteRef, gtVersion string) int {
	n := 0
	for _, r := range l.Records {
		if r.SuiteRef == suiteRef && r.GTVersion == gtVersion {
			n++
		}
	}
	return n
}
