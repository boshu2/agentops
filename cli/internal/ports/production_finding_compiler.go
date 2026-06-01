// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ProductionFindingCompiler satisfies FindingCompilerPort by
// rendering a FindingArtifact into the three compiler-target
// artifacts named in docs/contracts/finding-compiler.md: planning
// rules, pre-mortem checks, and constraints.
//
// Target selection:
//   - Honors artifact.Frontmatter["compiler_targets"] when present
//     (comma-separated list of "plan", "pre-mortem", "constraint").
//   - When the frontmatter key is absent OR empty, defaults to
//     emitting all three kinds — matches the contract's "adapter
//     chooses defaults (and documents them)" clause.
//
// Output rendering:
//   - Path follows the canonical layout in the contract:
//     .agents/planning-rules/<id>.md, .agents/pre-mortem-checks/<id>.md,
//     .agents/constraints/<id>.md.
//   - Body is a slim markdown wrapper: a kind-specific header line
//     plus the original artifact Body. Frontmatter is propagated via
//     a YAML block when present.
//
// This is a pure-Go transform — no subprocess, no filesystem. Callers
// that need to persist the outputs feed them into a CorpusWriterPort
// (cycle 113 productionCorpusWriter handles the on-disk side).
type ProductionFindingCompiler struct{}

func NewProductionFindingCompiler() *ProductionFindingCompiler {
	return &ProductionFindingCompiler{}
}

// Compile renders one FindingArtifact into its compiled outputs.
func (c *ProductionFindingCompiler) Compile(ctx context.Context, artifact FindingArtifact) ([]CompiledOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if artifact.ID == "" {
		return nil, errors.New("ProductionFindingCompiler: artifact.ID required")
	}
	kinds, err := resolveCompilerTargets(artifact.Frontmatter["compiler_targets"])
	if err != nil {
		return nil, fmt.Errorf("ProductionFindingCompiler %q: %w", artifact.ID, err)
	}
	out := make([]CompiledOutput, 0, len(kinds))
	for _, kind := range kinds {
		out = append(out, CompiledOutput{
			Kind: kind,
			Path: compiledPath(kind, artifact.ID),
			Body: renderCompiledBody(kind, artifact),
		})
	}
	return out, nil
}

// resolveCompilerTargets parses the comma-separated frontmatter value
// into a deterministic, deduplicated ordered list of kinds. Empty
// input returns the three defaults.
func resolveCompilerTargets(raw string) ([]CompiledOutputKind, error) {
	if strings.TrimSpace(raw) == "" {
		return []CompiledOutputKind{
			CompiledOutputPlanningRule,
			CompiledOutputPreMortemCheck,
			CompiledOutputConstraint,
		}, nil
	}
	seen := make(map[CompiledOutputKind]struct{}, 3)
	kinds := make([]CompiledOutputKind, 0, 3)
	for _, part := range strings.Split(raw, ",") {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		kind, ok := parseCompilerKind(name)
		if !ok {
			return nil, fmt.Errorf("unknown compiler_targets entry %q", name)
		}
		if _, dup := seen[kind]; dup {
			continue
		}
		seen[kind] = struct{}{}
		kinds = append(kinds, kind)
	}
	return kinds, nil
}

func parseCompilerKind(name string) (CompiledOutputKind, bool) {
	switch name {
	case string(CompiledOutputPlanningRule):
		return CompiledOutputPlanningRule, true
	case string(CompiledOutputPreMortemCheck):
		return CompiledOutputPreMortemCheck, true
	case string(CompiledOutputConstraint):
		return CompiledOutputConstraint, true
	}
	return "", false
}

// compiledPath returns the canonical relative path per finding-compiler.md.
func compiledPath(kind CompiledOutputKind, id string) string {
	switch kind {
	case CompiledOutputPlanningRule:
		return ".agents/planning-rules/" + id + ".md"
	case CompiledOutputPreMortemCheck:
		return ".agents/pre-mortem-checks/" + id + ".md"
	case CompiledOutputConstraint:
		return ".agents/constraints/" + id + ".md"
	}
	return ""
}

// renderCompiledBody builds the output body for one compiled kind.
// Shape: optional YAML frontmatter (deterministic key order) → kind-
// specific header → original artifact body.
func renderCompiledBody(kind CompiledOutputKind, artifact FindingArtifact) []byte {
	var out strings.Builder
	if len(artifact.Frontmatter) > 0 {
		keys := make([]string, 0, len(artifact.Frontmatter))
		for k := range artifact.Frontmatter {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out.WriteString("---\n")
		for _, k := range keys {
			out.WriteString(k)
			out.WriteString(": ")
			out.WriteString(artifact.Frontmatter[k])
			out.WriteByte('\n')
		}
		out.WriteString("---\n")
	}
	out.WriteString("# ")
	out.WriteString(compiledHeading(kind))
	out.WriteString(" (")
	out.WriteString(artifact.ID)
	out.WriteString(")\n\n")
	out.WriteString(artifact.Body)
	if !strings.HasSuffix(artifact.Body, "\n") {
		out.WriteByte('\n')
	}
	return []byte(out.String())
}

func compiledHeading(kind CompiledOutputKind) string {
	switch kind {
	case CompiledOutputPlanningRule:
		return "Planning Rule"
	case CompiledOutputPreMortemCheck:
		return "Pre-Mortem Check"
	case CompiledOutputConstraint:
		return "Constraint"
	}
	return "Compiled Finding"
}

// Compile-time assertion: ProductionFindingCompiler satisfies the port.
var _ FindingCompilerPort = (*ProductionFindingCompiler)(nil)
