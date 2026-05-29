package main

import (
	"encoding/json"
	"fmt"
	"os"

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
	out, err := json.MarshalIndent(ingestOutcomesScore(s), "", "  ")
	if err != nil {
		return fmt.Errorf("encode verdict: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(out))
	return nil
}

func init() {
	evalOutcomesCmd.AddCommand(evalOutcomesIngestCmd)
}
