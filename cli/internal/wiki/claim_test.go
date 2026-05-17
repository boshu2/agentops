package wiki

import (
	"errors"
	"strings"
	"testing"
)

// validClaim returns a well-formed Claim used as a mutation baseline.
func validClaim() Claim {
	return Claim{
		ID:              "claim-042",
		Text:            "the wiki bounded context consolidates frontmatter parsing",
		SourceRefs:      []string{".agents/plans/2026-05-17-wiki-bounded-context.md"},
		VolatilityClass: VolatilityReleaseBound,
		AuthorityClass:  AuthorityAgents,
	}
}

func TestClaim_Validate(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*Claim)
		wantErr bool
		// errSubstr, when set, must appear in the returned error message.
		errSubstr string
	}{
		{
			name:    "well-formed claim passes",
			mutate:  func(*Claim) {},
			wantErr: false,
		},
		{
			name:      "empty id fails",
			mutate:    func(c *Claim) { c.ID = "  " },
			wantErr:   true,
			errSubstr: "id",
		},
		{
			name:      "empty text fails",
			mutate:    func(c *Claim) { c.Text = "" },
			wantErr:   true,
			errSubstr: "text",
		},
		{
			name:      "nil source_refs fails",
			mutate:    func(c *Claim) { c.SourceRefs = nil },
			wantErr:   true,
			errSubstr: "source_ref",
		},
		{
			name:      "all-whitespace source_refs fails",
			mutate:    func(c *Claim) { c.SourceRefs = []string{"", "   "} },
			wantErr:   true,
			errSubstr: "source_ref",
		},
		{
			name:    "one non-empty source_ref among blanks passes",
			mutate:  func(c *Claim) { c.SourceRefs = []string{"", "cli/main.go"} },
			wantErr: false,
		},
		{
			name:      "unknown volatility_class fails",
			mutate:    func(c *Claim) { c.VolatilityClass = "glacial" },
			wantErr:   true,
			errSubstr: "volatility_class",
		},
		{
			name:      "unknown authority_class fails",
			mutate:    func(c *Claim) { c.AuthorityClass = "vibes" },
			wantErr:   true,
			errSubstr: "authority_class",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			claim := validClaim()
			c.mutate(&claim)
			err := claim.Validate()

			if c.wantErr {
				if err == nil {
					t.Fatalf("expected Validate to fail, got nil")
				}
				if !errors.Is(err, ErrInvalidClaim) {
					t.Errorf("expected error to wrap ErrInvalidClaim, got %v", err)
				}
				if c.errSubstr != "" && !strings.Contains(err.Error(), c.errSubstr) {
					t.Errorf("expected error to contain %q, got %q", c.errSubstr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("expected Validate to pass, got %v", err)
			}
		})
	}
}

func TestClaim_ValidateAcceptsAllVolatilityClasses(t *testing.T) {
	for _, v := range []VolatilityClass{
		VolatilityInvariant, VolatilityReleaseBound, VolatilityFast, VolatilityEphemeral,
	} {
		claim := validClaim()
		claim.VolatilityClass = v
		if err := claim.Validate(); err != nil {
			t.Errorf("volatility class %q should validate, got %v", v, err)
		}
	}
}

func TestClaim_ValidateAcceptsAllAuthorityClasses(t *testing.T) {
	for _, a := range []AuthorityClass{
		AuthorityCode, AuthorityGenerated, AuthoritySchema, AuthorityAgents, AuthorityExternal,
	} {
		claim := validClaim()
		claim.AuthorityClass = a
		if err := claim.Validate(); err != nil {
			t.Errorf("authority class %q should validate, got %v", a, err)
		}
	}
}

func TestVolatilityClass_Valid(t *testing.T) {
	cases := []struct {
		class VolatilityClass
		want  bool
	}{
		{VolatilityInvariant, true},
		{VolatilityReleaseBound, true},
		{VolatilityFast, true},
		{VolatilityEphemeral, true},
		{"", false},
		{"release-bound ", false},
		{"unknown", false},
	}
	for _, c := range cases {
		if got := c.class.Valid(); got != c.want {
			t.Errorf("VolatilityClass(%q).Valid() = %v, want %v", c.class, got, c.want)
		}
	}
}

func TestAuthorityClass_Valid(t *testing.T) {
	cases := []struct {
		class AuthorityClass
		want  bool
	}{
		{AuthorityCode, true},
		{AuthorityGenerated, true},
		{AuthoritySchema, true},
		{AuthorityAgents, true},
		{AuthorityExternal, true},
		{"", false},
		{"CODE", false},
		{"unknown", false},
	}
	for _, c := range cases {
		if got := c.class.Valid(); got != c.want {
			t.Errorf("AuthorityClass(%q).Valid() = %v, want %v", c.class, got, c.want)
		}
	}
}

// TestClaim_ConfidenceZeroValueIsAccepted documents that a Claim with no
// stated confidence (the zero-value Confidence) is structurally valid —
// confidence is advisory, not a required field.
func TestClaim_ConfidenceZeroValueIsAccepted(t *testing.T) {
	claim := validClaim()
	if claim.Confidence.Value != 0 {
		t.Fatalf("baseline claim should have zero-value confidence, got %v", claim.Confidence.Value)
	}
	if err := claim.Validate(); err != nil {
		t.Errorf("a claim with no stated confidence should validate, got %v", err)
	}

	claim.Confidence = ParseConfidence("high")
	if claim.Confidence.Value != ConfidenceHigh {
		t.Errorf("expected high confidence to coerce to %v, got %v", ConfidenceHigh, claim.Confidence.Value)
	}
	if err := claim.Validate(); err != nil {
		t.Errorf("a claim with a populated confidence should validate, got %v", err)
	}
}
