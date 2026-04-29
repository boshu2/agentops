package eval

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type BaselineAuditOptions struct {
	SuitePaths  []string
	Roots       []string
	BaselineDir string
}

type BaselineAuditReport struct {
	SuiteCount                 int                    `json:"suite_count"`
	BaselineCount              int                    `json:"baseline_count"`
	PolicyMismatchCount        int                    `json:"policy_mismatch_count"`
	MissingCompareBaselines    []BaselineAuditFinding `json:"missing_compare_baselines,omitempty"`
	UnexpectedBaselinesForNone []BaselineAuditFinding `json:"unexpected_baselines_for_none,omitempty"`
	OrphanBaselines            []BaselineAuditFinding `json:"orphan_baselines,omitempty"`
	StaleSuiteHashes           []BaselineAuditFinding `json:"stale_suite_hashes,omitempty"`
}

type BaselineAuditFinding struct {
	SuiteID      string `json:"suite_id,omitempty"`
	SuitePath    string `json:"suite_path,omitempty"`
	BaselinePath string `json:"baseline_path,omitempty"`
	Mode         string `json:"mode,omitempty"`
	ExpectedSHA  string `json:"expected_sha,omitempty"`
	ActualSHA    string `json:"actual_sha,omitempty"`
}

func AuditBaselinePolicy(opts BaselineAuditOptions) (*BaselineAuditReport, error) {
	paths, err := coverageSuitePaths(CoverageOptions{
		SuitePaths: opts.SuitePaths,
		Roots:      opts.Roots,
	})
	if err != nil {
		return nil, err
	}
	baselines, err := discoverBaselines(opts.BaselineDir)
	if err != nil {
		return nil, err
	}
	report := &BaselineAuditReport{
		SuiteCount:    len(paths),
		BaselineCount: len(baselines),
	}
	seenSuites := map[string]struct{}{}
	for _, path := range paths {
		suite, data, err := LoadSuite(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		seenSuites[suite.ID] = struct{}{}
		baselinePath, hasBaseline := baselines[suite.ID]
		mode := strings.TrimSpace(suite.BaselinePolicy.Mode)
		switch mode {
		case "compare":
			if !hasBaseline {
				report.MissingCompareBaselines = append(report.MissingCompareBaselines, BaselineAuditFinding{
					SuiteID:   suite.ID,
					SuitePath: path,
					Mode:      mode,
				})
			}
		case "none", "":
			if hasBaseline {
				report.UnexpectedBaselinesForNone = append(report.UnexpectedBaselinesForNone, BaselineAuditFinding{
					SuiteID:      suite.ID,
					SuitePath:    path,
					BaselinePath: baselinePath,
					Mode:         mode,
				})
			}
		}
		if hasBaseline {
			report.StaleSuiteHashes = append(report.StaleSuiteHashes, staleSuiteHashFinding(suite.ID, path, baselinePath, sha256Hex(data))...)
		}
	}
	for suiteID, baselinePath := range baselines {
		if _, ok := seenSuites[suiteID]; !ok {
			report.OrphanBaselines = append(report.OrphanBaselines, BaselineAuditFinding{
				SuiteID:      suiteID,
				BaselinePath: baselinePath,
			})
		}
	}
	sortBaselineAuditFindings(report.MissingCompareBaselines)
	sortBaselineAuditFindings(report.UnexpectedBaselinesForNone)
	sortBaselineAuditFindings(report.OrphanBaselines)
	sortBaselineAuditFindings(report.StaleSuiteHashes)
	report.PolicyMismatchCount = len(report.MissingCompareBaselines) + len(report.UnexpectedBaselinesForNone) + len(report.OrphanBaselines)
	return report, nil
}

func discoverBaselines(dir string) (map[string]string, error) {
	baselines := map[string]string{}
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return baselines, nil
	}
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return baselines, nil
		}
		return nil, fmt.Errorf("inspect baseline dir %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("baseline dir %s is not a directory", dir)
	}
	err = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".baseline.json") {
			return nil
		}
		suiteID := strings.TrimSuffix(entry.Name(), ".baseline.json")
		baselines[suiteID] = path
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk baseline dir %s: %w", dir, err)
	}
	return baselines, nil
}

func staleSuiteHashFinding(suiteID, suitePath, baselinePath, actualSHA string) []BaselineAuditFinding {
	baseline, err := LoadRun(baselinePath)
	if err != nil || strings.TrimSpace(baseline.Suite.SHA256) == "" || baseline.Suite.SHA256 == actualSHA {
		return nil
	}
	return []BaselineAuditFinding{{
		SuiteID:      suiteID,
		SuitePath:    suitePath,
		BaselinePath: baselinePath,
		ExpectedSHA:  baseline.Suite.SHA256,
		ActualSHA:    actualSHA,
	}}
}

func sortBaselineAuditFindings(findings []BaselineAuditFinding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].SuiteID != findings[j].SuiteID {
			return findings[i].SuiteID < findings[j].SuiteID
		}
		return findings[i].BaselinePath < findings[j].BaselinePath
	})
}
