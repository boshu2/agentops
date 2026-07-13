package ratchet

import "testing"

func TestMortemCanonicalValuesAndPermanentAliases(t *testing.T) {
	if got := string(StepPreMortem); got != "premortem" {
		t.Errorf("StepPreMortem = %q, want canonical value %q", got, "premortem")
	}
	if got := string(StepPostMortem); got != "postmortem" {
		t.Errorf("StepPostMortem = %q, want canonical value %q", got, "postmortem")
	}

	steps := AllSteps()
	if got, want := len(steps), 7; got != want {
		t.Fatalf("len(AllSteps()) = %d, want %d", got, want)
	}

	counts := map[Step]int{}
	for _, step := range steps {
		counts[step]++
	}
	if got := counts[StepPreMortem]; got != 1 {
		t.Errorf("AllSteps contains StepPreMortem %d times, want exactly once", got)
	}
	if got := counts[StepPostMortem]; got != 1 {
		t.Errorf("AllSteps contains StepPostMortem %d times, want exactly once", got)
	}

	for _, tc := range []struct {
		name  string
		input string
		want  string
	}{
		{name: "canonical premortem", input: "premortem", want: "premortem"},
		{name: "canonical postmortem", input: "postmortem", want: "postmortem"},
		{name: "hyphenated premortem", input: "  PRE-MORTEM  ", want: "premortem"},
		{name: "hyphenated postmortem", input: "post-mortem", want: "postmortem"},
		{name: "underscored premortem", input: "pre_mortem", want: "premortem"},
		{name: "underscored postmortem", input: "post_mortem", want: "postmortem"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(ParseStep(tc.input)); got != tc.want {
				t.Errorf("string(ParseStep(%q)) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
