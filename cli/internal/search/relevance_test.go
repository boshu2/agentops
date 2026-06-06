package search

import "testing"

// ag-32gx: QueryTokens must strip stopwords + punctuation so a verbose query (a whole
// task prompt) reduces to its salient terms. Without this, MatchRatio (matched/total)
// collapses for long queries and ranking falls back to generic freshness (the corpus-delta
// W1c retrieval-relevance gap).

func TestQueryTokens_StripsStopwordsAndPunctuation(t *testing.T) {
	got := QueryTokens("if any job is continue-on-error:true and it is in the summary.needs list")
	gotSet := map[string]bool{}
	for _, tk := range got {
		gotSet[tk] = true
	}
	// stopwords must be gone
	for _, sw := range []string{"if", "any", "is", "and", "it", "in", "the"} {
		if gotSet[sw] {
			t.Errorf("stopword %q should be stripped, got tokens %v", sw, got)
		}
	}
	// salient terms must survive (punctuation split: continue-on-error -> continue/error)
	for _, want := range []string{"job", "continue", "error", "true", "summary", "needs", "list"} {
		if !gotSet[want] {
			t.Errorf("salient token %q missing from %v", want, got)
		}
	}
	// dedup: "is"/"the" appear twice in input but are stopwords anyway; ensure no dupes
	seen := map[string]bool{}
	for _, tk := range got {
		if seen[tk] {
			t.Errorf("duplicate token %q in %v", tk, got)
		}
		seen[tk] = true
	}
}

// A verbose query (with stopwords) must still rank a learning matching its SALIENT terms
// above one that matches none — the directional fix for the retrieval-relevance gap.
func TestMatchRatio_VerboseQueryRanksRelevantAboveGeneric(t *testing.T) {
	q := QueryTokens("write a gate that fails when a continue-on-error job is an advisory PR check")
	relevant := MatchRatio(q, "CI advisory tier killed",
		"required or informational; nothing between",
		"a continue-on-error job must not be an advisory PR check")
	generic := MatchRatio(q, "auto til flight is boring",
		"journal note", "unrelated musing about essays and framing")
	if !(relevant > generic) {
		t.Errorf("relevant learning ratio (%.3f) must exceed generic (%.3f) for a verbose query", relevant, generic)
	}
	if generic != 0 {
		t.Errorf("generic learning should match no salient tokens, got ratio %.3f", generic)
	}
}

// Backstop: an all-stopword query yields no tokens -> MatchRatio returns 1.0 (no relevance
// signal -> fall back to freshness), preserving prior behavior for degenerate queries.
func TestQueryTokens_AllStopwordsYieldsEmpty(t *testing.T) {
	if got := QueryTokens("the and or of to in on for"); len(got) != 0 {
		t.Errorf("all-stopword query should yield no tokens, got %v", got)
	}
}
