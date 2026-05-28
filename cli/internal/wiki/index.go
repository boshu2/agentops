// This file implements wiki.WikiIndex — a persistent, content-addressed
// document index over one or more .agents/ corpus roots. It is the wiki
// bounded context's replacement for the dead term-index in
// cli/internal/search/index.go: instead of an inverted term map, WikiIndex
// stores one doc-record per file, keyed by path and pivoted on a content
// hash.
//
// The defining property is incremental reindex. When a single file changes,
// Reindex rewrites only that file's record; every unchanged record is carried
// forward byte-for-byte from the previous snapshot. The index persists as a
// JSONL doc-record store (one record per line), matching AgentOps corpus
// conventions and avoiding a CGO/SQLite dependency.
package wiki

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// indexableExtensions is the closed set of file extensions WikiIndex records.
// It matches the legacy search index (.md, .jsonl) so the replacement covers
// the same corpus surface.
var indexableExtensions = map[string]struct{}{
	".md":    {},
	".jsonl": {},
}

// isIndexable reports whether path has an extension WikiIndex records.
func isIndexable(path string) bool {
	_, ok := indexableExtensions[strings.ToLower(filepath.Ext(path))]
	return ok
}

// WikiIndex is a persistent, content-addressed document index over a set of
// corpus roots. It satisfies ports.WikiIndexPort.
//
// The index file is a JSONL doc-record store: one ports.WikiIndexRecord per
// line, path-sorted. WikiIndex resolves its roots through CorpusLocator, so
// cross-repo bases and the AO_AGENTS_DIR / AO_HOME overrides are honored.
//
// WikiIndex is not safe for concurrent use; serialize Reindex calls.
type WikiIndex struct {
	// indexPath is the JSONL file the index is persisted to.
	indexPath string
	// bases are the directories whose corpus roots are scanned. Each base
	// is resolved to its .agents/ dir via CorpusLocator.
	bases []string
	// locator resolves the .agents/ corpus directory for each base.
	locator CorpusLocator
}

// NewWikiIndex constructs a WikiIndex that persists to indexPath and scans the
// CorpusLocator-resolved .agents/ directory of each base in bases. At least
// one base is required.
func NewWikiIndex(indexPath string, bases ...string) (*WikiIndex, error) {
	if strings.TrimSpace(indexPath) == "" {
		return nil, fmt.Errorf("wiki index: indexPath is required")
	}
	if len(bases) == 0 {
		return nil, fmt.Errorf("wiki index: at least one base is required")
	}
	return &WikiIndex{indexPath: indexPath, bases: slices.Clone(bases)}, nil
}

// Reindex scans every indexable document under the configured corpus roots and
// updates the persisted JSONL index incrementally. A record is rewritten only
// when its content hash differs from the prior snapshot; records for missing
// files are dropped. The returned ports.WikiIndexResult names the Added,
// Updated, and Removed paths — unchanged records are intentionally absent.
func (w *WikiIndex) Reindex(ctx context.Context) (ports.WikiIndexResult, error) {
	var result ports.WikiIndexResult

	previous, err := w.load()
	if err != nil {
		return result, fmt.Errorf("load wiki index: %w", err)
	}

	scanned, err := w.scan(ctx)
	if err != nil {
		return result, fmt.Errorf("scan corpus: %w", err)
	}

	merged, result := mergeRecords(previous, scanned)

	if err := w.save(merged); err != nil {
		return ports.WikiIndexResult{}, fmt.Errorf("persist wiki index: %w", err)
	}
	return result, nil
}

// Records returns the current persisted index contents in path-sorted order.
// An absent index file yields an empty (non-nil) slice with no error.
func (w *WikiIndex) Records() ([]ports.WikiIndexRecord, error) {
	byPath, err := w.load()
	if err != nil {
		return nil, fmt.Errorf("load wiki index: %w", err)
	}
	out := make([]ports.WikiIndexRecord, 0, len(byPath))
	for _, rec := range byPath {
		out = append(out, rec)
	}
	slices.SortFunc(out, func(a, b ports.WikiIndexRecord) int {
		return strings.Compare(a.Path, b.Path)
	})
	return out, nil
}

// scan walks every configured corpus root and builds a fresh record for each
// indexable file found. The returned map is keyed by absolute path.
func (w *WikiIndex) scan(ctx context.Context) (map[string]ports.WikiIndexRecord, error) {
	out := make(map[string]ports.WikiIndexRecord)
	for _, base := range w.bases {
		root := w.locator.AgentsDir(base)
		if err := w.scanRoot(ctx, root, out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// scanRoot walks a single corpus root, adding a record for each indexable file
// to out. A non-existent root is treated as empty (no error) so callers can
// configure roots that do not yet exist.
func (w *WikiIndex) scanRoot(ctx context.Context, root string, out map[string]ports.WikiIndexRecord) error {
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat root %s: %w", root, err)
	}
	if !info.IsDir() {
		return nil
	}
	return filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk %s: %w", path, walkErr)
		}
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		if d.IsDir() || !isIndexable(path) {
			return nil
		}
		rec, err := buildRecord(root, path)
		if err != nil {
			return err
		}
		out[rec.Path] = rec
		return nil
	})
}

