package ports

import (
	"context"
	"errors"
	"sync"
)

// InMemoryCorpusWriter is a CorpusWriterPort backed by an in-process
// map. Intended for tests and ephemeral CLI dry-runs. Capture is
// idempotent and thread-safe (one mutex protects the underlying map).
//
// The adapter intentionally retains the Metadata map and the Body
// slice from the request rather than deep-copying. Callers that
// mutate request fields after Capture must construct fresh maps and
// slices themselves; this matches the InMemoryCorpusReader's
// no-defensive-copy posture.
type InMemoryCorpusWriter struct {
	mu      sync.Mutex
	entries map[string]storedEntry
}

type storedEntry struct {
	body     []byte
	metadata map[string]string
}

// NewInMemoryCorpusWriter returns a writer over an empty in-memory
// store.
func NewInMemoryCorpusWriter() *InMemoryCorpusWriter {
	return &InMemoryCorpusWriter{entries: map[string]storedEntry{}}
}

// Capture persists req under req.Path. Idempotent: a re-Capture for
// the same Path returns Created=false. Returns
// `errors.New("ports: CorpusWriteRequest.Path required")` if Path is
// empty (the only structural invariant).
func (w *InMemoryCorpusWriter) Capture(ctx context.Context, req CorpusWriteRequest) (CorpusWriteResult, error) {
	if err := ctx.Err(); err != nil {
		return CorpusWriteResult{}, err
	}
	if req.Path == "" {
		return CorpusWriteResult{}, errors.New("ports: CorpusWriteRequest.Path required")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	_, existed := w.entries[req.Path]
	w.entries[req.Path] = storedEntry{body: req.Body, metadata: req.Metadata}
	return CorpusWriteResult{ResolvedPath: req.Path, Created: !existed}, nil
}

// Snapshot returns a stable view of the current entry count. Test-only
// helper; not part of the port contract. Exported so cross-package
// tests can assert on adapter state without exporting `entries`.
func (w *InMemoryCorpusWriter) Snapshot() (count int, paths []string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	paths = make([]string, 0, len(w.entries))
	for p := range w.entries {
		paths = append(paths, p)
	}
	return len(w.entries), paths
}

// Compile-time assertion: InMemoryCorpusWriter satisfies the port.
var _ CorpusWriterPort = (*InMemoryCorpusWriter)(nil)
