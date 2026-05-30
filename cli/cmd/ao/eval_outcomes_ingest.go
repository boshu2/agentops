package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/boshu2/agentops/cli/internal/evalsubstrate"
	"github.com/spf13/cobra"
)

// outcomesScore is the score payload returned by an Outcomes grader (cloud
// Managed Agent, async job, or the local Codex/NTM path). It carries only
// aggregate + per-criterion scores — never holdout answers.
type outcomesScore struct {
	SourceTaskID     string             `json:"source_task_id"`
	JudgeContentHash string             `json:"judge_content_hash"`
	Aggregate        float64            `json:"aggregate"`
	Threshold        float64            `json:"threshold"`
	CriterionScores  map[string]float64 `json:"criterion_scores"`

	// Burn-ledger carriers (ag-hdqu0.5). Split is the eval split the candidate
	// was graded against; only "holdout" consumes quota. SuiteRef +
	// GroundTruthVersion key the burn; RunID identifies the consuming run.
	// Absent on dev-split or pre-.5 payloads (→ no burn).
	Split              string `json:"split,omitempty"`
	SuiteRef           string `json:"suite_ref,omitempty"`
	GroundTruthVersion string `json:"ground_truth_version,omitempty"`
	RunID              string `json:"run_id,omitempty"`
}

// outcomesVerdict is the one verdict record (skills/council/schemas/verdict.json
// shape). ao eval outcomes ingest emits this so an Outcomes score feeds the
// Knowledge Flywheel exactly as a local `ao eval run` verdict does — no second
// verdict format, no alternate bar.
type outcomesVerdict struct {
	Verdict               string             `json:"verdict"`
	Confidence            string             `json:"confidence"`
	KeyInsight            string             `json:"key_insight"`
	Recommendation        string             `json:"recommendation"`
	SchemaVersion         int                `json:"schema_version"`
	SatisfactionScore     *float64           `json:"satisfaction_score"`
	SatisfactionBreakdown map[string]float64 `json:"satisfaction_breakdown"`
	Findings              []map[string]any   `json:"findings"`
}

// ingestOutcomesScore maps an Outcomes score onto the council verdict record.
// PASS when aggregate meets the rubric threshold, FAIL when it falls below 70%
// of it, WARN in between. The aggregate becomes satisfaction_score and the
// per-criterion scores become satisfaction_breakdown.
func ingestOutcomesScore(s outcomesScore) outcomesVerdict {
	threshold := s.Threshold
	if threshold <= 0 {
		threshold = 1.0
	}
	verdict := "WARN"
	switch {
	case s.Aggregate >= threshold:
		verdict = "PASS"
	case s.Aggregate < threshold*0.7:
		verdict = "FAIL"
	}
	agg := s.Aggregate
	return outcomesVerdict{
		Verdict:    verdict,
		Confidence: "HIGH",
		KeyInsight: fmt.Sprintf("Outcomes aggregate %.4f vs threshold %.4f for task %q",
			s.Aggregate, threshold, s.SourceTaskID),
		Recommendation:        fmt.Sprintf("Outcomes grade ingested as %s; feeds the corpus via the eval-verdict pipeline.", verdict),
		SchemaVersion:         4,
		SatisfactionScore:     &agg,
		SatisfactionBreakdown: s.CriterionScores,
		Findings:              []map[string]any{},
	}
}

// registerOutcomesBurn is the write-side of the holdout burn ledger (ag-hdqu0.5),
// the counterpart to gate #3's read-side (#607). When an Outcomes grade was
// produced against the holdout split, ingest appends exactly one BurnRecord to
// the global ledger — so a cloud/async/Codex Outcomes run cannot silently bypass
// holdout quota the way a local holdout run cannot. Dev-split (and absent-split)
// grades register no burn, since the dev split is reusable.
//
// Gate #3 write-side enforcement (ag-62g68): if the (suite_ref, gt_version)
// holdout quota is already spent (Spent >= Budget, with a positive Budget), the
// burn is REFUSED and the ledger is returned unmutated — symmetric with the
// read-side gate #3 in gates.go, which refuses a local run on the same
// condition. Without this, a cloud/Codex Outcomes run could re-burn an exhausted
// holdout split, the exact statistical leak the ledger exists to prevent
// (Managed Agents are not ZDR). A non-positive Budget means no ceiling is
// configured (Day-4 input absent → no-op), so burns are recorded without refusal.
//
// Returns the updated ledger for the caller to persist; the global-Dolt store is
// a separate adapter concern, exactly as gate #3 reads an injected ledger.
func registerOutcomesBurn(led evalsubstrate.HoldoutBurnLedger, s outcomesScore) (evalsubstrate.HoldoutBurnLedger, error) {
	if s.Split != "holdout" {
		return led, nil
	}
	if led.Budget > 0 {
		if spent := led.Spent(s.SuiteRef, s.GroundTruthVersion); spent >= led.Budget {
			return led, fmt.Errorf("outcomes ingest: holdout burn refused — quota for (suite=%q, gt_version=%q) is exhausted (%d/%d spent); the holdout split is statistically spent and may not be re-observed (gate #3, no escape)",
				s.SuiteRef, s.GroundTruthVersion, spent, led.Budget)
		}
	}
	led.Records = append(led.Records, evalsubstrate.BurnRecord{
		SuiteRef:  s.SuiteRef,
		GTVersion: s.GroundTruthVersion,
		RunID:     s.RunID,
	})
	return led, nil
}

// requireJudgeHashParity is the gate #2 (rubric-drift parity) guard for ingest.
// An Outcomes score carries the judge_content_hash of the rubric it was graded
// against (SCHEMA rc2 drift key). If the caller supplies the active Suite's
// current judge hash and it differs from the score's, the rubric drifted between
// projection and grading — the score is stale and MUST be refused, exactly as a
// stale local judge self-invalidates. An empty expected hash means the caller
// did not configure a parity check (no-op), so legacy/dev flows are unaffected.
func requireJudgeHashParity(scoreHash, expected string) error {
	if expected == "" || scoreHash == expected {
		return nil
	}
	return fmt.Errorf("outcomes ingest: judge_content_hash mismatch — score was graded against %q but the active rubric is %q; the rubric drifted and this score is stale (gate #2 parity, refused)",
		scoreHash, expected)
}

var evalOutcomesIngestExpectHash string

var evalOutcomesIngestCmd = &cobra.Command{
	Use:   "ingest <score.json>",
	Short: "Ingest an Outcomes score payload into the one council verdict record",
	Args:  cobra.ExactArgs(1),
	RunE:  runEvalOutcomesIngest,
}

func runEvalOutcomesIngest(cmd *cobra.Command, args []string) error {
	raw, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("read %s: %w", args[0], err)
	}
	var s outcomesScore
	if err := json.Unmarshal(raw, &s); err != nil {
		return fmt.Errorf("parse %s: %w", args[0], err)
	}
	if err := requireJudgeHashParity(s.JudgeContentHash, evalOutcomesIngestExpectHash); err != nil {
		return err
	}
	out, err := json.MarshalIndent(ingestOutcomesScore(s), "", "  ")
	if err != nil {
		return fmt.Errorf("encode verdict: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(out))
	return nil
}

func init() {
	evalOutcomesIngestCmd.Flags().StringVar(&evalOutcomesIngestExpectHash, "expect-judge-hash", "",
		"refuse the ingest if the score's judge_content_hash does not match this value (gate #2 rubric-drift parity)")
	evalOutcomesCmd.AddCommand(evalOutcomesIngestCmd)
}
