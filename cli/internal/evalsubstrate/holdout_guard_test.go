package evalsubstrate

import "testing"

// TestScanForbiddenKeys_DetectsLeakKeys: the key-name backstop must trip on any
// object key matching the holdout-leak pattern, at any nesting depth, regardless
// of case — this is the structural defense that catches a leak via payload shape
// even when no known holdout value is supplied to ContainsAny.
func TestScanForbiddenKeys_DetectsLeakKeys(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		wantKey string
	}{
		{"top-level target", `{"target":"Paris"}`, "target"},
		{"nested ground_truth", `{"criteria":[{"id":"a","ground_truth_id":"gt-7"}]}`, "ground_truth_id"},
		{"nested expected_output", `{"criteria":[{"expected_output":"x"}]}`, "expected_output"},
		{"case-insensitive Answer", `{"Answer":"42"}`, "Answer"},
		{"substring answer_key", `{"criteria":[{"answer_key":"k"}]}`, "answer_key"},
		{"deep nesting", `{"a":{"b":{"c":{"holdout_target":1}}}}`, "holdout_target"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key, found := ScanForbiddenKeys([]byte(tc.payload))
			if !found {
				t.Fatalf("ScanForbiddenKeys(%s) = (%q, false), want a forbidden key", tc.payload, key)
			}
			if key != tc.wantKey {
				t.Errorf("ScanForbiddenKeys(%s) key = %q, want %q", tc.payload, key, tc.wantKey)
			}
		})
	}
}

// TestScanForbiddenKeys_CleanPayload: a correctly allowlisted rubric payload — and
// any object whose keys are all benign — must pass cleanly. The allowlisted keys
// (criteria/id/description/weight/...) deliberately do not match the pattern.
func TestScanForbiddenKeys_CleanPayload(t *testing.T) {
	clean := []string{
		`{"schema_version":1,"source_task_id":"t1","judge_content_hash":"sha256:x","criteria":[{"id":"accuracy","description":"correct","weight":0.7}]}`,
		`{}`,
		`{"criteria":[]}`,
		`[1,2,3]`,
	}
	for _, p := range clean {
		if key, found := ScanForbiddenKeys([]byte(p)); found {
			t.Errorf("ScanForbiddenKeys(%s) = (%q, true), want no forbidden key", p, key)
		}
	}
}

// TestScanForbiddenKeys_Unparseable: malformed JSON returns ("", false) — the
// marshal/parse step is the caller's gate, not this guard's job to error on.
func TestScanForbiddenKeys_Unparseable(t *testing.T) {
	if key, found := ScanForbiddenKeys([]byte("not json{")); found {
		t.Errorf("ScanForbiddenKeys(invalid) = (%q, true), want (\"\", false)", key)
	}
}

// TestRubric_HasForbiddenKey_CleanByConstruction: a Rubric built through the
// allowlisted projection can never carry a forbidden key. This pins the
// holdout-isolation invariant so a future struct field that smuggles ground
// truth would flip this assertion red.
func TestRubric_HasForbiddenKey_CleanByConstruction(t *testing.T) {
	task := Task{SchemaVersion: SchemaVersion, ID: "task-1", Description: "answer the question"}
	criteria := []Criterion{
		{ID: "accuracy", Description: "Names the correct city.", Weight: 0.7},
		{ID: "concision", Description: "One short sentence.", Weight: 0.3},
	}
	r := ProjectRubric(task, criteria, "sha256:abc")
	if key, found := r.HasForbiddenKey(); found {
		t.Fatalf("projected rubric carried forbidden key %q; allowlist invariant broken", key)
	}
}
