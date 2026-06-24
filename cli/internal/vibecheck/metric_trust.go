package vibecheck

import (
	"strings"
)

// MetricTrust computes the ratio of test-bearing commits to code-only commits.
// Higher is better. A commit is test-bearing if its MESSAGE references tests OR
// it TOUCHES a test file — the latter is essential under the AgentOps TDD pattern,
// where the test ships in the same feat:/fix: commit as the code (so a
// message-prefix-only heuristic undercounts test discipline to near zero).
// Threshold: >0.3 = good (passed). (age-mr1c)
func MetricTrust(events []TimelineEvent) Metric {
	if len(events) == 0 {
		return Metric{
			Name:      "trust",
			Value:     0,
			Threshold: 0.3,
			Passed:    false,
		}
	}

	testCommits := 0
	codeCommits := 0

	for _, e := range events {
		msg := strings.ToLower(strings.TrimSpace(e.Message))
		if isTestCommit(msg) || touchesTestFiles(e.Files) {
			testCommits++
		} else {
			codeCommits++
		}
	}

	if codeCommits == 0 {
		// All commits are test commits; trust is perfect.
		return Metric{
			Name:      "trust",
			Value:     1.0,
			Threshold: 0.3,
			Passed:    true,
		}
	}

	ratio := float64(testCommits) / float64(codeCommits)
	// Round to 2 decimal places.
	ratio = float64(int(ratio*100+0.5)) / 100.0

	return Metric{
		Name:      "trust",
		Value:     ratio,
		Threshold: 0.3,
		Passed:    ratio > 0.3,
	}
}

// touchesTestFiles returns true if any changed file is a test file across the
// languages in this repo (Go, Gherkin, bats, Python, JS/TS). This is what makes
// the trust metric TDD-aware: a feat:/fix: commit that ships its test in the same
// commit still counts toward test discipline.
func touchesTestFiles(files []string) bool {
	for _, f := range files {
		lower := strings.ToLower(f)
		base := lower
		if i := strings.LastIndexByte(lower, '/'); i >= 0 {
			base = lower[i+1:]
		}
		switch {
		case strings.HasSuffix(lower, "_test.go"),
			strings.HasSuffix(lower, ".feature"),
			strings.HasSuffix(lower, ".bats"),
			strings.HasSuffix(lower, "_test.py"),
			strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py"),
			strings.HasSuffix(lower, ".test.ts"), strings.HasSuffix(lower, ".test.tsx"),
			strings.HasSuffix(lower, ".spec.ts"), strings.HasSuffix(lower, ".spec.tsx"),
			strings.HasSuffix(lower, ".test.js"), strings.HasSuffix(lower, ".spec.js"),
			strings.Contains(lower, "/tests/"),
			strings.HasPrefix(lower, "tests/"):
			return true
		}
	}
	return false
}

// isTestCommit returns true if the commit message suggests test-related work.
func isTestCommit(msg string) bool {
	prefixes := []string{"test:", "test(", "tests:", "tests("}
	for _, p := range prefixes {
		if strings.HasPrefix(msg, p) {
			return true
		}
	}
	keywords := []string{"add test", "update test", "fix test", "write test", "testing"}
	for _, kw := range keywords {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}
