// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// productionCorpusWriter satisfies ports.CorpusWriterPort by writing
// CorpusWriteRequest bodies to disk under a root directory. Pairs
// with cycle 112's productionCorpusReader — together they let BC1
// corpus persistence flow through typed Go entry points without
// reaching for the full ao mine/forge machinery.
//
// File semantics:
//   - req.Path is treated as relative to rootDir. Absolute paths and
//     parent-traversal (`..`) are rejected to keep writes scoped to
//     the corpus root.
//   - The file's parent directory is created (0o755) if missing.
//   - When req.Metadata is non-empty, it is rendered as a YAML
//     frontmatter block at the top of the file (--- … ---). Keys
//     are emitted in sorted order so re-Capture is deterministic.
//     If req.Body already starts with `---\n` (caller-provided
//     frontmatter), Metadata is NOT injected — the body is written
//     as-is. This matches the contract's "adapters MAY ignore
//     Metadata keys they don't model" with the strictest possible
//     non-collision rule.
//   - Created=true on first materialization; Created=false on
//     idempotent re-capture of an existing path (the file is
//     overwritten in place — idempotent per port contract).
type productionCorpusWriter struct {
	mu      sync.Mutex
	rootDir string
}

func newProductionCorpusWriter(rootDir string) *productionCorpusWriter {
	return &productionCorpusWriter{rootDir: rootDir}
}

// Capture writes req.Body (with optional rendered frontmatter) to
// rootDir/req.Path. Idempotent.
func (w *productionCorpusWriter) Capture(ctx context.Context, req ports.CorpusWriteRequest) (ports.CorpusWriteResult, error) {
	if err := ctx.Err(); err != nil {
		return ports.CorpusWriteResult{}, err
	}
	if w.rootDir == "" {
		return ports.CorpusWriteResult{}, errors.New("productionCorpusWriter: rootDir required")
	}
	if req.Path == "" {
		return ports.CorpusWriteResult{}, errors.New("productionCorpusWriter: req.Path required")
	}
	if filepath.IsAbs(req.Path) {
		return ports.CorpusWriteResult{}, fmt.Errorf("productionCorpusWriter: req.Path %q must be relative to rootDir", req.Path)
	}
	cleaned := filepath.Clean(req.Path)
	if strings.HasPrefix(cleaned, "..") || cleaned == ".." {
		return ports.CorpusWriteResult{}, fmt.Errorf("productionCorpusWriter: req.Path %q escapes rootDir", req.Path)
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	full := filepath.Join(w.rootDir, cleaned)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return ports.CorpusWriteResult{}, fmt.Errorf("productionCorpusWriter mkdir %q: %w", filepath.Dir(full), err)
	}

	created := true
	if _, err := os.Stat(full); err == nil {
		created = false
	} else if !os.IsNotExist(err) {
		return ports.CorpusWriteResult{}, fmt.Errorf("productionCorpusWriter stat %q: %w", full, err)
	}

	body := renderCorpusBody(req.Body, req.Metadata)
	if err := os.WriteFile(full, body, 0o644); err != nil {
		return ports.CorpusWriteResult{}, fmt.Errorf("productionCorpusWriter write %q: %w", full, err)
	}
	return ports.CorpusWriteResult{ResolvedPath: full, Created: created}, nil
}

// renderCorpusBody injects YAML frontmatter from metadata unless body
// already starts with one. Empty metadata + non-frontmatter body
// returns body unchanged.
func renderCorpusBody(body []byte, metadata map[string]string) []byte {
	if len(metadata) == 0 {
		return body
	}
	if len(body) >= 4 && string(body[:4]) == "---\n" {
		return body
	}
	keys := make([]string, 0, len(metadata))
	for k := range metadata {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out strings.Builder
	out.WriteString("---\n")
	for _, k := range keys {
		out.WriteString(k)
		out.WriteString(": ")
		out.WriteString(metadata[k])
		out.WriteByte('\n')
	}
	out.WriteString("---\n")
	out.Write(body)
	return []byte(out.String())
}

// Compile-time assertion: productionCorpusWriter satisfies the port.
var _ ports.CorpusWriterPort = (*productionCorpusWriter)(nil)
