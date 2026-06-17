// practices: [capability-detection, hexagonal-architecture]
package orchestration

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// ProbeTools runs the orchestration tool matrix from toolsContract.
// FAIL only on contract violations (empty matrix); absent optional tools are WARN-level
// at the caller (preflight), not here.
func ProbeTools(ctx context.Context, contract ToolsContract, runner CommandRunner) ([]ToolReport, error) {
	if len(contract.Tools) == 0 {
		return nil, fmt.Errorf("tools contract: empty tool matrix")
	}
	reports := make([]ToolReport, 0, len(contract.Tools))
	for _, spec := range contract.Tools {
		report := ToolReport{ID: spec.ID}
		out, err := runner.Run(ctx, spec.Binary, spec.ProbeArgs...)
		if err != nil {
			report.Available = false
			reports = append(reports, report)
			continue
		}
		report.Available = true
		report.Version = extractVersion(string(out))
		if spec.ID == "ntm" {
			caps, capErr := ProbeNTM(ctx, runner)
			if capErr != nil {
				return nil, capErr
			}
			if !caps.Available {
				report.Available = false
			} else if len(caps.MissingDeps) > 0 {
				report.MissingDeps = caps.MissingDeps
			}
		}
		reports = append(reports, report)
	}
	return reports, nil
}

// ToolsForProfile filters tool reports to those required by profileID tags.
func ToolsForProfile(contract ToolsContract, profileID string, reports []ToolReport) []ToolReport {
	byID := map[string]ToolReport{}
	for _, r := range reports {
		byID[r.ID] = r
	}
	var out []ToolReport
	for _, spec := range contract.Tools {
		if !toolRequiredForProfile(spec, profileID) {
			continue
		}
		if r, ok := byID[spec.ID]; ok {
			out = append(out, r)
		} else {
			out = append(out, ToolReport{ID: spec.ID, Available: false})
		}
	}
	return out
}

func toolRequiredForProfile(spec ToolSpec, profileID string) bool {
	for _, tag := range spec.RequiredFor {
		if tag == profileID {
			return true
		}
	}
	return false
}

// VersionMeetsFloor returns true when discovered version satisfies minVersion.
// Uses naive dot-separated numeric comparison; non-parseable versions fail closed.
func VersionMeetsFloor(discovered, minVersion string) bool {
	discovered = strings.TrimSpace(discovered)
	minVersion = strings.TrimSpace(minVersion)
	if discovered == "" || minVersion == "" {
		return false
	}
	return compareVersionStrings(discovered, minVersion) >= 0
}

func compareVersionStrings(a, b string) int {
	aParts := versionParts(a)
	bParts := versionParts(b)
	n := len(aParts)
	if len(bParts) > n {
		n = len(bParts)
	}
	for i := 0; i < n; i++ {
		var av, bv int
		if i < len(aParts) {
			av = aParts[i]
		}
		if i < len(bParts) {
			bv = bParts[i]
		}
		if av > bv {
			return 1
		}
		if av < bv {
			return -1
		}
	}
	return 0
}

func versionParts(v string) []int {
	// Strip non-numeric suffix from first token (e.g. "1.2.3-beta").
	v = strings.Fields(v)[0]
	v = strings.TrimPrefix(v, "v")
	chunks := strings.Split(v, ".")
	var parts []int
	for _, c := range chunks {
		var n int
		for _, r := range c {
			if r < '0' || r > '9' {
				break
			}
			n = n*10 + int(r-'0')
		}
		parts = append(parts, n)
	}
	return parts
}

func extractVersion(out string) string {
	// atm/ntm often prints "ntm version v1.18.3-22-gHASH" — extract leading semver.
	re := regexp.MustCompile(`v?(\d+\.\d+\.\d+)`)
	if m := re.FindStringSubmatch(out); len(m) > 1 {
		return m[1]
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(lines[0])
}

// BuildToolsResult assembles the instrument result for ao orchestrate tools.
func BuildToolsResult(reports []ToolReport, runID string) InstrumentResult {
	checks := []CheckStatus{}
	status := VerdictStatusPass
	conf := VerdictConfidenceHigh
	for _, r := range reports {
		cs := CheckStatus{ID: "tool:" + r.ID, Status: VerdictStatusPass}
		if !r.Available {
			cs.Status = VerdictStatusWarn
			cs.Detail = "unavailable"
			status = VerdictStatusWarn
			conf = VerdictConfidenceMedium
		}
		checks = append(checks, cs)
	}
	return InstrumentResult{
		SchemaVersion: InstrumentSchemaVersionV1,
		Command:       InstrumentCommandTools,
		RunID:         runID,
		Verdict:       Verdict{Status: status, Confidence: conf},
		Tools:         reports,
		Checks:        checks,
	}
}
