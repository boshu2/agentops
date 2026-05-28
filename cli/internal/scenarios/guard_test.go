package scenarios

import "testing"

func TestHasScenariosBlock(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{
			name: "canonical heading with scenario body",
			in:   "Some intro.\n\n## Scenarios\nScenario: x\n  Given a\n  When b\n  Then c",
			want: true,
		},
		{
			name: "heading only",
			in:   "## Scenarios",
			want: true,
		},
		{
			name: "level-three heading",
			in:   "### Scenarios",
			want: true,
		},
		{
			name: "case-insensitive heading word",
			in:   "## scenarios",
			want: true,
		},
		{
			name: "heading with trailing parenthetical",
			in:   "## Scenarios (draft)",
			want: true,
		},
		{
			name: "indented and trailing whitespace heading",
			in:   "   ## Scenarios   ",
			want: true,
		},
		{
			name: "no heading, plain acceptance bullets",
			in:   "- Given a bead, when extract runs, then a block prints",
			want: false,
		},
		{
			name: "singular Scenario line is not the block heading",
			in:   "Scenario: foo\n  Given a\n  When b\n  Then c",
			want: false,
		},
		{
			name: "level-two singular Scenario heading does not match plural block",
			in:   "## Scenario: foo",
			want: false,
		},
		{
			name: "the word scenarios in prose is not a heading",
			in:   "This bead describes several scenarios in prose form.",
			want: false,
		},
		{
			name: "empty string",
			in:   "",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasScenariosBlock(tt.in); got != tt.want {
				t.Errorf("HasScenariosBlock(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
