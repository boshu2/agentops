package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/evalsubstrate"
)

type OutcomesRuntime interface {
	ReadFile(string) ([]byte, error)
	LoadBurnLedger(string) (evalsubstrate.HoldoutBurnLedger, error)
	SaveBurnLedger(string, evalsubstrate.HoldoutBurnLedger) error
	WriteOutcomesManifest(string, string, RunRecord) (string, error)
	Now() time.Time
}

type OutcomesService struct{ Runtime OutcomesRuntime }

type OutcomesCompileInput struct {
	Task             evalsubstrate.Task        `json:"task"`
	Criteria         []evalsubstrate.Criterion `json:"criteria"`
	JudgeContentHash string                    `json:"judge_content_hash"`
	HoldoutValues    []string                  `json:"holdout_values,omitempty"`
}

type OutcomesScore struct {
	SourceTaskID, JudgeContentHash             string
	Aggregate, Threshold                       float64
	CriterionScores                            map[string]float64
	Split, SuiteRef, GroundTruthVersion, RunID string
}

func (score *OutcomesScore) UnmarshalJSON(data []byte) error {
	type wire struct {
		SourceTaskID       string             `json:"source_task_id"`
		JudgeContentHash   string             `json:"judge_content_hash"`
		Aggregate          float64            `json:"aggregate"`
		Threshold          float64            `json:"threshold"`
		CriterionScores    map[string]float64 `json:"criterion_scores"`
		Split              string             `json:"split"`
		SuiteRef           string             `json:"suite_ref"`
		GroundTruthVersion string             `json:"ground_truth_version"`
		RunID              string             `json:"run_id"`
	}
	var value wire
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*score = OutcomesScore{SourceTaskID: value.SourceTaskID, JudgeContentHash: value.JudgeContentHash, Aggregate: value.Aggregate, Threshold: value.Threshold, CriterionScores: value.CriterionScores, Split: value.Split, SuiteRef: value.SuiteRef, GroundTruthVersion: value.GroundTruthVersion, RunID: value.RunID}
	return nil
}

type OutcomesVerdict struct {
	Verdict               string             `json:"verdict"`
	Confidence            string             `json:"confidence"`
	KeyInsight            string             `json:"key_insight"`
	Recommendation        string             `json:"recommendation"`
	SchemaVersion         int                `json:"schema_version"`
	SatisfactionScore     *float64           `json:"satisfaction_score"`
	SatisfactionBreakdown map[string]float64 `json:"satisfaction_breakdown"`
	Findings              []map[string]any   `json:"findings"`
}

type OutcomesIngestRequest struct{ ScorePath, ExpectedJudgeHash, BurnLedgerPath, ManifestDir, RunID string }
type OutcomesIngestResult struct {
	Verdict               OutcomesVerdict
	Warning, ManifestPath string
}

func (service OutcomesService) Compile(_ context.Context, path string) (evalsubstrate.Rubric, error) {
	raw, err := service.Runtime.ReadFile(path)
	if err != nil {
		return evalsubstrate.Rubric{}, fmt.Errorf("read %s: %w", path, err)
	}
	var input OutcomesCompileInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return evalsubstrate.Rubric{}, fmt.Errorf("parse %s: %w", path, err)
	}
	rubric := evalsubstrate.ProjectRubric(input.Task, input.Criteria, input.JudgeContentHash)
	if hit, found := rubric.ContainsAny(input.HoldoutValues); found {
		return evalsubstrate.Rubric{}, fmt.Errorf("outcomes compile: holdout value %q would leak into the rubric payload; refusing (Managed Agents are not ZDR)", hit)
	}
	if key, found := rubric.HasForbiddenKey(); found {
		return evalsubstrate.Rubric{}, fmt.Errorf("outcomes compile: emitted payload carries holdout-leak key %q; refusing (Managed Agents are not ZDR)", key)
	}
	return rubric, nil
}

func (service OutcomesService) Ingest(_ context.Context, request OutcomesIngestRequest) (OutcomesIngestResult, error) {
	raw, err := service.Runtime.ReadFile(request.ScorePath)
	if err != nil {
		return OutcomesIngestResult{}, fmt.Errorf("read %s: %w", request.ScorePath, err)
	}
	var score OutcomesScore
	if err := json.Unmarshal(raw, &score); err != nil {
		return OutcomesIngestResult{}, fmt.Errorf("parse %s: %w", request.ScorePath, err)
	}
	warning := outcomesHashWarning(score.JudgeContentHash, request.ExpectedJudgeHash)
	if request.ExpectedJudgeHash != "" && score.JudgeContentHash != request.ExpectedJudgeHash {
		return OutcomesIngestResult{}, fmt.Errorf("outcomes ingest: judge_content_hash mismatch — score was graded against %q but the active rubric is %q; the rubric drifted and this score is stale (gate #2 parity, refused)", score.JudgeContentHash, request.ExpectedJudgeHash)
	}
	if request.BurnLedgerPath != "" && score.Split == "holdout" {
		ledger, err := service.Runtime.LoadBurnLedger(request.BurnLedgerPath)
		if err != nil {
			return OutcomesIngestResult{}, err
		}
		updated, err := registerOutcomesBurn(ledger, score)
		if err != nil {
			return OutcomesIngestResult{}, err
		}
		if err := service.Runtime.SaveBurnLedger(request.BurnLedgerPath, updated); err != nil {
			return OutcomesIngestResult{}, err
		}
	}
	verdict := outcomesVerdict(score)
	result := OutcomesIngestResult{Verdict: verdict, Warning: warning}
	if request.ManifestDir != "" {
		runID := resolveOutcomeRunID(request.RunID, score)
		record := buildOutcomeManifest(score, verdict.Verdict, runID, service.Runtime.Now())
		path, err := service.Runtime.WriteOutcomesManifest(request.ManifestDir, runID, record)
		if err != nil {
			return OutcomesIngestResult{}, err
		}
		result.ManifestPath = path
	}
	return result, nil
}

