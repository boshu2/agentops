// This file implements wiki.Claim — the core domain entity of the wiki
// bounded context. Per the 2026-05-17 dueling-idea-wizards brainstorm,
// freshness, contradiction detection, and citation health all attach to a
// Claim (a single statement), not to a page. A Claim therefore carries the
// inputs the FreshnessPolicy port consumes: a volatility class (how fast the
// statement decays) and an authority class (how trustworthy its origin is).
package wiki

import (
	"errors"
	"fmt"
	"strings"
)

// VolatilityClass describes how fast a Claim's truth value decays. It is the
// primary input to the FreshnessPolicy port (W3): an invariant claim almost
// never needs re-verification, an ephemeral one is stale within hours.
//
// The ordering invariant → release-bound → fast → ephemeral runs from
// slowest- to fastest-decaying.
type VolatilityClass string

const (
	// VolatilityInvariant marks claims that hold indefinitely (e.g. a
	// mathematical fact or a frozen protocol constant).
	VolatilityInvariant VolatilityClass = "invariant"
	// VolatilityReleaseBound marks claims tied to a software release — true
	// until the next version ships (e.g. "ao 2.4 exposes `ao wiki`").
	VolatilityReleaseBound VolatilityClass = "release-bound"
	// VolatilityFast marks claims that change on the order of days (e.g.
	// "the inject ranker uses decay weighting").
	VolatilityFast VolatilityClass = "fast"
	// VolatilityEphemeral marks claims valid only for hours (e.g. a live
	// runtime observation or a queue depth).
	VolatilityEphemeral VolatilityClass = "ephemeral"
)

// validVolatility is the closed set of recognized volatility classes.
var validVolatility = map[VolatilityClass]struct{}{
	VolatilityInvariant:    {},
	VolatilityReleaseBound: {},
	VolatilityFast:         {},
	VolatilityEphemeral:    {},
}

// Valid reports whether v is a recognized VolatilityClass.
func (v VolatilityClass) Valid() bool {
	_, ok := validVolatility[v]
	return ok
}

// AuthorityClass describes how trustworthy a Claim's origin is. It is the
// second FreshnessPolicy input and the tiebreaker for contradiction
// resolution: a claim sourced from executable code outranks one sourced from
// narrative .agents/ prose, which outranks an unverified external page.
type AuthorityClass string

const (
	// AuthorityCode marks claims derived from executable source or its
	// generated output — the strongest authority.
	AuthorityCode AuthorityClass = "code"
	// AuthorityGenerated marks claims derived from generated artifacts
	// (e.g. `ao --help` output, a compiled reference).
	AuthorityGenerated AuthorityClass = "generated"
	// AuthoritySchema marks claims derived from a declared contract or
	// JSON schema.
	AuthoritySchema AuthorityClass = "schema"
	// AuthorityAgents marks claims sourced from .agents/ narrative
	// knowledge (learnings, playbooks, briefings).
	AuthorityAgents AuthorityClass = "agents"
	// AuthorityExternal marks claims sourced from outside the repo (an RFC,
	// a blog post, a vendor doc) — the weakest authority.
	AuthorityExternal AuthorityClass = "external"
)

// validAuthority is the closed set of recognized authority classes.
var validAuthority = map[AuthorityClass]struct{}{
	AuthorityCode:      {},
	AuthorityGenerated: {},
	AuthoritySchema:    {},
	AuthorityAgents:    {},
	AuthorityExternal:  {},
}

// Valid reports whether a is a recognized AuthorityClass.
func (a AuthorityClass) Valid() bool {
	_, ok := validAuthority[a]
	return ok
}

// ErrInvalidClaim is the sentinel wrapped by every Claim.Validate failure.
// Callers test membership with errors.Is(err, ErrInvalidClaim).
var ErrInvalidClaim = errors.New("invalid wiki claim")

// Claim is the core domain entity of the wiki bounded context: a single,
// independently-verifiable statement. Pages are collections of Claims;
// freshness, contradiction detection, and citation health all operate at
// Claim granularity.
//
// SourceRefs are the citations that back the Claim — each is an opaque
// reference token (a file path, a URL, an artifact ID). A Claim with no
// SourceRefs is unsupported and fails Validate.
type Claim struct {
	// ID is the Claim's stable identifier. Opaque; callers MUST NOT parse
	// substructure.
	ID string `json:"id"`
	// Text is the statement itself — exactly one assertion.
	Text string `json:"text"`
	// SourceRefs are the citations backing the Claim (file paths, URLs,
	// artifact IDs). At least one is required.
	SourceRefs []string `json:"source_refs"`
	// VolatilityClass is how fast the Claim decays; a FreshnessPolicy input.
	VolatilityClass VolatilityClass `json:"volatility_class"`
	// AuthorityClass is how trustworthy the Claim's origin is; a
	// FreshnessPolicy input and the contradiction-resolution tiebreaker.
	AuthorityClass AuthorityClass `json:"authority_class"`
	// Confidence is the tolerant-parsed confidence of the Claim, reusing the
	// codec's canonical [0,1] float. The zero value (Value 0) is a valid
	// "no stated confidence" signal and is not rejected by Validate.
	Confidence Confidence `json:"confidence"`
}

// Validate enforces the structural invariants of a Claim. It returns an
// error wrapping ErrInvalidClaim when:
//
//   - ID is empty or whitespace-only.
//   - Text is empty or whitespace-only.
//   - SourceRefs is empty, or every entry is whitespace-only (an
//     unsupported claim).
//   - VolatilityClass is not a recognized class.
//   - AuthorityClass is not a recognized class.
//
// A nil return means the Claim is well-formed. Validate does not assess the
// Claim's truth or freshness — that is the FreshnessPolicy port's job.
func (c Claim) Validate() error {
	if strings.TrimSpace(c.ID) == "" {
		return fmt.Errorf("%w: id is required", ErrInvalidClaim)
	}
	if strings.TrimSpace(c.Text) == "" {
		return fmt.Errorf("%w: text is required", ErrInvalidClaim)
	}
	if !c.hasSourceRef() {
		return fmt.Errorf("%w: at least one non-empty source_ref is required", ErrInvalidClaim)
	}
	if !c.VolatilityClass.Valid() {
		return fmt.Errorf("%w: unknown volatility_class %q", ErrInvalidClaim, c.VolatilityClass)
	}
	if !c.AuthorityClass.Valid() {
		return fmt.Errorf("%w: unknown authority_class %q", ErrInvalidClaim, c.AuthorityClass)
	}
	return nil
}

// hasSourceRef reports whether the Claim carries at least one non-empty,
// non-whitespace source reference.
func (c Claim) hasSourceRef() bool {
	for _, ref := range c.SourceRefs {
		if strings.TrimSpace(ref) != "" {
			return true
		}
	}
	return false
}
