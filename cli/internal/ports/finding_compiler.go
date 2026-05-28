package ports

import "context"

// FindingArtifact is the input to FindingCompilerPort.Compile. ID is
// the finding's stable identifier (matches the registry's id field).
// Frontmatter is the YAML key/value bag from the promoted artifact
// (.agents/findings/<id>.md frontmatter, governed by
// finding-artifact.schema.json). Body is the markdown content below
// the frontmatter. Adapters MAY use Frontmatter["compiler_targets"]
// to decide which CompiledOutputKind values to emit.
type FindingArtifact struct {
	ID          string
	Frontmatter map[string]string
	Body        string
}

// CompiledOutputKind enumerates the three compiler targets named in
// docs/contracts/finding-compiler.md ("Compiler Targets" table). The
// string values match the kebab-case names used in the contract's
// `compiler_targets` field.
type CompiledOutputKind string

const (
	CompiledOutputPlanningRule   CompiledOutputKind = "plan"
	CompiledOutputPreMortemCheck CompiledOutputKind = "pre-mortem"
	CompiledOutputConstraint     CompiledOutputKind = "constraint"
)

// CompiledOutput is one materialized artifact emitted by Compile. Path
// is the relative output path (e.g. `.agents/planning-rules/<id>.md`);
// adapters that don't write to a filesystem may treat Path as a logical
// key. Body is the file content the adapter would persist. Kind names
// which compiler target produced this output.
type CompiledOutput struct {
	Kind CompiledOutputKind
	Path string
	Body []byte
}

// FindingCompilerPort is the BC1 compile-side. It turns a promoted
// finding artifact into the advisory and mechanical outputs named in
// docs/contracts/finding-compiler.md "Compiler Targets" — planning
// rules, pre-mortem checks, and constraints. Callers — the
// `ao compile` path, dream's compounding loop, and any future
// cross-repo finding ingester — depend on this port so the compile
// behavior can be exercised against an in-memory adapter without
// standing up the real `.agents/findings/`, planning-rules,
// pre-mortem-checks, and constraints surfaces.
//
// Contract:
//
//   - Compile MUST return a non-nil (possibly empty) slice on success.
//   - The returned slice MUST NOT include duplicate Path values; a
//     given output is materialized at most once per artifact.
//   - When the input's Frontmatter constrains `compiler_targets`,
//     adapters SHOULD honor that constraint. When `compiler_targets`
//     is absent, the adapter chooses defaults (and documents them).
//   - Context cancellation MUST be honored on a best-effort basis.
//
// See docs/contracts/ubiquitous-language.md (BC1 row) for the
// canonical Corpus context surface and corpus_reader.go /
// corpus_writer.go for the read+write counterparts.
type FindingCompilerPort interface {
	Compile(ctx context.Context, artifact FindingArtifact) ([]CompiledOutput, error)
}