func outcomesHashWarning(scoreHash, expected string) string {
	if expected != "" || scoreHash == "" {
		return ""
	}
	return fmt.Sprintf("WARN outcomes ingest: score carries judge_content_hash %q but --expect-judge-hash was not provided; rubric-drift parity (gate #2) was NOT checked — pass --expect-judge-hash <active-rubric-hash> to enforce it", scoreHash)
}

func registerOutcomesBurn(ledger evalsubstrate.HoldoutBurnLedger, score OutcomesScore) (evalsubstrate.HoldoutBurnLedger, error) {
	if score.Split != "holdout" {
		return ledger, nil
	}
	if ledger.Budget > 0 && ledger.Spent(score.SuiteRef, score.GroundTruthVersion) >= ledger.Budget {
		return ledger, fmt.Errorf("outcomes ingest: holdout burn refused — quota for (suite=%q, gt_version=%q) is exhausted (%d/%d spent); the holdout split is statistically spent and may not be re-observed (gate #3, no escape)", score.SuiteRef, score.GroundTruthVersion, ledger.Spent(score.SuiteRef, score.GroundTruthVersion), ledger.Budget)
	}
	ledger.Records = append(ledger.Records, evalsubstrate.BurnRecord{SuiteRef: score.SuiteRef, GTVersion: score.GroundTruthVersion, RunID: score.RunID})
	return ledger, nil
}

func outcomesVerdict(score OutcomesScore) OutcomesVerdict {
	threshold := score.Threshold
	if threshold <= 0 {
		threshold = 1
	}
	band := "WARN"
	if score.Aggregate >= threshold {
		band = "PASS"
	} else if score.Aggregate < threshold*.7 {
		band = "FAIL"
	}
	aggregate := score.Aggregate
	return OutcomesVerdict{Verdict: band, Confidence: "HIGH", KeyInsight: fmt.Sprintf("Outcomes aggregate %.4f vs threshold %.4f for task %q", score.Aggregate, threshold, score.SourceTaskID), Recommendation: fmt.Sprintf("Outcomes grade ingested as %s; feeds the corpus via the eval-verdict pipeline.", band), SchemaVersion: 4, SatisfactionScore: &aggregate, SatisfactionBreakdown: score.CriterionScores, Findings: []map[string]any{}}
}

var outcomeRunIDInvalid = regexp.MustCompile(`[^a-zA-Z0-9._:-]`)

func resolveOutcomeRunID(explicit string, score OutcomesScore) string {
	for _, candidate := range []string{explicit, score.RunID, score.SourceTaskID} {
		cleaned := strings.TrimLeft(outcomeRunIDInvalid.ReplaceAllString(candidate, "-"), "-._:")
		if cleaned != "" {
			return cleaned
		}
	}
	return "outcomes-run"
}

func buildOutcomeManifest(score OutcomesScore, band, runID string, started time.Time) RunRecord {
	dimensions := map[Dimension]float64{}
	allowed := map[Dimension]bool{DimensionCorrectness: true, DimensionProcessAdherence: true, DimensionArtifactQuality: true, DimensionRuntimeCompatibility: true, DimensionEfficiency: true, DimensionSafety: true, DimensionLearningClosure: true}
	for key, value := range score.CriterionScores {
		dimension := Dimension(key)
		if allowed[dimension] {
			dimensions[dimension] = clampOutcomeScore(value)
		}
	}
	if len(dimensions) == 0 {
		dimensions[DimensionCorrectness] = clampOutcomeScore(score.Aggregate)
	}
	status, verdict := StatusInconclusive, VerdictAdvisory
	if band == "PASS" {
		status, verdict = StatusPass, VerdictPass
	} else if band == "FAIL" {
		status, verdict = StatusFail, VerdictFail
	}
	caseID := score.SourceTaskID
	if caseID == "" {
		caseID = "aggregate"
	}
	suiteID := score.SuiteRef
	if suiteID == "" {
		suiteID = "outcomes"
	}
	completed := started
	visibility := VisibilityPublicCanary
	if score.Split == "holdout" {
		visibility = VisibilityPrivateHoldout
	}
	return RunRecord{SchemaVersion: 1, RunID: runID, Suite: SuiteRef{ID: suiteID, Path: "out-of-process", Visibility: visibility, Tier: TierLive}, StartedAt: started, CompletedAt: &completed, Status: status, Verdict: verdict, Git: GitRecord{CandidateRef: "out-of-process-outcomes-grade", CandidateSHA: "0000000"}, Runtime: RuntimeRecord{Name: RuntimeManual}, Environment: EnvironmentRecord{ScrubbedEnvPrefixes: []string{}, NetworkAccess: NetworkUnknown}, CaseResults: []CaseResult{{ID: caseID, Status: status, Score: clampOutcomeScore(score.Aggregate), DimensionScores: dimensions}}, AggregateScore: clampOutcomeScore(score.Aggregate), DimensionScores: dimensions, Notes: []string{fmt.Sprintf("Out-of-process Outcomes grade ingested by `ao eval outcomes ingest` (band=%s); git/runtime/environment are stubs — the grade was produced outside this harness.", band)}}
}
func clampOutcomeScore(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
