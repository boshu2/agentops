// Package corpus_fs is a real adapter for the BC1 Corpus ports
// (CorpusReaderPort + CorpusWriterPort), backed by the filesystem
// (.agents/learnings/, .agents/findings/, or any markdown corpus root).
//
// It absorbs the corpus-FS coupling that the CLI previously reached by
// walking the .agents/ knowledge store directly at each callsite (the
// productionCorpusReader/productionCorpusWriter types that lived in
// package main). Read maps to a root-scoped recursive *.md walk +
// substring ranker; write maps to a scoped, idempotent file write with
// optional YAML-frontmatter rendering.
//
// Read contract (CorpusReaderPort):
//
//   - Lookup walks rootDir recursively for *.md files via os.OpenRoot,
//     which scopes reads to the corpus root and rejects symlink escapes.
//   - Title is the first `# ` heading line, falling back to the filename.
//   - Body is the entire file contents.
//   - Empty Query returns all files (still up to Limit) with Score 0 —
//     matches the port contract for "return everything ranked".
//   - DecayApplied is accepted but ignored (no decay signal source in
//     this FS adapter — documented per port contract).
//   - A missing rootDir is tolerated and yields an empty slice.
//
// Write contract (CorpusWriterPort):
//
//   - req.Path is treated as relative to rootDir. Absolute paths and
//     parent-traversal ("..") are rejected to keep writes scoped.
//   - The parent directory is created (0o755) if missing.
//   - When req.Metadata is non-empty AND req.Body does not already begin
//     with a `---\n` frontmatter block, Metadata is rendered as a sorted
//     YAML frontmatter block at the top of the file (deterministic
//     re-Capture). A caller-provided frontmatter body is written as-is.
//   - Capture is idempotent: Created=true on first materialization,
//     Created=false on re-capture of an existing path (overwritten in
//     place).
//
// This adapter is pure filesystem — no subprocess — so there is no
// environment-stripping concern (cf. the exec-backed sibling adapters).
package corpus_fs

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

// ErrRootRequired is returned by Writer.Capture when the adapter was
// constructed with an empty rootDir. Callers should surface it directly.
var ErrRootRequired = errors.New("corpus_fs: rootDir required")

// ErrPathRequired is returned by Writer.Capture when req.Path is empty.
var ErrPathRequired = errors.New("corpus_fs: req.Path required")

// ErrPathEscapesRoot is returned by Writer.Capture when req.Path is
// absolute or traverses outside rootDir (e.g. "../escape.md").
var ErrPathEscapesRoot = errors.New("corpus_fs: req.Path escapes rootDir")

// Reader satisfies ports.CorpusReaderPort by walking a corpus root and
// ranking *.md matches against the query. Root is the directory to walk;
// an empty Root yields an empty Lookup.
type Reader struct {
	// Root is the corpus directory to walk (e.g. <project>/.agents/learnings).
	Root string
}

// NewReader returns a Reader rooted at root. Pass "" to get a reader
// whose Lookup always returns an empty slice.
func NewReader(root string) *Reader { return &Reader{Root: root} }

// Compile-time interface check.
var _ ports.CorpusReaderPort = (*Reader)(nil)

// Lookup walks Root and returns matching CorpusItems sorted by Score
// descending (ties broken by Path ascending for determinism). A missing
// Root is tolerated and returns an empty slice.
func (r *Reader) Lookup(ctx context.Context, opts ports.LookupOptions) ([]ports.CorpusItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := make([]ports.CorpusItem, 0)
	if r.Root == "" {
		return out, nil
	}
	queryLower := strings.ToLower(opts.Query)

	root, err := os.OpenRoot(r.Root)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, fmt.Errorf("corpus_fs: open root %q: %w", r.Root, err)
	}
	defer func() { _ = root.Close() }()

	err = filepath.WalkDir(r.Root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return filepath.SkipAll
			}
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		relPath, err := filepath.Rel(r.Root, path)
		if err != nil {
			return fmt.Errorf("corpus_fs: resolve %q: %w", path, err)
		}
		// root.ReadFile is scoped to Root and rejects symlink escapes.
		body, err := root.ReadFile(relPath)
		if err != nil {
			return fmt.Errorf("corpus_fs: read %q: %w", path, err)
		}
		bodyStr := string(body)
		title := extractMarkdownTitle(bodyStr, d.Name())
		score := scoreCorpusMatch(queryLower, strings.ToLower(title), strings.ToLower(bodyStr))
		if opts.Query != "" && score == 0 {
			return nil
		}
		out = append(out, ports.CorpusItem{
			Path:  path,
			Title: title,
			Body:  bodyStr,
			Score: score,
		})
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Path < out[j].Path
	})
	if opts.Limit > 0 && len(out) > opts.Limit {
		out = out[:opts.Limit]
	}
	return out, nil
}

