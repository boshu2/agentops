package search

import "testing"

// TestSanitizeReach covers the blast-radius tier normalization + default (ag-bsf6).
func TestSanitizeReach(t *testing.T) {
	cases := map[string]string{
		"":         "pull", // absent → default
		"   ":      "pull", // blank → default
		"bogus":    "pull", // invalid → default
		"bead":     "bead",
		"pull":     "pull",
		"always":   "always",
		"ALWAYS":   "always", // case-insensitive
		"  bead  ": "bead",   // trimmed
	}
	for in, want := range cases {
		if got := SanitizeReach(in); got != want {
			t.Errorf("SanitizeReach(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestParseFrontMatter_Reach proves reach round-trips through frontmatter parse
// and defaults to pull when absent (ag-bsf6 acceptance).
func TestParseFrontMatter_Reach(t *testing.T) {
	fm, _ := ParseFrontMatter([]string{"---", "reach: always", "maturity: established", "---"})
	if fm.Reach != "always" {
		t.Fatalf("fm.Reach = %q, want %q", fm.Reach, "always")
	}
	if got := SanitizeReach(fm.Reach); got != "always" {
		t.Fatalf("sanitized reach = %q, want always", got)
	}

	// Absent reach → empty in frontmatter, sanitized to the pull default.
	fm2, _ := ParseFrontMatter([]string{"---", "maturity: provisional", "---"})
	if fm2.Reach != "" {
		t.Fatalf("absent reach should parse empty, got %q", fm2.Reach)
	}
	if got := SanitizeReach(fm2.Reach); got != "pull" {
		t.Fatalf("absent reach should default to pull, got %q", got)
	}
}

func TestSanitizeAuthoredReach_DowngradesAlways(t *testing.T) {
	for _, in := range []string{"always", "ALWAYS", "  always  "} {
		if got := SanitizeAuthoredReach(in); got != "pull" {
			t.Fatalf("SanitizeAuthoredReach(%q) = %q, want pull", in, got)
		}
	}
	if got := SanitizeAuthoredReach("bead"); got != "bead" {
		t.Fatalf("SanitizeAuthoredReach(bead) = %q, want bead", got)
	}
}

func TestComputeReach_AlwaysRequiresEstablishedCanon(t *testing.T) {
	cases := []struct {
		name         string
		authored     string
		maturity     string
		canon        bool
		wantComputed string
	}{
		{
			name:         "established canon computes always",
			maturity:     "established",
			canon:        true,
			wantComputed: "always",
		},
		{
			name:         "established non-canon remains pull",
			authored:     "always",
			maturity:     "established",
			canon:        false,
			wantComputed: "pull",
		},
		{
			name:         "canon candidate remains pull",
			authored:     "always",
			maturity:     "candidate",
			canon:        true,
			wantComputed: "pull",
		},
		{
			name:         "authored bead survives when not always",
			authored:     "bead",
			maturity:     "candidate",
			canon:        false,
			wantComputed: "bead",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeReach(tt.authored, tt.maturity, tt.canon)
			if got != tt.wantComputed {
				t.Fatalf("ComputeReach(%q, %q, %v) = %q, want %q", tt.authored, tt.maturity, tt.canon, got, tt.wantComputed)
			}
		})
	}
}
