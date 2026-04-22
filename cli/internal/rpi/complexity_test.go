package rpi

import "testing"

func TestContainsWholeWord(t *testing.T) {
	cases := []struct {
		text string
		kw   string
		want bool
	}{
		{"refactor the thing", "refactor", true},
		{"refactored the thing", "refactor", false},
		{"support the thing", "port", false},
		{"migrate this", "migrate", true},
		{"case insensitive REFACTOR", "refactor", true},
		{"every module now", "every module", true},
		{"everyone", "every", false},
	}
	for _, tc := range cases {
		if got := ContainsWholeWord(tc.text, tc.kw); got != tc.want {
			t.Errorf("ContainsWholeWord(%q, %q) = %v, want %v", tc.text, tc.kw, got, tc.want)
		}
	}
}

func TestClassifyComplexity(t *testing.T) {
	cases := []struct {
		name string
		goal string
		want ComplexityLevel
	}{
		{"trivial fix", "fix typo", ComplexityFast},
		{"short add", "add a helper", ComplexityFast},
		{"refactor triggers full", "refactor auth module", ComplexityFull},
		{"migrate triggers full", "migrate from v1 to v2", ComplexityFull},
		{"long description triggers full", "This is a very long goal description that exceeds one hundred and twenty characters so it must be considered a complex task for proper handling.", ComplexityFull},
		{"two scope keywords full", "change global throughout codebase", ComplexityFull},
		{"one scope keyword is standard", "update the codebase slightly but very small set", ComplexityStandard},
		{"medium length is standard", "update the login handler to use JWT instead of cookies and log events", ComplexityStandard},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyComplexity(tc.goal); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestScoreGoal(t *testing.T) {
	s := ScoreGoal("  Refactor all modules to use new API  ")
	if s.DescLen == 0 {
		t.Error("DescLen should be populated")
	}
	if s.ComplexKeywords < 1 {
		t.Errorf("expected refactor to match, got %+v", s)
	}
	if s.ScopeKeywords < 1 {
		t.Errorf("expected 'all' scope keyword, got %+v", s)
	}
}

func TestLevelFromScore(t *testing.T) {
	cases := []struct {
		name  string
		score ComplexityScore
		want  ComplexityLevel
	}{
		{"complex keyword -> full", ComplexityScore{DescLen: 20, ComplexKeywords: 1}, ComplexityFull},
		{"2 scope keywords -> full", ComplexityScore{DescLen: 20, ScopeKeywords: 2}, ComplexityFull},
		{"long desc -> full", ComplexityScore{DescLen: 200}, ComplexityFull},
		{"medium desc -> standard", ComplexityScore{DescLen: 60}, ComplexityStandard},
		{"1 scope keyword -> standard", ComplexityScore{DescLen: 20, ScopeKeywords: 1}, ComplexityStandard},
		{"short trivial -> fast", ComplexityScore{DescLen: 20}, ComplexityFast},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := LevelFromScore(tc.score); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