// buildRecord reads a file and produces its content-addressed index record.
func buildRecord(root, path string) (ports.WikiIndexRecord, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return ports.WikiIndexRecord{}, fmt.Errorf("resolve path %s: %w", path, err)
	}
	data, err := os.ReadFile(path) //nolint:gosec // path is corpus-root-bounded
	if err != nil {
		return ports.WikiIndexRecord{}, fmt.Errorf("read %s: %w", path, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return ports.WikiIndexRecord{}, fmt.Errorf("stat %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	return ports.WikiIndexRecord{
		Path:        abs,
		Root:        root,
		ContentHash: hex.EncodeToString(sum[:]),
		Size:        info.Size(),
		ModTimeUnix: info.ModTime().Unix(),
	}, nil
}

// mergeRecords reconciles a previous index snapshot against a freshly scanned
// one. It returns the merged record set plus a result classifying every path
// as Added, Updated, or Removed. A path present in both with an identical
// ContentHash carries the previous record forward unchanged — the incremental
// guarantee — and is reported in none of the result slices.
func mergeRecords(
	previous, scanned map[string]ports.WikiIndexRecord,
) (map[string]ports.WikiIndexRecord, ports.WikiIndexResult) {
	merged := make(map[string]ports.WikiIndexRecord, len(scanned))
	var result ports.WikiIndexResult

	for path, fresh := range scanned {
		prior, existed := previous[path]
		switch {
		case !existed:
			merged[path] = fresh
			result.Added = append(result.Added, path)
		case prior.ContentHash == fresh.ContentHash:
			// Unchanged: carry the prior record forward verbatim so its
			// persisted bytes are identical across reindexes.
			merged[path] = prior
		default:
			merged[path] = fresh
			result.Updated = append(result.Updated, path)
		}
	}
	for path := range previous {
		if _, stillPresent := scanned[path]; !stillPresent {
			result.Removed = append(result.Removed, path)
		}
	}

	slices.Sort(result.Added)
	slices.Sort(result.Updated)
	slices.Sort(result.Removed)
	return merged, result
}

// load reads the persisted JSONL index into a path-keyed map. An absent index
// file yields an empty map with no error. Malformed lines are skipped.
func (w *WikiIndex) load() (map[string]ports.WikiIndexRecord, error) {
	out := make(map[string]ports.WikiIndexRecord)
	f, err := os.Open(w.indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, fmt.Errorf("open %s: %w", w.indexPath, err)
	}
	defer func() { _ = f.Close() }() //nolint:errcheck // read-only, best-effort

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var rec ports.WikiIndexRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue // skip malformed lines
		}
		if rec.Path == "" {
			continue
		}
		out[rec.Path] = rec
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", w.indexPath, err)
	}
	return out, nil
}

// save writes the merged record set to the JSONL index atomically: it renders
// to a sibling temp file and renames it into place, so a crash mid-write
// cannot corrupt the index. Records are emitted in path-sorted order so
// successive snapshots are byte-comparable.
func (w *WikiIndex) save(records map[string]ports.WikiIndexRecord) error {
	if err := os.MkdirAll(filepath.Dir(w.indexPath), 0o750); err != nil {
		return fmt.Errorf("create index dir: %w", err)
	}

	ordered := make([]ports.WikiIndexRecord, 0, len(records))
	for _, rec := range records {
		ordered = append(ordered, rec)
	}
	slices.SortFunc(ordered, func(a, b ports.WikiIndexRecord) int {
		return strings.Compare(a.Path, b.Path)
	})

	tmp, err := os.CreateTemp(filepath.Dir(w.indexPath), ".wiki-index-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp index: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() //nolint:errcheck // no-op once renamed

	bw := bufio.NewWriter(tmp)
	for _, rec := range ordered {
		data, err := json.Marshal(rec)
		if err != nil {
			_ = tmp.Close() //nolint:errcheck // already failing
			return fmt.Errorf("marshal record %s: %w", rec.Path, err)
		}
		if _, err := bw.Write(append(data, '\n')); err != nil {
			_ = tmp.Close() //nolint:errcheck // already failing
			return fmt.Errorf("write record %s: %w", rec.Path, err)
		}
	}
	if err := bw.Flush(); err != nil {
		_ = tmp.Close() //nolint:errcheck // already failing
		return fmt.Errorf("flush index: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp index: %w", err)
	}
	if err := os.Rename(tmpName, w.indexPath); err != nil {
		return fmt.Errorf("rename temp index: %w", err)
	}
	return nil
}
