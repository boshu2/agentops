package ports

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
)

// InMemoryFindingCompiler is a FindingCompilerPort that emits the
// outputs as in-memory byte slices instead of writing to a real
// filesystem. Intended for tests and CLI dry-runs. The compile
// behavior is deliberately minimal: it produces a stub body for each
// target so callers can verify wiring without depending on the
// production compiler templates (which live in the package that owns
// the finding-compiler implementation).
//
// Target selection:
//   - When artifact.Frontmatter["compiler_targets"] is a non-empty
//     comma-separated list, only those targets are emitted (case
//     and whitespace insensitive). Unknown target strings are
//     silently skipped — callers can detect the gap by comparing
//     requested vs emitted slices.
//   - When compiler_targets is absent or empty, the adapter emits all
//     three targets (plan + premortem + constraint). This is the
//     adapter's documented default per the port contract.
type InMemoryFindingCompiler struct{}

// NewInMemoryFindingCompiler returns the zero-config adapter. The
// adapter holds no mutable state; concurrent callers are safe.
func NewInMemoryFindingCompiler() *InMemoryFindingCompiler {
	return &InMemoryFindingCompiler{}
}

// Compile produces stub outputs for the requested targets. Returns
// `errors.New("ports: FindingArtifact.ID required")` when the input
// has no ID — the structural invariant the contract relies on for
// per-artifact output paths.
func (c *InMemoryFindingCompiler) Compile(ctx context.Context, artifact FindingArtifact) ([]CompiledOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if artifact.ID == "" {
		return nil, errors.New("ports: FindingArtifact.ID required")
	}
	requested := parseCompilerTargets(artifact.Frontmatter["compiler_targets"])
	out := make([]CompiledOutput, 0, len(requested))
	seen := map[string]struct{}{}
	for _, kind := range requested {
		// A constraint is emitted ONLY for findings carrying valid mechanical
		// detector metadata; an advisory finding emits no constraint. This fake
		// mirrors the production compiler's SELECTION rule (the contract behavior
		// callers assert) — see search.BuildConstraintEntry for the structured
		// production emit. The precondition here must stay in lockstep with it;
		// both adapters' tests pin "advisory finding -> no constraint".
		if kind == CompiledOutputConstraint && !hasMechanicalDetectorMetadata(artifact.Frontmatter) {
			continue
		}
		if kind == CompiledOutputConstraint {
			_, ready, err := ReplayDetectorEvidence(
				strings.TrimSpace(artifact.Frontmatter["detector_pattern"]),
				strings.TrimSpace(artifact.Frontmatter["detector_kind"]),
				artifact.DetectorEvidence,
			)
			if err != nil {
				return nil, err
			}
			if !ready {
				continue
			}
		}
		var p string
		switch kind {
		case CompiledOutputPlanningRule:
			p = path.Join(".agents", "planning-rules", artifact.ID+".md")
		case CompiledOutputPremortemCheck:
			p = path.Join(".agents", "premortem-checks", artifact.ID+".md")
		case CompiledOutputConstraint:
			p = path.Join(".agents", "constraints", artifact.ID+".sh")
		default:
			continue
		}
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		body := fmt.Sprintf("# %s (kind=%s)\n\n%s\n", artifact.ID, kind, artifact.Body)
		out = append(out, CompiledOutput{Kind: kind, Path: p, Body: []byte(body)})
	}
	return out, nil
}

// hasMechanicalDetectorMetadata reports whether a finding's frontmatter carries
// the metadata required to compile a mechanical constraint: detectability =
// mechanical, a non-empty detector_pattern, non-empty constraint_path_globs, a
// compiled_at, and (when set) a "regex" detector_kind. Mirrors the precondition
// in search.BuildConstraintEntry — the two MUST agree (the fake must not claim a
// constraint the production builder would skip, and vice versa).
func hasMechanicalDetectorMetadata(fm map[string]string) bool {
	if strings.TrimSpace(fm["detectability"]) != "mechanical" {
		return false
	}
	if strings.TrimSpace(fm["detector_pattern"]) == "" {
		return false
	}
	if k := strings.TrimSpace(fm["detector_kind"]); k != "" && k != "regex" {
		return false
	}
	if strings.TrimSpace(fm["constraint_path_globs"]) == "" {
		return false
	}
	return strings.TrimSpace(fm["compiled_at"]) != ""
}

// parseCompilerTargets parses the comma-separated value from the
// artifact's frontmatter. Empty input returns the adapter's default
// set (all three targets).
func parseCompilerTargets(raw string) []CompiledOutputKind {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []CompiledOutputKind{
			CompiledOutputPlanningRule,
			CompiledOutputPremortemCheck,
			CompiledOutputConstraint,
		}
	}
	parts := strings.Split(trimmed, ",")
	result := make([]CompiledOutputKind, 0, len(parts))
	seen := make(map[CompiledOutputKind]struct{}, len(parts))
	appendUnique := func(kind CompiledOutputKind) {
		if _, exists := seen[kind]; exists {
			return
		}
		seen[kind] = struct{}{}
		result = append(result, kind)
	}
	for _, raw := range parts {
		name := strings.ToLower(strings.TrimSpace(raw))
		switch name {
		case "plan", "planning-rule", "planning_rule":
			appendUnique(CompiledOutputPlanningRule)
		case "pre-mortem", "pre_mortem", "premortem":
			appendUnique(CompiledOutputPremortemCheck)
		case "constraint", "constraints":
			appendUnique(CompiledOutputConstraint)
		default:
			// silently skip unknown — port contract notes this
		}
	}
	return result
}

// Compile-time assertion: InMemoryFindingCompiler satisfies the port.
var _ FindingCompilerPort = (*InMemoryFindingCompiler)(nil)
