// practices: [agile-manifesto, dora-metrics]
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/rpi"
	"github.com/boshu2/agentops/cli/internal/types"
)

func classifyByPhase(phaseNum int, verdict string) types.MemRLFailureClass {
	return rpi.ClassifyByPhase(phaseNum, verdict)
}

func classifyByVerdict(verdict string) types.MemRLFailureClass {
	return rpi.ClassifyByVerdict(verdict)
}

// --- Verdict extraction helpers ---

// extractCouncilVerdict reads a council report and returns the verdict (PASS/WARN/FAIL).
func extractCouncilVerdict(reportPath string) (string, error) {
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return "", fmt.Errorf("read report: %w", err)
	}

	re := regexp.MustCompile(`(?m)^## Council Verdict:\s*(PASS|WARN|FAIL)`)
	matches := re.FindSubmatch(data)
	if len(matches) < 2 {
		return "", fmt.Errorf("no verdict found in %s", reportPath)
	}
	return string(matches[1]), nil
}

// findLatestCouncilReport finds the most recent council report matching a pattern.
// When epicID is non-empty, reports whose filename contains the epicID are preferred.
// If no epic-scoped report is found, all pattern-matching reports are used as fallback.
func findLatestCouncilReport(cwd string, pattern string, notBefore time.Time, epicID string) (string, error) {
	councilDir := filepath.Join(cwd, ".agents", "council")
	entries, err := os.ReadDir(councilDir)
	if err != nil {
		return "", fmt.Errorf("read council directory: %w", err)
	}

	var matches []string
	var epicMatches []string
	for _, entry := range entries {
		fullPath, ok := matchCouncilEntry(entry, councilDir, pattern, notBefore)
		if !ok {
			continue
		}
		matches = append(matches, fullPath)
		if epicID != "" && strings.Contains(entry.Name(), epicID) {
			epicMatches = append(epicMatches, fullPath)
		}
	}

	selected := matches
	if len(epicMatches) > 0 {
		selected = epicMatches
	}

	if len(selected) == 0 {
		return "", fmt.Errorf("no council report matching %q found", pattern)
	}

	sort.Strings(selected)

	return selected[len(selected)-1], nil
}

func matchCouncilEntry(entry os.DirEntry, councilDir, pattern string, notBefore time.Time) (string, bool) {
	if entry.IsDir() {
		return "", false
	}
	name := entry.Name()
	if !strings.Contains(name, pattern) || !strings.HasSuffix(name, ".md") {
		return "", false
	}
	if !notBefore.IsZero() {
		info, err := entry.Info()
		if err != nil || info.ModTime().Before(notBefore) {
			return "", false
		}
	}
	return filepath.Join(councilDir, name), true
}

// extractCouncilFindings extracts structured findings from a council report.
func extractCouncilFindings(reportPath string, max int) ([]finding, error) {
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return nil, fmt.Errorf("read report: %w", err)
	}

	// Look for structured findings: FINDING: ... | FIX: ... | REF: ...
	re := regexp.MustCompile(`(?m)FINDING:\s*(.+?)\s*\|\s*FIX:\s*(.+?)\s*\|\s*REF:\s*(.+?)$`)
	allMatches := re.FindAllSubmatch(data, -1)

	var findings []finding
	for i, m := range allMatches {
		if i >= max {
			break
		}
		findings = append(findings, finding{
			Description: string(m[1]),
			Fix:         string(m[2]),
			Ref:         string(m[3]),
		})
	}

	// Fallback: if no structured findings, extract from "## Shared Findings" section
	if len(findings) == 0 {
		re2 := regexp.MustCompile(`(?m)^\d+\.\s+\*\*(.+?)\*\*\s*[—–-]\s*(.+)$`)
		allMatches2 := re2.FindAllSubmatch(data, -1)
		for i, m := range allMatches2 {
			if i >= max {
				break
			}
			findings = append(findings, finding{
				Description: string(m[1]) + ": " + string(m[2]),
				Fix:         "See council report",
				Ref:         reportPath,
			})
		}
	}

	return findings, nil
}
