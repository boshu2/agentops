package findingsynth

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

// TestKey_NormalizesCaseAndWhitespace proves the canonical key collides across
// case + whitespace differences and separates on a differing span.
func TestKey_NormalizesCaseAndWhitespace(t *testing.T) {
	f1 := Finding{Severity: "High", Title: "Null  Deref", File: "X.go", StartLine: 1, EndLine: 2}
	f2 := Finding{Severity: "  high ", Title: "null deref", File: "x.go", StartLine: 1, EndLine: 2}
	if Key(f1) != Key(f2) {
		t.Fatalf("case/whitespace variants must share a key:\n f1=%q\n f2=%q", Key(f1), Key(f2))
	}
	f3 := Finding{Severity: "High", Title: "Null  Deref", File: "X.go", StartLine: 3, EndLine: 4}
	if Key(f1) == Key(f3) {
		t.Fatalf("span-differing findings must not share a key: %q", Key(f1))
	}
}

// TestMerge_ThreeLanesOverlap_FamiliesUnionSorted proves three lanes reporting
// the same substance (different phrasing/case/whitespace) collapse to one finding
// whose families and lanes are the sorted-unique union, with first-seen (by lane
// id) identity + non-empty body preserved.
func TestMerge_ThreeLanesOverlap_FamiliesUnionSorted(t *testing.T) {
	lanes := []LaneFindings{
		{LaneID: "lane-gpt", Family: "gpt", Findings: []Finding{
			{Title: "  Null Pointer Deref  ", File: "pkg/x.go", StartLine: 10, EndLine: 12, Severity: "HIGH", Body: ""},
		}},
		{LaneID: "lane-claude", Family: "claude", Findings: []Finding{
			{Title: "Null pointer deref", File: "pkg/x.go", StartLine: 10, EndLine: 12, Severity: "high", Body: "nil deref at load"},
		}},
		{LaneID: "lane-gemini", Family: "gemini", Findings: []Finding{
			{Title: "null   POINTER deref", File: "pkg/x.go", StartLine: 10, EndLine: 12, Severity: "High", Body: "gemini says nil"},
		}},
	}
	out := Merge(lanes)
	if len(out) != 1 {
		t.Fatalf("want 1 merged finding, got %d: %+v", len(out), out)
	}
	got := out[0]
	if want := []string{"claude", "gemini", "gpt"}; !reflect.DeepEqual(got.Families, want) {
		t.Errorf("families: got %v want %v", got.Families, want)
	}
	if want := []string{"lane-claude", "lane-gemini", "lane-gpt"}; !reflect.DeepEqual(got.Lanes, want) {
		t.Errorf("lanes: got %v want %v", got.Lanes, want)
	}
	// First-seen by lane id (lane-claude sorts first) supplies identity + body.
	if got.Title != "Null pointer deref" {
		t.Errorf("title: got %q want %q", got.Title, "Null pointer deref")
	}
	if got.Severity != "high" {
		t.Errorf("severity: got %q want %q", got.Severity, "high")
	}
	if got.Body != "nil deref at load" {
		t.Errorf("body: got %q want %q (first-seen non-empty)", got.Body, "nil deref at load")
	}
}

// TestMerge_HigherCorroborationRanksFirst proves corroboration count dominates
// severity in the ordering: a 2/3-corroborated low outranks a 1/3 critical.
func TestMerge_HigherCorroborationRanksFirst(t *testing.T) {
	lanes := []LaneFindings{
		{LaneID: "lane-claude", Family: "claude", Findings: []Finding{
			{Title: "Race condition", File: "a.go", StartLine: 1, EndLine: 2, Severity: "low"},
		}},
		{LaneID: "lane-gemini", Family: "gemini", Findings: []Finding{
			{Title: "race CONDITION", File: "a.go", StartLine: 1, EndLine: 2, Severity: "low"},
		}},
		{LaneID: "lane-gpt", Family: "gpt", Findings: []Finding{
			{Title: "Integer overflow", File: "b.go", StartLine: 3, EndLine: 4, Severity: "critical"},
		}},
	}
	out := Merge(lanes)
	if len(out) != 2 {
		t.Fatalf("want 2 findings, got %d: %+v", len(out), out)
	}
	if out[0].Title != "Race condition" || len(out[0].Lanes) != 2 {
		t.Errorf("rank[0]: got title=%q lanes=%v; want the 2-lane race", out[0].Title, out[0].Lanes)
	}
	if out[1].Title != "Integer overflow" || len(out[1].Lanes) != 1 {
		t.Errorf("rank[1]: got title=%q lanes=%v; want the 1-lane overflow", out[1].Title, out[1].Lanes)
	}
}

// TestMerge_SeverityTiebreakWithinSameCorroboration proves that at equal
// corroboration the severity rank (critical>high>medium>low>unknown) orders.
func TestMerge_SeverityTiebreakWithinSameCorroboration(t *testing.T) {
	lanes := []LaneFindings{
		{LaneID: "lane-a", Family: "claude", Findings: []Finding{
			{Title: "low one", File: "a.go", StartLine: 1, EndLine: 1, Severity: "low"},
			{Title: "critical one", File: "b.go", StartLine: 1, EndLine: 1, Severity: "critical"},
			{Title: "medium one", File: "c.go", StartLine: 1, EndLine: 1, Severity: "medium"},
			{Title: "mystery one", File: "d.go", StartLine: 1, EndLine: 1, Severity: "spicy"},
		}},
	}
	out := Merge(lanes)
	gotOrder := make([]string, len(out))
	for i, f := range out {
		gotOrder[i] = f.Title
	}
	want := []string{"critical one", "medium one", "low one", "mystery one"}
	if !reflect.DeepEqual(gotOrder, want) {
		t.Errorf("severity order: got %v want %v", gotOrder, want)
	}
}

