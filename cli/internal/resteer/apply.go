package resteer

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/boshu2/agentops/cli/internal/goals"
)

// directiveHeadingRe matches a "### N. Title" directive heading. It mirrors the
// pattern used by the F1.0 patcher so the apply path renumbers exactly the
// lines the patcher recognizes as directive headings.
var directiveHeadingRe = regexp.MustCompile(`^(\s*###\s+)(\d+)(\.\s+.+)$`)

// steerFlips maps each Steer direction to its inverse. Only increase and
// decrease are flippable; hold and explore have no opposite and are left
// untouched (ADR-0006: a steer_flip on a non-directional Steer is a no-op).
var steerFlips = map[string]string{
	"increase": "decrease",
	"decrease": "increase",
}

// ApplyOutcome reports what an Apply call did to GOALS.md.
type ApplyOutcome struct {
	// DirectiveID is the stable ID of the mutated directive.
	DirectiveID string
	// MutationType is the mutation that was applied.
	MutationType string
	// FromPosition is the directive's display number before the mutation.
	FromPosition int
	// ToPosition is the directive's display number after the mutation
	// (equal to FromPosition for non-reordering mutations).
	ToPosition int
	// Detail is a human-readable one-line summary of the change.
	Detail string
}

// Apply performs the recommended mutation on GOALS.md content non-lossily and
// returns the patched content plus an outcome summary.
//
// The mutation is applied with the F1.0 line-buffer technique: only the lines
// belonging to the target directive (and, for a priority bump, the directive
// heading numbers) are rewritten; every other byte of GOALS.md is preserved.
// Apply never calls RenderGoalsMD / WriteMDGoals.
//
// Apply is the mutating half of F5; it must only be reached after the F5.3
// human-confirmation gate. It enforces the ADR-0006 invariants that protect a
// confirmed run from doing more than the recommendation describes:
//
//   - a steer_flip is refused unless policy.SteerFlipPermitted() (I-3);
//   - a priority bump is clamped to policy.MaxPriorityBump positions.
func Apply(data []byte, policy Policy, rec Recommendation) ([]byte, ApplyOutcome, error) {
	patcher, err := goals.NewGoalsPatcher(data)
	if err != nil {
		return nil, ApplyOutcome{}, fmt.Errorf("parse GOALS.md for re-steer apply: %w", err)
	}
	dir, ok := patcher.DirectiveByStableID(rec.DirectiveID)
	if !ok {
		return nil, ApplyOutcome{}, fmt.Errorf(
			"directive %q not found in GOALS.md; run `ao goals steer recommend` to refresh recommendations",
			rec.DirectiveID)
	}
	switch rec.MutationType {
	case MutationPriorityBump:
		return applyPriorityBump(data, patcher, policy, rec, dir)
	case MutationSteerFlip:
		return applySteerFlip(data, patcher, policy, rec, dir)
	default:
		return nil, ApplyOutcome{}, fmt.Errorf(
			"re-steer apply does not support mutation_type %q; only %q and %q are applicable mutations",
			rec.MutationType, MutationPriorityBump, MutationSteerFlip)
	}
}

// applyPriorityBump moves the target directive block up by the recommended
// (clamped) number of positions and renumbers every directive heading.
func applyPriorityBump(data []byte, patcher *goals.GoalsPatcher, policy Policy, rec Recommendation, dir goals.ParsedDirective) ([]byte, ApplyOutcome, error) {
	bump := clampBump(rec.PriorityBump, policy.MaxPriorityBump)
	from := dir.Number
	to := from - bump
	if to < 1 {
		to = 1
	}
	if to >= from {
		return data, ApplyOutcome{
			DirectiveID:  rec.DirectiveID,
			MutationType: MutationPriorityBump,
			FromPosition: from,
			ToPosition:   from,
			Detail:       fmt.Sprintf("directive #%d already at or above target priority; no change", from),
		}, nil
	}
	moved := moveDirectiveBlock(splitLines(data), patcher.Directives(), from, to)
	out := renumberDirectiveHeadings(moved)
	return joinLines(out), ApplyOutcome{
		DirectiveID:  rec.DirectiveID,
		MutationType: MutationPriorityBump,
		FromPosition: from,
		ToPosition:   to,
		Detail:       fmt.Sprintf("moved directive %q from position #%d to #%d (bump %d)", rec.DirectiveID, from, to, from-to),
	}, nil
}

