package ports

import "context"

// CorpusWriteRequest is the input to CorpusWriterPort.Capture. Path
// scopes WHERE the artifact should land (relative-path semantics
// owned by the adapter; the port is path-shape-agnostic). Body is
// the artifact content. Metadata is an open key/value bag for
// frontmatter-style fields adapters may persist alongside Body
// (maturity, tags, source, date). Adapters MAY ignore Metadata keys
// they don't model.
type CorpusWriteRequest struct {
	Path     string
	Body     []byte
	Metadata map[string]string
}

// CorpusWriteResult is the per-call outcome. ResolvedPath is the
// concrete path the adapter committed to (an InMemoryCorpusWriter
// returns the same Path that came in; a filesystem-backed adapter may
// return a fully resolved absolute path). Created is true if the
// adapter materialized a new artifact; false if Capture updated an
// existing one in place (idempotent re-capture).
type CorpusWriteResult struct {
	ResolvedPath string
	Created      bool
}

// CorpusWriterPort is the write-side of BC1 Corpus. Callers — the
// bd-close harvest path, /forge promotions, dream's compounding loop,
// and any future ingester — depend on this port so the persistence
// surface is swappable (filesystem vs in-memory vs durable-store
// backend) and testable without standing up the real storage.
//
// Contract:
//
//   - Capture MUST be idempotent: calling Capture twice with an
//     identical CorpusWriteRequest MUST NOT produce drift. The second
//     call MAY update Body/Metadata in place but the resulting
//     ResolvedPath MUST be the same.
//   - On success, ResolvedPath MUST be non-empty.
//   - Created indicates first-time materialization; subsequent
//     Capture calls for the same Path return Created=false.
//   - Errors MUST be wrapped with `fmt.Errorf` context per Go
//     conventions when the adapter has a meaningful upstream error.
//   - Context cancellation MUST be honored on a best-effort basis;
//     adapters that complete synchronously and quickly may ignore it.
//
// See docs/contracts/ubiquitous-language.md (BC1 row) for the
// canonical Corpus context surface and CorpusReaderPort's read-side
// counterpart in corpus_reader.go.
type CorpusWriterPort interface {
	Capture(ctx context.Context, req CorpusWriteRequest) (CorpusWriteResult, error)
}
