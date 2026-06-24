package vibecheck

import (
	"slices"
	"time"
)

// amnesiaWindow is the time window for detecting repeated work on the same file.
const amnesiaWindow = 60 * time.Minute

// amnesiaMinEdits is the minimum number of times a file must be modified
// within the window to trigger a finding.
const amnesiaMinEdits = 3

// DetectContextAmnesia flags files modified repeatedly within a short window.
// High edit churn MAY be context loss (re-doing work) OR legitimate forward
// iteration (TDD red-green-refactor, addressing review). The detector only sees
// the edit count, not which — so it reports the observable churn and prompts
// verification rather than asserting "lost context" it cannot determine.
// (age-c30w: the principled future signal is diff oscillation/reverts, not raw
// count; until then, honest framing avoids the false "lost context" claim.)
// fileEdit records a single edit to a file.
type fileEdit struct {
	ts  time.Time
	sha string
	msg string
}

// DetectContextAmnesia finds repeated rapid edits to the same file — high edit
// churn worth verifying (forward iteration vs rework), not an assertion of loss.
func DetectContextAmnesia(events []TimelineEvent) []Finding {
	if len(events) < amnesiaMinEdits {
		return nil
	}

	sorted := make([]TimelineEvent, len(events))
	copy(sorted, events)
	sortOldestFirst(sorted)

	fileEdits := buildFileEdits(sorted)

	var findings []Finding
	for file, edits := range fileEdits {
		if f, ok := detectAmnesiaInFile(file, edits); ok {
			findings = append(findings, f)
		}
	}
	return findings
}

// buildFileEdits groups events by file into timestamped edit lists.
func buildFileEdits(events []TimelineEvent) map[string][]fileEdit {
	result := make(map[string][]fileEdit)
	for _, ev := range events {
		for _, f := range ev.Files {
			result[f] = append(result[f], fileEdit{
				ts:  ev.Timestamp,
				sha: ev.SHA,
				msg: ev.Message,
			})
		}
	}
	return result
}

// detectAmnesiaInFile checks if a file was edited amnesiaMinEdits+ times within amnesiaWindow.
func detectAmnesiaInFile(file string, edits []fileEdit) (Finding, bool) {
	if len(edits) < amnesiaMinEdits {
		return Finding{}, false
	}

	slices.SortFunc(edits, func(a, b fileEdit) int {
		return a.ts.Compare(b.ts)
	})

	for i := range len(edits) - amnesiaMinEdits + 1 {
		windowEnd := edits[i].ts.Add(amnesiaWindow)
		count := 0
		for j := i; j < len(edits); j++ {
			if edits[j].ts.After(windowEnd) {
				break
			}
			count++
		}
		if count >= amnesiaMinEdits {
			return Finding{
				Severity: SeverityWarning,
				Category: "context-amnesia",
				Message:  file + " modified " + itoa(count) + " times within 1 hour — high edit churn; confirm it's forward iteration, not rework",
				File:     file,
			}, true
		}
	}
	return Finding{}, false
}

// itoa converts an int to a string without importing strconv in this file.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