// applySteerFlip inverts the directive's Steer direction via the F1.0 patcher's
// SetAttribute. It refuses unless the policy dual opt-in (ADR-0006 I-3) holds.
func applySteerFlip(data []byte, patcher *goals.GoalsPatcher, policy Policy, rec Recommendation, dir goals.ParsedDirective) ([]byte, ApplyOutcome, error) {
	if !policy.SteerFlipPermitted() {
		return nil, ApplyOutcome{}, fmt.Errorf(
			"steer_flip refused: ADR-0006 I-3 requires both allow_steer_flip:true and \"steer_flip\" in allowed_mutation_types; "+
				"set both in %s to permit it", DefaultPolicyRelPath)
	}
	flipped, ok := steerFlips[strings.ToLower(strings.TrimSpace(dir.Steer))]
	if !ok {
		return data, ApplyOutcome{
			DirectiveID:  rec.DirectiveID,
			MutationType: MutationSteerFlip,
			FromPosition: dir.Number,
			ToPosition:   dir.Number,
			Detail:       fmt.Sprintf("Steer %q has no opposite direction; no change", dir.Steer),
		}, nil
	}
	if err := patcher.SetAttribute(dir.Number, goals.AttrSteer, flipped); err != nil {
		return nil, ApplyOutcome{}, fmt.Errorf("apply steer_flip via patcher: %w", err)
	}
	return patcher.Bytes(), ApplyOutcome{
		DirectiveID:  rec.DirectiveID,
		MutationType: MutationSteerFlip,
		FromPosition: dir.Number,
		ToPosition:   dir.Number,
		Detail:       fmt.Sprintf("flipped Steer of directive %q from %q to %q", rec.DirectiveID, dir.Steer, flipped),
	}, nil
}

// clampBump bounds a recommended priority bump to [1, maxBump].
func clampBump(bump, maxBump int) int {
	if bump < 1 {
		return 1
	}
	if maxBump >= 1 && bump > maxBump {
		return maxBump
	}
	return bump
}

// splitLines splits GOALS.md content into lines, matching the F1.0 patcher's
// line model so block ranges line up exactly.
func splitLines(data []byte) []string {
	return strings.Split(string(data), "\n")
}

// joinLines is the inverse of splitLines.
func joinLines(lines []string) []byte {
	return []byte(strings.Join(lines, "\n"))
}

// moveDirectiveBlock returns lines with the directive whose display number is
// from spliced out and re-inserted immediately before the directive whose
// display number is to. Only the moved block changes position; every other
// line is preserved verbatim. Heading numbers are NOT adjusted here —
// renumberDirectiveHeadings does that as a separate, total pass.
func moveDirectiveBlock(lines []string, directives []goals.ParsedDirective, from, to int) []string {
	src, srcOK := directiveByNumber(directives, from)
	dst, dstOK := directiveByNumber(directives, to)
	if !srcOK || !dstOK || from == to {
		return lines
	}
	// Convert 1-based StartLine/EndLine to 0-based [lo, hi) slice bounds.
	srcLo, srcHi := src.StartLine-1, src.EndLine
	dstLo := dst.StartLine - 1
	block := append([]string(nil), lines[srcLo:srcHi]...)
	remaining := append(append([]string(nil), lines[:srcLo]...), lines[srcHi:]...)
	// The destination index shifts only when the removed block was before it,
	// which never happens for an upward move (from > to), so dstLo is stable.
	out := make([]string, 0, len(lines))
	out = append(out, remaining[:dstLo]...)
	out = append(out, block...)
	out = append(out, remaining[dstLo:]...)
	return out
}

// directiveByNumber finds a parsed directive by its display number.
func directiveByNumber(directives []goals.ParsedDirective, n int) (goals.ParsedDirective, bool) {
	for _, d := range directives {
		if d.Number == n {
			return d, true
		}
	}
	return goals.ParsedDirective{}, false
}

// renumberDirectiveHeadings rewrites every "### N. Title" heading so the
// display numbers are 1..K in source order. It touches only the numeric token
// of each heading line; the leading whitespace, the "### ", the dot, and the
// title are preserved exactly.
func renumberDirectiveHeadings(lines []string) []string {
	out := append([]string(nil), lines...)
	inDirectives := false
	n := 0
	for i, line := range out {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			inDirectives = strings.EqualFold(
				strings.TrimSpace(strings.TrimPrefix(trimmed, "## ")), "Directives")
			continue
		}
		if !inDirectives {
			continue
		}
		m := directiveHeadingRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		n++
		out[i] = m[1] + strconv.Itoa(n) + m[3]
	}
	return out
}
