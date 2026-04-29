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
	"security",
}

var DefaultCoverageDimensions = []string{
	string(DimensionCorrectness),
	string(DimensionProcessAdherence),
	string(DimensionArtifactQuality),
	string(DimensionRuntimeCompatibility),
	string(DimensionEfficiency),
	string(DimensionSafety),
	string(DimensionLearningClosure),
}

var DefaultCoverageRuntimes = []string{
	string(RuntimeStatic),
	string(RuntimeShell),
	string(RuntimeMock),
}

var DefaultCoverageEvidenceKinds = []string{
	string(EvidenceKindContractCanary),
	string(EvidenceKindGateWrapper),
	string(EvidenceKindBehaviorFixture),
	string(EvidenceKindBaselineRegression),
	string(EvidenceKindScorecardFixture),
	string(EvidenceKindLiveRuntime),
	string(EvidenceKindHoldout),
}

type CoverageOptions struct {
	SuitePaths            []string
	Roots                 []string
	RequiredDomains       []string
	RequiredDimensions    []string
	RequiredRuntimes      []string
	RequiredEvidenceKinds []string
}

type CoverageReport struct {
	SuiteCount                   int                       `json:"suite_count"`
	CaseCount                    int                       `json:"case_count"`
	CriticalCaseCount            int                       `json:"critical_case_count"`
	Suites                       []CoverageSuite           `json:"suites"`
	Domains                      map[string]CoverageBucket `json:"domains"`
	EvidenceKinds                map[string]CoverageBucket `json:"evidence_kinds"`
	Dimensions                   map[string]int            `json:"dimensions"`
	Runtimes                     map[string]int            `json:"runtimes"`
	RequiredDomains              []string                  `json:"required_domains,omitempty"`
	MissingRequiredDomains       []string                  `json:"missing_required_domains,omitempty"`
	RequiredEvidenceKinds        []string                  `json:"required_evidence_kinds,omitempty"`
	MissingRequiredEvidenceKinds []string                  `json:"missing_required_evidence_kinds,omitempty"`
	RequiredDimensions           []string                  `json:"required_dimensions,omitempty"`
	MissingRequiredDimensions    []string                  `json:"missing_required_dimensions,omitempty"`
	RequiredRuntimes             []string                  `json:"required_runtimes,omitempty"`
	MissingRequiredRuntimes      []string                  `json:"missing_required_runtimes,omitempty"`
}

