// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// productionCorpusReader satisfies ports.CorpusReaderPort by walking
// a corpus root (typically .agents/learnings/, .agents/findings/, or
// a combined directory) and ranking matches against the query.
//
// Ranking is intentionally simple — case-insensitive substring match
// of the query against title (h1 line or filename) and body, with a
// title hit weighted more than a body hit. This is not the full ao
// inject decay-ranker; it's a no-dependency reader that gives
// callers a typed entry point. Callers that need decay/citations
// should layer on top.
//
// File semantics:
//   - Walks rootDir recursively for *.md files. Non-md files are
//     skipped silently.
//   - Title is the first `# ` line, falling back to the filename.
//   - Body is the entire file contents.
//   - Empty Query returns all files (still up to Limit) with Score 0
//     — matches the port contract for "return everything ranked".
//   - DecayApplied is recorded but ignored by this reader (no decay
//     signal source in this adapter — documented per port contract).
type productionCorpusReader struct {
	rootDir string
}

func newProductionCorpusReader(rootDir string) *productionCorpusReader {
	return &productionCorpusReader{rootDir: rootDir}
}

// Lookup walks rootDir and returns matching CorpusItems sorted by
// Score descending. Missing rootDir is tolerated (returns empty).
func (r *productionCorpusReader) Lookup(ctx context.Context, opts ports.LookupOptions) ([]ports.CorpusItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := make([]ports.CorpusItem, 0)
	if r.rootDir == "" {
		return out, nil
	}
	queryLower := strings.ToLower(opts.Query)

	root, err := os.OpenRoot(r.rootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, fmt.Errorf("productionCorpusReader open root %q: %w", r.rootDir, err)
	}
	defer func() { _ = root.Close() }()

	err = filepath.WalkDir(r.rootDir, func(path string, d os.DirEntry, walkErr error) error {
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
		relPath, err := filepath.Rel(r.rootDir, path)
		if err != nil {
			return fmt.Errorf("productionCorpusReader resolve %q: %w", path, err)
		}
		body, err := root.ReadFile(relPath)
		if err != nil {
			return fmt.Errorf("productionCorpusReader read %q: %w", path, err)
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

// extractMarkdownTitle returns the first `# ` heading line or falls
// back to the filename when no h1 is present.
func extractMarkdownTitle(body, fallback string) string {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}
	return fallback
}

// scoreCorpusMatch is the ranking heuristic. Title hits weigh 2,
// body hits weigh 1. Empty query gives every entry score 0 (matches
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

// Compile-time assertion: productionCorpusReader satisfies the port.
var _ ports.CorpusReaderPort = (*productionCorpusReader)(nil)
