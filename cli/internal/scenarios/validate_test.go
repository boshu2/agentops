package scenarios

import (
	"strings"
	"testing"
)

func TestParseBlock(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		wantN   int    // expected scenario count on success (ignored when wantErr set)
		wantErr string // substring expected in the error; "" means expect no error
		check   func(t *testing.T, got []Scenario)
	}{
		{
			name:  "well-formed single scenario",
			text:  "## Scenarios\n\nScenario: extract prints a block\n  Given a bead with free-text acceptance\n  When extract runs\n  Then a Gherkin block is printed\n",
			wantN: 1,
			check: func(t *testing.T, got []Scenario) {
				if got[0].Name != "extract prints a block" {
					t.Errorf("name = %q", got[0].Name)
				}
				if got[0].Given != "a bead with free-text acceptance" || got[0].When != "extract runs" || got[0].Then != "a Gherkin block is printed" {
					t.Errorf("clauses = %+v", got[0])
				}
			},
		},
		{
			name:  "well-formed multiple scenarios",
			text:  "## Scenarios\nScenario: one\n  Given a\n  When b\n  Then c\nScenario: two\n  Given d\n  When e\n  Then f\n",
			wantN: 2,
		},
		{
			name:  "well-formed with And/But continuations",
			text:  "## Scenarios\nScenario: with continuations\n  Given a precondition\n  And another precondition\n  When the action happens\n  Then a result\n  But not the other result\n",
			wantN: 1,
		},
		{
			name:  "block is bounded by a following markdown heading",
			text:  "## Scenarios\nScenario: real\n  Given a\n  When b\n  Then c\n\n## Notes\nScenario: ghost\n  Given only-a-given\n",
			wantN: 1,
			check: func(t *testing.T, got []Scenario) {
				if got[0].Name != "real" {
					t.Errorf("expected only the in-block scenario, got %+v", got)
				}
			},
		},
		{
			name:  "scenarios block lives after descriptive prose",
			text:  "Slice 4 of ag-dwq. Adds a validator.\n\n## Scenarios\nScenario: parses after prose\n  Given a description with leading prose\n  When validate runs\n  Then it parses the block\n",
			wantN: 1,
		},
		{
			name:    "no scenarios block",
			text:    "Just some free-text acceptance with no block.\n- bullet one\n- bullet two\n",
			wantErr: "no '## Scenarios' block found",
		},
		{
			name:    "block present but no Scenario entries",
			text:    "## Scenarios\n\n(to be authored)\n",
			wantErr: "no 'Scenario:' entries",
		},
		{
			name:    "scenario missing a name",
			text:    "## Scenarios\nScenario:\n  Given a\n  When b\n  Then c\n",
			wantErr: "missing name",
		},
		{
			name:    "scenario missing Given step",
			text:    "## Scenarios\nScenario: no given\n  When b\n  Then c\n",
			wantErr: "missing Given step",
		},
		{
			name:    "scenario missing When step",
			text:    "## Scenarios\nScenario: no when\n  Given a\n  Then c\n",
			wantErr: "missing When step",
		},
		{
			name:    "scenario missing Then step",
			text:    "## Scenarios\nScenario: no then\n  Given a\n  When b\n",
			wantErr: "missing Then step",
		},
		{
			name:    "scenario steps out of order",
			text:    "## Scenarios\nScenario: reversed\n  Given a\n  Then c\n  When b\n",
			wantErr: "out of order",
		},
		{
			name:    "scenario with an empty step body",
			text:    "## Scenarios\nScenario: empty when\n  Given a\n  When\n  Then c\n",
			wantErr: "empty When step",
		},
		{
			name:    "And step before any primary keyword",
			text:    "## Scenarios\nScenario: leading and\n  And a\n  Given b\n  When c\n  Then d\n",
			wantErr: "before any Given/When/Then",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseBlock(tt.text)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("ParseBlock() = %+v, want error containing %q", got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tt.wantN {
				t.Fatalf("got %d scenarios, want %d: %+v", len(got), tt.wantN, got)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func TestValidate_WellFormedReturnsNil(t *testing.T) {
	text := "## Scenarios\nScenario: ok\n  Given a\n  When b\n  Then c\n"
	if err := Validate(text); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestValidate_MalformedReturnsError(t *testing.T) {
	text := "## Scenarios\nScenario: broken\n  Given a\n  Then c\n"
	if err := Validate(text); err == nil {
		t.Fatal("Validate() = nil, want error naming the missing When step")
	}
}

func TestValidate_RoundTripRenderOutput(t *testing.T) {
	// A block produced by Render must always pass Validate — the validator is
	// the inverse acceptance gate of the extractor.
	rendered := Render([]Scenario{{Name: "n", Given: "a", When: "b", Then: "c"}})
	if err := Validate(rendered); err != nil {
		t.Fatalf("Render output failed Validate: %v\n%s", err, rendered)
	}
}
