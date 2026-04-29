package eval

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAuditBaselinePolicyFindsPolicyMismatches(t *testing.T) {
	dir := t.TempDir()
	suiteDir := filepath.Join(dir, "evals")
	baselineDir := filepath.Join(dir, "baselines")
	nonePath := filepath.Join(suiteDir, "none.json")
	comparePath := filepath.Join(suiteDir, "compare.json")
	writeCoverageSuite(t, nonePath, "audit.none", "cli", "static")
	writeCoverageSuiteWithPolicy(t, comparePath, "audit.compare", "cli", "static", `"baseline_policy": {"mode": "compare"}`)
	writeBaselineRecord(t, filepath.Join(baselineDir, "audit.none.baseline.json"), "audit.none", nonePath, "")
	writeBaselineRecord(t, filepath.Join(baselineDir, "audit.orphan.baseline.json"), "audit.orphan", "evals/orphan.json", "")

	report, err := AuditBaselinePolicy(BaselineAuditOptions{
		Roots:       []string{suiteDir},
		BaselineDir: baselineDir,
	})
	if err != nil {
		t.Fatalf("AuditBaselinePolicy failed: %v", err)
	}
	if report.PolicyMismatchCount != 3 {
		t.Fatalf("policy mismatch count = %d, want 3; report=%+v", report.PolicyMismatchCount, report)
	}
	if len(report.UnexpectedBaselinesForNone) != 1 || report.UnexpectedBaselinesForNone[0].SuiteID != "audit.none" {
		t.Fatalf("unexpected baselines = %+v, want audit.none", report.UnexpectedBaselinesForNone)
	}
	if len(report.MissingCompareBaselines) != 1 || report.MissingCompareBaselines[0].SuiteID != "audit.compare" {
		t.Fatalf("missing compare baselines = %+v, want audit.compare", report.MissingCompareBaselines)
	}
	if len(report.OrphanBaselines) != 1 || report.OrphanBaselines[0].SuiteID != "audit.orphan" {
		t.Fatalf("orphan baselines = %+v, want audit.orphan", report.OrphanBaselines)
	}
}

func TestAuditBaselinePolicyReportsStaleSuiteHashSeparately(t *testing.T) {
	dir := t.TempDir()
	suiteDir := filepath.Join(dir, "evals")
	baselineDir := filepath.Join(dir, "baselines")
	suitePath := filepath.Join(suiteDir, "compare.json")
	writeCoverageSuiteWithPolicy(t, suitePath, "audit.compare", "cli", "static", `"baseline_policy": {"mode": "compare"}`)
	writeBaselineRecord(t, filepath.Join(baselineDir, "audit.compare.baseline.json"), "audit.compare", suitePath, "deadbeef")

	report, err := AuditBaselinePolicy(BaselineAuditOptions{
		Roots:       []string{suiteDir},
		BaselineDir: baselineDir,
	})
	if err != nil {
		t.Fatalf("AuditBaselinePolicy failed: %v", err)
	}
	if report.PolicyMismatchCount != 0 {
		t.Fatalf("policy mismatch count = %d, want 0", report.PolicyMismatchCount)
	}
	if len(report.StaleSuiteHashes) != 1 || report.StaleSuiteHashes[0].SuiteID != "audit.compare" {
		t.Fatalf("stale suite hashes = %+v, want audit.compare", report.StaleSuiteHashes)
	}
}

func writeCoverageSuiteWithPolicy(t *testing.T, path, id, domain, runtimeName, baselinePolicy string) {
	t.Helper()
	writeCoverageSuiteBody(t, path, id, domain, runtimeName, baselinePolicy)
}

func writeCoverageSuiteBody(t *testing.T, path, id, domain, runtimeName, baselinePolicy string) {
	t.Helper()
	body := `{
  "schema_version": 1,
  "id": "` + id + `",
  "name": "` + id + `",
  "domain": "` + domain + `",
  "visibility": "public_canary",
  "tier": "deterministic",
  "allowed_runtimes": ["` + runtimeName + `"],
  "scoring": {
    "aggregate_threshold": 1,
    "dimensions": [
      {"name": "correctness", "weight": 1, "threshold": 1}
    ]
  },
  ` + baselinePolicy + `,
  "cases": [
    {
      "id": "case",
      "title": "case",
      "kind": "artifact_check",
      "objective": "case",
      "runtime": "` + runtimeName + `",
      "expectations": [
        {"type": "file_exists", "target": "fixture.txt"}
      ],
      "critical": true
    }
  ]
}`
	writeBaselineAuditFile(t, path, body)
	writeBaselineAuditFile(t, filepath.Join(filepath.Dir(path), "fixture.txt"), "ok\n")
}

func writeBaselineAuditFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeBaselineRecord(t *testing.T, path, suiteID, suitePath, suiteSHA string) {
	t.Helper()
	started := time.Date(2026, 4, 29, 20, 0, 0, 0, time.UTC)
	run := &RunRecord{
		SchemaVersion: 1,
		RunID:         suiteID + "-baseline",
		Suite: SuiteRef{
			ID:         suiteID,
			Path:       suitePath,
			Visibility: VisibilityPublicCanary,
			Tier:       TierDeterministic,
			SHA256:     suiteSHA,
		},
		StartedAt: started,
		Status:    StatusPass,
		Verdict:   VerdictPass,
		Git: GitRecord{
			CandidateRef: "test",
			CandidateSHA: "0123456",
			Dirty:        false,
		},
		Runtime: RuntimeRecord{
			Name: RuntimeStatic,
			Live: false,
		},
		Environment: EnvironmentRecord{
			ScrubbedEnvPrefixes: []string{},
			NetworkAccess:       NetworkDisabled,
		},
		Baseline: &BaselineRecord{
			Mode:         BaselineModePromote,
			BaselinePath: path,
		},
		CaseResults: []CaseResult{{
			ID:     "case",
			Status: StatusPass,
			Score:  1,
			DimensionScores: map[Dimension]float64{
				DimensionCorrectness: 1,
			},
		}},
		AggregateScore: 1,
		DimensionScores: map[Dimension]float64{
			DimensionCorrectness: 1,
		},
	}
	if err := WriteRun(path, run); err != nil {
		t.Fatalf("write baseline %s: %v", path, err)
	}
}
