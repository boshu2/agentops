package wiki

import (
	"errors"
	"strings"
	"testing"
)

// validFinding returns a well-formed Kind=finding Artifact for use as a
// baseline that tests then mutate to exercise individual invariants.
func validFinding() Artifact {
	return Artifact{
		ID:       "finding-001",
		Kind:     ArtifactFinding,
		Title:    "inject ranker ignores decay",
		DedupKey: "finding-generator|inject-ranker-decay",
		Claims: []Claim{{
			ID:              "claim-001",
			Text:            "the inject ranker does not apply decay weighting",
			SourceRefs:      []string{"cli/internal/search/inject.go"},
			VolatilityClass: VolatilityReleaseBound,
			AuthorityClass:  AuthorityCode,
		}},
	}
}

func TestArtifact_Invariants(t *testing.T) {
	t.Run("finding without dedup_key fails with subtype-invariant error", func(t *testing.T) {
		a := validFinding()
		a.DedupKey = ""

		err := a.Validate()
		if err == nil {
			t.Fatal("expected Validate to reject a kind=finding artifact with no dedup_key, got nil")
		}
		if !errors.Is(err, ErrSubtypeInvariant) {
			t.Errorf("expected error to wrap ErrSubtypeInvariant, got %v", err)
		}
		if errors.Is(err, ErrInvalidArtifact) {
			t.Errorf("dedup_key omission is a subtype invariant, not a base invariant; "+
				"error should not wrap ErrInvalidArtifact: %v", err)
		}
		if !strings.Contains(err.Error(), "dedup_key") {
			t.Errorf("expected error message to name dedup_key, got %q", err.Error())
		}
	})

	t.Run("whitespace-only dedup_key on a finding still fails", func(t *testing.T) {
		a := validFinding()
		a.DedupKey = "   "

		err := a.Validate()
		if err == nil || !errors.Is(err, ErrSubtypeInvariant) {
			t.Fatalf("expected ErrSubtypeInvariant for whitespace-only dedup_key, got %v", err)
		}
	})

	t.Run("well-formed finding passes", func(t *testing.T) {
		if err := validFinding().Validate(); err != nil {
			t.Fatalf("expected a well-formed finding to validate, got %v", err)
		}
	})

	t.Run("non-finding kinds do not require dedup_key", func(t *testing.T) {
		for _, kind := range []ArtifactKind{ArtifactConcept, ArtifactSource, ArtifactSynthesis} {
			a := Artifact{ID: "art-" + string(kind), Kind: kind}
			if err := a.Validate(); err != nil {
				t.Errorf("kind %q must not require a dedup_key, got %v", kind, err)
			}
		}
	})

	t.Run("empty id fails with base-invariant error", func(t *testing.T) {
		a := validFinding()
		a.ID = "  "

		err := a.Validate()
		if err == nil || !errors.Is(err, ErrInvalidArtifact) {
			t.Fatalf("expected ErrInvalidArtifact for empty id, got %v", err)
		}
		if errors.Is(err, ErrSubtypeInvariant) {
			t.Errorf("empty id is a base invariant, not a subtype invariant: %v", err)
		}
	})

	t.Run("unknown kind fails with base-invariant error", func(t *testing.T) {
		a := validFinding()
		a.Kind = "bogus"

		err := a.Validate()
		if err == nil || !errors.Is(err, ErrInvalidArtifact) {
			t.Fatalf("expected ErrInvalidArtifact for unknown kind, got %v", err)
		}
		if !strings.Contains(err.Error(), "bogus") {
			t.Errorf("expected error to name the offending kind, got %q", err.Error())
		}
	})

	t.Run("invalid contained claim fails the artifact", func(t *testing.T) {
		a := validFinding()
		a.Claims = append(a.Claims, Claim{ID: "claim-bad", Text: "no sources"})

		err := a.Validate()
		if err == nil || !errors.Is(err, ErrInvalidArtifact) {
			t.Fatalf("expected ErrInvalidArtifact for an invalid contained claim, got %v", err)
		}
		if !errors.Is(err, ErrInvalidClaim) {
			t.Errorf("expected the wrapped claim error chain to reach ErrInvalidClaim, got %v", err)
		}
	})
}

func TestArtifactKind_Valid(t *testing.T) {
	cases := []struct {
		kind ArtifactKind
		want bool
	}{
		{ArtifactFinding, true},
		{ArtifactConcept, true},
		{ArtifactSource, true},
		{ArtifactSynthesis, true},
		{"", false},
		{"finding ", false},
		{"unknown", false},
	}
	for _, c := range cases {
		if got := c.kind.Valid(); got != c.want {
			t.Errorf("ArtifactKind(%q).Valid() = %v, want %v", c.kind, got, c.want)
		}
	}
}