// extractMarkdownTitle returns the first `# ` heading line or falls back
// to the filename when no h1 is present.
func extractMarkdownTitle(body, fallback string) string {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}
	return fallback
}

// scoreCorpusMatch is the ranking heuristic. Title hits weigh 2, body
// hits weigh 1. An empty query gives every entry score 0 (matches the
// "return everything ranked" port contract).
func scoreCorpusMatch(queryLower, titleLower, bodyLower string) float64 {
	if queryLower == "" {
		return 0
	}
	var s float64
	if strings.Contains(titleLower, queryLower) {
		s += 2
	}
	if strings.Contains(bodyLower, queryLower) {
		s += 1
	}
	return s
}

// Writer satisfies ports.CorpusWriterPort by writing CorpusWriteRequest
// bodies to disk under Root. Writes are serialized by a mutex so
// concurrent Capture calls do not interleave directory creation and file
// writes.
type Writer struct {
	mu sync.Mutex
	// Root is the corpus directory writes are scoped under.
	Root string
}

// NewWriter returns a Writer rooted at root. An empty root makes every
// Capture return ErrRootRequired.
func NewWriter(root string) *Writer { return &Writer{Root: root} }

// Compile-time interface check.
var _ ports.CorpusWriterPort = (*Writer)(nil)

// Capture writes req.Body (with optional rendered frontmatter) to
// Root/req.Path. Idempotent per the CorpusWriterPort contract.
func (w *Writer) Capture(ctx context.Context, req ports.CorpusWriteRequest) (ports.CorpusWriteResult, error) {
	if err := ctx.Err(); err != nil {
		return ports.CorpusWriteResult{}, err
	}
	if w.Root == "" {
		return ports.CorpusWriteResult{}, ErrRootRequired
	}
	if req.Path == "" {
		return ports.CorpusWriteResult{}, ErrPathRequired
	}
	if filepath.IsAbs(req.Path) {
		return ports.CorpusWriteResult{}, fmt.Errorf("%w: %q is absolute", ErrPathEscapesRoot, req.Path)
	}
	cleaned := filepath.Clean(req.Path)
	if strings.HasPrefix(cleaned, "..") || cleaned == ".." {
		return ports.CorpusWriteResult{}, fmt.Errorf("%w: %q traverses out", ErrPathEscapesRoot, req.Path)
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	full := filepath.Join(w.Root, cleaned)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return ports.CorpusWriteResult{}, fmt.Errorf("corpus_fs: mkdir %q: %w", filepath.Dir(full), err)
	}

	created := true
	if _, err := os.Stat(full); err == nil {
		created = false
	} else if !os.IsNotExist(err) {
		return ports.CorpusWriteResult{}, fmt.Errorf("corpus_fs: stat %q: %w", full, err)
	}

	body := renderCorpusBody(req.Body, req.Metadata)
	if err := os.WriteFile(full, body, 0o644); err != nil {
		return ports.CorpusWriteResult{}, fmt.Errorf("corpus_fs: write %q: %w", full, err)
	}
	return ports.CorpusWriteResult{ResolvedPath: full, Created: created}, nil
}

// renderCorpusBody injects YAML frontmatter from metadata unless body
// already starts with one. Empty metadata + non-frontmatter body returns
// body unchanged. Keys are emitted in sorted order so re-Capture is
// deterministic.
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
