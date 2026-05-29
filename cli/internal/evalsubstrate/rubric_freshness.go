package evalsubstrate

import "fmt"

// FreshAgainst rejects a stale Outcomes rubric: it returns an error unless the
// rubric's carried JudgeContentHash exactly matches currentJudgeHash. This is
// the drift-parity guard (ag-hdqu0.4, SCHEMA §rc2/gate-#2 parity) — a rubric is
// a derived artifact content-addressed off the judge, so a divergent or unknown
// hash means the rubric grades against an outdated bar and MUST NOT be used.
// An empty hash on either side cannot be verified and is rejected.
func (r Rubric) FreshAgainst(currentJudgeHash string) error {
	if currentJudgeHash == "" {
		return fmt.Errorf("outcomes rubric freshness: current judge_content_hash is empty; cannot verify rubric %q", r.SourceTaskID)
	}
	if r.JudgeContentHash == "" {
		return fmt.Errorf("outcomes rubric freshness: rubric %q carries no judge_content_hash; refusing to grade against an unverifiable bar", r.SourceTaskID)
	}
	if r.JudgeContentHash != currentJudgeHash {
		return fmt.Errorf("outcomes rubric freshness: rubric %q is stale (judge_content_hash %s != current %s); regenerate via `ao eval outcomes compile`",
			r.SourceTaskID, r.JudgeContentHash, currentJudgeHash)
	}
	return nil
}
