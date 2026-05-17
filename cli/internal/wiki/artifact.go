// This file implements wiki.Artifact — a typed envelope over a wiki document.
// An Artifact carries a Kind discriminator; different Kinds impose different
// required-field invariants (the subtype invariants). Specifically, an
// Artifact of Kind ArtifactFinding MUST carry a DedupKey, matching the
// dedup_key contract the overnight finding-generator already enforces
// (cli/internal/overnight/generator_sidecars.go).
package wiki

import (
	"errors"
	"fmt"
	"strings"
)

// ArtifactKind discriminates the subtype of a wiki Artifact. Each Kind has
// its own required-field invariants enforced by Artifact.Validate.
type ArtifactKind string

const (
	// ArtifactFinding is a promoted finding (an actionable defect or
	// improvement). Findings MUST carry a DedupKey so the single-writer
	// merge can deduplicate them.
	ArtifactFinding ArtifactKind = "finding"
	// ArtifactConcept is a Tier-1 concept draft extracted from a source.
	ArtifactConcept ArtifactKind = "concept"
	// ArtifactSource is an ingested source document (raw material for
	// concept/finding extraction).
	ArtifactSource ArtifactKind = "source"
	// ArtifactSynthesis is a Tier-2 synthesis page that aggregates and
	// reconciles other artifacts.
	ArtifactSynthesis ArtifactKind = "synthesis"
)

// validArtifactKind is the closed set of recognized artifact kinds.
var validArtifactKind = map[ArtifactKind]struct{}{
	ArtifactFinding:   {},
	ArtifactConcept:   {},
	ArtifactSource:    {},
	ArtifactSynthesis: {},
}

// Valid reports whether k is a recognized ArtifactKind.
func (k ArtifactKind) Valid() bool {
	_, ok := validArtifactKind[k]
	return ok
}

// ErrInvalidArtifact is the sentinel wrapped by every base-level (Kind- and
// ID-agnostic) Artifact.Validate failure. Callers test membership with
// errors.Is(err, ErrInvalidArtifact).
var ErrInvalidArtifact = errors.New("invalid wiki artifact")

// ErrSubtypeInvariant is the sentinel wrapped when an Artifact violates a
// per-Kind (subtype) invariant — for example a Kind=finding Artifact missing
// its DedupKey. It is distinct from ErrInvalidArtifact so callers can tell a
// structural rejection from a subtype-invariant breach with
// errors.Is(err, ErrSubtypeInvariant).
var ErrSubtypeInvariant = errors.New("artifact subtype invariant violated")

// Artifact is a typed envelope over a wiki document. Kind selects the
// subtype; ID is the stable identifier; Claims are the statements the
// document asserts. DedupKey is required only for Kind ArtifactFinding —
// see Validate.
type Artifact struct {
	// ID is the Artifact's stable identifier. Opaque to callers.
	ID string `json:"id"`
	// Kind discriminates the Artifact subtype and selects which invariants
	// Validate enforces.
	Kind ArtifactKind `json:"kind"`
	// Title is the human-readable artifact title.
	Title string `json:"title"`
	// DedupKey is the deduplication key. REQUIRED for Kind ArtifactFinding
	// (the subtype invariant); ignored for other kinds. Matches the
	// dedup_key field on overnight finding generator candidates.
	DedupKey string `json:"dedup_key,omitempty"`
	// Claims are the statements this Artifact asserts. Optional at the
	// envelope level; an Artifact MAY carry zero Claims.
	Claims []Claim `json:"claims,omitempty"`
}

// Validate enforces the Artifact's invariants in two layers:
//
// Base invariants (failures wrap ErrInvalidArtifact):
//   - ID is empty or whitespace-only.
//   - Kind is not a recognized ArtifactKind.
//   - Any contained Claim fails its own Claim.Validate.
//
// Subtype invariants (failures wrap ErrSubtypeInvariant):
//   - Kind ArtifactFinding requires a non-empty DedupKey. A finding
//     without a dedup_key is rejected — the overnight single-writer merge
//     cannot deduplicate it.
//
// A nil return means the Artifact satisfies every invariant for its Kind.
func (a Artifact) Validate() error {
	if strings.TrimSpace(a.ID) == "" {
		return fmt.Errorf("%w: id is required", ErrInvalidArtifact)
	}
	if !a.Kind.Valid() {
		return fmt.Errorf("%w: unknown kind %q", ErrInvalidArtifact, a.Kind)
	}
	for i, claim := range a.Claims {
		if err := claim.Validate(); err != nil {
			return fmt.Errorf("%w: claim %d (%q): %w", ErrInvalidArtifact, i, claim.ID, err)
		}
	}
	if err := a.validateSubtype(); err != nil {
		return err
	}
	return nil
}

// validateSubtype enforces the per-Kind invariants. Failures wrap
// ErrSubtypeInvariant.
func (a Artifact) validateSubtype() error {
	if a.Kind == ArtifactFinding && strings.TrimSpace(a.DedupKey) == "" {
		return fmt.Errorf("%w: kind=finding artifact %q requires a dedup_key", ErrSubtypeInvariant, a.ID)
	}
	return nil
}
