package ports

import "context"

// WikiIndexRecord is the port-level shape of a single indexed document. One
// record exists per indexable file under a corpus root. ContentHash is the
// stable digest of the file's bytes — it is the incremental-reindex pivot: a
// record is rewritten only when its ContentHash changes. Size and ModTimeUnix
// are advisory metadata, not identity inputs.
type WikiIndexRecord struct {
	// Path is the absolute filesystem path of the indexed document. It is
	// the record's identity key within the index.
	Path string `json:"path"`
	// Root is the corpus root (resolved via CorpusLocator) the document
	// was discovered under. It supports cross-repo indexing.
	Root string `json:"root"`
	// ContentHash is the hex-encoded digest of the document's bytes. A
	// reindex rewrites a record only when this value changes.
	ContentHash string `json:"content_hash"`
	// Size is the document size in bytes at index time (advisory).
	Size int64 `json:"size"`
	// ModTimeUnix is the file modification time in Unix seconds at index
	// time (advisory; not an identity input).
	ModTimeUnix int64 `json:"mod_time_unix"`
}

// WikiIndexPort is the BC seam for a persistent, content-addressed document
// index over one or more .agents/ corpus roots. It replaces the dead
// search.Index term-index with a doc-record store whose defining property is
// incremental reindex: when a single file changes, only that file's record is
// rewritten — every unchanged record is preserved byte-for-byte.
//
// Contract:
//
//   - Reindex MUST be incremental: a record whose on-disk content is
//     unchanged since the last Reindex MUST NOT be rewritten. The returned
//     WikiIndexResult reports which paths were Added, Updated, or Removed.
//   - Reindex MUST resolve corpus roots through wiki.CorpusLocator so that
//     cross-repo roots and the AO_AGENTS_DIR / AO_HOME overrides are honored.
//   - Records MUST return the index contents in a stable (path-sorted)
//     order so callers can diff successive snapshots.
//   - Context cancellation MUST be honored on a best-effort basis.
//
// The production implementation is wiki.WikiIndex.
type WikiIndexPort interface {
	// Reindex scans every indexable document under the configured corpus
	// roots and updates the persisted index incrementally, returning a
	// summary of the changes applied.
	Reindex(ctx context.Context) (WikiIndexResult, error)
	// Records returns the current index contents in path-sorted order.
	Records() ([]WikiIndexRecord, error)
}

// WikiIndexResult summarizes the effect of a single Reindex call. The three
// slices are disjoint and each is path-sorted. Unchanged records are not
// reported — their absence from all three slices is the incremental-reindex
// guarantee.
type WikiIndexResult struct {
	// Added lists paths newly present in the index.
	Added []string
	// Updated lists paths whose ContentHash changed since the last Reindex.
	Updated []string
	// Removed lists paths that were indexed previously but no longer exist.
	Removed []string
}
