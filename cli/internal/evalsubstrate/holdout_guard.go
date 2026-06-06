package evalsubstrate

import (
	"encoding/json"
	"regexp"
)

// forbiddenKeyPattern matches any JSON object key (at any nesting depth) that
// would signal a holdout ground-truth leak in an Outcomes payload. These are the
// shapes a leaking answer travels under — target/expected_output values,
// ground_truth ids, or a raw answer field. The match is case-insensitive and
// substring-wide on purpose: a key like "expected_output" or "answer_key" must
// trip it, not just an exact "answer".
var forbiddenKeyPattern = regexp.MustCompile(`(?i)target|ground_truth|expected_output|answer`)

// ScanForbiddenKeys re-parses an already-marshaled Outcomes payload and reports
// the first object key (at any nesting depth) that matches a holdout-leak
// pattern (target|ground_truth|expected_output|answer, case-insensitive), or
// ("", false) when no key matches. This is the structural, key-name backstop
// that complements the value-based ContainsAny scan: ContainsAny catches a known
// holdout *value* echoed into a description; ScanForbiddenKeys catches a leak via
// the payload *shape* if the projection ever drifts to carry a ground-truth
// field. Both run before a payload crosses the cloud boundary, because Managed
// Agents are not ZDR and a leak there is permanent. Unparseable input returns
// ("", false) — callers gate on the marshal step separately.
func ScanForbiddenKeys(payload []byte) (string, bool) {
	var v interface{}
	if err := json.Unmarshal(payload, &v); err != nil {
		return "", false
	}
	return walkForbiddenKeys(v)
}

// walkForbiddenKeys recursively descends a decoded JSON value, returning the
// first object key that matches forbiddenKeyPattern.
func walkForbiddenKeys(v interface{}) (string, bool) {
	switch t := v.(type) {
	case map[string]interface{}:
		for k, child := range t {
			if forbiddenKeyPattern.MatchString(k) {
				return k, true
			}
			if hit, found := walkForbiddenKeys(child); found {
				return hit, true
			}
		}
	case []interface{}:
		for _, child := range t {
			if hit, found := walkForbiddenKeys(child); found {
				return hit, true
			}
		}
	}
	return "", false
}

// HasForbiddenKey marshals the Rubric and scans the result for any holdout-leak
// key (defense-in-depth backstop, ag-uux93). It complements ContainsAny, which
// scans for known holdout *values*: this catches a leak via a *key name* if the
// payload shape ever drifts to carry one. A correctly allowlisted Rubric never
// trips it — that invariant is exactly what the guard pins under refactors.
func (r Rubric) HasForbiddenKey() (string, bool) {
	blob, err := json.Marshal(r)
	if err != nil {
		return "", false
	}
	return ScanForbiddenKeys(blob)
}
