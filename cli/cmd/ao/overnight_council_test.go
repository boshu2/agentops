package main

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/config"
)

func TestResolveDreamConsensusPolicy(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "default when empty", in: "", want: "majority"},
		{name: "default on whitespace", in: "   ", want: "majority"},
		{name: "custom preserved", in: "unanimous", want: "unanimous"},
		{name: "trimmed custom", in: "  quorum  ", want: "quorum"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveDreamConsensusPolicy(config.DreamConfig{ConsensusPolicy: tc.in})
			if got != tc.want {
				t.Fatalf("resolveDreamConsensusPolicy(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeDreamCouncilRunnerLabel(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{in: "codex", want: "codex"},
		{in: "  Codex  ", want: "codex"},
		{in: "claude_dream_council", want: "claude-dream-council"},
		{in: "Claude Dream Council", want: "claude-dream-council"},
		{in: "-codex-", want: "codex"},
		{in: "", want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got := normalizeDreamCouncilRunnerLabel(tc.in)
			if got != tc.want {
				t.Fatalf("normalizeDreamCouncilRunnerLabel(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestDreamCouncilRunnerMatches(t *testing.T) {
	tests := []struct {
		name              string
		expected, actual  string
		want              bool
	}{
		{name: "exact", expected: "codex", actual: "codex", want: true},
		{name: "case insensitive", expected: "codex", actual: "CODEX", want: true},
		{name: "dream-council suffix", expected: "claude", actual: "claude-dream-council", want: true},
		{name: "underscore suffix", expected: "claude", actual: "claude_dream_council", want: true},
		{name: "different runner", expected: "codex", actual: "claude", want: false},
		{name: "extra unexpected suffix", expected: "codex", actual: "codex-extra", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := dreamCouncilRunnerMatches(tc.expected, tc.actual)
			if got != tc.want {
				t.Fatalf("dreamCouncilRunnerMatches(%q, %q) = %v, want %v",
					tc.expected, tc.actual, got, tc.want)
			}
		})
	}
}

func TestValidateDreamCouncilRunnerReport(t *testing.T) {
	full := overnightCouncilRunnerReport{
		Runner:                 "codex",
		Headline:               "ship it",
		RecommendedKind:        "implement",
		RecommendedFirstAction: "run make test",
		Confidence:             "high",
	}
	if err := validateDreamCouncilRunnerReport("codex", full); err != nil {
		t.Fatalf("valid report should pass, got %v", err)
	}

	cases := []struct {
		name     string
		mutate   func(r *overnightCouncilRunnerReport)
		wantSub  string
	}{
		{name: "missing runner", mutate: func(r *overnightCouncilRunnerReport) { r.Runner = "" }, wantSub: "missing runner field"},
		{name: "runner mismatch", mutate: func(r *overnightCouncilRunnerReport) { r.Runner = "claude" }, wantSub: "runner mismatch"},
		{name: "missing headline", mutate: func(r *overnightCouncilRunnerReport) { r.Headline = "" }, wantSub: "missing headline"},
		{name: "missing recommended_kind", mutate: func(r *overnightCouncilRunnerReport) { r.RecommendedKind = "" }, wantSub: "missing recommended_kind"},
		{name: "missing recommended_first_action", mutate: func(r *overnightCouncilRunnerReport) { r.RecommendedFirstAction = "" }, wantSub: "missing recommended_first_action"},
		{name: "missing confidence", mutate: func(r *overnightCouncilRunnerReport) { r.Confidence = "" }, wantSub: "missing confidence"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := full
			tc.mutate(&r)
			err := validateDreamCouncilRunnerReport("codex", r)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tc.wantSub)
			}
		})
	}
}

func TestBuildDreamCouncilPrompt(t *testing.T) {
	packet := `{"run_id":"r1"}`
	noCreative := buildDreamCouncilPrompt("codex", packet, false)
	if !strings.Contains(noCreative, "You are the codex Dream Council runner.") {
		t.Fatalf("prompt missing runner intro: %q", noCreative)
	}
	if !strings.Contains(noCreative, packet) {
		t.Fatalf("prompt missing packet body")
	}
	if !strings.Contains(noCreative, `set runner exactly to "codex"`) {
		t.Fatalf("prompt missing runner assignment rule")
	}
	if !strings.Contains(noCreative, "Return wildcard_idea as an empty string") {
		t.Fatalf("non-creative prompt should insert the 'return empty' wildcard rule")
	}
	if strings.Contains(noCreative, "Include one bounded wildcard idea") {
		t.Fatalf("non-creative prompt should not include creative wildcard rule")
	}

	creative := buildDreamCouncilPrompt("claude", packet, true)
	if !strings.Contains(creative, "Include one bounded wildcard idea") {
		t.Fatalf("creative prompt should invite a wildcard idea")
	}
	if !strings.Contains(creative, `set runner exactly to "claude"`) {
		t.Fatalf("creative prompt should bind runner to claude")
	}
}

func TestExtractDreamClaudeStructuredOutput(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		envelope := `{"type":"result","structured_output":{"runner":"claude","headline":"x"}}`
		got, err := extractDreamClaudeStructuredOutput([]byte(envelope))
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		want := `{"runner":"claude","headline":"x"}`
		if string(got) != want {
			t.Fatalf("payload = %q, want %q", string(got), want)
		}
	})
	t.Run("error envelope", func(t *testing.T) {
		env := `{"type":"result","is_error":true,"message":"boom"}`
		_, err := extractDreamClaudeStructuredOutput([]byte(env))
		if err == nil || !strings.Contains(err.Error(), "reported error") {
			t.Fatalf("expected reported error err, got %v", err)
		}
	})
	t.Run("wrong type", func(t *testing.T) {
		env := `{"type":"progress"}`
		_, err := extractDreamClaudeStructuredOutput([]byte(env))
		if err == nil || !strings.Contains(err.Error(), "unexpected claude output type") {
			t.Fatalf("expected unexpected-type err, got %v", err)
		}
	})
	t.Run("missing structured_output", func(t *testing.T) {
		env := `{"type":"result"}`
		_, err := extractDreamClaudeStructuredOutput([]byte(env))
		if err == nil || !strings.Contains(err.Error(), "missing structured_output") {
			t.Fatalf("expected missing-structured_output err, got %v", err)
		}
	})
	t.Run("bad json", func(t *testing.T) {
		_, err := extractDreamClaudeStructuredOutput([]byte("not json"))
		if err == nil || !strings.Contains(err.Error(), "parse claude result envelope") {
			t.Fatalf("expected parse err, got %v", err)
		}
	})
}

