package vibecheck

import (
	"regexp"
	"strings"
	"time"
)

// successClaimPatterns matches commit messages that claim success.
var successClaimPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bfixed\s+(it|the|this|that|bug|issue|problem)`),
	regexp.MustCompile(`(?i)\bworking\b`),
	regexp.MustCompile(`(?i)\bdone\b`),
	regexp.MustCompile(`(?i)\bcomplete[ds]?\b`),
	regexp.MustCompile(`(?i)\b(bug|issue|problem)\s+resolved\b`),
	regexp.MustCompile(`(?i)\btests?\s+pass(es|ing)?\b`),
	regexp.MustCompile(`(?i)\bgreen\b`),
	regexp.MustCompile(`(?i)\bsuccessful(ly)?\b`),
	regexp.MustCompile(`(?i)\ball\s+tests?\b`),
	regexp.MustCompile(`(?i)\bready\b`),
	regexp.MustCompile(`(?i)\bshould\s+work`),
	regexp.MustCompile(`(?i)\bnow\s+works?\b`),
	regexp.MustCompile(`(?i)\bit\s+works?\b`),
}

// tentativePatterns marks commits as uncertain, excluding them from lies.
var tentativePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\btry\b`),
	regexp.MustCompile(`(?i)\battempt`),
	regexp.MustCompile(`(?i)\bmaybe\b`),
	regexp.MustCompile(`(?i)\bwip\b`),
	regexp.MustCompile(`(?i)\bwork\s*in\s*progress`),
	regexp.MustCompile(`(?i)\bexperiment`),
	regexp.MustCompile(`(?i)\btest(ing)?\b`),
	regexp.MustCompile(`(?i)\bdebug`),
	regexp.MustCompile(`(?i)\binvestigat`),
}

// followUpWindow is the max time after a success claim to look for fix commits.
const followUpWindow = 30 * time.Minute

// claimsSuccess returns true if the message matches any success claim pattern.
func claimsSuccess(msg string) bool {
	for _, p := range successClaimPatterns {
		if p.MatchString(msg) {
			return true
		}
	}
	return false
}

// isTentative returns true if the message contains tentative language.
func isTentative(msg string) bool {
	for _, p := range tentativePatterns {
		if p.MatchString(msg) {
			return true
		}
	}
	return false
}

// isFixMessage returns true if the commit message looks like a fix.
func isFixMessage(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.HasPrefix(lower, "fix") ||
		strings.Contains(lower, "fix:") ||
		strings.Contains(lower, "bugfix") ||
		strings.Contains(lower, "hotfix") ||
		strings.Contains(lower, "patch:")
}

// hasFileOverlap returns true if two file lists share any entry.
func hasFileOverlap(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(a))
	for _, f := range a {
		set[f] = struct{}{}
	}
	for _, f := range b {
		if _, ok := set[f]; ok {
			return true
		}
	}
	return false
}

// DetectTestsLie detects commits that claim success but are quickly followed
// by fix commits on the same files. This suggests the claim was premature.
func DetectTestsLie(events []TimelineEvent) []Finding {
	if len(events) < 2 {
		return nil
	}

	// Sort oldest first for forward scanning.
	sorted := make([]TimelineEvent, len(events))
	copy(sorted, events)
	sortOldestFirst(sorted)

	var findings []Finding
	for i, ev := range sorted {
		if !claimsSuccess(ev.Message) || isTentative(ev.Message) {
			continue
		}
		if f, ok := findContradictingFix(ev, sorted[i+1:]); ok {
			findings = append(findings, f)
		}
	}
	return findings
}

// filesRelated returns true only if two events share at least one file. When
// either file list is empty (an empty commit, or file data unavailable), the
// same-file relation CANNOT be confirmed, so it returns false — a fix only
// "contradicts" a success claim if it provably touches the same file (the
// detector's documented contract). The prior empty/empty => true fail-open
// over-correlated temporally-adjacent but unrelated commits whenever file data
// was missing (age-mr1c). hasFileOverlap already returns false on any empty list.
func filesRelated(a, b []string) bool {
	return hasFileOverlap(a, b)
}

// findContradictingFix scans subsequent events for a fix that contradicts the claimed success.
func findContradictingFix(claim TimelineEvent, following []TimelineEvent) (Finding, bool) {
	for _, next := range following {
		gap := next.Timestamp.Sub(claim.Timestamp)
		if gap > followUpWindow {
			break
		}
		if !isFixMessage(next.Message) {
			continue
		}
		if filesRelated(claim.Files, next.Files) {
			return Finding{
				Severity: SeverityCritical,
				Category: "tests-passing-lie",
				Message:  "Claimed success (" + claim.Message + ") but fix followed within " + gap.String() + ": " + next.Message,
			}, true
		}
	}
	return Finding{}, false
}
