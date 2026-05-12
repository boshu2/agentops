package ports

import "context"

// CitationKind enumerates the citation shapes the citation port
// recognizes. Values mirror the existing cli/cmd/ao/beads.go Citation
// struct's "kind" field: "file", "function", "symbol". String values
// stay lowercase to match the on-disk shape.
type CitationKind string

const (
	CitationKindFile     CitationKind = "file"
	CitationKindFunction CitationKind = "function"
	CitationKindSymbol   CitationKind = "symbol"
)

// CitationStatusResult is the verdict on a single citation.
// CitationStatusFresh means the underlying reference resolves at
// HEAD. CitationStatusStale means it does not. CitationStatusUnknown
// is the adapter's "could not decide" — used e.g. when the citation
// shape isn't recognized.
type CitationStatusResult string

const (
	CitationStatusFresh   CitationStatusResult = "FRESH"
	CitationStatusStale   CitationStatusResult = "STALE"
	CitationStatusUnknown CitationStatusResult = "UNKNOWN"
)

// CitationRequest is one verification input. Kind selects the lookup
// shape (file path, function name, free-form symbol). Raw is the
// verbatim text from the source artifact (e.g. a bead description).
// Cwd is the repository root the adapter should search from.
type CitationRequest struct {
	Kind CitationKind
	Raw  string
	Cwd  string
}

// CitationVerdict is the verification output. Status names the
// freshness verdict; Reason is a human-readable explanation
// (mandatory; adapters MUST populate it even on UNKNOWN); Resolved
// optionally names the single HEAD location the citation now points
// at when uniquely determined.
type CitationVerdict struct {
	Status   CitationStatusResult
	Reason   string
	Resolved string
}

// CitationPort wraps the citation-extraction + verify behavior so
// callers — `ao beads verify`, the dream-loop staleness check, and
// any future cross-repo citation auditor — can be exercised against
// an in-memory adapter without depending on the real repo grep + git
// surface. The verify helpers in cli/cmd/ao/beads.go
// (verifyFunctionCitation / verifySymbolCitation / verifyFileCitation)
// are the production-path implementation; this port is the contract
// they will satisfy once soc-pm5t (BC1.1 wire-up) routes callers
// through the port.
//
// Contract:
//
//   - Verify MUST return a non-nil CitationVerdict on success even when
//     Status is UNKNOWN.
//   - Reason MUST be non-empty.
//   - When CitationRequest.Raw is empty, Verify MUST return UNKNOWN
//     (the citation is malformed; there is nothing to resolve).
//   - When CitationRequest.Cwd is empty, behavior is adapter-defined:
//     the in-memory adapter accepts empty Cwd because it has no
//     filesystem dependency; a future filesystem-backed adapter MAY
//     reject empty Cwd with an explicit error.
//   - Context cancellation MUST be honored on a best-effort basis.
//
// See docs/contracts/ubiquitous-language.md (BC1 row) for the
// canonical Corpus context surface and the sibling
// corpus_reader.go / corpus_writer.go / finding_compiler.go ports.
type CitationPort interface {
	Verify(ctx context.Context, req CitationRequest) (CitationVerdict, error)
}
