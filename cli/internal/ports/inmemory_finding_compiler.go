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
//     three targets (plan + pre-mortem + constraint). This is the
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
		var p string
		switch kind {
		case CompiledOutputPlanningRule:
			p = path.Join(".agents", "planning-rules", artifact.ID+".md")
		case CompiledOutputPreMortemCheck:
			p = path.Join(".agents", "pre-mortem-checks", artifact.ID+".md")
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

// parseCompilerTargets parses the comma-separated value from the
// artifact's frontmatter. Empty input returns the adapter's default
// set (all three targets).
func parseCompilerTargets(raw string) []CompiledOutputKind {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []CompiledOutputKind{
			CompiledOutputPlanningRule,
			CompiledOutputPreMortemCheck,
			CompiledOutputConstraint,
		}
	}
	parts := strings.Split(trimmed, ",")
	result := make([]CompiledOutputKind, 0, len(parts))
	for _, raw := range parts {
		name := strings.ToLower(strings.TrimSpace(raw))
		switch name {
		case "plan", "planning-rule", "planning_rule":
			result = append(result, CompiledOutputPlanningRule)
		case "pre-mortem", "pre_mortem", "premortem":
			result = append(result, CompiledOutputPreMortemCheck)
		case "constraint", "constraints":
			result = append(result, CompiledOutputConstraint)
		default:
			// silently skip unknown — port contract notes this
		}
	}
	return result
}

// Compile-time assertion: InMemoryFindingCompiler satisfies the port.
var _ FindingCompilerPort = (*InMemoryFindingCompiler)(nil)