// TestMerge_SpanDifferingDoNotMerge proves same title/file/severity but a
// differing line span stays two distinct findings.
func TestMerge_SpanDifferingDoNotMerge(t *testing.T) {
	lanes := []LaneFindings{
		{LaneID: "lane-a", Family: "claude", Findings: []Finding{
			{Title: "Leak", File: "m.go", StartLine: 5, EndLine: 5, Severity: "medium"},
		}},
		{LaneID: "lane-b", Family: "gemini", Findings: []Finding{
			{Title: "Leak", File: "m.go", StartLine: 9, EndLine: 9, Severity: "medium"},
		}},
	}
	out := Merge(lanes)
	if len(out) != 2 {
		t.Fatalf("span-differing findings must not merge: got %d: %+v", len(out), out)
	}
	for _, f := range out {
		if len(f.Lanes) != 1 {
			t.Errorf("each finding stays single-lane: %q has lanes=%v", f.Title, f.Lanes)
		}
	}
}

// TestMerge_EmptyLanes_EmptyNonNil proves empty input yields a non-nil empty
// slice (marshals as [] not null).
func TestMerge_EmptyLanes_EmptyNonNil(t *testing.T) {
	for name, in := range map[string][]LaneFindings{
		"nil":       nil,
		"empty":     {},
		"emptyLane": {{LaneID: "lane-a", Family: "claude", Findings: nil}},
	} {
		out := Merge(in)
		if out == nil {
			t.Errorf("%s: want non-nil empty slice, got nil", name)
		}
		if len(out) != 0 {
			t.Errorf("%s: want len 0, got %d", name, len(out))
		}
	}
}

// TestMerge_DeterministicUnderPermutation proves the same lanes in any order
// marshal byte-identically, even with lanes contributing different bodies for the
// same key.
func TestMerge_DeterministicUnderPermutation(t *testing.T) {
	a := LaneFindings{LaneID: "a-claude", Family: "claude", Findings: []Finding{
		{Title: "Null deref", File: "x.go", StartLine: 10, EndLine: 12, Severity: "high", Body: "body-from-a"},
		{Title: "SQL inject", File: "y.go", StartLine: 5, EndLine: 5, Severity: "critical", Body: "b2-a"},
	}}
	b := LaneFindings{LaneID: "b-gemini", Family: "gemini", Findings: []Finding{
		{Title: "null   DEREF", File: "x.go", StartLine: 10, EndLine: 12, Severity: "High", Body: "body-from-b"},
		{Title: "Race", File: "z.go", StartLine: 1, EndLine: 3, Severity: "medium", Body: "race-b"},
	}}
	c := LaneFindings{LaneID: "c-gpt", Family: "gpt", Findings: []Finding{
		{Title: "  Null Deref ", File: "x.go", StartLine: 10, EndLine: 12, Severity: "HIGH", Body: ""},
		{Title: "sql INJECT", File: "y.go", StartLine: 5, EndLine: 5, Severity: "critical", Body: "b2-c"},
	}}

	perms := [][]LaneFindings{
		{a, b, c},
		{c, b, a},
		{b, a, c},
		{c, a, b},
		{b, c, a},
		{a, c, b},
	}
	var golden []byte
	for i, p := range perms {
		got, err := json.Marshal(Merge(p))
		if err != nil {
			t.Fatalf("perm %d: marshal: %v", i, err)
		}
		if i == 0 {
			golden = got
			continue
		}
		if string(got) != string(golden) {
			t.Fatalf("perm %d not byte-identical:\n golden=%s\n got   =%s", i, golden, got)
		}
	}
	// Lock the canonical body: first-seen by lane id (a-claude) wins.
	out := Merge(perms[0])
	if out[0].Body != "body-from-a" {
		t.Errorf("canonical body: got %q want %q", out[0].Body, "body-from-a")
	}
}

// ExampleMerge documents the package contract as a runnable doc example: three
// cross-family lanes reporting one shared finding plus a singleton collapse to a
// deduplicated, attribution-carrying list ordered by corroboration.
func ExampleMerge() {
	lanes := []LaneFindings{
		{LaneID: "lane-gpt", Family: "gpt", Findings: []Finding{
			{Title: "TOCTOU on config", File: "cfg.go", StartLine: 8, EndLine: 8, Severity: "high"},
		}},
		{LaneID: "lane-claude", Family: "claude", Findings: []Finding{
			{Title: "toctou on CONFIG", File: "cfg.go", StartLine: 8, EndLine: 8, Severity: "high"},
			{Title: "Unchecked error", File: "cfg.go", StartLine: 20, EndLine: 20, Severity: "low"},
		}},
	}
	for _, f := range Merge(lanes) {
		fmt.Printf("%s | families=%v | lanes=%v\n", f.Title, f.Families, f.Lanes)
	}
	// Output:
	// toctou on CONFIG | families=[claude gpt] | lanes=[lane-claude lane-gpt]
	// Unchecked error | families=[claude] | lanes=[lane-claude]
}
