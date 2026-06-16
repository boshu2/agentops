package eval

import (
	"context"
	"fmt"
	"strings"

	"github.com/boshu2/agentops/cli/internal/scenario"
)

// AnswerKeyJudge grades an arm DETERMINISTICALLY: it passes (score 1.0) iff the
// scenario's AnswerKey appears (case-insensitive substring) in the arm output,
// else fails (score 0.0). No LLM is consulted, so there is zero judge noise,
// position bias, or token cost — the rigorous grade for fact-recall OOD scenarios
// where the corpus holds a specific value the base model cannot know (age-k8u).
// It is selected (over the codex judge) whenever a scenario carries an AnswerKey.
type AnswerKeyJudge struct{}

// Judge implements ScenarioJudge. It requires scenario.AnswerKey to be set;
// an empty key is a misconfiguration (deterministic grading has nothing to match).
func (AnswerKeyJudge) Judge(_ context.Context, sc scenario.Scenario, _ ScenarioArm, outcome ArmOutcome) (JudgeVerdict, error) {
	key := strings.TrimSpace(sc.AnswerKey)
	if key == "" {
		return JudgeVerdict{}, fmt.Errorf("answer-key judge requires scenario.answer_key to be set")
	}
	present := answerKeyPresent(outcome.Output, key)
	score := 0.0
	if present {
		score = 1.0
	}
	return JudgeVerdict{
		AggregateScore: score,
		Vectors:        []VectorVerdict{{Dimension: "answer-key-present", Pass: present, Score: score}},
	}, nil
}

// answerKeyPresent reports whether key occurs in output as a WHOLE token (case-
// insensitive), i.e. not flanked by an alphanumeric character on either side. The
// boundary check stops short numeric/alnum keys from false-matching embedded
// substrings — e.g. "42" must NOT match "1423" (a false 1.0 there would even trip
// the control-arm ceiling pre-screen and abort with a bogus "no headroom" verdict —
// cross-family refuter r1).
func answerKeyPresent(output, key string) bool {
	o := strings.ToLower(output)
	k := strings.ToLower(strings.TrimSpace(key))
	if k == "" {
		return false
	}
	for from := 0; from+len(k) <= len(o); {
		i := strings.Index(o[from:], k)
		if i < 0 {
			return false
		}
		start := from + i
		end := start + len(k)
		beforeOK := start == 0 || !isAlnumByte(o[start-1])
		afterOK := end == len(o) || !isAlnumByte(o[end])
		if beforeOK && afterOK {
			return true
		}
		from = start + 1
	}
	return false
}

func isAlnumByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}
