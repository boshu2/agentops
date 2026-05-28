// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// productionFindingCompiler satisfies ports.FindingCompilerPort by
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
// (the corpus_fs.Writer real adapter handles the on-disk side).
type productionFindingCompiler struct{}

func newProductionFindingCompiler() *productionFindingCompiler {
	return &productionFindingCompiler{}
}

// Compile renders one FindingArtifact into its compiled outputs.
func (c *productionFindingCompiler) Compile(ctx context.Context, artifact ports.FindingArtifact) ([]ports.CompiledOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if artifact.ID == "" {
		return nil, errors.New("productionFindingCompiler: artifact.ID required")
	}
	kinds, err := resolveCompilerTargets(artifact.Frontmatter["compiler_targets"])
	if err != nil {
		return nil, fmt.Errorf("productionFindingCompiler %q: %w", artifact.ID, err)
	}
	out := make([]ports.CompiledOutput, 0, len(kinds))
	for _, kind := range kinds {
		out = append(out, ports.CompiledOutput{
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
func resolveCompilerTargets(raw string) ([]ports.CompiledOutputKind, error) {
	if strings.TrimSpace(raw) == "" {
		return []ports.CompiledOutputKind{
			ports.CompiledOutputPlanningRule,
			ports.CompiledOutputPreMortemCheck,
			ports.CompiledOutputConstraint,
		}, nil
	}
	seen := make(map[ports.CompiledOutputKind]struct{}, 3)
	kinds := make([]ports.CompiledOutputKind, 0, 3)
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

func parseCompilerKind(name string) (ports.CompiledOutputKind, bool) {
	switch name {
	case string(ports.CompiledOutputPlanningRule):
		return ports.CompiledOutputPlanningRule, true
	case string(ports.CompiledOutputPreMortemCheck):
		return ports.CompiledOutputPreMortemCheck, true
	case string(ports.CompiledOutputConstraint):
		return ports.CompiledOutputConstraint, true
	}
	return "", false
}

// compiledPath returns the canonical relative path per finding-compiler.md.
func compiledPath(kind ports.CompiledOutputKind, id string) string {
	switch kind {
	case ports.CompiledOutputPlanningRule:
		return ".agents/planning-rules/" + id + ".md"
	case ports.CompiledOutputPreMortemCheck:
		return ".agents/pre-mortem-checks/" + id + ".md"
	case ports.CompiledOutputConstraint:
		return ".agents/constraints/" + id + ".md"
	}
	return ""
}

// renderCompiledBody builds the output body for one compiled kind.
// Shape: optional YAML frontmatter (deterministic key order) → kind-
// specific header → original artifact body.
func renderCompiledBody(kind ports.CompiledOutputKind, artifact ports.FindingArtifact) []byte {
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

func compiledHeading(kind ports.CompiledOutputKind) string {
	switch kind {
	case ports.CompiledOutputPlanningRule:
		return "Planning Rule"
	case ports.CompiledOutputPreMortemCheck:
		return "Pre-Mortem Check"
	case ports.CompiledOutputConstraint:
		return "Constraint"
	}
	return "Compiled Finding"
}

// Compile-time assertion: productionFindingCompiler satisfies the port.
var _ ports.FindingCompilerPort = (*productionFindingCompiler)(nil)
