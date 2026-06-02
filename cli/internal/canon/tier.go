// practices: [ddd-bounded-context, anti-self-certification]

package canon

import (
	"os"
	"strings"
)

// Tier classifies what KIND of knowledge a learning is, which determines how it
// can be earned into canon. Not all knowledge is mechanically verifiable: a
// claim about code or gate output is falsifiable; a judgment call ("prefer X
// when Y") is not. The gate must not demand verification of the unverifiable —
// nor let unverifiable claims in on a single citation.
type Tier string

const (
	// TierFalsifiable: the learning asserts something checkable against the
	// codebase, a gate, or a command. It must be independently verified.
	TierFalsifiable Tier = "falsifiable"
	// TierHeuristic: judgment/experience that cannot be mechanically checked.
	// It earns canon on breadth of independent reuse instead of verification.
	TierHeuristic Tier = "heuristic"
)

// TierOf reads `canon_tier:` from a learning's YAML frontmatter, defaulting to
// falsifiable. The default is the STRICTER tier on purpose: heuristic is an
// explicit opt-out for judgment-only knowledge, so an unmarked learning cannot
// dodge the verification requirement by omission.
func TierOf(path string) Tier {
	content, err := os.ReadFile(path)
	if err != nil {
		return TierFalsifiable
	}
	inFrontmatter := false
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			break
		}
		if !inFrontmatter {
			continue
		}
		if strings.HasPrefix(trimmed, "canon_tier:") {
			v := unquote(strings.TrimSpace(strings.TrimPrefix(trimmed, "canon_tier:")))
			if Tier(v) == TierHeuristic {
				return TierHeuristic
			}
			return TierFalsifiable
		}
	}
	return TierFalsifiable
}

// GateFor returns the promotion policy for a tier:
//
//   - falsifiable: 1 cross-engineer citation AND 1 independent verification
//     (the DefaultGate — useful AND checked-true).
//   - heuristic:   3 cross-engineer citations, no verification. It cannot be
//     mechanically checked, so the only honest trust signal is breadth of
//     independent reuse by other engineers.
//
// Refutation hard-blocks both tiers (handled in Evaluate).
func GateFor(t Tier) Gate {
	if t == TierHeuristic {
		return Gate{MinCitations: 3, MinVerifications: 0, RequireReceipt: false}
	}
	return DefaultGate()
}

// EvaluateEntry resolves the entry's tier from its file, picks the tier-aware
// gate, and evaluates. When path is empty (tier unknown), it falls back to the
// strict falsifiable gate.
func EvaluateEntry(entryID, path string, cl *CitationLedger, vl *VerificationLedger) (Decision, error) {
	tier := TierFalsifiable
	if path != "" {
		tier = TierOf(path)
	}
	d, err := GateFor(tier).Evaluate(entryID, cl, vl)
	if err != nil {
		return d, err
	}
	d.Tier = tier
	return d, nil
}

// EntryPath returns the learning path recorded for entryID in either ledger
// (citations first, then verifications), or "" if the entry is untracked. Used
// by surfaces that have only an entry ID (e.g. `canon status`) to resolve tier.
func EntryPath(entryID string, cl *CitationLedger, vl *VerificationLedger) string {
	if cites, err := cl.ForEntry(entryID); err == nil {
		for _, c := range cites {
			if c.Path != "" {
				return c.Path
			}
		}
	}
	if vers, err := vl.ForEntry(entryID); err == nil {
		for _, v := range vers {
			if v.Path != "" {
				return v.Path
			}
		}
	}
	return ""
}