type CoverageSuite struct {
	ID                string   `json:"id"`
	Domain            string   `json:"domain"`
	Tier              string   `json:"tier"`
	Visibility        string   `json:"visibility"`
	EvidenceKinds     []string `json:"evidence_kinds"`
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
		Domains:       map[string]CoverageBucket{},
		EvidenceKinds: map[string]CoverageBucket{},
		Dimensions:    map[string]int{},
		Runtimes:      map[string]int{},
	}
	for _, path := range paths {
		suite, _, err := LoadSuite(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		addSuiteCoverage(report, suite)
	}
	report.RequiredDomains = normalizedCoverageValues(opts.RequiredDomains)
	report.MissingRequiredDomains = missingCoverageDomains(report.Domains, report.RequiredDomains)
	report.RequiredEvidenceKinds = normalizedCoverageValues(opts.RequiredEvidenceKinds)
	report.MissingRequiredEvidenceKinds = missingCoverageBuckets(report.EvidenceKinds, report.RequiredEvidenceKinds)
	report.RequiredDimensions = normalizedCoverageValues(opts.RequiredDimensions)
	report.MissingRequiredDimensions = missingCoverageDimensions(report.Dimensions, report.RequiredDimensions)
	report.RequiredRuntimes = normalizedCoverageValues(opts.RequiredRuntimes)
	report.MissingRequiredRuntimes = missingCoverageValues(report.Runtimes, report.RequiredRuntimes)
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
	addEvidenceKindCoverage(report, suite)
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
	evidenceKinds := coverageEvidenceKinds(suite)
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
		EvidenceKinds:     evidenceKinds,
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

func addEvidenceKindCoverage(report *CoverageReport, suite *Suite) {
	suiteKinds := map[string]struct{}{}
	caseBuckets := map[string]CoverageBucket{}
	for _, evalCase := range suite.Cases {
		kind := string(resolveEvidenceKind(suite, evalCase))
		suiteKinds[kind] = struct{}{}
		bucket := caseBuckets[kind]
		bucket.CaseCount++
		if evalCase.Critical {
			bucket.CriticalCaseCount++
		}
		caseBuckets[kind] = bucket
	}
	for kind, caseBucket := range caseBuckets {
		bucket := report.EvidenceKinds[kind]
		bucket.CaseCount += caseBucket.CaseCount
		bucket.CriticalCaseCount += caseBucket.CriticalCaseCount
		if _, ok := suiteKinds[kind]; ok {
			bucket.SuiteCount++
		}
		report.EvidenceKinds[kind] = bucket
	}
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

func coverageEvidenceKinds(suite *Suite) []string {
	seen := map[string]struct{}{}
	for _, evalCase := range suite.Cases {
		seen[string(resolveEvidenceKind(suite, evalCase))] = struct{}{}
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

func resolveEvidenceKind(suite *Suite, evalCase Case) EvidenceKind {
	if evalCase.EvidenceKind != "" {
		return evalCase.EvidenceKind
	}
	if suite.EvidenceKind != "" {
		return suite.EvidenceKind
	}
	marker := evidenceKindMarker(suite, evalCase)
	switch {
	case strings.Contains(marker, "baseline-regression") || strings.Contains(marker, "baseline regression"):
		return EvidenceKindBaselineRegression
	case strings.Contains(marker, "holdout"):
		return EvidenceKindHoldout
	case strings.Contains(marker, "scorecard"):
		return EvidenceKindScorecardFixture
	case isLiveEvidence(marker, evalCase.Runtime):
		return EvidenceKindLiveRuntime
	case evalCase.Kind == "scenario" || evalCase.Kind == "rpi_flow" || strings.Contains(marker, "behavior"):
		return EvidenceKindBehaviorFixture
	case isGateWrapperCase(evalCase.Kind):
		return EvidenceKindGateWrapper
	default:
		return EvidenceKindContractCanary
	}
}

func isLiveEvidence(marker string, runtime Runtime) bool {
	if strings.Contains(marker, "live-runtime") || strings.Contains(marker, "live runtime") {
		return true
	}
	switch runtime {
	case RuntimeClaude, RuntimeCodex, RuntimeManual:
		return true
	default:
		return false
	}
}

func isGateWrapperCase(kind string) bool {
	switch kind {
	case "command", "hook_event", "retrieval_query":
		return true
	default:
		return false
	}
}

func evidenceKindMarker(suite *Suite, evalCase Case) string {
	parts := []string{
		suite.ID,
		suite.Name,
		suite.Description,
		evalCase.ID,
		evalCase.Title,
		evalCase.Kind,
		evalCase.Objective,
	}
	parts = append(parts, suite.Tags...)
	parts = append(parts, evalCase.Tags...)
	return strings.ToLower(strings.Join(parts, " "))
}

func normalizedCoverageValues(values []string) []string {
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
	return missingCoverageBuckets(domains, required)
}

func missingCoverageBuckets(buckets map[string]CoverageBucket, required []string) []string {
	var missing []string
	for _, domain := range required {
		if buckets[domain].SuiteCount == 0 {
			missing = append(missing, domain)
		}
	}
	sort.Strings(missing)
	return missing
}

func missingCoverageDimensions(dimensions map[string]int, required []string) []string {
	return missingCoverageValues(dimensions, required)
}

func missingCoverageValues(counts map[string]int, required []string) []string {
	var missing []string
	for _, value := range required {
		if counts[value] == 0 {
			missing = append(missing, value)
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
