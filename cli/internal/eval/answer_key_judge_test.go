package eval

import (
	"context"
	"testing"

	"github.com/boshu2/agentops/cli/internal/scenario"
)

// TestAnswerKeyJudge_DeterministicMatch: the answer-key judge grades by exact
// case-insensitive substring presence — no LLM, zero judge noise. This is the
// rigorous grade for fact-recall OOD scenarios (age-k8u): with-gold (key present)
// scores 1.0, without-gold (key absent) scores 0.0 → a clean, valid delta.
func TestAnswerKeyJudge_DeterministicMatch(t *testing.T) {
	j := AnswerKeyJudge{}
	sc := scenario.Scenario{AnswerKey: "MTU 1500"}

	// present (case-insensitive) -> pass, score 1.0
	v, err := j.Judge(context.Background(), sc, ArmWithGold, ArmOutcome{Output: "the fix is to set mtu 1500 on eth0"})
	if err != nil {
		t.Fatal(err)
	}
	if v.AggregateScore != 1.0 || len(v.Vectors) != 1 || !v.Vectors[0].Pass {
		t.Errorf("present key should score 1.0 and pass; got %+v", v)
	}

	// absent -> fail, score 0.0
	v2, err := j.Judge(context.Background(), sc, ArmWithoutGold, ArmOutcome{Output: "I do not know that value"})
	if err != nil {
		t.Fatal(err)
	}
	if v2.AggregateScore != 0.0 || v2.Vectors[0].Pass {
		t.Errorf("absent key should score 0.0 and fail; got %+v", v2)
	}

	// empty AnswerKey is a misconfiguration -> error
	if _, err := j.Judge(context.Background(), scenario.Scenario{}, ArmWithGold, ArmOutcome{Output: "x"}); err == nil {
		t.Error("empty AnswerKey should error (deterministic grading needs a key)")
	}
}

// TestAnswerKeyJudge_WholeTokenBoundary: the key must match as a whole token, not
// an embedded substring — else a short key false-positives (refuter r1: "42" in
// "1423" would score 1.0 and, on the control arm, trip the ceiling pre-screen).
func TestAnswerKeyJudge_WholeTokenBoundary(t *testing.T) {
	j := AnswerKeyJudge{}
	cases := []struct {
		key, output string
		want        float64
	}{
		{"42", "the port is 1423 today", 0.0},                                 // embedded → no match
		{"42", "the answer is 42.", 1.0},                                      // delimited → match
		{"42", "42 units", 1.0},                                               // at string start → match
		{"widget", "a WidgetFactory builds it", 0.0},                          // embedded in a word → no match
		{"boshu2/agentops-beads", "remote is boshu2/agentops-beads here", 1.0}, // punctuated key, delimited → match
	}
	for _, tc := range cases {
		sc := scenario.Scenario{AnswerKey: tc.key}
		v, err := j.Judge(context.Background(), sc, ArmWithGold, ArmOutcome{Output: tc.output})
		if err != nil {
			t.Fatal(err)
		}
		if v.AggregateScore != tc.want {
			t.Errorf("key %q in %q: score %v, want %v", tc.key, tc.output, v.AggregateScore, tc.want)
		}
	}
}
