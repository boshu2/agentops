package eval

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildCoverageReportSummarizesDomains(t *testing.T) {
	dir := t.TempDir()
	writeCoverageSuite(t, filepath.Join(dir, "cli.json"), "coverage.cli", "cli", "shell")
	writeCoverageSuite(t, filepath.Join(dir, "skill.json"), "coverage.skill", "skill", "static")

	report, err := BuildCoverageReport(CoverageOptions{
		Roots:              []string{dir},
		RequiredDomains:    []string{"cli", "skill", "scenario"},
		RequiredDimensions: []string{"correctness", "efficiency"},
		RequiredRuntimes:   []string{"shell", "static", "mock"},
	})
	if err != nil {
		t.Fatalf("BuildCoverageReport failed: %v", err)
	}
	if report.SuiteCount != 2 || report.CaseCount != 2 || report.CriticalCaseCount != 2 {
		t.Fatalf("counts = suites:%d cases:%d critical:%d, want 2/2/2", report.SuiteCount, report.CaseCount, report.CriticalCaseCount)
	}
	if report.Domains["cli"].SuiteCount != 1 || report.Domains["skill"].SuiteCount != 1 {
		t.Fatalf("domain coverage = %+v, want cli and skill", report.Domains)
	}
	if report.EvidenceKinds[string(EvidenceKindContractCanary)].CaseCount != 2 {
		t.Fatalf("contract_canary evidence coverage = %+v, want two cases", report.EvidenceKinds[string(EvidenceKindContractCanary)])
	}
	if len(report.MissingRequiredDomains) != 1 || report.MissingRequiredDomains[0] != "scenario" {
		t.Fatalf("missing required domains = %v, want [scenario]", report.MissingRequiredDomains)
	}
	if report.Dimensions[string(DimensionCorrectness)] != 2 {
		t.Fatalf("correctness dimension count = %d, want 2", report.Dimensions[string(DimensionCorrectness)])
	}
	if len(report.MissingRequiredDimensions) != 1 || report.MissingRequiredDimensions[0] != "efficiency" {
		t.Fatalf("missing required dimensions = %v, want [efficiency]", report.MissingRequiredDimensions)
	}
	if len(report.MissingRequiredRuntimes) != 1 || report.MissingRequiredRuntimes[0] != "mock" {
		t.Fatalf("missing required runtimes = %v, want [mock]", report.MissingRequiredRuntimes)
	}
}

func TestBuildCoverageReportEvidenceKinds(t *testing.T) {
	dir := t.TempDir()
	writeCoverageSuite(t, filepath.Join(dir, "behavior.json"), "coverage.behavior", "rpi", "shell", string(EvidenceKindBehaviorFixture))
	writeCoverageSuite(t, filepath.Join(dir, "scorecard.json"), "coverage.rpi-scorecard", "rpi", "static")

	report, err := BuildCoverageReport(CoverageOptions{
		Roots:                 []string{dir},
		RequiredEvidenceKinds: []string{string(EvidenceKindBehaviorFixture), string(EvidenceKindLiveRuntime)},
	})
	if err != nil {
		t.Fatalf("BuildCoverageReport failed: %v", err)
	}
	if got := report.EvidenceKinds[string(EvidenceKindBehaviorFixture)].CaseCount; got != 1 {
		t.Fatalf("behavior_fixture case count = %d, want 1", got)
	}
	if got := report.EvidenceKinds[string(EvidenceKindScorecardFixture)].CaseCount; got != 1 {
		t.Fatalf("scorecard_fixture case count = %d, want 1", got)
	}
	if len(report.MissingRequiredEvidenceKinds) != 1 || report.MissingRequiredEvidenceKinds[0] != string(EvidenceKindLiveRuntime) {
		t.Fatalf("missing required evidence kinds = %v, want [live_runtime]", report.MissingRequiredEvidenceKinds)
	}
}

func TestBuildCoverageReportMissingRootFails(t *testing.T) {
	_, err := BuildCoverageReport(CoverageOptions{Roots: []string{filepath.Join(t.TempDir(), "missing")}})
	if err == nil {
		t.Fatalf("BuildCoverageReport missing root succeeded")
	}
}

func writeCoverageSuite(t *testing.T, path, id, domain, runtimeName string, evidenceKind ...string) {
	t.Helper()
	evidenceField := ""
	if len(evidenceKind) > 0 && evidenceKind[0] != "" {
		evidenceField = `,
      "evidence_kind": "` + evidenceKind[0] + `"`
	}
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
  "baseline_policy": {"mode": "none"},
  "cases": [
    {
      "id": "case",
      "title": "case",
      "kind": "artifact_check",
      "objective": "case"` + evidenceField + `,
      "runtime": "` + runtimeName + `",
      "expectations": [
        {"type": "file_exists", "target": "fixture.txt"}
      ],
      "critical": true
    }
  ]
}`
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), "fixture.txt"), []byte("ok\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}
