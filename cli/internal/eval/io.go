package eval

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func LoadSuite(path string) (*Suite, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read eval suite: %w", err)
	}
	var suite Suite
	if err := decodeStrict(data, &suite); err != nil {
		return nil, nil, fmt.Errorf("decode eval suite: %w", err)
	}
	if err := ValidateSuite(&suite); err != nil {
		return nil, nil, err
	}
	return &suite, data, nil
}

func LoadRun(path string) (*RunRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read eval run: %w", err)
	}
	var run RunRecord
	if err := decodeStrict(data, &run); err != nil {
		return nil, fmt.Errorf("decode eval run: %w", err)
	}
	if err := ValidateRun(&run); err != nil {
		return nil, err
	}
	return &run, nil
}

func WriteRun(path string, run *RunRecord) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("output path is required")
	}
	if err := ValidateRun(run); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create eval output directory: %w", err)
	}
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal eval run: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write eval run: %w", err)
	}
	return nil
}

func ValidateSuite(suite *Suite) error {
	var errs []string
	if suite.SchemaVersion != 1 {
		errs = append(errs, "schema_version must be 1")
	}
	if !idPattern.MatchString(suite.ID) {
		errs = append(errs, "id must match ^[a-z0-9][a-z0-9._-]*$")
	}
	if strings.TrimSpace(suite.Name) == "" {
		errs = append(errs, "name is required")
	}
	if !validVisibility(suite.Visibility) {
		errs = append(errs, fmt.Sprintf("visibility %q is not supported", suite.Visibility))
	}
	if !validTier(suite.Tier) {
		errs = append(errs, fmt.Sprintf("tier %q is not supported", suite.Tier))
	}
	if suite.Tier != TierDeterministic {
		errs = append(errs, fmt.Sprintf("tier %q is out of deterministic scope", suite.Tier))
	}
	if suite.Scoring.AggregateThreshold < 0 || suite.Scoring.AggregateThreshold > 1 {
		errs = append(errs, "scoring.aggregate_threshold must be in [0,1]")
	}
	if len(suite.Scoring.Dimensions) == 0 {
		errs = append(errs, "scoring.dimensions must contain at least one dimension")
	}
	for i, dim := range suite.Scoring.Dimensions {
		if !validDimension(dim.Name) {
			errs = append(errs, fmt.Sprintf("scoring.dimensions[%d].name is invalid", i))
		}
		if dim.Weight <= 0 {
			errs = append(errs, fmt.Sprintf("scoring.dimensions[%d].weight must be > 0", i))
		}
		if dim.Threshold < 0 || dim.Threshold > 1 {
			errs = append(errs, fmt.Sprintf("scoring.dimensions[%d].threshold must be in [0,1]", i))
		}
	}
	if strings.TrimSpace(suite.BaselinePolicy.Mode) == "" {
		errs = append(errs, "baseline_policy.mode is required")
	}
	if len(suite.Cases) == 0 {
		errs = append(errs, "cases must contain at least one case")
	}
	seenCases := make(map[string]struct{}, len(suite.Cases))
	for i, c := range suite.Cases {
		if !idPattern.MatchString(c.ID) {
			errs = append(errs, fmt.Sprintf("cases[%d].id is invalid", i))
		}
		if _, ok := seenCases[c.ID]; ok {
			errs = append(errs, fmt.Sprintf("cases[%d].id %q is duplicated", i, c.ID))
		}
		seenCases[c.ID] = struct{}{}
		if strings.TrimSpace(c.Title) == "" {
			errs = append(errs, fmt.Sprintf("cases[%d].title is required", i))
		}
		if strings.TrimSpace(c.Kind) == "" {
			errs = append(errs, fmt.Sprintf("cases[%d].kind is required", i))
		}
		if strings.TrimSpace(c.Objective) == "" {
			errs = append(errs, fmt.Sprintf("cases[%d].objective is required", i))
		}
		if c.Runtime != "" && !validDeterministicRuntime(c.Runtime) {
			errs = append(errs, fmt.Sprintf("cases[%d].runtime %q is out of deterministic scope", i, c.Runtime))
		}
		if len(c.Expectations) == 0 {
			errs = append(errs, fmt.Sprintf("cases[%d].expectations must contain at least one expectation", i))
		}
		for j, exp := range c.Expectations {
			if !validExpectationType(exp.Type) {
				errs = append(errs, fmt.Sprintf("cases[%d].expectations[%d].type %q is invalid", i, j, exp.Type))
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("eval suite validation failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

func ValidateRun(run *RunRecord) error {
	var errs []string
	if run.SchemaVersion != 1 {
		errs = append(errs, "schema_version must be 1")
	}
	if !runIDPattern.MatchString(run.RunID) {
		errs = append(errs, "run_id is invalid")
	}
	if strings.TrimSpace(run.Suite.ID) == "" {
		errs = append(errs, "suite.id is required")
	}
	if strings.TrimSpace(run.Suite.Path) == "" {
		errs = append(errs, "suite.path is required")
	}
	if !validVisibility(run.Suite.Visibility) {
		errs = append(errs, "suite.visibility is invalid")
	}
	if !validTier(run.Suite.Tier) {
		errs = append(errs, "suite.tier is invalid")
	}
	if run.StartedAt.IsZero() {
		errs = append(errs, "started_at is required")
	}
	if !validStatus(run.Status) {
		errs = append(errs, "status is invalid")
	}
	if !validVerdict(run.Verdict) {
		errs = append(errs, "verdict is invalid")
	}
	if strings.TrimSpace(run.Git.CandidateRef) == "" {
		errs = append(errs, "git.candidate_ref is required")
	}
	if !gitSHAPattern.MatchString(run.Git.CandidateSHA) {
		errs = append(errs, "git.candidate_sha is invalid")
	}
	if !validRuntime(run.Runtime.Name) {
		errs = append(errs, "runtime.name is invalid")
	}
	if !validNetwork(run.Environment.NetworkAccess) {
		errs = append(errs, "environment.network_access is invalid")
	}
	if len(run.CaseResults) == 0 {
		errs = append(errs, "case_results must contain at least one case")
	}
	if !scoreInRange(run.AggregateScore) {
		errs = append(errs, "aggregate_score must be in [0,1]")
	}
	if len(run.DimensionScores) == 0 {
		errs = append(errs, "dimension_scores must contain at least one score")
	}
	for dim, score := range run.DimensionScores {
		if !validDimension(dim) {
			errs = append(errs, fmt.Sprintf("dimension_scores.%s is invalid", dim))
		}
		if !scoreInRange(score) {
			errs = append(errs, fmt.Sprintf("dimension_scores.%s must be in [0,1]", dim))
		}
	}
	for i, result := range run.CaseResults {
		if strings.TrimSpace(result.ID) == "" {
			errs = append(errs, fmt.Sprintf("case_results[%d].id is required", i))
		}
		if !validStatus(result.Status) {
			errs = append(errs, fmt.Sprintf("case_results[%d].status is invalid", i))
		}
		if !scoreInRange(result.Score) {
			errs = append(errs, fmt.Sprintf("case_results[%d].score must be in [0,1]", i))
		}
		if len(result.DimensionScores) == 0 {
			errs = append(errs, fmt.Sprintf("case_results[%d].dimension_scores is required", i))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("eval run validation failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

func decodeStrict(data []byte, target any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(target)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func defaultNow() time.Time {
	return time.Now().UTC()
}

var (
	idPattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	runIDPattern  = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:-]*$`)
	gitSHAPattern = regexp.MustCompile(`^[a-fA-F0-9]{7,40}$`)
)

func validVisibility(v Visibility) bool {
	return v == VisibilityPublicCanary || v == VisibilityPrivateHoldout
}

func validTier(t Tier) bool {
	return t == TierDeterministic || t == TierHeadless || t == TierLive || t == TierRelease
}

func validRuntime(r Runtime) bool {
	return r == RuntimeStatic || r == RuntimeMock || r == RuntimeShell || r == RuntimeClaude || r == RuntimeCodex || r == RuntimeManual
}

func validDeterministicRuntime(r Runtime) bool {
	return r == RuntimeStatic || r == RuntimeMock || r == RuntimeShell
}

func validDimension(d Dimension) bool {
	switch d {
	case DimensionCorrectness, DimensionProcessAdherence, DimensionArtifactQuality,
		DimensionRuntimeCompatibility, DimensionEfficiency, DimensionSafety, DimensionLearningClosure:
		return true
	default:
		return false
	}
}

func validStatus(s Status) bool {
	return s == StatusPass || s == StatusFail || s == StatusError || s == StatusSkipped || s == StatusInconclusive
}

func validVerdict(v Verdict) bool {
	return v == VerdictPass || v == VerdictFail || v == VerdictImprovement || v == VerdictRegression || v == VerdictAdvisory || v == VerdictInconclusive
}

func validNetwork(n NetworkAccess) bool {
	return n == NetworkDisabled || n == NetworkEnabled || n == NetworkUnknown
}

func validExpectationType(kind string) bool {
	switch kind {
	case "exit_code", "stdout_contains", "stderr_contains", "json_path", "file_exists",
		"file_absent", "schema_valid", "artifact_contains", "score_at_least", "manual_review":
		return true
	default:
		return false
	}
}

func scoreInRange(score float64) bool {
	return score >= 0 && score <= 1
}