func TestSynthesizeDreamCouncil_ConsensusAndDisagreement(t *testing.T) {
	reports := []overnightCouncilRunnerReport{
		{
			Runner:                 "codex",
			RecommendedKind:        "implement",
			RecommendedFirstAction: "run make test",
			WildcardIdea:           "turn it off and on again",
		},
		{
			Runner:                 "claude",
			RecommendedKind:        "implement",
			RecommendedFirstAction: "run make test",
		},
	}
	got := synthesizeDreamCouncil([]string{"codex", "claude"}, nil, "majority", reports)

	if got.ConsensusKind != "implement" {
		t.Fatalf("ConsensusKind = %q, want %q", got.ConsensusKind, "implement")
	}
	if got.RecommendedFirstAction != "run make test" {
		t.Fatalf("RecommendedFirstAction = %q, want %q", got.RecommendedFirstAction, "run make test")
	}
	if len(got.Disagreements) != 0 {
		t.Fatalf("no disagreements expected, got %v", got.Disagreements)
	}
	wantCompleted := []string{"claude", "codex"}
	if !reflect.DeepEqual(got.CompletedRunners, wantCompleted) {
		t.Fatalf("CompletedRunners = %v, want %v", got.CompletedRunners, wantCompleted)
	}
	wantWildcards := []string{"codex: turn it off and on again"}
	if !reflect.DeepEqual(got.WildcardIdeas, wantWildcards) {
		t.Fatalf("WildcardIdeas = %v, want %v", got.WildcardIdeas, wantWildcards)
	}
}

func TestSynthesizeDreamCouncil_TiesPickLexicographicallyFirst(t *testing.T) {
	reports := []overnightCouncilRunnerReport{
		{Runner: "codex", RecommendedKind: "implement", RecommendedFirstAction: "A"},
		{Runner: "claude", RecommendedKind: "research", RecommendedFirstAction: "B"},
	}
	got := synthesizeDreamCouncil([]string{"codex", "claude"}, nil, "majority", reports)
	// counts tie (1 each); tie-break picks kind with smaller string: "implement" < "research".
	if got.ConsensusKind != "implement" {
		t.Fatalf("ConsensusKind = %q, want %q", got.ConsensusKind, "implement")
	}
	if got.RecommendedFirstAction != "A" {
		t.Fatalf("RecommendedFirstAction = %q, want %q", got.RecommendedFirstAction, "A")
	}
	// Claude disagrees.
	if len(got.Disagreements) != 1 || !strings.Contains(got.Disagreements[0], "claude prefers research") {
		t.Fatalf("disagreements = %v, want one mentioning 'claude prefers research'", got.Disagreements)
	}
}

func TestSynthesizeDreamCouncil_RecordsFailed(t *testing.T) {
	got := synthesizeDreamCouncil([]string{"codex", "claude"}, []string{"claude"}, "majority", nil)
	if !reflect.DeepEqual(got.FailedRunners, []string{"claude"}) {
		t.Fatalf("FailedRunners = %v, want [claude]", got.FailedRunners)
	}
	if got.ConsensusPolicy != "majority" {
		t.Fatalf("ConsensusPolicy = %q, want majority", got.ConsensusPolicy)
	}
	if got.ConsensusKind != "" {
		t.Fatalf("ConsensusKind should be empty when no reports, got %q", got.ConsensusKind)
	}
}

func TestWithDreamCouncilRunnerTimeout_UsesConfigured(t *testing.T) {
	parent := context.Background()
	ctx, cancel, timeout := withDreamCouncilRunnerTimeout(parent, 42*time.Second)
	defer cancel()
	if timeout != 42*time.Second {
		t.Fatalf("timeout = %v, want 42s", timeout)
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		t.Fatalf("ctx should carry a deadline")
	}
}

func TestWithDreamCouncilRunnerTimeout_FallsBackToDefault(t *testing.T) {
	parent := context.Background()
	_, cancel, timeout := withDreamCouncilRunnerTimeout(parent, 0)
	defer cancel()
	if timeout != dreamCouncilRunnerTimeout {
		t.Fatalf("timeout = %v, want default %v", timeout, dreamCouncilRunnerTimeout)
	}
}

func TestWithDreamCouncilRunnerTimeout_ShrinksToParentDeadline(t *testing.T) {
	parent, parentCancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer parentCancel()
	_, cancel, timeout := withDreamCouncilRunnerTimeout(parent, 10*time.Minute)
	defer cancel()
	if timeout > 10*time.Millisecond {
		t.Fatalf("timeout = %v, should have shrunk below 10ms (parent=5ms)", timeout)
	}
	if timeout <= 0 {
		t.Fatalf("timeout = %v, should be positive (floor of 1s when non-positive)", timeout)
	}
}
