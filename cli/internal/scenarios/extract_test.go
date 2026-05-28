package scenarios

import (
	"strings"
	"testing"
)

func TestExtract(t *testing.T) {
	tests := []struct {
		name       string
		acceptance string
		want       []Scenario
		wantErr    bool
	}{
		{
			name:       "single bullet with inline given/when/then",
			acceptance: "- Given a bead with free-text bullets, when extract runs, then a Gherkin block is printed to stdout",
			want: []Scenario{{
				Name:  "a Gherkin block is printed to stdout",
				Given: "a bead with free-text bullets",
				When:  "extract runs",
				Then:  "a Gherkin block is printed to stdout",
			}},
		},
		{
			name: "multiple bullets yield multiple scenarios",
			acceptance: "- Given a bead, when extract runs, then a block prints\n" +
				"- Given the --json flag, when extract runs, then structured JSON prints",
			want: []Scenario{
				{Name: "a block prints", Given: "a bead", When: "extract runs", Then: "a block prints"},
				{Name: "structured JSON prints", Given: "the --json flag", When: "extract runs", Then: "structured JSON prints"},
			},
		},
		{
			name:       "sentence form without bullet marker",
			acceptance: "Given an empty bead when extract runs then an error is returned.",
			want: []Scenario{{
				Name:  "an error is returned",
				Given: "an empty bead",
				When:  "extract runs",
				Then:  "an error is returned",
			}},
		},
		{
			name:       "mixed casing of keywords is normalized",
			acceptance: "GIVEN a draft bead WHEN the command runs THEN exit code is zero",
			want: []Scenario{{
				Name:  "exit code is zero",
				Given: "a draft bead",
				When:  "the command runs",
				Then:  "exit code is zero",
			}},
		},
		{
			name: "unparseable lines are skipped, parseable ones kept",
			acceptance: "- The tool must be fast and well documented.\n" +
				"- Given a valid bead, when extract runs, then scenarios print",
			want: []Scenario{
				{Name: "scenarios print", Given: "a valid bead", When: "extract runs", Then: "scenarios print"},
			},
		},
		{
			name:       "empty acceptance is an error",
			acceptance: "   \n  ",
			wantErr:    true,
		},
		{
			name:       "no given/when/then keywords is an error",
			acceptance: "- The CLI exits 0 on success and 1 on failure.",
			wantErr:    true,
		},
		{
			name:       "word-boundary: forgiven is not given, whenever lacks then",
			acceptance: "The user is forgiven whenever the form is submitted",
			wantErr:    true,
		},
		{
			name:       "prose mentioning Given/When/Then as a phrase is not a scenario",
			acceptance: "- Parse free-text into Given/When/Then triples using deterministic rules",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Extract(tt.acceptance)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Extract(%q) = %+v, want error", tt.acceptance, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Extract(%q) unexpected error: %v", tt.acceptance, err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("Extract(%q) returned %d scenarios, want %d\ngot: %+v", tt.acceptance, len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("scenario[%d]:\n got  %+v\n want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestRender(t *testing.T) {
	in := []Scenario{
		{Name: "a block prints", Given: "a bead", When: "extract runs", Then: "a block prints"},
		{Name: "json prints", Given: "the --json flag", When: "extract runs", Then: "JSON prints"},
	}
	want := "## Scenarios\n" +
		"\n" +
		"Scenario: a block prints\n" +
		"  Given a bead\n" +
		"  When extract runs\n" +
		"  Then a block prints\n" +
		"\n" +
		"Scenario: json prints\n" +
		"  Given the --json flag\n" +
		"  When extract runs\n" +
		"  Then JSON prints\n"

	got := Render(in)
	if got != want {
		t.Errorf("Render mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderRoundTripsThroughExtract(t *testing.T) {
	acceptance := "- Given a bead, when extract runs, then a Gherkin block prints to stdout"
	scen, err := Extract(acceptance)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	block := Render(scen)
	if !strings.HasPrefix(block, "## Scenarios\n") {
		t.Errorf("Render output must start with the '## Scenarios' heading, got:\n%s", block)
	}
	if strings.Count(block, "Scenario:") != 1 {
		t.Errorf("expected exactly one Scenario, got:\n%s", block)
	}
}
