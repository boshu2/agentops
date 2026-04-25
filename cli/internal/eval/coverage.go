package eval

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var DefaultCoverageDomains = []string{
	"cli",
	"hook",
	"skill",
	"rpi",
	"runtime",
	"retrieval",
	"scenario",
	"mixed",
}

type CoverageOptions struct {
	SuitePaths      []string
	Roots           []string
	RequiredDomains []string
}

type CoverageReport struct {
	SuiteCount             int                       `json:"suite_count"`
	CaseCount              int                       `json:"case_count"`
	CriticalCaseCount      int                       `json:"critical_case_count"`
	Suites                 []CoverageSuite           `json:"suites"`
	Domains                map[string]CoverageBucket `json:"domains"`
	Dimensions             map[string]int            `json:"dimensions"`
	Runtimes               map[string]int            `json:"runtimes"`
	RequiredDomains        []string                  `json:"required_domains,omitempty"`
	MissingRequiredDomains []string                  `json:"missing_required_domains,omitempty"`
}

type CoverageSuite struct {
	ID                string   `json:"id"`
	Domain            string   `json:"domain"`
	Tier              string   `json:"tier"`
	Visibility        string   `json:"visibility"`
	CaseCount         int      `json:"case_count"`
	CriticalCaseCount int      `json:"critical_case_count"`
	Dimensions        []string `json:"dimensions"`
	Runtimes          []string `json:"runtimes"`
}

type CoverageBucket struct {
	SuiteCount        int `json:"suite_count"`
	CaseCount         int `json:"case_count"`
	CriticalCaseCount int `json:"critical_case_count"`
}

func BuildCoverageReport(opts CoverageOptions) (*CoverageReport, error) {
	paths, err := coverageSuitePaths(opts)
	if err != nil {
		return nil, err
	}
	report := &CoverageReport{
		Domains:    map[string]CoverageBucket{},
		Dimensions: map[string]int{},
		Runtimes:   map[string]int{},
	}
	for _, path := range paths {
		suite, _, err := LoadSuite(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		addSuiteCoverage(report, suite)
	}
	report.RequiredDomains = normalizedCoverageDomains(opts.RequiredDomains)
	report.MissingRequiredDomains = missingCoverageDomains(report.Domains, report.RequiredDomains)
	return report, nil
}

func coverageSuitePaths(opts CoverageOptions) ([]string, error) {
	seen := map[string]struct{}{}
	var paths []string
	for _, path := range opts.SuitePaths {
		addCoveragePath(&paths, seen, path)
	}
	for _, root := range opts.Roots {
		discovered, err := discoverCoverageSuites(root)
		if err != nil {
			return nil, err
		}
		for _, path := range discovered {
			addCoveragePath(&paths, seen, path)
		}
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return nil, fmt.Errorf("no eval suites found")
	}
	return paths, nil
}

func discoverCoverageSuites(root string) ([]string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, nil
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("inspect eval coverage root %s: %w", root, err)
	}
	if !info.IsDir() {
		return []string{root}, nil
	}
	var paths []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk eval coverage root %s: %w", root, err)
	}
	sort.Strings(paths)
	return paths, nil
}

func addCoveragePath(paths *[]string, seen map[string]struct{}, path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	if _, ok := seen[path]; ok {
		return
	}
	seen[path] = struct{}{}
	*paths = append(*paths, path)
}

func addSuiteCoverage(report *CoverageReport, suite *Suite) {
	summary := coverageSuiteSummary(suite)
	report.Suites = append(report.Suites, summary)
	report.SuiteCount++
	report.CaseCount += summary.CaseCount
	report.CriticalCaseCount += summary.CriticalCaseCount
	addDomainCoverage(report, summary)
	for _, dim := range summary.Dimensions {
		report.Dimensions[dim]++
	}
	for _, runtime := range summary.Runtimes {
		report.Runtimes[runtime]++
	}
}

func coverageSuiteSummary(suite *Suite) CoverageSuite {
	dimensions := coverageDimensions(suite)
	runtimes := coverageRuntimes(suite)
	critical := 0
	for _, evalCase := range suite.Cases {
		if evalCase.Critical {
			critical++
		}
	}
	return CoverageSuite{
		ID:                suite.ID,
		Domain:            suite.Domain,
		Tier:              string(suite.Tier),
		Visibility:        string(suite.Visibility),
		CaseCount:         len(suite.Cases),
		CriticalCaseCount: critical,
		Dimensions:        dimensions,
		Runtimes:          runtimes,
	}
}

func addDomainCoverage(report *CoverageReport, suite CoverageSuite) {
	bucket := report.Domains[suite.Domain]
	bucket.SuiteCount++
	bucket.CaseCount += suite.CaseCount
	bucket.CriticalCaseCount += suite.CriticalCaseCount
	report.Domains[suite.Domain] = bucket
}

func coverageDimensions(suite *Suite) []string {
	seen := map[string]struct{}{}
	for _, dim := range suite.Scoring.Dimensions {
		seen[string(dim.Name)] = struct{}{}
	}
	for _, evalCase := range suite.Cases {
		for _, dim := range evalCase.Dimensions {
			seen[string(dim)] = struct{}{}
		}
	}
	return sortedCoverageKeys(seen)
}

func coverageRuntimes(suite *Suite) []string {
	seen := map[string]struct{}{}
	for _, runtime := range suite.Allowed {
		seen[string(runtime)] = struct{}{}
	}
	for _, evalCase := range suite.Cases {
		if evalCase.Runtime != "" {
			seen[string(evalCase.Runtime)] = struct{}{}
		}
	}
	if len(seen) == 0 {
		seen[string(inferRuntime(*suite))] = struct{}{}
	}
	return sortedCoverageKeys(seen)
}

func normalizedCoverageDomains(values []string) []string {
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		seen[value] = struct{}{}
	}
	return sortedCoverageKeys(seen)
}

func missingCoverageDomains(domains map[string]CoverageBucket, required []string) []string {
	var missing []string
	for _, domain := range required {
		if domains[domain].SuiteCount == 0 {
			missing = append(missing, domain)
		}
	}
	sort.Strings(missing)
	return missing
}

func sortedCoverageKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
